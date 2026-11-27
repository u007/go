// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"iter"
	"maps"
	"math/bits"
	"slices"
)

// This file implements function facts: what the body of a function proves to
// the strict code that calls it. The checker computes the facts of every
// function declared with a body, in every file, and the export data carries
// them. A missing fact means that nothing is known, so a function without a
// body, or from export data without facts, has none.
//
// The facts that a body proves with the facts known so far are set once the
// body is checked (see bodyChecked), so strict code checked later in the
// package uses them. Facts only grow: once all bodies are checked,
// computeFacts completes them for the export data.

const (
	// factNoReturn means that a call of the function never returns normally:
	// it panics, ends the goroutine or the program, or runs forever.
	factNoReturn uint64 = 1 << iota

	// factResult means that result 0 of the function is never nil. Result i
	// has the bit factResult << i, for i < maxFactResults.
	factResult

	// factFuncResult means that result 0 of the function is a function value
	// whose call never calls recover directly (see factNoRecover), or nil.
	// Result i has the bit factFuncResult << i, for i < maxFactResults.
	factFuncResult uint64 = factResult << maxFactResults

	// factParam means that a call of the function does not return normally
	// if parameter 0 is nil: the body dereferences it, which panics, before
	// any statement that may check or change it, or return. Parameter i has
	// the bit factParam << i, for i < maxFactParams.
	factParam uint64 = factFuncResult << maxFactResults

	// factRecv is factParam for the receiver of a method, which the facts
	// treat as parameter maxFactParams.
	factRecv uint64 = factParam << maxFactParams

	// factNoRecover means that the body of the function does not call
	// recover directly, so a deferred call of it never stops a panic. Only
	// the function that a defer statement names can recover the panic of
	// the function that deferred it: a recover in a call that it defers in
	// turn sees no panic (checked at run time), so it does not count.
	factNoRecover uint64 = factRecv << 1

	// factZero means that a call of the function may write zero values into
	// the elements of parameter 0, a slice: the body clears the parameter or
	// a slice of it, or passes one to such a parameter. Parameter i has the
	// bit factZero << i, for i < maxFactParams.
	factZero uint64 = factNoRecover << 1
)

// maxFactResults and maxFactParams are the numbers of results and parameters
// that facts describe.
const (
	maxFactResults = 10
	maxFactParams  = 20
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
	fn           *Func
	body         *syntax.BlockStmt
	returns      bool                       // the body has a return statement
	defers       bool                       // the body has a defer statement: a deferred call may change named results
	recovers     bool                       // the body calls recover directly
	deferRecover bool                       // a deferred call may call recover directly (see factsDefer)
	deferNeeds   []factNeed                 // facts of functions of the package without which a deferred call may recover
	deferVars    []*Var                     // local variables that defer statements call; see fnVar
	resultVars   []resultVar                // local variables that return statements return as results of function type
	loudFn       uint64                     // results that a return statement may leave a function that recovers, as factFuncResult << i
	fnDeps       []fnDep                    // results that are such functions if facts of other functions of the package hold
	maybeNil     uint64                     // results that a return statement may leave nil, as factResult << i
	deps         []factDep                  // results that are never nil if results of other functions of the package are
	derefs       map[*syntax.Name]factDep   // uses of parameters that panic if they are nil; see factsDeref
	params       []factDep                  // parameters dereferenced if parameters of other functions of the package are; see paramFacts
	calls        map[*syntax.CallExpr]*Func // call statements and their callees, nil for panic; see callStmt
	check        func()                     // checks the body
	version      goVersion                  // the language version of the body
	state        bodyState
	zeroes       []factDep // parameters whose elements may be zeroed if those of parameters of other functions of the package are
	deferIface   bool      // the body defers a method of an interface; see ifaceMethod
	scanned      bool      // paramFacts ran; see scanParams
	cyclic       bool      // a body checked for this one calls it back; see checkBody
}

// bodyState is the state of checking a body (see checkBody).
type bodyState uint8

const (
	bodyNew bodyState = iota
	bodyRunning
	bodyDone
)

// factNeed is the fact bit of fn, a function of the package being checked.
type factNeed struct {
	fn  *Func
	bit uint64
}

// resultVar records that a return statement returns v as the result of fact bit.
type resultVar struct {
	bit uint64
	v   *Var
}

// fnDep records that the fact bit of a function holds if need does.
type fnDep struct {
	bit  uint64
	need factNeed
}

