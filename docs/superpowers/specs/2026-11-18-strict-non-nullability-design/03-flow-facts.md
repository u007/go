# Strict non-nullability: Flow facts: narrowing and definite assignment

Status: **Phase 1 implemented; Phase 3 func and interface types implemented.** Line numbers predate the implementation.
Bare file names are in `src/cmd/compile/internal/types2/`. `syntax/`, `noder/`, and `types2/` are under `src/cmd/compile/internal/`. `go/types/` is under `src/`.

## Flow facts: narrowing and definite assignment (Phase 1)

Both features track a fact per local `*Var` along the control-flow paths of one
function body. One mechanism serves both. The two sections below define the
rules. The last subsection defines the shared state.

### Flow-sensitive narrowing

Required, because `f, err := os.Open()` yields a nullable `f` in strict code.
The fact is "proven non-nil".

Support in Phase 1:
- `if p != nil { ... }`
- `if p == nil { return }`
- `&&` short-circuit, also inside the condition: in `p != nil && p.x > 0` the
  right operand is checked with `p` non-nil, and in `p == nil || p.x > 0` too
- `||` on the false path: after `if err != nil || p == nil { return }`, `p` is
  non-nil
- `else`

Refuse to narrow when:
- the variable is reassigned in the narrowed block
- its address is taken (`&p`)
- its type has a method with a pointer receiver (`h.Clear()` takes `&h`)
- a function literal mentions it and the function binds that variable more
  than once

Phase 2 (implemented):
- A function literal, including one in a `go` or `defer` statement, keeps the
  narrowing of a captured variable. The rule is by variable, not by name: the
  variable must be bound exactly once in the enclosing top-level function. A
  declaration, an assignment (`=`, `op=`, `x++`, or a `:=` that assigns a
  variable already declared in the same block), a parameter or receiver, and a
  type switch guard each count once. A range variable counts twice, since it is
  bound per iteration. A named result counts twice, since a `return` assigns it.
  Another variable with the same name, in an inner block or a sibling block,
  does not count. The operand of `&` is never narrowed. A type switch's variable
  is treated as one variable: every clause shares the guard's position, so a
  binding in one clause excludes it in every clause. An enum switch's payload
  binding, such as `Err(e)`, is a variable bound once in its clause. Before
  checking, the pass cannot tell such a case from a call that takes a name, so
  a name in a case call is declared as that binding and also counts for the
  outer variable of the same name. That is conservative. The exclusions are
  computed before checking, by a scope-aware pass over the syntax tree that
  resolves each identifier to its declaring position (`narrowingExclusions`).
- An expression switch narrows per clause. A clause with one case expression
  gets that condition. A later clause and `default` get the negation of every
  earlier case. `switch p { case nil: ... }` works like `p == nil`.
- A clause entered by `fallthrough` starts from the join of its case entry and
  the end of the clause before it, so only facts that both paths establish hold.
  A `break` in it leaves with that state, less what the clause assigns.
  The clause before it keeps its own narrowing up to the `fallthrough`.
- A type switch and `select` add no narrowing, but keep the narrowing held
  before them in every clause. A `select` receive from a channel whose element
  is a strict pointer is rejected, since a closed channel yields nil. A receive
  from a channel whose element is non-strict, such as `net/rpc`'s `Call.Done`,
  is accepted, and its assignment kills the narrowing.

### Definite assignment for locals

Term: **definite assignment**. A compile-time proof that every path assigns a
local before its first read. It adds no runtime state and no nil check.

In a strict file, a local declared without an initializer is allowed if its type
is zero-unsafe. A type is zero-unsafe if it is a strict pointer, or a struct or
array containing a zero-unsafe type. An enum is zero-unsafe only if a payload of
its first variant is zero-unsafe.

```go
//go:strict
var p *Config           // allowed: no initializer
if useDefault {
	p = defaultConfig()
} else {
	p = load()
}
use(p)                  // ok: both branches assign
```

A read of `p` before it is definitely assigned is the error. Reads include `p.f`,
`*p`, passing `p`, and `&p`.

Join rules. All rules are conservative.

