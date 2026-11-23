// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is shared by the types2 and go/types checkers.

package p

type Fig enum {
	Dot
	Disc(float64)
	Box(float64, float64)
}

type Maybe[T any] enum {
	Nothing
	Just(T)
}

// An exhaustive enum switch is a terminating statement.
func area(f Fig) float64 {
	switch f {
	case Disc(r):
		return 3 * r * r
	case Box(w, h):
		return w * h
	case Dot:
		return 0
	}
}

func several(f Fig) bool {
	switch f {
	case Dot, Disc(_):
		return true
	default:
		return false
	}
}

func generic[T any](m Maybe[T], def T) T {
	switch m {
	case Just(v):
		return v
	case Nothing:
		return def
	}
}

func instance(m Maybe[string]) int {
	switch m {
	case Just(s):
		return len(s)
	case Nothing:
	}
	return 0
}

func labeled(f Fig) {
L:
	switch f {
	case Dot:
		break L
	default:
	}
}

func bad(f Fig, m Maybe[int]) {
	switch f /* ERROR "enum switch on Fig is missing cases Disc, Box (or a default)" */ {
	case Dot:
	}
	switch f {
	case Dot:
	case Dot /* ERROR "duplicate variant Dot in enum switch" */ :
	default:
	}
	switch f {
	case Blob /* ERROR "Blob is not a variant of Fig" */ :
	default:
	}
	switch f {
	case Disc /* ERROR "pattern for variant Disc has 0 payload values, want 1" */ :
	default:
	}
	switch f {
	case Box /* ERROR "pattern for variant Box has 1 payload values, want 2" */ (w):
	default:
	}
	switch f {
	case Dot, Disc(r /* ERROR "cannot bind r in a case with several variants" */):
	default:
	}
	switch f {
	case Disc(1 /* ERROR "invalid enum pattern Disc(1)" */):
	default:
	}
	switch f {
	case Fig_Dot /* ERROR "Fig_Dot is not a variant of Fig" */ ():
	default:
	}
	switch f {
	case Dot:
		fallthrough /* ERROR "cannot fallthrough in enum switch" */
	default:
	}
	switch f {
	case Disc(r /* ERROR "declared and not used" */):
	default:
	}
	switch m {
	case Just(n):
		var _ string = n /* ERROR "cannot use n" */
	case Nothing:
	}
}
