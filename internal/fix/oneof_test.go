// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestOneof(t *testing.T) {
	tests := []test{{
		desc:     "clear",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
m2.OneofField = nil
`,
		want: map[Level]string{
			Green: `
m2.ClearOneofField()
`,
		},
	}, {
		desc:     "multiassign",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `var ignored string`,
		in: `
m2.OneofField, ignored = &pb2.M2_StringOneof{StringOneof: "hello"}, ""
`,
		want: map[Level]string{
			Green: `
m2.OneofField, ignored = &pb2.M2_StringOneof{StringOneof: "hello"}, ""
`,
		},
	}, {
		desc:     "ignore variable oneof",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
oneofField := m2.OneofField
m2.OneofField = oneofField
`,
		want: map[Level]string{
			Green: `
oneofField := m2.OneofField
m2.OneofField = oneofField
`,
			Red: `
// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).
oneofField := m2.OneofField
// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).
m2.OneofField = oneofField
`,
		},
	}, {
		desc:     "has",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
_ = m2.OneofField != nil
_ = m2.OneofField == nil
_ = m3.OneofField != nil
_ = m3.OneofField == nil
if m2.OneofField != nil {
}
if m2.OneofField == nil {
}
if m3.OneofField != nil {
}
if m3.OneofField == nil {
}
if o := m2.OneofField; o != nil {
}
if o := m2.OneofField; o == nil {
}
if o := m3.OneofField; o != nil {
}
if o := m3.OneofField; o == nil {
}
`,
		want: map[Level]string{
			Red: `
_ = m2.HasOneofField()
_ = !m2.HasOneofField()
_ = m3.HasOneofField()
_ = !m3.HasOneofField()
if m2.HasOneofField() {
}
if !m2.HasOneofField() {
}
if m3.HasOneofField() {
}
if !m3.HasOneofField() {
}
if m2.HasOneofField() {
}
if !m2.HasOneofField() {
}
if m3.HasOneofField() {
}
if !m3.HasOneofField() {
}
`,
		},
	}, {
		desc:     "proto3 works", // Basic smoke test; proto3 is handled together with proto2.
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
m3.OneofField = &pb3.M3_StringOneof{StringOneof: "hello"}
m3.OneofField = &pb3.M3_EnumOneof{EnumOneof: pb3.M3_E_VAL}
m3.OneofField = &pb3.M3_MsgOneof{MsgOneof: &pb3.M3{}}
m3.OneofField = nil
_ = m3.OneofField != nil
`,
		want: map[Level]string{
			Green: `
m3.SetStringOneof("hello")
m3.SetEnumOneof(pb3.M3_E_VAL)
m3.SetMsgOneof(&pb3.M3{})
m3.ClearOneofField()
_ = m3.HasOneofField()
`,
		},
	}, {
		desc:     "build naming",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
m3.OneofField = &pb3.M3_Build{Build: 1}
m3.OneofField = &pb3.M3_Build{1}
m3.OneofField = &pb3.M3_Build{}
`,
		want: map[Level]string{
			Green: `
m3.SetBuild_(1)
m3.SetBuild_(1)
m3.SetBuild_(0)
`,
		},
	}, {
		desc:     "oneof: simple type switch, field",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch m2.OneofField.(type) {
case *pb2.M2_StringOneof,
  *pb2.M2_IntOneof:
case *pb2.M2_MsgOneof:
case *pb2.M2_EnumOneof:
case *pb2.M2_BytesOneof:
case nil:
default:
}

switch m3.OneofField.(type) {
case *pb3.M3_StringOneof, *pb3.M3_IntOneof:
case *pb3.M3_MsgOneof:
case *pb3.M3_EnumOneof:
case *pb3.M3_BytesOneof, nil:
default:
}
`,
		want: map[Level]string{
			Green: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case,
	pb2.M2_IntOneof_case:
case pb2.M2_MsgOneof_case:
case pb2.M2_EnumOneof_case:
case pb2.M2_BytesOneof_case:
case 0:
default:
}

switch m3.WhichOneofField() {
case pb3.M3_StringOneof_case, pb3.M3_IntOneof_case:
case pb3.M3_MsgOneof_case:
case pb3.M3_EnumOneof_case:
case pb3.M3_BytesOneof_case, 0:
default:
}
`,
		},
	}, {
		desc:     "oneof: type switch with naming conflicts",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch m3.OneofField.(type) {
case *pb3.M3_String_:
case *pb3.M3_Reset_:
case *pb3.M3_ProtoMessage_:
case *pb3.M3_Descriptor_:
default:
}
`,
		want: map[Level]string{
			Green: `
switch m3.WhichOneofField() {
case pb3.M3_String__case:
case pb3.M3_Reset__case:
case pb3.M3_ProtoMessage__case:
case pb3.M3_Descriptor__case:
default:
}
`,
		},
	}, {
		desc:     "oneof: type switch with in-file naming conflicts",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
co := &pb2.ConflictingOneof{}
switch co.Included.(type) {
case *pb2.ConflictingOneof_Sub_:
default:
}
`,
		want: map[Level]string{
			Green: `
co := &pb2.ConflictingOneof{}
switch co.WhichIncluded() {
case pb2.ConflictingOneof_Sub_case:
default:
}
`,
		},
	}, {
		desc:     "oneof: type switch with in-file naming conflicts in nested sub-message",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
co := &pb2.ConflictingOneof_DeepSub{}
switch co.DeeplyIncluded.(type) {
case *pb2.ConflictingOneof_DeepSub_Sub_:
default:
}
`,
		want: map[Level]string{
			Green: `
co := &pb2.ConflictingOneof_DeepSub{}
switch co.WhichDeeplyIncluded() {
case pb2.ConflictingOneof_DeepSub_Sub_case:
default:
}
`,
		},
	}, {
		desc:     "oneof: simple type switch, method",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch m2.GetOneofField().(type) {
case *pb2.M2_StringOneof, *pb2.M2_IntOneof:
case *pb2.M2_MsgOneof:
case *pb2.M2_EnumOneof:
case *pb2.M2_BytesOneof:
case nil:
default:
}

switch m3.GetOneofField().(type) {
case *pb3.M3_StringOneof, *pb3.M3_IntOneof:
case *pb3.M3_MsgOneof:
case *pb3.M3_EnumOneof:
case *pb3.M3_BytesOneof:
case nil:
default:
}
`,
		want: map[Level]string{
			Green: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case, pb2.M2_IntOneof_case:
case pb2.M2_MsgOneof_case:
case pb2.M2_EnumOneof_case:
case pb2.M2_BytesOneof_case:
case 0:
default:
}

switch m3.WhichOneofField() {
case pb3.M3_StringOneof_case, pb3.M3_IntOneof_case:
case pb3.M3_MsgOneof_case:
case pb3.M3_EnumOneof_case:
case pb3.M3_BytesOneof_case:
case 0:
default:
}
`,
		},
	}, {
		desc:     "oneof: type switch decorations",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch /* hello */ m2.OneofField.(type) /* world */ { // !
case /* A */ *pb2.M2_StringOneof /* B */ : /* C */
}

switch /* hello */ m2.GetOneofField().(type) /* world */ { // !
case /* A */ *pb2.M2_StringOneof /* B */ : /* C */
}
`,
		want: map[Level]string{
			Green: `
switch /* hello */ m2.WhichOneofField() /* world */ { // !
case /* A */ pb2.M2_StringOneof_case /* B */ : /* C */
}

switch /* hello */ m2.WhichOneofField() /* world */ { // !
case /* A */ pb2.M2_StringOneof_case /* B */ : /* C */
}
`,
		},
	}, {
		desc:     "oneof: default non printf like func",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func randomFunc(*pb2.M2, ...interface{}) { }`,
		in: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
default:
	randomFunc(m2, oneofField)
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
default:
	randomFunc(m2, oneofField)
}
`,
			Red: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
default:
	randomFunc(m2, oneofField)
}
`,
		},
	}, {
		desc:     "oneof: default with T verb",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func fmtErrorf(format string, a ...interface{}) { }`,
		in: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
default:
	fmtErrorf("%T", oneofField)
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.WhichOneofField(); oneofField {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
default:
	fmtErrorf("%v", oneofField)
}
`,
			Red: `
switch oneofField := m2.WhichOneofField(); oneofField {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
default:
	fmtErrorf("%v", oneofField)
}
`,
		},
	}, {
		desc:     "oneof: default with multiple verbs",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func fmtErrorf(format string, a ...interface{}) { }`,
		in: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
default:
	fmtErrorf("%v %T", m2, oneofField)
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.WhichOneofField(); oneofField {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
default:
	fmtErrorf("%v %v", m2, oneofField)
}
`,
			Red: `
switch oneofField := m2.WhichOneofField(); oneofField {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
default:
	fmtErrorf("%v %v", m2, oneofField)
}
`,
		},
	}, {
		desc:     "oneof: with assignment, field access",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func fmtErrorf(format string, a ...interface{}) { }`,
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
default:
	fmtErrorf("%v", oneofField)
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.WhichOneofField(); oneofField {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
case pb2.M2_MsgOneof_case:
	_ = m2.GetMsgOneof().GetS()
default:
	fmtErrorf("%v", oneofField)
}
`,
		},
	}, {
		desc:     "oneof: with assignment, field access, proto3",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func fmtErrorf(format string, a ...interface{}) { }`,
		in: `
switch oneofField := m3.OneofField.(type) {
case *pb3.M3_MsgOneof:
	_ = oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch m3.WhichOneofField() {
case pb3.M3_MsgOneof_case:
	_ = m3.GetMsgOneof().GetS()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, getter access",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func fmtErrorf(format string, a ...interface{}) { }`,
		in: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
default:
	fmtErrorf("%v", oneofField)
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.WhichOneofField(); oneofField {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
case pb2.M2_MsgOneof_case:
	_ = m2.GetMsgOneof().GetS()
default:
	fmtErrorf("%v", oneofField)
}
`,
		},
	}, {
		desc:          "oneof: with assignment, getter access, in --types_to_update_file",
		srcfiles:      []string{"code.go", "pkg_test.go"},
		typesToUpdate: map[string]bool{"google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto.M2": true},
		extra:         `func fmtErrorf(format string, a ...interface{}) { }`,
		in: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch m2.WhichOneofField() {
case pb2.M2_MsgOneof_case:
	_ = m2.GetMsgOneof().GetS()
}
`,
		},
	}, {
		desc:          "oneof: with assignment, getter access, not in --types_to_update_file",
		srcfiles:      []string{"code.go", "pkg_test.go"},
		extra:         `func fmtErrorf(format string, a ...interface{}) { }`,
		typesToUpdate: map[string]bool{"google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto.M3": true},
		in: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		},
	}, {
		desc:     "oneof: with assignment, shadowing, non-oneof",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `type O struct {StringOneof int}`,
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	{
		oneofField := &O{}
		_ = oneofField.StringOneof
	}
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case:
	{
		oneofField := &O{}
		_ = oneofField.StringOneof
	}
	_ = m2.GetStringOneof()
case pb2.M2_MsgOneof_case:
	_ = m2.GetMsgOneof().GetS()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, shadowing, oneof, nop in green",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra: `
type O struct {
	StringOneof string ` + "`protobuf:\"oneof\"`" + `
}`,
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	{
		oneofField := &O{}
		_ = oneofField.StringOneof
	}
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	{
		oneofField := &O{}
		_ = oneofField.StringOneof
	}
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = oneofField.MsgOneof.GetS()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, shadowing, oneof, red is risky",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra: `
type O struct {
	StringOneof string ` + "`protobuf:\"oneof\"`" + `
}`,
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	{
		oneofField := &O{}
		_ = oneofField.StringOneof
	}
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Red: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case:
	{
		oneofField := &O{}
		_ = oneofField.StringOneof
	}
	_ = m2.GetStringOneof()
case pb2.M2_MsgOneof_case:
	_ = m2.GetMsgOneof().GetS()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, field access, selector",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.M.OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch m2.GetM().WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = m2.GetM().GetStringOneof()
case pb2.M2_MsgOneof_case:
	_ = m2.GetM().GetMsgOneof().GetS()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, field access, call",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.GetM().OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch m2.GetM().WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = m2.GetM().GetStringOneof()
case pb2.M2_MsgOneof_case:
	_ = m2.GetM().GetMsgOneof().GetS()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, field access, other expr; red marker",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `var msgs []*pb2.M2`,
		in: `
switch oneofField := msgs[0].OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
}
`,
		want: map[Level]string{
			Red: `
switch msgs[0].WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = msgs[0].GetStringOneof()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, field access, other expr; green nop",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `var msgs []*pb2.M2`,
		in: `
switch oneofField := msgs[0].OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
}
`,
		want: map[Level]string{
			Green: `
switch msgs[0].WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = msgs[0].GetStringOneof()
}
`,
		},
	}, {
		desc:     "oneof: with assignment, method call",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_MsgOneof:
	_ = *oneofField.MsgOneof.S
}
`,
		want: map[Level]string{
			Green: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = m2.GetStringOneof()
case pb2.M2_MsgOneof_case:
	_ = m2.GetMsgOneof().GetS()
}
`,
		},
	}, {
		desc:     "oneof: with assignment; avoid extra side effects",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func f() *pb2.M2 { return nil }`,
		in: `
switch oneofField := f().M.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Green: `
switch xmsg := f().GetM(); xmsg.WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = xmsg.GetStringOneof()
case pb2.M2_IntOneof_case:
	_ = xmsg.GetIntOneof()
}
`,
		},
	}, {
		desc:     "oneof: with assignment; avoid extra side effects; call",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func f() *pb2.M2 { return nil }`,
		in: `
switch oneofField := f().GetM().GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Green: `
switch xmsg := f().GetM(); xmsg.WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = xmsg.GetStringOneof()
case pb2.M2_IntOneof_case:
	_ = xmsg.GetIntOneof()
}
`,
		},
	}, {
		desc:     "oneof: with assignment; avoid extra side effects; f",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func f() *pb2.M2 { return nil }`,
		in: `
switch oneofField := f().GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Green: `
switch xmsg := f(); xmsg.WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = xmsg.GetStringOneof()
case pb2.M2_IntOneof_case:
	_ = xmsg.GetIntOneof()
}
`,
		},
	}, {
		desc:     "oneof: with assignment; don't override the init",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func f() *pb2.M2 { return nil }`,
		in: `
switch x := 1; oneofField := f().M.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
	_ = x
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Red: `
switch x := 1; oneofField := f().GetM().GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
	_ = x
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
} /* DO_NOT_SUBMIT: missing rewrite for type switch with side effects and init statement */
`,
		},
	}, {
		desc:     "oneof: with assignment; don't override the init; green nop",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func f() *pb2.M2 { return nil }`,
		in: `
switch x := 1; oneofField := f().M.GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
	_ = x
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Green: `
switch x := 1; oneofField := f().GetM().GetOneofField().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
	_ = x
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		},
	}, {
		desc:     "oneof: with assignment; avoid extra side effects; no oneof",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func f() interface{} { return nil }`,
		in: `
switch oneofField := f().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
	`,
		want: map[Level]string{
			Red: `
switch oneofField := f().(type) {
case *pb2.M2_StringOneof:
	_ = oneofField.StringOneof
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		},
	}, {
		desc:     "oneof: with assignment; don't rewrite unrelated statement",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
default:
	var err error
	_ = err.Error()
}
`,
		want: map[Level]string{
			Red: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = oneofField
case pb2.M2_IntOneof_case:
	_ = m2.GetIntOneof()
default:
	var err error
	_ = err.Error()
}
`,
		},
	}, {
		desc:     "oneof: with assignment; oneof refs in case",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		},
	}, {
		desc:     "oneof: with assignment; oneof refs in default",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.OneofField.(type) {
default:
	_ = oneofField
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.OneofField.(type) {
default:
	_ = oneofField
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		},
	}, {
		desc:     "oneof: with assignment; oneof refs in case; red",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_StringOneof:
	_ = oneofField
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
}
`,
		want: map[Level]string{
			Red: `
switch m2.WhichOneofField() {
case pb2.M2_StringOneof_case:
	_ = oneofField
case pb2.M2_IntOneof_case:
	_ = m2.GetIntOneof()
}
`,
		},
	}, {
		desc:     "oneof: with assignment; oneof refs in default; red",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_IntOneof:
	_ = oneofField.IntOneof
default:
	_ = oneofField
}
`,
		want: map[Level]string{
			Red: `
switch m2.WhichOneofField() {
case pb2.M2_IntOneof_case:
	_ = m2.GetIntOneof()
default:
	_ = oneofField
}
`,
		},
	}, {
		desc:     "oneof: with assignment; default %v",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `func fmtErrorf(format string, a ...interface{}) { }`,
		in: `
switch oneofField := m2.OneofField.(type) {
default:
	fmtErrorf("bad oneof %v", oneofField)
	fmtErrorf("bad oneof %v", m2.GetOneofField())
	fmtErrorf("bad oneof %v", m2.OneofField)
}
		`,
		want: map[Level]string{
			Green: `
switch oneofField := m2.WhichOneofField(); oneofField {
default:
	fmtErrorf("bad oneof %v", oneofField)
	fmtErrorf("bad oneof %v", m2.WhichOneofField())
	fmtErrorf("bad oneof %v", m2.WhichOneofField())
}
`,
		},
	}}

	runTableTests(t, tests)
}