| Statement | `p` is assigned afterwards when |
|---|---|
| Sequence | any earlier statement assigns it on every path |
| `if` / `else` | both branches assign it, or the other branch terminates |
| `if` without `else` | never, unless the condition-false path already assigned it |
| `switch`, type switch | a `default` clause exists and every clause assigns it or terminates |
| `select` | every clause assigns it or terminates |
| `for`, `range` | never from the body; only the init and condition count |
| Terminating branch | excluded from the join |

Further rules:
- A clause with a `break` that targets its switch or select does not count as
  assigning. Existing `hasBreak` helpers answer this.
- A clause ending in `fallthrough` is excluded from the join. Each clause
  starts from the state at the switch entry, joined with the end of the clause
  before it when that clause falls through (see Phase 2).
- Terminating means `return`, `panic`, an infinite `for` without `break`, and
  the compound forms `isTerminating` already recognises. The exception is
  `goto`; see below.
- Only whole-value assignment counts: `p = v`, `p, q = f()`, `p := ...` forms.
  `s.f = v` does not initialize `s`. It reads `s`, so it is an error.
- Taking `&p` before `p` is definitely assigned is an error. Assigning through
  the pointer would otherwise bypass the proof.
- Capturing `p` in a closure before `p` is definitely assigned is an error, for
  reads, writes, and address-of. A closure assignment never counts for the
  enclosing function.
- `goto` rule: if a function body contains any `goto`, definite assignment is
  off in that function. A `var` of a zero-unsafe type without an initializer is
  then an error. A forward `goto` can skip an assignment. Go forbids only jumps
  over declarations (`labels.go`). This is the simplest sound rule.
- Applies to locals only. Package-level vars, struct fields, and collection
  elements cannot use it. A package-level var of a zero-unsafe type needs an
  initializer, or an `Option[T]` type.

### Relation between the two, and one shared mechanism

| Aspect | Narrowing | Definite assignment |
|---|---|---|
| Fact per `*Var` | proven non-nil | proven assigned |
| Starts as | unknown | unassigned, or assigned if the declaration has an initializer |
| Gained by | a nil check on a branch | a whole-value assignment |
| Lost by | reassignment, `&p`, closure write | never |
| Join at merge points | intersection | intersection |

One mechanism can serve both. Use one fact map per function body: `*Var` to a
small bit set with two bits. Use one fork-and-join driver in the statement
checker, shared by `if`, `switch`, `select`, loops, and terminating branches.
Join is bitwise AND at every merge point.

Differences the shared design must respect:
- Narrowing facts come from conditions, so they differ per branch. Assignment
  facts come from statements.
- Narrowing is killed by assignment. Assigned is never killed, because strict
  values cannot return to unset.
- Both facts persist after a block ends when the other path terminates, as in
  `if p == nil { return }`. A purely lexical scoped set cannot express this.
  The earlier draft's "scoped set" for narrowing is replaced by this map.
- A closure body is checked after the enclosing statement. It needs a snapshot
  of the facts taken at the closure expression, not the current state.
- Design both features together. Building narrowing first with a lexical set
  forces a rewrite when definite assignment arrives.


## Phase 3: narrowing of func and interface variables (implemented)

`flowNarrowable` accepts a local or parameter of nilable type. The nilable types
are pointer, function, and interface types, which is `isNilable`. A type
parameter is not nilable, although its underlying type is an interface. The
narrowing rules themselves are unchanged. A strict variable never needs
narrowing.

## Use of a nullable variable

A local variable or parameter of nullable pointer, function, or interface type
must be proven non-nil before a use that needs a value: a selector `v.x`, a
dereference `*v`, an index of a pointer to an array, or a call `v()`. The error
is `strict: v may be nil here; check v != nil first`.

A variable is proven non-nil when:
- a nil check narrows it (see above), or
- the value last assigned to it is proven non-nil: its type is strict, it is a
  narrowed variable, or it is a concrete value boxed in an interface, or
- it is the variable of a type switch clause with one interface type.

A result of a non-strict function is not proven, unless the facts of the
function prove it (see below). After `f, err := os.Open(p)`, a check of `err`
alone does not prove `f`; write `if err != nil || f == nil`.

