// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is shared by the types2 and go/types checkers.

package p

// Basic non-generic enum: unit and payload variants.
type Shape enum {
	Circle(float64)
	Rectangle(float64, float64)
	Point
}

// Generic enum.
type Option[T any] enum {
	Some(T)
	None
}

// Several payload types, including pointers and a type parameter.
type Tree[T any] enum {
	Leaf
	Node(T, *Tree[T])
}

// Duplicate variant name.
type Bad enum {
	A
	A /* ERROR "duplicate enum variant A" */
}

// An enum must have at least one variant.
type Empty enum /* ERROR "enum type must have at least one variant" */ {}

// The struct tag that carries the variants of an enum is reserved.
type Forged struct {
	_tag uint8 `go:enum:"A:0"` /* ERROR "struct tag key go:enum is reserved for enum types" */
}

// Backward compatibility: "enum" remains an ordinary identifier in every
// position outside the enum type-specification position.
type enum struct{ x int }

type Named enum

func returnsEnum() enum { return enum{} }

var returnsEnumVar = func() enum { return enum{} }

// An enum type is declared at package level: its constructors are.
func localEnum() {
	type L enum /* ERROR "enum type must be declared at package level" */ {
		A(int)
		B
	}
}
