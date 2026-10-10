# Minimum and maximum dimensions, and a point on a segment

Example: [`examples/scripts/limits`](../../examples/scripts/limits/limits.go), a telescopic rod
held between a shortest and a longest length, its end riding a slot.

Constrain > Minimum Dimension and Constrain > Maximum Dimension bound a dimension from one side.
Select what Distance / Diameter or Angle takes (two points, a line, a point and a line, a
workplane or a face, a circle or an arc, two lines or normals); the bound is made at what it
measures, labelled "min 30.00" or "max 30°", and its editor opens to type the bound in, as a
dimension's does, with expressions and names alike. With a dimension selected, the same entries
make it a bound, or a dimension again when it is that bound already. A distance from a line or a
plane keeps to its side, as the dimension does, and the bound is on how far. Constrain > On
Segment keeps a point on a line segment between its ends, where On Point / Curve / Plane keeps
it anywhere on the segment's line.

A bound holds only where the sketch would cross it. Inside it, it takes no degree of freedom: the
points move as they would without it, and the property browser counts the freedom they have.
Dragged against it, the sketch stops at the bound, which then takes one degree of freedom, as the
dimension of that value would; a point on a segment, stopped at an end, is fixed there. Two
bounds that can't hold together, or a bound past a dimension, fail to solve, and the property
browser names the bound. A bound made at the value a dimension holds, as a maximum of 40 on a line
40 long, is no conflict: it isn't counted as taking the freedom the dimension took.

From a script:

```go
rod := doc.Line(api.V(0, 0, 0), api.V(30, 0, 0))
doc.MinDistance(rod, api.Entity{}, 20) // or two points, a point and a line, a plane, a face
doc.MaxDistance(rod, api.Entity{}, 40)
doc.Angle(a, b, 30).SetLimit(api.Minimum)
doc.Radius(circle, 4).SetLimit(api.Maximum)
doc.OnSegment(slider, slot)
```

`Constraint.SetLimit(api.NoLimit)` makes a bound a dimension again; `Constraint.Set` and
`Constraint.SetExpr` change a bound as they change a dimension.

## Not yet

Ratios and differences of lengths can't be bounds. A bound is labelled with "min" and "max", as
the drawing's font has no ≥ or ≤. Exports of the drawing to DXF write a bound as the dimension it
bounds.
