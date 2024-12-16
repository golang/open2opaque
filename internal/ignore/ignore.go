// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ignore implements checking if a certain file (typically .proto or
// .go) should be ignored by the open2opaque pipeline.
package ignore

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/golang/glog"
)

// List allows checking if a certain file (typically .proto or .go files) should
// be ignored by the open2opaque pipeline.
type List struct {
	IgnoredFiles map[string]bool
	IgnoredDirs  []string
}

// Add adds the depot path to the ignore list.
func (l *List) Add(path string) {
	if strings.HasSuffix(path, "/") {
		l.IgnoredDirs = append(l.IgnoredDirs, path)
		return
	}
	l.IgnoredFiles[path] = true
}

// Contains returns true if the loaded ignorelist contains path.
func (l *List) Contains(path string) bool {
	if l == nil {
		return false
	}
	for _, dir := range l.IgnoredDirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return l.IgnoredFiles[path]
}

// LoadList loads an ignore list from files matching the provided glob pattern
// (see http://godoc/3/file/base/go/file#Match for the syntax definition).
func LoadList(pattern string) (*List, error) {
	matches, err := glob(pattern)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		lines = append(lines, strings.Split(strings.TrimSpace(string(b)), "\n")...)
	}
	l := &List{
		IgnoredFiles: make(map[string]bool, len(lines)),
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue // skip comments
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue // skip empty lines
		}
		l.Add(line)
	}
	log.Infof("Loaded ignore list from pattern %q: %d files and %d directories.", pattern, len(l.IgnoredFiles), len(l.IgnoredDirs))
	return l, nil
}

var glob = func(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}
