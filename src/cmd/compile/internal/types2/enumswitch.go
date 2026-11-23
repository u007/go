// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"strconv"
	"strings"
)

// An enum switch is an expression switch whose tag has an enum type. Its cases
// are variant patterns instead of expressions:
//
//	switch s {
//	case Circle(r):   // binds the payload of Circle
//	case Rect(w, _):  // _ ignores a payload value
//	case Point, Line: // several variants; no bindings
//	}
//
// The switch must list every variant or have a default clause.

// enumInfoOf returns the enum information of t if t is an enum type.
func enumInfoOf(t Type) *EnumInfo {
	if t == nil {
		return nil
	}
	if s, _ := t.Underlying().(*Struct); s != nil {
		return s.enumInfo
	}
	return nil
}

// An enumPattern is a variant pattern of an enum switch clause.
type enumPattern struct {
	expr    syntax.Expr
	variant *EnumVariantInfo
	binds   []*syntax.Name // one per payload value; nil for a unit variant
}

// enumPatterns checks the variant patterns of the clauses of the enum switch s
// on a value of type typ, and returns them by clause. It reports whether s has
// a default clause and whether the patterns are valid.
func (check *Checker) enumPatterns(s *syntax.SwitchStmt, typ Type, info *EnumInfo) (clauses [][]enumPattern, hasDefault, ok bool) {
	ok = true
	errorf := func(at poser, format string, args ...any) {
		check.errorf(at, InvalidExprSwitch, format, args...)
		ok = false
	}

	seen := make(map[string]bool)
	clauses = make([][]enumPattern, len(s.Body))
	for i, clause := range s.Body {
		if clause == nil {
			continue // error reported by the caller's loop
		}
		if clause.Cases == nil {
			hasDefault = true
		}
		if n := len(clause.Body); n > 0 {
			if b, _ := clause.Body[n-1].(*syntax.BranchStmt); b != nil && b.Tok == syntax.Fallthrough {
				errorf(b, "cannot fallthrough in enum switch")
			}
		}
		cases := syntax.UnpackListExpr(clause.Cases)
		for _, e := range cases {
			p := enumPattern{expr: e}
			bad := e // the part of e that is not a pattern, if any
			name, _ := e.(*syntax.Name)
			if call, _ := e.(*syntax.CallExpr); call != nil && !call.HasDots {
				bad = call.Fun
				name, _ = call.Fun.(*syntax.Name)
				p.binds = make([]*syntax.Name, len(call.ArgList))
				for j, arg := range call.ArgList {
					if p.binds[j], _ = arg.(*syntax.Name); p.binds[j] == nil && name != nil {
						bad, name = arg, nil
					}
				}
			}
			if name == nil {
				errorf(bad, "invalid enum pattern %s: want a variant name V or V(x, ...)", syntax.String(e))
				continue
			}
			for j := range info.Variants {
				if info.Variants[j].Name == name.Value {
					p.variant = &info.Variants[j]
				}
			}
			if p.variant == nil {
				errorf(name, "%s is not a variant of %s", name.Value, typ)
				continue
			}
			if seen[name.Value] {
				errorf(name, "duplicate variant %s in enum switch", name.Value)
				continue
			}
			seen[name.Value] = true
			if want := len(p.variant.PayloadTypes); len(p.binds) != want {
				errorf(name, "pattern for variant %s has %d payload values, want %d", name.Value, len(p.binds), want)
				continue
			}
			for _, b := range p.binds {
				if len(cases) > 1 && b.Value != "_" {
					errorf(b, "cannot bind %s in a case with several variants", b.Value)
				}
			}
			clauses[i] = append(clauses[i], p)
		}
	}

	if !hasDefault && ok {
		var missing []string
		for _, v := range info.Variants {
			if !seen[v.Name] {
				missing = append(missing, v.Name)
			}
		}
		if missing != nil {
			errorf(s.Tag, "enum switch on %s is missing cases %s (or a default)", typ, strings.Join(missing, ", "))
		}
	}
	return clauses, hasDefault, ok
}

// enumTmp is the name of the temporary that holds the tag value of a rewritten
// enum switch. Source cannot spell it, so it identifies the generated field
// selectors, also when the noder checks the rewritten tree a second time.
const enumTmp = ".enum"

// isEnumTmp reports whether x is the temporary of a rewritten enum switch.
func isEnumTmp(x syntax.Expr) bool {
	name, _ := x.(*syntax.Name)
	return name != nil && name.Value == enumTmp
}

