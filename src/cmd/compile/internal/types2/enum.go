// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	. "internal/types/errors"
	"strconv"
	"strings"
)

// EnumInfo records metadata about an enum's variants.
type EnumInfo struct {
	Variants []EnumVariantInfo
}

type EnumVariantInfo struct {
	Name         string
	Tag          int
	PayloadTypes []Type
}

func (check *Checker) enumType(styp *Struct, e *syntax.EnumType) {
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
		vname := v.Name.Value
		if seen[vname] {
			check.errorf(v.Name, DuplicateDecl, "duplicate enum variant %s", vname)
			continue
		}
		seen[vname] = true

		var payloadTypes []Type
		for i, texpr := range v.PayloadTypes {
			ptyp := check.varType(texpr)
			payloadTypes = append(payloadTypes, ptyp)
			fname := fmt.Sprintf("_%s_%d", vname, i)
			fld := NewField(v.Pos(), check.pkg, fname, ptyp, false)
			fields = append(fields, fld)
		}

		tagStrings = append(tagStrings, fmt.Sprintf("%s:%d", vname, len(payloadTypes)))

		variantInfos = append(variantInfos, EnumVariantInfo{
			Name:         vname,
			Tag:          tag,
			PayloadTypes: payloadTypes,
		})
	}

	// Field tag on _tag encoding enum variant metadata for export/import
	enumTag := fmt.Sprintf(`go:enum:"%s"`, strings.Join(tagStrings, ";"))
	tags := []string{enumTag}

	styp.fields = fields
	styp.tags = tags
	styp.enumInfo = &EnumInfo{Variants: variantInfos}
	styp.markComplete()
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
	name, ok := syntax.Unparen(x.expr).(*syntax.Name)
	return ok && name.Value == "None"
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
