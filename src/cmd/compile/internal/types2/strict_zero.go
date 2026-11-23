// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"strconv"
)

// The checks in this file ban the forms that can produce a zero value of a
// zero-unsafe type (see isZeroUnsafe). Each reports only when the construct is
// positioned in a strict file (Rule 1). Every message starts with "strict: ".

// strictNew reports new(T) when T is zero-unsafe.
func (check *Checker) strictNew(call *syntax.CallExpr, T Type) {
	if check.inStrictFile(call.Pos()) && check.isZeroUnsafe(T) {
		hint := check.sprintf("new(Option[%s])", T)
		if _, ok := T.Underlying().(*Struct); ok {
			hint = check.sprintf("&%s{...} with every field set", T)
		}
		check.errorf(call, InvalidStrict, "strict: new(%s) is not allowed: %s is zero-unsafe; use %s", T, T, hint)
	}
}

// strictMake reports make([]E, ...) when E is zero-unsafe. u is the underlying
// type of the first argument. Maps and channels are not checked here.
func (check *Checker) strictMake(call *syntax.CallExpr, u Type) {
	s, ok := u.(*Slice)
	if !ok || !check.inStrictFile(call.Pos()) || !check.isZeroUnsafe(s.elem) {
		return
	}
	check.errorf(call, InvalidMake, "strict: make([]%s, ...) is not allowed: %s is zero-unsafe; build the slice with a literal or append", s.elem, s.elem)
}

// strictRecover reports any call to recover in a strict file.
func (check *Checker) strictRecover(call *syntax.CallExpr) {
	if check.inStrictFile(call.Pos()) {
		check.error(call, InvalidStrict, "strict: recover is not allowed; a recovered panic returns zero results, which hold nil strict pointers; return an error value instead")
	}
}

// strictRangeChan reports a range over a channel whose element type is
// zero-unsafe. Each iteration receives an element, and a receive from a closed
// channel yields the zero value.
func (check *Checker) strictRangeChan(rangeVar syntax.Expr, t Type) {
	u, _ := commonUnder(t, nil)
	ch, ok := u.(*Chan)
	if !ok || !check.inStrictFile(rangeVar.Pos()) || !check.isZeroUnsafe(ch.elem) {
		return
	}
	check.errorf(rangeVar, InvalidRangeExpr, "strict: range over channel of zero-unsafe element type %s is not allowed; use a channel of Option[%s]", ch.elem, ch.elem)
}

// strictNamedResults reports each named result of zero-unsafe type of a
// function declaration or literal. A function type has no body, so its named
// results are never zero values. Blank
// results count as named. ftyp is the function's type, and
// results holds the corresponding variables, one per field.
func (check *Checker) strictNamedResults(ftyp *syntax.FuncType, results *Tuple) {
	for i, f := range ftyp.ResultList {
		if f.Name == nil || i >= results.Len() || !check.inStrictFile(f.Name.Pos()) || !check.isZeroUnsafe(results.At(i).typ) {
			continue
		}
		check.errorf(f.Name, BadDecl, "strict: named result %s of zero-unsafe type %s is not allowed; declare the result unnamed", f.Name.Value, results.At(i).typ)
	}
}

// strictPkgVarNoInit reports a package-level variable of zero-unsafe type that
// has no initializer. Local variables are handled by definite assignment.
func (check *Checker) strictPkgVarNoInit(obj *Var) {
	if obj.parent != check.pkg.scope || !check.inStrictFile(obj.pos) || !check.isZeroUnsafe(obj.typ) {
		return
	}
	check.errorf(obj, BadDecl, "strict: package-level variable %s of zero-unsafe type %s needs an initializer (or use Option[%s])", obj.name, obj.typ, obj.typ)
}

// strictStructLit reports each zero-unsafe field that a keyed or empty struct
// literal leaves unset. A positional literal is skipped: it must list every
// field, which the regular checker already requires.
func (check *Checker) strictStructLit(e *syntax.CompositeLit, base Type, utyp *Struct) {
	if !check.inStrictFile(e.Pos()) {
		return
	}
	set := make(map[string]bool, len(e.ElemList))
	for _, el := range e.ElemList {
		kv, ok := el.(*syntax.KeyValueExpr)
		if !ok {
			return
		}
		if key, ok := kv.Key.(*syntax.Name); ok {
			set[key.Value] = true
		}
	}
	if utyp.enumInfo != nil {
		check.strictEnumLit(e, base, utyp.enumInfo, set)
		return
	}
	for _, f := range utyp.fields {
		if !set[f.name] && check.isZeroUnsafe(f.typ) {
			check.errorf(e, MissingLitField, "strict: struct literal of type %s must set field %s, which is zero-unsafe (type %s)", base, f.name, f.typ)
		}
	}
}

