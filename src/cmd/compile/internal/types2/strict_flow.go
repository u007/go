// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// This file implements flow facts for strict code: definite assignment of
// local variables. A fact is a property of a local *Var at one program point
// of a function body. The statement checker forks the state at each branch,
// checks the branch from the fork's entry state, and joins the surviving exits
// with bitwise AND (see flowForkBegin). A dead state (after return, panic, or a
// branch statement) is the identity of the join, so a terminating branch drops
// out of it. Loops never contribute the facts of their body (see flowLoopExit).
//
// Only strict files track facts. In other files check.flow is nil and every
// function in this file does nothing.

// flowBits is the fact set of one tracked variable.
type flowBits uint8

const (
	// flowAssigned means that the variable is definitely assigned on every path
	// to the current point.
	flowAssigned flowBits = 1 << iota

	// flowNonNil means that a nullable pointer variable is proven non-nil on
	// every path to the current point, by narrowing (see flowRefine).
	flowNonNil
)

// flowState is the flow state at one program point of a strict function body.
// An entry in facts means that the variable has facts: a tracked variable (a
// local of zero-unsafe type declared without an initializer, which is tracked
// unassigned), or a narrowed variable. Other variables are assigned from their
// declaration and need no entry.
type flowState struct {
	facts    map[*Var]flowBits     // variables with facts
	captured map[*Var]bool         // variables declared outside this function body; shared, never changed
	writes   map[*syntax.Name]bool // identifiers that are assignment targets, not reads; shared
	excluded map[syntax.Pos]bool   // variables never narrowed, by declaration position: under &, or mentioned in a literal and not bound once; shared
	noFlow   bool                  // the body contains goto, so no variable is tracked or narrowed; shared
	dead     bool                  // the point is unreachable; a dead state is the identity of join
}

// clone returns a copy of s that can be changed without affecting s. Clone of
// nil is nil.
func (s *flowState) clone() *flowState {
	if s == nil {
		return nil
	}
	c := *s
	c.facts = make(map[*Var]flowBits, len(s.facts))
	for v, bits := range s.facts {
		c.facts[v] = bits
	}
	return &c
}

// joinFlow returns the state that holds at the merge of a and b: a fact holds
// if it holds in both. A dead state holds nowhere, so it is skipped. A tracked
// variable that is missing from one side was declared in that branch, and it
// goes out of scope at the merge, so it is dropped.
func joinFlow(a, b *flowState) *flowState {
	if a.dead {
		return b.clone()
	}
	if b.dead {
		return a.clone()
	}
	j := a.clone()
	for v, bits := range j.facts {
		if other, ok := b.facts[v]; ok {
			j.facts[v] = bits & other
		} else {
			delete(j.facts, v)
		}
	}
	return j
}

// flowFork is a multi-way branch: each branch is checked from entry, and
// joined accumulates the exits of the branches that reach the merge point.
type flowFork struct {
	entry  *flowState
	joined *flowState
}

// flowForkBegin starts a fork at the current point. Each branch is checked
// between flowBranchBegin and flowBranchEnd, and the fork is closed by
// flowForkEnd. A fork with no branches ends dead, so a select with no clauses
// does not fall through.
func (check *Checker) flowForkBegin() *flowFork {
	if check.flow == nil {
		return nil
	}
	entry := check.flow.clone()
	joined := entry.clone()
	joined.dead = true
	return &flowFork{entry: entry, joined: joined}
}

// flowBranchBegin makes the entry state current for the next branch.
func (check *Checker) flowBranchBegin(f *flowFork) {
	if f != nil {
		check.flow = f.entry.clone()
	}
}

// flowBranchEnd adds the current state, the exit of the branch, to the join.
// A branch that leaves the enclosing switch or select by break contributes the
// entry state instead, so the facts it assigned do not count after the statement.
// The break may follow an assignment, so that state loses the narrowing of every
// name that clause, the branch, assigns.
// An absent branch (an if without else, or a switch without default) is a
// branch with no statements: flowBranchBegin and flowBranchEnd with no body.
func (check *Checker) flowBranchEnd(f *flowFork, clause syntax.Node, leavesByBreak bool) {
	if f == nil {
		return
	}
	out := check.flow
	if leavesByBreak {
		out = f.entry.clone()
		out.dropAssigned(clause)
	}
	f.joined = joinFlow(f.joined, out)
}

