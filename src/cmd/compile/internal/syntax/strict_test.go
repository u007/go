// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"strings"
	"testing"
)

func TestStrictDirective(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		strict bool
		err    string // expected first error; "" for none
	}{
		{"none", "package p\n", false, ""},
		{"directive", "//go:strict\n\npackage p\n", true, ""},
		{"directive-after-comment", "// doc\n//go:strict\npackage p\n", true, ""},
		{"directive-trailing-space", "//go:strict \npackage p\n", true, ""},
		{"prefix-not-directive", "//go:strictfoo\npackage p\n", false, ""},
		{"after-package", "package p\n\n//go:strict\n", false, "go:strict must appear before package clause"},
		{"after-package-prose", "package p\n\n//go:strict is described here\n", false, ""},
		{"block-comment", "/*go:strict*/\npackage p\n", false, ""},
		{"duplicate", "//go:strict\n//go:strict\npackage p\n", false, "duplicate go:strict directive"},
		{"argument", "//go:strict foo\npackage p\n", false, "go:strict takes no arguments"},
		{"nostrict", "//go:nostrict\npackage p\n", false, ""},
		{"nostrict-after-package", "package p\n\n//go:nostrict\n", false, "go:nostrict must appear before package clause"},
		{"nostrict-argument", "//go:nostrict foo\npackage p\n", false, "go:nostrict takes no arguments"},
		{"nostrict-duplicate", "//go:nostrict\n//go:nostrict\npackage p\n", false, "duplicate go:nostrict directive"},
		{"both", "//go:strict\n//go:nostrict\npackage p\n", true, "go:strict and go:nostrict cannot both appear"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var errs []string
			errh := func(err error) { errs = append(errs, err.Error()) }

			// Parse with a nil pragma handler must still honor the directive.
			f, _ := Parse(NewFileBase("strict.go"), strings.NewReader(test.src), errh, nil, 0)

			if test.err == "" {
				if len(errs) != 0 {
					t.Fatalf("unexpected errors: %v", errs)
				}
				if f == nil {
					t.Fatal("no file returned")
				}
				if f.Strict != test.strict {
					t.Errorf("Strict = %v, want %v", f.Strict, test.strict)
				}
			} else if len(errs) == 0 || !strings.Contains(errs[0], test.err) {
				// The parser returns no file once an error precedes the package clause.
				t.Errorf("errors = %v, want first error containing %q", errs, test.err)
			}

			// The directive must not be forwarded to a non-nil pragma handler.
			var pragmas []string
			pragh := func(_ Pos, _ bool, text string, current Pragma) Pragma {
				pragmas = append(pragmas, text)
				return current
			}
			Parse(NewFileBase("strict.go"), strings.NewReader(test.src), func(error) {}, pragh, 0)
			for _, text := range pragmas {
				if fields := strings.Fields(text); len(fields) > 0 && fields[0] == "go:strict" {
					t.Errorf("pragma handler received %q", text)
				}
			}
		})
	}
}
