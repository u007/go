//go:strict

package boundary

func (t *T) M() {}

var gt = &T{}

// Composite types: strictness is not part of identity, so each use is checked.

func composites() {
	var s []*T = NS /* ERROR "strict: cannot use NS" */
	var a [2]*T = NA /* ERROR "strict: cannot use NA" */
	var m map[string]*T = NM /* ERROR "strict: cannot use NM" */
	var c chan *T = NC /* ERROR "strict: cannot use NC" */
	var pp **T = NPP /* ERROR "strict: cannot use NPP" */
	_, _, _, _, _ = s, a, m, c, pp

	ok := []*T{gt}
	var s2 []*T = ok
	s2 = append(s2, NS /* ERROR "strict: cannot use NS" */ ...)
	copy(s2, NS /* ERROR "strict: cannot copy NS" */)
	copy(s2, ok)
	_ = [][]*T{ok, NS /* ERROR "strict: cannot use NS" */}

	// A nullable composite stays usable as itself.
	ns := NS
	_ = ns
}

func takes(s []*T) {}

func compositeArg() {
	takes(NS /* ERROR "strict: cannot use NS" */)
	takes([]*T{gt})
	takes(nil)
}

// Generic instances keep the strictness of their own type arguments.

func instances() {
	sb := Box[*T]{v: gt}
	var b Box[*T] = sb
	b = NB /* ERROR "strict: cannot use NB" */
	_ = b
	_ = Box /* ERROR "strict: struct literal" */ [*T]{}
}

// Type assertions: an interface may hold a nil pointer.

type NP *T

func assertions(x any) {
	p := x.(*T)
	p /* ERROR "strict: p may be nil here" */ .M()
	if p != nil {
		p.M()
	}
	var q *T = x /* ERROR "strict: cannot use x.(*T)" */ .(*T)
	_ = q
	_ = x.(NP /* ERROR "strict: cannot assert named strict pointer type" */)
	_ = x.(T)

	switch v := x.(type) {
	case *T:
		v /* ERROR "strict: v may be nil here" */ .M()
		if v != nil {
			v.M()
		}
	case T:
		v.M()
	}
}

// Type parameters: a type argument may be a strict pointer.

func zeroAny[E any]() E {
	return *new /* ERROR "strict: new" */ (E)
}

func makeAny[E any](n int) []E {
	return make /* ERROR "strict: make" */ ([]E, n)
}

func varAny[E any]() E {
	var zero E
	return zero /* ERROR "used before it is definitely assigned" */
}

func zeroNum[E ~int | ~string](n int) (E, []E) {
	var zero E
	return zero, make([]E, n)
}

func zeroPtr[E ~*int | ~string]() E {
	var zero E
	return zero /* ERROR "used before it is definitely assigned" */
}

func appendAny[E any](s []E, e E) []E {
	var r []E
	r = append(r, s...)
	return append(r, e)
}

// Embedded pointers: the embedded field is the receiver.

type SEmb struct{ *T }

func embedded() {
	NE /* ERROR "strict: cannot call method M through embedded field of nullable type *T" */ .M()
	se := SEmb{T: gt}
	se.M()
}

// Function and interface types: results are checked like values, parameters
// in the other direction.

type Getter interface{ Get() *T }

type Tight struct{}

func (Tight) Get() *T { return gt }

func sget() *T { return gt }
func sput(*T)  {}

func funcs() {
	var get func() *T = NGet /* ERROR "strict: cannot use NGet" */
	var put func(*T) = NPut /* ERROR "strict: cannot use NPut" */
	get = sget
	put = sput
	_, _ = get, put

	NGet = sget
	NPut = sput /* ERROR "strict: cannot use sput" */

	var g Getter = Loose /* ERROR "strict: cannot use Loose{}" */ {}
	g = Tight{}
	var a any = Loose{}
	_, _ = g, a
}
