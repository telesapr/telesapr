# Draft

A draft leans the sides of an extrusion inward, so that a moulded part comes out of its mould.

Example: [`examples/scripts/draft`](../../examples/scripts/draft/draft.go), a moulded boss: a
rounded block with a bore, its sides drafted 3 degrees.

## What it does

The sketch's plane is the neutral plane: the sides pass through the sketch there. The far end is
the sketch's outline offset into the solid by the length times the tangent of the angle. A line's
side becomes a plane. An arc's side becomes a cone about the arc's centre. The outside narrows
and a hole widens. A negative angle leans the sides out instead. A two-sided extrusion leans both
ways from the plane.

The extrusion keeps its own entities at the far end and along the sides where they would be
without the draft, so constraints and sketches that refer to them keep working. They are hidden,
since they are off the solid. The faces are picked as an extrusion's: the ends, and the side of
each line segment.

## In the window

On the extrusion's group screen, the row `draft 0 deg [change]` takes the angle in degrees, as a
number or an expression. A draft at which the sides would meet, or that takes an arc away, is a
problem on the group: the screen says the draft is left out, and the extrusion is made without
it.

From a script:

```go
boss := doc.Features.Extrude(20, "boss")
boss.AsExtrude().SetDraft(3)
boss.SetExpr(sketch.GroupValC, "angle")
```

## Chosen sides and the neutral plane

A draft may lean only some of the sides. With the extrusion active, select them as lines, arcs
or circles of the extruded sketch, as their copies at the extrusion's ends, or as the side faces
themselves. Then choose Features > Draft Selected Sides. The group's screen says how many sides
are drafted, and *[all]* drafts every side again. The other sides stay upright.

The sides lean from the sketch's plane to begin with. The screen's *from the plane* row moves the
neutral plane along the extrusion. At that plane the solid has the sketch's outline, and it
narrows going away from it both ways, as a moulded part does from its parting line. At the far
end, the foot is the smaller end. A plane that lies off the extrusion is a problem on the group.

Where an arc meets a line at a sharp corner, as a D's arc meets its flat side, the arc's cone and
the line's plane meet in a conic curve, which the solid follows exactly. Example:
[`examples/scripts/parting`](../../examples/scripts/parting/parting.go).

From a script:

```go
boss.AsExtrude().SetDraft(3)
boss.AsExtrude().SetDraftNeutral(8)                // lean from the plane 8 up
boss.AsExtrude().SetDraftFaces([]api.Entity{arc})  // only the arc's side
```

## Not yet

- The sketch must be made of lines and arcs. Splines and text are refused.
- Where two arcs meet at a sharp corner, their cones meet in a curve of degree four. The solid
  follows it to within a millionth of a millimetre, not exactly.
- Sides that meet tangent must lean by the same angle.
- The far end's entities are not moved onto the drafted solid.
- A draft about a plane that is not square to the extrusion is not done.
