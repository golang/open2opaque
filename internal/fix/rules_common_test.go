// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kylelemons/godebug/diff"
	spb "google.golang.org/open2opaque/internal/dashboard"
	"google.golang.org/open2opaque/internal/o2o/fakeloader"
	"google.golang.org/open2opaque/internal/o2o/loader"
	"google.golang.org/open2opaque/internal/o2o/syncset"
)

const dumpSrcOnFail = false

type fakeLoaderBase struct {
	ImportPathToFiles map[string][]string
	PathToContent     map[string]string
	ExportFor         fakeloader.ExportForFunc
}

// flBase contains the fakeloader base values, i.e. the parts of the test which
// do not change between each test (testdata protobuf packages and Go Protobuf
// itself).
var flBase = func() *fakeLoaderBase {

	flb, err := goListBase()
	if err != nil {
		panic(err)
	}
	return flb
}()

func fixSource(ctx context.Context, src, srcfile string, cPkgSettings ConfiguredPackage, levels []Level) (lvl2src map[Level]string, lvl2stats map[Level][]*spb.Entry, err error) {
	// Wrap the single source file in a package that can be loaded. Also add fake support
	// packages (the proto library and compiled .proto file with messages).
	ruleName := "google.golang.org/open2opaque/internal/fix/testdata/fake"
	prefix := ruleName + "/"

	srcfile = prefix + srcfile

	// Add/overwrite the source file:
	flBase.ImportPathToFiles[ruleName] = []string{srcfile}
	flBase.PathToContent[srcfile] = src

	l := fakeloader.NewFakeLoader(
		flBase.ImportPathToFiles,
		flBase.PathToContent,
		nil,
		flBase.ExportFor)
	pkg, err := loader.LoadOne(ctx, l, &loader.Target{ID: ruleName})
	if err != nil {
		return nil, nil, fmt.Errorf("can't fix source: %v", err)
	}

	cPkg := ConfiguredPackage{
		Loader:         l,
		Pkg:            pkg,
		TypesToUpdate:  cPkgSettings.TypesToUpdate,
		BuilderTypes:   cPkgSettings.BuilderTypes,
		Levels:         levels,
		ProcessedFiles: syncset.New(),
		UseBuilders:    BuildersTestsOnly,
	}
	fixed, err := cPkg.Fix()
	if err != nil {
		return nil, nil, fmt.Errorf("can't fix source: %v", err)
	}
	var lvls []Level
	for lvl := range fixed {
		lvls = append(lvls, lvl)
	}
	sort.Slice(lvls, func(i, j int) bool {
		return !lvls[i].ge(lvls[j])
	})
	want := append([]Level{None}, levels...)
	if d := cmp.Diff(want, lvls); d != "" {
		return nil, nil, fmt.Errorf("Package() = %v;\nwant map with keys %v:\n%s\n", lvls, want, d)
	}
	for lvl, fs := range fixed {
		if len(fs) != 1 {
			return nil, nil, fmt.Errorf("Package(1 source) = %s/%d; want 1 for each None,Green,Yellow,Red", lvl, len(fs))
		}
	}

	// Extract code that the test cares about. It is between
	//    _ = "TEST CODE STARTS HERE"
	// and
	//    _ = "TEST CODE ENDS HERE"
	// Everything else wraps that code in a package that can be loaded.
	// Note that wrapped code can have different indentation levels than what the test
	// expects. Fix that too.
	const start = `_ = "TEST CODE STARTS HERE"`
	lvl2src = map[Level]string{}
	lvl2stats = map[Level][]*spb.Entry{}
	for lvl, srcs := range fixed {
		lvl2stats[lvl] = srcs[0].Stats
		src := srcs[0].Code

		sidx := strings.Index(src, start)
		if sidx < 0 {
			return nil, nil, fmt.Errorf("Fixed source doesn't contain start marker %q. Result:\n%s", start, src)
		}
		const end = `_ = "TEST CODE ENDS HERE"`
		eidx := strings.Index(src, end)
		if eidx < 0 {
			return nil, nil, fmt.Errorf("Fixed source doesn't contain end marker %q. Result:\n%s", end, src)
		}

		raw := strings.Split(src[sidx+len(start):eidx], "\n")
		first := 0
		for ; first < len(raw); first++ {
			if len(strings.TrimSpace(raw[first])) != 0 {
				break
			}
		}
		last := len(raw) - 1
		for ; last > 0; last-- {
			if len(strings.TrimSpace(raw[last])) != 0 {
				break
			}
		}
		if first > last {
			return nil, nil, fmt.Errorf("all output lines are empty in %q", raw)
		}
		var lines []string
		for i := first; i <= last; i++ {
			lines = append(lines, raw[i])
		}

		mintab := -1
		for i, ln := range lines {
			if strings.TrimSpace(ln) == "" {
				lines[i] = ""
				continue
			}
			var w int
			for _, ch := range ln {
				if ch != '\t' {
					break
				}
				w++
			}
			if mintab == -1 || w < mintab {
				mintab = w
			}
		}
		for i, ln := range lines {
			if ln != "" {
				ln = ln[mintab:]
			}
			lines[i] = ln
		}

		lvl2src[lvl] = strings.Join(lines, "\n")
	}

	// Update Location information for all statistics so that line 1 is the
	// first line introduced by the user.
	var offset int
	for _, ln := range strings.Split(src, "\n") {
		offset++
		if strings.Contains(ln, "TEST CODE STARTS HERE") {
			break
		}
	}
	for _, stats := range lvl2stats {
		for _, entry := range stats {
			entry.GetLocation().GetStart().Line -= int64(offset)
			entry.GetLocation().GetEnd().Line -= int64(offset)
		}
	}

	return lvl2src, lvl2stats, nil
}

