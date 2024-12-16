// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	log "github.com/golang/glog"
	"google.golang.org/open2opaque/internal/o2o/statsutil"
	"google.golang.org/open2opaque/internal/protodetecttypes"

	spb "google.golang.org/open2opaque/internal/dashboard"
)

// stats lists uses of protocol buffer types that are interesting from analysis
// standpoint. This function is called after all rewrites for level c.lvl are
// applied on the file. If generated is true, then Location.IsGeneratedFile
// will be set to true on the returned entries.
func stats(c *cursor, f *dst.File, generated bool) []*spb.Entry {
	// Temporarily disable stats after rewrites. Only calculate for code before our changes.
	if c.lvl != None {
		return nil
	}
	parents := map[dst.Node]dst.Node{}
	var out []*spb.Entry
	dstutil.Apply(f, func(cur *dstutil.Cursor) bool {
		n := cur.Node()
		parents[n] = cur.Parent()

		switch x := n.(type) {
		case nil:
			// ApplyFunc can be called with a nil node, for example, when processing a
			// function declaration
			return false
		case *dst.BadStmt, *dst.BadExpr, *dst.BadDecl, *dst.ImportSpec:
			// Don't recurse into nodes that can't refer to proto types.
			return false
		case *dst.GenDecl:
			// Only recurse into declarations that can reference protos.
			return x.Tok != token.IMPORT
		}

		switch n := n.(type) {
		case *dst.SelectorExpr:
			out = append(out, selectorStats(c, n, cur.Parent())...)
		case *dst.CallExpr:
			out = append(out, callStats(c, n, cur.Parent())...)
		case *dst.AssignStmt:
			out = append(out, assignStats(c, n, cur.Parent())...)
		case *dst.CompositeLit:
			out = append(out, compositeLitStats(c, n, cur.Parent())...)
		case *dst.SendStmt:
			out = append(out, sendStats(c, n, cur.Parent())...)
		case *dst.ReturnStmt:
			out = append(out, returnStats(c, n, cur.Parent(), parents)...)
		case *dst.TypeAssertExpr:
			out = append(out, typeAssertStats(c, n, cur.Parent())...)
		case *dst.TypeSpec:
			out = append(out, typeSpecStats(c, n, cur.Parent())...)
		case *dst.StructType:
			out = append(out, structStats(c, n, cur.Parent())...)
		}
		return true
	}, nil)
	if generated {
		for _, e := range out {
			e.GetLocation().IsGeneratedFile = true
		}
	}
	return out
}

func logSiloed(c *cursor, out []*spb.Entry, n, parent dst.Node) []*spb.Entry {
	return append(out, &spb.Entry{
		Status: &spb.Status{
			Type:  spb.Status_FAIL,
			Error: "type information missing; are dependencies in a silo?",
		},
		Location: location(c, n),
		Level:    toRewriteLevel(c.lvl),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", n),
			ParentType: fmt.Sprintf("%T", parent),
		},
	})
}

// logConversion appends an entry to the out slice if the conversion is an
// interesting one.
//
// Interesting conversions include all conversions that can make the open2opaque
// migration harder (e.g. converting protos to interface{}, unsafe.Pointer,
// etc.).
//
// Exactly one of srcExpr and src must be set. Those determine the
// type/expression that is being converted to type dst.
//
// The nodes n and parent build the context for the conversion. Parent should be
// the parent node of n in the AST.
//
// The use specifies the reason for the conversion (is it an implicit conversion
// to a function argument? explicit conversion in assignment? etc.).
func logConversion(c *cursor, out []*spb.Entry, srcExpr dst.Expr, src, dst types.Type, n, parent dst.Node, use *spb.Use) []*spb.Entry {
	if (srcExpr != nil) == (src != nil) {
		panic(fmt.Sprintf("logConversion: either srcExpr or src must set, but not both (srcExpr!=nil: %t, src!=nil: %t)", srcExpr != nil, src != nil))
	}
	if src == nil {
		src = c.typeOfOrNil(srcExpr)
	}
	if src == nil {
		return logSiloed(c, out, n, parent)
	}
	if !isInterfaceType(dst) {
		return out
	}
	t, ok := c.shouldLogCompositeType(src, true)
	if !ok {
		return out
	}
	return append(out, &spb.Entry{
		Location: location(c, n),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(t),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", n),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: use,
	})
}

// logShallowCopy appends an entry to the out slice if the expression is a
// shallow copy.
//
// Exactly one of srcExpr and src must be set. Those determine the
// type/expression that is being converted to type dst.
//
// The nodes n and parent build the context for the conversion. Parent should be
// the parent node of n in the AST.
//
// The use specifies the reason for the conversion (is it an implicit conversion
// to a function argument? explicit conversion in assignment? etc.).
func logShallowCopy(c *cursor, out []*spb.Entry, srcExpr dst.Expr, src types.Type, n, parent dst.Node, use *spb.Use) []*spb.Entry {
	if (srcExpr != nil) == (src != nil) {
		panic(fmt.Sprintf("logShallowCopy: either srcExpr or src must set, but not both (srcExpr!=nil: %t, src!=nil: %t)", srcExpr != nil, src != nil))
	}
	if src == nil {
		src = c.typeOfOrNil(srcExpr)
	}
	if src == nil {
		return logSiloed(c, out, n, parent)
	}

	if _, isPtr := src.Underlying().(*types.Pointer); isPtr {
		return out
	}
	t, ok := c.shouldLogCompositeType(src, false)
	if !ok {
		return out
	}

	return append(out, &spb.Entry{
		Location: location(c, n),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(t),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", n),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: use,
	})
}

