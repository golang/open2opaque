// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package protodetect provides function to identify the go_api_flag
// file and message options specified in the files.
package protodetect

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"

	pb "google.golang.org/open2opaque/internal/apiflagdata"
	descpb "google.golang.org/protobuf/types/descriptorpb"
	gofeaturespb "google.golang.org/protobuf/types/gofeaturespb"
	pluginpb "google.golang.org/protobuf/types/pluginpb"
)

func fromFeatureToOld(apiLevel gofeaturespb.GoFeatures_APILevel) pb.GoAPI {
	switch apiLevel {
	case gofeaturespb.GoFeatures_API_OPEN:
		return pb.GoAPI_OPEN_V1

	case gofeaturespb.GoFeatures_API_HYBRID:
		return pb.GoAPI_OPEN_TO_OPAQUE_HYBRID

	case gofeaturespb.GoFeatures_API_OPAQUE:
		return pb.GoAPI_OPAQUE_V0

	default:
		panic(fmt.Sprintf("unknown apilevel %v", apiLevel))
	}
}

func apiLevelForDescriptor(fd *descpb.FileDescriptorProto) gofeaturespb.GoFeatures_APILevel {
	return apiLevelForDescriptorOpts(protogen.Options{}, fd)
}

func apiLevelForDescriptorOpts(opts protogen.Options, fd *descpb.FileDescriptorProto) gofeaturespb.GoFeatures_APILevel {
	fopts := fd.Options
	if fopts == nil {
		fopts = &descpb.FileOptions{}
		fd.Options = fopts
	}
	fopts.GoPackage = proto.String("dummy/package")

	// Determine the API level by querying protogen using a stub plugin request.
	plugin, err := opts.New(&pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descpb.FileDescriptorProto{fd},
	})
	if err != nil {
		panic(err)
	}
	if got, want := len(plugin.Files), 1; got != want {
		panic(fmt.Sprintf("protogen returned %d plugin.Files entries, expected %d", got, want))
	}
	return plugin.Files[0].APILevel
}

// DefaultFileLevel returns the default go_api_flag option for given path.
func DefaultFileLevel(path string) gofeaturespb.GoFeatures_APILevel {
	if path == "testonly-opaque-default-dummy.proto" {
		// Magic filename to test the edition 2024+ behavior (Opaque API by
		// default) from setapi_test.go
		return gofeaturespb.GoFeatures_API_OPAQUE
	}

	fd := &descpb.FileDescriptorProto{Name: proto.String(path)}
	return apiLevelForDescriptor(fd)
}

// MapGoAPIFlag maps current and old values of the go API flag to current values.
func MapGoAPIFlag(val string) (pb.GoAPI, error) {
	// Old go_api_flag values are still used here in order to parse proto files from
	// older /google_src/files/... paths.
	// LINT.IfChange(go_api_flag_parsing)
	switch val {
	case "OPEN_V1":
		return pb.GoAPI_OPEN_V1, nil
	case "OPEN_TO_OPAQUE_HYBRID":
		return pb.GoAPI_OPEN_TO_OPAQUE_HYBRID, nil
	case "OPAQUE_V0":
		return pb.GoAPI_OPAQUE_V0, nil
	default:
		return pb.GoAPI_INVALID, fmt.Errorf("invalid value: %q", val)
	}
	// LINT.ThenChange()
}
