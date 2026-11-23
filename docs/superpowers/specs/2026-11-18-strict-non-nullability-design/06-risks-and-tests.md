# Strict non-nullability: Soundness risks and tests

Status: **Phase 1 implemented.** Line numbers predate the implementation.
Bare file names are in `src/cmd/compile/internal/types2/`. `syntax/`, `noder/`, and `types2/` are under `src/cmd/compile/internal/`. `go/types/` is under `src/`.

## Soundness risks to carry into the plan

Pre-existing:
- Generics: `subst` and instantiation must carry the `strict` bit.
- Interface and method-value conversions.
- Narrowing across assignments between the check and the use.

Definite assignment (verified by reading unless marked):
- `isTerminating` counts `goto` and `fallthrough` as terminating
  (`return.go:38`). That is right for missing-return analysis and wrong for a
  join. Definite assignment cannot reuse it unchanged.
- `usage` is a post-hoc scope walk over `usedVars`. It has no flow order, so it
  cannot be extended. Definite assignment needs a new in-order pass.
- Closure bodies are checked after the enclosing statement (`literals.go:96`).
  Without a facts snapshot, a closure sees the wrong state.
- Loops: a read before assignment inside a body is rejected even if an earlier
  iteration assigned. This is sound and may surprise users.
- `recover` leaves every result at its zero value, named or not. A strict
  function returning `*T` can return nil after a recovered panic. Decided: a
  call to `recover` in a strict file is an error in Phase 1 (hook:
  `builtins.go:822`). This follows from Go semantics, not from reading this
  tree. A strict function can still call a non-strict helper that recovers.
- Slice reslicing past `len` exposes zero elements (`s[:cap(s)]`). Banning
  `make` does not close it, because `append` growth zero-fills. Closed with a
  run-time bound: in strict files such a reslice panics past `len`.
- Instantiating a type parameter with a strict pointer makes `var zero T` a nil
  strict pointer. Closed for strict files: such a type parameter is
  zero-unsafe. Generic code in non-strict files is still unchecked.

`Option[T]` (verified by reading unless marked):
- Universe scope can hold a generic named type only with extra work.
  `universe.go:85-117` builds `error`, `any`, and `comparable` by hand with
  `NewNamed` and no type parameters. No universe type is generic today.
- Instantiation needs `check.context` and a resolved `fromRHS`. `Named` created
  without a checker has none. Suspected; the instantiation path was not run.
- Universe objects have a `nil` package. The enum field builder uses
  `check.pkg` (`types2/enum.go:43`, `:64`). A universe `Option` has no checker, so its
  fields get a `nil` package. That keeps `_Some_0` unreachable, which is wanted.
  It also must not break `sameId` field lookup.
- Export data writes `nil`-package objects as package path `builtin`
  (`noder/writer.go:460`). The reader maps it to `types.BuiltinPkg`
  (`noder/reader.go:370`). Writer special-cases `any`, `comparable`, `rune`
  by name. A generic universe type with a struct underlying has no such path.
  Suspected; the reader side for named objects was not traced.
- The noder synthesizes constructors from `EnumType` declarations per file
  (`noder/enumsyn.go:32`). A universe `Option` has no declaration, so it gets
  none. `Some` and `None` must be built-in functions in the checker, like `new`.
  `None` needs target typing.
- `go/types` has its own synthesis (`go/types/enum.go:24`), hand-written. The
  universe and builtin changes mirror by generation. The enum code does not.
  Keep the two in sync.
- Lowering `Option[*T]` to a bare pointer changes type layout. That needs
  `noder` and possibly backend work. This conflicts with the Goal, which says
  `noder`, SSA, and `runtime` stay untouched. Recommended: ship Phase 3 with the
  ordinary enum struct lowering (tag plus payload). Treat the bare-pointer niche
  as a later optimisation outside this spec's no-noder promise.
- Tag 0 is the first variant (`types2/enum.go:50`). A user enum whose first
  variant holds a strict pointer has a nil-payload zero value. Such an enum is
  zero-unsafe. `var e E` needs definite assignment.
- `Option[T]` narrowing is not the same fact as nil narrowing. It narrows an
  enum value to a payload. The enum switch is that match form.

## Test suite

- New `types2/testdata/strict/` directory. `testDirFiles` needs no
  registration; `TestLocal` at `check_test.go:408` is the model.
- New `TestStrict` in `check_test.go`.
- `//go:strict` must be the first line, before `package` (`p.top`).
- Do not add tests to shared `internal/types/testdata`, because `go/types`
  also consumes it.
- Cases: nil to strict pointer; missing initializer; incomplete `T{}`;
  definite assignment (if/else, switch with and without default, loop, `goto`,
  closure capture, `&p`, field-by-field);
  unchecked `os.Open` result; positive narrowing; strict-to-nonstrict passing;
  cross-file boundary in one package.

## Verification

- `cd src && ../bin/go test cmd/compile/internal/types2`
- `cd src && ../bin/go build std cmd`
- Confirm no new diagnostics in non-strict packages.

