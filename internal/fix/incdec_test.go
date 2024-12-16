// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestIncDecUnary(t *testing.T) {
	tests := []test{
		{
			desc:  "unary expressions: contexts",
			extra: `var x struct { M *pb2.M2 }`,
			in: `
*m2.I32++
(*m2.I32)++
*m2.M.I32++
(*m2.M.I32)++

*x.M.I32++

*m2.I32--
`,
			want: map[Level]string{
				Green: `
m2.SetI32(m2.GetI32() + 1)
m2.SetI32(m2.GetI32() + 1)
m2.GetM().SetI32(m2.GetM().GetI32() + 1)
m2.GetM().SetI32(m2.GetM().GetI32() + 1)

x.M.SetI32(x.M.GetI32() + 1)

m2.SetI32(m2.GetI32() - 1)
`,
			},
		},

		{
			desc: "unary expressions: proto3",
			in: `
m3.I32++
(m3.I32)++
m3.M.I32++
(m3.M.I32)++
m3.I32--
`,
			want: map[Level]string{
				Green: `
m3.SetI32(m3.GetI32() + 1)
m3.SetI32(m3.GetI32() + 1)
m3.GetM().SetI32(m3.GetM().GetI32() + 1)
m3.GetM().SetI32(m3.GetM().GetI32() + 1)
m3.SetI32(m3.GetI32() - 1)
`,
			},
		},

		{
			desc: "unary expressions: decorations",
			in: `
// hello
*m2.I32++ // world
`,
			want: map[Level]string{
				Green: `
// hello
m2.SetI32(m2.GetI32() + 1) // world
`,
			},
		},

		{
			desc: "unary expressions: no duplicated side-effects",
			extra: `
func f2() *pb2.M2 { return nil }
func f3() *pb3.M3 { return nil }
`,
			in: `
*f2().I32++
(*f2().I32)++
f3().I32++
(f3().I32)++
`,
			want: map[Level]string{
				Green: `
*f2().I32++       /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
(f2().GetI32())++ /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
f3().I32++        /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
(f3().GetI32())++ /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
`,
				Red: `
*f2().I32++       /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
(f2().GetI32())++ /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
f3().I32++        /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
(f3().GetI32())++ /* DO_NOT_SUBMIT: missing rewrite for inc/dec statement */
`,
			},
		}}

	runTableTests(t, tests)
}
