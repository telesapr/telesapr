# Constraints

Example: [`examples/scripts/ratio`](../../examples/scripts/ratio/ratio.go), a tray whose depth
is held to its width by a ratio and whose pocket is held to the outline by a difference, so one
dimension sizes the whole tray; [`equal`](../../examples/scripts/equal) and
[`text`](../../examples/scripts/text) and [`overlap`](../../examples/scripts/overlap) for the others below.

SolveSpace's constraints all hold, under the Constrain menu. TeleSAPR adds these, each one undo
step, each with a script call on `doc.Constrain`.

## Ratios and differences of lengths

Constrain > Length / Arc Ratio (Z) makes one length a multiple of another; Length / Arc
Difference (J) makes it longer by an amount. Each takes two line segments, two arcs (their
lengths along them), or a line segment and an arc, in which case the ratio or difference is the
line's over the arc's. A difference is a size, taken without its sign: for a shorter than b,
select them the other way round.

```go
constrain.Ratio(outer[1], outer[0], 0.5)       // depth is half the width
constrain.Difference(outer[0], pocket[0], 8)   // the pocket 8 mm narrower
```

## Equal distances and angles

Constrain > Equal Length / Radius (Q) also takes:

- a point and two line segments: the point as far from the first as the second is long;
- a line segment and two points, or two line segments with a point each: the points as far from
  their line as each other.

Constrain > Angle / Equal Angle (N) with three line segments or normals makes the two angles
that share the middle one equal; with four, the angles of the first pair and of the second.

```go
doc.Constrain.Equal(line, p, q)          // p and q equally far from line
doc.Constrain.EqualAngle(a, b, c)        // angle(a,b) == angle(b,c)
```

## Distance along a direction

Distance / Diameter with two points and a line segment or a normal measures the distance along
that direction, with its sign: the distance between two parallel faces, or a point's height
over a plane. `Constrain.ProjectedDistance(a, b, along, v)`.

## Faces, tangents, midpoints, symmetry, comments

- **Parallel / Tangent** and **Perpendicular** take two flat faces of the solid as well as line
  segments and normals (`Constrain.Parallel`, `Perpendicular`).
- **Tangent** for curves that don't share an end: select the end points to make tangent too
  (`Constrain.Tangent(arc, line, arcEnd)`; for two curves, an end of each).
- **At Midpoint** works inside a workplane; **Symmetric** with only a line segment selected makes
  its two ends symmetric about the workplane's axis, dropping its horizontal or vertical
  (`Constrain.Symmetric(line, Entity{}, Entity{})`).
- **Comment** with a point selected writes the text beside that point and follows it
  (`Constrain.CommentOn`); dragging a label, or `Constraint.SetLabelOffset`, moves it.
- A diameter can be shown as a radius from its screen (`Constraint.SetShowAsRadius(true)`).

## What lies at a point

When the property browser describes a point, ALSO HERE lists the other points at the same place
and the curves passing through it, so that what a click can't reach is picked from the list.
From a script, `Entity.Overlap()` returns them, and `Edit.Select` selects one.

## Text

A text's screen changes its string, its font and its kerning, keeping its corner, direction,
height and constraints. `Entity.SetText`, `SetFont` (a name from `Sketch.Fonts()`, "" for the
built-in font) and `SetKerning`.

## Not yet

A ratio or difference takes lengths of lines and arcs, not of cubics or of circles' radii.
