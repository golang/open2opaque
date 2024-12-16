// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package profile provides a simple way to gather timing statistics.
package profile

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type profile struct {
	records []record
}

type record struct {
	name string
	time time.Time
}

type key int

const profileKey key = 0

// NewContext returns a new context that has profiling information attached.
func NewContext(parent context.Context) context.Context {
	ctx := context.WithValue(parent, profileKey, &profile{})
	Add(ctx, "start")
	return ctx
}

// Add adds event and event with the given name to the profile attached to the context.
func Add(ctx context.Context, name string) {
	p, ok := ctx.Value(profileKey).(*profile)
	if !ok {
		return
	}
	p.records = append(p.records, record{
		name: name,
		time: time.Now(),
	})
}

// Dump prints the profile attached to the context as string.
func Dump(ctx context.Context) string {
	p, ok := ctx.Value(profileKey).(*profile)
	if !ok {
		return "<no profile>"
	}
	if len(p.records) == 1 {
		return "<empty profile>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "TOTAL: %s | %s", p.records[len(p.records)-1].time.Sub(p.records[0].time), p.records[0].name)
	for i := 1; i < len(p.records); i++ {
		fmt.Fprintf(&b, " %s %s", p.records[i].time.Sub(p.records[i-1].time), p.records[i].name)
	}
	return b.String()
}