// shouldLogCompositeType returns true if an expression should be logged when
// the composite type t is part of the expression.
//
// followPointers determines whether pointer types should be traversed when
// looking for the result. This is mainly useful for finding shallow copies.
//
// If shouldLogCompositeType returns true, then it also returns the reason why
// the type should be logged (i.e. the actual proto type referenced by the
// composite type t).
//
// For example, the following expression:
//
//	f(struct{   // assume: func f(interface{}) { }
//		 m *pb.M2,
//	}{
//		 m: m2,
//	})
//
// should be logged (if *pb.M2 is a proto type that should be tracked) because a
// value of type *pb.M2 is converted to interface{}.
//
// The following expression should not be tracked:
//
//	f(struct{   // assume: func f(interface{}) { }
//	 	ch chan *pb.M2,
//	}{
//	 	ch: make(chan *pb.M2),
//	})
//
// because it does not contain values of type *pb.M2. Only references to the
// type. (what should/shouldn't be tracked is a somewhat arbitrary choice).
//
// This functionality exists to statically track potential Go reflect usage on
// protos and shallow copies.
//
// NOTE: one could argue that it would be more accurate to track all references
// to proto types, not only those associated with values in expressions
// (e.g. the channel example above). We've tried that and the number of findings
// (false positives) is so large that the statistic becomes meaningless.
func (c *cursor) shouldLogCompositeType(t types.Type, followPointers bool) (out types.Type, _ bool) {
	// We use a cache for two reasons:
	//   - (major) to handle cyclic data structures
	//   - (minor) to improve performance
	//
	// Consider type:
	//
	//   type T struct {
	//     F *T
	//   }
	//
	// and shouldLogCompositeType call with T and followPointers=true. We
	// don't want a recursive call shouldLogCompositeType for the field F
	// after dereferencing the pointer as that would result in an infinite
	// recursion.
	//
	// At the time we see T for the first time, we don't know if it will
	// contain pointers or not, but we have to signal not to process T
	// again. Our approach involves three states for T in the cache:
	//
	//   1. an empty cache for T means that we've never seen this type (and
	//      hence should process it)
	//
	//   2. a cache with a nil value for T means either that:
	//
	//      - we're currently processing the type. We don't know whether it
	//        depends on protos or not, but we know that we shouldn't start
	//        processing it again; or
	//
	//      - we've processed the type before and it doesn't reference
	//        protos. We can return nil as the result.
	//
	//   3. a cache with a non-nil value for T means that we've processed
	//      the type before and that we can simply return the result.
	//
	// The result of shouldLogCompositeType can be different for a single
	// type, based on the followPointers value. Therefore, we have two
	// caches: one for each possdible value of followPointers. Technically
	// followPointers=false implies that there should be no infinite
	// recursion (in the current version of shouldLogCompositeType) but we
	// want to keep the code symmetric.
	cache := c.shouldLogCompositeTypeCache
	if followPointers {
		cache = c.shouldLogCompositeTypeCacheNoPtr
	}

	if cached := cache.At(t); cached != nil {
		ce := cached.(*cacheEntry)
		return ce.protoType, ce.protoType != nil
	}

	cache.Set(t, &cacheEntry{})

	defer func() {
		cache.Set(t, &cacheEntry{protoType: out})
	}()

	if _, isPtr := t.Underlying().(*types.Pointer); !followPointers && isPtr {
		return nil, false
	}
	if c.shouldTrackType(t) {
		return t, true
	}
	switch t := t.(type) {
	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			if t, ok := c.shouldLogCompositeType(t.At(i).Type(), followPointers); ok {
				return t, true
			}
		}
		return nil, false
	case *types.Interface:
		// The interface could contain a proto, however we would've caught
		// conversion to the interface type.
		return nil, false
	case *types.Named:
		return c.shouldLogCompositeType(t.Underlying(), followPointers)
	case *types.Pointer:
		if !followPointers {
			return nil, false
		}
		return c.shouldLogCompositeType(t.Elem(), followPointers)
	case *types.Signature:
		return nil, false
	case *types.TypeParam:
		return nil, false
	case *types.Slice:
		if !followPointers {
			return nil, false
		}
		return c.shouldLogCompositeType(t.Elem(), followPointers)
	case *types.Array:
		return c.shouldLogCompositeType(t.Elem(), followPointers)
	case *types.Basic:
		return nil, false
	case *types.Chan:
		return nil, false
	case *types.Map:
		if !followPointers {
			return nil, false
		}
		if t, ok := c.shouldLogCompositeType(t.Key(), followPointers); ok {
			return t, true
		}
		if t, ok := c.shouldLogCompositeType(t.Elem(), followPointers); ok {
			return t, true
		}
		return nil, false
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if t, ok := c.shouldLogCompositeType(t.Field(i).Type(), followPointers); ok {
				return t, true
			}
		}
		return nil, false
	case *types.Alias:
		return c.shouldLogCompositeType(types.Unalias(t), followPointers)
	default:
		panic(fmt.Sprintf("unrecognized type %T", t))
	}
}

