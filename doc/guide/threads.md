# Screw threads

Example: [`examples/scripts/bolt`](../../examples/scripts/bolt/bolt.go), a hexagon head bolt
with a thread of any designation, M8 unless you give another.

## Making a thread

Select a circle or an arc, such as the rim of an extruded circle, and choose Features > Screw
Thread... (Shift+E). The screen offers the standard thread nearest the circle's diameter. Fill
in the rows below and press **make the thread now**.

### The designation

The **thread** row holds the designation. These forms are read:

| Designation | Thread |
|---|---|
| `M8` | ISO metric, coarse pitch |
| `M8x1` | ISO metric, 1 mm pitch |
| `M16xPh3P1.5` | ISO metric, 1.5 mm pitch, 3 mm lead (two starts) |
| `Tr20x4`, `Tr20x8(P4)` | ISO metric trapezoidal |
| `1/4-20 UNC`, `#10 UNF`, `1/2-13 UNC (2 starts)` | unified inch |
| `1/4 BSW`, `3/8 BSF`, `3/8-20 BSW` | Whitworth, 55° |
| `G1/2`, `G1 1/2` | parallel pipe thread (BSP, ISO 228-1) |
| `1/2-10 ACME`, `1/2 ACME-2G` | general purpose ACME, 29° |
| `S20x4`, `S40x14(P7)` | metric buttress (DIN 513), flanks at 3° and 30° |
| `1-12 BUTT`, `1 BUTT`, `1-12 PUSH-BUTT`, `2-4 BUTT-2A` | buttress inch (ASME B1.9), flanks at 7° and 45° |

Tapered threads are not made.

### Choosing a size and a pitch

- **[choose]** lists the series: metric, unified, trapezoidal, Whitworth, pipe, ACME, buttress
  (the DIN 513 S threads) and BUTT (buttress inch). Pick a series to see its sizes. Pick a size
  to see its standard pitches. For M8 that is 1.25, the coarse pitch, then the ISO 261 fine
  pitches 1 and 0.75. A click takes the size or the pitch and keeps the starts and the hand.
- **[change]** takes any designation you type. If the thread is written another way, the screen
  says so beside it: `'M8x1.25' is written M8`. A size the series does not have is refused.
  The message gives a pitch that would make a thread, and the nearest standard size: `give its
  pitch, as in Tr21x4, or take the nearest, Tr20x4`.
- The S series takes its diameters and pitches from ISO 2902, from 10 mm up. A buttress inch
  size given without its threads per inch takes the middle of the pitches that B1.9 recommends
  for that diameter.

### Whitworth threads

Whitworth crests and roots are drawn flat, at the height where the standard's rounds would be.
The **rounded** crest (below) comes close to the standard's rounded crest.

### Buttress threads and the load flank

A buttress ridge leans. Its load flank, the flank that carries the load, is nearly square to the
axis. The other flank is the clearance flank.

- As drawn, a screw's clearance flank leads: it faces the end where the thread starts, the end
  that goes into the nut. A nut's load flank faces the nut's mouth.
- **load flank back**, beside the hand on the pitch row of a buttress thread, turns the ridge
  about. A screw and a nut mate only when their load flanks face each other.
- ASME B1.9 names the way by the flank that leads as the two parts are put together, on the
  screw and the nut alike. `BUTT` pulls, with the clearance flank leading. `PUSH-BUTT` pushes,
  with the load flank leading. `PULL-BUTT` is read as `BUTT`. Typing such a designation sets
  **load flank back** to match, and the thread's row writes the way its ridge makes. Made
  internal, a B1.9 thread keeps its way and its ridge turns.
- A DIN 513 designation says nothing about the way, so set the box by hand.

Example: [`examples/scripts/buttress`](../../examples/scripts/buttress/buttress.go), a screw
jack's S20x4 spindle, its nut with the load flank turned back, and a 1-12 BUTT ring.

### Rounded crest and root

- A buttress thread's **rounded** crest is the arc tangent to both flanks and to the standard
  crest. Its top lies off the crest's middle, towards the steeper flank. It goes with a rounded
  root too.
