// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statsutil_test

import (
	"testing"

	"google.golang.org/open2opaque/internal/o2o/statsutil"
)

func TestShortAndLongNameFrom(t *testing.T) {
	for _, tt := range []struct {
		long      string
		wantLong  string
		wantShort string
	}{
		{
			long:      "google.golang.org/open2opaque/random_go_proto.MyMessage",
			wantLong:  "google.golang.org/open2opaque/random_go_proto.MyMessage",
			wantShort: "MyMessage",
		},
	} {
		typ := statsutil.ShortAndLongNameFrom(tt.long)
		if got := typ.GetLongName(); got != tt.wantLong {
			t.Errorf("ShortAndLongNameFrom(%s): unexpected long name: got %q, want %q", tt.long, got, tt.wantLong)
		}
		if got := typ.GetShortName(); got != tt.wantShort {
			t.Errorf("ShortAndLongNameFrom(%s): unexpected short name: got %q, want %q", tt.long, got, tt.wantShort)
		}
	}
}
