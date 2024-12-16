// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

import pb2 "google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto"
import pb3 "google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto"
import proto "google.golang.org/protobuf/proto"
import "unsafe"
import "context"

var _ unsafe.Pointer
var _ = proto.String
var _ = context.Background

func test_function() {
	m2 := new(pb2.M2)
	m2a := new(pb2.M2)
	_, _ = m2, m2a
	m3 := new(pb3.M3)
	_ = m3
	_ = "TEST CODE STARTS HERE"

	_ = "TEST CODE ENDS HERE"
}
