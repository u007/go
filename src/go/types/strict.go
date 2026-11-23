// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"go/ast"
	"go/token"
	. "internal/types/errors"
	"slices"
)

// inStrictFile reports whether pos lies in a file that was checked by check and
// carries a //go:strict directive. A position inside a //line-directed region
// is attributed to the file that contains it. Positions in files that check did
// not check (for example, in imported packages) report false.
func (check *Checker) inStrictFile(pos token.Pos) bool {
	if !pos.IsValid() {
		return false
	}
	f := check.fset.File(pos)
	return f != nil && check.strictFiles[f]
}

// newPointerAt returns a new pointer type for elem. The result is strict iff
// pos lies in a strict file (see inStrictFile). Use it for every pointer type
// that the checker creates from source; NewPointer is for pointers without a
// source position.
func (check *Checker) newPointerAt(pos token.Pos, elem Type) *Pointer {
	return &Pointer{base: elem, strict: check.inStrictFile(pos)}
}

// isNilable reports whether the values of t can be nil: t, or the underlying
// type of t, is a pointer, function, or interface type. A type parameter is not
// nilable, although its underlying type is its constraint interface.
func isNilable(t Type) bool {
	if isTypeParam(t) {
		return false
	}
	switch Unalias(t).Underlying().(type) {
	case *Pointer, *Signature, *Interface:
		return true
	}
	return false
}

// isStrictNonNil reports whether t is a strict non-nil type: a pointer, function,
// or interface type whose values are non-nil by type. After Unalias:
//   - For an unnamed *Pointer, *Signature, or *Interface, the result is its
//     strict bit.
//   - For a Named type N whose underlying type is nilable, the result is true iff
//     N is declared in a strict file. The strict bit of the underlying type is
//     ignored, so `type P Q` declared in a strict file is strict even when Q is
//     declared in a non-strict file.
//   - Named types declared in files that check did not check (such as imported
//     types, and the predeclared error) are not strict.
//   - All other types report false.
func (check *Checker) isStrictNonNil(t Type) bool {
	switch t := Unalias(t).(type) {
	case *Named:
		if !isNilable(t) {
			return false
		}
		obj := t.Origin().obj
		if obj.pkg == check.pkg {
			return check.inStrictFile(obj.pos)
		}
		// An imported type has no file here. Its underlying type literal
		// carries the strictness of its declaration (see MarkStrict).
		return obj.pkg != nil && IsStrict(t.Underlying())
	default:
		return IsStrict(t)
	}
}

// IsStrict reports whether the pointer, function, or interface type literal t
// was written in a //go:strict file.
func IsStrict(t Type) bool {
	switch t := t.(type) {
	case *Pointer:
		return t.strict
	case *Signature:
		return t.strict
	case *Interface:
		return t.strict
	}
	return false
}

// MarkStrict records that the pointer, function, or interface type literal t
// was written in a //go:strict file. It is for importers, which rebuild the
// types of a package from its export data.
func MarkStrict(t Type) {
	switch t := t.(type) {
	case *Pointer:
		t.strict = true
	case *Signature:
		t.strict = true
	case *Interface:
		t.strict = true
	}
}

// strictForm returns the unnamed pointer, function, or interface type t with
// its strict bit set to strict. The nullable form (strict false) is the type of
// a value that may be nil: one that an interface holds, or a payload that an
// inactive enum variant leaves zero. The strict form is the type of a value
// that is proven non-nil. It returns nil for a named type, which has one form.
func strictForm(t Type, strict bool) Type {
	switch t := Unalias(t).(type) {
	case *Pointer:
		return &Pointer{base: t.base, strict: strict}
	case *Signature:
		s := *t
		s.strict = strict
		return &s
	case *Interface:
		t.typeSet() // complete the type set, so that the copy does not compute it later
		i := *t
		i.strict = strict
		i.check = nil // as for any interface whose type set is complete
		return &i
	}
	return nil
}

// strictFuncValue returns the type of the function value of type t that is
// named at pos: a declared function. Such a value is never nil, so in a strict
// file it has the strict form of its signature, whichever file declares it.
func (check *Checker) strictFuncValue(pos token.Pos, t Type) Type {
	sig, _ := Unalias(t).(*Signature)
	// A generic signature keeps the strictness of its declaration, which its
	// instances need (see strictInstanceResults).
	if sig == nil || sig.strict || sig.TypeParams().Len() > 0 || !check.inStrictFile(pos) {
		return t
	}
	s := *sig
	s.strict = true
	return &s
}

