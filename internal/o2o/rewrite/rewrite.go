// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rewrite implements the main open2opaque functionality: rewriting Go
// source code to use the opaque API.
package rewrite

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"flag"
	log "github.com/golang/glog"
	"github.com/google/subcommands"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/packages"
	"google.golang.org/open2opaque/internal/fix"
	"google.golang.org/open2opaque/internal/ignore"
	"google.golang.org/open2opaque/internal/o2o/errutil"
	"google.golang.org/open2opaque/internal/o2o/loader"
	"google.golang.org/open2opaque/internal/o2o/profile"
	"google.golang.org/open2opaque/internal/o2o/syncset"
	"google.golang.org/open2opaque/internal/o2o/wd"
	"google.golang.org/protobuf/proto"

	statspb "google.golang.org/open2opaque/internal/dashboard"
)

const kindTypeUsages = "typeusages"

// Cmd implements the rewrite subcommand of the open2opaque tool.
type Cmd struct {
	toUpdate              string
	toUpdateFile          string
	builderTypesFile      string
	builderLocationsFile  string
	levelsStr             string
	httpAddr              string
	outputFilterStr       string
	ignoreOutputFilterStr string
	parallelJobs          int
	dryRun                bool
	showWork              bool
	useBuilders           string
}

func (cmd *Cmd) levels() []string {
	return strings.Split(cmd.levelsStr, ",")
}

// Name implements subcommand.Command.
func (*Cmd) Name() string { return "rewrite" }

// Synopsis implements subcommand.Command.
func (*Cmd) Synopsis() string { return "Rewrite Go source code to use the Opaque API." }

// Usage implements subcommand.Command.
func (*Cmd) Usage() string {
	return `Usage: open2opaque rewrite -levels=yellow <target> [<target>...]

See http://godoc/3/net/proto2/go/open2opaque/open2opaque for documentation.

Command-line flag documentation follows:
`
}

// SetFlags implements subcommand.Command.
func (cmd *Cmd) SetFlags(f *flag.FlagSet) {
	const exampleMessageType = "google.golang.org/protobuf/types/known/timestamppb"

	f.StringVar(&cmd.toUpdate,
		"types_to_update",
		"",
		"Comma separated list of types to migrate. For example, '"+exampleMessageType+"'. Empty means 'all'. types_to_update_file overrides this flag.")

	f.StringVar(&cmd.toUpdateFile,
		"types_to_update_file",
		"",
		"Path to a file with one type to migrate per line. For example, '"+exampleMessageType+"'.")

	workdirHelp := "relative to the current directory"

	builderTypesFileDefault := ""
	f.StringVar(&cmd.builderTypesFile,
		"types_always_builders_file",
		builderTypesFileDefault,
		"Path to a file ("+workdirHelp+") with one type per line for which builders will always be used (instead of setters). For example, '"+exampleMessageType+"'.")

	builderLocationsFile := ""
	builderLocationsPath := "path"
	f.StringVar(&cmd.builderLocationsFile,
		"paths_always_builders_file",
		builderLocationsFile,
		"Path to a file ("+workdirHelp+") with one "+builderLocationsPath+" per line for which builders will always be used (instead of setters).")

	levelsHelp := ""
	f.StringVar(&cmd.levelsStr,
		"levels",
		"green",
		"Comma separated list of rewrite levels"+levelsHelp+". Levels can be: green, yellow, red. Each level includes the preceding ones: -levels=red enables green, yellow and red rewrites. Empty list means that no rewrites are requested; only analysis.")

	f.StringVar(&cmd.httpAddr,
		"http",
		"localhost:6060",
		"Address (host:port) to serve the net/http/pprof handlers on (for profiling).")

	f.StringVar(&cmd.outputFilterStr,
		"output_filter",
		".",
		"A regular expression that filters file names of files that should be written. This is useful, for example, to limit changes to '_test.go' files.")

	f.StringVar(&cmd.ignoreOutputFilterStr,
		"ignore_output_filter",
		"",
		"A regular expression that filters out file names of files that should be written. This is useful, for example, to limit changes to non-test files. It takes precedence over --output_filter.")

	f.IntVar(&cmd.parallelJobs,
		"parallel_jobs",
		20,
		"How many packages are analyzed in parallel.")

	f.BoolVar(&cmd.dryRun,
		"dry_run",
		false,
		"Do not modify any files, but run all the logic.")

	f.BoolVar(&cmd.showWork,
		"show_work",
		false,
		"For debugging: show your work mode. Logs to the INFO log every rewrite step that causes a change, and the diff of its changes.")

	useBuildersDefault := "everywhere"
	useBuildersHelp := ""
	useBuildersValues := "'tests', 'everywhere' and 'nowhere'"
	f.StringVar(&cmd.useBuilders,
		"use_builders",
		useBuildersDefault,
		"Determines where struct initialization rewrites will use builders instead of setters. Valid values are "+useBuildersValues+"."+useBuildersHelp)
}

