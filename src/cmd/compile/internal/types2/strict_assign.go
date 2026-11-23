// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import . "internal/types/errors"

// checkStrictTarget reports a strict-mode error if x is stored into a target
// of type T that is a strict non-nil type (see isStrictNonNil), the operation is
// in a strict file, and x is not proven non-nil (Rule 2). context describes the
// operation, as in check.assignment, and code is the error code to report. It
// reports whether an error was reported.
//
// x must still describe the value being stored: nil is recognized by its mode,
// so callers pass the operand before any step that changes that mode.
//
// A composite T is checked too: x must not have a nullable pointer where T has
// a strict pointer (see strictLostIn), because type identity ignores strictness.
func (check *Checker) checkStrictTarget(x *operand, T Type, code Code, context string) bool {
	if !check.inStrictFile(x.Pos()) {
		return false
	}
	if x.isNil() {
		if check.isStrictNonNil(T) {
			check.errorf(x, code, "strict: cannot use nil as %s value in %s", T, context)
			return true
		}
		return false
	}
	if check.isStrictNonNil(T) && !check.isStrictNonNil(x.typ()) && !x.nonNull && !boxes(x.typ(), T) {
		check.errorf(x, code, "strict: cannot use %s as non-nil %s value in %s", x, T, context)
		return true
	}
	if check.strictLostIn(x.typ(), T) {
		check.errorf(x, code, "strict: cannot use %s as %s value in %s: it holds nullable %ss where %s holds non-nil %ss", x, T, context, nilableKind(T), T, nilableKind(T))
		return true
	}
	return false
}

// boxes reports whether storing a value of type V into the interface type T
// yields a non-nil interface even if the value is nil. The interface holds the
// dynamic type of a concrete value, so only an interface-typed V can be a nil
// interface. A type parameter may be instantiated with an interface type, so it
// does not box.
func boxes(V, T Type) bool {
	return isNonTypeParamInterface(T) && !isNonTypeParamInterface(V) && !isTypeParam(V)
}

// strictLost reports whether a value of type V may hold a nil pointer where
// type T promises a non-nil one: V is nullable and T a strict non-nil type, or
// the same holds inside them (see strictLostIn).
func (check *Checker) strictLost(V, T Type) bool {
	if check.isStrictNonNil(T) && !check.isStrictNonNil(V) {
		return true
	}
	return check.strictLostIn(V, T)
}

// strictLostIn reports whether strictLost holds for a pair of corresponding
// component types of V and T: pointer bases, slice, array, channel and map
// elements, map keys, struct fields, function results, the methods of an
// interface T, and the type arguments of two instances. Function parameters
// are checked in the other direction: a function that expects non-nil
// arguments must not be used where callers may pass nil.
// V and T are assumed to be assignable, so their structures match.
func (check *Checker) strictLostIn(V, T Type) bool {
	V, T = Unalias(V), Unalias(T)
	vn, _ := V.(*Named)
	tn, _ := T.(*Named)
	if vn != nil && tn != nil && vn.Origin() == tn.Origin() {
		vargs, targs := vn.TypeArgs(), tn.TypeArgs()
		for i := 0; i < targs.Len() && i < vargs.Len(); i++ {
			if check.strictLost(vargs.At(i), targs.At(i)) {
				return true
			}
		}
		return false
	}

	switch t := T.Underlying().(type) {
	case *Pointer:
		if v, _ := V.Underlying().(*Pointer); v != nil {
			return check.strictLost(v.base, t.base)
		}
	case *Slice:
		if v, _ := V.Underlying().(*Slice); v != nil {
			return check.strictLost(v.elem, t.elem)
		}
	case *Array:
		if v, _ := V.Underlying().(*Array); v != nil {
			return check.strictLost(v.elem, t.elem)
		}
	case *Chan:
		if v, _ := V.Underlying().(*Chan); v != nil {
			return check.strictLost(v.elem, t.elem)
		}
	case *Map:
		if v, _ := V.Underlying().(*Map); v != nil {
			return check.strictLost(v.key, t.key) || check.strictLost(v.elem, t.elem)
		}
	case *Struct:
		if v, _ := V.Underlying().(*Struct); v != nil && len(v.fields) == len(t.fields) {
			for i, f := range t.fields {
				if check.strictLost(v.fields[i].typ, f.typ) {
					return true
				}
			}
		}
	case *Signature:
		if v, _ := V.Underlying().(*Signature); v != nil && v.params.Len() == t.params.Len() && v.results.Len() == t.results.Len() {
			for i := 0; i < t.params.Len(); i++ {
				if check.strictLost(t.params.At(i).typ, v.params.At(i).typ) {
					return true
				}
			}
			for i := 0; i < t.results.Len(); i++ {
				if check.strictLost(v.results.At(i).typ, t.results.At(i).typ) {
					return true
				}
			}
		}
	case *Interface:
		for _, m := range t.typeSet().methods {
			obj, _, _ := lookupFieldOrMethod(V, true, m.pkg, m.name, false)
			if f, _ := obj.(*Func); f != nil && check.strictLostIn(f.typ, m.typ) {
				return true
			}
		}
	}
	return false
}

