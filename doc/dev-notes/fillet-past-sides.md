# Fillets whose foot runs past the face's sides

Refreshed 2026-11-09 at branches `rd13-fillet-cone` and `rd13-runoff-both`.
Refreshed 2026-11-18 at branch `runoff-mixed-ends`.
Refreshed 2026-11-19 at branch `fillet-curved-side-cut`.
Refreshed 2026-11-19 at branch `ledge-cut-past-side`.
Refreshed 2026-11-19 at branch `ledge-cutback-35`.
Refreshed 2026-11-19 at branch `ledge-touch-finechord`.
Refreshed 2026-11-23 at branch `fillet-runoff-cuts-cross`.

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
  slab's part past the cylinder, within the chords, so a hole or notch the wall reaches refuses.
  Since 2026-11-19 the inset is at most half the room the solid's other faces leave inside the
  side, less the sag of the wall's chords (`curvedSide.clearance`: the corners of the mesh's
  triangles clipped to the side's height, flat faces square to the axis left out, whose corners
  by the rim stand off it by their outline's chords, coarser than the chord tolerance where the
  segment limit holds): a hole 0.3 from a disc's rim, which the wall at 0.4 cut into.
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
  face), the cutter is cut by what of the solid stands above the face's plane, the solid in a box
  whose bottom lies on the face (`curvedSide.lessAbove`, since 2026-11-19); the cut back then
  keeps the rod. Cut by the whole solid, as `beyondCutter` does past a flat side, the cutter's
  cone is split where it lies on the bore, at the bore's rims, and the cut back failed there.
  Where the solid stands in the cutter beside the side, below the face, the fillet is refused,
  saying so (`curvedSide.blocked`). The cut back by the cutter less the rod needed three changes
  in `pkg/srf` (`doc/dev-notes/k3-fuse-spike.md`, "The fillet past a curved side carried on").

Where the solid goes on below a cylinder or cone side (a ledge round a disc), the cutter starts
at the side's foot, its bottom on the ledge's top, coplanar faces the Boolean joins. Cutting the
cutter by the solid, as `beyondCutter` does past a flat side, puts the cutter's curved face on
the side: the cone cutter less the block round a drafted bore lost that face's loops, and less
what of the solid lies above the face it kept the rod's cylinder lying on the rod's. A fillet
taking material away whose cut reaches below the foot would cut into the ledge, which the
cutter leaves alone; it is refused where the solid below the foot's plane and past the side
changes (`keepBeyond`, `curvedSide.volumePastBelow`: the flux of a radial field that vanishes on
the side's cylinder or cone, whose divergence is 1 past it, through the mesh clipped below the
foot). Until 2026-11-19 the whole half space below the foot was compared, which counted a hole's
rim chamfered into the ledge within the side, the chamfer's own cut, and refused it.

Measured by `TestFilletPastConeSide`: a drafted disc (5°, -5°, 10°) with a rod off centre, a
drafted disc's hole, a drafted bore beside a rod, an upright or drafted disc on a ledge with a
rod or a hole, since 2026-11-19 a rod 9 from the axis of a bore widening at 10°, a hole's rim
cut 3 and 3.5 deep into the ledge and a hole on the ledge's disc 0.3 from its rim (2.2 to 3);
fillets and chamfers of 2.2 to 3.5: 102 of 102 sound, within 2.4e-4 of the ring's integral
within the cone (the heights inside it found exactly). `TestFilletPastConeSideRefused`: a hole's
rim 0.3 from the disc's side chamfered 3.5 deep, its cut into the ledge reaching past the side,
refused by name since 2026-11-19 (`curvedSide.cutsBelow`, set where `keepBeyond` finds the
solid past the side below the foot changed).

That case failed first in the slab, not the cut back: the wall inset 0.4 filled the hole's
sliver past it, 0.66 mm³ more than the slab's part past the side (`widenPastCurved`). With the
inset from the room left (above), the fillet and the cut back are sound and the solid past the
side below the foot is 0.041 mm³ short, the chamfer's cut into the ledge past r = 8. A sweep of
the family (`ledge-cut-past-side`, not kept as a test): a hole of radius 4 on the ledge's disc
0.2, 0.3, 0.5 and 0.8 from the rim at its top, upright and drafted 5°, fillets and chamfers of
2.2 to 3.5: 43 of 64 made before, 57 of 64 after, all made within 1.4e-4 of the integral. Left:
the two chamfers of 3.5 at 0.2 and 0.3 upright (by name); the chamfer of 3.5 at 0.5 upright,
whose cone meets the side's foot circle at one point at the ledge's top, and the fillets and
chamfers of 3.5 at 0.5 and 0.8 drafted 5°, all failing in the cut back (a loop of the slab's
outer wall not closing at the ledge's top, the chamfer's at the touching point), refused as
running off the face before and after.

