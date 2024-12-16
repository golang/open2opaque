// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errutil provides utilities for easily annotating Go errors.
package errutil

import "fmt"

// Annotatef annotates non-nil error with the given message.
//
// It's designed to be used in a defer, for example:
//
//	func g(arg string) (err error) {
//	   defer Annotate(&err, fmt.Sprintf("g(%s)")
//	   return errors.New("my error")
//	}
//
// Calling g("hello") will result in error message:
//
//	g(hello): my error
//
// Annotate allows using the above short form instead of the long form:
//
//	func g(arg string) (err error) {
//	   defer func() {
//	     if err != nil {
//	       err = fmt.Errorf("g(%s): %v", arg, err)
//	      }
//	   }()
//	   return errors.New("my error")
//	}
func Annotatef(err *error, format string, a ...any) {
	if *err != nil {
		*err = fmt.Errorf("%s: %v", fmt.Sprintf(format, a...), *err)
	}
}
