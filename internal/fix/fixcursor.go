// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strings"
	"unicode"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	log "github.com/golang/glog"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/types/typeutil"
	"google.golang.org/open2opaque/internal/ignore"
	"google.golang.org/open2opaque/internal/o2o/loader"
	"google.golang.org/open2opaque/internal/protodetecttypes"
)

// cursor is an argument to rewrite transformations. Rewrites can modify this state to share
// information with other DST nodes in the same package.
type cursor struct {
	*dstutil.Cursor // Describes node to rewrite (referred as "current node" below). See dstutil.Cursor.

	pkg        *loader.Package // Package containing the current node.
	curFile    *loader.File
	curFileDST *dst.File
	imports    *imports // Imports defined in the file containing the current node.
	typesInfo  *typesInfo
	loader     loader.Loader

	lvl Level // Specifies level of rewrites to apply to the current node.

	// A set of types to consider when updating code. Empty set means "update all".
	typesToUpdate map[string]bool

	// A set of types for which to always use builders, not setters.
	// (e.g. "google.golang.org/protobuf/types/known/timestamppb").
	//
	// An empty (or nil) builderTypes means: use setters or builders for
	// production/test code respectively (or follow the -use_builders flag if
	// set).
	builderTypes map[string]bool

	// A list of files for which to always use builders, not setters.
	builderLocations *ignore.List

	// A cache of shouldLogCompositeType results. The value for a given key can be:
	//  - missing: no information for that type
	//  - nil:     either:
	//             - the type is currently being processed (e.g. struct fields can refer
	//               to the struct type), or
	//             - the type was processed and it does not depend on protos
	//
	//  - non-nil: cached result for a type that should be processed
	// The cache serves as both: an optimization and a way to address cycles (see TestStats/recursive_type).
	//
	// The NoPtr version is used for calls with followPointers=false
	shouldLogCompositeTypeCache      *typeutil.Map
	shouldLogCompositeTypeCacheNoPtr *typeutil.Map

	// rewriteName is the function name of the open2opaque rewrite step that is
	// currently running, e.g. appendProtosPre.
	rewriteName string

	// ASTID is the astv.ASTID() for the current DST node.
	ASTID string

	// debugLog will be passed as astv.File.DebugLog
	debugLog map[string][]string

	// Where should the tool use builders?
	builderUseType BuilderUseType

	testonly bool // does this file belong to a testonly library?

	helperVariableNames map[string]bool

	numUnsafeRewritesByReason map[unsafeReason]int
}

func (c *cursor) Logf(format string, a ...any) {
	if c.ASTID == "" {
		return
	}
	msg := fmt.Sprintf(string(c.lvl)+"/"+c.rewriteName+": "+format, a...)
	c.debugLog[c.ASTID] = append(c.debugLog[c.ASTID], msg)
}

func (c *cursor) Replace(n dst.Node) {
	c.Cursor.Replace(n)
}

type unsafeReason int

const (
	// Unknown means the reason not nown or ambiguous
	Unknown unsafeReason = iota
	// PointerAlias means the rewrite removes pointer aliasing
	PointerAlias
	// SliceAlias means the rewrite removes slice aliasing
	SliceAlias
	// InexpressibleAPIUsage means the rewrite changes the behavior because
	// the original behavior cannot be expressed in the opaque API (this
	// usually leads to build failures).
	InexpressibleAPIUsage
	// PotentialBuildBreakage means the rewrite might induce a build breakage.
	PotentialBuildBreakage
	// EvalOrderChange means the evaluation order changes by the rewrite
	// (e.g. multi-assignments are rewritten into multiple single
	// assignments).
	EvalOrderChange
	// IncompleteRewrite means the rewrite is incomplete and further manual
	// changes are needed. In most cases a comment is left in the code.
	IncompleteRewrite
	// OneofFieldAccess means a oneof field is directly accessed in the
	// hybrid/open API. This cannot be expressed in the opaque API.
	OneofFieldAccess
	// ShallowCopy means the rewrite contains a shallow copy (before and
	// after) but shallow copies are unsafe in the opaque API.
	ShallowCopy
	// MaybeOneofChange means the rewrite produces code that might unset an
	// oneof field that was previously set to an invalid state (type set
	// but value not set).
	MaybeOneofChange
	// MaybeSemanticChange means the rewrite might produce invalid code
	// because identifier refer to different objects than before.
	MaybeSemanticChange
	// MaybeNilPointerDeref means the rewrite might produce code that leads
	// to nil pointer references that were not in the code before.
	MaybeNilPointerDeref
)

