# Chamfer

Examples: [`examples/scripts/chamfer`](../../examples/scripts/chamfer/chamfer.go), a block
with its top edges cut, and [`examples/scripts/edges`](../../examples/scripts/edges/edges.go),
an edge that no sketch entity draws, chamfered by its name.

New Group > Chamfer Edges (Shift+Q) cuts edges of the solid so far at a distance, 1 mm to
begin with. An edge that goes in, as a pocket's floor, is filled; a chamfered edge goes on into
the edges it meets smoothly, round a rounded rectangle's corners by cones. Select one or more of:

- line segments along edges between two flat faces, as the edges an extrusion makes;
- circles where a flat face meets a cylinder or a cone, as an extruded circle's rims;
- straight or circular edges of the solid itself, picked on the model. These include edges no
  entity draws, such as where a pocket's wall meets the block's face.

The group's screen lists the edges and takes a new distance, as a number or an expression.

From a script:

```go
c := doc.Chamfer([]api.Entity{edge}, 2, "")
c.SetDistance(1.5)
s := doc.Active().Solid()
doc.ChamferSolid([]api.SolidEdge{s.EdgeAt(api.V(15, 2, 10))}, 1, "")
```

## Edge names

An edge of the solid is saved by name: the two faces it lies between, its place among their
edges, and how many they had, not by where it is. Where an edit splits or joins the edges between
those two faces, the name says so on the group rather than move to another piece. A chamfer on a named edge stays on that edge when the model upstream
changes, as `edges` shows by moving a notch through a parameter. `Solid` gives a group's faces,
edges and vertices; `Edge(name)`, `EdgeAt(point)`, `EdgesBetween(a, b)` and `EdgesAlong(dir)`
find edges.

## Not yet

A chamfer at two distances is not done. Where the edit that cuts a chamfer can't make a corner,
as where a concave edge meets the cut, the chamfer falls back to a wedge and a Boolean, which
take edges that stick out only.
