# Scripts

Every feature can be scripted in Go (see the [README](../../README.md#scripting) and `help()` in
the console). This page covers what helps scripts be shared and reused: forms that ask for a script's values
when it runs, and a library of packages of your own.

## Forms

A script declares the values it asks for with an `api.Form`, a variable at its top level:

```go
var Form = api.Form{
	Title: "Washer",
	Fields: []api.Field{
		{Name: "outer", Label: "Outer diameter (mm)", Value: 30.0},
		{Name: "slots", Label: "Slots round the bore", Value: 0},
		{Name: "finish", Label: "Finish", Value: "metal", Choices: []string{"matte", "plastic", "glossy", "metal"}},
		{Name: "save", Label: "Save washer.tlspr", Value: true},
	},
}
```

and reads them with `Form.Float("outer")`, `Form.Int("slots")`, `Form.Text("finish")` and
`Form.Bool("save")`. In a script of statements, as the console takes them, it is
`var form = Form{...}`, without the package's name. Each `Value` is written as a literal, and
says what its field takes: `0` a whole number, `30.0` a number, `true` or `false` a check box,
a quoted string text, or one of the `Choices`.

- **Script > Run File** shows the fields in a dialog before the script runs, with the values
  written in it, and runs it with those you leave. **Cancel** doesn't run it; a value that
  doesn't fit its field (`many` slots) is refused with a message.
- `telesapr-cli run --set outer=40 --set slots=6 washer.go` gives the same values without a
  window.
- An AI program's `run_script` takes them as its `form` argument.

The values given are written into the script's text in place of the ones it was written with,
so the script runs the same way whichever way it was given them, and a field not given keeps
its value. The example is [`examples/scripts/form`](../../examples/scripts/form).

## The script library

A script imports a package of your own by its path in the **script library folder**:
`import "gears"` reads the Go files in `<library>/gears`, and `import "shop/brackets"` those in
`<library>/shop/brackets`. Library packages import each other the same way, and import
`github.com/telesapr/telesapr/pkg/script/api` as any script does. They have no `doc` of their
own: they draw on the `*api.Doc` they are given.

```go
// <library>/gears/gears.go
package gears

import "github.com/telesapr/telesapr/pkg/script/api"

func Blank(doc *api.Doc, teeth int, module, bore, width float64, name string) api.Group {
	...
}
```

```go
import "gears"

gears.Blank(doc, 24, 2, 8, 6, "gear")
```

The folder is the same for every way a script runs:

- In the window, the configuration screen's **script library folder**. Left empty it is
  `telesapr/library` in the user's configuration folder (`~/.config` on Linux, `%AppData%` on
  Windows, `~/Library/Application Support` on macOS). The console, Script > Run File and AI
  connections all use it; a change takes effect at the next import.
- On the command line, `--lib <dir>` before the model and the script:
  `telesapr-cli run --lib ~/parts examples/scripts/library/library.go`, and likewise for `repl`,
  `eval` and `mcp --headless`. Without it, the default folder above.

The go tool doesn't know the library, so a script file that imports from it can't be built with
`go build` or checked with `go vet`. Such a file carries the build tag `telesapr`, which the
scripts' interpreter has and the go tool doesn't:

```go
//go:build telesapr

package main
```

The example is [`examples/scripts/library`](../../examples/scripts/library), with its library in
`lib`.

## What it does not do yet

- A form's fields are a flat list: no units that follow the document's, no bounds, no fields
  that depend on others.
- A package is read once per interpreter: a library file changed while the console is open is
  seen after Script > Stop has stopped a script, or in a new window. Script files run from the
  menu start a new interpreter each time, and see it at once.
- There are no versions or downloads of packages: the library is a folder you keep.
