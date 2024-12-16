package version

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"flag"
	"github.com/google/subcommands"
)

// Cmd implements the version subcommand of the open2opaque tool.
type Cmd struct{}

// Name implements subcommand.Command.
func (*Cmd) Name() string { return "version" }

// Synopsis implements subcommand.Command.
func (*Cmd) Synopsis() string { return "print tool version" }

// Usage implements subcommand.Command.
func (*Cmd) Usage() string { return `Usage: open2opaque version` }

// SetFlags implements subcommand.Command.
func (*Cmd) SetFlags(*flag.FlagSet) {}

func synthesizeVersion(info *debug.BuildInfo) string {
	const fallback = "(devel)"
	settings := make(map[string]string)
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}

	rev, ok := settings["vcs.revision"]
	if !ok {
		return fallback
	}

	commitTime, err := time.Parse(time.RFC3339Nano, settings["vcs.time"])
	if err != nil {
		return fallback
	}

	modifiedSuffix := ""
	if settings["vcs.modified"] == "true" {
		modifiedSuffix += "+dirty"
	}

	// Go pseudo versions use 12 hex digits.
	if len(rev) > 12 {
		rev = rev[:12]
	}

	// Copied from x/mod/module/pseudo.go
	const PseudoVersionTimestampFormat = "20060102150405"

	return fmt.Sprintf("v?.?.?-%s-%s%s",
		commitTime.UTC().Format(PseudoVersionTimestampFormat),
		rev,
		modifiedSuffix)
}

// Execute implements subcommand.Command.
func (cmd *Cmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	info, ok := debug.ReadBuildInfo()
	mainVersion := info.Main.Version
	if !ok {
		mainVersion = "<runtime/debug.ReadBuildInfo failed>"
	}
	// As of Go 1.24 (https://tip.golang.org/doc/go1.24#go-command),
	// we get v0.0.0-20241211143045-0af77b971425+dirty for git builds.
	if mainVersion == "(devel)" {
		// Before Go 1.24, the main module version just contained "(devel)" when
		// building from git. Try and find a git revision identifier.
		mainVersion = synthesizeVersion(info)
	}
	fmt.Printf("open2opaque %s\n", mainVersion)
	return subcommands.ExitSuccess
}

// Command returns an initialized Cmd for registration with the subcommands
// package.
func Command() *Cmd {
	return &Cmd{}
}