Those five were made at branch `ledge-cutback-35` (2026-11-19), 62 of the 64:

- **The drafted four** failed in the raycast, not the cut back's construction. A face of the
  slab's outer wall at radius 15.4, wholly inside the cutter, kept its sides and top and lost
  its bottom on the cutter's bottom plane, and the loop did not close. The chain of its sides
  was classified by raycast from a point of its top edge; the first ray left the cutter through
  its cap 0.012 inside the far arc (radius 16.43), and the cap's trim put that point outside:
  the far arc is one piece of 156° under the fillet (`arcBreaks`), cut into the most chords a
  curve may have (`Tolerance.MaxSegments`), which sag 0.015 at a chord tolerance of 0.0005,
  thirty times it. The hit was 0.007 off the chords, past the band within which a ray's hit
  near a curved trim is not trusted (the chord tolerance), and the ray, meeting nothing else
  ahead, read outside. The band is now each chord's sag where that is more
  (`srf.curvedClearance`, the sag L·θ/8 from the turn between neighbouring chords): such a hit
  is not trusted and another ray decides. No golden moved; a chord's band is the tolerance as
  before wherever its curve is cut finely enough (`TestCurvedClearanceCoarseChords`).
- **The upright chamfer at 0.5** has its cone meet the ledge's top in a circle of 4.5 that
  touches the foot's circle of 8 from inside at one point. The cut back lost the ledge's top
  and the side on one half of the disc (543 naked edges). Mapped by the hole's distance from
  the rim: failing from 0.500 to 0.504 at a chord tolerance of 0.0005 (the circles 0 to 0.004
  apart, eight times the tolerance), made at 0.499 and 0.4995, the circles crossing by 0.0005,
  where the cut back finds the crossing and the sliver it cuts into the ledge past the side is
  within `keepBeyond`'s allowance, and made at 0.505; at a tolerance of 0.002 only 0.504 failed.
  The band does not follow the tolerance and is no chord's sag; where in the Boolean it breaks
  was not found (`doc/dev-notes/k3-fuse-spike.md`). A size moved within the chord tolerance cannot
  move the circles out of it from inside. Such a fillet is made at the nearest size at which
  the circles cross by a quarter to twice the tolerance (`footClearSize`, the cut's outline in
  the foot's plane measured from the edge's sections, `curvedSide.footReach`), as `nearSize`
  makes a fillet only just past a flat side; where none does it is refused, "its cut below the
  face only just meets the side's foot", for a cut within ten tolerances inside the foot
  (`TestFilletPastConeSideRefused`, the hole at 0.502). The chamfer at 0.5 is made at a size
  0.0007 larger, within 1.2e-4 of the ring's integral at 3.5.

`TestFilletPastConeSide` takes the five in (108 of 108). Left in the family: the two chamfers of
3.5 at 0.2 and 0.3 upright, by name (`cutsBelow`).

Both followed up at branch `ledge-touch-finechord` (2026-11-19):

