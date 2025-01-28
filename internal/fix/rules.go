// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"fmt"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"golang.org/x/exp/slices"
)

func init() {
	rewrites = []rewrite{
		// outputparam.go
		{name: "outputParamPre", pre: outputParamPre},
		// usepointers.go
		{name: "usePointersPre", pre: usePointersPre},
		// incdec.go
		{name: "incDecPre", pre: incDecPre},
		// The hasPre stage needs to run before convertToSetterPost because it
		// generates direct fields accesses on the lhs of assignments which
		// convertToSetterPost rewrites to setters.
		//
		// has.go
		{name: "hasPre", pre: hasPre},
		// converttosetter.go
		{name: "convertToSetterPost", post: convertToSetterPost},
		// oneofswitch.go
		{name: "oneofSwitchPost", pre: oneofSwitchPost},
		// appendprotos.go
		{name: "appendProtosPre", pre: appendProtosPre},
		// The assignSwapPre stage needs to run before assignPre and getPost
		// because it untangles swap assignments into two assignments, which
		// will afterwards be rewritten into getters (getPost) and setters
		// (assignPre).
		//
		// assignswap.go
		{name: "assignSwapPre", pre: assignSwapPre},
		// get.go
		{name: "getPre", pre: getPre},
		{name: "getPost", post: getPost},
		// assign.go
		{name: "assignPre", pre: assignPre},
		{name: "assignOpPre", pre: assignOpPre},
		{name: "assignPost", post: assignPost},
		// build.go
		{name: "buildPost", post: buildPost},
	}
}

const protoImport = "google.golang.org/protobuf/proto"

func markMissingRewrite(n dst.Node, what string) {
	marker := fmt.Sprintf("/* DO_NOT_SUBMIT: missing rewrite for %s */", what)
	decs := n.Decorations()
	for _, d := range decs.End {
		if d == marker {
			return
		}
	}
	decs.End = append([]string{marker}, decs.End...)
}

// visitorFunc is a convenience type to use a function literal as a dst.Visitor.
type visitorFunc func(n dst.Node) dst.Visitor

// Visit implements dst.Visitor.
func (v visitorFunc) Visit(n dst.Node) dst.Visitor {
	return v(n)
}

var (
	dstExprType  = reflect.TypeOf((*dst.Expr)(nil)).Elem()
	dstIdentType = reflect.TypeOf((*dst.Ident)(nil))
)

func isStatementOrDeclaration(n dst.Node) bool {
	switch n.(type) {
	case dst.Stmt:
		return true
	case dst.Decl:
		return true
	default:
		return false
	}
}

// addCommentAbove locates the specified expression (can be part of a line),
// walks up to the parent nodes until it encounters a statement or declaration
// (full line) and adds the specified comment.
func addCommentAbove(root dst.Node, expr dst.Expr, comment string) {
	// Pre-allocate to avoid memory allocations up until 100 levels of nesting.
	stack := make([]dst.Node, 0, 100)

	dstutil.Apply(root,
		func(cur *dstutil.Cursor) bool {
			verdict := true // keep recursing
			if cur.Node() == expr {
				// Insert comment above the closest dst.Stmt or dst.Decl.
				for i := len(stack) - 1; i >= 0; i-- {
					if !isStatementOrDeclaration(stack[i]) {
						continue
					}
					decs := stack[i].Decorations()
					if slices.Contains(decs.Start, comment) {
						// This statement contains multiple expressions, but the
						// comment was already added. Skip so that we do not add
						// the same comment multiple times.
						break
					}
					decs.Start = append(decs.Start, comment)
					decs.Before = dst.NewLine
					break
				}
				verdict = false // stop recursing
			}
			stack = append(stack, cur.Node()) // push
			return verdict
		},
		func(cur *dstutil.Cursor) bool {
			stack = stack[:len(stack)-1] // pop
			return true
		})
}

// scalarTypeZeroExpr returns an expression for zero value of the given type
// which must be a protocol buffer scalar type.
func scalarTypeZeroExpr(c *cursor, t types.Type) dst.Expr {
	out := &dst.Ident{}
	c.setType(out, t)

	if isBytes(t) {
		out.Name = "nil"
		return out
	}

	if isEnum(t) {
		out.Name = "0"
		return out
	}

	bt, ok := types.Unalias(t).(*types.Basic)
	if !ok {
		panic(fmt.Sprintf("scalarTypeZeroExpr called with %T", t))
	}
	basicLit := &dst.BasicLit{}
	c.setType(basicLit, bt)
	switch bt.Kind() {
	case types.UntypedBool, types.Bool:
		out.Name = "false"
		return out
	case types.UntypedInt, types.Int, types.Int32, types.Int64, types.Uint32, types.Uint64:
		basicLit.Kind = token.INT
		basicLit.Value = "0"
	case types.UntypedFloat, types.Float32, types.Float64:
		basicLit.Kind = token.FLOAT
		basicLit.Value = "0.0"
	case types.UntypedString, types.String:
		basicLit.Kind = token.STRING
		basicLit.Value = `""`
	default:
		panic(fmt.Sprintf("unrecognized kind %d of type %s", bt.Kind(), t))
	}
	return basicLit
}

func basicTypeHelperName(t *types.Basic) string {
	switch t.Kind() {
	case types.Bool:
		return "Bool"
	case types.Int:
		return "Int"
	case types.Int32:
		return "Int32"
	case types.Int64:
		return "Int64"
	case types.Uint32:
		return "Uint32"
	case types.Uint64:
		return "Uint64"
	case types.Float32:
		return "Float32"
	case types.Float64:
		return "Float64"
	case types.String:
		return "String"
	case types.UntypedBool:
		return "Bool"
	case types.UntypedInt:
		return "Int"
	case types.UntypedFloat:
		return "Float"
	case types.UntypedString:
		return "String"
	default:
		panic(fmt.Sprintf("unrecognized kind %d of type %s", t.Kind(), t))
	}
}

