//go:strict

package strict

type S struct{ p *int }

type N struct{ n int }

type A [2]*int

type Pair struct{ a, b *int }

type Outer struct{ in S }

type E enum {
	Hold(*int)
	Free
}

var gx int

// Item 6: package-level variables without an initializer.

var pkgBad /* ERROR "strict: package-level variable" */ *int

var pkgOK = &S{p: &gx}

var pkgN N

// Item 1: new(T).

func newBad() *S {
	return new /* ERROR "strict: new" */ (S)
}

func newOK() *N {
	return new(N)
}

// Item 2: make of a slice. Maps and channels are allowed.

func makeBad() {
	_ = make /* ERROR "strict: make" */ ([]*int, 3)
	_ = make /* ERROR "strict: make" */ ([]S, 0, 3)
}

func makeOK() {
	_ = make([]N, 3)
	_ = make(map[string]S)
	_ = make(chan S)
}

// Item 3: map reads. A map of zero-unsafe values may only be assigned and
// deleted, because a missing key yields a zero value.

func mapBad(mp map[string]*int, mm map[string]S) {
	_ = mp /* ERROR "strict: read of map" */ ["a"]
	v, ok := mm /* ERROR "strict: read of map" */ ["a"]
	_, _ = v, ok
}

func mapOK(mp map[string]*int, mn map[string]N, mm map[string]S) {
	mp["a"] = &gx
	delete(mp, "a")
	mm["b"] = S{p: &gx}
	_ = mn["a"]
	v, ok := mn["a"]
	_, _ = v, ok
}

// Item 4: range over a channel. Ranging over a slice or a map is allowed.

func rangeBad(ch chan *int) {
	for range ch /* ERROR "strict: range over channel" */ {
	}
}

func rangeOK(ch chan int, s []*int, m map[string]*int) {
	for v := range ch {
		_ = v
	}
	for range s {
	}
	for k, v := range m {
		_, _ = k, v
	}
}

// Item 5: named results. Unnamed results are allowed.

func namedBad() (r /* ERROR "strict: named result" */ *int) {
	return &gx
}

func namedBlankBad() (_ /* ERROR "strict: named result" */ *int) {
	return &gx
}

func unnamedOK() *int {
	return &gx
}

// A function type or an interface method has no body, so its named results
// are allowed. A function literal has one.

type F func() (r *int)

type I interface {
	M() (r *int)
}

var namedLit = func() (r /* ERROR "strict: named result" */ *int) { return &gx }

// Item 7: struct and array literal completeness. The error sits after the
// first token of the literal.

func litBad() {
	_ = S /* ERROR "strict: struct literal" */ {}
	_ = Pair /* ERROR "strict: struct literal" */ {a: &gx}
	_ = Pair /* ERROR "must set field a" */ /* ERROR "must set field b" */ {}
	_ = Outer{in: S /* ERROR "strict: struct literal" */ {}}
	_ = A /* ERROR "strict: array literal" */ {}
	_ = [ /* ERROR "strict: array literal" */ 2]*int{&gx}
}

func litOK() {
	_ = S{p: &gx}
	_ = N{}
	_ = Pair{a: &gx, b: &gx}
	_ = Outer{in: S{p: &gx}}
	_ = A{&gx, &gx}
	_ = [2]*int{0: &gx, 1: &gx}
}

// Item 7 for enums: a literal must set the zero-unsafe payload of the variant
// that its _tag selects, and no other variant's payload. Without _tag the
// literal is the first variant. A tag that is not a constant selects no known
// variant, so every payload is required.

const tagFree = 1

func enumFreeOK() E {
	return E{_tag: 1}
}

func enumFreeConstOK() E {
	return E{_tag: tagFree}
}

func enumHoldOK() E {
	return E{_tag: 0, _Hold_0: &gx}
}

func enumHoldBad() E {
	return E /* ERROR "strict: enum literal of type E must set field _Hold_0" */ {_tag: 0}
}

func enumNoTagBad() E {
	return E /* ERROR "strict: enum literal of type E must set field _Hold_0" */ {}
}

func enumDynTagBad(t uint8) E {
	return E /* ERROR "strict: enum literal of type E must set field _Hold_0" */ {_tag: t}
}

// The zero value of an enum is its first variant. O starts with a unit variant,
// so its zero value is valid and O is not zero-unsafe. E starts with Hold.

type O enum {
	None
	Some(*int)
}

var zeroO O

var zeroE /* ERROR "strict: package-level variable" */ E

func optNoneOK() O {
	return O{}
}

func optNewOK() *O {
	return new(O)
}

func optSomeBad() O {
	return O /* ERROR "strict: enum literal of type O must set field _Some_0" */ {_tag: 1}
}

// Item 8: recover. The shadowed name is an accepted case.

func recoverBad() any {
	return recover /* ERROR "strict: recover" */ ()
}

func recoverOK() int {
	recover := func() int { return 1 }
	return recover()
}

// Item 3 and item 4 for type parameters: the core type decides.

func genMapBad[M ~map[string]*int](m M) {
	_ = m /* ERROR "strict: read of map" */ ["a"]
}

func genRangeBad[C ~chan *int](c C) {
	for range c /* ERROR "strict: range over channel" */ {
	}
}

// A payload read does not check the tag, and the payload of an inactive
// variant is zero. A pointer payload reads as nullable; other zero-unsafe
// payloads cannot be read.

type W struct{ p *int }

type Q enum {
	None
	Ptr(*int)
	Wrap(W)
	Num(int)
}

func payloadRead(q Q) int {
	p := q._Ptr_0
	var s *int = p /* ERROR "strict: cannot use p" */
	_ = s
	_ = &q /* ERROR "cannot take address" */ ._Ptr_0
	_ = q /* ERROR "strict: cannot read enum payload field _Wrap_0" */ ._Wrap_0
	if p != nil {
		return *p + q._Num_0
	}
	return int(q._tag)
}

func payloadWrite(q Q) Q {
	q._Ptr_0 = &gx
	q._Ptr_0 = nil /* ERROR "strict: cannot use nil" */
	q._Wrap_0 = W{p: &gx}
	return q
}

// An enum switch checks the tag, so its payload bindings keep their declared
// types: a strict pointer payload is non-nil, and any payload can be read.

func payloadSwitch(q Q) *int {
	switch q {
	case Ptr(p):
		var s *int = p
		return s
	case Wrap(w):
		return w.p
	case Num(_), None:
	}
	return &gx
}

type GO[T any] enum {
	GNone
	GSome(T)
}

func payloadGeneric[T any](o GO[T], def T) T {
	_ = o /* ERROR "strict: cannot read enum payload field _GSome_0" */ ._GSome_0
	switch o {
	case GSome(v):
		return v
	case GNone:
		return def
	}
}

// The zero value of an instantiated enum is its first variant too.
var zeroGO GO[*int]
