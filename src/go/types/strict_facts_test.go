// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math/bits"
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
func NewB() *T { return new(T) }
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
func DeferLit() { defer func() { recover() }(); panic(0) }
func DeferVia() { defer rec(); panic(0) }
func DeferVar(f func()) { defer f(); panic(0) }
func DeferIface(i I) { defer i.M(); panic(0) }
func DeferIfaceVal(i I) { f := i.M; defer f(); panic(0) }
func DeferIfaceField(s struct{ I }) *T { defer s.M(); return &T{} }
func DeferDone() { defer done(); panic(0) }
func DeferPlain() *T { defer done(); return &T{} }
func DeferNamed() (t *T) { defer done(); return &T{} }
func DeferRec() *T { defer rec(); return &T{} }
func rec() { recover() }
func Quiet() func() { return func() {} }
func QuietFn() func() { return done }
func QuietVia() (int, func()) { return 0, Quiet() }
func QuietPair() (int, func()) { return QuietVia() }
func Loud() func() { return func() { recover() } }
func LoudFn() func() { return rec }
func LoudVar(f func()) func() { return f }
func LoudVia() func() { return Loud() }
func QuietRet() func() { f := Quiet(); return f }
func QuietRetLit() (int, func()) { f := func() {}; return 0, f }
func LoudRetNamed() (f func()) { f = done; return f }
func LoudRet() func() { f := Loud(); return f }
func LoudRetTwice() func() { f := Quiet(); if c { f = Loud() }; return f }
func LoudRetParam(f func()) func() { if c { f = Quiet() }; return f }
func LoudRetSwitch(x any) func() { switch f := x.(type) { case func(): if c { f = Quiet() }; return f }; return nil }
func DeferParam(f func()) { if c { f = Quiet() }; defer f(); panic(0) }
func DeferMap(m map[int]func()) { f := Quiet(); f, _ = m[0]; defer f(); panic(0) }
func DeferQuiet() { f := Quiet(); defer f(); panic(0) }
func DeferPair() { _, f := QuietPair(); defer f(); panic(0) }
func DeferLitVar() { f := func() {}; defer f(); panic(0) }
func DeferLoud() { f := Loud(); defer f(); panic(0) }
func DeferTwice() { f := Quiet(); if c { f = Loud() }; defer f(); panic(0) }
func DeferAddr() { f := Quiet(); g := &f; *g = Loud(); defer f(); panic(0) }
func DeferClosure() { f := Quiet(); func() { f = Loud() }(); defer f(); panic(0) }
func DeferRange(fs []func()) { for _, f := range fs { defer f() }; panic(0) }
func done() {}

type I interface{ M() }

func Star(p *int) int { return *p }
func Field(p *T) { p.N = 1 }
func Iface(i I) { i.M() }
func Call(f func()) { f() }
func Second(n int, p *T) int { n++; return p.N + n }
func Arr(p *[2]int) int { return p[0] }
func Pass(q *T) { Field(q) }
func DeferLate(p *T) { defer late(); p.N = 1 }
func DeferLateRec(p *T) { defer lateRec(); p.N = 1 }
func late() {}
func lateRec() { recover() }
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
func (t *T) Get() int { return t.N }
func (t T) Val() int { return t.N }
func ViaPtr(p *T) int { return p.Get() }
func ViaVal(p *T) int { return p.Val() }
func ViaSelf(p *T) { _ = p.Self().N }
func After(p *T) int { if c { println() }; for range 3 {}; return p.N }
func AfterGoto(p *T) int { if c { goto L }; return p.N; L: return 0 }
func AfterPanic(p *T) int { if c { panic(0) }; return p.N }
func AfterLit(p *T) int { if c { func() { return }() }; return p.N }
func AfterUse(p *T) int { if c { p = nil }; return p.N }
func DeferP(p *T) { defer rec(); _ = p.N }
func DeferArg(p *T) { defer Field(p); if p == nil { return } }
func DeferAfter(p *T) { defer done(); _ = p.N }