// convToUnsafePointerArg returns the x expression in unsafe.Pointer(x)
// conversion if expr has the form unsafe.Pointer(x).
func convToUnsafePointerArg(c *cursor, expr dst.Expr) (dst.Expr, bool) {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return nil, false
	}
	if sel, ok := call.Fun.(*dst.SelectorExpr); !ok || c.objectOf(sel.Sel) != types.Unsafe.Scope().Lookup("Pointer") {
		return nil, false
	}
	return call.Args[0], true
}

func typeName(t types.Type) string {
	if iface, ok := t.(*types.Interface); ok && iface.Empty() {
		return "interface{}"
	}
	return t.String()
}

func isInterfaceType(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

func hasInterfaceType(c *cursor, expr dst.Expr) bool {
	if ident, ok := expr.(*dst.Ident); ok && ident.Name == "_" {
		return false
	}
	return isInterfaceType(c.typeOf(expr))
}

func location(c *cursor, n dst.Node) *spb.Location {
	astNode := c.typesInfo.astMap[n]
	if astNode == nil {
		return nil
	}
	start := c.pkg.Fileset.Position(astNode.Pos())
	end := c.pkg.Fileset.Position(astNode.End())
	return &spb.Location{
		Package: c.pkg.TypePkg.Path(),
		File:    start.Filename,
		Start: &spb.Position{
			Line:   int64(start.Line),
			Column: int64(start.Column),
		},
		End: &spb.Position{
			Line:   int64(end.Line),
			Column: int64(end.Column),
		},
	}
}

func toRewriteLevel(lvl Level) spb.RewriteLevel {
	switch lvl {
	case None:
		return spb.RewriteLevel_NONE
	case Green:
		return spb.RewriteLevel_GREEN
	case Yellow:
		return spb.RewriteLevel_YELLOW
	case Red:
		return spb.RewriteLevel_RED
	default:
		panic("unrecognized fix.Level %s" + lvl)
	}
}

func toTypeProto(typ types.Type) *spb.Type {
	return statsutil.ShortAndLongNameFrom(typ.String())
}

func selectorStats(c *cursor, sel *dst.SelectorExpr, parent dst.Node) []*spb.Entry {
	if _, ok := c.trackedProtoFieldSelector(sel); ok {
		return []*spb.Entry{&spb.Entry{
			Location: location(c, sel),
			Level:    toRewriteLevel(c.lvl),
			Type:     toTypeProto(c.typeOf(sel.X)),
			Expr: &spb.Expression{
				Type:       fmt.Sprintf("%T", sel),
				ParentType: fmt.Sprintf("%T", parent),
			},
			Use: &spb.Use{
				Type: spb.Use_DIRECT_FIELD_ACCESS,
				DirectFieldAccess: &spb.FieldAccess{
					FieldName: sel.Sel.Name,
					FieldType: toTypeProto(c.typeOf(sel.Sel)),
				},
			},
		}}
	}
	t := c.typeOfOrNil(sel.X)
	if t == nil {
		return logSiloed(c, nil, sel, parent)
	}
	if !c.shouldTrackType(t) || !strings.HasPrefix(sel.Sel.Name, "XXX_") {
		return nil
	}
	return []*spb.Entry{&spb.Entry{
		Location: location(c, sel),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(t),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", sel),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: &spb.Use{
			Type: spb.Use_INTERNAL_FIELD_ACCESS,
			InternalFieldAccess: &spb.FieldAccess{
				FieldName: sel.Sel.Name,
				FieldType: toTypeProto(c.typeOf(sel.Sel)),
			},
		},
	}}
}

