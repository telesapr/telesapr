# Fillets whose foot runs past the face's sides

Refreshed 2026-11-09 at branches `rd13-fillet-cone` and `rd13-runoff-both`.

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
  fillet. A ledge round the disc below the side is taken since 2026-11-09 (below); any other
  of the solid lying in it is refused.
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
under, the mesh's chords).

## Cones, and ledges below a curved side (2026-11-09)

`fillet_past_side_cone.go` takes a side on a cone about an axis square to the face
(`coneSideOf`, by `srf.RoundFaceOf`): a drafted disc's rim, leaning in or out, or a drafted
bore. `curvedSide.k` is how much the radius grows per unit up the axis (0 on a cylinder), and
the cylinder's code takes it where it reads the radius (`radiusAt`, `radii`, `squared`).

- The slab is the cylinder's prism, its wall toward the solid clearing the cone at every height
  of the side, inset by 2% of the radius (at most 0.2: at one end of the side it stands off the
  rim by the cone's spread too, and 0.4 there reached a rod beside a drafted bore). A full
  side's corners are turned half a step: a corner on the solid's seam at angle 0 left a face of
  the slab's outer wall standing after the cut back round a disc drafted 10°.
- The cutter is the cone's profile revolved through `arcBreaks` by
  `srf.SShell.MakeFromRevolutionBetween` (pieces where the caller says, caps on a partial turn),
  the pieces of its caps merged into one plane each: left in pieces, faces of the slab stood
  after the cut back. The profile's cone line is split at the side's foot, so that the plane
  of the solid's bottom (or a ledge's top) finds the cutter's own exact circle there; marched,
  it left the bottom open. It is not split in the face's plane: a seam there, where the fillet's
  foot crosses the rim tangent to the plane, left the trims open.
- The cone goes on above the face as far as the cutter reaches (r + 1). Where the solid stands
  in it there (a rod 9 from the axis beside a bore drafted 10°, which is 9.06 across 2 above the
  face) or beside the side, the fillet is refused, saying so (`curvedSide.blocked`).

Where the solid goes on below a cylinder or cone side (a ledge round a disc), the cutter starts
at the side's foot, its bottom on the ledge's top, coplanar faces the Boolean joins. Cutting the
cutter by the solid, as `beyondCutter` does past a flat side, puts the cutter's curved face on
the side: the cone cutter less the block round a drafted bore lost that face's loops, and less
what of the solid lies above the face it kept the rod's cylinder lying on the rod's. A fillet
taking material away whose cut reaches below the foot would cut into the ledge, which the
cutter leaves alone; it is refused where the solid below the foot's plane changes
(`keepBeyond`, conservatively for a hole whose cut there runs all round it).

Measured by `TestFilletPastConeSide`: a drafted disc (5°, -5°, 10°) with a rod off centre, a
drafted disc's hole, a drafted bore beside a rod, an upright or drafted disc on a ledge with a
rod or a hole; fillets and chamfers of 2.2 to 3.5: 80 of 80 sound, within 2.4e-4 of the ring's
integral within the cone (the heights inside it found exactly). `TestFilletPastConeSideRefused`:
the rod beside the widening bore, a hole's rim chamfered into the ledge.

Left: a solid standing in the cone or cylinder carried on above the face (the solid past the side
put back, as `keepBeyond` does past a flat side, would need the cutter's curved face to meet the
solid's own), and a hole's rim cutting into a ledge; `ROADMAP.md`, "Fillet leftovers".

## An end running off both its faces (2026-11-09)

Not a circular foot but the rolling ball's open end (`pkg/topo/rollingrunoff*.go`): the end
section, in the flat end face's plane, can reach past the far end of both sides' edges with the
end face, as a plate's foot on a cone whose top is low at the end and whose cone is cut flat past
the plate's side. Each side's run-off alone was made; both at once were refused, because the end
cut (`rbEndCut`) held one run-off. It now holds a second (`also`, `runOffs`, `offOn`), and
`rollingrunoff_both.go` makes the end:

- each side is cut as a run-off alone is (`runOffCuts`: p1 on the end section, p2 where the
  contact comes back under the next face, that face's edge with the side split at p2);
- the patches at the end are cut by both next faces' planes in turn, with `clipConsumed`, which
  the fillet taking a face whole already used for the two ends' planes (`clipRunOffs`);
- the end face loses both its edges with the sides and takes the end section from one p1 to the
  other (`endSectionPart`), and each next face is closed as for one run-off (`closeNext`, split
  out of `closeRunOff` with the face edits it shares, `runOffEdit`).

Refused by name: the two cuts crossing on the fillet or on the end section, where the next faces
would meet across it (no test reaches it: the plate's foot on the cone runs past the flat all along
the edge first), a fillet taking material away at either, and both faces at one end with a run-off
at the other. Measured by `TestBlendRunsOffBothFaces` against the blend that runs off neither (the
top high, the cone uncut) less what of it lies above the low top's plane and past the flat,
measured by vertical lines through its mesh above the plane and above the cone's exact surface:
12 of 12 within 0.15%, 0.08% but for the fillet of 5.5. The roadmap's other way, the setback
patch, was not needed.

The ring band's 0.3% to 0.42% (`TestBlendRunsOffPastRingTop`) was the measure, not the kernel nor
the oracle. Vertical lines through the meshes, each class of column apart (the blend's region
above the exact cone, the band, under the cone), showed the blends as made matching the oracle
within 0.002% in the blend's region, the cone's mesh under them changing alike with and without a
run-off, and the whole excess in a strip a few thousandths wide at the band's outer side: the side's
chords lie inside its exact cylinder, so the band's mesh lacks that sliver, and the blend fills it
wherever it takes the side away, up to the top where the end runs off (0.09 mm³ of a chamfer of 2.5
at the top of 22). The test now measures both blends over the blend's region above the exact cone
(`ringAdded`): 53 of 53 within 0.04%.
