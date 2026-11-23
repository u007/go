// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

type Shape enum {
	Dot
	Circle(float64)
	Box(float64, float64)
}

// The init statement declares a variable used by the clauses.
func initDefine(f func() Shape) float64 {
	switch s := f(); s {
	case Circle(r):
		return r
	case Box(w, h):
		return w * h
	case Dot:
		return 0
	}
}

// The tag is a function literal, which is checked once.
func initFuncLit() float64 {
	switch x := 1.0; (func() Shape { var v Shape; return v })() {
	case Circle(r):
		return r + x
	case Box(w, h):
		return w * h + x
	case Dot:
		return x
	}
}

// An exhaustive switch with an init statement is terminating.
func initTerminating(f func() Shape) int {
	switch s := f(); s {
	case Dot:
		return 0
	default:
		return 1
	}
}

func initMissing(f func() Shape) {
	switch s := f(); s /* ERROR "enum switch on Shape is missing cases Box (or a default)" */ {
	case Dot:
	case Circle(r):
		_ = r
	}
}

// A labeled break leaves the switch, so the function can reach its end.
func initLabeled(f func() Shape) int {
L:
	switch s := f(); s {
	case Dot:
		break L
	case Circle(r):
		return int(r)
	case Box(w, h):
		return int(w * h)
	}
} /* ERROR "missing return" */
