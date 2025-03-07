// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2-devel
// 	protoc        v5.29.1
// source: go_api_enum.proto

package apiflagdata

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GoAPI int32

const (
	GoAPI_GO_API_UNSPECIFIED    GoAPI = 0
	GoAPI_INVALID               GoAPI = 1
	GoAPI_OPEN_V1               GoAPI = 2
	GoAPI_OPEN_TO_OPAQUE_HYBRID GoAPI = 3
	GoAPI_OPAQUE_V0             GoAPI = 4
)

// Enum value maps for GoAPI.
var (
	GoAPI_name = map[int32]string{
		0: "GO_API_UNSPECIFIED",
		1: "INVALID",
		2: "OPEN_V1",
		3: "OPEN_TO_OPAQUE_HYBRID",
		4: "OPAQUE_V0",
	}
	GoAPI_value = map[string]int32{
		"GO_API_UNSPECIFIED":    0,
		"INVALID":               1,
		"OPEN_V1":               2,
		"OPEN_TO_OPAQUE_HYBRID": 3,
		"OPAQUE_V0":             4,
	}
)

func (x GoAPI) Enum() *GoAPI {
	p := new(GoAPI)
	*p = x
	return p
}

func (x GoAPI) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (GoAPI) Descriptor() protoreflect.EnumDescriptor {
	return file_go_api_enum_proto_enumTypes[0].Descriptor()
}

func (GoAPI) Type() protoreflect.EnumType {
	return &file_go_api_enum_proto_enumTypes[0]
}

func (x GoAPI) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use GoAPI.Descriptor instead.
func (GoAPI) EnumDescriptor() ([]byte, []int) {
	return file_go_api_enum_proto_rawDescGZIP(), []int{0}
}

var File_go_api_enum_proto protoreflect.FileDescriptor

var file_go_api_enum_proto_rawDesc = []byte{
	0x0a, 0x11, 0x67, 0x6f, 0x5f, 0x61, 0x70, 0x69, 0x5f, 0x65, 0x6e, 0x75, 0x6d, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x25, 0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e,
	0x67, 0x6f, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x61,
	0x70, 0x69, 0x66, 0x6c, 0x61, 0x67, 0x64, 0x61, 0x74, 0x61, 0x2a, 0x69, 0x0a, 0x05, 0x47, 0x6f,
	0x41, 0x50, 0x49, 0x12, 0x16, 0x0a, 0x12, 0x47, 0x4f, 0x5f, 0x41, 0x50, 0x49, 0x5f, 0x55, 0x4e,
	0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x49,
	0x4e, 0x56, 0x41, 0x4c, 0x49, 0x44, 0x10, 0x01, 0x12, 0x0b, 0x0a, 0x07, 0x4f, 0x50, 0x45, 0x4e,
	0x5f, 0x56, 0x31, 0x10, 0x02, 0x12, 0x19, 0x0a, 0x15, 0x4f, 0x50, 0x45, 0x4e, 0x5f, 0x54, 0x4f,
	0x5f, 0x4f, 0x50, 0x41, 0x51, 0x55, 0x45, 0x5f, 0x48, 0x59, 0x42, 0x52, 0x49, 0x44, 0x10, 0x03,
	0x12, 0x0d, 0x0a, 0x09, 0x4f, 0x50, 0x41, 0x51, 0x55, 0x45, 0x5f, 0x56, 0x30, 0x10, 0x04, 0x22,
	0x04, 0x08, 0x05, 0x10, 0x05, 0x62, 0x08, 0x65, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x70,
	0xe8, 0x07,
}

var (
	file_go_api_enum_proto_rawDescOnce sync.Once
	file_go_api_enum_proto_rawDescData = file_go_api_enum_proto_rawDesc
)

func file_go_api_enum_proto_rawDescGZIP() []byte {
	file_go_api_enum_proto_rawDescOnce.Do(func() {
		file_go_api_enum_proto_rawDescData = protoimpl.X.CompressGZIP(file_go_api_enum_proto_rawDescData)
	})
	return file_go_api_enum_proto_rawDescData
}

var file_go_api_enum_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_go_api_enum_proto_goTypes = []any{
	(GoAPI)(0), // 0: net.proto2.go.open2opaque.apiflagdata.GoAPI
}
var file_go_api_enum_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_go_api_enum_proto_init() }
func file_go_api_enum_proto_init() {
	if File_go_api_enum_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_go_api_enum_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_go_api_enum_proto_goTypes,
		DependencyIndexes: file_go_api_enum_proto_depIdxs,
		EnumInfos:         file_go_api_enum_proto_enumTypes,
	}.Build()
	File_go_api_enum_proto = out.File
	file_go_api_enum_proto_rawDesc = nil
	file_go_api_enum_proto_goTypes = nil
	file_go_api_enum_proto_depIdxs = nil
}
