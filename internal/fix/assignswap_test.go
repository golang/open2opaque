// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import "testing"

func TestSetterSwap(t *testing.T) {
	tests := []test{
		{
			desc:     "int32 swap",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := &pb2.M2{
	I32: proto.Int32(23),
	Build: proto.Int32(42),
}
mypb.I32, mypb.Build = mypb.Build, mypb.I32
`,
			want: map[Level]string{
				Green: `
mypb := &pb2.M2{}
mypb.SetI32(23)
mypb.SetBuild_(42)
mypb.I32, mypb.Build = mypb.Build, mypb.I32
`,
				Yellow: `
mypb := &pb2.M2{}
mypb.SetI32(23)
mypb.SetBuild_(42)
m2h2, m2h3 := proto.ValueOrNil(mypb.HasBuild_(), mypb.GetBuild_), proto.ValueOrNil(mypb.HasI32(), mypb.GetI32)
mypb.I32 = m2h2
mypb.Build = m2h3
`,
				Red: `
mypb := &pb2.M2{}
mypb.SetI32(23)
mypb.SetBuild_(42)
m2h2, m2h3 := proto.ValueOrNil(mypb.HasBuild_(), mypb.GetBuild_), proto.ValueOrNil(mypb.HasI32(), mypb.GetI32)
if m2h2 != nil {
	mypb.SetI32(*m2h2)
} else {
	mypb.ClearI32()
}
if m2h3 != nil {
	mypb.SetBuild_(*m2h3)
} else {
	mypb.ClearBuild_()
}
`,
			},
		},

		{
			desc:     "int32 oneof copy",
			srcfiles: []string{"pkg.go"},
			extra:    `func defaultVal() *pb2.M2 { return nil }`,
			in: `
mypb := &pb2.M2{
	M: &pb2.M2{
		OneofField: &pb2.M2_IntOneof{
			IntOneof: 23,
		},
	},
}
mypb.M.OneofField = defaultVal().M.OneofField
`,
			want: map[Level]string{
				Green: `
m2h2 := &pb2.M2{}
m2h2.SetIntOneof(23)
mypb := &pb2.M2{}
mypb.SetM(m2h2)
mypb.GetM().OneofField = defaultVal().GetM().OneofField
`,
				Yellow: `
m2h2 := &pb2.M2{}
m2h2.SetIntOneof(23)
mypb := &pb2.M2{}
mypb.SetM(m2h2)
mypb.GetM().OneofField = defaultVal().GetM().OneofField
`,
			},
		},

		{
			desc:     "int32 proto3 getter swap",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := pb3.M3_builder{
	I32: 23,
	SecondI32: 42,
}.Build()
mypb.I32, mypb.SecondI32 = mypb.GetSecondI32(), mypb.GetI32()
`,
			want: map[Level]string{
				Green: `
mypb := pb3.M3_builder{
	I32:       23,
	SecondI32: 42,
}.Build()
mypb.I32, mypb.SecondI32 = mypb.GetSecondI32(), mypb.GetI32()
`,
				Yellow: `
mypb := pb3.M3_builder{
	I32:       23,
	SecondI32: 42,
}.Build()
m3h2, m3h3 := mypb.GetSecondI32(), mypb.GetI32()
mypb.SetI32(m3h2)
mypb.SetSecondI32(m3h3)
`,
				Red: `
mypb := pb3.M3_builder{
	I32:       23,
	SecondI32: 42,
}.Build()
m3h2, m3h3 := mypb.GetSecondI32(), mypb.GetI32()
mypb.SetI32(m3h2)
mypb.SetSecondI32(m3h3)
`,
			},
		},

		{
			desc:     "int32 nested proto3 getter swap",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		I32: 23,
		SecondI32: 42,
	}.Build(),
}.Build()
mypb.M.I32, mypb.M.SecondI32 = mypb.M.GetSecondI32(), mypb.M.GetI32()
		`,
			want: map[Level]string{
				Green: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		I32:       23,
		SecondI32: 42,
	}.Build(),
}.Build()
mypb.GetM().I32, mypb.GetM().SecondI32 = mypb.GetM().GetSecondI32(), mypb.GetM().GetI32()
`,
				Yellow: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		I32:       23,
		SecondI32: 42,
	}.Build(),
}.Build()
m3h2, m3h3 := mypb.GetM().GetSecondI32(), mypb.GetM().GetI32()
mypb.GetM().SetI32(m3h2)
mypb.GetM().SetSecondI32(m3h3)
`,
				Red: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		I32:       23,
		SecondI32: 42,
	}.Build(),
}.Build()
m3h2, m3h3 := mypb.GetM().GetSecondI32(), mypb.GetM().GetI32()
mypb.GetM().SetI32(m3h2)
mypb.GetM().SetSecondI32(m3h3)
`,
			},
		},

		{
			desc:     "int32 double-nested proto3 getter swap",
			srcfiles: []string{"pkg.go"},
			in: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		M: pb3.M3_builder{
			I32: 23,
			SecondI32: 42,
		}.Build(),
	}.Build(),
}.Build()
mypb.M.M.I32, mypb.M.M.SecondI32 = mypb.M.M.GetSecondI32(), mypb.M.M.GetI32()
`,
			want: map[Level]string{
				Green: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		M: pb3.M3_builder{
			I32:       23,
			SecondI32: 42,
		}.Build(),
	}.Build(),
}.Build()
mypb.GetM().GetM().I32, mypb.GetM().GetM().SecondI32 = mypb.GetM().GetM().GetSecondI32(), mypb.GetM().GetM().GetI32()
`,
				Yellow: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		M: pb3.M3_builder{
			I32:       23,
			SecondI32: 42,
		}.Build(),
	}.Build(),
}.Build()
m3h2, m3h3 := mypb.GetM().GetM().GetSecondI32(), mypb.GetM().GetM().GetI32()
mypb.GetM().GetM().SetI32(m3h2)
mypb.GetM().GetM().SetSecondI32(m3h3)
`,
				Red: `
mypb := pb3.M3_builder{
	M: pb3.M3_builder{
		M: pb3.M3_builder{
			I32:       23,
			SecondI32: 42,
		}.Build(),
	}.Build(),
}.Build()
m3h2, m3h3 := mypb.GetM().GetM().GetSecondI32(), mypb.GetM().GetM().GetI32()
mypb.GetM().GetM().SetI32(m3h2)
mypb.GetM().GetM().SetSecondI32(m3h3)
`,
			},
		},
	}

	runTableTests(t, tests)
}
