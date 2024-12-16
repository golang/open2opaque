// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package set provides set data structures.
package set

// Strings is a set of strings.
type Strings map[string]struct{}

// NewStrings constructs Strings with given strs.
func NewStrings(strs ...string) Strings {
	ret := Strings{}
	for _, s := range strs {
		ret.Add(s)
	}
	return ret
}

// Add adds given string.
func (s Strings) Add(str string) {
	s[str] = struct{}{}
}

// AddSet adds values from another Strings object.
func (s Strings) AddSet(strs Strings) {
	for str := range strs {
		s.Add(str)
	}
}

// Len returns the size of the set.
func (s Strings) Len() int {
	return len(s)
}

// Contains returns true if given string is in set, else false.
func (s Strings) Contains(str string) bool {
	_, ok := s[str]
	return ok
}

// ToSlice returns a slice of values.
func (s Strings) ToSlice() []string {
	ret := make([]string, 0, len(s))
	for k := range s {
		ret = append(ret, k)
	}
	return ret
}