// flowForkEnd makes the join of the branches the current state.
func (check *Checker) flowForkEnd(f *flowFork) {
	if f != nil {
		check.flow = f.joined
	}
}

// flowLoopEnter returns the state at the loop's exit and at the start of its
// condition, post statement, and body: the state after the init statement. The
// post statement and the body cannot add facts that hold after the loop, because
// the body may run zero times. A narrowing of a name that the loop assigns is
// dropped, since a later iteration sees the assigned value.
func (check *Checker) flowLoopEnter(loop syntax.Stmt) *flowState {
	st := check.flow.clone()
	if st != nil {
		st.dropAssigned(loop)
	}
	return st
}

// dropAssigned removes the narrowing of every name that n assigns.
func (s *flowState) dropAssigned(n syntax.Node) {
	names := assignedNames(n)
	for v, bits := range s.facts {
		if names[v.name] {
			s.facts[v] = bits &^ flowNonNil
		}
	}
}

// flowLoopBody makes the loop entry state current for checking the body.
func (check *Checker) flowLoopBody(entry *flowState) {
	check.flow = entry.clone()
}

// flowLoopExit makes the state after the loop current. An infinite loop with no
// break never exits, so the state after it is dead.
func (check *Checker) flowLoopExit(entry *flowState, infinite bool) {
	check.flow = entry.clone()
	if check.flow != nil {
		check.flow.dead = infinite
	}
}

// flowDead marks the current point unreachable. It follows return, branch
// statements, and calls to panic.
func (check *Checker) flowDead() {
	if check.flow != nil {
		check.flow.dead = true
	}
}

// flowRef checks a use of the local variable v by the identifier e. A read of a
// tracked variable that is not definitely assigned is an error. So is a write
// through a closure that captures such a variable. A write to a local in its
// own function is not a read, so it is allowed.
func (check *Checker) flowRef(e *syntax.Name, v *Var) {
	s := check.flow
	if s == nil || s.dead {
		return
	}
	bits, ok := s.facts[v]
	if !ok || bits&flowAssigned != 0 {
		return
	}
	if s.writes[e] && !s.captured[v] {
		return
	}
	check.errorf(e, UnassignedVar, "strict: %s used before it is definitely assigned", v.name)
}

// flowWrites marks the identifiers among lhs as assignment targets, so that
// checking them is not a read of the variable. Call it before checking the
// left-hand sides of an assignment.
func (check *Checker) flowWrites(lhs ...syntax.Expr) {
	s := check.flow
	if s == nil {
		return
	}
	for _, x := range lhs {
		if name, _ := syntax.Unparen(x).(*syntax.Name); name != nil {
			s.writes[name] = true
		}
	}
}

// flowAssign records that each identifier among lhs, a whole-value target, has
// been assigned. Call it after the right-hand side is checked, so that reads on
// the right-hand side see the old state.
func (check *Checker) flowAssign(lhs ...syntax.Expr) {
	s := check.flow
	if s == nil {
		return
	}
	for _, x := range lhs {
		name, _ := syntax.Unparen(x).(*syntax.Name)
		if name == nil || name.Value == "_" {
			continue
		}
		v, _ := check.lookup(name.Value).(*Var)
		if v == nil {
			continue
		}
		bits, ok := s.facts[v]
		switch {
		case check.flowProofs[v] && !s.noFlow && check.flowNarrowable(s, v):
			s.facts[v] = bits | flowAssigned | flowNonNil // the assigned value is proven non-nil
		case ok:
			// Assignment kills a narrowing: the new value may be nil.
			s.facts[v] = (bits | flowAssigned) &^ flowNonNil
		}
	}
}