- Roots are flat at the standard's truncation. For a B1.9 thread, **rounded root** at the end
  of the crest row rounds them as B1.9 draws them: an arc of 0.07141 pitches, tangent to both
  flanks. These arcs are small, 0.15 mm on a 1-12 BUTT. A chord tolerance coarser than a fifth
  of the arc's radius is refused, and the message says what tolerance to set (Edit >
  Preferences). The chord tolerance is how far the mesh may stray from the true surface. Near
  the root's flat the model leaves the standard's arc along its tangent, by up to a quarter of
  its radius, so that the Booleans meet the root at an angle.

### The other rows

- **right-hand** or **left-hand**, beside the pitch. `-LH` at the end of a designation means
  the same.
- **length**, in the document's unit, measured from the circle's plane. It starts at one and a
  half diameters. Type a number, or an expression of the parameters (Edit > Parameters) such as
  `len` or `2*d`. The thread then follows the parameter.
- **phase**, beside the length, in degrees. It turns the thread about its axis: the ridges
  move along the axis by that part of the lead, and are made a pitch shorter to leave them
  room. Use it to fit a thread to another by hand. It works with a run-out too.
- **clearance**, in the document's unit, moves the thread away from its mate.
- **class**, beside the clearance. **[choose]** lists the tolerance classes of the thread's
  form and side:
  - ISO metric screw: 6g, 6h, 6f, 6e, 4h, 8g. Nut: 6H, 4H, 5H, 7H, 6G (ISO 965-1).
  - Unified: 1A, 2A, 3A and 1B, 2B, 3B (ASME B1.1).
  - Trapezoidal: 7e, 8e and 7H, 8H (ISO 2903).
  - ACME, screw and nut: 2G, 3G, 4G, 5G (ASME B1.5, general purpose).
  - Buttress inch: 2A, 3A and 2B, 3B (ASME B1.9).
  - Metric buttress: 7e, 8e, 8c, 9c and 7H, 8H, 9H (DIN 513-3). The nut's deviation, 3.1758
    times the axial play of DIN 513-1, is not yet taken: H is basic.
  - Whitworth threads have no classes.

  A class sets the clearance to the middle of its zone: half the fundamental deviation plus
  half the pitch diameter tolerance, as a radius. So a 6g screw fits a 6H nut as real parts do
  on average (M10: 0.049 and 0.044 mm). The numbers come from the standards' formulas, and are
  within a few µm of the tables for the sizes checked. If you type a clearance, the thread
  keeps that and its class becomes none. Made internal, a screw's class becomes the nut's
  default. A cosmetic thread keeps its class as a record and leaves the part at the nominal
  size: the class is for whoever cuts the thread.
- **external** or **internal**. A bolt's thread turns the part down and puts the ridge around
  it. A nut's thread bores the circle out and puts the ridge inside.
- **to the end of the part**, for a part that ends where the thread does. A screw is turned
  down, and a nut bored, one pitch past the thread, with no shoulder. Otherwise the thread
  stops at a shoulder. If the part goes on past the thread, that pitch is a groove, and the
  window says so when the thread is made.
- **flip the direction** runs the thread against the circle's normal. A circle at the top of a
  part points out of it, so its thread runs down into the part this way. For a thread on a
  face this row is replaced by the rows under "On a face" below.
- **chamfered end** cuts the thread's free end, the circle's end, by a cone at 45° to the
  axis. The cone starts a little inside the minor diameter (0.4 of a pitch inside the ridges'
  feet) and reaches out past the crest. That is how ISO 4753 chamfers a screw's end, and a
  nut's mouth is countersunk at 90°. The thread then starts clean, not at a ridge's blunt end.
  A thread to the end of the part is chamfered at both ends. A cosmetic thread's part is cut
  the same way, and so is a rounded crest. Two cases cannot be chamfered: a thread too short
  for its chamfers, and a screw so coarse for its size that the cone would reach the axis past
  the end (a Tr12x3).
- **crest**: **standard** is the flat crest the standards give. **truncated** is cut a quarter
  of the ridge's height lower. **rounded** is an arc tangent to both flanks, with its top where
  the standard crest is. A printed thread is stronger and mates more easily without the point.
