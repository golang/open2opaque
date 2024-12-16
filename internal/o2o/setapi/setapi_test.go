// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package setapi_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/open2opaque/internal/o2o/setapi"
	gofeaturespb "google.golang.org/protobuf/types/gofeaturespb"
)

// We added new test functions for file modification below that cover the same
// and more aspects of setapi. But we keep this test function around since it
// still works and there is no point in taking the risk of reducing test
// coverage. However, if this should become a maintenance burden in the future,
// consider removing this function.
func TestModificationOld(t *testing.T) {
	const copyrightHeader = `// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

`
	testcases := []struct {
		desc  string
		input string
		task  setapi.Task
		want  string
	}{
		{
			input: "../../../testdata/flag_edition_test1_go_proto/flag_edition_test1.proto",
			desc:  "replace_file_option_with_HYBRID_where_default_is_OPAQUE_and_clean_up_file_and_messages",
			task: setapi.Task{
				Path:      "testonly-opaque-default-dummy.proto",
				Symbol:    "",
				TargetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test1;

import "google/protobuf/go_features.proto";

option features.(pb.go).api_level = API_HYBRID;

message M1 {
  option features.(pb.go).api_level = API_OPAQUE;

  message Nested1 {
    option features.(pb.go).api_level = API_HYBRID;

    message Nested2 {}
  }

  map<string, bool> map_field = 10;
}

message M2 {
  message Nested1 {
    
  }

  message Nested2 {}

  map<string, bool> map_field = 10;
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test1_go_proto/flag_edition_test1.proto",
			desc:  "replace_file_option_with_HYBRID_where_default_is_OPAQUE_and_skip_cleanup",
			task: setapi.Task{
				Path:        "testonly-opaque-default-dummy.proto",
				TargetAPI:   gofeaturespb.GoFeatures_API_HYBRID,
				SkipCleanup: true,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test1;

import "google/protobuf/go_features.proto";

option features.(pb.go).api_level = API_HYBRID;

message M1 {
  option features.(pb.go).api_level = API_OPAQUE;

  message Nested1 {
    option features.(pb.go).api_level = API_HYBRID;

    message Nested2 {}
  }

  map<string, bool> map_field = 10;
}

message M2 {
  message Nested1 {
    option features.(pb.go).api_level = API_HYBRID;
  }

  message Nested2 {}

  map<string, bool> map_field = 10;
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test2_go_proto/flag_edition_test2.proto",
			desc:  "insert_file_option_OPEN_where_default_is_OPEN_and_clean_up",
			task: setapi.Task{
				Path:      "google/some.proto",
				TargetAPI: gofeaturespb.GoFeatures_API_OPEN,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test2;

import "google/protobuf/go_features.proto";

message M1 {
  option features.(pb.go).api_level = API_OPAQUE;

  message Nested1 {
    option
        /*multi-line*/
        features.(pb.go)
            .api_level = API_HYBRID;

    message Nested2 {}
  }

  map<string, bool> map_field = 10;
}

message M2 {
  message Nested1 {
    option features.(pb.go).api_level = API_HYBRID;
  }

  message Nested2 {}

  map<string, bool> map_field = 10;
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test2_go_proto/flag_edition_test2.proto",
			desc:  "insert_message_M2_option_OPEN_using_fully-qualified_name_and_clean_up",
			task: setapi.Task{
				Path:      "testonly-opaque-default-dummy.proto",
				Symbol:    "net.proto2.go.open2opaque.testdata.flag_edition_test2.M2",
				TargetAPI: gofeaturespb.GoFeatures_API_OPEN,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test2;

import "google/protobuf/go_features.proto";

message M1 {
  

  message Nested1 {
    option
        /*multi-line*/
        features.(pb.go)
            .api_level = API_HYBRID;

    message Nested2 {}
  }

  map<string, bool> map_field = 10;
}

message M2 {
option features.(pb.go).api_level = API_OPEN;
  message Nested1 {
    option features.(pb.go).api_level = API_HYBRID;
  }

  message Nested2 {
option features.(pb.go).api_level = API_OPAQUE;}

  map<string, bool> map_field = 10;
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test5_go_proto/flag_edition_test5.proto",
			desc:  "inserting_message_M1_option_OPAQUE_works_with_leading_comment",
			task: setapi.Task{
				Path:      "testonly-opaque-default-dummy.proto",
				Symbol:    "net.proto2.go.open2opaque.testdata.flag_edition_test5.M1",
				TargetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test5;

import "google/protobuf/go_features.proto";

option features.(pb.go).api_level = API_HYBRID;

message M1 {
option features.(pb.go).api_level = API_OPAQUE;
  // Leading comment on message should not be misplaced
  // when API flag is inserted.
  int32 int_field = 1;
}

message M2 {
  /**
   * Leading block comment on message should not be misplaced
   * when API flag is inserted.
   */
  int32 int_field = 1;
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test5_go_proto/flag_edition_test5.proto",
			desc:  "inserting_message_M1_option_OPAQUE_works_with_leading_block_comment",
			task: setapi.Task{
				Path:      "testonly-opaque-default-dummy.proto",
				Symbol:    "net.proto2.go.open2opaque.testdata.flag_edition_test5.M2",
				TargetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test5;

import "google/protobuf/go_features.proto";

option features.(pb.go).api_level = API_HYBRID;

message M1 {
  // Leading comment on message should not be misplaced
  // when API flag is inserted.
  int32 int_field = 1;
}

message M2 {
option features.(pb.go).api_level = API_OPAQUE;
  /**
   * Leading block comment on message should not be misplaced
   * when API flag is inserted.
   */
  int32 int_field = 1;
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test2_go_proto/flag_edition_test2.proto",
			desc:  "delete_message_M1.Nested1_option_to_make_OPAQUE_and_skip_cleanup",
			task: setapi.Task{
				Path:        "testonly-opaque-default-dummy.proto",
				Symbol:      "M1.Nested1",
				TargetAPI:   gofeaturespb.GoFeatures_API_OPAQUE,
				SkipCleanup: true,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test2;

import "google/protobuf/go_features.proto";

message M1 {
  option features.(pb.go).api_level = API_OPAQUE;

  message Nested1 {
    

    message Nested2 {
option features.(pb.go).api_level = API_HYBRID;}
  }

  map<string, bool> map_field = 10;
}

message M2 {
  message Nested1 {
    option features.(pb.go).api_level = API_HYBRID;
  }

  message Nested2 {}

  map<string, bool> map_field = 10;
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test3_go_proto/flag_edition_test3.proto",
			desc:  "leading_comments_exempt_file-level_API_flag_changes_and_cleanup",
			task: setapi.Task{
				Path:      "testonly-opaque-default-dummy.proto",
				TargetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test3;

import "google/protobuf/go_features.proto";

// Leading comment exempts api_level from modifications.
option features.(pb.go).api_level = API_HYBRID;

message M1 {
  // Exemption on message level.
  option features.(pb.go).api_level = API_HYBRID;

  message Nested1 {
    // Exemption on nested-message level.
    option features.(pb.go).api_level = API_OPAQUE;
  }
}

message M2 {
  
}
`,
		},
		{
			input: "../../../testdata/flag_edition_test3_go_proto/flag_edition_test3.proto",
			desc:  "leading_comments_exempt_message_API_flag_changes_and_cleanup",
			task: setapi.Task{
				Path:      "testonly-opaque-default-dummy.proto",
				Symbol:    "M1.Nested1",
				TargetAPI: gofeaturespb.GoFeatures_API_OPEN,
			},
			want: copyrightHeader + `edition = "2023";

package net.proto2.go.open2opaque.testdata.flag_edition_test3;

import "google/protobuf/go_features.proto";

// Leading comment exempts api_level from modifications.
option features.(pb.go).api_level = API_HYBRID;

message M1 {
  // Exemption on message level.
  option features.(pb.go).api_level = API_HYBRID;

  message Nested1 {
    // Exemption on nested-message level.
    option features.(pb.go).api_level = API_OPAQUE;
  }
}

message M2 {
  
}
`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			var err error
			tc.task.Content, err = os.ReadFile(tc.input)
			if err != nil {
				t.Fatalf("runfiles.ReadFile: %v", err)
			}
			ctx := context.Background()
			contentClone := slices.Clone(tc.task.Content)
			got, err := setapi.Process(ctx, tc.task, "cat")
			if err != nil {
				t.Fatalf("setapi.Process: %v", err)
			}
			if !slices.Equal(tc.task.Content, contentClone) {
				t.Fatalf("setapi.Process modified Task.Content")
			}
			if diff := cmp.Diff(tc.want, string(got)); diff != "" {
				t.Fatalf("diff setapi.Process (-want +got):\n%v\n", diff)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	testcases := []struct {
		desc          string
		input         string
		symbol        string
		errorOnExempt bool
		wantErr       bool
	}{
		{
			desc:    "error_for_unknown_symbol",
			input:   "../../../testdata/flag_edition_test2_go_proto/flag_edition_test2.proto",
			symbol:  "UNKNOWN",
			wantErr: true,
		},
		{
			desc:          "error_for_file-level_flag_with_leading_comment_with_errorOnExempt",
			input:         "../../../testdata/flag_edition_test3_go_proto/flag_edition_test3.proto",
			errorOnExempt: true,
			wantErr:       true,
		},
		{
			desc:  "no_error_for_file-level_flag_with_leading_comment_without_errorOnExempt",
			input: "../../../testdata/flag_edition_test3_go_proto/flag_edition_test3.proto",
		},
		{
			desc:          "error_for_message_flag_with_leading_comment_with_errorOnExempt",
			input:         "../../../testdata/flag_edition_test3_go_proto/flag_edition_test3.proto",
			symbol:        "M1.Nested1",
			errorOnExempt: true,
			wantErr:       true,
		},
		{
			desc:          "error_for_message_flag_with_leading_comment_with_errorOnExempt_on_cleanup_for_M1",
			input:         "../../../testdata/flag_edition_test3_go_proto/flag_edition_test3.proto",
			symbol:        "M1",
			errorOnExempt: true,
			wantErr:       true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			task := setapi.Task{
				Path:          "testonly-opaque-default-dummy.proto",
				Symbol:        tc.symbol,
				TargetAPI:     gofeaturespb.GoFeatures_API_OPEN,
				ErrorOnExempt: tc.errorOnExempt,
			}
			var err error
			task.Content, err = os.ReadFile(tc.input)
			if err != nil {
				t.Fatalf("runfiles.ReadFile: %v", err)
			}
			ctx := context.Background()
			_, err = setapi.Process(ctx, task, "cat")
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("setapi.Process: got err=%q, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

const filenameWithOpaqueDefault = "testonly-opaque-default-dummy.proto"

func TestSetFileAPI(t *testing.T) {
	testCases := []struct {
		name          string
		protoIn       string
		targetAPI     gofeaturespb.GoFeatures_APILevel
		skipCleanup   bool
		errorOnExempt bool
		protoWant     string
		wantErr       bool
	}{
		{
			name: "edition_is_default_opaque_not_explicit__do_nothing",
			protoIn: `
edition = "2023";
package pkg;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
message M{}
`,
		},

		{
			name: "edition_is_default_opaque_explicit__remove_flag_and_eol_comment",
			protoIn: `
edition = "2023";
package pkg;option features.(pb.go).api_level = API_OPAQUE;  // end-of-line comment
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
message M{}
`,
		},

		{
			name: "edition_is_default_opaque_explicit__skip_cleanup_dont_remove",
			protoIn: `
edition = "2023";
package pkg;option features.(pb.go).api_level = API_OPAQUE;  // end-of-line comment
message M{}
`,
			targetAPI:   gofeaturespb.GoFeatures_API_OPAQUE,
			skipCleanup: true,
			protoWant: `
edition = "2023";
package pkg;option features.(pb.go).api_level = API_OPAQUE;  // end-of-line comment
message M{}
`,
		},

		{
			name: "edition_is_hybrid_explicit_want_opaque__remove_flag",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;

message M{}
`,
		},

		{
			name: "edition_is_default_opaque_not_explicit_want_hybrid__insert_flag",
			protoIn: `
edition = "2023";
package pkg;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_no_pkg_is_default_opaque_not_explicit_want_hybrid__insert_flag",
			protoIn: `
edition = "2023";
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_other_option_is_default_opaque_not_explicit_want_hybrid__insert_flag",
			protoIn: `
edition = "2023";
package pkg;
option java_package = "foo";
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
package pkg;
option java_package = "foo";
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_import_is_default_opaque_not_explicit_want_hybrid__insert_flag",
			protoIn: `
edition = "2023";
package pkg;
import "foo.proto";
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
package pkg;
import "foo.proto";
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_is_default_opaque_not_explicit_want_hybrid__insert_flag",
			protoIn: `
edition = "2023";
package pkg;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_no_pkg_is_default_opaque_not_explicit_want_hybrid__insert_flag",
			protoIn: `
edition = "2023";
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_is_open_explicit_want_hybrid__replace_flag",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_OPEN;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_is_opaque_explicit_want_hybrid__replace_flag",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_OPAQUE;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "edition_is_open_explicit_want_open__do_nothing",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_OPEN;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_OPEN,
			protoWant: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_OPEN;
message M{}
`,
		},

		{
			name: "leading_comment_prevents_replacement_with_errorOnExempt",
			protoIn: `
edition = "2023";
package pkg;
// leading comment
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
			targetAPI:     gofeaturespb.GoFeatures_API_OPEN,
			errorOnExempt: true,
			wantErr:       true,
		},

		{
			name: "leading_comment_prevents_replacement_without_errorOnExempt",
			protoIn: `
edition = "2023";
package pkg;
// leading comment
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_OPEN,
			protoWant: `
edition = "2023";
package pkg;
// leading comment
option features.(pb.go).api_level = API_HYBRID;
message M{}
`,
		},

		{
			name: "leading_comment_prevents_deletion_with_errorOnExempt",
			protoIn: `
edition = "2023";
package pkg;
// leading comment
option features.(pb.go).api_level = API_OPAQUE;
message M{}
`,
			targetAPI:     gofeaturespb.GoFeatures_API_OPAQUE,
			errorOnExempt: true,
			wantErr:       true,
		},

		{
			name: "leading_comment_prevents_deletion_without_errorOnExempt",
			protoIn: `
edition = "2023";
package pkg;
// leading comment
option features.(pb.go).api_level = API_OPAQUE;
message M{}
`,
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
// leading comment
option features.(pb.go).api_level = API_OPAQUE;
message M{}
`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := setapi.Task{
				Path:          filenameWithOpaqueDefault,
				Content:       []byte(tc.protoIn),
				TargetAPI:     tc.targetAPI,
				SkipCleanup:   tc.skipCleanup,
				ErrorOnExempt: tc.errorOnExempt,
			}
			contentClone := slices.Clone(task.Content)
			protoGot, err := setapi.Process(context.Background(), task, "cat")
			switch {
			case err != nil && tc.wantErr:
				return
			case err == nil && tc.wantErr:
				t.Fatalf("wanted error but got nil")
			case err != nil && !tc.wantErr:
				t.Fatalf("didn't want error but got: %v", err)
			}
			if !slices.Equal(task.Content, contentClone) {
				t.Fatalf("setapi.Process modified Task.Content")
			}
			if diff := cmp.Diff(tc.protoWant, string(protoGot)); diff != "" {
				t.Errorf("setFileAPI: diff (-want +got):\n%v\n", diff)
			}
		})
	}
}

func TestSetMsgAPI(t *testing.T) {
	testCases := []struct {
		name                                string
		protoIn                             string
		msgName                             string
		targetAPI                           gofeaturespb.GoFeatures_APILevel
		protoWant                           string
		skipCleanup                         bool
		wantErr                             bool
		wantErrorLeadingCommentPreventsEdit bool
	}{
		{
			name: "edition_file_default_opaque__message_non_explicit_opaque_set_to_opaque__do_nothing",
			protoIn: `
edition = "2023";
package pkg;
message M{
string s = 1;
}
`,
			msgName:   "M",
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
message M{
string s = 1;
}
`,
		},

		{
			name: "edition_file_default_opaque__message_explicit_opaque_set_to_opaque__remove",
			protoIn: `
edition = "2023";
package pkg;
message M{
option features.(pb.go).api_level = API_OPAQUE;
string s = 1;
}
`,
			msgName:   "M",
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
message M{

string s = 1;
}
`,
		},

		{
			name: "edition_file_default_opaque__fully_qualified_message_explicit_opaque_set_to_opaque__remove",
			protoIn: `
edition = "2023";
package pkg;
message M{
option features.(pb.go).api_level = API_OPAQUE;
string s = 1;
}
`,
			msgName:   "pkg.M",
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
message M{

string s = 1;
}
`,
		},

		{
			name: "edition_file_default_opaque__message_explicit_opaque_set_to_opaque__skip_cleanup_dont_remove",
			protoIn: `
edition = "2023";
package pkg;
message M{
option features.(pb.go).api_level = API_OPAQUE;
string s = 1;
}
`,
			msgName:     "M",
			targetAPI:   gofeaturespb.GoFeatures_API_OPAQUE,
			skipCleanup: true,
			protoWant: `
edition = "2023";
package pkg;
message M{
option features.(pb.go).api_level = API_OPAQUE;
string s = 1;
}
`,
		},

		{
			name: "edition_file_default_opaque__message_hybrid_set_to_opaque__remove",
			protoIn: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_HYBRID; // end-of-line comment
    string s = 1;
    message A2{
      option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
    }
  }
}
`,
			msgName:   "A.A1",
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
message A{
  message A1{
    
    string s = 1;
    message A2{
      option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
    }
  }
}
`,
		},

		{
			name: "edition_file_default_opaque__message_hybrid_set_to_hybrid__do_nothing",
			protoIn: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_HYBRID; // end-of-line comment
    string s = 1;
    message A2{
      option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
    }
  }
}
`,
			msgName:   "A.A1",
			targetAPI: gofeaturespb.GoFeatures_API_HYBRID,
			protoWant: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_HYBRID; // end-of-line comment
    string s = 1;
    message A2{
      
      string s = 1;
    }
  }
}
`,
		},

		{
			name: "edition_file_default_opaque__message_hybrid_set_to_open__replace",
			protoIn: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_HYBRID; // end-of-line comment
    string s = 1;
    message A2{
      option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
    }
  }
}
`,
			msgName:   "A.A1",
			targetAPI: gofeaturespb.GoFeatures_API_OPEN,
			protoWant: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_OPEN; // end-of-line comment
    string s = 1;
    message A2{
      option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
    }
  }
}
`,
		},

		{
			name: "edition_file_default_opaque__message_hybrid_set_to_opaque__remove_and_fix_children",
			protoIn: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_HYBRID;
    string s = 1;
    message A2{
      string s = 1;
      message A3 {
        string s = 1;
      }
    }
  }
}
message B{
}
`,
			msgName:   "A.A1",
			targetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			protoWant: `
edition = "2023";
package pkg;
message A{
  message A1{
    
    string s = 1;
    message A2{
option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
      message A3 {
        string s = 1;
      }
    }
  }
}
message B{
}
`,
		},

		{
			name: "edition_file_default_opaque__message_hybrid_set_to_open__replace_and_fix_children",
			protoIn: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_HYBRID;
    string s = 1;
    message A2{
      string s = 1;
      message A3 {
        string s = 1;
      }
    }
  }
}
message B{
}
`,
			msgName:   "A.A1",
			targetAPI: gofeaturespb.GoFeatures_API_OPEN,
			protoWant: `
edition = "2023";
package pkg;
message A{
  message A1{
    option features.(pb.go).api_level = API_OPEN;
    string s = 1;
    message A2{
option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
      message A3 {
        string s = 1;
      }
    }
  }
}
message B{
}
`,
		},

		{
			name: "edition_file_default_opaque__message_hybrid_set_to_opaque__removal_prevented_by_leading_comment",
			protoIn: `
edition = "2023";
package pkg;
message A{
  message A1{
    // leading comment prevents removal
    option features.(pb.go).api_level = API_HYBRID; // end-of-line comment
    string s = 1;
    message A2{
      option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
    }
  }
}
`,
			msgName:                             "A.A1",
			targetAPI:                           gofeaturespb.GoFeatures_API_OPAQUE,
			wantErr:                             true,
			wantErrorLeadingCommentPreventsEdit: true,
		},

		{
			name: "edition_file_default_opaque__message_hybrid_set_to_open__replacement_prevented_by_leading_comment",
			protoIn: `
edition = "2023";
package pkg;
message A{
  message A1{
    // leading comment prevents replacement
    option features.(pb.go).api_level = API_HYBRID; // end-of-line comment
    string s = 1;
    message A2{
      option features.(pb.go).api_level = API_HYBRID;
      string s = 1;
    }
  }
}
`,
			msgName:                             "A.A1",
			targetAPI:                           gofeaturespb.GoFeatures_API_OPEN,
			wantErr:                             true,
			wantErrorLeadingCommentPreventsEdit: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := setapi.Task{
				Path:          filenameWithOpaqueDefault,
				Content:       []byte(tc.protoIn),
				Symbol:        tc.msgName,
				TargetAPI:     tc.targetAPI,
				SkipCleanup:   tc.skipCleanup,
				ErrorOnExempt: true,
			}
			protoGot, err := setapi.Process(context.Background(), task, "cat")
			switch {
			case err != nil && tc.wantErr:
				if tc.wantErrorLeadingCommentPreventsEdit && !errors.Is(err, setapi.ErrLeadingCommentPreventsEdit) {
					t.Fatalf("wanted wantErrorLeadingCommentPreventsEdit but got: %v", err)
				}
				return
			case err == nil && tc.wantErr:
				t.Fatalf("wanted error but got nil")
			case err != nil && !tc.wantErr:
				t.Fatalf("didn't want error but got: %v", err)
			}
			if diff := cmp.Diff(tc.protoWant, string(protoGot)); diff != "" {
				t.Errorf("setMsgAPI: diff (-want +got):\n%v\n", diff)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	testCases := []struct {
		name      string
		protoIn   string
		protoWant string
	}{
		{
			name: "edition_is_default_opaque_with_explicit_opaque__remove",
			protoIn: `
edition = "2023";
package pkg;
option features.(pb.go).api_level = API_OPAQUE; // end-of-line comment
message M{}
`,
			protoWant: `
edition = "2023";
package pkg;

message M{}
`,
		},

		{
			name: "edition_is_default_opaque_some_messages_opaque__remove",
			protoIn: `
edition = "2023";
package pkg;
message A{
option features.(pb.go).api_level = API_OPAQUE; // end-of-line comment
message A1{
option features.(pb.go).api_level = API_OPEN; // end-of-line comment
message A2{
option features.(pb.go).api_level = API_OPAQUE;
}
}
}
message B{
// leading comment
option features.(pb.go).api_level = API_OPAQUE;
}
`,
			protoWant: `
edition = "2023";
package pkg;
message A{

message A1{
option features.(pb.go).api_level = API_OPEN; // end-of-line comment
message A2{
option features.(pb.go).api_level = API_OPAQUE;
}
}
}
message B{
// leading comment
option features.(pb.go).api_level = API_OPAQUE;
}
`,
		},

		{
			name: "edition_is_default_opaque_some_messages_opaque__remove_but_respect_api_inheritance",
			protoIn: `
edition = "2023";
package pkg;
message A{
option features.(pb.go).api_level = API_OPAQUE;
message A1{
option features.(pb.go).api_level = API_OPEN;
message A2{
// cannot clean up, because A2 inherits OPEN from A1
option features.(pb.go).api_level = API_OPAQUE;
}
}
}
message B{
option features.(pb.go).api_level = API_OPAQUE;
}
`,
			protoWant: `
edition = "2023";
package pkg;
message A{

message A1{
option features.(pb.go).api_level = API_OPEN;
message A2{
// cannot clean up, because A2 inherits OPEN from A1
option features.(pb.go).api_level = API_OPAQUE;
}
}
}
message B{

}
`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := setapi.Task{
				Path:      filenameWithOpaqueDefault,
				Content:   []byte(tc.protoIn),
				TargetAPI: gofeaturespb.GoFeatures_API_OPAQUE,
			}
			contentClone := slices.Clone(task.Content)
			protoGot, err := setapi.Process(context.Background(), task, "cat")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(task.Content, contentClone) {
				t.Fatalf("setapi.Process modified Task.Content")
			}
			if diff := cmp.Diff(tc.protoWant, string(protoGot)); diff != "" {
				t.Errorf("setMsgAPI: diff (-want +got):\n%v\n", diff)
			}
		})
	}
}
