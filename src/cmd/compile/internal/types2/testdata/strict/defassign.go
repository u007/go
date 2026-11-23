//go:strict

package strict

type S struct{ p *int }

type A [2]*int

var x int

func use(p *int) {}

func ret(p *int) *int { return p }

func ok(p *int) bool { return true }

// Sequence: a whole-value assignment counts, a read before it does not.

func seqOK() {
	var p *int
	p = &x
	use(p)
}

func seqRead() {
	var p *int
	use(p /* ERROR "used before it is definitely assigned" */)
}

func seqSelf() {
	var p *int
	p = ret(p /* ERROR "used before it is definitely assigned" */)
	use(p)
}

func seqPartial() {
	var p, q *int
	p, q = &x, &x
	use(p)
	use(q)
}

func seqPartialMiss() {
	var p, q *int
	p = &x
	use(p)
	use(q /* ERROR "used before it is definitely assigned" */)
}

func seqShortRedecl() {
	var p *int
	p, n := &x, 1
	use(p)
	_ = n
}

func seqShadow(c bool) {
	var p *int
	if c {
		p := &x
		use(p)
	}
	p = &x
	use(p)
}

// Writes that are not reads: field, element, dereference, address.

func fieldWrite() {
	var s S
	s /* ERROR "used before it is definitely assigned" */ .p = &x
	s = S{p: &x}
	use(s.p)
}

func fieldRead() {
	var s S
	use(s /* ERROR "used before it is definitely assigned" */ .p)
}

func structOK() {
	var s S
	s = S{p: &x}
	use(s.p)
}

func elemWrite() {
	var a A
	a /* ERROR "used before it is definitely assigned" */ [0] = &x
	a = A{&x, &x}
	use(a[1])
}

func addrOf() {
	var p *int
	q := &p /* ERROR "used before it is definitely assigned" */
	_ = q
	p = &x
	use(p)
}

// if / else.

func ifElseBoth(c bool) {
	var p *int
	if c {
		p = &x
	} else {
		p = &x
	}
	use(p)
}

