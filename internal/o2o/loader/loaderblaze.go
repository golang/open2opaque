// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loader

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

type BlazeLoader struct {
	dir string
}

func NewBlazeLoader(ctx context.Context, cfg *Config, dir string) (*BlazeLoader, error) {
	return &BlazeLoader{
		dir: dir,
	}, nil
}

// Close frees all resources that NewBlazeLoader() created. The BlazeLoader must
// not be used afterwards.
func (l *BlazeLoader) Close(context.Context) error { return nil }

func failBatch(targets []*Target, res chan LoadResult, err error) {
	for _, t := range targets {
		res <- LoadResult{
			Target: t,
			Err:    err,
		}
	}
}

// LoadPackage loads a batch of Go packages.
func (l *BlazeLoader) LoadPackages(ctx context.Context, targets []*Target, res chan LoadResult) {
	targetByID := make(map[string]*Target)
	patterns := make([]string, len(targets))
	for idx, t := range targets {
		patterns[idx] = t.ID
		targetByID[t.ID] = t
	}

	cfg := &packages.Config{
		Dir:     l.dir,
		Context: ctx,
		Mode: packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		failBatch(targets, res, err)
		return
	}

	// Validate the response: ensure we can associate each returned package with
	// a requested target, or fail the entire batch.
	for _, pkg := range pkgs {
		if strings.Contains(pkg.ID, ".test") {
			// go/packages returns test packages for the provided patterns,
			// e.g. "google.golang.org/o2o [google.golang.org/o2o.test]"
			// for pattern google.golang.org/o2o.
			continue
		}
		if _, ok := targetByID[pkg.ID]; !ok {
			failBatch(targets, res, fmt.Errorf("BUG: Load() returned package %s, which was not requested", pkg.ID))
			return
		}
		if len(pkg.Errors) > 0 {
			failBatch(targets, res, fmt.Errorf("Loading package failed:\n%s", pkg.Errors))
			return
		}
	}

LoadedPackage:
	for _, pkg := range pkgs {
		t := targetByID[pkg.ID]
		if t == nil {
			t = &Target{ID: pkg.ID}
		}
		result := &Package{
			Fileset:  pkg.Fset,
			TypeInfo: pkg.TypesInfo,
			TypePkg:  pkg.Types,
		}
		for idx, absPath := range pkg.CompiledGoFiles {
			b, err := os.ReadFile(absPath)
			if err != nil {
				res <- LoadResult{
					Target: t,
					Err:    err,
				}
				continue LoadedPackage
			}
			relPath := absPath
			libraryUnderTest := false
			generated := strings.HasSuffix(relPath, ".pb.go")
			f := &File{
				AST:              pkg.Syntax[idx],
				Path:             relPath,
				LibraryUnderTest: libraryUnderTest,
				Code:             string(b),
				Generated:        generated,
			}
			result.Files = append(result.Files, f)
		}
		res <- LoadResult{
			Target:  t,
			Package: result,
		}
	}
}