// flowSetNonNil records that the newly declared variable v is proven non-nil.
func (check *Checker) flowSetNonNil(v *Var) {
	if s := check.flow; s != nil && !s.noFlow && check.flowNarrowable(s, v) {
		s.facts[v] = flowAssigned | flowNonNil
	}
}

// flowVar returns the variable that the expression e names, or nil.
func (check *Checker) flowVar(e syntax.Expr) *Var {
	name, _ := syntax.Unparen(e).(*syntax.Name)
	if name == nil {
		return nil
	}
	v, _ := check.lookup(name.Value).(*Var)
	return v
}

// flowProve records whether the value x, about to be assigned to v, is proven
// non-nil: its type is strict, it is a narrowed variable, or it is a concrete
// value boxed in the interface type of v. flowAssign and flowDeclare read the
// record of the assignment they follow.
func (check *Checker) flowProve(v *Var, x *operand) {
	if check.flow == nil || v == nil {
		return
	}
	if check.flowProofs == nil {
		check.flowProofs = make(map[*Var]bool)
	}
	check.flowProofs[v] = check.provenNonNil(x, v.typ)
}

// strictUse reports a use of x that needs a non-nil value (a dereference, a
// selector, an index, or a call) if x has a nullable type and is not proven
// non-nil here. For a local variable or parameter it is an error. Any other
// expression is not tracked, so it is a warning (NilWarning). It reports
// whether it reported an error.
func (check *Checker) strictUse(x *operand) bool {
	s := check.flow
	if s == nil || s.dead || s.noFlow || x.nonNull || !x.isValid() {
		return false
	}
	if !isNilable(x.typ()) || check.isStrictNonNil(x.typ()) {
		return false
	}
	v := check.flowVar(x.expr)
	if v == nil || v.pkg != check.pkg || v.Parent() == nil || v.Parent() == v.pkg.Scope() {
		// Not a tracked variable: a field, a package variable, or a call
		// result. A library may have left it nil, so warn without failing.
		check.errorf(x, NilWarning, "strict: warning: %s may be nil; assign it to a variable and check it != nil", x)
		return false
	}
	check.errorf(x, InvalidStrict, "strict: %s may be nil here; check %s != nil first", v.name, v.name)
	return true
}

// flowNonNil reports whether the local variable v is proven non-nil here. The
// identifier hook sets the operand's nonNull from this answer.
func (check *Checker) flowNonNil(v *Var) bool {
	s := check.flow
	return s != nil && s.facts[v]&flowNonNil != 0
}

// flowRefine applies the narrowing that the condition cond gives on the branch
// where it evaluates to truth. Call it at the start of a branch of an if, or of
// a switch clause (see switchStmt).
// Supported forms: p != nil (true), p == nil (false), && of them (true), and
// || of them (false). Anything else narrows nothing.
func (check *Checker) flowRefine(cond syntax.Expr, truth bool) {
	s := check.flow
	if s == nil || s.dead || s.noFlow {
		return
	}
	for _, v := range check.nonNilVars(cond, truth, nil) {
		if check.flowNarrowable(s, v) {
			bits, ok := s.facts[v]
			if !ok {
				// Not tracked: the variable is assigned from its declaration.
				bits = flowAssigned
			}
			s.facts[v] = bits | flowNonNil
		}
	}
}

// nonNilVars appends to out the variables that cond proves non-nil when it
// evaluates to truth. It reads only the forms that flowRefine supports.
func (check *Checker) nonNilVars(cond syntax.Expr, truth bool, out []*Var) []*Var {
	op, ok := syntax.Unparen(cond).(*syntax.Operation)
	if !ok || op.Y == nil {
		return out
	}
	switch {
	case op.Op == syntax.AndAnd && truth:
		out = check.nonNilVars(op.X, true, out)
		return check.nonNilVars(op.Y, true, out)
	case op.Op == syntax.OrOr && !truth:
		// a || b is false only if both a and b are false.
		out = check.nonNilVars(op.X, false, out)
		return check.nonNilVars(op.Y, false, out)
	case op.Op == syntax.Neq && truth, op.Op == syntax.Eql && !truth:
		x, y := op.X, op.Y
		if check.isNil(x) {
			x, y = y, x
		}
		if !check.isNil(y) {
			return out
		}
		name, _ := syntax.Unparen(x).(*syntax.Name)
		if name == nil {
			return out
		}
		if v, _ := check.lookup(name.Value).(*Var); v != nil {
			out = append(out, v)
		}
	}
	return out
}