// enumSwitch rewrites the enum switch s, whose tag x has the enum type
// described by info, into an ordinary switch on the tag field:
//
//	switch .enum := x; .enum._tag {
//	case 0:
//		r := .enum._Circle_0
//		...
//	default:
//		panic("invalid enum tag")
//	}
//
// The default clause is added to a switch that lists every variant. It makes
// the switch a terminating statement, and it stops a value with a corrupt tag.
//
// If s has an init statement, s keeps it and becomes a tagless switch whose
// only clause is a default clause holding the switch on the tag. The init
// statement of that inner switch declares the temporary.
//
// The caller then checks the rewritten statement, and the compiler translates
// it like any other switch. enumSwitch reports whether the patterns are valid.
func (check *Checker) enumSwitch(s *syntax.SwitchStmt, x *operand, info *EnumInfo) bool {
	clauses, hasDefault, ok := check.enumPatterns(s, x.typ(), info)
	if !ok {
		return false
	}

	// The tag expression was checked already, so declare the temporary by hand.
	pos := s.Tag.Pos()
	lhs := syntax.NewName(pos, enumTmp)
	init := &syntax.AssignStmt{Op: syntax.Def, Lhs: lhs, Rhs: s.Tag}
	init.SetPos(pos)
	check.declare(check.scope, lhs, newVar(LocalVar, pos, check.pkg, enumTmp, x.typ()), pos)

	if check.enumGen == nil {
		check.enumGen = make(map[syntax.Expr]bool)
	}
	field := func(at syntax.Pos, name string) *syntax.SelectorExpr {
		sel := &syntax.SelectorExpr{X: syntax.NewName(at, enumTmp), Sel: syntax.NewName(at, name)}
		sel.SetPos(at)
		check.enumGen[sel] = true
		return sel
	}

	// sw is the switch on the tag: s itself, or an inner switch if s has an init.
	sw := s
	if s.Init != nil {
		// Positions of the new nodes must not decrease along the scopes the
		// noder opens (see dwarfgen.ScopeMarker), so they are at the tag.
		sw = &syntax.SwitchStmt{Body: s.Body, Rbrace: s.Rbrace}
		sw.SetPos(pos)
		clause := &syntax.CaseClause{Body: []syntax.Stmt{sw}, Colon: pos}
		clause.SetPos(pos)
		s.Body = []*syntax.CaseClause{clause}
		s.Tag = nil
	}
	sw.Init = init
	sw.Tag = field(pos, "_tag")
	for i, clause := range sw.Body {
		if clause == nil || clause.Cases == nil {
			continue
		}
		var tags []syntax.Expr
		var binds []syntax.Stmt
		for _, p := range clauses[i] {
			at := p.expr.Pos()
			tag := &syntax.BasicLit{Value: strconv.Itoa(p.variant.Tag), Kind: syntax.IntLit}
			tag.SetPos(at)
			tags = append(tags, tag)
			for j, b := range p.binds {
				if b.Value == "_" {
					continue
				}
				sel := field(b.Pos(), "_"+p.variant.Name+"_"+strconv.Itoa(j))
				bind := &syntax.AssignStmt{Op: syntax.Def, Lhs: b, Rhs: sel}
				bind.SetPos(b.Pos())
				binds = append(binds, bind)
			}
		}
		if len(tags) == 1 {
			clause.Cases = tags[0]
		} else {
			list := &syntax.ListExpr{ElemList: tags}
			list.SetPos(clause.Cases.Pos())
			clause.Cases = list
		}
		clause.Body = append(binds, clause.Body...)
	}

	if !hasDefault {
		msg := &syntax.BasicLit{Value: `"invalid enum tag"`, Kind: syntax.StringLit}
		fun := syntax.NewName(s.Rbrace, "panic")
		check.enumGen[fun] = true
		call := &syntax.CallExpr{Fun: fun, ArgList: []syntax.Expr{msg}}
		stmt := &syntax.ExprStmt{X: call}
		clause := &syntax.CaseClause{Body: []syntax.Stmt{stmt}, Colon: s.Rbrace}
		for _, n := range []syntax.Node{msg, call, stmt, clause} {
			n.SetPos(s.Rbrace)
		}
		sw.Body = append(sw.Body, clause)
	}
	return true
}
