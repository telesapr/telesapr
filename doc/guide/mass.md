# Density and mass

Example: [`examples/scripts/mass`](../../examples/scripts/mass/mass.go), a steel shaft in an
aluminium housing, the mass, centre of mass and inertia of each and of the whole.

A density belongs to a part, not to a feature: once two features' solids are joined, nothing
says which volume was whose. The parts are those Show Interfering Parts takes: each group
assembled into the model is a part at its own density, and the solid made before the first of
them is a part at the density last set among its groups. A model that assembles nothing is one
part.

A group's screen sets the density, in g/cm³, at the end of its appearance row:
`density not set [change]`.
Steel is about 7.85, aluminium 2.7, PLA 1.24. An empty field clears it. Only a group whose solid
joins the model or is assembled into it has one: an extrusion, lathe, revolve, helix, sweep,
loft or link. A fillet or a shell edits a solid that has one already, and a step and repeat
copies it. Setting a density changes no geometry, so nothing regenerates; it is one step of
undo, and saved with the model.

Analyze > Measure Volume gives the mass of the model up to the active group and its centre of
mass, and each part's mass where there are several. A part with no density set is taken at
1 g/cm³, water's, and the message says so. With no density set anywhere, it gives the volume
alone, as before.

From a script:

```go
housing.SetDensity(2.7) // aluminium, g/cm³; 0 clears it
shaft.SetDensity(7.85)  // steel
r := shaft.Analyze().MassProperties()
for _, p := range r.Parts {
	fmt.Println(p.Group, p.Volume, p.Density, p.DensitySet, p.Mass, p.CenterOfMass)
}
fmt.Println(r.Mass, r.CenterOfMass, r.Inertia[2][2], r.DensityUnset())
```

Masses are in g, lengths in mm. `Inertia` is the inertia tensor in g·mm² about the centre of
mass: on the diagonal the moments about the axes through it along x, y and z, off it the products
of inertia, negated. The whole's is about the whole's centre of mass.

The [bill of materials](bill-of-materials.md#bill-of-materials) gives each linked part's mass at
the density set on its link group.

## What it does not do yet

- Masses come from the solid's triangles, so a curved part is short of its volume as its mesh
  is, by the chord tolerance. Set the chord tolerance smaller (Configuration) for the last
  fractions of a percent; Analyze > [Ray-Cast Check](check-solid.md#ray-cast-check) measures the
  volume and inertia without the mesh, at a density of one, for the models it takes.
- A density set in a part's own file is not carried into a model that links it: set it on the
  link group.
- One density per part: a part made of two materials joined is not split by them.
- Analyze > Show Center of Mass, and `Analyze().CenterOfMass()` in a script, still take the model
  at one density throughout; Measure Volume's centre of mass takes the densities.