// flowNarrowable reports whether the narrowing of v is sound: v is a local or
// parameter of this package, of nullable pointer, function, or interface type,
// and v is not excluded (see narrowingExclusions).
func (check *Checker) flowNarrowable(s *flowState, v *Var) bool {
	if v.pkg != check.pkg || s.excluded[v.pos] {
		return false
	}
	if p := v.Parent(); p == nil || p == v.pkg.Scope() {
		return false // package-level: another function can assign it
	}
	return isNilable(v.typ) && !check.isStrictNonNil(v.typ)
}

// narrowingExclusions returns the variables that flowNarrowable must refuse in a
// function with signature sig and body, keyed by declaration position (Var.pos):
// the operands of &, and the variables that a function literal in body mentions
// and that are not bound exactly once. A literal may run at any later time, so a
// variable it sees must not change; a variable bound once keeps its declared
// value. Every binding counts: a declaration, a plain or := assignment, a
// parameter or receiver, and a type switch guard. A range variable counts twice,
// since it is bound per iteration. A named result counts as bound twice, since a return
// statement assigns it implicitly, so a literal that mentions one excludes it.
func narrowingExclusions(body *syntax.BlockStmt, sig *Signature) map[syntax.Pos]bool {
	s := &exclusionScan{
		bound:   make(map[syntax.Pos]int),
		mention: make(map[syntax.Pos]bool),
		ex:      make(map[syntax.Pos]bool),
		alias:   make(map[syntax.Pos]syntax.Pos),
	}
	s.open()
	if sig != nil {
		if sig.recv != nil {
			s.declare(sig.recv.name, sig.recv.pos, 1)
		}
		for i := range sig.params.Len() {
			v := sig.params.At(i)
			s.declare(v.name, v.pos, 1)
		}
		for i := range sig.results.Len() {
			v := sig.results.At(i)
			s.declare(v.name, v.pos, 2)
		}
	}
	s.stmts(body.List)
	for pos := range s.mention {
		if s.bound[pos] != 1 {
			s.ex[pos] = true
		}
	}
	return s.ex
}

// exclusionScan resolves the identifiers of a function body to the variables
// they denote, following Go's block scopes, for narrowingExclusions. Only the
// declaration position of a variable is recorded, so the result matches Var.pos.
type exclusionScan struct {
	scopes  []map[string]syntax.Pos   // innermost last; a zero position is a name that is not a variable
	depth   int                       // number of enclosing function literals
	bound   map[syntax.Pos]int        // bindings of each variable
	mention map[syntax.Pos]bool       // variables mentioned in a function literal
	ex      map[syntax.Pos]bool       // variables never narrowed
	alias   map[syntax.Pos]syntax.Pos // a case binding maps to the variable of the same name that it may shadow (see caseBinding)
}

func (s *exclusionScan) open()  { s.scopes = append(s.scopes, make(map[string]syntax.Pos)) }
func (s *exclusionScan) close() { s.scopes = s.scopes[:len(s.scopes)-1] }

// enter makes name denote the variable declared at pos in the innermost scope,
// or, with a zero position, a name that is not a variable (a type or constant).
// It does not count a binding.
func (s *exclusionScan) enter(name string, pos syntax.Pos) {
	if name != "_" && name != "" {
		s.scopes[len(s.scopes)-1][name] = pos
	}
}

// declare declares the variable name at pos in the innermost scope, bound n times.
func (s *exclusionScan) declare(name string, pos syntax.Pos, n int) {
	s.enter(name, pos)
	s.bound[pos] += n
}

