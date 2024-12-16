// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import "testing"

func TestHelperVarNameForName(t *testing.T) {
	for _, tt := range []struct {
		name string
		want string
	}{
		{
			name: "M2",
			want: "m2",
		},

		{
			name: "BigtableRowMutationArgs_Mod_SetCell",
			want: "bms",
		},

		{
			name: "BigtableRowMutationArgs",
			want: "brma",
		},

		{
			name: "Verylongonewordnamethatcannotbeabbreviated",
			want: "verylongon",
		},

		{
			name: "ESDimensions",
			want: "esd",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := helperVarNameForName(tt.name)
			if got != tt.want {
				t.Errorf("helperVarNameForName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
