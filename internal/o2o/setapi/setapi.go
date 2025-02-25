// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package setapi implements the setapi open2opaque subcommand, which sets the
// go_api_flag file option in .proto files.
package setapi

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"slices"
	"strings"

	"flag"
	"github.com/google/subcommands"
	"golang.org/x/sync/errgroup"
	pb "google.golang.org/open2opaque/internal/apiflagdata"
	"google.golang.org/open2opaque/internal/o2o/args"
	"google.golang.org/open2opaque/internal/protodetect"
	"google.golang.org/open2opaque/internal/protoparse"
	descpb "google.golang.org/protobuf/types/descriptorpb"
	gofeaturespb "google.golang.org/protobuf/types/gofeaturespb"
)

// Cmd implements the setapi subcommand of the open2opaque tool.
type Cmd struct {
	apiFlag     string
	inputFile   string
	skipCleanup bool
	maxProcs    uint
	protoFmt    string
	kind        string
}

// Name implements subcommand.Command.
func (*Cmd) Name() string { return "setapi" }

// Synopsis implements subcommand.Command.
func (*Cmd) Synopsis() string {
	return "Set the Go API level proto option."
}

// Usage implements subcommand.Command.
func (*Cmd) Usage() string {
	return `Usage: open2opaque setapi [-api=<` + strings.Join(validApis, "|") + `>] [-input_file=<input-list>] [<path/file.proto>] [<protopkg1>] [<protopkg2.MessageName>]

The setapi subcommand adjusts the Go API level option on the specified proto file(s) / package(s) / message(s).

The setapi subcommand can either read the proto file name(s) / package(s) / message(s)
from a text file (-input_file) or from the command line arguments, or both.

Command-line flag documentation follows:
`
}

// SetFlags implements subcommand.Command.
func (cmd *Cmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.apiFlag, "api", "OPAQUE", "set Go API level value to this, valid values: OPEN, HYBRID, and OPAQUE")
	f.StringVar(&cmd.inputFile, "input_file", "", "file containing list of proto source files / proto packages / proto messages to update (one per line)")
	f.BoolVar(&cmd.skipCleanup, "skip_cleanup", false, "skip the cleanup step, which removes the file-level flag if it equals the default and removes message flags if they equal the file-level API")
	f.UintVar(&cmd.maxProcs, "max_procs", 32, "max number of files concurrently processed")
	protofmtDefault := ""
	f.StringVar(&cmd.protoFmt, "protofmt", protofmtDefault, "if non-empty, a formatter program for .proto files")
}