// Execute implements subcommand.Command.
func (cmd *Cmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	if err := cmd.rewrite(ctx, f); err != nil {
		// Use fmt.Fprintf instead of log.Exit to generate a shorter error
		// message: users do not care about the current date/time and the fact
		// that our code lives in rewrite.go.
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

// Command returns an initialized Cmd for registration with the subcommands
// package.
func Command() *Cmd {
	return &Cmd{}
}

func (cmd *Cmd) rewrite(ctx context.Context, f *flag.FlagSet) error {
	subdir, err := wd.Adjust()
	if err != nil {
		return err
	}

	targets := f.Args()
	_ = subdir

	if len(targets) == 0 {
		f.Usage()
		return nil
	}
	return cmd.RewriteTargets(ctx, targets)
}

// RewriteTargets implements the rewrite functionality, and is called either
// from within this same package (open2opaque rewrite) or from the
// rewritepending package (open2opaque rewrite-pending wrapper).
func (cmd *Cmd) RewriteTargets(ctx context.Context, targets []string) error {

	targetsKind, err := verifyTargetsAreSameKind(targets)
	if err != nil {
		return err
	}
	if targetsKind == "unknown" {
		return fmt.Errorf("could not detect target kind of %q - neither a blaze target, nor a .go file, nor a go package import path (see http://godoc/3/net/proto2/go/open2opaque/open2opaque for instructions)", targets[0])
	}

	inputTypeUses := targetsKind == kindTypeUsages

	if inputTypeUses && len(targets) > 1 {
		return fmt.Errorf("When specifying the special value %q as target, you must not specify more than one target", kindTypeUsages)
	}

	if inputTypeUses && cmd.toUpdate == "" && cmd.toUpdateFile == "" {
		return fmt.Errorf("Please set either --types_to_update or --types_to_update_file to use %q", kindTypeUsages)
	}
	useSameClient := true

	outputFilterRe, err := regexp.Compile(cmd.outputFilterStr)
	if err != nil {
		return err
	}
	ignoreOutputFilterRe, err := regexp.Compile(cmd.ignoreOutputFilterStr)
	if err != nil {
		return err
	}

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Useful commands")
			fmt.Fprintln(w, "  go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30")
			fmt.Fprintln(w, "  go tool pprof http://localhost:6060/debug/pprof/heap")
			fmt.Fprintln(w, "  wget http://localhost:6060/debug/pprof/trace?seconds=5")
			fmt.Fprintln(w, "  wget http://localhost:6060/debug/pprof/goroutine?debug=2")
		})
		fmt.Println(http.ListenAndServe(cmd.httpAddr, nil))
	}()

	var lvls []fix.Level
	for _, lvl := range cmd.levels() {
		switch lvl {
		case "":
		case "green":
			lvls = append(lvls, fix.Green)
		case "yellow":
			if useSameClient {
				lvls = append(lvls, fix.Green)
			}
			lvls = append(lvls, fix.Yellow)
		case "red":
			if useSameClient {
				lvls = append(lvls, fix.Green)
				lvls = append(lvls, fix.Yellow)
			}
			lvls = append(lvls, fix.Red)
		default:
			return fmt.Errorf("unrecognized level name %q", lvl)
		}
	}

	toUpdateParts := strings.Split(cmd.toUpdate, ",")
	if len(toUpdateParts) == 1 && toUpdateParts[0] == "" {
		toUpdateParts = nil
	}
	typesToUpdate := newSet(toUpdateParts)
	if cmd.toUpdateFile != "" {
		b, err := os.ReadFile(cmd.toUpdateFile)
		if err != nil {
			log.ExitContext(ctx, err)
		}
		typesToUpdate = newSet(strings.Split(strings.TrimSpace(string(b)), "\n"))
	}

	builderTypes := map[string]bool{}
	if cmd.builderTypesFile != "" {
		fn := cmd.builderTypesFile
		b, err := os.ReadFile(fn)
		if err != nil {
			log.ExitContext(ctx, err)
		}
		builderTypes = newSet(strings.Split(strings.TrimSpace(string(b)), "\n"))
	}
	var builderLocations *ignore.List
	if cmd.builderLocationsFile != "" {
		fn := cmd.builderLocationsFile
		l, err := ignore.LoadList(fn)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			// File not found: user is running a newer tool in a client synced
			// to an older CL. Ignore and proceed without builderLocations.
		} else {
			builderLocations = l
		}
	}

	var pkgs []string
	switch targetsKind {

	case "go package import path":
		pkgs = targets

	default:
		return fmt.Errorf("BUG: unhandled targetsKind %q", targetsKind)
	}

	packagesToTargets := func(pkgs []string) ([]*loader.Target, error) {
		fmt.Printf("Resolving Go package names...\n")
		cfg := &packages.Config{
			Context: ctx,
		}
		loaded, err := packages.Load(cfg, pkgs...)
		if err != nil {
			return nil, err
		}
		targets := make([]*loader.Target, len(loaded))
		for idx, l := range loaded {
			targets[idx] = &loader.Target{ID: l.ID}
		}
		return targets, nil
	}

	targetsToRewrite, err := packagesToTargets(pkgs)
	if err != nil {
		return fmt.Errorf("can't read the package list: %v", err)
	}

	var builderUseType fix.BuilderUseType
	switch cmd.useBuilders {
	case "everywhere":
		builderUseType = fix.BuildersEverywhere
	case "nowhere":
		builderUseType = fix.BuildersNowhere
	case "tests":
		builderUseType = fix.BuildersTestsOnly
	default:
		return fmt.Errorf("invalid value for --use_builders flag. Valid values are 'tests', 'everywhere', 'everywhere-except-promising' and 'nowhere'")
	}

	cfg := &config{
		targets:              targetsToRewrite,
		typesToUpdate:        typesToUpdate,
		builderTypes:         builderTypes,
		builderLocations:     builderLocations,
		levels:               lvls,
		outputFilterRe:       outputFilterRe,
		ignoreOutputFilterRe: ignoreOutputFilterRe,
		useSameClient:        useSameClient,
		parallelJobs:         cmd.parallelJobs,
		dryRun:               cmd.dryRun,
		showWork:             cmd.showWork,
		useBuilder:           builderUseType,
	}

	if err := rewrite(ctx, cfg); err != nil {
		log.ExitContext(ctx, err)
	}

	return nil
}

