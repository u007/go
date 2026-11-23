//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a

type Node struct{ N int }

type Visitor interface{ Visit(n *Node) }

type Hook func(n *Node)

func New(n int) *Node { return &Node{N: n} }

func (n *Node) Self() *Node { return n }

func Walk(n *Node, v Visitor, h Hook) { v.Visit(n); h(n) }

// Pick is generic and strict: its body is checked, so its result keeps the
// strictness of its type argument.
func Pick[P any](a, b P, first bool) P {
	if first {
		return a
	}
	return b
}
