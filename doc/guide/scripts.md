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

## A model as a script

Example: [`examples/scripts/exportscript`](../../examples/scripts/exportscript/exportscript.go),
a plate whose width is a parameter, written out as the program that builds it.

File > Export > Script (.go) writes the drawing as a Go program that builds it again on a new
drawing: the parameters, every group in the order the model is built made by the call that
makes one of its kind, the requests of each sketch where they are solved to, and the
constraints with their values, names and expressions, the styles, images, splines of any
number of points (`doc.Sketch.Spline`, `doc.Sketch.PeriodicSpline`), kerned text, datum points
in groups that aren't sketches, and faces' own appearances. An image's file is named where it is, as a link's is. Edit it, give it a form, and run it as
any script. `telesapr-cli export-script model.tlspr` does the same without a window, and a
script calls `doc.File.ExportScript(path)`, or `doc.Script()` for the text and a list of what it
left out. An AI program's `export` tool writes it for a file ending `.go`.

What a DXF import draws as the window can't is written with calls of its own: an arc drawn in
3d (`doc.Sketch.ArcIn3D`), a workplane drawn in a workplane (`doc.Sketch.NewWorkplaneIn`) and
the same constraint twice (`doc.Constrain.Again`). An image whose file is missing is written as
its frame (`doc.Sketch.ImageFrame`), and the window says so. What the script can't write is a
comment in it starting `not written:`, and the window says so too: a face's appearance on a
face no point finds. Handles are numbered
afresh, so the script names what a group made by what it was made from (`g3.Made(r4, 1002)`)
and a request's points by their number (`r4.Part(1)`).

## What it does not do yet

- A form's fields are a flat list: no units that follow the document's, no bounds, no fields
  that depend on others.
- A package is read once per interpreter: a library file changed while the console is open is
  seen after Script > Stop has stopped a script, or in a new window. Script files run from the
  menu start a new interpreter each time, and see it at once.
- There are no versions or downloads of packages: the library is a folder you keep.
- An AI program's scripts are kept to some folders ([Drawing with an AI model](../ai.md#files));
  yours are not.
