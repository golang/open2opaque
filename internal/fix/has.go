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

// hasPre rewrites comparisons with nil in the following cases:
//   - for proto2 optional scalar fields, replace with "!m.HasF()" or "m.HasF()"
//   - for proto3 bytes fields, replace with "len(m.GetF()) == 0" or "len(m.GetF()) > 0"
//   - for simple conditionals (e.g. "if f := m.F; f != nil {") replace with "if m.HasF()"
//
// The function does not rewrite proto3 message fields, map fields, or repeated
// fields. Those are handled by changing the direct field access to a Get call.
//
// This function is executed by traversing the tree in preorder.
func hasPre(c *cursor) bool {
	// Handle a special case that shows up frequently
	//
	//   if f := m.F; f != nil {   =>   if m.HasF() {
	//
	// This works for singular, scalar fields and avoids red, incorrect rewrites
	// like the following:
	//
	//   if f := proto.Helper(m.GetF()); f != nil {
	if ifstmt, ok := c.Node().(*dst.IfStmt); ok {
		if ifstmt.Init == nil {
			return true
		}
		if ifstmt.Else != nil {
			// For now, focus on the most common case. Perhaps we could add handling
			// of "else" blocks one day.
			return true
		}
		lhs, op, ok := comparisonWithNil(ifstmt.Cond)
		if !ok {
			return true
		}
		condIdent, ok := lhs.(*dst.Ident)
		if !ok {
			return true
		}
		// The init statement must define a new name that's used in the comparison
		// with nil but is not used as a pointer otherwise. We only handle the
		// common case of "f := m.F" for now.
		def, ok := ifstmt.Init.(*dst.AssignStmt)
		if !ok || def.Tok != token.DEFINE || len(def.Lhs) != 1 || len(def.Rhs) != 1 {
			return true
		}
		rhsSel, ok := def.Rhs[0].(*dst.SelectorExpr)
		if !ok {
			return true
		}
		condObj := c.objectOf(condIdent)
		if defIdent, ok := def.Lhs[0].(*dst.Ident); !ok || c.objectOf(defIdent) != condObj {
			return true
		}
		if usesAsPointer(c, ifstmt.Body, condObj) {
			return true
		}

		if hasCall, ok := hasCallForProtoField(c, rhsSel, op, dst.NodeDecs{}); ok {
			ifstmt.Init = nil
			ifstmt.Cond = hasCall
			c.Replace(ifstmt)
			dstutil.Apply(ifstmt.Body, nil, func(cur *dstutil.Cursor) bool {
				star, ok := cur.Node().(*dst.StarExpr)
				if !ok {
					return true
				}
				ident, ok := star.X.(*dst.Ident)
				if !ok {
					return true
				}
				if c.objectOf(ident) == condObj {
					// Is the pointee assigned to?

					if as, ok := cur.Parent().(*dst.AssignStmt); ok {
						var found bool
						for _, l := range as.Lhs {
							if l == cur.Node() {
								// It is easier to replace the pointer
								// dereference with a direct field access here
								// and to rely on a later pass to rewrite it to
								// a setter. The alternative is to replace it
								// with a setter directly.
								clone := cloneSelectorExpr(c, rhsSel)
								star.X = clone
								found = true
							}
						}
						if found {
							return true
						}
					}

					// The pointee is used as value. It is safe to use the Getter.
					cur.Replace(sel2call(c, "Get", cloneSelectorExpr(c, rhsSel), nil, *rhsSel.Decorations()))
				}
				return true
			})
		}
		return true
	}

	// Handle conditionals that use a selector on the left-hand side:
	//
	//   m.F != nil   =>   m.HasF()
	//   m.F == nil   =>   !m.HasF()
	if _, _, ok := comparisonWithNil(c.Node()); !ok {
		return true
	}
	expr := c.Node().(*dst.BinaryExpr)
	if call, ok := hasCallForProtoField(c, expr.X, expr.Op, *expr.Decorations()); ok {
		if sel := expr.X.(*dst.SelectorExpr); !ok || isPtr(c.typeOf(sel.X)) || c.canAddr(sel.X) {
			c.Replace(call)
		} else if c.lvl.ge(Red) {
			c.ReplaceUnsafe(call, InexpressibleAPIUsage)
		}
		return false
	}
	field, ok := c.trackedProtoFieldSelector(expr.X)
	if !ok {
		return true
	}
	if s, ok := types.Unalias(c.typeOf(field)).(*types.Slice); ok {
		// use "len" for proto3 bytes fields.
		if bt, ok := types.Unalias(s.Elem()).(*types.Basic); ok && bt.Kind() == types.Byte {
			// m.F == nil   => len(m.GetF()) == 0
			// m.F != nil   => len(m.GetF()) != 0
			var getVal dst.Expr = field
			if isPtr(c.typeOf(field.X)) || c.canAddr(field.X) {
				getVal = sel2call(c, "Get", field, nil, dst.NodeDecs{})
			}
			lenCall := &dst.CallExpr{
				Fun:  dst.NewIdent("len"),
				Args: []dst.Expr{getVal},
			}
			c.setType(lenCall, types.Typ[types.Int])
			c.setType(lenCall.Fun, types.Universe.Lookup("len").Type())
			op := token.EQL
			if expr.Op == token.NEQ {
				op = token.NEQ
			}
			zero := &dst.BasicLit{Kind: token.INT, Value: "0"}
			c.setType(zero, types.Typ[types.Int])
			bop := &dst.BinaryExpr{
				X:    lenCall,
				Op:   op,
				Y:    zero,
				Decs: expr.Decs,
			}
			c.setType(bop, types.Typ[types.Bool])
			c.Replace(bop)
			return true
		}
	}

	// We don't handle repeated fields and maps explicitly here. We handle those
	// cases by rewriting the code to use Get calls:
	//
	//   m.F == nil   => m.GetF() == nil
	//   m.F != nil   => m.GetF() != nil
	//
	// We depend on the above and on the implementation detail that after:
	//
	//   m.SetF(nil)
	//
	// we guarantee:
	//
	//   m.GetF() == nil
	//
	// This works and preserves the old API behavior. However, it's
	// a discouraged pattern in new code. It's better to check the
	// length instead.
	//
	// We DO NOT do that as it couldn't be a green rewrite due to
	// the difference between nil and zero-length slices.

	return true
}