// strictRecv reports a strict-mode error if the method m, selected on x
// through the field index path index, may get a nil receiver. If the path crosses embedded fields, the receiver is the last
// embedded field, and x is only dereferenced. It reports whether an error was
// reported.
func (check *Checker) strictRecv(x *operand, index []int, m *Func) bool {
	recv := m.typ.(*Signature).recv
	// An interface method has no strict receiver. The call dispatches on the
	// interface value, and the method it reaches is not the checker's to prove
	// non-nil. A method found through a type parameter's constraint is one.
	if recv == nil || isNonTypeParamInterface(recv.typ) {
		return false
	}
	var emb Type // type of the last embedded field on the path, if any
	T := x.typ()
	for _, i := range index[:len(index)-1] {
		if p, _ := T.Underlying().(*Pointer); p != nil {
			T = p.base
		}
		s, _ := T.Underlying().(*Struct)
		if s == nil || i >= len(s.fields) {
			return false
		}
		emb = s.fields[i].typ
		T = emb
	}
	if emb == nil {
		// A non-pointer x is addressed implicitly.
		if _, ok := Unalias(x.typ()).(*Pointer); !ok {
			return false
		}
		return check.checkStrictTarget(x, recv.typ, IncompatibleAssign, "method receiver")
	}
	if !isNilable(emb) || !check.inStrictFile(x.Pos()) || !check.isStrictNonNil(recv.typ) || check.isStrictNonNil(emb) {
		return false
	}
	check.errorf(x, IncompatibleAssign, "strict: cannot call method %s through embedded field of nullable type %s", m.name, emb)
	return true
}

// strictPayloadRead adjusts x, the result of reading field f of a value of
// type T through the field index path index, if f is an enum payload field of
// zero-unsafe type and the read is in a strict file. The payload of an inactive
// variant holds zero values, and a read does not check the tag. So a payload of
// strict pointer type *E reads as a nullable, unaddressable *E, which the code
// must compare with nil. A payload of any other zero-unsafe type cannot be
// read; strictPayloadRead reports an error and returns true.
func (check *Checker) strictPayloadRead(x *operand, T Type, index []int, f *Var) bool {
	if f.name == "_tag" || !check.inStrictFile(x.Pos()) || !check.isZeroUnsafe(f.typ) {
		return false
	}
	var s *Struct // the struct that declares f
	for _, i := range index {
		if p, _ := T.Underlying().(*Pointer); p != nil {
			T = p.base
		}
		s, _ = T.Underlying().(*Struct)
		if s == nil || i >= len(s.fields) {
			return false
		}
		T = s.fields[i].typ
	}
	if s == nil || s.enumInfo == nil {
		return false
	}
	if n := strictForm(f.typ, false); n != nil {
		x.mode_ = value
		x.typ_ = n
		return false
	}
	check.errorf(x, MissingFieldOrMethod, "strict: cannot read enum payload field %s of zero-unsafe type %s: its variant may be inactive", f.name, f.typ)
	return true
}
