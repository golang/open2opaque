// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/token"

	"github.com/dave/dst"
)

func appendProtosPre(c *cursor) bool {
	// Handle the following as a single unit:
	//
	//   m.R = append(m.R, msg, ...)
	//      =>
	//   m.SetR(append(m.GetR(), msg, ...))
	//
	//
	// Note that this rewrite can be handled as two independent rewrites:
	//
	//  Step 1
	//    m.R = append(m.R, msg, ...)
	//      =>
	//    m.R = append(m.GetR(), msg, ...)
	//
	//  Step2
	//    m.R = append(m.GetR(), msg, ...)
	//      =>
	//    m.SetR(append(m.GetR(), msg, ...))
	//
	// So this function may seem redundant at first. However, in practice,
	// those multi-step rewrites are not very reliable in presence of
	// failures and it's not uncommon to end up with partially updated
	// pattern or a misplaced comment. Hence, we handle the pattern as a
	// whole.
	a, ok := c.Node().(*dst.AssignStmt)
	if !ok {
		c.Logf("ignoring %T (looking for AssignStmt)", c.Node())
		return true
	}
	if a.Tok != token.ASSIGN || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		c.Logf("ignoring AssignStmt with Tok %v (looking for ASSIGN)", a.Tok)
		return true
	}
	if len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		c.Logf("ignoring AssignStmt with len(lhs)=%d, len(rhs)=%d (looking for 1, 1)", len(a.Lhs), len(a.Rhs))
		return true
	}

	appendCall, ok := a.Rhs[0].(*dst.CallExpr)
	if !ok {
		c.Logf("ignoring AssignStmt with rhs %T (looking for CallExpr)", a.Rhs[0])
		return true
	}
	if len(appendCall.Args) == 0 {
		c.Logf("ignoring rhs CallExpr with %d args (looking for 1)", len(appendCall.Args))
		return true
	}
	ident, ok := appendCall.Fun.(*dst.Ident)
	if !ok {
		c.Logf("ignoring rhs CallExpr with Fun %T (looking for Ident)", appendCall.Fun)
		return true
	}
	if obj := c.objectOf(ident); obj.Name() != "append" || obj.Pkg() != nil {
		c.Logf("ignoring rhs CallExpr with Obj %v (looking for append())", obj)
		return true
	}

	lsel, ok := c.trackedProtoFieldSelector(a.Lhs[0])
	if !ok {
		c.Logf("ignoring: lhs is not a proto field selector")
		return true
	}
	rsel, ok := c.trackedProtoFieldSelector(appendCall.Args[0])
	if !ok {
		c.Logf("ignoring: rhs append() arg is not a proto field selector")
		return true
	}

	appendCall.Args[0] = sel2call(c, "Get", rsel, nil, *rsel.Decorations())
	c.Replace(c.expr2stmt(sel2call(c, "Set", lsel, appendCall, *c.Node().Decorations()), lsel))
	return true
}
