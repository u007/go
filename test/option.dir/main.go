// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"./a"
	"./b"
	"fmt"
)

type Node struct {
	val  int
	next Option[*Node]
}

type Server struct {
	cache Option[*int]
}

func sum(n *Node) int {
	total := 0
	for {
		total += n.val
		switch n.next {
		case Some(next):
			n = next
		case None:
			return total
		}
	}
}

func show(o Option[int]) string {
	switch o {
	case Some(v):
		return fmt.Sprint("some ", v)
	case None:
		return "none"
	}
}

func first[T any](xs []T) Option[T] {
	if len(xs) == 0 {
		return None
	}
	return Some(xs[0])
}

func area(s Option[float64]) (float64, bool) {
	switch o := s; o {
	case Some(r):
		return r * r, true
	case None:
		return 0, false
	}
}

// Option is shadowed by a local type, and Some by a local function.
func shadow() int {
	type Option int
	Some := func(n int) int { return 2 * n }
	var o Option = 4
	return int(o) + Some(1)
}

func must(ok bool, what string) {
	if !ok {
		panic("option: " + what)
	}
}

func main() {
	must(show(Some(3)) == "some 3" && show(None) == "none", "construct and switch")
	must(show(a.Some7()) == "some 7", "imported construct")

	var zero Option[int]
	must(a.Tag(zero) == 0 && show(zero) == "none", "zero value is None")
	var assigned Option[int] = None
	must(show(assigned) == "none", "None takes its target type")

	// Generic instantiation with pointer and string payloads.
	x := 5
	p, ok := a.Get(a.Ptr(&x))
	must(ok && *p == 5, "pointer payload")
	q, ok := a.Get(a.Ptr(nil))
	must(!ok && q == nil, "nil pointer is None")
	s, ok := a.Get(a.NoneString())
	must(!ok && s == "", "string None")
	must(a.Tag(a.Some7()) == 1 && a.Tag(a.NoneString()) == 0, "tag of imported values")

	// Recursive use through a pointer.
	list := &Node{val: 1, next: Some(&Node{val: 2, next: Some(&Node{val: 3})})}
	must(sum(list) == 6, "linked list")

	switch nested := Some(Some(1)); nested {
	case Some(inner):
		must(show(inner) == "some 1", "nested payload")
	case None:
		must(false, "nested is None")
	}

	firstInt := first([]int{9, 8})
	must(show(firstInt) == "some 9" && show(first([]int(nil))) == "none", "generic function")

	var srv Server
	must(a.Tag(srv.cache) == 0, "struct field starts as None")
	srv.cache = Some(&x)
	p2, ok := a.Get(srv.cache)
	must(ok && *p2 == 5, "struct field set")

	r, ok := area(Some(3.0))
	must(ok && r == 9, "switch with init")
	_, ok = area(None)
	must(!ok, "switch with init, None")

	must(shadow() == 6, "shadowing")

	// The package-level names of b shadow nothing here.
	bo := b.Some(5)
	must(bo.Value() == 5 && b.None == -1, "package-level shadow")
	_, ok = any(bo).(Option[int])
	must(!ok, "b.Option is not Option[int]")

	// Runtime type identity of instances across packages.
	_, ok = any(a.Some7()).(Option[int])
	must(ok, "Option[int] from a")
	_, ok = any(a.Ptr(&x)).(Option[*int])
	must(ok, "Option[*int] from a")
	_, ok = any(show).(Option[int])
	must(!ok, "func value is not Option")
}
