// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"context"
	"strings"
	"testing"

	"github.com/kylelemons/godebug/diff"
)

func TestCommon(t *testing.T) {
	tests := []test{{
		desc: "ignore messages that opted-out",
		in: `
m := &pb2.DoNotMigrateMe{B: proto.Bool(true)}
_ = *m.B
_ = m.B != nil
m.B = nil
`,
		want: map[Level]string{
			Green: `
m := &pb2.DoNotMigrateMe{B: proto.Bool(true)}
_ = *m.B
_ = m.B != nil
m.B = nil
`,
		},
	}, {
		desc: "proto2: set new empty message",
		in:   "m2.M = &pb2.M2{}",
		want: map[Level]string{
			Green: "m2.SetM(&pb2.M2{})",
		},
	}, {
		desc: "ignore custom types",
		extra: `
type T1 pb2.M2
type T2 *pb2.M2
`,
		in: `
m := &T1{M: nil}
_ = *m.I64
_ = m.I64
m.I64 = nil
_ = m.I64 == nil

var n T2
_ = *n.I64
_ = n.I64
n.I64 = nil
_ = n.I64 == nil
`,
		want: map[Level]string{
			Red: `
m := &T1{M: nil}
_ = *m.I64
_ = m.I64
m.I64 = nil
_ = m.I64 == nil

var n T2
_ = *n.I64
_ = n.I64
n.I64 = nil
_ = n.I64 == nil
`,
		},
	}, {
		desc: "proto2: set new non-empty message",
		in:   `m2.M = &pb2.M2{S: proto.String("hello")}`,
		want: map[Level]string{
			Green: `m2.SetM(pb2.M2_builder{S: proto.String("hello")}.Build())`,
		},
	}, {
		desc: "proto2: set existing message",
		in:   "m2.M = m2",
		want: map[Level]string{
			Green: "m2.SetM(m2)",
		},
	}, {
		desc: "proto2: set message field",
		in:   "m2.M = m2.M",
		want: map[Level]string{
			Green: "m2.SetM(m2.GetM())",
		},
	}, {
		desc:  "proto2: set message function result",
		extra: "func g2() *pb2.M2 { return nil }",
		in:    "m2.M = g2()",
		want: map[Level]string{
			Green: "m2.SetM(g2())",
		},
	}, {
		desc: "proto2: set scalar slice",
		in:   "m2.Is = []int32{1,2,3}",
		want: map[Level]string{
			Green: "m2.SetIs([]int32{1, 2, 3})",
		},
	}, {
		desc:  "proto2: set scalar field, nohelper",
		extra: `var s = new(string)`,
		in:    `m2.S = s // eol comment`,
		want: map[Level]string{
			Yellow: `m2.S = s // eol comment`,
			Red: `// eol comment
if s != nil {
	m2.SetS(*s)
} else {
	m2.ClearS()
}`,
		},
	}, {
		desc: "proto2: set bytes field",
		in: `
var b []byte
m2.Bytes = b
if m2.Bytes = b; false {
}
`,
		want: map[Level]string{
			Green: `
var b []byte
if b != nil {
	m2.SetBytes(b)
} else {
	m2.ClearBytes()
}
if m2.SetBytes(b); false {
}
`,
		},
	}, {
		desc: "proto2: set bytes field with func call",
		in: `
m2.Bytes = returnBytes()
`,
		extra: `
func returnBytes() []byte { return nil }
`,
		want: map[Level]string{
			Green: `
if x := returnBytes(); x != nil {
	m2.SetBytes(x)
} else {
	m2.ClearBytes()
}
`,
		},
	}, {
		desc: "proto3: set bytes field",
		in: `
var b []byte
m3.Bytes = b
if m3.Bytes = b; false {
}
`,
		want: map[Level]string{
			Green: `
var b []byte
m3.SetBytes(b)
if m3.SetBytes(b); false {
}
`,
		},
	}, {
		desc:  "proto2: set scalar field, nohelper, addr",
		extra: `var s = ""`,
		in:    `m2.S = &s`,
		want: map[Level]string{
			Green: `m2.SetS(s)`,
		},
	}, {
		desc: "proto2: set proto.Bool",
		in:   `m2.B = proto.Bool(true)`,
		want: map[Level]string{
			Green: `m2.SetB(true)`,
		},
	}, {
		desc: "proto2: set proto.Float32",
		in:   `m2.F32 = proto.Float32(42)`,
		want: map[Level]string{
			Green: `m2.SetF32(42)`,
		},
	}, {
		desc: "proto2: set proto.Float64",
		in:   `m2.F64 = proto.Float64(42)`,
		want: map[Level]string{
			Green: `m2.SetF64(42)`,
		},
	}, {
		desc: "proto2: set proto.Int",
		in:   `m2.I32 = proto.Int32(42)`,
		want: map[Level]string{
			Green: `m2.SetI32(42)`,
		},
	}, {
		desc:  "proto2: set proto.Int val",
		extra: `var v int`,
		in:    `m2.I32 = proto.Int32(int32(v))`,
		want: map[Level]string{
			Green: `m2.SetI32(int32(v))`,
		},
	}, {
		desc: "proto2: set proto.Int new",
		in:   `m2.I32 = new(int32)`,
		want: map[Level]string{
			Green: `m2.SetI32(0)`,
		},
	}, {
		desc: "proto2: set enum",
		in:   `m2.E = pb2.M2_E_VAL.Enum()`,
		want: map[Level]string{
			Green: `m2.SetE(pb2.M2_E_VAL)`,
		},
	}, {
		desc: "proto2: set enum new",
		in:   `m2.E = new(pb2.M2_Enum)`,
		want: map[Level]string{
			Green: `m2.SetE(pb2.M2_Enum(0))`,
		},
	}, {
		desc: "proto2: set proto.Int32",
		in:   `m2.I32 = proto.Int32(42)`,
		want: map[Level]string{
			Green: `m2.SetI32(42)`,
		},
	}, {
		desc: "proto2: set proto.Int64",
		in:   `m2.I64 = proto.Int64(42)`,
		want: map[Level]string{
			Green: `m2.SetI64(42)`,
		},
	}, {
		desc: "proto2: set proto.String",
		in:   `m2.S = proto.String("q")`,
		want: map[Level]string{
			Green: `m2.SetS("q")`,
		},
	}, {
		desc: "proto2: set proto.Uint32",
		in:   `m2.Ui32 = proto.Uint32(42)`,
		want: map[Level]string{
			Green: `m2.SetUi32(42)`,
		},
	}, {
		desc: "proto2: set proto.Uint64",
		in:   `m2.Ui64 = proto.Uint64(42)`,
		want: map[Level]string{
			Green: `m2.SetUi64(42)`,
		},
	}, {
		desc: "proto2: scalar field copy",
		in:   `m2.S = m2a.S // eol comment`,
		want: map[Level]string{
			Green: `
// eol comment
if m2a.HasS() {
	m2.SetS(m2a.GetS())
} else {
	m2.ClearS()
}
`,
		},
	}, {
		desc: "proto2: scalar field copy, lhs/rhs identical",
		in:   `m2.S = m2.S // eol comment`,
		want: map[Level]string{
			Green: `
// eol comment
if m2.HasS() {
	m2.SetS(m2.GetS())
} else {
	m2.ClearS()
}
`,
		},
	}, {
		desc: "proto3: set new empty message",
		in:   `m3.M = &pb3.M3{}`,
		want: map[Level]string{
			Green: `m3.SetM(&pb3.M3{})`,
		},
	}, {
		desc: "proto3: set new non-empty message",
		in:   `m3.M = &pb3.M3{S:"s"}`,
		want: map[Level]string{
			Green: `m3.SetM(pb3.M3_builder{S: "s"}.Build())`,
		},
	}, {
		desc: "proto3: set existing message",
		in:   "m3.M = m3",
		want: map[Level]string{
			Green: "m3.SetM(m3)",
		},
	}, {
		desc: "proto3: set message field",
		in:   "m3.M = m3.M",
		want: map[Level]string{
			Green: "m3.SetM(m3.GetM())",
		},
	}, {
		desc: "proto3: set message field new",
		in:   "m3.M = new(pb3.M3)",
		want: map[Level]string{
			Green: "m3.SetM(new(pb3.M3))",
		},
	}, {
		desc:  "proto3: set message function result",
		extra: "func g3() *pb3.M3 { return nil }",
		in:    "m3.M = g3()",
		want: map[Level]string{
			Green: "m3.SetM(g3())",
		},
	}, {
		desc: "proto3: set scalar slice",
		in:   "m3.Is = []int32{1,2,3}",
		want: map[Level]string{
			Green: "m3.SetIs([]int32{1, 2, 3})",
		},
	}, {
		desc: "deref set",
		extra: `
var s string
var i32 int32
type S struct {
	Proto *pb2.M2
}
`,
		in: `
*m2.S = "hello"
*m2.I32 = 1
*m2.S = s
*m2.I32 = i32

*m2.M.S = "hello"
*m2.M.I32 = 1
*m2.M.S = s
*m2.M.I32 = i32

ss := &S{}
*ss.Proto.M.S = "hello"
*ss.Proto.M.I32 = 1
*ss.Proto.M.S = s
*ss.Proto.M.I32 = i32
`,
		want: map[Level]string{
			Green: `
m2.SetS("hello")
m2.SetI32(1)
m2.SetS(s)
m2.SetI32(i32)

m2.GetM().SetS("hello")
m2.GetM().SetI32(1)
m2.GetM().SetS(s)
m2.GetM().SetI32(i32)

ss := &S{}
ss.Proto.GetM().SetS("hello")
ss.Proto.GetM().SetI32(1)
ss.Proto.GetM().SetS(s)
ss.Proto.GetM().SetI32(i32)
`,
			Red: `
m2.SetS("hello")
m2.SetI32(1)
m2.SetS(s)
m2.SetI32(i32)

m2.GetM().SetS("hello")
m2.GetM().SetI32(1)
m2.GetM().SetS(s)
m2.GetM().SetI32(i32)

ss := &S{}
ss.Proto.GetM().SetS("hello")
ss.Proto.GetM().SetI32(1)
ss.Proto.GetM().SetS(s)
ss.Proto.GetM().SetI32(i32)
`,
		},
	}, {
		desc: "proto3: scalar field copy",
		in:   `m3.S = m3.S`,
		want: map[Level]string{
			Green: `m3.SetS(m3.GetS())`,
		},
	}, {
		desc: "proto2: clear message",
		in:   "m2.M = nil",
		want: map[Level]string{
			Green: "m2.ClearM()",
		},
	}, {
		desc: "proto2: clear scalar",
		in:   "m2.S = nil",
		want: map[Level]string{
			Green: "m2.ClearS()",
		},
	}, {
		desc: "proto2: clear enum",
		in:   "m2.E = nil",
		want: map[Level]string{
			Green: "m2.ClearE()",
		},
	}, {
		desc: "proto2: clear bytes",
		in:   "m2.Bytes = nil",
		want: map[Level]string{
			Green: "m2.ClearBytes()",
		},
	}, {
		desc: "proto2: clear scalar slice",
		in:   "m2.Is = nil",
		want: map[Level]string{
			Green: "m2.SetIs(nil)",
		},
	}, {
		desc: "proto2: clear message slice",
		in:   "m2.Ms = nil",
		want: map[Level]string{
			Green: "m2.SetMs(nil)",
		},
	}, {
		desc: "proto2: clear map",
		in:   "m2.Map = nil",
		want: map[Level]string{
			Green: "m2.SetMap(nil)",
		},
	}, {
		desc: "proto3: clear message",
		in:   "m3.M = nil",
		want: map[Level]string{
			Green: "m3.ClearM()",
		},
	}, {
		desc: "proto3: clear bytes",
		in:   "m3.Bytes = nil",
		want: map[Level]string{
			Green: "m3.SetBytes(nil)",
		},
	}, {
		desc:  "proto3 value: clear bytes",
		extra: "var m3val pb3.M3",
		in:    "m3val.Bytes = nil",
		want: map[Level]string{
			Green: "m3val.SetBytes(nil)",
		},
	}, {
		desc: "proto3: clear scalar slice",
		in:   "m3.Is = nil",
		want: map[Level]string{
			Green: "m3.SetIs(nil)",
		},
	}, {
		desc: "proto3: clear message slice",
		in:   "m3.Ms = nil",
		want: map[Level]string{
			Green: "m3.SetMs(nil)",
		},
	}, {
		desc: "proto3: clear map",
		in:   "m3.Map = nil",
		want: map[Level]string{
			Green: "m3.SetMap(nil)",
		},
	}, {
		desc: "proto2: get message ptr",
		in:   "_ = m2.M",
		want: map[Level]string{
			Green: "_ = m2.GetM()",
		},
	}, {
		desc: "proto2: get message value",
		in:   "_ = *m2.M",
		want: map[Level]string{
			Green: "_ = *m2.GetM()",
		},
	}, {
		desc: "proto2: get scalar ptr",
		in:   `_ = m2.S`,
		want: map[Level]string{
			Green:  `_ = m2.S`,
			Yellow: `_ = proto.ValueOrNil(m2.HasS(), m2.GetS)`,
			Red:    `_ = proto.ValueOrNil(m2.HasS(), m2.GetS)`,
		},
	}, {
		desc: "proto2: get scalar ptr from side effect free expr",
		in:   `_ = m2.Ms[0].I32`,
		want: map[Level]string{
			Green:  `_ = m2.GetMs()[0].I32`,
			Yellow: `_ = proto.ValueOrNil(m2.GetMs()[0].HasI32(), m2.GetMs()[0].GetI32)`,
			Red:    `_ = proto.ValueOrNil(m2.GetMs()[0].HasI32(), m2.GetMs()[0].GetI32)`,
		},
	}, {
		desc:  "proto2: get scalar ptr from side effect expr (index)",
		extra: `func f() int { return 0 }`,
		in:    `_ = m2.Ms[f()].I32`,
		want: map[Level]string{
			Green:  `_ = m2.GetMs()[f()].I32`,
			Yellow: `_ = func(msg *pb2.M2) *int32 { return proto.ValueOrNil(msg.HasI32(), msg.GetI32) }(m2.GetMs()[f()])`,
			Red:    `_ = func(msg *pb2.M2) *int32 { return proto.ValueOrNil(msg.HasI32(), msg.GetI32) }(m2.GetMs()[f()])`,
		},
	}, {
		desc:  "proto2: get scalar ptr from side effect expr (receiver)",
		extra: `func f() []*pb2.M2 { return nil }`,
		in:    `_ = f()[0].I32`,
		want: map[Level]string{
			Green:  `_ = f()[0].I32`,
			Yellow: `_ = func(msg *pb2.M2) *int32 { return proto.ValueOrNil(msg.HasI32(), msg.GetI32) }(f()[0])`,
			Red:    `_ = func(msg *pb2.M2) *int32 { return proto.ValueOrNil(msg.HasI32(), msg.GetI32) }(f()[0])`,
		},
	}, {
		desc: "proto2: get enum ptr",
		in:   `_ = m2.E`,
		want: map[Level]string{
			Green:  `_ = m2.E`,
			Yellow: `_ = proto.ValueOrNil(m2.HasE(), m2.GetE)`,
			Red:    `_ = proto.ValueOrNil(m2.HasE(), m2.GetE)`,
		},
	}, {
		desc: "proto2: get scalar value",
		in:   "_ = *m2.S",
		want: map[Level]string{
			Green: "_ = m2.GetS()",
		},
	}, {
		desc: "proto2: scalar slice",
		in:   "_ = m2.Is",
		want: map[Level]string{
			Green: "_ = m2.GetIs()",
		},
	}, {
		desc: "proto2: scalar slice and index",
		in:   "_ = m2.Is[0]",
		want: map[Level]string{
			Green: "_ = m2.GetIs()[0]",
		},
	}, {
		desc: "proto2: message slice",
		in:   "_ = m2.Ms",
		want: map[Level]string{
			Green: "_ = m2.GetMs()",
		},
	}, {
		desc: "proto2: message slice and index",
		in:   "_ = m2.Ms[0]",
		want: map[Level]string{
			Green: "_ = m2.GetMs()[0]",
		},
	}, {
		desc:  "proto2: get in function args",
		extra: "func g2(*string, string, *pb2.M2, []int32, []*pb2.M2) { }",
		in:    "g2(m2.S, *m2.S, m2.M, m2.Is, m2.Ms)",
		want: map[Level]string{
			Green: "g2(m2.S, m2.GetS(), m2.GetM(), m2.GetIs(), m2.GetMs())",
		},
	}, {
		desc: "proto2 assignments: nested",
		in: `
_ = func() int {
	m2.I32 = proto.Int32(23)
	return 0
}()
`,
		want: map[Level]string{
			Red: `
_ = func() int {
	m2.SetI32(23)
	return 0
}()
`,
		},
	}, {
		desc: "proto2 assignments: no side-effects",
		extra: `
var cnt int
func f2() *pb2.M2 {
	cnt++
	return nil
}
`,
		in: `
newM := f2()
m2.B = newM.B
m2.GetM().B = newM.GetM().B
m2.M.B = newM.M.B

type E struct {
  Proto *pb2.M2
}
e := &E{}
e.Proto.B = newM.B
e.Proto.GetM().B = newM.GetM().B
e.Proto.M.B = newM.M.B
m2.B = e.Proto.B
m2.GetM().B = e.Proto.GetM().B
m2.M.B = e.Proto.M.B
`,

		want: map[Level]string{
			Red: `
newM := f2()
if newM.HasB() {
	m2.SetB(newM.GetB())
} else {
	m2.ClearB()
}
if newM.GetM().HasB() {
	m2.GetM().SetB(newM.GetM().GetB())
} else {
	m2.GetM().ClearB()
}
if newM.GetM().HasB() {
	m2.GetM().SetB(newM.GetM().GetB())
} else {
	m2.GetM().ClearB()
}

type E struct {
	Proto *pb2.M2
}
e := &E{}
if newM.HasB() {
	e.Proto.SetB(newM.GetB())
} else {
	e.Proto.ClearB()
}
if newM.GetM().HasB() {
	e.Proto.GetM().SetB(newM.GetM().GetB())
} else {
	e.Proto.GetM().ClearB()
}
if newM.GetM().HasB() {
	e.Proto.GetM().SetB(newM.GetM().GetB())
} else {
	e.Proto.GetM().ClearB()
}
if e.Proto.HasB() {
	m2.SetB(e.Proto.GetB())
} else {
	m2.ClearB()
}
if e.Proto.GetM().HasB() {
	m2.GetM().SetB(e.Proto.GetM().GetB())
} else {
	m2.GetM().ClearB()
}
if e.Proto.GetM().HasB() {
	m2.GetM().SetB(e.Proto.GetM().GetB())
} else {
	m2.GetM().ClearB()
}
`,
		}}, {
		desc: "proto2 assignments: side-effects rhs",
		extra: `
var cnt int
func f2() *pb2.M2 {
	cnt++
	return nil
}
`,
		in: `
m2.B = f2().B // eol comment
`,
		want: map[Level]string{
			Green: `
// eol comment
if x := f2(); x.HasB() {
	m2.SetB(x.GetB())
} else {
	m2.ClearB()
}
`}}, {
		desc: "proto2 assignments: side-effects lhs",
		extra: `
var cnt int
func f2() *pb2.M2 {
	cnt++
	return nil
}
`,
		in: `
f2().B = m2.B
`,
		want: map[Level]string{
			Green: `
if m2.HasB() {
	f2().SetB(m2.GetB())
} else {
	f2().ClearB()
}
`}}, {
		desc: "proto2 assignments: side-effects lhs and rhs",
		extra: `
var cnt int
func f2() *pb2.M2 {
	cnt++
	return nil
}
`,
		in: `
f2().B = f2().B
`,
		want: map[Level]string{
			Green: `
if x := f2(); x.HasB() {
	f2().SetB(x.GetB())
} else {
	f2().ClearB()
}
`}}, {
		desc:  "assign []byte",
		extra: "var v string",
		in: `
m2.Bytes = []byte("hello")
m2.Bytes = []byte(v)
`,
		want: map[Level]string{
			Green: `
m2.SetBytes([]byte("hello"))
m2.SetBytes([]byte(v))
`,
		},
	}, {
		desc: "increment non-proto",
		in: `
for i := 0; i < 10; i++ {
}
`,
		want: map[Level]string{
			Green: `
for i := 0; i < 10; i++ {
}
`, Red: `
for i := 0; i < 10; i++ {
}
`,
		},
	}, {
		desc: "proto2: don't call methods on non-addressable receiver",
		extra: `
func f2() pb2.M2{ return pb2.M2{} }
`,
		in: `
_ = f2().B
_ = f2().Bytes
_ = f2().F32
_ = f2().F64
_ = f2().I32
_ = f2().I64
_ = f2().Ui32
_ = f2().Ui64
_ = f2().S
_ = f2().M
_ = f2().Is
_ = f2().Ms
_ = f2().Map
_ = f2().E

_ = f2().B != nil
_ = f2().Bytes != nil
_ = f2().F32 != nil
_ = f2().F64 != nil
_ = f2().I32 != nil
_ = f2().I64 != nil
_ = f2().Ui32 != nil
_ = f2().Ui64 != nil
_ = f2().S != nil
_ = f2().M != nil
_ = f2().Is != nil
_ = f2().Ms != nil
_ = f2().Map != nil
_ = f2().E != nil

if f2().B != nil {
}
`,
		want: map[Level]string{
			Green: `
_ = f2().B
_ = f2().Bytes
_ = f2().F32
_ = f2().F64
_ = f2().I32
_ = f2().I64
_ = f2().Ui32
_ = f2().Ui64
_ = f2().S
_ = f2().M
_ = f2().Is
_ = f2().Ms
_ = f2().Map
_ = f2().E

_ = f2().B != nil
_ = f2().Bytes != nil
_ = f2().F32 != nil
_ = f2().F64 != nil
_ = f2().I32 != nil
_ = f2().I64 != nil
_ = f2().Ui32 != nil
_ = f2().Ui64 != nil
_ = f2().S != nil
_ = f2().M != nil
_ = f2().Is != nil
_ = f2().Ms != nil
_ = f2().Map != nil
_ = f2().E != nil

if f2().B != nil {
}
`,
			Red: `
_ = func(msg *pb2.M2) *bool { return proto.ValueOrNil(msg.HasB(), msg.GetB) }(f2())
_ = f2().GetBytes()
_ = func(msg *pb2.M2) *float32 { return proto.ValueOrNil(msg.HasF32(), msg.GetF32) }(f2())
_ = func(msg *pb2.M2) *float64 { return proto.ValueOrNil(msg.HasF64(), msg.GetF64) }(f2())
_ = func(msg *pb2.M2) *int32 { return proto.ValueOrNil(msg.HasI32(), msg.GetI32) }(f2())
_ = func(msg *pb2.M2) *int64 { return proto.ValueOrNil(msg.HasI64(), msg.GetI64) }(f2())
_ = func(msg *pb2.M2) *uint32 { return proto.ValueOrNil(msg.HasUi32(), msg.GetUi32) }(f2())
_ = func(msg *pb2.M2) *uint64 { return proto.ValueOrNil(msg.HasUi64(), msg.GetUi64) }(f2())
_ = func(msg *pb2.M2) *string { return proto.ValueOrNil(msg.HasS(), msg.GetS) }(f2())
_ = f2().GetM()
_ = f2().GetIs()
_ = f2().GetMs()
_ = f2().GetMap()
_ = func(msg *pb2.M2) *pb2.M2_Enum { return proto.ValueOrNil(msg.HasE(), msg.GetE) }(f2())

_ = func(msg *pb2.M2) *bool { return proto.ValueOrNil(msg.HasB(), msg.GetB) }(f2()) != nil
_ = f2().HasBytes()
_ = func(msg *pb2.M2) *float32 { return proto.ValueOrNil(msg.HasF32(), msg.GetF32) }(f2()) != nil
_ = func(msg *pb2.M2) *float64 { return proto.ValueOrNil(msg.HasF64(), msg.GetF64) }(f2()) != nil
_ = func(msg *pb2.M2) *int32 { return proto.ValueOrNil(msg.HasI32(), msg.GetI32) }(f2()) != nil
_ = func(msg *pb2.M2) *int64 { return proto.ValueOrNil(msg.HasI64(), msg.GetI64) }(f2()) != nil
_ = func(msg *pb2.M2) *uint32 { return proto.ValueOrNil(msg.HasUi32(), msg.GetUi32) }(f2()) != nil
_ = func(msg *pb2.M2) *uint64 { return proto.ValueOrNil(msg.HasUi64(), msg.GetUi64) }(f2()) != nil
_ = func(msg *pb2.M2) *string { return proto.ValueOrNil(msg.HasS(), msg.GetS) }(f2()) != nil
_ = f2().HasM()
_ = f2().GetIs() != nil
_ = f2().GetMs() != nil
_ = f2().GetMap() != nil
_ = func(msg *pb2.M2) *pb2.M2_Enum { return proto.ValueOrNil(msg.HasE(), msg.GetE) }(f2()) != nil

if func(msg *pb2.M2) *bool { return proto.ValueOrNil(msg.HasB(), msg.GetB) }(f2()) != nil {
}
`,
		},
	}, {
		desc: "proto3: don't call methods on non-addressable receiver",
		extra: `
func f3() pb3.M3{ return pb3.M3{} }
`,
		in: `
_ = f3().B
_ = f3().Bytes
_ = f3().F32
_ = f3().F64
_ = f3().I32
_ = f3().I64
_ = f3().Ui32
_ = f3().Ui64
_ = f3().S
_ = f3().M
_ = f3().Is
_ = f3().Ms
_ = f3().Map

_ = f3().Bytes != nil
_ = f3().M != nil
_ = f3().Is != nil
_ = f3().Ms != nil
_ = f3().Map != nil
`,
		want: map[Level]string{
			Green: `
_ = f3().B
_ = f3().Bytes
_ = f3().F32
_ = f3().F64
_ = f3().I32
_ = f3().I64
_ = f3().Ui32
_ = f3().Ui64
_ = f3().S
_ = f3().M
_ = f3().Is
_ = f3().Ms
_ = f3().Map

_ = len(f3().Bytes) != 0
_ = f3().M != nil
_ = f3().Is != nil
_ = f3().Ms != nil
_ = f3().Map != nil
`,
			Red: `
_ = f3().GetB()
_ = f3().GetBytes()
_ = f3().GetF32()
_ = f3().GetF64()
_ = f3().GetI32()
_ = f3().GetI64()
_ = f3().GetUi32()
_ = f3().GetUi64()
_ = f3().GetS()
_ = f3().GetM()
_ = f3().GetIs()
_ = f3().GetMs()
_ = f3().GetMap()

_ = len(f3().GetBytes()) != 0
_ = f3().HasM()
_ = f3().GetIs() != nil
_ = f3().GetMs() != nil
_ = f3().GetMap() != nil
`,
		},
	}}

	runTableTests(t, tests)
}

