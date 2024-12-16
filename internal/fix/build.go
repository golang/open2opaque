// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"go/token"
	"go/types"

	"github.com/dave/dst"
)

// buildPost rewrites the code to use builders for object construction. This
// function is executed by traversing the tree in postorder. It visits every
// node (always returns true to continue the traversal) because Builder calls can
// be nested.
func buildPost(c *cursor) bool {
	// &pb.M{F: V}   =>   pb.M_builder{F: V}.Build()
	if _, ok := c.Node().(*dst.UnaryExpr); ok {
		if lit, ok := c.builderCLit(c.Node(), c.Parent()); ok {
			if !c.useBuilder(lit) {
				c.Logf("requested to not use builders for this file or type %v", c.typeOf(lit))
				return true
			}
			expr := c.Node().(dst.Expr)
			incompleteRewrite := !updateBuilderElements(c, lit)
			if incompleteRewrite && !c.lvl.ge(Red) {
				c.Logf("returning, no builder elements updated")
				return true
			}
			if call, ok := newBuildCall(c, c.typeOf(expr), lit.Type, lit, *expr.Decorations()); ok {
				if incompleteRewrite {
					c.ReplaceUnsafe(call, IncompleteRewrite)
				} else {
					c.Replace(call)
				}
				c.Logf("successfully generated builder")
			}
			return true
		}
	}

	// K: {F: V}   =>  K: pb.M_builder{F: V}.Build()   for map KVs and slice/array KVs
	//
	// We assert on the key value here and not on the composite literal because:
	//   - we can only access the parent of the node, but not its grandparent
	//   - the relevant container, in case of KV composite literal values, is a grandparent as the KV is the parent.
	// An alternative implementation would track parents of all nodes.
	if kv, ok := c.Node().(*dst.KeyValueExpr); ok {
		lit, ok := kv.Value.(*dst.CompositeLit)
		if !ok || lit.Type != nil || len(lit.Elts) == 0 || !c.shouldUpdateType(c.typeOf(lit)) || !isPtr(c.typeOf(lit)) {
			return true
		}
		if !c.useBuilder(lit) {
			c.Logf("requested to not use builders for this file or type %v", c.typeOf(lit))
			return true
		}
		typ, ok := parentValPBType(c, kv)
		if !ok {
			return true
		}
		incompleteRewrite := !updateBuilderElements(c, lit)
		if incompleteRewrite && !c.lvl.ge(Red) {
			return true
		}
		if call, ok := newBuildCall(c, c.typeOf(lit), typ, lit, dst.NodeDecs{}); ok {
			kv.Value = call
			if incompleteRewrite {
				c.ReplaceUnsafe(kv, IncompleteRewrite)
			} else {
				c.Replace(kv)
			}
		}
		return true
	}

	// {F: V}   =>  pb.M_builder{F: V}.Build()     when {F: V} is a protobuf (e.g. "[]*pb.M{{F0: V0}, {F1:V1}}")
	lit, ok := c.Node().(*dst.CompositeLit)
	if !ok || lit.Type != nil || len(lit.Elts) == 0 || !c.shouldUpdateType(c.typeOf(lit)) || !isPtr(c.typeOf(lit)) {
		return true
	}
	if !c.useBuilder(lit) {
		c.Logf("requested to not use builders for this file or type %v", c.typeOf(lit))
		return true
	}
	typ, ok := parentValPBType(c, lit)
	if !ok {
		return true
	}
	incompleteRewrite := !updateBuilderElements(c, lit)
	if incompleteRewrite && !c.lvl.ge(Red) {
		return true
	}
	if call, ok := newBuildCall(c, c.typeOf(lit), typ, lit, dst.NodeDecs{}); ok {
		if incompleteRewrite {
			c.ReplaceUnsafe(call, IncompleteRewrite)
		} else {
			c.Replace(call)
		}
	}
	return true
}

