//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A local variable or parameter of nullable type must be proven non-nil before
// a use that needs a value: a selector, a dereference, an index, or a call.

package strict

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type useImpl struct{}

func (useImpl) Read(p []byte) (int, error) { return 0, nil }

func useUnchecked(name string) {
	f, err := os.Open(name)
	if err != nil {
		return
	}
	f /* ERROR "strict: f may be nil here; check f != nil first" */ .Close()
}

func useChecked(name string) {
	f, err := os.Open(name)
	if err != nil || f == nil {
		return
	}
	f.Close()
	_ = *f
}

func useErr(name string) string {
	_, err := os.Open(name)
	_ = err /* ERROR "strict: err may be nil here" */ .Error()
	if err != nil {
		return err.Error()
	}
	return ""
}

// Unassigned: the zero value is nil.
func useUnassigned() {
	var r io.Reader
	r /* ERROR "strict: r may be nil here" */ .Read(nil)
	var fn filepath.WalkFunc
	fn /* ERROR "strict: fn may be nil here" */ ("", nil, nil)
}

// A value that is proven non-nil needs no check.
func useProven(s string) {
	var r io.Reader = useImpl{} // a concrete value boxed in an interface
	r.Read(nil)
	var w io.Writer = &strings.Builder{}
	w.Write(nil)
	r = strings.NewReader(s) // boxed pointer: the interface is not nil
	r.Read(nil)
	fn := func() {} // a function literal has a strict type
	fn()
}

// A result of a non-strict function is not proven.
func useCallResult(s string) {
	br := bufio.NewReader(strings.NewReader(s))
	br /* ERROR "strict: br may be nil here" */ .ReadByte()
	if br != nil {
		br.ReadByte()
	}
}

// An assignment replaces the proof.
func useReassign(name string) {
	var r io.Reader = useImpl{}
	r.Read(nil)
	f, _ := os.Open(name)
	if f != nil {
		r = f
		r.Read(nil)
	}
	r = io.MultiReader() // an interface result of a non-strict function
	r /* ERROR "strict: r may be nil here" */ .Read(nil)
	g := f
	_ = *g /* ERROR "strict: g may be nil here" */
	if f != nil {
		h := f // a copy of a narrowed variable is proven
		h.Close()
	}
}

// A join keeps a proof only if every path has it.
func useJoin(c bool) {
	var r io.Reader = useImpl{}
	if c {
		r = io.MultiReader()
	}
	r /* ERROR "strict: r may be nil here" */ .Read(nil)
}

// Parameters of nullable type are checked too.
func useParam(r io.Reader, err error) {
	r /* ERROR "strict: r may be nil here" */ .Read(nil)
	if err == nil {
		return
	}
	_ = err.Error()
}

// A type switch case of interface type matches no nil interface.
func useTypeSwitch(x any) {
	switch v := x.(type) {
	case io.Reader:
		v.Read(nil)
	case *os.File:
		v /* ERROR "strict: v may be nil here" */ .Close()
	}
}

// Expressions other than variables are not tracked.
func useUntracked() {
	os.Stdout.Close()
	strings.NewReader("").Len()
}

func useIndex(x any) {
	p := x.(*[4]int) // an assertion yields the nullable form
	_ = p /* ERROR "strict: p may be nil here" */ [0]
	_ = p /* ERROR "strict: p may be nil here" */ [1:]
	if p != nil {
		_ = p[0]
	}
}