// usesAsPointer returns whether the pointer target is used without being
// dereferenced.
func usesAsPointer(c *cursor, b *dst.BlockStmt, target types.Object) bool {
	var out bool
	dstutil.Apply(b, nil, func(cur *dstutil.Cursor) bool {
		// Is current node a usage of target without dereferencing it?
		if ident, ok := cur.Node().(*dst.Ident); ok && c.objectOf(ident) == target && !isStarExpr(cur.Parent()) {
			out = true
			return false // terminate traversal immediately
		}
		return true
	})
	return out
}

// hasCallForProtoField returns a "has" call for the given proto field selector, x.
//
// For example, for "m.F", it returns "m.HasF()". The op determines the context
// in which "m.F" is used. Only "==" and "!=" have an effect here, with the
// expectation that "x" is used as "m.F OP nil"
func hasCallForProtoField(c *cursor, x dst.Expr, op token.Token, decs dst.NodeDecs) (hasCall dst.Expr, ok bool) {
	field, ok := c.trackedProtoFieldSelector(x)
	if !ok {
		return nil, false
	}
	if !c.useClearOrHas(field) {
		return nil, false
	}
	call := sel2call(c, "Has", field, nil, decs)
	if op == token.EQL {
		// m.F == nil   =>  !m.HasF()
		return not(c, call), true
	} else if op == token.NEQ {
		// m.F != nil   =>   m.HasF()
		return call, true
	}
	return nil, false
}

// comparisonWithNil checks that n is a comparison with nil. If so, it returns
// the left-hand side and the comparison operator. Otherwise, it returns false.
func comparisonWithNil(n dst.Node) (lhs dst.Expr, op token.Token, ok bool) {
	x, ok := n.(*dst.BinaryExpr)
	if !ok {
		return nil, 0, false
	}
	if x.Op != token.EQL && x.Op != token.NEQ {
		return nil, 0, false
	}
	if ident, ok := x.Y.(*dst.Ident); !ok || ident.Name != "nil" {
		return nil, 0, false
	}
	return x.X, x.Op, true
}

func isStarExpr(x dst.Node) bool {
	_, ok := x.(*dst.StarExpr)
	return ok
}

func not(c *cursor, expr dst.Expr) dst.Expr {
	out := &dst.UnaryExpr{
		Op: token.NOT,
		X:  expr,
	}
	c.setType(out, c.underlyingTypeOf(expr))
	return out
}
