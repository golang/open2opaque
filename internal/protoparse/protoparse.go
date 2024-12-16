// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package protoparse provides function to parse proto source files and identify the go_api_flag
// file and message options specified in the files.
package protoparse

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/open2opaque/internal/protodetect"
	"google.golang.org/protobuf/proto"

	pb "google.golang.org/open2opaque/internal/apiflagdata"
	descpb "google.golang.org/protobuf/types/descriptorpb"
	gofeaturespb "google.golang.org/protobuf/types/gofeaturespb"
)

// TextRange describes a location in a proto file. Please note that the column
// indices are code-point indices, not byte indices.
type TextRange struct {
	BeginLine int
	BeginCol  int
	EndLine   int
	EndCol    int
}

// SpanToTextRange converts a proto2.SourceCodeInfo.Location.span to a
// TextRange.
func SpanToTextRange(span []int32) TextRange {
	if len(span) < 3 && len(span) > 4 {
		panic(fmt.Sprintf("input %v isn't a proto2.SourceCodeInfo.Location.span", span))
	}
	if len(span) == 3 {
		// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L1209
		return TextRange{
			BeginLine: int(span[0]),
			BeginCol:  int(span[1]),
			EndLine:   int(span[0]),
			EndCol:    int(span[2]),
		}
	}

	return TextRange{
		BeginLine: int(span[0]),
		BeginCol:  int(span[1]),
		EndLine:   int(span[2]),
		EndCol:    int(span[3]),
	}
}

// ToByteRange converts line and column information to a byte range.
func (tr TextRange) ToByteRange(content []byte) (beginByte, endByte int, err error) {
	if tr.EndLine < tr.BeginLine {
		return -1, -1, fmt.Errorf("EndLine %d < BeginLine %d", tr.EndLine, tr.BeginLine)
	}
	if tr.EndLine == tr.BeginLine && tr.EndCol < tr.BeginCol {
		return -1, -1, fmt.Errorf("EndCol %d < BeginCol %d in the same line", tr.EndCol, tr.BeginCol)
	}
	{
		lines := bytes.Split(content, []byte{'\n'})[tr.BeginLine : tr.EndLine+1]
		if bytes.Contains(bytes.Join(lines, []byte{'\n'}), []byte{'\t'}) {
			// The parser deals with tabs in a complicated manner, see
			// https://github.com/bufbuild/protocompile/blob/c91057b816eb7f827dfa83ff5288b74ead9d4fe5/ast/file_info.go#L362-L364.
			// We currently don't support this.
			return -1, -1, fmt.Errorf("line range contains tabs")
		}
	}
	beginByte = -1
	endByte = -1
	from := 0
	newlineIdx := 0
	lineNumber := 0
	for newlineIdx >= 0 {
		newlineIdx := bytes.IndexByte(content[from:], '\n')
		to := from + newlineIdx + 1
		if newlineIdx == -1 {
			to = len(content) - 1
		}
		lineRunes := bytes.Runes(content[from:to])
		if lineNumber == tr.BeginLine {
			if tr.BeginCol > len(lineRunes) {
				return -1, -1, fmt.Errorf("BeginCol %d is out of range, line is %q", tr.BeginCol, string(lineRunes))
			}
			beginByte = from + len(string(lineRunes[:tr.BeginCol]))
		}
		if lineNumber == tr.EndLine {
			if tr.EndCol > len(lineRunes) {
				return -1, -1, fmt.Errorf("EndCol %d is out of range, line is %q", tr.EndCol, string(lineRunes))
			}
			endByte = from + len(string(lineRunes[:tr.EndCol]))
			break
		}

		from = to
		lineNumber++
	}
	if endByte == -1 {
		return -1, -1, fmt.Errorf("EndLine %d is out of range, number lines is %d", tr, lineNumber)
	}
	return beginByte, endByte, nil
}

// APIInfo contains information about an explicit API flag definition.
type APIInfo struct {
	TextRange         TextRange
	HasLeadingComment bool
	path              []int32
}

// FileOpt contains the Go API level info for a file along with other proto
// info.
type FileOpt struct {
	// File name containing relative path
	File string
	// Proto package name.
	Package string
	// Go API level. This can be an implicit value via default.
	GoAPI gofeaturespb.GoFeatures_APILevel
	// Whether go_api_flag option is explicitly set in proto file or not.
	IsExplicit bool
	// APIInfo is nil if IsExplicit is false.
	APIInfo *APIInfo
	// Options of messages defined at the file level. Nested messages are stored
	// as their children.
	MessageOpts []*MessageOpt
	// SourceCodeInfo is set if parsed results includes it.
	SourceCodeInfo *descpb.SourceCodeInfo
	// Proto syntax: "proto2", "proto3", "editions", or "editions_go_api_flag".
	// The latter is set for editions protos that use the old go_api_flag
	// explicitly.
	Syntax string
}