- **run-out** is how far before its shoulder the thread fades out, as a die leaves it. Over
  that length the turned or bored surface leaves the root and tapers to the crest (on a screw,
  to a tenth of the thread's depth short of the crest), so the ridge sinks into the part. 0 is
  none. A thread to the end of the part has no shoulder and takes none.
- **starts level** is offered for a thread of several starts. Every start then begins at the
  circle's plane, each with a ridge and a helix of its own, turned a part of a turn from the
  last.

The thread must fit its part. Three cases are refused before anything is made, and the
message says how far the part reaches: a thread longer than the part, a thread that runs off
the part (the circle's normal pointing out of it), and a bolt's thread on a rod thinner than
its root. If the part is made shorter later, by a parameter or an edit, the thread's helix
group carries that problem until the part is long enough again.

## Fitting a nut to its bolt

A thread is drawn from its circle the same way whatever is to go on it. So a nut only fits its
bolt where it happens to sit a whole number and a half of pitches from where the bolt's ridge
starts. Anywhere else, their ridges collide.

A bolt and its nut are two parts, linked into one model. Their threads cannot share a solid,
because each one's turning or boring would cut the other's ridges. **Assembly > Mate Screw** on
a rim of each, the bolt's and the nut's hole's, fits them. When the two circles carry threads of
one size, pitch, starts and hand (a screw's and a nut's, made on the circle or on the same axis
of the same part, as on the hole's face or its other rim), the mate takes the threads' lead. It
first moves the nut along the axis, by less than half a pitch, to where its ridges lie between
the bolt's. Turning it by the mate's `turn` parameter then keeps them there.

Mismatched threads (M8 on M10, or two screws) are refused. So are two buttress threads whose
load flanks face the same way along the axis, since no move fits them: make one with **load
flank back**. A cosmetic thread gives the mate its lead but has no ridges to fit, so the part
stays where it is.

Example: [`examples/scripts/threadfit`](../../examples/scripts/threadfit/threadfit.go), a 6g
bolt and a 6H nut screwed down it, checked for interferences at each turn.

```go
o := api.ThreadOptions{Designation: "M10", Class: "6g", Length: 25, Reverse: true}
doc.Features.Thread(end, o)
mate := doc.Assembly.Screw(boltRim, nutRim, 0) // 0: the threads' lead, and fitted
api.ThreadClasses("M10", true)                 // "6H", "4H", "5H", "6G", "7H"
```

## On a face

Select a cylindrical face of the solid, a shaft or a hole, or the circular edge at one of its
ends. Then choose Features > Screw Thread..., or **Screw Thread...** on the right-click menu, or
the thread button on the toolbar. The face says the rest:

- A shaft carries a screw's thread, and a hole a nut's.
- The thread starts at the face's end that is the part's end: a shaft's tip, or the mouth of a
  hole. If both ends are free it starts at the upper one. If you picked an edge, it starts
  there. It runs along the face into the part.
- The screen offers the standard thread nearest a shaft. For a hole it offers the thread the
  hole is drawn or drilled for, at its nominal size or its tap drill's size: a hole of 8.5 mm
  is offered M10.

In place of the external, internal, through and flip rows, the screen has:

- **the face's full length**. It is on to begin with when one and a half diameters would reach
  past the face. The thread then runs the whole face, and through where the face reaches the
  part's far end. The face is measured again at every regeneration, and the thread follows its
  length without being edited: a rod made longer by a parameter carries a longer thread. If
  the face comes to reach the part's end, or comes to stop short of it, the shape of the
  turning changes. The thread reports that on its group and takes it up the next time it is
  edited. A face too short for the thread is reported the same way. Typing a length turns the
  box off.
- **from the face's other end** starts the thread at the other end.

The thread is drawn on a construction circle of the face's diameter, in a datum at the face's
end. That datum is a group before the thread's own. It follows the face when the part changes
upstream, and the thread follows with it. A hole wider than the thread's root, or a shaft
thinner than it, is refused. A hole drawn at the thread's nominal size is bored a little past
it, by a quarter of the ridge's overlap, so that the two cylinders do not lie on each other.

```go
s := doc.Active().Solid()
doc.Features.ThreadOnFace(s.FaceAt(api.V(4, 0, 10)), api.ThreadOptions{Length: 12})
doc.Features.ThreadOnFace(hole, api.ThreadOptions{FullLength: true})       // a nut's thread
doc.Features.ThreadOnEdge(s.EdgeAt(api.V(0, 4, 0)), api.ThreadOptions{})   // from that rim
```

Example: [`examples/scripts/threadface`](../../examples/scripts/threadface/threadface.go).

## Cosmetic threads

**as modelled / cosmetic**, at the end of the through row (or the full-length row), chooses how
the thread is made. A modelled thread, the default, has its turns: a ridge swept along a helix.
A cosmetic thread is recorded on the part but not modelled. Drawings show a thread by
convention anyway, and most threads in an assembly need no more. A cosmetic thread:

- leaves a screw's shaft as it is when it is as thick as the thread's major diameter, and
  turns it down to that diameter when it is thicker;
- bores a nut's part to the minor diameter, the tap drill's size, unless a hole at least that
  wide already runs the whole thread. Such a hole is left as it is, whether drawn at the tap
  drill or at the nominal size;
- is one group, a sketch that keeps the thread, plus a lathe after it only where the part is
  cut. There is no helix and no Boolean on coincident flanks, so it is quick to make and to
  regenerate.

Drawings show it by the drafting convention, as they show a modelled thread. The window draws
its turns as a thin helix on the part. Its sketch's screen has **[edit thread]**. Switching a
thread between modelled and cosmetic is one step of undo. It keeps the thread's first sketch
and its lathe, so later groups stay attached to them. Crest and level starts shape only a
modelled thread. Exports write the part as it is: STL and STEP carry no thread, and mass,
interference and clearance take the part's own surfaces.

```go
c := doc.Features.ThreadOnFace(stud, api.ThreadOptions{Length: 12, Cosmetic: true}).AsThread()
o := c.Options()
o.Cosmetic = false
c = c.Edit(o) // now modelled; the helix group keeps it
```

Example: [`examples/scripts/threadcosmetic`](../../examples/scripts/threadcosmetic/threadcosmetic.go).

## The groups

**make the thread now** adds four groups: a sketch and a lathe that turn the part down or bore
it, then a ridge sketch and a helix that sweeps the ridge. With level starts, each further start
adds a ridge sketch and a helix of its own. With a chamfer, a sketch and a lathe that cut it come
last and hold the finished part (`ThreadGroup.Finished`). A thread on a face has a datum group
before them all. Undo takes them all away at once. Afterwards the thread's last group is active,
and the window shows the thread's own screen: what it is, whether its solid came out sound, and
what you can do with it.

The group list shows the thread as one row, "g007-M8-thread (screw thread, 4 groups)". **[+]**
shows its groups under it, and **[−]** folds them again. The row's box shows or hides all of
them. Its arrows move the whole thread past the group before or after it. Its radio button makes
the thread's last group active. Its name opens the thread's screen.

That screen belongs to the first helix group (for a cosmetic thread, its sketch). It keeps what
the thread was made with, in place of the helix's own rows (its angle and pitch, which the
thread sets). It has:

- **[edit thread]**, at the top;
- **[show them in the group list]**, which does what **[+]** does;
- **[suppress the thread]**, which leaves every one of its groups out of the model, and with
  them whatever uses them, until **[bring the thread back]**;
- **[delete the thread]**, which deletes its groups in one step of undo.

The screen of each of the thread's other groups says first that it is one of the thread's
groups, and has **[edit thread]**. **[edit thread]** opens the same screen as making a thread,
filled in from the thread. **make the thread again now** draws the groups again from what the
screen shows, in one step of undo. The groups keep their handles (a level start's only while
there are as many starts), and the sketches keep their lines where the thread keeps its shape.
So later groups that use the thread stay attached. A ridge taken away with a start is reported
as a broken reference. You can still edit the groups one by one, but an edit of the thread
draws them again from its screen.

From a script:

```go
doc.Features.Thread(rim, api.ThreadOptions{Designation: "M8", Length: 20})
doc.Features.Thread(hole, api.ThreadOptions{Internal: true, Through: true})
api.ThreadNearest(7.9) // "M8"
api.ThreadSizes("pipe")           // "G1/16", "G1/8", ...
api.ThreadPitches("M8")           // "M8", "M8x1", "M8x0.75"
api.ThreadDesignation("m8x1.25")  // "M8"
doc.Features.Thread(lead, api.ThreadOptions{Designation: "1/2-10 ACME", LeftHand: true})

t := g.AsThread() // g, the thread's helix group, as Features.Thread returns it
o := t.Options()
o.Designation, o.Crest, o.LengthExpr = "M8x1", "rounded", "len"
t = t.Edit(o) // the view of the group that keeps it, another after Cosmetic changes

t, ok := anyGroup.ThreadOf() // the thread a group is one of the groups of
t.SetVisible(false)          // as the box of the thread's row
t.MoveBefore(other)          // the thread's groups together
t.SetSuppressed(true)        // [suppress the thread]
t.Delete()                   // [delete the thread]
```

```go
doc.Features.Thread(rod, api.ThreadOptions{Designation: "M8", Length: 12, RunOut: 3})
doc.Features.Thread(lead, api.ThreadOptions{Designation: "M12xPh3P1", Length: 14, LevelStarts: true})
doc.Features.Thread(stud, api.ThreadOptions{Designation: "M10", Length: 20, Chamfer: true})
```

Examples: [`examples/scripts/threadsizes`](../../examples/scripts/threadsizes/threadsizes.go),
[`examples/scripts/threadedit`](../../examples/scripts/threadedit/threadedit.go),
[`examples/scripts/threadrunout`](../../examples/scripts/threadrunout/threadrunout.go),
[`examples/scripts/threadchamfer`](../../examples/scripts/threadchamfer/threadchamfer.go).

### How several starts, run-outs and chamfers are built

Without level starts, one helix sweeps all the starts. Their ridges sit a pitch apart in the
ridge sketch. That is a turn of 1/n between them, and it leaves the first (n-1) pitches with
fewer ridges.

Where a ridge runs into the run-out's cone, its flanks cross the cone at the thread's helix
angle. If a flank of the ridge's end would meet the circle where the cone begins, the end is
moved along the axis by up to a fifth of a pitch, off that circle, from where the phase put it.
Two tests sweep the run-outs: `TestThreadRunOutSweep` makes ten threads, screws and nuts from M3
to Tr20, at three lengths and five run-outs each, 150 in all. `TestThreadPhasedRunOutSweep`
makes the same ten at six phases and three run-outs, 180 in all. All come out sound. That is
what is tested, not a promise for every thread. If one does not come out sound, the thread
tries other planes for its ridges. If none works, the window says the solid is not sound, and
`Group.Analyze().Check()` says so from a script.

A chamfered thread's ridges start past the end, and for a thread to the end of the part they
stop past it too. They reach so far that the chamfer's cone takes their blunt ends whole. A
ridge that crossed the part's flat end would cut off a sliver that the Boolean loses. So the
turning or boring first cuts the end back by a cone of its own, inside the ridges' feet and
parallel to the chamfer's cone. The chamfer then takes that cone away together with the ridges'
ends, and no two faces lie on each other. Whether the Booleans succeed depends on where those
cones and the ridges' ends lie. So a chamfered thread tries seven placements of the hidden cone
and ridge ends, each at every plane for its ridges, before the window says it is not sound.
`TestThreadChamferSweep` makes eleven sizes of every form, as screws, studs, nuts and tapped
holes, at three lengths and five chord tolerances, 660 in all. All come out sound since the
Boolean's curves were refined beside grazing meets (2026-10-29).

A drawing shows the chamfer by the convention: the cone's outline at 45°, its edges thick, and
the root's thin lines running to the cone. Looking along the axis, the chamfer's circle is left
out.

## Not yet

Tapered threads and a variable pitch are not planned ("Parked" in the roadmap). A run-out
through a face is not done.
