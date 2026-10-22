# Left after the interface and scripting parity: editing what a feature is built from, relinking, a section command

The parity work on `core-surface-parity` (October 2026) brought every edit the window makes within
reach of a script, and every script call within reach of the window, through one `pkg/sketch`
call. `pkg/script/api/parity_window_test.go` holds the two to each other and has no `GAP:` entry
left. What follows is what neither side can do. These are new features rather than parity work, so
they were left out of that branch.

## Editing what a feature is built from

A feature takes its inputs when it is made, and neither the window nor a script can change them
afterwards. The only way to change them is to delete the feature and make it again, which loses
what later groups refer to in it.

| Feature | Made by | Inputs, in `sketch.Group` | Shown afterwards |
|---|---|---|---|
| Fillet | `NewFillet` (`fillet.go:28`) | `Edges` (sketch entities) and `SolidEdges` (`[]topo.Name`), with `EdgeRadii` in their order | read-only (`Group.Edges`) |
| Chamfer | `NewChamfer` (`chamfer.go:28`) | `Edges`, `SolidEdges` | read-only |
| Shell | `NewShell` (`shell.go:25`) | `SolidFaces` (`[]srf.FaceID`) | read-only |
| Sweep | `NewSweepAlong` (`sweep.go:45`) | `Edges` as the path, `SolidEdges` for a path along the solid | read-only |
| Path pattern | `NewPathPattern` (`pattern.go:31`) | `Edges` as the path | read-only |
| Loft | `NewSmoothLoft` (`loft.go:55`) | `OpA`, `OpB` and the groups between them (`LoftProfiles`) | read-only (`LoftGroup.Profiles`) |
| Mirror | `NewMirror` (`mirror.go:56`) | `Predef.EntityB`, the plane | read-only (`MirrorGroup.Plane`) |
| Every feature on a source | its constructor | `OpA`, the group it works on | read-only |

Two inputs can already be edited afterwards, and they are the pattern to follow:

- **An extrusion's drafted sides:** `Sketch.SetDraftFaces` (`draft.go:63`), called by the group
  screen and by `ExtrudeGroup.SetDraftFaces`.
- **A loft's guide curves:** `Sketch.SetLoftGuides` (`loftguide.go:25`), called by the group
  screen and by `LoftGroup.SetGuides`.

Each one checks the new inputs as the constructor would, stores them, and marks the group dirty.
The caller remembers the undo step.

What an edit of the others has to settle:

- **Per-edge values.** `EdgeRadii` is kept in the order of `Edges` and then `SolidEdges`. Adding
  or removing an edge must carry each radius with its own edge, not with its position.
- **Names.** `SolidEdges` and `SolidFaces` are names in the solid so far (`pkg/topo`). An edit
  takes new names from the solid as it is before the feature, as the constructor does. A name that
  no longer resolves becomes a broken reference (`ClearBrokenReferences`, `broken.go:127`), not a
  panic.
- **Later groups.** A group after the feature may refer to edges or faces the feature made, as a
  sketch on one of its faces or a chamfer of one of its edges does. When an edit removes those, the
  later group gets the same broken-reference report as when an earlier group is deleted.
- **Sources.** Changing `OpA` or a loft's profiles changes what the group depends on, and the same
  cycle and order checks apply as for moving a group (`CheckMoveGroup`, `history.go`).
- **The window.** Picking the new inputs is the same selection as making the feature. The screen
  needs a "[change]" link that takes the current selection, as the draft's sides and the loft's
  guides do (`textscreens.go`).
- **Scripts.** A setter on each typed group, for example `FilletGroup.SetEdges`,
  `ShellGroup.SetFaces`, `SweepGroup.SetPath`, `LoftGroup.SetProfiles` and `MirrorGroup.SetPlane`,
  each reaching the same `pkg/sketch` call as the window. The parity test then wants a row for each
  one on both sides.

## Pointing a link at a different file

`Group.LinkFile` (`group.go`) is set when a part is linked (`Assembly.Link`, `Assembly.LinkStep`,
Assembly > Link), and nothing changes it afterwards. The window asks for a new place only when the
file is missing as the drawing opens (`locateMissingFile`, `pkg/ui/menufile.go`). It offers nothing
for a file that is present but should be replaced, such as a newer revision saved under another
name or a STEP file in place of a `.tlspr`.

A relink would set `LinkFile` (saved as the group's `link`, `pkg/persist/tlspr/encode.go`), read the part again
through the same loader as linking, and keep the placement and scale (`LinkGroup.Offset`,
`Orientation`, `ExprScale`). What later groups refer to in the old part (edges, faces, points) may
not be in the new one. Those references are reported as broken, as for a missing file the user
declines to find. It is one undo step, and on the group screen it would be a "[relink]" link with
a file chooser beside the link's path. The script call would be `LinkGroup.SetFile(path)`.

## A command-line section export

A script exports a 2d section with `File.ExportSection`. `telesapr-cli` has `export-view`,
`export-wireframe`, `export-drawing`, `export-mesh` and `export-surfaces` (`cmd/telesapr-cli/main.go`),
but no `export-section`. Those commands take input files and view options. A section also needs a
plane, which none of their flags names. Either a `--plane` option (a workplane by name, or a point
and a normal) or the section saved with the view would give the command something to cut with. Until
then, `telesapr-cli run` with a script that calls `File.ExportSection` is the way from the command
line.