// splitName splits a qualified Go declaration name into package path
// and bare identifier.
func splitName(name string) (pkgPath, ident string) {
	i := strings.LastIndex(name, ".")
	return name[:i], name[i+1:]
}

// keys returns the keys of set in sorted order.
func keys(set map[string]bool) []string {
	var res []string
	for k := range set {
		res = append(res, k)
	}
	sort.Strings(res)
	return res
}

type config struct {
	// pkgs is the list of Go packages to rewrite
	targets []*loader.Target

	// Name of the run. This ends up (for example) in client names.
	runName string

	// A set of types to consider when updating code
	// (e.g. "google.golang.org/protobuf/types/known/timestamppb").
	//
	// An empty (or nil) typesToUpdate means "update all types".
	typesToUpdate map[string]bool

	// A set of types for which to always use builders, not setters.
	// (e.g. "google.golang.org/protobuf/types/known/timestamppb").
	//
	// An empty (or nil) builderTypes means: use setters or builders for
	// production/test code respectively (or follow the -use_builders flag if
	// set).
	builderTypes map[string]bool

	builderLocations *ignore.List

	levels []fix.Level

	outputFilterRe, ignoreOutputFilterRe *regexp.Regexp

	useSameClient bool

	parallelJobs int

	dryRun bool

	showWork bool

	useBuilder fix.BuilderUseType
}

func (c *config) createLoader(ctx context.Context, dir string) (_ loader.Loader, cl int64, _ error) {

	fmt.Println("Starting the Blaze loader")
	l, err := loader.NewBlazeLoader(ctx, &loader.Config{}, dir)
	if err != nil {
		return nil, 0, err
	}
	return l, 0, nil
}

