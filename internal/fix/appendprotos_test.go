// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestAppend(t *testing.T) {
	tt := test{
		in: `
m2.Ms = append(m2.Ms, nil)
m2.Is = append(m2.Is, 1)
m3.Ms = append(m3.Ms, nil)
m3.Is = append(m3.Is, 1)

m3.Is = append(m3.Is, 1, 2, 3)
m3.Is = append(m3.Is, append(m3.Is, 1)...)

// append with a comment
m3.Is = append(m3.Is, 1)

m3.Is = append(m3.Is)
`,
		want: map[Level]string{
			Green: `
m2.SetMs(append(m2.GetMs(), nil))
m2.SetIs(append(m2.GetIs(), 1))
m3.SetMs(append(m3.GetMs(), nil))
m3.SetIs(append(m3.GetIs(), 1))

m3.SetIs(append(m3.GetIs(), 1, 2, 3))
m3.SetIs(append(m3.GetIs(), append(m3.GetIs(), 1)...))

// append with a comment
m3.SetIs(append(m3.GetIs(), 1))

m3.SetIs(append(m3.GetIs()))
`,
		},
	}

	runTableTest(t, tt)
}
