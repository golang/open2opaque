// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/token"
	"go/types"

	"github.com/dave/dst"
)

// assignPre rewrites assignments. This function is executed by traversing the tree in preorder.
// Assignments with operations are handled by assignOpPre.
func assignPre(c *cursor) bool {
	stmt, ok := c.Node().(*dst.AssignStmt)
	if !ok {
		c.Logf("ignoring %T (looking for AssignStmt)", c.Node())
		return true
	}
	if stmt.Tok != token.ASSIGN {
		c.Logf("ignoring AssignStmt with Tok %v (looking for ASSIGN)", stmt.Tok)
		return true
	}

	// Not handled: shallow copy support.

	if len(stmt.Lhs) != len(stmt.Rhs) {
		c.Logf("ignoring AssignStmt with len(lhs)=%d != len(rhs)=%d", len(stmt.Lhs), len(stmt.Rhs))
		// Not handled: assignments where len(lhs)  != len(rhs):
		//  - calls
		//  - chan ops
		//  - map Access
		//  - type assertions
		return true
	}

	// Handle the most common case: single assignment.
	if len(stmt.Lhs) == 1 {
		c.Logf("len(lhs) = 1")
		lhs, rhs := stmt.Lhs[0], stmt.Rhs[0]

		// *m.F = v    =>    m.SetF(v)
		if star, ok := lhs.(*dst.StarExpr); ok {
			c.Logf("lhs is a StarExpr")
			sel, ok := c.trackedProtoFieldSelector(star.X)
			if !ok {
				c.Logf("ignoring: lhs is not a proto field selector")
				return true
			}
			c.Replace(c.expr2stmt(sel2call(c, "Set", sel, rhs, *stmt.Decorations()), sel))
			c.Logf("rewriting AssignStmt")
			return true
		}

		// Handle "m.F = v"
		field, ok := c.protoFieldSelector(lhs)
		if !ok {
			c.Logf("ignoring: lhs is not a proto field selector")
			return true
		}
		// Check if either side is a proto field selector in -types_to_update
		// and update the whole assignment.
		_, lhsOk := c.trackedProtoFieldSelector(lhs)
		_, rhsOk := c.trackedProtoFieldSelector(rhs)
		if !lhsOk && !rhsOk {
			c.Logf("ignoring: neither lhs nor rhs are (tracked) proto field selectors")
			return true
		}
		c.Logf("attempting to rewrite...")
		if a, ok := rewriteFieldAssign(c, field, rhs, *stmt.Decorations()); ok {
			c.Logf("...success")
			c.Replace(a)
		} else {
			c.Logf("...failure")
		}

		return true
	}

	// Rewriting multi-assignment may change order of evaluation. Hence this is a yellow rewrite.
	if c.lvl.le(Green) {
		return true
	}

	if err := assignmentIsSwap(c, stmt); err == nil && !c.lvl.ge(Yellow) {
		// Swaps are yellow rewrites because getPost only handles them in the yellow level.
		return true
	} else if err != nil {
		c.Logf("%s", err.Error())
	}

	// Multi-assignment in simple statements is handled by assignPost. It requires updating
	// grandparent (calling parent.InsertBefore) of the rewritten node and the dstutil.Cursor API
	// doesn't provide a way to do that. Here, we only update statements in blocks because we can
	// call c.InsertBefore.

	if _, ok := c.Cursor.Parent().(*dst.BlockStmt); !ok {
		c.Logf("ignoring: multi-assignment outside of BlockStmt (handled by assignPost)")
		return true
	}

	// Don't change the multi-assignment structure if there are no proto-related
	// rewrites in there.
	var usesProtos bool
	for _, lhs := range stmt.Lhs {
		if star, ok := lhs.(*dst.StarExpr); ok {
			if _, ok := c.trackedProtoFieldSelector(star.X); ok {
				usesProtos = true
				break
			}
		}
		if _, ok := c.trackedProtoFieldSelector(lhs); ok {
			usesProtos = true
			break
		}
	}
	if !usesProtos {
		c.Logf("ignoring AssignStmt without protos")
		return true
	}

	var decs dst.NodeDecs
	lastIdx := len(stmt.Lhs) - 1
	for i, lhs := range stmt.Lhs {
		switch i {
		case 0:
			decs = dst.NodeDecs{
				Before: stmt.Decorations().Before,
				Start:  stmt.Decorations().Start,
			}
		case lastIdx:
			decs = dst.NodeDecs{
				After: stmt.Decorations().After,
				End:   stmt.Decorations().End,
			}
		default:
			decs = dst.NodeDecs{}
		}
		rhs := stmt.Rhs[i]

		// rewrite "*m.F = v"
		if star, ok := lhs.(*dst.StarExpr); ok {
			if sel, ok := c.trackedProtoFieldSelector(star.X); ok {
				c.Logf("rewriting %v = %v", sel.Sel.Name, rhs)
				c.InsertBefore(c.expr2stmt(sel2call(c, "Set", sel, rhs, *stmt.Decorations()), sel))
			} else {
				c.Logf("ignoring: lhs is not a proto field selector")
			}
			continue
		}

		// rewrite "m.F = v"
		if field, ok := c.trackedProtoFieldSelector(lhs); ok {
			c.Logf("lhs %d is a proto field selector", i)
			if a, ok := rewriteFieldAssign(c, field, rhs, decs); ok {
				c.Logf("rewriting %v = %v", field.Sel.Name, rhs)
				c.InsertBefore(a)
				continue
			}
		}
		as := &dst.AssignStmt{
			Lhs: []dst.Expr{lhs},
			Tok: token.ASSIGN,
			Rhs: []dst.Expr{rhs},
		}
		as.Decs.NodeDecs = decs
		c.Logf("rewriting %v = %v", lhs, rhs)
		c.InsertBefore(as)
	}
	c.Delete()
	return true
}

