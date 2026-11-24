// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"math/bits"
)

// This file implements function facts: what the body of a function proves to
// the strict code that calls it. The checker computes the facts of every
// function declared with a body, in every file, once all bodies of the package
// are checked (see computeFacts), and the export data carries them. A missing
// fact means that nothing is known, so a function without a body, or from
// export data without facts, has none.
//
// Strict code uses the facts of imported functions only: the facts of its own
// package are not known while its bodies are checked.

const (
	// factNoReturn means that a call of the function never returns normally:
	// it panics, ends the goroutine or the program, or runs forever.
	factNoReturn uint64 = 1 << iota

	// factResult means that result 0 of the function is never nil. Result i
	// has the bit factResult << i, for i < maxFactResults.
	factResult

	// factParam means that a call of the function panics if parameter 0 is
	// nil: the body dereferences it before any statement that may check,
	// change, or leave it. Parameter i has the bit factParam << i, for
	// i < maxFactParams.
	factParam uint64 = factResult << maxFactResults
)

// maxFactResults and maxFactParams are the numbers of results and parameters
// that facts describe.
const (
	maxFactResults = 31
	maxFactParams  = 32
)

// noReturnSeeds lists, by package path, the functions that never return but
// whose bodies cannot show it: they are written in assembly, or end the
// goroutine through the runtime.
var noReturnSeeds = map[string][]string{
	"runtime": {"Goexit"},
	"syscall": {"Exit"},
}

// FuncFacts returns the facts of the function f, an opaque bit set that
// export data carries (see SetFuncFacts).
func FuncFacts(f *Func) uint64 {
	return f.Origin().facts
}

// FuncDerefs returns, for each parameter that the facts of f say is
// dereferenced, in parameter order, the position of the use that panics if it
// is nil. Export data carries them with the facts (see SetFuncFacts).
func FuncDerefs(f *Func) []syntax.Pos {
	return f.Origin().derefs
}

// SetFuncFacts records the facts of the function f and the positions of its
// dereferences (see FuncDerefs). It is for importers, which read them from the
// export data of a package.
func SetFuncFacts(f *Func, facts uint64, derefs []syntax.Pos) {
	f.facts, f.derefs = facts, derefs
}

// derefPos returns the position where f dereferences parameter j, which its
// facts say it dereferences.
func derefPos(f *Func, j int) syntax.Pos {
	below := FuncFacts(f) & (factParam<<j - factParam)
	return FuncDerefs(f)[bits.OnesCount64(below)]
}

// setDeref records that the function of b dereferences parameter i at pos.
func (b *bodyFacts) setDeref(i int, pos syntax.Pos) {
	fn := b.fn
	bit := factParam << i
	below := fn.facts & (bit - factParam)
	k := bits.OnesCount64(below)
	fn.facts |= bit
	fn.derefs = append(fn.derefs, pos)
	copy(fn.derefs[k+1:], fn.derefs[k:])
	fn.derefs[k] = pos
}

// bodyFacts is what checking the body of a declared function records for its facts.
type bodyFacts struct {
	fn       *Func
	body     *syntax.BlockStmt
	returns  bool                     // the body has a return statement
	defers   bool                     // the body has a defer statement: the deferred call may recover a panic or change the results
	maybeNil uint64                   // results that a return statement may leave nil, as factResult << i
	deps     []factDep                // results that are never nil if results of other functions of the package are
	derefs   map[*syntax.Name]factDep // uses of parameters that panic if they are nil; see factsDeref
	params   []factDep                // parameters dereferenced if parameters of other functions of the package are; see paramFacts
}

// factDep records that fact i of a function holds if fact j of fn, a function
// of the package being checked, holds. In derefs, a nil fn means that the use
// itself panics if parameter i is nil.
type factDep struct {
	i, j int
	fn   *Func
	pos  syntax.Pos // the use, if fn is nil
}

// declareBody prepares the facts of fn, whose body is checked with signature sig.
func (check *Checker) declareBody(fn *Func, sig *Signature, body *syntax.BlockStmt) {
	if check.bodies == nil {
		check.bodies = make(map[*Signature]*bodyFacts)
	}
	check.bodies[sig] = &bodyFacts{fn: fn, body: body}
}

// factsReturn records a return statement of the current function. A bare return
// of named results returns their current values, which may be nil.
func (check *Checker) factsReturn(bare bool) {
	if b := check.bodies[check.sig]; b != nil {
		b.returns = true
		if bare {
			b.maybeNil = ^uint64(0)
		}
	}
}

