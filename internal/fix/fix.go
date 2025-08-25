// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fix rewrites Go packages to use opaque version of the Go protocol
// buffer API.
package fix

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	log "github.com/golang/glog"
	"golang.org/x/tools/go/types/typeutil"
	spb "google.golang.org/open2opaque/internal/dashboard"
	"google.golang.org/open2opaque/internal/ignore"
	"google.golang.org/open2opaque/internal/o2o/loader"
	"google.golang.org/open2opaque/internal/o2o/syncset"
)

// Level represents the riskiness of a fix ranging from "safe to submit" to
// "needs fixes by humans".
type Level string

const (
	// None means that no transforms to the code are applied. Useful for
	// gathering statistics about unmodified code.
	None = Level("none")

	// Green fixes are considered safe to submit without human review. Those
	// fixes preserve the behavior of the program.
	Green = Level("green")

	// Yellow fixes are safe to submit except for programs that depend on
	// unspecified behavior, internal details, code that goes against the
	// good coding style or guidelines, etc.
	// Yellow fixes should be reviewed.
	Yellow = Level("yellow")

	// Red fixes can change behavior of the program. Red fixes are proposed
	// when we can't prove that the fix is safe or when the fix results in
	// code that we don't consider readable.
	// Red fixes should go through extensive review and analysis.
	Red = Level("red")
)

// ge returns true if lvl is greater or equal to rhs.
func (lvl Level) ge(rhs Level) bool {
	switch lvl {
	case None:
		return rhs == None
	case Green:
		return rhs == None || rhs == Green
	case Yellow:
		return rhs == None || rhs == Green || rhs == Yellow
	case Red:
		return true
	}
	panic("unreachable; lvl = '" + lvl + "'")
}

// le returns true if lvl is less or equal to rhs.
func (lvl Level) le(rhs Level) bool {
	return lvl == rhs || !lvl.ge(rhs)
}

// FixedFile represents a single file after applying fixes.
type FixedFile struct {
	// Path to the file
	Path         string
	OriginalCode string               // Code before applying fixes.
	Code         string               // Code after applying fixes.
	Modified     bool                 // Whether the file was modified by this tool.
	Generated    bool                 // Whether the file is a generated file.
	Stats        []*spb.Entry         // List of proto accesses in Code (i.e. after applying rewrites).
	Drifted      bool                 // Whether the file has drifted between CBT and HEAD.
	RedFixes     map[unsafeReason]int // Number of fixes per unsafe category.
}

func (f *FixedFile) String() string {
	return fmt.Sprintf("[FixedFile %s Code=<redacted,len=%d> Modified=%t Generated=%t Stats=<redacted,len=%d>]",
		f.Path, len(f.Code), f.Modified, f.Generated, len(f.Stats))
}

// Result describes what was fixed. For all change levels.
type Result map[Level][]*FixedFile

// AllStats returns all of the generated stats entries.
func (r Result) AllStats() []*spb.Entry {
	var stats []*spb.Entry
	for _, lvl := range []Level{None, Green, Yellow, Red} {
		for _, f := range r[lvl] {
			if lvl > None && !f.Modified {
				continue
			}
			stats = append(stats, f.Stats...)
		}
	}
	return stats
}

// ReportStats calls report on each given stats entry after setting
// its Status field based on err. If err is non-nil, it also reports
// an empty entry for pkgPath.
func ReportStats(stats []*spb.Entry, pkgPath string, err error, report func(*spb.Entry)) {
	st := &spb.Status{Type: spb.Status_OK}
	if err != nil {
		st = &spb.Status{
			Type:  spb.Status_FAIL,
			Error: strings.TrimSpace(err.Error()),
		}
		report(&spb.Entry{
			Status: st,
			Location: &spb.Location{
				Package: pkgPath,
			},
		})
	}

	for _, e := range stats {
		if e.Status == nil {
			e.Status = st
		}
		report(e)
	}
}

// rewrite describes transformations done on a DST with a pre- and post-
// traversal. Exactly one of the pre/post must be set.
type rewrite struct {
	name string
	pre  func(c *cursor) bool
	post func(c *cursor) bool
}

var rewrites []rewrite

