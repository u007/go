// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"go/ast"
	"go/token"
	. "internal/types/errors"
	"reflect"
	"strconv"
	"strings"
)

// enumConstructorDecls returns, for each file that declares enum types, the
// signature-only constructor functions and tag constants of its enum variants.
// The map has an entry only for files that declare at least one variant.
//
// The compiler's noder synthesizes the constructor bodies; this bridges the
// tooling pipeline, which is driven by go/ast. The declarations have no body
// (like assembly or external functions), which is sufficient for type checking.
//
// The declarations are not added to the files. The checker declares them with
// the file's own declarations (see collectObjects), so they share the file's
// scope, Go version and //go:strict directive, and the caller's AST is left
// unchanged.
func enumConstructorDecls(files []*ast.File) map[*ast.File][]ast.Decl {
	var out map[*ast.File][]ast.Decl
	for _, f := range files {
		var extra []ast.Decl
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				et, ok := ts.Type.(*ast.EnumType)
				if !ok {
					continue
				}
				seen := make(map[string]bool)
				for tag, v := range et.Variants {
					// A duplicate variant is reported by enumType; only its first
					// occurrence gets a constructor, so no redeclaration follows.
					if seen[v.Name.Name] {
						continue
					}
					seen[v.Name.Name] = true
					name := ts.Name.Name + "_" + v.Name.Name
					extra = append(extra,
						makeEnumConstructor(ts, v, name),
						setMissingPos(makeEnumTagConst(name, tag), ts.Name.Pos()))
				}
			}
		}
		if len(extra) > 0 {
			if out == nil {
				out = make(map[*ast.File][]ast.Decl)
			}
			out[f] = extra
		}
	}
	return out
}

