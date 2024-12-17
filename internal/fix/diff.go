// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func udiff(x, y []byte) ([]byte, error) {
	if bytes.Equal(x, y) {
		return nil, nil
	}
	xp, err := pipe(x)
	if err != nil {
		return nil, err
	}
	defer xp.Close()
	yp, err := pipe(y)
	if err != nil {
		return nil, err
	}
	defer yp.Close()

	var stderr bytes.Buffer
	cmd := exec.Command("diff", "-u", "/dev/fd/3", "/dev/fd/4")
	cmd.ExtraFiles = []*os.File{xp, yp}
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if ee, ok := err.(*exec.ExitError); ok {
		if exitErrorMeansDiff(ee) {
			err = nil
		}
	}
	if err != nil {
		return nil, err
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("diff: %s", &stderr)
	}
	nl := []byte("\n")
	lines := bytes.Split(stdout, nl)
	if len(lines) < 2 {
		return stdout, nil
	}
	if strings.HasPrefix(string(lines[0]), "--- /dev/fd/3\t") &&
		strings.HasPrefix(string(lines[1]), "+++ /dev/fd/4\t") {
		stdout = bytes.Join(lines[2:], nl)
	}
	return stdout, nil
}

func pipe(data []byte) (*os.File, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("os.Pipe: %v", err)
	}
	go func() {
		pw.Write(data)
		pw.Close()
	}()
	return pr, nil
}
