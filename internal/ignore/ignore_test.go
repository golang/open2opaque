// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ignore_test

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/open2opaque/internal/ignore"
)

func TestIgnoreList(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(`
a/b/

	`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(`
c/d/e/some.proto
	`), 0644); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		loadPattern string
		path        string
		want        bool
	}{
		{
			loadPattern: filepath.Join(dir, "file*.txt"),
			path:        "a/b/some.proto",
			want:        true,
		},
		{
			loadPattern: filepath.Join(dir, "file1*.txt"),
			path:        "a/b/some.proto",
			want:        true,
		},
		{
			loadPattern: filepath.Join(dir, "file2*.txt"),
			path:        "a/b/some.proto",
			want:        false,
		},
		{
			loadPattern: filepath.Join(dir, "file*.txt"),
			path:        "a/b/x/some.proto",
			want:        true,
		},
		{
			loadPattern: filepath.Join(dir, "file*.txt"),
			path:        "a/x/some.proto",
			want:        false,
		},
		{
			loadPattern: filepath.Join(dir, "file*.txt"),
			path:        "c/d/e/some.proto",
			want:        true,
		},
		{
			loadPattern: filepath.Join(dir, "file1*.txt"),
			path:        "c/d/e/some.proto",
			want:        false,
		},
	}

	for _, tc := range testCases {
		ignoreList, err := ignore.LoadList(tc.loadPattern)
		if err != nil {
			t.Fatal(err)
		}
		if got := ignoreList.Contains(tc.path); got != tc.want {
			t.Errorf("Using pattern %q, Contains(%s) = %v, want %v", tc.loadPattern, tc.path, got, tc.want)
		}
	}
}
