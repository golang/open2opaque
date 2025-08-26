// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type jsonPackage struct {
	ImportPath string
	Dir        string
	Name       string
	Export     string
	GoFiles    []string

	Error struct {
		Err string
	}
}

func goListBase() (*fakeLoaderBase, error) {
	goList := exec.Command("go", "list", "-e",
		"-json=ImportPath,Error,Dir,GoFiles,Export",
		"-export=true",
		"-deps=true",
		// https://cs.opensource.google/go/x/tools/+/master:go/packages/golist.go;l=818-824;drc=977f6f71501a7b7b9b35d6125bf740401be8ce29
		"-pgo=off",
		"google.golang.org/open2opaque/internal/fix/testdata/fake")
	goList.Stderr = os.Stderr
	stdout, err := goList.Output()
	if err != nil {
		return nil, err
	}

	// Filter the packages for which we store data and load files. This reduces
	// memory usage and makes the program easier to debug.
	interestingPackages := map[string]bool{
		// Imported by testdata/fake/fake.go:
		"google.golang.org/open2opaque/internal/fix/testdata/proto2test_go_proto": true,
		"google.golang.org/open2opaque/internal/fix/testdata/proto3test_go_proto": true,
		"google.golang.org/protobuf/proto":                                        true,
		"google.golang.org/protobuf/types/gofeaturespb":                           true,
		"google.golang.org/protobuf/types/known/emptypb":                          true,
		// Imported by .pb.go files:
		"google.golang.org/protobuf/reflect/protoreflect": true,
		"google.golang.org/protobuf/runtime/protoimpl":    true,
	}

	importPathToFiles := make(map[string][]string)
	importPathToExport := make(map[string][]byte)
	for dec := json.NewDecoder(bytes.NewBuffer(stdout)); dec.More(); {
		p := new(jsonPackage)
		if err := dec.Decode(p); err != nil {
			return nil, fmt.Errorf("JSON decoding failed: %v", err)
		}
		if !interestingPackages[p.ImportPath] {
			continue
		}
		goFiles := make([]string, len(p.GoFiles))
		for idx, fn := range p.GoFiles {
			goFiles[idx] = filepath.Join(p.Dir, fn)
		}
		importPathToFiles[p.ImportPath] = goFiles
		b, err := os.ReadFile(p.Export)
		if err != nil {
			return nil, err
		}
		importPathToExport[p.ImportPath] = b
	}

	pathToContent := make(map[string]string)
	for _, goFiles := range importPathToFiles {
		for _, fn := range goFiles {
			b, err := os.ReadFile(fn)
			if err != nil {
				return nil, err
			}
			pathToContent[fn] = string(b)
		}
	}

	return &fakeLoaderBase{
		ImportPathToFiles: importPathToFiles,
		PathToContent:     pathToContent,
		ExportFor: func(importPath string) []byte {
			return importPathToExport[importPath]
		},
	}, nil
}
