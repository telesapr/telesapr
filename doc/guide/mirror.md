# Mirror

Example: [`examples/scripts/mirror`](../../examples/scripts/mirror/mirror.go), a symmetric
bracket: one half drawn and extruded, the other half made by a mirror.

A mirror reflects the active group in a plane and joins the reflection to the solid.

## In the window

1. Make the group to reflect active.
2. Select the plane: a workplane, or a flat face of the solid.
3. Choose Features > Mirror (Shift+Y). A right click on a flat face offers the same as Mirror
   in This Plane.

The reflection follows its group: change the half, and the other half changes with it. The
sketch's entities are reflected too, so a circle's copy can be constrained and measured.

The group's screen names the plane, with "[change]" to reflect in the selected workplane or
face instead. It has:

- **solid**: "with original" or "reflection only";
- the box **the whole solid so far**. A mirror reflects its source group, the one that was
  active when the mirror was made. With the box checked, it reflects the whole solid up to it
  instead. A body made of several groups needs this: a hole cut by a later group is then
  reflected too.

## From a script

```go
half := doc.Features.Extrude(8, "half")
m := doc.Features.Mirror(doc.Workplane("YZ"), "bracket")
m.AsMirror().SetKeepOriginal(false) // only the reflection
m.AsMirror().SetMirrorSolid(true)   // the whole solid so far
```

## Not yet

A part symmetric about the plane is kept as it is, because it is its own reflection. A part
that only partly coincides with its reflection, a face here and there, can fail as a Boolean.
