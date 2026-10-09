# Screw threads

Example: [`examples/scripts/bolt`](../../examples/scripts/bolt/bolt.go), a hexagon head bolt
with a thread of any designation, M8 unless given.

Select a circle or an arc, such as the rim of an extruded circle, and choose Features > Screw
Thread... (Shift+E). The screen offers the standard thread nearest the circle's diameter, and
takes:

- **thread**, the designation:

  | Designation | Thread |
  |---|---|
  | `M8` | ISO metric, coarse pitch |
  | `M8x1` | ISO metric, 1 mm pitch |
  | `M16xPh3P1.5` | ISO metric, 1.5 mm pitch, 3 mm lead (two starts) |
  | `Tr20x4`, `Tr20x8(P4)` | ISO metric trapezoidal |
  | `1/4-20 UNC`, `#10 UNF`, `1/2-13 UNC (2 starts)` | unified inch |

  A designation ending in `-LH` is left-handed;
- **length**, from the circle's plane, one and a half diameters to begin with;
- **clearance**, which moves the thread away from its mate;
- **external** or **internal**: a bolt's thread, the part turned down and the ridge around it,
  or a nut's, the circle bored out and the ridge inside;
- **through the far end**, for a part that ends where the thread does; otherwise the thread
  stops at a shoulder;
- **against the circle's normal**, to run the other way.

"make the thread now" adds four groups: a sketch and a lathe that turn the part down or bore it,
and a ridge sketch and a helix that sweeps it. Undo takes all four away at once. The thread's
groups are ordinary groups afterwards; the thread's length is a dimension of its sketch.

From a script:

```go
doc.Thread(rim, api.ThreadOptions{Designation: "M8", Length: 20})
doc.Thread(hole, api.ThreadOptions{Internal: true, Through: true})
api.ThreadNearest(7.9) // "M8"
```

## Not yet

Tapered threads, variable pitch, run-out through a face, cosmetic threads drawn without the
ridge, and one group in place of four are not planned ("Left out" in the roadmap).
