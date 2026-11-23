//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Option[T] is the nullable form of a strict type.

package strict

import "os"

type optCache struct{ n int }

type optServer struct {
	cache Option[*optCache]
	hook  Option[func() int]
}

// The zero value of an Option is None, so no initializer is needed.
var optGlobal Option[*optCache]

func optUse(c *optCache) int { return c.n }

func optFile(f *os.File) {}

func optZero() {
	var s optServer
	var local Option[*optCache]
	_ = optServer{}
	_ = make([]Option[*optCache], 2)
	s.cache = Some(&optCache{n: 7})
	s.hook = Some(func() int { return 2 })
	switch s.cache {
	case Some(c):
		_ = optUse(c) // the payload is a strict pointer
	case None:
	}
	switch s.hook {
	case Some(f):
		_ = f()
	case None:
	}
	local = None
	optGlobal = local
}

func optMap(m map[string]Option[*optCache]) Option[*optCache] {
	return m["k"] // a miss is None
}

func optNil() {
	var o Option[*optCache] = nil /* ERROR "cannot use nil as Option[*optCache] value" */
	_ = o
}

// A nullable pointer does not enter an Option of a strict pointer unchecked.
func optNullable() {
	f, _ := os.Open("x")
	var o Option[*os.File] = Some /* ERROR "strict: cannot use Some(f)" */ (f)
	n := Some(f)
	o = n /* ERROR "strict: cannot use n" */
	switch n {
	case Some(p):
		optFile(p /* ERROR "strict: cannot use p" */)
	case None:
	}
	switch o {
	case Some(p):
		optFile(p)
	case None:
	}
}

// Some of a narrowed variable has a strict payload.
func optNarrowed() Option[*os.File] {
	f, _ := os.Open("x")
	if f == nil {
		return None
	}
	return Some(f)
}
