# Bug: a script stops with "1:1: nil pointer" on a slice literal of method results

Found 2026-11-14 while modelling over MCP (version 0.6.5, yaegi v0.16.1). Worked round in
TeleSAPR 2026-11-17; the fault itself is upstream and not fixed there.

Refreshed 2026-11-17.

## What happened

A script stopped before it ran a single statement, with:

    script:1:1: nil pointer

(`a.go:1:1: nil pointer` from `telesapr-cli run`). Nothing was printed and nothing drawn: the
interpreter panicked while compiling. The position was always 1:1, so the message named neither
the line nor the construct.

The trigger is a composite literal whose element calls a method on an element of a slice that
compiled code returned:

```go
tl := doc.Sketch.Polyline(true, a, b, c)   // []api.Entity from compiled code
ents := []api.Entity{tl[0].Start()}        // panicked at compile time
```

The same thing without TeleSAPR crashes plain yaegi v0.16.1, so the fault is upstream:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	ds, _ := os.ReadDir(".")
	xs := []string{ds[0].Name()}
	fmt.Println(xs)
}
```

| Variant (plain yaegi) | Result |
|---|---|
| `[]string{ds[0].Name()}`, `ds` from `os.ReadDir` | crash |
| `[]bool{ds[0].IsDir()}`, `[]any{ds[0].Name()}` | crash |
| `[]string{(ds[0]).Name()}`, `[]string{ds[0:1][0].Name()}` | crash |
| `[]string{fs[0].Name}`, a field, `fs` from `reflect.VisibleFields` | crash |
| `[][]string{{ds[0].Name()}}`, `map[string][]string{"a": {ds[0].Name()}}` | crash |
| `struct{A string}{ds[0].Name()}` | crash |
| `var xs = []string{ds[0].Name()}`, `fmt.Println([]string{ds[0].Name()})` | crash |
| `[]string{0: ds[0].Name()}`, the element keyed | works |
| `struct{A string}{A: ds[0].Name()}`, `map[string]string{"a": ds[0].Name()}` | works |
| `x := ds[0].Name()`, `fmt.Println(ds[0].Name())`, not in a literal | works |
| `[]string{d.Name()}` with `d := ds[0]` | works |
| `[]string{strings.ToUpper(ds[0].Name())}`, `[]int{len(ds[0].Name())}` | works |
| `[]os.DirEntry{ds[0]}`, the index expression is the element | works |
| `dm["x"].Name()`, `da[0].Name()` with `dm`, `da` a map and an array built in the script | works |

## Where yaegi panics, and why

`interp/cfg.go`, the pre-order visit of a `compositeLitExpr` (about line 400): for each element
that is not a key-value, a unary or binary expression, a composite literal, an index or a type
assertion, it calls `nodeType` to give the element its type before the element is compiled. For
`ds[0].Name()` that is `nodeType2` on the call, then on the selector `ds[0].Name`, then on the
index expression `ds[0]`.

`interp/type.go`, `nodeType2`, `case indexExpr` (about line 816): the type of `ds` is a `valueT`
(a type of compiled Go, held as a `reflect.Type`), and the switch on `lt.cat` handles `arrayT`,
`mapT`, `sliceT`, `variadicT` and `genericT` only. So `t` stays nil, with no error. The caller,
`case selectorExpr` (line 978), reads `lt.incomplete` on that nil and panics. Outside a literal
the element is typed after its children are compiled, from their types, and an element with a key
goes through the `keyValueExpr` path, which never calls `nodeType` on it; neither reaches the
missing case.

The fix upstream is a `valueT` case in `nodeType2`'s `indexExpr`:

```go
case valueT:
	switch rt := lt.rtype; rt.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		t = valueTOf(rt.Elem(), withScope(sc))
	case reflect.String:
		t = sc.getType("byte")
	case reflect.Ptr:
		if rt.Elem().Kind() == reflect.Array {
			t = valueTOf(rt.Elem().Elem(), withScope(sc))
		}
	}
```

Tried on a copy of v0.16.1: the program above and the crashing rows of the table compile and
run, and `go test -short ./interp` fails the same 25 tests as without it (all of the environment:
GOPATH-relative `_test` imports, `Test_previousRoot`, `switch13.go` wording, `issue-1010.go`).
It is to be reported to traefik/yaegi with the `os.ReadDir` program; yaegi is a required module
with no replace directive, so TeleSAPR takes the fix when a release has it.
`TestYaegiLiteralOfCompiledSliceElement` (`pkg/script/literal_keys_test.go`) fails when an
upgrade fixes it, and then `keyElements` can go.

## What TeleSAPR does

- **Keys the element** (`pkg/script/literal_keys.go`, `keyElements`). Before the interpreter
  compiles source, each element of a slice or an array literal that needs it is given the key
  Go gives it anyway: `[]api.Entity{tl[0].Start()}` becomes `[]api.Entity{0: tl[0].Start()}`.
  An element without a key has the index after the element before; after a keyed element `n:`
  the key written is `(n) + 1`, a constant as Go requires. It means the same and evaluates the
  same, in the same order: nothing is moved out of the literal, so a literal in a loop's header,
  a closure, a call's arguments or behind `&&` is no different. An element needs a key when the
  chain its type is worked out from (a call's function, `append`'s first argument, a selector's
  value, parentheses, `*`, a slice, an index) holds an index expression below the element
  itself. Keying an element that didn't need it is harmless. Literals with an elided type,
  `[][]string{{...}}`, `map[K][]T{k: {...}}`, `[]*T{{...}}`, take their type from the literal
  they are in. A key puts no line break in, so no line moves; a place the interpreter gives on a
  line that got a key is counted back to the source as written (`unkey`, through `unplace`).
  It is applied where the engine hands source to the interpreter: `eval` (statements, the
  console, programs), `show` (a console's expression) and `tooMany`. So the window's console,
  `telesapr-cli run`, `eval` and `repl`, and MCP's `run_script` all have it.
- **Was not done: hoisting** the element into a variable before the statement. A copy of
  `tl[0]` loses a pointer method's change to the slice's element, `&tl[0]` can't be taken of a
  map's element, and a statement position isn't there for a literal in a loop's condition, a
  closure, behind `&&` (`len(ds) > 0 && f([]string{ds[0].Name()})` would index an empty slice),
  or in an `if`'s init that defines the slice. Keying has none of these.
- **Says what it can't key.** A struct literal without field names (the field names would be
  needed, and a named type's aren't known before compiling), a literal of a type named rather
  than spelled out (`Names{...}`: a slice or a struct), and a positional element after a key
  that spans lines are left as they are. When the interpreter panics while compiling
  (`compiling`: `(*Interpreter).CompileAST` is on the panic's stack, which no panic of a running
  script has), `compileFault` finds the first such element and reports it at its line and
  column: `the interpreter (yaegi) failed while compiling gs[0].Name() as an element of a
  literal (nil pointer); put it in a variable first and write the variable in the literal, as
  v := gs[0].Name()`. Any other panic while compiling says that the interpreter failed while
  compiling, and that it is not the script's fault. Statements are compiled one at a time, so it
  is placed at the statement's first line; in a program the place isn't known, and the error
  carries the file's name only, not 1:1.
- The `api_help` primer's "What the interpreter can't do" and the guide's scripting page ("Not
  yet") say what is left: a struct literal without field names.

## Workaround for what is left

Put the element or the result in a variable first:

```go
n := gs[0].Name()
p := P{n}
```
