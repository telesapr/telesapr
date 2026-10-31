# Fillet

A fillet rounds an edge. A convex edge (one that sticks out) is rounded off. A concave edge
(one that goes in, like the floor of a pocket or the foot of a boss) is filled in.

Examples:

- [`examples/scripts/fillet`](../../examples/scripts/fillet/fillet.go): a block with its upright
  edges rounded.
- [`roundbox`](../../examples/scripts/roundbox), [`tray`](../../examples/scripts/tray),
  [`ramp`](../../examples/scripts/ramp), [`plinth`](../../examples/scripts/plinth),
  [`spinner`](../../examples/scripts/spinner) and [`rib`](../../examples/scripts/rib): corners,
  filled edges, and edges with a radius of their own.
- [`soap`](../../examples/scripts/soap): a rim rounded all the way round.
- [`knob`](../../examples/scripts/knob): a fillet beside a chamfer.
- [`casing`](../../examples/scripts/casing): two radii in one fillet.

## In the window

Choose Features > Fillet Edges (Shift+F). It rounds edges of the solid built so far. The radius
starts at 1 mm. Select the same kind of things as for a chamfer:

- line segments that lie along edges between two flat faces;
- circles where a flat face meets a cylinder or a cone, or where a cylinder and a cone, or two
  cones, meet about one axis, such as a frustum turned on a shaft;
- straight or circular edges of the solid, picked on the model.

The radius takes a number or an expression. A radius too large for the faces is a problem on
the group, named on its screen, and the solid passes through unrounded.

The group's screen lists the edges. Under each edge, "[own radius]" gives that edge a radius of
its own, and "[remove]" takes the edge out of the fillet. "[change]" beside the list takes the
current selection as the fillet's edges instead. With nothing suitable selected, it activates
the group before the fillet, whose solid the edges are rounded on, says what to pick, and the
link reads "[take the selection]" until you do. An edge's own radius stays with it through such
a change.

From a script:

```go
f := doc.Features.Fillet(edges, 8, "")
f.AsFillet().SetRadius(6)
f.AsFillet().SetEdgeRadius(1, 3) // the second edge alone
doc.Features.FilletSolid([]api.SolidEdge{s.EdgeAt(api.V(15, 0, 5))}, 1.5, "")
```

## What it makes

- A convex edge is rounded by an exact cylinder, or by a torus round a circle. A concave edge
  is filled.
- A rounded edge takes with it the edges it meets smoothly, as round a rounded rectangle.
- Where two rounded edges meet, they are mitred. Where three meet at the corner of a block, a
  sphere rounds the corner.
- The foot of a rib or a boss with sharp edges can be filled all round. Where two of the foot's
  fillets meet below a sharp convex edge, they are mitred outside the rib, beyond that edge.
  Where they turn by different angles, as below a sloping top, the sloping face grows out to the
  mitre. The sharp edge is shortened to where the fillets reach up it.

**Edges of different radii, or of both kinds.** Where edges of different radii meet, or convex
edges meet concave ones, the fillet rounds them one radius and one kind at a time, the larger
radius first. The later edges go on round the corners the earlier ones rounded. For example, a
block's upright edges at 8 mm and its top edges at 3 mm: the top's fillet follows the arcs the
upright fillets cut in the top face, with a torus at each. A boss's sides and the foot that joins
it to a plate work the same way.

