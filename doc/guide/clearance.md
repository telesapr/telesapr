# Clearance and walls

Example: [`examples/scripts/clearance`](../../examples/scripts/clearance/clearance.go), a pin
9 mm across in a plate's bore 10 mm across, and a box hollowed to walls 1.2 mm thick.

## What it does

Before a model is printed or machined, two things are worth checking: are any two parts too
close together, and is any wall too thin? Analyze > Clearance and Walls opens a screen in the
property browser that checks both, on the model up to the active group.

### Gaps between parts

The screen's first row is **gaps less than** a distance, 1 mm to start with. [change] sets it.

The parts are the same ones Show Interfering Parts checks: each group assembled into the model,
and the solid made before the first of them.

[check gaps] lists every pair of parts nearer than the distance, the nearest first, and draws a
line across each gap on the model. Parts that touch or overlap have a gap of 0. Which of those
overlap, Show Interfering Parts tells.

With **solids within parts** ticked, a part made of several solids, such as a linked assembly,
is taken solid by solid, and the gaps between its own solids are listed too, as
"g004-tile solid 2". A part linked as a mesh is taken by its pieces: the triangles that are
joined by shared corners.

### Walls thinner than a thickness

The second row is **walls thinner than** a thickness, also 1 mm to start with.

[check walls] measures the walls of each part. From points on each triangle of the part's mesh,
it casts a ray straight into the solid, square to the triangle, to where the ray leaves the
part again. The screen says what the thinnest wall is and where, and draws a line across each
point thinner than asked.

How the points are chosen:

- They are no further apart than half the thickness asked about, or a fiftieth of the model's
  size. So a thin spot narrower than the triangles around it, such as near the tip of a cone,
  is not missed between their middles.
- Around the thinnest spots the points are placed closer again.
- Between two points a wall can be thinner still, as under the tip of a narrow groove. A ray
  from one flat triangle to the one it leaves through grows or shrinks evenly as it moves, so
  the corners of where the two triangles overlap are measured too. The thinnest spot is among
  them.
- A sharp edge, as where a thread's ridge runs out of an end face, has a thickness of nothing
  there, and is listed so.

The screen says how far apart the points were.

Each solid of a part is measured on its own. A hollow's inside is measured together with the
solid around it, so a ray from one solid does not stop on another standing on it.

### How exact it is

Gaps are first found on the parts' meshes. Then the nearest points are followed onto the exact
surfaces the triangles stand for. So a pin in a bore comes out at its true gap, even though the
meshes' chords are a little nearer. A part linked as a mesh has no exact surfaces: its
triangles are its geometry, so the gap is followed on them and onto the other part's surfaces.

The screen says how far the gaps may be off: the chord tolerances of the two meshes (how far
each mesh may stray from its true surface), plus more where the points were followed onto the
surfaces and moved.

A wall is measured between two points of the mesh, each within the chord tolerance of the
surface. The thinnest points are then followed onto the two surfaces of the wall.

When the last hundredths matter, set the chord tolerance smaller on the configuration screen.

## From a script

```go
for _, c := range doc.Assembly.Clearances(1) { // gaps under 1 mm; 0 lists every pair
	fmt.Println(c.A.Name(), c.B.Name(), c.Gap, c.Tolerance)
}
for _, c := range doc.Assembly.ClearancesWithin(1) { // and between the solids of a part
	fmt.Println(c.A.Name(), c.SolidA, c.B.Name(), c.SolidB, c.Gap)
}
r := group.Analyze().WallThickness(1.5) // points thinner than 1.5 mm
fmt.Println(r.Thinnest.Thickness, len(r.Thin), r.Samples)
```

The MCP `measure` tool reports both when asked with `gaps` and `walls`
([Drawing with an AI model](ai.md)).

## Not yet

- A wall between two triangles that no measured ray joined, narrower than the points measured
  around it, can still be missed.
