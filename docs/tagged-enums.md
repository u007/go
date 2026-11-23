# Tagged Enums (Go 1.28 prototype)

## Syntax

```go
type Shape enum {
	Circle(float64)
	Rectangle(float64, float64)
	Point            // unit variant (no payload)
}

type Option[T any] enum { // generic enums are supported
	Some(T)
	None
}
```

## `enum` is a context-sensitive soft keyword

`enum` introduces an enum type **only in a type-declaration position, immediately
followed by `{`**. Everywhere else it is an ordinary identifier, preserving Go 1
compatibility:

| Code | Meaning |
|---|---|
| `type Shape enum { ... }` | enum type |
| `var enum = 5` | variable named `enum` |
| `type enum struct{ x int }` | defined type named `enum` |
| `type T enum` | `T` with underlying type `enum` |
| `type Q enum.Bar` | qualified type name `enum.Bar` |
| `func f() enum { ... }` | result type `enum`, then function body |

The recognition lives in the type-specification parser (`typeSpecType` /
`parseTypeSpecType`) of both the compiler (`cmd/compile/internal/syntax`) and the
tooling (`go/parser`). It is deliberately **not** in general type parsing, because
`enum` followed by `{` is otherwise ambiguous with a function body.

## Lowering

An enum lowers to a struct — no SSA or backend changes:

```go
type Shape struct {
	_tag      uint8
	_Circle_0 float64
	_Rectangle_0 float64
	_Rectangle_1 float64
	// Point contributes no fields
}
```

Variant metadata is stored on `types2.Struct` / `go/types.Struct` (`EnumInfo`:
name, tag, payload types) and also encoded in the `_tag` field tag
(`go:enum:"Circle:1;Rectangle:2;Point:0"`: each variant with its number of
payload values). Export data carries the tag, and `NewStruct` rebuilds
`EnumInfo` from it, so an imported enum keeps its variants.

## Version gate

Both checkers require `go 1.28+`:

```
./main.go:3:10: enum type requires go1.28 or later (-lang was set to go1.27; check go.mod)
```

## Tooling

- `go/ast` — `EnumType` + `EnumVariant` nodes; `Walk` traverses them.
- `go/parser` — parses enum type declarations.
- `go/printer` / `gofmt` — formats enum variants (golden test: `src/cmd/gofmt/testdata/enum.*`).
- `go/types` — type-checks enums (used by `go vet` and IDEs).
- Vendored `x/tools` inspector — patched for the new nodes (see `TODO.md`).

## Constructors and tag constants

For every variant the compiler synthesizes

```go
func Shape_Circle(p0 float64) Shape { return Shape{_tag: 0, _Circle_0: p0} }
const _Shape_Circle uint8 = 0
```

plus generic forms for generic enums (`func Option_Some[T any](p0 T) Option[T]`),
so construction works across packages. `go/types` / `go vet` / gopls see
equivalent signature-only declarations through a tooling bridge (see `TODO.md`).

## Enum switch

An expression switch whose tag has an enum type is an enum switch. Its cases
are variant patterns, not expressions.

```go
switch s {
case Circle(r):    // binds the payload
	return 3.14 * r * r
case Rect(w, _):   // _ ignores a payload value
	return w
case Point, Line:  // several variants; no bindings
	return 0
}
```

Rules:
- The switch must list every variant or have a `default` clause.
- A switch that lists every variant is a terminating statement.
- A pattern names a variant of the tag's type. No package qualifier is used.
- A pattern must give one name or `_` per payload value.
- A clause with several patterns cannot bind payloads.
- `fallthrough` is an error. An init statement is allowed.
- The tag expression is evaluated once.

Implementation:
- The compiler's checker (`types2/enumswitch.go`) rewrites the statement into
  a switch on the `_tag` field, with one payload read per binding. A switch
  that lists every variant gets a `default` clause that panics, so a corrupt
  tag is caught. The rewritten statement is checked and compiled as usual.
  The generated field reads resolve in the enum's package, so an imported enum
  works. The generated `panic` always means the predeclared function.
  If the switch has an init statement, the statement keeps it and becomes a
  tagless switch whose default clause holds the switch on the tag.
