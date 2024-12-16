// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"fmt"
	"reflect"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func cloneSelectorExpr(c *cursor, src *dst.SelectorExpr) *dst.SelectorExpr {
	out := dst.Clone(src).(*dst.SelectorExpr)

	// Match type information for all subexpressions to corresponding type
	// information from the source.
	//
	// Call c.setType for all dst.Expr.
	// Call c.setUse for all *dst.Ident.
	var cloneTypes func(from, to reflect.Value)
	cloneTypes = func(from, to reflect.Value) {
		if !from.IsValid() || !to.IsValid() || from.IsZero() || to.IsZero() || isNil(from) || isNil(to) {
			return
		}
		// from and to are clones so we assume that they are structurally
		// similar. Unfortunately, they are not actually the same as dst.Clone
		// loses (for example) information about Object references.
		if from.Kind() != to.Kind() { // A cheap sanity check.
			panic(fmt.Sprintf("bad kind %s vs %s (%+v vs %+v)", from.Kind(), to.Kind(), from, to))
		}
		switch from.Kind() {
		case reflect.Array, reflect.Slice:
			for i := 0; i < from.Len(); i++ {
				cloneTypes(from.Index(i), to.Index(i))
			}
		case reflect.Interface:
			if from.IsNil() {
				return
			}
			cloneTypes(from.Elem(), to.Elem())
		case reflect.Map:
			// The DST package doesn't use maps for anything significant here.
			panic("map is not supported")
		case reflect.Ptr:
			if from.IsNil() {
				return
			}
			cloneTypes(from.Elem(), to.Elem())
		case reflect.Struct:
			for i := 0; i < from.NumField(); i++ {
				cloneTypes(from.Field(i), to.Field(i))
			}
		}
		if from.Type().AssignableTo(dstExprType) && from.Kind() != reflect.Interface {
			c.setType(to.Interface().(dst.Expr), c.typeOf(from.Interface().(dst.Expr)))
		}
		if from.Type() == dstIdentType {
			// If the source identifier has an unknown use then don't try to
			// propagate it to the new identifier. This could happen for manually
			// created dst.Ident and we are currently really bad at maintaining
			// that use information.
			if use := c.objectOf(from.Interface().(*dst.Ident)); use != nil {
				c.setUse(to.Interface().(*dst.Ident), use)
			}
		}
	}
	cloneTypes(reflect.ValueOf(src), reflect.ValueOf(out))
	updateAstMapTree(c, src, out)
	return out
}

func cloneIdent(c *cursor, ident *dst.Ident) *dst.Ident {
	out := &dst.Ident{
		Name: ident.Name,
		Path: ident.Path,
	}
	c.setType(out, c.typeOf(ident))
	if use := c.objectOf(ident); use != nil {
		c.setUse(out, use)
	}
	updateASTMap(c, ident, out)
	return out
}

// cloneSelectorCallExpr clones call expression of the form "m.F()"
func cloneSelectorCallExpr(c *cursor, src *dst.CallExpr) *dst.CallExpr {
	sel, ok := src.Fun.(*dst.SelectorExpr)
	if !ok {
		panic("invalid cloneSelectorCallExpr call: src.Fun must be a selector")
	}
	call := &dst.CallExpr{
		Fun: cloneSelectorExpr(c, sel),
	}
	c.setType(call, c.typeOf(src))
	return call
}

// cloneIndexExpr clones call expression of the form "m.F()"
func cloneIndexExpr(c *cursor, src *dst.IndexExpr) *dst.IndexExpr {
	idx := &dst.IndexExpr{
		X:     cloneExpr(c, src.X),
		Index: cloneExpr(c, src.Index),
	}
	c.setType(idx.X, c.typeOf(src.X))
	c.setType(idx.Index, c.typeOf(src.Index))
	c.setType(idx, c.typeOf(src))
	return idx
}

// cloneIndexExpr clones call expression of the form "m.F()"
func cloneBasicLit(c *cursor, src *dst.BasicLit) *dst.BasicLit {
	bl := &dst.BasicLit{
		Kind:  src.Kind,
		Value: src.Value,
	}
	c.setType(bl, c.typeOf(src))
	return bl
}

// cloneExpr dispatches to any of the functions above.
func cloneExpr(c *cursor, src dst.Expr) dst.Expr {
	switch v := src.(type) {
	case *dst.Ident:
		return cloneIdent(c, v)
	case *dst.IndexExpr:
		return cloneIndexExpr(c, v)
	case *dst.BasicLit:
		return cloneBasicLit(c, v)
	case *dst.CallExpr:
		return cloneSelectorCallExpr(c, v)
	case *dst.SelectorExpr:
		return cloneSelectorExpr(c, v)
	default:
		panic(fmt.Sprintf("unhandled type for cloneExpr %T", src))
	}
}

func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// updateAstMapTree updates a the astMap for a cloned dst tree. This function
// assumes that source and copy are exact copies of each other. After this
// function each node in copy will be mapped to the same ast node as the
// associated node in source.
func updateAstMapTree(c *cursor, source, copy dst.Node) {
	var nodes []dst.Node
	dstutil.Apply(source, func(cur *dstutil.Cursor) bool {
		nodes = append(nodes, cur.Node())
		return true
	}, nil)
	cnt := 0
	dstutil.Apply(copy, func(cur *dstutil.Cursor) bool {
		updateASTMap(c, nodes[cnt], cur.Node())
		cnt++
		return true
	}, nil)
}

func updateASTMap(c *cursor, source dst.Node, copy dst.Node) {
	// Retain the mapping between dave/dst and go/ast nodes.
	// This is necessary for looking up position information.
	// The cloned ident will not be at the same position as
	// its source, but it will be close enough for our purposes.
	if n, ok := c.typesInfo.astMap[source]; ok {
		c.typesInfo.astMap[copy] = n
	}
}
