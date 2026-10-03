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

## Configurations

Example: [`examples/scripts/configurations`](../../examples/scripts/configurations/configurations.go),
one plate in three sizes.

A configuration is a named set of the parameters' expressions: the same model small and large,
from one table filled two ways. Under CONFIGURATIONS on the parameters screen,
`[add a configuration of the table as it is]` makes one and makes it active; while it is active,
a change to the table is a change to it too. Choosing another writes its expressions into the
table, and the model follows; choosing `none` leaves the table as it is, and changes it alone. A
parameter renamed is renamed in every configuration; one that an inactive configuration's
expression uses can't be deleted. The file keeps the configurations and which is active.

```go
must(doc.AddConfiguration("small"))
must(doc.AddConfiguration("large"))
must(doc.SetParameter("w", "100"))   // large's w
must(doc.ActivateConfiguration("small"))
```

## Dimensions by name

Example: [`examples/scripts/dimensions`](../../examples/scripts/dimensions/dimensions.go), a
plate whose height and hole read its width dimension.

A dimension can be named, and expressions read its value by the name, as they read a
parameter. Double-click the dimension and type `width = 60`: the label shows
`width = 60.00`, and another dimension can be `width*2/3`. The value read is the one the label
shows, in the document's unit: a distance without the sign of its side, a diameter shown as a
radius as the radius. Typing a new name renames every use; `= 60` takes the name away, which is
refused while something uses it. Parameters and dimensions share the names. A reference
dimension measures, and can't be named. Changing the document's unit keeps the lengths that
read a named length.

```go
doc.Length(sides[0], 60).SetName("width")
must(doc.Length(sides[1], 40).SetExpr("width*2/3"))
must(doc.Dimension("width").SetExpr("90"))
```

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

A parameter that reads a named length changes with the document's unit; changing the unit lists
it among the ones to check. A configuration holds parameters' expressions only, not group
values or which groups are suppressed.
