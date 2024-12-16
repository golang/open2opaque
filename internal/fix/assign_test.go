// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestEliminateClearer(t *testing.T) {
	tests := []test{
		{
			desc:     "basic has/set without clearer",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{
	I32: m2.I32,
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
if m2.HasI32() {
	mypb.SetI32(m2.GetI32())
}
_ = mypb
`,
			},
		},

		{
			desc:     "helper and has/set without clearer",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{
	M: &pb2.M2{
		I32: m2.I32,
	},
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
m2h2 := &pb2.M2{}
if m2.HasI32() {
	m2h2.SetI32(m2.GetI32())
}
mypb := &pb2.M2{}
mypb.SetM(m2h2)
_ = mypb
`,
			},
		},

		{
			desc:     "clearer not safe: function call instead of empty composite literal",
			srcfiles: []string{"pkg.go"},
			extra: `
func preparedProto() *pb2.M2 {
	result := &pb2.M2{}
	result.SetI32(42)
	return result
}
`,
			in: `
mypb := preparedProto()
mypb.I32 = m2.I32
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := preparedProto()
if m2.HasI32() {
	mypb.SetI32(m2.GetI32())
} else {
	mypb.ClearI32()
}
_ = mypb
`,
			},
		},

		{
			desc:     "scope: clearer not safe: function call outside of scope",
			srcfiles: []string{"pkg.go"},
			extra: `
func preparedProto() *pb2.M2 {
	result := &pb2.M2{}
	result.SetI32(42)
	return result
}
`,
			in: `
mypb := preparedProto()
if mypb.GetI32() > 0 {
	mypb.I32 = m2.I32
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := preparedProto()
if mypb.GetI32() > 0 {
	if m2.HasI32() {
		mypb.SetI32(m2.GetI32())
	} else {
		mypb.ClearI32()
	}
}
_ = mypb
`,
			},
		},

		{
			desc:     "scope: message used inside condition body",
			srcfiles: []string{"pkg.go"},
			extra: `
func externalCondition() bool { return true }
`,
			in: `
mypb := &pb2.M2{}
if externalCondition() {
	mypb.I32 = m2.I32
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
if externalCondition() {
	if m2.HasI32() {
		mypb.SetI32(m2.GetI32())
	} else {
		mypb.ClearI32()
	}
}
_ = mypb
`,
			},
		},

		{
			desc:     "scope: clearer not safe due to intermediate proto.Merge",
			srcfiles: []string{"pkg.go"},
			extra: `
func externalCondition() bool { return true }
`,
			in: `
mypb := &pb2.M2{}
if externalCondition() {
	proto.Merge(mypb, m2)
}
mypb.I32 = m2.I32
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
if externalCondition() {
	proto.Merge(mypb, m2)
}
if m2.HasI32() {
	mypb.SetI32(m2.GetI32())
} else {
	mypb.ClearI32()
}
_ = mypb
`,
			},
		},

		{
			desc:     "scope: shadowing",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{}
proto.Merge(mypb, m2)
mypb.I32 = m2.I32
{
	mypb := &pb2.M2{}
	mypb.I32 = m2.I32
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
proto.Merge(mypb, m2)
if m2.HasI32() {
	mypb.SetI32(m2.GetI32())
} else {
	mypb.ClearI32()
}
{
	mypb := &pb2.M2{}
	if m2.HasI32() {
		mypb.SetI32(m2.GetI32())
	}
}
_ = mypb
`,
			},
		},

		{
			desc:     "plain usage",
			extra:    `func f() *int32 {return nil }`,
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{}
mypb.I32 = f()
mypb.I32 = m2.I32
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
mypb.I32 = f()
if m2.HasI32() {
	mypb.SetI32(m2.GetI32())
} else {
	mypb.ClearI32()
}
_ = mypb
`,
			},
		},

		{
			desc:     "setter",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{}
mypb.SetI32(int32(42))
mypb.I32 = m2.I32
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
mypb.SetI32(int32(42))
if m2.HasI32() {
	mypb.SetI32(m2.GetI32())
} else {
	mypb.ClearI32()
}
_ = mypb
`,
			},
		},

		{
			desc:     "direct field assignment",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{}
mypb.I32 = proto.Int32(int32(42))
mypb.I32 = m2.I32
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
mypb.SetI32(int32(42))
if m2.HasI32() {
	mypb.SetI32(m2.GetI32())
} else {
	mypb.ClearI32()
}
_ = mypb
`,
			},
		},

		{
			desc:     "conditional initialization",
			srcfiles: []string{"pkg.go"},
			in: `
_ = func(m *pb2.M2) {
	if m == nil {
		m = &pb2.M2{}
	}
	m.I32 = m2.I32
}
`,
			want: map[Level]string{
				Green: `
_ = func(m *pb2.M2) {
	if m == nil {
		m = &pb2.M2{}
	}
	if m2.HasI32() {
		m.SetI32(m2.GetI32())
	} else {
		m.ClearI32()
	}
}
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestProtoToProtoAssignWhenUpdatingOnlyOne(t *testing.T) {
	// Before b/266919153, open2opaque would incorrectly not apply some of its
	// rewrites when only part of an expression was matched by
	// -types_to_update. For example, an assignment m2.I32 = other.I32 would
	// only get rewritten correctly if the left *and* right side were in
	// -types_to_update.

	tests := []test{
		{
			desc:          "int32",
			typesToUpdate: map[string]bool{"google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto.OtherProto2": true},
			in: `
other := new(pb2.OtherProto2)
m2.I32 = other.I32
`,
			want: map[Level]string{
				Red: `
other := new(pb2.OtherProto2)
if other.HasI32() {
	m2.SetI32(other.GetI32())
} else {
	m2.ClearI32()
}
`,
			},
		},

		{
			desc:          "int32, sides reversed",
			typesToUpdate: map[string]bool{"google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto.OtherProto2": true},
			in: `
other := new(pb2.OtherProto2)
other.I32 = m2.I32
`,
			want: map[Level]string{
				Red: `
other := new(pb2.OtherProto2)
if m2.I32 != nil {
	other.SetI32(*m2.I32)
} else {
	other.ClearI32()
}
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestRemoveLinebreakForShortLines(t *testing.T) {
	tests := []test{
		{
			desc:     "short line",
			srcfiles: []string{"pkg.go"},
			extra:    `func shortName(*pb2.M2) {}`,
			in: `
shortName(
	&pb2.M2{
		I32: proto.Int32(42),
	},
)
`,
			want: map[Level]string{
				Green: `
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
shortName(m2h2)
`,
			},
		},

		{
			desc:     "short line, multiple parameters",
			srcfiles: []string{"pkg.go"},
			extra:    `func shortName(*pb2.M2, *pb2.M2) {}`,
			in: `
shortName(
	&pb2.M2{
		I32: proto.Int32(42),
	},
	&pb2.M2{
		I32: proto.Int32(42),
	},
)
`,
			want: map[Level]string{
				Green: `
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
m2h3 := &pb2.M2{}
m2h3.SetI32(42)
shortName(m2h2, m2h3)
`,
			},
		},

		{
			desc:     "short line, append",
			srcfiles: []string{"pkg.go"},
			in: `
var result []*pb2.M2
result = append(result,
	&pb2.M2{
		I32: proto.Int32(42),
	})
`,
			want: map[Level]string{
				Green: `
var result []*pb2.M2
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
result = append(result, m2h2)
`,
			},
		},

		{
			desc:     "long line",
			srcfiles: []string{"pkg.go"},
			extra:    `func veryLongFunctionNameWhichIfYouCombineItWithItsArgumentsWillLikelyNotComfortablyFitIntoOneLine(*pb2.M2) {}`,
			in: `
veryLongFunctionNameWhichIfYouCombineItWithItsArgumentsWillLikelyNotComfortablyFitIntoOneLine(
	&pb2.M2{
		I32: proto.Int32(42),
	},
)
`,
			want: map[Level]string{
				Green: `
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
veryLongFunctionNameWhichIfYouCombineItWithItsArgumentsWillLikelyNotComfortablyFitIntoOneLine(
	m2h2,
)
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestMultiAssign(t *testing.T) {
	tests := []test{{
		desc: "no rewrite when there are no protos involved",
		extra: `
type NotAProto struct {
  S *string
  Field struct{}
}
var a, b *NotAProto
func g() *string { return nil }
`,
		in: "a.S, b.S = nil, g()",
		want: map[Level]string{
			Red: "a.S, b.S = nil, g()",
		},
	}, {
		desc: "multi-clear",
		in: `
m2.B, m2.S, m2.Is, m2.Ms, m2.M, m2.Map = nil, nil, nil, nil, nil, nil
m3.Is, m3.Ms, m3.M, m3.Map = nil, nil, nil, nil
`,
		want: map[Level]string{
			Green: `
m2.B, m2.S, m2.Is, m2.Ms, m2.M, m2.Map = nil, nil, nil, nil, nil, nil
m3.Is, m3.Ms, m3.M, m3.Map = nil, nil, nil, nil
`,
			Yellow: `
m2.ClearB()
m2.ClearS()
m2.SetIs(nil)
m2.SetMs(nil)
m2.ClearM()
m2.SetMap(nil)
m3.SetIs(nil)
m3.SetMs(nil)
m3.ClearM()
m3.SetMap(nil)
`,
		},
	}, {
		desc: "multi-assign",
		in: `
m2.B, m2.S, m2.Is, m2.Ms, m2.M = proto.Bool(true), proto.String("s"), []int32{1}, []*pb2.M2{{},{}}, &pb2.M2{}
m3.Is, m3.Ms, m3.M = []int32{1}, []*pb3.M3{{},{}}, &pb3.M3{}
`,
		want: map[Level]string{
			Green: `
m2.B, m2.S, m2.Is, m2.Ms, m2.M = proto.Bool(true), proto.String("s"), []int32{1}, []*pb2.M2{{}, {}}, &pb2.M2{}
m3.Is, m3.Ms, m3.M = []int32{1}, []*pb3.M3{{}, {}}, &pb3.M3{}
`,
			Yellow: `
m2.SetB(true)
m2.SetS("s")
m2.SetIs([]int32{1})
m2.SetMs([]*pb2.M2{{}, {}})
m2.SetM(&pb2.M2{})
m3.SetIs([]int32{1})
m3.SetMs([]*pb3.M3{{}, {}})
m3.SetM(&pb3.M3{})
`,
		},
	}, {
		desc: "multi-assign mixed with non-proto",
		in: `
var n int
_ = n
m2.S, m2.S, n, m2.M = proto.String("s"), nil, 42, &pb2.M2{}
`,
		want: map[Level]string{
			Green: `
var n int
_ = n
m2.S, m2.S, n, m2.M = proto.String("s"), nil, 42, &pb2.M2{}
`,
			Yellow: `
var n int
_ = n
m2.SetS("s")
m2.ClearS()
n = 42
m2.SetM(&pb2.M2{})
`,
		},
	}, {
		desc: "set bytes field",
		in: `
var b []byte
var x bool
m2.Bytes, x = b, true
_ = x
`,
		want: map[Level]string{
			Green: `
var b []byte
var x bool
m2.Bytes, x = b, true
_ = x
`,
			Yellow: `
var b []byte
var x bool
if b != nil {
	m2.SetBytes(b)
} else {
	m2.ClearBytes()
}
x = true
_ = x
`,
		},
	}, {
		desc: "proto3",
		in: `
m3.S, m3.M, m3.Is = "", &pb3.M3{}, []int32{1}
`,
		want: map[Level]string{
			Green: `m3.S, m3.M, m3.Is = "", &pb3.M3{}, []int32{1}`,
			Yellow: `
m3.SetS("")
m3.SetM(&pb3.M3{})
m3.SetIs([]int32{1})
`,
		},
	}, {
		// Skipped: single multi-valued expressions are not supported yet
		desc: "single multi-valued expression, maps",
		in: `
m := map[int]string{}

m3.S, m3.B = m[1]

var ok bool
_ = ok
m3.S, ok = m[1]
`,
		want: map[Level]string{
			Red: `
m := map[int]string{}

m3.S, m3.B = m[1]

var ok bool
_ = ok
m3.S, ok = m[1]
`,
		},
	}, {
		// Skipped: single multi-valued expressions are not supported yet
		desc: "single multi-valued expression, maps",
		in: `
var s interface{} = "s"

m3.S, m3.B = s.(string)

var ok bool
_ = ok
m3.S, ok = s.(string)
`,
		want: map[Level]string{
			Red: `
var s interface{} = "s"

m3.S, m3.B = s.(string)

var ok bool
_ = ok
m3.S, ok = s.(string)
`,
		},
	}, {
		// Skipped: single multi-valued expressions are not supported yet
		desc:  "single multi-valued expression, maps",
		extra: `func g() (string, *bool, bool) { return "", nil, false } `,
		in: `
var ok bool
m3.S, m2.B, ok = g()
_ = ok
`,
		want: map[Level]string{
			Red: `
var ok bool
m3.S, m2.B, ok = g()
_ = ok
`,
		},
	}, {
		desc: "no rewrite for init simple statement when there are no protos involved",
		extra: `
type NotAProto struct {
  S *string
  Field struct{}
}
var a, b *NotAProto
func g() *string { return nil }
`,
		in: `if a.S, b.S = nil, g(); true {
}
`,
		want: map[Level]string{
			Red: `if a.S, b.S = nil, g(); true {
}
`,
		},
	}, {
		desc: "if init simple statement",
		in: `
if m3.S, m3.M = "", (&pb3.M3{}); m3.B {
	m3.B, m3.Is = true, nil
}
`,
		want: map[Level]string{
			Red: `
m3.SetS("")
m3.SetM(&pb3.M3{})

if m3.GetB() {
	m3.SetB(true)
	m3.SetIs(nil)
}
`,
		},
	}, {
		desc: "for init simple statement",
		in: `
for m3.S, m3.M = "", (&pb3.M3{}); m3.B; {
	m3.B, m3.Is = true, nil
}
`,
		want: map[Level]string{
			Green: `
for m3.S, m3.M = "", (&pb3.M3{}); m3.GetB(); {
	m3.B, m3.Is = true, nil
}
`,
			Yellow: `
m3.SetS("")
m3.SetM(&pb3.M3{})

for m3.GetB() {
	m3.SetB(true)
	m3.SetIs(nil)
}
`,
		},
	}, {
		desc: "simple for post statement",
		skip: "support multi-assignment in post-statements",
		in: `
var n int
for ; ; m2.S, m3.S = proto.String("s"), "s" {
	n++
}
`,
		want: map[Level]string{
			Green: `
var n int
for ; ; m2.S, m3.S = proto.String("s"), "s" {
	n++
}
`,
			Yellow: `
var n int
for {
	n++
	m2.SetS("s")
	m3.SetS("s")

}
`,
		},
	}, {
		desc: "for post statement + continue",
		skip: "support multi-assignment in post-statements",
		in: `
var n int
for ; ; m2.S, m3.S = proto.String("s"), "s" {
	n++
	if n % 2==0 {
		continue
	}
}
`,
		want: map[Level]string{
			Green: `
var n int
for ; ; m2.S, m3.S = proto.String("s"), "s" {
	n++
	if n%2 == 0 {
		continue
	}
}
`,
			Yellow: `
var n int
for {
	n++
	if n%2 == 0 {
		goto postStmt

	}
postStmt:
	m2.SetS("s")
	m3.SetS("s")

}
`,
		},
	}, {
		desc: "nested loops",
		skip: "support multi-assignment in post-statements",
		in: `
for ; ; m3.S, m3.B = "", false {
	continue
	for {
		continue
	}
}
`,
		want: map[Level]string{
			Green: `
for ; ; m3.S, m3.B = "", false {
	continue
	for {
		continue
	}
}
`,
			Yellow: `
for {
	goto postStmt

	for {
		continue
	}
postStmt:
	m3.SetS("")
	m3.SetB(false)

}
`,
		},
	}, {
		skip: "support nested rewritten loops with multi-assignment post-stmt",
		desc: "nested rewritten loops",
		in: `
for ; ; m3.S, m3.B = "", false {
	if true {
		continue
	}
	for  ; ; m3.S, m3.B = "", false{
		continue
	}
}
`,
		want: map[Level]string{
			Green: `
for ; ; m3.S, m3.B = "", false {
	if true {
		continue
	}
	for ; ; m3.S, m3.B = "", false {
		continue
	}
}
`,
			Yellow: `
for {
	if true {
		goto postStmt
	}
	for {
		goto postStmt2
	poststmt:
		m3.SetS("")
		m3.SetB(false)
	}
postStmt:
	m3.SetS("")
	m3.SetB(false)
}
`,
		},
	}, {
		// goto can't jump over declarations
		desc: "for post statement + continue + declarations",
		skip: "support multi-assignment in post-statements",
		in: `
var n int
for ; ; m2.S, m3.S = proto.String("s"), "s" {
	n++
	if n%2==0 {
		continue
	}
	m := 1
	_ = m
}
`,
		want: map[Level]string{
			Yellow: `
var n int
for ; ; m2.S, m3.S = proto.String("s"), "s" {
	n++
	if n%2 == 0 {
		continue
	}
	m := 1
	_ = m
}
`,
			// This is red because the resulting code is not very
			// readable. It's better if it's manually inspected and
			// rewritten.
			Red: `
var n int
for ; ; func() {
	m2.SetS("s")
	m3.SetS("s")
}() {
	n++
	if n%2 == 0 {
		continue
	}
	m := 1
	_ = m
}
`,
		},
	}, {
		desc: "multi-assign field deref",
		in: `
var v int
*m2.S, *m2.I32, v = "hello", 1, 42
_ = v
`,
		want: map[Level]string{
			Red: `
var v int
m2.SetS("hello")
m2.SetI32(1)
v = 42
_ = v
`,
		},
	}}

	runTableTests(t, tests)
}

func TestBuildToSetRewrite(t *testing.T) {
	const vars = `
var bytes []byte
var is []int32
var m2s []*pb2.M2
var m3s []*pb3.M3
var m map[string]bool

var b bool
var f32 float32
var f64 float64
var i32 int32
var i64 int64
var ui32 uint32
var ui64 uint64
var s string
var e2 pb2.M2_Enum
var e3 pb3.M3_Enum

var bPtr *bool
var f32Ptr *float32
var f64Ptr *float64
var i32Ptr *int32
var i64Ptr *int64
var ui32Ptr *uint32
var ui64Ptr *uint64
var sPtr *string
var e2Ptr *pb2.M2_Enum
`

	tests := []test{
		{
			desc:     "use builders in tests",
			srcfiles: []string{"code_test.go"},
			in: `
a := []*pb2.M2{
	&pb2.M2{S: nil},
}
_ = a

b := &pb2.M2{
	M: &pb2.M2{S: nil},
}
_ = b

c := &pb2.M2{
	M: &pb2.M2{
		S: nil,
	},
}
_ = c
`,
			want: map[Level]string{
				Yellow: `
a := []*pb2.M2{
	pb2.M2_builder{S: nil}.Build(),
}
_ = a

b := pb2.M2_builder{
	M: pb2.M2_builder{S: nil}.Build(),
}.Build()
_ = b

c := pb2.M2_builder{
	M: pb2.M2_builder{
		S: nil,
	}.Build(),
}.Build()
_ = c
`,
			},
		},

		{
			desc:     "use builders in codelabs",
			srcfiles: []string{"spanner_codelab.go"},
			in: `
a := []*pb2.M2{
	&pb2.M2{S: nil},
}
_ = a

b := &pb2.M2{
	M: &pb2.M2{S: nil},
}
_ = b

c := &pb2.M2{
	M: &pb2.M2{
		S: nil,
	},
}
_ = c
`,
			want: map[Level]string{
				Yellow: `
a := []*pb2.M2{
	pb2.M2_builder{S: nil}.Build(),
}
_ = a

b := pb2.M2_builder{
	M: pb2.M2_builder{S: nil}.Build(),
}.Build()
_ = b

c := pb2.M2_builder{
	M: pb2.M2_builder{
		S: nil,
	}.Build(),
}.Build()
_ = c
`,
			},
		},

		{
			desc:     "use builders if configured",
			srcfiles: []string{"code.go"},
			builderTypes: map[string]bool{
				"google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto.M2": true,
			},
			in: `
a := []*pb2.M2{
	&pb2.M2{S: nil},
}
_ = a
`,
			want: map[Level]string{
				Green: `
a := []*pb2.M2{
	pb2.M2_builder{S: nil}.Build(),
}
_ = a
`,
			},
		},

		{
			desc:     "use builders if too deeply nested",
			srcfiles: []string{"code.go"},
			in: `
_ = &pb2.M2{
	M: &pb2.M2{
		M: &pb2.M2{
			M: &pb2.M2{
			}, // 4 levels of nesting
		},
	},
}
		`,
			want: map[Level]string{
				Green: `
_ = pb2.M2_builder{
	M: pb2.M2_builder{
		M: pb2.M2_builder{
			M: &pb2.M2{}, // 4 levels of nesting
		}.Build(),
	}.Build(),
}.Build()
`,
			},
		},

		{
			desc:     "use builders (shallow nesting)",
			srcfiles: []string{"code.go"},
			in: `
_ = &pb2.M2{
	M: &pb2.M2{
		M: &pb2.M2{
			M: &pb2.M2{
			}, // 4 levels of nesting
		},
	},
	Ms: []*pb2.M2{
		&pb2.M2{
			I32: proto.Int32(23),
		}, // 3 levels of nesting
	},
}
`,
			want: map[Level]string{
				Green: `
_ = pb2.M2_builder{
	M: pb2.M2_builder{
		M: pb2.M2_builder{
			M: &pb2.M2{}, // 4 levels of nesting
		}.Build(),
	}.Build(),
	Ms: []*pb2.M2{
		pb2.M2_builder{
			I32: proto.Int32(23),
		}.Build(), // 3 levels of nesting
	},
}.Build()
`,
			},
		},

		{
			desc:     "use builders if too many messages are involved",
			srcfiles: []string{"code.go"},
			in: `
_ = &pb2.M2{
	Ms: []*pb2.M2{
		// four proto messages involved in the literal:
		&pb2.M2{I32: proto.Int32(23)},
		&pb2.M2{I32: proto.Int32(23)},
		&pb2.M2{I32: proto.Int32(23)},
		&pb2.M2{I32: proto.Int32(23)},
	},
}
		`,
			want: map[Level]string{
				Green: `
_ = pb2.M2_builder{
	Ms: []*pb2.M2{
		// four proto messages involved in the literal:
		pb2.M2_builder{I32: proto.Int32(23)}.Build(),
		pb2.M2_builder{I32: proto.Int32(23)}.Build(),
		pb2.M2_builder{I32: proto.Int32(23)}.Build(),
		pb2.M2_builder{I32: proto.Int32(23)}.Build(),
	},
}.Build()
`,
			},
		},

		{
			desc:     "builders for one-liners in tests",
			srcfiles: []string{"code_test.go"},
			in: `
a := &pb2.M2{S: nil}
_ = a

b := &pb2.M2{
	S: nil,
}
_ = b
`,
			want: map[Level]string{
				Yellow: `
a := pb2.M2_builder{S: nil}.Build()
_ = a

b := pb2.M2_builder{
	S: nil,
}.Build()
_ = b
`,
			},
		},

		{
			desc:     "setters for one-liners outside tests",
			srcfiles: []string{"code.go"},
			in: `
a := &pb2.M2{S:nil}
_ = a

b := &pb2.M2{
	S:nil,
}
_ = b
`,
			want: map[Level]string{
				Yellow: `
a := &pb2.M2{}
a.ClearS()
_ = a

b := &pb2.M2{}
b.ClearS()
_ = b
`,
			},
		},

		{
			desc:     "clit to non-builder: proto2: new objs",
			srcfiles: []string{"code.go"},
			in: `
mm2 := &pb2.M2{
	B:     proto.Bool(true),
	Bytes: []byte("hello"),
	F32:   proto.Float32(1),
	F64:   proto.Float64(2),
	I32:   proto.Int32(3),
	I64:   proto.Int64(4),
	Ui32:  proto.Uint32(5),
	Ui64:  proto.Uint64(6),
	S:     proto.String("world"),
	M:     &pb2.M2{},
	Is:    []int32{10, 11},
	Ms:    []*pb2.M2{{}, {}},
	Map:   map[string]bool{"a": true},
	E:     pb2.M2_E_VAL.Enum(),
}
_ = mm2`,
			want: map[Level]string{
				Yellow: `
mm2 := &pb2.M2{}
mm2.SetB(true)
mm2.SetBytes([]byte("hello"))
mm2.SetF32(1)
mm2.SetF64(2)
mm2.SetI32(3)
mm2.SetI64(4)
mm2.SetUi32(5)
mm2.SetUi64(6)
mm2.SetS("world")
mm2.SetM(&pb2.M2{})
mm2.SetIs([]int32{10, 11})
mm2.SetMs([]*pb2.M2{{}, {}})
mm2.SetMap(map[string]bool{"a": true})
mm2.SetE(pb2.M2_E_VAL)
_ = mm2
`,
			},
		},

		{
			desc:     "clit to non-builder: proto3: new objs",
			srcfiles: []string{"code.go"},
			in: `
mm3 := &pb3.M3{
	B:     true,
	Bytes: []byte("hello"),
	F32:   1,
	F64:   2,
	I32:   3,
	I64:   4,
	Ui32:  5,
	Ui64:  6,
	S:     "world",
	M:     &pb3.M3{},
	Is:    []int32{10, 11},
	Ms:    []*pb3.M3{{}, {}},
	Map:   map[string]bool{"a": true},
	E:     pb3.M3_E_VAL,
}
_ = mm3
`,
			want: map[Level]string{
				Yellow: `
mm3 := &pb3.M3{}
mm3.SetB(true)
mm3.SetBytes([]byte("hello"))
mm3.SetF32(1)
mm3.SetF64(2)
mm3.SetI32(3)
mm3.SetI64(4)
mm3.SetUi32(5)
mm3.SetUi64(6)
mm3.SetS("world")
mm3.SetM(&pb3.M3{})
mm3.SetIs([]int32{10, 11})
mm3.SetMs([]*pb3.M3{{}, {}})
mm3.SetMap(map[string]bool{"a": true})
mm3.SetE(pb3.M3_E_VAL)
_ = mm3
`,
			}},

		{
			desc:     "clit to non-builder: proto2 vars",
			srcfiles: []string{"code.go"},
			extra:    vars,
			in: `
mm2 := &pb2.M2{
	M:     m2,
	Is:    is,
	Ms:    m2s,
	Map:   m,
}
_ = mm2
`,
			want: map[Level]string{
				Yellow: `
mm2 := &pb2.M2{}
mm2.SetM(m2)
mm2.SetIs(is)
mm2.SetMs(m2s)
mm2.SetMap(m)
_ = mm2
`,
			},
		},

		{
			desc:     "clit to non-builder: proto3 vars",
			srcfiles: []string{"code.go"},
			extra:    vars,
			in: `
mm3 := &pb3.M3{
	B:     b,
	F32:   f32,
	F64:   f64,
	I32:   i32,
	I64:   i64,
	Ui32:  ui32,
	Ui64:  ui64,
	S:     s,
	M:     &pb3.M3{},
	Is:    is,
	Ms:    m3s,
	Map:   m,
	E:     e3,
}
_ = mm3
`,
			want: map[Level]string{
				Yellow: `
mm3 := &pb3.M3{}
mm3.SetB(b)
mm3.SetF32(f32)
mm3.SetF64(f64)
mm3.SetI32(i32)
mm3.SetI64(i64)
mm3.SetUi32(ui32)
mm3.SetUi64(ui64)
mm3.SetS(s)
mm3.SetM(&pb3.M3{})
mm3.SetIs(is)
mm3.SetMs(m3s)
mm3.SetMap(m)
mm3.SetE(e3)
_ = mm3
`}}, {
			desc:     "clit to non-builder: preserve proto2 presence from message",
			srcfiles: []string{"code.go"},
			extra:    vars,
			in: `
mm2 := &pb2.M2{
	Bytes: m2.Bytes, // eol comment
	B:     m2.B,
	F32:   m2.F32,
	F64:   m2.F64,
	I32:   m2.I32,
	I64:   m2.I64,
	Ui32:  m2.Ui32,
	Ui64:  m2.Ui64,
	S:     m2.S,
	E:     m2.E,
}
_ = mm2
`,
			want: map[Level]string{
				Yellow: `
mm2 := &pb2.M2{}
// eol comment
if x := m2.GetBytes(); x != nil {
	mm2.SetBytes(x)
}
if m2.HasB() {
	mm2.SetB(m2.GetB())
}
if m2.HasF32() {
	mm2.SetF32(m2.GetF32())
}
if m2.HasF64() {
	mm2.SetF64(m2.GetF64())
}
if m2.HasI32() {
	mm2.SetI32(m2.GetI32())
}
if m2.HasI64() {
	mm2.SetI64(m2.GetI64())
}
if m2.HasUi32() {
	mm2.SetUi32(m2.GetUi32())
}
if m2.HasUi64() {
	mm2.SetUi64(m2.GetUi64())
}
if m2.HasS() {
	mm2.SetS(m2.GetS())
}
if m2.HasE() {
	mm2.SetE(m2.GetE())
}
_ = mm2
`},
		},

		{
			desc:     "clit to non-builder: preserve proto2 presence from var",
			srcfiles: []string{"code.go"},
			extra:    vars,
			in: `
mm2 := &pb2.M2{
	Bytes: bytes,
	B:     bPtr,
	F32:   f32Ptr,
	F64:   f64Ptr,
	I32:   i32Ptr,
	I64:   i64Ptr,
	Ui32:  ui32Ptr,
	Ui64:  ui64Ptr,
	S:     sPtr,
	E:     e2Ptr,
}
_ = mm2
`,
			want: map[Level]string{
				Yellow: `
mm2 := &pb2.M2{}
if bytes != nil {
	mm2.SetBytes(bytes)
}
mm2.B = bPtr
mm2.F32 = f32Ptr
mm2.F64 = f64Ptr
mm2.I32 = i32Ptr
mm2.I64 = i64Ptr
mm2.Ui32 = ui32Ptr
mm2.Ui64 = ui64Ptr
mm2.S = sPtr
if e2Ptr != nil {
	mm2.SetE(*e2Ptr)
}
_ = mm2
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestBuildersAreGreenInTests(t *testing.T) {
	tt := test{
		in: `
_ = []*pb2.M2{
	&pb2.M2{S: nil},
}
`,
		want: map[Level]string{
			Green: `
_ = []*pb2.M2{
	pb2.M2_builder{S: nil}.Build(),
}
`,
		},
	}

	runTableTest(t, tt)
}

// TestAssignOperations tests that assignment operations like token.ADD_ASSIGN (+=) are rewritten.
func TestAssignOperations(t *testing.T) {
	tests := []test{
		{
			desc: "ADD_ASSIGN proto2 int32",
			in: `
*m2.I32 += 5
`,
			want: map[Level]string{
				Green: `
m2.SetI32(m2.GetI32() + 5)
`,
			},
		},
		{
			desc: "SUB_ASSIGN proto3 int64",
			in: `
m3.I64 -= 5
`,
			want: map[Level]string{
				Green: `
m3.SetI64(m3.GetI64() - 5)
`,
			},
		},
		{
			desc:     "never nil enum",
			srcfiles: []string{"pkg.go"},
			in: `
var b pb2.M2_Enum
m2.E = &b
`,
			want: map[Level]string{
				Green: `
var b pb2.M2_Enum
m2.SetE(b)
`,
			},
		},
		{
			desc: "QUO_ASSIGN proto2 float64 end-of-line-comment",
			in: `
*m2.F64 /= 5. // comment
`,
			want: map[Level]string{
				Green: `
m2.SetF64(m2.GetF64() / 5.) // comment
`,
			},
		},
		{
			desc: "AND_ASSIGN proto2 uint32 newline-before",
			in: `
_ = "something"

*m2.Ui32 &= 42
`,
			want: map[Level]string{
				Green: `
_ = "something"

m2.SetUi32(m2.GetUi32() & 42)
`,
			},
		},
		{
			desc: "SHL_ASSIGN proto3 uint64 newline-after",
			in: `
m3.Ui64 <<= 2

_ = "something"
`,
			want: map[Level]string{
				Green: `
m3.SetUi64(m3.GetUi64() << 2)

_ = "something"
`,
			},
		},
		{
			desc: "string-concatenation proto2",
			in: `
*m2.S = "hello "
*m2.S += "world!"
`,
			want: map[Level]string{
				Green: `
m2.SetS("hello ")
m2.SetS(m2.GetS() + "world!")
`,
			},
		},
		{
			desc: "non-proto DEFINE AssignStmt no-rewrite",
			in: `
x := 5
_ = x
`,
			want: map[Level]string{
				Green: `
x := 5
_ = x
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestNilCheck(t *testing.T) {
	tt := test{
		desc:  "side effect free expression",
		extra: `type mytype struct { e *pb2.M2_Enum }`,
		in: `
mt := mytype{}
m := &pb2.M2{}
m.E = mt.e
_ = m
`,
		want: map[Level]string{
			Green: `
mt := mytype{}
m := &pb2.M2{}
if mt.e != nil {
	m.SetE(*mt.e)
}
_ = m
`,
		},
	}

	runTableTest(t, tt)
}