func rewrite(ctx context.Context, cfg *config) (err error) {
	defer errutil.Annotatef(&err, "rewrite() failed")

	log.InfoContextf(ctx, "Configuration: %+v", cfg)

	cutoff := ""
	if len(cfg.targets) > 50 {
		cutoff = " (listing first 50)"
	}
	fmt.Printf("rewriting %d packages:%s\n", len(cfg.targets), cutoff)
	for idx, t := range cfg.targets {
		fmt.Printf("  %s\n", t.ID)
		if idx >= 50 {
			break
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Start the loader before creating more clients. This allows loading typeinfo sstables while we wait on citc to setup clients.
	l, loaderCL, err := cfg.createLoader(ctx, wd)
	if err != nil {
		return err
	}
	defer l.Close(ctx)

	start := time.Now()
	resc := make(chan fixResult)

	pkgCfg := packageConfig{
		loader:               l,
		outputFilterRe:       cfg.outputFilterRe,
		ignoreOutputFilterRe: cfg.ignoreOutputFilterRe,
		dryRun:               cfg.dryRun,
		configuredPkg: fix.ConfiguredPackage{
			ProcessedFiles: syncset.New(), // avoid processing files multiple times
			ShowWork:       cfg.showWork,
			TypesToUpdate:  cfg.typesToUpdate,
			Levels:         cfg.levels,
			UseBuilders:    cfg.useBuilder,
		},
	}

	// Load and process targets in batches of up to cfg.parallelJobs
	// packages. This happens in a separate goroutine; the main goroutine just
	// collects and prints results.
	go func() {
		ln := len(cfg.targets)
		for idx := 0; idx < ln; idx += cfg.parallelJobs {
			end := idx + cfg.parallelJobs
			if end > ln {
				end = ln
			}
			fixPackageBatch(ctx, pkgCfg, cfg.targets[idx:end], resc)
		}
		close(resc)
	}()

	fmt.Printf("Loading packages (in batches of up to %d)...\n", cfg.parallelJobs)

	writtenByPath := make(map[string]bool)
	var total, fail int
	for res := range resc {
		profile.Add(res.ctx, "main/gotresp")

		total++
		if res.err != nil {
			fail++
		}

		for p := range res.written {
			writtenByPath[p] = true
		}

		tused := time.Since(start)
		tavg := tused / time.Duration(total)
		tleft := time.Duration(len(cfg.targets)-total) * tavg
		profile.Add(res.ctx, "done")

		fmt.Printf(`PROCESSED %d packages (total patterns: %d)
	Last package:         %s
	Total time:           %s
	Package profile:      %s
	Failures:             %d (%.2f%%)
	Average time:         %s
	Estimated until done: %s
	Estimated done at:    %s
	Error:                %v

`, total, len(cfg.targets), res.ruleName, tused, profile.Dump(res.ctx), fail, 100.0*float64(fail)/float64(total), tavg, tleft, time.Now().Add(tleft), res.err)

	}

	_ = loaderCL // Used in Google-internal code.

	writtenFiles := make([]string, 0, len(writtenByPath))
	for fname := range writtenByPath {
		writtenFiles = append(writtenFiles, fname)
	}
	sort.Strings(writtenFiles)

	successful := total - fail
	fmt.Printf("\nProcessed %d packages:\n", total)
	fmt.Printf("\tsuccessfully analyzed: %d\n", successful)
	fmt.Printf("\tfailed to load/rewrite: %d\n", fail)
	fmt.Printf("\t.go files rewritten: %d\n", len(writtenFiles))
	if len(writtenFiles) > 0 {
		fmt.Println("\nYou should see the modified files.")
		if err := fixBuilds("", writtenFiles); err != nil {
			fmt.Fprintf(os.Stderr, "Can't fix builds: %v\n", err)
		}
	}
	if fail > 0 {
		return fmt.Errorf("%d packages could not be rewritten", fail)
	}

	return nil
}

func runGoimports(dir string, files []string) error {
	fmt.Printf("\tRunning goimports on %d files\n", len(files))
	// Limit concurrent processes.
	parallelism := make(chan struct{}, 20)
	eg, ctx := errgroup.WithContext(context.Background())
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		eg.Go(func() error {
			parallelism <- struct{}{}
			defer func() { <-parallelism }()
			cmd := exec.CommandContext(ctx, "goimports", "-w", f)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("goimports -w %s failed: %s\n%s", f, err, out)
			}
			return nil
		})
	}
	return eg.Wait()
}

var fixBuilds = func(dir string, files []string) error {
	return runGoimports(dir, files)
}

type fixResult struct {
	ruleName string
	err      error
	stats    []*statspb.Entry
	ctx      context.Context
	drifted  []string
	written  map[string]bool
}

type packageConfig struct {
	loader               loader.Loader
	outputFilterRe       *regexp.Regexp
	ignoreOutputFilterRe *regexp.Regexp
	dryRun               bool
	configuredPkg        fix.ConfiguredPackage
}