func TestOneofBuilder(t *testing.T) {
	tests := []test{
		{
			desc:     "bytes builder",
			srcfiles: []string{"pkg_test.go"},
			extra:    `var bytes []byte`,
			in: `
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{BytesOneof: []byte("hello")}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{BytesOneof: []byte{}}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{BytesOneof: bytes}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{[]byte("hello")}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{bytes}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{nil}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{BytesOneof: nil}}
`,
			want: map[Level]string{
				Green: `
_ = pb2.M2_builder{BytesOneof: []byte("hello")}.Build()
_ = pb2.M2_builder{BytesOneof: []byte{}}.Build()
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{BytesOneof: bytes}}
_ = pb2.M2_builder{BytesOneof: []byte("hello")}.Build()
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{bytes}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{nil}}
_ = &pb2.M2{OneofField: &pb2.M2_BytesOneof{BytesOneof: nil}}
`,
				Red: `
_ = pb2.M2_builder{BytesOneof: []byte("hello")}.Build()
_ = pb2.M2_builder{BytesOneof: []byte{}}.Build()
_ = pb2.M2_builder{BytesOneof: proto.ValueOrDefaultBytes(bytes)}.Build()
_ = pb2.M2_builder{BytesOneof: []byte("hello")}.Build()
_ = pb2.M2_builder{BytesOneof: proto.ValueOrDefaultBytes(bytes)}.Build()
_ = pb2.M2_builder{BytesOneof: []byte{}}.Build()
_ = pb2.M2_builder{BytesOneof: []byte{}}.Build()
_ = pb2.M2_builder{BytesOneof: []byte{}}.Build()
`,
			},
		},
	}
	runTableTests(t, tests)
}

