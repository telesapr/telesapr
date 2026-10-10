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
group's screen lists the path and has two settings:

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
the plane that halves the corner, as a picture frame's moulding is. A corner next to a cubic is
refused, as is a twisted sweep or one held to a direction with a corner, and a piece too short for
the mitres at its ends.

A corner radius, set on the sweep's screen ("corners mitred [round]"), rounds every corner on the
path instead, as a pipe is bent: the pieces either side are cut back to where an arc of the
radius, tangent to both, joins them, and the profile turns about that arc's axis. The solid is
exact and no surface is intersected, so it works for any profile, and beside an arc, where a mitre
can't join an arc's turned sides: a corner next to an arc needs a radius, and the sweep says so
until it has one. A radius too large for a piece, too small for the profile to clear the arc's
axis, or a corner out of the plane of the arc beside it, is a problem on the group. Example:
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

## Not yet

A closed path is not done, nor a corner radius on a twisted sweep or one along a cubic. Repeating a solid along a path is
[Pattern Along Path](pattern.md).
