# Sweep

Examples: [`examples/scripts/sweep`](../../examples/scripts/sweep/sweep.go), a bent tube: a
ring swept along a straight, a bend and a straight;
[`examples/scripts/spring`](../../examples/scripts/spring/spring.go), a coil spring: a circle
swept along a helix drawn as cubics.

Features > Sweep Along Path (Shift+U) moves the active group's sketch, in its workplane, along
a path. Select the line segments, arcs and cubics of the path, joined end to end and meeting
tangent; they may lie in a workplane or anywhere in 3d. The profile's workplane must be active
(Sketch > In Workplane). The path is an ordinary sketch, so its dimensions drive the sweep.

The profile keeps its attitude to the path, turning with the path's direction and no more (by
rotation-minimising frames), so a ring swept along a helix doesn't spin about the wire. The
group's screen lists the path, with "[change]" to sweep along the selection instead and
"[remove]" on each piece's row, and has two settings:

- **twist** turns the profile about the path by that many degrees over the whole of it, evenly
  along its length: a square swept along a line and twisted by 90 is a twisted bar.
- **frames** "turn least" is the default. "held to x", "y" or "z" keeps the profile's attitude
  to that axis instead, as far as the path's direction allows: a thread's profile swept along
  a helix held to the helix's axis keeps facing it. The path must never run along the axis it
  is held to, or the group says so.

Along lines and arcs alone, untwisted and not held, the surfaces are exact: cylinders, tori and
the like. Otherwise the surface is fitted to the true sweep: the profile is placed at sections
along each piece of the path and a surface is passed through them, and sections are added until
it is within a tenth of the chord tolerance of the true sweep everywhere between them. The
pieces' surfaces meet tangent where the true sweep is smooth.

From a script:

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

A twisted sweep's sides of straight edges are twisted ruled faces, which the mesher grids with
rows along the path but no columns across: their mesh's volume is off by up to about 2%, as a
twisted loft's is; the surface itself is right. Where the path's curvature jumps, as where a
line meets an arc, a twisted sweep has a crease, since the true one has.

## Corners and edges of the solid

Two line segments of the path may meet at an angle: there the sweep is mitred, each piece cut by
the plane that halves the corner, as a picture frame's moulding is. A line and an arc, or two
arcs, may meet at an angle too, in the arc's plane. No plane can mitre them, since a plane through
the corner cuts the arc's turned sides where the line's don't pass: instead each point of the
profile is followed along both pieces, a line along the straight one and a circle round the arc,
and the sides meet where those cross. On the outside of the turn both pieces go on past the corner
to fill it, on the inside each stops where the other's side cuts it, as at a mitre between lines.
A corner too sharp for the arc beside it, where the line's side passes outside the arc's (on a
bar 2 wide round an arc of radius 12, a corner of 150°), or a piece too short for the joint, is a
problem on the group, which a corner radius then rounds. Example:
[`examples/scripts/hook`](../../examples/scripts/hook/hook.go), a round bar up a shank and round
a bend that leaves it at a right angle. A corner next to a cubic is refused, and a piece too
short for the mitres at its ends.

A sweep along a cubic, or held to a direction, is mitred at a corner between two lines too: along
a line its frames don't turn, so the two straight runs are prisms cut by the plane that halves the
corner. Held to a direction, the frames must turn at the corner as the path does, so the
direction must be the corner's axis (across the path's plane); held otherwise, the corner is a
problem on the group. A twisted sweep takes no sharp corner, since the twist turns the frames
along a line and the mitre's two sides wouldn't meet: give it a corner radius, and the twist runs
round the corner's arc. A corner beside an arc on such a sweep needs a corner radius too.
Example: [`examples/scripts/zigzag`](../../examples/scripts/zigzag/zigzag.go), a rail along a
cubic and a sharp corner, and a twisted bar round a rounded one.

A corner radius, set on the sweep's screen ("corners mitred [round]"), rounds every corner on the
path instead, as a pipe is bent: the pieces either side are cut back to where an arc of the
radius, tangent to both, joins them, and the profile turns about that arc's axis. The solid is
exact and no surface is intersected, so it works for any profile, beside an arc as beside a line.
A radius too large for a piece, too small for the profile to clear the arc's axis, or a corner out
of the plane of the arc beside it, is a problem on the group. Example:
[`examples/scripts/elbow`](../../examples/scripts/elbow/elbow.go), a duct with two rounded corners.

```go
duct := doc.Features.Sweep(path, "duct")
duct.AsSweep().SetCornerRadius(8)
```

The path may be edges of the solid so far, picked on the model, alone or with sketch entities:
a straight edge is a line, a circular one an arc, any other a curve. The sweep keeps the edges'
names, so it follows them when the model upstream changes. Example:
[`examples/scripts/bead`](../../examples/scripts/bead/bead.go), a bead along two top edges of a
block.

```go
bead := doc.Features.SweepEdges([]api.SolidEdge{front, right}, nil, "bead")
```

## Closed paths

A path that closes, a circle or pieces that come back to where they began, is swept round from
the profile and back to it, with no caps: a ring, a gasket, a picture frame. Draw the profile
across the path where it is smooth, not at a corner: the sweep starts where the profile's plane
crosses the path, at the crossing nearest the profile, so the joint where the solid closes is
smooth, and the path's corners are mitred, or rounded by a corner radius, as an open path's are.
Example: [`examples/scripts/frame`](../../examples/scripts/frame/frame.go), a picture frame
mitred at its four corners.

Round lines and arcs in a plane the profile comes back as it left, and the surfaces are exact.
Round a cubic, lines and arcs out of a plane, or with a twist, the surface is fitted, and the turn
the frames come back with, which the path's shape decides, is taken out evenly along it; a twist
must then be whole turns, or the profile wouldn't meet itself, and lines and arcs out of a plane
take no corners. A cubic in a plane takes corners between lines, mitred, as an open path's. A profile at a corner of the path, or whose plane doesn't cross it, is a problem
on the group.

```go
d.Features.SketchIn(d.Workplane("ZX"), "path")
ring := d.Sketch.Circle(api.V(10, 0, 0), 10)
d.Features.SketchIn(d.Workplane("XY"), "profile")
d.Sketch.Circle(api.V(0, 0, 0), 2)
d.Features.Sweep([]api.Entity{ring}, "o-ring")
```

## Not yet

A corner radius on a twisted sweep or one along a cubic is not done. Repeating a solid along a
path is [Pattern Along Path](pattern.md).