// oneofGetterOrNil determines whether call is like msg.GetFoo(), where foo is a
// oneof field of the message type of msg. It returns a corresponding stats
// entry if this is the case, and nil otherwise.
func oneofGetterOrNil(c *cursor, call *dst.CallExpr, parent dst.Node) *spb.Entry {
	sel, sig, ok := c.protoFieldSelectorOrAccessor(call.Fun)
	if !ok || sig == nil {
		return nil
	}
	if !strings.HasPrefix(sel.Sel.Name, "Get") {
		return nil
	}
	field := strings.TrimPrefix(sel.Sel.Name, "Get")

	t := c.typeOfOrNil(sel.X)
	if t == nil {
		return nil // skip over expression without type info (silo'ed?)
	}
	fullQual, isMsg := c.messageTypeName(t)
	if !isMsg {
		return nil
	}
	fullQual = strings.Trim(fullQual, `"`)
	parts := strings.Split(fullQual, ".")
	if len(parts) < 2 {
		log.Errorf("not a fully qualified type name: %s", fullQual)
		return nil
	}
	msg, pkg := parts[len(parts)-1], strings.Join(parts[:len(parts)-1], ".")
	expectRetType := pkg + ".is" + msg + "_" + field

	res := sig.Results()
	if res.Len() != 1 {
		return nil
	}
	if res.At(0).Type().String() != expectRetType {
		return nil
	}
	return &spb.Entry{
		Location: location(c, call),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(t),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", call),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: &spb.Use{
			Type: spb.Use_METHOD_CALL,
			MethodCall: &spb.MethodCall{
				Method: sel.Sel.Name,
				Type:   spb.MethodCall_GET_ONEOF,
			},
		},
	}
}

// buildGetterEntryOrNil determines whether call is like msg.GetBuild() to get
// the value of the field called build of the proto message msg. It returns a
// corresponding stats Entry if this is the case, and nil otherwise.
func buildGetterEntryOrNil(c *cursor, call *dst.CallExpr, parent dst.Node) *spb.Entry {
	sel, sig, ok := c.protoFieldSelectorOrAccessor(call.Fun)
	if !ok || sig == nil {
		return nil
	}
	if sel.Sel.Name != "GetBuild" {
		return nil
	}
	t := c.typeOfOrNil(sel.X)
	if t == nil {
		return nil // skip over expression without type info (silo'ed?)
	}
	_, isMsg := c.messageTypeName(t)
	if !isMsg {
		return nil
	}
	return &spb.Entry{
		Location: location(c, call),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(t),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", call),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: &spb.Use{
			Type: spb.Use_METHOD_CALL,
			MethodCall: &spb.MethodCall{
				Method: sel.Sel.Name, Type: spb.MethodCall_GET_BUILD,
			},
		},
	}
}

func callStats(c *cursor, call *dst.CallExpr, parent dst.Node) []*spb.Entry {
	if c.isBuiltin(call.Fun) {
		return nil
	}

	var f types.Object
	switch expr := call.Fun.(type) {
	case *dst.Ident:
		f = c.objectOf(expr)
	case *dst.SelectorExpr:
		f = c.objectOf(expr.Sel)
	}
	var fname, fpkg string
	if f != nil {
		fname = f.Name()
		if p := f.Pkg(); p != nil {
			fpkg = p.Path()
		}
	} else if _, ok := call.Fun.(*dst.InterfaceType); !ok {
		// We could probably drop this branch. After AST transformations we may
		// have no knowledge of the actual function object/type (e.g. if we
		// don't maintain it correctly). It does not matter because none of the
		// rewritten functions have interfaces as arguments, and it's safe to
		// skip them. However, for explicit conversions, there's no
		// corresponding function object/type. Addressing this todo requires
		// updating the rewriter to correctly object identity for all introduced
		// function and method calls.
		return nil
	}

	if entry := oneofGetterOrNil(c, call, parent); entry != nil {
		return []*spb.Entry{entry}
	}
	if entry := buildGetterEntryOrNil(c, call, parent); entry != nil {
		return []*spb.Entry{entry}
	}

	if len(call.Args) == 0 {
		return nil
	}

	var out []*spb.Entry

	if len(call.Args) == 1 {
		ft := c.typeOf(call.Fun)
		// explicit conversion: interface{}(m)
		if hasInterfaceType(c, call.Fun) {
			// should we consider this a shallow copy too?
			return logConversion(c, out, call.Args[0], nil, c.typeOf(call.Fun), call, parent, &spb.Use{
				Type: spb.Use_CONVERSION,
				Conversion: &spb.Conversion{
					Context:      spb.Conversion_EXPLICIT,
					DestTypeName: ft.String(),
					FuncArg: &spb.FuncArg{
						FunctionName: fname,
						PackagePath:  fpkg,
						Signature:    ft.String(),
					},
				},
			})
		}
		// explicit conversion: unsafe.Pointer(m)
		if arg, ok := convToUnsafePointerArg(c, call); ok {
			if t, ok := c.shouldLogCompositeType(c.typeOf(arg), true); ok {
				return append(out, &spb.Entry{
					Location: location(c, call),
					Level:    toRewriteLevel(c.lvl),
					Type:     toTypeProto(t),
					Expr: &spb.Expression{
						Type:       fmt.Sprintf("%T", call),
						ParentType: fmt.Sprintf("%T", parent),
					},
					Use: &spb.Use{
						Type: spb.Use_CONVERSION,
						Conversion: &spb.Conversion{
							DestTypeName: ft.String(),
							Context:      spb.Conversion_EXPLICIT,
						},
					},
				})
			}
		}
	}

	// A function call. Look for arguments that are implicitly converted to an interface or shallow-copied.
	// For example:
	//   "proto.Clone(m)"
	//   "f(*m)"
	//   "f(g())" where g() returns at least one protocol buffer message

	ft, ok := c.typeOf(call.Fun).(*types.Signature)
	if !ok {
		return out
	}

	var argTypes []types.Type
	arg0t := c.typeOfOrNil(call.Args[0])
	if arg0t == nil {
		return logSiloed(c, out, call.Args[0], parent)
	}
	if tuple, ok := arg0t.(*types.Tuple); ok {
		// Handle "f(g())"-style calls where g() returns a tuple of results.
		for i := 0; i < tuple.Len(); i++ {
			argTypes = append(argTypes, tuple.At(i).Type())
		}
	} else {
		// Handle "f(a,b,c)"-style calls.
		for _, a := range call.Args {
			t := c.typeOfOrNil(a)
			if t == nil {
				return logSiloed(c, out, a, parent)
			}
			argTypes = append(argTypes, t)
		}
	}

	for i, argType := range argTypes {
		var t types.Type
		if ft.Variadic() {
			if i < ft.Params().Len()-1 {
				t = ft.Params().At(i).Type()
			} else {
				t = ft.Params().At(ft.Params().Len() - 1).Type().(*types.Slice).Elem()
			}
		} else {
			t = ft.Params().At(i).Type()
		}
		out = logConversion(c, out, nil, argType, t, call, parent, &spb.Use{
			Type: spb.Use_CONVERSION,
			Conversion: &spb.Conversion{
				Context:      spb.Conversion_CALL_ARGUMENT,
				DestTypeName: typeName(t),
				FuncArg: &spb.FuncArg{
					FunctionName: fname,
					PackagePath:  fpkg,
					Signature:    ft.String(),
				},
			},
		})
		out = logShallowCopy(c, out, nil, argType, call, parent, &spb.Use{
			Type: spb.Use_SHALLOW_COPY,
			ShallowCopy: &spb.ShallowCopy{
				Type: spb.ShallowCopy_CALL_ARGUMENT,
			},
		})
	}

	return out
}

