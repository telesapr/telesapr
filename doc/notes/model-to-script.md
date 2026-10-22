# Model to script (WP-D5, item 15)

A model edited in the window is written out as a Go script that builds it again through
`pkg/script/api`, so hand edits become a program that can be read, changed and run
(`telesapr-cli run`, Script > Run File, MCP `run_script`).

## Shape

`pkg/script/api/export_script.go` walks the sketch as `describe.go` does and writes one
`package main` program:

- the unit, the parameters and the configurations (`Params.Set`, `AddConfiguration`);
- every group in `GroupOrder`, created by the `Features`/`Assembly` call the window's own
  command reaches (`SketchIn`, `SketchAt`, `SketchOnLines`, `SketchOnNormal`, `SketchOnFace`,
  `Sketch3D`, `Extrude`, `Lathe`, `Revolve`, `Helix`, `Translate`, `Rotate`, `Mirror`,
  `Chamfer`/`ChamferSolid`, `Fillet`/`FilletSolid`, `Shell`/`ShellSolid`, `Loft`,
  `LoftThrough`, `Sweep`, `PatternAlongPath`, `Emboss`/`Engrave`, `Assembly.Link`), then its
  options through the group views' setters (combine, colour, opacity, appearance, density,
  suppression, two-sided, held depth or angle, draft, twist, tangency, radii, expressions);
- each group's own parameters (an extrusion's vector, a revolve's angle, a link's placement)
  set from the model with `Group.Param(i).Set`, which every kind shares;
- the group's requests (`Sketch.Point`, `Line`, `Circle`, `Arc`, `Cubic`, `NewWorkplane`,
  `Text`) at the places they are solved to, in the workplane each was drawn in, construction
  kept;
- its constraints with `Constrain.Raw`, which takes any type, then value or expression,
  reference, name, label offset, show-as-radius and limit; comments with `Constrain.Comment`;
- datums (`Datums.Name`) once every group is made.

A thread (the four groups `AddThread` writes, its options on the helix group) is written as
one `Features.Thread` call on its circle, so it stays one thread group.

## Names in the script

- A group is a variable `gN` (N its handle), a request `rN`.
- A request's own entity is the variable; another entity of it is `rN.Part(i)`, i the index
  the request's entities have (a line's ends 1 and 2, a circle's centre 1, ...).
- An entity a group made (an extrusion's faces, a copy) is `gN.Made(from, copy)`: the key of
  the group's remap, the entity it was made from and its copy number. The index the remap
  gives it isn't the same when the group is made again: a saved remap keeps the numbers of
  the model's whole history (first tried, 12 fixtures failed on it).
- The reference workplanes and the origin are `doc.Workplane("XY")` and `doc.Origin()`;
  other reference entities `doc.Entity(h)`.
- An edge or a face of a solid (fillet, chamfer, shell, sketch on face, wrap) is found by a
  point on it, `doc.Active().Solid().EdgeAt(p)` / `FaceAt(p)`, since a face's id is a handle
  that a model with deleted requests doesn't make again.

`Group.Made` and `Entity.Part` are the two calls added for this.

## Not written yet

Each is a `// not written:` comment in the script and a note in the result, never code
that would build something else:

- images (a file outside the model), periodic cubics, request styles;
- constraints on a third or fourth entity (`EntityC`, `EntityD`);
- group kinds with no `Features` call of their own: imported meshes and STEP links as
  solids are written as links; anything else is a comment.
- a sketch's diagnostics, selection, view and drawing.

## The round trip

`TestScriptRoundTrip` (`pkg/script`, under `HEAVY`) takes every `testdata/**/*.tlspr`,
writes its script, runs it on a new document and compares: the groups (kind, name), their
requests and constraints counted, each group's DOF and whether it solved, and the volume of
each solid group's mesh. Byte equality isn't the aim: handles of a model with deletions are
numbered again. The fixtures that don't come back are listed with the reason in
`scriptRoundTripKnown`, a list that may only shrink.

## State

Done on `wp-d5-model-script`: the emitter, File > Export > Script, `telesapr-cli
export-script`, `Doc.Script`, `File.ExportScript`, the example `exportscript`. The round trip
brings back 267 of 296 fixtures.

Found on the way:

- Configurations: adding one makes it active, and what is set then goes into it, so the
  script makes none active between them.
- A signed distance's expression is stored as `-(…)`; the script writes the label's text
  (`sketch.ConstraintText`), which the constraint wraps again.
- Two comments of the same words in different places were one to the sketch's duplicate
  rule, which dropped the second; a comment's offset now counts (`equalConstraint`).
- Suppression is set at the end, after what is made from the group.
- The window's host runs a call on its own thread at once only once a script has posted to
  it; a menu entry calling the interface says so first (`ScriptHost.onLoop`).

Next: images (the file beside the model), splines through more points (`AddSplinePoint`
places a point in the nearest span, so the points would have to go in order), constraints on
four entities (`Constrain.Raw` takes two), request styles, a face's own appearance, the MCP
`export` tool for `.go`.