func TestOneofSetter(t *testing.T) {
	tests := []test{{
		desc:     "basic",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra: `
var str string
var num int64
`,
		in: `
m2.OneofField = &pb2.M2_StringOneof{StringOneof: "hello"}
m2.OneofField = &pb2.M2_StringOneof{StringOneof: str}
m2.OneofField = &pb2.M2_StringOneof{"hello"}
m2.OneofField = &pb2.M2_StringOneof{str}
m2.OneofField = &pb2.M2_StringOneof{}
m2.OneofField = &pb2.M2_IntOneof{IntOneof: 42}
m2.OneofField = &pb2.M2_IntOneof{IntOneof: num}
m2.OneofField = &pb2.M2_IntOneof{42}
m2.OneofField = &pb2.M2_IntOneof{num}
m2.OneofField = &pb2.M2_IntOneof{}
`,
		want: map[Level]string{
			Green: `
m2.SetStringOneof("hello")
m2.SetStringOneof(str)
m2.SetStringOneof("hello")
m2.SetStringOneof(str)
m2.SetStringOneof("")
m2.SetIntOneof(42)
m2.SetIntOneof(num)
m2.SetIntOneof(42)
m2.SetIntOneof(num)
m2.SetIntOneof(0)
`,
		},
	}, {
		desc:     "enum",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `var enum pb2.M2_Enum`,
		in: `
m2.OneofField = &pb2.M2_EnumOneof{EnumOneof: pb2.M2_E_VAL}
m2.OneofField = &pb2.M2_EnumOneof{EnumOneof: enum}
m2.OneofField = &pb2.M2_EnumOneof{enum}
m2.OneofField = &pb2.M2_EnumOneof{}
m2.OneofField = &pb2.M2_EnumOneof{EnumOneof: 0}
m2.OneofField = &pb2.M2_EnumOneof{0}
`,
		want: map[Level]string{
			Green: `
m2.SetEnumOneof(pb2.M2_E_VAL)
m2.SetEnumOneof(enum)
m2.SetEnumOneof(enum)
m2.SetEnumOneof(0)
m2.SetEnumOneof(0)
m2.SetEnumOneof(0)
`,
		},
	}, {
		desc:     "bytes",
		srcfiles: []string{"code.go", "pkg_test.go"},
		extra:    `var bytes []byte`,
		in: `
m2.OneofField = &pb2.M2_BytesOneof{BytesOneof: []byte("hello")}
m2.OneofField = &pb2.M2_BytesOneof{BytesOneof: bytes}
m2.OneofField = &pb2.M2_BytesOneof{[]byte("hello")}
m2.OneofField = &pb2.M2_BytesOneof{bytes}
m2.OneofField = &pb2.M2_BytesOneof{}
m2.OneofField = &pb2.M2_BytesOneof{nil}
m2.OneofField = &pb2.M2_BytesOneof{BytesOneof: nil}
`,
		want: map[Level]string{
			Green: `
m2.SetBytesOneof([]byte("hello"))
m2.OneofField = &pb2.M2_BytesOneof{BytesOneof: bytes}
m2.SetBytesOneof([]byte("hello"))
m2.OneofField = &pb2.M2_BytesOneof{bytes}
m2.OneofField = &pb2.M2_BytesOneof{}
m2.OneofField = &pb2.M2_BytesOneof{nil}
m2.OneofField = &pb2.M2_BytesOneof{BytesOneof: nil}
`,
			Red: `
m2.SetBytesOneof([]byte("hello"))
m2.SetBytesOneof(proto.ValueOrDefaultBytes(bytes))
m2.SetBytesOneof([]byte("hello"))
m2.SetBytesOneof(proto.ValueOrDefaultBytes(bytes))
m2.SetBytesOneof([]byte{})
m2.SetBytesOneof([]byte{})
m2.SetBytesOneof([]byte{})
`,
		},
	}, {
		desc:     "nontest: message clit empty",
		srcfiles: []string{"code.go"},
		in: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: &pb2.M2{}}
m2.OneofField = &pb2.M2_MsgOneof{&pb2.M2{}}
m2.OneofField = &pb2.M2_MsgOneof{}
m2.OneofField = &pb2.M2_MsgOneof{nil}
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: nil}
`,
		want: map[Level]string{
			Green: `
m2.SetMsgOneof(&pb2.M2{})
m2.SetMsgOneof(&pb2.M2{})
m2.OneofField = &pb2.M2_MsgOneof{}
m2.OneofField = &pb2.M2_MsgOneof{nil}
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: nil}
`,
		},
	}, {
		desc:     "nontest: message clit",
		srcfiles: []string{"code.go"},
		in: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: &pb2.M2{B: proto.Bool(true)}}
m2.OneofField = &pb2.M2_MsgOneof{&pb2.M2{B: proto.Bool(true)}}
`,
		want: map[Level]string{
			// We could detect that this is safe in theory but it
			// requires data flow analysis to prove that m2h2 and
			// m2h3 are never nil
			Green: `
m2h2 := &pb2.M2{}
m2h2.SetB(true)
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: m2h2}
m2h3 := &pb2.M2{}
m2h3.SetB(true)
m2.OneofField = &pb2.M2_MsgOneof{m2h3}
`,
			Yellow: `
m2h2 := &pb2.M2{}
m2h2.SetB(true)
m2.SetMsgOneof(proto.ValueOrDefault(m2h2))
m2h3 := &pb2.M2{}
m2h3.SetB(true)
m2.SetMsgOneof(proto.ValueOrDefault(m2h3))
`,
		},
	}, {
		desc:     "test: message clit",
		srcfiles: []string{"pkg_test.go"},
		in: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: &pb2.M2{B: proto.Bool(true)}}
