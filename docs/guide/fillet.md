# Fillet

Examples: [`examples/scripts/fillet`](../../examples/scripts/fillet/fillet.go), a block with
its upright edges rounded; [`roundbox`](../../examples/scripts/roundbox),
[`tray`](../../examples/scripts/tray), [`ramp`](../../examples/scripts/ramp),
[`plinth`](../../examples/scripts/plinth) and [`spinner`](../../examples/scripts/spinner) for
corners, filled edges and edges of their own radius.

New Group > Fillet Edges (Shift+F) rounds edges of the solid so far, 1 mm to begin with. It
takes the same selection as a chamfer:

- line segments along edges between two flat faces;
- circles where a flat face meets a cylinder or a cone;
- straight or circular edges of the solid, picked on the model.

What it makes:

- An edge that sticks out is rounded off by an exact cylinder, or a torus round a circle. One
  that goes in, as a pocket's floor or a boss's foot, is filled.
- A rounded edge takes with it the edges it meets smoothly, as round a rounded rectangle.
- Where two rounded edges meet, they are mitred; where three meet at a corner of a block, a
  sphere rounds the corner.
- The group's screen lists the edges; each has an "[own radius]" link that gives it a radius
  of its own, where it meets no other rounded edge.

The radius takes an expression. A radius too large for the faces is a problem on the group,
named on its screen, and the solid passes through unrounded.

From a script:

```go
f := doc.Fillet(edges, 8, "")
f.SetRadius(6)
f.SetEdgeRadius(1, 3) // the second edge alone
doc.FilletSolid([]api.SolidEdge{s.EdgeAt(api.V(15, 0, 5))}, 1.5, "")
```

## Not yet

Fillets between two curved faces, and corners where more than three rounded edges meet, or
where convex meets concave at a vertex, are refused with a message; they are roadmap 22. A
fillet whose radius stops its chain at a rounded corner names the curved face rather than
saying so (15.7).