// factsDefer records a defer statement of the current function.
func (check *Checker) factsDefer() {
	if b := check.bodies[check.sig]; b != nil {
		b.defers = true
	}
}

// factsResult records the value x of the expression e that a return statement
// of the current function returns as result i of type T. If e is a call, x is
// result j of the call.
func (check *Checker) factsResult(i, j int, x *operand, e syntax.Expr, T Type) {
	b := check.bodies[check.sig]
	if b == nil || i >= maxFactResults || check.provenNonNil(x, T) {
		return
	}
	if call, _ := syntax.Unparen(e).(*syntax.CallExpr); call != nil {
		if fn := check.callee(call); fn != nil && j < maxFactResults {
			if fn.pkg == check.pkg {
				b.deps = append(b.deps, factDep{i: i, j: j, fn: fn})
				return
			}
			if FuncFacts(fn)&(factResult<<j) != 0 {
				return
			}
		}
	}
	b.maybeNil |= factResult << i
}

// provenNonNil reports whether the value x, assigned to a variable of type T, is
// proven non-nil: its type is strict, it is a narrowed variable or a call of a
// function whose result is never nil, it is a concrete value boxed in the
// interface type T, or it is an address or a function literal. T may be nil.
func (check *Checker) provenNonNil(x *operand, T Type) bool {
	if !x.isValid() || x.isNil() {
		return false
	}
	if x.nonNull || check.isStrictNonNil(x.typ()) || T != nil && boxes(x.typ(), T) {
		return true
	}
	switch e := syntax.Unparen(x.expr).(type) {
	case *syntax.Operation:
		return e.Op == syntax.And && e.Y == nil
	case *syntax.FuncLit:
		return true
	}
	return false
}

// callee returns the declared function or method that call calls, or nil: for
// a call of a variable, of a function value, or of a method value. A method of
// an interface is returned too, and it has no facts.
func (check *Checker) callee(call *syntax.CallExpr) *Func {
	e := syntax.Unparen(call.Fun)
	if ix, _ := e.(*syntax.IndexExpr); ix != nil {
		e = syntax.Unparen(ix.X) // an instantiation
	}
	var name *syntax.Name
	switch e := e.(type) {
	case *syntax.Name:
		name = e
	case *syntax.SelectorExpr:
		name = e.Sel
	}
	if fn := check.funcUses[name]; fn != nil {
		return fn.Origin()
	}
	return nil
}

// callStmt handles the expression statement x. After a call of panic, or of a
// function that never returns, the point is unreachable. A call statement of a
// declared function body is recorded for computeFacts.
func (check *Checker) callStmt(x syntax.Expr) {
	call, _ := syntax.Unparen(x).(*syntax.CallExpr)
	if call == nil {
		return
	}
	var fn *Func // nil for panic
	if name, _ := syntax.Unparen(call.Fun).(*syntax.Name); name == nil || !check.isPanicName(name) {
		if fn = check.callee(call); fn == nil {
			return
		}
	}
	if fn == nil || FuncFacts(fn)&factNoReturn != 0 {
		check.flowDead()
	}
	if check.bodies[check.sig] != nil {
		if check.stmtCalls == nil {
			check.stmtCalls = make(map[*syntax.CallExpr]*Func)
		}
		check.stmtCalls[call] = fn
	}
}

// paramIndex returns the index of the parameter of the current declared
// function body that e denotes, or -1.
func (check *Checker) paramIndex(e syntax.Expr) int {
	name, _ := syntax.Unparen(e).(*syntax.Name)
	if name == nil || check.sig.params == nil {
		return -1
	}
	for i, v := range check.sig.params.vars {
		if i < maxFactParams && v.name == name.Value && check.lookup(name.Value) == v {
			return i
		}
	}
	return -1
}

// factsDeref records that x, the operand of a use that panics if x is nil (a
// dereference, a field selector through a pointer, a method value of an
// interface, an index of a pointer to an array, or a call), may be a parameter
// of the current declared function body.
func (check *Checker) factsDeref(x *operand) {
	if b := check.bodies[check.sig]; b != nil {
		if i := check.paramIndex(x.expr); i >= 0 {
			b.addDeref(x.expr, factDep{i: i, pos: x.Pos()})
		}
	}
}

func (b *bodyFacts) addDeref(e syntax.Expr, d factDep) {
	if b.derefs == nil {
		b.derefs = make(map[*syntax.Name]factDep)
	}
	b.derefs[syntax.Unparen(e).(*syntax.Name)] = d
}