func assignStats(c *cursor, as *dst.AssignStmt, parent dst.Node) []*spb.Entry {
	// a!=b && b==1 happens when right-hand side returns a tuple (e.g. map access or a function call)
	if a, b := len(as.Lhs), len(as.Rhs); a != b && b != 1 {
		panic(fmt.Sprintf("invalid assignment: lhs has %d exprs, rhs has %d exprs", a, b))
	}

	var out []*spb.Entry
	for i, lhs := range as.Lhs {
		// Ignore valid situations where a dst.Ident may have no type. For example:
		// n (*dst.Ident) has no known type in
		//   switch n := in.(type) {
		if _, ok := parent.(*dst.TypeSwitchStmt); ok && !c.hasType(lhs) {
			continue
		}

		lhst := c.typeOfOrNil(lhs)
		if lhst == nil {
			out = logSiloed(c, out, lhs, parent)
			continue
		}
		conversion := &spb.Use{
			Type: spb.Use_CONVERSION,
			Conversion: &spb.Conversion{
				Context:      spb.Conversion_ASSIGNMENT,
				DestTypeName: typeName(lhst),
			},
		}
		shallowCopy := &spb.Use{
			Type: spb.Use_SHALLOW_COPY,
			ShallowCopy: &spb.ShallowCopy{
				Type: spb.ShallowCopy_ASSIGN,
			},
		}
		if len(as.Lhs) == len(as.Rhs) {
			out = logConversion(c, out, as.Rhs[i], nil, lhst, as, parent, conversion)
			out = logShallowCopy(c, out, as.Rhs[i], nil, as, parent, shallowCopy)
		} else {
			rhst := c.typeOfOrNil(as.Rhs[0])
			if rhst == nil {
				out = logSiloed(c, out, as.Rhs[0], parent)
				continue
			}

			if _, ok := rhst.(*types.Tuple); !ok {
				continue
			}

			out = logConversion(c, out, nil, rhst.(*types.Tuple).At(i).Type(), lhst, as, parent, conversion)
			out = logShallowCopy(c, out, nil, rhst.(*types.Tuple).At(i).Type(), as, parent, shallowCopy)
		}
	}
	return out
}

func constructor(c *cursor, lit *dst.CompositeLit, parent dst.Node) *spb.Entry {
	ct := &spb.Entry{
		Location: location(c, lit),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(c.typeOf(lit)),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", lit),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: &spb.Use{
			Type:        spb.Use_CONSTRUCTOR,
			Constructor: &spb.Constructor{},
		},
	}
	ctype := c.typeOf(lit).String()
	switch {
	case strings.HasSuffix(ctype, "_builder"):
		// Consider an empty builder as builder type, not empty.
		ct.GetUse().GetConstructor().Type = spb.Constructor_BUILDER
	case len(lit.Elts) == 0:
		ct.GetUse().GetConstructor().Type = spb.Constructor_EMPTY_LITERAL
	default:
		ct.GetUse().GetConstructor().Type = spb.Constructor_NONEMPTY_LITERAL
	}
	return ct
}

