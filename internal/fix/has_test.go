// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestAvoidRedundantHaser(t *testing.T) {
	tests := []test{
		{
			desc:     "basic non-nil check",
			srcfiles: []string{"pkg.go"},
			in: `
m := &pb2.M2{}
if m2.I32 != nil {
	m.I32 = m2.I32
}
`,
			want: map[Level]string{
				Green: `
m := &pb2.M2{}
if m2.HasI32() {
	m.SetI32(m2.GetI32())
}
`,
			},
		},

		{
			desc:     "basic haser check",
			srcfiles: []string{"pkg.go"},
			in: `
m := &pb2.M2{}
if m2.HasI32() {
	m.I32 = m2.I32
}
`,
			want: map[Level]string{
				Green: `
m := &pb2.M2{}
if m2.HasI32() {
	m.SetI32(m2.GetI32())
}
`,
			},
		},

		{
			desc:     "modification after has check",
			extra:    `func f() *int32 { return nil }`,
			srcfiles: []string{"pkg.go"},
			in: `
m := &pb2.M2{}
if m2.HasI32() {
	m2.I32 = f()
	m.I32 = m2.I32
}

if m2.I32 != nil {
	m2.I32 = f()
	m.I32 = m2.I32
}
`,
			want: map[Level]string{
				Green: `
m := &pb2.M2{}
if m2.HasI32() {
	m2.I32 = f()
	if m2.HasI32() {
		m.SetI32(m2.GetI32())
	} else {
		m.ClearI32()
	}
}

if m2.HasI32() {
	m2.I32 = f()
	if m2.HasI32() {
		m.SetI32(m2.GetI32())
	} else {
		m.ClearI32()
	}
}
`,
			},
		},

		{
			desc:     "shadowing",
			extra:    `func f() *pb2.M2 { return nil }`,
			srcfiles: []string{"pkg.go"},
			in: `
m := &pb2.M2{}
if m2.HasI32() {
	m2 := f()
	m.I32 = m2.I32
}
`,
			want: map[Level]string{
				Green: `
m := &pb2.M2{}
if m2.HasI32() {
	m2 := f()
	if m2.HasI32() {
		m.SetI32(m2.GetI32())
	} else {
		m.ClearI32()
	}
}
`,
			},
		},

		{
			desc:     "usage in lhs of assignment",
			srcfiles: []string{"pkg.go"},
			in: `
m := &pb2.M2{}
if m2.HasI32() {
	m2.I32 = proto.Int32(int32(42))
	m.I32 = m2.I32
}
`,
			want: map[Level]string{
				Green: `
m := &pb2.M2{}
if m2.HasI32() {
	m2.SetI32(int32(42))
	if m2.HasI32() {
		m.SetI32(m2.GetI32())
	} else {
		m.ClearI32()
	}
}
`,
			},
		},

		{
			desc:     "usage of different field",
			srcfiles: []string{"pkg.go"},
			in: `
m := &pb2.M2{}
if m2.HasI32() {
	m2.S = proto.String("Hello")
	m.I32 = m2.I32
}

if m2.HasI32() {
	m2.SetS("Hello")
	m.I32 = m2.I32
}
`,
			want: map[Level]string{
				Green: `
m := &pb2.M2{}
if m2.HasI32() {
	m2.SetS("Hello")
	m.SetI32(m2.GetI32())
}

if m2.HasI32() {
	m2.SetS("Hello")
	m.SetI32(m2.GetI32())
}
`,
			},
		},

		{
			desc:     "comp literal (non-test)",
			srcfiles: []string{"pkg.go"},
			in: `
if m2.HasI32() {
	m := &pb2.M2{
		I32: m2.I32,
	}
	_ = m
}

if m2.I32 != nil {
	m := &pb2.M2{
		I32: m2.I32,
	}
	_ = m
}
`,
			want: map[Level]string{
				Red: `
if m2.HasI32() {
	m := &pb2.M2{}
	m.SetI32(m2.GetI32())
	_ = m
}

if m2.HasI32() {
	m := &pb2.M2{}
	m.SetI32(m2.GetI32())
	_ = m
}
`,
			},
		},

		{
			desc:     "comp literal (test)",
			srcfiles: []string{"pkg_test.go"},
			in: `
if m2.HasI32() {
	m := &pb2.M2{
		I32: m2.I32,
	}
	_ = m
}

if m2.I32 != nil {
	m := &pb2.M2{
		I32: m2.I32,
	}
	_ = m
}
`,
			want: map[Level]string{
				Red: `
if m2.HasI32() {
	m := pb2.M2_builder{
		I32: m2.GetI32(),
	}.Build()
	_ = m
}

if m2.HasI32() {
	m := pb2.M2_builder{
		I32: m2.GetI32(),
	}.Build()
	_ = m
}
`,
			},
		},

		{
			desc:     "return",
			srcfiles: []string{"pkg.go"},
			in: `
_ = func() *int32 {
	if m2.HasI32() {
		return m2.I32
	}
	return nil
}

_ = func() int32 {
	if m2.HasI32() {
		return *m2.I32
	}
	return int32(0)
}
`,
			want: map[Level]string{
				Red: `
_ = func() *int32 {
	if m2.HasI32() {
		return proto.Int32(m2.GetI32())
	}
	return nil
}

_ = func() int32 {
	if m2.HasI32() {
		return m2.GetI32()
	}
	return int32(0)
}
`,
			},
		},

		{
			desc:     "assign",
			srcfiles: []string{"pkg.go"},
			in: `
if m2.HasI64() {
	i := m2.I64
	_ = i
}
`,

			want: map[Level]string{
				Red: `
if m2.HasI64() {
	i := proto.Int64(m2.GetI64())
	_ = i
}
`,
			},
		},

		{
			desc:     "getter",
			srcfiles: []string{"pkg.go"},
			in: `
if m2.GetF32() != 0 {
	m := &pb2.M2{}
	m.F32 = m2.F32
	_ = m
}
if *m2.F32 != 0 {
	m := &pb2.M2{}
	m.F32 = m2.F32
	_ = m
}
`,

			want: map[Level]string{
				Green: `
if m2.GetF32() != 0 {
	m := &pb2.M2{}
	m.SetF32(m2.GetF32())
	_ = m
}
if m2.GetF32() != 0 {
	m := &pb2.M2{}
	m.SetF32(m2.GetF32())
	_ = m
}
`,
			},
		},

		{
			desc:     "getter: bytes",
			srcfiles: []string{"pkg.go"},
			in: `
if m2.GetBytes() != nil {
	m := &pb2.M2{}
	m.Bytes = m2.Bytes
	_ = m
}
if nil != m2.Bytes  {
	m := &pb2.M2{}
	m.Bytes = m2.Bytes
	_ = m
}
`,

			want: map[Level]string{
				Green: `
if m2.GetBytes() != nil {
	m := &pb2.M2{}
	if x := m2.GetBytes(); x != nil {
		m.SetBytes(x)
	}
	_ = m
}
if nil != m2.GetBytes() {
	m := &pb2.M2{}
	if x := m2.GetBytes(); x != nil {
		m.SetBytes(x)
	}
	_ = m
}
`,
			},
		},

		{
			desc:     "getter: different field",
			srcfiles: []string{"pkg.go"},
			in: `
if *m2.I64 != 0 {
	m := &pb2.M2{}
	m.I32 = m2.I32
	_ = m
}
if 0 != m2.GetI64()  {
	m := &pb2.M2{}
	m.I32 = m2.I32
	_ = m
}
`,

			want: map[Level]string{
				Green: `
if m2.GetI64() != 0 {
	m := &pb2.M2{}
	if m2.HasI32() {
		m.SetI32(m2.GetI32())
	}
	_ = m
}
if 0 != m2.GetI64() {
	m := &pb2.M2{}
	if m2.HasI32() {
		m.SetI32(m2.GetI32())
	}
	_ = m
}
`,
			},
		},
	}
	runTableTests(t, tests)
}