// Execute implements subcommand.Command.
func (cmd *Cmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	if err := cmd.setapi(ctx, f); err != nil {
		// Use fmt.Fprintf instead of log.Exit to generate a shorter error
		// message: users do not care about the current date/time and the fact
		// that our code lives in setapi.go.
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

// Task defines the modification of the Go API level of a proto file or a
// particular message.
type Task struct {
	// Path of the proto file.
	Path string
	// The content of the proto file.
	Content []byte
	// If not empty, set the API level only for the message with this
	// package-local or fully-qualified name, e.g. Msg.NestedMsg or
	// pkgname.Msg.NestedMsg.
	// If empty, set the file-level API level.
	Symbol string

	TargetAPI gofeaturespb.GoFeatures_APILevel
	// If true, skip cleanup steps. If false, perform the following steps:
	// - if all messages in a file are on the same API level, set the whole file
	//   to that level.
	// - remove API flags from all messages that don't need it because they are
	//   on the same API level as the file (or of its parent message for nested
	//   messages in edition protos that use the new edition-feature API flag).
	SkipCleanup bool
	// A leading comment before the Go API flag prevents setapi from
	// modifying it. If a modification was prevent by this mechanism and
	// ErrorOnExempt is true, Process returns an error. Otherwise, the original
	// content is returned.
	ErrorOnExempt bool
}

func (cmd *Cmd) setapi(ctx context.Context, f *flag.FlagSet) error {
	api, err := parseAPIFlag(cmd.apiFlag)
	if err != nil {
		return err
	}

	var inputs []string
	if cmd.inputFile != "" {
		var err error
		if inputs, err = readInputFile(cmd.inputFile); err != nil {
			return fmt.Errorf("error reading file: %v", err)
		}
	}
	inputs = append(inputs, f.Args()...)

	var tasks []Task
	kind := cmd.kind
	if kind == "" {
		kind = "proto_filename"
	}
	for _, input := range inputs {
		_, filename, symbol, err := args.ToProtoFilename(ctx, input, kind)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		tasks = append(tasks, Task{
			Path:          filename,
			Symbol:        symbol,
			Content:       content,
			TargetAPI:     api,
			SkipCleanup:   cmd.skipCleanup,
			ErrorOnExempt: true,
		})
	}

	if len(tasks) == 0 {
		return fmt.Errorf("missing inputs, use either -list (one input per line) and / or pass input file name(s) / package(s) / message(s) as non-flag arguments")
	}

	// Error out if there are non-unique paths to avoid conflicting inputs like
	// editing the file-level flag of a file and a message flag in the same file.
	if conflicts := conflictingTasks(tasks); len(conflicts) > 0 {
		var l []string
		for _, c := range conflicts {
			l = append(l, c.Path)
		}
		return fmt.Errorf("conflicting, non-unique proto files in inputs: %s", strings.Join(l, ", "))
	}

	outputs := make([][]byte, len(tasks))

	protofmt := cmd.protoFmt
	if protofmt == "" {
		protofmt = "cat"
	}
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(int(cmd.maxProcs))
	for itask, task := range tasks {
		itask, task := itask, task
		eg.Go(func() error {
			var err error
			outputs[itask], err = Process(ctx, task, protofmt)
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("aborting without writing any proto files: %v", err)
	}

	// Write the outputs back to the input files.
	eg, ctx = errgroup.WithContext(ctx)
	eg.SetLimit(int(cmd.maxProcs))
	for itask, task := range tasks {
		itask, task := itask, task
		eg.Go(func() error {
			return os.WriteFile(task.Path, outputs[itask], 0644)
		})
	}
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error while writing proto files: %v", err)
	}
	return nil
}

var (
	apiMap = map[string]gofeaturespb.GoFeatures_APILevel{
		"OPEN":   gofeaturespb.GoFeatures_API_OPEN,
		"HYBRID": gofeaturespb.GoFeatures_API_HYBRID,
		"OPAQUE": gofeaturespb.GoFeatures_API_OPAQUE,

		// For convenience, allow enum names like API_HYBRID as synonyms.
		// These are used in editions feature options lines like:
		//
		//   option features.(pb.go).api_level = API_HYBRID;
		"API_OPEN":   gofeaturespb.GoFeatures_API_OPEN,
		"API_HYBRID": gofeaturespb.GoFeatures_API_HYBRID,
		"API_OPAQUE": gofeaturespb.GoFeatures_API_OPAQUE,
	}
	// Don't use the keys of apiMap to report valid values to the user, we want
	// in a specific order.
	validApis = []string{"OPEN", "HYBRID", "OPAQUE"}
)

func parseAPIFlag(flag string) (gofeaturespb.GoFeatures_APILevel, error) {
	if flag == "" {
		return gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED, fmt.Errorf("missing --api flag value")
	}
	if api, ok := apiMap[flag]; ok {
		return api, nil
	}
	return gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED, fmt.Errorf("invalid --api flag value: %v, valid values: %s", flag, strings.Join(validApis, ", "))
}

func readInputFile(name string) ([]string, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, line := range strings.Split(string(b), "\n") {
		if line := strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func conflictingTasks(tasks []Task) []Task {
	var conflicts []Task
	present := make(map[string]bool)
	for _, task := range tasks {
		if present[task.Path] {
			conflicts = append(conflicts, task)
		}
		present[task.Path] = true
	}
	return conflicts
}

var logf = func(format string, a ...any) { fmt.Fprintf(os.Stderr, "[setapi] "+format+"\n", a...) }

func parse(path string, content []byte, skipMessages bool) (*protoparse.FileOpt, error) {
	parser := protoparse.NewParserWithAccessor(func(string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content)), nil
	})
	fopt, err := parser.ParseFile(path, skipMessages)
	if err != nil {
		return nil, fmt.Errorf("protoparse.ParseFile: %v", err)
	}
	return fopt, nil
}

func parentAPI(path string, content []byte, msgName string) (gofeaturespb.GoFeatures_APILevel, error) {
	fopt, err := parse(path, content, false)
	if err != nil {
		return gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED, err
	}
	msgName = strings.TrimPrefix(msgName, fopt.Package+".")
	var mopt *protoparse.MessageOpt
	for _, moptLoop := range fopt.MessageOpts {
		if o := findMsg(moptLoop, msgName); o != nil {
			mopt = o
			break
		}
	}
	if mopt == nil {
		return gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED, fmt.Errorf("cannot find message %q", msgName)
	}
	result := fopt.GoAPI
	if fopt.Syntax == "editions" && mopt.Parent != nil {
		result = mopt.Parent.GoAPI
	}
	return result, nil
}

func traverseMsgTree(opt *protoparse.MessageOpt, f func(*protoparse.MessageOpt) error) error {
	if err := f(opt); err != nil {
		return err
	}
	for _, c := range opt.Children {
		if err := traverseMsgTree(c, f); err != nil {
			return err
		}
	}
	return nil
}

// Process modifies the API level of a proto file or of a particular message in
// a proto file, see the doc comment of the type Task for more details. Before
// returning the modified file content, the file is formatted by executing
// formatter (use "cat" if you don't have a formatter handy). This function
// doesn't modify the []byte task.Content.
func Process(ctx context.Context, task Task, formatter string) ([]byte, error) {
	if task.Path == "" {
		return nil, fmt.Errorf("path is empty")
	}
	if len(task.Content) == 0 {
		return nil, fmt.Errorf("content is empty")
	}

	// Clone the input file content in case the caller will use the slice after
	// the call.
	content := slices.Clone(task.Content)

	var err error
	if task.Symbol == "" {
		content, err = setFileAPI(task.Path, content, task.TargetAPI, task.SkipCleanup, task.ErrorOnExempt)
		if err != nil {
			return nil, fmt.Errorf("setFileAPI: %v", err)
		}
	} else {
		parentAPI, err := parentAPI(task.Path, content, task.Symbol)
		if err != nil {
			return nil, fmt.Errorf("parentAPI: %v", err)
		}
		content, err = setMsgAPI(task.Path, content, task.Symbol, parentAPI, task.TargetAPI, task.SkipCleanup)
		if err != nil {
			if errors.Is(err, ErrLeadingCommentPreventsEdit) && !task.ErrorOnExempt {
				// Don't error out if leading comment exempted edit but ErrorOnExempt
				// is false. Could write this a lot more compact, but this makes it easy
				// to understand.
			} else {
				return nil, err
			}
		}
	}

	if !task.SkipCleanup {
		content, err = cleanup(task.Path, content)
		if err != nil {
			return nil, fmt.Errorf("cleanup: %v", err)
		}
	}
	content, err = FormatFile(ctx, content, formatter)
	if err != nil {
		return nil, fmt.Errorf("FormatFile: %v", err)
	}
	return content, nil
}

func fromFeatureToOld(apiLevel gofeaturespb.GoFeatures_APILevel) pb.GoAPI {
	switch apiLevel {
	case gofeaturespb.GoFeatures_API_OPEN:
		return pb.GoAPI_OPEN_V1

	case gofeaturespb.GoFeatures_API_HYBRID:
		return pb.GoAPI_OPEN_TO_OPAQUE_HYBRID

	case gofeaturespb.GoFeatures_API_OPAQUE:
		return pb.GoAPI_OPAQUE_V0

	default:
		panic(fmt.Sprintf("unknown apilevel %v", apiLevel))
	}
}

func byteRangeWithEOLComment(in []byte, tr protoparse.TextRange) (beginByte, endByte int, err error) {
	beginByte, endByte, err = tr.ToByteRange(in)
	if err != nil {
		return -1, -1, fmt.Errorf("tr.ToByteRange: %v", err)
	}
	if tr.BeginLine == tr.EndLine {
		if nextNewlineByte := bytes.IndexByte(in[endByte:], '\n'); nextNewlineByte != -1 {
			if lineSuffix := in[endByte : endByte+nextNewlineByte]; bytes.HasPrefix(bytes.TrimSpace(lineSuffix), []byte("//")) {
				endByte += nextNewlineByte
			}
		}
	}
	return beginByte, endByte, nil
}

func replaceTextRange(in []byte, tr protoparse.TextRange, insert []byte) ([]byte, error) {
	beginByte, endByte, err := tr.ToByteRange(in)
	if err != nil {
		return nil, fmt.Errorf("tr.ToByteRange: %v", err)
	}
	in = slices.Delete(in, beginByte, endByte)
	return slices.Insert(in, beginByte, insert...), nil
}

func levelLineFor(fopt *protoparse.FileOpt, targetAPI gofeaturespb.GoFeatures_APILevel) (string, error) {
	if fopt.Syntax == "editions" {
		return fmt.Sprintf("option features.(pb.go).api_level = %s;", targetAPI), nil
	}
	return "", fmt.Errorf("file %s is not using Protobuf Editions (https://protobuf.dev/editions/overview/) yet", fopt.File)
}

func setFileAPI(path string, content []byte, targetAPI gofeaturespb.GoFeatures_APILevel, skipCleanup, errorOnExempt bool) ([]byte, error) {
	fopt, err := parse(path, content, true)
	if err != nil {
		return nil, err
	}
	if fopt.IsExplicit && fopt.APIInfo == nil {
		return nil, fmt.Errorf("BUG: fopt.APIInfo is nil")
	}

	if defaultAPI := protodetect.DefaultFileLevel(path); defaultAPI == targetAPI {
		if !fopt.IsExplicit {
			logf("File %s is already on the target API level by default, doing nothing", path)
			return content, nil
		}
		if fopt.APIInfo.HasLeadingComment {
			if errorOnExempt {
				return nil, fmt.Errorf("API flag of file %s has a leading comment that prevents removing it", path)
			}
			logf("API flag of file %s has a leading comment that prevents removing it", path)
			return content, nil
		}
		if fopt.GoAPI == targetAPI && skipCleanup {
			logf("skipping cleanup: not removing the API flag of file %s although it's at the default API level", path)
			return content, nil
		}
		from, to, err := byteRangeWithEOLComment(content, fopt.APIInfo.TextRange)
		if err != nil {
			return nil, fmt.Errorf("byteRangeWithEOLComment: %v", err)
		}
		logf("Removing the API flag of file %s to use the default API level", path)
		return slices.Delete(content, from, to), nil
	}

	levelLine, err := levelLineFor(fopt, targetAPI)
	if err != nil {
		return nil, err
	}

	if !fopt.IsExplicit {
		logf("Inserting API flag %q into file %s", targetAPI, path)

		ln, err := fileOptionLineNumber(fopt.SourceCodeInfo)
		if err != nil {
			return nil, fmt.Errorf("fileOptionLineNumber: %v", err)
		}
		return insertLine(content, levelLine, ln), nil
	}

	// File is currently explicitly set to API value and target API isn't the
	// default API.
	if fopt.GoAPI == targetAPI {
		logf("File %s is already on the target API level, do nothing", path)
		return content, nil
	}
	if fopt.APIInfo.HasLeadingComment {
		if errorOnExempt {
			return nil, fmt.Errorf("API flag of file %s has a leading comment that prevents replacing it", path)
		}
		logf("API flag of file %s has a leading comment that prevents replacing it", path)
		return content, nil
	}
	logf("Replacing the API flag of file %s", path)
	content, err = replaceTextRange(content, fopt.APIInfo.TextRange, []byte(levelLine))
	if err != nil {
		return nil, fmt.Errorf("replaceTextRange: %v", err)
	}
	return content, nil
}

func findMsg(opt *protoparse.MessageOpt, name string) *protoparse.MessageOpt {
	if opt.Message == name {
		return opt
	}
	for _, c := range opt.Children {
		if childOpt := findMsg(c, name); childOpt != nil {
			return childOpt
		}
	}
	return nil
}

// ErrLeadingCommentPreventsEdit signals that no modifications were made because
// a leading comment exempted an API flag from modification. This error is
// ignored if Task.ErrorOnExempt is false.
var ErrLeadingCommentPreventsEdit = errors.New("leading comment prevents edit, check the logs for more details")

func setMsgAPI(path string, content []byte, msgName string, parentAPI, targetAPI gofeaturespb.GoFeatures_APILevel, skipCleanup bool) ([]byte, error) {
	// This function is called recursively. Parse content every time because the
	// position information changes when parent messages are manipulated.
	fopt, err := parse(path, content, false)
	if err != nil {
		return nil, err
	}
	msgName = strings.TrimPrefix(msgName, fopt.Package+".")
	var mopt *protoparse.MessageOpt
	for _, moptLoop := range fopt.MessageOpts {
		if o := findMsg(moptLoop, msgName); o != nil {
			mopt = o
			break
		}
	}
	if mopt == nil {
		return nil, fmt.Errorf("cannot find massage %q", msgName)
	}
	if mopt.IsExplicit && mopt.APIInfo == nil {
		return nil, fmt.Errorf("BUG: mopt.APIInfo is nil")
	}

	if parentAPI == targetAPI {
		if !mopt.IsExplicit {
			logf("Message %q is already on the target API level, doing nothing", msgName)
			return content, nil
		}
		if mopt.APIInfo.HasLeadingComment {
			logf("Changing API level of message %q was prevented by a leading comment of the API flag", msgName)
			if mopt.GoAPI != targetAPI {
				// Report an error to abort the whole operation if we would have
				// changed the API flag instead of just cleaning it up because it's the
				// same as the parent. If we didn't, we might change nested messages to
				// the wrong API level.
				return nil, ErrLeadingCommentPreventsEdit
			}
			return content, nil
		}
		if mopt.GoAPI != targetAPI && fopt.Syntax == "editions" {
			logf("Before changing API flag of message %q, descending into children to prevent recursive change of API level", msgName)
			for _, child := range mopt.Children {
				content, err = setMsgAPI(path, content, child.Message, targetAPI, child.GoAPI, skipCleanup)
				if err != nil {
					return nil, err
				}
			}
		}
		if mopt.GoAPI == targetAPI && skipCleanup {
			logf("skipping cleanup: not removing the API flag of message %q although it's at the parent API level", msgName)
			return content, nil
		}
		from, to, err := byteRangeWithEOLComment(content, mopt.APIInfo.TextRange)
		if err != nil {
			return nil, fmt.Errorf("byteRangeWithEOLComment: %v", err)
		}
		logf("Removing the API flag of message %q to use the parent API level", msgName)
		return slices.Delete(content, from, to), nil
	}

	if !mopt.IsExplicit {
		if fopt.Syntax == "editions" {
			logf("Before changing API flag of message %q, descending into children to prevent recursive change of API level", msgName)
			for _, child := range mopt.Children {
				content, err = setMsgAPI(path, content, child.Message, targetAPI, child.GoAPI, skipCleanup)
				if err != nil {
					return nil, err
				}
			}
		}
		logf("Inserting API flag %q into message %q", targetAPI, msgName)
		idx, err := msgOptionInsertionByteIdx(content, fopt.SourceCodeInfo, mopt.LocPath)
		if err != nil {
			return nil, fmt.Errorf("msgOptionInsertionByteIdx: %v", err)
		}
		insertion := fmt.Sprintf("\noption go_api_flag = %q;", fromFeatureToOld(targetAPI))
		if fopt.Syntax == "editions" {
			insertion = fmt.Sprintf("\noption features.(pb.go).api_level = %s;", targetAPI)
		}
		return slices.Concat(content[:idx], []byte(insertion), content[idx:]), nil
	}

	// Message is currently explicitly set to API value and target API isn't the
	// parent API.
	if mopt.GoAPI == targetAPI {
		logf("Message %q is already on the target API level, do nothing", msgName)
		return content, nil
	}
	if mopt.APIInfo.HasLeadingComment {
		logf("Changing API level of message %q was prevented by a leading comment of the API flag", msgName)
		return content, ErrLeadingCommentPreventsEdit
	}
	if fopt.Syntax == "editions" {
		logf("Before changing API flag of message %q, descending into children to prevent recursive change of API level", msgName)
		for _, child := range mopt.Children {
			content, err = setMsgAPI(path, content, child.Message, targetAPI, child.GoAPI, skipCleanup)
			if err != nil {
				return nil, err
			}
		}
	}
	logf("Replacing the API flag of message %q", msgName)
	insert := fmt.Sprintf("option go_api_flag = %q;", fromFeatureToOld(targetAPI))
	if fopt.Syntax == "editions" {
		insert = fmt.Sprintf("option features.(pb.go).api_level = %s;", targetAPI)
	}
	content, err = replaceTextRange(content, mopt.APIInfo.TextRange, []byte(insert))
	if err != nil {
		return nil, fmt.Errorf("replaceTextRange: %v", err)
	}
	return content, nil
}

func cleanup(path string, content []byte) ([]byte, error) {
	// Cleanup 1: If all messages are on the same API level, flip the file API
	// level to that value. If said level is the default for the path, don't use
	// an explicit flag.
	var err error
	content, err = cleanupAllMsgsToFile(path, content)
	if err != nil {
		return nil, err
	}

	// Cleanup 2: Remove the API flag from all messages that have the same API
	// level as their parents. The parent is usually the file API level, but in
	// case of editions proto that use the new edition feature, the parent of
	// a nested message is its parent message.
	//
	// Parse again after the previous cleanup might have modified content.
	content, err = cleanupSameAsParent(path, content)
	if err != nil {
		return nil, err
	}

	// Cleanup 3: ensure the go_features.proto import is present if and only if
	// any parts of the file set the features.(pb.go).api_level option.
	return cleanupGoFeaturesImport(path, content)
}

func cleanupAllMsgsToFile(path string, content []byte) ([]byte, error) {
	fopt, err := parse(path, content, false)
	if err != nil {
		return nil, err
	}
	apiMap := map[gofeaturespb.GoFeatures_APILevel]struct{}{}
	for _, moptLoop := range fopt.MessageOpts {
		traverseMsgTree(moptLoop, func(mopt *protoparse.MessageOpt) error {
			apiMap[mopt.GoAPI] = struct{}{}
			return nil
		})
	}
	if len(apiMap) != 1 {
		// Not all messages are on the same API level.
		return content, nil
	}
	targetAPI := fopt.MessageOpts[0].GoAPI
	if fopt.GoAPI == targetAPI {
		// File is already on the target API level, nothing to do.
		return content, nil
	}
	// All messages have same API level.
	// Consider cleanup as a nice to have, don't error out on leading-comment
	// exemptions.
	const errorOnExempt = false
	const skipCleanup = false
	content, err = setFileAPI(path, content, targetAPI, skipCleanup, errorOnExempt)
	if err != nil {
		return nil, fmt.Errorf("setFileAPI: %v", err)
	}
	return content, nil
}

func cleanupSameAsParent(path string, content []byte) ([]byte, error) {
	fopt, err := parse(path, content, false)
	if err != nil {
		return nil, err
	}
	// Collect all byte ranges of API flags that should be removed.
	type byteRange struct {
		from, to int
	}
	var removeByteRanges []byteRange
	for _, moptLoop := range fopt.MessageOpts {
		if err := traverseMsgTree(moptLoop, func(mopt *protoparse.MessageOpt) error {
			if !mopt.IsExplicit {
				return nil
			}
			if mopt.APIInfo == nil {
				return fmt.Errorf("BUG: mopt.APIInfo is nil")
			}
			if mopt.APIInfo.HasLeadingComment {
				return nil
			}
			parentAPI := fopt.GoAPI
			if fopt.Syntax == "editions" && mopt.Parent != nil {
				parentAPI = mopt.Parent.GoAPI
			}
			if mopt.GoAPI != parentAPI {
				return nil
			}
			// Message has same API level as parent (or file) and API flag is
			// explicit.
			from, to, err := byteRangeWithEOLComment(content, mopt.APIInfo.TextRange)
			if err != nil {
				return fmt.Errorf("TextRange.ToByteRange: %v", err)
			}
			removeByteRanges = append(removeByteRanges, byteRange{from: from, to: to})
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if len(removeByteRanges) == 0 {
		return content, nil
	}

	slices.SortFunc(removeByteRanges, func(a, b byteRange) int {
		return cmp.Compare(a.from, b.from)
	})
	for i := 1; i < len(removeByteRanges); i++ {
		if removeByteRanges[i].from < removeByteRanges[i-1].to {
			return nil, fmt.Errorf("text ranges overlap")
		}
	}

	for _, br := range slices.Backward(removeByteRanges) {
		content = slices.Delete(content, br.from, br.to)
	}
	return content, nil
}

func insertLine(content []byte, insertLine string, ln int32) []byte {
	lines := bytes.Split(content, []byte{'\n'})
	result := slices.Clone(lines[:ln])
	result = append(result, []byte(insertLine))
	result = append(result, lines[ln:]...)
	return bytes.Join(result, []byte{'\n'})
}

func messageUsesFeature(mopt *protoparse.MessageOpt) bool {
	if mopt.IsExplicit {
		return true // editions feature set at message level
	}
	for _, mo := range mopt.Children {
		if messageUsesFeature(mo) {
			return true
		}
	}
	return false
}

func usesFeature(fopt *protoparse.FileOpt) bool {
	if fopt.Syntax != "editions" {
		return false // only files on editions can use editions features
	}
	if fopt.IsExplicit {
		return true // editions feature set at file level
	}
	for _, mo := range fopt.MessageOpts {
		if messageUsesFeature(mo) {
			return true
		}
	}
	return false
}

func cleanupGoFeaturesImport(path string, content []byte) ([]byte, error) {
	const featuresProto = "google/protobuf/go_features.proto"
	fopt, err := parse(path, content, false)
	if err != nil {
		return nil, fmt.Errorf("parse: %v", err)
	}
	desc := fopt.Desc
	goFeaturesImported := slices.ContainsFunc(
		desc.GetDependency(),
		func(path string) bool {
			return path == featuresProto
		})
	usesFeature := usesFeature(fopt)

	if usesFeature && !goFeaturesImported {
		ln, err := featuresImportLineNumber(desc.GetSourceCodeInfo())
		if err != nil {
			return nil, err
		}
		importLine := fmt.Sprintf("import %q;", featuresProto)
		return insertLine(content, importLine, ln), nil
	}

	if !usesFeature && goFeaturesImported {
		importNum := 0
		for _, loc := range desc.GetSourceCodeInfo().GetLocation() {
			path := loc.GetPath()
			span := loc.GetSpan()
			if len(path) < 1 {
				continue
			}
			if path[0] != importFieldNum {
				continue
			}
			// Found an import. Is it the go_features.proto import?
			if max := len(desc.GetDependency()); importNum >= max {
				return nil, fmt.Errorf("BUG: too many imports in SourceCodeInfo: got index %d, want at most %d", importNum, max)
			}
			imported := desc.GetDependency()[importNum]
			importNum++
			if imported != featuresProto {
				continue
			}
			ln := spanEndLine(span)
			lines := bytes.Split(content, []byte{'\n'})
			result := slices.Clone(lines[:ln])
			// lines[ln] deleted by omitting it.
			result = append(result, lines[ln+1:]...)
			return bytes.Join(result, []byte{'\n'}), nil
		}
		return nil, fmt.Errorf("BUG: could not locate import line in SourceCodeInfo")
	}

	return content, nil
}

func spanEndLine(span []int32) int32 {
	// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L1209
	if len(span) == 4 {
		return span[2]
	}
	return span[0]
}

const (
	// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L105-L137
	syntaxFieldNum      = 12
	editionFieldNum     = 14
	editionDeprFieldNum = 13
	packageFieldNum     = 2
	importFieldNum      = 3
	optionFieldNum      = 8
)

func fieldNumToLine(info *descpb.SourceCodeInfo) map[int32]int32 {
	// pos contains a mapping of proto field numbers to line numbers. For syntax
	// and package constructs, the line numbers represent where they are declared.
	// For import and options, the line number represents the last import/option
	// if it exists.
	pos := map[int32]int32{}
	for _, loc := range info.GetLocation() {
		path := loc.GetPath()
		span := loc.GetSpan()
		if len(path) < 1 {
			continue
		}
		switch fieldNum := path[0]; fieldNum {
		case syntaxFieldNum, editionFieldNum, editionDeprFieldNum, packageFieldNum, importFieldNum, optionFieldNum:
			endLine := spanEndLine(span)
			// If not set, map returns 0.
			if endLine > pos[fieldNum] {
				pos[fieldNum] = endLine
			}
		}
	}
	return pos
}

// featuresImportLineNumber returns the line number to insert the file-level
// go_features.proto import. It uses the following heuristics to determine the
// line number:
// If there are any import lines, return the line number after the last one.
// If there is a package statement line, return the line number after it.
// If there is a syntax statement line, return the line number after it.
// Note that this func assumes that an earlier protoparser.Parser.ParseFile
// invocation will return error already if there is no syntax statement.
func featuresImportLineNumber(info *descpb.SourceCodeInfo) (int32, error) {
	pos := fieldNumToLine(info)

	if lnum, ok := pos[importFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[packageFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[syntaxFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[editionFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[editionDeprFieldNum]; ok {
		return lnum + 1, nil
	}

	return 0, fmt.Errorf("cannot determine line number for go_features.proto import")
}

// fileOptionLineNumber returns the line number to insert the file-level
// go_api_flag option. It uses the following heuristics to determine the
// line number:
// If there are any file option settings, return the line number after the last
// one.
// If there are any import lines, return the line number after the last one.
// If there is a package statement line, return the line number after it.
// If there is a syntax statement line, return the line number after it.
// Note that this func assumes that an earlier protoparser.Parser.ParseFile
// invocation will return error already if there is no syntax statement.
func fileOptionLineNumber(info *descpb.SourceCodeInfo) (int32, error) {
	pos := fieldNumToLine(info)

	if lnum, ok := pos[optionFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[importFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[packageFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[syntaxFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[editionFieldNum]; ok {
		return lnum + 1, nil
	}
	if lnum, ok := pos[editionDeprFieldNum]; ok {
		return lnum + 1, nil
	}

	return 0, fmt.Errorf("cannot determine line number for file-level API flag")
}

func msgOptionInsertionByteIdx(content []byte, info *descpb.SourceCodeInfo, msgPath []int32) (int, error) {
	const (
		// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L105
		nameFieldNum = 1
	)
	namePath := append(slices.Clone(msgPath), nameFieldNum)
	nameEndIdx := math.MaxInt

	for _, loc := range info.GetLocation() {
		path := loc.GetPath()
		if len(path) <= len(msgPath) {
			// Paths in the message (name, fields, option, ...) have to be longer than
			// the message path.
			continue
		}
		if slices.Equal(path, namePath) {
			var err error
			_, nameEndIdx, err = protoparse.SpanToTextRange(loc.GetSpan()).ToByteRange(content)
			if err != nil {
				return -1, fmt.Errorf("TextRange.ToByteRange: %v", err)
			}
			break
		}
	}
	if nameEndIdx == math.MaxInt {
		return -1, fmt.Errorf("BUG: cannot find message name")
	}

	// Look for the next opening curly after the name. If this happens to be in a
	// comment, this function will produce wrong results.
	offset := bytes.IndexByte(content[nameEndIdx:], '{')
	if offset == -1 {
		return -1, fmt.Errorf(`cannot find "{"`)
	}
	return nameEndIdx + offset + 1, nil
}

// FormatFile runs formatter on input and returns the formatted result.
func FormatFile(ctx context.Context, input []byte, formatter string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, formatter)
	cmd.Stdin = bytes.NewReader(input)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("formatter error: %v", err)
	}
	if stderr.Len() > 0 {
		return nil, fmt.Errorf("formatter stderr: %s", stderr.String())
	}
	return stdout.Bytes(), nil
}
