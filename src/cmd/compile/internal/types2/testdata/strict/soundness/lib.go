// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is not strict: it plays a library. Each function below returns a
// value that is non-nil on most paths but nil on one, through a form that a
// summary of the function could misjudge. A strict caller must check every one
// before use; a "never nil" summary for any of them would be unsound.

package soundness

type T struct{ n int }

func (t *T) M() int { return t.n }

// Mostly returns a non-nil value, and nil on a rare path.
func Mostly(n int) *T {
	if n == 42 {
		return nil
	}
	return &T{n: n}
}

// Named sets its named result, then clears it on one path.
func Named(n int) (t *T) {
	t = &T{}
	if n < 0 {
		t = nil
	}
	return
}

// Deferred returns non-nil, but a deferred call clears the named result.
func Deferred() (t *T) {
	defer func() { t = nil }()
	return &T{}
}

// Recovered returns its zero result after a recovered panic.
func Recovered(n int) (t *T) {
	defer func() { recover() }()
	if n == 0 {
		panic("zero")
	}
	return &T{}
}

// Recursive is nil at the base of the recursion only.
func Recursive(n int) *T {
	if n == 0 {
		return nil
	}
	if n == 1 {
		return &T{}
	}
	return Recursive(n - 1)
}

// Global returns a package variable that any package may reset.
var Shared = &T{}

func Global() *T { return Shared }

// Via returns the result of a function value that the caller cannot see.
var Source = func() *T { return &T{} }

func Via() *T { return Source() }

// Closure returns a function whose result is nil on its second call.
func Closure() func() *T {
	calls := 0
	return func() *T {
		calls++
		if calls > 1 {
			return nil
		}
		return &T{}
	}
}

// Generic returns its argument, which may be nil for a pointer type argument.
func Generic[P any](p P) P { return p }

// Getter is implemented by types whose method may return nil.
type Getter interface{ Get() *T }

type Some struct{}

func (Some) Get() *T { return &T{} }

type None struct{}

func (None) Get() *T { return nil }

// Iface returns an interface value; the dynamic method decides.
func Iface(b bool) Getter {
	if b {
		return Some{}
	}
	return None{}
}

// Method has a pointer receiver; its method value may be taken on nil.
func (t *T) Self() *T { return t }

// Err returns a nil error and a non-nil value, or the reverse; and once both nil.
func Err(n int) (*T, error) {
	switch n {
	case 0:
		return nil, nil
	case 1:
		return nil, errBad
	}
	return &T{}, nil
}

type bad struct{}

func (bad) Error() string { return "bad" }

var errBad error = bad{}

// Field holds a pointer that is set in Init and cleared in Close.
type Conn struct{ Buf *T }

func (c *Conn) Init()  { c.Buf = &T{} }
func (c *Conn) Close() { c.Buf = nil }

// Elems returns a slice whose elements may be nil.
func Elems() []*T { return []*T{{}, nil} }

// Lookup returns a map value; a miss is nil.
func Lookup(k string) *T { return map[string]*T{"a": {}}[k] }

// Async publishes its result through a channel; a closed channel yields nil.
func Async() <-chan *T {
	c := make(chan *T)
	close(c)
	return c
}

// Zero returns the zero value of its type argument: nil for a pointer type.
func Zero[P any]() P {
	var z P
	return z
}

// Cell holds a value that is unset until Set: Get then returns the zero value.
type Cell[P any] struct {
	v   P
	set bool
}

func NewCell[P any]() *Cell[P] { return &Cell[P]{} }

func (c *Cell[P]) Get() P  { return c.v }
func (c *Cell[P]) Set(v P) { c.v, c.set = v, true }

// First returns the first element, or the zero value of an empty slice.
func First[P any](s []P) P {
	var z P
	if len(s) > 0 {
		return s[0]
	}
	return z
}

// Apply calls f; its result type mentions no type parameter.
func Apply[P any](p P, f func(P) int) int { return f(p) }
