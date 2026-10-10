# Sketch on a face

Example: [`examples/scripts/pedestal`](../../examples/scripts/pedestal/pedestal.go), a base with a
column drawn on its top and a cap drawn on the column's top, which go up with the base when it is
made thicker.

Features > Sketch On Face (Shift+B) starts a sketch whose workplane lies in a flat face of the
solid so far. Select one face, an extrusion's or one no entity stands for, as a chamfer's or a
shell's, picked on the model; a right click on a face offers it too, and
Sketch In New Workplane does the same with a face selected. The workplane's normal points out of
the solid, its origin is on a corner of the face and its first axis runs along one of the face's
edges, with the face beside the origin on the positive side of both axes. A face with no straight
edge, as a cylinder's cap, puts the origin at the centre of its circle.

The sketch follows the face. When an edit before it moves or turns the face (a deeper extrusion,
another draft angle), the workplane, what is drawn in it and the solids made from it go with the
face in the same regeneration. The corner and the edge are chosen by the edges' names, not by
where they are, so they stay the same corner and edge while the face keeps its edges. When the
face goes, as when the group that made it is suppressed, the workplane stays where it was and
the group says why; it follows the face again when the face comes back. The group's screen names
the face.

From a script:

```go
base := doc.Extrude(8, "base")
doc.SketchOnFace(base.Solid().FaceAt(api.V(30, 20, 8)), "")
doc.Rect(api.V(20, 13, 8), api.V(40, 27, 8))
column := doc.Extrude(12, "column")
```

`Solid.Face` takes a face by its identity instead of a point on it, and `Group.Face` gives the
face a sketch lies in.

## Not yet

The face can't be changed on the group's screen: delete the sketch and start it on the other
face. The corner and the edge are chosen for you; to draw from another corner, constrain the
sketch to the face's edges. A depth left free on an extrusion from the sketch is kept as near
its old vector as the solve can, so when the face turns, the extrusion comes out a little shorter;
a dimension holds it.
