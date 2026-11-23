//go:strict

package assign

import (
	"os"
	"sync/atomic"
)

type S struct{ n int }

type SP *S

type Holder struct {
	p *S
	n int
}

type Arr [2]*S

// nf is nullable: its type comes from package os, which is not strict.
var nf = os.Stdout

var sp *S = &S{}

var sp2 SP = sp

var bad1 *S = nil /* ERROR "strict: cannot use nil" */

var bad2 *os.File = nf /* ERROR "strict: cannot use nf" */

var ok3 *os.File = &os.File{}

func takeS(p *S)          {}
func takeFile(f *os.File) {}
func many(ps ...*S)       {}

func giveFile() *os.File {
	return nf /* ERROR "strict: cannot use nf" */
}

func giveS() *S {
	return nil /* ERROR "strict: cannot use nil" */
}

func nilUses() {
	var p *S = nil /* ERROR "strict: cannot use nil" */
	_ = p

	h := Holder{p: nil /* ERROR "strict: cannot use nil" */, n: 1}
	_ = h

	takeS(nil /* ERROR "strict: cannot use nil" */)

	sv := &S{}
	sv = nil /* ERROR "strict: cannot use nil" */
	_ = sv

	arr := Arr{&S{}, sp}
	arr[0] = nil /* ERROR "strict: cannot use nil" */
	_ = arr

	many(sp, nil /* ERROR "strict: cannot use nil" */)

	var r SP = nil /* ERROR "strict: cannot use nil" */
	_ = r

	_ = SP(nil /* ERROR "strict: cannot use nil" */)
	_ = (*S)(nil /* ERROR "strict: cannot use nil" */)
}

func sendNil(ch chan *S) {
	ch <- nil /* ERROR "strict: cannot use nil" */
}

func nullableUses(ch chan *os.File) {
	takeFile(nf /* ERROR "strict: cannot use nf" */)

	f, err := os.Open("x")
	takeFile(f /* ERROR "strict: cannot use f" */)
	_ = err

	ch <- nf /* ERROR "strict: cannot use nf" */

	var g *os.File = nf /* ERROR "strict: cannot use nf" */
	_ = g

	_ = (*os.File)(nf /* ERROR "strict: cannot use nf" */)
}

func receives(ch chan *S, ich chan int) int {
	_ = <- /* ERROR "strict: cannot receive" */ ch

	select {
	case v := <- /* ERROR "strict: cannot receive" */ ch:
		_ = v
	default:
	}

	return <-ich
}

func accepted() {
	x := 1
	px := &x
	var pi *int = px
	_ = pi

	takeS(sp)
	takeS(&S{})
	var q *S = sp
	var r SP = sp
	_, _ = q, r
	sp2 = sp

	takeFile(&os.File{})
	many(sp, &S{})

	var n64 int64
	atomic.AddInt64(&n64, 1)

	_ = nf.Name()
}
