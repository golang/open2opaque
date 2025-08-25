// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestBuild(t *testing.T) {
	tests := []test{{
		desc: "proto2: empty composite literal",
		in: `
msg := &pb2.M2{}
msg = &pb2.M2{}
msg.Ms = []*pb2.M2{{},{}}
msg = func() *pb2.M2 {
	return &pb2.M2{}
}()
func(*pb2.M2) { }(&pb2.M2{})
_=msg
`,
		want: map[Level]string{
			Green: `
msg := &pb2.M2{}
msg = &pb2.M2{}
msg.SetMs([]*pb2.M2{{}, {}})
msg = func() *pb2.M2 {
	return &pb2.M2{}
}()
func(*pb2.M2) {}(&pb2.M2{})
_ = msg
`,
		},
	}, {
		desc: "proto2: non-empty composite literal",
		in: `
msg := &pb2.M2{S:nil}
msg = &pb2.M2{S:nil}
msg.Ms = []*pb2.M2{{S:nil},{S:nil}}
msg = func() *pb2.M2 {
 	return &pb2.M2{S:nil}
}()
func(*pb2.M2) {}(&pb2.M2{S:nil})
_ = msg
`,
		want: map[Level]string{
			Green: `
msg := pb2.M2_builder{S: nil}.Build()
msg = pb2.M2_builder{S: nil}.Build()
msg.SetMs([]*pb2.M2{pb2.M2_builder{S: nil}.Build(), pb2.M2_builder{S: nil}.Build()})
msg = func() *pb2.M2 {
	return pb2.M2_builder{S: nil}.Build()
}()
func(*pb2.M2) {}(pb2.M2_builder{S: nil}.Build())
_ = msg
`,
		},
	}, {
		desc: "proto3: empty composite literal",
		in: `
msg := &pb3.M3{}
msg = &pb3.M3{}
msg.Ms = []*pb3.M3{{},{}}
msg = func() *pb3.M3 {
	return &pb3.M3{}
}()
func(*pb3.M3) { }(&pb3.M3{})
_=msg
`,
		want: map[Level]string{
			Green: `
msg := &pb3.M3{}
msg = &pb3.M3{}
msg.SetMs([]*pb3.M3{{}, {}})
msg = func() *pb3.M3 {
	return &pb3.M3{}
}()
func(*pb3.M3) {}(&pb3.M3{})
_ = msg
`,
		},
	}, {
		desc: "proto3: non-empty composite literal",
		in: `
msg := &pb3.M3{M:nil}
msg = &pb3.M3{M:nil}
msg.Ms = []*pb3.M3{{M:nil},{M:nil}}
msg = func() *pb3.M3 {
	return &pb3.M3{M:nil}
}()
func(*pb3.M3) { }(&pb3.M3{M:nil})
_=msg
`,
		want: map[Level]string{
			Green: `
msg := pb3.M3_builder{M: nil}.Build()
msg = pb3.M3_builder{M: nil}.Build()
msg.SetMs([]*pb3.M3{pb3.M3_builder{M: nil}.Build(), pb3.M3_builder{M: nil}.Build()})
msg = func() *pb3.M3 {
	return pb3.M3_builder{M: nil}.Build()
}()
func(*pb3.M3) {}(pb3.M3_builder{M: nil}.Build())
_ = msg
`,
		},
	}, {
		desc: "builder naming conflict",
		in: `
_ = &pb2.M2{
	Build: proto.Int32(1),
}
_ = []*pb2.M2{{Build: proto.Int32(1)}}
_ = map[int]*pb2.M2{
	0: {Build: proto.Int32(1)},
}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{
	Build_: proto.Int32(1),
}.Build()
_ = []*pb2.M2{pb2.M2_builder{Build_: proto.Int32(1)}.Build()}
_ = map[int]*pb2.M2{
	0: pb2.M2_builder{Build_: proto.Int32(1)}.Build(),
}
`,
		},
	}, {
		desc: "proto2: scalars",
		in: `
_ = &pb2.M2{
	B: proto.Bool(true),
	F32: proto.Float32(1),
	F64: proto.Float64(1),
	I32: proto.Int32(1),
	I64: proto.Int64(1),
	Ui32: proto.Uint32(1),
	Ui64: proto.Uint64(1),
	S: proto.String("hello"),
	E: pb2.M2_E_VAL.Enum(),
}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{
	B:    proto.Bool(true),
	F32:  proto.Float32(1),
	F64:  proto.Float64(1),
	I32:  proto.Int32(1),
	I64:  proto.Int64(1),
	Ui32: proto.Uint32(1),
	Ui64: proto.Uint64(1),
	S:    proto.String("hello"),
	E:    pb2.M2_E_VAL.Enum(),
}.Build()
`,
		},
	}, {
		desc: "proto3: scalars",
		in: `_ = &pb3.M3{
	B: true,
	F32: 1,
	F64: 1,
	I32: 1,
	I64: 1,
	Ui32: 1,
	Ui64: 1,
	S: "hello",
}
`,
		want: map[Level]string{
			Green: `
_ = pb3.M3_builder{
	B:    true,
	F32:  1,
	F64:  1,
	I32:  1,
	I64:  1,
	Ui32: 1,
	Ui64: 1,
	S:    "hello",
}.Build()
`,
		},
	}, {
		desc: "scalar slices",
		in: `
_ = &pb2.M2{Is: []int32{1, 2, 3}}
_ = &pb3.M3{Is: []int32{1, 2, 3}}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{Is: []int32{1, 2, 3}}.Build()
_ = pb3.M3_builder{Is: []int32{1, 2, 3}}.Build()
`,
		},
	}, {
		desc: "msg slices",
		in: `
_ = &pb2.M2{Ms: []*pb2.M2{{}, &pb2.M2{}}}
_ = &pb2.M2{Ms: []*pb2.M2{{M: nil}, {M: nil}}}
_ = &pb3.M3{Ms: []*pb3.M3{{}, &pb3.M3{}}}
_ = &pb3.M3{Ms: []*pb3.M3{{M: nil}, {M: nil}}}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{Ms: []*pb2.M2{{}, &pb2.M2{}}}.Build()
_ = pb2.M2_builder{Ms: []*pb2.M2{pb2.M2_builder{M: nil}.Build(), pb2.M2_builder{M: nil}.Build()}}.Build()
_ = pb3.M3_builder{Ms: []*pb3.M3{{}, &pb3.M3{}}}.Build()
_ = pb3.M3_builder{Ms: []*pb3.M3{pb3.M3_builder{M: nil}.Build(), pb3.M3_builder{M: nil}.Build()}}.Build()
`,
		},
	}, {
		desc: "key-value builders",
		in: `
_ = map[int]*pb2.M2{
	1: {},
	2: {M:nil},
	3: &pb2.M2{},
	4: &pb2.M2{M: nil},
	5: &pb2.M2{
		Ms: []*pb2.M2{{},{M:nil},&pb2.M2{},&pb2.M2{M:nil}},
	},
}
_ = [...]*pb2.M2 {
	1: {},
	2: {M:nil},
	3: &pb2.M2{},
	4: &pb2.M2{M: nil},
	5: &pb2.M2{
		Ms: []*pb2.M2{{},{M:nil},&pb2.M2{},&pb2.M2{M:nil}},
	},
}
`,
		want: map[Level]string{
			Green: `
_ = map[int]*pb2.M2{
	1: {},
	2: pb2.M2_builder{M: nil}.Build(),
	3: &pb2.M2{},
	4: pb2.M2_builder{M: nil}.Build(),
	5: pb2.M2_builder{
		Ms: []*pb2.M2{{}, pb2.M2_builder{M: nil}.Build(), &pb2.M2{}, pb2.M2_builder{M: nil}.Build()},
	}.Build(),
}
_ = [...]*pb2.M2{
	1: {},
	2: pb2.M2_builder{M: nil}.Build(),
	3: &pb2.M2{},
	4: pb2.M2_builder{M: nil}.Build(),
	5: pb2.M2_builder{
		Ms: []*pb2.M2{{}, pb2.M2_builder{M: nil}.Build(), &pb2.M2{}, pb2.M2_builder{M: nil}.Build()},
	}.Build(),
}
`,
		},
	}, {
		desc: "composite lit msg in slice",
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
	}, {
		desc: "composite lit msg single line in slice",
		in: `
_ = []*pb2.M2{ &pb2.M2{S: nil} }
`,
		want: map[Level]string{
			Green: `
_ = []*pb2.M2{pb2.M2_builder{S: nil}.Build()}
`,
		},
	}, {
		desc: "multi-line msg slices",
		in: `
_ = []*pb2.M2{
	&pb2.M2{},
	&pb2.M2{M: nil},
	&pb2.M2{
		M: nil,
	},
}
_ = []*pb3.M3{
	{},
	{B: true},
	{
		S: "hello",
	},
}
`,
		want: map[Level]string{
			Green: `
_ = []*pb2.M2{
	&pb2.M2{},
	pb2.M2_builder{M: nil}.Build(),
	pb2.M2_builder{
		M: nil,
	}.Build(),
}
_ = []*pb3.M3{
	{},
	pb3.M3_builder{B: true}.Build(),
	pb3.M3_builder{
		S: "hello",
	}.Build(),
}
`,
		},
	}, {
		desc: "oneofs: easy cases",
		in: `
_ = &pb2.M2{OneofField: &pb2.M2_StringOneof{"hello"}}
_ = &pb2.M2{OneofField: &pb2.M2_StringOneof{StringOneof: "hello"}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{[]byte("hello")}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{BytesOneof: []byte("hello")}}
_ = &pb2.M2{OneofField: &pb2.M2_EnumOneof{pb2.M2_E_VAL}}
_ = &pb2.M2{OneofField: &pb2.M2_EnumOneof{EnumOneof: pb2.M2_E_VAL}}
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{&pb2.M2{}}}
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{MsgOneof: &pb2.M2{}}}
_ = &pb3.M3{OneofField: &pb3.M3_StringOneof{"hello"}}
_ = &pb3.M3{OneofField: &pb3.M3_StringOneof{StringOneof: "hello"}}
_ = &pb3.M3{OneofField: &pb3.M3_BytesOneof{[]byte("hello")}}
_ = &pb3.M3{OneofField: &pb3.M3_BytesOneof{BytesOneof: []byte("hello")}}
_ = &pb3.M3{OneofField: &pb3.M3_EnumOneof{pb3.M3_E_VAL}}
_ = &pb3.M3{OneofField: &pb3.M3_EnumOneof{EnumOneof: pb3.M3_E_VAL}}
_ = &pb3.M3{OneofField: &pb3.M3_MsgOneof{&pb3.M3{}}}
_ = &pb3.M3{OneofField: &pb3.M3_MsgOneof{MsgOneof: &pb3.M3{}}}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{StringOneof: proto.String("hello")}.Build()
_ = pb2.M2_builder{StringOneof: proto.String("hello")}.Build()
_ = pb2.M2_builder{BytesOneof: []byte("hello")}.Build()
_ = pb2.M2_builder{BytesOneof: []byte("hello")}.Build()
_ = pb2.M2_builder{EnumOneof: pb2.M2_E_VAL.Enum()}.Build()
_ = pb2.M2_builder{EnumOneof: pb2.M2_E_VAL.Enum()}.Build()
_ = pb2.M2_builder{MsgOneof: &pb2.M2{}}.Build()
_ = pb2.M2_builder{MsgOneof: &pb2.M2{}}.Build()
_ = pb3.M3_builder{StringOneof: proto.String("hello")}.Build()
_ = pb3.M3_builder{StringOneof: proto.String("hello")}.Build()
_ = pb3.M3_builder{BytesOneof: []byte("hello")}.Build()
_ = pb3.M3_builder{BytesOneof: []byte("hello")}.Build()
_ = pb3.M3_builder{EnumOneof: pb3.M3_E_VAL.Enum()}.Build()
_ = pb3.M3_builder{EnumOneof: pb3.M3_E_VAL.Enum()}.Build()
_ = pb3.M3_builder{MsgOneof: &pb3.M3{}}.Build()
_ = pb3.M3_builder{MsgOneof: &pb3.M3{}}.Build()
`,
		},
	}, {
		desc: "oneofs: messages",
		in: `
_ = &pb2.M2{
	OneofField: &pb2.M2_MsgOneof{
		MsgOneof: &pb2.M2{M: nil},
	},
}

_ = &pb2.M2{
	OneofField: &pb2.M2_MsgOneof{&pb2.M2{M: nil}},
}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{
	MsgOneof: pb2.M2_builder{M: nil}.Build(),
}.Build()

_ = pb2.M2_builder{
	MsgOneof: pb2.M2_builder{M: nil}.Build(),
}.Build()
`,
		},
	}, {
		desc: "oneofs: literal int enums",
		in: `
_ = &pb3.M3{
	OneofField: &pb3.M3_EnumOneof{42},
}
`,
		want: map[Level]string{
			Green: `
_ = &pb3.M3{
	OneofField: &pb3.M3_EnumOneof{42},
}
`,
		},
	}, {
		desc: "oneof: basic-type zero value",
		in: `
_ = &pb2.M2{OneofField: &pb2.M2_StringOneof{}}
_ = &pb2.M2{OneofField: &pb2.M2_IntOneof{}}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{StringOneof: proto.String("")}.Build()
_ = pb2.M2_builder{IntOneof: proto.Int64(0)}.Build()
`,
		},
	}, {
		desc:  "oneofs: nested messages",
		extra: `var mOuter2 pb2.M2Outer`,
		in: `
_ = &pb2.M2Outer{
		OuterOneof: mOuter2.OuterOneof,
	}

_ = &pb2.M2Outer{
		OuterOneof: &pb2.M2Outer_InnerMsg{
			InnerMsg: &pb2.M2Outer_MInner{
				InnerOneof: &pb2.M2Outer_MInner_StringInner{
					StringInner: "Hello World!",
				},
			},
		},
	}
`,
		want: map[Level]string{
			Red: `
_ = pb2.M2Outer_builder{
	InnerMsg:    mOuter2.GetInnerMsg(),
	StringOneof: proto.ValueOrNil(mOuter2.HasStringOneof(), mOuter2.GetStringOneof),
}.Build()

_ = pb2.M2Outer_builder{
	InnerMsg: pb2.M2Outer_MInner_builder{
		StringInner: proto.String("Hello World!"),
	}.Build(),
}.Build()
`,
		},
	}, {
		desc: "oneofs: preserve comments",
		in: `
_ = &pb2.M2{
	// comment before
	OneofField: /*comment1*/ m2.GetOneofField(), // eol comment
	}
`,
		want: map[Level]string{
			Yellow: `
_ = pb2.M2_builder{
	// comment before
	BytesOneof:/*comment1*/ m2.GetBytesOneof(), // eol comment
	EmptyOneof:                                 m2.GetEmptyOneof(),
	EnumOneof:                                  proto.ValueOrNil(m2.HasEnumOneof(), m2.GetEnumOneof),
	IntOneof:                                   proto.ValueOrNil(m2.HasIntOneof(), m2.GetIntOneof),
	MsgOneof:                                   m2.GetMsgOneof(),
	StringOneof:                                proto.ValueOrNil(m2.HasStringOneof(), m2.GetStringOneof),
}.Build()
`,
			Red: `
_ = pb2.M2_builder{
	// comment before
	BytesOneof:/*comment1*/ m2.GetBytesOneof(), // eol comment
	EmptyOneof:                                 m2.GetEmptyOneof(),
	EnumOneof:                                  proto.ValueOrNil(m2.HasEnumOneof(), m2.GetEnumOneof),
	IntOneof:                                   proto.ValueOrNil(m2.HasIntOneof(), m2.GetIntOneof),
	MsgOneof:                                   m2.GetMsgOneof(),
	StringOneof:                                proto.ValueOrNil(m2.HasStringOneof(), m2.GetStringOneof),
}.Build()
`,
		},
	}, {
		desc:     "oneofs: preserve comments when using setters",
		srcfiles: []string{"pkg.go"},
		in: `
_ = &pb2.M2{
	// comment before
	OneofField: /*comment1*/ &pb2.M2_StringOneof{"hello"}, // eol comment
	}
`,
		want: map[Level]string{
			Green: `
m2h2 := &pb2.M2{}
// comment before
m2h2.SetStringOneof("hello") // eol comment
_ = m2h2
`,
		},
	}, {

		desc:  "oneofs: propagate oneof",
		extra: `func F() *pb2.M2_MsgOneof {return nil}`,
		in: `
var scalarOneof *pb2.M2_StringOneof
_ = &pb2.M2{OneofField: scalarOneof}

var msgOneof *pb2.M2_MsgOneof
_ = &pb2.M2{OneofField: msgOneof}

_ = &pb2.M2{OneofField: F()}

ifaceOneof := m2.GetOneofField()
_ = &pb2.M2{OneofField: ifaceOneof}

_ = &pb2.M2{
	OneofField: m2.GetOneofField(),
	S: proto.String("42"),
	}
`,
		want: map[Level]string{
			Yellow: `
var scalarOneof *pb2.M2_StringOneof
_ = &pb2.M2{OneofField: scalarOneof}

var msgOneof *pb2.M2_MsgOneof
_ = &pb2.M2{OneofField: msgOneof}

_ = &pb2.M2{OneofField: F()}

ifaceOneof := m2.GetOneofField()
_ = &pb2.M2{OneofField: ifaceOneof}

_ = pb2.M2_builder{
	S:           proto.String("42"),
	BytesOneof:  m2.GetBytesOneof(),
	EmptyOneof:  m2.GetEmptyOneof(),
	EnumOneof:   proto.ValueOrNil(m2.HasEnumOneof(), m2.GetEnumOneof),
	IntOneof:    proto.ValueOrNil(m2.HasIntOneof(), m2.GetIntOneof),
	MsgOneof:    m2.GetMsgOneof(),
	StringOneof: proto.ValueOrNil(m2.HasStringOneof(), m2.GetStringOneof),
}.Build()
`,
			Red: `
var scalarOneof *pb2.M2_StringOneof
_ = pb2.M2_builder{StringOneof: proto.String(scalarOneof.StringOneof)}.Build()

var msgOneof *pb2.M2_MsgOneof
_ = pb2.M2_builder{MsgOneof: proto.ValueOrDefault(msgOneof.MsgOneof)}.Build()

_ = pb2.M2_builder{MsgOneof: proto.ValueOrDefault(F().MsgOneof)}.Build()

ifaceOneof := m2.GetOneofField()
_ = pb2.M2_builder{OneofField: ifaceOneof}.Build()

_ = pb2.M2_builder{
	S:           proto.String("42"),
	BytesOneof:  m2.GetBytesOneof(),
	EmptyOneof:  m2.GetEmptyOneof(),
	EnumOneof:   proto.ValueOrNil(m2.HasEnumOneof(), m2.GetEnumOneof),
	IntOneof:    proto.ValueOrNil(m2.HasIntOneof(), m2.GetIntOneof),
	MsgOneof:    m2.GetMsgOneof(),
	StringOneof: proto.ValueOrNil(m2.HasStringOneof(), m2.GetStringOneof),
}.Build()
`,
		},
	}, {
		desc: "oneofs: potentially nil message",
		in: `
msg := &pb2.M2{}
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{msg}}
`,
		want: map[Level]string{
			Green: `
msg := &pb2.M2{}
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{msg}}
`,
			Red: `
msg := &pb2.M2{}
_ = pb2.M2_builder{MsgOneof: proto.ValueOrDefault(msg)}.Build()
`,
		},
	}, {
		desc: "oneofs: msg zero value",
		in: `
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{}}
_ = &pb2.M2{OneofField: &pb2.M2_EmptyOneof{}}
`,
		want: map[Level]string{
			Green: `
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{}}
_ = &pb2.M2{OneofField: &pb2.M2_EmptyOneof{}}
`,
			Red: `
_ = pb2.M2_builder{MsgOneof: &pb2.M2{}}.Build()
_ = pb2.M2_builder{EmptyOneof: &xpb.Empty{}}.Build()
`,
		},
	}, {
		desc: "oneofs: enum zero value",
		in: `
_ = &pb2.M2{OneofField: &pb2.M2_EnumOneof{}}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{EnumOneof: 0}.Build()
`,
			Red: `
_ = pb2.M2_builder{EnumOneof: 0}.Build()
`,
		},
	}, {
		desc:  "oneofs: oneofs builder rewrite",
		extra: `func F() *pb2.M2_MsgOneof2 {return nil}`,
		in: `
msg := &pb2.M2{}
_ = &pb2.M2{
	OneofField: &pb2.M2_StringOneof{"hello"},
	OneofField2: &pb2.M2_EnumOneof2{},
}
_ = &pb2.M2{
	OneofField: &pb2.M2_StringOneof{"hello"},
	OneofField2: &pb2.M2_MsgOneof2{msg},
}
_ = &pb2.M2{
	OneofField: &pb2.M2_StringOneof{"hello"},
	OneofField2: &pb2.M2_MsgOneof2{},
}
_ = &pb2.M2{
	OneofField: &pb2.M2_StringOneof{"hello"},
	OneofField2: F(),
}
`,
		want: map[Level]string{
			Green: `
msg := &pb2.M2{}
_ = pb2.M2_builder{
	StringOneof: proto.String("hello"),
	EnumOneof2:  0,
}.Build()
_ = &pb2.M2{
	OneofField:  &pb2.M2_StringOneof{"hello"},
	OneofField2: &pb2.M2_MsgOneof2{msg},
}
_ = &pb2.M2{
	OneofField:  &pb2.M2_StringOneof{"hello"},
	OneofField2: &pb2.M2_MsgOneof2{},
}
_ = &pb2.M2{
	OneofField:  &pb2.M2_StringOneof{"hello"},
	OneofField2: F(),
}
`,
			Yellow: `
msg := &pb2.M2{}
_ = pb2.M2_builder{
	StringOneof: proto.String("hello"),
	EnumOneof2:  0,
}.Build()
_ = pb2.M2_builder{
	StringOneof: proto.String("hello"),
	MsgOneof2:   proto.ValueOrDefault(msg),
}.Build()
_ = pb2.M2_builder{
	StringOneof: proto.String("hello"),
	MsgOneof2:   &pb2.M2{},
}.Build()
_ = &pb2.M2{
	OneofField:  &pb2.M2_StringOneof{"hello"},
	OneofField2: F(),
}
`,
		},
	}}

	runTableTests(t, tests)
}
