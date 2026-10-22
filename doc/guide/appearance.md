# Appearance and shading

Example: [`examples/scripts/appearance`](../../examples/scripts/appearance/appearance.go), a
shaft in a housing, metal and plastic, the housing's top matte.

With **shade solids by their appearance** on (Configuration, on the home screen), the window
lights each face by its appearance as well as its colour: the light from the sky above a little
more than from the ground below, each of the two lights' diffuse light, and a highlight toward
the viewer. The GL renderer does this for each pixel, in a shader; the software renderer, which
draws without GL and for pictures, for each corner of the triangles. Off, the window lights as
SolveSpace does.

The appearances are **matte** (no highlight), **plastic** (a soft white one, the default),
**glossy** (a sharp, strong white one) and **metal** (less diffuse light, a highlight of the
face's colour). A group's screen sets its solid's, under its colour; a script can give a single
face its own:

```go
shaft.SetAppearance("metal")
doc.View.SetFaceAppearance(housing.Solid().FaceAt(api.V(15, 15, 15)), "matte")
```

A face's own appearance is kept by the face's identity in the group that made it, and saved with
the model.

## glTF

Example: [`examples/scripts/gltf`](../../examples/scripts/gltf/gltf.go), a block seen through
and a pin beside it, each its own colour.

File > Export > Triangle Mesh writes binary glTF 2.0 for a file ending `.glb`, for Blender, a
three.js viewer or a web page: the mesh of the solid up to the active group, one material for
each colour, a colour seen through (a group's opacity below 1) as a blended one, and lengths in
metres, which is what glTF asks for. `telesapr-cli export-mesh --output %.glb model.tlspr`
does the same without a window, and a script calls `doc.File.Export("model.glb")`. A motion study
saves its frames as `.glb` too.

## Not yet

Textures, reflections of an environment, and appearances made by the user; a face's own
appearance set from the window; drawings and exports in the appearance's colours. A glTF file is
one mesh: its materials are plain colours, not matte, glossy or metal, and the parts of an
assembly are not nodes of their own.
