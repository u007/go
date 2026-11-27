//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "./f"

// Each p is nullable: f.Maybe is not strict and may return nil.

func noReturn(t *f.T) int {
	p := f.Maybe()
	switch {
	case p == nil:
		f.Fail()
	case p.N == 0:
		p = f.Maybe()
		if p == nil {
			f.FailVia()
		}
	case p.N == 1:
		p = f.Maybe()
		if p == nil {
			f.Exit()
		}
	case p.N == 2:
		p = f.Maybe()
		if p == nil {
			f.Spin()
		}
	case p.N == 3:
		p = f.Maybe()
		if p == nil {
			f.Block()
		}
	default:
		p = f.Maybe()
		if p == nil {
			t.Stop()
		}
	}
	return p.N
}

func mayReturn() int {
	p := f.Maybe()
	if p == nil {
		f.Deferred()
	}
	q := f.Maybe()
	if q == nil {
		f.Early()
	}
	return p.N + q.N // ERROR "strict: p may be nil here" "strict: q may be nil here"
}

func params() {
	p := f.Maybe()
	_ = f.Use(p)                // ERROR "strict: warning: Use panics if its parameter p is nil \(it dereferences it at .*f.go:68:39\); p .* may be nil"
	_ = f.UseVia(1, p)          // ERROR "strict: warning: UseVia panics if its parameter p is nil \(it dereferences it at .*f.go:68:39\)"
	_ = f.Use(nil)              // ERROR "strict: warning: Use panics if its parameter p is nil \(it dereferences it at .*f.go:68:39\); the argument is nil"
	_ = (*f.T).Take(f.New(), p) // ERROR "strict: warning: Take panics if its parameter p is nil \(it dereferences it at .*f.go:70:39\)"
	_ = f.New().Take(p)         // ERROR "strict: warning: Take panics if its parameter p is nil \(it dereferences it at .*f.go:70:39\)"
	_ = f.UseLate(p)            // ERROR "strict: warning: UseLate panics if its parameter p is nil \(it dereferences it at .*f.go:120:32\)"
	f.Call(nil)                 // ERROR "strict: warning: Call panics if its parameter fn is nil \(it dereferences it at .*f.go:71:32\)"
	_ = f.Guarded(p)
	_ = f.Use(f.New())
	_ = f.Use(&f.T{})
	f.Call(func() {})
	if p != nil {
		_ = f.Use(p)
	}
}

// Generic methods carry facts across packages.
func genericMethods(b f.B) int {
	p := f.Maybe()
	if p == nil {
		b.Fail[int](0)
	}
	_ = b.Maybe[int](0).N // ERROR "strict: warning: b.Maybe\[int\]\(0\) .* may be nil"
	return p.N + b.Make[string]("").N
}

func main() {
	_ = genericMethods(f.B{})
	_ = f.New().N
	_ = f.Wrap().N
	_ = f.Recurse(2).N
	_ = f.Boxed().Error()
	f.Lit()()
	_ = f.New().Fresh().N
	x := f.New() // proven non-nil
	_ = x.N
	a, b := f.Pair()
	_, _ = a, b

	_ = f.Maybe().N    // ERROR "strict: warning: f.Maybe\(\) .* may be nil"
	_ = f.Recovers().N // ERROR "strict: warning: f.Recovers\(\) .* may be nil"
	_ = f.Rescued().N  // ERROR "strict: warning: f.Rescued\(\) .* may be nil"
	_ = f.Locked().N
	_ = f.Cancelled().N
	_ = f.Named().N      // ERROR "strict: warning: f.Named\(\) .* may be nil"
	_ = f.New().Self().N // ERROR "strict: warning: f.New\(\).Self\(\) .* may be nil"
	_ = f.Mutual().N     // ERROR "strict: warning: f.Mutual\(\) .* may be nil"
	_ = f.Any(x)
	y := f.Maybe()
	_ = y.N // ERROR "strict: y may be nil here"
	_ = noReturn(f.New()) + mayReturn()
	params()
}
