# Rolling-ball corners: pivots and mitres at an inside corner

Refreshed 2026-11-18 at branch `wrapped-band-speed`.

Where two rolling-ball chains end at one vertex with a crease between their walls that the ball
goes round (a pocket's rim in a curved face, the foot of a band raised on a tube at the band's
corners, `concaveAt`), `insideMitres` (`pkg/topo/rollingmitre_inside.go`) decides how they meet:

- their two feet on the crease one point within the chain's tolerance (`rb.tol`, a tenth of the
  chord tolerance in mm, at least `pivotSameFoot`): an inside mitre, the two swept surfaces cut
  where they cross;
- otherwise a corner patch swung round the crease (`pkg/topo/rollingpivot_corner.go`): sections
  from the crease to the shared face, each the arc through its two feet tangent to the face, the
  foot on the crease moving evenly from one fillet's to the other's.

So the same corner is a mitre at a coarse tolerance and a patch at a fine one. The band on a
tube is such a corner: one fillet touches the band's end (a plane square to the axis) at the
ball's height, 7.5, the other the band's side (a plane through the axis) at √(7.5² − 0.2²), 0.0027
lower: in `TestFilletWrappedStrip`'s model a patch from a chord tolerance of about 0.045% down
(`rb.tol` is 0.00117 mm at 0.02%).

## The corner patch's ends (2026-11-17)

The patch is skinned along the swing through its sections (`oneSpan`, a quintic; `manySpans`).
Across each end it takes the derivative of its own sections (`ownDerivatives`), not the
fillet's.

Until 2026-11-17 it took the fillet's (`inwardDerivative` scaled to the swing's path), its own
sections moved by Hermite terms to match, but for the crease's point, which runs along the
crease. The fillet runs on along its edge, every control point at about the ball's speed; the
corner turns about the crease, a point at the speed of its distance from it, the arc's middle
control point (on the crease below the vertex) not at all. A quintic held to the fillet's speeds
carried the points near the crease, whose whole swing at a radius of 0.2 is a few hundredths of
a millimetre, a fifth of the ball's path out along the fillet's edge and back:

- the patch folded, by about 0.002 mm (the tangent along the swing turned through 180° at three
  quarters of the way up the section; its normal vanished inside the patch), and a mesh finer
  than that crossed itself: the strip of `TestFilletWrappedStrip`, all four edges, was refused
  as open at chord tolerances of 0.015% and finer (0.02% meshed past the fold);
- its normals turned from the fillets' by up to 0.46° on the strip and 4.7° on the dish's pocket
  (`dishPocket`) near the crease.

With its own derivatives the normals lie within 1e-4° of the fillets' along both ends on both,
the strip is sound at 0.02%, 0.015%, 0.01% and 0.005%, and the dish's corners take 0.3% to 1.1%
less, nearer OpenCASCADE's (`TestFilletPivotCorner`: r 2, four edges, 32.34 → 31.99 mm³ against
its 31.71). Against OpenCASCADE's fillet of the strip built from exact cylinders, which rounds
the corner, our faces lie within 0.016 mm at the corners (0.011 before: the fold bulged towards
its round) and as close as before elsewhere.

## Unlike sections either side of a patch (2026-11-17)

A corner's sections are of the degree and spans of the higher of the two fillets'
(`sectionsLike`). A pointed arch's leg on a tube is fitted in quintics of two spans, its foot
across the tube in quadratics of one: the corner's end at the foot was in two pieces, the foot's
patch's side in one, and the two never shared their edge (`TestFilletWrappedArch` at 0.02% and
finer, open). `splitAtPivots` (`pkg/topo/rollingpivot.go`) splits the fillets' patches' sides at
the corners of the corner's patches on its ends, as `splitAtMitres` does at a mitre of two radii.
`splitAtPoints` takes a point only once and none whose nearest point on the curve wasn't found
(a NaN it used to split at).

## The order of the picks (2026-11-18)