// makeEnumConstructor builds a signature-only declaration
//
//	func Type_Variant[TypeParams](p0 T0, ...) Type[TypeArgs]
//
// for one enum variant. Payload and constraint expressions are shared with the
// enum declaration (they resolve identically in the same scope), but they are
// only read: every node this function creates is given its position before the
// shared expressions are attached.
func makeEnumConstructor(ts *ast.TypeSpec, v *ast.EnumVariant, name string) *ast.FuncDecl {
	pos := ts.Name.Pos()

	var params []*ast.Field
	for i := range v.Payload {
		params = append(params, &ast.Field{Names: []*ast.Ident{{Name: fmt.Sprintf("p%d", i)}}})
	}

	var result ast.Expr
	targs := typeParamUse(ts.TypeParams)
	if targs == nil {
		result = &ast.Ident{Name: ts.Name.Name, NamePos: pos}
	} else {
		result = &ast.IndexListExpr{X: &ast.Ident{Name: ts.Name.Name, NamePos: pos}, Indices: targs}
	}

	// A body is required for generic functions, and harmless for others: this
	// declaration exists only for tooling type checking and is never executed
	// (the compiler synthesizes the real bodies in the noder).
	body := &ast.BlockStmt{List: []ast.Stmt{
		&ast.ExprStmt{X: &ast.CallExpr{
			Fun:  &ast.Ident{Name: "panic"},
			Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"enum constructor"`}},
		}},
	}}

	tparams := cloneTypeParams(ts.TypeParams)
	fd := &ast.FuncDecl{
		Name: &ast.Ident{Name: name, NamePos: pos},
		Type: &ast.FuncType{
			TypeParams: tparams,
			Params:     &ast.FieldList{List: params},
			Results:    &ast.FieldList{List: []*ast.Field{{Type: result}}},
		},
		Body: body,
	}
	setMissingPos(fd, pos)

	for i, pt := range v.Payload {
		params[i].Type = cloneExpr(pt)
	}
	if tparams != nil {
		for i, f := range ts.TypeParams.List {
			tparams.List[i].Type = cloneExpr(f.Type)
		}
	}
	return fd
}

// cloneExpr returns a deep copy of e. The constructor needs its own nodes: the
// checker records one object per identifier, and an identifier of the enum
// declaration must keep the object it denotes there.
func cloneExpr(e ast.Expr) ast.Expr {
	return cloneValue(reflect.ValueOf(&e).Elem()).Interface().(ast.Expr)
}

func cloneValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() || v.Type() == reflect.TypeFor[*ast.Object]() {
			return reflect.Zero(v.Type())
		}
		c := reflect.New(v.Elem().Type())
		c.Elem().Set(cloneValue(v.Elem()))
		if v.Kind() == reflect.Pointer {
			return c
		}
		return c.Elem().Convert(v.Type())
	case reflect.Struct:
		c := reflect.New(v.Type()).Elem()
		for i := range v.NumField() {
			c.Field(i).Set(cloneValue(v.Field(i)))
		}
		return c
	case reflect.Slice:
		c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			c.Index(i).Set(cloneValue(v.Index(i)))
		}
		return c
	}
	return v
}

// makeEnumTagConst builds "const _Type_Variant uint8 = tag".
func makeEnumTagConst(name string, tag int) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{&ast.ValueSpec{
			Names:  []*ast.Ident{{Name: "_" + name}},
			Type:   &ast.Ident{Name: "uint8"},
			Values: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: fmt.Sprint(tag)}},
		}},
	}
}

// cloneTypeParams returns a copy of list with fresh name identifiers and no
// constraints; the caller attaches the constraint expressions of list, which
// are shared with the enum declaration.
func cloneTypeParams(list *ast.FieldList) *ast.FieldList {
	if list == nil {
		return nil
	}
	nf := &ast.FieldList{}
	for _, f := range list.List {
		var names []*ast.Ident
		for _, n := range f.Names {
			names = append(names, &ast.Ident{Name: n.Name})
		}
		nf.List = append(nf.List, &ast.Field{Names: names})
	}
	return nf
}

// typeParamUse returns fresh identifiers referring to each type parameter in
// list, for use as type arguments.
func typeParamUse(list *ast.FieldList) []ast.Expr {
	if list == nil {
		return nil
	}
	var out []ast.Expr
	for _, f := range list.List {
		for _, n := range f.Names {
			out = append(out, &ast.Ident{Name: n.Name})
		}
	}
	return out
}

// EnumInfo records metadata about an enum's variants.
// It is attached to the Struct type that an enum lowers to.
type EnumInfo struct {
	Variants []EnumVariantInfo
}

// EnumVariantInfo describes a single variant of an enum type.
type EnumVariantInfo struct {
	Name         string
	Tag          int
	PayloadTypes []Type
}

// EnumInfo returns the enum variant metadata if this struct was lowered from
// an enum, or nil.
func (s *Struct) EnumInfo() *EnumInfo { return s.enumInfo }

// SetEnumInfo sets the enum variant metadata on this struct.
func (s *Struct) SetEnumInfo(info *EnumInfo) { s.enumInfo = info }

// enumType type-checks an enum type and fills in styp as the struct it lowers to.
func (check *Checker) enumType(styp *Struct, e *ast.EnumType) {
	if !check.verifyVersionf(e, go1_28, "enum type") {
		styp.markComplete()
		return
	}
	if check.sig != nil {
		// Constructors and tag constants are package-level declarations.
		check.error(e, BadDecl, "enum type must be declared at package level")
		styp.markComplete()
		return
	}

	if len(e.Variants) == 0 {
		check.error(e, BadDecl, "enum type must have at least one variant")
		styp.markComplete()
		return
	}
	if len(e.Variants) > 255 {
		check.error(e, BadDecl, "enum type cannot have more than 255 variants")
		styp.markComplete()
		return
	}

	// First field is the discriminant tag: _tag uint8
	tagField := NewField(e.Pos(), check.pkg, "_tag", Typ[Uint8], false)
	fields := []*Var{tagField}

	var variantInfos []EnumVariantInfo
	seen := make(map[string]bool)
	var tagStrings []string

	for tag, v := range e.Variants {
		vname := v.Name.Name
		if seen[vname] {
			check.errorf(v.Name, DuplicateDecl, "duplicate enum variant %s", vname)
			continue
		}
		seen[vname] = true

		var payloadTypes []Type
		for i, texpr := range v.Payload {
			ptyp := check.varType(texpr)
			payloadTypes = append(payloadTypes, ptyp)
			fname := fmt.Sprintf("_%s_%d", vname, i)
			fld := NewField(texpr.Pos(), check.pkg, fname, ptyp, false)
			fields = append(fields, fld)
		}

		tagStrings = append(tagStrings, fmt.Sprintf("%s:%d", vname, len(payloadTypes)))

		variantInfos = append(variantInfos, EnumVariantInfo{
			Name:         vname,
			Tag:          tag,
			PayloadTypes: payloadTypes,
		})
	}

	// Field tag on _tag encoding enum variant metadata for export/import.
	enumTag := fmt.Sprintf(`go:enum:"%s"`, strings.Join(tagStrings, ";"))
	styp.fields = fields
	styp.tags = []string{enumTag}
	styp.enumInfo = &EnumInfo{Variants: variantInfos}
	styp.markComplete()
}

// setMissingPos gives pos to every node in the synthesized declaration d that
// has no position, and returns d. The checker reports errors at, and decides
// strictness from (see inStrictFile), the positions of these nodes, so they must
// lie inside the declaring file. d must not contain caller-owned nodes.
func setMissingPos(d ast.Decl, pos token.Pos) ast.Decl {
	set := func(p *token.Pos) {
		if !p.IsValid() {
			*p = pos
		}
	}
	ast.Inspect(d, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			set(&n.NamePos)
		case *ast.BasicLit:
			set(&n.ValuePos)
		case *ast.GenDecl:
			set(&n.TokPos)
		case *ast.FuncType:
			set(&n.Func)
		case *ast.FieldList:
			set(&n.Opening)
			set(&n.Closing)
		case *ast.BlockStmt:
			set(&n.Lbrace)
			set(&n.Rbrace)
		case *ast.CallExpr:
			set(&n.Lparen)
			set(&n.Rparen)
		case *ast.IndexListExpr:
			set(&n.Lbrack)
			set(&n.Rbrack)
		}
		return true
	})
	return d
}

// isOption reports whether t is an instance of the predeclared Option type.
func isOption(t Type) bool {
	n := asNamed(t)
	return n != nil && n.Origin().obj == universeOption
}

// isNone reports whether x is the predeclared None. Only the universe scope
// has a nil-mode operand named None, so the name is enough.
func isNone(x *operand) bool {
	if !x.isNil() || x.expr == nil {
		return false
	}
	name, ok := ast.Unparen(x.expr).(*ast.Ident)
	return ok && name.Name == "None"
}

// enumInfoOfTag returns the enum information of a struct with the given fields
// and tags, or nil if it is not the lowered form of an enum. The tag of the
// _tag field lists each variant with its number of payload values. A struct
// read from export data gets its enum information this way.
func enumInfoOfTag(fields []*Var, tags []string) *EnumInfo {
	if len(tags) == 0 || fields[0].name != "_tag" {
		return nil
	}
	list, ok := strings.CutPrefix(tags[0], `go:enum:"`)
	if !ok {
		return nil
	}
	info := new(EnumInfo)
	rest := fields[1:]
	for tag, v := range strings.Split(strings.TrimSuffix(list, `"`), ";") {
		name, count, _ := strings.Cut(v, ":")
		n, err := strconv.Atoi(count)
		if err != nil || n < 0 || n > len(rest) {
			return nil
		}
		var payload []Type
		for _, f := range rest[:n] {
			payload = append(payload, f.typ)
		}
		rest = rest[n:]
		info.Variants = append(info.Variants, EnumVariantInfo{Name: name, Tag: tag, PayloadTypes: payload})
	}
	if len(rest) > 0 {
		return nil
	}
	return info
}
