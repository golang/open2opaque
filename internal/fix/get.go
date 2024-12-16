// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// assignGet rewrites a direct scalar field access on the rhs of variable
// definitions, e.g.:
//
//	v := m.Field
//	=>
//	var v *fieldType
//	if m.HasField() {
//	  v = proto.Helper(m.GetField())
//	}
func assignGet(c *cursor) {
	n := c.Node()
	as, ok := n.(*dst.AssignStmt)
	if !ok {
		c.Logf("ignoring %T (looking for AssignStmt)", n)
		return
	}
	if len(as.Lhs) != 1 {
		c.Logf("ignoring: len(Lhs) != 1")
		return
	}
	lhsID, ok := as.Lhs[0].(*dst.Ident)
	if !ok {
		c.Logf("ignoring lhs %T (looking for Ident)", as.Lhs[0])
		return
	}
	if len(as.Rhs) != 1 {
		c.Logf("ignoring: len(Rhs) != 1")
		return
	}
	if as.Tok != token.DEFINE {
		c.Logf("ignoring %v (looking for token.DEFINE)", as.Tok)
		return
	}
	rhs := as.Rhs[0]
	if !isPtrToBasic(c.underlyingTypeOf(rhs)) {
		c.Logf("ignoring: accessed field is not a scalar field")
		return
	}
	if !c.isSideEffectFree(rhs) {
		c.Logf("ignoring: accessor expression is not side effect free")
		return
	}
	field, ok := c.trackedProtoFieldSelector(rhs)
	if !ok {
		c.Logf("ignoring: rhs is not a proto field selector")
		return
	}

	lhsExpr := dst.Clone(lhsID).(*dst.Ident)
	c.setType(lhsExpr, c.typeOf(lhsID))
	field2 := cloneSelectorExpr(c, field) // for Get
	asStmt := &dst.AssignStmt{
		Tok: token.DEFINE,
		Lhs: []dst.Expr{lhsExpr},
	}

	if hasNeeded(c, field) {
		field3 := cloneSelectorExpr(c, field) // for Get
		rhs := valueOrNil(c,
			// Intentionally drop node decorations to avoid spurious line breaks
			// inside a proto.ValueOrNil() call.
			sel2call(c, "Has", field2, nil, dst.NodeDecs{}),
			field3,
			*n.Decorations())
		// Avoid a line break between return and proto.ValueOrNil().
		rhs.Decorations().Before = dst.None
		asStmt.Rhs = append(asStmt.Rhs, rhs)
	} else {
		asStmt.Rhs = append(asStmt.Rhs, c.newProtoHelperCall(sel2call(c, "Get", field2, nil, *rhs.Decorations()), nil))
	}
	moveDecsBeforeStart(asStmt, asStmt.Rhs[0])
	c.ReplaceUnsafe(asStmt, PointerAlias)
}

// assignGet rewrites a direct scalar field access in a return statement, e.g.:
//
//	return m.Field
//	=>
//	if !m.HasField() {
//	 return nil
//	}
//	return proto.Helper(m.GetField())
func returnGet(c *cursor) {
	n := c.Node()
	rs, ok := n.(*dst.ReturnStmt)
	if !ok {
		c.Logf("ignoring %T (looking for ReturnStmt)", n)
		return
	}
	// Technically we could handle this case but it is not very common and
	// would require some work to properly clone all the nodes and keep
	// the types up to date.
	if len(rs.Results) != 1 {
		c.Logf("ignoring: len(Results) != 1")
		return
	}
	rhs := rs.Results[0]
	if !isPtrToBasic(c.underlyingTypeOf(rhs)) {
		c.Logf("ignoring: accessed field is not a scalar field")
		return
	}
	if !c.isSideEffectFree(rhs) {
		c.Logf("ignoring: accessor expression is not side effect free")
		return
	}
	field, ok := c.trackedProtoFieldSelector(rhs)
	if !ok {
		c.Logf("ignoring: rhs is not a proto field selector")
		return
	}

	ret := &dst.ReturnStmt{}
	if hasNeeded(c, field) {
		// return proto.ValueOrNil(m.HasField(), m.GetField)
		field1 := cloneSelectorExpr(c, field) // for Has
		field2 := cloneSelectorExpr(c, field) // for Get

		ret.Results = append(ret.Results, valueOrNil(c,
			// Intentionally drop node decorations to avoid spurious line breaks
			// inside a proto.ValueOrNil() call.
			sel2call(c, "Has", field1, nil, dst.NodeDecs{}),
			field2,
			*n.Decorations()))
	} else {
		// return proto.Helper(m.GetField())
		field2 := cloneSelectorExpr(c, field) // for Get
		ret.Results = append(ret.Results, c.newProtoHelperCall(sel2call(c, "Get", field2, nil, *rhs.Decorations()), nil))
	}
	moveDecsBeforeStart(ret, ret.Results[0])
	c.ReplaceUnsafe(ret, PointerAlias)
}

