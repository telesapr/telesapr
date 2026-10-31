# Sweep

Examples: [`examples/scripts/sweep`](../../examples/scripts/sweep/sweep.go), a bent tube: a
ring swept along a straight, a bend and a straight;
[`examples/scripts/spring`](../../examples/scripts/spring/spring.go), a coil spring: a circle
swept along a helix drawn as cubics.

A sweep moves a sketch (the *profile*) along a path, and makes the solid it passes through.

## In the window

1. Draw the path: line segments, arcs and cubics, joined end to end and meeting tangent. They
   may lie in a workplane or anywhere in 3d. The path is an ordinary sketch, so its dimensions
   drive the sweep.
2. Draw the profile in a workplane, as closed loops, and make that sketch active (Sketch > In
   Workplane).
3. Select the pieces of the path and choose Features > Sweep Along Path (Shift+U).

The profile keeps its attitude to the path: it turns with the path's direction and no more
(*rotation-minimising frames*). So a ring swept along a helix does not spin about the wire.

The group's screen lists the path, with "[change]" to sweep along the selection instead and
"[remove]" on each piece's row. It has two settings:

- **twist** turns the profile about the path by that many degrees over the whole path, evenly
  along its length. A square swept along a line and twisted by 90 is a twisted bar.
- **frames**: "turn least" is the default. "held to x", "y" or "z" keeps the profile's attitude
  to that axis instead, as far as the path's direction allows. A thread's profile swept along a
  helix and held to the helix's axis keeps facing the axis. The path must never run along the
  axis it is held to, or the group says so.

Along lines and arcs alone, with no twist and no held axis, the surfaces are exact: cylinders,
tori and the like. Otherwise the surface is *fitted* to the true sweep: the profile is placed at
sections along each piece of the path, a surface is passed through them, and sections are added
until the surface is within a tenth of the chord tolerance of the true sweep everywhere between
them. (The chord tolerance is how far a mesh may stray from the true surface, set on the
configuration screen.) The pieces' surfaces meet tangent where the true sweep is smooth.

## From a script

```go
path := []api.Entity{
	doc.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 30)),
	bend,
	doc.Sketch.Line(end, end.Plus(api.V(40, 0, 0))),
}
doc.Features.SketchIn(doc.Workplane("XY"), "ring")
doc.Sketch.Circle(api.V(0, 0, 0), 6)
tube := doc.Features.Sweep(path, "tube")
tube.AsSweep().SetTwist(90)               // degrees over the whole path
tube.AsSweep().SetSweepUp(api.V(0, 0, 1)) // frames held to z; zero turns them least again
```

## Limits

A twisted sweep's sides of straight edges are twisted ruled faces. The mesher grids them with
rows along the path but no columns across, so their mesh's volume is off by up to about 2%, as
a twisted loft's is. The surface itself is right. Where the path's curvature jumps, as where a
line meets an arc, a twisted sweep has a crease, because the true one has.

## Corners in the path

Two pieces of the path may meet at an angle, a *corner*. What happens there depends on the
pieces.

**Two line segments.** The sweep is mitred: each piece is cut by the plane that halves the
corner, as a picture frame's moulding is.

**A line and an arc, or two arcs, in the arc's plane.** No plane can mitre them: a plane
through the corner cuts the arc's turned sides where the line's do not pass. Instead each point
of the profile is followed along both pieces, a line along the straight one and a circle round
the arc, and the sides meet where those cross. On the outside of the turn both pieces go on
past the corner to fill it. On the inside each stops where the other's side cuts it, as at a
mitre between lines. Example: [`examples/scripts/hook`](../../examples/scripts/hook/hook.go), a
round bar up a shank and round a bend that leaves it at a right angle.

These corners are problems on the group, which a corner radius (below) then rounds:

- a corner too sharp for the arc beside it, where the line's side passes outside the arc's (a
  bar 2 wide round an arc of radius 12 at a corner of 150°);
- a piece too short for the joint at its ends.

A corner next to a cubic is refused.