func TestShallowCopies(t *testing.T) {
	// Shallow copy rewrites lose scalar field aliasing. All are red rewrites.

	// https://golang.org/ref/spec#Address_operators
	// "For an operand x of type T, the address operation &x generates a
	// pointer of type *T to x. The operand must be addressable, that is,
	// either a variable, pointer indirection, or slice indexing operation;
	// or a field selector of an addressable struct operand; or an array
	// indexing operation of an addressable array."
	tests := skip([]test{{
		desc: `definition, rhs is addressable`,
		in: `
m := *m2
_ = &m
`,
		wantRed: `
var m pb2.M2
proto.Assign(&m, m2)
_ = &m
`,
	}, {
		desc:  `definition, rhs is not addressable`,
		extra: `func g() pb2.M2 { return pb2.M2{} }`,
		in: `
m := g()
_ = &m
`,
		wantRed: `
m := g()
_ = &m
`,
	}, { // Both lhs (left-hand side) and rhs (right-hand side) are addressable.
		desc:    `proto2: lhs is addressable, rhs is an empty composite literal`,
		extra:   `var m pb2.M2`,
		in:      `m = pb2.M2{}`,
		wantRed: `proto.Assign(&m, &pb2.M2{})`,
	}, {
		desc:    `proto3: lhs is addressable, rhs is an empty composite literal`,
		extra:   `var m pb3.M3`,
		in:      `m = pb3.M3{}`,
		wantRed: `proto.Assign(&m, &pb3.M3{})`,
	}, {
		desc:    `lhs is addressable, rhs is non-empty composite literal`,
		extra:   `var m pb2.M2`,
		in:      `m = pb2.M2{M: nil}`,
		wantRed: `proto.Assign(&m, pb2.M2_builder{M: nil}.Build())`,
	}, {
		desc:    `lhs and rhs are addressable: pointer indirections`,
		in:      `*m2 = *m2`,
		wantRed: `proto.Assign(m2, m2)`,
	}, {
		desc:    `lhs and rhs are addressable: pointer indirection 2`,
		extra:   `var m pb2.M2; func mp() *pb2.M2 { return nil }`,
		in:      `m = *mp()`,
		wantRed: `proto.Assign(&m, mp())`,
	}, {
		desc:    `lhs and rhs are addressable: variables`,
		extra:   `var m pb2.M2`,
		in:      `m = m`,
		wantRed: `proto.Assign(&m, &m)`,
	}, {
		desc:  `lhs and rhs are addressable: addressable expr in parens`,
		extra: `var m pb2.M2`,
		in:    `(m) = (m)`,
		want: map[Level]string{
			Yellow: `m = m`,
			Red:    `proto.Assign(&m, &m)`,
		},
	}, {
		desc:    `lhs and rhs are addressable: addressable slice index`,
		extra:   `var ms = []pb2.M2{}`,
		in:      `ms[0] = ms[0]`,
		wantRed: `proto.Assign(&ms[0], &ms[0])`,
	}, {
		desc:    `lhs and rhs are addressable: non-addressable slice index`,
		extra:   `func s() []pb2.M2 { return nil }`,
		in:      `s()[0] = s()[0]`,
		wantRed: `proto.Assign(&s()[0], &s()[0])`,
	}, {
		desc:    `lhs and rhs are addressable: addressable array index`,
		extra:   `var ms [1]pb2.M2`,
		in:      `ms[0] = ms[0]`,
		wantRed: `proto.Assign(&ms[0], &ms[0])`,
	}, {
		desc:    `lhs and rhs are addressable: field selector`,
		extra:   `var t struct{m pb2.M2}`,
		in:      `t.m = t.m`,
		wantRed: `proto.Assign(&t.m, &t.m)`,
	}, { // Only rhs is addressable.
		desc:    `proto2: lhs is not addressable, rhs is an empty struct literal`,
		extra:   `var m = map[int]pb2.M2{}`,
		in:      `m[0] = pb2.M2{}`,
		wantRed: `m[0] = pb2.M2{}`,
	}, {
		desc:    `proto3: lhs is not addressable, rhs is an empty struct literal`,
		extra:   `var m = map[int]pb3.M3{}`,
		in:      `m[0] = pb3.M3{}`,
		wantRed: `m[0] = pb3.M3{}`,
	}, {
		desc:    `lhs is not addressable, rhs is a non-empty composite literal`,
		extra:   `var m = map[int]pb2.M2{}`,
		in:      `m[0] = pb2.M2{M: nil}`,
		wantRed: `m[0] = *pb2.M2_builder{M: nil}.Build()`,
	}, {
		desc:    `lhs is not addressable, rhs is addressable: pointer indirection`,
		extra:   `var m = map[int]pb2.M2{}`,
		in:      `m[0] = *m2`,
		wantRed: `m[0] = *proto.Clone(m2).(*pb2.M2)`,
	}, {
		desc:    `lhs is not addressable, rhs is addressable: addressable slice index`,
		extra:   `var m = map[int]pb2.M2{}; var ms []pb2.M2`,
		in:      `m[0] = ms[0]`,
		wantRed: `m[0] = *proto.Clone(&ms[0]).(*pb2.M2)`,
	}, {
		desc:    `lhs is not addressable, rhs is addressable: not-addressable slice index`,
		extra:   `var m = map[int]pb2.M2{}; func s() []pb2.M2{return nil}`,
		in:      `m[0] = s()[0]`,
		wantRed: `m[0] = *proto.Clone(&s()[0]).(*pb2.M2)`,
	}, {
		desc:    `lhs is not addressable, rhs is addressable: addressable array index`,
		extra:   `var m = map[int]pb2.M2{}; var ms [1]pb2.M2`,
		in:      `m[0] = ms[0]`,
		wantRed: `m[0] = *proto.Clone(&ms[0]).(*pb2.M2)`,
	}, {
		desc:    `lhs is not addressable, rhs is addressable: field selector`,
		extra:   `var m = map[int]pb2.M2{}; var t struct {m pb2.M2} `,
		in:      `m[0] = t.m`,
		wantRed: `m[0] = *proto.Clone(&t.m).(*pb2.M2)`,
	}, {
		desc:    `lhs is an underscore, rhs is addressable`,
		in:      `_ = *m2`,
		wantRed: `_ = *proto.Clone(m2).(*pb2.M2)`,
	}, { // Rhs is not addressable => no rewrite.
		desc:    `lhs is addressable, rhs is not addressable: map access`,
		extra:   `var m = map[int]pb2.M2{}`,
		in:      `*m2 = m[0]`,
		wantRed: `*m2 = m[0]`,
	}, {
		desc:    `lhs is addressable, rhs is not addressable: array index`,
		extra:   `func m() [1]pb2.M2 {return [1]pb2.M2{} }`,
		in:      `*m2 = m()[0]`,
		wantRed: `*m2 = m()[0]`,
	}, {
		desc:    `lhs is addressable, rhs is not addressable: func result`,
		extra:   `func m() pb2.M2 { return pb2.M2{} }`,
		in:      `*m2 = m()`,
		wantRed: `*m2 = m()`,
	}, {
		desc:    `lhs is addressable, rhs is not addressable: type conversion`,
		in:      `*m2 = pb2.M2(pb2.M2{})`,
		wantRed: `*m2 = pb2.M2(pb2.M2{})`,
	}, {
		desc:    `lhs is addressable, rhs is not addressable: chan receive`,
		extra:   `var ch chan pb2.M2`,
		in:      `*m2 = <-ch`,
		wantRed: `*m2 = <-ch`,
	}, { // Neither rhs nor lhs is addressable
		desc:    `lhs is not addressable, rhs is not addressable`,
		extra:   `var m = map[int]pb2.M2{}`,
		in:      `m[0] = m[0]`,
		wantRed: `m[0] = m[0]`,
	}, { // No lhs (not assignment context)
		desc:    `addressable function argument`,
		extra:   `func g(pb2.M2) {}; var m pb2.M2`,
		in:      `g(m)`,
		wantRed: `g(*proto.Clone(&m).(*pb2.M2))`,
	}, {
		desc:    `don't rewrite proto.Clone`,
		extra:   `func g(pb2.M2) {}; var m pb2.M2`,
		in:      `g(*proto.Clone(&m).(*pb2.M2))`,
		wantRed: `g(*proto.Clone(&m).(*pb2.M2))`,
	}, {
		desc:    `empty maker function argument`,
		extra:   `func g(pb2.M2) {}`,
		in:      `g(pb2.M2{})`,
		wantRed: `g(pb2.M2{})`,
	}, {
		desc:    `non-empty maker function argument`,
		extra:   `func g(pb2.M2) {}`,
		in:      `g(pb2.M2{M: nil})`,
		wantRed: `g(*pb2.M2_builder{M: nil}.Build())`,
	}, {
		desc:    `slice of values`,
		in:      `_ = []pb2.M2{{M: nil}, pb2.M2{M: nil}, pb2.M2{}, {}}`,
		wantRed: `_ = []pb2.M2{*pb2.M2_builder{M: nil}.Build(), *pb2.M2_builder{M: nil}.Build(), pb2.M2{}, {}}`,
	}, {
		desc:    `non-addressable function argument`,
		extra:   `func g(pb2.M2) {}; var m map[int]pb2.M2`,
		in:      `g(m[0])`,
		wantRed: `g(m[0])`,
	}})

	runTableTests(t, tests)
}