func (c *cursor) ReplaceUnsafe(n dst.Node, rt unsafeReason) {
	c.numUnsafeRewritesByReason[rt]++
	c.Replace(n)
}

func (c *cursor) InsertBefore(n dst.Node) {
	c.Cursor.InsertBefore(n)
}

func (c *cursor) underlyingTypeOf(expr dst.Expr) types.Type {
	return c.typeOf(expr).Underlying()
}

func (c *cursor) underlyingTypeOfOrNil(expr dst.Expr) types.Type {
	t := c.typeOfOrNil(expr)
	if t == nil {
		return nil
	}
	return t.Underlying()
}

func (c *cursor) isBuiltin(expr dst.Expr) bool {
	tv, ok := c.typesInfo.types[expr]
	return ok && tv.IsBuiltin()
}

func (c *cursor) hasType(expr dst.Expr) bool {
	return c.typesInfo.typeOf(expr) != nil
}

func (c *cursor) typeAndValueOf(expr dst.Expr) (types.TypeAndValue, bool) {
	tv, ok := c.typesInfo.types[expr]
	return tv, ok
}

func (c *cursor) typeOfOrNil(expr dst.Expr) types.Type {
	t := c.typesInfo.typeOf(expr)
	if t != nil {
		return t
	}
	// We don't know the type of "_" (it doesn't have one). Don't panic
	// because this is a known possibility and using "invalid" type here
	// makes writing rules using c.typeOf easier.
	if ident, ok := expr.(*dst.Ident); ok && ident.Name == "_" {
		return types.Typ[types.Invalid]
	}
	return nil
}

func (c *cursor) typeOf(expr dst.Expr) types.Type {
	if t := c.typeOfOrNil(expr); t != nil {
		return t
	}
	// The following code is unreachable. It exists to provide better error messages when there are
	// bugs in how we handle type information (e.g. forget to update type info map). This is
	// technically dead code but please don't delete it. It is very helpful during development.
	buf := new(bytes.Buffer)
	if err := dst.Fprint(buf, expr, dst.NotNilFilter); err != nil {
		buf = bytes.NewBufferString("<can't print the expression>")
	}
	panic(fmt.Sprintf("don't know type for expression %T %s", expr, buf.String()))
}

func (c *cursor) objectOf(ident *dst.Ident) types.Object {
	return c.typesInfo.objectOf(ident)
}

func (c *cursor) setType(expr dst.Expr, t types.Type) {
	c.typesInfo.types[expr] = types.TypeAndValue{Type: t}
}

func (c *cursor) setUse(ident *dst.Ident, o types.Object) {
	c.typesInfo.uses[ident] = o
	c.setType(ident, o.Type())
}

func (c *cursor) setVoidType(expr dst.Expr) {
	c.typesInfo.types[expr] = voidType
}

var voidType types.TypeAndValue

func (c *cursor) isTest() bool {
	return strings.HasSuffix(c.curFile.Path, "_test.go") || c.testonly
}

func findEnclosingLiteral(file *dst.File, needle *dst.CompositeLit) *dst.CompositeLit {
	var enclosing *dst.CompositeLit
	dstutil.Apply(file,
		func(cur *dstutil.Cursor) bool {
			if enclosing == nil {
				if cl, ok := cur.Node().(*dst.CompositeLit); ok {
					enclosing = cl
				}
			}

			return true
		},
		func(cur *dstutil.Cursor) bool {
			if cur.Node() == needle {
				return false // found the needle, stop traversal
			}

			if cur.Node() == enclosing {
				// Leaving the top-level *dst.CompositeLit, the needle was not
				// found.
				enclosing = nil
				return true
			}

			return true
		})
	return enclosing
}

func deepestNesting(lit *dst.CompositeLit) int {
	var nesting, deepest int
	dstutil.Apply(lit,
		func(cur *dstutil.Cursor) bool {
			if _, ok := cur.Node().(*dst.CompositeLit); ok {
				nesting++
				if nesting > deepest {
					deepest = nesting
				}
			}
			return true
		},
		func(cur *dstutil.Cursor) bool {
			if _, ok := cur.Node().(*dst.CompositeLit); ok {
				nesting--
			}
			return true
		})
	return deepest
}

