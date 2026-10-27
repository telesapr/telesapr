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
  | `1/4 BSW`, `3/8 BSF`, `3/8-20 BSW` | Whitworth, 55° |
  | `G1/2`, `G1 1/2` | parallel pipe thread (BSP, ISO 228-1) |
  | `1/2-10 ACME`, `1/2 ACME-2G` | general purpose ACME, 29° |

  **[choose]** lists the series (metric, unified, trapezoidal, Whitworth, pipe, ACME), the sizes
  of one, and the standard pitches of the thread's size (for M8: 1.25, the coarse one, then the
  ISO 261 fine 1 and 0.75); a click takes a size or a pitch, keeping the starts and the hand.
  **[change]** takes any designation typed; one written otherwise than typed says so beside it
  (`'M8x1.25' is written M8`), and a size the series lacks is refused with a pitch that makes a
  thread and the nearest standard size (`give its pitch, as in Tr21x4, or take the nearest,
  Tr20x4`). Whitworth crests and roots are flat at the height of the standard's rounds; the
  rounded crest comes close to the standard's crest. Buttress and tapered threads are not made;
- **right-hand** or **left-hand**, beside the pitch; `-LH` at the end of a designation is the
  same;
- **length**, in the document's unit, from the circle's plane, one and a half diameters to begin with; a plain number,
  or an expression of the parameters (Edit > Parameters), as `len` or `2*d`, which the thread
  then follows;
- **length** has the **phase** beside it, in degrees: the thread turned about its axis, its
  ridges moved along by that part of the lead (and a pitch shorter, to leave them the room). It
  fits a thread to another by hand; a thread with a run-out takes none;
- **clearance**, in the document's unit, which moves the thread away from its mate, and beside it the **class**, whose
  **[choose]** lists the tolerance classes of the thread's form and side: 6g, 6h, 6f, 6e, 4h, 8g
  for an ISO metric screw and 6H, 4H, 5H, 7H, 6G for its nut (ISO 965-1); 1A, 2A, 3A and 1B, 2B,
  3B for unified threads (ASME B1.1); 7e, 8e and 7H, 8H for trapezoidal ones (ISO 2903).
  Whitworth and ACME threads have none. A class sets the clearance to the middle of its zone:
  half the fundamental deviation plus half the pitch diameter tolerance, as a radius, so that a
  6g screw fits a 6H nut as their parts do on average (M10: 0.049 and 0.044 mm). The numbers
  come from the standards' formulas, within a few µm of their tables for the sizes checked; a
  clearance typed is the thread's own again, its class none. Made internal, a screw's class
  becomes the nut's default. A cosmetic thread keeps its class as a record and its part at the
  nominal size: the class is for whoever cuts the thread;
- **external** or **internal**: a bolt's thread, the part turned down and the ridge around it,
  or a nut's, the circle bored out and the ridge inside;
- **to the end of the part**, for a part that ends where the thread does: a screw's is turned
  down, a nut's bored, a pitch past the thread, with no shoulder; otherwise the thread stops at a
  shoulder. On a part that goes on past the thread that pitch is a groove, and the window says so
  when the thread is made;
- **flip the direction**, to run against the circle's normal: a circle at the top of a part
  points out of it, and its thread runs down into the part this way; beside it (beside the place,
  on a face),
