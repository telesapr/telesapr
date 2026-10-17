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
boss := doc.Features.Extrude(20, "boss")
boss.AsExtrude().SetDraft(3)
boss.SetExpr(sketch.GroupValC, "angle")
```

## Chosen sides and the neutral plane

A draft may lean only some sides: select them, as lines, arcs or circles of the extruded sketch,
their copies at the extrusion's ends, or the faces of the sides, with the extrusion active, and
choose Features > Draft Selected Sides. The group's screen says how many sides are drafted, and
*[all]* drafts every side again. The rest stay upright.

The sides lean from the sketch's plane to begin with. The screen's *from the plane* row moves that
plane, the neutral one, along the extrusion: there the solid has the sketch's outline, and it
narrows going away from it both ways, as a moulded part does from its parting line. At the far
end, the foot is the smaller end. A plane off the extrusion is a problem on the group.

Where an arc meets a line at a sharp corner, as a D's arc meets its flat side, the arc's cone and
the line's plane meet in a conic, which the solid follows exactly. Example:
[`examples/scripts/parting`](../../examples/scripts/parting/parting.go).

From a script:

```go
boss.AsExtrude().SetDraft(3)
boss.AsExtrude().SetDraftNeutral(8)                // lean from the plane 8 up
boss.AsExtrude().SetDraftFaces([]api.Entity{arc})  // only the arc's side
```

## Not yet

The sketch must be of lines and arcs. Where two arcs meet at a sharp corner, their cones meet in
a curve of degree four, which the solid follows within a millionth of a millimetre. Sides that
meet tangent must lean alike. Splines and text
are refused. The far end's entities are not moved onto the solid. A draft about a plane that is
not square to the extrusion is not done.
