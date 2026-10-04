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
r := group.Check()
if !r.Sound() {
	fmt.Println(r.Summary())
}
fmt.Println(r.Faces, r.Volume, r.CenterOfMass)
```

`group.Solid().Problems()` lists the topology's problems by face and edge.

Every solid feature is tested by checking many variants of it this way, in sweeps; the tests and
their failure rates are listed in [`docs/done-audit.md`](../done-audit.md).
