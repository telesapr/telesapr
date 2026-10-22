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
- **length**, from the circle's plane, one and a half diameters to begin with; a plain number,
  or an expression of the parameters (Edit > Parameters), as `len` or `2*d`, which the thread
  then follows;
- **clearance**, which moves the thread away from its mate;
- **external** or **internal**: a bolt's thread, the part turned down and the ridge around it,
  or a nut's, the circle bored out and the ridge inside;
- **through the far end**, for a part that ends where the thread does; otherwise the thread
  stops at a shoulder;
- **against the circle's normal**, to run the other way;
- **crest**: **standard**, the flat crest the standards give; **truncated**, cut a quarter of the
  ridge's height lower; or **rounded**, an arc tangent to both flanks whose top is where the
  standard crest is. A printed thread is stronger and mates more easily without the point;
- **run-out**, how far before its shoulder the thread fades out, as a die leaves it: over that
  length the turned or bored surface leaves the root and tapers to the crest (to a tenth of the
  thread's depth short of it on a screw), so the ridge sinks into the part. 0 is none; a thread
  through the far end has no shoulder and takes none;
- **starts level**, offered for a thread of several starts: every start begins at the circle's
  plane, each a ridge and a helix of its own turned a part of a turn from the last.

"make the thread now" adds four groups: a sketch and a lathe that turn the part down or bore it,
and a ridge sketch and a helix that sweeps it; with level starts, a ridge sketch and a helix for
each further start after them. Undo takes them all away at once. The thread's last group is
active afterwards.

The first helix group keeps what the thread was made with. Its screen has **[edit thread]**, which
opens the same screen filled in from the thread: "make the thread again now" draws the
groups again from what it shows, in one step of undo. The groups keep their handles (a level
start's only while there are as many starts), and the
sketches their lines where the thread keeps its shape, so later groups that use the thread stay
attached; a ridge taken away with a start is reported as a broken reference. The groups can
still be edited one by one, but an edit of the thread draws them again from its screen.

From a script:

```go
doc.Features.Thread(rim, api.ThreadOptions{Designation: "M8", Length: 20})
doc.Features.Thread(hole, api.ThreadOptions{Internal: true, Through: true})
api.ThreadNearest(7.9) // "M8"

t := g.AsThread() // g, the thread's helix group, as Features.Thread returns it
o := t.Options()
o.Designation, o.Crest, o.LengthExpr = "M8x1", "rounded", "len"
t.Edit(o)
```

```go
doc.Features.Thread(rod, api.ThreadOptions{Designation: "M8", Length: 12, RunOut: 3})
doc.Features.Thread(lead, api.ThreadOptions{Designation: "M12xPh3P1", Length: 14, LevelStarts: true})
```

Examples: [`examples/scripts/threadedit`](../../examples/scripts/threadedit/threadedit.go),
[`examples/scripts/threadrunout`](../../examples/scripts/threadrunout/threadrunout.go).

Without level starts, several starts are swept by one helix, their ridges a pitch apart in the
ridge sketch: that is a turn of 1/n between them, and leaves the first (n-1) pitches with fewer
ridges.

Where a ridge runs into the run-out's cone, its flanks cross the cone at the thread's helix
angle. Every run-out of the sweep (`TestThreadRunOutSweep`, 72 lengths and threads) comes out
sound; should one not, the thread tries other planes for its ridges, and the window says the
solid isn't sound if none does.

## Not yet

Tapered threads, variable pitch, and run-out through a face are not planned ("Parked" in the
roadmap).
