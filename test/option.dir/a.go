// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a

// Option, Some, and None are predeclared; this package uses them without
// declaring them.

func Some7() Option[int] { return Some(7) }

func NoneString() Option[string] { return None }

func Ptr(p *int) Option[*int] {
	if p == nil {
		return None
	}
	return Some(p)
}

// Get returns the payload of o and whether o is Some.
func Get[T any](o Option[T]) (T, bool) {
	switch o {
	case Some(v):
		return v, true
	case None:
		var z T
		return z, false
	}
}

// Tag reports the variant of o as an int: 1 for Some, 0 for None.
func Tag[T any](o Option[T]) int {
	switch o {
	case Some(_):
		return 1
	case None:
		return 0
	}
}
