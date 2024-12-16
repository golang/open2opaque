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

// convertToSetterPost rewrites all non-empty composite literals into setters.
// For example:
//
//	 m := &pb.M{
//	   StringField: proto.String("Hello"),
//	 }
//
//	=>
//
//	 m := &pb.M{}
//	 m.SetStringField("Hello")
//
// That is, we DO NOT do the following:
//
//	m := pb.M_builder{
//	  StringField: proto.String("Hello"),
//	}.Build()
//
// We use setters instead of builders to avoid performance regressions,
// see the Opaque API FAQ: https://protobuf.dev/reference/go/opaque-faq/
func convertToSetterPost(c *cursor) bool {
	n := c.Node()
	// We need to work on the statement level, because we need to insert a new
	// statement (new variable declaration).
	if _, ok := n.(dst.Stmt); !ok {
		return true
	}

	// Ensure that the current statement is part of a slice so that we can
	// prepend additional statements using c.InsertBefore().
	switch p := c.Parent().(type) {
	case *dst.BlockStmt:
		// This statement is part of the BlockStmt.List slice
	case *dst.CaseClause:
		// This statement is part of the CaseClause.Body slice
	case *dst.CommClause:
		if p.Comm == n {
			return true
		}
		// This statement is part of the CommClause.Body slice
	default:
		return true
	}

	var assignmentReused *dst.AssignStmt

	var labelName *dst.Ident
	if ls, ok := n.(*dst.LabeledStmt); ok {
		labelName = ls.Label
	}

	// Find proto struct literals within the current statement and extract them.
	dstutil.Apply(n,
		nil,
		func(cur *dstutil.Cursor) bool {
			// Extract the builder composite literal into a helper variable and
			// modify it by calling the corresponding setter functions.
			lit, ok := c.builderCLit(cur.Node(), cur.Parent())
			if !ok {
				return true
			}
			if c.useBuilder(lit) {
				c.Logf("requested to use builders for this file or type %v", c.typeOf(lit))
				return true
			}

			// builderCLit returning ok means cur.Node() is a UnaryExpr (&{…})
			// or a CompositeLit ({…}), both of which implement Expr.
			litExpr := cur.Node().(dst.Expr)

			var exName string // name of the extracted part of the literal
			var exSource *dst.Ident
			shallowCopies := 0
			if as, ok := cur.Parent().(*dst.AssignStmt); ok {
				// Only rewrite shallow copies in red level, because it requires
				// follow-up changes to the code (e.g. changing the shallow copy
				// to use proto.Merge()).
				for _, lhs := range as.Lhs {
					if _, ok := lhs.(*dst.StarExpr); ok {
						if c.lvl.le(Yellow) {
							c.Logf("shallow copy detected, skipping")
							return true
						}
						shallowCopies++
					}
				}
			}
			if as, ok := n.(*dst.AssignStmt); ok && n == cur.Parent() {
				// For assignments (result := &pb.M2{…}) we reuse the variable
				// name (result) instead of introducing a helper only to
				// ultimately assign result := helper.
				for idx, rhs := range as.Rhs {
					if rhs != cur.Node() {
						continue
					}
					if as.Tok == token.ASSIGN && !types.Identical(c.typeOf(as.Lhs[idx]), c.typeOf(rhs)) {
						// If the static type is different then we might not be able
						// to call methods on it, e.g.:
						//
						//  var myMsg proto.Message
						//  myMsg = &pb2.M2{S: proto.String("Hello")}
						//
						// If we translate this as is, it would fail type checking:
						//
						//  var myMsg proto.Message
						//  myMsg = &pb2.M2{}
						//  myMsg.SetS("Hello") // compile-time error
						break
					}
					if id, ok := as.Lhs[idx].(*dst.Ident); ok && id.Name != "_" {
						exSource = id
						exName = id.Name
						assignmentReused = as
					}
				}
			}
			if exName == "" {
				exName = c.helperNameFor(c.Node(), c.typeOf(litExpr))
			}

			qualifiedLitExpr := litExpr
			if cl, ok := qualifiedLitExpr.(*dst.CompositeLit); ok {
				// The expression is a CompositeLit without explicit type, which
				// is valid within a slice initializer, but not outside, so we
				// need to wrap it in a UnaryExpr and assign a type identifier.
				typ := c.typeOf(litExpr)
				qualifiedLitExpr = &dst.UnaryExpr{
					Op: token.AND,
					X:  litExpr,
				}
				updateASTMap(c, litExpr, qualifiedLitExpr)
				c.setType(qualifiedLitExpr, typ)
				cl.Type = c.selectorForProtoMessageType(typ)
			}

			exIdent := &dst.Ident{Name: exName}
			if exSource != nil {
				updateASTMap(c, exSource, exIdent)
			} else {
				updateASTMap(c, lit, exIdent)
			}
			c.setType(exIdent, c.typeOf(qualifiedLitExpr))
			tok := token.DEFINE
			if assignmentReused != nil {
				tok = assignmentReused.Tok
			}
			assign := (dst.Stmt)(&dst.AssignStmt{
				Lhs: []dst.Expr{exIdent},
				Tok: tok,
				Rhs: []dst.Expr{qualifiedLitExpr},
			})

			replacement := cloneIdent(c, exIdent)
			// Move line break decorations from the literal to its replacement.
			replacement.Decorations().Before = litExpr.Decorations().Before
			replacement.Decorations().After = litExpr.Decorations().After

			if ce, ok := cur.Parent().(*dst.CallExpr); ok {
				for idx, arg := range ce.Args {
					if arg != cur.Node() {
						continue
					}
					if !fitsOnSameLine(ce, replacement) {
						continue
					}
					replacement.Decorations().Before = dst.None
					replacement.Decorations().After = dst.None
					if idx == 0 {
						continue
					}
					previous := ce.Args[idx-1]
					previous.Decorations().After = dst.None
				}
			}

			// Move end-of-line comments from the literal to its replacement.
			replacement.Decorations().End = litExpr.Decorations().End

			// Move decorations (line break and comments) of the containing
			// statement to the first inserted helper variable.
			assign.Decorations().Before = n.Decorations().Before
			assign.Decorations().Start = n.Decorations().Start
			n.Decorations().Before = dst.None
			n.Decorations().Start = nil
			// Move decorations (line break and comments) of the literal to its
			// corresponding helper variable.
			if assign.Decorations().Before == dst.None {
				assign.Decorations().Before = litExpr.Decorations().Before
			}
			assign.Decorations().Start = append(assign.Decorations().Start, litExpr.Decorations().Start...)

			// Remove all decorations from the composite literal to ensure that
			// there is no line break within the new AssignStmt (would fail to
			// compile). Comments have been retained in the lines above.
			litExpr.Decorations().Before = dst.None
			litExpr.Decorations().After = dst.None
			litExpr.Decorations().Start = nil
			litExpr.Decorations().End = nil

			// If the current node is a LabeledStmt, move the label to the first
			// inserted helper variable assignment.
			if labelName != nil {
				// Turn a Stmt with a label into just the Stmt, without a label.
				c.Replace(n.(*dst.LabeledStmt).Stmt)
				// Add the label to the assignment we are inserting.
				assign = &dst.LabeledStmt{
					Label: labelName,
					Stmt:  assign,
				}
				labelName = nil
			}

			// Rewrite the composite literal to assignments.
			elts := lit.Elts
			lit.Elts = nil
			// Replace references to exIdent.Name in the right-hand side:
			// now that we have cleared the RHS, references to exIdent.Name
			// refer to the new struct literal! b/277902682
			shadows := false
			for _, e := range elts {
				kv := e.(*dst.KeyValueExpr)
				dstutil.Apply(kv.Value,
					func(cur *dstutil.Cursor) bool {
						if _, ok := cur.Node().(*dst.SelectorExpr); ok {
							// skip over SelectorExprs to avoid false positives
							// when part of the selector (e.g. cmd.zone) happens
							// to match the identifier we are looking for
							// (e.g. zone).
							return false
						}

						id, ok := cur.Node().(*dst.Ident)
						if !ok {
							return true
						}
						if id.Name == exIdent.Name {
							shadows = true
						}
						return true
					},
					nil)
			}
			if shadows {
				// The right-hand side references exIdent. Introduce a helper
				// variable (e.g. cri2 := cri) and update all RHS references to
				// use the helper variable:
				helperName := c.helperNameFor(cur.Node(), c.typeOf(litExpr))
				helperIdent := &dst.Ident{Name: helperName}
				updateASTMap(c, lit, helperIdent)
				c.setType(helperIdent, c.typeOf(litExpr))

				helperAssign := &dst.AssignStmt{
					Lhs: []dst.Expr{helperIdent},
					Tok: token.DEFINE,
					Rhs: []dst.Expr{cloneIdent(c, exIdent)},
				}
				// Move decorations (line break and comments) from assign to
				// helperAssign, which just became the first inserted variable.
				helperAssign.Decorations().Before = assign.Decorations().Before
				helperAssign.Decorations().Start = assign.Decorations().Start
				assign.Decorations().Before = dst.None
				assign.Decorations().Start = nil
				c.InsertBefore(helperAssign)

				for _, e := range elts {
					kv := e.(*dst.KeyValueExpr)
					dstutil.Apply(kv.Value,
						func(cur *dstutil.Cursor) bool {
							if _, ok := cur.Node().(*dst.SelectorExpr); ok {
								// skip over SelectorExprs to avoid false positives
								// when part of the selector (e.g. cmd.zone) happens
								// to match the identifier we are looking for
								// (e.g. zone).
								return false
							}

							id, ok := cur.Node().(*dst.Ident)
							if !ok {
								return true
							}
							if id.Name == exIdent.Name {
								id.Name = helperIdent.Name
							}
							return true
						},
						nil)
				}
			}
			c.InsertBefore(assign)
			for _, e := range elts {
				kv := e.(*dst.KeyValueExpr)
				lhs := &dst.SelectorExpr{
					X:   cloneIdent(c, exIdent),
					Sel: cloneIdent(c, kv.Key.(*dst.Ident)),
				}
				c.setType(lhs, c.typeOf(lhs.Sel))
				a := &dst.AssignStmt{
					Lhs: []dst.Expr{lhs},
					Tok: token.ASSIGN,
					Rhs: []dst.Expr{kv.Value},
				}
				*a.Decorations() = *kv.Decorations()
				c.InsertBefore(a)
			}

			c.numUnsafeRewritesByReason[ShallowCopy] += shallowCopies
			cur.Replace(replacement)

			return true
		})

	if assignmentReused != nil && isIdenticalAssignment(n.(*dst.AssignStmt)) {
		c.Delete()
	}

	return true
}

func fitsOnSameLine(call *dst.CallExpr, replacement *dst.Ident) bool {
	combinedLen := len(replacement.Name)
	dstutil.Apply(call,
		func(cur *dstutil.Cursor) bool {
			if cur.Node() == call.Args[len(call.Args)-1] {
				// Skip the last argument, because we are about to replace it.
				return false // skip children
			}
			if id, ok := cur.Node().(*dst.Ident); ok {
				combinedLen += len(id.Name)
			}
			return true
		},
		nil)
	return combinedLen < 80
}

// isIdenticalAssignment reports whether the provided assign statement has the
// same names on the left hand side and right hand side, e.g. “foo := foo” or
// “foo, bar := foo, bar”. This can happen as part of convertToSetter() and
// results in the deletion of the now-unnecessary assignment.
func isIdenticalAssignment(as *dst.AssignStmt) bool {
	if len(as.Lhs) != len(as.Rhs) {
		return false
	}

	for idx := range as.Lhs {
		lhs, ok := as.Lhs[idx].(*dst.Ident)
		if !ok {
			return false
		}
		rhs, ok := as.Rhs[idx].(*dst.Ident)
		if !ok {
			return false
		}
		if lhs.Name != rhs.Name {
			return false
		}
	}

	return true
}