- **Where the Boolean broke** for the circles 0 to 0.004 apart. The cutter's circle at the
  side's foot is one arc of 143° under the chamfer (`arcBreaks`) in 32 chords, which sag up to
  0.005 there: the band of failures was that sag, not the tolerance's. The chords dipped into the
  chamfer's cone where the circle stays outside it; each crossing was refined onto the cone
  (`PointOnCurve`), which with no root stops where the two come closest, 0.0013 off the circle,
  and the circle was split there, the marcher took a seed there and went off along the cone and
  the cutter's wall a second time, and the cone's circle on the ledge's top kept pieces between
  the chords and the arc, inside the ledge's face. A split or seed refined off its exact curve by
  more than the chord tolerance is now dropped, and the chord halved with the curve's own points
  until it no longer crosses (`srf.missesCurve`, `srf.halveMissed`), so the mesh keeps to its
  side too. Within the chord tolerance a near miss is left as it was, to the fuse
  (`snapBetween`): a pin 0.03 over a bore at 0.1 is fused there (`TestBetweenFusedSaysHowFar`),
  and dropping its splits made the Boolean pass and come out wrong. The fixture is
  `srf.TestBooleanTouchingRingSweep`: a disc with a conical hole less, with and united with a
  ring whose bottom's inner circle lies 1 to 20 chord tolerances outside the hole's, the touching
  point on a seam of either or inside a patch, at 0.0005 and 0.002: 42 of 168 failed, 0 now.
  The holes 0.5005 to 0.504 from the rim are made at 3.5 (`TestFilletPastConeSideNearFoot`).
  Touching exactly (0.500) the Booleans still fail: near the point the two circles lie within
  `LengthEps` of each other for 0.009 along them, and seeds land anywhere there; handing the
  point over as a touch (`SShell.Touch`) did not help, the marcher's other seeds still a hair from
  it. `footClearSize` stays for that case and makes it at 0.0007 larger.
- **The fine chord.** At a chord setting of 0.001 (tolerance 0.00018) the slab past the rim was
  refused: the solid grew by 0.096 more than lies past the cylinder, and the allowance, twice the
  tolerance over the side's area, was 0.055. The rim's mesh stands inside the cylinder by its
  chords' sag, 0.0024 for a quarter of radius 8 in 32 chords, and the slab fills that. The
  allowance now takes the larger of the tolerance and the side's edges' own sag
  (`curvedSide.sag`); holes 2 and 0.5 from the rim, filleted and chamfered 2.5 and 3.5, are made
  (`TestFilletPastConeSideFineChord`). The family of 64 at the default stays 62.

Putting the solid back past the side below the foot (tried 2026-11-19, not kept): the solid
before the fillet intersected with the cutter's ring from below the solid to halfway up the side
(to the side's foot itself, the ring's inner face touched the side along the foot's circle and
the intersection was the whole ledge), then united with the filleted widened solid before the
cut back. Four changes in `pkg/srf` made it sound, at 570.64 against 570.598 + 0.041
(`doc/dev-notes/k3-fuse-spike.md`, "The ring below a ledge's foot"); the last of them, the arc
where a plane square to a cone's axis meets it taken exactly, moved `TestSolidGoldens` taken for
every such cone, and taken only for a cone with a marched edge in the plane left the two faces
of the ledge's top meeting along the chamfer's arc with different points. A cylinder's ring
united with the solid after the cut back failed where its inner face met the side's along the
foot's circle.

Left: a hole's rim whose cut into a ledge reaches past the side, or touches the side's foot
exactly where no size near crosses it; a solid standing in the cutter beside the side below the face; `ROADMAP.md`, "Fillet leftovers".

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
would meet across it (no test reached it: the plate's foot on the cone runs past the flat all along
the edge first; made since 2026-11-23, below), a fillet taking material away at either, and both
faces at both ends (made since 2026-11-23). Measured by `TestBlendRunsOffBothFaces` against the blend that runs off neither (the
top high, the cone uncut) less what of it lies above the low top's plane and past the flat,
measured by vertical lines through its mesh above the plane and above the cone's exact surface:
12 of 12 within 0.15%, 0.08% but for the fillet of 5.5. The roadmap's other way, the setback
patch, was not needed.

Since 2026-11-18 one end running off both its faces and the other end one face is made: it was
refused by name only, as nothing had reached it. Each end is cut by its own next faces, as one
alone and as both at one end are; the only change was `runOffApart`, which split the fillet
between the two ends' cuts at the end that runs off both by its first `p2` only, and now takes
the further of its two. `TestBlendRunsOffBothFacesAndOne`: the plate on the cone with its top two
planes, low at both ends and 10 high at the middle (`peakedConePlate`), so that the end at the
cone's wide end runs off the top and the flat and the narrow end the top only; chamfers of 2.15
and fillets of 3.5 and 4, the narrow end's top 0.2 to 1 above the foot: 9 of 9 sound, within
0.02% (0.007% the chamfers, 0.005% the fillets of 3.5) of the blend of the plate high at both
ends on the cone uncut less what of it lies above the two planes or past the flat (one column
counted once, above the lower of the two). A chamfer of 2.25 or a fillet of 4.5 is refused as
"runs off faces at both its ends": the narrow end's cut reaches back along the edge past where
the wide end's begins (`TestBlendRunsOffBothFacesAndOneOverlapping`); a flat at 2.5 lets the
foot on the cone run past it all along. Still refused: both ends running off both faces (no
fixture: the cone narrows, so the flat is reached at the wide end only), and a face taken whole
with an end running off both (`consumedRunOff` now refuses it by name, where `consumedLoops`
would have cut that end by one next face). The two cuts crossing on the fillet is still reached
by no test: every size tried that reaches past the flat at both sides' cuts runs past it all
along first.