func compositeLitStats(c *cursor, lit *dst.CompositeLit, parent dst.Node) []*spb.Entry {
	var out []*spb.Entry

	t := c.typeOfOrNil(lit)
	if t == nil {
		return logSiloed(c, out, lit, parent)
	}

	// isBuilderType is cheaper.
	if c.isBuilderType(t) || c.shouldTrackType(t) {
		out = append(out, constructor(c, lit, parent))
	}
	if len(lit.Elts) == 0 {
		return out
	}

	// Conversion in composite literal construction.
Elt:
	for i, e := range lit.Elts {
		var val dst.Expr
		if kv, ok := e.(*dst.KeyValueExpr); ok {
			val = kv.Value
		} else {
			val = e
		}
		use := func(t types.Type) *spb.Use {
			return &spb.Use{
				Type: spb.Use_CONVERSION,
				Conversion: &spb.Conversion{
					Context:      spb.Conversion_COMPOSITE_LITERAL_ELEMENT,
					DestTypeName: typeName(t),
				},
			}
		}
		shallowCopyUse := &spb.Use{
			Type: spb.Use_SHALLOW_COPY,
			ShallowCopy: &spb.ShallowCopy{
				Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT,
			},
		}
		switch t := c.underlyingTypeOf(lit).(type) {
		case *types.Pointer: // e.g. []*struct{m *pb.M}{{m2}}
			if kv, ok := e.(*dst.KeyValueExpr); ok {
				t := c.typeOfOrNil(kv.Key)
				if t == nil {
					out = logSiloed(c, out, lit, parent)
					continue Elt
				}
				out = logConversion(c, out, val, nil, t, e, lit, use(t))
				out = logShallowCopy(c, out, val, nil, e, lit, shallowCopyUse)
			} else {
				if st, ok := t.Elem().Underlying().(*types.Struct); ok {
					t := st.Field(i).Type()
					out = logConversion(c, out, val, nil, t, e, lit, use(t))
					out = logShallowCopy(c, out, val, nil, e, lit, shallowCopyUse)
				}
			}
		case *types.Struct: // e.g. []struct{m *pb.M}{{m2}}
			if kv, ok := e.(*dst.KeyValueExpr); ok {
				t := c.typeOfOrNil(kv.Key)
				if t == nil {
					out = logSiloed(c, out, lit, parent)
					continue Elt
				}
				out = logConversion(c, out, val, nil, t, e, lit, use(t))
				out = logShallowCopy(c, out, val, nil, e, lit, shallowCopyUse)
			} else {
				t := t.Field(i).Type()
				out = logConversion(c, out, val, nil, t, e, lit, use(t))
				out = logShallowCopy(c, out, val, nil, e, lit, shallowCopyUse)
			}
		case *types.Slice: // e.g. []*pb.M2{m2}
			out = logConversion(c, out, val, nil, t.Elem(), e, lit, use(t.Elem()))
			out = logShallowCopy(c, out, val, nil, e, lit, shallowCopyUse)
		case *types.Array: // e.g. [1]*pb.M2{m2}
			out = logConversion(c, out, val, nil, t.Elem(), e, lit, use(t.Elem()))
			out = logShallowCopy(c, out, val, nil, e, lit, shallowCopyUse)
		case *types.Map: // e.g. map[*pb.M2]*pb.M2{m2: m2}
			kv, ok := e.(*dst.KeyValueExpr)
			if !ok {
				ae := c.typesInfo.astMap[e]
				panic("can't process a map element: not a key-value at " +
					c.pkg.Fileset.Position(ae.Pos()).String())
			}
			out = logConversion(c, out, kv.Key, nil, t.Key(), kv, lit, use(t.Key()))
			out = logConversion(c, out, val, nil, t.Elem(), e, lit, use(t.Elem()))
			out = logShallowCopy(c, out, kv.Key, nil, kv, lit, shallowCopyUse) // impossible?
			out = logShallowCopy(c, out, val, nil, e, lit, shallowCopyUse)     // map[Key]pb.M{Key{}: *m2}
		case *types.Basic:
			if t.Kind() == types.Invalid {
				out = logSiloed(c, out, lit, parent)
				continue Elt
			}
		default:
			ae := c.typesInfo.astMap[e]
			panic(fmt.Sprintf("unrecognized composite literal type %T (%v) at %s at level %s",
				t, t, c.pkg.Fileset.Position(ae.Pos()).String(), c.lvl))
		}
	}

	// Write to an internal field.
	if t := c.typeOf(lit); c.shouldTrackType(t) {
		var s *types.Struct
		if p, ok := t.Underlying().(*types.Pointer); ok {
			s = p.Elem().Underlying().(*types.Struct)
		} else {
			s = t.Underlying().(*types.Struct)
		}
		for i, e := range lit.Elts {
			var fname string
			var ftype types.Type
			if kv, ok := e.(*dst.KeyValueExpr); ok {
				f := kv.Key.(*dst.Ident)
				fname, ftype = f.Name, c.typeOf(f)
			} else {
				f := s.Field(i)
				fname, ftype = f.Name(), f.Type()
			}
			if !strings.HasPrefix(fname, "XXX_") {
				continue
			}
			out = append(out, &spb.Entry{
				Location: location(c, e),
				Level:    toRewriteLevel(c.lvl),
				Type:     toTypeProto(t),
				Expr: &spb.Expression{
					Type:       fmt.Sprintf("%T", e),
					ParentType: fmt.Sprintf("%T", lit),
				},
				Use: &spb.Use{
					Type: spb.Use_INTERNAL_FIELD_ACCESS,
					InternalFieldAccess: &spb.FieldAccess{
						FieldName: fname,
						FieldType: toTypeProto(ftype),
					},
				},
			})
		}
	}

	return out
}

