# Editing what a feature is built from, and a section from the command line

Two things neither the window nor a script can do, left after the interface and scripting parity
of `core-surface-parity` (October 2026; `pkg/script/api/parity_window_test.go` has no `GAP:` entry).
They are new features, not parity work. The third item of the original note, pointing a link at
another file, was built in work package B3 (`Sketch.SetLinkFile`, `LinkGroup.SetFile`).

## Editing what a feature is built from

A feature takes its inputs when it is made, and nothing changes them afterwards: the only way is to
delete the feature and make it again, which loses what later groups refer to in it.

| Feature | Made by (`pkg/sketch`) | Inputs, in `sketch.Group` | Shown afterwards |
|---|---|---|---|
| Fillet | `NewFillet` (`fillet.go`) | `Edges` (sketch entities) and `SolidEdges` (`[]topo.Name`), with `EdgeRadii` in their order | read-only (`Group.Edges`) |
| Chamfer | `NewChamfer` (`chamfer.go`) | `Edges`, `SolidEdges` | read-only |
| Shell | `NewShell` (`shell.go`) | `SolidFaces` (`[]srf.FaceID`) | read-only |
| Sweep | `NewSweepAlong` (`sweep.go`) | `Edges` as the path, `SolidEdges` for a path along the solid | read-only |
| Path pattern | `NewPathPattern` (`pattern.go`) | `Edges` as the path | read-only |
| Loft | `NewSmoothLoft` (`loft.go`) | `OpA`, `OpB` and the groups between them (`LoftProfiles`) | read-only (`LoftGroup.Profiles`) |
| Mirror | `NewMirror` (`mirror.go`) | `Predef.EntityB`, the plane | read-only (`MirrorGroup.Plane`) |
| Every feature on a source | its constructor | `OpA`, the group it works on | read-only |

Three inputs can be edited afterwards, and they are the pattern to follow: an extrusion's drafted
sides (`Sketch.SetDraftFaces`, `draft.go`), a loft's guide curves (`Sketch.SetLoftGuides`,
`loftguide.go`) and a link's file (`Sketch.SetLinkFile`, `link_params.go`). Each checks the new
inputs as the constructor would, stores them and marks the group dirty; the caller remembers the
undo step; the group screen has a "[change]" link that takes the selection, and the typed group in
`pkg/script/api` a setter reaching the same call.

What an edit of the others has to settle:

- **Per-edge values.** `EdgeRadii` is kept in the order of `Edges` and then `SolidEdges`. Adding
  or removing an edge must carry each radius with its own edge, not with its position.
- **Names.** `SolidEdges` and `SolidFaces` are names in the solid so far (`pkg/topo`). An edit
  takes new names from the solid as it is before the feature, as the constructor does. A name that
  no longer resolves becomes a broken reference (`ClearBrokenReferences`, `broken.go`), not a
  panic.
- **Later groups.** A group after the feature may refer to edges or faces the feature made, as a
  sketch on one of its faces or a chamfer of one of its edges does. When an edit removes those, the
  later group gets the same broken-reference report as when an earlier group is deleted.
- **Sources.** Changing `OpA` or a loft's profiles changes what the group depends on, and the same
  cycle and order checks apply as for moving a group (`CheckMoveGroup`, `history.go`).
- **Scripts.** A setter on each typed group, for example `FilletGroup.SetEdges`,
  `ShellGroup.SetFaces`, `SweepGroup.SetPath`, `LoftGroup.SetProfiles` and `MirrorGroup.SetPlane`.
  The parity test then wants a row for each one on both sides.

## A command-line section export

A script exports a 2d section with `File.ExportSection`. `telesapr-cli` has `export-view`,
`export-wireframe`, `export-drawing`, `export-mesh`, `export-surfaces` and `export-script`
(`cmd/telesapr-cli/main.go`), but no `export-section`: a section needs a plane, which none of their
flags names. Either a `--plane` option (a workplane by name, or a point and a normal) or the
section saved with the view would give the command something to cut with. Until then,
`telesapr-cli run` with a script that calls `File.ExportSection` is the way from the command line.
