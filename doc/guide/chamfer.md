# Chamfer

Examples: [`examples/scripts/chamfer`](../../examples/scripts/chamfer/chamfer.go), a block
with its top edges cut, [`examples/scripts/edges`](../../examples/scripts/edges/edges.go),
an edge that no sketch entity draws, chamfered by its name, and
[`examples/scripts/bevel`](../../examples/scripts/bevel/bevel.go), a bevel of two distances.

Features > Chamfer Edges (Shift+Q) cuts edges of the solid so far at a distance, 1 mm to
begin with. An edge that goes in, as a pocket's floor, is filled; a chamfered edge goes on into
the edges it meets smoothly, round a rounded rectangle's corners by cones. Select one or more of:

- line segments along edges between two flat faces, as the edges an extrusion makes;
- circles where a flat face meets a cylinder or a cone, as an extruded circle's rims;
- straight or circular edges of the solid itself, picked on the model. These include edges no
  entity draws, such as where a pocket's wall meets the block's face. To take every edge round a
  face, right-click the face and choose Select Face Edges
  ([`examples/scripts/face_edges`](../../examples/scripts/face_edges/face_edges.go);
  `Edit.SelectFaceEdges` in a script).

The group's screen lists the edges and takes a new distance, as a number or an expression.
An edge named by the faces it lies between is hovered and selected on the model from its row, as a
line's is. "[change]" beside the edges takes the selection as the chamfer's edges instead, and "[remove]"
on an edge's row takes it out; with nothing selected, "[change]" activates the group before the
chamfer, whose edges it cuts, and says what to pick.
Edges between curved faces, and edges that end on one, are chamfered as they are filleted:
see [Fillet](fillet.md#between-two-curved-faces).

## Two distances

A chamfer may cut back further on one face than on the other: its screen's *second distance*
row, "the same" to begin with, takes a distance for each edge's second face. An edge's first
face is the one of the lower identity, the one its name gives first; where that comes out the
wrong way round, *[swap]* exchanges the two distances, and *[same]* makes the chamfer one of a
single distance again.

From a script:

```go
c := doc.Features.Chamfer([]api.Entity{edge}, 2, "")
c.AsChamfer().SetDistance(1.5)
c.AsChamfer().SetSecondDistance(3) // 1.5 on the first face, 3 on the second
c.AsChamfer().SwapDistances()      // 3 on the first, 1.5 on the second
s := doc.Active().Solid()
doc.Features.ChamferSolid([]api.SolidEdge{s.EdgeAt(api.V(15, 2, 10))}, 1, "")
```

## Corners

Where every edge at a corner is chamfered, three or more straight edges between flat faces, the
chamfers' lines on each face cross, and the corner is cut as OpenCASCADE's chamfers cut it. Where
three chamfers of one distance meet at a square corner, the corner is the triangle between the
three points where their lines cross. Otherwise, where the distances differ, an edge has two, or
the faces don't all meet square, one chamfer, the one of the largest distance whose lines cross
the others' furthest out, ends across between its two crossing points, the other two are cut
square to their edges where their lines cross its own, and the face between them takes the line
from one cut to the other; the hole left is a plane, or a twisted four-sided patch with straight
sides where its corners don't lie in one. At a corner of four or more, as a pyramid's apex, the
chamfers end between the points where their lines cross, and the hole is filled the same way, or
by triangles about its middle past four sides. Example:
[`examples/scripts/chamfercorner`](../../examples/scripts/chamfercorner/chamfercorner.go), a
block with every edge chamfered at two distances.

OpenCASCADE parts from this in two places: at a corner of three where one crossing point lies
close to the line between the other two it makes the triangle of the three points, where here the
chamfers are cut as above (a few tenths of a cubic millimetre apart at distances of 1 to 3 mm);
and at a corner of four or more it fills the hole with a plate that bulges out past the plane of
its corners, where here it is that plane.

## Edge names

An edge of the solid is saved by name: the two faces it lies between, its place among their
edges, and how many they had, not by where it is. Where an edit splits or joins the edges between
those two faces, the name says so on the group rather than move to another piece. A chamfer on a named edge stays on that edge when the model upstream
changes, as `edges` shows by moving a notch through a parameter. `Solid` gives a group's faces,
edges and vertices; `Edge(name)`, `EdgeAt(point)`, `EdgesBetween(a, b)` and `EdgesAlong(dir)`
find edges.

## Not yet

A chamfer by a distance and an angle is not done, nor a distance of an edge's own as a fillet's
own radius, and a chamfer of two distances can't take circular edges that meet other chamfered
edges at a corner, nor straight edges that meet another chamfered edge at a corner whose third
edge stays sharp. Where the edit that cuts a chamfer can't make a corner, as where a concave edge
meets the cut, the chamfer falls back to a wedge and a Boolean, which take edges that stick out
only.