m2.OneofField = &pb2.M2_MsgOneof{&pb2.M2{B: proto.Bool(true)}}
`,
		want: map[Level]string{
			Green: `
m2.SetMsgOneof(pb2.M2_builder{B: proto.Bool(true)}.Build())
m2.SetMsgOneof(pb2.M2_builder{B: proto.Bool(true)}.Build())
`,
		},
	}, {
		desc:     "test: message clit with decorations",
		srcfiles: []string{"pkg_test.go"},
		in: `
_ = &pb2.M2{
	OneofField: &pb2.M2_MsgOneof{
		// Comment above MsgOneof
		MsgOneof: &pb2.M2{
			// Comment above B
			B: proto.Bool(true), // end of B line
		}, // end of MsgOneof line
	},
}

_ = &pb2.M2{
	OneofField: &pb2.M2_MsgOneof{
		// Comment above MsgOneof
		&pb2.M2{
			// Comment above B
			B: proto.Bool(true), // end of B line
		}, // end of MsgOneof line
	},
}
`,
		want: map[Level]string{
			Green: `
_ = pb2.M2_builder{
	// Comment above MsgOneof
	MsgOneof: pb2.M2_builder{
		// Comment above B
		B: proto.Bool(true), // end of B line
	}.Build(), // end of MsgOneof line
}.Build()

