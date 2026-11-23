// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a

type Option[T any] enum {
	None
	Some(T)
}

type Color enum {
	Red
	RGB(uint8, uint8, uint8)
}
