// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func genProtos() error {
	for _, pb := range []struct {
		Dir      string
		Basename string
		Package  string
	}{
		{
			Dir:      "internal/apiflagdata",
			Basename: "go_api_enum.proto",
			Package:  "google.golang.org/open2opaque/internal/apiflagdata",
		},

		{
			Dir:      "internal/dashboard",
			Basename: "stats.proto",
			Package:  "google.golang.org/open2opaque/internal/dashboard",
		},

		{
			Dir:      "internal/fix/testdata/proto2test_go_proto",
			Basename: "proto2test.proto",
			Package:  "google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto",
		},

		{
			Dir:      "internal/fix/testdata/proto3test_go_proto",
			Basename: "proto3test.proto",
			Package:  "google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto",
		},
	} {
		log.Printf("protoc %s/%s", pb.Dir, pb.Basename)
		protoc := exec.Command("protoc",
			"-I=.",
			"--go_out=.",
			"--go_opt=M"+pb.Basename+"="+pb.Package,
			"--go_opt=paths=source_relative")
		if strings.HasSuffix(pb.Dir, "/proto3test_go_proto") {
			protoc.Args = append(protoc.Args, "--go_opt=default_api_level=API_HYBRID")
		}
		protoc.Args = append(protoc.Args, pb.Basename)
		protoc.Dir = pb.Dir
		protoc.Stderr = os.Stderr
		if err := protoc.Run(); err != nil {
			return fmt.Errorf("%v: %v", protoc.Args, err)
		}
	}
	return nil
}

func main() {
	if err := genProtos(); err != nil {
		log.Fatal(err)
	}
}