func TestProto2ScalarAliasing(t *testing.T) {
	tests := skip([]test{{
		desc: "proto2: only def + alias",
		skip: "make red rewrite work for scalar aliasing",
		in: `
s := "hello world"
m2.S = &s`,
		want: map[Level]string{
			Yellow: `
s := "hello world"
m2.S = &s`,
			Red: `
m2.SetS(s)
`,
		}}, {
		desc: "proto2: no access after alias",
		skip: "make red rewrite work for scalar aliasing",
		in: `
s := "hello"
s = "world"
m2.S = &s`,
		want: map[Level]string{
			Yellow: `
s := "hello"
s = "world"
m2.S = &s`,
			Red: `
s := "hello"
s = "world"
m2.SetS(s)
`,
		}}, {
		desc:  "proto2: only reads after alias",
		skip:  "make red rewrite work for scalar aliasing",
		extra: "func g(string) { }", in: `
s := "hello world"
m2.S = &s
g(s)
`,
		want: map[Level]string{
			Yellow: `
s := "hello world"
m2.S = &s
g(s)
`,
			Red: `
s := "hello world"
m2.SetS(s)
g(s)
`,
		}}, {
		extra: "var b = true",
		desc:  "proto2: conditionals prevent inlining",
		skip:  "make red rewrite work for scalar aliasing",
		in: `
s := "hello"
if b {
	s = "world"
}
m2.S = &s
`,
		want: map[Level]string{
			Yellow: `
s := "hello"
if b {
s = "world"
}
m2.S = &s
`,
			Red: `
s := "hello"
if b {
s = "world"
}
m2.SetS(s)
`,
		}}, {
		desc: "aliasing of message fields",
		skip: "make red rewrite work for scalar aliasing",
		in: `
n2 := m()
m2.S = n2.S
`,
		want: map[Level]string{
			Yellow: `
n2 := m()
m2.S = n2.S
`,
			Red: `
n2 := m()
m2.SetS(n2.GetS())
`,
		},
	}})

	runTableTests(t, tests)
}

