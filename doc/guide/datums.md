# Datums

Examples: [`examples/scripts/datums`](../../examples/scripts/datums/datums.go), a block's top and
bottom named and two blocks mated by those names;
[`examples/scripts/datums_solid`](../../examples/scripts/datums_solid/datums_solid.go), a plate
whose fillet's face is named and two plates mated by it.

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
  straight edge its normal runs along the edge from its start. The datum names that workplane.

Each datum row has [select], [rename], [delete] and [mate]. [mate] on two names, own or linked,
the part that stays first, offers [flush], [concentric] and [hinge]: the mates the Assembly menu
makes of a selection, made of the two names. A workplane mates as a face when flush and as an axis
when concentric; through a link, a datum on a solid comes as the workplane's normal through its
origin, which mates the same.

From a script:

```go
corner := doc.Datums.NameFace("corner", filleted.Solid().FaceAt(api.V(38, 28, 5)))
doc.Datums.NameEdge("front", filleted.Solid().EdgeAt(api.V(20, 0, 0)))
// in the assembly
doc.Assembly.Concentric(doc.Datums.Get("a.corner"), doc.Datums.Get("b.corner"))
```

## What it does not do yet

- A face neither flat nor round about one axis, and an edge neither straight nor round, can't be
  named.
- A datum on a solid follows its face only while it comes after the groups that make the face and
  before the active group: groups after the active one are not made again.
