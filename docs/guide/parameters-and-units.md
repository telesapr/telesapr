# Parameters and units

Example: [`examples/scripts/plate`](../../examples/scripts/plate/plate.go), a mounting plate
whose sizes all come from a table of parameters, and
[`examples/scripts/feedback`](../../examples/scripts/feedback/feedback.go), for units,
measuring and what a sketch says about itself.

## Parameters

A parameter is a name for a value, such as `width = 60` or `margin = height/5`. A dimension
whose value is an expression of parameters changes when they do, and so does everything built
on it.

In the window:

- **Edit > Parameters** (Ctrl+Shift+M) opens the table. Add a parameter, give it an expression,
  rename it or delete it there. A parameter may name earlier ones.
- Double-click a dimension and type an expression, such as `width - 2*margin`, in place of a
  number. The editor shows the expression again the next time.
- A group's own values, such as an extrusion's depth or a fillet's radius, take expressions on
  the group's screen.
- Renaming a parameter rewrites every expression that uses it. Deleting one that is used is
  refused, and the message says what uses it.

From a script:

```go
must(doc.SetParameter("width", "60"))
must(doc.SetParameter("height", "width*2/3"))
must(doc.Length(side, 60).SetExpr("width"))
must(chamfer.SetExpr(sketch.GroupPitch, "c*2"))
for _, p := range doc.Parameters() {
	fmt.Println(p.Name, p.Expr, p.Value)
}
```

`RenameParameter` and `DeleteParameter` do what the table does; `doc.ExprErrors()` lists
expressions that do not evaluate.

## The document's unit

View > Dimension Units sets the unit the document is in: millimetres, metres, inches, or feet
and inches. It belongs to the document and is saved with it. Lengths typed in the window, and
lengths in expressions, are in that unit: in an inch document, `1/4` is a quarter inch, and is
kept and saved as typed. Changing the unit keeps every size, rewriting each expression to give
the same length, and is one undo step.

From a script, `doc.SetUnits(core.UnitInches)` does the same, and `doc.Units()` reads it. The
script methods that take plain numbers, such as `doc.Length(line, 50.8)`, always take mm; only
text, as in `SetExpr("2")`, is in the document's unit.

## What a sketch says about itself

- **Freedom.** Entities are drawn in one colour when the constraints fix them and in another
  while they can still move. From a script, `entity.Free()` is `sketch.FreedomFixed`,
  `FreedomPartly` or `FreedomFree`.
- **Conflicts.** When a group's constraints contradict each other, its screen lists the
  constraints any one of which could be removed, and the canvas marks them. From a script,
  `group.Conflicts()`.
- **Measure.** Analyze > Measure Selection shows the length, radius, distance or angle of what
  is selected on the canvas, without adding a constraint. From a script, `doc.Measure(a, b)`.
- **Find Command.** Edit > Find Command (Ctrl+K) finds any menu command by typing part of its
  name. From a script, `api.Commands()` lists them and `doc.RunCommand("View > Zoom To Fit")`
  runs one.

## Not yet

A constraint's value can't be read by the constraint's name (roadmap 16.1), and there are no
named sets of parameter values (16.2).