// assignPost rewrites multi-assignments in simple statements. This function is executed by
// traversing the tree in postorder.
func assignPost(c *cursor) bool {
	// Splitting multi-assignments is a yellow rewrite because it changes order of evaluation.
	if c.lvl.le(Green) {
		return true
	}

	// If either expression a or b is a proto direct field access, rewrite:
	//
	//    if a, b = f(), g(); cond {  // Similar for init statements in "for" and "switch".
	//      =>
	//    a = f()
	//    b = g()
	//    if cond {
	//
	// and apply single-assignment rewrite rules for individual assign statements.
	var initStmt *dst.AssignStmt
	n := c.Node()
	switch n := n.(type) {
	case *dst.IfStmt:
		if isMultiProtoAssign(c, n.Init) {
			initStmt = n.Init.(*dst.AssignStmt)
			n.Init = nil
		}
	case *dst.ForStmt:
		if isMultiProtoAssign(c, n.Init) {
			initStmt = n.Init.(*dst.AssignStmt)
			n.Init = nil
		}
	case *dst.SwitchStmt:
		if isMultiProtoAssign(c, n.Init) {
			initStmt = n.Init.(*dst.AssignStmt)
			n.Init = nil
		}
	case *dst.TypeSwitchStmt:
		if isMultiProtoAssign(c, n.Init) {
			initStmt = n.Init.(*dst.AssignStmt)
			n.Init = nil
		}
	}
	if initStmt == nil {
		return true
	}
	for i, lhs := range initStmt.Lhs {
		rhs := initStmt.Rhs[i]
		if a, ok := rewriteFieldAssign(c, lhs, rhs, dst.NodeDecs{}); ok {
			c.InsertBefore(a)
			continue
		}
		c.InsertBefore(&dst.AssignStmt{
			Lhs: []dst.Expr{lhs},
			Tok: token.ASSIGN,
			Rhs: []dst.Expr{rhs},
		})
	}
	c.numUnsafeRewritesByReason[EvalOrderChange]++
	return true
}

// assignOpPre rewrites assignment operations (x op= y).
// This function is executed by traversing the tree in preorder.
func assignOpPre(c *cursor) bool {
	stmt, ok := c.Node().(*dst.AssignStmt)
	if !ok {
		return true
	}
	if stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE {
		return false
	}

	// Assignment operations must have exactly one lhs and one rhs value,
	// see https://go.dev/ref/spec#Assignment_statements.
	lhs, rhs := stmt.Lhs[0], stmt.Rhs[0]

	if star, ok := lhs.(*dst.StarExpr); ok {
		lhs = star.X
	}
	sel, ok := c.trackedProtoFieldSelector(lhs)
	if !ok {
		return false
	}

	tok := stmt.Tok
	switch stmt.Tok {
	case token.ADD_ASSIGN:
		tok = token.ADD
	case token.SUB_ASSIGN:
		tok = token.SUB
	case token.MUL_ASSIGN:
		tok = token.MUL
	case token.QUO_ASSIGN:
		tok = token.QUO
	case token.REM_ASSIGN:
		tok = token.REM
	case token.AND_ASSIGN:
		tok = token.AND
	case token.OR_ASSIGN:
		tok = token.OR
	case token.XOR_ASSIGN:
		tok = token.XOR
	case token.SHL_ASSIGN:
		tok = token.SHL
	case token.SHR_ASSIGN:
		tok = token.SHR
	case token.AND_NOT_ASSIGN:
		tok = token.AND_NOT
	default:
		c.Logf("unexpected token: %v", stmt.Tok)
		return false
	}
	binExpr := &dst.BinaryExpr{
		X:  sel2call(c, "Get", sel, nil, dst.NodeDecs{}),
		Op: tok,
		Y:  rhs,
	}
	c.setType(binExpr, c.typeOf(lhs))

	startEndDec := dst.NodeDecs{
		Start: stmt.Decorations().Start,
		End:   stmt.Decorations().End,
	}
	selClone := cloneSelectorExpr(c, sel)
	c.Replace(c.expr2stmt(sel2call(c, "Set", selClone, binExpr, startEndDec), selClone))

	return false
}

