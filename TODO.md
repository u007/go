# TODO — Tagged Enums (Phase 1)

Status: Phase 1 is implemented end to end and verified — parser, both type
checkers, compiler lowering, auto-generated constructors and tag constants, the
tooling bridge, and tests. Remaining items are upstream/maintenance concerns.

## Constructors and tag constants (Step 3) — DONE

`cmd/compile/internal/noder/enumsyn.go` synthesizes, before type checking:

```go
func Shape_Circle(p0 float64) Shape { return Shape{_tag: 0, _Circle_0: p0} }
const _Shape_Circle uint8 = 0
```

plus generic forms (`func Option_Some[T any](p0 T) Option[T]`). Verified
same-package and cross-package (run + vet).

### Tooling bridge
`go/types` cannot run the noder, so `go/types/enum.go`'s
`enumConstructorDecls` builds signature-only `*ast.FuncDecl` / tag `const`
nodes. `collectObjects` declares them with the declaring file's own
declarations, so `go vet`/gopls resolve them. The caller's AST is not changed
(`TestEnumConstructorBridgeLeavesAST`, `src/go/types/enum_test.go`). The
synthesized identifiers are in `Info.Defs` but not in the AST.

## Tests

- Shared checker testdata: `src/internal/types/testdata/check/enums.go`
  (basic/generic/pointer+type-param variants, duplicate and empty-enum errors,
  `enum`-as-identifier compatibility).
- Branch coverage: `TestEnumTooManyVariants` in
  `src/cmd/compile/internal/types2/enum_test.go` and `src/go/types/enum_test.go`
  (the >255-variants path, impractical as testdata).
- Parser: `src/cmd/compile/internal/syntax/testdata/enums.go`,
  `.../enum_compat.go`. gofmt golden: `src/cmd/gofmt/testdata/enum.*`.

## Upstream dependency patch (NOT submitted)

- `src/vendor/golang.org/x/tools/go/ast/{edge/edge.go,inspector/walk.go}` carries
  the inspector support for `ast.EnumType` / `ast.EnumVariant`.
- The upstream-submittable change is `docs/patches/x-tools-enum-inspector.patch`.
- `internal/gcimporter/predeclared.go` (in `src/vendor` and `src/cmd/vendor`)
  lists the predeclared `Option` in the unused slot of gc's internal `any`.
- `internal/gcimporter` (`iexport.go`, `iimport.go`, `ureader.go`) and
  `internal/pkgbits` carry the strict bit of a type; `unitchecker` reads
  `Strict` from the vet config. Patch for all of these and for `Option`:
  `docs/patches/x-tools-strict.patch`.
- `src/cmd/vendor/golang.org/x/mod/modfile/rule.go` parses the `strict`
  statement of `go.mod`. Patch: `docs/patches/x-mod-strict.patch`.
- `moddeps` reports all of these files.
- **Nothing has been pushed to `golang.org/x/tools`** — this is an offline
  worktree. Land the patch upstream, bump the dependency, and re-vendor.
- Confirmed: non-short `cmd/internal/moddeps` (`TestAllDependencies`) **fails**,
  and the diff it reports is exactly those two vendored files. `go mod vendor`
  would revert them. Both are expected until upstreamed.

## Build environment

- `src/go.mod` / `src/cmd/go.mod` are at `go 1.28` (diff empty), built with the
  go1.28 bootstrap.
- `src/cmd/dist/build.go` and `src/cmd/go/internal/cfg/cfg.go` worktree hacks
  were **reverted** (diff empty). The tree rebuilds and passes all checks
  without them; `GOTOOLCHAIN=local` must be passed on the command line instead.
- `VERSION` (untracked) is required by `make.bash` (`devel go1.28-tagged_enums`).
  `Makefile` (untracked) is an optional build/install wrapper. Neither is part
  of the requested feature; do not stage them with the feature change.

## Build / verification

- Build: `cd src && env -u GOROOT -u GOFLAGS GOROOT_BOOTSTRAP=/Users/james/sdk/go1.28-tagged_enums GOTOOLCHAIN=local ./make.bash`
- **Always run the built toolchain with `GOTOOLCHAIN=local`.** The host shell
  exports `GOTOOLCHAIN=go1.27.4`, which silently re-execs the stock SDK.

## Strict non-nullability (`//go:strict`) — Phase 1

Spec: `docs/superpowers/specs/2026-11-18-strict-non-nullability-design/INDEX.md`.
Implemented in `cmd/compile/internal/syntax` (directive) and
`cmd/compile/internal/types2` (`strict*.go` plus hooks). Tests:
`types2/testdata/strict/`, `TestStrict`, `TestStrictHelpers`,
`test/strictslice.go` (run-time reslice bound).

### Project-level strict mode

