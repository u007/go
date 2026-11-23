// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"go/ast"
	"go/token"
	. "internal/types/errors"
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
//
// The compiler's checker (types2) rewrites an enum switch into a switch on the
// tag field. This checker must not change the caller's syntax tree, so it
// checks the clauses directly. Both report the same errors.

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
	expr    ast.Expr
	name    *ast.Ident // the variant name in expr
	variant *EnumVariantInfo
	binds   []*ast.Ident // one per payload value; nil for a unit variant
}

// enumPatterns checks the variant patterns of the clauses of the enum switch s
// on a value of type typ, and returns them by clause. It reports whether s has
// a default clause and whether the patterns are valid.
func (check *Checker) enumPatterns(s *ast.SwitchStmt, typ Type, info *EnumInfo) (clauses [][]enumPattern, hasDefault, ok bool) {
	ok = true
	errorf := func(at positioner, format string, args ...any) {
		check.errorf(at, InvalidExprSwitch, format, args...)
		ok = false
	}

	seen := make(map[string]bool)
	clauses = make([][]enumPattern, len(s.Body.List))
	for i, c := range s.Body.List {
		clause, _ := c.(*ast.CaseClause)
		if clause == nil {
			continue // error reported by enumSwitch
		}
		if clause.List == nil {
			hasDefault = true
		}
		if n := len(clause.Body); n > 0 {
			if b, _ := clause.Body[n-1].(*ast.BranchStmt); b != nil && b.Tok == token.FALLTHROUGH {
				errorf(b, "cannot fallthrough in enum switch")
			}
		}
		for _, e := range clause.List {
			p := enumPattern{expr: e}
			bad := e // the part of e that is not a pattern, if any
			p.name, _ = e.(*ast.Ident)
			if call, _ := e.(*ast.CallExpr); call != nil && !call.Ellipsis.IsValid() {
				bad = call.Fun
				p.name, _ = call.Fun.(*ast.Ident)
				p.binds = make([]*ast.Ident, len(call.Args))
				for j, arg := range call.Args {
					if p.binds[j], _ = arg.(*ast.Ident); p.binds[j] == nil && p.name != nil {
						bad, p.name = arg, nil
					}
				}
			}
			if p.name == nil {
				errorf(bad, "invalid enum pattern %s: want a variant name V or V(x, ...)", ExprString(e))
				continue
			}
			for j := range info.Variants {
				if info.Variants[j].Name == p.name.Name {
					p.variant = &info.Variants[j]
				}
			}
			if p.variant == nil {
				errorf(p.name, "%s is not a variant of %s", p.name.Name, typ)
				continue
			}
			if seen[p.name.Name] {
				errorf(p.name, "duplicate variant %s in enum switch", p.name.Name)
				continue
			}
			seen[p.name.Name] = true
			if want := len(p.variant.PayloadTypes); len(p.binds) != want {
				errorf(p.name, "pattern for variant %s has %d payload values, want %d", p.name.Name, len(p.binds), want)
				continue
			}
			for _, b := range p.binds {
				if len(clause.List) > 1 && b.Name != "_" {
					errorf(b, "cannot bind %s in a case with several variants", b.Name)
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

// enumSwitch checks the enum switch s, whose tag x has the enum type described
// by info. Each payload binding of a pattern declares a variable of the
// payload type in the scope of its clause.
func (check *Checker) enumSwitch(inner stmtContext, s *ast.SwitchStmt, x *operand, info *EnumInfo, label string) {
	clauses, hasDefault, ok := check.enumPatterns(s, x.typ(), info)
	if !ok {
		return
	}
	if !hasDefault {
		if check.enumExhaustive == nil {
			check.enumExhaustive = make(map[*ast.SwitchStmt]bool)
		}
		check.enumExhaustive[s] = true
	}

	// The constructor of a variant stands for the variant in a pattern, so
	// that tools find an object and a type for each pattern.
	var prefix string
	scope := check.pkg.scope
	if named := asNamed(x.typ()); named != nil {
		obj := named.Origin().obj
		prefix, scope = obj.name+"_", Universe // the predeclared Option has no package
		if obj.pkg != nil {
			scope = obj.pkg.scope
		}
	}

	check.multipleDefaults(s.Body.List)

	fork := check.flowForkBegin()
	for i, c := range s.Body.List {
		clause, _ := c.(*ast.CaseClause)
		if clause == nil {
			check.error(c, InvalidSyntaxTree, "incorrect expression switch case")
			continue
		}
		check.flowBranchBegin(fork)
		check.openScope(clause, "case")
		for _, p := range clauses[i] {
			if ctor, _ := scope.Lookup(prefix + p.variant.Name).(*Func); ctor != nil {
				check.recordUse(p.name, ctor)
				if p.expr != ast.Expr(p.name) {
					check.recordTypeAndValue(p.name, value, ctor.typ, nil)
				}
			}
			check.recordTypeAndValue(p.expr, value, x.typ(), nil)
			for j, b := range p.binds {
				if b.Name == "_" {
					check.recordDef(b, nil)
					continue
				}
				check.declare(check.scope, b, newVar(LocalVar, b.Pos(), check.pkg, b.Name, p.variant.PayloadTypes[j]), clause.Colon)
			}
		}
		check.stmtList(inner|finalSwitchCase, clause.Body)
		check.closeScope()
		check.flowBranchEnd(fork, clause, hasBreakList(clause.Body, label, true))
	}
	check.flowForkEnd(fork)
}