Not checked: expressions other than variables, package-level variables, and
functions that contain `goto`. Cost: one map lookup per use, in strict files
only (`strictUse`, `flowProve` in `strict_flow.go`).

## Function facts

The checker computes six facts for every function and method declared with a
body, in every file, after all bodies of the package are checked
(`strict_facts.go`):
A deferred call *may recover* if the function it calls may call `recover`
directly: a function literal that mentions `recover`, `recover` itself, a
function or method without the fourth fact below, or a function value that is
not proven quiet. A function value is quiet if it is a function literal that
does not mention `recover`, a declared function or method with the fourth
fact, a call result with the fifth fact, or a local variable with exactly one
bound value that is quiet and whose address is not taken (`defer cancel()`,
`return cancel`). A parameter, a named result and a type switch variable are
not such a variable.
A directly deferred method of an interface (`defer x.Close()`) is assumed quiet,
except for the never-returns fact: its implementations are not known,
and strict code cannot call `recover`. A non-strict method whose body calls
`recover` itself breaks the assumption, and a fact may then hide a nil warning.
- **Never returns**: the body has no return statement and no deferred call
  that may recover, and ends in a
  terminating statement, where a call that never returns also terminates.
  Seeds: `runtime.Goexit`, `syscall.Exit`.
- **Result i is never nil**: the body has no deferred call that may recover,
  no defer statement if the results are named, and no bare return, and every
  return statement returns an address, a call of `new`, a function literal,
  a concrete value boxed in an interface, a strict or narrowed value, or a
  result of a function whose result is never nil.
- **Parameter i is dereferenced** (a call does not return normally if it is
  nil; the receiver of a method counts as a parameter): the body has
  no deferred call that may recover, and the first top-level statement that mentions the
  parameter surely evaluates a use that panics on nil (`*p`, a field through
  `p`, a method value of an interface `p`, an index of a pointer to an array,
  a call of `p`, a method value that loads `*p`), or passes it to a parameter
  or receiver that is dereferenced (not boxed in an interface on the way).
  An operand that is not evaluated (a constant `len` or `cap`, an `unsafe`
  function, a range over an array without an element variable) is no use. In a statement other than a simple
  statement, only the header is surely evaluated, and the scan ends after a
  statement that has a return or goto statement. A mention as `&p`, in a
  function literal, or in the right operand of `&&` or `||` decides nothing. A defer statement that mentions the parameter
  decides it without a use, and does not end the scan.
- **Never calls recover**: the body does not call `recover` directly, so a
  deferred call of the function never stops a panic.
- **Result i is a function that never calls recover**: under the conditions
  of the never-nil fact, every return statement returns a quiet function
  value for result i.
- **Parameter i may be zeroed**: the body calls `clear` on the parameter, a
  slice, or on a slice expression of it, or passes one to a parameter with
  this fact. Unlike the other facts it says what may happen, so it only adds
  warnings. Facts describe 10 results and 20 parameters.

Strict code uses the facts of imported functions, and of functions of its own
package whose bodies are checked before it. Each body's facts are set once it is
checked, with the facts known so far, and a body in a strict file first checks
the bodies of the package that it may call, found by name, and those bodies
do the same, whatever their files. In a cycle of calls, a body checked before
the body that it calls back does not know that body's facts; the facts of the
cycle are completed when that body is checked. Once all bodies
are checked, the facts are completed for the export data. A call statement that
never returns ends the flow, so `if p == nil { t.Fatal() }` narrows `p`. A call
whose result is never nil needs no check, also as one of several results, so `errors.New(s).Error()` gives no
warning. An argument that may be nil, or is `nil`, for a dereferenced
parameter is a warning (`NilWarning`) that names the
function, the parameter, and the position of the use that panics; export data
carries one position per dereferenced parameter. Facts travel in export data, so the build cache rebuilds dependents
when they change. An argument for a parameter that may be zeroed is a warning if its elements
must not be nil (`slices.Delete(s, 0, 1)` leaves nil in the old `s`), unless
the argument is a call or a composite literal, or the call is assigned back to
it: `s = slices.Delete(s, 0, 1)`, `x.s = f(x.s)`. A missing fact means nothing is known.
