// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"go/token"
	"strconv"
	"strings"
	"testing"
)

func TestStrictDirective(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		strict bool
		line   int    // line of the first error, if err != ""
		err    string // expected first error message; "" for none
	}{
		{"none", "package p\n", false, 0, ""},
		{"directive", "//go:strict\n\npackage p\n", true, 0, ""},
		{"directive-after-comment", "// doc\n//go:strict\npackage p\n", true, 0, ""},
		{"directive-trailing-space", "//go:strict \npackage p\n", true, 0, ""},
		{"directive-tab", "//go:strict\t\npackage p\n", true, 0, ""},
		{"prefix-not-directive", "//go:strictfoo\npackage p\n", false, 0, ""},
		{"after-package", "package p\n\n//go:strict\n", false, 3, "go:strict must appear before package clause"},
		{"after-package-prose", "package p\n\n//go:strict is described here\n", false, 0, ""},
		{"block-comment", "/*go:strict*/\npackage p\n", false, 0, ""},
		{"duplicate", "//go:strict\n//go:strict\npackage p\n", false, 2, "duplicate go:strict directive"},
		{"argument", "//go:strict foo\npackage p\n", false, 1, "go:strict takes no arguments"},
		{"nostrict", "//go:nostrict\npackage p\n", false, 0, ""},
		{"nostrict-after-package", "package p\n\n//go:nostrict\n", false, 3, "go:nostrict must appear before package clause"},
		{"nostrict-argument", "//go:nostrict foo\npackage p\n", false, 1, "go:nostrict takes no arguments"},
		{"nostrict-duplicate", "//go:nostrict\n//go:nostrict\npackage p\n", false, 2, "duplicate go:nostrict directive"},
		{"both", "//go:strict\n//go:nostrict\npackage p\n", true, 2, "go:strict and go:nostrict cannot both appear"},
	}

	// The directive must be honored whether or not comments are kept in the AST,
	// since the parser always sees comment tokens.
	modes := []struct {
		name string
		mode Mode
	}{
		{"no-comments", 0},
		{"comments", ParseComments},
	}

	for _, m := range modes {
		for _, test := range tests {
			t.Run(m.name+"/"+test.name, func(t *testing.T) {
				fset := token.NewFileSet()
				f, err := ParseFile(fset, "strict.go", test.src, m.mode)

				if test.err == "" {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if f == nil {
						t.Fatal("no file returned")
					}
					if f.Strict != test.strict {
						t.Errorf("Strict = %v, want %v", f.Strict, test.strict)
					}
					return
				}

				if err == nil {
					t.Fatalf("no error reported, want %q", test.err)
				}
				msg := err.Error()
				if !strings.Contains(msg, test.err) {
					t.Fatalf("error = %q, want it to contain %q", msg, test.err)
				}
				if want := "strict.go:" + strconv.Itoa(test.line) + ":"; !strings.Contains(msg, want) {
					t.Errorf("error = %q, want position %q", msg, want)
				}
			})
		}
	}
}