_ = pb2.M2_builder{
	// Comment above MsgOneof
	MsgOneof: pb2.M2_builder{
		// Comment above B
		B: proto.Bool(true), // end of B line
	}.Build(), // end of MsgOneof line
}.Build()
`,
		},
	}, {
		desc:     "nontest: nested message clit with decorations",
		srcfiles: []string{"pkg.go"},
		in: `
_ = &pb2.M2{
	OneofField: &pb2.M2_MsgOneof{
		// Comment above MsgOneof
		MsgOneof: &pb2.M2{
			// Comment above B
			B: proto.Bool(true), // end of B line
		}, // end of MsgOneof line
	},
}
`,
		want: map[Level]string{
			// We could detect that this is safe in theory but it
			// requires data flow analysis to prove that m2h2 is
			// never nil
			Green: `
m2h2 := &pb2.M2{}
// Comment above B
m2h2.SetB(true) // end of B line
m2h3 := &pb2.M2{}
m2h3.OneofField = &pb2.M2_MsgOneof{
	// Comment above MsgOneof
	MsgOneof: m2h2, // end of MsgOneof line
}
_ = m2h3
`,
			Yellow: `
m2h2 := &pb2.M2{}
// Comment above B
m2h2.SetB(true) // end of B line
m2h3 := &pb2.M2{}
// Comment above MsgOneof
m2h3.SetMsgOneof(proto.ValueOrDefault(m2h2)) // end of MsgOneof line
_ = m2h3
`,
		},
	}, {
		desc:     "nontest message clit var",
		srcfiles: []string{"code.go"},
		extra:    `var msg *pb2.M2`,
		in: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: msg}
m2.OneofField = &pb2.M2_MsgOneof{msg}
`,
		want: map[Level]string{
			Green: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: msg}