func isNeverNilSliceExpr(c *cursor, e dst.Expr) bool {
	// Is this a string to byte conversion?
	if ce, ok := e.(*dst.CallExpr); ok && len(ce.Args) == 1 {
		if bt, ok := types.Unalias(c.typeOf(ce.Args[0])).(*types.Basic); ok && bt.Kind() == types.String {
			if at, ok := ce.Fun.(*dst.ArrayType); ok {
				if id, ok := at.Elt.(*dst.Ident); ok && id.Name == "byte" {
					return true
				}
			}
		}
	}
	if _, ok := e.(*dst.CompositeLit); ok {
		if _, ok := types.Unalias(c.typeOf(e)).(*types.Slice); ok {
			return true
		}
	}
	se, ok := e.(*dst.SliceExpr)
	if !ok {
		return false
	}
	if bl, ok := se.Low.(*dst.BasicLit); ok && bl.Value != "0" {
		return true
	}
	if bl, ok := se.High.(*dst.BasicLit); ok && bl.Value != "0" {
		return true
	}
	return false
}

// rewriteFieldAssign rewrites a direct field assignment
//
//	lhs = rhs    where lhs is "m.F"
//
// to a form that works in the opaque proto API world.
func rewriteFieldAssign(c *cursor, lhs, rhs dst.Expr, decs dst.NodeDecs) (dst.Stmt, bool) {
	lhsSel, ok := c.protoFieldSelector(lhs)
	if !ok {
		c.Logf("ignoring: lhs is not a proto field selector")
		return nil, false
	}

	// Drop parens around rhs, if any. Rhs becomes an argument to a function call. It's never
	// necessary to keep it in parenthesis. One situation where rhs is a ParenExpr happens when it
	// used to be a composite literal in a simple statement. For example:
	//
	//    if _, M.F = nil, (&pb.M{}); {
	if pe, ok := rhs.(*dst.ParenExpr); ok {
		rhs = pe.X
	}

	// m.F = proto.{String, Int, ...}(V)   =>   m.SetF(V)
	if arg, ok := protoHelperCall(c, rhs); ok {
		c.Logf("rewriting proto helper call")
		return c.expr2stmt(sel2call(c, "Set", lhsSel, arg, decs), lhsSel), true
	}

	// m.F = pb.EnumValue.Enum()  =>  m.SetF(pb.EnumValue)
	if enumVal, ok := enumHelperCall(c, rhs); ok {
		c.Logf("rewriting Enum() call")

		if t := c.typeOfOrNil(enumVal); t != nil {
			if pt, ok := types.Unalias(t).(*types.Pointer); ok {
				enumVal = &dst.StarExpr{X: enumVal}
				c.setType(enumVal, pt.Elem())
			}
		}

		return c.expr2stmt(sel2call(c, "Set", lhsSel, enumVal, decs), lhsSel), true
	}

	// m.F = new(MsgType)     =>  m.SetF(new(MsgType))
	// m.F = new(BasicType)   =>  m.SetF(ZeroValueOf(BasicType))
	// m.F = new(EnumType)    =>  m.SetF(EnumType(0))
	if arg, ok := newConstructorCall(c, rhs); ok {
		c.Logf("rewriting constructor")
		return c.expr2stmt(sel2call(c, "Set", lhsSel, arg, decs), lhsSel), true
	}

	// m.F = nil    =>   m.ClearF()
	if ident, ok := rhs.(*dst.Ident); ok && ident.Name == "nil" {
		c.Logf("rewriting nil assignment...")
		if c.useClearOrHas(lhsSel) {
			c.Logf("...with Clear()")
			return c.expr2stmt(sel2call(c, "Clear", lhsSel, nil, decs), lhsSel), true
		}
		c.Logf("...with Set()")
		return c.expr2stmt(sel2call(c, "Set", lhsSel, rhs, decs), lhsSel), true
	}

	// This condition is intentionally placed after the ClearF condition just
	// above so that we do not need to handle the nil assignment case.
	//
	//   m.F = ptrToEnum
	// =>
	//   if ptrToEnum != nil {
	//     m1.SetF(*ptrToEnum)
	//   } else {
	//     m1.ClearF()
	//   }
	if t := c.typeOfOrNil(lhsSel); t != nil && isPtr(t) {
		et := t.Underlying().(*types.Pointer).Elem()
		_, fieldCopy := c.trackedProtoFieldSelector(rhs)
		c.Logf("isEnum(%v) = %v, fieldCopy = %v", et, isEnum(et), fieldCopy)
		if !fieldCopy && isEnum(et) {
			// special case for m.E = &eVal => m.SetE(eVal)
			if ue, ok := rhs.(*dst.UnaryExpr); ok && ue.Op == token.AND {
				stmt := c.expr2stmt(sel2call(c, "Set", lhsSel, ue.X, decs), lhsSel)
				return stmt, true
			}
			lhs2 := cloneSelectorExpr(c, lhsSel)
			ifStmt, v := ifNonNil(c, lhsSel, rhs, decs)
			ifStmt.Body.List = []dst.Stmt{
				c.expr2stmt(sel2call(c, "Set", lhs2, deref(c, cloneExpr(c, v)), dst.NodeDecs{}), lhs2),
			}
			return ifStmt, true
		}
	}

	// m.F = []byte(v)       =>   m.SetF([]byte(v))
	// m.F = []byte("...")   =>   m.SetF([]byte("..."))
	if isBytesConversion(c, rhs) {
		c.Logf("rewriting []byte conversion")
		return c.expr2stmt(sel2call(c, "Set", lhsSel, rhs, decs), lhsSel), true
	}

	// For proto2 bytes field, rewrite statement
	//   m.F = Expr
	// =>
	//   if x := Expr; x != nil {
	//     m.SetF(x)
	//   }
	//
	// Or, if we cannot determine whether Clear() is safe to omit:
	//
	//   if x := Expr; x != nil {
	//     m.SetF(x)
	//   } else {
	//     m.ClearF()
	//   }
	//
	// Can only rewrite for statements and not for expressions.
	if _, ok := c.Parent().(*dst.BlockStmt); ok {
		c.Logf("rewriting assignment with rhs Expr")
		f := c.objectOf(lhsSel.Sel).(*types.Var)
		explicitPresence := fieldHasExplicitPresence(c.typeOf(lhsSel.X), f.Name())
		if slice, ok := types.Unalias(f.Type()).(*types.Slice); ok && explicitPresence {
			if basic, ok := types.Unalias(slice.Elem()).(*types.Basic); ok && basic.Kind() == types.Uint8 {
				if isNeverNilSliceExpr(c, rhs) {
					stmt := c.expr2stmt(sel2call(c, "Set", lhsSel, rhs, decs), lhsSel)
					return stmt, true
				}
				// Duplicate LHS so that it can be used in both "then" and "else" blocks.
				// It's ok, regardless of the exact details of LHS, because:
				//  - before this change, LHS was evaluated exactly once, and
				//  - after this change, LHS is still evaluated exactly once
				lhsSelClone := cloneSelectorExpr(c, lhsSel)
				ifStmt, v := ifNonNil(c, lhsSel, rhs, decs)
				ifStmt.Body.List = []dst.Stmt{
					c.expr2stmt(sel2call(c, "Set", lhsSelClone, cloneExpr(c, v), dst.NodeDecs{}), lhsSelClone),
				}
				return ifStmt, true
			}
		}
	}

	//  m.Oneof = &pb.M_Oneof{K: V}  => m.SetK(V)
	//  m.Oneof = &pb.M_Oneof{V}     => m.SetK(V)
	//  m.Oneof = &pb.M_Oneof{}      => m.SetK(ZeroValueOf(BasicType))
	//
	// Where K is the field name of the sole field in M_Oneof.
	if isOneof(c.typeOf(lhsSel.Sel)) {
		c.Logf("rewriting oneof wrapper")
		name, typ, val, oneofDecs, ok := destructureOneofWrapper(c, rhs)
		if !ok {
			c.Logf("ignoring: destructuring oneof wrapper failed")
			if c.lvl.ge(Red) {
				c.numUnsafeRewritesByReason[OneofFieldAccess]++
				addCommentAbove(c.Node(), lhsSel.X, "// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).")
			}
			return nil, false
		}
		if val == nil {
			if !isScalar(typ) {
				c.Logf("...failed because lhs is not a scalar type")
				return nil, false
			}
			val = scalarTypeZeroExpr(c, typ)
		}
		lhsSel.Sel.Name = name
		c.Logf("...success")
		if oneofDecs != nil {
			decs.Start = append(decs.Start, (*oneofDecs).Start...)
			decs.End = append(decs.End, (*oneofDecs).End...)
		}
		return c.expr2stmt(sel2call(c, "Set", lhsSel, val, decs), lhsSel), true
	}

	// m1.F = m2.F
	//
	// [proto2]
	//   if m2.HasF() {
	//     m1.SetF(m2.GetF())
	//   } else {
	//     m1.ClearF()
	//   }
	// (or variations based on existence of side effects when evaluating m1 or m2)
	//
	// [proto3]
	//   m1.SetF(m2.GetF())
	isProto2 := isPtrToBasic(c.underlyingTypeOf(lhsSel)) // Bytes fields don't behave like other proto2 scalars.
	if rhsSel, ok := c.trackedProtoFieldSelector(rhs); ok {
		c.Logf("rewriting proto field to proto field assignment")
		if !isProto2 {
			return c.expr2stmt(sel2call(c, "Set", lhsSel, sel2call(c, "Get", rhsSel, nil, dst.NodeDecs{}), decs), lhsSel), true
		}

		// If RHS has no side effects then just evaluate it inline. However, if it
		// is not known to be side-effect free then evaluate it once, in the init
		// statement.
		var rhs1 *dst.SelectorExpr // We need two copies of rhs. They are identical.
		var initStmt dst.Stmt
		if c.isSideEffectFree(rhsSel) {
			rhs1 = rhsSel
		} else {
			v := &dst.Ident{Name: "x"}
			c.setType(v, c.typeOf(rhsSel.X))

			initStmt = &dst.AssignStmt{
				Lhs: []dst.Expr{v},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{rhsSel.X},
			}

			rhs1 = &dst.SelectorExpr{
				X:   &dst.Ident{Name: v.Name},
				Sel: rhsSel.Sel,
			}
			c.setType(rhs1, c.typeOf(rhsSel))
			c.setType(rhs1.X, c.typeOf(v))
		}
		rhs2 := cloneSelectorExpr(c, rhs1)

		lhs1, lhs2 := lhsSel, cloneSelectorExpr(c, lhsSel) // We need two copies of LHS. They are identical.
		var elseStmt dst.Stmt
		if clearNeeded(c, lhsSel) {
			c.Logf("Clear() statement is needed")
			elseStmt = c.expr2stmt(sel2call(c, "Clear", lhs1, nil, dst.NodeDecs{}), lhs1)
		}

		// Move end-of-line comments to above the if conditional.
		if len(decs.End) > 0 {
			decs.Start = append(decs.Start, decs.End...)
			decs.End = nil
		}
		var stmt dst.Stmt = c.expr2stmt(sel2call(c, "Set", lhs2, sel2call(c, "Get", rhs2, nil, dst.NodeDecs{}), dst.NodeDecs{}), lhs2)
		if hasNeeded(c, rhsSel) {
			stmt = &dst.IfStmt{
				Init: initStmt,
				Cond: sel2call(c, "Has", rhs1, nil, dst.NodeDecs{}),
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						stmt,
					},
				},
				Else: elseStmt,
				Decs: dst.IfStmtDecorations{NodeDecs: decs},
			}
		}
		return stmt, true
	}

	// m.F = V  =>  m.SetF(V)
	if !isProto2 {
		c.Logf("rewriting direct field access to setter")
		return c.expr2stmt(sel2call(c, "Set", lhsSel, rhs, decs), lhsSel), true
	}

	// m.F = &V  =>  m.SetF(V)
	if isAddr(rhs) {
		c.Logf("rewriting direct field access to setter (rhs &Expr)")
		return c.expr2stmt(sel2call(c, "Set", lhsSel, deref(c, rhs), decs), lhsSel), true
	}

	// m.F = V   =>  m.SetF(*V)      (red rewrite: loses aliasing)
	if c.lvl.ge(Red) {
		c.Logf("rewriting direct field access to setter (losing pointer aliasing)")
		lhs2 := cloneSelectorExpr(c, lhsSel)
		ifStmt, v := ifNonNil(c, lhsSel, rhs, decs)
		ifStmt.Body.List = []dst.Stmt{
			c.expr2stmt(sel2call(c, "Set", lhs2, deref(c, cloneExpr(c, v)), dst.NodeDecs{}), lhs2),
		}
		c.numUnsafeRewritesByReason[PointerAlias]++
		return ifStmt, true
	}

	c.Logf("no applicable rewrite found")
	return nil, false
}