// typesInfo contains type information for a type-checked package similar to types.Info but with
// dst.Node instead of ast.Node.
type typesInfo struct {
	types  map[dst.Expr]types.TypeAndValue
	uses   map[*dst.Ident]types.Object
	defs   map[*dst.Ident]types.Object
	astMap map[dst.Node]ast.Node
	dstMap map[ast.Node]dst.Node
}

// typeOf returns the types.Type of the given expression or nil if not found. It is equivalent to
// types.Info.TypeOf.
func (info *typesInfo) typeOf(e dst.Expr) types.Type {
	if t, ok := info.types[e]; ok {
		return t.Type
	}
	if id, _ := e.(*dst.Ident); id != nil {
		if obj := info.objectOf(id); obj != nil {
			return obj.Type()
		}
	}
	return nil
}

// objectOf returns the types.Object denoted by the given id, or nil if not found. It is equivalent
// to types.Info.TypeOf.
func (info *typesInfo) objectOf(id *dst.Ident) types.Object {
	if obj := info.defs[id]; obj != nil {
		return obj
	}
	return info.uses[id]
}

// A cacheEntry stores a proto type (if any). Presence of a cacheEntry indicates
// that the type in question has been processed before, no matter whether
// protoType is nil or non-nil.
type cacheEntry struct {
	protoType types.Type
}

func init() {
	t, err := types.Eval(token.NewFileSet(), nil, token.NoPos, "func(){}()")
	if err != nil {
		panic("can't initialize void type: " + err.Error())
	}
	if !t.IsVoid() {
		panic("can't initialize the void type")
	}
	voidType = t
}

// BuilderUseType categorizes when builders instead of setters are used to
// rewrite struct literal initialization.
type BuilderUseType int

const (
	// BuildersNowhere means never use builders
	BuildersNowhere BuilderUseType = 0
	// BuildersEverywhere means always use builders
	BuildersEverywhere BuilderUseType = 1
	// BuildersTestsOnly means use builders only in tests
	BuildersTestsOnly BuilderUseType = 2
	// BuildersEverywhereExceptPromising means always use builders, except for
	// .go files that touch promising protos (many fleet-wide unmarshals).
	BuildersEverywhereExceptPromising BuilderUseType = 3
)

// ConfiguredPackage contains a package and all configuration necessary to
// rewrite the package.
type ConfiguredPackage struct {
	Loader           loader.Loader
	Pkg              *loader.Package
	TypesToUpdate    map[string]bool
	BuilderTypes     map[string]bool
	BuilderLocations *ignore.List
	Levels           []Level
	ProcessedFiles   *syncset.Set
	ShowWork         bool
	Testonly         bool
	UseBuilders      BuilderUseType
}

