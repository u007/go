//go:strict

package ifaces

import (
	"container/list"
	"context"
	"flag"
	"fmt"
	"sync"
)

// An interface type written in a strict file is strict: nil is not assignable
// to it, and a nullable interface value needs a nil check before it is used as
// non-nil. The predeclared any is an interface type too, so it is strict where it
// is written. The predeclared error is exempt.

type I interface{ M() }

type C interface{ Done() <-chan struct{} }

type Ex interface {
	error
	Code() int
}

type impl struct{}

func (impl) M() {}

type Front interface {
	Front() *list.Element
}

var nilI I = nil /* ERROR "strict: cannot use nil as I value" */

var nilAny any = nil // any is exempt

var pkgI /* ERROR "strict: package-level variable pkgI" */ I

var pkgAny any // any is exempt

var okI I = impl{}

var okPtr I = &impl{}

// Boxing: a concrete value is stored with its dynamic type, so the interface is
// never nil, even when the value itself is a nil pointer or function.
var boxInt any = 5

var boxStr any = "s"

func boxNil() {
	p := list /* ERROR "strict: warning: list.New() (value of type *list.List) may be nil" */ .New().Front() // a nullable *list.Element
	var a any = p
	f := flag.Usage // a nullable func
	var b any = f
	_, _ = a, b
}

// background returns a nullable interface of package context, which is not
// strict. The checker does not use the function facts of its own package, so
// the result is not proven non-nil.
func background() context.Context { return context.Background() }

// Nullable interface from package context, which is not strict.
func nullable() {
	ctx := background()
	var c C = ctx /* ERROR "strict: cannot use ctx" */
	_ = c
	takeAny(ctx)
	fmt.Println(ctx)
	if ctx != nil {
		var c2 C = ctx
		_ = c2
		takeAny(ctx)
	}
	if ctx == nil {
		return
	}
	var c3 C = ctx
	_ = c3
}

// A narrowing ends when the variable is assigned again.
func narrowingKilled() {
	ctx := background()
	if ctx != nil {
		ctx = background()
		var c C = ctx /* ERROR "strict: cannot use ctx" */
		_ = c
	}
}

// Values of strict interface types pass to non-strict code freely.
func toStd(i I) {
	fmt.Println(i)
	var s fmt.Stringer
	_ = s
}

func takeI(i I)     {}
func takeAny(v any) {}

// Locals need definite assignment, as strict pointers do.
func locals() {
	var i I
	i = impl{}
	takeI(i)

	var j I
	takeI(j /* ERROR "strict: j used before it is definitely assigned" */)
}

// A type assertion yields the nullable form of a strict unnamed interface type.
// A named strict interface type has no nullable form, so asserting it is an
// error.
func assertions(x any) {
	v := x.(interface{ M() })
	takeI(v /* ERROR "strict: cannot use v" */)
	if v != nil {
		takeI(v)
	}
	_ = x.(I /* ERROR "strict: cannot assert named strict interface type I" */)

	switch y := x.(type) {
	case interface{ M() }:
		takeI(y) // a nil interface matches no interface case
	}
}

// The predeclared any is exempt, like error.
func anyZero(x any) any {
	return nil
}

func anyMap(m map[string]any) any {
	return m["k"]
}

func anyParam(v any) any {
	return v
}

// A generic instance over any keeps the strictness of its type argument.
func id[T any](x T) T { return x }

func generic() {
	_ = id[any](nil)
	_ = id[I](impl{})
}

// The predeclared error is exempt: nil is assignable to it, and its results
// need no non-nil proof.
func errorsExempt() error {
	return nil
}

func errorLocal() {
	var e error
	_ = e
	var e2 error = nil
	_ = e2
	takeErr(nil)
	err := fmt.Errorf("x")
	takeErr(err)
}

func takeErr(e error) {}

func errorNamed() (err error) {
	return
}

// A strict interface that embeds error is strict: only the error part is exempt.
func exNil() Ex {
	return nil /* ERROR "strict: cannot use nil as Ex value" */
}

// A value whose method returns a nullable pointer cannot satisfy an interface
// whose method returns a strict one.
func methodResults() {
	l := list.New()
	var f Front = l /* ERROR "strict: cannot use l" */
	_ = f
}

// A method call through an interface, or through a type parameter's constraint,
// has no strict receiver: the call dispatches on the interface value.
func callMethods[T interface{ M() }](x T, s I) {
	x.M()
	s.M()
}

// any is exempt, so a callback with any parameters is accepted.
func rangeCallback(m *sync.Map) {
	m.Range(func(k, v any) bool { return true })
}

// In a default, nil, or multi-type clause the bound variable has the type of
// the switch operand, which is not an asserted type.
func typeSwitchOperand(s I) {
	switch v := s.(type) {
	case impl:
		v.M()
	case nil:
		_ = v
	case interface{ N() }, interface{ O() }:
		v.M()
	default:
		v.M()
	}
}