// factsArgs handles the arguments args of call, a call of fn with signature
// sig. An argument that is a parameter of the current declared function body
// is dereferenced if fn dereferences its parameter. In a strict file, an
// argument that may be nil for a parameter of an imported function that
// dereferences it is a warning: a library function panics on it.
func (check *Checker) factsArgs(call *syntax.CallExpr, fn *Func, sig *Signature, args []*operand) {
	if len(args) != len(call.ArgList) || call.HasDots {
		return // a call f(g()) whose arguments are the results of g, or f(s...)
	}
	b := check.bodies[check.sig]
	strict := check.flow != nil && !check.flow.dead && !check.flow.noFlow && fn.pkg != check.pkg
	if b == nil && !strict {
		return
	}
	params := fn.Signature().params
	off := sig.params.Len() - params.Len() // 1 for a method expression, whose first argument is the receiver
	for k, a := range args {
		j := k - off
		if j < 0 || j >= maxFactParams || sig.variadic && k == sig.params.Len()-1 {
			continue
		}
		bit := FuncFacts(fn) & (factParam << j)
		if b != nil {
			if i := check.paramIndex(a.expr); i >= 0 {
				switch {
				case fn.pkg == check.pkg:
					b.addDeref(a.expr, factDep{i: i, j: j, fn: fn})
				case bit != 0:
					b.addDeref(a.expr, factDep{i: i, pos: derefPos(fn, j)})
				}
			}
		}
		if strict && bit != 0 && a.isValid() && (a.isNil() || isNilable(a.typ()) && !check.provenNonNil(a, sig.params.vars[k].typ)) {
			name := params.vars[j].name
			if name == "" || name == "_" {
				name = check.sprintf("%d", j)
			}
			what := check.sprintf("%s may be nil", a)
			if a.isNil() {
				what = "the argument is nil"
			}
			check.errorf(a, NilWarning, "strict: warning: %s panics if its parameter %s is nil (it dereferences it at %s); %s", fn.name, name, derefPos(fn, j), what)
		}
	}
}

// isPanicName reports whether name denotes the predeclared panic, or is a panic
// generated for an enum switch.
func (check *Checker) isPanicName(name *syntax.Name) bool {
	b, _ := check.lookup(name.Value).(*Builtin)
	return b != nil && b.id == _Panic || check.enumGen[name]
}

// computeFacts computes the facts of the functions declared in the package,
// after all bodies are checked.
func (check *Checker) computeFacts() {
	for _, name := range noReturnSeeds[check.pkg.path] {
		if fn, _ := check.pkg.scope.Lookup(name).(*Func); fn != nil {
			fn.facts |= factNoReturn
		}
	}

	// A body without return and defer statements never returns if it ends in
	// a terminating statement, where a call that never returns terminates.
	// Starting from no such call, iterate to the least fixed point.
	for changed := true; changed; {
		changed = false
		for call, fn := range check.stmtCalls {
			if fn == nil || FuncFacts(fn)&factNoReturn != 0 {
				if check.noReturnCalls == nil {
					check.noReturnCalls = make(map[*syntax.CallExpr]bool)
				}
				check.noReturnCalls[call] = true
			}
		}
		for _, b := range check.bodies {
			if b.fn.facts&factNoReturn == 0 && !b.returns && !b.defers && check.isTerminating(b.body, "") {
				b.fn.facts |= factNoReturn
				changed = true
			}
		}
	}

	// A result is never nil if every return statement returns a value that is
	// proven non-nil, or a result of a function of the package that is never
	// nil. Starting from all such results, iterate to the greatest fixed
	// point: a recursive call returns only what the other returns return.
	for sig, b := range check.bodies {
		if !b.defers {
			n := min(sig.results.Len(), maxFactResults)
			b.fn.facts |= (factResult<<n - factResult) &^ b.maybeNil
		}
	}
	for changed := true; changed; {
		changed = false
		for _, b := range check.bodies {
			for _, d := range b.deps {
				if bit := factResult << d.i; b.fn.facts&bit != 0 && FuncFacts(d.fn)&(factResult<<d.j) == 0 {
					b.fn.facts &^= bit
					changed = true
				}
			}
		}
	}

	// A parameter is dereferenced if the body dereferences it before any
	// statement that may check, change, or leave it, or passes it to a
	// function that dereferences it. Iterate to the least fixed point.
	for _, b := range check.bodies {
		if len(b.derefs) > 0 && !b.defers {
			b.paramFacts()
		}
	}
	for changed := true; changed; {
		changed = false
		for _, b := range check.bodies {
			for _, d := range b.params {
				if bit := factParam << d.i; b.fn.facts&bit == 0 && FuncFacts(d.fn)&(factParam<<d.j) != 0 {
					b.setDeref(d.i, derefPos(d.fn, d.j))
					changed = true
				}
			}
		}
	}
}