func isNeverNilExpr(c *cursor, e dst.Expr) bool {
	// Is this taking the address of something?
	if ue, ok := e.(*dst.UnaryExpr); ok && ue.Op == token.AND {
		return true
	}
	// Is this a builder call?
	if ce, ok := e.(*dst.CallExpr); ok && len(ce.Args) == 0 {
		sel, ok := ce.Fun.(*dst.SelectorExpr)
		if !ok {
			return false
		}
		if !c.isBuilderType(c.typeOf(sel.X)) {
			return false
		}
		// As of 2023-06-29 there is only one function defined on the builder.
		// Thus, technically this check is not needed but it is a second layer
		// of defense.
		if sel.Sel.Name != "Build" {
			return false
		}
		return true
	}
	return false
}

// updateBuilderElements does necessary rewrites to elements when changing a
// composite literal into a builder.
//
// Returns ok==true if all elements could be handled and ok==false if there was
// a case that we can't handle yet and hence didn't rewrite anything.
func updateBuilderElements(c *cursor, lit *dst.CompositeLit) (ok bool) {
	// A list of updates to execute if there are no hard cases.
	var updates []func()

	// Handle oneof fields in builders.
	//
	//    pb.M{F: pb.M_Oneof{K: V}}
	//    pb.M{F: pb.M_Oneof{V}}
	//    pb.M{F: pb.M_Oneof{}}
	// =>
	//    pb.M_builder{K: V'}.Build()
	//
	// Where
	//
	//    F used to be the made up "oneof field"
	//    K is the name of the only field in the oneof wrapper for the field
	//    V' is V made into a pointer for basic types (it's a pointer already for
	//       other types). If V is not present then this is a pointer to the zero
	//       value for basic types and no rewrite for enums/messages.
	for _, e := range lit.Elts {
		c.Logf("updating composite literal element")
		kv, ok := e.(*dst.KeyValueExpr)
		if !ok {
			c.Logf("skipping %T (looking for KeyValueExpr)", e)
			continue
		}

		// Skip over fields that are not oneof.
		if _, ok := c.underlyingTypeOf(kv.Key).(*types.Interface); !ok {
			c.Logf("skipping none oneof field", e)
			continue
		}

		// Check that the value is a oneof that we can rewrite: address of a
		// composite literal with exactly one field that has a "oneof" tag
		fieldName, fieldType, fieldValue, decs, ok := destructureOneofWrapper(c, kv.Value)
		if !ok {
			// RHS is not a oneof wrapper but a oneof field itself.
			// Try generating an exhaustive list covering all cases.
			updates, ok = generateOneofBuilderCases(c, updates, lit, kv)
			if !ok {
				c.Logf("failed to generate exhaustive list of oneof cases")
				return false
			}
			// At this point we know that we can replace the oneof field with
			// key value pairs for its cases.
			e := e
			updates = append(updates, func() {
				var idx int
				// We know that this always find the element because there is
				// exactly one attempt to rewrite this KeyValueExpr.
				for i, ne := range lit.Elts {
					if e == ne {
						idx = i
					}
				}
				// Remove the KeyValueExpr for the oneof field since it was
				// replaced by KeyValueExpr's for all cases in
				// generateOneofBuilderCases().
				lit.Elts = append(lit.Elts[:idx], lit.Elts[idx+1:]...)
			})
			c.Logf("generated exhaustive list of oneof cases")
			continue
		}

		// Don't rewrite the oneof in
		//
		//   &pb.M2{OneofField: &pb.M2_MsgOneof{}}
		//   &pb.M2{OneofField: &pb.M2_EnumOneof{}}
		//
		// It's a tricky case and should be very rare because "oneof
		// with a type but without a value" is not a valid protocol buffers
		// concept.
		//
		// It happens due to a mismatch between what's allowed in protocol buffers
		// and the Go structs representing protocol buffers in the open API.
		// This information will not be marshalled to the wire and the field
		// is effectively discarded during marshalling when it does not have
		// a value.
		//
		// The opaque API does not allow this. While the struct can technically
		// represent this, you cannot use the API to bring the struct into
		// this state.
		//
		// For enums: this should be rare and we don't want to guess the default
		// value.
		if fieldValue == nil && !isBasic(fieldType) {
			c.Logf("returning: RHS is nil of Type %T (looking for types.Basic)", fieldType)
			return false
		}

		// If the wrapped value can be nil, it is generally not safe to
		// rewrite because this changes behavior from a set oneof field with
		// type but no value to a completely unset oneof.
		unsafeRewrite := false
		if !isNeverNilExpr(c, fieldValue) && !isNeverNilSliceExpr(c, fieldValue) && !isBasic(fieldType) && !isEnum(fieldType) {
			if !c.lvl.ge(Yellow) {
				c.Logf("returning: RHS is nil of Type %T (looking for types.Basic)", fieldType)
				return false
			}
			unsafeRewrite = true
		}

		// Handle `M{Oneof: OneofBasicField{}}`
		if fieldValue == nil {
			updates = append(updates, func() {
				kv.Key.(*dst.Ident).Name = fieldName // Rename the key to field name from the oneof wrapper.
				kv.Value = c.newProtoHelperCall(nil, fieldType.(*types.Basic))
			})
			c.Logf("generated RHS for field %v", fieldName)
			continue
		}

		// We don't handle assigning literal integers to enum fields:
		//
		//   1. This is a rare way to set enums (most code uses enum
		//      constants, not integer literals)
		//
		//   2. Handling this case requires a lot of extra machinery. We
		//      must be able to construct AST by knowing only the type
		//      that we want. This requires inspecting imports, ensuring
		//      that they are not shadowed, and potentially adding new
		//      imports.
		t := c.typeOf(fieldValue)
		if _, ok := fieldValue.(*dst.BasicLit); ok && isEnum(t) {
			c.Logf("returning: assignment of int literal to enum")
			return false
		}

		// If it's not a pointer and not []byte then make it a pointer
		// because everything in the builder is a pointer.
		//
		// In practice, isPtr checks if the type is a message because
		// non-pointer, non-[]byte types in oneof wrappers are either
		// enums (*types.Named) or basic types (*types.Basic).
		if !isPtr(t) && !isSlice(t) {
			fieldValue = c.newProtoHelperCall(fieldValue, t)
		}

		updates = append(updates, func() {
			if decs != nil {
				kv.Decorations().Start = append(kv.Decorations().Start, decs.Start...)
				kv.Decorations().End = append(kv.Decorations().End, decs.End...)
			}
			kv.Key.(*dst.Ident).Name = fieldName // Rename the key to field name from the oneof wrapper.
			kv.Value = fieldValue
			if unsafeRewrite {
				c.numUnsafeRewritesByReason[MaybeOneofChange]++
			}
		})
	}

	// If we are here then we're confident that we can rewrite the composite
	// literal to a builder.

	for _, u := range updates {
		u()
	}

	// Rename fields to deal with naming conflicts:
	for _, e := range lit.Elts {
		kv, ok := e.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		sel, ok := kv.Key.(*dst.Ident)
		if !ok {
			continue
		}
		sel.Name = fixConflictingNames(c.typeOf(lit), "", sel.Name)
	}

	c.Logf("updated of expressions of composite literal")
	return true
}