// A very simple test that verfies that fundamentals work:
//   - fake objects are setup
//   - test packages are setup
//   - packages are setup and loaded
//   - basic rewrites work
func TestSmokeTest(t *testing.T) {
	tt := test{
		extra: `func g() string { return "" }`,
		in:    `m2.S = proto.String(g())`,
		want: map[Level]string{
			Green: `m2.SetS(g())`,
		},
	}
	runTableTest(t, tt)
}

func TestDoesntRewriteNonProtos(t *testing.T) {
	src := `notAProto.S = proto.String(g())`
	tt := test{
		extra: `
type NotAProto struct {
  S *string
  Field struct{}
}
var notAProto *NotAProto
func g() string { return "" }
`,
		in: src,
		want: map[Level]string{
			Green: src,
		},
	}
	runTableTest(t, tt)
}

// skip marks tests as skipped.
func skip(ts []test) []test {
	for i := range ts {
		ts[i].skip = "enable when fix.Rewrite is implemented"
	}
	return ts
}

type test struct {
	skip string // A reason to skip the test. Useful for disabling test-cases that don't work yet.
	desc string
	// Code added after the function enclosing the input.
	extra string
	// Input is wrapped in a package-level function. The package
	// defines M2 and M3 as proto2 and proto3 messages
	// respectively. m2 and m3 are variables of types *M2 and *M3
	// respectively.
	in string

	// Name of the source file(s) to test with. Defaults to
	// []string{"pkg_test.go"} if empty.
	srcfiles []string

	typesToUpdate map[string]bool
	builderTypes  map[string]bool

	// Each test uses either want or wantRed but not both.
	want    map[Level]string
	wantRed string // Used in tests that only do Red rewrites.

	// Asserts what expressions should be logged for gethering statistcs purposes. It may seem
	// unnecessary to assert the result per level. This verifies that we don't lose type information
	// necessary to calculate statistics.
	wantStats map[Level][]*spb.Entry
}

func runTableTest(t *testing.T, tt test) {
	t.Helper()

	if tt.skip != "" {
		t.Skip(tt.skip)
	}

	in := NewSrc(tt.in, tt.extra)
	srcfiles := tt.srcfiles
	if len(srcfiles) == 0 {
		srcfiles = []string{"pkg_test.go"}
	}
	for _, srcfile := range srcfiles {
		cpkg := ConfiguredPackage{
			TypesToUpdate: tt.typesToUpdate,
			BuilderTypes:  tt.builderTypes,
		}
		got, _, err := fixSource(context.Background(), in, srcfile, cpkg, []Level{Green, Yellow, Red})
		if err != nil {
			t.Fatalf("fixSource(%q) failed: %v; Full input:\n%s", tt.in, err, in)
		}
		failSrc := "<redacted because dumpSrcOnFail==false>"
		if dumpSrcOnFail {
			failSrc = in
		}

		for _, lvl := range []Level{Green, Yellow, Red} {
			want, ok := tt.want[lvl]
			if !ok {
				continue
			}
			want = trimNL(want)
			if d := diff.Diff(want, got[lvl]); d != "" {
				t.Errorf("fixSource(%q) = (%s) %q\nwant\n'%s'\ndiff:\n%s\nFull input source:\n------------------------------\n%s\n------------------------------\n", tt.in, lvl, got[lvl], want, d, failSrc)
			}
		}
	}
}

func runTableTests(t *testing.T, tests []test) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Helper()
			runTableTest(t, tt)
		})
	}
}

// Verify that basic level calculation works. If this test fails than other tests are likely to be incorrect.
func TestLevels(t *testing.T) {
	tt := test{
		extra: `
func b() bool { return false }
func s() string { return "" }
func m() *pb2.M2 { return nil }
`,
		in: `
m2.S = proto.String("s")
m2.B, m2.S = proto.Bool(b()), proto.String(s())
m2.S = m().S
`,
		want: map[Level]string{
			Green: `
m2.SetS("s")
m2.B, m2.S = proto.Bool(b()), proto.String(s())
if x := m(); x.HasS() {
	m2.SetS(x.GetS())
} else {
	m2.ClearS()
}
`,
			// ignore evaluation order
			Yellow: `
m2.SetS("s")
m2.SetB(b())
m2.SetS(s())
if x := m(); x.HasS() {
	m2.SetS(x.GetS())
} else {
	m2.ClearS()
}
`,
			Red: `
m2.SetS("s")
m2.SetB(b())
m2.SetS(s())
if x := m(); x.HasS() {
	m2.SetS(x.GetS())
} else {
	m2.ClearS()
}
`,
		},
	}

	runTableTest(t, tt)
}
