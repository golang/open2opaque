// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"context"
	"fmt"
	"testing"

	"github.com/dave/dst"
	"github.com/google/go-cmp/cmp"
	spb "google.golang.org/open2opaque/internal/dashboard"
	"google.golang.org/open2opaque/internal/o2o/statsutil"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestStats(t *testing.T) {
	t.Setenv("GODEBUG", "gotypesalias=1")

	const extra = `
type NotAProto struct {
  S *string
  Field struct{}
}
var notAProto *NotAProto
func g() string { return "" }
`
	loc := func(startLn, startCol, endLn, endCol int64) *spb.Location {
		return &spb.Location{
			Package: "google.golang.org/open2opaque/internal/fix/testdata/fake",
			File:    "google.golang.org/open2opaque/internal/fix/testdata/fake/pkg_test.go",
			Start: &spb.Position{
				Line:   startLn,
				Column: startCol,
			},
			End: &spb.Position{
				Line:   endLn,
				Column: endCol,
			},
		}
	}
	const protoMsg = "google.golang.org/protobuf/proto.Message"
	m2Val := statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto.M2")
	m2 := statsutil.ShortAndLongNameFrom("*google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto.M2")
	m3Val := statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3")
	m3 := statsutil.ShortAndLongNameFrom("*google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3")
	expr := func(node, parent any) *spb.Expression {
		return &spb.Expression{
			Type:       fmt.Sprintf("%T", node),
			ParentType: fmt.Sprintf("%T", parent),
		}
	}
	entry := func(loc *spb.Location, typ *spb.Type, expr *spb.Expression, use any) *spb.Entry {
		e := &spb.Entry{
			Location: loc,
			Level:    toRewriteLevel(None),
			Type:     typ,
			Expr:     expr,
		}
		switch use := use.(type) {
		case *spb.Use:
			e.Use = use
		case *spb.Constructor:
			e.Use = &spb.Use{
				Type:        spb.Use_CONSTRUCTOR,
				Constructor: use,
			}
		case *spb.Conversion:
			e.Use = &spb.Use{
				Type:       spb.Use_CONVERSION,
				Conversion: use,
			}
		case *spb.TypeAssertion:
			e.Use = &spb.Use{
				Type:          spb.Use_TYPE_ASSERTION,
				TypeAssertion: use,
			}
		case *spb.TypeDefinition:
			e.Use = &spb.Use{
				Type:           spb.Use_TYPE_DEFINITION,
				TypeDefinition: use,
			}
		case *spb.Embedding:
			e.Use = &spb.Use{
				Type:      spb.Use_EMBEDDING,
				Embedding: use,
			}
		case *spb.ShallowCopy:
			e.Use = &spb.Use{
				Type:        spb.Use_SHALLOW_COPY,
				ShallowCopy: use,
			}
		case *spb.MethodCall:
			e.Use = spb.Use_builder{
				Type:       spb.Use_METHOD_CALL,
				MethodCall: use,
			}.Build()
		default:
			panic(fmt.Sprintf("Bad 'use' type: %T", use))
		}
		return e
	}
	directFieldAccess := func(fieldName, shortType, longType string) *spb.Use {
		return &spb.Use{
			Type: spb.Use_DIRECT_FIELD_ACCESS,
			DirectFieldAccess: &spb.FieldAccess{
				FieldName: fieldName,
				FieldType: &spb.Type{
					ShortName: shortType,
					LongName:  longType,
				},
			},
		}
	}
	typeMissing := func(loc *spb.Location, expr *spb.Expression) *spb.Entry {
		return spb.Entry_builder{
			Status: spb.Status_builder{
				Type:  spb.Status_FAIL,
				Error: "type information missing; are dependencies in a silo?",
			}.Build(),
			Location: loc,
			Level:    toRewriteLevel(None),
			Expr:     expr,
		}.Build()
	}

	callStmt := expr(&dst.CallExpr{}, &dst.ExprStmt{})
	callCall := expr(&dst.CallExpr{}, &dst.CallExpr{})
	callConv := func(dstType, fname, fpkg, fsig string, c spb.Conversion_Context) *spb.Conversion {
		return &spb.Conversion{
			DestTypeName: dstType,
			FuncArg: &spb.FuncArg{
				FunctionName: fname,
				PackagePath:  fpkg,
				Signature:    fsig,
			},
			Context: c,
		}
	}
	retStmt := expr(&dst.ReturnStmt{}, &dst.BlockStmt{})

	assignStmt := expr(&dst.AssignStmt{}, &dst.BlockStmt{})
	assignConv := func(dstType string) *spb.Conversion {
		return &spb.Conversion{
			DestTypeName: dstType,
			Context:      spb.Conversion_ASSIGNMENT,
		}
	}

	kvCLit := expr(&dst.KeyValueExpr{}, &dst.CompositeLit{})
	starCLit := expr(&dst.StarExpr{}, &dst.CompositeLit{})
	identCLit := expr(&dst.Ident{}, &dst.CompositeLit{})
	elemConv := func(dstType string) *spb.Conversion {
		return &spb.Conversion{
			DestTypeName: dstType,
			Context:      spb.Conversion_COMPOSITE_LITERAL_ELEMENT,
		}
	}

	selAssign := expr(&dst.SelectorExpr{}, &dst.AssignStmt{})

	cLitUnary := expr(&dst.CompositeLit{}, &dst.UnaryExpr{})
	cLitCLit := expr(&dst.CompositeLit{}, &dst.CompositeLit{})
	emptyLiteral := &spb.Constructor{Type: spb.Constructor_EMPTY_LITERAL}
	nonEmptyLiteral := &spb.Constructor{Type: spb.Constructor_NONEMPTY_LITERAL}
	builderLiteral := &spb.Constructor{Type: spb.Constructor_BUILDER}

	tests := []test{{
		desc:  "direct field accesses",
		extra: extra,
		in: `
// simple proto2/proto3 access
_ = m2.S
m2.S = nil
_ = m3.S
m3.S = ""

// repeated fields
m3.Ms[0] = nil
m3.Ms = append(m3.Ms, m3)

// multiple selector expressions
m3.M.M.S = ""

// accessing a method is not a direct field access
_ = m3.GetS

// direct field access in non-protos don't count
var s NotAProto
_ = s.S
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				// simple proto2/proto3 access
				entry(loc(3, 5, 3, 9), m2, selAssign, directFieldAccess("S", "*string", "*string")),
				entry(loc(4, 1, 4, 5), m2, selAssign, directFieldAccess("S", "*string", "*string")),
				entry(loc(5, 5, 5, 9), m3, selAssign, directFieldAccess("S", "string", "string")),
				entry(loc(6, 1, 6, 5), m3, selAssign, directFieldAccess("S", "string", "string")),
				// repeated fields
				entry(loc(9, 1, 9, 6), m3, expr(&dst.SelectorExpr{}, &dst.IndexExpr{}), directFieldAccess("Ms", "[]*M3", "[]*google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3")),
				entry(loc(10, 1, 10, 6), m3, selAssign, directFieldAccess("Ms", "[]*M3", "[]*google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3")),
				entry(loc(10, 16, 10, 21), m3, expr(&dst.SelectorExpr{}, &dst.CallExpr{}), directFieldAccess("Ms", "[]*M3", "[]*google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3")),
				// multiple selector expressions
				entry(loc(13, 1, 13, 9), m3, selAssign, directFieldAccess("S", "string", "string")),
				entry(loc(13, 1, 13, 7), m3, expr(&dst.SelectorExpr{}, &dst.SelectorExpr{}), directFieldAccess("M", "*M3", "*google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3")),
				entry(loc(13, 1, 13, 5), m3, expr(&dst.SelectorExpr{}, &dst.SelectorExpr{}), directFieldAccess("M", "*M3", "*google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3")),
			},
		},
	}, {
		desc: "conversion in call expression",
		extra: extra + `
func retProto() *pb2.M2 { return nil }
func protoIn(*pb2.M2) { }
func protoIn2(*pb2.M2, *pb2.M2) { }
func efaceIn(interface{}) { }
func efaceIn2(a,b interface{}) { }
func efaceVararg(format string, args ...interface{}) { }
func msgVararg(format string, args ...proto.Message) { }

type T struct{}
func (T) Method(interface{}) {}
`,
		in: `
protoIn(m2)        // ignored: no conversion
protoIn(&pb2.M2{})  // ignored: no conversion

efaceIn(notAProto) // ignored: not a proto

efaceIn(m2)
efaceIn2(m2, &pb2.M2{})

proto.Clone(m2)
proto.Marshal(m2)

proto.Clone(proto.Message(m2))
proto.Marshal(proto.Message(m2))

efaceVararg("", m2, m2)
msgVararg("", m2)

T{}.Method(m2)
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(3, 10, 3, 18), m2Val, cLitUnary, emptyLiteral),
				entry(loc(7, 1, 7, 12), m2, callStmt, callConv("interface{}", "efaceIn", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(8, 1, 8, 24), m2, callStmt, callConv("interface{}", "efaceIn2", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(a interface{}, b interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(8, 1, 8, 24), m2, callStmt, callConv("interface{}", "efaceIn2", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(a interface{}, b interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(8, 15, 8, 23), m2Val, cLitUnary, emptyLiteral),
				entry(loc(10, 1, 10, 16), m2, callStmt, callConv(protoMsg, "Clone", "google.golang.org/protobuf/proto", "func(m "+protoMsg+") "+protoMsg, spb.Conversion_CALL_ARGUMENT)),
				entry(loc(11, 1, 11, 18), m2, callStmt, callConv(protoMsg, "Marshal", "google.golang.org/protobuf/proto", "func(m "+protoMsg+") ([]byte, error)", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(13, 13, 13, 30), m2, callCall, callConv(protoMsg, "Message", "google.golang.org/protobuf/proto", protoMsg, spb.Conversion_EXPLICIT)),
				entry(loc(14, 15, 14, 32), m2, callCall, callConv(protoMsg, "Message", "google.golang.org/protobuf/proto", protoMsg, spb.Conversion_EXPLICIT)),
				entry(loc(16, 1, 16, 24), m2, callStmt, callConv("interface{}", "efaceVararg", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(format string, args ...interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(16, 1, 16, 24), m2, callStmt, callConv("interface{}", "efaceVararg", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(format string, args ...interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(17, 1, 17, 18), m2, callStmt, callConv(protoMsg, "msgVararg", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(format string, args ..."+protoMsg+")", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(19, 1, 19, 15), m2, callStmt, callConv("interface{}", "Method", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
			},
		},
	}, {
		desc: "conversion in assignment",
		extra: `
func multival() (interface{}, *pb2.M2, int, *pb2.M2) {
  return nil, nil, 0, nil
}
`,
		in: `
x := m2            // ignored: no conversion
var mm *pb2.M2 = m2 // ignored: no conversion

var in interface{}
in = m2

var min proto.Message
min = m3

var n int
n, in = 1, m2
in, n2 := m2, 0
in, in = m2, m3

var m map[string]*pb2.M2
in, ok := m[""]
in, _ = m[""]

in, in, n, in = multival() // eface->eface, *pb.M->eface, int->int, *pb.M->eface

_, _, _, _, _, _, _ = mm, n, n2, ok, x, in, min
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(6, 1, 6, 8), m2, assignStmt, assignConv("interface{}")),    // in = m2
				entry(loc(9, 1, 9, 9), m3, assignStmt, assignConv(protoMsg)),         // min = m3
				entry(loc(12, 1, 12, 14), m2, assignStmt, assignConv("interface{}")), // n, in = 1, m2
				entry(loc(13, 1, 13, 16), m2, assignStmt, assignConv("interface{}")), // in, n2 := m2, 0
				entry(loc(14, 1, 14, 16), m2, assignStmt, assignConv("interface{}")), // in, in = m2, m3
				entry(loc(14, 1, 14, 16), m3, assignStmt, assignConv("interface{}")), // in, in = m2, m3
				entry(loc(17, 1, 17, 16), m2, assignStmt, assignConv("interface{}")), // in, ok := m[""]
				entry(loc(18, 1, 18, 14), m2, assignStmt, assignConv("interface{}")), // in, _ = m[""]
				entry(loc(20, 1, 20, 27), m2, assignStmt, assignConv("interface{}")), // in, in, n, in = multival() : second arg
				entry(loc(20, 1, 20, 27), m2, assignStmt, assignConv("interface{}")), // in, in, n, in = multival() : last arg
			},
		},
	}, {
		desc: "conversion in construction",
		in: `
type t struct {
	eface interface{}
	msg proto.Message
	m *pb2.M2
}

_ = t{
  eface: m2,
  msg: m3,
  m: m2, // ignore: no conversion
}

_ = &t{
  eface: m2,
  msg: m3,
  m: m2, // ignore: no conversion
}

_ = &t{m: m2} // ignore: no conversions
_ = &t{eface: m2}

_ = []struct{m interface{}}{{m: m2}}
_ = []struct{m *pb2.M2}{{m: m2}} // ignore: no conversion

_ = map[int]interface{}{0: m2}
_ = map[interface{}]int{m2: 0}
_ = map[interface{}]interface{}{m2: m2}

_ = [...]interface{}{0:m2}
_ = []interface{}{0:m2}

_ = &t{m2,m2,m2} // 2 findings + 1 ignored (no conversion)
_ = []interface{}{m2}
_ = []struct{m interface{}}{{m2}}

_ = []*t{{m2, m2, m2}}
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(9, 3, 9, 12), m2, kvCLit, elemConv("interface{}")),
				entry(loc(10, 3, 10, 10), m3, kvCLit, elemConv(protoMsg)),
				entry(loc(15, 3, 15, 12), m2, kvCLit, elemConv("interface{}")),
				entry(loc(16, 3, 16, 10), m3, kvCLit, elemConv(protoMsg)),
				entry(loc(21, 8, 21, 17), m2, kvCLit, elemConv("interface{}")),
				entry(loc(23, 30, 23, 35), m2, kvCLit, elemConv("interface{}")),
				entry(loc(26, 25, 26, 30), m2, kvCLit, elemConv("interface{}")),
				entry(loc(27, 25, 27, 30), m2, kvCLit, elemConv("interface{}")),
				entry(loc(28, 33, 28, 39), m2, kvCLit, elemConv("interface{}")),
				entry(loc(28, 33, 28, 39), m2, kvCLit, elemConv("interface{}")),
				entry(loc(30, 22, 30, 26), m2, kvCLit, elemConv("interface{}")),
				entry(loc(31, 19, 31, 23), m2, kvCLit, elemConv("interface{}")),
				entry(loc(33, 8, 33, 10), m2, identCLit, elemConv("interface{}")),
				entry(loc(33, 11, 33, 13), m2, identCLit, elemConv(protoMsg)),
				entry(loc(34, 19, 34, 21), m2, identCLit, elemConv("interface{}")),
				entry(loc(35, 30, 35, 32), m2, identCLit, elemConv("interface{}")),
				entry(loc(37, 11, 37, 13), m2, identCLit, elemConv("interface{}")),
				entry(loc(37, 15, 37, 17), m2, identCLit, elemConv(protoMsg)),
			},
		},
	}, {
		desc: "conversions to unsafe.Pointer",
		extra: `
func f(unsafe.Pointer) {}
func g(*pb2.M2) unsafe.Pointer{ return nil }
`,
		in: `
_ = unsafe.Pointer(m2)
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(2, 5, 2, 23), m2, expr(&dst.CallExpr{}, &dst.AssignStmt{}), &spb.Conversion{
					DestTypeName: "unsafe.Pointer",
					Context:      spb.Conversion_EXPLICIT,
				}),
			},
		},
	}, {
		desc: "composite types: contexts",
		extra: `
type s struct {m *pb2.M2}
func f(interface{}) {}
func g() (int, uintptr, unsafe.Pointer) {
	for {
		return 0, 0, unsafe.Pointer(&pb2.M2{})
	}
	return 0, 0, nil
}
`,
		in: `
// Various contexts:
f(&s{m: m2}) // conversion in call arg
var in interface{}
in = &s{m2} // conversion in assignment
in = struct{s *s}{s: &s{m2}}
_ = struct{s interface{}}{s: m2}
_ = struct{s interface{}}{m2}
f(unsafe.Pointer(&m2))
in = <-make(chan *pb2.M2) // assignment from *pb.M to interface{} in assignment
make(chan interface{}) <- m2
_ = func() interface{} {
	if true {
		return m2
	}
	return nil
}()

type namedChan chan interface{}
make(namedChan) <- m2

_ = in
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(3, 1, 3, 13), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(5, 1, 5, 12), m2, assignStmt, assignConv("interface{}")),
				entry(loc(6, 1, 6, 29), m2, assignStmt, assignConv("interface{}")),
				entry(loc(7, 27, 7, 32), m2, kvCLit, elemConv("interface{}")),
				entry(loc(8, 27, 8, 29), m2, expr(&dst.Ident{}, &dst.CompositeLit{}), elemConv("interface{}")),
				entry(loc(9, 3, 9, 22), m2, expr(&dst.CallExpr{}, &dst.CallExpr{}), &spb.Conversion{
					DestTypeName: "unsafe.Pointer",
					Context:      spb.Conversion_EXPLICIT,
				}),
				entry(loc(10, 1, 10, 26), m2, assignStmt, assignConv("interface{}")),
				entry(loc(11, 1, 11, 29), m2, expr(&dst.SendStmt{}, &dst.BlockStmt{}), &spb.Conversion{
					DestTypeName: "interface{}",
					Context:      spb.Conversion_CHAN_SEND,
				}),
				entry(loc(14, 3, 14, 12), m2, retStmt, &spb.Conversion{
					DestTypeName: "interface{}",
					Context:      spb.Conversion_FUNC_RET,
				}),
				entry(loc(20, 1, 20, 22), m2, expr(&dst.SendStmt{}, &dst.BlockStmt{}), &spb.Conversion{
					DestTypeName: "interface{}",
					Context:      spb.Conversion_CHAN_SEND,
				}),
				// This is for function "g" which is defined in "extra" (outside "in") and hence the line number is out of range.
				entry(loc(31, 16, 31, 41), m2, expr(&dst.CallExpr{}, &dst.ReturnStmt{}), &spb.Conversion{
					DestTypeName: "unsafe.Pointer",
					Context:      spb.Conversion_EXPLICIT,
				}),
				entry(loc(31, 32, 31, 40), m2Val, cLitUnary, emptyLiteral),
			},
		},
	}, {
		desc: "composite types: types",
		extra: `
func f(interface{}) {}
func g(_, _ interface{}) {}
`,
		in: `
f(&m2)
f([]*pb2.M2{})
f([1]*pb2.M2{{}})
f(map[int]*pb2.M2{})
f(map[*pb2.M2]int{})
g(func() (_,_ *pb2.M2) { return }()) // generates two entries
f(make(chan *pb2.M2))            // ignored: no proto value provided to reflection, only type
f(func(*pb2.M2){})               // ignored: no proto value provided to reflection, only type
f(func(a,b,c int, m *pb2.M2){})  // ignored: no proto value provided to reflection, only type

type msg pb2.M2
f(&msg{})

f(struct{m *pb2.M2}{m: m2})

_ = make(map[int]bool, len([]int{})) // make sure that builtins don't mess things up with variadic functions
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(2, 1, 2, 7), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(3, 1, 3, 15), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(4, 1, 4, 18), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(4, 14, 4, 16), m2, cLitCLit, emptyLiteral),
				entry(loc(5, 1, 5, 21), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(6, 1, 6, 21), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(7, 1, 7, 37), m2, callStmt, callConv("interface{}", "g", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(_ interface{}, _ interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(7, 1, 7, 37), m2, callStmt, callConv("interface{}", "g", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(_ interface{}, _ interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(12, 6, 12, 16), m2Val, expr(&dst.TypeSpec{}, &dst.GenDecl{}), &spb.TypeDefinition{NewType: statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.msg")}),
				entry(loc(13, 1, 13, 10), statsutil.ShortAndLongNameFrom("*google.golang.org/open2opaque/internal/fix/testdata/fake.msg"), callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
				entry(loc(13, 4, 13, 9), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.msg"), cLitUnary, emptyLiteral),
				entry(loc(15, 1, 15, 28), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
			},
		},
	}, {
		desc: "composite types: constructors",
		in: `
m2 = &pb2.M2{S: proto.String("s")}
_ = pb2.M2{
	S: proto.String("s"),
	B: proto.Bool(true),
}
_ = pb2.M2_builder{
	S: proto.String("builder"),
}.Build()
m3s := []*pb3.M3{
	{S: "pointer"},
	{},
	pb3.M3_builder{}.Build(),
}
m3s = append(m3s, &pb3.M3{})

_ = []pb3.M3{
	{S: "shallow"},
	{},
}

type NotMsg struct{ M *pb2.M2 }
_ = &NotMsg{ M: &pb2.M2{} }
_ = NotMsg{}
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(2, 7, 2, 35), m2Val, cLitUnary, nonEmptyLiteral),
				entry(loc(3, 1, 6, 2), m2Val, assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(3, 5, 6, 2), m2Val, expr(&dst.CompositeLit{}, &dst.AssignStmt{}), nonEmptyLiteral),
				entry(loc(7, 5, 9, 2), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto.M2_builder"), expr(&dst.CompositeLit{}, &dst.SelectorExpr{}), builderLiteral),
				entry(loc(11, 2, 11, 16), m3, cLitCLit, nonEmptyLiteral),
				entry(loc(12, 2, 12, 4), m3, cLitCLit, emptyLiteral),
				entry(loc(13, 2, 13, 18), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3_builder"), expr(&dst.CompositeLit{}, &dst.SelectorExpr{}), builderLiteral),
				entry(loc(15, 20, 15, 28), m3Val, cLitUnary, emptyLiteral),
				entry(loc(18, 2, 18, 16), m3Val, cLitCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(19, 2, 19, 4), m3Val, cLitCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(18, 2, 18, 16), m3Val, cLitCLit, nonEmptyLiteral),
				entry(loc(19, 2, 19, 4), m3Val, cLitCLit, emptyLiteral),
				entry(loc(23, 18, 23, 26), m2Val, cLitUnary, emptyLiteral),
			},
		},
	}, {
		desc: "type assertions",
		extra: `type NotAProto struct {
  S *string
  Field struct{}
}`,
		in: `
var in interface{}
_ = in.(*pb3.M3)
_ = in.(*NotAProto)

switch in.(type) {}
switch n := in.(type) { case int: _ = n }
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(3, 5, 3, 17), m3, expr(&dst.TypeAssertExpr{}, &dst.AssignStmt{}), &spb.TypeAssertion{
					SrcType: &spb.Type{
						ShortName: "interface{}",
						LongName:  "interface{}",
					},
				}),
			},
		},
	}, {
		desc: "type defs",
		extra: `type NotAProto struct {
  S *string
  Field struct{}
}`,
		in: `
type alias = *pb3.M3     // ignored
type notProto NotAProto // ignored
type myProto pb2.M2
type myProtoPtr *pb3.M3
type myProtoPtr2 myProtoPtr
type myProtoPtr3 *myProtoPtr // ignored: pointer to pointer

// Composite types are not interesting to us.
type ignored1 struct { _ *pb2.M2 }
type ignored2 func(*pb3.M3)
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(4, 6, 4, 20), m2Val, expr(&dst.TypeSpec{}, &dst.GenDecl{}), &spb.TypeDefinition{NewType: statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.myProto")}),
				entry(loc(5, 6, 5, 24), m3, expr(&dst.TypeSpec{}, &dst.GenDecl{}), &spb.TypeDefinition{NewType: statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.myProtoPtr")}),
				entry(loc(6, 6, 6, 28), m3, expr(&dst.TypeSpec{}, &dst.GenDecl{}), &spb.TypeDefinition{NewType: statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.myProtoPtr2")}),
			},
		},
	}, {
		desc: "type embedding",
		extra: `type NotAProto struct {
  S *string
  Field struct{}
}`,
		in: `
type ignored struct {
	NotAProto
	Named *pb3.M3
	_ *pb3.M3
}

type T struct {
	n int
	*pb3.M3
}

type U struct {
	_, _ int
	_ int
	pb3.M3
}

type V struct {
	_, _, _ int
	T            // ignored
}

type W struct {
	_, _, _ int
	_ func(*pb3.M3)    // ignored
}

type named pb3.M3
type X struct {
	named
}
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(10, 2, 10, 9), m3, expr(&dst.StructType{}, &dst.TypeSpec{}), &spb.Embedding{FieldIndex: 1}),
				entry(loc(16, 2, 16, 8), m3Val, expr(&dst.StructType{}, &dst.TypeSpec{}), &spb.Embedding{FieldIndex: 3}),
				entry(loc(29, 6, 29, 18), m3Val, expr(&dst.TypeSpec{}, &dst.GenDecl{}), &spb.TypeDefinition{NewType: statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.named")}),
				entry(loc(31, 2, 31, 7), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.named"), expr(&dst.StructType{}, &dst.TypeSpec{}), &spb.Embedding{FieldIndex: 0}),
			},
		},
	}, {
		desc:  "recursive type",
		extra: "func f(interface{}){}",
		in: `
type S struct {
  S *S
  M *pb2.M2
}
f(&S{})
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(6, 1, 6, 8), m2, callStmt, callConv("interface{}", "f", "google.golang.org/open2opaque/internal/fix/testdata/fake", "func(interface{})", spb.Conversion_CALL_ARGUMENT)),
			},
		},
	}, {
		desc:  "mismatched number of return values",
		extra: `func f() (*pb2.M2, *pb2.M2) { return nil, nil }`,
		in: `
_ = func() (m interface{}) {
	m = &pb2.M2{}
	return
}
_ = func() (a,b interface{}) {
	return f()
}
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(3, 2, 3, 15), m2, assignStmt, assignConv("interface{}")),
				entry(loc(3, 7, 3, 15), m2Val, cLitUnary, emptyLiteral),
				entry(loc(7, 2, 7, 12), m2, retStmt, &spb.Conversion{
					DestTypeName: "interface{}",
					Context:      spb.Conversion_FUNC_RET,
				}),
				entry(loc(7, 2, 7, 12), m2, retStmt, &spb.Conversion{
					DestTypeName: "interface{}",
					Context:      spb.Conversion_FUNC_RET,
				}),
			},
		},
	}, {
		// Copy a proto message by value. This test doesn't check for copying a
		// composite type that results in a proto shallow copy).
		desc: "direct shallow copies",
		extra: `
func args(int, pb2.M2) {}
func ret() (_ int, _ pb2.M2) { return } // naked return so that we don't trigger analysis
`,
		in: `
copy := *m2   // 0: assign-shallow-copy
copy = *m2    // 1: assign-shallow-copy
var n int
n, copy = 0, *m2  // 2: assign-shallow-copy
_,_ = copy,n // 3: assign-shallow-copy

args(0, *m2)  // 4: call-argument-shallow-copy

args(ret()) // 5: call-argument-shallow-copy

func() (int, pb2.M2) {
	return 0, *m2 // 6: call-argument-return-shallow-copy
}()

func() (int, pb2.M2) {
	return ret() // 7: call-argument-return-shallow-copy
}()

(*m2).GetS()  // ignored: non-pointer receiver is fine

m := map[string]pb2.M2{
	"": *m2,  // 8: composite-literal-shallow-copy
}
copy, _ = m[""]  // 9: assign-shallow-copy

s := &struct {
	m pb2.M2
} {
	m: *m2,  // 11: composite-literal-shallow-copy
}
_ = s

ch := make(chan pb2.M2)
ch <- *m2  // 12: chan-send-shallow-copy

var in interface{} = *m2
_ = in

_ = []*struct{m pb2.M2}{
	{*m2},         // 14: composite-literal-shallow-copy
	{m: *m2},      // 16: composite-literal-shallow-copy
}
_ = []struct{m pb2.M2}{
	{*m2},         // 17,18: composite-literal-shallow-copy of the entire struct '{*m2}' and of the message itself '*m2'
	{m: *m2},      // 20,22: composite-literal-shallow-copy of the entire struct '{m: *m2}' and of the message itself '*m2'
}
_ = []pb2.M2{*m2}  // 23: composite-literal-shallow-copy of the element '*m2'
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(2, 1, 2, 12), m2Val, assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(3, 1, 3, 11), m2Val, assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(5, 1, 5, 17), m2Val, assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(6, 1, 6, 13), m2Val, assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(8, 1, 8, 13), m2Val, callStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_CALL_ARGUMENT}),
				entry(loc(10, 1, 10, 12), m2Val, callStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_CALL_ARGUMENT}),
				entry(loc(13, 2, 13, 15), m2Val, retStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_FUNC_RET}),
				entry(loc(17, 2, 17, 14), m2Val, retStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_FUNC_RET}),
				entry(loc(23, 2, 23, 9), m2Val, kvCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(25, 1, 25, 16), m2Val, assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(30, 2, 30, 8), m2Val, kvCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(35, 1, 35, 10), m2Val, expr(&dst.SendStmt{}, &dst.BlockStmt{}), &spb.ShallowCopy{Type: spb.ShallowCopy_CHAN_SEND}),
				entry(loc(41, 3, 41, 6), m2Val, starCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(42, 3, 42, 9), m2Val, kvCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(45, 2, 45, 7), m2Val, cLitCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(46, 2, 46, 10), m2Val, cLitCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(45, 3, 45, 6), m2Val, starCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(46, 3, 46, 9), m2Val, kvCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
				entry(loc(48, 14, 48, 17), m2Val, starCLit, &spb.ShallowCopy{Type: spb.ShallowCopy_COMPOSITE_LITERAL_ELEMENT}),
			},
		},
	}, {
		// Copy a container with a proto struct.
		desc: "indirect shallow copies",
		extra: `
type msg pb2.M2
type S struct{m msg}
func args(_, _ S){}
func argsp(_, _ *S){}`,
		in: `
s := S{}  // 0: shallow copy in definition
_ = s     // 2: shallow copy in assignment
args(func() (_, _ S) { return }())  // 3,4: shallow via a tuple (twice because there are two values in the tuple)
_ = [1]pb2.M2{}    // 5: copy an array

// Those are OK because of the indirection
sp := &S{}
_ = sp
argsp(func() (_, _ *S) { return }())
_ = [1]*pb2.M2{}

`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(2, 1, 2, 9), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.msg"), assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(3, 1, 3, 6), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.msg"), assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),
				entry(loc(4, 1, 4, 35), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.msg"), callStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_CALL_ARGUMENT}),
				entry(loc(4, 1, 4, 35), statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.msg"), callStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_CALL_ARGUMENT}),
				entry(loc(5, 1, 5, 16), m2Val, assignStmt, &spb.ShallowCopy{Type: spb.ShallowCopy_ASSIGN}),

				// "type S struct{m msg}" definition:
				entry(loc(17, 6, 17, 16), m2Val, expr(&dst.TypeSpec{}, &dst.GenDecl{}), &spb.TypeDefinition{NewType: statsutil.ShortAndLongNameFrom("google.golang.org/open2opaque/internal/fix/testdata/fake.msg")}),
			},
		},
	}, {
		desc: "type information missing",
		in: `
_ = siloedpb.Message{}
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				typeMissing(loc(2, 1, 2, 23), assignStmt),
				typeMissing(loc(2, 1, 2, 23), assignStmt),
				typeMissing(loc(2, 5, 2, 23), expr(&dst.CompositeLit{}, &dst.AssignStmt{})),
				typeMissing(loc(2, 5, 2, 21), expr(&dst.SelectorExpr{}, &dst.CompositeLit{})),
				typeMissing(loc(4, 2, 4, 27), assignStmt),
				typeMissing(loc(4, 2, 4, 27), assignStmt),
			},
		},
	}, {
		// The method GetFoo for a oneof field foo only exists in the OPEN API.
		desc: "GetOneof",
		in: `
switch x := m2.GetOneofField().(type) {
	case *pb2.M2_StringOneof:
		_ = x
	default:
}
_ = m3.GetOneofField()
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(2, 13, 2, 31), m2, expr(&dst.CallExpr{}, &dst.TypeAssertExpr{}), &spb.MethodCall{Method: "GetOneofField", Type: spb.MethodCall_GET_ONEOF}),
				entry(loc(7, 5, 7, 23), m3, expr(&dst.CallExpr{}, &dst.AssignStmt{}), &spb.MethodCall{Method: "GetOneofField", Type: spb.MethodCall_GET_ONEOF}),
			},
		},
	}, {
		desc: "GetBuild",
		in: `
_ = m2.GetBuild()
`,
		wantStats: map[Level][]*spb.Entry{
			None: []*spb.Entry{
				entry(loc(2, 5, 2, 18), m2, expr(&dst.CallExpr{}, &dst.AssignStmt{}), &spb.MethodCall{Method: "GetBuild", Type: spb.MethodCall_GET_BUILD}),
			},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			in := NewSrc(tt.in, tt.extra)
			statsOnly := []Level{}
			_, got, err := fixSource(context.Background(), in, "pkg_test.go", ConfiguredPackage{}, statsOnly)
			if err != nil {
				t.Fatalf("fixSources(%q) failed: %v; Full input:\n%s", tt.in, err, in)
			}
			for _, lvl := range []Level{None} {
				want, ok := tt.wantStats[lvl]
				if !ok {
					continue
				}
				if len(want) != len(got[lvl]) {
					t.Errorf("len(want)=%d != len(got[lvl])=%d", len(want), len(got[lvl]))
				}
				for idx := range want {
					if diff := cmp.Diff(want[idx], got[lvl][idx], protocmp.Transform()); diff != "" {
						// We don't print got/want because it's a lot of output and the diff is almost always enough.
						t.Errorf("[%s] fixSources(level=%s, message %d) = diff:\n%s", tt.desc, lvl, idx, diff)
					}
				}
			}
		})
	}
}

func TestSliceLiteral(t *testing.T) {
	const src = `
for _, _ = range map[*[]*pb2.M2]string{
		{}: "UNKNOWN_NOTIFICATION_TYPE",
		{
			{
				S: nil,
			},
		}: "UNKNOWN_NOTIFICATION_TYPE",
} {
	panic("irrelevant")
}
`
	in := NewSrc(src, "")
	_, got, err := fixSource(context.Background(), in, "pkg_test.go", ConfiguredPackage{}, []Level{Green, Yellow, Red})
	if err != nil {
		t.Fatalf("fixSources(%q) failed: %v; Full input:\n%s", src, err, in)
	}
	t.Logf("got: %v", got)
}
