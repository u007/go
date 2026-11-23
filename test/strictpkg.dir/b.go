//go:nostrict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a

// Zero is generic but not strict: it may return the zero value of P.
func Zero[P any]() P {
	var z P
	return z
}
