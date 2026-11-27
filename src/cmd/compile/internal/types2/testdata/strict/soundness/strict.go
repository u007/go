//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Soundness guard: the values below that may be nil on a rare path (see
// lib.go), such as f(), g.Get(), and c.Get(), must be rejected when used
// unchecked. These cases must keep failing as the checker learns facts about
// functions (summaries): "may be nil" stays the default. A wrapper that is
// never nil, such as the results of Closure, Iface, and NewCell, may be
// proven.

package soundness

func take(t *T) {}

func results() {
	take(Mostly /* ERROR "strict: cannot use Mostly(1)" */ (1))
	take(Named /* ERROR "strict: cannot use Named(1)" */ (1))
	take(Deferred /* ERROR "strict: cannot use Deferred()" */ ())
	take(Recovered /* ERROR "strict: cannot use Recovered(1)" */ (1))
	take(Recursive /* ERROR "strict: cannot use Recursive(2)" */ (2))
	take(Global /* ERROR "strict: cannot use Global()" */ ())
	take(Via /* ERROR "strict: cannot use Via()" */ ())
	take(Lookup /* ERROR "strict: cannot use Lookup(\"a\")" */ ("a"))
}

func uses() {
	m := Mostly(1)
	_ = m /* ERROR "strict: m may be nil here" */ .n
	n := Named(1)
	n /* ERROR "strict: n may be nil here" */ .M()
	d := Deferred()
	_ = *d /* ERROR "strict: d may be nil here" */
	r := Recursive(2)
	r /* ERROR "strict: r may be nil here" */ .M()
}

// A function value, a closure result, and an interface method.
func indirect() {
	f := Closure()
	take(f /* ERROR "strict: cannot use f()" */ ()) // Closure never returns nil
	g := Iface(false)
	take(g /* ERROR "strict: cannot use g.Get()" */ .Get()) // Iface never returns nil
	h := Source
	take(h /* ERROR "strict: h may be nil here" */ /* ERROR "strict: cannot use h()" */ ())
}

// A method value bound to a possibly nil receiver.
func methodValue() {
	t := Mostly(1)
	if t == nil {
		return
	}
	s := t.Self
	take(s /* ERROR "strict: cannot use s()" */ ())
	take(t /* ERROR "strict: cannot use t.Self()" */ .Self())
}

// A generic function instantiated with a nullable pointer type.
func generic() {
	take(Generic /* ERROR "strict: cannot use Generic(Mostly(1))" */ (Mostly(1)))
	_ = Generic[*T](nil /* ERROR "strict: cannot use nil as *T value" */)
	take(Zero /* ERROR "strict: cannot use Zero[*T]()" */ [*T]()) // the zero value of a strict pointer type argument is nil
}

// A nil error does not prove the other result: Err(0) returns nil, nil.
func errorResult() {
	t, err := Err(0)
	if err != nil {
		return
	}
	t /* ERROR "strict: t may be nil here" */ .M()
	if t != nil {
		t.M()
	}
}

// A field is not tracked across calls: Close clears it after Init set it.
func field(c *Conn) {
	c.Init()
	c.Close()
	take(c /* ERROR "strict: cannot use c.Buf" */ .Buf)
	b := c.Buf
	b /* ERROR "strict: b may be nil here" */ .M()
}

// Elements, map values, and channel receives of library types may be nil.
func collections() {
	for _, e := range Elems() {
		e /* ERROR "strict: e may be nil here" */ .M()
	}
	v := Lookup("zz")
	v /* ERROR "strict: v may be nil here" */ .M()
	x := <-Async()
	x /* ERROR "strict: x may be nil here" */ .M()
}

// A check of one variable does not prove another that aliases it later.
func aliasing() {
	a := Mostly(1)
	b := a
	if a == nil {
		return
	}
	b /* ERROR "strict: b may be nil here" */ .M()
}

// A narrowing does not survive a call that may reset a package variable.
func shared() {
	s := Shared
	if s == nil {
		return
	}
	s.M() // s is a local copy: a later reset of Shared does not change it
	Shared = nil
	take(Shared /* ERROR "strict: cannot use Shared" */)
}

// A narrowing ends at reassignment, also inside a loop and a closure.
func reassign() {
	t := Mostly(1)
	if t == nil {
		return
	}
	for i := 0; i < 2; i++ {
		t /* ERROR "strict: t may be nil here" */ .M()
		t = Mostly(i)
	}
	u := Mostly(2)
	if u != nil {
		func() { u = Mostly(42) }()
		u /* ERROR "strict: u may be nil here" */ .M()
	}
}

// A generic library function may return the zero value of a type parameter,
// so a strict type argument is nullable in its results, inferred or explicit.
func genericResults(ts []*T) {
	take(First /* ERROR "strict: cannot use First(ts)" */ (ts))
	f := First(ts)
	f /* ERROR "strict: f may be nil here" */ .M()
	if f != nil {
		f.M()
	}
	_ = Apply(&T{}, func(t *T) int { return t.M() }) // no result mentions P
	g := First[*T]           // an instance names a declared function: never nil
	var h func([]*T) *T = g /* ERROR "strict: cannot use g" */
	_ = h
}

// A generic library type: its value comes from a generic constructor, whose
// result holds nullable type arguments.
func genericType() {
	c := NewCell[*T]()
	var d *Cell[*T] = c /* ERROR "strict: cannot use c" */
	_ = d
	take(c /* ERROR "strict: cannot use c.Get()" */ .Get()) // NewCell never returns nil
	_ = Cell /* ERROR "strict: struct literal of type Cell[*T] must set field v" */ [*T]{}
}

// A named strict type has no nullable form: it cannot instantiate a non-strict
// generic function whose result mentions its type parameter.
type NP *T

func namedArg(p NP) {
	_ = Zero /* ERROR "strict: cannot instantiate P with named strict type NP" */ [NP]()
	_ = Apply(p, func(NP) int { return 0 })
}