func (c *cursor) messagesInvolved(lit *dst.CompositeLit) int {
	var involved int
	dstutil.Apply(lit,
		func(cur *dstutil.Cursor) bool {
			if cur.Node() == lit {
				// Do not count the top-level composite literal itself.
				return true
			}
			if cl, ok := cur.Node().(*dst.CompositeLit); ok {
				// Verify this composite literal is a proto message (builder or
				// actual message), as opposed to a []int32{…}, for example.
				t := c.typeOf(cl)
				if strings.HasSuffix(t.String(), "_builder") {
					involved++
					return true
				}
				if _, ok := c.messageTypeName(t); ok {
					involved++
				}
			}
			return true
		},
		nil)
	return involved
}

func (c *cursor) useBuilder(lit *dst.CompositeLit) bool {
	if c.builderUseType == BuildersEverywhere {
		return true
	}

	if c.builderLocations.Contains(c.curFile.Path) {
		return true
	}

	// We treat codelabs like test code: performance cannot be a concern, so
	// prefer readability.
	isTestOrCodelab := c.isTest() || strings.Contains(c.curFile.Path, "codelab")
	if c.builderUseType == BuildersTestsOnly && isTestOrCodelab {
		return true
	}

	elem := c.typeOf(lit)
	if ptr, ok := types.Unalias(elem).(*types.Pointer); ok {
		elem = ptr.Elem()
	}
	if named, ok := types.Unalias(elem).(*types.Named); ok {
		obj := named.Obj()
		typeName := obj.Pkg().Path() + "." + obj.Name()
		c.Logf("always use builders for %q? %v", typeName, c.builderTypes[typeName])
		if c.builderTypes[typeName] {
			return true
		}
	}

	// Check for the nesting level: too deeply nested literals cannot be
	// converted to setters without a loss of readability, so stick to builders
	// for these.
	enclosing := findEnclosingLiteral(c.curFileDST, lit)
	if enclosing == nil {
		c.Logf("BUG?! No enclosing literal found for %p / %v", lit, lit)
		return false // use setters
	}
	messagesInvolved := c.messagesInvolved(enclosing)
	deepestNesting := deepestNesting(enclosing)

	c.Logf("CompositeLit nesting: %d", deepestNesting)
	c.Logf("Messages involved: %d", messagesInvolved)
	// NOTE(stapelberg): The deepestNesting condition might seem irrelevant
	// thanks to the messagesInvolved condition, but keeping both allows us to
	// adjust the number of either threshold without having to disable/re-enable
	// the relevant code.
	if deepestNesting >= 4 || messagesInvolved >= 4 {
		return true // use builders
	}

	return false // use setters
}

// grabNameInScope finds and returns a free name (starting with prefix) in the
// provided scope, reserving it in the scope to ensure subsequent calls return a
// different name.
func grabNameInScope(pkg *types.Package, s *types.Scope, prefix string) string {
	// Find a free name
	name := prefix
	cnt := 2
	for _, obj := s.LookupParent(name, token.NoPos); obj != nil; _, obj = s.LookupParent(name, token.NoPos) {
		middle := ""
		if unicode.IsNumber(rune(prefix[len(prefix)-1])) {
			// Inject an extra h (stands for helper) if the prefix ends in a
			// number: Generate m2h2 instead of m22, which might be misleading.
			middle = "h"
		}
		name = fmt.Sprintf("%s%s%d", prefix, middle, cnt)
		cnt++
	}

	// Insert an object with this name in the scope so that subsequent calls of
	// grabNameInScope() cannot return the same name.
	s.Insert(types.NewTypeName(token.NoPos, pkg, name, nil))

	return name
}

