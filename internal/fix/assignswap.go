// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/dave/dst"
)

// invariant: selFromExpr() must only be called on expressions for which
// c.protoFieldSelectorOrAccessor() returns ok. assignmentIsSwap() ensures that
// for all lhs/rhs expressions.
func selFromExpr(c *cursor, expr dst.Expr) *dst.SelectorExpr {
	if ce, ok := expr.(*dst.CallExpr); ok {
		sel, _, _ := c.protoFieldSelectorOrAccessor(ce.Fun)
		return sel
	}
	return expr.(*dst.SelectorExpr)
}

func idFromExpr(expr dst.Expr) *dst.Ident {
	if expr == nil {
		return nil
	}
	switch x := expr.(type) {
	case *dst.Ident:
		return x
	case *dst.CallExpr:
		return idFromExpr(x.Fun)
	case *dst.SelectorExpr:
		return idFromExpr(x.Sel)
	default:
		return nil
	}
}

func assignmentIsSwap(c *cursor, stmt *dst.AssignStmt) error {
	if stmt.Tok != token.ASSIGN {
		return fmt.Errorf("ignoring AssignStmt with Tok %v (looking for ASSIGN)", stmt.Tok)
	}

	if len(stmt.Lhs) != len(stmt.Rhs) {
		return fmt.Errorf("ignoring AssignStmt with len(lhs)=%d != len(rhs)=%d (cannot be a swap)", len(stmt.Lhs), len(stmt.Rhs))
	}

	if len(stmt.Lhs) == 1 {
		return fmt.Errorf("ignoring AssignStmt with 1 lhs/rhs (assignment, not swap)")
	}

	for _, expr := range append(append([]dst.Expr(nil), stmt.Lhs...), stmt.Rhs...) {
		if ce, ok := expr.(*dst.CallExpr); ok {
			expr = ce.Fun
		}
		se, ok := expr.(*dst.SelectorExpr)
		if !ok {
			return fmt.Errorf("ignoring AssignStmt: %T is not a SelectorExpr (cannot be a swap)", expr)
		}
		if idFromExpr(se.X) == nil {
			return fmt.Errorf("ignoring AssignStmt: could not find Ident in SelectorExpr.X %T", expr)
		}
		sel, _, ok := c.protoFieldSelectorOrAccessor(expr)
		if !ok {
			return fmt.Errorf("ignoring AssignStmt: %T is not a proto field selector", expr)
		}
		t := c.typeOfOrNil(sel.X)
		if t == nil {
			continue // no type info (silo'ed?), assume tracked
		}
		if !c.shouldUpdateType(t) {
			return fmt.Errorf("should not update type %v", t)
		}
	}

	c.Logf("rewriting swap")

	// As soon as we find any repetition among the left and right hand side, we
	// treat the assignment as a swap.
	for _, lhs := range stmt.Lhs {
		lhsSel := selFromExpr(c, lhs)
		lhsXObj := c.objectOf(idFromExpr(lhsSel.X))
		lhsSelName := c.objectOf(lhsSel.Sel).Name()
		for _, rhs := range stmt.Rhs {
			rhsSel := selFromExpr(c, rhs)
			rhsXObj := c.objectOf(idFromExpr(rhsSel.X))
			rhsSelName := c.objectOf(rhsSel.Sel).Name()
			// If the RHS is a function call (LHS of an assignment cannot be a
			// function call), it must be a getter (the other accessors do not
			// return a value), so remove the Get prefix to match the LHS name.
			if _, ok := rhs.(*dst.CallExpr); ok {
				rhsSelName = strings.TrimPrefix(rhsSelName, "Get")
			}

			if lhsXObj == rhsXObj && lhsSelName == rhsSelName {
				return nil
			}
		}
	}
	return fmt.Errorf("ignoring AssignStmt: no repetitions among lhs/rhs")
}

// assignSwapPre splits swaps (m.F1, m.F2 = m.F2, m.F1) into two separate assign
// statements which can then be rewritten by subsequent rewrite stages.
func assignSwapPre(c *cursor) bool {
	// NOTE(stapelberg): This stage is only yellow level because the subsequent
	// getPost and assignPre stages only deal with pointer-typed variables in
	// the yellow level. But, safety-wise, this could go into the green level.
	if !c.lvl.ge(Yellow) {
		return true
	}

	stmt, ok := c.Node().(*dst.AssignStmt)
	if !ok {
		c.Logf("ignoring %T (looking for AssignStmt)", c.Node())
		return true
	}
	if err := assignmentIsSwap(c, stmt); err != nil {
		c.Logf("%s", err.Error())
		return true
	}

	assign2 := &dst.AssignStmt{
		Lhs: stmt.Lhs,
		Tok: token.ASSIGN,
		Rhs: nil, // will be filled with helper variable names
	}
	stmt.Lhs = nil // will be filled with helper variable names
	for _, rhs := range stmt.Rhs {
		rhsSel := selFromExpr(c, rhs)
		helperName := c.helperNameFor(rhs, c.typeOf(rhsSel.X))
		helperIdent := &dst.Ident{Name: helperName}
		updateASTMap(c, rhs, helperIdent)
		c.setType(helperIdent, c.typeOf(rhs))
		stmt.Lhs = append(stmt.Lhs, helperIdent)
		assign2.Rhs = append(assign2.Rhs, cloneIdent(c, helperIdent))
	}
	stmt.Tok = token.DEFINE
	assign2.Decorations().After = stmt.Decorations().After
	assign2.Decorations().End = stmt.Decorations().End
	stmt.Decorations().After = dst.None
	stmt.Decorations().End = nil
	c.InsertAfter(assign2)

	return true
}