Since 2026-11-23 the two cuts crossing on the fillet, and both ends running off both faces, are
made; both needed a fixture first. `slantConePlate` turns the flat to a slant, y = c0 + s·x, so
that it comes in close beside the plate at the cone's wide end (c0 1.6 to 2.4) and leaves the foot
room further along (s 0.1 or 0.2): the end section there crosses the flat above where it crosses
the low top, and of 120 sizes tried, 40 were refused as "the faces past them meet across its
end". `rollingrunoff_crossed.go` makes them (`crossedRunOffs`, from `bothRunOffs`): the top and
the flat, both flat, meet across the fillet along the line their planes share, from x, where
their cuts on the fillet cross (`crossedX`, the one end their curves across share, found as
`consumedX` finds the ridge's x), to m, where that line meets the end face's plane, past both
w on the end face's edges with the next faces. The patches past x are cut away whole
(`clipRunOffs` keeps them as empty loops), the patch holding x keeps the corner the two cuts make;
the end face takes no end section, only the lines from each w to m; each next face takes its cut
from p2 to x, the line from x to m and the line from m to its w (`closeRunOffCrossed`). 36 of the
40 came out sound at once; the other four, chamfers of 4 with s 0.2, are open at the plate's
other end, where the flat leaves the cone at x = 10 just where the plate ends, not at the
crossing. `TestBlendRunsOffBothFacesCutsCrossing`, 16 of them, within 0.06% of the uncut,
high-topped plate's blend less what lies above the top or past the slanted flat
(`crossedOracle`), as far as the blends that don't cross.

Both ends running off both faces needed the cone to end with the plate at x = 10, so that the end
face there is one plane across both (`shortConePlate`; on the long cone the end at x = 10 ends on
a face going on, and a foot past the flat there comes out open, refused only by the closed check),
and two flats meeting at x = 5 that come in at both ends. Each end was then refused by name
only: with `bothEndsRunOffBoth` taken out, each end is cut by its own next faces as one end
running off both is, and 54 of 60 sizes came out sound, the other six refused by name (the two
ends' cuts overlapping, `runOffApart`, or the face taken whole, `consumedRunOff`).
`TestBlendRunsOffBothFacesAtBothEnds`, 12 of them, within 0.05% of the reference blend less what
lies above the top's planes or past the flats (`bothEndsOracle`). That reference is on the cone
cut by a flat that takes nothing: on the cone uncut the blend's own meshed volume moves by up to
0.16% (the cone's faces and chords fall otherwise), which put one fillet 0.12% off.

Still refused: a crossing whose next faces or end face are curved (no fixture), and an end
running off both faces taking material away at one of them; no fixture reaches it either: a
convex edge's foot on its second face would have to run off that face past its far corner at the
end only.

The ring band's 0.3% to 0.42% (`TestBlendRunsOffPastRingTop`) was the measure, not the kernel nor
the oracle. Vertical lines through the meshes, each class of column apart (the blend's region
above the exact cone, the band, under the cone), showed the blends as made matching the oracle
within 0.002% in the blend's region, the cone's mesh under them changing alike with and without a
run-off, and the whole excess in a strip a few thousandths wide at the band's outer side: the side's
chords lie inside its exact cylinder, so the band's mesh lacks that sliver, and the blend fills it
wherever it takes the side away, up to the top where the end runs off (0.09 mm³ of a chamfer of 2.5
at the top of 22). The test now measures both blends over the blend's region above the exact cone
(`ringAdded`): 53 of 53 within 0.04%.
