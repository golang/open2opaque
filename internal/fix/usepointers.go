// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"google.golang.org/open2opaque/internal/protodetecttypes"
)

// usePointersPre replaces usage of Go proto structs as values with pointer to structs (i.e. the
// only correct way to handle protos).
//
// NOTE: usePointersPre is called (once) for the *dst.File. Recursion aborts
// because it returns false; the dstutil.Apply() calls within the function
// traverse the entire file.
func usePointersPre(c *cursor) bool {
	// The rewrite below is only for the Red level.
	if c.lvl.le(Yellow) {
		return false
	}

	// x := pb.M{}       =>   x := &pb.M{}
	// x := pb.M{F: V}   =>   x := &pb.M{F: V}
	// This is ok if x is never copied (shallow copy).
	// It could be ok in other cases too but we implement the common case for now.
	// We only deal with the short-form definition for now (the long form hasn't showed up in practice).
	dstutil.Apply(c.Node(), func(cur *dstutil.Cursor) bool {
		switch n := cur.Node().(type) {
		case *dst.BlockStmt:
			for j, stmt := range n.List {
				ident, ok := nonPtrProtoStructDef(c, stmt)
				if !ok {
					continue
				}
				if replaceWithPtr(c, ident, n.List[j+1:]) {
					c.setType(ident, types.NewPointer(c.typeOf(ident)))
					a := stmt.(*dst.AssignStmt)
					a.Rhs[0] = addr(c, a.Rhs[0])
					c.numUnsafeRewritesByReason[PotentialBuildBreakage]++
				}
			}

		case *dst.Field:
			T := c.typeOf(n.Type)
			_, alreadyStar := n.Type.(*dst.StarExpr)
			if (protodetecttypes.Type{T: T}).IsMessage() && !alreadyStar {
				sexpr := &dst.StarExpr{X: n.Type}
				c.setType(sexpr, types.NewPointer(T))
				n.Type = sexpr
				c.numUnsafeRewritesByReason[PotentialBuildBreakage]++
			}

		}
		return true
	}, nil)

	// Rewrite all non-pointer composite literals, even though it breaks
	// compilation. We annotate these rewrites with a FIXME comment. It is up to
	// the user to change the usage of this literal to cope with it now being a
	// pointer.
	dstutil.Apply(c.Node(), func(cur *dstutil.Cursor) bool {
		lit, ok := cur.Node().(*dst.CompositeLit)
		if !ok {
			return true
		}
		if isAddr(cur.Parent()) {
			// The code already takes the address of this composite literal
			// (e.g. &pb.M2{literal}), resulting in a pointer. Skip.
			return true
		}

		typ := c.typeOf(lit)
		if _, ok := typ.Underlying().(*types.Pointer); ok {
			// The composite literal implicitly is already a pointer type
			// (e.g. []*pb.M2{{literal}}). Skip.
			return true
		}

		if !c.shouldUpdateType(typ) {
			return true
		}

		addCommentAbove(c.Node(), lit, "// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)")

		c.numUnsafeRewritesByReason[IncompleteRewrite]++
		cur.Replace(addr(c, lit))

		return true
	}, nil)
	return false
}

// nonPtrProtoStructDef returns true if stmt is of the form:
//
//	x := pb.M{...}
//
// It also returns the assigned identifier ('x' in this example) if that's the case.
func nonPtrProtoStructDef(c *cursor, stmt dst.Stmt) (*dst.Ident, bool) {
	// Not handled: multi-assign
	a, ok := stmt.(*dst.AssignStmt)
	if !ok || a.Tok != token.DEFINE || len(a.Lhs) != 1 {
		return nil, false
	}
	lit, ok := a.Rhs[0].(*dst.CompositeLit)
	if !ok {
		return nil, false
	}
	ident, ok := a.Lhs[0].(*dst.Ident)
	if !ok {
		// Shouldn't happen as only identifiers can be on LHS of a definition.
		return nil, false
	}
	if !c.shouldUpdateType(c.typeOf(lit)) {
		return nil, false
	}
	return ident, true
}

// replaceWithPtr modifies (if possible) stmts so that they work if ident was a pointer type (it
// must be a non-pointer).  For example, for identifier 'x' the following statements:
//
//	x.S = nil
//	_ = x.GetS()
//	f(&x)
//
// would be changed to:
//
//	x.S = nil
//	_ = x.GetS()
//	f(x)           // notice dropped "&"
//
// Changes are only made if they are possible. Any usage of the identifier for shallow copies
// prevents rewrites. For example, in the following example statements won't be changed:
//
//	f(&x)
//	E
//
// where E is any of
//
//	x = pb.M{}
//	y = x
//	*p = x
//	f(x)
//	[]pb.M{x}
//
// or anything else that's not a field access, a method call, &x, or an argument to a printer function.
//
// replaceWithPtr returns true if it modifies stmts.
func replaceWithPtr(c *cursor, ident *dst.Ident, stmts []dst.Stmt) bool {
	b := &dst.BlockStmt{List: stmts}
	canRewrite := true
	dstutil.Apply(b, func(cur *dstutil.Cursor) bool {
		n, ok := cur.Node().(*dst.Ident)
		if !ok {
			return true
		}
		if c.objectOf(n) != c.objectOf(ident) {
			return false
		}
		if isAddr(cur.Parent()) {
			return false
		}

		// If n's parent is a selector expression then we have one of those situations:
		//
		//   n.Field
		//   n.Func()
		//
		// This works whether n is a pointer or a value.
		if _, ok := cur.Parent().(*dst.SelectorExpr); ok {
			return false
		}

		// Address expressions are ok. We have to drop "&" once ident becomes a pointer.
		if e, ok := cur.Parent().(*dst.UnaryExpr); ok && e.Op == token.AND {
			return false
		}

		// If n is an argument to t.Errorf or other function used for printing then it's ok to make
		// n a pointer. That will change output format from:
		//   {F:...}
		// to
		//   &{F:...}
		// A shallow copy like this is incorrect. It's rare to depend on exact log statement though (I'm sure it happens).
		if c.lvl.ge(Yellow) && c.looksLikePrintf(cur.Parent()) {
			return false
		}

		// Otherwise we found a use of ident that can't be changed to a pointer.
		canRewrite = false

		return false
	}, nil)
	if !canRewrite {
		return false
	}

	// We can change the type of ident to be a pointer. We must replace all "&ident" expressions with "ident".
	dstutil.Apply(b, func(cur *dstutil.Cursor) bool {
		ue, ok := cur.Node().(*dst.UnaryExpr)
		if !ok || ue.Op != token.AND {
			return true
		}
		v, ok := ue.X.(*dst.Ident)
		if !ok || c.objectOf(ident) != c.objectOf(v) {
			return true
		}
		cur.Replace(v)
		return false
	}, nil)
	return true
}
