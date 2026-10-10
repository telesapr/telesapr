# Minimum and maximum distances, and a point on a segment

Example: [`examples/scripts/limits`](../../examples/scripts/limits/limits.go), a telescopic rod
held between a shortest and a longest length, its end riding a slot.

Constrain > Minimum Distance and Constrain > Maximum Distance bound the distance between two
points, or a line segment's length, from one side. Select two points or a line; the bound is made
at the distance as it is, labelled "min 30.00" or "max 30.00", and its editor opens to type the
bound in, as a dimension's does, with expressions and names alike. Constrain > On Segment keeps a
point on a line segment between its ends, where On Point / Curve / Plane keeps it anywhere on the
segment's line.

A bound holds only where the sketch would cross it. Inside it, it takes no degree of freedom: the
points move as they would without it, and the property browser counts the freedom they have.
Dragged against it, the sketch stops at the bound, which then takes one degree of freedom, as a
dimension of that value would; a point on a segment, stopped at an end, is fixed there. Two
bounds that can't hold together, or a bound past a dimension, fail to solve, and the property
browser names the bound. A bound made at the value a dimension holds, as a maximum of 40 on a line
40 long, is no conflict: it isn't counted as taking the freedom the dimension took.

From a script:

```go
rod := doc.Line(api.V(0, 0, 0), api.V(30, 0, 0))
doc.MinDistance(rod, api.Entity{}, 20) // or two points: doc.MinDistance(a, b, 20)
doc.MaxDistance(rod, api.Entity{}, 40)
doc.OnSegment(slider, slot)
```

`Constraint.Set` and `Constraint.SetExpr` change a bound as they change a dimension.

## Not yet

The bounds are on the distance between two points (and so on a line's length) only: not on the
distance from a point to a line or a plane, a radius or an angle. A bound is labelled with "min"
and "max", as the drawing's font has no ≥ or ≤. Exports of the drawing to DXF leave the bounds out,
as they leave out every constraint but the dimensions.