// clearNeeded figures out if for the specified SelectorExpr, a Clear() call
// needs to be inserted before or not.
//
// clearNeeded looks at the AST of the current scope, considering all statements
// between the initial assignment and the SelectorExpr:
//
//	…                                   // (not checked yet)
//	mm2 := &mypb.Message{}              // initial assignment
//	mm2.SetBytes([]byte("hello world")) // checked by clearNeeded()
//	…                                   // checked by clearNeeded()
//	mm2.I32 = 23                        // SelectorExpr
//	proto.Merge(mm2, src)               // (not checked anymore)
//	…
func clearNeeded(c *cursor, sel *dst.SelectorExpr) bool {
	// sel is something like dst.SelectorExpr{
	//   X:   &dst.Ident{"mm2"},
	//   Sel: &dst.Ident{"I32"},
	// }
	if _, ok := sel.X.(*dst.Ident); !ok {
		return true
	}

	innerMost, _ := c.enclosingASTStmt(sel.X)
	if innerMost == nil {
		return true
	}
	enclosing, ok := c.typesInfo.dstMap[innerMost]
	if !ok {
		c.Logf("BUG: no corresponding dave/dst node for go/ast node %T / %+v", innerMost, innerMost)
		return true
	}
	first := compositLiteralInitialer(enclosing, sel.X.(*dst.Ident).Name)
	if first == nil {
		// The variable we are looking for is not initialized
		// unconditionally in this scope.
		return true
	}
	firstSeen := false
	lastSeen := false
	usageFound := false
	var visit visitorFunc
	xObj := c.objectOf(sel.X.(*dst.Ident))
	selObj := c.objectOf(sel.Sel)
	selName := fixConflictingNames(c.typeOf(sel.X), "Set", sel.Sel.Name)

	visit = func(n dst.Node) dst.Visitor {
		if n == first {
			firstSeen = true
			// Don't check the children of the definition because
			// it would look like a usage.
			return nil
		}
		if !firstSeen {
			// As long as we have not seen the first node we don't
			// need to look at any of the statements because they
			// cannot influence first.
			return visit
		}
		if lastSeen {
			return nil
		}

		if as, ok := n.(*dst.AssignStmt); ok {
			for _, lhs := range as.Lhs {
				if lhs == sel {
					lastSeen = true
					// Skip recursing into children; all subsequent visit() calls
					// will return immediately.
					return nil
				}

				// Is the field assigned to?
				if usesObject(c, lhs, xObj) && usesObject(c, lhs, selObj) {
					usageFound = true
					return nil
				}
			}
		}

		// Access is okay if it's not a setter for the field
		// and if it is not assigned to (checked above).
		if doesNotModifyField(c, n, selName) {
			return nil
		}

		if id, ok := n.(*dst.Ident); ok && id.Name == sel.X.(*dst.Ident).Name {
			c.Logf("found non-proto-field-selector usage of %q", id.Name)
			usageFound = true
		}

		return visit // recurse into children
	}
	dst.Walk(visit, enclosing)
	// Clear() calls are definitely needed when:
	//
	// 1. !firstSeen — we couldn’t find the declaration of <sel> in the
	//    innermost scope.
	//
	// 2. or !lastSeen — we couldn’t find the usage of <sel> (bug?)
	//
	// 3. or usageFound — we did find a usage that we didn’t expect.
	return !firstSeen || !lastSeen || usageFound
}

