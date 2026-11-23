# Strict non-nullability: Verified hook sites

Status: **Phase 1 implemented; Phase 3 func and interface types implemented.** Line numbers predate the implementation.
Bare file names are in `src/cmd/compile/internal/types2/`. `syntax/`, `noder/`, and `types2/` are under `src/cmd/compile/internal/`. `go/types/` is under `src/`.

## Verified hook sites

| Concern | File:line |
|---|---|
| `assignment` (nil + nullable→strict) | `assignments.go:21` |
| `varDecl` missing initializer | `decl.go:375` |
| `shortVarDecl`, `initVar`, `initVars`, `assignVars` | `assignments.go` |
| return statements | `return.go` |
| call arguments | `call.go` |
| struct/array/slice/map literals (recursive) | `literals.go:156-260` |
| `IfStmt` narrowing | `stmt.go:597-617` |
| range, send, select recv, type-switch bindings | `stmt.go`, `range.go` |
| `new`, `make`, `append`, `(*T)(nil)` | `builtins.go`, `expr.go` |
| strictness lookup map | `check.go:141,261-335` (mirror `check.versions`) |
| Per-function flow state: create before the body, discard after | `stmt.go:32-53` (`funcBody`: environment at `:32`, `stmtList` at `:41`, `usage` at `:53`) |
| Unused-variable pass; shows the existing post-hoc scope walk is not flow-sensitive | `stmt.go:56-84` (`usage`); `check.usedVars` at `check.go:146` |
| Statement driver; fork and join point for `if`, `switch`, `select`, `for` | `stmt.go:414`, `:597` (`if`), `:619` (`switch`), `:632` (`select`), `:675` (`for`) |
| Switch and type-switch clause loops | `stmt.go:724` (`switchStmt`), `:772` (`typeSwitchStmt`) |
| Read of a variable; where "read before assignment" is detected | `typexpr.go:107-113` (`case *Var` in `ident`) |
| Assignment target that is not a read; model for "assigned, not used" | `assignments.go:184-215` (`lhsVar`) |
| Local `var` declaration without initializer | `decl.go:774-800` (`declStmt` `VarDecl`), then `decl.go:375` |
| Address-of; where `&p` of an unassigned var is caught | `expr.go:130-147` (`unary`, `case syntax.And`) |
| Closure body checked later, after the enclosing statement; needs a facts snapshot | `literals.go:80-98` (`funcLit`, `check.later` at `:96`); `check.go:486` (`processDelayed`) |
| Termination analysis reused for joins | `return.go:16` (`isTerminating`), `:88` (`isTerminatingSwitch`), `:108-180` (`hasBreak` family) |
| `isTerminating` treats `goto` as terminating; unsafe for definite assignment | `return.go:38` |
| Label and `goto` pass; already rejects jumps over declarations | `labels.go:14` (`labels`), `:95` (`blockBranches`, `varDeclPos`); called from `stmt.go:44` |
| Named results (banned when zero-unsafe) | after `funcType` in `decl.go` (`funcDecl`) and `literals.go` (`funcLit`) |
| `recover` builtin | `builtins.go:822` (`_Recover`) |
| Predeclared types: `error`, `any`, `comparable` | `universe.go:75-117` (`defPredeclaredTypes`); lookups at `:287-289` |
| Enum type as the checker sees it: a `Struct` with `enumInfo` | `types2/enum.go:25-89` (`enumType`); `struct.go:20,56` |
| Enum variant tag assignment by declaration order | `types2/enum.go:50` |
| Enum constructor synthesis in the compiler | `noder/enumsyn.go:32`; called at `noder/noder.go:79` |
| Enum constructor synthesis for tooling | `go/types/enum.go:24`; called at `go/types/check.go:439` |
| Universe objects in export data | `noder/writer.go:454-460` (`pkgIdx`, `nil` pkg), `:485-487` and `:552` (special cases); `noder/reader.go:370` |

All new logic lives in a new `types2/strict.go`. Existing files get small hook
calls only. `go/types` mirrors `stmt.go`, `builtins.go`, and `universe.go`
by generation. `go/types/enum.go` is hand-written and needs a manual mirror.


## Phase 3 hook sites (implemented)

| Concern | File:line (types2) |
|---|---|
| Strict bit on a func type or a func declaration, literal, or interface method | `signature.go` `funcType` (`sig.strict` from the `func` keyword) |
| Strict bit on an interface type | `interface.go` `interfaceType` |
| Strict bit copied by substitution | `subst.go` (`*Signature` and `*Interface` cases) |
| `!` marker for instances | `typestring.go` `strictMark` |
| `any` and the empty interface literal are exempt | `interface.go` `interfaceType` |
| Declared function named in a strict file | `typexpr.go` `ident`, `*Func` case; `call.go` qualified `*Func` case (`strictFuncValue`) |
| Method value and method expression | `call.go` (`sig.strict`; `Signature` literal in the method-expression case) |
| Boxing and nilable checks on assignment | `strict_assign.go` `checkStrictTarget`, `boxes` |
| Receiver checks; interface methods skipped | `strict_assign.go` `strictRecv` |
| Narrowing of nilable variables | `strict_flow.go` `flowNarrowable` |

`go/types` mirrors each change by hand, except `strict_assign.go`, `subst.go`,
and `typestring.go`, which are generated.