- Strict is the default: every Go file of a main module at go 1.28 or later
  is strict. `strict off` in `go.mod` turns it off for the module, and a
  `//go:nostrict` line before `package` exempts one file. `cmd/go` passes
  `-strict` to the compiler and `Strict` in the vet config, for packages of
  main modules only. GOROOT, dependencies, and modules before go 1.28 stay
  non-strict. Generated files (`_testmain.go`, `_cgo_gotypes.go`) carry
  `//go:nostrict`. Test: `cmd/go/testdata/script/mod_strict.txt`.
- Older Go toolchains reject a `go.mod` that contains `strict off`.
- gopls and other tools that load packages without `cmd/go` do not know the
  module default. They treat a file without `//go:strict` as non-strict.
- Pre-existing, unrelated: `cmd/go` script test `version_goexperiment` fails
  because `VERSION` holds `devel go1.28-tagged_enums`, which
  `go/version.IsValid` rejects.
- Strictness crosses packages. Export data marks a pointer, function, or
  interface type that a strict file writes (`pkgbits.TypeStrict`; `strictType`
  in vet fact files). Only packages with strict types use the new code, so
  export data of other packages is unchanged. Test: `test/strictpkg.go`.
- An imported named type is strict if its underlying type literal is. This
  differs from the local rule (declaring file) for `type P Q` where `Q` comes
  from a file of the other strictness.
- Open: a `strict` line in the `go.mod` of a dependency is ignored. A tool
  built with an unpatched `x/tools` cannot read export data of a strict
  package.

- Generic functions: a call in a strict file of a generic function that is
  not strict treats a strict type argument as nullable in the results
  (`strictInstanceResults`): such a function may return the zero value of a
  type parameter, as `cmp.Or[*T]()` does. A named strict type argument there
  is an error. Each function's declared strictness is exported as a trailing
  bool of its object record (readers that do not know it stop before it), and
  as tag `'S'` for strict generic functions in vet fact files.
- Soundness tests: `types2/testdata/strict/soundness/` (a non-strict library
  file whose functions return nil on rare paths: named and deferred results,
  recover, recursion, package variables, function values, closures,
  interfaces, method values, generics, error results, fields, collections,
  aliasing, reassignment). Every unchecked use must stay an error.

- Library nil warnings (Phase 1 of the library-warning plan): in a strict
  file, a use that needs a value (selector, dereference, index of a pointer to
  an array, call of a function variable) of a nullable value that is not a
  local variable — a field, a package variable, a map or slice element, or a
  call result — reports `strict: warning: X may be nil; assign it to a
  variable and check it != nil`. Code `NilWarning`: the checker passes it to
  the Error handler but never returns it from Check and never stops on it.
  The compiler prints it with `base.WarnfAt`; `cmd/go` replays it from the
  build cache; vet prints it to stderr and exits 0. Tests: `use.go`
  (`useUntracked`). A tool that treats every reported error as fatal (gopls
  shows them as errors) sees warnings as errors.
- Function facts (Phase 2): both checkers compute, for every function and
  method declared with a body in every file (std and dependencies too), two
  facts once all bodies are checked (`strict_facts.go`): *never returns* (no
  return or defer statement, and the body ends in a terminating statement,
  where a call that never returns terminates; least fixed point; seeds
  `runtime.Goexit`, `syscall.Exit`) and *result i never nil* (every return
  proves it: an address, a function literal, a concrete value boxed in an
  interface, a strict or narrowed value, or a call of a function whose result
  is never nil; greatest fixed point; none with a defer statement or a bare
  return). Strict code uses the facts of imported functions: a call statement
  that never returns ends the flow (`if p == nil { t.Fatal() }` narrows), and
  a call whose result is never nil needs no check (`errors.New(x).Error()`).
  Facts travel as a trailing `uint64` of each function record and after the
  methods of each type record in the unified export data, and after each
  signature in vet's own fact files (std `vendor` copy only, so the
  `go list -export` format stays upstream). The build cache keys on export
  data, so a fact change rebuilds dependents. Tests: `TestFuncFacts` (both
  checkers), `test/strictfacts.go`.
  - Gap: a package does not use its own facts (they are computed after its
    bodies are checked), so a same-package helper that never returns does not
    narrow, and a same-package constructor still warns.
  - Gap: no facts for generic methods (their records are read before the
    trailing data), for multi-value assignments (`p, err := f()` does not
    narrow `p` even if result 0 is never nil), or for functions with any
    defer statement.
