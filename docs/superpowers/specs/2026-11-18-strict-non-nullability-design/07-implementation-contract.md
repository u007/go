# Strict non-nullability: Phase 1 implementation contract

Status: **Phase 1 and Phase 3 func and interface types implemented** in `syntax`, `types2`, `go/parser`, and `go/types`. Predeclared `Option[T]` is still open. Known gaps are in `TODO.md`.
Bare file names are in `src/cmd/compile/internal/types2/`.

This part fixes the rules that every Phase 1 check shares. It refines the
operand rule in the representation part, after the foundation was built.

## Terms

- Strict file: a file with `//go:strict` before `package`.
  `check.inStrictFile(pos)` answers this.
- Strict non-nil type: `check.isStrictNonNil(t)` (Phase 3 name; Phase 1 called it
  `isStrictPointer`). An unnamed pointer, function, or interface type written in a
  strict file, or a named type with such an underlying type declared in a strict
  file.
- Zero-unsafe type: `check.isZeroUnsafe(t)`.
- Nullable pointer: any pointer type that is not a strict pointer. Phase 3 extends
  the term to function and interface types: any value of such a type that is not
  a strict non-nil type is nullable.

## Rule 1: only strict files get diagnostics

Report a strict-mode error only when the offending construct is positioned in a
strict file. Code in a non-strict file never gets a new error, even if it uses a
type declared in a strict file.

## Rule 2: non-nil is decided by type, plus narrowing

An operand is proven non-nil in these cases:

1. Its type is a strict non-nil type: a strict pointer, function, or interface
   type. Every producer of a strict type is checked, so the type carries the
   proof. This covers variables, fields, call results, `&x`, `new(T)`, and
   composite literals in strict files, and function literals in strict files.
2. It is an identifier for a local variable of nilable type that narrowing has
   proven non-nil at this point. The flow checker sets `operand.nonNull` for
   this case, and for no other.
3. It is a function value named in a strict file: a declared function, a method
   value, or a method expression. Such a value is never nil, so the checker gives
   it the strict form of its signature (`strictFuncValue`, and `sig.strict` for
   method values and expressions). This holds whichever file declares the
   function.

This replaces the earlier list that treated calls and selectors as nullable.
With the type-level bit, that list made every strict call result unusable.

## Rule 3: the strict target check

A strict target is an assignment destination whose type is a strict non-nil
type: a variable, field, element, parameter, result, or channel element.

When the assignment is in a strict file and the target type is a strict non-nil
type, the value must be proven non-nil by Rule 2. Untyped `nil` is always
rejected. One exception: a value of a concrete type, meaning neither an
interface nor a type parameter, stored into a strict interface target is
boxed with its dynamic type, so the interface is never nil. It needs no proof.
An interface-typed value, or a type-parameter value, is still checked.

A `:=` declaration takes its type from the value. `f, err := os.Open(name)`
makes `f` a nullable pointer, with no error. The error comes later, if `f`
reaches a strict target without narrowing.

Dereferencing a nullable pointer was not an error in Phase 1. It now is, for
local variables and parameters; see part 03, "Use of a nullable variable".

## Rule 4: messages and codes

- Every strict-mode message starts with `strict: `.
- Use the closest existing code in `internal/types/errors`. Add no new codes in
  Phase 1.

## File ownership during the parallel build

| Area | Files |
|---|---|
| Strict target check | `assignments.go`, `call.go`, `conversions.go`, `expr.go`, new `strict_assign.go` |
| Zero-value bans, literal completeness, `recover` | `builtins.go`, `index.go`, `range.go`, `signature.go`, composite-literal code in `literals.go`, package-level part of `varDecl` in `decl.go`, new `strict_zero.go` |
| Flow facts | `stmt.go`, `typexpr.go`, `declStmt` in `decl.go`, `funcLit` in `literals.go`, new `strict_flow.go` |
| Function facts | `stmt.go`, `assignments.go` (return values), `recording.go` (`funcUses`), `return.go`, `call.go` (selectors, call arguments), `expr.go` and `index.go` (dereferences), new `strict_facts.go`; export: `noder/writer.go` and the importers |

Each area has its own testdata file under `testdata/strict/`.
