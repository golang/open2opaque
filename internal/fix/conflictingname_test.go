// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestConflictingName(t *testing.T) {
	tests := []test{
		{
			desc: "resolve conflict with Build",
			in: `
_ = &pb2.M2{
	Build: proto.Int32(1),
}

_ = *m2.Build
_ = m2.Build != nil

m2.Build = proto.Int32(1)
m2.Build = nil
`,
			want: map[Level]string{
				Green: `
_ = pb2.M2_builder{
	Build_: proto.Int32(1),
}.Build()

_ = m2.GetBuild_()
_ = m2.HasBuild_()

m2.SetBuild_(1)
m2.ClearBuild_()
`,
			},
		},

		{
			desc: "resolve conflict with message methods",
			in: `
_ = &pb2.M2{
	ProtoMessage_: proto.Int32(1),
	Reset_: proto.Int32(1),
	String_: proto.Int32(1),
	Descriptor_: proto.Int32(1),
}

_ = *m2.ProtoMessage_
_ = m2.ProtoMessage_ != nil
m2.ProtoMessage_ = nil
m2.ProtoMessage_ = proto.Int32(1)

_ = *m2.Reset_
_ = m2.Reset_ != nil
m2.Reset_ = nil
m2.Reset_ = proto.Int32(1)

_ = *m2.String_
_ = m2.String_ != nil
m2.String_ = nil
m2.String_ = proto.Int32(1)

_ = *m2.Descriptor_
_ = m2.Descriptor_ != nil
m2.Descriptor_ = nil
m2.Descriptor_ = proto.Int32(1)
`,
			want: map[Level]string{
				Green: `
_ = pb2.M2_builder{
	ProtoMessage: proto.Int32(1),
	Reset:        proto.Int32(1),
	String:       proto.Int32(1),
	Descriptor:   proto.Int32(1),
}.Build()

_ = m2.GetProtoMessage()
_ = m2.HasProtoMessage()
m2.ClearProtoMessage()
m2.SetProtoMessage(1)

_ = m2.GetReset()
_ = m2.HasReset()
m2.ClearReset()
m2.SetReset(1)

_ = m2.GetString()
_ = m2.HasString()
m2.ClearString()
m2.SetString(1)

_ = m2.GetDescriptor()
_ = m2.HasDescriptor()
m2.ClearDescriptor()
m2.SetDescriptor(1)
`,
			},
		},

		{
			desc: "resolve conflict with message methods due to oneofs",
			in: `
_ = &pb3.M3{
	OneofField: &pb3.M3_ProtoMessage_{""},
}
_ = &pb3.M3{
	OneofField: &pb3.M3_Reset_{""},
}
_ = &pb3.M3{
	OneofField: &pb3.M3_String_{""},
}
_ = &pb3.M3{
	OneofField: &pb3.M3_Descriptor_{""},
}
`,
			want: map[Level]string{
				Green: `
_ = pb3.M3_builder{
	ProtoMessage: proto.String(""),
}.Build()
_ = pb3.M3_builder{
	Reset: proto.String(""),
}.Build()
_ = pb3.M3_builder{
	String: proto.String(""),
}.Build()
_ = pb3.M3_builder{
	Descriptor: proto.String(""),
}.Build()
`,
			},
		},
	}

	runTableTests(t, tests)
}
