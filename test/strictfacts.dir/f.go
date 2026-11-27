//go:nostrict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package f

import (
	"context"
	"os"
	"sync"
)

type T struct{ N int }

type E struct{}

func (*E) Error() string { return "" }

var c bool

// Never nil.
func New() *T  { return &T{} }
func Wrap() *T { return New() }
func Recurse(n int) *T {
	if n == 0 {
		return &T{}
	}
	return Recurse(n - 1)
}
func Boxed() error     { var p *E; return p }
func Lit() func()      { return func() {} }
func (t *T) Fresh() *T { return &T{N: t.N} }
func Pair() (*T, *T)   { return &T{}, nil }

// May be nil.
func Maybe() *T {
	if c {
		return &T{}
	}
	return nil
}
func Recovers() *T       { defer func() { recover() }(); return &T{} }
func Named() (t *T)      { t = &T{}; return }
func (t *T) Self() *T    { return t }
func Any[P any](v P) any { return v }
func Mutual() *T         { return mutual() }
func mutual() *T {
	if c {
		return nil
	}
	return Mutual()
}

// Never return.
func Fail()    { panic(0) }
func FailVia() { Fail() }
func Exit()    { os.Exit(1) }
func Spin() {
	for {
	}
}
func Block()       { select {} }
func (t *T) Stop() { FailVia() }

// Dereference a parameter.
func Use(p *T) int           { return p.N }
func UseVia(n int, p *T) int { return Use(p) + n }
func (t *T) Take(p *T) int   { return p.N + t.N }
func Call(fn func())         { fn() }

// May not dereference.
func Guarded(p *T) int {
	if p == nil {
		return 0
	}
	return p.N
}

// May return.
func Deferred() { defer Rescue(); panic(0) } // Rescue may stop the panic
func Early() {
	if c {
		return
	}
	panic(0)
}

var mu sync.Mutex

// Never nil: the deferred Unlock, of package sync, never calls recover.
func Locked() *T {
	mu.Lock()
	defer mu.Unlock()
	return &T{}
}

// Rescue calls recover, so a function that defers it may return nil.
func Rescue() { recover() }

func Rescued() *T {
	defer Rescue()
	return &T{}
}

// Generic methods have facts too.
type B struct{}

func (B) Make[P any](v P) *T { return &T{} }
func (B) Fail[P any](v P)    { panic(v) }
func (B) Maybe[P any](v P) *T {
	if c {
		return nil
	}
	return &T{}
}

// Dereference through a method, after statements that cannot return.
func (t *T) Get() int { return t.N }
func UseLate(p *T) int {
	if c {
		println()
	}
	return p.Get()
}

// Never nil: the deferred cancel, a result of context.WithCancel, is a
// function that never calls recover.
func Cancelled() *T {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	return &T{}
}
