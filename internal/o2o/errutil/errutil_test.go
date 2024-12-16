// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errutil

import (
	"errors"
	"testing"
)

func TestAnnotateHasNoEffectOnNilError(t *testing.T) {
	f := func() (err error) {
		defer Annotatef(&err, "f")
		return nil
	}
	if err := f(); err != nil {
		t.Errorf("f() failed: %v", err)
	}
}

func TestAnnotateAddsInfoToNonNilError(t *testing.T) {
	f := func() (err error) {
		defer Annotatef(&err, "g")
		return errors.New("test error")
	}
	want := "g: test error"
	if err := f(); err == nil || err.Error() != want {
		t.Errorf("f(): %v; want %s", err, want)
	}
}
