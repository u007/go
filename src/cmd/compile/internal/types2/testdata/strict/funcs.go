//go:strict

package funcs

import (
	"flag"
	"slices"
	"strings"
)

// A function type written in a strict file is strict: nil is not assignable to
// it, and a nullable function value needs a nil check before it is used as
// non-nil. A declared function, a method value, a method expression, and a
// function literal are never nil.

type F func(int) int

type H struct {
	onStart func()
	name    string
}

type Box[T any] struct{ v T }

var nilF func() = nil /* ERROR "strict: cannot use nil as func() value" */

var pkgF /* ERROR "strict: package-level variable pkgF" */ func()

var okF = func() {}

func takeF(f func())    {}
func takeFF(f F)        {}
func takeFfn(f func()) {}

// Nullable: flag.Usage is a variable of a non-strict package, so it may be nil.
func nullable() {
	f := flag.Usage
	takeF(f /* ERROR "strict: cannot use f" */)
	var g func() = f /* ERROR "strict: cannot use f" */
	_ = g
	nilOK := f == nil // comparing with nil is not an assignment
	_ = nilOK
}

// Narrowing by a nil check makes the nullable value usable.
func narrowing() {
	f := flag.Usage
	if f != nil {
		takeF(f)
		var h func() = f
		_ = h
	}
	if f == nil {
		return
	}
	takeF(f)
}

// A narrowing ends when the variable is assigned again.
func narrowingKilled() {
	f := flag.Usage
	if f != nil {
		f = flag.Usage
		takeF(f /* ERROR "strict: cannot use f" */)
	}
}

// A narrowing by && holds on the true branch.
func narrowingAnd(ok bool) {
	f := flag.Usage
	if ok && f != nil {
		takeF(f)
	}
}

// Local strict variables need definite assignment, as strict pointers do.
func locals() {
	var f func()
	f = func() {}
	takeF(f)

	var g func()
	takeF(g /* ERROR "strict: g used before it is definitely assigned" */)
}

// Struct literals must set strict function fields.
func literals() {
	_ = H /* ERROR "strict: struct literal of type H must set field onStart" */ {name: "x"}
	_ = H{onStart: func() {}, name: "x"}
	_ = H{onStart: nil /* ERROR "strict: cannot use nil as func() value" */, name: "x"}
	_ = F(nil /* ERROR "strict: cannot use nil as F value in conversion" */)
}

// Named results of function type are zero values, which a strict file bans.
func namedResult() (f /* ERROR "strict: named result f" */ func()) {
	return
}

func unnamedResult() func() {
	return func() {}
}

// Declared functions, method values, method expressions, and function literals
// are never nil, whichever file declares them.
type buf struct{ n int }

func (b *buf) Reset() { b.n = 0 }

func decl(r rune) rune { return r }

func methods(b *buf) {
	var reset func() = b.Reset
	reset()
	var expr func(*buf) = (*buf).Reset
	expr(b)
	var fd func(rune) rune = decl
	var fs func(string) string = strings.ToUpper
	_, _ = fd, fs
	takeFfn(okF)
}

// A strict function value may be passed to a function of a non-strict package,
// and nil may too: the boundary is not checked there.
func passStd(f func()) {
	_ = strings.Map(nil, "x")
	_ = strings.Map(decl, "x")
	takeFfn(f)
}

// The error type is exempt, as are results of type error.
func errorsExempt() error {
	return nil
}

func errorFuncs() {
	var ef func() error = func() error { return nil }
	_ = ef
	type E func() error
	var e E = func() error { return nil }
	_ = e
}

func errorNamed() (err error) {
	return
}

// Generic instances keep the strictness of their type arguments.
func generic() {
	okBox := Box[func()]{v: okF}
	_ = okBox
	f := flag.Usage
	_ = Box[func()]{v: f /* ERROR "strict: cannot use f" */}
	takeFF(F(func(i int) int { return i }))
}

func zeroT[T any]() T {
	var z T
	return z /* ERROR "strict: z used before it is definitely assigned" */
}

func makeFuncs(n int) {
	_ = make /* ERROR "strict: make" */ ([]func(), n)
	_ = new /* ERROR "strict: new" */ (func())
}

// A generic function from a non-strict package is never nil either.
func instantiated() {
	var idx func([]int, int) int = slices.Index[[]int, int]
	_ = idx
}
