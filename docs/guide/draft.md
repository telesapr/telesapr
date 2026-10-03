# Draft

Example: [`examples/scripts/draft`](../../examples/scripts/draft/draft.go), a moulded boss: a
rounded block with a bore, its sides drafted 3 degrees.

An extrusion's draft leans its sides into the solid, so that a moulded part comes out of its
mould. The sketch's plane is the neutral one: the sides pass through the sketch there, and the
far end is the sketch offset into the solid by the length times the angle's tangent. A line's
side is a plane; an arc's is a cone about the arc's centre. The outside narrows, a hole widens. A
negative draft leans the sides out. A two-sided extrusion leans both ways from the plane.

On the extrusion's group screen, `draft 0 deg [change]` takes the angle in degrees, as a number
or an expression. A draft at which the sides meet, or that takes an arc away, is a problem on the
group: the screen says the draft is left out, and the extrusion is made without it.

The extrusion's own entities at the far end and along the sides stay where they would be
without the draft, so that constraints and sketches that refer to them keep working; they are
hidden, being off the solid. The faces are picked as an extrusion's: the ends, and the side of
each line segment.

From a script:

```go
boss := doc.Extrude(20, "boss")
boss.SetDraft(3)
boss.SetExpr(sketch.GroupValC, "angle")
```

## Not yet

The sketch must be of lines and arcs, each arc meeting its neighbours tangent, as a rounded
rectangle's corners or a circle's quarters do; where an arc meets a line at a corner, the drafted
sides meet in a curve, and the draft is refused. Splines and text are refused. The far end's
entities are not moved onto the solid. A draft on chosen faces only, and a neutral plane other
than the sketch's, are not done (roadmap E13 as sized).