- **chamfered end**: the thread's free end, the circle's, cut by a cone at 45° to the axis from a
  little inside the minor diameter (0.4 of a pitch inside the ridges' feet) out past the crest, as
  ISO 4753 chamfers a screw's end and a nut's mouth is countersunk at 90°, so that the thread
  starts clean rather than at a ridge's blunt end; a thread to the end of the part is chamfered at
  both ends. A cosmetic thread's part is cut so too. A modelled thread with a rounded crest can't
  be chamfered (the cone would touch the crest, not cut it), nor one too short for its chamfers;
- **crest**: **standard**, the flat crest the standards give; **truncated**, cut a quarter of the
  ridge's height lower; or **rounded**, an arc tangent to both flanks whose top is where the
  standard crest is. A printed thread is stronger and mates more easily without the point;
- **run-out**, how far before its shoulder the thread fades out, as a die leaves it: over that
  length the turned or bored surface leaves the root and tapers to the crest (to a tenth of the
  thread's depth short of it on a screw), so the ridge sinks into the part. 0 is none; a thread
  to the end of the part has no shoulder and takes none;
- **starts level**, offered for a thread of several starts: every start begins at the circle's
  plane, each a ridge and a helix of its own turned a part of a turn from the last.

The thread must fit its part. One longer than the part reaches along it, one that runs off the
part (the circle's normal pointing out of it), and a bolt's thread on a rod thinner than its root
are refused before anything is made, saying how far the part reaches. A part made shorter later,
by a parameter or an edit, leaves the thread's helix group with that problem until it is long
enough again.

## Fitting a nut to its bolt

A thread is drawn from its circle the same way whatever is to go on it, so a nut fits its bolt
only where it happens to sit a whole number and a half of pitches from where the bolt's ridge
starts; elsewhere their ridges collide. A bolt and its nut are two parts, linked into one model
(their threads can't share a solid: each one's turning or boring would cut the other's ridges),
and **Assembly > Mate Screw** of a rim of each, the bolt's and the nut's hole's, fits them. When
the two circles carry threads, a screw's and a nut's of one size, pitch, starts and hand (made
on the circle, or on the same axis of the same part, as on the hole's face or its other rim),
the mate takes the threads' lead and first moves the nut along the axis, by less than half a
pitch, to where its ridges lie between the bolt's; turning it by the mate's `turn` then keeps
them there. Mismatched threads (M8 on M10, two screws) are refused. A cosmetic thread gives the
mate its lead but has no ridges to fit, so the part stays where it is. Example:
[`examples/scripts/threadfit`](../../examples/scripts/threadfit/threadfit.go), a 6g bolt and a
6H nut screwed down it, checked for interferences at each turn.

```go
o := api.ThreadOptions{Designation: "M10", Class: "6g", Length: 25, Reverse: true}
doc.Features.Thread(end, o)
mate := doc.Assembly.Screw(boltRim, nutRim, 0) // 0: the threads' lead, and fitted
api.ThreadClasses("M10", true)                 // "6H", "4H", "5H", "6G", "7H"
```

## On a face

Select a cylindrical face of the solid, a shaft or a hole, or the circular edge at one of its
ends, and choose Features > Screw Thread... (or **Screw Thread...** on the right-click menu, or
the thread button of the toolbar). The face says the rest: a shaft carries a screw's thread and a
hole a nut's; the thread starts at the face's end that is the part's end (a shaft's tip, the
mouth of a hole), or with both ends free at the upper one, or at the end of the edge picked, and
runs along the face into the part. The screen offers the standard thread nearest a shaft, and
for a hole the one it is drawn or drilled for, at its nominal size or its tap drill's (a hole
of 8.5 mm is offered M10). In place of external or internal, through and the circle's normal it
has:

- **the face's full length**, on to begin with when one and a half diameters would reach past
  the face: the thread runs the whole face, through where the face reaches the part's far end.
  The face is measured again at every regeneration, and the thread follows its length without
  being edited: a rod made longer by a parameter carries a longer thread. A face come to reach
  the part's end, or to stop short of it, changes the shape of the turning, which the thread
  reports on its group and takes up when it is next edited; so does a face too short for the
  thread. Typing a length turns it off;
- **from the face's other end**, to start at the other end.

The thread is drawn on a construction circle of the face's diameter in a datum at the face's
end, a group before the thread's own: the datum follows the face when the part changes
upstream, and the thread with it. A hole wider than the thread's root, or a shaft thinner, is
refused. A hole drawn at the thread's nominal size is bored a little past it, by a quarter of
the ridge's overlap, so that the two cylinders don't lie on each other.

```go
s := doc.Active().Solid()
doc.Features.ThreadOnFace(s.FaceAt(api.V(4, 0, 10)), api.ThreadOptions{Length: 12})
doc.Features.ThreadOnFace(hole, api.ThreadOptions{FullLength: true})       // a nut's thread
doc.Features.ThreadOnEdge(s.EdgeAt(api.V(0, 4, 0)), api.ThreadOptions{})   // from that rim
```

Example: [`examples/scripts/threadface`](../../examples/scripts/threadface/threadface.go).

## Cosmetic threads

**as modelled / cosmetic**, at the end of the through (or full-length) row, chooses how the
thread is made. A modelled thread, the default, has its turns: a ridge swept by a helix. A
cosmetic thread is recorded on the part and not modelled, as drawings show a thread anyway and
most threads in an assembly need no more:

- a screw's thread leaves the shaft as it is when it is as thick as the thread's major diameter,
  and turns it down to that diameter when it is thicker;
- a nut's thread bores the part to the minor diameter, the tap drill's, unless a hole at least
  that wide runs the whole thread already, which it leaves as it is (a hole drawn at the tap
  drill, or at the nominal size);
- it is one group, a sketch that keeps the thread, and a lathe after it only where the part is
  cut; no helix, no Boolean on coincident flanks, so it is quick to make and to regenerate.

Drawings show it by the drafting convention, as they show a modelled thread; the window draws its
turns as a thin helix on the part. Its sketch's screen has **[edit thread]**, and switching a
thread between modelled and cosmetic is one step of undo that keeps the thread's first sketch and
its lathe, so later groups stay attached to them. Crest and level starts shape only a modelled
thread. Exports write the part as it is: STL and STEP carry no thread, and mass, interference
and clearance take the part's own surfaces.

```go
c := doc.Features.ThreadOnFace(stud, api.ThreadOptions{Length: 12, Cosmetic: true}).AsThread()
o := c.Options()
o.Cosmetic = false
c = c.Edit(o) // now modelled; the helix group keeps it
```

Example: [`examples/scripts/threadcosmetic`](../../examples/scripts/threadcosmetic/threadcosmetic.go).

## The groups

"make the thread now" adds four groups: a sketch and a lathe that turn the part down or bore it,
and a ridge sketch and a helix that sweeps it; with level starts, a ridge sketch and a helix for
each further start after them; with a chamfer, a sketch and a lathe that cut it last, which hold
the finished part (`ThreadGroup.Finished`); for a thread on a face, a datum before them. Undo takes them
all away at once. The thread's last group is active afterwards, and the window shows the thread's
own screen: what it is, whether its solid came out sound, and what to do with it.

The group list shows the thread as one row, "g007-M8-thread (screw thread, 4 groups)", and
**[+]** shows its groups under it (**[−]** folds them again). The row's box shows or hides all of
them, its arrows move the thread as one past the group before or after it, its radio button makes
the thread's last group active, and its name opens the thread's screen. That screen, on the first
helix group (a cosmetic thread's sketch), keeps what the thread was made with in place of the helix's own rows (its angle and
pitch, which the thread sets), and has:

- **[edit thread]**, at the top;
- **[show them in the group list]**, as **[+]** does;
- **[suppress the thread]**, which leaves every one of its groups out of the model, and with them
  what uses them, until **[bring the thread back]**;
- **[delete the thread]**, which deletes its groups in one step of undo.

The screen of each of the thread's other groups says first that it is one of them, with
**[edit thread]**. **[edit thread]**
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

Without level starts, several starts are swept by one helix, their ridges a pitch apart in the
ridge sketch: that is a turn of 1/n between them, and leaves the first (n-1) pitches with fewer
ridges.

Where a ridge runs into the run-out's cone, its flanks cross the cone at the thread's helix
angle. Where a flank of the ridge's end would meet the circle at which the cone begins, the end
is moved along the axis by up to a fifth of a pitch, off it. The run-outs of the sweep
(`TestThreadRunOutSweep`: ten threads, screws and nuts from M3 to Tr20, three lengths each and
five run-outs, 150 in all) come out sound. That is what is tested, not a promise for every
thread: should one not come out sound, the thread tries other planes for its ridges, and the
window says the solid isn't sound if none does, as `Group.Analyze().Check()` does from a script.

A chamfered thread's ridges start past the end (and, to the end of the part, stop past it), so far
that the chamfer's cone takes their blunt ends whole. A ridge that crossed the part's flat end
would cut a sliver of it that the Boolean loses, so the turning or the boring first cuts the end
back by a cone of its own, inside the ridges' feet and parallel to the chamfer's, which the
chamfer then takes away with the ridges' ends: no two faces lie on each other. Where the Booleans
fail depends on where those cones and the ridges' ends lie, so a chamfered thread tries seven
placements of the hidden cone and ridge ends, each at every plane for its ridges, before the
window says it isn't sound. `TestThreadChamferSweep` (eleven sizes of every form, screws, studs,
nuts and tapped holes, three lengths each, at five chord tolerances, 660 in all) has 4 unsound,
all at the coarsest chord tolerance, 0.5%; an M8 stud of 30 mm is one that no placement makes
sound at the default. A drawing shows the chamfer by the convention: the cone's outline at 45°,
its edges thick, the root's thin lines running to the cone; looking along the axis the chamfer's
circle is left out.

## Not yet

Tapered threads, variable pitch, and run-out through a face are not planned ("Parked" in the
roadmap).
