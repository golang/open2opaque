// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loader_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/open2opaque/internal/o2o/fakeloader"
	"google.golang.org/open2opaque/internal/o2o/loader"
)

// fakingBlaze is true when NewFakeLoader is used
var fakingBlaze = true

var prefix = func() string {
	p := "google.golang.org/open2opaque/fake/"
	return p
}()

var pathPrefix = func() string {
	p := "google.golang.org/open2opaque/fake/"
	return p
}()

var newLoader func(ctx context.Context, cfg *loader.Config) (loader.Loader, error)

func init() {
	testPkgs := map[string][]string{
		prefix + "dummy": []string{
			prefix + "dummy.go",
		},
		prefix + "dummy_test": []string{
			prefix + "dummy.go",
			prefix + "dummy_test.go",
		},
		prefix + "dummycmd": []string{
			prefix + "dummycmd.go",
		},
	}
	testFiles := map[string]string{
		prefix + "dummy.go":      `package dummy`,
		prefix + "dummy_test.go": `package dummy`,
		prefix + "dummycmd.go": `package main
func main() {}`,
	}
	newLoader = func(ctx context.Context, cfg *loader.Config) (loader.Loader, error) {
		return fakeloader.NewFakeLoader(testPkgs, testFiles, nil, nil), nil
	}
}

var tests = []struct {
	desc      string
	ruleName  string
	wantFiles []string
}{{
	desc:      "library",
	ruleName:  prefix + "dummy",
	wantFiles: []string{pathPrefix + "dummy.go"},
}, {
	desc:     "test",
	ruleName: prefix + "dummy_test",
	wantFiles: []string{
		pathPrefix + "dummy.go",
		pathPrefix + "dummy_test.go",
	},
}, {
	desc:      "binary",
	ruleName:  prefix + "dummycmd",
	wantFiles: []string{pathPrefix + "dummycmd.go"},
}}

func setup(t *testing.T) (dir string) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	return dir
}

// TestUsesRealBlaze exists so that the test output has a reminder that this
// test can be run locally with a real blaze. It also verifies that we don't
// accidentally fake blaze locally.
func TestUsesRealBlaze(t *testing.T) {
	if fakingBlaze {
		t.Skip("Can't use real blaze on borg. All tests will use a fake one. Run this test locally to use real blaze.")
	}
}

func testConfig(t *testing.T) *loader.Config {
	t.Helper()

	cfg := &loader.Config{}

	return cfg
}

func TestDiscoversSourceFiles(t *testing.T) {
	setup(t)
	l, err := newLoader(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			pkg, err := loader.LoadOne(context.Background(), l, &loader.Target{ID: tt.ruleName})
			if err != nil {
				t.Fatalf("LoadPackage(%s) failed: %v", tt.ruleName, err)
			}
			var got []string
			for _, f := range pkg.Files {
				got = append(got, f.Path)
			}
			sort.Strings(got)
			if d := cmp.Diff(tt.wantFiles, got); d != "" {
				t.Errorf("LoadPackage(%s) = %v; want %v; diff:\n%s", tt.ruleName, got, tt.wantFiles, d)
			}
		})
	}
}

func TestParallelQueries(t *testing.T) {
	setup(t)
	l, err := newLoader(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	errc := make(chan error)
	const runs = 3 // chosen arbitrarily
	for i := 0; i < runs; i++ {
		go func() {
			for _, tt := range tests {
				pkg, err := loader.LoadOne(context.Background(), l, &loader.Target{ID: tt.ruleName})
				if err != nil {
					errc <- fmt.Errorf("%s: LoadPackage(%s) failed: %v", tt.desc, tt.ruleName, err)
					continue
				}
				var got []string
				for _, f := range pkg.Files {
					got = append(got, f.Path)
				}
				sort.Strings(got)
				if d := cmp.Diff(tt.wantFiles, got); d != "" {
					errc <- fmt.Errorf("%s: LoadPackage(%s) = %v; want %v; diff:\n%s", tt.desc, tt.ruleName, got, tt.wantFiles, d)
				}
				errc <- nil
			}
		}()
	}
	for i := 0; i < runs*len(tests); i++ {
		if err := <-errc; err != nil {
			t.Error(err)
		}
	}
}

func TestSingleFailureDoesntAffectOtherTargets(t *testing.T) {
	setup(t)
	l, err := newLoader(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	errc := make(chan error)
	const runs = 5 // chosen arbitrarily
	for i := 0; i < runs; i++ {
		go func() {
			for _, tt := range tests {
				pkg, err := loader.LoadOne(context.Background(), l, &loader.Target{ID: tt.ruleName})
				if err != nil {
					errc <- fmt.Errorf("%s: LoadPackage(%s) failed: %v", tt.desc, tt.ruleName, err)
					continue
				}
				var got []string
				for _, f := range pkg.Files {
					got = append(got, f.Path)
				}
				sort.Strings(got)
				if d := cmp.Diff(tt.wantFiles, got); d != "" {
					errc <- fmt.Errorf("%s: LoadPackage(%s) = %v; want %v; diff:\n%s", tt.desc, tt.ruleName, got, tt.wantFiles, d)
				}
				errc <- nil
			}
			in := "google.golang.org/open2opaque/internal/fix/testdata/DOES_NOT_EXIST"
			got, err := loader.LoadOne(context.Background(), l, &loader.Target{ID: in})
			if err == nil {
				errc <- fmt.Errorf("LoadPackage(%s) succeeded (files: %v); want failure (package doesn't exist)", in, got.Files)
				return
			}
			errc <- nil
		}()
	}
	for i := 0; i < runs*(len(tests)+1); i++ {
		if err := <-errc; err != nil {
			t.Error(err)
		}
	}
}