func TestHas(t *testing.T) {
	tests := []test{{
		desc: "proto2: if-has",
		in: `
if e := m2.E; e != nil {
	_ = *e
	_ = *e
	_ = *e
}

if e := m2.E; e == nil {
	_ = *e
}

// New name must be used in the conditional.
var f *int
if e := m2.E; f != nil {
	_ = *e
}

// We don't apply this rewrite when comparison with nil is ok.
if m := m2.GetM(); m != nil {
	_ = m
}

// The code can't use the pointer value.
if e := m2.E; e != nil {
	_ = e
}
`,
		want: map[Level]string{
			Green: `
if m2.HasE() {
	_ = m2.GetE()
	_ = m2.GetE()
	_ = m2.GetE()
}

if !m2.HasE() {
	_ = m2.GetE()
}

// New name must be used in the conditional.
var f *int
if e := m2.E; f != nil {
	_ = *e
}

// We don't apply this rewrite when comparison with nil is ok.
if m := m2.GetM(); m != nil {
	_ = m
}

// The code can't use the pointer value.
if e := m2.E; e != nil {
	_ = e
}
`,
		}}, {
		desc: "proto2: has",
		in: `
_ = m2.E != nil
_ = m2.B != nil
_ = m2.Bytes != nil
_ = m2.F32 != nil
_ = m2.F64 != nil
_ = m2.I32 != nil
_ = m2.I64 != nil
_ = m2.Ui32 != nil
_ = m2.Ui64 != nil
_ = m2.M != nil
_ = m2.Is != nil
_ = m2.Ms != nil
_ = m2.Map != nil
`,
		want: map[Level]string{
			Green: `
_ = m2.HasE()
_ = m2.HasB()
_ = m2.HasBytes()
_ = m2.HasF32()
_ = m2.HasF64()
_ = m2.HasI32()
_ = m2.HasI64()
_ = m2.HasUi32()
_ = m2.HasUi64()
_ = m2.HasM()
_ = m2.GetIs() != nil
_ = m2.GetMs() != nil
_ = m2.GetMap() != nil
`}}, {
		desc: "proto2: doesn't have",
		in: `
_ = m2.E == nil
_ = m2.B == nil
_ = m2.Bytes == nil
_ = m2.F32 == nil
_ = m2.F64 == nil
_ = m2.I32 == nil
_ = m2.I64 == nil
_ = m2.Ui32 == nil
_ = m2.Ui64 == nil
_ = m2.M == nil
_ = m2.Is == nil
_ = m2.Ms == nil
_ = m2.Map == nil
`,
		want: map[Level]string{
			Green: `
_ = !m2.HasE()
_ = !m2.HasB()
_ = !m2.HasBytes()
_ = !m2.HasF32()
_ = !m2.HasF64()
_ = !m2.HasI32()
_ = !m2.HasI64()
_ = !m2.HasUi32()
_ = !m2.HasUi64()
_ = !m2.HasM()
_ = m2.GetIs() == nil
_ = m2.GetMs() == nil
_ = m2.GetMap() == nil
`}}, {
		desc: "proto3: has",
		in: `
_ = m3.Bytes != nil
_ = m3.M != nil
_ = m3.Is != nil
_ = m3.Ms != nil
_ = m3.Map != nil
`,
		want: map[Level]string{
			Green: `
_ = len(m3.GetBytes()) != 0
_ = m3.HasM()
_ = m3.GetIs() != nil
_ = m3.GetMs() != nil
_ = m3.GetMap() != nil
`}}, {
		desc: "proto3: doesn't have",
		in: `
_ = m3.Bytes == nil
_ = m3.M == nil
_ = m3.Is == nil
_ = m3.Ms == nil
_ = m3.Map == nil
`,
		want: map[Level]string{
			Green: `
_ = len(m3.GetBytes()) == 0
_ = !m3.HasM()
_ = m3.GetIs() == nil
_ = m3.GetMs() == nil
_ = m3.GetMap() == nil
`}}, {
		desc: "proto3 value: has",
		in: `
var m3val pb3.M3
_ = m3val.Bytes != nil
_ = m3val.M != nil
_ = m3val.Is != nil
_ = m3val.Ms != nil
_ = m3val.Map != nil
`,
		want: map[Level]string{
			Green: `
var m3val pb3.M3
_ = len(m3val.GetBytes()) != 0
_ = m3val.HasM()
_ = m3val.GetIs() != nil
_ = m3val.GetMs() != nil
_ = m3val.GetMap() != nil
`}}, {
		desc: "proto3 value: doesn't have",
		in: `
var m3val pb3.M3
_ = m3val.Bytes == nil
_ = m3val.M == nil
_ = m3val.Is == nil
_ = m3val.Ms == nil
_ = m3val.Map == nil
`,
		want: map[Level]string{
			Green: `
var m3val pb3.M3
_ = len(m3val.GetBytes()) == 0
_ = !m3val.HasM()
_ = m3val.GetIs() == nil
_ = m3val.GetMs() == nil
_ = m3val.GetMap() == nil
`}},
	}

	runTableTests(t, tests)
}
