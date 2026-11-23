// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"go/types"
)

func checkEnum(t *testing.T, src string) (firstErr error) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "enum.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	conf := types.Config{
		GoVersion: "go1.28",
		Error:     func(err error) { firstErr = err },
	}
	conf.Check("p", fset, []*ast.File{f}, nil)
	return firstErr
}

// TestEnumTooManyVariants exercises the "more than 255 variants" branch, which
// is impractical to express as a testdata file.
func TestEnumTooManyVariants(t *testing.T) {
	var b strings.Builder
	b.WriteString("package p\ntype E enum {\n")
	for i := 0; i < 256; i++ {
		fmt.Fprintf(&b, "V%d\n", i)
	}
	b.WriteString("}\n")

	err := checkEnum(t, b.String())
	if err == nil {
		t.Fatal("expected an error for more than 255 variants, got nil")
	}
	if !strings.Contains(err.Error(), "more than 255 variants") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnumConstructorBridgeLeavesAST verifies that the go/types tooling bridge
// for enum constructors does not modify the caller's AST: the constructors are
// declared without adding declarations to the files, so checking the same files
// again succeeds without duplicate-declaration errors.
func TestEnumConstructorBridgeLeavesAST(t *testing.T) {
	const enumSrc = `package p

type E enum {
	A(int)
	B
}
`
	const useSrc = `package p

var _ = E_A(1)
var _ = E_B()
var _ uint8 = _E_A
`
	fset := token.NewFileSet()
	files := make([]*ast.File, 2)
	for i, src := range []string{enumSrc, useSrc} {
		f, err := parser.ParseFile(fset, fmt.Sprintf("file%d.go", i), src, 0)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		files[i] = f
	}
	var before []int
	for _, f := range files {
		before = append(before, len(f.Decls))
	}
	for i := 0; i < 2; i++ {
		var firstErr error
		conf := types.Config{
			GoVersion: "go1.28",
			Error: func(err error) {
				if firstErr == nil {
					firstErr = err
				}
			},
		}
		conf.Check("p", fset, files, nil)
		if firstErr != nil {
			t.Fatalf("check pass %d: %v", i, firstErr)
		}
		for j, f := range files {
			if n := len(f.Decls); n != before[j] {
				t.Fatalf("check pass %d: file %d has %d declarations, want %d", i, j, n, before[j])
			}
		}
	}
}

// TestEnumConstructorBridgeImportInPayload verifies that an import named only
// by an enum payload type resolves in the declaring file, so the import is used
// and no error is reported.
func TestEnumConstructorBridgeImportInPayload(t *testing.T) {
	const src = `package p

import "unsafe"

type E enum {
	A(unsafe.Pointer)
}

var _ = E_A(nil)
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "enum.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	conf := types.Config{GoVersion: "go1.28", Importer: unsafeImporter{}, Error: func(err error) { t.Error(err) }}
	conf.Check("p", fset, []*ast.File{f}, nil)
}

// unsafeImporter imports only package unsafe.
type unsafeImporter struct{}

func (unsafeImporter) Import(path string) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	return nil, fmt.Errorf("unexpected import %q", path)
}

// TestEnumConstructorBridgePositions verifies that the declarations the tooling
// bridge declares carry valid positions inside the file. Tools compute a file's
// extent from File.Pos and File.End; a declaration without positions makes
// that extent negative when a comment precedes the package clause.
func TestEnumConstructorBridgePositions(t *testing.T) {
	const src = `// A leading comment moves the package clause away from offset 0.

package p

type E[T any] enum {
	A(T)
	B
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "enum.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	conf := types.Config{GoVersion: "go1.28", Error: func(err error) { t.Error(err) }}
	conf.Check("p", fset, []*ast.File{f}, nil)

	if f.End() < f.Pos() {
		t.Errorf("File.End() = %d is before File.Pos() = %d", f.End(), f.Pos())
	}
	for _, d := range f.Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if n.Pos() < f.FileStart || n.Pos() > f.FileEnd || n.End() < n.Pos() {
				t.Errorf("%T at %d-%d does not start inside the file %d-%d", n, n.Pos(), n.End(), f.FileStart, f.FileEnd)
			}
			return true
		})
	}
}

// TestEnumConstructorBridgeTypeParam verifies that an identifier of a generic
// enum declaration denotes the enum's type parameter, not the constructor's.
func TestEnumConstructorBridgeTypeParam(t *testing.T) {
	const src = "package p\n\ntype Tree[T any] enum {\n\tLeaf(T)\n\tEmpty\n}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object)}
	conf := types.Config{GoVersion: "go1.28"}
	if _, err := conf.Check("p", fset, []*ast.File{f}, info); err != nil {
		t.Fatal(err)
	}
	ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	def := info.Defs[ts.TypeParams.List[0].Names[0]]
	use := info.Uses[ts.Type.(*ast.EnumType).Variants[0].Payload[0].(*ast.Ident)]
	if def == nil || use != def {
		t.Errorf("T in Leaf(T) denotes %v, want the type parameter of Tree %v", use, def)
	}
}
