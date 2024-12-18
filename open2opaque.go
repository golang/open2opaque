// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The program open2opaque migrates Go code using Go Protobuf from the Open API
// to the Opaque API.
//
// See https://go.dev/blog/protobuf-opaque for context.
package main

import (
	"context"
	"fmt"
	"io"
	_ "net/http/pprof"
	"os"
	"path"

	"flag"
	"github.com/google/subcommands"
	"google.golang.org/open2opaque/internal/o2o/rewrite"
	"google.golang.org/open2opaque/internal/o2o/setapi"
	"google.golang.org/open2opaque/internal/o2o/version"
)

const groupOther = "working with this tool"

func registerVersion(commander *subcommands.Commander) {
	commander.Register(version.Command(), groupOther)
}

func main() {
	ctx := context.Background()

	commander := subcommands.NewCommander(flag.CommandLine, path.Base(os.Args[0]))

	// Prepend general documentation before the regular help output.
	defaultExplain := commander.Explain
	commander.Explain = func(w io.Writer) {
		fmt.Fprintf(w, `The open2opaque tool migrates Go packages from the Go Protobuf Open Struct API to the Opaque API.

For documentation, see:
* https://go.dev/blog/protobuf-opaque
* https://protobuf.dev/reference/go/opaque-migration/

Report issues at https://github.com/golang/open2opaque/issues

`)
		defaultExplain(w)
	}

	// Comes last in the help output (alphabetically)
	commander.Register(commander.HelpCommand(), groupOther)
	commander.Register(commander.FlagsCommand(), groupOther)
	registerVersion(commander)

	// Comes first in the help output (alphabetically)
	const groupRewrite = "automatically rewriting Go code"
	commander.Register(rewrite.Command(), groupRewrite)

	const groupFlag = "managing the API level"
	commander.Register(setapi.Command(), groupFlag)

	flag.Usage = func() {
		commander.HelpCommand().Execute(ctx, flag.CommandLine)
	}

	flag.Parse()

	code := int(commander.Execute(ctx))
	logFlush()
	os.Exit(code)
}
