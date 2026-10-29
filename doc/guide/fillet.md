# Fillet

Examples: [`examples/scripts/fillet`](../../examples/scripts/fillet/fillet.go), a block with
its upright edges rounded; [`roundbox`](../../examples/scripts/roundbox),
[`tray`](../../examples/scripts/tray), [`ramp`](../../examples/scripts/ramp),
[`plinth`](../../examples/scripts/plinth), [`spinner`](../../examples/scripts/spinner) and
[`rib`](../../examples/scripts/rib) for corners, filled edges and edges of their own radius.

Features > Fillet Edges (Shift+F) rounds edges of the solid so far, 1 mm to begin with. It
takes the same selection as a chamfer:

- line segments along edges between two flat faces;
- circles where a flat face meets a cylinder or a cone, or where a cylinder and a cone, or two
  cones, meet about one axis, as a frustum turned on a shaft;
- straight or circular edges of the solid, picked on the model.

What it makes:

- An edge that sticks out is rounded off by an exact cylinder, or a torus round a circle. One
  that goes in, as a pocket's floor or a boss's foot, is filled.
- A rounded edge takes with it the edges it meets smoothly, as round a rounded rectangle.
- Where two rounded edges meet, they are mitred; where three meet at a corner of a block, a
  sphere rounds the corner.
- The foot of a rib or a boss with sharp edges can be filled all round. Where two of the foot's
  fillets meet below a sharp edge that sticks out, they are mitred outside the rib, beyond that
  edge, and where they turn by different angles, as below a sloping top, the face that slopes
  grows out to the mitre. The sharp edge is shortened to where the fillets reach up it.
- The group's screen lists the edges; each has an "[own radius]" link that gives it a radius
  of its own, and a "[remove]" link that takes it out of the fillet. "[change]" beside the list
  takes the selection as the fillet's edges instead; with nothing selected it activates the
  group before the fillet, whose edges a fillet rounds, and says what to pick. An edge's own
  radius stays with it through a change.
- Where edges of different radii meet, or edges that stick out meet edges that go in, the
  fillet rounds them one radius and kind at a time, the larger first, and the later edges go
  on round the corners the earlier ones rounded. A block's upright edges at 8 mm and its top's
  at 3 mm: the top's fillet follows the arcs the upright fillets cut in the top face, with a
  torus at each. A boss's sides and the foot that joins it to a plate work the same way.
