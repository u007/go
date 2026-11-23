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
