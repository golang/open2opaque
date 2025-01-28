// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/dave/dst"
)

// isGetterFor return true if expr is a getter for the proto field sel
func isGetterFor(c *cursor, expr dst.Expr, sel *dst.SelectorExpr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}
	dstSel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := dstSel.X.(*dst.Ident); !ok || c.objectOf(id) != c.objectOf(sel.X.(*dst.Ident)) {
		return false
	}
	if dstSel.Sel.Name != "Get"+sel.Sel.Name {
		return false
	}
	return true
}

// isDirectFieldAccess return true if expr is a dereferencing direct field
// access for the proto field sel
func isDirectFieldAccess(c *cursor, expr dst.Expr, sel *dst.SelectorExpr) bool {
	se, ok := expr.(*dst.StarExpr)
	if !ok {
		return false
	}
	otherSel, ok := se.X.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	otherID, ok := otherSel.X.(*dst.Ident)
	if !ok {
		return false
	}
	if c.objectOf(otherID) != c.objectOf(sel.X.(*dst.Ident)) {
		return false
	}
	if c.objectOf(otherSel.Sel) != c.objectOf(sel.Sel) {
		return false
	}
	return true
}

// isFieldAccessFor returns true if expr is a field access for sel (either
// getter of direct).
func isFieldAccessFor(c *cursor, expr dst.Expr, sel *dst.SelectorExpr) bool {
	return isGetterFor(c, expr, sel) || isDirectFieldAccess(c, expr, sel)
}

// guaranteesExistenceOf return true if the expr is a boolean expression that
// guarantees that sel is set, e.g.:
//
//	if m.GetX() != "" {
func guaranteesExistenceOf(c *cursor, expr dst.Expr, sel *dst.SelectorExpr) bool {
	_, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}
	fieldType := c.typeOf(sel)
	if pt, ok := types.Unalias(fieldType).(*types.Pointer); ok {
		fieldType = pt.Elem()
	}
	// If expr a binary condition we check if it is a comparison against nil
	// or if it's conjunction and either of the branches is a haser.
	if bin, ok := expr.(*dst.BinaryExpr); ok {
		if bin.Op == token.LAND {
			return guaranteesExistenceOf(c, bin.X, sel) || guaranteesExistenceOf(c, bin.Y, sel)
		}
		if bin.Op == token.NEQ {
			// Is this `if m.GetX() != 0 {` ?
			// (0 is representative for the type specific zero value)
			foundZero := false
			expr := bin.X
			if isScalarTypeZeroExpr(c, fieldType, expr) {
				foundZero = true
				expr = bin.Y
			}
			if !foundZero && !isScalarTypeZeroExpr(c, fieldType, bin.Y) {
				return false
			}
			return isFieldAccessFor(c, expr, sel)

		}
		return false

	}
	return false
}

// isHaserFor return true if the expr is a boolean expression that
// implements a has-check for the message field specified by sel.
func isHaserFor(c *cursor, expr dst.Expr, sel *dst.SelectorExpr) bool {
	xID, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}
	xObj := c.objectOf(xID)
	selObj := c.objectOf(sel.Sel)

	// If expr a binary condition we check if it is a comparison against nil
	// or if it's conjunction and either of the branches is a haser.
	if bin, ok := expr.(*dst.BinaryExpr); ok {
		if bin.Op == token.LAND {
			return isHaserFor(c, bin.X, sel) || isHaserFor(c, bin.Y, sel)
		}
		if bin.Op == token.NEQ {
			// Is this `if m.X != nil {` ?
			// Or  `if nil != m.X {` ?
			foundNil := false
			expr := bin.X
			if c.typeOf(expr) == types.Typ[types.UntypedNil] {
				foundNil = true
				expr = bin.Y
			}
			if !foundNil && c.typeOf(bin.Y) != types.Typ[types.UntypedNil] {
				return false
			}
			bSel, ok := expr.(*dst.SelectorExpr)
			if !ok {
				return false
			}
			bID, ok := bSel.X.(*dst.Ident)
			if !ok {
				return false
			}
			if c.objectOf(bID) != xObj || c.objectOf(bSel.Sel) != selObj {
				return false
			}
			return true
		}
		return false

	}

	// Is this `if m.HasX() {` ?
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}
	dstSel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := dstSel.X.(*dst.Ident); !ok || c.typesInfo.objectOf(id) != xObj {
		return false
	}
	if dstSel.Sel.Name != "Has"+sel.Sel.Name {
		return false
	}

	return true
}