func (c *cursor) helperNameFor(n dst.Node, t types.Type) string {
	// dst.Node objects do not have position information, so we need to
	// look up the corresponding ast.Node to get to the scope.
	astNode, ok := c.typesInfo.astMap[n]
	if !ok {
		log.Fatalf("BUG: %s: no corresponding go/ast node for dave/dst node %T / %p / %v", c.pkg.TypePkg.Path(), n, n, n)
	}
	inner := c.pkg.TypePkg.Scope().Innermost(astNode.Pos())
	if _, ok := astNode.(*ast.IfStmt); ok {
		// An *ast.IfStmt creates a new scope. For inserting a helper before the
		// *ast.IfStmt, we are interested in the parent scope, i.e. the scope
		// that contains the *ast.IfStmt, and our helper variable.
		inner = inner.Parent()
	}
	helperName := grabNameInScope(c.pkg.TypePkg, inner, helperVarNameForType(t))
	c.helperVariableNames[helperName] = true
	return helperName
}

func isCompositeLit(n, parent dst.Node) (*dst.CompositeLit, bool) {
	// CompositeLits are often wrapped in a UnaryExpr (&pb.M2{…}), but not
	// always: when defining a slice, the type is inferred, e.g. []*pb.M2{{…}}.
	expr, ok := n.(dst.Expr)
	if !ok {
		return nil, false
	}
	if ue, ok := expr.(*dst.UnaryExpr); ok {
		if ue.Op != token.AND {
			return nil, false
		}
		expr = ue.X
	} else {
		// Ensure the parent is not a UnaryExpr to avoid triggering twice for
		// UnaryExprs (once on the UnaryExpr, once on the contained
		// CompositeLit).
		if _, ok := parent.(*dst.UnaryExpr); ok {
			return nil, false
		}
	}
	lit, ok := expr.(*dst.CompositeLit)
	return lit, ok
}

// builderCLit returns a composite literal that is a non-empty one representing
// a protocol buffer.
func (c *cursor) builderCLit(n dst.Node, parent dst.Node) (*dst.CompositeLit, bool) {
	lit, ok := isCompositeLit(n, parent)
	if !ok {
		return nil, false
	}
	if len(lit.Elts) == 0 { // Don't use builders for constructing zero values.
		return nil, false
	}
	if !c.shouldUpdateType(c.typeOf(lit)) {
		return nil, false
	}
	for _, e := range lit.Elts {
		// This shouldn't be possible because of noUnkeyedLiteral (or
		// XXX_NoUnkeyedLiterals) field included in all structs representing
		// protocol buffers. We handle this just in case we run into a very old
		// proto.
		if _, ok := e.(*dst.KeyValueExpr); !ok {
			return nil, false
		}
	}
	return lit, true
}

func (c *cursor) selectorForProtoMessageType(t types.Type) *dst.SelectorExpr {
	// Get to the elementary type (pb.M2) if this is a pointer type (*pb.M2).
	elem := t
	if ptr, ok := types.Unalias(elem).(*types.Pointer); ok {
		elem = ptr.Elem()
	}
	named, ok := types.Unalias(elem).(*types.Named)
	if !ok {
		log.Fatalf("BUG: proto message unexpectedly not a named type (but %T)?!", elem)
	}
	obj := named.Obj()

	sel := &dst.SelectorExpr{
		X:   &dst.Ident{Name: c.imports.name(obj.Pkg().Path())},
		Sel: &dst.Ident{Name: obj.Name()},
	}
	c.setType(sel, t)
	c.setType(sel.X, types.Typ[types.Invalid])
	c.setType(sel.Sel, types.Typ[types.Invalid])
	return sel
}

// isSideEffectFree returns true if x can be safely called a different number of
// times after the rewrite. It returns false if it can't say for sure that
// that's the case.
//
// x must be one of "X", "X.F" or "X.GetF()" where any call is known to have no
// relevant side effects (e.g. method calls on protocol buffers).
func (c *cursor) isSideEffectFree(x dst.Expr) bool {
	var sel *dst.SelectorExpr
	switch x := x.(type) {
	case *dst.Ident:
		return true
	case *dst.BasicLit:
		return true
	case *dst.IndexExpr:
		return c.isSideEffectFree(x.Index) && c.isSideEffectFree(x.X)
	case *dst.SelectorExpr:
		sel = x
	case *dst.CallExpr:
		callSel, ok := x.Fun.(*dst.SelectorExpr)
		if !ok {
			return false
		}
		sel = callSel
		if _, ok := c.messageTypeName(c.typeOf(callSel.X)); !ok {
			return false
		}
	default:
		return false
	}

	var hasCall bool
	dstutil.Apply(sel, func(cur *dstutil.Cursor) bool {
		call, ok := cur.Node().(*dst.CallExpr)
		if !ok {
			return true
		}
		// If this is a method call on a proto then we don't report it as we know
		// that those don't have relevant side effects.
		if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
			if _, ok := c.messageTypeName(c.typeOf(sel.X)); ok {
				return true
			}
		}
		hasCall = true
		return true
	}, nil)
	return !hasCall
}