- Parameter facts (Phase 3): a third fact, *parameter i is dereferenced*: a
  call panics if it is nil. The checker records the uses of a parameter that
  panic on nil while checking a body (`factsDeref`, `factsArgs`), then reads
  the top-level statements in order: the first statement that mentions the
  parameter must surely evaluate such a use, or pass the parameter to a
  dereferenced parameter (least fixed point). Any statement other than a
  simple one ends the scan after its header. In a strict file, an argument
  that may be nil for a dereferenced parameter of an imported function is a
  warning naming the function, the parameter, and the position of the use
  that panics (for a parameter passed on, the use in the callee). Facts
  share the `uint64`: results 0–30, parameters 0–31; the positions follow
  the facts in export data (`FuncDerefs`). Tests: `TestFuncFacts`,
  `test/strictfacts.go` (`params`).
  - Gap: must-dereference only. A dereference after an early return or inside
    a branch (`if c { return }; p.N`) gives no fact, so no warning.
  - Gap: vet's fact files keep lines but not columns (x/tools `iimport`
    uses a line-only file set), so vet prints `print.go:298:1` where the
    compiler prints `print.go:298:11`, and vet prints the absolute GOROOT
    path where the compiler prints `$GOROOT`.
  - Gap: no receiver facts, so a pointer-receiver method call on a parameter
    (`p.M()`) does not dereference it, and a value-receiver method call is not
    counted either. Slice and map parameters get no fact (nil is valid for
    reads). Functions with any defer statement get none.

### Known soundness gaps (not fixed)

- **Non-strict code writes nil into strict data it receives.** Generic
  standard library functions make this reachable: since Go 1.22,
  `slices.Delete`, `Compact`, and `Replace` zero the tail of the caller's
  backing array, so strict code that keeps the original slice reads nil from
  a strict `[]*T` (confirmed: `s := []*T{a, b}; slices.Delete(s, 0, 1);
  s[1].M()` crashes). A generic that writes the zero value through a `*P`
  parameter does the same. Candidate for the planned library warnings.

- **Non-strict code.** Non-strict files get no strict errors. They can pass
  `nil` to a strict function, write `nil` into a strict slice they receive, or
  instantiate a generic function that makes zero values. This is the design
  boundary (Rule 1); only a runtime check at the boundary can close it.

### Deferred

- API check: `go test cmd/api -run TestCheck` fails on 22 new exported names:
  `go/ast` `EnumType`, `EnumVariant` (fields and methods), `File.Strict`; and
  `go/types` `EnumInfo`, `EnumVariantInfo`, `Struct.EnumInfo`,
  `Struct.SetEnumInfo`, `IsStrict`, `MarkStrict`; and `go/ast`
  `File.NoStrict`. To close it:
  1. Open an issue on our own repository for the new API (one issue, or one
     for enums and one for `//go:strict`). No accepted upstream proposal exists.
  2. Add `api/next/<issue>.txt` with the 22 lines the test prints, each ending
     in `#<issue>`.
  3. If the work goes upstream, rename the file to the real proposal number.
  The check does not look the number up; it only matches file name and suffix.
- Enum switch (`docs/tagged-enums.md`) limits:
  - Enum information crosses packages in the `go:enum` struct tag of `_tag`
    (`enumInfoOfTag`). Both checkers reject that tag key in source. A struct
    built through the `NewStruct` API with that tag is still taken for an enum.
  - `types2` rewrites the statement and `go/types` checks it directly. Keep the
    two `enumswitch.go` files in sync; the shared testdata guards the errors.
- Phase 3, predeclared `Option[T]`: done for construction, enum switch, and
  cross-package use (`docs/tagged-enums.md`, "Predeclared `Option[T]`").
  It is the nullable syntax in strict code: its zero value is `None`, an enum
  switch gives a strict payload, a nullable value cannot enter a strict
  `Option`, and `Some` of a narrowed variable has a strict payload
  (`types2/testdata/strict/option.go`).
  Open:
  - A tool built with an unpatched `x/tools` (gopls, staticcheck) fails on
    export data that mentions `Option`. See the upstream patch section.
  - Written off: a bare-pointer layout for `Option[*T]`. It stays a tag plus a
    pointer.
  - The universe names are not in `api/next/`.
- Phase 3, func and interface types: done (spec parts 01, 02, 03, 05, 07).
  A func or interface type written in a strict file is strict, with the pointer
  rules: nil is not assignable, nullable values need a nil check first,
  zero-value rules apply, and `error` is exempt. Predicate:
  `check.isStrictNonNil`. Open:
  - `any` and the empty interface literal are exempt, as `error` is. A named
    interface type declared in a strict file is strict.
  - Use check (spec part 03, "Use of a nullable variable"): a local variable
    or parameter of nullable type must be proven non-nil before a selector, a
    dereference, an index, or a call. A value assigned from a proven non-nil
    expression needs no check. Tests: `types2/testdata/strict/use.go`.
    Not checked: expressions other than variables (`os.Stdout.Close()`,
    `f().x`), package-level variables, and functions that contain `goto`.
  - A type-parameter value is nullable when stored in a strict interface. It
    needs a nil check first.
  - Method calls on an interface, including a method found through a
    type-parameter constraint, have no receiver check.
  - Generic function instances take their strictness from the use site, not
    the declaration.
  - Written off: a named type from a non-strict package, such as `io.Reader`,
    stays nullable in a strict file (spec decision 5).
