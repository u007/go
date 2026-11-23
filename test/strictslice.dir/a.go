// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:strict

package a

var x = 1

// Ptrs returns a slice with len 1 and spare capacity.
func Ptrs() []*int {
	var s []*int
	for len(s) == cap(s) {
		s = append(s, &x)
	}
	return s[:1]
}

func Grow(s []*int) []*int         { return s[:cap(s)] }
func Grow3(s []*int) []*int        { return s[0:cap(s):cap(s)] }
func Shrink(s []*int) []*int       { return s[:len(s)-1] }
func Keep(s []*int) []*int         { return s[:len(s)] }
func GrowGeneric[E any](s []E) []E { return s[:cap(s)] }

// Ints has no zero-unsafe elements, so it may grow into its capacity.
func Ints(s []int) []int { return s[:cap(s)] }
