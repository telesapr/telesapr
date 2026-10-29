# Check Solid

Every example calls it: see [`examples/scripts/plate`](../../examples/scripts/plate/plate.go)
for one that fails when the solid is not sound.

Analyze > Check Solid (Ctrl+Shift+O) reports on the active group's solid:

- **sound or not.** A solid is sound when its mesh has no naked edges (edges with no mate, where
  the mesh is open), no edge passes through a triangle, the Boolean that made it classified
  every edge, and its faces, edges and vertices fit together: every edge between exactly two
  faces, every loop closed, and an Euler characteristic that agrees with the mesh's;
- **faces and patches**: a face may be several surface patches joined smoothly, as a cylinder's
  wall is;
- **volume, centre of mass and bounding box**, in the document's unit.

The volume is the mesh's, made of chords within the chord tolerance (Configuration), which is a
share of the whole model's size. A group's volume can therefore change a little when a later group
makes the model larger: its curved faces are cut into coarser chords. Set the chord tolerance
smaller, or compare volumes once the model is whole, when the last digits matter.

What is wrong is listed by kind, and the edges at fault are drawn on the model, as Analyze > Show
Naked Edges draws naked ones.

From a script:

```go
r := group.Analyze().Check()
if !r.Sound() {
	fmt.Println(r.Summary())
}
fmt.Println(r.Faces, r.Volume, r.CenterOfMass)
```

`group.Solid().Problems()` lists the topology's problems by face and edge.

Every solid feature is tested by checking many variants of it this way, in sweeps; the tests and
their failure rates are listed in [`doc/done-audit.md`](../done-audit.md).

## Ray-cast check

Example: [`examples/scripts/raycheck`](../../examples/scripts/raycheck/raycheck.go), a plate with
a turned spindle through it, four bolt holes repeated round it and a slot cut across.

Check Solid judges the kernel's B-rep by its own mesh. Analyze > Ray-Cast Check judges it against
a second, independent account of the solid: it rebuilds the model from the groups' definitions
alone (each extrusion's profile and depth, each lathe's or revolve's profile, axis and angle,
the motions that place the copies of a step and repeat, a pattern along a path or a mirror,
joined, cut or intersected as the groups say), with no surface intersection, and casts rays
through it. It reports:

- **the volume, centre of mass and principal moments of inertia** (for a density of one, in
  the view's unit to the fifth power) by three grids of rays, one along each axis, made finer
  until the three volumes agree within 0.1% and with the grids of half as many rays (or 256
  rays a side), beside the mesh's volume;
- **the rays that disagree** with the mesh: where the ray is inside one and not the other over a
  stretch further than twice the chord tolerance from the mesh. The rays are drawn on the model;
- **the mesh's edges**, made vertex to vertex: each should be used by two triangles, once each
  way. Edges used once, more than twice or twice the same way, and degenerate triangles, are
  counted.

From a script, `group.Analyze().RayCheck()` gives the same report, with the whole inertia tensor about the
centre of mass in `Measure.Inertia` (mm⁵), and `group.Analyze().Probe(from, dir)` the stretches of a line
inside the solid.

### Not yet

The rebuilt model takes extrusions, drafted or not, lathes, revolves, helices of a profile drawn
in a plane through their axis, sweeps along line segments and arcs (mitred or with a corner radius,
the profile square to an arc's axis), sweeps the kernel fits along their path (along a cubic,
twisted, held to a direction, or round a closed path out of a plane), and the copies step and
repeats, patterns along a path and mirrors make of them; a model with any other solid group (a
fillet, a chamfer, a loft, a shell, a STEP link) is refused, naming the group.

A fitted sweep is rebuilt by placing the profile along the path by frames of the check's own: they
turn with the path and no more, or are held to the direction, and the twist turns them evenly by
length. The kernel fits its surfaces between sections of the same sweep, so the rays show where the
fit strays; where a ray meets the swept sides, the rebuilt model samples the crossing, as a
helix's.

A helix's rays agree with the mesh as a lathe's do, within the chord tolerance: the kernel's
patches follow the true helix within a few millionths of each quarter turn's rise. A ray that
grazes a helix's inner wall can still disagree by a few hundredths of a millimetre, where the
rebuilt model samples the crossing.
