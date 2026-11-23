// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestStrictHelpers(t *testing.T) {
	const strictSrc = `//go:strict

package p

type SP *int                 // strict pointer: written in a strict file
type SQ NS                   // named in a strict file over a non-strict pointer
type SS struct{ p *int }     // zero-unsafe
type SA [2]*int              // zero-unsafe
type SZ [0]*int              // not zero-unsafe: no elements
type SL struct{ next *SL }   // zero-unsafe through a recursive strict pointer
type SE enum { A(*int); B }  // zero-unsafe: the first variant holds a strict pointer
type SO enum { N; S(*int) }  // not zero-unsafe: the zero value is the unit variant N
type SC []*int               // not zero-unsafe: slices are never zero-unsafe

type SF func()             // strict function type: written in a strict file
type SI interface{ M() }     // strict interface type

type impl struct{}

func (impl) M() {}

var sp *int = new(int)
var sn = new(int)
var sf = func() {}
var si interface{ M() } = impl{}
`
	const nonStrictSrc = `package p

type NS *int
type NSS struct{ p *int }
type NE enum { A(*int); B }

type NF func()
type NI interface{ M() }

var np *int
var nf = func() {}
var ni interface{ M() } = impl{}
`

	fset := token.NewFileSet()
	var files []*ast.File
	for _, s := range []struct{ name, src string }{
		{"strict.go", strictSrc},
		{"nonstrict.go", nonStrictSrc},
	} {
		f, err := parser.ParseFile(fset, s.name, s.src, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		files = append(files, f)
	}
	if !files[0].Strict || files[1].Strict {
		t.Fatalf("Strict flags: strict.go=%v nonstrict.go=%v; want true, false", files[0].Strict, files[1].Strict)
	}

	pkg := NewPackage("p", "p")
	check := NewChecker(&Config{GoVersion: "go1.28"}, fset, pkg, nil)
	if err := check.Files(files); err != nil {
		t.Fatal(err)
	}

	typeOf := func(name string) Type {
		obj := pkg.Scope().Lookup(name)
		if obj == nil {
			t.Fatalf("%s not declared", name)
		}
		return obj.Type()
	}

	// Pointer bits: set for pointers written in the strict file, and only there.
	for _, tc := range []struct {
		name   string
		strict bool
	}{
		{"sp", true},
		{"sn", true}, // new(int) in the strict file
		{"np", false},
		{"sf", true},
		{"si", true},
		{"nf", false},
		{"ni", false},
	} {
		if got := check.isStrictNonNil(typeOf(tc.name)); got != tc.strict {
			t.Errorf("isStrictNonNil(%s) = %v, want %v", tc.name, got, tc.strict)
		}
	}

	// Named types: the declaring file decides, not the bit of the underlying pointer.
	for _, tc := range []struct {
		name   string
		strict bool
	}{
		{"SP", true},
		{"SQ", true},  // strict declaration over the non-strict *int
		{"NS", false}, // non-strict declaration
		{"SS", false}, // a struct is not a pointer
		{"SF", true},
		{"NF", false},
		{"SI", true},
		{"NI", false},
	} {
		if got := check.isStrictNonNil(typeOf(tc.name)); got != tc.strict {
			t.Errorf("isStrictNonNil(%s) = %v, want %v", tc.name, got, tc.strict)
		}
	}

	// Zero-unsafe: strict pointers, and structs, arrays, and enums containing them.
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"SS", true},
		{"NSS", false},
		{"SA", true},
		{"SZ", false},
		{"SL", true},
		{"SE", true},
		{"SO", false},
		{"NE", false},
		{"SC", false},
		{"sp", true},
		{"np", false},
		{"SF", true},
		{"NF", false},
		{"SI", true},
		{"NI", false},
		{"sf", true},
		{"nf", false},
		{"si", true},
		{"ni", false},
	} {
		if got := check.isZeroUnsafe(typeOf(tc.name)); got != tc.want {
			t.Errorf("isZeroUnsafe(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Strictness is not part of identity.
	if !Identical(typeOf("sp"), typeOf("np")) {
		t.Errorf("Identical(sp, np) = false, want true")
	}
	if !Identical(typeOf("sf"), typeOf("nf")) || !Identical(typeOf("si"), typeOf("ni")) {
		t.Errorf("Identical(sf, nf) or Identical(si, ni) = false, want true")
	}

	// error and any are declared outside every file: they are not strict.
	for _, obj := range []string{"error", "any"} {
		if got := check.isStrictNonNil(Universe.Lookup(obj).Type()); got {
			t.Errorf("isStrictNonNil(%s) = true, want false", obj)
		}
	}

	// An instance keeps the strictness of its type arguments, though the types are
	// identical: strict and non-strict arguments give distinct instances.
	ctxt := NewContext()
	for _, arg := range [][2]Type{
		{typeOf("sf"), typeOf("nf")},
		{typeOf("si"), typeOf("ni")},
	} {
		if ctxt.instanceHash(typeOf("SP"), []Type{arg[0]}) == ctxt.instanceHash(typeOf("SP"), []Type{arg[1]}) {
			t.Errorf("instanceHash(%s) = instanceHash(%s), want distinct instances", arg[0], arg[1])
		}
	}
}