// MessageOpt contains the Go API level info for a message.
type MessageOpt struct {
	// Proto message name. Includes parent name if nested, e.g. A.B for message
	// B that is defined in body of A.
	Message string
	// Go API level. This can be an implicit value via file option or in case of
	// editions features via the parent message.
	GoAPI gofeaturespb.GoFeatures_APILevel
	// Whether go_api_flag option is explicitly set in proto message or not.
	IsExplicit bool
	// APIInfo is nil if IsExplicit is false.
	APIInfo *APIInfo
	// FileDescriptorProto.source_code_info.location.path of this message:
	// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L1202
	// Example: The 1st nested message of the 6th message in the file is in path
	// [4, 5, 3, 0]; 4 is the field number of FileDescriptorProto.message_type, 5
	// is the index for the 6th message, 3 is DescriptorProto.nested_type, 0 is
	// the index for the first nested message.
	LocPath []int32
	// Options of the parent message. If this is e.g. the message B which is
	// defined in the body of message A, then A is the parent. Parent is nil for
	// messages defined at the file level, i.e. non-nested messages.
	Parent *MessageOpt
	// Options of the child messages. If this is e.g. message A and messages
	// B and C are defined in the body of message A, then B and C are the
	// children.
	Children []*MessageOpt
}

// Parser parses proto source files for go_api_flag values.
type Parser struct {
	parser protoparse.Parser
}

// NewParser constructs a Parser with default file accessor.
func NewParser() *Parser {
	return &Parser{protoparse.Parser{
		InterpretOptionsInUnlinkedFiles: true,
		IncludeSourceCodeInfo:           true,
	}}
}

// NewParserWithAccessor constructs a Parser with a custom file accessor.
func NewParserWithAccessor(acc protoparse.FileAccessor) *Parser {
	return &Parser{protoparse.Parser{
		InterpretOptionsInUnlinkedFiles: true,
		IncludeSourceCodeInfo:           true,
		Accessor:                        acc,
	}}
}

func fromOldToFeature(apiLevel pb.GoAPI) (gofeaturespb.GoFeatures_APILevel, error) {
	switch apiLevel {
	case pb.GoAPI_OPEN_V1:
		return gofeaturespb.GoFeatures_API_OPEN, nil

	case pb.GoAPI_OPEN_TO_OPAQUE_HYBRID:
		return gofeaturespb.GoFeatures_API_HYBRID, nil

	case pb.GoAPI_OPAQUE_V0:
		return gofeaturespb.GoFeatures_API_OPAQUE, nil

	default:
		return gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED, fmt.Errorf("unknown apilevel %v", apiLevel)
	}
}

func uninterpretedGoAPIFeature(opts []*descpb.UninterpretedOption) (gofeaturespb.GoFeatures_APILevel, int) {
	for i, opt := range opts {
		nameParts := opt.GetName()
		if len(nameParts) != 3 ||
			nameParts[0].GetNamePart() != "features" ||
			nameParts[1].GetNamePart() != "pb.go" ||
			nameParts[2].GetNamePart() != "api_level" {
			continue
		}
		v := string(opt.GetIdentifierValue())
		switch v {
		case "API_OPEN":
			return gofeaturespb.GoFeatures_API_OPEN, i
		case "API_HYBRID":
			return gofeaturespb.GoFeatures_API_HYBRID, i
		case "API_OPAQUE":
			return gofeaturespb.GoFeatures_API_OPAQUE, i
		default:
			panic(fmt.Sprintf("unknown features.(pb.go).api_level value %v", v))
		}
	}
	return gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED, -1
}

func fileGoAPIEditions(desc *descpb.FileDescriptorProto) (gofeaturespb.GoFeatures_APILevel, bool, []int32, error) {
	if proto.HasExtension(desc.GetOptions().GetFeatures(), gofeaturespb.E_Go) {
		panic("unimplemented: Go extension features are fully parsed in file options")
	}
	api, idx := uninterpretedGoAPIFeature(desc.GetOptions().GetUninterpretedOption())
	if api == gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED {
		return protodetect.DefaultFileLevel(desc.GetName()), false, nil, nil
	}
	const (
		// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L122
		fileOptionsField = 8
		// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L553
		uninterpretedOptionField = 999
	)
	return api, true, []int32{fileOptionsField, uninterpretedOptionField, int32(idx)}, nil
}

func traverseMsgOpts(opt *MessageOpt, f func(*MessageOpt)) {
	f(opt)
	for _, c := range opt.Children {
		traverseMsgOpts(c, f)
	}
}

