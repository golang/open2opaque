// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loader

import (
	"context"
	"fmt"
	"os"

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
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		failBatch(targets, res, err)
		return
	}

	if got, want := len(pkgs), len(patterns); got != want {
		failBatch(targets, res, fmt.Errorf("BUG: Load(%s) resulted in %d packages, want %d packages", patterns, got, want))
		return
	}

	// Validate the response: ensure we can associate each returned package with
	// a requested target, or fail the entire batch.
	for _, pkg := range pkgs {
		if _, ok := targetByID[pkg.ID]; !ok {
			failBatch(targets, res, fmt.Errorf("BUG: Load() returned package %s, which was not requested", pkg.ID))
			return
		}
	}

LoadedPackage:
	for _, pkg := range pkgs {
		t := targetByID[pkg.ID]
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
			f := &File{
				AST:              pkg.Syntax[idx],
				Path:             relPath,
				LibraryUnderTest: t.LibrarySrcs[relPath],
				Code:             string(b),
			}
			result.Files = append(result.Files, f)
		}
		res <- LoadResult{
			Target:  t,
			Package: result,
		}
	}
}