func isInterfaceVararg(t types.Type) bool {
	vararg, ok := types.Unalias(t).(*types.Slice)
	if !ok {
		return false
	}
	_, ok = types.Unalias(vararg.Elem()).(*types.Interface)
	return ok
}

func isString(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.String
}

// looksLikePrintf returns true for any function that looks like a printing one. For example:
// fmt.Print, log.Print, log.Info, (*testing.T).Error, etc.
//
// We can't enumerate all such functions so we use a heuristic that tries to classify a call
// expression based on its type.
func (c *cursor) looksLikePrintf(n dst.Node) bool {
	// We say that a function/method is a printer if:
	//  - its first argument is a "string" (format) and it is followed by "...interface{}" argument (arguments), or
	//  - its sole argument is "...interface{}" (arguments)
	call, ok := n.(*dst.CallExpr)
	if !ok {
		return false
	}
	if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
		if strings.HasSuffix(sel.Sel.Name, "Errorf") {
			return true
		}
	}
	sig, ok := types.Unalias(c.typeOf(call.Fun)).(*types.Signature)
	if !ok {
		return false
	}
	switch p := sig.Params(); p.Len() {
	case 1:
		return isInterfaceVararg(p.At(0).Type())
	case 2:
		return isString(p.At(0).Type()) && isInterfaceVararg(p.At(1).Type())
	default:
		return false
	}
}

// shouldUpdateType returns true for types which we consider to represent protocol buffers generated
// by the proto generator that the user requested to migrate. That is, types that should be
// considered for a rewrite during the open2opaque protocol buffer migration.
//
// There's also a shouldTrackType function, which returns true for types that we
// want to track for the migration purposes. For example, we want to track
// operations on type T in
//
//	type T pb.M
//
// but we don't want to rewrite accesses to type yet.
func (c *cursor) shouldUpdateType(t types.Type) bool {
	if !c.shouldTrackType(t) {
		return false
	}
	_, ok := c.messageTypeName(t)
	return ok
}

// messageTypeName returns the name of the protocol buffer message with type
// t. It returns an empty string and false if t is not a protocol buffer message
// type.
func (c *cursor) messageTypeName(t types.Type) (name string, ok bool) {
	name = t.String()

	if nt, ok := types.Unalias(t).(*types.Named); ok && isPtr(nt.Underlying()) {
		// A non-pointer named type whose underlying type is a pointer can be
		// neither proto struct nor pointer to a proto struct. If we were to
		// return "true" for such type, it was most likely "type T *pb.M" for some
		// proto type "pb.M".
		return "", false
	}

	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}

	nt, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return "", false
	}

	// ProtoMessage exists on proto structs, but not on custom named types based
	// on protos ("type T pb.M"). We don't want to rewrite code using such types
	// with rules that use messageTypeName.
	var hasProtoMessage bool
	for i := 0; i < nt.NumMethods(); i++ {
		if nt.Method(i).Name() == "ProtoMessage" {
			hasProtoMessage = true
		}
	}
	if !hasProtoMessage {
		return "", false
	}

	if st := (protodetecttypes.Type{T: t}); !st.IsMessage() || st.MessageAPI() == protodetecttypes.OpenAPI {
		return "", false
	}

	return strings.TrimPrefix(name, "*"), true
}

// canAddr returns true the expression if it's legal to take address of that
// expression.
func (c *cursor) canAddr(x dst.Expr) bool {
	tv, ok := c.typeAndValueOf(x)
	if !ok {
		return false
	}
	return tv.Addressable()
}