Edges of a flat pocket's rim that meet at an inside corner go first to step 10, which checks the
picks' corners in turn and is refused at the first it can't make: at an end left against a wall
not picked, "runs back along it", or, where the two radii differ, "fillets of radius r and R meet
here". Only the first sent them on to the rolling ball (`filletPicksOther`); the second went to
the stages, one radius at a time, whose first stage ends at the corner and is refused there,
"runs back along it", and then the setbacks, refused the same, and that was the answer. Which
came first followed the picks' order, not the radii: x+ then y- of radii 1 and 2 were refused, y-
then x+ made, the same solid, and y+ 0.5, x+ 1, y- 2 picked in that order likewise. Where the
stages and the setbacks are both refused so, the rolling ball is now tried last
(`filletRunsBackLast`), which makes them (`TestFilletPocketRimRadii`, five lines with the picks
against the rim, on OpenCASCADE's volumes of the same solids picked the other way). Tried first:
the rolling ball straight after the stages, before the setbacks. A ledge's corner of two radii,
which the setbacks make in OpenCASCADE's 10 faces, came out of the rolling ball in 50 to 60
(`TestFilletRadiiPairOracle`), and where the ball refused, its reason hid a setback that works
(`TestFilletBlendCorner`). Chamfers never took that
road: chamfers of two distances meeting two at a time pass step 10's corner check.

## Chamfers of two distances in a curved face (2026-11-18)

Refused at rd13, "the fillet's two sides don't meet in a mitre here", from `insideMitreRoot`: it
looked for the band where the mate's swept surface lies above the point between two rises, as a
ball's does. A chamfer's mate is a plane, which stays above the point from where the search
starts, so the height only rises through zero; since 2026-11-16 (`planeBand`, made for a band's
top round a pointed arch's tip) the root is taken there, and the dish's chamfers of two distances
are made with it, as the same check turned off again shows. OpenCASCADE's makeChamfer makes them
too, each edge at its distance both ways (2026-11-18): its faces and ours within 0.0006 mm near
each corner and 0.003 mm along the rest of the rim, the most at an end against a wall not picked;
its volumes within 0.035% of ours at 0.01 mm (`TestChamferPivotRadii`, 12 lines, the mixed
chains among them).

The volumes are measured on the mesh carried out to its surfaces (`surfaceVolume` in
`pkg/topo/surface_volume_test.go`: each triangle takes the gap to its surface at its edges'
midpoints, which a quadratic gap vanishing at the corners integrates exactly) and taken from the
dish's closed form (`dishVolume`), not as the dish meshed before less the dish meshed after. That
difference swung by 1% to 8% of a fillet between 0.01 mm and 0.002 mm, a lone edge's too, and was
taken for a fault at the corners (2026-11-18): the cone's chords sag 0.1 mm³ over the whole dish
before and 0.66 mm³ after, once the cone is trimmed anew. Carried to the surfaces, at 0.002 mm
(`TestDishPocketFineVolumes`): a lone edge within 2e-6 of OpenCASCADE, the flat face's corner of
two radii 3e-5, the dish's 0.06% (its corner differs), and the corner patch 0.45% under, as at
0.01 mm (`TestFilletPivotCorner`, 0.7% to 1.7% under, about 0.03 mm³ a corner at a radius of 1
and 0.15 at 2), where OpenCASCADE runs each fillet on into the other past the crease.

## A group's chains side by side (2026-11-18)

Two chains meeting in a mitre share it, its last section: blended in order, the first to reach it
fitted it (`curvedEndSection`) and the second copied it (`fromMate`), so the chains of a group
were made one after another, the groups side by side (`blendGroups`). Now every mitre between two
chains is fitted first, by the chain that comes first in the group, as the kinked ones already
were, and then the chains are blended at once (`blendChainsAtOnce`,
`pkg/topo/rollingchains_atonce.go`). What a chain's blend reads of another is fixed by then: the
mitres, the radii, and the centres its ball runs through near its ends (`centresNear`, a cache
filled on first use, so it is filled for every mitred end before any chain reads another's; a
chain's `section` changes the chain, `robust` and `lastFoot`, and is never called on another chain
while they blend). A chain that fits its mitre uses it in its blend (`blendOpen`'s case for an
end's own `shared`) rather than fitting it again.

The result is the order's to the bit, and so is the error: the chains up to the first whose mitre
couldn't be fitted are blended, and the error is the first in order. A chain's blend changes the
chain for the next try at other spans (`blendGroup`): its ends' planes turned to face on, its
`target` and, kept where the eased blend worked, `easeRamp`; the chains after the first that failed
are put back as they were (`rbBlendState`), as if not reached. Checked on the 121 mitred fillets
of the tests of `pkg/topo`, `pkg/script/api` and `pkg/sketch`, each shell's surfaces and curves
hashed to the bit before and after: all the same, and the barrel's saved file the same to the byte,
with `TELESAPR_PARALLEL=off` too.

Measured on 32 threads, with other work on the machine: the barrel's chamfer of four bands' tops
(four groups of six chains) blended in 1.6 s, 3.6 s before; the fillet's whole mitred stage 4.2 s,
6.3 s before; the regeneration about 10 s, 12 s before. `BenchmarkFilletWrappedArch` (ten chains
in one group) 2.9 to 3.5 s, 5.0 to 5.3 s before; on one core 5.4 s against 5.3 s, the mitres'
`endSection` worked out once more. The four arches' feet on the same barrel (40 edges, four groups
of six chains, picked in a script after the chamfer is deleted; FilletSolid makes the fillet twice):
each fillet's blends 1.1 s, 3.1 s before, and the whole 13 s, 15 s before, most of what is left
`putBlends` (the topology built again, closest points, 3.5 s each) and `unsound`, on one core.

## Left

- The corner patches of the strip are joined into the fillets' faces or kept apart (`Faces`)
  differently at different tolerances: 15 faces at 0.02%, 14 at 0.015%.
