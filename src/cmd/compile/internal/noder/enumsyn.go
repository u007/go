// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"fmt"
	"strings"

	"cmd/compile/internal/syntax"
)

// synthesizeEnumConstructors appends auto-generated constructor functions and
// variant tag constants for every enum type declared in the package.
//
// For a declaration like
//
//	type Shape enum { Circle(float64); Point }
//
// it generates
//
//	func Shape_Circle(p0 float64) Shape { return Shape{_tag: 0, _Circle_0: p0} }
//	const _Shape_Circle uint8 = 0
//	func Shape_Point() Shape { return Shape{_tag: 1} }
//	const _Shape_Point uint8 = 1
//
// The generated source is parsed and appended to the declaring file's
// declaration list, so it participates in ordinary type checking, export data,
// and cross-package visibility (the constructors are exported when the enum
// type and variant names are).
func synthesizeEnumConstructors(noders []*noder) {
	for _, p := range noders {
		if p.file == nil {
			continue
		}

		var body strings.Builder
		found := false
		for _, d := range p.file.DeclList {
			td, ok := d.(*syntax.TypeDecl)
			if !ok || td.Alias {
				continue
			}
			et, ok := td.Type.(*syntax.EnumType)
			if !ok {
				continue
			}
			found = true
			writeEnumConstructors(&body, td, et)
		}
		if !found {
			continue
		}

		pkg := "p"
		if p.file.PkgName != nil {
			pkg = p.file.PkgName.Value
		}
		src := "package " + pkg + "\n" + body.String()

		base := syntax.NewFileBase("enum_constructors")
		if pos := p.file.Pos(); pos.IsKnown() {
			// A line base anchored in the declaring file keeps the constructors
			// part of that file for per-file properties: its Go version and
			// its //go:strict directive.
			fb := pos.FileBase()
			base = syntax.NewLineBase(pos, fb.Filename()+" (enum constructors)", fb.Trimmed(), 1, 1)
		}
		nf, err := syntax.Parse(base, strings.NewReader(src), func(error) {}, nil, 0)
		if err != nil || nf == nil {
			// The synthesized source should always be valid for a valid enum
			// declaration. If it is not, skip constructors rather than failing
			// the original program; the enum type itself still compiles.
			continue
		}
		p.file.DeclList = append(p.file.DeclList, nf.DeclList...)
	}
}

// writeEnumConstructors writes the constructor functions and tag constants for
// one enum type declaration.
func writeEnumConstructors(b *strings.Builder, td *syntax.TypeDecl, et *syntax.EnumType) {
	typeName := td.Name.Value
	tparams, targs := typeParamText(td.TParamList)
	for tag, v := range et.Variants {
		vname := v.Name.Value

		// func Type_Variant[TypeParams](p0 T0, ...) Type[TypeArgs] {
		//     return Type[TypeArgs]{_tag: tag, _Variant_0: p0, ...}
		// }
		fmt.Fprintf(b, "func %s_%s%s(", typeName, vname, tparams)
		for i, pt := range v.PayloadTypes {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(b, "p%d %s", i, syntax.String(pt))
		}
		b.WriteString(") ")
		b.WriteString(typeName)
		b.WriteString(targs)
		b.WriteString(" { return ")
		b.WriteString(typeName)
		b.WriteString(targs)
		b.WriteString("{_tag: ")
		fmt.Fprintf(b, "%d", tag)
		for i := range v.PayloadTypes {
			fmt.Fprintf(b, ", _%s_%d: p%d", vname, i, i)
		}
		b.WriteString("} }\n")

		// const _Type_Variant uint8 = tag
		fmt.Fprintf(b, "const _%s_%s uint8 = %d\n", typeName, vname, tag)
	}
}

// typeParamText renders an enum's type parameter list twice: decl is the
// declaration form, e.g. "[T any, U comparable]", and use is the reference
// form, e.g. "[T, U]". Both are empty for a non-generic enum.
func typeParamText(list []*syntax.Field) (decl, use string) {
	if len(list) == 0 {
		return "", ""
	}
	d := make([]string, 0, len(list))
	u := make([]string, 0, len(list))
	for _, f := range list {
		if f.Name == nil {
			continue
		}
		name := f.Name.Value
		u = append(u, name)
		if f.Type != nil {
			d = append(d, name+" "+syntax.String(f.Type))
		} else {
			d = append(d, name)
		}
	}
	return "[" + strings.Join(d, ", ") + "]", "[" + strings.Join(u, ", ") + "]"
}
