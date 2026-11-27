//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Strict code uses the function facts of its own package: the bodies that a
// strict body may call are checked before it, whatever the order of the
// declarations and files.

package samepkg

import (
	"io"
	"os"
)

// A helper declared below its caller that never returns narrows.
func narrowBelow(name string) {
	f, _ := os.Open(name)
	if f == nil {
		die("no file")
	}
	f.Close()
}

func die(msg string) { panic(msg) }

// So do a function and a method of a non-strict file.
func narrowLib(name string, l L) {
	f, _ := os.Open(name)
	if f == nil {
		Exit("no file")
	}
	f.Close()
	g, _ := os.Open(name)
	if g == nil {
		l.Exit()
	}
	g.Close()
}

// A helper that may return does not narrow.
func noNarrow(name string) {
	f, _ := os.Open(name)
	if f == nil {
		maybeDie(true)
	}
	f /* ERROR "strict: f may be nil here" */ .Close()
}

func maybeDie(c bool) {
	if c {
		panic("die")
	}
}

// A cycle of calls proves nothing: neither function is known to never return.
func noNarrowCycle(name string) {
	f, _ := os.Open(name)
	if f == nil {
		ping()
	}
	f /* ERROR "strict: f may be nil here" */ .Close()
}

func ping() { pong() }
func pong() { ping() }

// A result that is never nil needs no check; one that may be nil warns.
func results() {
	Reader().Read(nil)
	Maybe /* ERROR "strict: warning: Maybe(false) (value of interface type io.Reader) may be nil" */ (false).Read(nil)
	_ = newErr().Error()
}

type myErr struct{}

func (myErr) Error() string { return "err" }

func newErr() error { return myErr{} }

// An argument that may be nil for a dereferenced parameter warns.
func params() {
	Use(nil /* ERROR "strict: warning: Use panics if its parameter r is nil" */)
	Use(Maybe /* ERROR "strict: warning: Use panics if its parameter r is nil" */ (true))
	Use(Reader())
}

// The missing return check is the language's own: a call of a function that
// never returns does not end a body.
func missing() int {
	die("missing")
} /* ERROR "missing return" */

// A result of a call with several results is proven if it is never nil.
func pairs() {
	r, err := Pair()
	r.Read(nil)
	_ = err
	m, _ := MaybePair()
	m /* ERROR "strict: m may be nil here" */ .Read(nil)
	var n io.Reader
	n, err = Pair()
	n.Read(nil)
}

// The facts of a function of a non-strict file that returns a result of
// another function, or of a cycle of calls, are known.
func chains() {
	Chain().Read(nil)
	CycA(1).Read(nil)
	CycB(1).Read(nil)
	CycNil /* ERROR "strict: warning: CycNil(1) (value of interface type io.Reader) may be nil" */ (1).Read(nil)
}

type box struct{ s []*Impl }

// A function that may zero the elements of its argument, which must not be
// nil, warns, unless the argument is a new value or gets the result.
func zeroed(s []*Impl, b box, n []int) {
	s = Drop(s)
	b.s = DropVia(b.s)
	_ = Drop(s /* ERROR "strict: warning: Drop may leave nil elements in s" */)
	t := DropVia(s /* ERROR "strict: warning: DropVia may leave nil elements in s" */)
	s = Drop(t /* ERROR "strict: warning: Drop may leave nil elements in t" */)
	_ = Drop([]*Impl{{}})
	_ = Drop(n)
}

// A variable whose type has a pointer-receiver method is never narrowed: a
// method call takes its address and may set it to nil.
func ptrMethod() {
	h := GetH()
	if h != nil {
		h /* ERROR "strict: h may be nil here" */ .Clear()
		h /* ERROR "strict: h may be nil here" */ ()
	}
}
