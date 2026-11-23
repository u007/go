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

// ok is a case expression that uses f as a strict argument.
func ok(f *os.File) bool { return true }

// A closure keeps the narrowing of a captured variable that is bound once.

func closureKeeps() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		h := func() { take(f) }
		h()
	}
}

func closureKeepsNested() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() {
			func() { take(f) }()
		}()
	}
}

func closureOwnCheck() {
	f, err := os.Open("a")
	_ = err
	h := func() {
		if f != nil {
			take(f)
		}
	}
	h()
}

func goroutineKeeps() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() { take(f) }()
	}
}

func deferKeeps() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		defer func() { take(f) }()
	}
}

// A map from a non-strict function has nullable values: the element type is not written here.

func rangeNarrow() {
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", nil, 0)
	_ = err
	for _, p := range pkgs {
		if p != nil {
			takePkg(p)
		}
	}
}

// A closure loses the narrowing when the variable is assigned after it.

func closureReassigned() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() { take(f /* ERROR "strict: " */) }()
		f, err = os.Open("b")
		_ = err
	}
}

func closureWriteInGoroutine() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() {
			f, err = os.Open("c")
			_ = err
		}()
		take(f /* ERROR "strict: " */)
	}
}

// A goroutine that narrows a variable that another goroutine may write races with it.
func goroutineRacesWrite() {
	f, err := os.Open("a")
	_ = err
	go func() {
		if f != nil {
			take(f /* ERROR "strict: " */)
		}
	}()
	f, err = os.Open("b")
	_ = err
}

// Redeclaring a variable in the same block assigns it.

func redeclared() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		go func() { take(f /* ERROR "strict: " */) }()
	}
	f, n := os.Open("b")
	_, _ = f, n
}

// A range variable is bound once per iteration, so a closure never keeps its narrowing.

func rangeCaptured() {
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", nil, 0)
	_ = err
	for _, p := range pkgs {
		if p != nil {
			go func() { takePkg(p /* ERROR "strict: " */) }()
		}
	}
}

// A type switch variable assigned in its clause is not kept by a closure.

func typeSwitchGuard(x any) {
	switch v := x.(type) {
	case *os.File:
		if v != nil {
			go func() { take(v /* ERROR "strict: " */) }()
			v = nil
		}
	}
}

// A tagless switch narrows the clause whose case is a non-nil test.

func switchCaseNotNil() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f != nil:
		take(f)
	}
}

// Later clauses and default know that earlier cases are false.

func switchNegationDefault() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f == nil:
		return
	default:
		take(f)
	}
}

func switchNegationAfterTerminating() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f == nil:
		return
	}
	take(f)
}

// Default runs only if every case is false, even when default is written first.

func switchDefaultFirst() {
	f, err := os.Open("a")
	_ = err
	switch {
	default:
		take(f)
	case f == nil:
		return
	}
}

func switchNegationNotInCase() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f != nil:
	default:
		take(f /* ERROR "strict: " */)
	}
}

// A clause with several cases proves nothing about each of them.

func switchMultiCase() {
	f, err := os.Open("a")
	g, err2 := os.Open("b")
	_, _ = err, err2
	switch {
	case f != nil, g != nil:
		take(f /* ERROR "strict: " */)
	}
}

// A narrowing from an earlier case holds in the expressions of a later case.

func switchLaterCaseExpr() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f == nil:
		return
	case ok(f):
	}
}

func switchEarlierCaseExpr() {
	f, err := os.Open("a")
	_ = err
	switch {
	case ok(f /* ERROR "strict: " */):
	case f == nil:
		return
	}
}

// A clause entered by fallthrough does not have its case true.

func switchFallthrough() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f == nil:
		fallthrough
	default:
		take(f /* ERROR "strict: " */)
	}
}

// An expression switch narrows by comparison with nil.

func switchTagNil() {
	f, err := os.Open("a")
	_ = err
	switch f {
	case nil:
		take(f /* ERROR "strict: " */)
	default:
		take(f)
	}
}

func switchTagNilAfter() {
	f, err := os.Open("a")
	_ = err
	switch f {
	case nil:
		return
	}
	take(f)
}

// A switch inside a closure narrows a captured variable that is bound once.

func switchInClosure() {
	f, err := os.Open("a")
	_ = err
	go func() {
		switch {
		case f != nil:
			take(f)
		}
	}()
}