func trimNL(s string) string {
	if len(s) != 0 && s[0] == '\n' {
		s = s[1:]
	}
	if len(s) != 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}

func TestUnparentExpr(t *testing.T) {
	tests := skip([]test{{
		desc: "proto2: clear scalar",
		in:   "(m2.S) = nil",
		want: map[Level]string{
			Green: "m2.ClearS()",
		}}, {
		desc: "proto2: clear scalar in if",
		in: `
if true {
	(m2.S) = nil
}
`,
		want: map[Level]string{
			Green: `
if true {
	m2.ClearS()
}
`,
		}}, {
		desc: "proto2: clear scalar 2",
		in:   "((m2.S)) = nil",
		want: map[Level]string{
			Green: "m2.ClearS()",
		}}, {
		desc: "proto2: clear message",
		in:   "(m2.M) = nil",
		want: map[Level]string{
			Green: "m2.ClearM()",
		}}, {
		desc: "proto2: clear message 2",
		in:   "((m2.M)) = nil",
		want: map[Level]string{
			Green: "m2.ClearM()",
		}}, {
		desc: "proto2: clear parenthesized scalar slice",
		in:   "(m2.Is) = nil",
		want: map[Level]string{
			Green: "m2.SetIs(nil)",
		}}, {
		desc: "proto2: clear parenthesized message slice",
		in:   "(m2.Ms) = nil",
		want: map[Level]string{
			Green: "m2.SetMs(nil)",
		}}, {
		desc: "proto2: clear parenthesized oneof",
		in:   "(m2.OneofField) = nil",
		want: map[Level]string{
			Green: "m2.ClearOneofField()",
		}}, {
		desc: "proto2: get on lhs used for indexing",
		in: `
var ns []int
ns[*m2.I32] = 0
ns[(*m2.I32)] = 0
ns[*(m2.I32)] = 0
`,
		want: map[Level]string{
			Green: `
var ns []int
ns[m2.GetI32()] = 0
ns[m2.GetI32()] = 0
ns[m2.GetI32()] = 0
`,
		}}, {
		desc: "proto2: get on lhs used for indexing",
		in: `
var ns []int
ns[m3.I32] = 0
ns[(m3.I32)] = 0
`,
		want: map[Level]string{
			Green: `
var ns []int
ns[m3.GetI32()] = 0
ns[m3.GetI32()] = 0
`,
		}}, {
		desc: "proto3: clear message",
		in:   "(m3.M) = nil",
		want: map[Level]string{
			Green: "m3.ClearM()",
		}}, {
		desc: "proto3: clear message 3",
		in:   "((m3.M)) = nil",
		want: map[Level]string{
			Green: "m3.ClearM()",
		}}, {
		desc: "proto3: clear parenthesized scalar slice",
		in:   "(m3.Is) = nil",
		want: map[Level]string{
			Green: "m3.SetIs(nil)",
		}}, {
		desc: "proto3: clear parenthesized message slice",
		in:   "(m3.Ms) = nil",
		want: map[Level]string{
			Green: "m3.SetMs(nil)",
		}}, {
		desc: "proto3: clear parenthesized oneof",
		in:   "(m3.OneofField) = nil",
		want: map[Level]string{
			Green: "m3.ClearOneofField()",
		}},
	})

	runTableTests(t, tests)
}

