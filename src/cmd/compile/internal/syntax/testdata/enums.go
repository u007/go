// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

// Basic tagged enum with unit and payload variants
type Shape enum {
	Circle(float64)
	Rectangle(float64, float64)
	Point
}

// Generic tagged enum
type Option[T any] enum {
	Some(T)
	None
}

// Generic Result enum
type Result[T, E any] enum {
	Ok(T)
	Err(E)
}

// Enum containing pointer payload (recursive)
type List enum {
	Cons(int, *List)
	Nil
}

// Ensure backward compatibility: "enum" used as identifier
var enum = 123
var _ = enum + 1
func useEnum(enum int) int { return enum }
type myStruct struct {
	enum string
}

// A function whose result type is a type named "enum" must not be mistaken
// for an enum literal: the "{" here starts the function body, not a variant list.
func returnsEnum() enum { return 0 }
var _ = func() enum { return 1 }
