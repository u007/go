// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"./a"
)

func panics(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	f()
	return ""
}

func main() {
	const want = "slice bounds out of range"
	for name, f := range map[string]func(){
		"Grow":        func() { a.Grow(a.Ptrs()) },
		"Grow3":       func() { a.Grow3(a.Ptrs()) },
		"GrowGeneric": func() { a.GrowGeneric(a.Ptrs()) },
	} {
		if msg := panics(f); !strings.Contains(msg, want) {
			panic(fmt.Sprintf("%s: got panic %q, want %q", name, msg, want))
		}
	}

	s := a.Ptrs()
	if n := len(a.Keep(s)); n != 1 {
		panic(fmt.Sprintf("Keep: len = %d, want 1", n))
	}
	if c := cap(a.Keep(s)); c != cap(s) {
		panic(fmt.Sprintf("Keep: cap = %d, want %d", c, cap(s)))
	}
	if n := len(a.Shrink(s)); n != 0 {
		panic(fmt.Sprintf("Shrink: len = %d, want 0", n))
	}
	if n := len(a.Ints(make([]int, 1, 4))); n != 4 {
		panic(fmt.Sprintf("Ints: len = %d, want 4", n))
	}
}
