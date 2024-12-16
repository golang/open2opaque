// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"context"
	"go/token"
	"go/types"
	"sort"

	"github.com/dave/dst"
	"google.golang.org/open2opaque/internal/o2o/loader"
	"google.golang.org/open2opaque/internal/protodetecttypes"
)

// generateOneofBuilderCases transforms oneofs in composite literals to be
// compatible with builders that don't have a field for the oneof but a field
// for each case instead. The RHS is either a direct field access or getter
// oneof field. The only other values that could be assigned to the oneof field
// would be the wrapper types which must be handled before this function is
// called.
//
//		 &pb.M{
//	    OneofField: m2.OneofField,
//		 }
//		 =>
//		 &pb.M{
//		   OneofField: m2.OneofField,
//		   BytesOneof: m2.GetBytesOneof(),
//		   IntOneof: func(msg *pb2.M2) *int64 {
//		   	if !msg.HasIntOneof() {
//		   		return nil
//		   	}
//		   	return proto.Int64(msg.GetIntOneof())
//		   }(m2),
//		 }
//
// Removal of the oneof KeyValueExpr happens later.
// Changing from &pb.M to pb.M_builder{...}.Build() happens in another step.
//
// Note: The exact results differ and depend on the valid cases for the oneof
// field. For scalar fields, we have to use self invoking function literals to
// be able to check if the case is unset and pass nil to the builder field if
// so.
func generateOneofBuilderCases(c *cursor, updates []func(), lit *dst.CompositeLit, kv *dst.KeyValueExpr) ([]func(), bool) {
	t := c.typesInfo.typeOf(kv.Value)
	if !isOneof(t) {
		c.Logf("returning: not a oneof field")
		return nil, false
	}
	c.Logf("try generating oneof cases...")
	// First we need to collect an exhaustive list of alternatives. We do this
	// by collecting all wrapper types for the given field that are defined in
	// the generated proto package.
	nt, ok := t.(*types.Named)
	if !ok {
		c.Logf("ignoring oneof of type %T (looking for *types.Named)", c.Node())
		return nil, false
	}
	pkg := nt.Obj().Pkg()

	targetID := pkg.Path()

	p, err := loader.LoadOne(context.Background(), c.loader, &loader.Target{ID: targetID})
	if err != nil {
		c.Logf("failed to load proto package %q: %v", pkg.Path(), err)
		return nil, false
	}

	// Get the message name from which the oneof field is taken
	rhsSel, ok := kv.Value.(*dst.SelectorExpr)
	if !ok {
		// If it is not a direct field access, it must be a call expression,
		// i.e. the getter for the oneof field.
		// For now we don't support anything else (e.g. variables).
		callExpr, ok := kv.Value.(*dst.CallExpr)
		if !ok {
			c.Logf("ignoring value %T (looking for SelectorExpr or CallExpr)", kv.Value)
			return nil, false
		}
		rhsSel, ok = callExpr.Fun.(*dst.SelectorExpr)
		if !ok {
			c.Logf("ignoring value %T (looking for SelectorExpr)", callExpr.Fun)
			return nil, false
		}
	}

	// Oneofs are guaranteed to have *types.Interface as underlying type.
	it := t.Underlying().(*types.Interface)
	type typeAndName struct {
		name string
		typ  *types.Struct
	}
	// Collect wrapper types to generate an exhaustive list of cases
	var wrapperTypes []typeAndName
	for _, idt := range p.TypeInfo.Defs {
		// Some things (like `package p`) are toplevel definitions that don't
		// have an associated object.
		if idt == nil {
			continue
		}
		// All wrapper types are exported *types.Named
		if !idt.Exported() {
			continue
		}
		nt, ok := idt.Type().(*types.Named)
		if !ok {
			continue
		}
		if !types.Implements(types.NewPointer(nt), it) {
			continue
		}
		// All wrapper types are structs with exactly one field.
		// The one field is named that same as the case itself.
		st := nt.Underlying().(*types.Struct)
		name := st.Field(0).Name()
		wrapperTypes = append(wrapperTypes, typeAndName{name, st})
	}

	sort.Slice(wrapperTypes, func(i, j int) bool {
		return wrapperTypes[i].name < wrapperTypes[j].name
	})

	first := true
	for _, tan := range wrapperTypes {
		caseName := tan.name

		// Generate Value
		st := tan.typ
		if st.NumFields() != 1 {
			c.Logf("wrapper type has %d fields (expected 1)", st.NumFields())
			return nil, false
		}
		fieldType := st.Field(0).Type()

		var rhsExpr dst.Expr
		if _, ok = fieldType.Underlying().(*types.Basic); ok {
			rhsExpr = funcLiteralForOneofField(c, rhsSel, caseName, fieldType)
		} else {
			rhsExpr = oneOfSelector(c, "Get", caseName, rhsSel.X, fieldType, nil, *rhsSel.Decorations())
		}

		// Generate KeyValueExpr
		nKey := &dst.Ident{Name: caseName}
		c.setType(nKey, types.NewPointer(fieldType))
		nKeyVal := &dst.KeyValueExpr{
			Key:   nKey,
			Value: rhsExpr,
		}
		// Duplicate the decorations to all children.
		if first {
			nKeyVal.Decs = kv.Decs
			first = false
		}
		nKeyVal.Decorations().After = dst.NewLine
		c.setType(nKeyVal, types.Typ[types.Invalid])
		c.Logf("generated KeyValueExpr for %v", caseName)
		updates = append(updates, func() {
			lit.Elts = append(lit.Elts, nKeyVal)
		})
	}

	c.Logf("generated %d oneof cases", len(wrapperTypes))
	return updates, true
}

