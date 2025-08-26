// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dave/dst"
	log "github.com/golang/glog"
	"golang.org/x/tools/go/ast/astutil"
)

// imports provides an API to work with package imports. It is a convenience wrapper around
// type and AST information.
type imports struct {
	path2pkg     map[string]*types.Package
	renameByPath map[string]string // import path -> name; for renamed packages
	renameByName map[string]string // name -> import path
	importsToAdd []*dst.ImportSpec
}

// newImports creates imports for the package.
func newImports(pkg *types.Package, f *ast.File) *imports {
	out := &imports{
		path2pkg:     make(map[string]*types.Package),
		renameByPath: make(map[string]string),
		renameByName: make(map[string]string),
	}

	// path2pkg maps from import path to *types.Package, but for the entire
	// package-under-analysis, not just for the file-under-analysis.
	path2pkg := make(map[string]*types.Package)
	for _, imp := range pkg.Imports() {
		path2pkg[imp.Path()] = imp
	}

	astutil.Apply(f, func(c *astutil.Cursor) bool {
		s, ok := c.Node().(*ast.ImportSpec)
		if !ok {
			return true
		}
		path, err := strconv.Unquote(s.Path.Value)
		if err != nil {
			log.Errorf("malformed source: %v", err)
			return false
		}

		if pkg, ok := path2pkg[path]; ok {
			out.path2pkg[path] = pkg
		}

		if s.Name == nil { // no rename
			return false
		}
		out.renameByPath[path] = s.Name.Name
		out.renameByName[s.Name.Name] = path
		return false
	}, nil)

	return out
}

// name returns the name of import with the given import path. For example:
//
//	"google.golang.org/protobuf/proto"          => "proto"
//	goproto "google.golang.org/protobuf/proto"  => "goproto"
//
// In case the import does not yet exist, it will be queued for addition.
func (imp *imports) name(path string) string {
	// Is the package already imported by the input source code?
	if v, ok := imp.renameByPath[path]; ok {
		return v
	}

	// Check if we already tried to add the import.
	for _, i := range imp.importsToAdd {
		if s, err := strconv.Unquote(i.Path.Value); err == nil && s == path {
			if i.Name != nil {
				return i.Name.Name
			}
			return filepath.Base(path)
		}
	}

	// Import doesn't exist and we didn't try to add it yet. Add a new import.
	p := imp.path2pkg[path]
	if p == nil {
		// path is an import path that does not occur in the source file. There
		// are two situations in which this can happen:
		//
		// 1. The proto package (from third_party/golang/protobuf) was not
		//    imported, but is now necessary because helper functions like
		//    proto.String() are used after the rewrite.
		//
		// 2. A proto message is referenced without a corresponding import. For
		//    example, mypb.GetSubmessage() could be defined in the separate
		//    package myextrapb.
		//
		// We find an available name and add the required import(s).
		name := imp.findAvailableName(path)
		spec := &dst.ImportSpec{
			Path: &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)},
		}
		if strings.HasSuffix(name, "pb") {
			// The third_party proto package is not renamed, but all generated
			// proto packages are.
			spec.Name = &dst.Ident{Name: name}
		}
		imp.importsToAdd = append(imp.importsToAdd, spec)
		return name

	}
	return p.Name()
}

// lookup returns a objects with givne name from import identified by the provided import path or nil if it doesn't exist.
func (imp *imports) lookup(path, name string) types.Object {
	p := imp.path2pkg[path]
	if p == nil {
		return nil
	}
	return p.Scope().Lookup(name)
}

// findAvailableName returns an available name to import a generated proto
// package as.
//
// We try xpb, x2pb, x3pb, etc. (x stands for expression protobuf, or extra
// protobuf). This way, humans editing the source can recognize the placeholder
// name and replace it with something more descriptive and more inline with the
// respective team style.
func (imp *imports) findAvailableName(path string) string {
	// Google-internally, all protobuf generated code packages end in go_proto.
	// Externally, we at least recognize the well-known types,
	// such that our tests behave the same way internally and externally.
	isProtoImport := strings.HasSuffix(path, "go_proto") ||
		strings.HasPrefix(path, "google.golang.org/protobuf/types/")
	if !isProtoImport {
		// default name for non proto imports, assumed to be available
		return filepath.Base(path)
	}

	name := "xpb"
	cnt := 2
	for {
		if _, ok := imp.renameByName[name]; !ok {
			break // name available
		}
		name = fmt.Sprintf("x%dpb", cnt)
		cnt++
	}
	imp.renameByName[name] = path
	imp.renameByPath[path] = name
	return name
}
