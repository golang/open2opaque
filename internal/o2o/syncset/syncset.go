// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package syncset implements sets that can be safely accessed concurrently.
package syncset

import "sync"

// New returns a new, empty set.
func New() *Set {
	return &Set{
		set: map[string]struct{}{},
	}
}

// Set is a set of strings.
type Set struct {
	mu  sync.Mutex
	set map[string]struct{}
}

// Add adds the value v to the set and returns true if it wasn't in the set
// before. It returns false otherwise.
func (s *Set) Add(v string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.set[v]
	if !ok {
		s.set[v] = struct{}{}
		return true
	}
	return false
}
