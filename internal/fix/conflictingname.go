// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/types"
	"strings"
)

func fixConflictingNames(t types.Type, prefix, name string) string {
	if prefix != "" {
		if pt, ok := t.(*types.Pointer); ok {
			t = pt.Elem()
		}
		st := t.(*types.Named).Underlying().(*types.Struct)
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Name() == prefix+name {
				return "_" + name
			}
		}
	}
	// A "build" field in the protocol buffer results in a "Build_" field in
	// the builder (the "_" is added to avoid conflicts with the "Build"
	// method). As a result, the generator renames all "Build" accessor
	// methods in the message (?!).
	if name == "Build" {
		name = "Build_"
	}
	// Before the migration, the following names conflicted with methods on
	// the message struct. The corresponding accessor methods are deprecated
	// and we should use versions without the "_" suffix.
	for _, s := range []string{"Reset_", "String_", "ProtoMessage_", "Descriptor_"} {
		if name == s {
			name = strings.TrimSuffix(name, "_")
		}
	}
	return name
}

// See usedNames in
// https://go.googlesource.com/protobuf/+/refs/heads/master/compiler/protogen/protogen.go
var conflictingNames = map[string]bool{
	"Reset":               true,
	"String":              true,
	"ProtoMessage":        true,
	"Marshal":             true,
	"Unmarshal":           true,
	"ExtensionRangeArray": true,
	"ExtensionMap":        true,
	"Descriptor":          true,
}