// fnVar is what the bindings of a variable of function type prove: a call of
// the variable never calls recover directly if exactly one value is bound to
// it (other than its zero value, whose call panics), the value is proven so
// (see quietFn), and its address is not taken.
type fnVar struct {
	binds int
	loud  bool       // a bound value may recover, or the address is taken
	needs []factNeed // the facts that the proof needs
}

// quietFn reports whether a call of the function value x never calls recover
// directly: x is a function literal that does not mention recover, a declared
// function or method without a recover call, or a result of a call that is
// always such a function. If the proof needs a fact of a function of the
// package, need is that fact.
func (check *Checker) quietFn(x *operand) (ok bool, need factNeed) {
	e := syntax.Unparen(x.expr)
	var fn *Func
	bit := factNoRecover
	if d, ok := check.resultOf[e]; ok {
		fn, bit = d.fn, factFuncResult<<d.j // a result of a call with several results
	} else {
		switch e := e.(type) {
		case *syntax.FuncLit:
			return !mentionsRecover(e.Body), factNeed{}
		case *syntax.CallExpr:
			fn, bit = check.callee(e), factFuncResult
		case *syntax.Name:
			fn = check.funcUses[e]
		case *syntax.SelectorExpr:
			fn = check.funcUses[e.Sel]
		}
	}
	switch {
	case fn == nil:
		return false, factNeed{}
	case fn.pkg == check.pkg:
		return true, factNeed{fn.Origin(), bit}
	}
	return FuncFacts(fn)&bit != 0, factNeed{}
}

// factsBind records that the value x is bound to the variable v.
func (check *Checker) factsBind(v *Var, x *operand) {
	if v == nil || v.typ == nil {
		return
	}
	if _, ok := v.typ.Underlying().(*Signature); !ok {
		return
	}
	f := check.fnVar(v)
	f.binds++
	if ok, need := check.quietFn(x); !ok {
		f.loud = true
	} else if need.fn != nil {
		f.needs = append(f.needs, need)
	}
}

// factsAddr records that the address of the variable that e denotes, if any,
// is taken: the variable may change at any time.
func (check *Checker) factsAddr(e syntax.Expr) {
	if v := check.flowVar(e); v != nil {
		check.fnVar(v).loud = true
	}
}

// localFn returns the local variable that e denotes, if any: its bindings are
// all in the body of the current function. A named result is not one: each
// return statement binds it.
func (check *Checker) localFn(e syntax.Expr) *Var {
	if v := check.flowVar(e); v != nil && v.kind == LocalVar {
		return v
	}
	return nil
}

// quietVar reports whether a call of the variable v never calls recover
// directly, once all its bindings are known (see fnVar), and the facts that
// the proof needs.
func (check *Checker) quietVar(v *Var) ([]factNeed, bool) {
	f := check.fnVars[v]
	if f == nil || f.binds != 1 || f.loud || ptrMethods(v.typ) {
		return nil, false
	}
	return f.needs, true
}

func (check *Checker) fnVar(v *Var) *fnVar {
	f := check.fnVars[v]
	if f == nil {
		if check.fnVars == nil {
			check.fnVars = make(map[*Var]*fnVar)
		}
		f = new(fnVar)
		check.fnVars[v] = f
	}
	return f
}

// factDep records that fact i of a function holds if fact j of fn, a function
// of the package being checked, holds. In derefs, a nil fn means that the use
// itself panics if parameter i is nil.
type factDep struct {
	i, j int
	fn   *Func
	pos  syntax.Pos // the use, if fn is nil
}

// declareBody prepares the facts of fn, whose body is checked with signature
// sig by calling checkFn (see checkBody).
func (check *Checker) declareBody(fn *Func, sig *Signature, body *syntax.BlockStmt, checkFn func()) *bodyFacts {
	if check.bodies == nil {
		check.bodies = make(map[*Signature]*bodyFacts)
		check.methodBodies = make(map[string][]*bodyFacts)
	}
	b := &bodyFacts{fn: fn, body: body, check: checkFn, version: check.version}
	check.bodies[sig] = b
	if sig.recv != nil {
		check.methodBodies[fn.name] = append(check.methodBodies[fn.name], b)
	}
	return b
}