func fixPackageBatch(ctx context.Context, cfg packageConfig, targets []*loader.Target, resc chan fixResult) {
	results := make(chan loader.LoadResult, len(targets))
	go func() {
		cfg.loader.LoadPackages(ctx, targets, results)
		close(results)
	}()

	var wg sync.WaitGroup
	for res := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := profile.NewContext(ctx)
			if err := res.Err; err != nil {
				resc <- fixResult{
					ruleName: res.Target.ID,
					err:      err,
					ctx:      ctx,
				}
				return
			}
			profile.Add(ctx, "main/scheduled")

			cfg := cfg // copy so that we can safely modify
			cfg.configuredPkg.Testonly = res.Target.Testonly
			cfg.configuredPkg.Loader = cfg.loader
			cfg.configuredPkg.Pkg = res.Package
			stats, drifted, written, err := fixPackage(ctx, cfg)
			profile.Add(ctx, "main/fixed")
			resc <- fixResult{
				ruleName: res.Target.ID,
				err:      err,
				stats:    stats,
				ctx:      ctx,
				drifted:  drifted,
				written:  written,
			}
		}()
	}
	wg.Wait()
}

// fixPackage loads a Go package
// from the input client, applies transformations to it, and writes results to
// the output client.
func fixPackage(ctx context.Context, cfg packageConfig) (stats []*statspb.Entry, drifted []string, written map[string]bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %s", r)
		}
	}()

	fixed, err := cfg.configuredPkg.Fix()
	if err != nil {
		return nil, nil, nil, err
	}
	profile.Add(ctx, "fix/fixed")

	written = make(map[string]bool)
	for _, lvl := range cfg.configuredPkg.Levels {
		for _, f := range fixed[lvl] {
			fname := f.Path
			if !f.Modified {
				log.InfoContextf(ctx, "Skipping writing [NOT MODIFIED] %s %s to %s: not modified", lvl, f.Path, fname)
				continue
			}
			if f.Generated {
				log.InfoContextf(ctx, "Skipping writing [GENERATED FILE] %s %s to %s: generated files can't be overwritten", lvl, f.Path, fname)
				continue
			}
			if strings.HasPrefix(f.Path, "net/proto2/go/") {
				log.InfoContextf(ctx, "Skipping writing [IGNORED PATH] %s %s to %s: generated files can't be overwritten", lvl, f.Path, fname)
				continue
			}
			if cfg.outputFilterRe.FindString(fname) == "" || cfg.ignoreOutputFilterRe.FindString(fname) != "" {
				log.InfoContextf(ctx, "Skipping writing [OUTPUT FILTER] %s %s to %s", lvl, f.Path, fname)
				continue
			}
			if cfg.dryRun {
				log.InfoContextf(ctx, "Skipping writing [DRY RUN] %s %s to %s", lvl, f.Path, fname)
				continue
			}
			if f.Drifted {
				drifted = append(drifted, f.Path)
			}
			log.InfoContextf(ctx, "Writing %s %s to %s", lvl, f.Path, fname)
			if err := os.WriteFile(fname, []byte(f.Code), 0644); err != nil {
				return nil, nil, nil, err
			}
			written[fname] = true
		}
	}
	profile.Add(ctx, "fix/wrotefiles")

	stats = fixed.AllStats()
	profile.Add(ctx, "fix/donestats")

	return stats, drifted, written, nil
}

func newSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	out := make(map[string]bool)
	for _, s := range ss {
		out[s] = true
	}
	return out
}

type rowAdder interface {
	AddRow(context.Context, proto.Message) error
}

type nullRowAdder struct{}

func (*nullRowAdder) AddRow(context.Context, proto.Message) error {
	return nil
}

func targetKind(target string) string {
	return "go package import path"
}

func verifyTargetsAreSameKind(targets []string) (string, error) {
	counters := make(map[string]int)
	targetKinds := make([]string, len(targets))
	for idx, target := range targets {
		kind := targetKind(target)
		targetKinds[idx] = kind
		counters[kind]++
	}
	if len(counters) > 1 {
		firstKind := targetKinds[0]
		otherIdx := 0
		for otherIdx < len(targetKinds)-1 && targetKinds[otherIdx] == firstKind {
			otherIdx++
		}
		return "", fmt.Errorf("target kinds unexpectedly not the same: target %q is of kind %q, but target %q is of kind %q", targets[0], firstKind, targets[otherIdx], targetKinds[otherIdx])
	}
	return targetKinds[0], nil
}
