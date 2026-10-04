# Fillet

Examples: [`examples/scripts/fillet`](../../examples/scripts/fillet/fillet.go), a block with
its upright edges rounded; [`roundbox`](../../examples/scripts/roundbox),
[`tray`](../../examples/scripts/tray), [`ramp`](../../examples/scripts/ramp),
[`plinth`](../../examples/scripts/plinth) and [`spinner`](../../examples/scripts/spinner) for
corners, filled edges and edges of their own radius.

New Group > Fillet Edges (Shift+F) rounds edges of the solid so far, 1 mm to begin with. It
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
- The group's screen lists the edges; each has an "[own radius]" link that gives it a radius
  of its own.
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
- Two top edges of different radii over an upright edge left sharp are refused, naming the
  radii: the blend would need a crease. Round the upright edge too, or give the two one
  radius.

The radius takes an expression. A radius too large for the faces is a problem on the group,
named on its screen, and the solid passes through unrounded.

From a script:

```go
f := doc.Fillet(edges, 8, "")
f.SetRadius(6)
f.SetEdgeRadius(1, 3) // the second edge alone
doc.FilletSolid([]api.SolidEdge{s.EdgeAt(api.V(15, 0, 5))}, 1.5, "")
```

## Between two curved faces

Where two curved faces meet along an edge that is no circle about their common axis (a branch
pipe on a pipe, a spout on a pot, a rod through a ball, a band's side on a cone), the fillet rolls
a ball of its radius along the edge, touching both faces, and the rounded face is the surface it
sweeps: fitted through the ball's arcs to within a tenth of the chord tolerance, the faces cut
back along the curves it touches them on. A chamfer is made the same way, its sections straight
between the points at its distance along each face from the edge. The edge must be picked alone.

The edge may close on itself, as such joints do, or end where it meets a flat face at each end, as
a band's side on a cone ends at the base and at the band's end. There the fillet ends in the flat
face's plane, in the curve where that plane cuts the surface the ball sweeps, and the flat face
takes that curve as its edge; where one of the faces goes on past the flat face's plane (the cone
above the band's end), the flat face grows by the fillet's section up to where the plane cuts that
face. Where the fillet's end runs off one face past the end of its edge with the flat face, as a
wide chamfer of the band's side does past the band's top, and the face it runs onto is flat, the
fillet goes on to that face: its plane cuts the fillet too, and both the end face and that face
grow by the fillet's end. Where the ball can't touch both faces (the radius too large for the
room beside a hole), where the fillet's end runs off onto a curved face or off both faces, or
where a fillet that takes material away runs off onto the next face, the fillet is refused at the
place.

The end face may be curved too, as a box's upright edge between flat walls ends at the top on a
domed lid: the ball's sweep is cut by the dome, and the fillet's end is a curve of degree 5
fitted to where the two meet, the dome's edge too, keeping to both within half the chord
tolerance; where the dome bends too much across a wide fillet for one such curve, the fillet is
refused at the place ("make it smaller"). A chamfer there is not made yet, and a face that goes
on past the curved face, so that the fillet's end would stand out of it, is refused. Examples: [`examples/scripts/tee`](../../examples/scripts/tee/tee.go),
[`examples/scripts/bandfillet`](../../examples/scripts/bandfillet/bandfillet.go).

## Not yet

Fillets between curved faces along an edge that ends at a curved face the side faces go on
past, chamfers of an edge that ends at a curved face, corners where convex
meets concave at a vertex that the stages can't round, setback corners of chamfers, and a
blend that runs off its face onto a curved face, or that takes material away and runs off onto
the next face, are refused with a message; they are the rest
of roadmap 22. A fillet whose radius stops its chain at a rounded corner names the curved face rather than
saying so (15.7).