// isMultiProtoAssign returns true if stmt is a multi-assignment that assigns at least one protocol
// buffer field. Note that irregular assignments (e.g. "a,b := m[c]") are not considered to be
// multi-assignments.
func isMultiProtoAssign(c *cursor, stmt dst.Stmt) bool {
	a, ok := stmt.(*dst.AssignStmt)
	if !ok || len(a.Lhs) < 2 || len(a.Lhs) != len(a.Rhs) {
		return false
	}
	for _, lhs := range a.Lhs {
		if _, ok := c.trackedProtoFieldSelector(lhs); ok {
			return true
		}
	}
	return false
}

// isBytesConversion returns true if x is an explicit conversion to a slice of
// bytes. That is, when x has the form "[]byte(...)".
func isBytesConversion(c *cursor, x dst.Expr) bool {
	call, ok := x.(*dst.CallExpr)
	if !ok {
		return false
	}
	fun, ok := call.Fun.(*dst.ArrayType)
	if !ok {
		return false
	}
	ident, ok := fun.Elt.(*dst.Ident)
	if !ok {
		return false
	}
	return c.objectOf(ident) == types.Universe.Lookup("byte")
}

// newConstructorCall returns true if x is a 'new' call. It also returns an
// argument that should be provided to a corresponding proto setter if the new
// call was to be replaced by a set call.
func newConstructorCall(c *cursor, expr dst.Expr) (dst.Expr, bool) {
	call, ok := expr.(*dst.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	ident, ok := call.Fun.(*dst.Ident)
	if !ok {
		return nil, false
	}
	if c.objectOf(ident) != types.Universe.Lookup("new") {
		return nil, false
	}

	t := c.typeOf(call.Args[0])
	if t, ok := types.Unalias(t).(*types.Basic); ok {
		return scalarTypeZeroExpr(c, t), true
	}

	if _, ok := types.Unalias(t).(*types.Named); !ok {
		return nil, false
	}

	// Message
	if _, ok := t.Underlying().(*types.Struct); ok {
		return call, true // new(M) is fine
	}

	// Enum
	if !isBasic(t.Underlying()) {
		return nil, false
	}
	zero := &dst.Ident{Name: "0"}
	c.setType(zero, types.Typ[types.UntypedInt])
	conv := &dst.CallExpr{
		Fun:  call.Args[0],
		Args: []dst.Expr{zero},
	}
	c.setType(conv, t)
	return conv, true
}

// enumHelperCall returns true if expr is a enum helper call (e.g.
// "pb.MyMessage_MyEnumVal.Enum()"). If so, it also returns the enum value
// ("MyEnumVal" in the previous example).
func enumHelperCall(c *cursor, expr dst.Expr) (dst.Expr, bool) {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return nil, false
	}
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil, false
	}
	if sel.Sel.Name != "Enum" {
		return nil, false
	}
	var res dst.Expr = sel.X
	// It is possible to use methods enums as if they were free functions by
	// passing the receiver as first argument. We know that the Enum method
	// does not have any parameters. This means if it is called with an argument
	// it must be used as free function and the argument is the receiver, e.g.:
	// (e.g. "pb.MyMessage_MyEnum.Enum(pb.MyMessage_MyEnumVal)").
	//
	// Note: we cannot use the type system to determine whether the free function
	// or method is used because these are two are the same from the type
	// systems's point of view.
	if len(call.Args) == 1 {
		res = call.Args[0]
	}
	return res, true
}