m2.OneofField = &pb2.M2_MsgOneof{msg}
`,
			Yellow: `
m2.SetMsgOneof(proto.ValueOrDefault(msg))
m2.SetMsgOneof(proto.ValueOrDefault(msg))
`,
		},
	}, {
		desc:     "message clit var",
		srcfiles: []string{"pkg_test.go"},
		extra:    `var msg *pb2.M2`,
		in: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: msg}
m2.OneofField = &pb2.M2_MsgOneof{msg}
`,
		want: map[Level]string{
			Green: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: msg}
m2.OneofField = &pb2.M2_MsgOneof{msg}
`,
			Yellow: `
m2.SetMsgOneof(proto.ValueOrDefault(msg))
m2.SetMsgOneof(proto.ValueOrDefault(msg))
`,
		},
	}, {
		desc:     "message builder",
		srcfiles: []string{"code.go", "pkg_test.go"},
		in: `
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: pb2.M2_builder{B: proto.Bool(true)}.Build()}
m2.OneofField = &pb2.M2_MsgOneof{pb2.M2_builder{B: proto.Bool(true)}.Build()}
m2.OneofField = &pb2.M2_MsgOneof{MsgOneof: pb2.M2_builder{}.Build()}
m2.OneofField = &pb2.M2_MsgOneof{pb2.M2_builder{}.Build()}
`,
		want: map[Level]string{
			Green: `