// define handles the identifier name of a short variable declaration with n
// bindings. A name already declared in the innermost scope is assigned, not
// declared, as in Go.
func (s *exclusionScan) define(name *syntax.Name, n int) {
	if pos, ok := s.scopes[len(s.scopes)-1][name.Value]; ok {
		s.count(pos, n)
		return
	}
	s.declare(name.Value, name.Pos(), n)
}

// caseBinding handles a name n that is an argument in a case such as Circle(r)
// of an expression switch. In an enum switch the name binds a payload value, a
// variable bound once in the clause. Whether the switch is an enum switch is
// known only after checking, so the name is declared here, and it is also an
// alias of the variable of that name that the case would use otherwise. A use,
// a mention, or a binding of the name then counts for both.
func (s *exclusionScan) caseBinding(name *syntax.Name) {
	if name.Value == "_" {
		return
	}
	outer := s.lookup(name.Value)
	s.declare(name.Value, name.Pos(), 1)
	if outer.IsKnown() {
		s.alias[name.Pos()] = outer
	}
}

// count counts n bindings of the variable at pos and of the variables that it aliases.
func (s *exclusionScan) count(pos syntax.Pos, n int) {
	for ; pos.IsKnown(); pos = s.alias[pos] {
		s.bound[pos] += n
	}
}

// lookup returns the declaration position of the variable that name denotes, or
// the zero position if name denotes no local variable.
func (s *exclusionScan) lookup(name string) syntax.Pos {
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if pos, ok := s.scopes[i][name]; ok {
			return pos
		}
	}
	return syntax.Pos{}
}

// use records a mention of name, if it denotes a variable inside a function literal.
func (s *exclusionScan) use(name *syntax.Name) {
	if s.depth == 0 {
		return
	}
	for pos := s.lookup(name.Value); pos.IsKnown(); pos = s.alias[pos] {
		s.mention[pos] = true
	}
}

// bind counts n bindings of the variable that x denotes, if x is an identifier.
func (s *exclusionScan) bind(x syntax.Expr, n int) {
	if name, _ := syntax.Unparen(x).(*syntax.Name); name != nil {
		s.count(s.lookup(name.Value), n)
	}
}

func (s *exclusionScan) node(n syntax.Node) {
	if n != nil {
		syntax.Inspect(n, s.visit)
	}
}

func (s *exclusionScan) expr(x syntax.Expr) {
	if x != nil {
		syntax.Inspect(x, s.visit)
	}
}

func (s *exclusionScan) stmts(list []syntax.Stmt) {
	for _, st := range list {
		s.node(st)
	}
}

// funcLit checks a function literal: its parameters and results are declared in
// a scope of their own, and its body is part of the enclosing function.
func (s *exclusionScan) funcLit(n *syntax.FuncLit) {
	s.depth++
	s.open()
	for _, f := range n.Type.ParamList {
		if f.Name != nil {
			s.declare(f.Name.Value, f.Name.Pos(), 1)
		}
	}
	for _, f := range n.Type.ResultList {
		if f.Name != nil {
			s.declare(f.Name.Value, f.Name.Pos(), 2)
		}
	}
	if n.Body != nil {
		s.stmts(n.Body.List)
	}
	s.close()
	s.depth--
}