// isSelectorExprWithIdent return true if e is a simple selector expression where
// X is of type *dst.Ident.
func isSelectorExprWithIdent(e dst.Expr) bool {
	rhsSel, ok := e.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	if _, ok := rhsSel.X.(*dst.Ident); !ok {
		return false
	}
	return true
}

// ifNonNil returns a *dst.IfStmt that checks if rhs is not nil (if rhs is nil,
// Clear() is called). If needed, a temporary variable (x) is introduced to only
// evaluate rhs once. The returned *dst.Ident refers either to the temporary
// variable (if needed) or to rhs. If possible, the Clear() call will be elided.
func ifNonNil(c *cursor, lhsSel *dst.SelectorExpr, rhs dst.Expr, decs dst.NodeDecs) (*dst.IfStmt, dst.Expr) {
	var elseStmt dst.Stmt
	if clearNeeded(c, lhsSel) {
		c.Logf("Clear() statement is needed")
		elseStmt = c.expr2stmt(sel2call(c, "Clear", lhsSel, nil, dst.NodeDecs{}), lhsSel)
	}

	var v dst.Expr
	var initAssign dst.Stmt
	if rhsIdent, ok := rhs.(*dst.Ident); ok {
		// If RHS is already an identifier, we skip generating an
		// initializer in the if statement (x := rhs) and just use
		// the RHS directly.
		v = rhsIdent
	} else if isSelectorExprWithIdent(rhs) {
		v = rhs
	} else {
		v = &dst.Ident{Name: "x"}
		c.setType(v, c.typeOf(rhs))
		initAssign = &dst.AssignStmt{
			Lhs: []dst.Expr{cloneIdent(c, v.(*dst.Ident))},
			Tok: token.DEFINE,
			Rhs: []dst.Expr{rhs},
		}
	}

	untypedNil := &dst.Ident{Name: "nil"}
	c.setType(untypedNil, types.Typ[types.UntypedNil])
	cond := &dst.BinaryExpr{
		X:  v,
		Op: token.NEQ,
		Y:  untypedNil,
	}
	c.setType(cond, types.Typ[types.Bool])

	// Move end-of-line comments to above the if conditional.
	if len(decs.End) > 0 {
		decs.Start = append(decs.Start, decs.End...)
		decs.End = nil
	}

	return &dst.IfStmt{
		Init: initAssign,
		Cond: cond,
		Body: &dst.BlockStmt{},
		Else: elseStmt,
		Decs: dst.IfStmtDecorations{NodeDecs: decs},
	}, v
}

