// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package args translates command-line arguments from proto message names or
// prefixes to proto file names.
package args

import (
	"context"
	"strings"
)

// ToProtoFilename translates the specified arg into a (filename, symbol) pair
// using go/global-protodb. If only a prefix is specified, symbol will be empty.
func ToProtoFilename(ctx context.Context, arg, kind string) (detectedKind, filename, symbol string, _ error) {
	if kind == "proto_filename" ||
		(kind == "autodetect" && strings.HasSuffix(arg, ".proto")) {
		return "proto_filename", arg, "", nil
	}

	return "", arg, "", nil
}