// checkBody checks the body of b, unless it is checked or being checked, and
// sets its facts. In a strict file it first checks the bodies of the functions
// of the package that the body may call, found by name, so that their facts
// are known, and so on for those bodies (deep), whatever their files. In a
// cycle of calls, a body checked first does not know the facts of the others;
// the facts of the cycle are complete when the body that the cycle came back
// to is checked.
func (check *Checker) checkBody(b *bodyFacts, deep bool) {
	if b.state != bodyNew {
		return
	}
	b.state = bodyRunning
	start := len(check.checked)
	if deep || check.inStrictFile(b.body.Pos()) {
		syntax.Inspect(b.body, func(n syntax.Node) bool {
			var callees []*bodyFacts
			switch n := n.(type) {
			case *syntax.Name:
				if fn, _ := check.pkg.scope.Lookup(n.Value).(*Func); fn != nil {
					if c := check.bodies[fn.Signature()]; c != nil {
						callees = []*bodyFacts{c}
					}
				}
			case *syntax.SelectorExpr:
				callees = check.methodBodies[n.Sel.Value]
				delete(check.methodBodies, n.Sel.Value) // each is checked or being checked below
			}
			for _, c := range callees {
				if c.state == bodyNew {
					version := check.version
					check.version = c.version
					check.checkBody(c, true)
					check.version = version
				} else if c.state == bodyRunning && c != b {
					c.cyclic = true
				}
			}
			return true
		})
	}
	b.check()
	check.bodyChecked(b)
	b.state = bodyDone
	check.checked = append(check.checked, b)
	if b.cyclic {
		fixFacts(check, slices.Values(check.checked[start:]))
	}
}

// bodyChecked sets the facts that the checked body of b proves with the facts
// known so far. A function of the package whose body is not checked yet has
// none, so a fact that depends on it waits for computeFacts.
func (check *Checker) bodyChecked(b *bodyFacts) {
	// The bindings of the variables that defer statements call are all known.
	for _, v := range b.deferVars {
		if needs, ok := check.quietVar(v); ok {
			b.deferNeeds = append(b.deferNeeds, needs...)
		} else {
			b.deferRecover = true
		}
	}
	b.deferVars = nil
	for _, r := range b.resultVars {
		needs, ok := check.quietVar(r.v)
		if !ok {
			b.loudFn |= r.bit
		}
		for _, need := range needs {
			b.fnDeps = append(b.fnDeps, fnDep{r.bit, need})
		}
	}
	if !b.recovers {
		b.fn.facts |= factNoRecover
	}
	check.noReturnFact(b)
	b.resultTop()
	for b.resultDeps() {
	}
	b.scanParams()
	for b.paramDeps() || b.zeroDeps() {
	}
}

// scanParams runs paramFacts once, when no deferred call of the body may
// recover. A deferred function of the package whose body is not checked yet
// may, so computeFacts tries again.
func (b *bodyFacts) scanParams() {
	if !b.scanned && len(b.derefs) > 0 && !b.mayRecover() {
		b.scanned = true
		b.paramFacts()
	}
}

// factsReturn records a return statement of the current function. A bare return
// of named results returns their current values, which may be nil.
func (check *Checker) factsReturn(bare bool) {
	if b := check.bodies[check.sig]; b != nil {
		b.returns = true
		if bare {
			b.maybeNil = ^uint64(0)
			b.loudFn = ^uint64(0)
		}
	}
}

// factsDefer records the checked defer statement of the current function
// that defers call. The deferred call may stop a panic if the function it
// calls may call recover directly: a function literal that mentions recover,
// recover itself, a function without the fact factNoRecover, or a function
// value.
func (check *Checker) factsDefer(call syntax.Expr) {
	b := check.bodies[check.sig]
	if b == nil {
		return
	}
	b.defers = true
	c, _ := syntax.Unparen(call).(*syntax.CallExpr)
	if c == nil {
		b.deferRecover = true
		return
	}
	switch fun := syntax.Unparen(c.Fun).(type) {
	case *syntax.FuncLit:
		b.deferRecover = b.deferRecover || mentionsRecover(fun.Body)
		return
	case *syntax.Name:
		if bi, _ := check.lookup(fun.Value).(*Builtin); bi != nil {
			b.deferRecover = b.deferRecover || bi.id == _Recover
			return
		}
	}
	switch fn := check.callee(c); {
	case fn == nil:
		// A call of a local variable never recovers if its one value does not.
		if v := check.localFn(c.Fun); v != nil {
			b.deferVars = append(b.deferVars, v)
		} else {
			b.deferRecover = true
		}
	case ifaceMethod(fn):
		b.deferIface = true
	case fn.pkg == check.pkg:
		b.deferNeeds = append(b.deferNeeds, factNeed{fn, factNoRecover})
	case FuncFacts(fn)&factNoRecover == 0:
		b.deferRecover = true
	}
}

