// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protoparse_test

import (
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/open2opaque/internal/protodetect"
	"google.golang.org/open2opaque/internal/protoparse"
	gofeaturespb "google.golang.org/protobuf/types/gofeaturespb"
)

const filenameWithOpaqueDefault = "foo.proto"

func TestTextRangeToByteRange(t *testing.T) {
	testCases := []struct {
		in        string
		textRange protoparse.TextRange
		want      string
	}{
		{
			in: "0123456789",
			textRange: protoparse.TextRange{
				BeginLine: 0,
				BeginCol:  2,
				EndLine:   0,
				EndCol:    6,
			},
			want: "2345",
		},
		{
			in: `0Hello, 世界0
1Hello, 世界1
2Hello, 世界2
3Hello, 世界3`,
			textRange: protoparse.TextRange{
				BeginLine: 1,
				BeginCol:  8,
				EndLine:   3,
				EndCol:    6,
			},
			want: `世界1
2Hello, 世界2
3Hello`,
		},
	}
	for _, tc := range testCases {
		from, to, err := tc.textRange.ToByteRange([]byte(tc.in))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(tc.want, string(tc.in[from:to])); diff != "" {
			t.Errorf("TextRange.ToByteRange: diff (-want +got):\n%v\n", diff)
		}
	}
}

func TestFileOpt(t *testing.T) {
	testCases := []struct {
		name                  string
		protoIn               string
		wantOpt               *protoparse.FileOpt
		wantAPIStr            string
		wantHasLeadingComment bool
	}{
		{
			name: "proto3_default_opaque",
			protoIn: `
syntax = "proto3";
package pkg;
message M{}
`,
			wantOpt: &protoparse.FileOpt{
				File:       filenameWithOpaqueDefault,
				Package:    "pkg",
				GoAPI:      protodetect.DefaultFileLevel("dummy.proto"),
				IsExplicit: false,
				Syntax:     "proto3",
			},
		},

		{
			name: "edition_default_opaque",
			protoIn: `
edition = "2023";
package pkg;
message M{}
`,
			wantOpt: &protoparse.FileOpt{
				File:       filenameWithOpaqueDefault,
				Package:    "pkg",
				GoAPI:      protodetect.DefaultFileLevel("dummy.proto"),
				IsExplicit: false,
				Syntax:     "editions",
			},
		},

		{
			name: "edition_explicit_hybrid",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
			wantOpt: &protoparse.FileOpt{
				File:       filenameWithOpaqueDefault,
				Package:    "pkg",
				GoAPI:      gofeaturespb.GoFeatures_API_HYBRID,
				IsExplicit: true,
				Syntax:     "editions",
			},
			wantAPIStr: "option features.(pb.go).api_level = API_HYBRID;",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := protoparse.NewParserWithAccessor(func(string) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(tc.protoIn)), nil
			})
			got, err := parser.ParseFile(filenameWithOpaqueDefault, true)
			if err != nil {
				t.Fatal(err)
			}
			ingores := cmp.Options{
				cmpopts.IgnoreFields(protoparse.FileOpt{}, "APIInfo"),
				cmpopts.IgnoreFields(protoparse.FileOpt{}, "SourceCodeInfo"),
				cmpopts.IgnoreFields(protoparse.FileOpt{}, "MessageOpts"),
				cmpopts.IgnoreFields(protoparse.FileOpt{}, "Desc"),
			}
			if diff := cmp.Diff(tc.wantOpt, got, ingores...); diff != "" {
				t.Errorf("parser.ParseFile(): diff (-want +got):\n%v\n", diff)
			}

			if !tc.wantOpt.IsExplicit {
				// File doesn't explicitly define API flag, don't check content of
				// APIInfo
				return
			}
			if got.APIInfo == nil {
				t.Fatal("API flag is set explicitly, but APIInfo is nil")
			}

			if got, want := got.APIInfo.HasLeadingComment, tc.wantHasLeadingComment; got != want {
				t.Fatalf("got APIInfo.HasLeadingComment %v, want %v", got, want)
			}

			from, to, err := got.APIInfo.TextRange.ToByteRange([]byte(tc.protoIn))
			if err != nil {
				t.Fatalf("TextRange.ToByteRange: %v", err)
			}
			gotAPIStr := string([]byte(tc.protoIn)[from:to])
			if diff := cmp.Diff(tc.wantAPIStr, gotAPIStr); diff != "" {
				t.Errorf("API string: diff (-want +got):\n%v\n", diff)
			}
		})
	}
}

func traverseMsg(opt *protoparse.MessageOpt, f func(*protoparse.MessageOpt)) {
	f(opt)
	for _, c := range opt.Children {
		traverseMsg(c, f)
	}
}