// Fix fixes a Go package.
func (cpkg *ConfiguredPackage) Fix() (Result, error) {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Sprintf("can't process package: %v", r))
		}
	}()

	// Pairing of loader.File with associated dst.File.
	type filePair struct {
		loaderFile *loader.File
		dstFile    *dst.File
	}
	files := []filePair{}

	// Convert AST to DST and also produce corresponding typesInfo.
	dec := decorator.NewDecorator(cpkg.Pkg.Fileset)
	for _, f := range cpkg.Pkg.Files {
		dstFile, err := dec.DecorateFile(f.AST)
		if err != nil {
			return nil, err
		}
		files = append(files, filePair{f, dstFile})
	}
	info := dstTypesInfo(cpkg.Pkg.TypeInfo, dec)

	// Only check for file drift (between Compilations Bigtable and Piper HEAD)
	// when working in a CitC client, not when running as FlumeGo job in prod.
	driftCheck := false
	if wd, err := os.Getwd(); err == nil {
		driftCheck = strings.HasPrefix(wd, "/google/src/cloud/")
	}

	out := make(Result)
	for _, rec := range files {
		f := rec.loaderFile
		if f.LibraryUnderTest {
			// Skip library code: non-test code uses setters instead of
			// builders, so it is important to skip library code when processing
			// the _test target.
			log.Infof("skipping library file %s", f.Path)
			continue
		}
		if !cpkg.ProcessedFiles.Add(f.Path) {
			continue
		}

		dstFile := rec.dstFile
		fmtSource := func() string {
			var buf bytes.Buffer
			if err := decorator.Fprint(&buf, dstFile); err != nil {
				log.Fatalf("BUG: decorator.Fprint: %v", err)
			}
			return buf.String()
		}
		c := &cursor{
			pkg:                              cpkg.Pkg,
			curFile:                          f,
			curFileDST:                       dstFile,
			imports:                          newImports(cpkg.Pkg.TypePkg, f.AST),
			typesInfo:                        info,
			loader:                           cpkg.Loader,
			lvl:                              None,
			typesToUpdate:                    cpkg.TypesToUpdate,
			builderTypes:                     cpkg.BuilderTypes,
			builderLocations:                 cpkg.BuilderLocations,
			shouldLogCompositeTypeCache:      new(typeutil.Map),
			shouldLogCompositeTypeCacheNoPtr: new(typeutil.Map),
			debugLog:                         make(map[string][]string),
			builderUseType:                   cpkg.UseBuilders,
			testonly:                         cpkg.Testonly,
			helperVariableNames:              make(map[string]bool),
			numUnsafeRewritesByReason:        map[unsafeReason]int{},
		}
		knownNoType := exprsWithNoType(c, dstFile)
		out[None] = append(out[None], &FixedFile{
			Path:      f.Path,
			Code:      f.Code,
			Generated: f.Generated,
			Stats:     stats(c, dstFile, f.Generated),
		})
		for _, lvl := range cpkg.Levels {
			if lvl == None {
				continue
			}
			if cpkg.ShowWork {
				log.Infof("----- LEVEL %s -----", lvl)
			}
			c.imports.importsToAdd = nil
			for _, r := range rewrites {
				before := ""
				if cpkg.ShowWork {
					before = fmtSource()
				}

				c.lvl = lvl
				if (r.pre != nil) == (r.post != nil) {
					// We enforce this so that it's easier to accurately detect
					// which DST transformation loses type information.
					panic(fmt.Sprintf("exactly one rewrite.pre or rewrite.post must be set; r.pre set: %t; r.post set: %t", r.pre != nil, r.post != nil))
				}
				if r.pre != nil {
					dstutil.Apply(dstFile, makeApplyFn(r.name, cpkg.ShowWork, r.pre, c), nil)
				}
				if r.post != nil {
					dstutil.Apply(dstFile, nil, makeApplyFn(r.name, cpkg.ShowWork, r.post, c))
				}
				// Walk the dst and verify that all expressions that should have
				// the type set, have the type set. The idea is that we can run
				// this over all our code and identify type bugs in
				// open2opaque rewrites.
				//
				//
				// We've considered the following alternative:
				//
				//  for each transformation (e.g. green 'hasPre')
				//   repeat until there are no changes:
				//     type-check
				//     apply the transformation
				//
				// We've discarded this approach because it prevents doing
				// transformation that can't be type checked. For example:
				// introducing builders. The problem is that:
				//   - not all protos are on the open_struct API
				//   - we use an offline job to provide type information for
				//   dependencies and can't easily make it generate the new API
				dstutil.Apply(dstFile, func(cur *dstutil.Cursor) bool {
					x, ok := cur.Node().(dst.Expr)
					if !ok {
						return true
					}
					if knownNoType[x] {
						return true
					}
					if _, ok := c.typesInfo.types[x]; !ok {
						buf := new(bytes.Buffer)
						err := dst.Fprint(buf, x, func(name string, v reflect.Value) bool {
							return name != "Decs" && name != "Obj" && name != "Path"
						})
						if err != nil {
							buf = bytes.NewBufferString("<can't print the expression>")
						}
						panic(fmt.Sprintf("BUG: can't determine type of expression after a rewrite; level: %s; file: %s; rewrite %s; expr:\n%s",
							c.lvl, rec.loaderFile.Path, r.name, buf))
					}
					return true
				}, nil)

				if cpkg.ShowWork {
					after := fmtSource()
					// We are intentionally using udiff instead of
					// cmp.Diff here, because it is too cumbersome to get a
					// line-based diff out of cmp.Diff.
					//
					// While udiff calling out to diff(1) is not the most
					// efficient arrangement, at least the output format is
					// familiar to readers.
					diff, err := udiff([]byte(before), []byte(after))
					if err != nil {
						return nil, err
					}
					if diff != nil {
						log.Infof("rewrite %s changed:\n%s", r.name, string(diff))
					}
				}
			}

			if len(c.imports.importsToAdd) > 0 {
				dstutil.Apply(dstFile, nil, func(cur *dstutil.Cursor) bool {
					if _, ok := cur.Node().(*dst.ImportSpec); !ok {
						return true // skip node, looking for ImportSpecs only
					}
					decl, ok := cur.Parent().(*dst.GenDecl)
					if !ok {
						panic(fmt.Sprintf("BUG: parent of ImportSpec is type %T, wanted GenDecl", cur.Parent()))
					}
					if cur.Index() < len(decl.Specs)-1 {
						return true // skip import, waiting for the last one
					}
					// This is the last import, so we add to the very end of
					// the import list.
					for _, imp := range c.imports.importsToAdd {
						cur.InsertAfter(imp)
						c.setType(imp.Path, types.Typ[types.Invalid])
						if imp.Name != nil {
							c.setType(imp.Name, types.Typ[types.Invalid])
						}
					}
					return false // import added, abort traversal
				})
			}

			var buf bytes.Buffer
			if err := decorator.Fprint(&buf, dstFile); err != nil {
				return nil, err
			}
			code := buf.String()
			modified := f.Code != code
			drifted := false
			if modified && !f.Generated && driftCheck &&
				// The paths that our unit tests use (test/pkg/...) do not refer
				// to actual files and hence cannot be read.
				!strings.HasPrefix(f.Path, "test/pkg/") {
				// Check whether the source has changed between the local CitC
				// client and reading it from the go/compilations-bigtable
				// loader.
				b, err := os.ReadFile(f.Path)
				if err != nil {
					return nil, err
				}
				// Our loader formats the source when loading from
				// go/compilations-bigtable, so we need to format here, too.
				formattedContents, err := format.Source(b)
				if err != nil {
					return nil, err
				}
				drifted = f.Code != string(formattedContents)
			}
			out[lvl] = append(out[lvl], &FixedFile{
				Path:         f.Path,
				OriginalCode: f.Code,
				Code:         code,
				Modified:     modified,
				Generated:    f.Generated,
				Drifted:      drifted,
				Stats:        stats(c, dstFile, f.Generated),
				RedFixes:     c.numUnsafeRewritesByReason,
			})
		}
	}
	return out, nil
}

