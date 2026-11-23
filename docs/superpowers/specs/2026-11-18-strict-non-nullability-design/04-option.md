# Strict non-nullability: Option[T]

Status: **Phase 1 implemented.** Line numbers predate the implementation.
Bare file names are in `src/cmd/compile/internal/types2/`. `syntax/`, `noder/`, and `types2/` are under `src/cmd/compile/internal/`. `go/types/` is under `src/`.

## `Option[T]`: the nullable surface syntax (Phase 3)

`Option[T]` is the only way to express a runtime "unset" state in strict code.
Use it for:
- struct fields filled at a later lifecycle stage
- lazy initialization
- package-level vars set in init paths
- map misses and collection slots

`Option`, `Some`, and `None` are predeclared. They live in the universe scope,
like `error` and `any`, not in a std package. Reasons:
- No import is needed.
- It is backward compatible. Universe names can be shadowed, as `any`, `min`,
  `max`, and `clear` were when added.
- The compiler must recognise the type to narrow after a `None` check.
- Phase 3 uses the ordinary enum struct lowering (tag plus payload). Lowering
  `Option[*T]` to a bare pointer is a later optimisation, because it needs
  `noder` work.
- A std package needs enum export and import. Both exist: export data carries
  the variants in a struct tag (`docs/tagged-enums.md`).

Surface forms. The match form is the enum switch (`docs/tagged-enums.md`):
an expression switch on an enum value with variant patterns as cases.

```go
type Server struct {
	cache Option[*Cache]   // unset until Start
}
s.cache = Some(newCache())
switch s.cache {
case Some(c): c.Warm()     // c is a strict *Cache here
case None:    return
}
```

Rules:
- Reading the payload requires a `None` check or a match first.
- After the check, the value narrows to the strict non-nullable type.
- The zero value of `Option[T]` is `None`. The declaration must list `None`
  first so it gets tag 0. `Some` first would make the zero value `Some(nil)`.
- A dummy placeholder such as `&Config{}` overwritten later is not the
  recommended pattern. It hides a missing value as a valid-looking one.

Names. The short forms `Some(x)` and `None` are the predeclared surface. Today
the noder synthesizes `Option_Some[T](x)` and `_Option_Some` for user enums.
The predeclared `Some(x)` means `Option_Some[T](x)` with `T` inferred. The
predeclared `None` has no `T` to infer. It needs the target type, like untyped
`nil`. The `Option_` names stay valid for user-declared enums only.

State today (verified by reading):
- Constructors and tag constants exist for user enums (`noder/enumsyn.go`).
- The enum switch exists for local and imported enums. It needs no new syntax
  node.
- Payload fields such as `_Some_0` are unexported. Code outside the declaring
  package reads them only through an enum switch.

Dependency order for Phase 3:
1. Enum match and payload access. Done, including imported enums.
2. Predeclared `Option[T]`, with `Some` and `None`. Done.
3. Use of `Option[T]` as the nullable syntax, including narrowing. Done: the
   enum switch is the check, and `Some` of a narrowed variable has a strict
   payload.

