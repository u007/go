# Opt-in strict non-nullable pointers for the Go compiler frontend

Status: **Phase 1 and Phase 3 func and interface types implemented** in `syntax`, `types2`, `go/parser`, and `go/types`. Known gaps are in `TODO.md`.
Branch note: the current worktree/branch (`rust_style_tagged_enums`) holds
uncommitted Rust-style enum work. This is a **separate feature** and should get
its own branch/worktree before implementation.

## Goal

Add an opt-in mode that rejects nil-pointer misuse at compile time for packages
that opt in, without changing `noder`, SSA backends, or `src/runtime`, and
without breaking GOROOT standard libraries or unmodified third-party packages.

## Parts

| Part | Content |
|---|---|
| `01-activation-and-representation.md` | Activation, representation, and boundary |
| `02-enforcement-and-zero-values.md` | Enforcement rules and zero-value gaps |
| `03-flow-facts.md` | Flow facts: narrowing and definite assignment |
| `04-option.md` | Option[T] |
| `05-hook-sites.md` | Verified hook sites |
| `06-risks-and-tests.md` | Soundness risks and tests |
| `07-implementation-contract.md` | Phase 1 shared rules and file ownership |

## Phases

1. Pointers only: nil ban, zero-value ban, struct completeness, narrowing, and
   definite assignment for locals. Narrowing and definite assignment share one
   flow-state mechanism, so build them together.
2. Closures, goroutines, `switch` narrowing. Done; see part 03.
3. Func and interface types, and `Option[T]`. Func and interface types: done (see
   part 02, section "Phase 3"; `TODO.md`). Dependency order:
   1. Enum match and payload access. Done (enum switch, local and imported enums).
   2. Predeclared `Option[T]`, `Some`, `None`. Done (`docs/tagged-enums.md`).
   3. `Option[T]` as the nullable syntax, with narrowing after a check. Done.
   Enum constructors (`TODO.md` Step 3) are already done and do not block this.

## Resolved decisions

1. Nullable surface syntax: `Option[T]`, predeclared in the universe scope with
   `Some` and `None`. See part 04. Implementation waits on Phase 3.
2. Definite assignment for locals is allowed in Phase 1. See part 03.
3. `recover`: a call to `recover` in a strict file is an error in Phase 1.
4. Zero-value gaps: strict code bans them in Phase 1. See part 02.
5. Named-type strictness: a named type is strict if its declaring file is
   strict.
6. `error` is exempt. `any` and the empty interface literal are exempt too.
7. Activation: strict by default for main modules at go 1.28 or later
   (compiler flag `-strict`). Opt out per module with `strict off` in `go.mod`,
   or per file with `//go:nostrict`. Outside such modules, `//go:strict` opts a
   file in. See `TODO.md`, "Project-level strict mode".
8. Phase scope: implement Phase 1 first.
9. Location: Phase 1 is implemented in the `rust_style_tagged_enums` worktree,
   next to the enum work. This overrides the branch note above.

## Open decisions

None.
