// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fakeloader contains a hermetic loader implementation that does not
// depend on any other systems and can be used in tests.
package fakeloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"sync"

	"golang.org/x/tools/go/gcexportdata"
	"google.golang.org/open2opaque/internal/o2o/loader"
)

// ExportForFunc is a caller-supplied function that returns the export data (.x
// file) for the package with the specified import path.
type ExportForFunc func(importPath string) []byte

type fakeLoader struct {
	// fake input from the test
	pkgs      map[string][]string
	files     map[string]string
	generated map[string]string
	exportFor ExportForFunc
}

// NewFakeLoader returns a Loader that can be used in tests. It works in the
// same way as loader returned from NewLoader, except it fakes expensive/flaky
// dependencies (e.g. executing commands).
//
// pkgs maps target rule names (e.g. "//base/go:flag") to list of files in that package
// (e.g. "net/proto2/go/open2opaque/testdata/dummycmd.go"). File paths should be in the form
// expected by blaze, e.g. "//net/proto2/go/open2opaque/testdata:dummy.go". Names of test packages
// should have "_test" suffix.
//
// files maps all files (e.g. "//test/pkg:updated.go"), referenced by the pkgs map, to their content.
// generated maps generated files (e.g. "//test/pkg:generated.go"), referenced by the pkgs map, to their content.
//
// Unlike the loader returned by NewLoader, the fake loader is inexpensive. It's
// intended that multiple fake loaders are created (perhaps one per test).
func NewFakeLoader(pkgs map[string][]string, files, generated map[string]string, exportFor ExportForFunc) loader.Loader {
	l := &fakeLoader{
		pkgs:      pkgs,
		files:     files,
		generated: generated,
		exportFor: exportFor,
	}
	return l
}

func (fl *fakeLoader) Close(context.Context) error { return nil }

func (fl *fakeLoader) loadPackage(ctx context.Context, target *loader.Target) (_ *loader.Package, err error) {
	// target.ID is e.g. google.golang.org/open2opaque/fix/testdata/dummy
	files, ok := fl.pkgs[target.ID]
	if !ok {
		return nil, fmt.Errorf("no such fake package: %q", target.ID)
	}

	pkgPath := target.ID

	pkg := &loader.Package{
		Fileset: token.NewFileSet(),
	}

	var asts []*ast.File
	for _, f := range files {
		fname := f
		generated := false
		code, ok := fl.files[f]
		if !ok {
			if code, ok = fl.generated[f]; !ok {
				return nil, errors.New("no source code")
			}
			generated = true
		}
		ast, err := parser.ParseFile(pkg.Fileset, fname, code, parser.ParseComments|parser.SpuriousErrors)
		if err != nil {
			return nil, err
		}
		asts = append(asts, ast)
		pkg.Files = append(pkg.Files, &loader.File{
			AST:       ast,
			Path:      fname,
			Generated: generated,
			Code:      code,
		})
	}

	pkg.TypeInfo = &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}
	imp, err := newFakeImporter(fl.files, fl.generated, fl.exportFor)
	if err != nil {
		return nil, err
	}
	cfg := types.Config{
		Importer:                 imp,
		FakeImportC:              true,
		DisableUnusedImportCheck: true,
	}
	if pkg.TypePkg, err = cfg.Check(pkgPath, pkg.Fileset, asts, pkg.TypeInfo); err != nil {
		// Ignore loading errors for the siloedpb package, emulating the
		// behavior of the GAP loader with DropSiloedFiles enabled.
		if !strings.Contains(err.Error(), "siloedpb") {
			return nil, err
		}
	}
	return pkg, nil
}

// LoadPackages loads a batch of Go packages by concurrently calling
// loadPackage() for all targets, as the fake loader does not require any
// synchronization.
func (fl *fakeLoader) LoadPackages(ctx context.Context, targets []*loader.Target, res chan loader.LoadResult) {
	var wg sync.WaitGroup
	wg.Add(len(targets))
	for _, t := range targets {
		t := t // copy
		go func() {
			defer wg.Done()
			p, err := fl.loadPackage(ctx, t)
			res <- loader.LoadResult{
				Target:  t,
				Package: p,
				Err:     err,
			}
		}()
	}
	wg.Wait()
}

func newFakeImporter(files, generated map[string]string, exportFor ExportForFunc) (*fakeImporter, error) {
	imp := &fakeImporter{
		checked:   make(map[string]*types.Package),
		files:     files,
		generated: generated,
		exportFor: exportFor,
	}
	return imp, nil
}

type fakeImporter struct {
	checked          map[string]*types.Package
	files, generated map[string]string
	exportFor        ExportForFunc
}

func (imp *fakeImporter) readXFile(importPath string, xFile []byte) (*types.Package, error) {
	// Load the package from the .x file if we have not yet loaded it.
	r, err := gcexportdata.NewReader(bytes.NewReader(xFile))
	if err != nil {
		return nil, err
	}
	pkg, err := gcexportdata.Read(r, token.NewFileSet(), imp.checked, importPath)
	if err != nil {
		return nil, err
	}
	imp.checked[importPath] = pkg
	return pkg, nil
}

const fakeSync = `
package sync

type Once struct {}

func (*Once) Do(func()) {}
`

const fakeReflect = `
package reflect

type Type interface {
	PkgPath() string
}

func TypeOf(any) Type { return nil }
`

const fakeContext = `package context

type Context interface{}

func Background() Context
`

func (imp *fakeImporter) parseAndTypeCheck(pkgPath, fileName, contents string) (*types.Package, error) {
	fs := token.NewFileSet()
	afile, err := parser.ParseFile(fs, fileName, contents, parser.ParseComments|parser.SpuriousErrors)
	if err != nil {
		return nil, err
	}
	cfg := types.Config{Importer: imp}
	pkg, err := cfg.Check(pkgPath, fs, []*ast.File{afile}, nil)
	if err != nil {
		return nil, err
	}
	imp.checked[pkgPath] = pkg
	return pkg, nil
}

func (imp *fakeImporter) Import(pkgPath string) (_ *types.Package, err error) {
	if pkgPath == "unsafe" {
		return types.Unsafe, nil
	}
	if p, ok := imp.checked[pkgPath]; ok && p.Complete() {
		return p, nil
	}
	if pkgPath == "sync" {
		return imp.parseAndTypeCheck(pkgPath, "sync.go", fakeSync)
	}
	if pkgPath == "reflect" {
		return imp.parseAndTypeCheck(pkgPath, "reflect.go", fakeReflect)
	}
	if pkgPath == "context" {
		return imp.parseAndTypeCheck(pkgPath, "context.go", fakeContext)
	}

	b := imp.exportFor(pkgPath)
	if b == nil {
		return nil, fmt.Errorf("tried to load package %q for which there is no export data", pkgPath)
	}
	return imp.readXFile(pkgPath, b)
}