// strictMapRead reports a read of a map element of zero-unsafe type elem,
// including the comma-ok form. A missing key yields the zero value. The index
// expression that is an assignment target is a write and is allowed; lhsVar
// records that target in check.strictLHS.
func (check *Checker) strictMapRead(e *syntax.IndexExpr, elem Type) {
	if syntax.Expr(e) == check.strictLHS || !check.inStrictFile(e.Pos()) || !check.isZeroUnsafe(elem) {
		return
	}
	check.errorf(e, InvalidIndex, "strict: read of map value of zero-unsafe type %s is not allowed; use map[K]Option[%s]", elem, elem)
}

// strictArrayLit reports an array literal that lists fewer elements than the
// array length when the element type is zero-unsafe. Duplicate and out-of-range
// indices are already errors, so a short list means an element is unset.
func (check *Checker) strictArrayLit(e *syntax.CompositeLit, base Type, utyp *Array) {
	if int64(len(e.ElemList)) >= utyp.len || !check.inStrictFile(e.Pos()) || !check.isZeroUnsafe(utyp.elem) {
		return
	}
	check.errorf(e, InvalidLit, "strict: array literal of type %s must list all %d elements; element type %s is zero-unsafe", base, utyp.len, utyp.elem)
}

// strictEnumLit reports each zero-unsafe payload field that a keyed enum
// literal leaves unset in the variant that its _tag selects. The payload fields
// of the other variants are not live, so they may stay zero. If the tag is not
// a known constant, any variant may be selected, and every payload is required.
// set holds the field names that the literal sets.
func (check *Checker) strictEnumLit(e *syntax.CompositeLit, base Type, info *EnumInfo, set map[string]bool) {
	tag, known := check.enumLitTag(e)
	for _, v := range info.Variants {
		if known && int64(v.Tag) != tag {
			continue
		}
		for i, pt := range v.PayloadTypes {
			if name := fmt.Sprintf("_%s_%d", v.Name, i); !set[name] && check.isZeroUnsafe(pt) {
				check.errorf(e, MissingLitField, "strict: enum literal of type %s must set field %s of variant %s, which is zero-unsafe (type %s)", base, name, v.Name, pt)
			}
		}
	}
}

// enumLitTag returns the constant value of the _tag element of the keyed enum
// literal e. A literal without a _tag element has the zero tag. The result is
// not ok if the tag is not an integer literal or a named integer constant.
func (check *Checker) enumLitTag(e *syntax.CompositeLit) (tag int64, ok bool) {
	for _, el := range e.ElemList {
		kv, _ := el.(*syntax.KeyValueExpr)
		if kv == nil {
			return 0, false
		}
		if key, _ := kv.Key.(*syntax.Name); key == nil || key.Value != "_tag" {
			continue
		}
		switch v := syntax.Unparen(kv.Value).(type) {
		case *syntax.BasicLit:
			if v.Kind == syntax.IntLit {
				if n, err := strconv.ParseInt(v.Value, 0, 64); err == nil {
					return n, true
				}
			}
		case *syntax.Name:
			if c, _ := check.lookup(v.Value).(*Const); c != nil {
				check.objDecl(c)
				if c.val != nil && c.val.Kind() == constant.Int {
					return constant.Int64Val(c.val)
				}
			}
		}
		return 0, false
	}
	return 0, true
}

// strictSliceExpr records the slice expression e, which reslices a slice of
// type s, in check.LenBounded if its high bound could expose zero values of a
// zero-unsafe element type: append leaves such values between the length and
// the capacity of a slice. The compiler then bounds e by the length at run
// time. This check has no go/types counterpart.
func (check *Checker) strictSliceExpr(e *syntax.SliceExpr, s *Slice) {
	if check.LenBounded != nil && e.Index[1] != nil && check.inStrictFile(e.Pos()) && check.isZeroUnsafe(s.elem) {
		check.LenBounded[e] = true
	}
}