// doesNotModifyField returns true if n does not modify a proto field named name.
func doesNotModifyField(c *cursor, n dst.Node, name string) bool {
	if selFun, sig, ok := c.protoFieldSelectorOrAccessor(n); ok {
		if sig == nil || selFun.Sel.Name != "Set"+name {
			// Skip recursing into children: proto field selector usages are
			// okay; we could still skip the Clear() methods.
			return true
		}
	}
	return false
}

// compositLiteralInitialer return the node that is a direct child of enclosing
// and initialized a variabled named `name` unconditionally with a composite
// literal. Returns nil if there is no such node.
func compositLiteralInitialer(enclosing dst.Node, name string) dst.Node {
	for _, n := range directChildren(enclosing) {
		as, ok := n.(*dst.AssignStmt)
		if !ok {
			continue
		}
		if len(as.Lhs) != len(as.Rhs) {
			continue
		}
		for i, lhs := range as.Lhs {
			if id, ok := lhs.(*dst.Ident); ok && id.Name == name {
				// We consider everything but composite literal
				// initialization as unknown and thus unsafe.
				if _, ok := isCompositeLit(as.Rhs[i], as); ok {
					return n
				}
			}
		}
	}
	return nil
}

func usesObject(c *cursor, expr dst.Expr, obj types.Object) bool {
	found := false
	var visit visitorFunc
	visit = func(n dst.Node) dst.Visitor {
		id, ok := n.(*dst.Ident)
		if !ok {
			return visit
		}

		if c.objectOf(id) == obj {
			found = true
			return nil
		}
		return visit
	}
	dst.Walk(visit, expr)
	return found
}

