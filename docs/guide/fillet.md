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
  torus at each. A boss's sides and the foot that joins it to a plate work the same way. A
  corner where the later edges can't go on round, as a top ring larger than the upright
  edges, or two top edges of different radii over an upright edge left sharp, is refused,
  saying why.

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
face. Where the ball can't touch both faces (the radius too large for the room beside a hole), or
the fillet's end runs off a face past the end of its edge with the flat face, the fillet is refused
at the place. Examples: [`examples/scripts/tee`](../../examples/scripts/tee/tee.go),
[`examples/scripts/bandfillet`](../../examples/scripts/bandfillet/bandfillet.go).

## Not yet

Fillets between curved faces along an edge that ends at a curved face, and corners where more
than three rounded edges meet, or where convex meets concave at a vertex, are refused with a
message; they are the rest of roadmap 22. A fillet whose radius stops its chain at a rounded corner names the curved face rather than
saying so (15.7).