- `go/types` must not change the caller's syntax tree. It checks the clauses
  directly (`go/types/enumswitch.go`) and reports the same errors.
- Tests: `internal/types/testdata/check/enumswitch.go` (both checkers) and
  `test/enumswitch.go` (run).

## Predeclared `Option[T]`

`Option`, `Some`, and `None` are predeclared in the universe scope, like `any`.
They need go1.28 (`predeclared Option requires go1.28 or later`). A package or
local declaration with the same name shadows them.

- `Option[T]` is a generic enum lowered like a user enum: `_tag uint8` and
  `_Some_0 T`, with the tag `go:enum:"None:0;Some:1"`. `None` is tag 0, so the
  zero value of `Option[T]` is `None`.
- `Some(x)` is a builtin (`types2/builtins.go`, `_Some`). Its type is
  `Option[T]`, where `T` is the type of `x` after untyped defaulting.
  `Some(nil)` and `Some(None)` are errors. The noder writes the call as the
  composite literal `Option[T]{_tag: 1, _Some_0: x}` (`noder/writer.go`,
  `someLit`), which uses only existing forms.
- `None` is nil-like. It is an untyped value that takes the type of an `Option`
  target, and it is written as the zero value of that type (`exprZero`).
  `nil` is not assignable to an `Option`, and `None` is not assignable to a
  pointer, slice, or other nil-able type. `var x = None` has no target and is an
  error. `None` cannot be compared with `==`, as `nil` cannot be compared with
  a struct.
- Enum switches on `Option` work for the same reasons as for user enums. The
  universe type has a nil package, so `types2` and `go/types` resolve the
  generated `_tag` and payload selectors with that nil package.
- Export data has no format change. The writer does not stub the universe
  `Option`: `doObj` writes its full declaration in each package that refers to
  it, under the existing `builtin` path, and the reader declares it on first
  use (`typeExt` writes no pragma for it). Two other compiler changes make
  instances work:
  - `reflectdata/reflect.go` (`writtenByWriteBasicTypes`): a universe struct is
    not a basic type, so each package emits its type descriptor.
  - `noder/reader.go` (`exprZero`): the zero value of a struct is typechecked.
    `ir.NewZero` returns an untyped composite literal for structs.
- `go/types` mirrors all of it. `universe.go`, `builtins.go`, `operand.go` are
  generated from `types2`. `enum.go` (`isOption`, `isNone`), `expr.go`,
  `typexpr.go`, and `enumswitch.go` are hand-written mirrors.
- Tests: the shared checker tests `src/internal/types/testdata/check/option.go`,
  `option_shadow.go` (package-level shadowing), and `option1.27.go` (the go1.28
  gate, `-lang=go1.27`); `TestBuiltinSignatures` entries for `Some`; run test
  `test/option.go` with `test/option.dir/` (local and package-level shadowing,
  cross-package use, runtime type identity of instances).
- `src/builtin/builtin.go` documents `Option`, `Some`, and `None` (`go doc
  builtin.Option`).

Not done:
- No bare-pointer layout for `Option[*T]`. It is a tag plus a pointer. This
  optimisation is written off.
- The export writer in the vendored `x/tools` (`internal/gcimporter`) knows
  `Option` as a predeclared type (`predeclared.go`). It takes the unused slot
  of gc's internal `any`, because the table is full. `go vet`, `go test`, and
  `go list -export` work. A tool built with an unpatched `x/tools` (gopls,
  staticcheck) fails on export data that mentions `Option`.
- The universe names are not in `api/next/`.

## Limits (Phase 1)

- The struct tag key `go:enum` is reserved; a struct type that uses it is an
  error. A struct built through the `NewStruct` API with that tag is taken for
  an enum.

- An enum type must be declared at package level. Its constructors and tag
  constants are package-level declarations.
- If the noder checks a package a second time (`GOEXPERIMENT=simd`), the
  generated `panic` of an enum switch follows normal scoping, so a shadowed
  `panic` is used.
- Flat field layout, not a union: every variant's payload occupies its own fields.
- Constructor/tag synthesis exists in two places (compiler `noder` and the
  `go/types` tooling bridge); keep the naming scheme in sync if it changes.
