// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestConvertToSetter(t *testing.T) {
	tests := []test{
		{
			desc:     "assignment",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{
	I32: proto.Int32(23),
	M:   &pb2.M2{
		I32: proto.Int32(42),
	},
}
_ = mypb
for idx, val := range []int32{123, 456} {
	idx, val := idx, val
	go func() { println(idx, val) }()
}
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
mypb := &pb2.M2{}
mypb.SetI32(23)
mypb.SetM(m2h2)
_ = mypb
for idx, val := range []int32{123, 456} {
	idx, val := idx, val
	go func() { println(idx, val) }()
}
`,
			},
		},

		{
			desc:     "multi assignment",
			srcfiles: []string{"pkg.go"},
			in: `
my1, my2 := &pb2.M2{
	I32: proto.Int32(23),
}, &pb2.M2{
	I64: proto.Int64(12345),
}
_, _ = my1, my2
`,
			want: map[Level]string{
				Red: `
my1 := &pb2.M2{}
my1.SetI32(23)
my2 := &pb2.M2{}
my2.SetI64(12345)
_, _ = my1, my2
`,
			},
		},

		{
			desc:     "struct literal field",
			srcfiles: []string{"pkg.go"},
			in: `
func() *pb2.M2 {
	return &pb2.M2{
		I32: proto.Int32(23),
	}
}()
`,
			want: map[Level]string{
				Red: `
func() *pb2.M2 {
	m2h2 := &pb2.M2{}
	m2h2.SetI32(23)
	return m2h2
}()
`,
			},
		},

		{
			desc:     "nested builder",
			srcfiles: []string{"pkg.go"},
			in: `
_ = &pb2.M2{
	I32:   proto.Int32(23),
	M:     &pb2.M2{
		I32: proto.Int32(42),
	},
	Is:    []int32{10, 11},
}
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
m2h3 := &pb2.M2{}
m2h3.SetI32(23)
m2h3.SetM(m2h2)
m2h3.SetIs([]int32{10, 11})
_ = m2h3
`,
			},
		},

		{
			desc:     "if conditional",
			srcfiles: []string{"pkg.go"},
			extra:    `func validateProto(msg *pb2.M2) bool { return false }`,
			in: `
if msg := (&pb2.M2{
	I32:   proto.Int32(23),
	M:     &pb2.M2{
		I32: proto.Int32(42),
	},
	Is:    []int32{10, 11},
}); validateProto(msg) {
	println("validated")
}
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
m2h3 := &pb2.M2{}
m2h3.SetI32(23)
m2h3.SetM(m2h2)
m2h3.SetIs([]int32{10, 11})
if msg := (m2h3); validateProto(msg) {
	println("validated")
}
`,
			},
		},

		{
			desc:     "case clause",
			srcfiles: []string{"pkg.go"},
			extra:    `func validateProto(msg *pb2.M2) bool { return false }`,
			in: `
switch {
case validateProto(nil):
_ = &pb2.M2{
	I32:   proto.Int32(23),
}
}
`,
			want: map[Level]string{
				Red: `
switch {
case validateProto(nil):
	m2h2 := &pb2.M2{}
	m2h2.SetI32(23)
	_ = m2h2
}
`,
			},
		},

		{
			desc:     "select clause",
			srcfiles: []string{"pkg.go"},
			in: `
dummy := make(chan bool)
select {
case <-dummy:
_ = &pb2.M2{
	I32:   proto.Int32(23),
}
}
`,
			want: map[Level]string{
				Red: `
dummy := make(chan bool)
select {
case <-dummy:
	m2h2 := &pb2.M2{}
	m2h2.SetI32(23)
	_ = m2h2
}
`,
			},
		},

		{
			desc:     "never nil byte expression",
			srcfiles: []string{"pkg.go"},
			in: `
var b []byte
m2.Bytes = b[2:3]
m2.Bytes = b[:3]
m2.Bytes = b[2:]
m2.Bytes = b[:]
m2.Bytes = b[0:]
m2.Bytes = b[:0]
m2.Bytes = b[0:0]
`,
			want: map[Level]string{
				Green: `
var b []byte
m2.SetBytes(b[2:3])
m2.SetBytes(b[:3])
m2.SetBytes(b[2:])
if x := b[:]; x != nil {
	m2.SetBytes(x)
} else {
	m2.ClearBytes()
}
if x := b[0:]; x != nil {
	m2.SetBytes(x)
} else {
	m2.ClearBytes()
}
if x := b[:0]; x != nil {
	m2.SetBytes(x)
} else {
	m2.ClearBytes()
}
if x := b[0:0]; x != nil {
	m2.SetBytes(x)
} else {
	m2.ClearBytes()
}
`,
			},
		},

		{
			desc:     "variable declaration block",
			srcfiles: []string{"pkg.go"},
			in: `
var (
_ = &pb2.M2{
	I32:   proto.Int32(23),
}
)
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(23)
var (
	_ = m2h2
)
`,
			},
		},

		{
			desc:     "defer statement",
			srcfiles: []string{"pkg.go"},
			extra:    `func validateProto(msg *pb2.M2) bool { return false }`,
			in: `
defer validateProto(&pb2.M2{
	I32:   proto.Int32(23),
})
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(23)
defer validateProto(m2h2)
`,
			},
		},

		{
			desc:     "go statement",
			srcfiles: []string{"pkg.go"},
			extra:    `func validateProto(msg *pb2.M2) bool { return false }`,
			in: `
go validateProto(&pb2.M2{
	I32:   proto.Int32(23),
})
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(23)
go validateProto(m2h2)
`,
			},
		},

		{
			desc:     "for loop",
			srcfiles: []string{"pkg.go"},
			extra:    `func validateProto(msg *pb2.M2) bool { return false }`,
			in: `
for m2 := (&pb2.M2{
	I32:   proto.Int32(23),
}); m2 != nil; m2 = nil {
}
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(23)
for m2 := (m2h2); m2 != nil; m2 = nil {
}
`,
			},
		},

		{
			desc:     "range statement",
			srcfiles: []string{"pkg.go"},
			in: `
for _, idx := range []int{1, 2, 3} {
	_ = idx
	_ = &pb2.M2{
		I32:   proto.Int32(23),
	}
}
`,
			want: map[Level]string{
				Red: `
for _, idx := range []int{1, 2, 3} {
	_ = idx
	m2h2 := &pb2.M2{}
	m2h2.SetI32(23)
	_ = m2h2
}
`,
			},
		},

		{
			desc:     "labeled statement",
			srcfiles: []string{"pkg.go"},
			in: `
goto validate
println("this line is skipped")
validate:
println(&pb2.M2{
	I32:   proto.Int32(23),
})
`,
			want: map[Level]string{
				Red: `
	goto validate
	println("this line is skipped")
validate:
	m2h2 := &pb2.M2{}
	m2h2.SetI32(23)
	println(m2h2)
`,
			},
		},

		{
			desc:     "slice",
			srcfiles: []string{"pkg.go"},
			in: `
for _, msg := range []*pb2.M2{
	{
		I32: proto.Int32(23),
	},
	&pb2.M2{
		I64: proto.Int64(123),
	},
} {
	println(msg)
}
		`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetI32(23)
m2h3 := &pb2.M2{}
m2h3.SetI64(123)
for _, msg := range []*pb2.M2{
	m2h2,
	m2h3,
} {
	println(msg)
}
`,
			},
		},

		{
			desc:     "conditional assignment",
			srcfiles: []string{"pkg.go"},
			in: `
var mypb *pb2.M2
if true {
	mypb = &pb2.M2{
		I32: proto.Int32(23),
	}
}
_ = mypb
`,
			want: map[Level]string{
				Red: `
var mypb *pb2.M2
if true {
	mypb = &pb2.M2{}
	mypb.SetI32(23)
}
_ = mypb
`,
			},
		},

		{
			desc:     "assignment with different type",
			srcfiles: []string{"pkg.go"},
			in: `
var mypb proto.Message
if true {
	mypb = &pb2.M2{
		I32: proto.Int32(23),
	}
}
_ = mypb
`,
			want: map[Level]string{
				Green: `
var mypb proto.Message
if true {
	m2h2 := &pb2.M2{}
	m2h2.SetI32(23)
	mypb = m2h2
}
_ = mypb
`,
			},
		},

		{
			desc:     "slice with comments",
			srcfiles: []string{"pkg.go"},
			in: `
for _, msg := range []*pb2.M2{
	// Comment above first literal
	{
		I32: proto.Int32(23), // End-of-line comment for I32
	},
	// Comment above second literal
	&pb2.M2{
		// Comment above I64
		I64: proto.Int64(/* before 123 */ 123 /* after 123 */),
		// Comment at the end of the second literal
	}, // End-of-line comment after second literal
} {
	println(msg)
}
		`,
			want: map[Level]string{
				Red: `
// Comment above first literal
m2h2 := &pb2.M2{}
m2h2.SetI32(23) // End-of-line comment for I32
// Comment above second literal
m2h3 := &pb2.M2{}
// Comment above I64
m2h3.SetI64(123 /* after 123 */)
// Comment at the end of the second literal
for _, msg := range []*pb2.M2{
	m2h2,
	m2h3, // End-of-line comment after second literal
} {
	println(msg)
}
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestPointerDereference(t *testing.T) {

	tests := []test{
		{
			desc:     "assignment, lhs proto field dereference",
			srcfiles: []string{"pkg.go"},
			in: `
if val := m2.I32; val != nil {
	*val = int32(42)
}
`,
			want: map[Level]string{
				Red: `
if m2.HasI32() {
	m2.SetI32(int32(42))
}
`,
			},
		},

		{
			desc:     "multi-assign, lhs proto field dereference",
			srcfiles: []string{"pkg.go"},
			in: `
var i int
if val := m2.I32; val != nil {
	i, *val, i = 5, int32(42), 6
}
_ = i
`,
			want: map[Level]string{
				Red: `
var i int
if m2.HasI32() {
	i = 5
	m2.SetI32(int32(42))
	i = 6
}
_ = i
`,
			},
		},

		{
			desc:     "multiple assignments, lhs proto field dereference",
			srcfiles: []string{"pkg.go"},
			in: `
var i int
if val := m2.I32; val != nil {
	*val, i = int32(42), 5
	i, *val = 6, int32(43)
}
_ = i
`,
			want: map[Level]string{
				Red: `
var i int
if m2.HasI32() {
	m2.SetI32(int32(42))
	i = 5
	i = 6
	m2.SetI32(int32(43))
}
_ = i
`,
			},
		},

		{
			desc:     "multi-assign, lhs and rhs",
			srcfiles: []string{"pkg.go"},
			in: `
var i int
if val := m2.I32; val != nil {
	*val, i = *val+2, 5
	i, *val = 6, *val+5
}
_ = i
`,
			want: map[Level]string{
				Red: `
var i int
if m2.HasI32() {
	m2.SetI32(m2.GetI32() + 2)
	i = 5
	i = 6
	m2.SetI32(m2.GetI32() + 5)
}
_ = i
`,
			},
		},

		{
			desc:     "ifstmt: potential side effect initializer",
			srcfiles: []string{"pkg.go"},
			extra:    "func f(m *pb2.M2) *int32 { return m.I32 }",
			in: `
if val := f(m2); val != nil {
	*val = int32(42)
}
`,
			want: map[Level]string{
				Red: `
if val := f(m2); val != nil {
	*val = int32(42)
}
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestEnum(t *testing.T) {
	tests := []test{
		{
			desc:     "basic case",
			srcfiles: []string{"pkg.go"},
			in: `
_ = &pb2.M2{
	E: pb2.M2_E_VAL.Enum(),
}
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetE(pb2.M2_E_VAL)
_ = m2h2
`,
			},
		},

		{
			desc:     "free function with explicit receiver",
			srcfiles: []string{"pkg.go"},
			in: `
_ = &pb2.M2{
	E: pb2.M2_Enum.Enum(pb2.M2_E_VAL),
}
`,
			want: map[Level]string{
				Red: `
m2h2 := &pb2.M2{}
m2h2.SetE(pb2.M2_E_VAL)
_ = m2h2
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestNameGeneration(t *testing.T) {
	tests := []test{
		{
			desc:     "basic case",
			srcfiles: []string{"pkg.go"},
			in: `
m2h2 := 5
{
	_ = &pb2.M2{
		S: proto.String("Hello World!"),
	}
}
_ = m2h2
`,
			want: map[Level]string{
				Red: `
m2h2 := 5
{
	m2h3 := &pb2.M2{}
	m2h3.SetS("Hello World!")
	_ = m2h3
}
_ = m2h2
`,
			},
		},

		{
			desc:     "local definition",
			srcfiles: []string{"pkg.go"},
			in: `
if m2h2 := int32(5); m2h2 < 5 {
	_ = &pb2.M2{
		S:   proto.String("Hello World!"),
		I32: proto.Int32(m2h2),
	}
}
`,
			want: map[Level]string{
				Red: `
if m2h2 := int32(5); m2h2 < 5 {
	m2h3 := &pb2.M2{}
	m2h3.SetS("Hello World!")
	m2h3.SetI32(m2h2)
	_ = m2h3
}
`,
			},
		},

		{
			desc:     "sub message",
			srcfiles: []string{"pkg.go"},
			in: `
if op2 := (&pb2.OtherProto2{}); op2 != nil {
	_ = &pb2.OtherProto2{
		M:  &pb2.OtherProto2{},
		Ms: []*pb2.OtherProto2{op2},
	}
}
`,
			want: map[Level]string{
				Red: `
if op2 := (&pb2.OtherProto2{}); op2 != nil {
	op2h2 := &pb2.OtherProto2{}
	op2h2.SetM(&pb2.OtherProto2{})
	op2h2.SetMs([]*pb2.OtherProto2{op2})
	_ = op2h2
}
`,
			},
		},

		{
			desc:     "within if statement",
			srcfiles: []string{"pkg.go"},
			extra:    "func extra(*pb2.M2) bool { return true }",
			in: `
if extra(&pb2.M2{I32: proto.Int32(42)}) {
}

if extra(&pb2.M2{I32: proto.Int32(42)}) {
}
`,
			want: map[Level]string{
				Green: `
m2h2 := &pb2.M2{}
m2h2.SetI32(42)
if extra(m2h2) {
}

m2h3 := &pb2.M2{}
m2h3.SetI32(42)
if extra(m2h3) {
}
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestParentheses(t *testing.T) {
	tests := []test{
		{
			desc:     "basic case",
			srcfiles: []string{"pkg.go"},
			in: `
p := int32(42)
m2.I32 = &((p))
`,
			want: map[Level]string{
				Red: `
p := int32(42)
m2.SetI32(p)
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestUnshadow(t *testing.T) {
	tests := []test{
		{
			desc:     "struct literal",
			srcfiles: []string{"pkg.go"},
			in: `
m2 = &pb2.M2{M: m2}
`,
			want: map[Level]string{
				Green: `
m2h2 := m2
m2 = &pb2.M2{}
m2.SetM(m2h2)
`,
			},
		},

		{
			desc:     "selectorexpr",
			srcfiles: []string{"pkg.go"},
			in: `
var unrelated struct{
	zone string
}
zone := &pb2.M2{S: proto.String(unrelated.zone)}
_ = zone
`,
			want: map[Level]string{
				Green: `
var unrelated struct {
	zone string
}
zone := &pb2.M2{}
zone.SetS(unrelated.zone)
_ = zone
`,
			},
		},

		{
			desc:     "struct literal, define",
			srcfiles: []string{"pkg.go"},
			in: `
{
	m2 := &pb2.M2{M: m2}
	_ = m2
}
`,
			want: map[Level]string{
				Green: `
{
	m2h2 := m2
	m2 := &pb2.M2{}
	m2.SetM(m2h2)
	_ = m2
}
`,
			},
		},

		{
			desc:     "append",
			srcfiles: []string{"pkg.go"},
			in: `
all := &pb2.M2{}
all.Ms = append(all.Ms, m2)
`,
			want: map[Level]string{
				Green: `
all := &pb2.M2{}
all.SetMs(append(all.GetMs(), m2))
`,
			},
		},
	}

	runTableTests(t, tests)
}

func TestConflictingNames(t *testing.T) {
	tests := []test{
		{
			desc:     "struct literal",
			srcfiles: []string{"pkg.go"},
			in: `
m := &pb2.SetterNameConflict{
	Stat: proto.Int32(int32(5)),
	SetStat: proto.Int32(int32(42)),
	GetStat_: proto.Int32(int32(42)),
	HasStat: proto.Int32(int32(42)),
	ClearStat: proto.Int32(int32(42)),
}
_ = m
_ = *m.Stat
if m.Stat != nil || m.Stat == nil {
	m.Stat = nil
}
`,
			want: map[Level]string{
				// The SetGetStat_ is wrong. It should be SetGetStat.
				// Fixing it requires the proto descriptor and thus,
				// is not worth it.
				// Technically we should also generate Get_Stat()
				// instead of GetStat() but again it requires the
				// proto descriptor.
				// Both of these cases should be extremely rare.
				Green: `
m := &pb2.SetterNameConflict{}
m.Set_Stat(int32(5))
m.SetSetStat(int32(42))
m.SetGetStat_(int32(42))
m.SetHasStat(int32(42))
m.SetClearStat(int32(42))
_ = m
_ = m.GetStat()
if m.Has_Stat() || !m.Has_Stat() {
	m.Clear_Stat()
}
`,
			},
		},
	}

	runTableTests(t, tests)
}