func TestMessageOpt(t *testing.T) {
	type msgInfo struct {
		Opt               *protoparse.MessageOpt
		APIString         string
		HasLeadingComment bool
	}

	testCases := []struct {
		name    string
		protoIn string
		want    []msgInfo
	}{

		{
			name: "edition_file_hybrid_message_one_msg_open",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message A{
  option features.(pb.go).api_level = API_OPEN;
  message A1{}
}
`,
			want: []msgInfo{
				{
					Opt: &protoparse.MessageOpt{
						Message:    "A",
						GoAPI:      gofeaturespb.GoFeatures_API_OPEN,
						IsExplicit: true,
					},
					APIString: `option features.(pb.go).api_level = API_OPEN;`,
				},
				{
					Opt: &protoparse.MessageOpt{
						Message: "A.A1",
						// API level of parent message is inherited.
						GoAPI:      gofeaturespb.GoFeatures_API_OPEN,
						IsExplicit: false,
					},
				},
			},
		},

		{
			name: "edition_file_hybrid_message_one_msg_open_with_leading_comment",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message A{
  // leading comment
  option features.(pb.go).api_level = API_OPEN;
  message A1{
    option features.(pb.go).api_level = API_OPAQUE;
  }
}
`,
			// https://github.com/bufbuild/protocompile/blob/main/ast/file_info.go#L362-L364
			want: []msgInfo{
				{
					Opt: &protoparse.MessageOpt{
						Message:    "A",
						GoAPI:      gofeaturespb.GoFeatures_API_OPEN,
						IsExplicit: true,
					},
					APIString:         `option features.(pb.go).api_level = API_OPEN;`,
					HasLeadingComment: true,
				},
				{
					Opt: &protoparse.MessageOpt{
						Message:    "A.A1",
						GoAPI:      gofeaturespb.GoFeatures_API_OPAQUE,
						IsExplicit: true,
					},
					APIString:         `option features.(pb.go).api_level = API_OPAQUE;`,
					HasLeadingComment: false,
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := protoparse.NewParserWithAccessor(func(string) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(tc.protoIn)), nil
			})
			gotFopt, err := parser.ParseFile(filenameWithOpaqueDefault, false)
			if err != nil {
				t.Fatal(err)
			}

			// Flatten the tree of message options and wrap into []msgInfo to allow
			// easy comparison. Check tree structure below.
			var gotInfo []msgInfo
			for _, optLoop := range gotFopt.MessageOpts {
				traverseMsg(optLoop, func(opt *protoparse.MessageOpt) {
					info := msgInfo{Opt: opt}
					if opt.IsExplicit {
						if opt.APIInfo == nil {
							t.Fatalf("API flag is set explicitly, but APIInfo of message %q is nil", opt.Message)
						}
						info.HasLeadingComment = opt.APIInfo.HasLeadingComment
						from, to, err := opt.APIInfo.TextRange.ToByteRange([]byte(tc.protoIn))
						if err != nil {
							t.Fatalf("TextRange.ToByteRange: %v", err)
						}
						info.APIString = string([]byte(tc.protoIn)[from:to])
					}
					gotInfo = append(gotInfo, info)
				})
			}

			ingores := cmp.Options{
				cmpopts.IgnoreFields(protoparse.MessageOpt{}, "APIInfo"),
				cmpopts.IgnoreFields(protoparse.MessageOpt{}, "LocPath"),
				cmpopts.IgnoreFields(protoparse.MessageOpt{}, "Parent"),
				cmpopts.IgnoreFields(protoparse.MessageOpt{}, "Children"),
			}
			if diff := cmp.Diff(tc.want, gotInfo, ingores...); diff != "" {
				t.Errorf("diff (-want +got):\n%v\n", diff)
			}

			// Check the tree structure of the message options. If I'm called A.A1.A2,
			// is my parent called A.A1?
			for _, optOuter := range gotFopt.MessageOpts {
				traverseMsg(optOuter, func(opt *protoparse.MessageOpt) {
					parts := strings.Split(opt.Message, ".")
					if len(parts) == 1 {
						if opt.Parent != nil {
							t.Errorf("parent pointer of top-level message %q isn't nil", opt.Message)
						}
						return
					}
					if opt.Parent == nil {
						t.Errorf("parent pointer of nested message %q is nil", opt.Message)
					}
					if got, want := opt.Parent.Message, strings.Join(parts[:len(parts)-1], "."); got != want {
						t.Errorf("got name of parent message %q, want %q", got, want)
					}
				})
			}
		})
	}
}
