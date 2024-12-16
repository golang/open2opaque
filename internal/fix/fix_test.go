// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/open2opaque/internal/o2o/fakeloader"
	"google.golang.org/open2opaque/internal/o2o/loader"
	"google.golang.org/open2opaque/internal/o2o/syncset"
)

func TestLevelsComparison(t *testing.T) {
	all := []Level{None, Green, Yellow, Red}

	var got []Level
	for i := 0; i < 10; i++ {
		got = append(got, all...)
	}
	sort.Slice(got, func(i, j int) bool {
		return got[i].le(got[j])
	})

	var want []Level
	for _, lvl := range all {
		for i := 0; i < 10; i++ {
			want = append(want, lvl)
		}
	}

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Sort returned %v want %v; diff:\n%s", got, want, d)
	}
}

func TestPopulatesModifiedAndGeneratedFlags(t *testing.T) {
	ruleName := "fake"
	prefix := ruleName + "/"
	pathPrefix := prefix

	l := fakeloader.NewFakeLoader(
		map[string][]string{ruleName: []string{
			prefix + "updated.go",
			prefix + "nochange.go",
			prefix + "generated_updated.go",
			prefix + "generated_nochange.go",
		}},
		map[string]string{
			prefix + "updated.go": `package test

type MessageState struct{}

type M struct {
	state MessageState ` + "`" + `protogen:"hybrid.v1"` + "`" + `
	S  *string
}

func (*M) Reset() { }
func (*M) String() string { return "" }
func (*M) ProtoMessage() { }

func f(m *M) {
	_ = m.S != nil
}
`,
			prefix + "nochange.go": "package test\n",
		},
		map[string]string{
			prefix + "generated_updated.go": `package test

func g(m *M) {
	_ = m.S != nil
}
`,
			prefix + "generated_nochange.go": "package test\n",
		},
		nil)

	ctx := context.Background()
	for _, lvl := range []Level{None, Green, Yellow, Red} {
		t.Run(string(lvl), func(t *testing.T) {
			pkg, err := loader.LoadOne(ctx, l, &loader.Target{ID: ruleName})
			if err != nil {
				t.Fatalf("Can't load %q: %v:", ruleName, err)
			}

			cPkg := ConfiguredPackage{
				Pkg:            pkg,
				Levels:         []Level{lvl},
				ProcessedFiles: syncset.New(),
				UseBuilders:    BuildersTestsOnly,
			}
			got, err := cPkg.Fix()
			if err != nil {
				t.Fatalf("Can't fix %q: %v", ruleName, err)
			}

			want := []*FixedFile{
				{Path: pathPrefix + "updated.go", Modified: true, Generated: false},
				{Path: pathPrefix + "nochange.go", Modified: false, Generated: false},
				{Path: pathPrefix + "generated_updated.go", Modified: true, Generated: true},
				{Path: pathPrefix + "generated_nochange.go", Modified: false, Generated: true},
			}
			ignored := []string{"Code", "Stats", "OriginalCode", "RedFixes"}
			if lvl == None {
				ignored = append(ignored, "Modified")
			}

			if d := cmp.Diff(want, got[lvl], cmpopts.IgnoreFields(FixedFile{}, ignored...)); d != "" {
				t.Fatalf("Package() = %s, want %s; diff:\n%s", got[lvl], want, d)
			}
		})
	}
}