// ParseFile reads the given proto source file name
// and determines the API level. If skipMessages is set to
// true, return value will have nil MessageOpts field.
func (p *Parser) ParseFile(name string, skipMessages bool) (*FileOpt, error) {
	descs, err := p.parser.ParseFilesButDoNotLink(name)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", name, err)
	}
	desc := descs[0]

	var fileAPI gofeaturespb.GoFeatures_APILevel
	var explicit bool
	var sciPath []int32

	syntax := desc.GetSyntax()
	fileAPI, explicit, sciPath, err = fileGoAPIEditions(desc)
	if err != nil {
		return nil, fmt.Errorf("fileGoAPIEditions: %v", err)
	}

	var mopts []*MessageOpt
	if !skipMessages {
		const (
			// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L117
			descriptorProtoField = 4
		)
		for i, m := range desc.GetMessageType() {
			var mopt *MessageOpt
			mopt = readMessagesEditions(m, fileAPI, "", []int32{descriptorProtoField, int32(i)})
			mopts = append(mopts, mopt)
		}
	}
	var info *APIInfo
	var allAPIInfos []*APIInfo
	if explicit {
		info = &APIInfo{path: sciPath}
		allAPIInfos = append(allAPIInfos, info)
	}

	for _, mLoop := range mopts {
		traverseMsgOpts(mLoop, func(m *MessageOpt) {
			allAPIInfos = append(allAPIInfos, m.APIInfo)
		})
	}
	// The APIInfos only contain SourceCodeInfo paths so far. Now, fill in the
	// line and column information by directly modifying the pointee text ranges
	// in allTextRanges.
	fillAPIInfos(allAPIInfos, desc.GetSourceCodeInfo())

	return &FileOpt{
		File:           name,
		Package:        desc.GetPackage(),
		GoAPI:          fileAPI,
		IsExplicit:     explicit,
		APIInfo:        info,
		MessageOpts:    mopts,
		SourceCodeInfo: desc.GetSourceCodeInfo(),
		Syntax:         syntax,
	}, nil
}

func fillAPIInfos(infos []*APIInfo, info *descpb.SourceCodeInfo) {
	m := make(map[string]*APIInfo)
	for _, info := range infos {
		if info != nil {
			m[fmt.Sprint(info.path)] = info
		}
	}
	for _, loc := range info.GetLocation() {
		if info, ok := m[fmt.Sprint(loc.GetPath())]; ok {
			info.TextRange = SpanToTextRange(loc.GetSpan())
			leading := strings.TrimSpace(loc.GetLeadingComments())
			switch {
			default:
				info.HasLeadingComment = leading != ""
			}
		}
	}
}

func readMessagesEditions(m *descpb.DescriptorProto, parentAPI gofeaturespb.GoFeatures_APILevel, namePrefix string, msgPath []int32) *MessageOpt {
	if m.GetOptions().GetMapEntry() {
		// Map-entry messages are auto-generated and their Go API level cannot
		// be adjusted in the proto file.
		return nil
	}
	name := m.GetName()
	if namePrefix != "" {
		name = namePrefix + "." + m.GetName()
	}

	// If not set, default to parent value. This is the file API for a message
	// at the file level or the API of the parent message for a nested message.
	msgAPI := parentAPI
	var info *APIInfo
	var isSet bool
	if api, idx := uninterpretedGoAPIFeature(m.GetOptions().GetUninterpretedOption()); api != gofeaturespb.GoFeatures_API_LEVEL_UNSPECIFIED {
		msgAPI = api
		isSet = true
		const (
			// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L160
			messageOptionsField = 7
			// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L638
			uninterpretedOptionField = 999
		)
		info = &APIInfo{path: append(slices.Clone(msgPath), messageOptionsField, uninterpretedOptionField, int32(idx))}
	}

	mopt := &MessageOpt{
		Message:    name,
		GoAPI:      msgAPI,
		IsExplicit: isSet,
		APIInfo:    info,
		LocPath:    slices.Clone(msgPath),
	}

	for i, n := range m.GetNestedType() {
		const (
			// https://github.com/protocolbuffers/protobuf/blob/v29.1/src/google/protobuf/descriptor.proto#L147
			nestedDescriptorProtoField = 3
		)
		// Pass msgAPI as parent API: edition features are inherited by nested messages.
		nopt := readMessagesEditions(n, msgAPI, name, append(slices.Clone(msgPath), nestedDescriptorProtoField, int32(i)))
		if nopt == nil {
			continue
		}
		mopt.Children = append(mopt.Children, nopt)
		nopt.Parent = mopt
	}
	return mopt
}