// isScalarTypeZeroExpr returns true if e is a zero value for t
func isScalarTypeZeroExpr(c *cursor, t types.Type, e dst.Expr) bool {
	if _, ok := types.Unalias(t).(*types.Basic); !isBytes(t) && !isEnum(t) && !ok {
		return false
	}
	zeroExpr := scalarTypeZeroExpr(c, t)
	if id0, ok := zeroExpr.(*dst.Ident); ok {
		if id1, ok := e.(*dst.Ident); ok {
			return id0.Name == id1.Name
		}
	}
	if bl0, ok := zeroExpr.(*dst.BasicLit); ok {
		if bl1, ok := e.(*dst.BasicLit); ok {
			// floats can be compared to either 0 or 0.0
			// scalarTypeZeroExpr generates the more specific 0.0
			// but we would like to allow comparison against 0 as
			// well.
			if bl0.Kind == token.FLOAT && bl1.Value == "0" {
				return true
			}
			return bl0.Value == bl1.Value && bl0.Kind == bl1.Kind
		}
	}
	return false
}

// hasNeeded implements a basic dataflow analysis to find out if the scope
// enclosing sel guarantees that sel is set (non-nil). We consider this
// guaranteed if there is either a `sel != nil` or a `${sel.X}.Has${sel.Sel}()`
// condition and the field is not modified afterwards.
// In the absence of bugs, this check never produces false-positives but it may
// produce false-negatives.
func hasNeeded(c *cursor, sel *dst.SelectorExpr) bool {
	selX, ok := sel.X.(*dst.Ident)
	if !ok {
		return true
	}

	innerMost, opener := c.enclosingASTStmt(sel)
	if _, ok := opener.(*ast.IfStmt); !ok {
		return true
	}
	dstIf, ok := c.typesInfo.dstMap[opener]
	if !ok {
		c.Logf("BUG: no corresponding dave/dst node for go/ast node %T / %+v (was c.typesInfo.dstMap not updated across rewrites?)", opener, opener)
		return true
	}

	cond := dstIf.(*dst.IfStmt).Cond
	if !isHaserFor(c, cond, sel) && !guaranteesExistenceOf(c, cond, sel) {
		return true
	}

	enclosing, ok := c.typesInfo.dstMap[innerMost]
	if !ok {
		c.Logf("BUG: no corresponding dave/dst node for go/ast node %T / %+v", innerMost, innerMost)
		return true
	}
	lastSeen := false
	usageFound := false

	xObj := c.objectOf(selX)
	selObj := c.objectOf(sel.Sel)
	var visit visitorFunc
	visit = func(n dst.Node) dst.Visitor {
		if lastSeen {
			return nil
		}

		if se, ok := n.(*dst.SelectorExpr); ok && se == sel {
			lastSeen = true
			// Skip recursing into children; all subsequent visit() calls
			// will return immediately.
			return nil
		}

		if as, ok := n.(*dst.AssignStmt); ok {
			// Is the field that was checked assigned to?
			for _, lhs := range as.Lhs {
				if usesObject(c, lhs, xObj) && usesObject(c, lhs, selObj) {
					usageFound = true
					return nil
				}
			}
		}

		// Access is okay if it's not a setter for the field
		// and if it is not assigned to (checked above).
		if doesNotModifyField(c, n, sel.Sel.Name) {
			return nil
		}

		if id, ok := n.(*dst.Ident); ok && c.objectOf(id) == xObj {
			c.Logf("found non-proto-field-selector usage of %q", id.Name)
			usageFound = true
			return nil
		}

		return visit // recurse into children
	}
	dst.Walk(visit, enclosing)

	return !lastSeen || usageFound
}
