//go:strict

package strict

type P *int

type S struct{ p *int }

type A [2]*int

type L struct{ next *L }

type N struct{ n int }

func f(p *int) *int { return p }

func (s *S) m() *S { return s }

func g() {
	x := 1
	q := &x
	r := &S{p: q}
	_ = f(q)
	_ = r.m()
	_ = N{}
}
