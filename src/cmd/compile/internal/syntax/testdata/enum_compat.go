// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse-only backward-compatibility cases: "enum" must remain an ordinary
// identifier outside the type-specification position that introduces an enum.

package p

// "enum" as a defined type name with a struct underlying type.
type enum struct{ x int }

// A type whose underlying type is the type named "enum".
type Named enum

// "enum" used as a qualified (package) type name.
type Qualified enum.Bar

// "enum" as a function result followed immediately by a function body: the "{"
// starts the body, not an enum variant list.
func returnsEnum() enum { return enum{} }
var returnsEnumVar = func() enum { return enum{} }
