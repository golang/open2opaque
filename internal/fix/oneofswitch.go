// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"fmt"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// oneofSwitchPost handles type switches of protocol buffer oneofs.
func oneofSwitchPost(c *cursor) bool {
	stmt, ok := c.Node().(*dst.TypeSwitchStmt)
	if !ok {
		c.Logf("ignoring %T (looking for TypeSwitchStmt)", c.Node())
		return true
	}
	initStmt := stmt.Init

	var typeAssert *dst.TypeAssertExpr
	// oneofIdent is the variable introduced for the expression under the type
	// assertion (e.g. "x" in "x := m.F.(type)").
	//
	// We keep track of it so that we can replace it with accesses to m later on.
	//
	// This can be nil as "m.F.(type)" is also valid (type-switch without
	// assignment), in which case we don't have to rewrite any references to the
	// oneof in type-switch clauses.
	var oneofIdent *dst.Ident

	switch a := stmt.Assign.(type) {
	case *dst.AssignStmt: // "switch x := m.F.(type) {"
		if len(a.Lhs) != 1 || len(a.Rhs) != 1 {
			c.Logf("ignoring AssignStmt with len(lhs)=%d, len(rhs)=%d (looking for 1, 1)", len(a.Lhs), len(a.Rhs))
			return true
		}
		ident, ok := a.Lhs[0].(*dst.Ident)
		if !ok {
			c.Logf("ignoring Lhs=%T (looking for Ident)", a.Lhs[0])
			return true
		}
		oneofIdent = ident
		typeAssert, ok = a.Rhs[0].(*dst.TypeAssertExpr)
		if !ok {
			c.Logf("ignoring Rhs=%T (looking for TypeAssertExpr)", a.Rhs[0])
			return true
		}
	case *dst.ExprStmt: // "switch m.F.(type) {"
		typeAssert, ok = a.X.(*dst.TypeAssertExpr)
		if !ok {
			c.Logf("ignoring TypeSwitchStmt.Assign.X=%T (looking for TypeAssertExpr)", a.X)
			return true
		}
	default:
		c.Logf("ignoring TypeSwitchStmt.Assign=%T (looking for TypeAssertExpr or AssignStmt)", a)
		return true
	}
	var oneofScope *types.Scope
	if oneofIdent != nil {
		ooi, ok := c.typesInfo.astMap[oneofIdent]
		if !ok {
			panic(fmt.Sprintf("failed to retrieve ast.Node for dst.Node: %v", oneofIdent))
		}
		oneofScope = c.pkg.TypePkg.Scope().Innermost(ooi.Pos())
	}

	// Below we assume how the "Which" method is named based on either the name of
	// the corresponding field or "Get" method. This isn't necessarily sound in
	// the presence of naming conflicts but we don't want to repeat that
	// complicated logic here.
	var whichSig *types.Signature
	var whichName string
	var recv dst.Expr // "X" in "X.F.(type)" or "X.GetF().(type)"

	// We track oneof field and getter name here to keep the name-related logic
	// in one place, at the top. We use those to rewrite oneof field access
	// through field/getter later on. Ideally, we would use object identity
	// instead of comparison by name, but we can't get that without messing with
	// names, so we might as well got for name comaprisons.
	var oneofFieldName string // "F" in "X.F.(type)" or "X.GetF().(type)"
	var oneofGetName string   // "GetF" corresponding to oneofFieldName

	// Find the WhichF() method that should replace "m.F.(type)" in
	//   switch m.F.(type) {
	// =>
	//   switch m.WhichF() {
	if field, ok := c.trackedProtoFieldSelector(typeAssert.X); ok {
		if !isOneof(c.typeOf(field)) {
			c.Logf("ignoring field %v because it is not a oneof", field)
			return true
		}
		oneofFieldName = field.Sel.Name
		oneofGetName = "Get" + field.Sel.Name
		whichName = "Which" + oneofFieldName
		c.Logf("rewriting TypeAssertExpr on oneof field %s to %s CallExpr", oneofFieldName, whichName)
		whichSig = whichOneofSignature(c.typeOf(field.X), whichName)
		recv = field.X
	}

	// Find the WhichF() method that should replace "m.GetF().(type)" in
	//   switch m.GetF().(type) {
	// =>
	//   switch m.WhichF() {
	if call, ok := typeAssert.X.(*dst.CallExpr); ok {
		sel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok || !isOneof(c.typeOf(call)) || !strings.HasPrefix(sel.Sel.Name, "Get") || !c.shouldUpdateType(c.typeOf(sel.X)) {
			// Even if this is a call returning a oneof, we can't do anything about
			// it. We don't add DO_NOT_SUBMIT tag because it's too easy to
			// accidentally add it to unrelated type switches here.
			c.Logf("ignoring: TypeAssertExpr recondition is not met")
			return true
		}
		recv = sel.X
		oneofGetName = sel.Sel.Name
		oneofFieldName = oneofGetName[len("Get"):]
		whichName = "Which" + oneofFieldName
		c.Logf("rewriting TypeAssertExpr on oneof field %s to %s CallExpr", oneofFieldName, whichName)
		whichSig = whichOneofSignature(c.typeOf(sel.X), whichName)
	}

	// Not a oneof access (call or field) or we couldn't find the "Which" method.
	if whichSig == nil {
		c.Logf("ignoring: cannot find Which${Field} method")
		return true
	}

	// If recv may have side effects, then move it to the init statement if
	// possible (bail otherwise) in order to avoid cloning side effects into
	// the body of the type-switch.
	if !c.isSideEffectFree(recv) {
		if initStmt != nil {
			if c.lvl.ge(Red) {
				c.numUnsafeRewritesByReason[IncompleteRewrite]++
				markMissingRewrite(stmt, "type switch with side effects and init statement")
			}
			c.Logf("ignoring: cannot move init statement with side effects")
			return true
		}
		newVar := dst.NewIdent("xmsg")
		c.setType(newVar, c.typeOf(recv))
		initStmt = &dst.AssignStmt{
			Lhs: []dst.Expr{newVar},
			Tok: token.DEFINE,
			Rhs: []dst.Expr{recv},
		}
		recv = cloneIdent(c, newVar)
	}

	// Construct the "recv.WhichOneofField()" method call.
	whichMethod := &dst.SelectorExpr{
		X:   recv,
		Sel: dst.NewIdent(whichName),
	}
	c.setType(whichMethod, whichSig)
	c.setType(whichMethod.Sel, whichSig)
	whichCall := &dst.CallExpr{Fun: whichMethod}
	c.setType(whichCall, whichSig.Results().At(0).Type())

	// First, verify that we can do necessary rewrites to the entire type switch
	// statement.
	//
	// In "switch x := m.F.(type) {" and similar, we replace all references
	// to "x" (the oneof object, "oneofIdent") with method calls on "m".
	definedVarUsedInDefaultCase := false
	if oneofIdent != nil {
		var maybeUnsafe bool
		if !c.lvl.ge(Red) {
			// Unfortunately "x" in "x := X.(type)" has no identity, so we must
			// rely on name matching to replace all instances of "x" with
			// something else. We want to be careful: if any clause refers to
			// more than one "x" ("x" has identity in non-default clauses) that
			// could be a oneof then we don't do anything.
			//
			// If any non-default clause refers to "x" (that could be a oneof)
			// not in the context of a selector (i.e. it's not "x.F") then we
			// also don't do anything. Oneofs are not a "thing" and can't be
			// referenced like this in the new API.
			//
			// The sole exception is printf-like calls in the default clause
			// (see below), where we replace the oneof reference with a "Which"
			// method call.
			for _, clause := range stmt.Body.List {
				var clauseIdent *dst.Ident
				dstutil.Apply(clause, func(cur *dstutil.Cursor) bool {
					ident, ok := cur.Node().(*dst.Ident)
					if !ok || ident.Name != oneofIdent.Name {
						return true
					}
					isDefaultClause := clause.(*dst.CaseClause).List == nil // Ident has no type in "default".
					if !isDefaultClause && !isOneofWrapper(c, ident) {
						return true
					}
					// Usually we don't allow references to the "oneof" itself, but
					// we make an exception for print-like statements as
					// it's comment to say: `fmt.Errorf("unknown case %v", oneofField)`.
					if c.looksLikePrintf(cur.Parent()) {
						return true
					}
					if _, ok := cur.Parent().(*dst.SelectorExpr); !ok {
						maybeUnsafe = true
						return false
					}
					if clauseIdent == nil {
						clauseIdent = ident
						return true
					}
					if c.objectOf(clauseIdent) == c.objectOf(ident) {
						return true
					}
					maybeUnsafe = true
					return false
				}, nil)
				if maybeUnsafe {
					c.Logf("ignoring: cannot rewrite oneof usage safely")
					return true
				}
			}
		}

		// Then do the necessary rewrites.
		for _, clause := range stmt.Body.List {
			dstutil.Apply(clause, func(cur *dstutil.Cursor) bool {
				n := cur.Node()
				// First, handle direct references to the oneof itself,
				// in default clauses:
				//   fmt.Errorf("Unknown case %v", oneofField)
				// =>
				//   fmt.Errorf("Unknown case %v", m.WhichOneofField())
				//
				// This works well, because oneof cases have String() method.
				//
				// Note that we do the rewrite regardless of what's in the format
				// string (we don't want to get into the business of parsing format
				// strings now), so we risk:
				//   fmt.Errorf("Unknown case %T", m.WhichOneofField())
				// Which is not ideal, but not unreasonable.
				if c.looksLikePrintf(cur.Parent()) {
					// rewrite the formatting verb (if any) to %v:
					//  fmt.Errorf("%T", m2.OneofField) => fmt.Errorf("%v", m2.OneofField)
					// The other part of the expression is rewritten below.
					// This function does not handle escaped '%' character.
					// Implementing a full formatting string parser is out of scope here.
					rewriteVerb := func() {
						call := cur.Parent().(*dst.CallExpr)
						argIndex := slices.Index(call.Args, cur.Node().(dst.Expr))
						if argIndex == -1 {
							panic(fmt.Sprintf("could not find argument %v in call expression %v. Did you call rewriteVerb after replacing the Node?", cur.Node(), call))
						}
						for i, p := range call.Args {
							slit, ok := p.(*dst.BasicLit)
							if !ok {
								continue
							}
							if slit.Kind != token.STRING {
								continue
							}
							numFormattedArg := argIndex - i
							idx := -1
							for i, r := range slit.Value {
								if r == '%' {
									numFormattedArg--
									if numFormattedArg == 0 {
										idx = i
										break
									}
								}
							}
							// There are not enough formatting verbs.
							if idx == -1 {
								return
							}
							if idx == len(slit.Value) || slit.Value[idx+1] == 'v' {
								break
							}
							// No need to clone as a literal cannot be shared between Nodes.
							slit.Value = slit.Value[:idx] + "%v" + slit.Value[idx+2:]
						}
					}
					//   switch m := f(); oneofField := m.OneofField.(type) {...
					//   default:
					//     return fmt.Errorf("Unknown case %v", oneofField)
					// =>
					//   switch m := f(); oneofField := m.OneofField.(type) {...
					//   default:
					//     return fmt.Errorf("Unknown case %v", m.WhichOneofField())
					if ident, ok := n.(*dst.Ident); ok && ident.Name == oneofIdent.Name {
						rewriteVerb()
						definedVarUsedInDefaultCase = true
						if initStmt == nil {
							return true
						}
						c.Logf("rewriting usage of %q", oneofIdent.Name)
						if maybeUnsafe {
							c.numUnsafeRewritesByReason[MaybeSemanticChange]++
						}
						cur.Replace(cloneSelectorCallExpr(c, whichCall))
						return true
					}
					//   fmt.Errorf("Unknown case %v", m.OneofField)
					// =>
					//   fmt.Errorf("Unknown case %v", m.WhichOneofField())
					if field, ok := c.trackedProtoFieldSelector(n); ok && isOneof(c.typeOf(field)) && field.Sel.Name == oneofFieldName {
						c.Logf("rewriting usage of %q", oneofIdent.Name)
						if maybeUnsafe {
							c.numUnsafeRewritesByReason[MaybeSemanticChange]++
						}
						rewriteVerb()
						cur.Replace(cloneSelectorCallExpr(c, whichCall))
						return true
					}
					//   fmt.Errorf("Unknown case %v", m.GetOneofField())
					// =>
					//   fmt.Errorf("Unknown case %v", m.WhichOneofField())
					if call, ok := n.(*dst.CallExpr); ok && isOneof(c.typeOf(call)) {
						if sel, ok := call.Fun.(*dst.SelectorExpr); ok && sel.Sel.Name == oneofGetName {
							c.Logf("rewriting usage of %q", oneofIdent.Name)
							if maybeUnsafe {
								c.numUnsafeRewritesByReason[MaybeSemanticChange]++
							}
							rewriteVerb()
							cur.Replace(cloneSelectorCallExpr(c, whichCall))
							return true
						}
					}
				}

				// Handle "oneofField.F" => "recv.F"
				// Subsequent passes may then do more rewrites. For example:
				//   "recv.F" => "recv.{Get,Set,Has,Clear}F()"
				sel, ok := n.(*dst.SelectorExpr)
				if !ok {
					c.Logf("ignoring usage %v of type %T (looking for SelectorExpr)", n, n)
					return true
				}
				ident, ok := sel.X.(*dst.Ident)
				if !ok {
					c.Logf("ignoring usage %v of type SelectorExpr.X.%T (looking for SelectorExpr.X.Ident)", sel, sel.X)
					return true
				}
				// We cannot do an object based comparison here
				// because variable declared in the initializer
				// of a switch don't have an associated object
				// (or there is a bug in our tooling that removes
				// this association).
				if ident.Name != oneofIdent.Name {
					c.Logf("ignoring %v because it is not using %v", sel, oneofIdent)
					return true
				}
				isDefaultClause := clause.(*dst.CaseClause).List == nil
				if !isDefaultClause && !isOneofWrapper(c, ident) {
					c.Logf("ignoring usage %v: not in default clause and not a oneof wrapper", sel)
					return true
				}

				// Check if the name is shadwed in a nested block:
				//
				//  switch oneofField := m2.OneofField.(type) {
				//  case *pb.M2_StringOneof:
				//  	{
				//  		oneofField := &O{}
				//  		_ = oneofField.StringOneof
				//  	}
				//  }
				//
				// For switch-case statements there is one scope for the switch
				// which has child scopes, one for each case clause. A variable
				// defined in the switch condition (oneofField in the example)
				// is not part of the parent (switch) scope but is defined in
				// every child (case) scope.
				// This means to check if an identifier references the variable
				// defined by the switch condition, we must check if the
				// referenced object is any of the child (case) scopes.
				if oneofScope == nil {
					c.Logf("ignoring usage %v: no scope for oneof found", n)
					return true
				}
				astNode, ok := c.typesInfo.astMap[ident]
				if !ok {
					panic(fmt.Sprintf("failed to retrieve ast.Node for dst.Node: %p", ident))
				}
				scope := c.pkg.TypePkg.Scope().Innermost(astNode.Pos())
				s, _ := scope.LookupParent(ident.Name, astNode.Pos())
				if s == nil {
					panic(fmt.Sprintf("invalid package: cannot resolve %v", astNode))
				}
				found := false
				for i := 0; i < oneofScope.NumChildren(); i++ {
					if oneofScope.Child(i) == s {
						found = true
						break
					}
				}
				// The variable defined in the switch is shadowed by another
				// definition.
				if !found {
					c.Logf("ignoring usage %v: does not reference oneof type switch variable", astNode)
					return true
				}

				c.Logf("rewriting usage %v", recv)
				switch recv := recv.(type) {
				case *dst.Ident:
					sel.X = cloneIdent(c, recv)
				case *dst.IndexExpr:
					sel.X = cloneIndexExpr(c, recv)
				case *dst.SelectorExpr:
					sel.X = cloneSelectorExpr(c, recv)
				case *dst.CallExpr:
					sel.X = cloneSelectorCallExpr(c, recv)
				default:
					panic(fmt.Sprintf("unsupported receiver AST type %T in oneof type switch", recv))
				}
				if maybeUnsafe {
					c.numUnsafeRewritesByReason[MaybeSemanticChange]++
				}
				return true
			}, nil)
		}
	}

	// Rewrite clauses:
	//   case *pb.M_Oneof:
	// =>
	//   case pb.M_Oneof_case:
	for _, clause := range stmt.Body.List {
		cl, ok := clause.(*dst.CaseClause)
		if !ok {
			continue
		}
		for i, typ := range cl.List {
			if ident, ok := typ.(*dst.Ident); ok && ident.Name == "nil" {
				zero := &dst.BasicLit{Kind: token.INT, Value: "0"}
				c.setType(zero, types.Typ[types.Int32])
				c.Logf("rewriting nil-case to 0-case in %v", cl)
				cl.List[i] = zero
				continue
			}

			se, ok := typ.(*dst.StarExpr)
			if !ok {
				c.Logf("skipping case %v of type %T (looking for StarExpr)", cl, typ)
				continue
			}
			sel, ok := se.X.(*dst.SelectorExpr)
			if !ok {
				c.Logf("skipping case %v of type %T (looking for SelectorExpr)", cl, se)
				continue
			}

			// Split the oneof wrapper type name into its parts:
			parts := strings.Split(strings.TrimRight(sel.Sel.Name, "_"), "_")
			// parts[0] is the parent of the oneof field
			// parts[len(parts)-1] is the oneof field
			if len(parts) < 2 {
				c.Logf("skipping case %v, sel %q does not split into parent_field", cl, sel.Sel.Name)
				continue
			}
			field := parts[len(parts)-1]
			suffix := "_case"
			// There are two cases in which the wrapper type name ends in an
			// underscore:
			//
			// 1. The field name itself conflicts. This means the field was
			//    called reset, marshal, etc., conflicting with the proto
			//    generated code method names. In this case, the _case constant
			//    will use two underscores.
			//
			// 2. The field name conflicts with another name in the .proto file.
			//    In this case, the _case constant will not use two underscores.
			if strings.HasSuffix(sel.Sel.Name, "_") && !conflictingNames[field] {
				suffix = "case"
			}

			sel.Sel.Name += suffix
			sel.Decs.Before = se.Decs.Before
			sel.Decs.Start = append(sel.Decs.Start, se.Decs.Start...)
			sel.Decs.End = append(sel.Decs.End, se.Decs.End...)
			sel.Decs.After = se.Decs.After
			c.Logf("rewriting case %+#v", cl.List[i])
			cl.List[i] = sel
		}
	}

	c.Logf("rewriting SwitchStmt")

	// The TypeSwitchStmt initializes a local variable that we need to keep
	// and there is no init statement.
	//   switch oneofField := m.OneofField.(type) {...
	//   default:
	//     return fmt.Errorf("Unknown case %v", oneofField)
	// =>
	//   switch oneofField := m.WhichOneof() {...
	//   default:
	//     return fmt.Errorf("Unknown case %v", oneofField)
	// If it were not used we would rewrite it to:
	//   switch oneofField := m.OneofField.(type) {
	//   ...
	//   default:
	//     return fmt.Errorf("Unknown case")
	// =>
	//   switch m.WhichOneof() {...
	//   ...
	//   default:
	//     return fmt.Errorf("Unknown case")
	var tag dst.Expr = cloneSelectorCallExpr(c, whichCall)
	if definedVarUsedInDefaultCase && initStmt == nil {
		if oneofIdent == nil {
			panic("identNil")
		}
		c.setType(oneofIdent, types.Typ[types.Invalid])
		initStmt = &dst.AssignStmt{
			Lhs: []dst.Expr{
				cloneIdent(c, oneofIdent),
			},
			Rhs: []dst.Expr{tag},
			Tok: token.DEFINE,
		}
		tag = cloneIdent(c, oneofIdent)
		c.setType(tag, c.typeOf(oneofIdent))
	}
	// Change the type from TypeSwitchStmt to SwitchStmt.
	c.Replace(&dst.SwitchStmt{
		Init: initStmt,
		Tag:  tag,
		Body: stmt.Body,
		Decs: dst.SwitchStmtDecorations{
			NodeDecs: stmt.Decs.NodeDecs,
			Switch:   stmt.Decs.Switch,
			Init:     stmt.Decs.Init,
			Tag:      stmt.Decs.Assign,
		},
	})

	return true
}

// whichOneofSignature returns the signature type of the method, of type t, with
// the given name. It must have "Which" prefix (i.e. it is the method for
// getting the oneof type). It returns an in-band nil (instead of a separate
// "ok" bool) if such method does not exist in order to simplify the caller code
// above.
func whichOneofSignature(t types.Type, name string) *types.Signature {
	if !strings.HasPrefix(name, "Which") {
		panic(fmt.Sprintf("whichOneofSignature called with %q; must have 'Which' prefix", name))
	}
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return nil
	}
	typ, ok := ptr.Elem().(*types.Named)
	if !ok {
		return nil
	}
	var m *types.Func
	for i := 0; i < typ.NumMethods(); i++ {
		m = typ.Method(i)
		if m.Name() == name {
			break
		}
	}
	if m == nil {
		return nil
	}
	sig := m.Type().(*types.Signature)
	if sig.Results().Len() != 1 {
		return nil
	}
	return sig
}
