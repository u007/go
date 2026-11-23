//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "./a"

type vis struct{}

func (vis) Visit(n *a.Node) {}

func use(n *a.Node) int { return n.N }

func main() {
	n := a.New(3) // a strict result: no nil check is needed
	_ = use(n) + n.N + n.Self().N
	var v a.Visitor = vis{}
	var h a.Hook = func(*a.Node) {}
	a.Walk(n, v, h)

	var p *a.Node = nil         // ERROR "strict: cannot use nil as \*a.Node value"
	var w a.Visitor = nil       // ERROR "strict: cannot use nil as a.Visitor value"
	a.Walk(nil, v, h)           // ERROR "strict: cannot use nil as \*a.Node value"
	_ = use(a.Pick(n, n, true)) // a strict generic: the result is non-nil
	_ = use(a.Zero[*a.Node]())  // ERROR "strict: cannot use a.Zero\[\*a.Node\]\(\)"
	var g a.Hook                // ERROR "declared and not used"
	_, _ = p, w
}
