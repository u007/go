# Strict non-nullability: Activation, representation, and boundary

Status: **Phase 1 implemented; Phase 3 func and interface types implemented.** Line numbers predate the implementation.
Bare file names are in `src/cmd/compile/internal/types2/`. `syntax/`, `noder/`, and `types2/` are under `src/cmd/compile/internal/`. `go/types/` is under `src/`.

## Activation

- Add `Strict bool` to `syntax.File` (`src/cmd/compile/internal/syntax/nodes.go`).
- In `syntax/parser.go`, detect the exact directive `go:strict` in the
  directive handler (currently lines 77-87 forward every `go:` text to
  `pragh`; line 421 attaches the file pragma).
  - Require `p.top` (must appear before `package`).
  - Reject duplicate directives and arguments.
  - Set `f.Strict` directly and do **not** call `pragh` for this directive.
    This matters because `types2/check_test.go:66` calls
    `syntax.Parse(base, r, errh, nil, mode)` with a nil pragma handler.
- Resolution: go with the directive first. Defer the optional `-strict` flag
  (`base/flag.go` + `gc/main.go`) so noder stays untouched.

## Representation

Do not wrap `Type`. Nullability has two independent parts.

### Type-level: strictness by declaration

Add `strict bool` to `Pointer` (`types2/pointer.go:11-13`).

- `Identical` compares only `base` (`predicates.go:314-318`), so the extra
  field does not change assignability or identity.
- Set the bit when a pointer type expression is written in a strict file.
- Copy the bit during substitution and instantiation.

All construction sites that must set or copy the bit (verified by grep,
non-test files):

| File:line | Site |
|---|---|
| `pointer.go:13` | `NewPointer` |
| `builtins.go:742` | `new` |
| `builtins.go:1033` | `unsafe` slice data |
| `builtins.go:1048` | `unsafe.String` arg |
| `builtins.go:1060` | `recordBuiltinType` |
| `builtins.go:1079` | `unsafe.String` pointer |
| `expr.go:147` | unary address-of `&x` |
| `expr.go:1213` | pointer expression |
| `named.go:527` | named pointer |
| `signature.go:316` | method receiver |
| `subst.go:142` | substitution |
| `typexpr.go:322` | type expression `*T` |

Phase 3 repeats the pattern for `Interface` (`interface.go:16`) and
`Signature` (`signature.go:20`). Status: **implemented.** The bit is set in
`funcType` (`signature.go`, from the `func` keyword position, so func types, func
declarations, literals, and interface methods all get it) and in `interfaceType`
(`interface.go`). `subst.go` copies it, and `typestring.go` writes it as `!` when
hashing (`strictMark`), so `Box[func()]` from strict and non-strict files are
separate instances. The shared predicate is `isStrictNonNil` (`strict.go`), which
replaces `isStrictPointer` and covers pointers, functions, and interfaces. The
bit never goes on an empty interface: `any` is exempt (`interfaceType`).

Named-type rule: a named type is strict if its **declaring file** is strict.
State this explicitly in the spec and tests.

### Operand-level: proven non-nil

Add `nonNull bool` to `operand` (`types2/operand.go`). Default is false
("possibly nil").

Set true only for:
- identifiers bound to known non-nil strict vars, params, and results
- `&x`
- `new(T)` where `T` is strict
- composite literals of strict pointer or strict struct type
- narrowed variables

Everything else counts as nullable: calls, selectors, index, receive, type
assertions, dereference, and untyped nil.

## Boundary model

Any pointer type declared in a non-strict file has `strict == false`, so it is
nullable. All imported results are therefore nullable. No export-data change is
needed. This intentionally drops cross-package strict-to-strict guarantees.

Superseded: export data now carries the strict bit of a type literal, so a
type written in a strict file of another package is strict here too. Types of
non-strict packages stay nullable. See `TODO.md`, "Project-level strict mode".