m2.SetMsgOneof(pb2.M2_builder{B: proto.Bool(true)}.Build())
m2.SetMsgOneof(pb2.M2_builder{B: proto.Bool(true)}.Build())
m2.SetMsgOneof(pb2.M2_builder{}.Build())
m2.SetMsgOneof(pb2.M2_builder{}.Build())
`,
		},
	}, {
		desc:     "variable of wrapper type",
		srcfiles: []string{"code.go"},
		in: `
var scalarOneof *pb2.M2_StringOneof
_ = &pb2.M2{OneofField: scalarOneof}
`,
		want: map[Level]string{
			Green: `
var scalarOneof *pb2.M2_StringOneof
m2h2 := &pb2.M2{}
m2h2.OneofField = scalarOneof
_ = m2h2
`,
			Red: `
var scalarOneof *pb2.M2_StringOneof
m2h2 := &pb2.M2{}
m2h2.SetStringOneof(scalarOneof.StringOneof)
_ = m2h2
`,
		},
	}, {
		desc:     "oneof field",
		srcfiles: []string{"code.go"},
		in: `
_ = &pb2.M2{OneofField: m2.OneofField}
`,
		want: map[Level]string{
			Green: `
m2h2 := &pb2.M2{}
m2h2.OneofField = m2.OneofField
_ = m2h2
`,
			Red: `
