//go:strict

package strict

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// take takes a strict pointer.
func take(f *os.File) {}

// takePkg takes a strict pointer.
func takePkg(p *ast.Package) {}

// strictErr is a non-nil interface type declared in this file.
type strictErr interface{ Error() string }

// takeErr takes a strict interface.
func takeErr(e strictErr) {}

// isOK takes a value that a case may pass to it.
func isOK(e error) bool { return true }

// res is an enum whose payload is a nilable error from a non-strict package.
type res enum {
	Okay
	Err(error)
}

// The narrowing of a variable depends on the bindings of that variable, not of
// its name. A literal that mentions an outer variable keeps its narrowing when
// no other variable with the same name is bound. An inner declaration that
// shadows the name does not count against the outer variable.

func shadowInnerBlock() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		{
			f, err := os.Open("b")
			_ = err
			_ = f
		}
		go func() { take(f) }()
	}
}

// A literal that mentions a shadowing variable excludes it when that variable is
// reassigned, and the outer variable keeps its narrowing.

func shadowInnerReassigned() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		{
			f, err := os.Open("b")
			_ = err
			if f != nil {
				go func() { take(f /* ERROR "strict: " */) }()
			}
			f = nil
		}
		go func() { take(f) }()
	}
}

// Variables declared in sibling blocks are different variables.

func siblingBlocks(c bool) {
	if c {
		f, err := os.Open("a")
		_ = err
		if f != nil {
			go func() { take(f) }()
		}
	} else {
		f, err := os.Open("b")
		_ = err
		if f != nil {
			go func() { take(f /* ERROR "strict: " */) }()
		}
		f = nil
	}
}

// A literal parameter shadows the outer variable. The literal's own parameter is
// bound once, so it keeps its narrowing even though the outer variable is
// reassigned and mentioned by another literal.

func literalParamShadows() {
	f, err := os.Open("a")
	_ = err
	g := func() {}
	if f != nil {
		g = func() { take(f /* ERROR "strict: " */) }
	}
	f, err = os.Open("b")
	_ = err
	h := func(err error) {
		if err != nil {
			takeErr(err)
		}
	}
	_, _ = g, h
	_ = f
}

// A := in a literal declares its own variable when the name is only in an outer
// scope.

func literalLocalShadows() {
	f, err := os.Open("a")
	_ = err
	go func() {
		f, err := os.Open("b")
		_ = err
		if f != nil {
			take(f)
		}
	}()
	f, err = os.Open("c")
	_ = err
	_ = f
}

// A := in the same scope as the declaration assigns the variable, so a literal
// that mentions it excludes it.

func redeclaredSameScope() {
	f, err := os.Open("a")
	if f != nil {
		go func() { take(f /* ERROR "strict: " */) }()
	}
	f, n := os.Open("b")
	_, _ = f, n
	_ = err
}

// A literal that reassigns a variable excludes it, even when the literal is
// nested in another literal.

func reassignedInNestedLiteral() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() {
			func() {
				f, err = os.Open("c")
				_ = err
			}()
		}()
		take(f /* ERROR "strict: " */)
	}
}

// The operand of & is never narrowed. Only that variable is affected: a
// shadowing variable that is not addressed keeps its narrowing.

func addressOnlyShadow() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() { take(f) }()
	}
	{
		f, err := os.Open("b")
		_ = err
		p := &f
		_ = p
		if f != nil {
			take(f /* ERROR "strict: " */)
		}
	}
}

// A range variable is bound once per iteration, so a literal never keeps its
// narrowing. Here it shadows an outer variable, which a literal keeps.

func rangeShadowsOuter() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() { take(f) }()
	}
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", nil, 0)
	_ = err
	for _, f := range pkgs {
		if f != nil {
			go func() { takePkg(f /* ERROR "strict: " */) }()
		}
	}
}

// A type switch variable bound once keeps its narrowing in a literal.