func exprsWithNoType(cur *cursor, f *dst.File) map[dst.Expr]bool {
	out := map[dst.Expr]bool{}
	dstutil.Apply(f, func(c *dstutil.Cursor) bool {
		x, ok := c.Node().(dst.Expr)
		if !ok {
			return true
		}
		if _, ok := cur.typesInfo.types[x]; !ok {
			out[x] = true
		}
		return true
	}, nil)
	return out
}

// dstTypesInfo generates typesInfo from given types.Info and dst mapping.
func dstTypesInfo(orig *types.Info, dec *decorator.Decorator) *typesInfo {
	dstMap := dec.Dst
	info := &typesInfo{
		types:  map[dst.Expr]types.TypeAndValue{},
		defs:   map[*dst.Ident]types.Object{},
		uses:   map[*dst.Ident]types.Object{},
		astMap: dec.Ast.Nodes,
		dstMap: dec.Dst.Nodes,
	}

	for astExpr, tav := range orig.Types {
		if dstExpr, ok := dstMap.Nodes[astExpr]; ok {
			info.types[dstExpr.(dst.Expr)] = tav
		}
	}

	for astIdent, obj := range orig.Defs {
		if dstIdent, ok := dstMap.Nodes[astIdent]; ok {
			info.defs[dstIdent.(*dst.Ident)] = obj
		}
	}

	for astIdent, obj := range orig.Uses {
		if dstIdent, ok := dstMap.Nodes[astIdent]; ok {
			info.uses[dstIdent.(*dst.Ident)] = obj
		}
	}

	return info
}

// makeApplyFn adapts a rewrite to work with dstutil.Apply package.
func makeApplyFn(name string, showWork bool, f func(c *cursor) bool, cur *cursor) dstutil.ApplyFunc {
	if f == nil {
		return nil
	}
	cur.rewriteName = name
	return func(c *dstutil.Cursor) bool {
		cur.Logf("entering")
		defer cur.Logf("leaving")
		cur.Cursor = c
		return f(cur)
	}
}
