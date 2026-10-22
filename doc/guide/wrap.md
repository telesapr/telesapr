# Wrap and emboss

Example: [`examples/scripts/wrap`](../../examples/scripts/wrap/wrap.go), a handle turned from a
cone and a cylinder, a word raised on the cylinder and a band of squares sunk into the cone.

Features > Wrap Sketch Onto Face rolls a sketch onto a cylinder or a cone of the solid, as paper
is rolled onto it, and raises it from the face (an emboss) or sinks it into the face (an
engraving). Draw the sketch in a workplane parallel to the face's axis, as closed loops; text
counts, each letter a loop of its own. With that sketch active (Sketch > In Workplane), select
one cylinder or cone of the solid and choose the menu entry. Faces can't be picked while
sketching unless View > Show > Selectable Faces is on. A right click on the face offers the same
as Wrap Sketch Onto This Face.

The sketch lands on the side of the face towards it, and seen from there it looks as it was
drawn. Lengths in the sketch are lengths on the face: a word 30 mm long covers 30 mm round a
cylinder, whatever its radius. On a cone the sketch is laid on the cone along the line of the
cone that faces it and rolled on from there. Lines across or along a cylinder's axis wrap
exactly; other curves within 1e-7 mm.

The group starts raised 1 mm. Its screen shows the face, the **height**, which takes a number or
an expression of the parameters, and **raised** or **sunk**. The band reaches a little through
the face, into the wall under an emboss or into the air over an engraving, so that the Boolean
crosses the face rather than touching it; that part is hidden in the result. Where the wall, or
the room before the solid starts again, is too thin for it, the group says how thin and where.

From a script:

```go
cylinder := handle.Solid().FaceAt(api.V(12, 0, 25))
doc.Features.SketchAt(doc.Origin(), api.V(1, 0, 0), api.V(0, 0, 1), "word")
doc.Sketch.Text("GRIP", "", api.V(-9, 0, 22), 6)
word := doc.Features.Emboss(cylinder, 0.8, "raised")
word.AsWrap().SetHeight(1)
// a sketch of squares, then
doc.Features.Engrave(cone, 0.4, "sunk")
```

`AsWrap().Height()` and `Engraved()` read the group back; `SetExpr(sketch.GroupPitch, "h")` gives
the height an expression.

## What it does not do yet

- Only cylinders and cones are wrapped onto, not spheres, tori, lofts or other curved faces.
- One loop with its holes, such as one letter, may go at most 350° round the axis; a row of
  letters may go further, since each is its own loop.
- An engraving as deep as the axis, and a sketch on a cone that reaches past its apex, are
  refused.
- Where the Boolean of a loop with the face fails at a corner of it, the group tries again loop
  by loop, and reaching through the face by other amounts; where none joins, it is refused with a
  problem on the group, and a smaller height may join. The solid so far passes through unchanged.
- The face can't be changed on the group's screen: delete the group and wrap again onto the
  other face.
