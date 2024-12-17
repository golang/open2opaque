// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !linux

package fix

import "os/exec"

// exit status 1 means there's a diff, but no other failure
func exitErrorMeansDiff(*exec.ExitError) bool {
	// No way to inspect the exit code on plan9.
	// We just assume all diffs fail.
	return false
}