// Move decorations (line breaks and comments) from src to dest.
func moveDecsBeforeStart(dest, src dst.Node) {
	dest.Decorations().Before = src.Decorations().Before
	dest.Decorations().Start = src.Decorations().Start
	src.Decorations().Before = dst.None
	src.Decorations().Start = nil
}

// getPre rewrites the code to use Get methods. This function is executed by
// traversing the tree in preorder. getPre rewrites assignment and return
// statements that assign/return with exactly one direct scalar field access
// expression. getPost handles all other cases of direct field access rewrites
// that need getter.
func getPre(c *cursor) bool {
	if _, ok := c.Parent().(*dst.BlockStmt); !ok {
		c.Logf("ignoring node with parent of type %T (looking for BlockStmt)", c.Parent())
		return true
	}
	if !c.lvl.ge(Yellow) {
		return true
	}
	assignGet(c)
	returnGet(c)
	return true
}

// getPost rewrites the code to use Get methods. This function is executed by
// traversing the tree in postorder
func getPost(c *cursor) bool {
	// &m.F  => proto.Helper(m.GetF())   // proto3 scalars
	// &m.F  => no rewrite               // everything else
	if ue, ok := c.Node().(*dst.UnaryExpr); ok && ue.Op == token.AND && c.lvl.ge(Red) {
		field, ok := c.trackedProtoFieldSelector(ue.X)
		if !ok {
			return true
		}
		if t := c.typeOf(field); isScalar(t) && !isPtrToBasic(t) {
			c.ReplaceUnsafe(c.newProtoHelperCall(sel2call(c, "Get", field, nil, *c.Node().Decorations()), t), PointerAlias)
			return true
		}
		markMissingRewrite(field, "address of field")
		return true
	}
	if ue, ok := c.Parent().(*dst.UnaryExpr); ok && ue.Op == token.AND {
		return true
	}

	if isLValue(c) {
		return true
	}
	n, ok := c.Node().(dst.Expr)
	if !ok {
		return true
	}

	if _, ok := c.Parent().(*dst.IncDecStmt); ok {
		return true
	}

	// *m.F  =>  m.GetF()    for proto2 scalars
	if isDeref(n) && isBasic(c.underlyingTypeOf(n)) {
		field, ok := c.trackedProtoFieldSelector(dstutil.Unparen(addr(c, n)))
		if !ok {
			return true
		}
		c.Replace(sel2call(c, "Get", field, nil, *n.Decorations()))
		return true
	}

	field, ok := c.trackedProtoFieldSelector(n)
	if !ok {
		return true
	}

	// Oneofs are not fields (members of the oneof union are fields) and should
	// not have the Get method. In the open API, oneofs could be used as objects
	// of their own which was incompatible with the proto spec.
	//
	// Hence we have to explicitly ignore those cases.
	if isOneof(c.typeOf(field)) {
		if c.lvl.ge(Red) {
			c.numUnsafeRewritesByReason[OneofFieldAccess]++
			addCommentAbove(c.Parent(), field, "// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).")
		}
		return true
	}

	// m.F => m.GetF()   for all except proto2 scalar fields.
	if !isPtrToBasic(c.underlyingTypeOf(n)) {
		if isPtr(c.typeOf(field.X)) || c.canAddr(field.X) {
			c.Replace(sel2call(c, "Get", field, nil, *n.Decorations()))
		} else if c.lvl.ge(Red) {
			c.ReplaceUnsafe(sel2call(c, "Get", field, nil, *n.Decorations()), InexpressibleAPIUsage)
		}
		return true
	}

	// for proto2 scalars:
	//   m.F  =>  m.GetF().Enum()           for enums
	//   m.F  =>  proto.Helper(m.GetF())    otherwise
	if c.lvl.ge(Yellow) { // for proto2 scalars we loose aliasing
		// Don't do this rewrite:
		//   *m.F    =>    *proto.Helper(m.GetF())
		// as it rarely makes sense.
		//
		// We could get here if "*m.F" wasn't rewritten to "m.GetF()"
		// for some reason (e.g. we don't rewrite "*m.F++" to "m.GetF()++").
		if _, ok := c.Parent().(*dst.StarExpr); ok {
			return true
		}
		if hasNeeded(c, field) {
			c.ReplaceUnsafe(funcLiteralForHas(c, n, field), PointerAlias)
		} else {
			c.ReplaceUnsafe(sel2call(c, "Get", field, nil, *n.Decorations()), PointerAlias)
		}
		return true
	}

	return true
}