// enclosingStmt returns the closest ast.Node that opens a scope (opener) and
// the ast.Node representing this scope (scope). E.g. for
//
//	if ... {
//	  n
//	}
//
// it returns the ast.IfStmt and the ast.IfStmt.Body nodes.
// Either both or none of the return values are nil.
func (c *cursor) enclosingASTStmt(n dst.Node) (scope ast.Node, opener ast.Node) {
	lhsAst, ok := c.typesInfo.astMap[n]
	if !ok {
		c.Logf("BUG: no corresponding go/ast node for dave/dst node %T / %+v (was c.typesInfo.astMap not updated across rewrites?)", n, n)
		return nil, nil
	}
	path, _ := astutil.PathEnclosingInterval(c.curFile.AST, lhsAst.Pos(), lhsAst.End())
	for i := 0; i < len(path)-1; i++ {
		// Check for types that open a new scope.
		switch path[i].(type) {
		case *ast.BlockStmt,
			*ast.CaseClause,
			*ast.CommClause:
			return path[i], path[i+1]
		}
	}
	return nil, nil
}

// useClearOrHas returns true for field accesses (sel) that should be handled
// with either Has or Clear call in the context of a comparison/assignment
// with/to nil.
func (c *cursor) useClearOrHas(sel *dst.SelectorExpr) bool {
	if isOneof(c.typeOf(sel.Sel)) {
		return true
	}
	f := c.objectOf(sel.Sel).(*types.Var)
	// Handle messages (either proto2 or proto3) and proto2 enums.
	if p, ok := types.Unalias(f.Type()).(*types.Pointer); ok {
		if _, ok := types.Unalias(p.Elem()).(*types.Named); ok {
			return true
		}
	}

	// Handle non-enum scalars.
	if fieldHasExplicitPresence(c.typeOf(sel.X), f.Name()) {
		switch ft := types.Unalias(f.Type()).(type) {
		case *types.Pointer:
			if _, ok := types.Unalias(ft.Elem()).(*types.Basic); ok {
				return true
			}
		case *types.Slice:
			// Use Clear for bytes field, but not other repeated fields.
			if basic, ok := types.Unalias(ft.Elem()).(*types.Basic); ok && basic.Kind() == types.Uint8 {
				return true
			}
		}
	}
	return false
}

// fieldHasExplicitPresence reports whether the specified field has explicit
// presence: proto2 and edition 2023+ have explicit presence, proto3 defaults
// to implicit presence, but offers explicit presence through the optional
// keyword. See also https://protobuf.dev/programming-guides/field_presence/
//
// This function considers both value and pointer types to be protocol buffer
// messages.
func fieldHasExplicitPresence(m types.Type, field string) bool {
	p, ok := m.Underlying().(*types.Pointer)
	if ok {
		m = p.Elem()
	}
	s, ok := m.Underlying().(*types.Struct)
	if !ok {
		return true
	}

	numFields := s.NumFields()
	for i := 0; i < numFields; i++ {
		if s.Field(i).Name() == field {
			// Following relies on current generator behavior of having def value always being at
			// the end of the tag if it exists because value of "def" may contain ",".  It also
			// relies on the proto3 text not being in the first position.
			st := s.Tag(i)
			pb := reflect.StructTag(st).Get("protobuf")
			if i := strings.Index(pb, ",def="); i > -1 {
				pb = pb[:i]
			}
			if !strings.Contains(pb, ",proto3") {
				return true // not proto3? field has explicit presence
			}
			// proto3 optional fields are implemented with synthetic oneofs.
			return strings.Contains(pb, ",oneof")
		}
	}
	return true
}