// visit is the callback of syntax.Inspect. It handles the nodes that open a
// scope or declare names, and returns true for the others, whose children it
// then visits.
func (s *exclusionScan) visit(n syntax.Node) bool {
	switch n := n.(type) {
	case *syntax.Name:
		s.use(n)
	case *syntax.SelectorExpr:
		s.expr(n.X) // Sel names a field or method, not a variable
		return false
	case *syntax.BranchStmt:
		return false // a label
	case *syntax.LabeledStmt:
		s.node(n.Stmt)
		return false
	case *syntax.Operation:
		if n.Op == syntax.And && n.Y == nil {
			if name, ok := syntax.Unparen(n.X).(*syntax.Name); ok {
				for pos := s.lookup(name.Value); pos.IsKnown(); pos = s.alias[pos] {
					s.ex[pos] = true
				}
			}
		}
	case *syntax.FuncLit:
		s.funcLit(n)
		return false
	case *syntax.BlockStmt:
		s.open()
		s.stmts(n.List)
		s.close()
		return false
	case *syntax.IfStmt:
		s.open()
		s.node(n.Init)
		s.expr(n.Cond)
		s.node(n.Then)
		s.node(n.Else)
		s.close()
		return false
	case *syntax.ForStmt:
		s.open()
		s.node(n.Init)
		s.expr(n.Cond)
		s.node(n.Post)
		s.node(n.Body)
		s.close()
		return false
	case *syntax.RangeClause:
		// The range expression is evaluated before the variables are declared.
		s.expr(n.X)
		for _, x := range syntax.UnpackListExpr(n.Lhs) {
			if name, ok := x.(*syntax.Name); ok && n.Def {
				s.define(name, 2)
				s.use(name)
			} else {
				s.bind(x, 2)
				s.expr(x)
			}
		}
		return false
	case *syntax.AssignStmt:
		if n.Op == syntax.Def {
			// The right-hand side is evaluated before the names are declared.
			s.expr(n.Rhs)
			for _, x := range syntax.UnpackListExpr(n.Lhs) {
				if name, ok := x.(*syntax.Name); ok {
					s.define(name, 1)
					s.use(name)
				}
			}
			return false
		}
		for _, x := range syntax.UnpackListExpr(n.Lhs) {
			s.bind(x, 1)
		}
		s.expr(n.Lhs)
		s.expr(n.Rhs) // nil for x++ and x--
		return false
	case *syntax.DeclStmt:
		for _, d := range n.DeclList {
			switch d := d.(type) {
			case *syntax.VarDecl:
				s.expr(d.Type)
				s.expr(d.Values)
				for _, name := range d.NameList {
					s.declare(name.Value, name.Pos(), 1)
				}
			case *syntax.ConstDecl:
				s.expr(d.Type)
				s.expr(d.Values)
				for _, name := range d.NameList {
					s.enter(name.Value, syntax.Pos{})
				}
			case *syntax.TypeDecl:
				s.enter(d.Name.Value, syntax.Pos{})
				s.expr(d.Type)
			}
		}
		return false
	case *syntax.SwitchStmt:
		s.open()
		s.node(n.Init)
		guard, _ := n.Tag.(*syntax.TypeSwitchGuard)
		if guard != nil {
			s.expr(guard.X)
			if guard.Lhs != nil {
				s.bound[guard.Lhs.Pos()]++ // one variable, declared in every clause
			}
		} else {
			s.expr(n.Tag)
		}
		for _, c := range n.Body {
			s.open()
			if guard != nil && guard.Lhs != nil {
				s.enter(guard.Lhs.Value, guard.Lhs.Pos())
			}
			s.expr(c.Cases)
			for _, e := range syntax.UnpackListExpr(c.Cases) {
				if call, ok := e.(*syntax.CallExpr); ok {
					for _, arg := range call.ArgList {
						if name, ok := arg.(*syntax.Name); ok {
							s.caseBinding(name)
						}
					}
				}
			}
			s.stmts(c.Body)
			s.close()
		}
		s.close()
		return false
	case *syntax.SelectStmt:
		for _, c := range n.Body {
			s.open()
			s.node(c.Comm)
			s.stmts(c.Body)
			s.close()
		}
		return false
	}
	return true
}

// flowCaseCond returns the condition that the case expression e of the
// expression switch s tests: e itself in a tagless switch, and tag == e otherwise.
func flowCaseCond(s *syntax.SwitchStmt, e syntax.Expr) syntax.Expr {
	if s.Tag == nil {
		return e
	}
	return &syntax.Operation{Op: syntax.Eql, X: s.Tag, Y: e}
}

// flowCaseFalse records in the entry state of fork that the case expression e
// of the switch s is false. A later clause is entered only when it is.
func (check *Checker) flowCaseFalse(fork *flowFork, s *syntax.SwitchStmt, e syntax.Expr) {
	if fork == nil {
		return
	}
	saved := check.flow
	check.flow = fork.entry
	check.flowRefine(flowCaseCond(s, e), false)
	check.flow = saved
}

