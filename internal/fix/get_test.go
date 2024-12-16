// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestGet(t *testing.T) {
	tests := []test{{
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
		desc: "proto2: get scalar ptr with comments",
		in: `
// before line
extra := m2.S // end of line
_ = extra
`,
		want: map[Level]string{
			Green: `
// before line
extra := m2.S // end of line
_ = extra
`,
			Yellow: `
// before line
extra := proto.ValueOrNil(m2.HasS(), m2.GetS) // end of line
_ = extra
`,
			Red: `
// before line
extra := proto.ValueOrNil(m2.HasS(), m2.GetS) // end of line
_ = extra
`,
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
		desc: "proto3: get message ptr",
		in:   "_ = m3.M",
		want: map[Level]string{
			Green: "_ = m3.GetM()",
		},
	}, {
		desc: "proto3: get message value",
		in:   "_ = *m3.M",
		want: map[Level]string{
			Green: "_ = *m3.GetM()",
		},
	}, {
		desc: "proto3: get scalar",
		in:   "_ = m3.S",
		want: map[Level]string{
			Green: "_ = m3.GetS()",
		},
	}, {
		desc: "field address",
		in: `
_ = &m2.S
_ = &m2.Is
_ = &m2.M
_ = &m2.Ms

_ = &m3.S
_ = &m3.Is
_ = &m3.M
_ = &m3.Ms
`,
		want: map[Level]string{
			Yellow: `
_ = &m2.S
_ = &m2.Is
_ = &m2.M
_ = &m2.Ms

_ = &m3.S
_ = &m3.Is
_ = &m3.M
_ = &m3.Ms
`,
			Red: `
_ = &m2.S  /* DO_NOT_SUBMIT: missing rewrite for address of field */
_ = &m2.Is /* DO_NOT_SUBMIT: missing rewrite for address of field */
_ = &m2.M  /* DO_NOT_SUBMIT: missing rewrite for address of field */
_ = &m2.Ms /* DO_NOT_SUBMIT: missing rewrite for address of field */

_ = proto.String(m3.GetS())
_ = &m3.Is /* DO_NOT_SUBMIT: missing rewrite for address of field */
_ = &m3.M  /* DO_NOT_SUBMIT: missing rewrite for address of field */
_ = &m3.Ms /* DO_NOT_SUBMIT: missing rewrite for address of field */
`,
		},
	}, {
		desc: "proto3: scalar slice",
		in:   "_ = m3.Is",
		want: map[Level]string{
			Green: "_ = m3.GetIs()",
		},
	}, {
		desc: "proto3: scalar slice and index",
		in:   "_ = m3.Is[0]",
		want: map[Level]string{
			Green: "_ = m3.GetIs()[0]",
		},
	}, {
		desc: "proto3: message slice",
		in:   "_ = m3.Ms",
		want: map[Level]string{
			Green: "_ = m3.GetMs()",
		},
	}, {
		desc: "proto3: message slice and index",
		in:   "_ = m3.Ms[0]",
		want: map[Level]string{
			Green: "_ = m3.GetMs()[0]",
		},
	}, {
		desc:  "proto3: get in function args",
		extra: "func g3(string, *pb3.M3, []int32, []*pb3.M3) { }",
		in:    "g3(m3.S, m3.M, m3.Is, m3.Ms)",
		want: map[Level]string{
			Green: "g3(m3.GetS(), m3.GetM(), m3.GetIs(), m3.GetMs())",
		},
	}, {
		desc: "rewriting Get only affects fields",
		in:   "_ = m2.GetS",
		want: map[Level]string{
			Green:  "_ = m2.GetS",
			Yellow: "_ = m2.GetS",
		},
	}, {
		desc: "proto2: chained get",
		in:   "_ = *m2.M.M.M.Ms[0].M.S",
		want: map[Level]string{
			Green: "_ = m2.GetM().GetM().GetM().GetMs()[0].GetM().GetS()",
		},
	}, {
		desc: "proto3: chained get",
		in:   "_ = m3.M.M.M.Ms[0].M.S",
		want: map[Level]string{
			Green: "_ = m3.GetM().GetM().GetM().GetMs()[0].GetM().GetS()",
		},
	}}

	runTableTests(t, tests)
}

func TestNoFuncLiteral(t *testing.T) {
	tt := []test{
		{
			desc: "return scalar field",
			in: `
_ = func() *float32 {
	// before
	return m2.F32 // end of line
}
`,
			want: map[Level]string{
				Red: `
_ = func() *float32 {
	// before
	return proto.ValueOrNil(m2.HasF32(), m2.GetF32) // end of line
}
`,
			},
		},

		{
			desc: "return non-scalar fields",
			in: `
_ = func() any { // needs to be any because oneof field types are not exported
	return m2.OneofField
}

_ = func() []byte {
	return m2.Bytes
}

_ = func() *pb2.M2 {
	return m2.M
}
`,
			want: map[Level]string{
				Red: `
_ = func() any { // needs to be any because oneof field types are not exported
	// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).
	return m2.OneofField
}

_ = func() []byte {
	return m2.GetBytes()
}

_ = func() *pb2.M2 {
	return m2.GetM()
}
`,
			},
		},

		{
			desc: "define var from field",
			in: `
f := m2.F32
_ = f

of := m2.OneofField
_ = of
`,
			want: map[Level]string{
				Red: `
f := proto.ValueOrNil(m2.HasF32(), m2.GetF32)
_ = f

// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).
of := m2.OneofField
_ = of
`,
			},
		},

		{
			desc: "define var from non-scalar fields",
			in: `
of := m2.OneofField
_ = of

b := m2.Bytes
_ = b

m := m2.M
_ = m
`,
			want: map[Level]string{
				Red: `
// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).
of := m2.OneofField
_ = of

b := m2.GetBytes()
_ = b

m := m2.GetM()
_ = m
`,
			},
		},

		{
			desc: "derefence potential nil pointer",
			in: `
_ = *(m2.I32) + 35
`,
			want: map[Level]string{
				Green: `
_ = m2.GetI32() + 35
`,
				Red: `
_ = m2.GetI32() + 35
`,
			},
		},
	}

	runTableTests(t, tt)
}
