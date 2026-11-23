// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

// Package-level declarations shadow the predeclared Option, Some, and None.

type Option int

const None Option = 0

func Some(x int) Option { return Option(x) }

var _ Option = Some(1)
var _ Option = None

// The predeclared generic type is not in scope here.
var _ Option /* ERROR "not a generic type" */ [int]
