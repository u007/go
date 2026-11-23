//go:strict

package strict

import (
	"net/rpc"
	"os"
)

// take takes a strict pointer.
func take(f *os.File) {}

// takeCall takes a strict pointer.
func takeCall(c *rpc.Call) {}

// A clause entered by fallthrough starts from the join of its fallthrough
// entry and its case entry: the case is not tested on the fallthrough path.

func fallthroughTargetJoin() {
	f, err := os.Open("a")
	_ = err
	g, err := os.Open("b")
	_ = err
	switch {
	case g == nil:
		fallthrough
	case f != nil:
		take(f /* ERROR "strict: " */)
	}
}

// A fallthrough clause keeps the narrowing of its own case, and of the clause
// before it, on both paths into the next clause.

func fallthroughKeepsNarrowing() {
	f, err := os.Open("a")
	_ = err
	g, err := os.Open("b")
	_ = err
	switch {
	case f != nil:
		take(f)
		fallthrough
	case f != nil && g != nil:
		take(f)
	}
}

// Clauses that are not fallthrough targets narrow as before.

func fallthroughOtherClauses() {
	f, err := os.Open("a")
	_ = err
	g, err := os.Open("b")
	_ = err
	switch {
	case g == nil:
		fallthrough
	case f == nil:
		return
	case f != nil:
		take(f)
	default:
		take(f)
	}
}

// A fact from the fallthrough path does not hold on the case path into the
// next clause, so the join drops it.

func fallthroughJoinDropsFact() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f != nil:
		fallthrough
	case true:
		take(f /* ERROR "strict: " */)
	}
}

// The state of a fallthrough clause does not reach the statement after the switch.

func fallthroughNoFactAfter() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f != nil:
		fallthrough
	default:
	}
	take(f /* ERROR "strict: " */)
}

// Definite assignment joins the same way: a variable assigned only on the
// fallthrough path is unassigned on the case path.

func fallthroughDefiniteAssign(g *os.File) {
	var f *os.File
	switch {
	case g == nil:
		f = &os.File{}
		fallthrough
	default:
		_ = f /* ERROR "strict: " */
	}
}

// A type switch keeps a narrowing held before it, in every clause.

func typeSwitchKeepsNarrowing(x any) {
	f, err := os.Open("a")
	_ = err
	if f == nil {
		return
	}
	switch v := x.(type) {
	case *os.File:
		take(f)
		_ = v
	default:
		take(f)
	}
}

// The variable bound by a type switch clause is nullable, and a nil check in
// its clause narrows it, as it does any other nullable pointer.

func typeSwitchBoundNarrow(x any) {
	switch v := x.(type) {
	case *os.File:
		if v != nil {
			take(v)
		}
		take(v /* ERROR "strict: " */)
	}
}

// An assignment in a type switch clause kills the narrowing after the switch.

func typeSwitchAssignKills(x any) {
	f, err := os.Open("a")
	_ = err
	if f == nil {
		return
	}
	switch x.(type) {
	case int:
		f, err = os.Open("b")
		_ = err
	}
	take(f /* ERROR "strict: " */)
}

// A break in a type switch clause joins the entry state, not the narrowing
// made in that clause.

func typeSwitchBreakJoinsEntry(x any) {
	f, err := os.Open("a")
	_ = err
	switch x.(type) {
	case int:
		if f == nil {
			return
		}
		break
	}
	take(f /* ERROR "strict: " */)
}

// A select keeps a narrowing held before it, in every clause.

func selectKeepsNarrowing(ch chan *os.File, done chan bool) {
	f, err := os.Open("a")
	_ = err
	if f == nil {
		return
	}
	select {
	case <-done:
		take(f)
	case ch <- f:
	default:
		take(f)
	}
	take(f)
}

// Channel elements declared in non-strict code are received from as usual. The
// select keeps the narrowing of a variable that it does not assign.

func selectKeepsNullableNarrowing(call *rpc.Call) {
	c := <-call.Done
	if c == nil {
		return
	}
	select {
	case <-call.Done:
		takeCall(c)
	case call.Done <- c:
	default:
		takeCall(c)
	}
	takeCall(c)
}

// A receive assignment in a select clause kills the narrowing, in the clause and
// after the select.

func selectAssignKillsNullable(call *rpc.Call) {
	c := <-call.Done
	if c == nil {
		return
	}
	select {
	case c = <-call.Done:
		takeCall(c /* ERROR "strict: " */)
	default:
	}
	takeCall(c /* ERROR "strict: " */)
}

// A tuple receive assignment kills the narrowing the same way.

func selectTupleAssignKillsNullable(call *rpc.Call) {
	c := <-call.Done
	if c == nil {
		return
	}
	var ok bool
	select {
	case c, ok = <-call.Done:
		takeCall(c /* ERROR "strict: " */)
	default:
	}
	_ = ok
	takeCall(c /* ERROR "strict: " */)
}

// A receive declaration in a select clause declares a new nullable variable in
// the clause only. The outer variable keeps its narrowing after the select.

func selectDeclaresNewNullable(call *rpc.Call) {
	c := <-call.Done
	if c == nil {
		return
	}
	select {
	case c := <-call.Done:
		takeCall(c /* ERROR "strict: " */)
	default:
	}
	takeCall(c)
}

// A channel whose element is a strict pointer cannot be received from, since a
// closed channel yields nil. The rejected receive still assigns f, so the
// narrowing is killed in the clause.

func selectAssignKills(ch chan *os.File) {
	f, err := os.Open("a")
	_ = err
	if f == nil {
		return
	}
	select {
	case f = <- /* ERROR "strict: cannot receive" */ ch:
		take(f /* ERROR "strict: " */)
	default:
	}
}

// The tuple form is rejected the same way, and kills the narrowing the same way.

func selectTupleAssignKills(ch chan *os.File) {
	f, err := os.Open("a")
	_ = err
	if f == nil {
		return
	}
	var ok bool
	select {
	case f, ok = <- /* ERROR "strict: cannot receive" */ ch:
		take(f /* ERROR "strict: " */)
	default:
	}
	_ = ok
}

// A receive declaration from a strict channel is rejected too. The outer
// variable keeps its narrowing after the select.

func selectDeclaresNewVar(ch chan *os.File) {
	f, err := os.Open("a")
	_ = err
	if f == nil {
		return
	}
	select {
	case f := <- /* ERROR "strict: cannot receive" */ ch:
		_ = f
	default:
	}
	take(f)
}

// A clause that assigns and then breaks leaves with the assigned value.
func breakAfterAssign(n int) {
	f, _ := os.Open("a")
	if f == nil {
		return
	}
	switch n {
	case 1:
		f = nil
		break
	case 2:
		if n > 0 {
			f, _ = os.Open("b")
			break
		}
	}
	take(f /* ERROR "strict: cannot use f" */)
}

func breakNoAssign(n int) {
	f, _ := os.Open("a")
	if f == nil {
		return
	}
	switch n {
	case 1:
		break
	}
	take(f)
}