// Exactly one of expr or t should be non-nil. expr can be nil only if t is *types.Basic
func (c *cursor) newProtoHelperCall(expr dst.Expr, t types.Type) dst.Expr {
	if _, ok := types.Unalias(t).(*types.Basic); expr == nil && !ok {
		panic(fmt.Sprintf("t must be *types.Basic if expr is nil, but it is %T", t))
	}
	if t == nil && expr == nil {
		panic("t and expr can't be both nil")
	}
	if t == nil {
		t = c.typeOf(expr)
	}

	// Enums are represented as named types in generated files.
	if t, ok := types.Unalias(t).(*types.Named); ok {
		fun := &dst.SelectorExpr{
			X:   expr,
			Sel: &dst.Ident{Name: "Enum"},
		}
		c.setType(fun, types.NewSignature(
			types.NewParam(token.NoPos, nil, "_", t),
			types.NewTuple(),
			types.NewTuple(types.NewParam(token.NoPos, nil, "_", types.NewPointer(t))),
			false))
		c.setType(fun.Sel, c.typeOf(fun))

		out := &dst.CallExpr{Fun: fun}
		c.setType(out, types.NewPointer(t))
		return out
	}
	bt := types.Unalias(t).(*types.Basic)
	if expr == nil {
		expr = scalarTypeZeroExpr(c, bt)
	}
	hname := basicTypeHelperName(bt)
	helper := c.imports.lookup(protoImport, hname)
	out := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: c.imports.name(protoImport)},
			Sel: &dst.Ident{Name: hname},
		},
		Args: []dst.Expr{expr},
	}
	c.setType(out, types.NewPointer(t))
	if helper != nil {
		c.setType(out.Fun, helper.Type())
		c.setUse(out.Fun.(*dst.SelectorExpr).Sel, helper)
	} else {
		// The "proto" package was not imported, so we do not have an actual
		// type to assign.
		c.setType(out.Fun, types.Typ[types.Invalid])
		c.setType(out.Fun.(*dst.SelectorExpr).Sel, types.Typ[types.Invalid])
	}
	// We set the type for "proto" identifier to Invalid because that's consistent with what the
	// typechecker does on new code. We need to distinguish "invalid" type from "no type was
	// set" as the code panics on the later in order to catch issues with missing type updates.
	c.setType(out.Fun.(*dst.SelectorExpr).X, types.Typ[types.Invalid])

	return out
}

// trackedProtoFieldSelector is a wrapper around protoFieldSelector that only
// returns the field selector if the underlying proto message type should be
// updated (i.e. it was specified in -types_to_update).
func (c *cursor) trackedProtoFieldSelector(expr dst.Node) (*dst.SelectorExpr, bool) {
	sel, ok := c.protoFieldSelector(expr)
	if !ok {
		return nil, false
	}
	t := c.typeOfOrNil(sel.X)
	if t == nil {
		return nil, false // skip over expression without type info (silo'ed?)
	}
	if !c.shouldUpdateType(t) {
		return nil, false
	}
	return sel, true
}

// protoFieldSelector checks whether expr is of the form "m.F" where m is a
// protocol buffer message and "F" is a field. It returns expr as DST selector
// if that's the case and true. Returns false otherwise.
func (c *cursor) protoFieldSelector(expr dst.Node) (*dst.SelectorExpr, bool) {
	sel, ok := expr.(*dst.SelectorExpr)
	if !ok {
		return nil, false
	}
	t := c.typeOfOrNil(sel.X)
	if t == nil {
		return nil, false // skip over expression without type info (silo'ed?)
	}
	if _, messageType := c.messageTypeName(t); !messageType {
		return nil, false
	}
	if strings.HasPrefix(sel.Sel.Name, "XXX_") {
		return nil, false
	}
	if _, ok := types.Unalias(c.underlyingTypeOf(sel.Sel)).(*types.Signature); ok {
		return nil, false
	}
	return sel, true
}

// protoFieldSelectorOrAccessor is like protoFieldSelector, but also permits
// accessor methods like GetX, HasX, ClearX, SetX. If the expression is an
// accessor, the second return value contains its signature (nil otherwise).
func (c *cursor) protoFieldSelectorOrAccessor(expr dst.Node) (*dst.SelectorExpr, *types.Signature, bool) {
	sel, ok := expr.(*dst.SelectorExpr)
	if !ok {
		return nil, nil, false
	}
	t := c.typeOfOrNil(sel.X)
	if t == nil {
		return nil, nil, false // skip over expression without type info (silo'ed?)
	}
	if _, messageType := c.messageTypeName(t); !messageType {
		return nil, nil, false
	}
	if strings.HasPrefix(sel.Sel.Name, "XXX_") {
		return nil, nil, false
	}
	if sig, ok := types.Unalias(c.underlyingTypeOf(sel.Sel)).(*types.Signature); ok {
		if strings.HasPrefix(sel.Sel.Name, "Has") ||
			strings.HasPrefix(sel.Sel.Name, "Clear") ||
			strings.HasPrefix(sel.Sel.Name, "Set") ||
			strings.HasPrefix(sel.Sel.Name, "Get") {
			return sel, sig, true
		}
		return nil, nil, false
	}
	return sel, nil, true
}