func typeSwitchKept(x any) {
	switch v := x.(type) {
	case *os.File:
		if v != nil {
			go func() { take(v) }()
		}
	}
}

// The clauses of a type switch share one variable, so an assignment in one
// clause excludes the variable in every clause.

func typeSwitchOtherClause(x any) {
	switch v := x.(type) {
	case *os.File:
		if v != nil {
			go func() { take(v /* ERROR "strict: " */) }()
		}
	case *os.Process:
		v = nil
	}
}

// A select receive declares its variable in the clause, bound once.

func selectCommKept(ch chan error) {
	select {
	case e := <-ch:
		if e != nil {
			go func() { takeErr(e) }()
		}
	}
}

// A var declaration with an initializer binds once. A var declared with one
// and then assigned is bound twice.

func varDeclForms() {
	var kept, _ = os.Open("a")
	if kept != nil {
		go func() { take(kept) }()
	}
	var reassigned, _ = os.Open("b")
	reassigned, _ = os.Open("c")
	if reassigned != nil {
		go func() { take(reassigned /* ERROR "strict: " */) }()
	}
}

// A parameter is bound once, and a literal keeps its narrowing.

func paramKept(err error) {
	if err != nil {
		go func() { takeErr(err) }()
	}
}

// A named result is assigned by a return statement, so a literal that mentions
// it excludes it, even though the body assigns it only once.

func namedResult() (err error) {
	err = nil
	if err != nil {
		go func() { takeErr(err /* ERROR "strict: " */) }()
	}
	return
}

// A named result is zero-initialized and reassigned by every return statement.
// A deferred literal that mentions it excludes it, even though the body assigns
// it once and narrows it, so the literal cannot see nil.

func namedResultDeferred() (err error) {
	_, err = os.Open("a")
	if err != nil {
		defer func() { takeErr(err /* ERROR "strict: " */) }()
		return nil
	}
	return err
}

// A constant that shadows a variable is not that variable. A literal that
// mentions the constant does not exclude the variable.

func constShadowsVar() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		{
			const f = 1
			go func() { _ = f }()
		}
		take(f)
	}
	f, err = os.Open("b")
	_ = err
}

// An enum switch binds a payload in a case such as Err(e). The binding is a
// variable of the clause, bound once, so a literal in the clause keeps its
// narrowing.

func enumBindingKept(r res) {
	switch r {
	case Err(e):
		if e != nil {
			go func() { takeErr(e) }()
		}
	case Okay:
	}
}

// An assignment to the binding in its clause excludes it for a literal.

func enumBindingReassigned(r res) {
	switch r {
	case Err(e):
		if e != nil {
			go func() { takeErr(e /* ERROR "strict: " */) }()
		}
		e = nil
	case Okay:
	}
}

// A binding shadows an outer variable of the same name. The outer variable is
// not assigned by the clause, so both keep their narrowing.

func enumBindingShadowsOuter(r res) {
	_, e := os.Open("a")
	if e != nil {
		go func() { takeErr(e) }()
	}
	switch r {
	case Err(e):
		if e != nil {
			go func() { takeErr(e) }()
		}
	case Okay:
	}
}

// A case may pass a name to a function, which is not a binding. The name then
// keeps its narrowing, since the clause does not assign it.

func callCaseArg() {
	_, e := os.Open("a")
	if e != nil {
		switch {
		case isOK(e):
			go func() { takeErr(e) }()
		}
	}
}

// A binding whose clause assigns it also counts for the outer variable of the
// same name, since the pass cannot tell an enum binding from a call argument.
// The outer variable is then excluded: a conservative result.

func enumBindingAssignsAlias(r res) {
	_, e := os.Open("a")
	if e != nil {
		go func() { takeErr(e /* ERROR "strict: " */) }()
	}
	switch r {
	case Err(e):
		e = nil
		_ = e
	case Okay:
	}
}