- Where the later edges can't go on round, as a top ring larger than the upright edges, or
  three edges of three radii at a corner, or more than three rounded edges at a corner (a
  pyramid's apex), each fillet is cut short of the corner, set back, and the hole is filled
  by patches tangent to the fillets and the faces round it. The setback is half the largest
  radius past where the fillets' tangent lines cross. Corners of radii up to about 3 to 1
  are always filled; where radii 5 or 6 to 1 sit side by side the patches can fold, and the
  corner is refused at its point. Example:
  [`examples/scripts/setback`](../../examples/scripts/setback/setback.go).
- An edge that sticks out and one that goes in, meeting at a corner whose third edge is left
  sharp (a ledge's top edge with the inside upright below it, a rib's upright with one of its
  foot's edges), are blended there by one patch, tangent to both fillets and the face, the
  sharp edge ending in its corner. Where one fillet's radius is several times how far the
  other reaches along the face, the patch would fold, and the corner is refused at its point.
- Where all three edges at such a corner are rounded and the two of one kind differ in radius,
  as a ledge's two top edges at 1 and 2 mm over the inside upright, or a rib's two foot edges
  under its upright, the edge of the other kind is rounded first, and the two go on round the
  arc it leaves with a radius that goes from one's to the other's along it, tangent all round.
- At a corner of four or more edges of both kinds, as the middle of a saddle where two ridges
  and two valleys meet, all of them rounded, each fillet is set back and the hole filled by
  patches, as at a corner of three radii.
- Two top edges of different radii over an upright edge left sharp are refused, naming the
  radii: the blend would need a crease. Round the upright edge too, or give the two one
  radius.

The radius takes an expression. A radius too large for the faces is a problem on the group,
named on its screen, and the solid passes through unrounded.

From a script:

```go
f := doc.Features.Fillet(edges, 8, "")
f.AsFillet().SetRadius(6)
f.AsFillet().SetEdgeRadius(1, 3) // the second edge alone
doc.Features.FilletSolid([]api.SolidEdge{s.EdgeAt(api.V(15, 0, 5))}, 1.5, "")
```

## Between two curved faces

Where two curved faces meet along an edge that is no circle about their common axis (a branch
pipe on a pipe, a spout on a pot, a rod through a ball, a band's side on a cone), the fillet rolls
a ball of its radius along the edge, touching both faces, and the rounded face is the surface it
sweeps: fitted through the ball's arcs to within a tenth of the chord tolerance, the faces cut
back along the curves it touches them on. A chamfer is made the same way, its sections straight
between the points at its distance along each face from the edge. Such an edge, and an edge that
ends on a curved face (below), is rounded on its own: picked with other edges whose fillets don't
reach it, the others are rounded first and then each such edge alone, as the lid's four upright
edges picked together are. Where its fillet meets another picked edge's at a corner whose third
edge stays sharp, as a lid's upright edge and the bottom edge it meets, the two are mitred there,
chamfers as fillets. Fillets of different radii are mitred too: the smaller ends against the
larger on its own wall, and the larger's end turns there onto that wall, down to the corner, at
each of its ends, whatever the radii beyond; chamfers of different distances are mitred alike.
Other meetings at a vertex the two share are refused there ("pick it alone").

An edge along a crease that fades out, as a loft's from a corner of its square foot up to where
its sides meet smoothly, is rounded or chamfered up to where its faces turn by almost nothing,
and ends there; the crease goes on above it.

The edge may close on itself, as such joints do, or end where it meets a flat face at each end, as
a band's side on a cone ends at the base and at the band's end. There the fillet ends in the flat
face's plane, in the curve where that plane cuts the surface the ball sweeps, and the flat face
takes that curve as its edge; where one of the faces goes on past the flat face's plane (the cone
above the band's end), the flat face grows by the fillet's section up to where the plane cuts that
face. Where the fillet's end runs off one face past the end of its edge with the flat face, as a
wide chamfer of the band's side does past the band's top, and the face it runs onto is flat, the
fillet goes on to that face: its plane cuts the fillet too, and both the end face and that face
grow by the fillet's end. The face it runs onto may be curved, as a band's rounded top: its
surface is carried on past its edge with the face the fillet ran off, it cuts the fillet, and it
grows over the fillet's end, sharing with the end face the line or the curve where the end
face's plane meets that surface. Where the ball can't touch both faces (the radius too large
for the room beside a hole), where the fillet's end runs off both faces, where it only just runs
off onto a curved face (the face would grow by a sliver thinner than the chord tolerance: make
the fillet a little smaller or larger), or where a fillet that takes material away runs off
onto the next face, the fillet is refused at the place.

The end face may be curved too, as a box's upright edge between flat walls ends at the top on a
domed lid: the ball's sweep is cut by the dome, and the fillet's end is a curve of degree 5
fitted to where the two meet, the dome's edge too, keeping to both within half the chord
tolerance; where the dome bends too much across a wide fillet for one such curve, the fillet is
refused at the place ("make it smaller"). A chamfer ends on the curved face the same way, its end
fitted to where it meets it, as does a chamfer of a loft's foot edge where the sides meet in
creases. Where a side's face goes on past the curved face, as the cone past a band's rounded
top, the curved face grows by the fillet's end up to where the side's face meets its surface
carried on, as a flat end face does. A chamfer ending so is refused where its end can't be
fitted. Examples: [`examples/scripts/tee`](../../examples/scripts/tee/tee.go),
[`examples/scripts/bandfillet`](../../examples/scripts/bandfillet/bandfillet.go).

## Not yet

A rib rounded all over, the edges of its top rounded as well as its foot filled (round the
foot alone, or the top's edges alone), fillets between curved faces along an edge that ends at
a curved face the side faces go on past, chamfers meeting at a crease or at a corner whose third
edge stays sharp, corners where convex
meets concave at a vertex that the stages can't round with some edge there left sharp, and a
blend that takes material away and runs off onto the next face, or that runs off at both its
ends, are refused with a message; they are the rest
of roadmap 22. A crease that fades out is rounded only up to where it does. A fillet whose radius stops its chain at a rounded corner names the curved face rather than
saying so (15.7).