// ifaceMethod reports whether fn is a method of an interface. Its
// implementations are not known: the facts assume that a deferred call of one
// does not call recover directly, which strict code cannot do (see
// strictRecover). The never-returns fact does not assume it (see deferIface):
// strict code after a call that returns would not be checked.
func ifaceMethod(fn *Func) bool {
	recv := fn.Signature().recv
	return recv != nil && IsInterface(recv.typ)
}

// factsRecover records a call of recover in the current function.
func (check *Checker) factsRecover() {
	if b := check.bodies[check.sig]; b != nil {
		b.recovers = true
	}
}

// mentionsRecover reports whether body, outside nested function literals,
// mentions the name recover. A nested literal cannot recover the panic (see
// factNoRecover).
func mentionsRecover(body *syntax.BlockStmt) bool {
	found := false
	syntax.Inspect(body, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.FuncLit:
			return false
		case *syntax.Name:
			found = found || n.Value == "recover"
		}
		return !found
	})
	return found
}

// mayRecover reports whether a deferred call of the body of b may stop a
// panic.
func (b *bodyFacts) mayRecover() bool {
	if b.deferRecover {
		return true
	}
	if len(b.deferVars) > 0 {
		return true // not known before bodyChecked
	}
	for _, n := range b.deferNeeds {
		if FuncFacts(n.fn)&n.bit == 0 {
			return true
		}
	}
	return false
}

