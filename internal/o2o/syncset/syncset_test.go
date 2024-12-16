// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncset

import (
	"sync"
	"testing"
)

func TestSyncset(t *testing.T) {
	s := New()
	if !s.Add("hello") {
		t.Error("AddNew() returned false for an empty set")
	}
	if s.Add("hello") {
		t.Error("AddNew('hello') returned true for a set containing 'hello'")
	}
	if !s.Add("world") {
		t.Error("AddNew('world') returned false for a set without 'world'")
	}
}

func TestNoRace(t *testing.T) {
	var wg sync.WaitGroup
	s := New()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Add("hello")
		}()
	}
	wg.Wait()
}