type B struct{ arr [4]int }
func (*B) M() {}
type H func()
func (h *H) Clear() { *h = nil }
func BoxPass(p *B) { Iface(p) }
func LenArr(p *B) int { return len(p.arr) }
func RangeArr(p *B) { for i := range p.arr { _ = i } }
func RangeArrVal(p *B) { for _, v := range p.arr { _ = v } }
func RangePtr(p *[2]int) { for range p {} }
func ValRecv(x H) { x.Clear() }
func DeferIfaceRes(i I) *T { defer i.M(); return &T{} }
func Zero(s []*T) { clear(s) }
func ZeroTail(s []*T, n int) []*T { s = s[:n]; clear(s[n:cap(s)]); return s }
func ZeroSecond(n int, s []*T) { clear((s[n:])) }
func ZeroVia(s []*T) { Zero(s[1:]) }
func ZeroGen[S ~[]E, E any](s S) { clear(s) }
func ZeroCycA(s []*T) { ZeroCycB(s) }
func ZeroCycB(s []*T) { if c { ZeroCycA(s) }; clear(s) }
func ZeroRec(s []*T) { if c { ZeroRec(s) } }
func ZeroMap(m map[int]*T) { clear(m) }
func ZeroLocal(s []*T) { t := make([]*T, 1); clear(t) }
func ZeroShadow(s []*T) { { s := []*T{}; clear(s) } }
func ZeroElem(s [][]*T) { clear(s[0]) }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg := NewPackage("p", "p")
	if err := NewChecker(&Config{}, fset, pkg, nil).Files([]*ast.File{f}); err != nil {
		t.Fatal(err)
	}
	r0 := factResult
	for name, want := range map[string]uint64{
		"New":     r0,
		"NewB":    r0,
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

		"Deferred": factNoReturn, // println does not recover
		"Early":    0,
		"Inner":    factNoReturn, // the return is the literal's
		"Lit2":     0,

		"DeferLit":        0,
		"DeferVia":        0,
		"DeferVar":        0,
		"DeferIface":      0, // an implementation may recover
		"DeferIfaceRes":   factResult,
		"DeferIfaceVal":   0,
		"DeferIfaceField": factResult,
		"DeferDone":       factNoReturn,
		"DeferPlain":      r0,
		"DeferNamed":      0, // a deferred call may change a named result
		"DeferRec":        0,

		"DeferQuiet":   factNoReturn,
		"DeferPair":    factNoReturn,
		"DeferLitVar":  factNoReturn,
		"DeferLoud":    0,
		"DeferParam":   0,
		"DeferMap":     0,
		"DeferTwice":   0,
		"DeferAddr":    0,
		"DeferClosure": 0,
		"DeferRange":   0,
	} {
		obj := pkg.Scope().Lookup(name)
		if obj == nil {
			obj, _, _ = LookupFieldOrMethod(NewPointer(pkg.Scope().Lookup("T").Type()), false, pkg, name)
		}
		if got := FuncFacts(obj.(*Func)) &^ (factNoRecover | factRecv | (factParam - factFuncResult)); got != want {
			t.Errorf("%s: facts %#b, want %#b", name, got, want)
		}
	}
	p0 := factParam
	for name, want := range map[string]uint64{
		"Star":         p0,
		"Field":        p0,
		"DeferLate":    p0,
		"DeferLateRec": 0,
		"Iface":        p0,
		"Call":         p0,
		"Second":       p0 << 1,
		"Arr":          p0,
		"Pass":         p0,
		"Cond":         p0,
		"Range":        p0,
		"Over":         p0,

		"BoxPass": 0, "LenArr": 0, "RangeArr": 0, "RangeArrVal": p0, "RangePtr": 0, "ValRecv": 0,
		"Checked":    0,
		"Later":      0,
		"InLit":      0,
		"Defer":      p0, // println does not recover
		"AndAnd":     0,
		"PtrMethod":  0,
		"Reassign":   0,
		"Addr":       0,
		"Shadow":     0,
		"Recur":      0,
		"DeferP":     0,
		"DeferArg":   0,
		"DeferAfter": p0,
		"ViaPtr":     p0,
		"ViaVal":     p0,
		"ViaSelf":    0,
		"After":      p0,
		"AfterGoto":  0,
		"AfterPanic": p0,
		"AfterLit":   p0,
		"AfterUse":   0,
	} {
		if got := FuncFacts(pkg.Scope().Lookup(name).(*Func)) & (factRecv - factParam); got != want {
			t.Errorf("%s: parameter facts %#b, want %#b", name, got, want)
		}
	}
	// A dereference through a call is the use in the callee.
	pass := FuncDerefs(pkg.Scope().Lookup("Pass").(*Func))
	field := FuncDerefs(pkg.Scope().Lookup("Field").(*Func))
	if len(pass) != 1 || len(field) != 1 || pass[0] != field[0] {
		t.Errorf("Pass: dereferences %v, want those of Field, %v", pass, field)
	}
	// A result that is always a function that does not recover.
	f0 := factFuncResult
	for name, want := range map[string]uint64{
		"Quiet": f0, "QuietFn": f0, "QuietVia": f0 << 1, "QuietPair": f0 << 1, "Lit": f0,
		"Loud": 0, "LoudFn": 0, "LoudVar": 0, "LoudVia": 0,
		"QuietRet": f0, "QuietRetLit": f0 << 1,
		"LoudRet": 0, "LoudRetTwice": 0, "LoudRetParam": 0, "LoudRetSwitch": 0, "LoudRetNamed": 0,
	} {
		if got := FuncFacts(pkg.Scope().Lookup(name).(*Func)) & (factParam - factFuncResult); got != want {
			t.Errorf("%s: function result facts %#b, want %#b", name, got, want)
		}
	}
	// A method that dereferences its receiver has factRecv.
	for name, want := range map[string]uint64{"Get": factRecv, "Fresh": factRecv, "Self": 0, "Val": 0} {
		obj, _, _ := LookupFieldOrMethod(NewPointer(pkg.Scope().Lookup("T").Type()), false, pkg, name)
		if got := FuncFacts(obj.(*Func)) & factRecv; got != want {
			t.Errorf("%s: receiver fact %#b, want %#b", name, got, want)
		}
	}
	// A body that calls recover directly has no factNoRecover.
	for name, want := range map[string]bool{"rec": false, "done": true, "Recovers": true} {
		if got := FuncFacts(pkg.Scope().Lookup(name).(*Func))&factNoRecover != 0; got != want {
			t.Errorf("%s: no recover %v, want %v", name, got, want)
		}
	}
	// A parameter whose elements the body may zero.
	z0 := factZero
	for name, want := range map[string]uint64{
		"Zero": z0, "ZeroTail": z0, "ZeroSecond": z0 << 1, "ZeroVia": z0, "ZeroGen": z0, "ZeroCycA": z0, "ZeroCycB": z0,
		"ZeroRec": 0, "ZeroMap": 0, "ZeroLocal": 0, "ZeroShadow": 0, "ZeroElem": 0,
	} {
		if got := FuncFacts(pkg.Scope().Lookup(name).(*Func)) &^ (factZero - 1); got != want {
			t.Errorf("%s: zeroed facts %#b, want %#b", name, got, want)
		}
	}
	// Each dereferenced parameter has one position, also when a fact is set
	// once its body is checked and again when all bodies are.
	for _, name := range pkg.Scope().Names() {
		if fn, _ := pkg.Scope().Lookup(name).(*Func); fn != nil {
			if n, want := len(FuncDerefs(fn)), bits.OnesCount64(FuncFacts(fn)&(factNoRecover-factParam)); n != want {
				t.Errorf("%s: %d dereference positions, want %d", name, n, want)
			}
		}
	}
}
