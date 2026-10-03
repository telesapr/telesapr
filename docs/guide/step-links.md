# STEP links

Example: [`examples/scripts/step`](../../examples/scripts/step/step.go), which links a STEP
file and prints what was read; given none, it exports a plate as STEP and links it back.

New Group > Link / Assemble... (Shift+I) takes a STEP file (`.step`, `.stp`) as well as a
TeleSAPR model or a triangle mesh: an STL file, binary or text, or a Wavefront OBJ file, whose
faces take the colours of the materials its `.mtl` file gives. A mesh links with a point at its
origin, the corners of its bounding box, and a point at each vertex on its sharp edges. A STEP
file or a mesh comes in where it was drawn, its coordinates the file's; a TeleSAPR model comes in
off the middle of the view, beside what is drawn. A STEP file's solids are read and triangulated into a reference
mesh, placed in the model, with a point at each vertex and at each circle's centre, so that
the mesh can be constrained and measured against. The group's screen says what was read: the
file's schema and unit, how many shapes, faces and triangles it gave, the chord tolerance it was
triangulated to, and the entities it could not read, by name. A file with
no solid that the reader supports links empty, and says so.

The link keeps the file's path. On opening, and on undo and redo, the file is read again, so a
changed file shows its changes; one that hasn't changed (the same size and modification time) is
taken from memory, so a part linked many times is read once. A STEP file's triangles are kept
on disk too, in the user's cache folder (`telesapr/step`, or `TELESAPR_CACHE_DIR`), so the next
session doesn't triangulate it again; deleting the folder only costs that time.

From a script:

```go
g, err := doc.Link("parts/bracket.step", "bracket")
fmt.Println(g.LinkInfo(), g.LinkProblem())
fmt.Println(g.Check().Volume)
```

## Not yet

The link is a mesh: its faces and edges can't be filleted, chamfered or cut, and it takes no
part in Booleans. Reading a STEP file as a solid is roadmap 21. A linked file is placed at the
middle of the view rather than where it was drawn (16.13). CSG and swept solids, and
tessellated STEP files, are reported by name and not read.
