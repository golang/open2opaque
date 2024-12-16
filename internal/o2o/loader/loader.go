// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loader loads Go packages.
package loader

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// Package represents a loaded Go package.
type Package struct {
	Files    []*File        // Files in the package.
	Fileset  *token.FileSet // For translating between positions and locations in files.
	TypeInfo *types.Info    // Type information for the package (e.g. object identity, identifier uses/declarations, expression types).
	TypePkg  *types.Package // Describes the package (e.g. import objects, package scope).
}

func (p Package) String() string {
	var paths []string
	for _, f := range p.Files {
		paths = append(paths, f.Path)
	}
	return fmt.Sprintf("Go Package %s with files:\n\t%s", p.TypePkg.Path(), strings.Join(paths, "\n\t"))
}

// File represents a single file in a loaded package.
type File struct {
	// Parsed file.
	AST *ast.File

	Path string

	// For go_test targets, this field indicates whether the file belongs to the
	// test code itself (one or more _test.go files), or whether the file
	// belongs to the package-under-test (via the go_test target’s library
	// attribute).
	//
	// For other targets (go_binary or go_library), this field is always false.
	LibraryUnderTest bool

	// Source code from the file.
	Code string

	// True if the file was generated (go_embed_data, genrule, etc.).
	Generated bool
}

// Target represents a package to be loaded. It is identified by the opaque ID
// field, which is interpreted by the loader (which, in turn, delegates to the
// gopackagesdriver).
type Target struct {
	// ID is an opaque identifier for the package when using the packages loader.
	ID string

	// Testonly indicates that this package should be considered test code. This
	// attribute needs to be passed in because go/packages does not have the
	// concept of test only code, only Blaze does.
	Testonly bool

	LibrarySrcs map[string]bool
}

// LoadResult represents the result of loading an individual target. The Target
// field is always set. If something went wrong, Err is non-nil and Package is
// nil. Otherwise, Err is nil and Package is non-nil.
type LoadResult struct {
	Target  *Target
	Package *Package
	Err     error
}

// Loader loads Go packages.
type Loader interface {
	// LoadPackages loads a batch of Go packages.
	//
	// The method does not return a slice of results, but instead writes each
	// result to the specified result channel as the result arrives. This allows
	// for concurrency with loaders that support it, like the Compilations
	// Bigtable loader (processing results while the load is still ongoing).
	LoadPackages(context.Context, []*Target, chan LoadResult)
	Close(context.Context) error
}

// LoadOne is a convenience function that loads precisely one target, saving the
// caller the mechanics of having to work with a batch of targets.
func LoadOne(ctx context.Context, l Loader, t *Target) (*Package, error) {
	results := make(chan LoadResult, 1)
	l.LoadPackages(context.Background(), []*Target{t}, results)
	res := <-results
	return res.Package, res.Err
}