// paramFacts sets the parameter facts of b that its own uses prove, and
// records in b.params those that depend on other functions of the package.
// It reads the top-level statements of the body in order. For each parameter,
// the first statement that mentions it decides: the parameter is dereferenced
// if the statement surely evaluates a use recorded in b.derefs, and it is not
// mentioned under & or in a function literal. A statement other than a simple
// statement may check the parameter or leave the function, so it ends the scan
// after its header (init, condition, tag, or range expression).
func (b *bodyFacts) paramFacts() {
	s := paramScan{b: b, names: make(map[string]int)}
	sig := b.fn.Signature()
	for i, v := range sig.params.vars {
		if i < maxFactParams && v.name != "" && v.name != "_" {
			s.names[v.name] = i
			s.pending |= 1 << i
		}
	}
	for _, st := range b.body.List {
		simple := true
		switch st := st.(type) {
		case *syntax.ExprStmt, *syntax.AssignStmt, *syntax.SendStmt, *syntax.DeclStmt, *syntax.EmptyStmt:
			s.visit(st, true)
		case *syntax.IfStmt:
			s.visit(st.Init, true)
			s.visit(st.Cond, true)
			s.visit(st.Then, false)
			s.visit(st.Else, false)
			simple = false
		case *syntax.SwitchStmt:
			s.visit(st.Init, true)
			s.visit(st.Tag, true)
			for _, c := range st.Body {
				s.visit(c, false)
			}
			simple = false
		case *syntax.ForStmt:
			s.visit(st.Init, true)
			s.visit(st.Cond, true)
			s.visit(st.Post, false)
			s.visit(st.Body, false)
			simple = false
		case *syntax.ReturnStmt:
			s.visit(st, true)
			simple = false
		default:
			s.visit(st, false)
			simple = false
		}
		s.decide()
		if !simple || s.pending == 0 {
			return
		}
	}
}

// paramScan is the state of paramFacts.
type paramScan struct {
	b       *bodyFacts
	names   map[string]int // parameter indices by name
	pending uint64         // parameters not decided yet, as 1 << i
	mention uint64         // parameters that the current statement mentions
	escape  uint64         // parameters that it mentions under & or in a function literal
	found   []factDep      // the recorded uses that it surely evaluates
}

// visit scans n, a part of the current statement. If sure is set, the part is
// surely evaluated, but for the right operand of && and || and the bodies of
// function literals.
func (s *paramScan) visit(n syntax.Node, sure bool) {
	if n == nil {
		return
	}
	syntax.Inspect(n, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.Name:
			if i, ok := s.names[n.Value]; ok {
				s.mention |= 1 << i
				if d, ok := s.b.derefs[n]; ok && sure {
					s.found = append(s.found, d)
				}
			}
		case *syntax.FuncLit:
			s.escapes(n.Body)
			return false
		case *syntax.Operation:
			switch {
			case n.Op == syntax.AndAnd || n.Op == syntax.OrOr:
				s.visit(n.X, sure)
				s.visit(n.Y, false)
				return false
			case n.Op == syntax.And && n.Y == nil:
				s.escapes(n.X)
				return false
			}
		}
		return true
	})
}

// escapes records the parameters that n mentions as mentioned under & or in a
// function literal, which may change them at any time.
func (s *paramScan) escapes(n syntax.Node) {
	syntax.Inspect(n, func(n syntax.Node) bool {
		if n, _ := n.(*syntax.Name); n != nil {
			if i, ok := s.names[n.Value]; ok {
				s.mention |= 1 << i
				s.escape |= 1 << i
			}
		}
		return true
	})
}

// decide decides the pending parameters that the current statement mentions.
func (s *paramScan) decide() {
	sure := s.pending & s.mention &^ s.escape
	for _, d := range s.found {
		if sure&(1<<d.i) == 0 {
			continue
		}
		if d.fn == nil {
			if s.b.fn.facts&(factParam<<d.i) == 0 {
				s.b.setDeref(d.i, d.pos)
			}
		} else {
			s.b.params = append(s.b.params, d)
		}
	}
	s.pending &^= s.mention
	s.mention, s.escape, s.found = 0, 0, nil
}
