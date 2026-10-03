# Mirror

Example: [`examples/scripts/mirror`](../../examples/scripts/mirror/mirror.go), a symmetric
bracket: one half drawn and extruded, the other half made by a mirror.

New Group > Mirror (Shift+Y) reflects the active group in a plane and joins the reflection to
the solid. Select first:

- a workplane, or
- a flat face of the solid.

The reflection follows its group: change the half and the other half changes with it. The
group's screen names the plane and has the box "solid with original": cleared, only the
reflection is kept. Entities are reflected too, so a circle's copy can be constrained and
measured.

From a script:

```go
half := doc.Extrude(8, "half")
m := doc.Mirror(doc.Workplane("YZ"), "bracket")
m.SetKeepOriginal(false) // only the reflection
```

## Not yet

A mirror reflects its source group, the active one when it was made, and not the whole solid so
far: a hole cut by a later group is not reflected unless it is in the mirrored group's own
sketch. Reflecting the solid so far is roadmap 15.5. A part mirrored in a plane of its own,
whose faces then coincide with the reflection's, is roadmap 13.4.