// funcLiteralForOneofField is similar to funcLiteralForHas but does not operate
// on existing (Go struct) fields but on proto message fields. Such fields don't
// exist in the Go type system and thus we cannot reuse funcLiteralForHas.
func funcLiteralForOneofField(c *cursor, field *dst.SelectorExpr, fieldName string, fieldType types.Type) *dst.CallExpr {
	msg := field.X.(*dst.Ident)
	msgType := c.typeOf(msg)

	getSel := &dst.Ident{
		Name: "Get" + fixConflictingNames(msgType, "Get", fieldName),
	}
	getX := cloneIdent(c, msg)
	getCall := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   getX,
			Sel: getSel,
		},
	}
	value := types.NewParam(token.NoPos, nil, "_", fieldType)
	recv := types.NewParam(token.NoPos, nil, "_", c.underlyingTypeOf(msg))
	c.setType(getSel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(value), false))
	c.setType(getCall.Fun, c.typeOf(getSel))
	c.setType(getCall, fieldType)

	hasSel := &dst.Ident{
		Name: "Has" + fixConflictingNames(msgType, "Has", fieldName),
	}
	hasX := &dst.Ident{Name: "msg"}
	c.setType(hasX, types.Typ[types.Invalid])
	hasCall := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   hasX,
			Sel: hasSel,
		},
	}
	c.setType(hasSel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(types.NewParam(token.NoPos, nil, "_", types.Typ[types.Bool])), false))
	c.setType(hasCall.Fun, c.typeOf(hasSel))
	c.setType(hasCall, types.Typ[types.Bool])

	// oneof fields are synthetically generated, so there are no node
	// decorations to carry over.
	var emptyDecs dst.NodeDecs
	if c.isSideEffectFree(msg) {
		// Call proto.ValueOrNil() directly, no function literal needed.
		hasX.Name = msg.Name
		field2 := cloneSelectorExpr(c, field)
		field2.Sel.Name = fieldName
		return valueOrNil(c, hasCall, field2, emptyDecs)
	}

	var retElemType dst.Expr = &dst.Ident{Name: fieldType.String()}
	fieldTypePtr := types.NewPointer(fieldType)
	c.setType(retElemType, fieldType)
	retType := &dst.StarExpr{X: retElemType}
	c.setType(retType, fieldTypePtr)

	msgTypePtr := types.NewPointer(msgType)
	msgParamSel := c.selectorForProtoMessageType(msgType)
	msgParamType := &dst.StarExpr{X: msgParamSel}
	c.setType(msgParamType, msgTypePtr)

	msgParam := &dst.Ident{Name: "msg"}
	c.setType(msgParam, types.Typ[types.Invalid])

	field2 := cloneSelectorExpr(c, field)
	field2.Sel.Name = fieldName
	funcLit := &dst.FuncLit{
		// func(msg *pb.M2) <type> {
		Type: &dst.FuncType{
			Params: &dst.FieldList{
				List: []*dst.Field{
					&dst.Field{
						Names: []*dst.Ident{msgParam},
						Type:  msgParamType,
					},
				},
			},
			Results: &dst.FieldList{
				List: []*dst.Field{
					&dst.Field{
						Type: retType,
					},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				// return proto.ValueOrNil(…)
				&dst.ReturnStmt{
					Results: []dst.Expr{
						valueOrNil(c, hasCall, field2, emptyDecs),
					},
				},
			},
		},
	}
	// We do not know whether the proto package was imported, so we may not be
	// able to construct the correct type signature. Set the type to invalid,
	// like we do for any code involving the proto package.
	c.setType(funcLit, types.Typ[types.Invalid])
	c.setType(funcLit.Type, types.Typ[types.Invalid])

	msgArg := dst.Clone(msg).(dst.Expr)
	c.setType(msgArg, msgType)
	call := &dst.CallExpr{
		Fun: funcLit,
		Args: []dst.Expr{
			msgArg,
		},
	}
	c.setType(call, fieldTypePtr)

	return call
}

