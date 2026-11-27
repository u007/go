// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is not strict: it plays a library inside the package. Its file
// sorts after facts.go, so its bodies are checked first only because the
// strict bodies call them.

package samepkg

import "io"

// Use dereferences r.
func Use(r io.Reader) { r.Read(nil) }

// Exit never returns.
func Exit(msg string) { panic(msg) }

type Impl struct{}

func (Impl) Read([]byte) (int, error) { return 0, nil }

// Reader never returns nil.
func Reader() io.Reader { return Impl{} }

// Maybe returns nil on one path.
func Maybe(c bool) io.Reader {
	if c {
		return nil
	}
	return Impl{}
}

type L struct{}

// Exit never returns.
func (L) Exit() { panic("exit") }

// Pair never returns a nil result 0; MaybePair may.
func Pair() (io.Reader, error) { return Impl{}, nil }

func MaybePair() (io.Reader, error) { return nil, nil }

// Chain returns the result of a helper declared below it.
func Chain() io.Reader { return chained() }

func chained() io.Reader { return Impl{} }

// CycA and CycB call each other and never return nil; CycNil and cycNil may.
func CycA(n int) io.Reader {
	if n > 0 {
		return CycB(n)
	}
	return Impl{}
}

func CycB(n int) io.Reader { return CycA(n - 1) }

func CycNil(n int) io.Reader {
	if n > 0 {
		return cycNil(n)
	}
	return nil
}

func cycNil(n int) io.Reader { return CycNil(n - 1) }

// Drop zeroes the elements of s that it removes; DropVia passes s to it.
func Drop[S ~[]E, E any](s S) S { clear(s[1:]); return s[:1] }

func DropVia[S ~[]E, E any](s S) S { return Drop(s) }

// H has a method that sets its receiver to nil.
type H func()

func (h *H) Clear() { *h = nil }

func GetH() H { return func() {} }
