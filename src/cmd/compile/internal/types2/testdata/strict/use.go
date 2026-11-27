//go:strict

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A local variable or parameter of nullable type must be proven non-nil before
// a use that needs a value: a selector, a dereference, an index, or a call.

package strict

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
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
	r = multi() // an interface result of a non-strict type
	r /* ERROR "strict: r may be nil here" */ .Read(nil)
	g := f
	_ = *g /* ERROR "strict: g may be nil here" */
	if f != nil {
		h := f // a copy of a narrowed variable is proven
		h.Close()
	}
}

// multi returns a nullable io.Reader that may be nil.
func multi() io.Reader { return nil }

// A join keeps a proof only if every path has it.
func useJoin(c bool) {
	var r io.Reader = useImpl{}
	if c {
		r = multi()
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

// Expressions other than local variables are not tracked: a use that needs a
// value is a warning (NilWarning), which does not fail the build.
func useUntracked(rs []io.Reader, u url.URL) {
	os /* ERROR "strict: warning: os.Stdout (variable of type *os.File) may be nil; assign it to a variable and check it != nil" */ .Stdout.Close()
	flag /* ERROR "strict: warning: flag.Lookup(\"x\") (value of type *flag.Flag) may be nil" */ .Lookup("x").Name = ""
	rs /* ERROR "strict: warning: rs[0] (variable of interface type io.Reader) may be nil" */ [0].Read(nil)
	_ = u /* ERROR "strict: warning: u.User (variable of type *url.Userinfo) may be nil" */ .User.Username()
	if user := u.User; user != nil {
		_ = user.Username() // a checked variable: no warning
	}
	fmt.Println(os.Args) // a declared function and a non-pointer value: no warning
	// A call result of interface type may be nil too, unless the facts of
	// the function prove it is not.
	_ = errors /* ERROR "strict: warning: errors.Unwrap(nil) (value of interface type error) may be nil" */ .Unwrap(nil).Error()
}

func useIndex(x any) {
	p := x.(*[4]int) // an assertion yields the nullable form
	_ = p /* ERROR "strict: p may be nil here" */ [0]
	_ = p /* ERROR "strict: p may be nil here" */ [1:]
	if p != nil {
		_ = p[0]
	}
}