func addr(c *cursor, expr dst.Expr) dst.Expr {
	if e, ok := expr.(*dst.StarExpr); ok {
		return e.X
	}
	out := &dst.UnaryExpr{
		Op: token.AND,
		X:  expr,
	}
	updateASTMap(c, expr, out)
	c.setType(out, types.NewPointer(c.typeOf(expr)))
	return out
}

func isAddr(expr dst.Node) bool {
	ue, ok := expr.(*dst.UnaryExpr)
	return ok && ue.Op == token.AND
}

// expr2stmt wraps an expression as a statement
func (c *cursor) expr2stmt(expr dst.Expr, src dst.Node) *dst.ExprStmt {
	stmt := &dst.ExprStmt{X: expr}
	updateASTMap(c, src, stmt)
	return stmt
}

func isPtr(t types.Type) bool {
	_, ok := t.Underlying().(*types.Pointer)
	return ok
}

func isBasic(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.Basic)
	return ok
}

func isBytes(t types.Type) bool {
	s, ok := types.Unalias(t).(*types.Slice)
	if !ok {
		return false
	}
	elem, ok := types.Unalias(s.Elem()).(*types.Basic)
	return ok && elem.Kind() == types.Byte
}

func isEnum(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	_, ok = n.Underlying().(*types.Basic)
	return ok
}

func isScalar(t types.Type) bool {
	return isBasic(t) || isEnum(t) || isBytes(t)
}

func isMsg(t types.Type) bool {
	p, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	_, ok = p.Elem().Underlying().(*types.Struct)
	return ok
}

func isOneof(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

func isOneofWrapper(c *cursor, x dst.Expr) bool {
	t := c.underlyingTypeOf(x)
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem().Underlying()
	}
	s, ok := types.Unalias(t).(*types.Struct)
	if !ok || s.NumFields() != 1 {
		return false
	}
	for _, tag := range strings.Split(reflect.StructTag(s.Tag(0)).Get("protobuf"), ",") {
		if tag == "oneof" {
			return true
		}
	}
	return false
}

func isPtrToBasic(t types.Type) bool {
	p, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	_, ok = p.Elem().Underlying().(*types.Basic)
	return ok
}

// sel2call converts a selector expression to a call expression (a direct field access to a method
// call). For example, "m.F = v" is replaced with "m.SetF(v)" where "m.F" is the selector
// expression, "v" is the val, and "Set" is the prefix.
//
// If val is nil, it is not added as an argument to the call expression. Prefix can be one of "Get",
// "Set", "Clear", "Has",
//
// This function does the necessary changes to field names to resolve conflicts.
func sel2call(c *cursor, prefix string, sel *dst.SelectorExpr, val dst.Expr, decs dst.NodeDecs) *dst.CallExpr {
	name := fixConflictingNames(c.typeOf(sel.X), prefix, sel.Sel.Name)
	fnsel := &dst.Ident{
		Name: prefix + name,
	}
	fn := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   sel.X,
			Sel: fnsel,
		},
	}
	fn.Decs.NodeDecs = decs
	if val != nil {
		val = dstutil.Unparen(val)
		fn.Args = []dst.Expr{val}
	}

	t := c.underlyingTypeOf(sel.Sel)
	if isPtrToBasic(t) {
		t = types.Unalias(t).(*types.Pointer).Elem()
	}
	var pkg *types.Package
	if use := c.objectOf(sel.Sel); use != nil {
		pkg = use.Pkg()
	}
	value := types.NewParam(token.NoPos, pkg, "_", t)
	recv := types.NewParam(token.NoPos, pkg, "_", c.underlyingTypeOf(sel.X))
	switch prefix {
	case "Get":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(value), false))
		c.setType(fn, t)
	case "Set":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(value), types.NewTuple(), false))
		c.setVoidType(fn)
	case "Clear":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(), false))
		c.setVoidType(fn)
	case "Has":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(types.NewParam(token.NoPos, pkg, "_", types.Typ[types.Bool])), false))
		c.setType(fn, types.Typ[types.Bool])
	default:
		panic("bad function name prefix '" + prefix + "'")
	}
	c.setType(fn.Fun, c.typeOf(fnsel))
	return fn
}

// NewSrc creates a test Go package for test examples.
//
// Tests can access:
//
//	a fake version of the proto package, "proto"
//	a fake generated profile package, "pb"
//	a fake proto2 object, "m2"
//	a fake proto3 object, "m3"
//
// Note that there's only one instance of "m2" and "m3". We may need more
// instances when the analysis is smart enough to do different rewrites for
// operations on two different objects than on a single one. Currently, for
// example:
//
//	m2.S = m2.S
//
// is not recognized as having the same object on both sides. Hence we consider
// it as losing aliasing and clear semantics when rewritten as:
//
//	m2.SetS(m2.GetS())
//
// newSrc doesn't introduce new access patterns recognized by the migration tool
// so that tests can rely on all returned accesses coming from code added in
// those tests.
func NewSrc(in, extra string) string {
	return `// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

import pb2 "google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto"
import pb3 "google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto"
import proto "google.golang.org/protobuf/proto"
import "unsafe"
import "context"

var _ unsafe.Pointer
var _ = proto.String
var _ = context.Background

func test_function() {
	m2 := new(pb2.M2)
	m2a := new(pb2.M2)
	_, _ = m2, m2a
	m3 := new(pb3.M3)
	_ = m3
	_ = "TEST CODE STARTS HERE"
` + in + `
	_ = "TEST CODE ENDS HERE"
}
` + extra
}
