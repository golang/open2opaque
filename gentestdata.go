// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"log"
	"os"

	"google.golang.org/open2opaque/internal/fix"
)

func genTestdataFake() error {
	log.Printf("generating testdata/fake")
	content := fix.NewSrc("", "")
	if err := os.MkdirAll("internal/fix/testdata/fake", 0777); err != nil {
		return err
	}
	if err := os.WriteFile("internal/fix/testdata/fake/fake.go", []byte(content), 0666); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := genTestdataFake(); err != nil {
		log.Fatal(err)
	}
}