// factsResult records the value x of the expression e that a return statement
// of the current function returns as result i of type T. If e is a call, x is
// result j of the call.
func (check *Checker) factsResult(i, j int, x *operand, e syntax.Expr, T Type) {
	b := check.bodies[check.sig]
	if b == nil || i >= maxFactResults {
		return
	}
	if _, ok := T.Underlying().(*Signature); ok {
		if v := check.localFn(e); v != nil {
			b.resultVars = append(b.resultVars, resultVar{factFuncResult << i, v})
		} else if ok, need := check.quietFn(x); !ok {
			b.loudFn |= factFuncResult << i
		} else if need.fn != nil {
			b.fnDeps = append(b.fnDeps, fnDep{factFuncResult << i, need})
		}
	}
	if check.provenNonNil(x, T) {
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
// interface type T, or it is an address, a call of new, or a function
// literal. T may be nil.
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
	case *syntax.CallExpr:
		if name, _ := syntax.Unparen(e.Fun).(*syntax.Name); name != nil {
			b, _ := check.lookup(name.Value).(*Builtin)
			return b != nil && b.id == _New
		}
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
	if b := check.bodies[check.sig]; b != nil {
		if b.calls == nil {
			b.calls = make(map[*syntax.CallExpr]*Func)
		}
		b.calls[call] = fn
	}
}

// paramIndex returns the index of the parameter of the current declared
// function body that e denotes, maxFactParams for its receiver, or -1.
func (check *Checker) paramIndex(e syntax.Expr) int {
	name, _ := syntax.Unparen(e).(*syntax.Name)
	if name == nil {
		return -1
	}
	if v := check.sig.recv; v != nil && v.name == name.Value && check.lookup(name.Value) == v {
		return maxFactParams // the receiver
	}
	if check.sig.params == nil {
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
	name := syntax.Unparen(e).(*syntax.Name)
	if old, ok := b.derefs[name]; ok && old.fn == nil {
		return // the use itself panics
	}
	b.derefs[name] = d
}

// factsArgs handles the arguments args of call, a call of fn with signature
// sig. An argument that is a parameter of the current declared function body
// is dereferenced if fn dereferences its parameter. In a strict file, an
// argument that may be nil for a parameter that fn dereferences is a warning:
// fn panics on it.
func (check *Checker) factsArgs(call *syntax.CallExpr, fn *Func, sig *Signature, args []*operand) {
	if len(args) != len(call.ArgList) || call.HasDots {
		return // a call f(g()) whose arguments are the results of g, or f(s...)
	}
	b := check.bodies[check.sig]
	strict := check.flow != nil && !check.flow.dead && !check.flow.noFlow
	if b == nil && !strict {
		return
	}
	params := fn.Signature().params
	off := sig.params.Len() - params.Len() // 1 for a method expression, whose first argument is the receiver
	if recv := fn.Signature().recv; b != nil && recv != nil && off == 0 && isPointer(recv.typ) {
		// A method call x.m(): m may dereference its receiver x.
		fun := syntax.Unparen(call.Fun)
		if ix, _ := fun.(*syntax.IndexExpr); ix != nil {
			fun = syntax.Unparen(ix.X) // an instantiation
		}
		if sel, _ := fun.(*syntax.SelectorExpr); sel != nil {
			// A value x of another type is not dereferenced: the call takes &x.
			if i := check.paramIndex(sel.X); i >= 0 && isPointer(check.flowVar(sel.X).typ) {
				switch {
				case fn.pkg == check.pkg:
					b.addDeref(sel.X, factDep{i: i, j: maxFactParams, fn: fn})
				case FuncFacts(fn)&factRecv != 0:
					b.addDeref(sel.X, factDep{i: i, pos: derefPos(fn, maxFactParams)})
				}
			}
		}
	}
	for k, a := range args {
		j := k - off
		if j < 0 || j >= maxFactParams || sig.variadic && k == sig.params.Len()-1 {
			continue
		}
		bit := FuncFacts(fn) & (factParam << j)
		if b != nil {
			// A pointer boxed in an interface parameter is not nil there.
			if i := check.paramIndex(a.expr); i >= 0 && a.isValid() && k < sig.params.Len() && !boxes(a.typ(), sig.params.vars[k].typ) {
				switch {
				case fn.pkg == check.pkg:
					b.addDeref(a.expr, factDep{i: i, j: j, fn: fn})
				case bit != 0:
					b.addDeref(a.expr, factDep{i: i, pos: derefPos(fn, j)})
				}
			}
		}
		if b != nil {
			if i := check.paramIndex(sliceRoot(a.expr)); i >= 0 && i < maxFactParams {
				switch {
				case fn.pkg == check.pkg:
					b.zeroes = append(b.zeroes, factDep{i: i, j: j, fn: fn})
				case FuncFacts(fn)&(factZero<<j) != 0:
					b.fn.facts |= factZero << i
				}
			}
		}
		if strict && FuncFacts(fn)&(factZero<<j) != 0 && a.isValid() && check.keepsZeroed(call, a) {
			check.errorf(a, NilWarning, "strict: warning: %s may leave nil elements in %s (it clears elements of its parameter %s); use only its result", fn.name, a.expr, params.vars[j].name)
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

// factsUneval removes the uses recorded in n, an expression that is checked
// but not evaluated: the operand of a constant len or cap, of an unsafe
// function, or of a range over an array without an element variable.
func (check *Checker) factsUneval(n syntax.Node) {
	if b := check.bodies[check.sig]; b != nil {
		syntax.Inspect(n, func(n syntax.Node) bool {
			if name, _ := n.(*syntax.Name); name != nil {
				delete(b.derefs, name)
			}
			return true
		})
	}
}

// sliceRoot returns the operand of the slice expressions of e, or e.
func sliceRoot(e syntax.Expr) syntax.Expr {
	for {
		e = syntax.Unparen(e)
		s, _ := e.(*syntax.SliceExpr)
		if s == nil {
			return e
		}
		e = s.X
	}
}

// factsZero records that the current declared function body clears x. If x is
// a slice of a parameter, a call may zero the elements of the argument.
func (check *Checker) factsZero(x *operand) {
	if b := check.bodies[check.sig]; b != nil && underIs(x.typ(), func(u Type) bool { _, ok := u.(*Slice); return ok }) {
		if i := check.paramIndex(sliceRoot(x.expr)); i >= 0 && i < maxFactParams {
			b.fn.facts |= factZero << i
		}
	}
}

// keepsZeroed reports whether strict code may read a nil element after call
// zeroes elements of its argument a: the elements must not be nil, a is not a
// new value, and the call is not assigned back to a, as in s = f(s).
// shortcut: s = f(s) is accepted although f may zero elements that its result
// keeps; upgrade when a fact says which elements a function zeroes.
func (check *Checker) keepsZeroed(call *syntax.CallExpr, a *operand) bool {
	s, _ := a.typ().Underlying().(*Slice)
	if s == nil || !check.isZeroUnsafe(s.elem) {
		return false
	}
	switch syntax.Unparen(a.expr).(type) {
	case *syntax.CallExpr, *syntax.CompositeLit:
		return false
	}
	return call != check.backCall || !pureRef(a.expr) || check.sprintf("%s", a.expr) != check.sprintf("%s", check.backTo)
}

// pureRef reports whether e is a name, or a selector of one: evaluating it
// twice in one statement denotes the same variable.
func pureRef(e syntax.Expr) bool {
	switch e := syntax.Unparen(e).(type) {
	case *syntax.Name:
		return true
	case *syntax.SelectorExpr:
		return pureRef(e.X)
	}
	return false
}

// zeroDeps sets the zeroed facts of the parameters of the function of b that
// its body passes to a zeroed parameter of a function of the package. It
// reports whether it set one.
func (b *bodyFacts) zeroDeps() bool {
	changed := false
	for _, d := range b.zeroes {
		if bit := factZero << d.i; b.fn.facts&bit == 0 && FuncFacts(d.fn)&(factZero<<d.j) != 0 {
			b.fn.facts |= bit
			changed = true
		}
	}
	return changed
}

// isPanicName reports whether name denotes the predeclared panic, or is a panic
// generated for an enum switch.
func (check *Checker) isPanicName(name *syntax.Name) bool {
	b, _ := check.lookup(name.Value).(*Builtin)
	return b != nil && b.id == _Panic || check.enumGen[name]
}

// computeFacts completes the facts of the functions declared in the package,
// after all bodies are checked (see bodyChecked).
func (check *Checker) computeFacts() {
	for _, name := range noReturnSeeds[check.pkg.path] {
		if fn, _ := check.pkg.scope.Lookup(name).(*Func); fn != nil {
			fn.facts |= factNoReturn
		}
	}

	fixFacts(check, maps.Values(check.bodies))
}

// fixFacts completes the facts of bodies, all checked, with the facts known
// so far of the other functions.
func fixFacts(check *Checker, bodies iter.Seq[*bodyFacts]) {
	// Iterate to the least fixed point.
	for changed := true; changed; {
		changed = false
		for b := range bodies {
			changed = check.noReturnFact(b) || changed
		}
	}

	// Starting from all results whose returns are proven, iterate to the
	// greatest fixed point: a recursive call returns only what the other
	// returns return. It contains the facts set so far.
	for b := range bodies {
		b.resultTop()
	}
	for changed := true; changed; {
		changed = false
		for b := range bodies {
			changed = b.resultDeps() || changed
		}
	}

	// Iterate to the least fixed point.
	for b := range bodies {
		b.scanParams()
	}
	for changed := true; changed; {
		changed = false
		for b := range bodies {
			changed = b.paramDeps() || changed
			changed = b.zeroDeps() || changed
		}
	}
}

// noReturnFact sets the never-returns fact of the function of b if its body,
// without return statements and deferred calls that may recover (see
// mayRecover), ends in a terminating statement, where
// a call that never returns terminates. It reports whether it set the fact.
// Call it only after the body is checked: the body's missing return check
// must not see these calls as terminating.
func (check *Checker) noReturnFact(b *bodyFacts) bool {
	if b.fn.facts&factNoReturn != 0 || b.returns || b.deferIface || b.mayRecover() {
		return false
	}
	for call, fn := range b.calls {
		if fn == nil || FuncFacts(fn)&factNoReturn != 0 {
			if check.noReturnCalls == nil {
				check.noReturnCalls = make(map[*syntax.CallExpr]bool)
			}
			check.noReturnCalls[call] = true
		}
	}
	if !check.isTerminating(b.body, "") {
		return false
	}
	b.fn.facts |= factNoReturn
	return true
}

// resultTop sets the never-nil facts of the results of the function of b that
// every return statement of its body returns proven non-nil, or as a result of
// a function of the package (see resultDeps). A deferred call that may recover
// returns zero results, and one may change named results.
func (b *bodyFacts) resultTop() {
	results := b.fn.Signature().results
	if !b.mayRecover() && !(b.defers && results.Len() > 0 && results.vars[0].name != "") {
		n := min(results.Len(), maxFactResults)
		b.fn.facts |= (factResult<<n - factResult) &^ b.maybeNil
		for i := range n {
			if _, ok := results.vars[i].typ.Underlying().(*Signature); ok {
				b.fn.facts |= factFuncResult << i &^ b.loudFn
			}
		}
	}
}

// resultDeps clears the never-nil facts of the results of the function of b
// that a return statement returns as a result of a function of the package
// that may be nil. It reports whether it cleared one.
func (b *bodyFacts) resultDeps() bool {
	changed := false
	for _, d := range b.deps {
		if bit := factResult << d.i; b.fn.facts&bit != 0 && FuncFacts(d.fn)&(factResult<<d.j) == 0 {
			b.fn.facts &^= bit
			changed = true
		}
	}
	for _, d := range b.fnDeps {
		if b.fn.facts&d.bit != 0 && FuncFacts(d.need.fn)&d.need.bit == 0 {
			b.fn.facts &^= d.bit
			changed = true
		}
	}
	return changed
}

// paramDeps sets the dereferenced facts of the parameters of the function of b
// that its body passes to a dereferenced parameter of a function of the
// package (see paramFacts). It reports whether it set one.
func (b *bodyFacts) paramDeps() bool {
	changed := false
	for _, d := range b.params {
		if bit := factParam << d.i; b.fn.facts&bit == 0 && FuncFacts(d.fn)&(factParam<<d.j) != 0 {
			b.setDeref(d.i, derefPos(d.fn, d.j))
			changed = true
		}
	}
	return changed
}

// paramFacts sets the parameter facts of b that its own uses prove, and
// records in b.params those that depend on other functions of the package.
// It reads the top-level statements of the body in order. For each parameter,
// the first statement that mentions it decides: the parameter is dereferenced
// if the statement surely evaluates a use recorded in b.derefs, and it is not
// mentioned under & or in a function literal. In a statement other than a
// simple statement, only the header (init, condition, tag, or range
// expression) is surely evaluated, and the scan ends after a statement that
// may return or jump (see leaves).
func (b *bodyFacts) paramFacts() {
	s := paramScan{b: b, names: make(map[string]int)}
	sig := b.fn.Signature()
	for i := range sig.Params().Len() {
		if v := sig.params.vars[i]; i < maxFactParams && v.name != "" && v.name != "_" {
			s.names[v.name] = i
			s.pending |= 1 << i
		}
	}
	if v := sig.recv; v != nil && v.name != "" && v.name != "_" {
		s.names[v.name] = maxFactParams
		s.pending |= 1 << maxFactParams
	}
	for _, st := range b.body.List {
		simple := true
		switch st := st.(type) {
		case *syntax.ExprStmt, *syntax.AssignStmt, *syntax.SendStmt, *syntax.DeclStmt, *syntax.EmptyStmt:
			s.visit(st, true)
		case *syntax.CallStmt:
			// A deferred call runs at the end, and a go statement's call in
			// another goroutine: neither decides by a use.
			s.visit(st, false)
		case *syntax.IfStmt:
			s.visit(st.Init, true)
			s.visit(st.Cond, true)
			s.visit(st.Then, false)
			s.visit(st.Else, false)
			simple = !leaves(st)
		case *syntax.SwitchStmt:
			s.visit(st.Init, true)
			s.visit(st.Tag, true)
			for _, c := range st.Body {
				s.visit(c, false)
			}
			simple = !leaves(st)
		case *syntax.ForStmt:
			s.visit(st.Init, true)
			s.visit(st.Cond, true)
			s.visit(st.Post, false)
			s.visit(st.Body, false)
			simple = !leaves(st)
		case *syntax.ReturnStmt:
			s.visit(st, true)
			simple = !leaves(st)
		default:
			s.visit(st, false)
			simple = !leaves(st)
		}
		s.decide()
		if !simple || s.pending == 0 {
			return
		}
	}
}

// leaves reports whether the statement st may leave the function normally, or
// jump: it has a return or goto statement outside function literals. Any other
// statement either ends before the next one or never ends.
func leaves(st syntax.Stmt) bool {
	found := false
	syntax.Inspect(st, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.FuncLit:
			return false
		case *syntax.ReturnStmt:
			found = true
		case *syntax.BranchStmt:
			found = found || n.Tok == syntax.Goto
		}
		return !found
	})
	return found
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
				// Only &p takes the address of p; &p.f and &T{p.f} use it.
				if _, ok := syntax.Unparen(n.X).(*syntax.Name); ok {
					s.escapes(n.X)
					return false
				}
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
