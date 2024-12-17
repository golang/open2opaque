// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"os/exec"
	"syscall"
)

// exit status 1 means there's a diff, but no other failure
func exitErrorMeansDiff(ee *exec.ExitError) bool {
	ws, ok := ee.Sys().(syscall.WaitStatus)
	return ok && ws.ExitStatus() == 1
}
