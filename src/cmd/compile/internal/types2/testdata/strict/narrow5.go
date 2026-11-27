//go:strict

package strict

import "os"

// The right operand of && is checked with the narrowing that its left operand
// gives when true, and that of || when false.
func shortCircuit(name string) bool {
	f, _ := os.Open(name)
	if f == nil && f /* ERROR "strict: f may be nil here" */ .Fd() > 0 {
		return true
	}
	if f != nil || f /* ERROR "strict: f may be nil here" */ .Fd() > 0 {
		return true
	}
	ok := f != nil && f.Fd() > 0
	if ok || f /* ERROR "strict: f may be nil here" */ .Fd() > 0 {
		return true
	}
	if f != nil && f.Fd() > 0 {
		return true
	}
	return f == nil || f.Fd() > 0
}

// A break leaves a clause with the state at its start, which includes the
// state of the clause that fell through to it.
func fallBreak(name string, n int, c bool) {
	f, _ := os.Open(name)
	if f == nil {
		return
	}
	switch n {
	case 0:
		f, _ = os.Open(name)
		fallthrough
	case 1:
		if c {
			break
		}
	default:
	}
	f /* ERROR "strict: f may be nil here" */ .Close()
}

func fallBreakCase(name string, n int, c bool) {
	f, _ := os.Open(name)
	switch {
	case f == nil:
		fallthrough
	case c:
		if n > 0 {
			break
		}
		n++
	default:
	}
	f /* ERROR "strict: f may be nil here" */ .Close()
}
