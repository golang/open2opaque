// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fix

import (
	"bytes"
	"fmt"
	"go/types"
	"reflect"

	"github.com/dave/dst"
)

// dump exists for debugging purposes. It prints information about provided objects. Don't delete it.
func (c *cursor) dump(args ...any) {
	toString := func(n dst.Node) string {
		buf := new(bytes.Buffer)
		err := dst.Fprint(buf, n, func(name string, val reflect.Value) bool {
			return dst.NotNilFilter(name, val) && name != "Obj" && name != "Decs"
		})
		if err != nil {
			buf = bytes.NewBufferString("<can't print the expression>")
		}
		return buf.String()
	}
	defer fmt.Println("------------------------------")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered", r)
		}
	}()
	fmt.Println("==============================")
	for _, a := range args {
		switch a := a.(type) {
		case nil:
			fmt.Println("<nil>")
		case dst.Expr:
			fmt.Print("EXPR: `")
			fmt.Printf("DST TYPE: %T\n", a)
			fmt.Println(toString(a))
			fmt.Println("`")
			fmt.Println("EXPR TYPE: ", c.typeOf(a))
		case *dst.FieldList, *dst.Field:
			fmt.Printf("DST TYPE: %T\n", a)
			fmt.Println("*dst.FieldList and *dst.Field can't be printed")
		case dst.Node:
			fmt.Printf("DST TYPE: %T\n", a)
			if _, ok := a.(*dst.FieldList); ok {
				fmt.Print("*dst.FieldList can't be printed")
			} else {
				fmt.Println(toString(a))
			}
			fmt.Println("")
		case string:
			fmt.Println("label:", a)
		case bool, int:
			fmt.Println("value:", a)
		case types.Type:
			if a == nil {
				fmt.Println("<no type information>")
			} else {
				fmt.Printf("%T\n", a)
			}
		case types.TypeAndValue:
			fmt.Printf("TypeAndValue%+v\n", a)
		case types.Object:
			fmt.Printf("Object: .Package=%s .Name=%s .Id=%s\n", a.Pkg(), a.Name(), a.Id())
		default:
			fmt.Printf("unrecognized argument of type %T\n", a)
		}
	}
}
