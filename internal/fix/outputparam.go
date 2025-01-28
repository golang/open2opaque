// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"google.golang.org/open2opaque/internal/protodetecttypes"
)

// Recognize the *resp = <message> pattern and rewrite it to proto.Merge.
func outputParamPre(c *cursor) bool {
	as, ok := c.Node().(*dst.AssignStmt)
	if !ok {
		return true
	}

	if len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		c.Logf("ignoring AssignStmt with len(lhs)=%d, len(rhs)=%d (looking for 1, 1)", len(as.Lhs), len(as.Rhs))
		return true
	}

	switch as.Rhs[0].(type) {
	case *dst.Ident: // variable assignment
	case *dst.CompositeLit: // struct literal
	case *dst.CallExpr: // builder
	default:
		c.Logf("ignoring AssignStmt with rhs %T, want Ident, CompositeLit or CallExpr", as.Rhs[0])
		return true
	}

	se, ok := as.Lhs[0].(*dst.StarExpr)
	if !ok {
		c.Logf("ignoring AssignStmt with lhs %T, want StarExpr", as.Lhs[0])
		return true
	}
	if _, ok := se.X.(*dst.Ident); !ok {
		c.Logf("ignoring AssignStmt with lhs.X %T, want Ident", se.X)
		return true
	}

	T := c.typeOf(se)
	if !(protodetecttypes.Type{T: T}).IsMessage() {
		c.Logf("ignoring AssignStmt for non-proto message (%v)", T)
		return true
	}

	if resetNeeded(c, as, se.X.(*dst.Ident)) {
		resetFun := &dst.SelectorExpr{
			X:   &dst.Ident{Name: c.imports.name(protoImport)},
			Sel: &dst.Ident{Name: "Reset"},
		}
		resetCall := &dst.CallExpr{
			Fun: resetFun,
			Args: []dst.Expr{
				cloneIdent(c, se.X.(*dst.Ident)),
			},
			Decs: dst.CallExprDecorations{
				NodeDecs: dst.NodeDecs{
					Before: as.Decs.NodeDecs.Before,
					Start:  as.Decs.NodeDecs.Start,
				},
			},
		}
		as.Decs.NodeDecs.Before = dst.None
		as.Decs.NodeDecs.Start = nil
		c.setType(resetCall, types.Typ[types.Invalid])
		c.setType(resetFun, types.Typ[types.Invalid])
		c.setType(resetFun.X, types.Typ[types.Invalid])
		c.setType(resetFun.Sel, types.Typ[types.Invalid])
		c.InsertBefore(&dst.ExprStmt{X: resetCall})
	}

	// Replace the AssignStmt with a call to proto.Merge
	mergeFun := &dst.SelectorExpr{
		X:   &dst.Ident{Name: c.imports.name(protoImport)},
		Sel: &dst.Ident{Name: "Merge"},
	}
	unaryRHS := &dst.UnaryExpr{
		Op: token.AND,
		X:  as.Rhs[0],
	}
	mergeCall := &dst.CallExpr{
		Fun: mergeFun,
		Args: []dst.Expr{
			se.X,
			unaryRHS,
		},
		Decs: dst.CallExprDecorations{
			NodeDecs: as.Decs.NodeDecs,
		},
	}
	c.setType(mergeCall, types.Typ[types.Invalid])
	c.setType(mergeFun, types.Typ[types.Invalid])
	c.setType(mergeFun.X, types.Typ[types.Invalid])
	c.setType(mergeFun.Sel, types.Typ[types.Invalid])
	c.setType(unaryRHS, types.NewPointer(T))
	stmt := &dst.ExprStmt{X: mergeCall}
	c.Replace(stmt)
	updateASTMap(c, as, stmt)

	return true
}

func isStubbyHandler(c *cursor, ft *ast.FuncType) bool {
	if ft.Results == nil || len(ft.Results.List) != 1 {
		c.Logf("not a stubby handler: not precisely one return value")
		return false
	}
	if id, ok := ft.Results.List[0].Type.(*ast.Ident); !ok || id.Name != "error" {
		c.Logf("not a stubby handler: not returning an error")
		return false
	}
	if got, want := len(ft.Params.List), 3; got != want {
		c.Logf("incorrect number of parameters for a stubby handler: got %d, want %d", got, want)
		return false
	}
	params := ft.Params.List

	se, ok := params[0].Type.(*ast.SelectorExpr)
	if !ok {
		c.Logf("first parameter is not context.Context")
		return false
	}
	if id, ok := se.X.(*ast.Ident); !ok || id.Name != "context" {
		c.Logf("first parameter is not context.Context")
		return false
	}
	if se.Sel.Name != "Context" {
		c.Logf("first parameter is not context.Context")
		return false
	}

	// Both remaining parameters must be proto messages
	for _, param := range params[1:] {
		dstT, ok := c.typesInfo.dstMap[param.Type]
		if !ok {
			c.Logf("BUG: no corresponding dave/dst node for go/ast node %T / %+v", param.Type, param.Type)
			return false
		}

		T := c.typeOf(dstT.(dst.Expr))
		ptr, ok := types.Unalias(T).(*types.Pointer)
		if !ok {
			c.Logf("parameter %T is not a proto message", param.Type)
			return false
		}
		T = ptr.Elem()
		if !(protodetecttypes.Type{T: T}).IsMessage() {
			c.Logf("parameter %T is not a proto message", param.Type)
			return false
		}
	}

	return true
}

// resetNeeded figures out if for the specified AssignStmt, a proto.Reset() call
// needs to be inserted before or not.
//
// resetNeeded looks at the AST of the current scope, considering all statements
// between the scope opener (function declaration) and the AssignStmt:
//
//	func (*Srv) Handler(ctx context.Context, req *pb.Req, resp *pb.Resp) error {
//	  …            // checked by resetNeeded()
//	  mm2.I32 = 23 // uses mm2, not resp
//	  *resp = mm2  // AssignStmt
//	}
func resetNeeded(c *cursor, as *dst.AssignStmt, id *dst.Ident) bool {
	innerMost, opener := c.enclosingASTStmt(as)
	if innerMost == nil {
		return true
	}
	enclosing, ok := c.typesInfo.dstMap[innerMost]
	if !ok {
		c.Logf("BUG: no corresponding dave/dst node for go/ast node %T / %+v", innerMost, innerMost)
		return true
	}

	var ft *ast.FuncType
	switch x := opener.(type) {
	case *ast.FuncDecl:
		ft = x.Type
	case *ast.FuncLit:
		ft = x.Type
	default:
		c.Logf("scope opener is %T, not a FuncDecl or FuncLit", opener)
		return true
	}

	if !isStubbyHandler(c, ft) {
		c.Logf("scope opener is not a stubby handler")
		return true
	}

	lastSeen := false
	usageFound := false
	var visit visitorFunc
	xObj := c.objectOf(id)

	visit = func(n dst.Node) dst.Visitor {
		if n == as {
			lastSeen = true
			return nil
		}
		if lastSeen {
			return nil
		}

		if id, ok := n.(*dst.Ident); ok {
			// Is the variable ever used?
			if usesObject(c, id, xObj) {
				usageFound = true
				return nil
			}
		}

		return visit // recurse into children
	}
	dst.Walk(visit, enclosing)
	// proto.Reset() calls are definitely needed when:
	//
	// 1. !lastSeen — we couldn’t find the usage of <sel> (bug?)
	//
	// 2. or usageFound — we did find a usage that we didn’t expect.
	return !lastSeen || usageFound
}