// directChildren returns a list of dst.Stmts if the n is a node opens a scope.
func directChildren(n dst.Node) []dst.Stmt {
	switch t := n.(type) {
	case *dst.BlockStmt:
		return t.List
	case *dst.CaseClause:
		return t.Body
	case *dst.CommClause:
		return t.Body
	}
	return nil
}

func deref(c *cursor, expr dst.Expr) dst.Expr {
	ue, ok := expr.(*dst.UnaryExpr)
	if ok {
		return ue.X
	}
	out := &dst.StarExpr{X: expr}
	c.setType(out, types.Unalias(c.underlyingTypeOf(expr)).(*types.Pointer).Elem())
	return out
}

func stringIn(s string, ss []string) bool {
	for _, v := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// protoHelperCall returns true if expr is a proto helper call (e.g. "proto.String(s)"). If so, it
// also returns argument to the helper.
func protoHelperCall(c *cursor, expr dst.Expr) (dst.Expr, bool) {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return nil, false
	}
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil, false
	}
	if sel.Sel.Name == "Int" && !isBasicLit(call.Args[0]) {
		// "m.F = proto.Int(v)" => "m.SetF(int32(v))"
		x := &dst.CallExpr{
			Fun:  dst.NewIdent("int32"),
			Args: []dst.Expr{call.Args[0]},
		}
		c.setType(x, types.Universe.Lookup("int32").Type())
		c.setType(x.Fun, types.Universe.Lookup("int32").Type())
		return x, true
	}
	if !stringIn(sel.Sel.Name, []string{"Bool", "Float32", "Float64", "Int", "Int32", "Int64", "String", "Uint32", "Uint64"}) {
		return nil, false
	}
	if c.objectOf(sel.Sel).Pkg().Path() != protoImport {
		return nil, false
	}
	return call.Args[0], true
}

func isBasicLit(x dst.Expr) bool {
	_, ok := x.(*dst.BasicLit)
	return ok
}