func sendStats(c *cursor, send *dst.SendStmt, parent dst.Node) []*spb.Entry {
	underlying := c.underlyingTypeOfOrNil(send.Chan)
	if underlying == nil {
		return logSiloed(c, nil, send, parent)
	}
	dst := underlying.(*types.Chan).Elem()
	out := logConversion(c, nil, send.Value, nil, dst, send, parent, &spb.Use{
		Type: spb.Use_CONVERSION,
		Conversion: &spb.Conversion{
			Context:      spb.Conversion_CHAN_SEND,
			DestTypeName: typeName(dst),
		},
	})
	out = append(out, logShallowCopy(c, nil, send.Value, nil, send, parent, &spb.Use{
		Type: spb.Use_SHALLOW_COPY,
		ShallowCopy: &spb.ShallowCopy{
			Type: spb.ShallowCopy_CHAN_SEND,
		},
	})...)
	return out
}

func returnStats(c *cursor, ret *dst.ReturnStmt, parent dst.Node, parents map[dst.Node]dst.Node) []*spb.Entry {
	p := parent
	for i := 0; ; i++ {
		if i > 1000 {
			panic("too many parent nodes; is there a cycle in the parent structure?")
		}
		_, isfunc := p.(*dst.FuncDecl)
		_, isflit := p.(*dst.FuncLit)
		if isfunc || isflit {
			break
		}
		p = parents[p]
	}
	var sig *types.Signature
	switch p := p.(type) {
	case *dst.FuncDecl:
		pt := c.typeOfOrNil(p.Name)
		if pt == nil {
			return logSiloed(c, nil, ret, parent)
		}
		sig = pt.(*types.Signature)
	case *dst.FuncLit:
		pt := c.typeOfOrNil(p)
		if pt == nil {
			return logSiloed(c, nil, ret, parent)
		}
		sig = pt.(*types.Signature)
	default:
		panic(fmt.Sprintf("invalid parent function type %T; must be *dst.FuncDecl or *dst.FuncLit", p))
	}

	// Naked returns: there's no conversion possible because return values
	// already have the same type as the function specifies. Any conversion
	// to that type was already captured in assignments before the return
	// statement.
	if len(ret.Results) == 0 {
		return nil
	}

	// Handle the special case of returning the result of a function call as
	// multiple results:
	//   func f() (a,b T) {
	//     return g()
	//   }
	if len(ret.Results) == 1 && sig.Results().Len() > 1 {
		rt0 := c.typeOfOrNil(ret.Results[0])
		if rt0 == nil {
			return logSiloed(c, nil, ret.Results[0], parent)
		}
		rt := rt0.(*types.Tuple)
		if a, b := sig.Results().Len(), rt.Len(); a != b {
			panic(fmt.Sprintf("number of function return value types (%d) doesn't match the number of returned values as a tuple (%d)", a, b))
		}
		var out []*spb.Entry
		for i := 0; i < rt.Len(); i++ {
			dst := sig.Results().At(i).Type()
			out = logConversion(c, out, nil, rt.At(i).Type(), dst, ret, parent, &spb.Use{
				Type: spb.Use_CONVERSION,
				Conversion: &spb.Conversion{
					Context:      spb.Conversion_FUNC_RET,
					DestTypeName: typeName(dst),
				},
			})
			out = logShallowCopy(c, out, nil, rt.At(i).Type(), ret, parent, &spb.Use{
				Type: spb.Use_SHALLOW_COPY,
				ShallowCopy: &spb.ShallowCopy{
					Type: spb.ShallowCopy_FUNC_RET,
				},
			})
		}
		return out
	}

	// Handle the typical case: the return statement has one value for each
	// return value in the function signature.
	if a, b := sig.Results().Len(), len(ret.Results); a != b {
		panic(fmt.Sprintf("number of function return value types (%d) doesn't match the number of returned values (%d)", a, b))
	}
	var out []*spb.Entry
	for i, srcExpr := range ret.Results {
		dst := sig.Results().At(i).Type()
		out = logConversion(c, out, srcExpr, nil, dst, ret, parent, &spb.Use{
			Type: spb.Use_CONVERSION,
			Conversion: &spb.Conversion{
				Context:      spb.Conversion_FUNC_RET,
				DestTypeName: typeName(dst),
			},
		})
		out = logShallowCopy(c, out, srcExpr, nil, ret, parent, &spb.Use{
			Type: spb.Use_SHALLOW_COPY,
			ShallowCopy: &spb.ShallowCopy{
				Type: spb.ShallowCopy_FUNC_RET,
			},
		})
	}
	return out
}

