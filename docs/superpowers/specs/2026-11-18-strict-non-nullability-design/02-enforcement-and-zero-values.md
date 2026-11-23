# Strict non-nullability: Enforcement rules and zero-value gaps

Status: **Phase 1 implemented; Phase 3 func and interface types implemented.** Line numbers predate the implementation.
Bare file names are in `src/cmd/compile/internal/types2/`. `syntax/`, `noder/`, and `types2/` are under `src/cmd/compile/internal/`. `go/types/` is under `src/`.

## Enforcement rules (Phase 1)

- Assigning `nil` to a strict pointer type is an error.
- Assigning a possibly-nil value to a strict non-nullable target is an error,
  unless the value is narrowed.
- A local `var p *T` without an initializer is allowed only if definite
  assignment proves `p` is assigned before its first read. Every other
  declaration of a zero-unsafe type without an initializer is an error.
- A type is zero-unsafe if it is a strict pointer, or a struct or array
  containing a zero-unsafe type.
- An enum is zero-unsafe only if a payload of its first variant is zero-unsafe.
  The zero value of an enum is its first variant.
- An enum literal must set the zero-unsafe payload of the variant its `_tag`
  selects. Payloads of other variants may stay unset.
- Enum constructors in a strict file are strict: `Opt_Some(nil)` is an error.
- A struct literal of a strict struct type must initialize every non-nullable
  field. This check recurses over struct fields and array elements.
- A non-nil strict value may be passed to a non-strict function. Passing a
  possibly-nil value into a strict target is the only rejected direction.
- A call to `recover` in a strict file is an error. A recovered panic returns
  zero results, which would be nil strict pointers.

## Zero-value gaps

Each form below can produce a zero value of a zero-unsafe type. The rule for
each is a ban or `Option[T]`. Decided: strict code bans the form in Phase 1
when the element or target type is zero-unsafe. Best-effort checking is
rejected, because a strict type that can still be nil cannot be trusted.

| Form | Rule |
|---|---|
| `var s S`, local | definite assignment |
| `var s S`, package level | error without an initializer; use `Option[T]` |
| `new(S)` | error |
| `make([]T, n)`, `make([]T, 0, n)` | error; build the slice with a literal or `append` |
| `make(chan T)` receive, closed-channel receive | error to receive; use `chan Option[T]` |
| map miss, `m[k]` or `v, ok := m[k]` | error to read; use `map[K]Option[V]` |
| array `[N]T` declaration | definite assignment with a complete array literal |
| nested structs and arrays | same rule as `var s S`, applied recursively |
| named results of a function declaration or literal | error; results must be unnamed. Function types and interface methods are exempt: they have no body |
| `(*T)(nil)` | error under the nil ban |

Why `Option[T]` fits: its zero value is `None`. A map miss, a fresh slot, or an
unset field is therefore a valid `None`, not a nil pointer.

Reslicing: `append` growth leaves zero elements between `len` and `cap`. In a
strict file, a slice expression with a high bound on a slice of zero-unsafe
element type panics at run time if the bound exceeds `len`. Shrinking forms
such as `s[:len(s)-1]` are unaffected. The checker marks the expression
(`Info.LenBounded`), and the compiler emits the bound check.

## Strictness inside other types

Type identity ignores strictness, so these rules apply in strict files:

- Composite types. A value is rejected if it holds a nullable pointer where
  the target type holds a strict pointer. The check covers pointer bases,
  slice, array, channel and map elements, map keys, struct fields, and type
  arguments. It applies to every strict target check and to `copy`.
- Function types. Results follow the composite rule. Parameters are checked
  in the other direction: a function with a strict parameter is rejected
  where the target type lets callers pass nil.
- Interfaces. A value is rejected for an interface target if one of its
  methods breaks the function rule against the interface's method.
- Generic instances. An instance keeps the strictness of its own type
  arguments. `Box[*T]` written in a strict file and in a non-strict file are
  identical types but separate instances.
- Type parameters. A type parameter of a strict file is zero-unsafe, unless
  its constraint lists specific types and none is zero-unsafe. So `new(T)`,
  `make([]T, n)` and an unassigned `var zero T` are errors for `T any`.
- Type assertions. An interface may hold a nil pointer. `x.(*T)` and a
  `case *T` clause variable yield a nullable `*T`; compare it with `nil`
  first. Asserting a named strict pointer type is an error.
- Enum payload reads. A read such as `e._Some_0` does not check the tag, and
  the payload of an inactive variant is zero. A payload of strict pointer type
  `*T` reads as a nullable, unaddressable `*T`. A payload of another
  zero-unsafe type cannot be read. Writes are checked against the declared
  type. An enum switch checks the tag, so its payload bindings keep their
  declared types: a strict pointer payload is non-nil there.
- Embedded pointers. A method call through an embedded field passes that
  field as the receiver. The call is an error if the field is a nullable
  pointer and the receiver is strict.


## Phase 3: func and interface types (implemented)

Status: **implemented** in `types2` and `go/types`. A func or interface type
written in a strict file is strict, and it follows the pointer rules above: nil
is not assignable to it, a nullable value needs a nil check before it is used as
non-nil, the zero-value rules apply, and `error` is exempt. The checks use the
shared predicate `isStrictNonNil`. Differences from pointers:

- **Boxing.** A value of a concrete type, meaning neither an interface nor a type
  parameter, that is stored in a strict interface needs no proof. The interface
  holds the value's dynamic type, so it is never nil. Only an interface-typed
  value, or a type-parameter value, must be narrowed first.
- **Function values named in a strict file are non-nil.** A declared function, a
  method value, a method expression, or a function literal takes the strict form
  of its signature when it is named in a strict file, whichever file declares it
  (`strictFuncValue`). A generic function's instance takes the strictness of the
  value it is instantiated from.
- **Interface method calls have no strict receiver.** The call dispatches on the
  interface value, so `strictRecv` does not check it. That includes a method
  found through a type parameter's constraint.
- **Assertions and payloads.** An assertion to a strict unnamed type yields its
  nullable form, and so does a payload read. A named strict type has no nullable
  form, so asserting it is an error. The message keeps the pointer wording.
- **`error` is exempt.** It is a named type of the universe, outside every file.
- **`any` is exempt.** `any` and the empty interface literal are never strict,
  as `error` is not. A value of every origin is boxed in `any`, and callbacks
  and interfaces of non-strict packages use it in parameters.