// This is like sel2call but for oneof fields which are not actual (Go struct)
// fields in the generated Go struct and thus we cannot use sel2call directly
func oneOfSelector(c *cursor, prefix, fieldName string, msg dst.Expr, fieldType types.Type, val dst.Expr, decs dst.NodeDecs) *dst.CallExpr {
	name := fixConflictingNames(c.typeOf(msg), prefix, fieldName)
	fnsel := &dst.Ident{
		Name: prefix + name,
	}
	selX := dst.Clone(msg).(dst.Expr)
	c.setType(selX, types.Typ[types.Invalid])
	fn := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   selX,
			Sel: fnsel,
		},
	}
	if val != nil {
		fn.Args = []dst.Expr{val}
	}

	value := types.NewParam(token.NoPos, nil, "_", fieldType)
	recv := types.NewParam(token.NoPos, nil, "_", c.typeOf(msg))
	switch prefix {
	case "Get":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(value), false))
		c.setType(fn, fieldType)
	case "Set":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(value), types.NewTuple(), false))
		c.setVoidType(fn)
	case "Clear":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(), false))
		c.setVoidType(fn)
	case "Has":
		c.setType(fnsel, types.NewSignature(recv, types.NewTuple(), types.NewTuple(types.NewParam(token.NoPos, nil, "_", types.Typ[types.Bool])), false))
		c.setType(fn, types.Typ[types.Bool])
	default:
		panic("bad function name prefix '" + prefix + "'")
	}
	c.setType(fn.Fun, c.typeOf(fnsel))
	return fn
}

// destructureOneofWrapper returns K (field name), V (assigned value), and
// typeof(K) (type of the field) for a oneof wrapper expression
//
//	"&OneofWrapper{K: V}"
//
// and equivalents that omit either K, or V, or both.
func destructureOneofWrapper(c *cursor, x dst.Expr) (string, types.Type, dst.Expr, *dst.NodeDecs, bool) {
	c.Logf("destructuring one of wrapper")
	ue, ok := x.(*dst.UnaryExpr)
	if !ok || ue.Op != token.AND {
		return oneofWrapperSelector(c, x)
	}
	clit, ok := ue.X.(*dst.CompositeLit)
	if !ok {
		return oneofWrapperSelector(c, x)
	}
	s, ok := c.underlyingTypeOf(clit).(*types.Struct)
	if !ok || s.NumFields() != 1 {
		return oneofWrapperSelector(c, x)
	}
	if !isOneofWrapper(c, clit) {
		return oneofWrapperSelector(c, x)
	}
	var decs *dst.NodeDecs
	var val dst.Expr
	switch {
	case len(clit.Elts) > 1:
		panic("oneof wrapper clit has multiple elements")
	case len(clit.Elts) == 0: // &Oneof{}
		val = nil
	case isKV(clit.Elts[0]): // &Oneof{K: V}
		// We are about to replace clit.Elts[0], so propagate its decorations.
		decs = clit.Elts[0].Decorations()
		val = clit.Elts[0].(*dst.KeyValueExpr).Value
	default: // &Oneof{V}
		val = clit.Elts[0]
		// Propagate the decorations and clear them at the value level.
		var decsCopy dst.NodeDecs
		decsCopy = *val.Decorations()
		decs = &decsCopy
		val.Decorations().Before = dst.None
		val.Decorations().After = dst.None
		val.Decorations().Start = nil
		val.Decorations().End = nil
	}
	if ident, ok := val.(*dst.Ident); ok && ident.Name == "nil" {
		val = nil
	}

	valType := s.Field(0).Type()
	if isBytes(valType) && !isNeverNilSliceExpr(c, val) {
		if !c.lvl.ge(Yellow) {
			c.numUnsafeRewritesByReason[IncompleteRewrite]++
			c.Logf("ignoring: rewrite level smaller than Yellow")
			return "", nil, nil, nil, false
		}
		if val == nil {
			id := &dst.Ident{
				Name: "byte",
			}
			c.setType(id, valType.(*types.Slice).Elem())
			typ := &dst.ArrayType{
				Elt: id,
			}
			c.setType(typ, valType)
			val = &dst.CompositeLit{
				Type: typ,
			}
			c.setType(val, valType)
			return s.Field(0).Name(), valType, val, decs, true
		}
		c.numUnsafeRewritesByReason[MaybeOneofChange]++
		// NOTE(lassefolger): This ValueOrDefaultBytes() call is only
		// necessary in builders, but we don’t have enough context in
		// this part of the code to omit it for setters.
		return s.Field(0).Name(), valType, valueOrDefault(c, "ValueOrDefaultBytes", val), decs, true
	}
	isMsgOneof := false
	if ptr, ok := valType.(*types.Pointer); ok && (protodetecttypes.Type{T: ptr.Elem()}.IsMessage()) {
		isMsgOneof = true
	}
	if isMsgOneof && val != nil && !isNeverNilExpr(c, val) {
		if !c.lvl.ge(Yellow) {
			c.numUnsafeRewritesByReason[IncompleteRewrite]++
			c.Logf("ignoring: rewrite level smaller than Yellow")
			return "", nil, nil, nil, false
		}
		c.numUnsafeRewritesByReason[MaybeOneofChange]++
		return s.Field(0).Name(), valType, valueOrDefault(c, "ValueOrDefault", val), decs, true
	}

	return s.Field(0).Name(), valType, val, decs, true
}

