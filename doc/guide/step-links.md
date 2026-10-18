# STEP links

Examples: [`examples/scripts/step`](../../examples/scripts/step/step.go), which links a STEP
file and prints what was read; given none, it exports a plate as STEP and links it back.
[`examples/scripts/stepsolid`](../../examples/scripts/stepsolid/stepsolid.go) links a STEP
file as a solid and cuts a slot across it.

Assembly > Link / Assemble... (Shift+I) takes a STEP file (`.step`, `.stp`) as well as a
TeleSAPR model or a triangle mesh: an STL file, binary or text, or a Wavefront OBJ file, whose
faces take the colours of the materials its `.mtl` file gives. A mesh links with a point at its
origin, the corners of its bounding box, and a point at each vertex on its sharp edges. A STEP
file or a mesh comes in where it was drawn, its coordinates the file's; a TeleSAPR model comes in
off the middle of the view, beside what is drawn, but where it was drawn when a script links it
(`doc.Assembly.Link`), so that what a script makes doesn't depend on the view. A STEP file's solids are read and triangulated into a reference
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

## Packed models

Saving a model as `.tlsprz` packs it: a zip of the model with what it links (and what those link,
all in one folder), its pictures, and the kept triangles of its STEP files, so that it can be sent
and opened elsewhere as it was. Opening one unpacks it into a folder of its own; its links then
name the files there, so saving it packed again packs them again. Saving it as a `.tlspr` keeps
those links, into that folder: to work on the parts, unpack it somewhere of your own first (a zip
tool will do; the model is `model.tlspr`).

## As a solid

The group's screen of a STEP link has "read as reference mesh / solid". Read as a solid, the
file's faces are exact: planes, cylinders, cones, spheres, tori, surfaces of revolution and of
extrusion, B-splines, and offsets of them, each cut into the kernel's patches along its knots (a
quarter turn for one that turns about an axis) and trimmed by its edges. A curve or surface of a
degree above 5, the kernel's highest, is written at degree 5 in pieces, within a ten-thousandth of
a millimetre; so is an offset of a surface other than a plane, cylinder, cone, sphere or torus,
whose offsets are surfaces of the same kind. Edges may be lines, circles, ellipses, parabolas,
hyperbolas and B-splines. The linked part then takes part in Booleans, so a later group can cut it
or join it, as a solid made here does; the screen says "read as a solid, n faces exact". A file
that can't be read as one comes in as the reference mesh, and the screen says why, as "read as a
mesh, not a solid: ...".

Files are written to a tolerance, and edges miss their faces by up to it: the reader takes a
thousandth of a millimetre, and the mesh of a face from a file puts its edges' points where the
edges are, so faces that meet in the file meet in the mesh. An edge that runs along the line where
the pieces of a face's surface meet, within the tolerance of it, is put on that line. What it
can't take yet: edges further from their faces than the tolerance. Of the files TeleSAPR is tested
on, the 14 shapes made in FreeCAD, the 6 of higher degrees, offsets and conics, and 5 of 9 from
other systems read as sound solids.

## From a script

```go
g, err := doc.Assembly.Link("parts/bracket.step", "bracket")
fmt.Println(g.AsLink().Info(), g.AsLink().Problem())
fmt.Println(g.Analyze().Check().Volume)

part, err := doc.Assembly.LinkStep("parts/bracket.step", "bracket", true) // as a solid
part.AsLink().AsSolid()                                              // true
part.AsLink().SetAsSolid(false)                                      // back to a mesh
```

## Not yet

CSG and swept solids, and tessellated STEP files, are reported by name and not read. Booleans on
imported solids fail where the kernel's do on its own: a cylinder cut through a cylinder, a cone
through its apex.
