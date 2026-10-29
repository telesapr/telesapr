# Clearance and walls

Example: [`examples/scripts/clearance`](../../examples/scripts/clearance/clearance.go), a pin 9
across in a plate's bore 10 across, and a box hollowed to walls 1.2 thick.

Analyze > Clearance and Walls opens a screen in the property browser that checks the model up to
the active group for two things before it is printed or machined:

- **gaps between parts** less than a distance (1 mm to start with; [change] sets it). The parts
  are those Show Interfering Parts checks: each group assembled into the model, and the solid
  made before the first of them. [check gaps] lists each pair nearer than the distance, the
  nearest first, and draws a line across each gap. Parts that touch or overlap have a gap of 0;
  Show Interfering Parts says which overlap. With **solids within parts** checked, a part made of
  several solids, as a linked assembly, is taken by its solids, and a part linked as a mesh by
  its pieces (the triangles joined by the corners they share): the gaps between them are listed
  too, as "g004-tile solid 2".
- **walls thinner** than a thickness. [check walls] casts rays from points of each triangle of
  each part's mesh, square to it, into the solid to where it leaves the part again, says what the
  thinnest wall is and where, and draws a line across each point thinner than asked. The points
  are no further apart than half the thickness asked about, or a fiftieth of the model's size,
  and closer again about the thinnest, so that a spot thinner than the triangles about it, as
  near the tip of a cone, isn't missed between their middles. The screen says how far apart they
  were. Between the points the mesh's wall can be thinner still, as under the tip of a narrow
  groove: a ray from a flat triangle to the one it leaves through grows or shrinks evenly as it
  moves, so the corners of where one triangle covers the other are measured too, and the
  thinnest is among them. A sharp edge, as where a thread's ridge runs out of an end face, is as
  thin as nothing there, and is listed so.

A gap is found on the parts' meshes, then followed onto the surfaces the nearest triangles stand
for, so that a pin in a bore comes out at its true gap though the meshes' chords are a little
nearer. A part linked as a mesh, with no shell, is taken by its triangles, which are its
geometry: the gap is followed on them and onto the other part's surfaces. The screen says how far
the gaps may be off: the two meshes' chord tolerances, more where the points were followed onto
the surfaces and moved. A wall is measured between two points of the mesh, each within the chord
tolerance of the surface; the thinnest points are then followed onto the two surfaces of the
wall. Set the chord tolerance smaller (Configuration) when the last hundredths matter.

From a script:

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

The MCP `measure` tool reports both with `gaps` and `walls` ([AI](../ai.md)).

## What it does not do yet

- A wall between two triangles no measured ray joined, narrower than the points measured about
  it, can still be missed.
- Walls are measured part by part, not by the solids or pieces within a part: a hollow's inside
  is a piece of its own, and measured apart it would have no wall.
