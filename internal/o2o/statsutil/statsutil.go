// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package statsutil provides functions for working with the open2opaque stats
// proto.
package statsutil

import (
	"strings"

	statspb "google.golang.org/open2opaque/internal/dashboard"
)

// ShortAndLongNameFrom takes a long name
// and returns a combined short and long name statspb.Type.
func ShortAndLongNameFrom(long string) *statspb.Type {
	short := long
	if idx := strings.LastIndex(long, "."); idx >= 0 {
		short = long[idx+1:]
		stars := strings.LastIndex(long, "*")
		if stars != -1 {
			short = long[:stars+1] + short
		}
	}
	return &statspb.Type{
		LongName:  long,
		ShortName: short,
	}
}