func ifElseOne(c bool) {
	var p *int
	if c {
		p = &x
	} else {
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func ifNoElse(c bool) {
	var p *int
	if c {
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func ifNoElseBefore(c bool) {
	var p *int
	p = &x
	if c {
		p = &x
	}
	use(p)
}

func ifElseThenReturns(c bool) {
	var p *int
	if c {
		return
	} else {
		p = &x
	}
	use(p)
}

func ifElseElseReturns(c bool) {
	var p *int
	if c {
		p = &x
	} else {
		return
	}
	use(p)
}

func ifElsePanic(c bool) {
	var p *int
	if c {
		p = &x
	} else {
		panic("no")
	}
	use(p)
}

func ifElseChain(c, d bool) {
	var p *int
	if c {
		p = &x
	} else if d {
		p = &x
	} else {
		p = &x
	}
	use(p)
}

func ifElseChainMiss(c, d bool) {
	var p *int
	if c {
		p = &x
	} else if d {
		p = &x
	} else {
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func ifNested(c, d bool) {
	var p *int
	if c {
		if d {
			p = &x
		} else {
			p = &x
		}
	} else {
		p = &x
	}
	use(p)
}

func ifNestedMiss(c, d bool) {
	var p *int
	if c {
		if d {
			p = &x
		}
	} else {
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

// switch and type switch.

func switchDefault(n int) {
	var p *int
	switch n {
	case 1:
		p = &x
	default:
		p = &x
	}
	use(p)
}

func switchNoDefault(n int) {
	var p *int
	switch n {
	case 1:
		p = &x
	case 2:
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func switchClauseMiss(n int) {
	var p *int
	switch n {
	case 1:
		p = &x
	default:
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func switchClauseReturns(n int) {
	var p *int
	switch n {
	case 1:
		p = &x
	default:
		return
	}
	use(p)
}

func switchClauseBreak(n int) {
	var p *int
	switch n {
	case 1:
		p = &x
		break
	default:
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func switchBreakNested(n int, c bool) {
	var p *int
	switch n {
	case 1:
		if c {
			break
		}
		p = &x
	default:
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func switchFallthrough(n int) {
	var p *int
	switch n {
	case 1:
		p = &x
		fallthrough
	default:
		p = &x
	}
	use(p)
}

func switchFallthroughMiss(n int) {
	var p *int
	switch n {
	case 1:
		p = &x
		fallthrough
	default:
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func typeSwitch(v any) {
	var p *int
	switch v.(type) {
	case int:
		p = &x
	default:
		p = &x
	}
	use(p)
}

func typeSwitchNoDefault(v any) {
	var p *int
	switch v.(type) {
	case int:
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func typeSwitchBind(v any) {
	var p *int
	switch w := v.(type) {
	case int:
		p = &x
		_ = w
	default:
		p = &x
	}
	use(p)
}

// select: every clause must assign, and a default clause is a clause.

func selectAll(ch chan int) {
	var p *int
	select {
	case <-ch:
		p = &x
	default:
		p = &x
	}
	use(p)
}

func selectMiss(ch chan int) {
	var p *int
	select {
	case <-ch:
		p = &x
	default:
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func selectClauseReturns(ch chan int) {
	var p *int
	select {
	case <-ch:
		p = &x
	case ch <- 1:
		return
	}
	use(p)
}

func selectBreak(ch chan int) {
	var p *int
	select {
	case <-ch:
		p = &x
		break
	default:
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

// for and range: nothing assigned in the body counts after the loop, and a
// read in the body before an assignment of the same iteration is an error.

func forBodyAfter(c bool) {
	var p *int
	for c {
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func forBodyRead(c bool) {
	var p *int
	for c {
		use(p /* ERROR "used before it is definitely assigned" */)
		p = &x
	}
}

func forBodyAssignFirst(c bool) {
	var p *int
	for c {
		p = &x
		use(p)
	}
}

func forInit(c bool) {
	var p *int
	for p = &x; c; {
		use(p)
	}
	use(p)
}

func forCond() {
	var p *int
	for ok(p /* ERROR "used before it is definitely assigned" */) {
	}
}

func forContinue(c bool) {
	var p *int
	for c {
		if c {
			continue
		}
		p = &x
		use(p)
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func forInfiniteBreak() {
	var p *int
	for {
		p = &x
		break
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func forInfiniteNoBreakJoin(c bool) {
	var p *int
	if c {
		for {
		}
	} else {
		p = &x
	}
	use(p)
}

func forLabelBreak() {
	var p *int
L:
	for {
		p = &x
		break L
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func forLabelContinue(c bool) {
	var p *int
L:
	for c {
		for c {
			continue L
		}
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func forLocal(c bool) {
	for c {
		var p *int
		p = &x
		use(p)
	}
}

func rangeAfter(xs []int) {
	var p *int
	for range xs {
		p = &x
	}
	use(p /* ERROR "used before it is definitely assigned" */)
}

func rangeRead(xs []int) {
	var p *int
	for range xs {
		use(p /* ERROR "used before it is definitely assigned" */)
		p = &x
	}
}

func rangeAssignFirst(xs []int) {
	var p *int
	for range xs {
		p = &x
		use(p)
	}
}

// Closures: a literal must not mention a variable that is unassigned where it
// is created, and a closure never assigns for the enclosing function.

func closureBeforeAssign() {
	var p *int
	f := func() {
		use(p /* ERROR "used before it is definitely assigned" */)
	}
	p = &x
	f()
}

func closureAfterAssign() {
	var p *int
	p = &x
	f := func() {
		use(p)
	}
	f()
}

func closureWriteUnassigned() {
	var p *int
	f := func() {
		p /* ERROR "used before it is definitely assigned" */ = &x
	}
	f()
	use(p /* ERROR "used before it is definitely assigned" */)
}

func closureWriteAssigned() {
	var p *int
	p = &x
	f := func() {
		p = &x
	}
	f()
	use(p)
}

func closureNested() {
	var p *int
	f := func() {
		g := func() {
			use(p /* ERROR "used before it is definitely assigned" */)
		}
		g()
	}
	p = &x
	f()
}

func closureLocal() {
	f := func() {
		var p *int
		use(p /* ERROR "used before it is definitely assigned" */)
	}
	f()
}

func deferClosure() {
	var p *int
	defer func() {
		use(p /* ERROR "used before it is definitely assigned" */)
	}()
	p = &x
}

func goClosure() {
	var p *int
	go func() {
		use(p /* ERROR "used before it is definitely assigned" */)
	}()
	p = &x
}

func deferRead() {
	var p *int
	defer use(p /* ERROR "used before it is definitely assigned" */)
	p = &x
}

// Terminating branches drop out of the join.

func panicBranch(c bool) {
	var p *int
	if c {
		panic("no")
	} else {
		p = &x
	}
	use(p)
}

// Not tracked: a local with an initializer, or a type whose zero value is safe.

func untracked() {
	var n int
	_ = n
	var p *int = &x
	use(p)
}

// goto: the function gets no definite assignment, so a zero-unsafe local with
// no initializer is an error.

func gotoFunc(c bool) {
	var p /* ERROR "needs an initializer" */ *int
	if c {
		goto L
	}
	p = &x
L:
	use(p)
}

// A range clause with = assigns its variables before each run of the body.
func rangeAssign(ps []*int) {
	var p *int
	for _, p = range ps {
		use(p)
	}
	use(p /* ERROR "strict: p used before it is definitely assigned" */)
}

func rangeAssignRead() {
	var p *int
	for _, p = range []*int{p /* ERROR "strict: p used before it is definitely assigned" */} {
	}
}
