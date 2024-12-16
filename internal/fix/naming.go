// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/types"
	"strings"
	"unicode"

	log "github.com/golang/glog"
)

func helperVarNameForType(t types.Type) string {
	// Get to the elementary type (pb.M2) if this is a pointer type (*pb.M2).
	elem := t
	if ptr, ok := elem.(*types.Pointer); ok {
		elem = ptr.Elem()
	}
	named, ok := elem.(*types.Named)
	if !ok {
		log.Fatalf("BUG: proto message unexpectedly not a named type (but %T)?!", elem)
	}
	return helperVarNameForName(named.Obj().Name())
}

// helperVarNameForName produces a name for a helper variable for the specified
// package-local name.
func helperVarNameForName(packageLocal string) string {
	fullName := strings.ToLower(packageLocal[:1]) + packageLocal[1:]
	if len(fullName) < 10 {
		return fullName
	}
	// The name is too long for a helper variable. Abbreviate if possible.
	if strings.Contains(fullName, "_") {
		// Split along the underscores and use the first letter of each word,
		// turning BigtableRowMutationArgs_Mod_SetCell into bms.
		parts := strings.Split(fullName, "_")
		abbrev := ""
		for _, part := range parts {
			abbrev += strings.ToLower(part[:1])
		}
		return abbrev
	}
	if parts := strings.FieldsFunc(packageLocal, unicode.IsLower); len(parts) > 1 {
		// We split around the lowercase characters, leaving us with only the
		// uppercase characters.
		return strings.ToLower(strings.Join(parts, ""))
	} else if len(parts) == 1 && len(parts[0]) > 1 {
		// The name starts with multiple uppercase letters, but is followed by
		// only lowercase letters (e.g. ESDimensions). Return all the uppercase
		// letters we have.
		return strings.ToLower(parts[0])
	}
	// The name is too long and cannot be abbreviated based on uppercase
	// letters. Cut off at the closest vowel.
	return strings.TrimRightFunc(fullName[:10], func(r rune) bool {
		return r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u'
	})
}
