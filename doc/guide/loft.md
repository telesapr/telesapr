# Loft

Examples: [`examples/scripts/loft`](../../examples/scripts/loft/loft.go), a round-to-square
duct adapter; [`examples/scripts/vase`](../../examples/scripts/vase/vase.go), a vase lofted
smoothly through four profiles.

A loft joins the closed loop of the active group's sketch, in a workplane, to those of other
sketches. Activate one sketch, select any entity of each other one, and choose Features > Loft
To Selected Profile (Shift+O).

- With one other sketch picked, the two loops are joined by ruled faces, curve by curve.
- With two or more, one smooth surface goes through all the loops, in the order of the history
  from the active sketch, which must be the first or the last of them. Since the window shows
  only the groups up to the active one, activate the last profile and pick the earlier ones.

Loops need not have as many curves each: a circle counts as four arcs, and a loop with fewer
curves has its longest split in two until the counts match, so a triangle lofts to a square.

The group's screen lists the profiles and has "turn the second by [-1] [+1]" (for a smooth
loft, "turn those after the first"), which matches the loops' curves with an offset: a square
turned a quarter has saddle faces. A smooth loft first turns each profile to start at the curve
nearest where the one before it starts; the turn twists from there. A large twist can fold the
surface through itself, which Analyze > Check Solid reports.

From a script:

```go
l := doc.Features.Loft(bottom, top, "duct")
l.AsLoft().SetTurn(1)

v := doc.Features.LoftThrough([]Group{foot, belly, neck, lip}, "vase")
v.AsLoft().Profiles() // foot, belly, neck, lip
```

## How the smooth surface is made

Each loop becomes one B-spline curve, its pieces raised to one degree, a knot at each joint;
the curves are then interpolated across the profiles by a cubic B-spline (or a quadratic
through three), at parameters spaced by the distances between the profiles. Arcs stay exact:
circles about one axis loft to a true surface of revolution. The surface passes through each
profile exactly; where a profile has a corner, the surface has a crease there that fades out
toward smooth profiles, and the window draws it as an edge.

## Square to the ends

A loft's screen says how it *leaves the first* profile and *reaches the last*: free, shaped by
the profiles alone, or square to the profile's plane, by a number. With a number the surface
leaves the profile along its normal, its slope across there that many times the loft's length:
1 is a natural curve, more holds the normal longer. A loft of two profiles with an end set is
one smooth surface, not ruled patches. Example:
[`examples/scripts/nozzle`](../../examples/scripts/nozzle/nozzle.go), a square duct turning
into a round one, square to both.

```go
loft.AsLoft().SetTangency(1, 1) // square to both ends
loft.AsLoft().SetTangency(0, 0) // free again
```

## Along a solid's sides

A profile drawn on a face of the solid before the loft, along that face's edges, can be left
along the solid's sides instead: [along sides] beside the end on the loft's screen. At each
point of the profile the loft leaves in the tangent plane of the side face meeting the edge
there, so the side carries on into the loft without a crease; at a corner between two sides it
leaves along the line they meet in. Where the sides are square to the face that is the same as
square to it; where they lean, as a drafted extrusion's or a cone's do, the loft follows the
lean. The number is the slope's reach as for square ends, 1 where it is 0. A profile that is not
on the edges of a side below its face is the loft's problem. Example:
[`examples/scripts/spire`](../../examples/scripts/spire/spire.go), a spire on a tapered post.

```go
loft.AsLoft().SetAlongSides(true, false) // leave the first profile along the sides
```

## Guide curves

A loft can be held to guide curves: line segments, arcs and cubics drawn in a sketch before the
loft, each chain of them that joins end to end one guide, running from a point of the first
profile through every profile to the last. Activate the loft, select the curves and choose
Features > Guide Loft by Selected Curves; the loft's screen says how many curves guide it, and
[none] takes them away.

One guide moves the loft's sections across, each within its own plane, so that the loft runs
along the guide where it would have run straight; two move, turn and scale them, so that the
loft widens and narrows between them. The surface holds the guides at a dozen sections between
each two profiles and closely between them, and still goes through every profile. A guide that
misses a profile, or doesn't start or end on the end profiles, is the loft's problem, with how
far it is off. Example: [`examples/scripts/horn`](../../examples/scripts/horn/horn.go), a horn
flared by two guides.

```go
loft.AsLoft().SetGuides([]api.Entity{right, left}) // two guides
loft.AsLoft().SetGuides(nil)                       // none again
```

## Not yet

Three guides or more are refused, and a guided loft can't also follow a solid's sides; nor is
tangency set at a profile between the ends.
