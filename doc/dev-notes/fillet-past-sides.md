# Fillets whose foot runs past the face's sides

Refreshed 2026-11-09 at branch `rd12-curved-side`.

A circular fillet or chamfer (round a rod's foot, a hole's rim) whose foot runs off the flat face
it rolls on is refused by `contactsClear` (`pkg/topo/fillet_contact.go`), which says which sides
the foot runs into and how far (`runOff`). Where every side is one the kernel can take,
`filletPastSides` (`pkg/topo/fillet_runoff_sides.go`) makes it as it reads: the face widened
past the sides by slabs, the fillet made on the widened solid, and that solid cut back by the
sides. The cut faces take the side's identity, so the side grows up by the fillet's sections.

## Flat sides

- Square to the face: the slab is the side's outline swept outward; the cutter a box on the
  side's plane, larger than the slab every way but toward the solid.
- Leaning: `fillet_past_side_slant.go` (`slabAlong`, `slantUp`, `slabCorners`), up to 53°.
- With holes: the side taken by its outer loop; the slab caps the holes, the cut opens them.
- With the solid going on past: `fillet_past_side_beyond.go` (`beyondCutter`, `keepBeyond`).
- Just reaching, or just past, a side: `touchingSize`, `nearSize`.

## Curved sides (2026-11-09)

`fillet_past_side_curved.go` takes a side lying on a cylinder square to the face, wholly below
it, whose edges with the face make one arc or the whole circle (`curvedExtent`): a disc's rim, a
bore's wall (the solid outside the cylinder, `sgn` -1), a rounded end's arc.

- The slab takes in the whole of the cylinder's faces, as a flat side's slab takes in the whole
  side: a ring round a disc, a disc filling a bore, a sector over an arc (`curvedSlab`). Its wall
  toward the solid is a polygon inset into the solid by 5% of the radius (at most 0.5), since a
  wall on the cylinder would lie on the side; the union is checked to have grown by just the
  slab's part past the cylinder, within the chords, so a hole or notch near the rim refuses.
- The cutter is the cylinder's ring or sector from the rim outward past the slab, or for a bore
  the bore's cylinder (`curvedCutter`), from below the side to above the face by more than the
  fillet. Where any of the solid lies in it (a ledge round the disc) it is refused.
- Its circle's arcs are rational quadratics; the piece the foot runs past over is one arc of up
  to 0.9π, so that no seam of the cutter's face lies under the fillet (`arcBreaks`).

What failed first:

- Slab and cutter over only the angles the foot runs past, the rest of the rim standing on the
  same cylinder beside the cutter's curved face: once filleted, the difference lost the cut
  cylinder's bottom and ends from its trims. With the cutter a little wider than the slab, so
  that its face lay on the rim for a stretch, it failed even with no fillet. Booleans in `pkg/srf`
  join coincident planes, not coincident curved faces; so nothing of the side may be left on
  the cutter's cylinder.
- A seam of the cutter's circle under the fillet (arcs of a quarter turn from angle 0): the
  fillet's section met the seam and the trim there stayed open.
- The cutter's arc in the face's plane meets the fillet exactly at the foot, where the fillet is
  tangent to that plane: the split of the arc there, refined onto the fillet, is a double root
  and stood 2.8e-4 (more than the chord tolerance) off the foot's own split, and the trims did
  not close. `pkg/srf/boolean_split_crossing.go` puts such a split where the curve crosses an
  exact trim curve of the surface it grazes, within ten chord tolerances; only where the curve
  is tangent to the surface there (within a hundredth) and the two curves cross at more than a
  tenth of a radian. A flat side's line crossing a foot that only just reaches it crosses at a
  hundredth of a radian or less, and is left as it was (`TestFilletRodJustPastSides`).

Measured by `TestFilletPastCurvedSide` (32 of 32 within 2.7e-4 of the ring's integral, all
under, the mesh's chords) and `TestFilletPastCurvedSideRefused`.

Left: cones (a drafted disc's rim), and a solid going on past a cylinder; `ROADMAP.md`, "Fillet
leftovers".
