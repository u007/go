// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

// Option, Some, and None are predeclared.

func construct() {
	var a Option[int] = None
	var b Option[*int] = Some(new(int))
	var c = Some(1.5) // Option[float64]
	var d Option[string] = Some("x")
	var e Option[int] = Some(1)
	var f Option[int] = a
	var g Option[float64] = c
	_, _, _, _, _, _ = a, b, c, d, e, f
	_ = g
}

// Some and None are not declared in this package; the universe names are
// used.

func returnNone() Option[string] {
	return None
}

func argNone(o Option[bool]) {}

func passNone() {
	argNone(None)
}

func nilIsNotOption() {
	var o Option[int] = nil // ERROR "cannot use nil as Option[int] value"
	_ = o
}

func noneIsNotPointer() {
	var p *int = None // ERROR "cannot use None as *int value"
	_ = p
}

func noneNeedsTarget() {
	var x = None // ERROR "use of untyped nil"
	_ = x
}

func someOfNil() {
	_ = Some(nil /* ERROR "use of untyped nil in argument to Some" */)
}

func switchOption(o Option[int]) int {
	switch o {
	case Some(v):
		return v
	case None:
		return 0
	}
	return -1
}

func switchOptionDefault(o Option[int]) int {
	switch o {
	case Some(v):
		return v
	default:
		return 0
	}
}

func switchOptionMissing(o Option[int]) int {
	switch o /* ERROR "missing cases" */ {
	case Some(v):
		return v
	}
	return 0
}

// An alias of the predeclared instance.
type Option2 = Option[int]

var _ Option2 = None
var _ Option2 = Some(1)
