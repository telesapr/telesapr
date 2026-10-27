# Editing what a feature is built from, and a section from the command line

Two things neither the window nor a script can do, left after the interface and scripting parity
of `core-surface-parity` (October 2026; `pkg/script/api/parity_window_test.go` has no `GAP:` entry).
They are new features, not parity work. The third item of the original note, pointing a link at
another file, was built in work package B3 (`Sketch.SetLinkFile`, `LinkGroup.SetFile`).

## Editing what a feature is built from

Built 2026-10-27 (ROADMAP F7). A feature's inputs are changed by a setter in `pkg/sketch` beside
its constructor, each of which checks the new inputs as the constructor does, including the trial
on a copy of the solid before the feature, and stores them only when the trial succeeds
(`feature_inputs.go` has the shared checks):

| Feature | Setter (`pkg/sketch`) | Typed group (`pkg/script/api`) |
|---|---|---|
| Fillet | `SetFilletEdges` | `FilletGroup.SetEdges` |
| Chamfer | `SetChamferEdges` | `ChamferGroup.SetEdges` |
| Shell | `SetShellFaces` | `ShellGroup.SetFaces`, `Faces` |
| Sweep | `SetSweepPath` | `SweepGroup.SetPath` |
| Path pattern | `SetPatternPath` | `RepeatGroup.SetPath` |
| Loft | `SetLoftProfiles` | `LoftGroup.SetProfiles` |
| Mirror | `SetMirrorPlane` | `MirrorGroup.SetPlane` |

What was settled:

- **Per-edge values.** `carryEdgeRadii` moves each of a fillet's own radii with its edge, by
  identity (`HEntity` for sketch edges, `topo.Name` for the solid's), and drops the radius of an
  edge taken away. A loft's tangency at a profile between the ends stays with the profile.
- **Names.** Names and face identities are resolved against the solid before the feature
  (`solidBefore`), never the active group's; an input made at or after the feature is refused
  (`inputBefore`, through `GroupsInOrder` and `faceGroup`).
- **Later groups.** Nothing is pruned at the edit: the next regeneration finds a name that no
  longer resolves and reports it on the later group, as `TestFeatureInputsBreakLater` shows.
- **The window.** The group's screen has [change] beside its inputs and [remove] on each row
  (`pkg/ui/featureinputs.go`). Solid edges and faces are hovered on the active group's solid, so
  [change] with nothing suitable selected activates the group before the feature, says what to
  select, and the screen stays while it is picked (`shownState.inputsFor`), as the drawing's
  does; the link then reads [take the selection].
- **Not built.** A feature's source (`OpA`) is not changed this way: that makes the group another
  feature, and the move and dependency checks of the history are the place for it if it is ever
  asked for.

## A command-line section export

A script exports a 2d section with `File.ExportSection`. `telesapr-cli` has `export-view`,
`export-wireframe`, `export-drawing`, `export-mesh`, `export-surfaces` and `export-script`
(`cmd/telesapr-cli/main.go`), but no `export-section`: a section needs a plane, which none of their
flags names. Either a `--plane` option (a workplane by name, or a point and a normal) or the
section saved with the view would give the command something to cut with. Until then,
`telesapr-cli run` with a script that calls `File.ExportSection` is the way from the command line.
