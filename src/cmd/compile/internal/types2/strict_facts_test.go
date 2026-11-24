// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"strings"
	"testing"
)

// TestFuncFacts checks the facts that the checker computes for the functions
// of a package. A missing fact is sound; a wrong one is not, so each fact has
// cases where it must not hold.
func TestFuncFacts(t *testing.T) {
	const src = `
package p

type T struct{ N int }

type E struct{}

func (*E) Error() string { return "" }

var c bool

func New() *T { return &T{} }
func Wrap() *T { return New() }
func Recurse(n int) *T { if n == 0 { return &T{} }; return Recurse(n - 1) }
func Boxed() error { var p *E; return p }
func Lit() func() { return func() {} }
func (t *T) Fresh() *T { return &T{N: t.N} }
func Pair() (*T, *T) { return &T{}, nil }
func Pair2() (*T, *T) { return Pair() }

func Maybe() *T { if c { return &T{} }; return nil }
func Recovers() *T { defer func() { recover() }(); return &T{} }
func Named() (t *T) { t = &T{}; return }
func (t *T) Self() *T { return t }
func Any[P any](v P) any { return v }
func Mutual() *T { return mutual() }
func mutual() *T { if c { return nil }; return Mutual() }
func Var() *T { f := New; return f() }

func Fail() { panic(0) }
func FailVia() { Fail() }
func Spin() { for {} }
func Block() { select {} }
func Loop() { L: goto L }
func Int() int { panic(0) }
func (t *T) Stop() { FailVia() }

func Deferred() { defer println(); panic(0) }
func Early() { if c { return }; panic(0) }
func Inner() { func() { return }(); panic(0) }
func Lit2() { f := func() { panic(0) }; f() }

type I interface{ M() }

func Star(p *int) int { return *p }
func Field(p *T) { p.N = 1 }
func Iface(i I) { i.M() }
func Call(f func()) { f() }
func Second(n int, p *T) int { n++; return p.N + n }
func Arr(p *[2]int) int { return p[0] }
func Pass(q *T) { Field(q) }
func Cond(p *T) int { if p.N > 0 { return 1 }; return 0 }
func Range(p *T) { for i := 0; i < p.N; i++ {} }
func Over(p *T) { for range p.N {} }

func Checked(p *T) int { if p == nil { return 0 }; return p.N }
func Later(p *T) int { if c { return 0 }; return p.N }
func InLit(p *T) { func() { _ = p.N }() }
func Defer(p *T) { defer println(); _ = p.N }
func AndAnd(p *T) bool { return p != nil && p.N > 0 }
func PtrMethod(p *T) { _ = p.Self() }
func Reassign(p *T) int { p = New(); return p.N }
func Addr(p *T) int { q := &p; _ = q; return p.N }
func Shadow(p *T) { if p := New(); p.N > 0 {} }
func Recur(p *T) { Recur(p) }
`
	f, err := syntax.Parse(syntax.NewFileBase("p.go"), strings.NewReader(src), nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg := NewPackage("p", "p")
	if err := NewChecker(&Config{}, pkg, nil).Files([]*syntax.File{f}); err != nil {
		t.Fatal(err)
	}
	r0 := factResult
	for name, want := range map[string]uint64{
		"New":     r0,
		"Wrap":    r0,
		"Recurse": r0,
		"Boxed":   r0,
		"Lit":     r0,
		"Fresh":   r0,
		"Pair":    r0,
		"Pair2":   r0,

		"Maybe":    0,
		"Recovers": 0,
		"Named":    0,
		"Self":     0,
		"Any":      0,
		"Mutual":   0,
		"mutual":   0,
		"Var":      0,

		"Fail":    factNoReturn,
		"FailVia": factNoReturn,
		"Spin":    factNoReturn,
		"Block":   factNoReturn,
		"Loop":    factNoReturn,
		"Int":     factNoReturn | r0, // vacuous: it never returns
		"Stop":    factNoReturn,

		"Deferred": 0,
		"Early":    0,
		"Inner":    factNoReturn, // the return is the literal's
		"Lit2":     0,
	} {
		obj := pkg.Scope().Lookup(name)
		if obj == nil {
			obj, _, _ = LookupFieldOrMethod(NewPointer(pkg.Scope().Lookup("T").Type()), false, pkg, name)
		}
		if got := FuncFacts(obj.(*Func)); got != want {
			t.Errorf("%s: facts %#b, want %#b", name, got, want)
		}
	}
	p0 := factParam
	for name, want := range map[string]uint64{
		"Star":   p0,
		"Field":  p0,
		"Iface":  p0,
		"Call":   p0,
		"Second": p0 << 1,
		"Arr":    p0,
		"Pass":   p0,
		"Cond":   p0,
		"Range":  p0,
		"Over":   p0,

		"Checked":   0,
		"Later":     0,
		"InLit":     0,
		"Defer":     0,
		"AndAnd":    0,
		"PtrMethod": 0,
		"Reassign":  0,
		"Addr":      0,
		"Shadow":    0,
		"Recur":     0,
	} {
		if got := FuncFacts(pkg.Scope().Lookup(name).(*Func)) &^ (factParam - 1); got != want {
			t.Errorf("%s: parameter facts %#b, want %#b", name, got, want)
		}
	}
	// A dereference through a call is the use in the callee.
	pass := FuncDerefs(pkg.Scope().Lookup("Pass").(*Func))
	field := FuncDerefs(pkg.Scope().Lookup("Field").(*Func))
	if len(pass) != 1 || len(field) != 1 || pass[0] != field[0] {
		t.Errorf("Pass: dereferences %v, want those of Field, %v", pass, field)
	}
}