**Along a cubic, or held to a direction.** Such a sweep is fitted, but a corner between two
lines is still mitred: along a line its frames do not turn, so the two straight runs are prisms
cut by the plane that halves the corner. Held to a direction, the frames must turn at the
corner as the path does, so the held direction must be the corner's axis (across the path's
plane). Held otherwise, the corner is a problem on the group. A corner beside an arc, in the
arc's plane, is joined as on a rigid sweep: the lines and arcs about it are swept rigidly, the
rest of the path is fitted, and the two meet on the section between them. Along an arc the
frames turn about its axis, so a held direction must be that axis, and a twist is held still
there.

**Twisted.** A twist would turn the frames along a line, and the mitre's two sides would not
meet. So a twisted sweep holds its twist still along each line beside a corner, over the
stretch the mitre spans (the profile's reach from the path over the cosine of half the turn,
and a tenth more). It turns along the rest of the path, and its rate climbs from nothing over
the profile's reach, so the twisted faces leave the mitred ones tangent. Where every piece is a
line beside a corner and held whole, there is nowhere left to twist, and the corner is a
problem on the group; a corner radius then rounds it, and the twist runs round the corner's
arc. Example: [`examples/scripts/zigzag`](../../examples/scripts/zigzag/zigzag.go), a rail
along a cubic and a sharp corner, and a twisted bar round a rounded corner.

**Corner radius.** On the sweep's screen, "corners mitred [round]" takes a radius, and the row
then reads "corner radius ... [change]". Every corner on the path is rounded instead of mitred,
as a pipe is bent: the pieces either side are cut back to where an arc of that radius, tangent
to both, joins them, and the profile turns about that arc's axis. The solid is exact and no
surface is intersected, so it works for any profile, beside an arc as beside a line, and on a
fitted sweep as on a rigid one. A problem on the group: a radius too large for a piece, too
small for the profile to clear the arc's axis, or a corner out of the plane of the arc beside
it. Example: [`examples/scripts/elbow`](../../examples/scripts/elbow/elbow.go), a duct with two
rounded corners.

```go
duct := doc.Features.Sweep(path, "duct")
duct.AsSweep().SetCornerRadius(8)
```

## Along edges of the solid

The path may be edges of the solid made so far, picked on the model, alone or with sketch
entities. A right click on an edge offers Sweep Along Edges. A straight edge is a line, a
circular one an arc, any other a curve. The sweep keeps the edges' names, so it follows them
when the model upstream changes. Example:
[`examples/scripts/bead`](../../examples/scripts/bead/bead.go), a bead along two top edges of a
block.

```go
bead := doc.Features.SweepEdges([]api.SolidEdge{front, right}, nil, "bead")
```

## Closed paths

A path that closes, a circle or pieces that come back to where they began, is swept round from
the profile and back to it, with no caps: a ring, a gasket, a picture frame.

Draw the profile across the path where the path is smooth, not at a corner. The sweep starts
where the profile's plane crosses the path, at the crossing nearest the profile, so the joint
where the solid closes is smooth. The path's corners are mitred, or rounded by a corner radius,
as an open path's are. Example: [`examples/scripts/frame`](../../examples/scripts/frame/frame.go),
a picture frame mitred at its four corners.

- Round lines and arcs in a plane, the profile comes back as it left, and the surfaces are
  exact.
- Round a cubic, round lines and arcs out of a plane, or with a twist, the surface is fitted.
  The path's shape decides how far the frames have turned when they come back, and that turn
  is taken out evenly along the path. A twist must then be whole turns, or the profile would
  not meet itself. Lines and arcs out of a plane take no corners. A cubic in a plane takes
  corners between lines, mitred, as an open path's.
- A profile at a corner of the path, or whose plane does not cross the path, is a problem on
  the group.

```go
d.Features.SketchIn(d.Workplane("ZX"), "path")
ring := d.Sketch.Circle(api.V(10, 0, 0), 10)
d.Features.SketchIn(d.Workplane("XY"), "profile")
d.Sketch.Circle(api.V(0, 0, 0), 2)
d.Features.Sweep([]api.Entity{ring}, "o-ring")
```

## Not yet

A corner next to a cubic is refused, and so is a closed path out of a plane with a corner.
Repeating a solid along a path, instead of sweeping a profile, is
[Pattern Along Path](pattern.md).