**Setback corners.** Sometimes the later edges can't go on round the corner: a top ring larger
than the upright edges, three edges of three radii at a corner, or more than three rounded edges
at a corner (a pyramid's apex). Then each fillet is cut short of the corner (set back), and the
hole is filled by patches tangent to the fillets and the faces round it. The setback is half the
largest radius past where the fillets' tangent lines cross. Corners with radii up to about 3 to 1
are always filled. Where radii of 5 or 6 to 1 sit side by side, the patches can fold, and the
corner is refused at its point. Example:
[`examples/scripts/setback`](../../examples/scripts/setback/setback.go).

**A convex and a concave edge meeting, the third edge left sharp.** Take a ledge's top edge with
the inside upright below it, or a rib's upright with one of its foot edges. The two fillets are
blended there by one patch, tangent to both fillets and the face, and the sharp edge ends in its
corner. Where one fillet's radius is several times how far the other reaches along the face, the
patch would fold, and the corner is refused at its point.

**All three rounded, two of one kind with different radii.** For example, a ledge's two top
edges at 1 and 2 mm over the inside upright, or a rib's two foot edges under its upright. The
edge of the other kind is rounded first. The two then go on round the arc it leaves, with a
radius that changes from one's to the other's along it, tangent all round.

**Four or more edges of both kinds at a corner.** Take the middle of a saddle, where two ridges
and two valleys meet, all rounded. Each fillet is set back and the hole is filled by patches,
as at a corner of three radii.

**Refused.** Two top edges of different radii over an upright edge that stays sharp are refused,
and the message names the radii: the blend would need a crease. Round the upright edge too, or
give the two edges one radius.

## Between two curved faces

Two curved faces can meet along an edge that is not a circle about a shared axis: a branch pipe
on a pipe, a spout on a pot, a rod through a ball, a band's side on a cone. There the fillet
rolls a ball of its radius along the edge, touching both faces. The rounded face is the surface
the ball sweeps. It is fitted through the ball's arcs to within a tenth of the chord tolerance
(how far a mesh may stray from the true surface, set on the configuration screen). The faces
are cut back along the curves where the ball touches them. A chamfer is made the same way, with
straight sections between the points at its distance along each face from the edge.

Such an edge, and an edge that ends on a curved face (below), is rounded on its own. When it is
picked together with other edges whose fillets don't reach it, the others are rounded first,
then each such edge alone. A lid's four upright edges picked together are done this way. Where
its fillet meets another picked edge's fillet at a corner whose third edge stays sharp, as a lid's
upright edge meets the bottom edge, the two are mitred there, chamfers the same as fillets.
Fillets of different radii are mitred too: the smaller ends against the larger on its own wall,
and the larger's end turns there onto that wall, down to the corner, at each of its ends,
whatever the radii beyond. Chamfers of different distances are mitred alike. Any other meeting
at a vertex the two share is refused there, with the message "pick it alone".

An edge along a crease that fades out, as a loft's crease from a corner of its square foot up to
where its sides meet smoothly, is rounded or chamfered up to where its faces turn by almost
nothing, and ends there. The crease goes on above it.

**Where the edge ends.** The edge may close on itself, as such joints do. Or it may end where it
meets a flat face at each end, as a band's side on a cone ends at the base and at the band's end.
There the fillet ends in the flat face's plane, along the curve where that plane cuts the ball's
swept surface, and the flat face takes that curve as its edge. Where one of the faces goes on
past the flat face's plane (the cone above the band's end), the flat face grows by the fillet's
section up to where the plane cuts that face.

The fillet's end may run off one face past the end of its edge with the flat face, as a wide
chamfer of the band's side does past the band's top. If the face it runs onto is flat, the fillet
goes on to that face: its plane cuts the fillet too, and both the end face and that face grow by
the fillet's end. If the face it runs onto is curved, as a band's rounded top, that surface is
carried on past its edge with the face the fillet ran off, it cuts the fillet, and it grows over
the fillet's end, sharing with the end face the line or curve where the end face's plane meets
that surface.

The fillet is refused at the place in these cases:

- the ball can't touch both faces (the radius is too large for the room beside a hole);
- the fillet's end runs off both faces;
- it only just runs off onto a curved face, so the face would grow by a sliver thinner than the
  chord tolerance (make the fillet a little smaller or larger);
- a fillet that takes material away runs off onto the next face.

**A curved end face.** The end face may be curved too, as a box's upright edge between flat walls
ends at the top on a domed lid. The ball's sweep is cut by the dome, and the fillet's end is a
curve of degree 5 fitted to where the two meet, the dome's edge as well, keeping to both within
half the chord tolerance. Where the dome bends too much across a wide fillet for one such curve,
the fillet is refused at the place with "make it smaller". A chamfer ends on a curved face the
same way, its end fitted to where it meets it. So does a chamfer of a loft's foot edge where the
sides meet in creases. Where a side's face goes on past the curved face, as the cone past a
band's rounded top, the curved face grows by the fillet's end up to where the side's face meets
its surface carried on, as a flat end face does. A chamfer ending this way is refused where its
end can't be fitted.

Examples: [`examples/scripts/tee`](../../examples/scripts/tee/tee.go),
[`examples/scripts/bandfillet`](../../examples/scripts/bandfillet/bandfillet.go).

## Not yet

These are refused with a message:

- A rib rounded all over, with the edges of its top rounded as well as its foot filled. Round
  the foot alone, or the top's edges alone.
- A fillet between curved faces along an edge that ends at a curved face which the side faces go
  on past.
- Chamfers meeting at a crease, or at a corner whose third edge stays sharp.
- A corner where a convex edge meets a concave one at a vertex that the stages can't round while
  some edge there is left sharp.
- A blend that takes material away and runs off onto the next face, or that runs off at both of
  its ends.

Also:

- A crease that fades out is rounded only up to where it fades.
- A fillet whose radius stops its chain at a rounded corner names the curved face in its message
  rather than saying that the radius is the problem.
