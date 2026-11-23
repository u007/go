// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2_test

import (
	"fmt"
	"strings"
	"testing"

	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// TestEnumTooManyVariants exercises the "more than 255 variants" branch, which
// is impractical to express as a testdata file.
func TestEnumTooManyVariants(t *testing.T) {
	var b strings.Builder
	b.WriteString("package p\ntype E enum {\n")
	for i := 0; i < 256; i++ {
		fmt.Fprintf(&b, "V%d\n", i)
	}
	b.WriteString("}\n")

	f, err := syntax.Parse(syntax.NewFileBase("enum.go"), strings.NewReader(b.String()), func(error) {}, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	conf := types2.Config{GoVersion: "go1.28"}
	_, err = conf.Check("p", []*syntax.File{f}, nil)
	if err == nil {
		t.Fatal("expected an error for more than 255 variants, got nil")
	}
	if !strings.Contains(err.Error(), "more than 255 variants") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnumSwitchRecheck verifies that a rewritten enum switch on an enum of
// another package checks again: the noder repeats type checking after a
// midway rewrite of the syntax tree.
func TestEnumSwitchRecheck(t *testing.T) {
	const src = `package p

func f(o Option[int]) int {
	switch o {
	case Some(v):
		return v
	case None:
		return 0
	}
}
`
	f := mustParse(src)
	for range 2 {
		conf := types2.Config{GoVersion: "go1.28"}
		if _, err := conf.Check("p", []*syntax.File{f}, nil); err != nil {
			t.Fatal(err)
		}
	}
}