// parentValPBType returns the expression representing the type of x in the parent. For example:
//
//	In:
//	   []*pb.M{   // [1]
//	     {M:nil}, // [2]
//	   }
//	the return value for composite literal "{M:nil}" from line [2] is "pb.M" from line [1].
//
//	In:
//
//	  map[int]*pb.M{  // [1]
//	    0: {M:nil},   // [2]
//	  }
//	the return value for key-value expression "0: {M:nil}" on line [2] is "pb.M" from line [1].
func parentValPBType(c *cursor, x dst.Expr) (dst.Expr, bool) {
	plit, ok := c.Parent().(*dst.CompositeLit)
	if !ok {
		return nil, false
	}
	var typ dst.Expr
	switch t := plit.Type.(type) {
	case *dst.ArrayType:
		typ = t.Elt
	case *dst.MapType:
		typ = t.Value
	default:
		return nil, false
	}
	se, ok := typ.(*dst.StarExpr)
	if !ok {
		return nil, false
	}
	return se.X, true
}

// newBuildCall wraps the provided elements in builder Build call for the provided type.
// t is the type of Build() result (pointer to a message struct).
// typ is DST representing the protobuf struct type (e.g. selector "pb.M").
// lit is the source composite literal (e.g. "pb.M{F: V}"). It has a non-zero number of elements.
func newBuildCall(c *cursor, t types.Type, typ dst.Expr, lit *dst.CompositeLit, parentDecs dst.NodeDecs) (dst.Expr, bool) {
	sel, ok := typ.(*dst.SelectorExpr)
	if !ok {
		// Could happen if someone creates a new named type in their package. For example:
		//   type MyMsg pb.M
		return nil, false
	}

	msgType := types.NewPointer(c.typeOf(sel.Sel)) // *pb.M
	builder := &dst.SelectorExpr{
		// Clone the selector in case we might duplicate it when we rewrite:
		//
		//  []*pb.M{{F:nil}, {F:nil}}
		//
		// to:
		//
		//  []*pb.M{
		//    pb.M_builder{F:nil}.Build(),
		//    pb.M_builder{F:nil}.Build(),
		//  }
		X:   cloneSelectorExpr(c, sel).X,
		Sel: &dst.Ident{Name: sel.Sel.Name + "_builder"},
	}
	pkg := c.objectOf(sel.Sel).Pkg()
	builderType := types.NewNamed( // pb.M_builder in the same package as pb.M
		types.NewTypeName(token.NoPos, pkg, sel.Sel.Name+"_builder", nil),
		types.NewStruct(nil, nil),
		nil)
	builderType.AddMethod(types.NewFunc(token.NoPos, pkg, "Build", types.NewSignature( // func (pb.M_Builder) Build() *pb.M
		types.NewParam(token.NoPos, pkg, "_", builderType),
		types.NewTuple(),
		types.NewTuple(types.NewParam(token.NoPos, pkg, "_", msgType)),
		false)))
	c.setType(builder, builderType)
	updateASTMap(c, typ, builder)
	c.setType(builder.Sel, builderType)

	builderLit := &dst.CompositeLit{
		Type: builder,
		Elts: lit.Elts,
	}
	c.setType(builderLit, builderType) // pb.M_builder{...} has the same type as pb.M_builder
	updateASTMap(c, typ, builderLit)

	// pb.M_builder{...}.Build is the only pb.M_builder method.
	fun := &dst.SelectorExpr{
		X:   builderLit,
		Sel: &dst.Ident{Name: "Build"},
	}
	c.setType(fun, builderType.Method(0).Type())
	c.setType(fun.Sel, builderType.Method(0).Type())
	updateASTMap(c, typ, fun)

	// pb.M_builder{...}.Build() returns *pb.M
	buildCall := &dst.CallExpr{Fun: fun}
	c.setType(buildCall, msgType)
	updateASTMap(c, typ, buildCall)

	// Update decorations (comments and whitespace).
	builderLit.Decs = lit.Decs
	builderLit.Decs.After = dst.None
	builderLit.Decs.End = nil

	decs := dst.NodeDecs{
		After: lit.Decs.After,
		End:   lit.Decs.End,
	}
	if b := parentDecs.Before; b != dst.None {
		decs.Before = b
	}
	decs.Start = append(parentDecs.Start, decs.Start...)
	decs.End = append(decs.End, parentDecs.End...)
	if a := parentDecs.After; a != dst.None {
		decs.After = a
	}
	buildCall.Decs.NodeDecs = decs

	return buildCall, true
}

func isSlice(t types.Type) bool {
	_, ok := t.Underlying().(*types.Slice)
	return ok
}
