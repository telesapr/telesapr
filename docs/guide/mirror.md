# Mirror

Example: [`examples/scripts/mirror`](../../examples/scripts/mirror/mirror.go), a symmetric
bracket: one half drawn and extruded, the other half made by a mirror.

Features > Mirror (Shift+Y) reflects the active group in a plane and joins the reflection to
the solid. Select first:

- a workplane, or
- a flat face of the solid.

The reflection follows its group: change the half and the other half changes with it. The
group's screen names the plane, and has "solid with original" or "reflection only", and the box
"the whole solid so far". A mirror reflects its source group, the active one when it was made;
with that box checked it reflects the whole solid up to it, as a body made of several groups
needs: a hole cut by a later group is reflected too. Entities are reflected too, so a circle's copy can be constrained and
measured.

From a script:

```go
half := doc.Features.Extrude(8, "half")
m := doc.Features.Mirror(doc.Workplane("YZ"), "bracket")
m.AsMirror().SetKeepOriginal(false) // only the reflection
m.AsMirror().SetMirrorSolid(true)   // the whole solid so far
```

## Not yet

A part symmetric about the plane is kept as it is (it is its own reflection); one that only
partly coincides with its reflection, a face here and there, can fail as a Boolean (F5).
