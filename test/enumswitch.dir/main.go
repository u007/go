// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"./a"
	"fmt"
)

type Shape enum {
	Point
	Circle(float64)
	Rect(float64, float64)
}

type Opt[T any] enum {
	None
	Some(T)
}

type Node struct {
	val  int
	next Opt[*Node]
}

func area(s Shape) float64 {
	switch s {
	case Circle(r):
		return 3 * r * r
	case Rect(w, h):
		return w * h
	case Point:
		return 0
	}
}

func kind(s Shape) string {
	switch s {
	case Point, Circle(_):
		return "round"
	default:
		return "other"
	}
}

func sum(n *Node) int {
	total := 0
	for {
		total += n.val
		switch n.next {
		case Some(p):
			n = p
		case None:
			return total
		}
	}
}

func get[T any](o Opt[T], def T) T {
	switch o {
	case Some(v):
		return v
	case None:
		return def
	}
}

var calls int

func mk() Shape { calls++; return Shape_Rect(2, 5) }

func must(ok bool, what string) {
	if !ok {
		panic(what)
	}
}

// An enum switch works on an imported enum.
func unwrap(o a.Option[string]) string {
	switch o {
	case Some(s):
		return s
	case None:
		return ""
	}
}

func red(c a.Color) uint8 {
	switch c {
	case RGB(r, _, _):
		return r
	case Red:
		return 255
	}
}

// A local panic does not capture the check for a corrupt tag.
func shadow(s Shape) int {
	panic := 1
	switch s {
	case Point:
		return panic
	case Circle(_), Rect(_, _):
		return 2
	}
}

// An enum switch may have an init statement.
func initArea() float64 {
	switch s := mk(); s {
	case Rect(w, h):
		return w * h
	case Circle(_), Point:
		return -1
	}
}

func main() {
	calls = 0
	must(initArea() == 10 && calls == 1, "init statement")
	calls = 0

	must(unwrap(a.Option_Some("x")) == "x" && unwrap(a.Option_None[string]()) == "", "imported generic enum")
	must(red(a.Color_RGB(7, 8, 9)) == 7 && red(a.Color_Red()) == 255, "imported enum")
	must(shadow(Shape_Point()) == 1 && shadow(Shape_Circle(1)) == 2, "shadowed panic")

	must(area(Shape_Circle(2)) == 12 && area(Shape_Rect(2, 3)) == 6 && area(Shape_Point()) == 0, "area")
	must(kind(Shape_Point()) == "round" && kind(Shape_Circle(1)) == "round" && kind(Shape_Rect(1, 1)) == "other", "kind")

	list := &Node{val: 1, next: Opt_Some(&Node{val: 2, next: Opt_Some(&Node{val: 3})})}
	must(sum(list) == 6, "sum")
	must(get(Opt_Some(7), 0) == 7 && get(Opt_None[int](), 9) == 9, "get")

	// The tag expression is evaluated once.
	switch mk() {
	case Rect(w, _):
		must(w == 2, "bind")
	default:
		panic("wrong variant")
	}
	must(calls == 1, "tag evaluated once")

	// A value with a corrupt tag reaches no clause of an exhaustive switch.
	defer func() {
		must(fmt.Sprint(recover()) == "invalid enum tag", "corrupt tag")
	}()
	area(Shape{_tag: 9})
}
