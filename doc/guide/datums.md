# Datums

Examples: [`examples/scripts/datums`](../../examples/scripts/datums/datums.go), a block's top and
bottom named and two blocks mated by those names;
[`examples/scripts/datums_solid`](../../examples/scripts/datums_solid/datums_solid.go), a plate
whose fillet's face is named and two plates mated by it;
[`examples/scripts/datums_freeform`](../../examples/scripts/datums_freeform/datums_freeform.go),
a lens-shaped block whose curved wall and rim are named, and two blocks laid wall to wall.

A datum is a name a part gives what it offers others: a point, a workplane, a face, or a line,
circle or arc as an axis. An assembly that links the part finds it as `link.name`, so a mate
holds by what the part declares rather than by where things happen to be.

Edit > Datums lists the part's own datums and those of each model it links. With something
selected on the model, the screen offers to name it:

- **an entity** (a point, a workplane, a face an entity makes, a line, circle or arc):
  [name the selected entity];
- **a face or an edge of the solid that no entity makes**, as a fillet's face, a face a Boolean
  left, or an edge of the solid: [name the selected face of the solid] or [... edge ...]. A
  workplane is added after the active group that follows the face or edge each time the model
  changes: in a flat face it lies in the face; on a face round about an axis (a cylinder, a cone,
  a fillet) and on a circular edge its normal runs along the axis and its origin is on it; on a
  straight edge its normal runs along the edge from its start. On any other face, as a loft's,
  a blend's or a sphere's, it touches the face where it was clicked, its normal out of the
  solid; on any other edge, as a cubic's or a helix's, its normal runs along the edge where it
  was clicked. That place is kept as a share of the face's extent each way, or of the edge's
  length from its start, so it moves with the face or edge when the model changes; a face or
  edge selected from a list, as a feature's screen's, is taken at its middle. The datum names
  that workplane, and the screen says which of these it is: `plane`,
  `axis`, `line`, `face-point` or `edge-point`.

Each datum row has [select], [rename], [delete] and [mate]. [mate] on two names, own or linked,
the part that stays first, offers [flush], [concentric] and [hinge]: the mates the Assembly menu
makes of a selection, made of the two names. The Assembly menu's Mate Flush and Mate Concentric
also take two such workplanes, or linked datums' normals, selected on the model. A workplane
mates as a face when flush and as an axis when concentric; through a link, a datum on a solid
comes as the workplane's normal through its origin, which mates the same. A flat face's datum is
refused as an axis, its normal running through a corner of the face, and an edge's line, straight
or curved, is refused as a plane.

From a script:

```go
corner := doc.Datums.NameFace("corner", filleted.Solid().FaceAt(api.V(38, 28, 5)))
doc.Datums.NameEdge("front", filleted.Solid().EdgeAt(api.V(20, 0, 0)))
// a lens's wall, touched low down near one end rather than at its middle
doc.Datums.NameFaceAt("foot", lens.Solid().FaceAt(api.V(10, 6, 2.5)), api.V(4.53125, 4.5, 1.25))
// in the assembly
doc.Assembly.Concentric(doc.Datums.Get("a.corner"), doc.Datums.Get("b.corner"))
```

A datum on a solid follows its face even from a group after the active one: each regeneration
makes the groups up to the last such datum, though the groups after the active one stay hidden,
so a mate, or a model linking the saved part, finds it where the face is.

## What it does not do yet

- A datum on a face neither flat nor round, or on an edge neither straight nor round, is taken at
  the middle of the face or edge; a point picked on it isn't kept.
