// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/token"
	"go/types"

	"github.com/dave/dst"
)

func incDecPre(c *cursor) bool {
	stmt, ok := c.Node().(*dst.IncDecStmt)
	if !ok {
		return true
	}
	x := stmt.X
	if pe, ok := x.(*dst.ParenExpr); ok {
		x = pe.X
	}
	if se, ok := x.(*dst.StarExpr); ok {
		x = se.X
	}
	field, ok := c.trackedProtoFieldSelector(x)
	if !ok {
		return true
	}
	if !c.isSideEffectFree(field) {
		markMissingRewrite(stmt, "inc/dec statement")
		return true
	}
	val := &dst.BinaryExpr{
		X: sel2call(c, "Get", cloneSelectorExpr(c, field), nil, *field.Decorations()),
		Y: dst.NewIdent("1"),
	}
	c.setType(val.Y, types.Typ[types.UntypedInt])
	c.setType(val, c.typeOf(field))
	if stmt.Tok == token.INC {
		val.Op = token.ADD
	} else {
		val.Op = token.SUB
	}
	// Not handled: decorations from the inner nodes (ParenExpr,
	// StarExpr). While those are unlikely to be there, we would ideally not
	// lose those. Perhaps we need a more general solution instead of handling
	// decorations on a case-by-case basis.
	c.Replace(c.expr2stmt(sel2call(c, "Set", field, val, *stmt.Decorations()), field))
	return true
}
