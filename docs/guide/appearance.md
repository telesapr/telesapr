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

## Not yet

Textures, reflections of an environment, and appearances made by the user; a face's own
appearance set from the window; drawings and exports in the appearance's colours.