// hasFallthrough reports whether the statements of a switch clause contain a
// fallthrough. The state at its end goes to the next clause, not past the switch.
func hasFallthrough(body []syntax.Stmt) bool {
	for _, s := range body {
		if b, ok := s.(*syntax.BranchStmt); ok && b.Tok == syntax.Fallthrough {
			return true
		}
	}
	return false
}

// assignedNames returns the names that loop assigns anywhere, in its header or
// body. A narrowing of such a name does not survive the loop, because the body
// may run and assign it, and the state after the loop is the state on entry.
func assignedNames(loop syntax.Node) map[string]bool {
	names := make(map[string]bool)
	add := func(lhs syntax.Expr) {
		for _, x := range syntax.UnpackListExpr(lhs) {
			if name, ok := syntax.Unparen(x).(*syntax.Name); ok {
				names[name.Value] = true
			}
		}
	}
	syntax.Inspect(loop, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.AssignStmt:
			add(n.Lhs)
		case *syntax.RangeClause:
			if !n.Def {
				add(n.Lhs)
			}
		}
		return true
	})
	return names
}

// flowDeclare tracks the variables vars declared by the local var declaration
// s, if they have no initializer and their zero value is unsafe. A function
// that contains goto tracks nothing, so such a variable is an error there.
func (check *Checker) flowDeclare(s *syntax.VarDecl, vars []*Var) {
	st := check.flow
	if st == nil {
		return
	}
	for _, v := range vars {
		if s.Values != nil {
			if check.flowProofs[v] {
				check.flowSetNonNil(v) // the initial value is proven non-nil
			}
			continue
		}
		if !check.isZeroUnsafe(v.typ) {
			continue
		}
		if st.noFlow {
			check.errorf(v, UnassignedVar, "strict: %s needs an initializer: the function contains goto", v.name)
			continue
		}
		st.facts[v] = 0
	}
}

// flowSnapshot returns the state that a function literal captures at its
// expression. The body is checked later, so it starts from this snapshot.
func (check *Checker) flowSnapshot() *flowState {
	return check.flow.clone()
}

// flowClosure runs body, which checks the body of a function literal, with the
// snapshot outer as the state of its enclosing function.
func (check *Checker) flowClosure(outer *flowState, body func()) {
	saved := check.flow
	check.flow = outer
	body()
	check.flow = saved
}

// flowEnter returns the state for checking the body of a function with
// signature sig. outer is the state current when the body starts: for a
// function literal, the snapshot taken at the literal (see flowClosure); for a
// declared function, nil. The variables of outer are captured: they are tracked
// with their facts at the literal, so a use of a variable that is unassigned
// there is an error. A narrowing carried in is sound because flowNarrowable
// excludes every captured variable that could change (see narrowingExclusions).
func (check *Checker) flowEnter(body *syntax.BlockStmt, sig *Signature, outer *flowState) *flowState {
	if !check.inStrictFile(body.Pos()) {
		return nil
	}
	st := &flowState{
		facts:    make(map[*Var]flowBits),
		captured: make(map[*Var]bool),
		writes:   make(map[*syntax.Name]bool),
		noFlow:   hasGoto(body),
	}
	if outer == nil {
		st.excluded = narrowingExclusions(body, sig)
	} else {
		// A literal's body is part of its enclosing function, so the
		// exclusions of the enclosing function apply to it as well.
		st.excluded = outer.excluded
		for v, bits := range outer.facts {
			st.facts[v] = bits
			st.captured[v] = true
		}
	}
	return st
}

// hasGoto reports whether body contains a goto statement, including one in a
// function literal inside it. Such a body has no definite assignment, because a
// forward goto can skip an assignment.
func hasGoto(body *syntax.BlockStmt) bool {
	found := false
	syntax.Inspect(body, func(n syntax.Node) bool {
		if b, ok := n.(*syntax.BranchStmt); ok && b.Tok == syntax.Goto {
			found = true
		}
		return !found
	})
	return found
}
