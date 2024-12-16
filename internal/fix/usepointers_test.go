// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestEnumPointer(t *testing.T) {
	tests := []test{
		{
			desc:     "enum (value)",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{
	E: pb2.M2_E_VAL.Enum(),
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
mypb.SetE(pb2.M2_E_VAL)
_ = mypb
`,
			},
		},

		{
			desc:     "enum (pointer)",
			srcfiles: []string{"pkg.go"},
			in: `
ptr := pb2.M2_E_VAL.Enum()
mypb := &pb2.M2{
	E: ptr,
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
ptr := pb2.M2_E_VAL.Enum()
mypb := &pb2.M2{}
if ptr != nil {
	mypb.SetE(*ptr)
}
_ = mypb
`,
			},
		},

		{
			desc:     "enum (Enum() on pointer)",
			srcfiles: []string{"pkg.go"},
			in: `
ptr := pb2.M2_E_VAL.Enum()
mypb := &pb2.M2{
	E: ptr.Enum(),
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
ptr := pb2.M2_E_VAL.Enum()
mypb := &pb2.M2{}
mypb.SetE(*ptr)
_ = mypb
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestUsePointersNotValues(t *testing.T) {
	tests := []test{
		{
			desc:  "simple example",
			extra: `func f(*pb2.M2) {}`,
			in: `
m := pb2.M2{S:nil}
f(&m)
`,
			want: map[Level]string{
				Green: `
m := pb2.M2{S: nil}
f(&m)
`,
				Red: `
m := pb2.M2_builder{S: nil}.Build()
f(m)
`,
			},
		},

		{
			desc: "empty literal",
			in: `
m := pb2.M2{}
_ = &m
`,
			want: map[Level]string{
				Green: `
m := pb2.M2{}
_ = &m
`,
				Red: `
m := &pb2.M2{}
_ = m
`,
			},
		},

		{
			desc:  "method calls",
			extra: `func f(*pb2.M2) {}`,
			in: `
m := pb2.M2{S:nil}
_ = m.GetS()
f(&m)
`,
			want: map[Level]string{
				Green: `
m := pb2.M2{S: nil}
_ = m.GetS()
f(&m)
`,
				Red: `
m := pb2.M2_builder{S: nil}.Build()
_ = m.GetS()
f(m)
`,
			},
		},

		{
			desc:  "direct field accesses",
			extra: `func f(*pb2.M2) {}`,
			in: `
m := pb2.M2{S: nil}
m.S = proto.String("")
m.S = nil
f(&m)
`,
			want: map[Level]string{
				Green: `
m := pb2.M2{S: nil}
m.SetS("")
m.ClearS()
f(&m)
`,
				Red: `
m := pb2.M2_builder{S: nil}.Build()
m.SetS("")
m.ClearS()
f(m)
`,
			},
		},

		{
			desc: "addr is stored",
			in: `
m := pb2.M2{S: nil}
m.M = &m
`,
			want: map[Level]string{
				Green: `
m := pb2.M2{S: nil}
m.SetM(&m)
`,
				Red: `
m := pb2.M2_builder{S: nil}.Build()
m.SetM(m)
`,
			},
		},

		{
			desc:  "argument to function printer",
			extra: `func fmtPrintln(string, ...interface{})`,
			in: `
m := pb2.M2{S: nil}
fmtPrintln("", m)
`,
			want: map[Level]string{
				Green: `
m := pb2.M2{S: nil}
fmtPrintln("", m)
`,
				Red: `
m := pb2.M2_builder{S: nil}.Build()
fmtPrintln("", m)
`,
			},
		},

		{
			desc: "argument to method printer",
			extra: `
type T struct{}
func (*T) println(string, ...interface{})
var t *T
`,
			in: `
m := pb2.M2{S: nil}
t.println("", m)
`,
			want: map[Level]string{
				Green: `
m := pb2.M2{S: nil}
t.println("", m)
`,
				Red: `
m := pb2.M2_builder{S: nil}.Build()
t.println("", m)
`,
			},
		},

		{
			desc: "array of values",
			in: `
ms := []pb2.M2{{S: nil}, pb2.M2{S: nil}, {}}
_ = ms
`,
			want: map[Level]string{
				Red: `
// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)
ms := []pb2.M2{&{S: nil}, pb2.M2_builder{S: nil}.Build(), &{}}
_ = ms
`,
			},
		},

		{
			desc:  "array value",
			extra: `func f(*pb2.M2) {}`,
			in: `
m := pb2.M2{}
ms := []pb2.M2{m}
f(&m)
_ = ms
`,
			want: map[Level]string{
				Red: `
// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)
m := &pb2.M2{}
ms := []pb2.M2{m}
f(&m)
_ = ms
`,
			},
		},

		{
			desc: "shallow copy, func call",
			extra: `
func f(*pb2.M2) {}
func g(pb2.M2) {}`,
			in: `
m := pb2.M2{S: nil}
f(&m)
g(m)
`,
			want: map[Level]string{
				Red: `
// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)
m := pb2.M2_builder{S: nil}.Build()
f(&m)
g(m)
`,
			},
		},

		{
			desc: "shallow copy, return value",
			extra: `
func f(*pb2.M2) {}
func g(pb2.M2) {}`,
			in: `
m := func() pb2.M2 {
	return pb2.M2{S: nil}
}()
f(&m)
g(m)
`,
			want: map[Level]string{
				Red: `
m := func() *pb2.M2 {
	// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)
	return pb2.M2_builder{S: nil}.Build()
}()
f(&m)
g(m)
`,
			},
		},

		{
			desc:  "shallow copy, output arg",
			extra: `func f(*pb2.M2) {}`,
			in: `
func(out *pb2.M2) {
	m := pb2.M2{S: nil}
	f(&m)
	*out = m
}(m2)
`,
			want: map[Level]string{
				Red: `
func(out *pb2.M2) {
	m := pb2.M2_builder{S: nil}.Build()
	f(m)
	proto.Reset(out)
	proto.Merge(out, m)
}(m2)
`,
			},
		},

		{
			desc:     "shallow copy, direct assignment to output arg",
			srcfiles: []string{"pkg.go"},
			in: `
var out *pb2.M2
*out = pb2.M2{S: nil}
`,
			want: map[Level]string{
				Green: `
var out *pb2.M2
proto.Reset(out)
m2h2 := &pb2.M2{}
m2h2.ClearS()
proto.Merge(out, m2h2)
`,
			},
		},

		{
			desc:     "shallow copy, conditional assignment to output arg",
			srcfiles: []string{"pkg.go"},
			in: `
var out *pb2.M2
if out != nil {
	*out = pb2.M2{S: nil}
}
`,
			want: map[Level]string{
				Green: `
var out *pb2.M2
if out != nil {
	proto.Reset(out)
	m2h2 := &pb2.M2{}
	m2h2.ClearS()
	proto.Merge(out, m2h2)
}
`,
			},
		},

		{
			desc:  "shallow copy, simple copy",
			extra: `func f(*pb2.M2) {}`,
			in: `
m := pb2.M2{S: nil}
var copy pb2.M2 = m
_ = copy
f(&m)
f(&copy)
`,
			want: map[Level]string{
				Red: `
// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)
m := pb2.M2_builder{S: nil}.Build()
var copy pb2.M2 = m
_ = copy
f(&m)
f(&copy)
`,
			},
		},

		{
			desc: "shallow copy, reassigned",
			extra: `
func f(*pb2.M2) { }
func g() pb2.M2{ return pb2.M2{} }
`,
			in: `
// existing comment to illustrate comment addition
m := pb2.M2{S: nil}
m = g()
f(&m)
`,
			want: map[Level]string{
				Red: `
// existing comment to illustrate comment addition
// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)
m := pb2.M2_builder{S: nil}.Build()
m = g()
f(&m)
`,
			},
		},

		{
			desc: "struct field definition",
			in: `
for _, tt := range []struct {
	want pb2.M2
}{
	{
		want: pb2.M2{S: nil},
	},
} {
	_ = tt.want
}
`,
			want: map[Level]string{
				Red: `
// DO NOT SUBMIT: fix callers to work with a pointer (go/goprotoapi-findings#message-value)
for _, tt := range []struct {
	want *pb2.M2
}{
	{
		want: pb2.M2_builder{S: nil}.Build(),
	},
} {
	_ = tt.want
}
`,
			},
		},

		{
			desc: "Stubby method handler response assignment",
			in: `
func(ctx context.Context, req *pb2.M2, resp *pb2.M2) error {
	*resp = pb2.M2{S: nil}
	return nil
}(context.Background(), nil, nil)
`,
			want: map[Level]string{
				Red: `
func(ctx context.Context, req *pb2.M2, resp *pb2.M2) error {
	proto.Merge(resp, pb2.M2_builder{S: nil}.Build())
	return nil
}(context.Background(), nil, nil)
`,
			},
		},

		{
			desc: "Stubby method handler, mutating response assignment",
			in: `
func(ctx context.Context, req *pb2.M2, resp *pb2.M2) error {
	*resp = pb2.M2{I32: proto.Int32(42)}
	if true {
		*resp = pb2.M2{S: nil}
	}
	return nil
}(context.Background(), nil, nil)
`,
			want: map[Level]string{
				Red: `
func(ctx context.Context, req *pb2.M2, resp *pb2.M2) error {
	proto.Merge(resp, pb2.M2_builder{I32: proto.Int32(42)}.Build())
	if true {
		proto.Reset(resp)
		proto.Merge(resp, pb2.M2_builder{S: nil}.Build())
	}
	return nil
}(context.Background(), nil, nil)
`,
			},
		},

		{
			desc: "Stubby method handler response assignment with comments",
			in: `
var response *pb2.M2
// above response assignment
*response = pb2.M2{
	// above field
	S: nil, // end of field line
} // end of message line
`,
			want: map[Level]string{
				Red: `
var response *pb2.M2
// above response assignment
proto.Reset(response)
proto.Merge(response, pb2.M2_builder{
	// above field
	S: nil, // end of field line
}.Build()) // end of message line
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestNoUnconditionalDereference(t *testing.T) {
	// Before b/259702553, the open2opaque tool unconditionally rewrote code
	// such that it de-referenced pointers.

	tests := []test{
		{
			desc:  "assignment",
			extra: `func funcReturningIntPointer() *int32 { return nil }`,
			in: `
m2.I32 = funcReturningIntPointer()
`,
			want: map[Level]string{
				Red: `
if x := funcReturningIntPointer(); x != nil {
	m2.SetI32(*x)
} else {
	m2.ClearI32()
}
`,
			},
		},

		{
			desc: "struct literal field",
			in: `
_ = &pb2.M2{
	I32: m2a.I32,
}
`,
			want: map[Level]string{
				Red: `
_ = pb2.M2_builder{
	I32: proto.ValueOrNil(m2a.HasI32(), m2a.GetI32),
}.Build()
`,
			},
		},
	}

	runTableTests(t, tests)
}
