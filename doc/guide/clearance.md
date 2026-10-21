# Clearance and walls

Example: [`examples/scripts/clearance`](../../examples/scripts/clearance/clearance.go), a pin 9
across in a plate's bore 10 across, and a box hollowed to walls 1.2 thick.

Analyze > Clearance and Walls opens a screen in the property browser that checks the model up to
the active group for two things before it is printed or machined:

- **gaps between parts** less than a distance (1 mm to start with; [change] sets it). The parts
  are those Show Interfering Parts checks: each group assembled into the model, and the solid
  made before the first of them. [check gaps] lists each pair nearer than the distance, the
  nearest first, and draws a line across each gap. Parts that touch or overlap have a gap of 0;
  Show Interfering Parts says which overlap.
- **walls thinner** than a thickness. [check walls] casts a ray from the middle of each triangle
  of each part's mesh, square to it, into the solid to where it leaves the part again, says what
  the thinnest wall is and where, and draws a line across each point thinner than asked.

A gap is found on the parts' meshes, then, where both parts have shells, followed onto the
surfaces the nearest triangles stand for, so that a pin in a bore comes out at its true gap though
the meshes' chords are a little nearer. The screen says how far the gaps may be off: the two
meshes' chord tolerances, more where the points were followed onto the surfaces and moved. A wall
is measured between two points of the mesh, each within the chord tolerance of the surface. Set
the chord tolerance smaller (Configuration) when the last hundredths matter.

From a script:

```go
for _, c := range doc.Assembly.Clearances(1) { // gaps under 1 mm; 0 lists every pair
	fmt.Println(c.A.Name(), c.B.Name(), c.Gap, c.Tolerance)
}
r := group.Analyze().WallThickness(1.5) // points thinner than 1.5 mm
fmt.Println(r.Thinnest.Thickness, len(r.Thin), r.Samples)
```

## What it does not do yet

- A wall is measured at the middle of each triangle. A spot thinner than the triangles about it,
  as near the tip of a cone, can fall between them; a finer chord tolerance measures more points.
- A part linked as a mesh, with no shell, is taken by its triangles; the gaps to it are not
  followed onto surfaces.
- The gaps between the solids a linked assembly holds, within one part, are not checked.