m2h2 := &pb2.M2{}
// DO NOT SUBMIT: Migrate the direct oneof field access (go/go-opaque-special-cases/oneof.md).
m2h2.OneofField = m2.OneofField
_ = m2h2
`,
		},
	}}
	runTableTests(t, tests)
}

func TestMaybeUnsafeOneof(t *testing.T) {
	tests := []test{{
		desc:     "oneof message wrapper variable field",
		srcfiles: []string{"code.go"},
		in: `
var msgOneof *pb2.M2_MsgOneof
_ = &pb2.M2{OneofField: msgOneof}
`,
		want: map[Level]string{
			Green: `
var msgOneof *pb2.M2_MsgOneof
m2h2 := &pb2.M2{}
m2h2.OneofField = msgOneof
_ = m2h2
`,
			Yellow: `
var msgOneof *pb2.M2_MsgOneof
m2h2 := &pb2.M2{}
m2h2.OneofField = msgOneof
_ = m2h2
`,
			Red: `
var msgOneof *pb2.M2_MsgOneof
m2h2 := &pb2.M2{}
m2h2.SetMsgOneof(proto.ValueOrDefault(msgOneof.MsgOneof))
_ = m2h2
`,
		},
	},

		{
			desc:     "oneof message wrapper variable in builder",
			srcfiles: []string{"code_test.go"},
			in: `
var msgOneof *pb2.M2_MsgOneof
_ = &pb2.M2{OneofField: msgOneof}
`,
			want: map[Level]string{
				Green: `
var msgOneof *pb2.M2_MsgOneof
_ = &pb2.M2{OneofField: msgOneof}
`,
				Yellow: `
var msgOneof *pb2.M2_MsgOneof
_ = &pb2.M2{OneofField: msgOneof}
`,
				Red: `
var msgOneof *pb2.M2_MsgOneof
_ = pb2.M2_builder{MsgOneof: proto.ValueOrDefault(msgOneof.MsgOneof)}.Build()
`,
			},
		},

		{
			desc:     "oneof message field",
			srcfiles: []string{"code.go"},
			in: `
var msg *pb2.M2
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{msg}}
`,
			want: map[Level]string{
				Green: `
var msg *pb2.M2
m2h2 := &pb2.M2{}
m2h2.OneofField = &pb2.M2_MsgOneof{msg}
_ = m2h2
`,
				Yellow: `
var msg *pb2.M2
m2h2 := &pb2.M2{}
m2h2.SetMsgOneof(proto.ValueOrDefault(msg))
_ = m2h2
`,
			},
		},

		{
			desc:     "oneof message field in builder",
			srcfiles: []string{"code_test.go"},
			in: `
var msg *pb2.M2
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{msg}}
`,
			want: map[Level]string{
				Green: `
var msg *pb2.M2
_ = &pb2.M2{OneofField: &pb2.M2_MsgOneof{msg}}
`,
				Yellow: `
var msg *pb2.M2
_ = pb2.M2_builder{MsgOneof: proto.ValueOrDefault(msg)}.Build()
`,
			},
		},
	}
	runTableTests(t, tests)
}

func TestNestedSwitch(t *testing.T) {
	tests := []test{
		{
			desc:     "nested switch",
			extra:    `func fmtErrorf(format string, a ...interface{}) { }`,
			srcfiles: []string{"code_test.go"},
			in: `
switch oneofField := m2.OneofField.(type) {
case *pb2.M2_BytesOneof:
	_ = oneofField.BytesOneof
case *pb2.M2_MsgOneof:
	switch nestedOneof := oneofField.MsgOneof.OneofField.(type) {
	case *pb2.M2_StringOneof:
		_ = nestedOneof.StringOneof
	default:
		fmtErrorf("%v", oneofField)
	}
default:
	fmtErrorf("%v", oneofField)
}
`,
			want: map[Level]string{
				Red: `
switch oneofField := m2.WhichOneofField(); oneofField {
case pb2.M2_BytesOneof_case:
	_ = m2.GetBytesOneof()
case pb2.M2_MsgOneof_case:
	switch m2.GetMsgOneof().WhichOneofField() {
	case pb2.M2_StringOneof_case:
		_ = m2.GetMsgOneof().GetStringOneof()
	default:
		fmtErrorf("%v", oneofField)
	}
default:
	fmtErrorf("%v", oneofField)
}
`,
			},
		},
	}
	runTableTests(t, tests)
}
