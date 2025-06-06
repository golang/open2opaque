// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2-devel
// 	protoc        v5.29.1
// source: proto3test.proto

//go:build !protoopaque

package proto3test_go_proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type M3_Enum int32

const (
	M3_E_VAL M3_Enum = 0
)

// Enum value maps for M3_Enum.
var (
	M3_Enum_name = map[int32]string{
		0: "E_VAL",
	}
	M3_Enum_value = map[string]int32{
		"E_VAL": 0,
	}
)

func (x M3_Enum) Enum() *M3_Enum {
	p := new(M3_Enum)
	*p = x
	return p
}

func (x M3_Enum) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (M3_Enum) Descriptor() protoreflect.EnumDescriptor {
	return file_proto3test_proto_enumTypes[0].Descriptor()
}

func (M3_Enum) Type() protoreflect.EnumType {
	return &file_proto3test_proto_enumTypes[0]
}

func (x M3_Enum) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

type M3 struct {
	state protoimpl.MessageState `protogen:"hybrid.v1"`
	B     bool                   `protobuf:"varint,1,opt,name=b,proto3" json:"b,omitempty"`
	Bytes []byte                 `protobuf:"bytes,2,opt,name=bytes,proto3" json:"bytes,omitempty"`
	F32   float32                `protobuf:"fixed32,3,opt,name=f32,proto3" json:"f32,omitempty"`
	F64   float64                `protobuf:"fixed64,4,opt,name=f64,proto3" json:"f64,omitempty"`
	I32   int32                  `protobuf:"varint,5,opt,name=i32,proto3" json:"i32,omitempty"`
	I64   int64                  `protobuf:"varint,6,opt,name=i64,proto3" json:"i64,omitempty"`
	Ui32  uint32                 `protobuf:"varint,7,opt,name=ui32,proto3" json:"ui32,omitempty"`
	Ui64  uint64                 `protobuf:"varint,8,opt,name=ui64,proto3" json:"ui64,omitempty"`
	S     string                 `protobuf:"bytes,9,opt,name=s,proto3" json:"s,omitempty"`
	M     *M3                    `protobuf:"bytes,10,opt,name=m,proto3" json:"m,omitempty"`
	Is    []int32                `protobuf:"varint,11,rep,packed,name=is,proto3" json:"is,omitempty"`
	Ms    []*M3                  `protobuf:"bytes,12,rep,name=ms,proto3" json:"ms,omitempty"`
	Map   map[string]bool        `protobuf:"bytes,29,rep,name=map,proto3" json:"map,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"varint,2,opt,name=value"`
	E     M3_Enum                `protobuf:"varint,13,opt,name=e,proto3,enum=net.proto2.go.open2opaque.o2o.test3.M3_Enum" json:"e,omitempty"`
	// Types that are valid to be assigned to OneofField:
	//
	//	*M3_StringOneof
	//	*M3_IntOneof
	//	*M3_MsgOneof
	//	*M3_EnumOneof
	//	*M3_BytesOneof
	//	*M3_Build
	//	*M3_ProtoMessage_
	//	*M3_Reset_
	//	*M3_String_
	//	*M3_Descriptor_
	OneofField isM3_OneofField `protobuf_oneof:"oneof_field"`
	SecondI32  int32           `protobuf:"varint,30,opt,name=second_i32,json=secondI32,proto3" json:"second_i32,omitempty"`
	OptB       *bool           `protobuf:"varint,31,opt,name=opt_b,json=optB,proto3,oneof" json:"opt_b,omitempty"`
	OptBytes   []byte          `protobuf:"bytes,32,opt,name=opt_bytes,json=optBytes,proto3,oneof" json:"opt_bytes,omitempty"`
	OptF32     *float32        `protobuf:"fixed32,33,opt,name=opt_f32,json=optF32,proto3,oneof" json:"opt_f32,omitempty"`
	OptF64     *float64        `protobuf:"fixed64,34,opt,name=opt_f64,json=optF64,proto3,oneof" json:"opt_f64,omitempty"`
	OptI32     *int32          `protobuf:"varint,35,opt,name=opt_i32,json=optI32,proto3,oneof" json:"opt_i32,omitempty"`
	OptI64     *int64          `protobuf:"varint,36,opt,name=opt_i64,json=optI64,proto3,oneof" json:"opt_i64,omitempty"`
	OptUi32    *uint32         `protobuf:"varint,37,opt,name=opt_ui32,json=optUi32,proto3,oneof" json:"opt_ui32,omitempty"`
	OptUi64    *uint64         `protobuf:"varint,38,opt,name=opt_ui64,json=optUi64,proto3,oneof" json:"opt_ui64,omitempty"`
	OptS       *string         `protobuf:"bytes,39,opt,name=opt_s,json=optS,proto3,oneof" json:"opt_s,omitempty"`
	OptM       *M3             `protobuf:"bytes,40,opt,name=opt_m,json=optM,proto3,oneof" json:"opt_m,omitempty"`
	// Repeated fields and maps cannot be optional.
	OptE          *M3_Enum `protobuf:"varint,41,opt,name=opt_e,json=optE,proto3,enum=net.proto2.go.open2opaque.o2o.test3.M3_Enum,oneof" json:"opt_e,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *M3) Reset() {
	*x = M3{}
	mi := &file_proto3test_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *M3) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*M3) ProtoMessage() {}

func (x *M3) ProtoReflect() protoreflect.Message {
	mi := &file_proto3test_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *M3) GetB() bool {
	if x != nil {
		return x.B
	}
	return false
}

func (x *M3) GetBytes() []byte {
	if x != nil {
		return x.Bytes
	}
	return nil
}

func (x *M3) GetF32() float32 {
	if x != nil {
		return x.F32
	}
	return 0
}

func (x *M3) GetF64() float64 {
	if x != nil {
		return x.F64
	}
	return 0
}

func (x *M3) GetI32() int32 {
	if x != nil {
		return x.I32
	}
	return 0
}

func (x *M3) GetI64() int64 {
	if x != nil {
		return x.I64
	}
	return 0
}

func (x *M3) GetUi32() uint32 {
	if x != nil {
		return x.Ui32
	}
	return 0
}

func (x *M3) GetUi64() uint64 {
	if x != nil {
		return x.Ui64
	}
	return 0
}

func (x *M3) GetS() string {
	if x != nil {
		return x.S
	}
	return ""
}

func (x *M3) GetM() *M3 {
	if x != nil {
		return x.M
	}
	return nil
}

func (x *M3) GetIs() []int32 {
	if x != nil {
		return x.Is
	}
	return nil
}

func (x *M3) GetMs() []*M3 {
	if x != nil {
		return x.Ms
	}
	return nil
}

func (x *M3) GetMap() map[string]bool {
	if x != nil {
		return x.Map
	}
	return nil
}

func (x *M3) GetE() M3_Enum {
	if x != nil {
		return x.E
	}
	return M3_E_VAL
}

func (x *M3) GetOneofField() isM3_OneofField {
	if x != nil {
		return x.OneofField
	}
	return nil
}

func (x *M3) GetStringOneof() string {
	if x != nil {
		if x, ok := x.OneofField.(*M3_StringOneof); ok {
			return x.StringOneof
		}
	}
	return ""
}

func (x *M3) GetIntOneof() int64 {
	if x != nil {
		if x, ok := x.OneofField.(*M3_IntOneof); ok {
			return x.IntOneof
		}
	}
	return 0
}

func (x *M3) GetMsgOneof() *M3 {
	if x != nil {
		if x, ok := x.OneofField.(*M3_MsgOneof); ok {
			return x.MsgOneof
		}
	}
	return nil
}

func (x *M3) GetEnumOneof() M3_Enum {
	if x != nil {
		if x, ok := x.OneofField.(*M3_EnumOneof); ok {
			return x.EnumOneof
		}
	}
	return M3_E_VAL
}

func (x *M3) GetBytesOneof() []byte {
	if x != nil {
		if x, ok := x.OneofField.(*M3_BytesOneof); ok {
			return x.BytesOneof
		}
	}
	return nil
}

func (x *M3) GetBuild_() int32 {
	if x != nil {
		if x, ok := x.OneofField.(*M3_Build); ok {
			return x.Build
		}
	}
	return 0
}

// Deprecated: Use GetBuild_ instead.
func (x *M3) GetBuild() int32 {
	return x.GetBuild_()
}

func (x *M3) GetProtoMessage() string {
	if x != nil {
		if x, ok := x.OneofField.(*M3_ProtoMessage_); ok {
			return x.ProtoMessage_
		}
	}
	return ""
}

// Deprecated: Use GetProtoMessage instead.
func (x *M3) GetProtoMessage_() string {
	return x.GetProtoMessage()
}

func (x *M3) GetReset() string {
	if x != nil {
		if x, ok := x.OneofField.(*M3_Reset_); ok {
			return x.Reset_
		}
	}
	return ""
}

// Deprecated: Use GetReset instead.
func (x *M3) GetReset_() string {
	return x.GetReset()
}

func (x *M3) GetString() string {
	if x != nil {
		if x, ok := x.OneofField.(*M3_String_); ok {
			return x.String_
		}
	}
	return ""
}

// Deprecated: Use GetString instead.
func (x *M3) GetString_() string {
	return x.GetString()
}

func (x *M3) GetDescriptor() string {
	if x != nil {
		if x, ok := x.OneofField.(*M3_Descriptor_); ok {
			return x.Descriptor_
		}
	}
	return ""
}

// Deprecated: Use GetDescriptor instead.
func (x *M3) GetDescriptor_() string {
	return x.GetDescriptor()
}

func (x *M3) GetSecondI32() int32 {
	if x != nil {
		return x.SecondI32
	}
	return 0
}

func (x *M3) GetOptB() bool {
	if x != nil && x.OptB != nil {
		return *x.OptB
	}
	return false
}

func (x *M3) GetOptBytes() []byte {
	if x != nil {
		return x.OptBytes
	}
	return nil
}

func (x *M3) GetOptF32() float32 {
	if x != nil && x.OptF32 != nil {
		return *x.OptF32
	}
	return 0
}

func (x *M3) GetOptF64() float64 {
	if x != nil && x.OptF64 != nil {
		return *x.OptF64
	}
	return 0
}

func (x *M3) GetOptI32() int32 {
	if x != nil && x.OptI32 != nil {
		return *x.OptI32
	}
	return 0
}

func (x *M3) GetOptI64() int64 {
	if x != nil && x.OptI64 != nil {
		return *x.OptI64
	}
	return 0
}

func (x *M3) GetOptUi32() uint32 {
	if x != nil && x.OptUi32 != nil {
		return *x.OptUi32
	}
	return 0
}

func (x *M3) GetOptUi64() uint64 {
	if x != nil && x.OptUi64 != nil {
		return *x.OptUi64
	}
	return 0
}

func (x *M3) GetOptS() string {
	if x != nil && x.OptS != nil {
		return *x.OptS
	}
	return ""
}

func (x *M3) GetOptM() *M3 {
	if x != nil {
		return x.OptM
	}
	return nil
}

func (x *M3) GetOptE() M3_Enum {
	if x != nil && x.OptE != nil {
		return *x.OptE
	}
	return M3_E_VAL
}

func (x *M3) SetB(v bool) {
	x.B = v
}

func (x *M3) SetBytes(v []byte) {
	if v == nil {
		v = []byte{}
	}
	x.Bytes = v
}

func (x *M3) SetF32(v float32) {
	x.F32 = v
}

func (x *M3) SetF64(v float64) {
	x.F64 = v
}

func (x *M3) SetI32(v int32) {
	x.I32 = v
}

func (x *M3) SetI64(v int64) {
	x.I64 = v
}

func (x *M3) SetUi32(v uint32) {
	x.Ui32 = v
}

func (x *M3) SetUi64(v uint64) {
	x.Ui64 = v
}

func (x *M3) SetS(v string) {
	x.S = v
}

func (x *M3) SetM(v *M3) {
	x.M = v
}

func (x *M3) SetIs(v []int32) {
	x.Is = v
}

func (x *M3) SetMs(v []*M3) {
	x.Ms = v
}

func (x *M3) SetMap(v map[string]bool) {
	x.Map = v
}

func (x *M3) SetE(v M3_Enum) {
	x.E = v
}

func (x *M3) SetStringOneof(v string) {
	x.OneofField = &M3_StringOneof{v}
}

func (x *M3) SetIntOneof(v int64) {
	x.OneofField = &M3_IntOneof{v}
}

func (x *M3) SetMsgOneof(v *M3) {
	if v == nil {
		x.OneofField = nil
		return
	}
	x.OneofField = &M3_MsgOneof{v}
}

func (x *M3) SetEnumOneof(v M3_Enum) {
	x.OneofField = &M3_EnumOneof{v}
}

func (x *M3) SetBytesOneof(v []byte) {
	if v == nil {
		v = []byte{}
	}
	x.OneofField = &M3_BytesOneof{v}
}

func (x *M3) SetBuild_(v int32) {
	x.OneofField = &M3_Build{v}
}

func (x *M3) SetProtoMessage(v string) {
	x.OneofField = &M3_ProtoMessage_{v}
}

func (x *M3) SetReset(v string) {
	x.OneofField = &M3_Reset_{v}
}

func (x *M3) SetString(v string) {
	x.OneofField = &M3_String_{v}
}

func (x *M3) SetDescriptor(v string) {
	x.OneofField = &M3_Descriptor_{v}
}

func (x *M3) SetSecondI32(v int32) {
	x.SecondI32 = v
}

func (x *M3) SetOptB(v bool) {
	x.OptB = &v
}

func (x *M3) SetOptBytes(v []byte) {
	if v == nil {
		v = []byte{}
	}
	x.OptBytes = v
}

func (x *M3) SetOptF32(v float32) {
	x.OptF32 = &v
}

func (x *M3) SetOptF64(v float64) {
	x.OptF64 = &v
}

func (x *M3) SetOptI32(v int32) {
	x.OptI32 = &v
}

func (x *M3) SetOptI64(v int64) {
	x.OptI64 = &v
}

func (x *M3) SetOptUi32(v uint32) {
	x.OptUi32 = &v
}

func (x *M3) SetOptUi64(v uint64) {
	x.OptUi64 = &v
}

func (x *M3) SetOptS(v string) {
	x.OptS = &v
}

func (x *M3) SetOptM(v *M3) {
	x.OptM = v
}

func (x *M3) SetOptE(v M3_Enum) {
	x.OptE = &v
}

func (x *M3) HasM() bool {
	if x == nil {
		return false
	}
	return x.M != nil
}

func (x *M3) HasOneofField() bool {
	if x == nil {
		return false
	}
	return x.OneofField != nil
}

func (x *M3) HasStringOneof() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_StringOneof)
	return ok
}

func (x *M3) HasIntOneof() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_IntOneof)
	return ok
}

func (x *M3) HasMsgOneof() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_MsgOneof)
	return ok
}

func (x *M3) HasEnumOneof() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_EnumOneof)
	return ok
}

func (x *M3) HasBytesOneof() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_BytesOneof)
	return ok
}

func (x *M3) HasBuild_() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_Build)
	return ok
}

func (x *M3) HasProtoMessage() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_ProtoMessage_)
	return ok
}

func (x *M3) HasReset() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_Reset_)
	return ok
}

func (x *M3) HasString() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_String_)
	return ok
}

func (x *M3) HasDescriptor() bool {
	if x == nil {
		return false
	}
	_, ok := x.OneofField.(*M3_Descriptor_)
	return ok
}

func (x *M3) HasOptB() bool {
	if x == nil {
		return false
	}
	return x.OptB != nil
}

func (x *M3) HasOptBytes() bool {
	if x == nil {
		return false
	}
	return x.OptBytes != nil
}

func (x *M3) HasOptF32() bool {
	if x == nil {
		return false
	}
	return x.OptF32 != nil
}

func (x *M3) HasOptF64() bool {
	if x == nil {
		return false
	}
	return x.OptF64 != nil
}

func (x *M3) HasOptI32() bool {
	if x == nil {
		return false
	}
	return x.OptI32 != nil
}

func (x *M3) HasOptI64() bool {
	if x == nil {
		return false
	}
	return x.OptI64 != nil
}

func (x *M3) HasOptUi32() bool {
	if x == nil {
		return false
	}
	return x.OptUi32 != nil
}

func (x *M3) HasOptUi64() bool {
	if x == nil {
		return false
	}
	return x.OptUi64 != nil
}

func (x *M3) HasOptS() bool {
	if x == nil {
		return false
	}
	return x.OptS != nil
}

func (x *M3) HasOptM() bool {
	if x == nil {
		return false
	}
	return x.OptM != nil
}

func (x *M3) HasOptE() bool {
	if x == nil {
		return false
	}
	return x.OptE != nil
}

func (x *M3) ClearM() {
	x.M = nil
}

func (x *M3) ClearOneofField() {
	x.OneofField = nil
}

func (x *M3) ClearStringOneof() {
	if _, ok := x.OneofField.(*M3_StringOneof); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearIntOneof() {
	if _, ok := x.OneofField.(*M3_IntOneof); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearMsgOneof() {
	if _, ok := x.OneofField.(*M3_MsgOneof); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearEnumOneof() {
	if _, ok := x.OneofField.(*M3_EnumOneof); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearBytesOneof() {
	if _, ok := x.OneofField.(*M3_BytesOneof); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearBuild_() {
	if _, ok := x.OneofField.(*M3_Build); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearProtoMessage() {
	if _, ok := x.OneofField.(*M3_ProtoMessage_); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearReset() {
	if _, ok := x.OneofField.(*M3_Reset_); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearString() {
	if _, ok := x.OneofField.(*M3_String_); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearDescriptor() {
	if _, ok := x.OneofField.(*M3_Descriptor_); ok {
		x.OneofField = nil
	}
}

func (x *M3) ClearOptB() {
	x.OptB = nil
}

func (x *M3) ClearOptBytes() {
	x.OptBytes = nil
}

func (x *M3) ClearOptF32() {
	x.OptF32 = nil
}

func (x *M3) ClearOptF64() {
	x.OptF64 = nil
}

func (x *M3) ClearOptI32() {
	x.OptI32 = nil
}

func (x *M3) ClearOptI64() {
	x.OptI64 = nil
}

func (x *M3) ClearOptUi32() {
	x.OptUi32 = nil
}

func (x *M3) ClearOptUi64() {
	x.OptUi64 = nil
}

func (x *M3) ClearOptS() {
	x.OptS = nil
}

func (x *M3) ClearOptM() {
	x.OptM = nil
}

func (x *M3) ClearOptE() {
	x.OptE = nil
}

const M3_OneofField_not_set_case case_M3_OneofField = 0
const M3_StringOneof_case case_M3_OneofField = 14
const M3_IntOneof_case case_M3_OneofField = 15
const M3_MsgOneof_case case_M3_OneofField = 16
const M3_EnumOneof_case case_M3_OneofField = 17
const M3_BytesOneof_case case_M3_OneofField = 18
const M3_Build_case case_M3_OneofField = 24
const M3_ProtoMessage__case case_M3_OneofField = 25
const M3_Reset__case case_M3_OneofField = 26
const M3_String__case case_M3_OneofField = 27
const M3_Descriptor__case case_M3_OneofField = 28

func (x *M3) WhichOneofField() case_M3_OneofField {
	if x == nil {
		return M3_OneofField_not_set_case
	}
	switch x.OneofField.(type) {
	case *M3_StringOneof:
		return M3_StringOneof_case
	case *M3_IntOneof:
		return M3_IntOneof_case
	case *M3_MsgOneof:
		return M3_MsgOneof_case
	case *M3_EnumOneof:
		return M3_EnumOneof_case
	case *M3_BytesOneof:
		return M3_BytesOneof_case
	case *M3_Build:
		return M3_Build_case
	case *M3_ProtoMessage_:
		return M3_ProtoMessage__case
	case *M3_Reset_:
		return M3_Reset__case
	case *M3_String_:
		return M3_String__case
	case *M3_Descriptor_:
		return M3_Descriptor__case
	default:
		return M3_OneofField_not_set_case
	}
}

type M3_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	B     bool
	Bytes []byte
	F32   float32
	F64   float64
	I32   int32
	I64   int64
	Ui32  uint32
	Ui64  uint64
	S     string
	M     *M3
	Is    []int32
	Ms    []*M3
	Map   map[string]bool
	E     M3_Enum
	// Fields of oneof OneofField:
	StringOneof  *string
	IntOneof     *int64
	MsgOneof     *M3
	EnumOneof    *M3_Enum
	BytesOneof   []byte
	Build_       *int32
	ProtoMessage *string
	Reset        *string
	String       *string
	Descriptor   *string
	// -- end of OneofField
	SecondI32 int32
	OptB      *bool
	OptBytes  []byte
	OptF32    *float32
	OptF64    *float64
	OptI32    *int32
	OptI64    *int64
	OptUi32   *uint32
	OptUi64   *uint64
	OptS      *string
	OptM      *M3
	// Repeated fields and maps cannot be optional.
	OptE *M3_Enum
}

func (b0 M3_builder) Build() *M3 {
	m0 := &M3{}
	b, x := &b0, m0
	_, _ = b, x
	x.B = b.B
	x.Bytes = b.Bytes
	x.F32 = b.F32
	x.F64 = b.F64
	x.I32 = b.I32
	x.I64 = b.I64
	x.Ui32 = b.Ui32
	x.Ui64 = b.Ui64
	x.S = b.S
	x.M = b.M
	x.Is = b.Is
	x.Ms = b.Ms
	x.Map = b.Map
	x.E = b.E
	if b.StringOneof != nil {
		x.OneofField = &M3_StringOneof{*b.StringOneof}
	}
	if b.IntOneof != nil {
		x.OneofField = &M3_IntOneof{*b.IntOneof}
	}
	if b.MsgOneof != nil {
		x.OneofField = &M3_MsgOneof{b.MsgOneof}
	}
	if b.EnumOneof != nil {
		x.OneofField = &M3_EnumOneof{*b.EnumOneof}
	}
	if b.BytesOneof != nil {
		x.OneofField = &M3_BytesOneof{b.BytesOneof}
	}
	if b.Build_ != nil {
		x.OneofField = &M3_Build{*b.Build_}
	}
	if b.ProtoMessage != nil {
		x.OneofField = &M3_ProtoMessage_{*b.ProtoMessage}
	}
	if b.Reset != nil {
		x.OneofField = &M3_Reset_{*b.Reset}
	}
	if b.String != nil {
		x.OneofField = &M3_String_{*b.String}
	}
	if b.Descriptor != nil {
		x.OneofField = &M3_Descriptor_{*b.Descriptor}
	}
	x.SecondI32 = b.SecondI32
	x.OptB = b.OptB
	x.OptBytes = b.OptBytes
	x.OptF32 = b.OptF32
	x.OptF64 = b.OptF64
	x.OptI32 = b.OptI32
	x.OptI64 = b.OptI64
	x.OptUi32 = b.OptUi32
	x.OptUi64 = b.OptUi64
	x.OptS = b.OptS
	x.OptM = b.OptM
	x.OptE = b.OptE
	return m0
}

type case_M3_OneofField protoreflect.FieldNumber

func (x case_M3_OneofField) String() string {
	md := file_proto3test_proto_msgTypes[0].Descriptor()
	if x == 0 {
		return "not set"
	}
	return protoimpl.X.MessageFieldStringOf(md, protoreflect.FieldNumber(x))
}

type isM3_OneofField interface {
	isM3_OneofField()
}

type M3_StringOneof struct {
	StringOneof string `protobuf:"bytes,14,opt,name=string_oneof,json=stringOneof,proto3,oneof"`
}

type M3_IntOneof struct {
	IntOneof int64 `protobuf:"varint,15,opt,name=int_oneof,json=intOneof,proto3,oneof"`
}

type M3_MsgOneof struct {
	MsgOneof *M3 `protobuf:"bytes,16,opt,name=msg_oneof,json=msgOneof,proto3,oneof"`
}

type M3_EnumOneof struct {
	EnumOneof M3_Enum `protobuf:"varint,17,opt,name=enum_oneof,json=enumOneof,proto3,enum=net.proto2.go.open2opaque.o2o.test3.M3_Enum,oneof"`
}

type M3_BytesOneof struct {
	BytesOneof []byte `protobuf:"bytes,18,opt,name=bytes_oneof,json=bytesOneof,proto3,oneof"`
}

type M3_Build struct {
	Build int32 `protobuf:"varint,24,opt,name=build,proto3,oneof"`
}

type M3_ProtoMessage_ struct {
	ProtoMessage_ string `protobuf:"bytes,25,opt,name=proto_message,json=protoMessage,proto3,oneof"`
}

type M3_Reset_ struct {
	Reset_ string `protobuf:"bytes,26,opt,name=reset,proto3,oneof"`
}

type M3_String_ struct {
	String_ string `protobuf:"bytes,27,opt,name=string,proto3,oneof"`
}

type M3_Descriptor_ struct {
	Descriptor_ string `protobuf:"bytes,28,opt,name=descriptor,proto3,oneof"`
}

func (*M3_StringOneof) isM3_OneofField() {}

func (*M3_IntOneof) isM3_OneofField() {}

func (*M3_MsgOneof) isM3_OneofField() {}

func (*M3_EnumOneof) isM3_OneofField() {}

func (*M3_BytesOneof) isM3_OneofField() {}

func (*M3_Build) isM3_OneofField() {}

func (*M3_ProtoMessage_) isM3_OneofField() {}

func (*M3_Reset_) isM3_OneofField() {}

func (*M3_String_) isM3_OneofField() {}

func (*M3_Descriptor_) isM3_OneofField() {}

var File_proto3test_proto protoreflect.FileDescriptor

var file_proto3test_proto_rawDesc = []byte{
	0x0a, 0x10, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x23, 0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67,
	0x6f, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32,
	0x6f, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x33, 0x22, 0xc9, 0x0b, 0x0a, 0x02, 0x4d, 0x33, 0x12, 0x0c,
	0x0a, 0x01, 0x62, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x01, 0x62, 0x12, 0x14, 0x0a, 0x05,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x62, 0x79, 0x74,
	0x65, 0x73, 0x12, 0x10, 0x0a, 0x03, 0x66, 0x33, 0x32, 0x18, 0x03, 0x20, 0x01, 0x28, 0x02, 0x52,
	0x03, 0x66, 0x33, 0x32, 0x12, 0x10, 0x0a, 0x03, 0x66, 0x36, 0x34, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x01, 0x52, 0x03, 0x66, 0x36, 0x34, 0x12, 0x10, 0x0a, 0x03, 0x69, 0x33, 0x32, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x03, 0x69, 0x33, 0x32, 0x12, 0x10, 0x0a, 0x03, 0x69, 0x36, 0x34, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x03, 0x69, 0x36, 0x34, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x69,
	0x33, 0x32, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x75, 0x69, 0x33, 0x32, 0x12, 0x12,
	0x0a, 0x04, 0x75, 0x69, 0x36, 0x34, 0x18, 0x08, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x75, 0x69,
	0x36, 0x34, 0x12, 0x0c, 0x0a, 0x01, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x01, 0x73,
	0x12, 0x35, 0x0a, 0x01, 0x6d, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x6e, 0x65,
	0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67, 0x6f, 0x2e, 0x6f, 0x70, 0x65, 0x6e,
	0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32, 0x6f, 0x2e, 0x74, 0x65, 0x73, 0x74,
	0x33, 0x2e, 0x4d, 0x33, 0x52, 0x01, 0x6d, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x73, 0x18, 0x0b, 0x20,
	0x03, 0x28, 0x05, 0x52, 0x02, 0x69, 0x73, 0x12, 0x37, 0x0a, 0x02, 0x6d, 0x73, 0x18, 0x0c, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32,
	0x2e, 0x67, 0x6f, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e,
	0x6f, 0x32, 0x6f, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x33, 0x2e, 0x4d, 0x33, 0x52, 0x02, 0x6d, 0x73,
	0x12, 0x42, 0x0a, 0x03, 0x6d, 0x61, 0x70, 0x18, 0x1d, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x30, 0x2e,
	0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67, 0x6f, 0x2e, 0x6f, 0x70,
	0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32, 0x6f, 0x2e, 0x74, 0x65,
	0x73, 0x74, 0x33, 0x2e, 0x4d, 0x33, 0x2e, 0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52,
	0x03, 0x6d, 0x61, 0x70, 0x12, 0x3a, 0x0a, 0x01, 0x65, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x2c, 0x2e, 0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67, 0x6f, 0x2e,
	0x6f, 0x70, 0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32, 0x6f, 0x2e,
	0x74, 0x65, 0x73, 0x74, 0x33, 0x2e, 0x4d, 0x33, 0x2e, 0x45, 0x6e, 0x75, 0x6d, 0x52, 0x01, 0x65,
	0x12, 0x23, 0x0a, 0x0c, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x5f, 0x6f, 0x6e, 0x65, 0x6f, 0x66,
	0x18, 0x0e, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x0b, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67,
	0x4f, 0x6e, 0x65, 0x6f, 0x66, 0x12, 0x1d, 0x0a, 0x09, 0x69, 0x6e, 0x74, 0x5f, 0x6f, 0x6e, 0x65,
	0x6f, 0x66, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x03, 0x48, 0x00, 0x52, 0x08, 0x69, 0x6e, 0x74, 0x4f,
	0x6e, 0x65, 0x6f, 0x66, 0x12, 0x46, 0x0a, 0x09, 0x6d, 0x73, 0x67, 0x5f, 0x6f, 0x6e, 0x65, 0x6f,
	0x66, 0x18, 0x10, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67, 0x6f, 0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61,
	0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32, 0x6f, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x33, 0x2e, 0x4d, 0x33,
	0x48, 0x00, 0x52, 0x08, 0x6d, 0x73, 0x67, 0x4f, 0x6e, 0x65, 0x6f, 0x66, 0x12, 0x4d, 0x0a, 0x0a,
	0x65, 0x6e, 0x75, 0x6d, 0x5f, 0x6f, 0x6e, 0x65, 0x6f, 0x66, 0x18, 0x11, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x2c, 0x2e, 0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67, 0x6f,
	0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32, 0x6f,
	0x2e, 0x74, 0x65, 0x73, 0x74, 0x33, 0x2e, 0x4d, 0x33, 0x2e, 0x45, 0x6e, 0x75, 0x6d, 0x48, 0x00,
	0x52, 0x09, 0x65, 0x6e, 0x75, 0x6d, 0x4f, 0x6e, 0x65, 0x6f, 0x66, 0x12, 0x21, 0x0a, 0x0b, 0x62,
	0x79, 0x74, 0x65, 0x73, 0x5f, 0x6f, 0x6e, 0x65, 0x6f, 0x66, 0x18, 0x12, 0x20, 0x01, 0x28, 0x0c,
	0x48, 0x00, 0x52, 0x0a, 0x62, 0x79, 0x74, 0x65, 0x73, 0x4f, 0x6e, 0x65, 0x6f, 0x66, 0x12, 0x16,
	0x0a, 0x05, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x18, 0x18, 0x20, 0x01, 0x28, 0x05, 0x48, 0x00, 0x52,
	0x05, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x12, 0x25, 0x0a, 0x0d, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x19, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52,
	0x0c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x16, 0x0a,
	0x05, 0x72, 0x65, 0x73, 0x65, 0x74, 0x18, 0x1a, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x05,
	0x72, 0x65, 0x73, 0x65, 0x74, 0x12, 0x18, 0x0a, 0x06, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x18,
	0x1b, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x06, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x12,
	0x20, 0x0a, 0x0a, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x18, 0x1c, 0x20,
	0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x0a, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f,
	0x72, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x5f, 0x69, 0x33, 0x32, 0x18,
	0x1e, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x49, 0x33, 0x32,
	0x12, 0x18, 0x0a, 0x05, 0x6f, 0x70, 0x74, 0x5f, 0x62, 0x18, 0x1f, 0x20, 0x01, 0x28, 0x08, 0x48,
	0x01, 0x52, 0x04, 0x6f, 0x70, 0x74, 0x42, 0x88, 0x01, 0x01, 0x12, 0x20, 0x0a, 0x09, 0x6f, 0x70,
	0x74, 0x5f, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x20, 0x20, 0x01, 0x28, 0x0c, 0x48, 0x02, 0x52,
	0x08, 0x6f, 0x70, 0x74, 0x42, 0x79, 0x74, 0x65, 0x73, 0x88, 0x01, 0x01, 0x12, 0x1c, 0x0a, 0x07,
	0x6f, 0x70, 0x74, 0x5f, 0x66, 0x33, 0x32, 0x18, 0x21, 0x20, 0x01, 0x28, 0x02, 0x48, 0x03, 0x52,
	0x06, 0x6f, 0x70, 0x74, 0x46, 0x33, 0x32, 0x88, 0x01, 0x01, 0x12, 0x1c, 0x0a, 0x07, 0x6f, 0x70,
	0x74, 0x5f, 0x66, 0x36, 0x34, 0x18, 0x22, 0x20, 0x01, 0x28, 0x01, 0x48, 0x04, 0x52, 0x06, 0x6f,
	0x70, 0x74, 0x46, 0x36, 0x34, 0x88, 0x01, 0x01, 0x12, 0x1c, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x5f,
	0x69, 0x33, 0x32, 0x18, 0x23, 0x20, 0x01, 0x28, 0x05, 0x48, 0x05, 0x52, 0x06, 0x6f, 0x70, 0x74,
	0x49, 0x33, 0x32, 0x88, 0x01, 0x01, 0x12, 0x1c, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x5f, 0x69, 0x36,
	0x34, 0x18, 0x24, 0x20, 0x01, 0x28, 0x03, 0x48, 0x06, 0x52, 0x06, 0x6f, 0x70, 0x74, 0x49, 0x36,
	0x34, 0x88, 0x01, 0x01, 0x12, 0x1e, 0x0a, 0x08, 0x6f, 0x70, 0x74, 0x5f, 0x75, 0x69, 0x33, 0x32,
	0x18, 0x25, 0x20, 0x01, 0x28, 0x0d, 0x48, 0x07, 0x52, 0x07, 0x6f, 0x70, 0x74, 0x55, 0x69, 0x33,
	0x32, 0x88, 0x01, 0x01, 0x12, 0x1e, 0x0a, 0x08, 0x6f, 0x70, 0x74, 0x5f, 0x75, 0x69, 0x36, 0x34,
	0x18, 0x26, 0x20, 0x01, 0x28, 0x04, 0x48, 0x08, 0x52, 0x07, 0x6f, 0x70, 0x74, 0x55, 0x69, 0x36,
	0x34, 0x88, 0x01, 0x01, 0x12, 0x18, 0x0a, 0x05, 0x6f, 0x70, 0x74, 0x5f, 0x73, 0x18, 0x27, 0x20,
	0x01, 0x28, 0x09, 0x48, 0x09, 0x52, 0x04, 0x6f, 0x70, 0x74, 0x53, 0x88, 0x01, 0x01, 0x12, 0x41,
	0x0a, 0x05, 0x6f, 0x70, 0x74, 0x5f, 0x6d, 0x18, 0x28, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x27, 0x2e,
	0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67, 0x6f, 0x2e, 0x6f, 0x70,
	0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32, 0x6f, 0x2e, 0x74, 0x65,
	0x73, 0x74, 0x33, 0x2e, 0x4d, 0x33, 0x48, 0x0a, 0x52, 0x04, 0x6f, 0x70, 0x74, 0x4d, 0x88, 0x01,
	0x01, 0x12, 0x46, 0x0a, 0x05, 0x6f, 0x70, 0x74, 0x5f, 0x65, 0x18, 0x29, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x2c, 0x2e, 0x6e, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x2e, 0x67, 0x6f,
	0x2e, 0x6f, 0x70, 0x65, 0x6e, 0x32, 0x6f, 0x70, 0x61, 0x71, 0x75, 0x65, 0x2e, 0x6f, 0x32, 0x6f,
	0x2e, 0x74, 0x65, 0x73, 0x74, 0x33, 0x2e, 0x4d, 0x33, 0x2e, 0x45, 0x6e, 0x75, 0x6d, 0x48, 0x0b,
	0x52, 0x04, 0x6f, 0x70, 0x74, 0x45, 0x88, 0x01, 0x01, 0x1a, 0x36, 0x0a, 0x08, 0x4d, 0x61, 0x70,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38,
	0x01, 0x22, 0x11, 0x0a, 0x04, 0x45, 0x6e, 0x75, 0x6d, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x5f, 0x56,
	0x41, 0x4c, 0x10, 0x00, 0x42, 0x0d, 0x0a, 0x0b, 0x6f, 0x6e, 0x65, 0x6f, 0x66, 0x5f, 0x66, 0x69,
	0x65, 0x6c, 0x64, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x6f, 0x70, 0x74, 0x5f, 0x62, 0x42, 0x0c, 0x0a,
	0x0a, 0x5f, 0x6f, 0x70, 0x74, 0x5f, 0x62, 0x79, 0x74, 0x65, 0x73, 0x42, 0x0a, 0x0a, 0x08, 0x5f,
	0x6f, 0x70, 0x74, 0x5f, 0x66, 0x33, 0x32, 0x42, 0x0a, 0x0a, 0x08, 0x5f, 0x6f, 0x70, 0x74, 0x5f,
	0x66, 0x36, 0x34, 0x42, 0x0a, 0x0a, 0x08, 0x5f, 0x6f, 0x70, 0x74, 0x5f, 0x69, 0x33, 0x32, 0x42,
	0x0a, 0x0a, 0x08, 0x5f, 0x6f, 0x70, 0x74, 0x5f, 0x69, 0x36, 0x34, 0x42, 0x0b, 0x0a, 0x09, 0x5f,
	0x6f, 0x70, 0x74, 0x5f, 0x75, 0x69, 0x33, 0x32, 0x42, 0x0b, 0x0a, 0x09, 0x5f, 0x6f, 0x70, 0x74,
	0x5f, 0x75, 0x69, 0x36, 0x34, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x6f, 0x70, 0x74, 0x5f, 0x73, 0x42,
	0x08, 0x0a, 0x06, 0x5f, 0x6f, 0x70, 0x74, 0x5f, 0x6d, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x6f, 0x70,
	0x74, 0x5f, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var file_proto3test_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto3test_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto3test_proto_goTypes = []any{
	(M3_Enum)(0), // 0: net.proto2.go.open2opaque.o2o.test3.M3.Enum
	(*M3)(nil),   // 1: net.proto2.go.open2opaque.o2o.test3.M3
	nil,          // 2: net.proto2.go.open2opaque.o2o.test3.M3.MapEntry
}
var file_proto3test_proto_depIdxs = []int32{
	1, // 0: net.proto2.go.open2opaque.o2o.test3.M3.m:type_name -> net.proto2.go.open2opaque.o2o.test3.M3
	1, // 1: net.proto2.go.open2opaque.o2o.test3.M3.ms:type_name -> net.proto2.go.open2opaque.o2o.test3.M3
	2, // 2: net.proto2.go.open2opaque.o2o.test3.M3.map:type_name -> net.proto2.go.open2opaque.o2o.test3.M3.MapEntry
	0, // 3: net.proto2.go.open2opaque.o2o.test3.M3.e:type_name -> net.proto2.go.open2opaque.o2o.test3.M3.Enum
	1, // 4: net.proto2.go.open2opaque.o2o.test3.M3.msg_oneof:type_name -> net.proto2.go.open2opaque.o2o.test3.M3
	0, // 5: net.proto2.go.open2opaque.o2o.test3.M3.enum_oneof:type_name -> net.proto2.go.open2opaque.o2o.test3.M3.Enum
	1, // 6: net.proto2.go.open2opaque.o2o.test3.M3.opt_m:type_name -> net.proto2.go.open2opaque.o2o.test3.M3
	0, // 7: net.proto2.go.open2opaque.o2o.test3.M3.opt_e:type_name -> net.proto2.go.open2opaque.o2o.test3.M3.Enum
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_proto3test_proto_init() }
func file_proto3test_proto_init() {
	if File_proto3test_proto != nil {
		return
	}
	file_proto3test_proto_msgTypes[0].OneofWrappers = []any{
		(*M3_StringOneof)(nil),
		(*M3_IntOneof)(nil),
		(*M3_MsgOneof)(nil),
		(*M3_EnumOneof)(nil),
		(*M3_BytesOneof)(nil),
		(*M3_Build)(nil),
		(*M3_ProtoMessage_)(nil),
		(*M3_Reset_)(nil),
		(*M3_String_)(nil),
		(*M3_Descriptor_)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto3test_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto3test_proto_goTypes,
		DependencyIndexes: file_proto3test_proto_depIdxs,
		EnumInfos:         file_proto3test_proto_enumTypes,
		MessageInfos:      file_proto3test_proto_msgTypes,
	}.Build()
	File_proto3test_proto = out.File
	file_proto3test_proto_rawDesc = nil
	file_proto3test_proto_goTypes = nil
	file_proto3test_proto_depIdxs = nil
}
