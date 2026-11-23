//go:strict

package strict

import "os"

// useFile takes a strict pointer. A *os.File from os.Open is nullable, so
// passing it here needs a proof of non-nil: a narrowing.
func useFile(f *os.File) {}

func rejectPlain() {
	f, err := os.Open("a")
	_ = err
	useFile(f /* ERROR "strict: " */)
}

func narrowNotNil() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		useFile(f)
	}
}

func narrowNotNilParens() {
	f, err := os.Open("a")
	_ = err
	if (f != nil) {
		useFile(f)
	}
}

func narrowNilReturn() {
	f, err := os.Open("a")
	_ = err
	if f == nil {
		return
	}
	useFile(f)
}

func narrowNilElse() {
	f, err := os.Open("a")
	_ = err
	if f == nil {
	} else {
		useFile(f)
	}
}

func narrowNilReturnAfterOther(c bool) {
	f, err := os.Open("a")
	_ = err
	if c {
		useFile(f /* ERROR "strict: " */)
	}
	if f == nil {
		return
	}
	useFile(f)
}

func narrowAnd() {
	f, err := os.Open("a")
	_ = err
	g, err := os.Open("b")
	_ = err
	if f != nil && g != nil {
		useFile(f)
		useFile(g)
	}
}

func narrowAndPartial() {
	f, err := os.Open("a")
	_ = err
	g, err := os.Open("b")
	_ = err
	if f != nil && g == nil {
		useFile(f)
		useFile(g /* ERROR "strict: " */)
	}
}

func narrowOrRejected() {
	f, err := os.Open("a")
	_ = err
	g, err := os.Open("b")
	_ = err
	if f != nil || g != nil {
		useFile(f /* ERROR "strict: " */)
	}
}

func narrowSwitch() {
	f, err := os.Open("a")
	_ = err
	switch {
	case f != nil:
		useFile(f)
	}
}

func narrowReassignRejected() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		f, err = os.Open("b")
		_ = err
		useFile(f /* ERROR "strict: " */)
	}
}

func narrowAddrRejected() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		q := &f
		_ = q
		useFile(f /* ERROR "strict: " */)
	}
}

// A closure that only reads f cannot change it, so the narrowing survives it.
func narrowClosureRead() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		h := func() {
			_ = f
		}
		h()
		useFile(f)
	}
}

func narrowClosureAssignRejected() {
	f, err := os.Open("a")
	_ = err
	h := func() {
		f, err = os.Open("c")
		_ = err
	}
	if f != nil {
		h()
		useFile(f /* ERROR "strict: " */)
	}
}

func narrowLoopAssignRejected(c bool) {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		for c {
			useFile(f /* ERROR "strict: " */)
			f, err = os.Open("b")
			_ = err
		}
		useFile(f /* ERROR "strict: " */)
	}
}

func narrowLoopNoAssign(c bool) {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		for c {
			useFile(f)
		}
	}
}

func narrowRejectedWhenNotLocal() {
	useFile(pkgFile /* ERROR "strict: " */)
	if pkgFile != nil {
		useFile(pkgFile /* ERROR "strict: " */)
	}
}

// pkgFile has a nullable type, taken from os.Stdout: a *os.File written here
// would be strict, and a package-level strict pointer needs an initializer.
var pkgFile = os.Stdout

func narrowStrictTarget() {
	f, err := os.Open("a")
	_ = err
	if f != nil {
		var s *os.File = f
		useFile(s)
	}
}

func narrowIfInit() {
	f, err := os.Open("a")
	_ = err
	if f = pkgFile; f != nil {
		useFile(f)
	}
}

func condFile(f *os.File) bool { return true }

// A narrowing made before a loop must not reach the loop's condition, post
// statement, or body when the loop assigns the variable: a later iteration
// sees the assigned value.

func loopCondAfterAssign() {
	f, err := os.Open("a")
	if f == nil {
		return
	}
	for condFile(f /* ERROR "strict: " */) {
		f, err = os.Open("b")
	}
	_ = err
}

func loopPostAfterAssign(c bool) {
	f, err := os.Open("a")
	if f == nil {
		return
	}
	for ; c; useFile(f /* ERROR "strict: " */) {
		f, err = os.Open("b")
	}
	_ = err
}

func rangeBodyAfterAssign(xs []int) {
	f, err := os.Open("a")
	if f == nil {
		return
	}
	for range xs {
		useFile(f /* ERROR "strict: " */)
		f, err = os.Open("b")
	}
	_ = err
}

// When a || b is false, both a and b are false. So after an early return on
// err != nil || f == nil, f is proven non-nil.

func narrowOrFalseBranch() {
	f, err := os.Open("a")
	if err != nil || f == nil {
		return
	}
	useFile(f)
}

func narrowOrElse() {
	f, err := os.Open("a")
	g, err2 := os.Open("b")
	_, _ = err, err2
	if f == nil || g == nil {
		useFile(f /* ERROR "strict: " */)
	} else {
		useFile(f)
		useFile(g)
	}
}