func funcLiteralForHas(c *cursor, n dst.Expr, field *dst.SelectorExpr) dst.Node {
	nodeElemType := c.typeOf(n)
	if ptr, ok := nodeElemType.(*types.Pointer); ok {
		nodeElemType = ptr.Elem()
	}
	msgType := c.typeOf(field.X)

	// We need two copies of field. They are identical.
	field1 := cloneSelectorExpr(c, field) // for Has
	field2 := cloneSelectorExpr(c, field) // for Get

	if c.isSideEffectFree(field.X) {
		// Call proto.ValueOrNil() directly, no function literal needed.
		return valueOrNil(c,
			sel2call(c, "Has", field1, nil, *n.Decorations()),
			field2,
			*n.Decorations())
	}

	var retElemType dst.Expr = &dst.Ident{Name: nodeElemType.String()}
	if named, ok := nodeElemType.(*types.Named); ok {
		pkgID := &dst.Ident{Name: c.imports.name(named.Obj().Pkg().Path())}
		c.setType(pkgID, types.Typ[types.Invalid])
		pkgSel := &dst.Ident{Name: named.Obj().Name()}
		c.setType(pkgSel, types.Typ[types.Invalid])
		retElemType = &dst.SelectorExpr{
			X:   pkgID,
			Sel: pkgSel,
		}
	}
	c.setType(retElemType, nodeElemType)
	retType := &dst.StarExpr{X: retElemType}
	c.setType(retType, types.NewPointer(nodeElemType))

	msgParamSel := c.selectorForProtoMessageType(msgType)
	msgParamType := &dst.StarExpr{X: msgParamSel}
	c.setType(msgParamType, msgType)

	msgParam := &dst.Ident{Name: "msg"}
	c.setType(msgParam, msgType)
	field1.X = &dst.Ident{Name: "msg"}
	c.setType(field1.X, msgType)
	field2.X = &dst.Ident{Name: "msg"}
	c.setType(field2.X, msgType)

	untypedNil := &dst.Ident{Name: "nil"}
	c.setType(untypedNil, types.Typ[types.UntypedNil])

	funcLit := &dst.FuncLit{
		// func(msg *pb.M2) <type> {
		Type: &dst.FuncType{
			Params: &dst.FieldList{
				List: []*dst.Field{
					&dst.Field{
						Names: []*dst.Ident{msgParam},
						Type:  msgParamType,
					},
				},
			},
			Results: &dst.FieldList{
				List: []*dst.Field{
					&dst.Field{
						Type: retType,
					},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				// return proto.ValueOrNil(…)
				&dst.ReturnStmt{
					Results: []dst.Expr{
						valueOrNil(c,
							sel2call(c, "Has", field1, nil, *n.Decorations()),
							field2,
							*n.Decorations()),
					},
				},
			},
		},
	}
	// We do not know whether the proto package was imported, so we may not be
	// able to construct the correct type signature. Set the type to invalid,
	// like we do for any code involving the proto package.
	c.setType(funcLit, types.Typ[types.Invalid])
	c.setType(funcLit.Type, types.Typ[types.Invalid])

	call := &dst.CallExpr{
		Fun: funcLit,
		Args: []dst.Expr{
			field.X,
		},
	}
	c.setType(call, c.typeOf(n))

	return call
}

func valueOrNil(c *cursor, has dst.Expr, sel *dst.SelectorExpr, decs dst.NodeDecs) *dst.CallExpr {
	fnsel := &dst.Ident{Name: "ValueOrNil"}
	get := sel2call(c, "Get", sel, nil, decs)
	fn := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: c.imports.name(protoImport)},
			Sel: fnsel,
		},
		Args: []dst.Expr{
			has,
			get.Fun,
		},
	}
	fn.Decs.NodeDecs = decs

	t := c.underlyingTypeOf(sel.Sel)
	var pkg *types.Package
	if use := c.objectOf(sel.Sel); use != nil {
		pkg = use.Pkg()
	}
	value := types.NewParam(token.NoPos, pkg, "_", t)
	recv := types.NewParam(token.NoPos, pkg, "_", c.underlyingTypeOf(sel.X))

	getterType := types.NewSignature(recv, types.NewTuple(), types.NewTuple(value), false)
	getterParam := types.NewParam(token.NoPos, pkg, "_", getterType)
	boolParam := types.NewParam(token.NoPos, pkg, "_", types.Typ[types.Bool])
	c.setType(fnsel, types.NewSignature(nil, types.NewTuple(boolParam, getterParam), types.NewTuple(value), false))
	c.setType(fn, t)

	c.setType(fn.Fun, c.typeOf(fnsel))

	// We set the type for "proto" identifier to Invalid because that's consistent with what the
	// typechecker does on new code. We need to distinguish "invalid" type from "no type was
	// set" as the code panics on the later in order to catch issues with missing type updates.
	c.setType(fn.Fun.(*dst.SelectorExpr).X, types.Typ[types.Invalid])
	return fn
}

func isDeref(n dst.Node) bool {
	_, ok := n.(*dst.StarExpr)
	return ok
}

// true if c.Node() is on left-hand side of an assignment
func isLValue(c *cursor) bool {
	p, ok := c.Parent().(*dst.AssignStmt)
	if !ok {
		return false
	}
	for _, ch := range p.Lhs {
		if ch == c.Node() {
			return true
		}
	}
	return false
}