func TestPreserveComments(t *testing.T) {
	tests := []test{
		{
			desc: "comments around CompositeLit",
			in: `
// Comment above.
_ = &pb2.M2{S:nil} // Inline comment.
// Comment below.
`,
			want: map[Level]string{
				Green: `
// Comment above.
_ = pb2.M2_builder{S: nil}.Build() // Inline comment.
// Comment below.
`,
			},
		},

		{
			desc: "comments in CompositeLit",
			in: `
_ = &pb2.M2{ // Comment here.
	// Comment above field S
	S: nil, // Inside literal.
} // After.
`,
			want: map[Level]string{
				Green: `
_ = pb2.M2_builder{ // Comment here.
	// Comment above field S
	S: nil, // Inside literal.
}.Build() // After.
`,
			},
		},

		{
			desc:     "CompositeLit to setters",
			srcfiles: []string{"nontest.go"},
			in: `
_ = &pb2.M2{ // Comment here.
	// Comment above field S
	S: nil, // Inside literal.
	// Comment above nested message M.
	M: &pb2.M2{
		// Comment above field I32 in nested M.
		I32: proto.Int32(32),
	},
} // After.
`,
			want: map[Level]string{
				Green: `
m2h2 := &pb2.M2{}
// Comment above field I32 in nested M.
m2h2.SetI32(32)
m2h3 := &pb2.M2{ // Comment here.
}
// Comment above field S
m2h3.ClearS() // Inside literal.
// Comment above nested message M.
m2h3.SetM(m2h2)
_ = m2h3 // After.
`,
			},
		},

		{
			desc: "set clear has",
			in: `
// Comment 1
// More comments
m2.S = proto.String("hello")  // Comment 2
// Comment 3
m2.S = nil // Comment 4
// Comment 5
_ = m2.S != nil // Comment 6
// Comment 7
_ = m2.S == nil // Comment 8
// Comment 9
`,
			want: map[Level]string{
				Green: `
// Comment 1
// More comments
m2.SetS("hello") // Comment 2
// Comment 3
m2.ClearS() // Comment 4
// Comment 5
_ = m2.HasS() // Comment 6
// Comment 7
_ = !m2.HasS() // Comment 8
// Comment 9
`,
			},
		},

		{
			desc: "multi-assign",
			in: `
var n int
_ = n
// Comment 1
n, m2.B, m2.S = 42, proto.Bool(true), proto.String("s") // Comment 2
// Comment 3
`,
			want: map[Level]string{
				Yellow: `
var n int
_ = n
// Comment 1
n = 42
m2.SetB(true)
m2.SetS("s") // Comment 2
// Comment 3
`,
			},
		},

		{
			desc: "multi-line msg slices",
			in: `
_ = []*pb2.M2{
	// Comment 1
	&pb2.M2{}, // Comment 2
	&pb2.M2{M: nil}, // Comment 3
	// Comment 4
	&pb2.M2{ // Comment 5
		// Comment 6
		M: nil, // Comment 7
	}, // Comment 8
	// Comment 9
}

_ = []*pb3.M3{
	// Comment 1
	{}, // Comment 2
	{B: true}, // Comment 3
	// Comment 4
	{ // Comment 5
		// Comment 6
		S: "hello", // Comment 7
		// Comment 8
	}, // Comment 9
}
`,
			want: map[Level]string{
				Green: `
_ = []*pb2.M2{
	// Comment 1
	&pb2.M2{},                      // Comment 2
	pb2.M2_builder{M: nil}.Build(), // Comment 3
	// Comment 4
	pb2.M2_builder{ // Comment 5
		// Comment 6
		M: nil, // Comment 7
	}.Build(), // Comment 8
	// Comment 9
}

_ = []*pb3.M3{
	// Comment 1
	{},                              // Comment 2
	pb3.M3_builder{B: true}.Build(), // Comment 3
	// Comment 4
	pb3.M3_builder{ // Comment 5
		// Comment 6
		S: "hello", // Comment 7
		// Comment 8
	}.Build(), // Comment 9
}
`,
			},
		},

		{
			desc: "if init simple statement",
			in: `
// Comment 1
if m3.S, m3.M = "", (&pb3.M3{}); m3.B { // Comment 2
	// Comment 3
	m3.B, m3.Is = true, nil  // Comment 4
	// Comment 5
}
// Comment 6
`,
			want: map[Level]string{
				Yellow: `
m3.SetS("")
m3.SetM(&pb3.M3{})

// Comment 1
if m3.GetB() { // Comment 2
	// Comment 3
	m3.SetB(true)
	m3.SetIs(nil) // Comment 4
	// Comment 5
}
// Comment 6
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestAllowRenamingProtoPackage(t *testing.T) {
	in := `package p

import pb2 "google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto"
import protolib "google.golang.org/protobuf/proto"

var _ = protolib.String

func f() {
  m := &pb2.M2{}
  _ = "TEST CODE STARTS HERE"
  _ = m.S
  _ = "TEST CODE ENDS HERE"
}
`
	want := `_ = protolib.ValueOrNil(m.HasS(), m.GetS)`

	got, _, err := fixSource(context.Background(), in, "pkg_test.go", ConfiguredPackage{}, []Level{Green, Yellow, Red})
	if err != nil {
		t.Fatalf("fixSource() failed: %v\nFull source:\n%s\n------------------------------", err, in)
	}
	if d := diff.Diff(want, got[Red]); d != "" {
		t.Errorf("fixSource(%q) = (red) %q; want %s\ndiff:\n%s\n", in, got, want, d)
	}
}

func TestAddsProtoImport(t *testing.T) {
	// The m2.M assignment will be rewritten to:
	//
	// m2.SetM(pb2.M2_builder{S: proto.String(m2.GetS())}.Build())
	//
	// Because the package does not currently import the proto package,
	// a new proto import should be added.
	const src = `// 	_ = "TEST CODE STARTS HERE"
package p

import pb2 "google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto"

func test_function() {
	m2 := new(pb2.M2)
	m2.M = pb2.M2_builder{S: m2.S}.Build()
	_ = "TEST CODE ENDS HERE"
}
`

	gotAll, _, err := fixSource(context.Background(), src, "code.go", ConfiguredPackage{}, []Level{Green, Yellow, Red})
	if err != nil {
		t.Fatalf("fixSource() failed: %v; Full input:\n%s", err, src)
	}
	got := gotAll[Red]
	if !strings.Contains(got, "google.golang.org/protobuf/proto") {
		t.Fatalf("proto import not added: %q", got)
	}
}