func typeAssertStats(c *cursor, ta *dst.TypeAssertExpr, parent dst.Node) []*spb.Entry {
	// Ignore type assertions that don't have known types. This could
	// happen, for example, in a type switch. The expression:
	//   in.(type)
	// has no known type in:
	//   switch n := in.(type) {
	if !c.hasType(ta) {
		// Not handled: case with a proto type.
		return nil
	}

	t, ok := c.shouldLogCompositeType(c.typeOf(ta), true)
	if !ok {
		return nil
	}
	return []*spb.Entry{&spb.Entry{
		Location: location(c, ta),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(t),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", ta),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: &spb.Use{
			Type: spb.Use_TYPE_ASSERTION,
			TypeAssertion: &spb.TypeAssertion{
				SrcType: toTypeProto(c.typeOf(ta.X)),
			},
		},
	}}
}

func typeSpecStats(c *cursor, ts *dst.TypeSpec, parent dst.Node) []*spb.Entry {
	if ts.Assign {
		// Type aliases are not interesting.
		return nil
	}
	t := c.typeOf(ts.Type)
	if !c.shouldTrackType(t) {
		return nil
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		// For U in:
		//   type T *pb.M3
		//   type U T
		// we want t==*pb.M3, but for U in
		//   type U pb.M3
		//   we want t==pb.M3 (not the underlying struct)
		t = p
	}
	return []*spb.Entry{&spb.Entry{
		Location: location(c, ts),
		Level:    toRewriteLevel(c.lvl),
		Type:     toTypeProto(t),
		Expr: &spb.Expression{
			Type:       fmt.Sprintf("%T", ts),
			ParentType: fmt.Sprintf("%T", parent),
		},
		Use: &spb.Use{
			Type: spb.Use_TYPE_DEFINITION,
			TypeDefinition: &spb.TypeDefinition{
				NewType: toTypeProto(c.typeOf(ts.Name)),
			},
		},
	}}
}

func structStats(c *cursor, st *dst.StructType, parent dst.Node) []*spb.Entry {
	var idx int
	var out []*spb.Entry
	for _, f := range st.Fields.List {
		idx += len(f.Names)
		if len(f.Names) != 0 {
			continue
		}
		t := c.typeOf(f.Type)
		if !c.shouldTrackType(t) {
			continue
		}
		out = append(out, &spb.Entry{
			Location: location(c, f),
			Level:    toRewriteLevel(c.lvl),
			Type:     toTypeProto(t),
			Expr: &spb.Expression{
				Type:       fmt.Sprintf("%T", st),
				ParentType: fmt.Sprintf("%T", parent),
			},
			Use: &spb.Use{
				Type: spb.Use_EMBEDDING,
				Embedding: &spb.Embedding{
					FieldIndex: int64(idx),
				},
			},
		})
	}
	return out
}

// isBuilderType returns true for types which we consider to represent builder types generated
// by the proto generator that builds protocol buffer messages.
func (c *cursor) isBuilderType(t types.Type) bool {
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	nt, ok := t.(*types.Named)
	if !ok {
		return false
	}
	if !strings.HasSuffix(nt.String(), "_builder") {
		return false
	}

	// Check whether the type has a method called "Build" and it takes no argument
	// and returns a proto.
	for i := 0; i < nt.NumMethods(); i++ {
		f := nt.Method(i)
		if f.Name() != "Build" {
			continue
		}
		sig, ok := f.Type().(*types.Signature)
		if !ok {
			return false
		}
		if sig.Params() != nil {
			return false
		}
		res := sig.Results()
		if res == nil || res.Len() != 1 {
			return false
		}
		return c.shouldTrackType(res.At(0).Type())
	}
	return false
}

// shouldTrackType returns true for types which we consider to represent protocol buffers generated
// by the proto generator that the user requested to migrate. That is, types that should be
// considered for a rewrite during the open2opaque protocol buffer migration.
//
// The function returns true for all references to proto messages. Including
// accesses that we are currently not rewriting (e.g. via custom named types
// whose underlying type is a protocol buffer struct, or messages that
// explicitly stay on the OPEN_V1 API).
//
// Also see the shouldUpdateType function which returns true for types that we
// are currently rewriting.
func (c *cursor) shouldTrackType(t types.Type) bool {
	name := strings.TrimPrefix(t.String(), "*")

	t = t.Underlying()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if !(protodetecttypes.Type{T: t}.IsMessage()) {
		return false
	}
	name = strings.TrimPrefix(name, "*")
	return len(c.typesToUpdate) == 0 || c.typesToUpdate[name]
}