// nilableKind names the kind of the nilable type t, as the messages do.
func nilableKind(t Type) string {
	switch Unalias(t).Underlying().(type) {
	case *Signature:
		return "func"
	case *Interface:
		return "interface"
	}
	return "pointer"
}

// isZeroUnsafe reports whether the zero value of t contains a strict non-nil
// type, that is, a nil value that a strict file forbids. It reports true iff:
//   - t is a strict non-nil type (see isStrictNonNil), or
//   - t's underlying type is a struct with a zero-unsafe field, or
//   - t's underlying type is an array of positive length whose element type is
//     zero-unsafe.
//
// A type parameter declared in a strict file is zero-unsafe unless its
// constraint lists specific types and none of them is zero-unsafe.
//
// An enum is zero-unsafe iff a payload type of its first variant is
// zero-unsafe: the zero value of an enum is its first variant, and the payload
// fields of the other variants are not live in it. Slices, maps, channels, and
// nullable types other than strict ones are never zero-unsafe. The recursion
// terminates on recursive types.
func (check *Checker) isZeroUnsafe(t Type) bool {
	return check.zeroUnsafe(t, make(map[Type]bool))
}

// zeroUnsafe implements isZeroUnsafe. seen holds the types whose check is in
// progress or complete; a type seen before is reported as false, which is sound
// because any true result returns immediately.
func (check *Checker) zeroUnsafe(t Type, seen map[Type]bool) bool {
	if check.isStrictNonNil(t) {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t] = true

	if tp, _ := Unalias(t).(*TypeParam); tp != nil {
		// A type parameter of a strict file may be instantiated with any type
		// of its type set. Without specific types, that includes a strict
		// pointer.
		return check.inStrictFile(tp.obj.pos) && !tp.typeset(func(t, _ Type) bool {
			return t != nil && !check.zeroUnsafe(t, seen)
		})
	}

	switch u := t.Underlying().(type) {
	case *Struct:
		if u.enumInfo != nil {
			// The zero value of an enum is its first variant, so only that
			// variant's payload is live in it.
			if len(u.enumInfo.Variants) > 0 {
				for _, pt := range u.enumInfo.Variants[0].PayloadTypes {
					if check.zeroUnsafe(pt, seen) {
						return true
					}
				}
			}
			return false
		}
		for _, f := range u.fields {
			if check.zeroUnsafe(f.typ, seen) {
				return true
			}
		}
	case *Array:
		return u.len > 0 && check.zeroUnsafe(u.elem, seen)
	}
	return false
}

// strictAssertType returns the type of the value that a type assertion or a
// type switch clause at pos yields for the asserted type T. An interface may
// hold a nil pointer, function, or interface value, so in a strict file an
// asserted strict type yields its nullable form, which the code must compare
// with nil before it uses the value as non-nil. A named strict type has no
// nullable form, so asserting it is an error.
func (check *Checker) strictAssertType(at positioner, T Type) Type {
	if !check.inStrictFile(at.Pos()) || !check.isStrictNonNil(T) {
		return T
	}
	if n := strictForm(T, false); n != nil {
		return n
	}
	check.errorf(at, InvalidAssert, "strict: cannot assert named strict %s type %s: the interface may hold a nil %s", nilableKind(T), T, T)
	return T
}

// strictInstanceResults returns inst, the instance of the generic signature
// typ with targs, as a call in a strict file sees it. A generic function that is
// not strict may return the zero value of a type parameter, so a strict
// non-nil type argument is nullable in its results: the results are those of
// the instance with the nullable forms of such arguments. A named strict type
// has no nullable form, so it is an error as a type argument whose type
// parameter a result mentions.
func (check *Checker) strictInstanceResults(pos token.Pos, expr ast.Expr, typ *Signature, targs []Type, inst *Signature) *Signature {
	if typ.strict || !check.inStrictFile(pos) {
		return inst
	}
	var loose []Type
	for i, t := range targs {
		if !check.isStrictNonNil(t) {
			continue
		}
		tpar := typ.TypeParams().At(i)
		if !isParameterized([]*TypeParam{tpar}, typ.results) {
			continue
		}
		n := strictForm(t, false)
		if n == nil {
			check.errorf(expr, InvalidStrict, "strict: cannot instantiate %s with named strict type %s: the function is not strict and may return a nil %s", tpar, t, t)
			return inst
		}
		if loose == nil {
			loose = slices.Clone(targs)
		}
		loose[i] = n
	}
	res := *inst
	res.strict = true // an instance names a declared function, which is never nil
	if loose != nil {
		res.results = check.instance(pos, typ, loose, nil, check.context()).(*Signature).results
	}
	return &res
}