func oneofWrapperSelector(c *cursor, x dst.Expr) (string, types.Type, dst.Expr, *dst.NodeDecs, bool) {
	var decs *dst.NodeDecs
	if !c.lvl.ge(Yellow) {
		c.Logf("ignoring: rewrite level smaller than Yellow")
		return "", nil, nil, nil, false
	}
	if !isOneofWrapper(c, x) {
		c.Logf("ignoring: not a one of wrapper")
		return "", nil, nil, nil, false
	}
	if !isNeverNilExpr(c, x) {
		if c.lvl.le(Yellow) {
			c.Logf("ignoring: potential nil expression and rewrite level not Red")
			// This could be handled with self calling func literals but
			// we should only do so if there is a significant number of
			// locations that need this.
			c.numUnsafeRewritesByReason[IncompleteRewrite]++
			return "", nil, nil, nil, false
		}
		c.numUnsafeRewritesByReason[MaybeNilPointerDeref]++
	}
	// It is possible that this rewrite unsets the oneof field where it was
	// previously set to a type but without value (which is not a valid
	// proto message), e.g.:
	//
	//  m.OneofField = pb.OneofWrapper{nil}
	c.numUnsafeRewritesByReason[MaybeOneofChange]++

	t := c.underlyingTypeOf(x)
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem().Underlying()
	}
	s := t.(*types.Struct)
	val := &dst.SelectorExpr{
		X: x,
		Sel: &dst.Ident{
			Name: s.Field(0).Name(),
		},
	}
	valType := s.Field(0).Type()
	c.setType(val.Sel, valType)
	c.setType(val, s.Field(0).Type())
	if ptr, ok := valType.(*types.Pointer); ok && (protodetecttypes.Type{T: ptr.Elem()}.IsMessage()) {
		return s.Field(0).Name(), valType, valueOrDefault(c, "ValueOrDefault", val), decs, true
	}
	return s.Field(0).Name(), s.Field(0).Type(), val, decs, true
}

func isKV(x dst.Expr) bool {
	_, ok := x.(*dst.KeyValueExpr)
	return ok
}

const protooneofdefaultImport = "google.golang.org/protobuf/protooneofdefault"

func valueOrDefault(c *cursor, fun string, val dst.Expr) dst.Expr {
	fnsel := &dst.Ident{Name: fun}
	fn := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: c.imports.name(protooneofdefaultImport)},
			Sel: fnsel,
		},
		Args: []dst.Expr{
			val,
		},
	}

	t := c.underlyingTypeOf(val)
	value := types.NewParam(token.NoPos, nil, "_", t)
	c.setType(fnsel, types.NewSignature(nil, types.NewTuple(value), types.NewTuple(value), false))
	c.setType(fn, t)

	c.setType(fn.Fun, c.typeOf(fnsel))

	// We set the type for "proto" identifier to Invalid because that's consistent with what the
	// typechecker does on new code. We need to distinguish "invalid" type from "no type was
	// set" as the code panics on the later in order to catch issues with missing type updates.
	c.setType(fn.Fun.(*dst.SelectorExpr).X, types.Typ[types.Invalid])
	return fn
}
