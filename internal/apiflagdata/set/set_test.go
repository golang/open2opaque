// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set_test

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/open2opaque/internal/apiflagdata/set"
)

var empty = struct{}{}

func TestNewStrings(t *testing.T) {
	testcases := []struct {
		input []string
		want  set.Strings
	}{
		{
			input: nil,
			want:  set.Strings{},
		},
		{
			input: []string{"hello"},
			want: set.Strings{
				"hello": empty,
			},
		},
		{
			input: []string{"foo", "bar"},
			want: set.Strings{
				"bar": empty,
				"foo": empty,
			},
		},
		{
			input: []string{"foo", "bar", "foo"},
			want: set.Strings{
				"bar": empty,
				"foo": empty,
			},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			got := set.NewStrings(tc.input...)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("diff -want +got\n%s", diff)
			}
		})
	}
}

func TestStringsAdd(t *testing.T) {
	testcases := []struct {
		s     set.Strings
		input string
		want  set.Strings
	}{
		{
			s:     set.NewStrings(),
			input: "foo",
			want: set.Strings{
				"foo": empty,
			},
		},
		{
			s:     set.NewStrings("foo"),
			input: "bar",
			want: set.Strings{
				"bar": empty,
				"foo": empty,
			},
		},
		{
			s:     set.NewStrings("foo"),
			input: "foo",
			want: set.Strings{
				"foo": empty,
			},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			tc.s.Add(tc.input)
			if diff := cmp.Diff(tc.want, tc.s); diff != "" {
				t.Errorf("diff -want +got\n%s", diff)
			}
		})
	}
}

func TestStringsAddSet(t *testing.T) {
	testcases := []struct {
		s1   set.Strings
		s2   set.Strings
		want set.Strings
	}{
		{
			s1: set.NewStrings(),
			s2: set.NewStrings("a", "b"),
			want: set.Strings{
				"a": empty,
				"b": empty,
			},
		},
		{
			s1: set.NewStrings("a", "b"),
			s2: set.NewStrings(),
			want: set.Strings{
				"a": empty,
				"b": empty,
			},
		},
		{
			s1: set.NewStrings("a"),
			s2: set.NewStrings("b"),
			want: set.Strings{
				"a": empty,
				"b": empty,
			},
		},
		{
			s1: set.NewStrings("a", "b"),
			s2: set.NewStrings("b", "c"),
			want: set.Strings{
				"a": empty,
				"b": empty,
				"c": empty,
			},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			tc.s1.AddSet(tc.s2)
			if diff := cmp.Diff(tc.want, tc.s1); diff != "" {
				t.Errorf("diff -want +got\n%s", diff)
			}
		})
	}
}

func TestStringsContains(t *testing.T) {
	testcases := []struct {
		s     set.Strings
		input string
		want  bool
	}{
		{
			s:     set.NewStrings(),
			input: "bar",
			want:  false,
		},
		{
			s:     set.NewStrings("foo"),
			input: "foo",
			want:  true,
		},
		{
			s:     set.NewStrings("bar"),
			input: "qux",
			want:  false,
		},
		{
			s:     set.NewStrings("foo", "bar"),
			input: "foo",
			want:  true,
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			got := tc.s.Contains(tc.input)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStringsSlice(t *testing.T) {
	testcases := [][]string{
		{},
		{"a"},
		{"z", "a", "j"},
	}

	for _, input := range testcases {
		t.Run("", func(t *testing.T) {
			s := set.NewStrings(input...)
			got := s.ToSlice()
			// Sort the returned slice and input before doing the comparison.
			sort.Strings(got)
			sort.Strings(input)
			if diff := cmp.Diff(input, got); diff != "" {
				t.Errorf("diff -want +got\n%s", diff)
			}
		})
	}
}
