# Rolling-ball corners: pivots and mitres at an inside corner

Refreshed 2026-11-19 at branches `pocket-curved-crease`, `fillet-end-leaning-wall`, `pocket-crease-lean-back`, `rim-fine-tol-corner`, `apex-seam-chamfer`, `pivot-shared-seam`, `step-seam-corners`, `seam-near-point` and `dish-chamfer-wall-foot`; 2026-11-20 at `step-chamfer-one-face` and `step-dish-rims`; 2026-11-23 at `fillet-foot-at-edge`.
Refreshed 2026-11-23 at `wrapped-cutback-speed`: a chain's sections at once.
Refreshed 2026-11-24 at `chamfer-section-spacing`: a chamfer's sections by how its edge bends.
Refreshed 2026-11-24 at `fillet-foot-below-edge`: a foot below the floor all along, the wall taken whole.

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

## A curved crease (2026-11-19)

Refused by name until 2026-11-19 ("can't yet turn an inside corner whose crease is curved"),
unless the two feet were one point and the corner a mitre. The fixtures are `curvedPocket` in
`pkg/topo/fillet_pivot_curved_test.go`: a pocket in a block's flat top or in the dish, its x+
wall a cylinder lying on its side (axis along y, leaning 23.6° at the block's top), and with
curvedY its y+ wall too (axis along x). The crease where the x+ wall meets a flat y wall is an
arc in that wall's plane; where it meets the leaning y+ wall, the two cylinders' curve. In the
flat top a leaning wall and a flat one put the feet at two depths, so the corner is a patch
even there.

The swing about a line placed the ball's centre a radius from `v + e·h`, the crease's tangent
at the vertex; a curved crease leaves that line by its sag, x²/2R, 0.01 mm at a depth of 1 on a
radius of 50, ten times `rb.tol`. Now (`pkg/topo/rollingpivot_crease.go`) the depth h is still
measured along e, but the crease's point at h is the crease's own (`pivotCrease.point`: Newton's
steps onto both walls and the plane square to e at once), and the frame the swing turns in is
u0 turned square to the crease's tangent there (`frameAt`), the turn theta measured in the
frame at b's foot. The corner's foot on the crease moves evenly in depth along the crease, and
`dropCrease` finds the crease's piece between the feet by depth (`pivotCrease.on`). A straight
crease takes none of this: its corners are the same to the bit.

Three things more were needed:

- A chamfer's frame turns from wall a's normal to wall b's, into the solid, as a fillet's turns
  between its balls' centres. Its own sections' parts square to the crease, which the straight
  swing turns between, turned 167° the short way, through the pocket, where two leaning walls
  meet in the dish: the corner's foot on the cone ran into the pocket and the mesh crossed
  itself. Its foot on the shared face comes from its depth, angle, distance from the crease and
  offset along it, each moved evenly from a's foot to b's (`curvedChamferAt`), so that at either
  end it is that chamfer's foot.
- The corner's parameter along the swing is the share of the swing's path's length, sampled at
  65 turns. Taken as straight between the samples (the straight crease's `at`), the turn's rate
  jumps at each; where the swing's speed changes along it, as a chamfer's does whose feet lie at
  two distances from the crease, the sections came out a few tenths of a micrometre off smooth
  at the samples, and the corner wasn't fitted to 0.001 mm through 64 of them (0.0012 left,
  sound only at 256). Through a cubic between the samples (`phiAt`) it fits through 8. Taken
  evenly in the turn instead, the chamfer fitted, but the fillet's corner where two leaning walls
  meet in the dish crossed the shallower fillet's mesh near the crease.
- A chamfer's path is measured on its point before it is put on the face (`pivotSwing.c`): the
  foot on the face can stand nearly still as the turn starts.

Against OpenCASCADE (`scripts/fillet-pivot-curved-fixtures.py`, its makeFillet and makeChamfer of
the same edges; `TestFilletPivotCurvedCrease`, 22 lines, 23 since the crease leaning back below): where both walls lean alike in the
flat top the feet are one point and the corner a mitre, within 1e-5 of its volume and its faces
within 0.00001 mm of ours. A corner patch takes less, as at a straight crease: in the block
about 0.026 r³ a fillet's corner and 0.064 d³ a chamfer's, measured against each edge alone,
whose volumes agree to 3e-5. Our STEP read by FreeCADCmd, sampled within 3 radii of each corner:
a fillet's faces lie within 0.0005 mm of ours, ours up to 0.08 r of its (0.11 r in the dish);
a chamfer's within 0.08 mm of ours, ours up to 0.18 to 0.28 d of its: it runs each fillet on into the other past the crease, where ours
fills between their ends. In the dish ours take 1.2% to 3.1% less, but there OpenCASCADE's
Volume itself is off by a few tenths of a mm³ (a y+ fillet alone with both walls leaning measured a
fifth more than ours, where both fillets' faces lie on the ball's envelope and ours is within 0.1%
of its integral: "The fifth on the dish's y+ edge" below).

Of a grid of 128 (both tops, one or two walls leaning, the corners x+/y+, x+/y- and x-/y+ and
the whole rim, radii 0.5 to 2, fillets and chamfers, at 0.01 mm), 110 were made sound by the
first round, 117 since the crease leaning back below. The 11 refused are chains ending against a
leaning wall not picked ("the fillet's surface can't be fitted", the y+ chain's chamfer at the
x+ wall, made since `fillet-end-leaning-wall`, "Chains ending against a leaning wall" below) and a
chamfer of 2 crossing the dish's seam at x = 0; none at a curved crease's corner.

### A crease leaning back (2026-11-19)

Refused as not closed until then: the dish with both walls leaning, filleted, the whole rim and
the pair x- and y+ alone, at radii 0.5 to 1.5 (and the whole rim at 2), the mesh crossing itself
at the x- wall's corner both times, not at the y+ chain's end against the x+ wall. The crease
there, an arc in the x- wall's plane, leans back under the y+ fillet, whose foot on it lies nearer
the vertex: going down the crease from that foot goes 26° into the fillet's side of its end
section's plane (at the dish's other two corners 15° and 36°; in the block's flat top none).

Why the corner crossed, and why turning its sections can't mend it while it stays tangent to the
fillet: at the nearer foot the corner, the fillet and the wall are tangent to the wall's plane.
In that plane the fillet's end section leaves the foot up the wall, and the crease down it,
leaning 26° past the line of the section into the fillet's side. The fillet holds the 90° from
its foot on the wall to its end section, the wall the 64° from that foot to the crease, so the
corner must take the 206° from the section round the other way to the crease; a patch whose two
sides leave a corner along the section and along the crease spans the 154° between them through
the fillet and the wall instead. So with its foot moving down the crease from the start, the
corner lay over the wall's face below the fillet's foot and crossed the fillet's mesh near its
end at any chord tolerance (the corner of the two leaning walls, made at 0.01 mm, crossed at
0.002 mm too). The true ball shows the way out: swinging off the fillet its centre moves on
along the fillet's edge, tangent there to the fillet's path (both touch the shared face, and the
wall's normal at the foot is the radius to the crease), so it touches the crease above the
nearer foot at first, on crease already gone, and comes back down to it only later (at the x-
wall's corner, r 1, from depth 0.533 up to 0.405 at 30% of the swing from that end and back by
60%).

So where the crease leans back under the nearer fillet (`pivotFrame.leansBack`, its tangent
there into the side of the fillet's end section where the fillet's next section lies), the
corner's foot on the crease stands at the nearer foot for the share of the swing's path over
which the ball touches the crease above it (`holdShare`, 0.18 to 0.60 on the dish, at most
0.75), then moves on to the farther foot as the square of the rest of the way, so that its pace
is smooth where it starts (`creaseDepth`). While it stands the sections, each the arc through
the foot and the ball's foot on the shared face, turn about the foot as the ball does, off the
wall; the corner there has its side on the crease all one point, a pole. It is skinned in two
(`pivotSpan`, `skinSpan`), each part through its own sections and taking its own derivatives at
its ends: the depth is smooth at the split, so the two join with the same derivative there. The
crease is not split at the corner's breaks that lie at the nearer foot (`splitCrease`), and the
pole sides are left out of the patches' loops (`put`), as `AddBSplineFace` does. The normals
across the corner's ends keep within 0.0005° of the fillets'. A chamfer and a crease that
doesn't lean back (all of the block's) take none of this and are the same to the bit; the dish's
corners that lean back take 0.02% to 0.16% more than before, toward OpenCASCADE's. Sound at
0.002 mm too (`TestFilletPivotCurvedCreaseLeansBack` at 0.01).

A straight crease leans back too (second fix of the day, branch `rim-fine-tol-corner`): the
dish's x-/y- corner, two flat walls meeting in a vertical line, where the top falls away from the
corner along the x- edge, so the x- fillet's end section leaves its foot on the crease 9° off
the wall's vertical, into the fillet's side. The whole rim at 0.002 mm, one wall or both
leaning, fillets of 1 and 2, and the pair x- and y- alone, were refused as not closed (sound at
0.005 and coarser; on 09970236, before the curved crease work, the pair crossed at 0.002 too, so
it is not new). The crossing edges ran from the x- fillet's foot on the crease, one on the
x- fillet's mesh, one on the x- wall's, through a triangle of the corner patch from a point of
the crease just below that foot to the end section: the same 189°-to-fill as above (171° from
the crease round to the end section through the fillet and wall), at 9° in place of 26°, so
the fold is a few micrometres and only a mesh at 0.002 mm showed it. `holdShare` there is 0.57
(r 1 and 2). Now `leansBack` and the hold are taken on a straight crease too (`newPivotCorner`;
`section` puts the foot between the two feet at the share `creaseDepth` gives): the whole rims
of the dish at r 1, 1.5 and 2, fillets and chamfers, one wall or both leaning, are sound at
0.002 and 0.005 mm (4 of 24 refused before, all fillets; the chamfer of 2 with both walls
leaning was refused at both by the apex's seam, made since: "Feet past a pole" below); the
block's 24 were sound before and are the same. Of the 23 OpenCASCADE lines of
`TestFilletPivotCurvedCrease` only the dish's two whole rims filleted at r 1 (the x+ wall leaning, and both)
move, by 0.006% and 0.008%, toward its; the rest are the same to the printed digit.
`TestFilletPivotStraightCreaseLeansBack` (the pair at 0.002 mm, about 50 s of processor, skipped
under -short).

Against OpenCASCADE (its makeFillet, `scripts/fillet-pivot-curved-fixtures.py`): the whole rim
at r 0.5, 1 and 2 took 1.180408, 4.588486 and 17.869764, ours 3.5%, 2.1% and 4.0% less, as the
dish's other whole rims take 1.8% to 2.5% less (`TestFilletPivotCurvedCrease`, the line at r 1).
Its faces lie within 0.0004 mm of ours near the corners at r 1, ours up to 0.11 r of its, as
before; at r 2, 0.11 r to 0.12 r both ways, at the x+/y- corner, the same as before this change.
The pair x- and y+: 2.077566 at r 1 and 4.111163 at r 1.5, ours 19.5% and 11.0% less, but there
the edges alone already differ by as much (r 1: y+ alone 0.986673 against 1.233103, x- alone
0.697145 against 0.790484), which is OpenCASCADE's Volume, not the ends ("The fifth on the dish's
y+ edge" below).

Tried first: each section the conic through the two feet, tangent to the shared face and at the
crease square to the ball's radius way(phi). All seven made sound at 0.01 mm, but the normals
across the corner's ends turned 30° to 69° from the fillets' (the sections' shape changes at once
off the arc, so the corner moves across itself), and the whole rim crossed at 0.002 mm. Turned
only toward the nearer end, by (1 - s)² of the way, it crossed at 0.01 mm. The foot held at the
nearer depth as t² without standing (the first round's try) still crossed here. The foot
standing exactly needs the pole: the crease's split at a break of no length was refused ("the
fillets' corner doesn't follow the crease") until `splitCrease` passed over them.

## A group's chains side by side (2026-11-18)

Two chains meeting in a mitre share it, its last section: blended in order, the first to reach it
fitted it (`curvedEndSection`) and the second copied it (`fromMate`), so the chains of a group
were made one after another, the groups side by side (`blendGroups`). Now every mitre between two
chains is fitted first, by the chain that comes first in the group, as the kinked ones already
were, and then the chains are blended at once (`blendChainsAtOnce`,
`pkg/topo/rollingchains_atonce.go`). What a chain's blend reads of another is fixed by then: the
mitres, the radii, and the centres its ball runs through near its ends (`centresNear`, a cache
filled on first use, so it is filled for every mitred end before any chain reads another's; a
chain's `section` is never called on another chain while they blend; since 2026-11-23 it changes
nothing of the chain, below). A chain that fits its mitre uses it in its blend (`blendOpen`'s case for an
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
Since then (branch `wrapped-put-speed`) those run on all cores, the same to the bit
(`doc/dev-notes/performance.md`): the whole 5 to 6 s, `putBlends` about 2 s of each fillet and
the check with the look for turned-back creases about 2.3 s, `BenchmarkFilletWrappedArch` 1.75 s.

A chain's eased ramp (`easeRamp`, kept where the eased blend works) carries over to a later
blend of the same chain, which starts eased: this is meant, and said so where the field is
declared. The paces that made the skin run back come from the chain's ends and are the same in
every blend of it, and an eased ramp is sound wherever an even one is. Only a chain with no
mitre is eased, so no chain of a mitred group carries it; no test reaches a later blend of an
eased chain.

## A chain's sections at once (2026-11-23)

After the chains of a group were blended side by side, the critical path of a group was the first
chain fitting both its mitres in turn, then the slowest blend, and each of those a few searches
one after another, each of a hundred or more sections one after another: a mitre's feet
(`mitreFoot`), its middle and its sixteen arc lengths (`mitreRoot`, `mitreRoots`), the points a
fitted end passes through (`fitCut`), the blend's scan for seams and the seams between its
samples (`crossings`), its sections and its misfits round by round. While a group is blended its
shell's surfaces are projected on independently (`ProjectIndependently`) and the chains are
traced and mitred, so a section depends only on its arc length. Within `blendingAtOnce`
(`pkg/topo/rollingsection_kept.go`) a chain's searches that don't depend on each other run at
once (`rollingBall.each`, `eachWhile`, through `pkg/internal/par`), every result taken in the
order the loop took it; in order, outside, a loop that returned early still stops there
(`eachWhile`), since what it would not have projected moves where the next projection starts.
For that a section changes nothing of its chain: `robust` is an argument (`ballSectionBy`), not a
field set round the second try; `lastFoot`, which only orders `footNear`'s search, is atomic;
`uvLoops`, `mitreC` and `mitreS` are filled under locks.

What is found again is kept: every section found while blending, by its arc length's bits
(`keptSection`; about half the sections were found before, the searches of one end starting
from one bracket and both feet scanning one grid), cloned when handed out; a mitre's last
section by its end's plane (`endSection`, `keepsEndSection`: each blend of the chain found it
again, and the fit before them), and the arc lengths between its ends (`mitreInterior`), found
beside its feet. A pole snap (`poleSnaps`) is put on after the kept section; while a chain has
any, its end sections are found afresh. What is found ahead only fills those: the sections of a
golden-section step's two points and of a mitre foot's scan (`warmSections`), the points of a
mitre's fit to the cut (`warmMitreCut`) beside its end section, the end sections each chain
copies from its mate beside the fits. A blend's patches are put in together (`FaceAdder.addAll`:
prepared in order, the open edges each takes or leaves being the next's, each turned to face out
at once, since that projects only on the patch's own copy of its surface, and put in in order);
the curves they take are found by a filed place (`FaceAdder.findCurve`) and a face's surface by
one too (`Topology.surface`), where an edited shell's are out of their handles' order and
`FindSurface` looked through them all, a tenth of the barrel's chamfer.

The same to the bit: every result of the fillet and chamfer tests of `pkg/topo`, `pkg/sketch`
and `pkg/script/api` hashed before and after (surfaces with their closest-point caches, trims and
curves), the same set but for the three tests whose results differ from run to run before as
after; the barrel's saved file the same to the byte, with `TELESAPR_PARALLEL=off` too, and the
fixtures' hashes (`scripts/perf-fixture-hashes.sh`). `BenchmarkFilletWrappedArch` 1.50 s to
0.74 s on 32 threads, 1.90 s to 1.23 s on 4, 4.99 s to 4.02 s on one (28 100 sections found
where 59 100 were); `BenchmarkFilletWrappedArchAdd` 1.45 s to 0.70 s; the barrel with its
mouldings' tops chamfered regenerates in 4.8 s, 5.9 s before.

Left on one core: cutting the faces back (`cutBack`, 0.06 s of the arch's 0.75 s, a second of
the barrel's): its sections and the heights of the next faces where a foot dips (`dipsOn`,
`rbRunOff.h`) are projected on the shell's surfaces with each projection starting where the one
before ended (`SSurface.Cached`), so found at once or independently they would differ in their
last bits; and `FaceAdder`'s preparing and putting in, a few milliseconds. A mitre's searches end
in halvings, one section after another. What is left of a group's blend (the centres near the
mitres, 0.1 s, the fits, 0.1 s, the blends, 0.06 s on the arch) runs on all cores, the centres
as many sections as there are cores to find them.

## A chamfer's sections by how its edge bends (2026-11-24)

A blend's first sections were a twelfth of the edge or two radii apart, whichever is less, a
chamfer's radius being its distance: for the barrel's chamfers of 0.2 a section every 0.4 mm
whatever the edge did, 4816 of the solid's 5354 patches, and the misfit added almost nothing on
top. A fillet needs the radius in it, its arc turning with the ball; a chamfer is flat across, so
how far apart its sections must be is set by how the edge and its faces bend. `firstSections`
(`pkg/topo/rollingchamfer_spacing.go`) keeps the fillets' rule and for a chamfer spaces them by
a density along the dense edge (`chamferSpacing`): at each of its pieces, the length along which
the edge's tangent or either face's normal (kept from `trace`, `rollingBall.nrm`) turns by
`chamferTurn` (π/8), and no more than 2 `chamferTurnChange` (π/64) over how much their rate of
turn changes within a twelfth of the edge either side, the length kept between the old spacing
and a twelfth of the edge, so there are never more sections than before. The sections between
two crossings are spread evenly in that density. The second limit is for where the bending
changes at once, a sketch's line going on into an arc: with the first alone the pointed arch's
spans across its legs' ends were 2.6 mm, and the skin, smooth across its sections, rang 1e-3 mm
off the true chamfer along the straight legs and 4e-3 near the joins, under the fit's 5.5e-3 so
that the misfit added nothing; with it, 5e-4 along the legs (`TestWrappedArchTop`) and 1.6e-3 at
worst. Where the spacing comes out h all along (an edge shorter than twelve times h, or bending
fast everywhere) the sections are the fillets' to the bit: the trough's chamfer of 2.001, its foot
dipping under the floor (`TestFilletTroughPastFloor`), took 0.48% less than its integral, not
0.13%, with its sections moved by 1e-14 mm. The misfit between sections is checked as before, at three points along each span of an
open chain; a closed chain's at its middle.

`TestChamferWrappedBossDense` and `TestWrappedArchTop` sample every patch of the chamfer on a
16 by 16 grid over its whole square, edges and all, against the chamfer's closed form (the top a
cylinder, the side the radial lines over the wrapped sketch curve, the line square to the edge
from its point at the distance on one face to that on the other), and fail past the fit's
tolerance anywhere outside a millimetre of the mitres. Worst before and after: a wrapped disc's
closed chain 1.3e-6 and 2.0e-5 mm (64 patches to 32), half a disc's two mitred chains 2.7e-4 and
5.0e-4 (112 to 82), the arch's ten 4.4e-4 and 1.6e-3 (708 to 268); at a chord tolerance of
0.01% the disc's the same and half a disc's 2.7e-4 both (112 to 86). The barrel: 4816 chamfer
patches to 1264, its file 23.1 MB to 7.1 MB, STL 4.5 to 1.3 MB, STEP 4.9 to 2.4 MB, regeneration
5.3 s to 2.9 s; the group sound, its mesh volume 1780.701 mm³ and 1780.677.

## Chains ending against a leaning wall (2026-11-19)

Of the 128 lines of the curved crease's grid, most refused were not corners but chains that turn
at a corner of `curvedPocket` and end at their other end against a wall not picked that leans,
"the fillet's surface can't be fitted" (all chamfers in the dish). A lone edge with the same end
was made. A lone open chain is blended by `blend`, which, failing, fits the end sections near a
curved end to their cuts (`fitCuts`) and tries two spans; a chain that pivots at an inside corner
has a mate there and is blended as a group (`blendGroup`), which tried `fitCuts` only for a chain
mitred at one end (`partlyMitred`): a pivot end is no mitre. The group now tries it for any chain
with an end that meets no other chain (`endsAtFace`); its mitres' and pivots' sections are left
as they were, so nothing made before changes.

The block's x- edge chamfered alone was refused "a curved edge of a face at the fillet is part of
a longer curve": the y walls' edges with the leaning x+ wall are the profile's arc, drawn to
z = 25 and cut by the block's top at 20, and the planar chamfer (`edgeCurve`) took only a curve
that spans the edge. It now takes the piece between the edge's ends (`pieceBetween`), checked
against the edge's middle point.

The grid of ends (both tops, one or two walls leaning, the lone edges x+, y+, y-, x- and the
chains x-/y-, y+/x-/y- and x-/y-/x+, r 0.5, 1, 1.5, 2, fillets and chamfers: 224) made 199 sound
before, 218 then (222 by devel fbcf98b5, 224 since "Feet past a pole" below), each sound by `pkg/check`. Left then: the leaning crease's corner of the section above
(the whole y+/x-/y- chain in the dish with both walls leaning, filleted, four radii), and a chamfer
of 2 on the dish's y+ edge, alone or in a chain, whose foot on the top passes within 0.4 mm of the
cone's apex and crosses a seam that starts there, not at the edge ("crosses a seam of a face that
doesn't reach the edge").

**The fifth on the dish's y+ edge.** The lone y+ edge in the dish with both walls leaning,
filleted: OpenCASCADE's solids' volumes took 1.2331 at r 1, ours 0.9867. Neither end was wrong.
The ball's envelope integrated directly (`scripts/fillet-envelope-volume.go`: a point is taken
when in the solid, between the x- wall's plane and the x+ wall's cylinder carried on past the
pocket, and in the spandrel of the ball whose normal plane along the spine holds it; the spine
where the top's cone offset by r meets the y+ wall's cylinder grown by r) gives 0.256720,
0.986804, 2.130773 and 3.630505 at r 0.5, 1, 1.5 and 2, ours 0.257339, 0.986673, 2.129536 and 3.626902 (+0.24%, -0.01%, -0.06%,
-0.10%). OpenCASCADE's fillet faces, sampled, lie within 0.0005 mm of that envelope from the
x- wall (x = -12) to the x+ wall (x = 11.55): its fillet is right too, and the fifth is its
volume integration of the two solids of 75 000 mm³, off by 0.25 mm³ (a tessellation at 0.0005 mm
of the same two solids differs by 0.948; measured within a box about the rim, 1.316, and as the
Boolean cut between them, 0.898). The same holds for most dish lines with both walls leaning:
there OpenCASCADE's volumes, however measured, are off by up to a few tenths of a mm³, against
2e-5 in the block, and the chains' bands in `TestFilletEndLeaningWall` are wider.

## Feet past a pole (2026-11-19)

The chamfer of 2 on the dish's y+ edge: the edge crosses the top 1.56 from the cone's apex, and
the chamfer's foot on the top, 2 from it, passes 0.40 beyond the apex. The lathed top is four
patches whose seams all start at the apex; the edge runs through all four, the foot through the
two far ones only, crossing the seam between them, which runs from the apex outward and never
meets the edge. `cutBack` cut each crossed seam from the ball's point to the seam's end on the
chain, and found none. The other three seams run from the apex to the edge, inside the strip the
chamfer takes, and the two near patches' pieces round the apex lie wholly in it.

So a crossed seam that doesn't reach the edge is looked at for a pole (`seamPole`, in
`rollingball_pole.go`): a vertex of it every other edge of which is a seam between faces of the
same side that either runs to the chain or is crossed by the ball's point too. The seam is cut at
the crossing and its piece to the pole goes; so do the pole's seams to the chain, and a patch
round the pole with no foot on it loses that loop (the dish's near quarters keep the rest of
their faces on the far side of the pocket, and must still close, `trimsClose`), or, with nothing
left, is dropped. A single crossing within a speck of the pole (`poleSpeck`, below) is refused
by name ("passes through a pole of the face"): the seam's piece to the pole would be a speck.

On a ball the foot can go on to a patch the edge never touches (a pocket turned 45° about z, its
rim 1 or 0.5 from the pole in plan: the edge runs through three quarters, the foot through the
fourth too), and `footNear`, which looks only among the side's faces, lost the ball there ("the
ball the fillet rolls doesn't touch the face"). The patches collapsed at a pole within 4 radii of
the chain are now taken among the side's faces (`addPoleFaces`), and such a patch, with no trim
along the edge to say which way its loop runs, takes the way of a face of the side collapsed at
the same pole (`poleForwards`, read before any face is cut back).

Checked: the dish's y+ chamfer of 1.5, 1.75 and 2 within 0.25% of the chamfer's own integral
(`scripts/fillet-envelope-volume.go -chamfer`: in each normal plane of the edge, the line between
the top's and the wall's points d from the edge, as `chamferSection` puts them; 21.663, 28.870,
36.661 at -n 120 -dx 0.04, still moving by up to 0.06% between -n 60 and 120; ours 21.653, 28.804, 36.615), OpenCASCADE's
makeChamfer 21.673, 28.924, 36.625. In the y+/x-/y- chain the chamfer of 2 takes 100.045 to
OpenCASCADE's 101.576 (-1.5%); the x-/y- chain alone, which passes no apex, is -0.94%. The grid
of 224 ends made 222 on devel fbcf98b5 (the leaning crease's corner had been made by then), 224
now, and taking in the patches round a pole changed none of its volumes. The ball's pocket
(`spherePolePocket`, `TestFilletPastPole`), rims 1 and 0.5 from the pole, r 0.5 to 2.5, fillets
and chamfers: 17 of 20 made, at most 3 before, each within 5e-4 of OpenCASCADE's measured within
a box about the rim (`scripts/fillet-sphere-pole-fixtures.py`). The three left (the two fillets
whose ball's point passes through the pole, as OpenCASCADE refuses them, and the chamfer of 1 with
the rim 1 from the pole, whose foot passes 3.1e-4 short of it) are made since (below).

### A foot a hair from the pole (2026-11-19, `seam-near-point`)

A foot that passes a speck short of the pole crosses the two seams either side of the patch the
edge runs through a speck from the pole, and cut there they leave a sliver of that patch between
the foot and the pole: at a chord tolerance of 0.01 mm every case whose crossings lay 0.00085 mm
from the pole or less came out open, from 0.0014 mm all were sound. Past the pole by as little,
the single crossing's cut was refused by name within the chord tolerance; through it exactly, the
two crossings fall at one arc length (`crossings` takes them as one, between patches that meet
only at the pole) and were refused too.

So such crossings now go through the pole (`nearPoles`, `rollingball_polepass.go`): two crossings
of one side in a row, into a patch and out of it, both within `poleSpeck` of a vertex at which
that patch is collapsed, become one, from the patch before to the patch after, at the arc length
of the few sampled between them where the foot passes nearest; and a lone crossing between two
patches that share no seam, within it of such a vertex, is kept. The section there has its foot put
on the pole (`rollingBall.section` wraps `ballSection`, `snapFoot` moving the section's control
points by the shift, less towards its other foot). `cutBack` then cuts no seam for it: the pole's
seams that run to the chain go, with the patches between them other than the crossing's two
(`passPole`), as `seamPole` drops them; the patches round the pole beyond keep their seams.

`poleSpeck` is twice the fit's tolerance (0.002 mm at a chord tolerance of 0.01 mm). The surface
between the moved section and its neighbours strays from the ball by about a fifth of the shift,
so up to about four times the fit's tolerance it is still fitted; with the chord tolerance as the
speck, the cases whose crossings lay 0.0096 mm off were refused "can't be fitted" (0.0021 mm),
and the volumes of those moved by 0.003 mm and more stray by up to 6e-4. The single crossing's
refusal takes the same speck: cut from 0.0014 mm off past the pole, the seams close.

`TestFilletNearPole`: the rim's w0 moved from where the foot passes through the pole by −0.05 to
0.05 (finest 1e-5 about it), r 0.5, 1 and 1.5, fillets and chamfers, and the chamfer of 1 at w0
−1: 115 of 115 sound (53 before: 42 refused "passes through a pole", 6 through it exactly "crosses
a seam of a face that doesn't reach the edge", 14 not closed). Volumes against the section's
integral (`spherePoleTaken`: the rim is a circle about the axis through the ball's centre square to
the wall, so the section is the same in every half plane through it, swept between the pocket's
side walls), which agrees with OpenCASCADE within 1e-5 where it is sound (all of
`TestFilletPastPole`'s table): ours within 4e-4 where the section was moved, 1.2e-4 elsewhere.
OpenCASCADE refuses the fillets through the pole and the chamfer of 1 at w0 −0.999, and its
chamfers short of the pole by up to a tenth take up to 0.4% less than the integral (5.078 for the
chamfer of 1 at w0 −1, the integral 5.0962, ours 5.0965), where past it they agree within 1e-5.

From 2.25 the dish's y+ chamfer was refused as not closed: its foot on the leaning wall runs past
the wall's foot on the pocket's floor (the naked edges are along the wall's edge with the floor at
z = 4), a run-off, not the apex. See the next section.

## A foot past the wall's foot, along the chain (2026-11-19)

The dish's y+ rim crosses the cone 1.56 from the apex, so the wall below it stands only 1.56 tall
there (the floor at z = 4, the rim at 5.56 near x = 0), 2.2 measured along the leaning wall, and
some 5 near the ends. From 2.25 the chamfer's foot on the wall runs below the floor near the
middle and comes back before either end. The run-off machinery (`rollingrunoff*.go`) looks only at
an end: an end section past the far end of a side's edge with the end face (`runOffAt`). Nothing
looked at a foot leaving its face in the middle: the ball's feet are found on the wall's surface
carried on (`footNear`, `ext` 1 for an open chain), the fillet was made, and `cutBack` trimmed the
wall by a contact curve that crossed its edge with the floor, the floor untouched; the topology
validated and the mesh came out open.

Now `dipsOn` (`rollingrunoff_dip.go`) samples each side's contact curve (16 points a patch)
against the faces across the side's faces' edges, but the chain's, the ends' faces and the faces
an end runs off onto, each as an end's next face is held (`nextFace`: its plane, or its surface,
signed so that the chain lies on the side kept). A run of points past it that starts and ends
short of the chain's ends and crosses it on that edge both ways is a dip, from a to b. `cutDips`
clips the patches between by it (`clipOne`, every row: the y+ chamfer has two, its x+ end being
curved; the loops by row and patch, `rbBlend.rowLoops`), takes a and b from the cut's ends, and
splits the edge at both; the edge's piece between goes from the wall (`drop`), the contact pieces
are cut there (`dipPieces`), and `closeDips` puts the cut into the floor in that piece's place
(`runOffEdit.put`; a curved next face grown first). The wall is left two faces, pinched to nothing
between a and b.

A foot that only just reaches the edge, the face pinched to a point (no sample past it, the
highest within a tenth of the chord tolerance under it, on the edge), or runs past it by less
than that, is refused there, "only just reaches the edge of its face" (`footAtEdgeMsg`): left
whole the mesh is open, cut it is a speck. The chain y+/x-/y- at 3.25 is one: its foot on the
upright y- wall, 3.25 tall at x = 0, touches the floor there within the fit. Since 2026-11-23
`FilletPicksMade` takes that refusal and makes the fillet again from the solid before at the
sizes nearest the one asked for, a quarter, a half, three quarters and the whole of the chord
tolerance's worth either side (`footAtEdgeSize`: the size moved by no more than moves the
surface by the chord tolerance, (sqrt 2 - 1)|r - c| for a fillet, |r - c|/sqrt 2 for a chamfer,
as `footClearSize` measures it), and keeps the first that comes out closed; the group says the
size made (`Group.Note`). Either way is taken: past the edge by enough the dip is cut, short of
it by enough the face is left whole. The chain at 3.25 is made at 3.2465, short of the floor,
within 0.02% of `pastFloor` at that size; the trough's fillet at 2 at 2.006, past it, within 5e-5
of the integral; its chamfer at 1.9965, short, 0.13% under it as its neighbours. The cost is a
fillet per size tried: the chain's first try is made, 30 s at 0.01% with `pastFloor`'s chamfer.
A pick of a size of its own is not moved, and stays refused. Both a dip and a touch need the foot to
stand clear of the next face by more than that somewhere before and somewhere after: a contact
that starts on a face it meets at a corner the chain turns lies along it within the fit, 1e-6
past it, and was first taken for a dip that grazes it (`TestFilletWrappedArch`).

Timing: the look costs a projection per point on a curved next face, and a chain round a barrel
meets many faces; looked at only for an edge some point of the contact comes within a step of
(`comesNear`), and only for its points within twice the size of it, `BenchmarkFilletWrappedArch`
is 1.49 s to 1.51 s on 32 cores (2.35 s looking at every point). The dish's chamfers cost what
they did when refused, about 1.7 s each at 0.01%.

Checked: the dish's y+ chamfer 2.25 to 3.5 alone and in the y+/x-/y- chain, 12 of 12 sound, the chain at 3.25 at
3.2465 (`TestFilletEndLeaningWallsChamferPastFloor`, `TestFilletEndLeaningWallsChamferTouchesFloor`), each within 0.15% (0.02% to 0.11% under) of the
same chamfer of the pocket with its floor at 2.5, which no foot reaches, less what that takes
below z = 4 (`pastFloor`, `surfaceVolumeAbove`: the meshes' triangles carried out to their
surfaces, those cut by the plane taken as chords). The edge alone within 0.03% of the chamfer's
integral cut at the floor (`scripts/fillet-envelope-volume.go -chamfer -n 120 -dx 0.04 -floor 4`:
54.181806, 73.621150 and 94.331227 at 2.5, 3 and 3.5, ours 54.195187, 73.611944 and 94.358796;
uncut 54.242049 at 2.5 and 96.527108 at 3.5, 2.3% more). The dish's fillets never get there: the rim's edge is 155° open, the foot on the wall
0.22 r from it, and from 2.25 the ball is lost on the cone near its apex first. The trough
(`troughPocket`, `fillet_dip_trough_test.go`: a block whose top is a concave cylinder of radius 20,
its pocket's walls 2 tall where the top is lowest), whose fillet is a torus cut by the floor's
plane and the end walls' and integrates in closed form round the axis: fillets of 2.001 to 3
within 1e-4 of it (2.001 cuts a dip 0.001 deep), at 2 made at 2.006 (`footAtEdgeSize`); chamfers 0.08% to 0.12%
under it before any reaches the floor (0.5 to 1.9, the same at 0.01% and 0.002%: the integral cuts
the chamfer's triangles square to the edge by the end walls, where the chamfer's end sections are
its own) and on past it as smoothly, 0.13% to 0.16% under from 2.001 to 3.

Past 3.41 the trough's foot is below the floor all along, from end wall to end wall, which
an end's run-off refused by name ("runs off the face on side 2 all along"): it found no point
where the contact came back above the floor's plane. Both ends run off onto the floor, so the two
run-offs are one cut, as `consumedRunOff`'s two next faces meeting at a ridge are, with no ridge
(`wholeRunOff`, `rollingrunoff_whole.go`, 2026-11-24): every patch is cut by the floor's plane
once (`wholeLoops`), the cut runs from one end's p1 to the other's, the floor loses its edge with
the wall from w to w and takes the cut, put in once by the end at 0 (`closeNext`, `wholeLead`),
each end face takes its section to p1 and the line from p1 to w with the floor, and the wall goes
(`dropSurfaceOf`). Sound at 3.5, 4, 5 and 6.5, fillets 2e-5 to 1.2e-4 over the integral and
chamfers 0.13% to 0.17% under (`TestFilletTroughWallTakenWhole`); from 7 the foot on the top
reaches the block's side at y = 10 and the mesh is refused as open. The chamfer of 4 first failed
"can't follow where the next face's plane cuts it": a straight section crossed the plane exactly
on one of `clipLoop`'s 64 samples (h = 0 there), which neither interval took as a crossing; a
sample on the plane is now passed over. OpenCASCADE (`makeFillet` in FreeCADCmd) agrees with the
integral to 4e-5 up to 1.99, takes less from where the foot reaches the floor (0.07% at 2.1, 0.8%
at 2.3, 2.3% at 2.5, 7.8% at 3) and refuses from 3.5, its chamfers from 2.5: its ball leaves the
wall where the foot reaches the floor and rolls on the wall's edge with it, where ours rolls on
as if the wall went on below and the floor's plane cuts the fillet, as for a dip.

## A corner on a seam of the shared face (2026-11-19)

`seamPocket` (`pkg/topo/fillet_pivot_seam_test.go`): `dishPocket`'s dish, its quarters one sheet
as a lathed sketch's are, a square pocket 8 on a side whose corner at (16, off) points along +x,
turned by rot about it. The quarters' seam at y = 0 then runs from the rim, inside the fillets'
bands, out past the corner: one fillet's foot on the dish lies on one quarter, the other's on the
next. Each topology face is a quarter, so `pivotCorner.fd` (a's foot's) and `fdb` (b's) differ.

- **A corner patch** (feet at two depths, one radius; refused "crosses a seam of the face they
  share" before). `pivotFrame.seamCrossing` finds, on the patch's own side on the shared face
  (`sideCurves`), where the quarter's parameter across the seam reaches its side (`seamParam`),
  by bisection on the side's Bézier; the side and its patch's loop are split there
  (`splitAtSeam`, `seamLoop`), the seam cut there (`splitNearest`) and its piece back to the rim
  dropped from both quarters, each quarter given the side up to the seam from its own fillet's
  foot (`carryAcrossSeam`). The seam's end on the rim is the vertex of either chain nearest the
  corner on an edge between the two quarters (`Topology.seamBetween`). The point lies about
  9 µm off the cone, as the whole side does (the corner's side strays from the cone by up to
  9.4 µm, measured the same with the pocket turned off the seam: the sections are on it, the
  quintic between them is held to them only from u = 0.05 to 0.95).
- **Tried first**: the corner skinned in two at the seam (`skin` with a break), a section through
  the crossing, exactly on the cone. The break put a vertex on the crease; near the nearer foot
  the corner leaves the crease tangent to the owner wall, and its sections a little short of the
  near end leave the crease leaning past the wall's plane by about 1e-4 mm (at t = 0.9, 9° past
  it at 0.0007 mm from the crease), so the ear clipper's triangles from that vertex crossed the
  wall's: 7 of 90 corners of one radius refused as not closed, as a break forced at t from 0.8
  to 0.95 on the same corner turned off the seam was too. The single skin has the same lean
  (its normal turns over across u = 0.96..0.97 at the near end) and meshes clean.
- **A kinked mitre** (two radii or distances; "a loop doesn't close" before). The mitre's point
  lies on one quarter while one chain's edge, and so its side faces (`rollingBall.side`), lie
  wholly on the other: its foot crosses the seam a little before the mitre onto a quarter it is
  never looked for on, carried on along its own quarter's surface past the patch (which, a
  rational quadratic in the angle, is the same cone), and its band is cut into that quarter up to
  the mitre. `mitreSeams` (`rollingmitre_seam.go`), after the faces are cut back, walks that
  quarter's trims from the mitre's point back to the trim that crosses the seam, cuts it there,
  moves the trims on to the mitre to the next quarter, cuts the seam there and drops its piece
  back to the rim; the fillets' patch loops are split at the point (`splitAtSeams`). A seam
  leaving the mitre's vertex through the mitre's point (the corner on the seam, unturned) is
  `splitMitreSeams`'s. The seam's side of the patch is the one every point of the seam lies on
  (`seamParam`): taken at the seam's middle point first, it was the wrong one for the arch wrapped
  on a tube centred on its seam (`TestFilletWrappedArch`), whose seam is a line of two points
  ending at the tube's end, a corner of the patch where both parameters sit on a side, so the
  mitre's point on the seam was taken as past it and its foot refused as not cut at the seam.
- **Just short of the mitre** (2026-11-19, `seam-near-point`): a chamfer of 1 and 1.5 at
  (−0.3, 20°) and its mirror. The smaller chain's own foot crosses the seam 0.008 mm short of the
  mitre, within its ramp, and the section `rampCrossings` puts on the seam stands so near the
  mitre's (whose weights are not 1) that the skin's weights interpolate below zero between them,
  refused "just short of its end" (`crossingNearEnd`) before. Now, where the skin fails so, the
  ramp's crossings on the shared face within a hundredth of the span of an inside mitre are left
  without a section and the chain blended again (`rampSkipNear`, `rollingramp_mitre.go`): the foot
  crosses the seam within the end patch, whose side is cut back into the patch short of the seam
  up to the mitre's point past it, and `mitreSeam` carries it across as for the kinked mitre
  above, taking that patch (`mitreFootFace`) where the end section's own foot is already on the
  patch past the seam. Where the blend again fails, the refusal is the one before. Both within
  1e-5 and 4.3e-4 of OpenCASCADE's (13.135870, 13.135707); the mirror's 4.3e-4 is as its
  neighbours', made with their section on the seam: the mirror at 1.48 and 1.52 takes 3e-4 more
  than the same pocket turned 45° off the seam, where the first case itself takes 4e-5 less.
  `TestFilletPivotSeamNearEndSweep` (off −0.34 to −0.26 by 0.01, the smaller size 0.96 to 1.04,
  fillets and chamfers): 126 of 126 sound, 114 before (8 fillets, 4 chamfers refused "just short
  of its end").

`TestFilletPivotSeamSweep` (off −0.3 to 0.3 with 0, rot −20, 0, 20, sizes 0.5 to 1.5 alike or
unlike, fillets and chamfers, 0.01 mm): 210 of 210 sound since `seam-near-point` (208 at
`pivot-shared-seam`, 54 before: 110 refused by name, 44 "a loop doesn't close", 2 weights below
zero). Against OpenCASCADE
(`scripts/fillet-pivot-seam-fixtures.py`, FreeCADCmd; Part Fillet and Part Chamfer features for a
size per edge): across its own seam this near the corner its fillets came out inside out (a
compound of negative volume, one edge alone too), so its dish is turned to put its seam at −x.
`TestFilletPivotSeam`, 12 lines: fillets' corner patches 1% to 1.8% under its volumes, as the
dish's others are; chamfers' corner patches 0.05% over and 0.8% under; mitres within 0.05%. The
same pocket turned 45° about the axis, its corner well inside a quarter, takes within 0.12% of
ours (the fillets' fit sampled elsewhere). Our STEP read by FreeCADCmd: our faces within
0.0003 mm of its near a mitre, 0.04 to 0.14 mm near a corner patch (0.08 r a fillet's, as
before); the other way the comparison is spoiled by our files (below). Turned 30° instead, the
chamfers of 1 and 1.5 at (0.1, 0) were refused as not closed, away from any seam.

Timing: a corner across the seam costs what one off it does (`TestFilletPivotSeamQuick`, 0.64 s
at 0.05 mm against 0.34 s for `TestFilletPivotCornerQuick`'s, a pocket's two corners there);
the 210-line sweep runs in 36 s on 16 cores.

### STEP files of corners on a seam (2026-11-19)

`seamPocket`'s rounded corners, written as STEP, read back by FreeCADCmd as a shell of no solid,
and turned off the seam as two shells; the meshes were sound. Two faults of the export, neither
the fillet's nor the seam's:

- **Faces left out.** The corner's two walls were not in the file. The crease's line, which both
  fillets' feet end on, passes 4.7e-6 mm off its trims' ends (the vertex where the feet meet it),
  and a wall's loop of the pieces of its trims' exact curves was open by that much, past
  `LengthEps`: `ExportSurface` (now `surfaceJob`) writes no face for a loop that doesn't close (it says so since
  2026-11-20, through `Options.Warn`, branch `step-export-open-face`). A trim's piece of its
  exact curve now ends on the trim's own ends where they lie apart by up to the chord tolerance
  (`snapSectionEnds`, `pkg/srf/shell.go`): 4.7e-6 and 7e-6 mm are all that move in these files,
  and the goldens, whose points are written to 1e-5, don't.
- **A hole written as a face.** Turned 45° into one quarter of the cone, the pocket's outline is a
  hole in that quarter. `FindOuterFacesFrom` tells holes from outer loops by their outlines in
  the surface's parameters, and the quarter's two straight sides up to the apex have only their
  ends there: the apex, a side of the patch folded to a point, takes the parameters of one corner
  of the side, so the outline ran (0, 1) to (1, 0) on a diagonal, cutting off the half of the
  square the hole lies in. The hole was taken for a face of its own, written apart from the dish
  (a solid and an open shell, `TestFilletPivotSeamStep`'s "run the same way by both its faces" on
  reading). A point of the outline at a pole now stands for two along the folded side, at the
  parameters of the points before and after it (`uvOffPoles`, `pkg/srf/poles.go`, the sides
  found as `openPoles` finds them for the mesh). The plain pocket, unrounded, turned off the seam
  was written the same way before.

FreeCADCmd (`scripts/fillet-pivot-seam-fixtures.py` with `STEP=dir`, the files written by
`TestFilletPivotSeamStep` with `TELESAPR_STEP_OUT=dir`): the corner on the seam and turned off
it, fillets of 1 and chamfers of 1 and 1.5, one surface per face and one per patch, each one valid
solid of our faces; sampled both ways within 3 sizes of the corner, our faces and its lie within
0.00016 mm and 0.00012 mm of each other for the fillets of 1 on the seam, 0.0003 and 0.00016 mm
for the chamfers of 1 and 1.5 (the way from its faces to ours was spoiled by the missing walls
before). Its `Volume` of the files written one surface per face is off (74 764 for the
fillets on the seam against 74 950 one per patch): its integration over a lathe's B-spline of four
rational spans, C0 at the knots; its own tessellation of the same file gives 74 953, and a lathed
cylinder unpocketed 100 443 by `Volume` against 100 528 tessellated and π·40²·20 = 100 531.
Measure such files by their tessellation.

Our own reader closed all of them but the chamfers of 1 and 1.5 written one surface per face,
which it left open by a sliver 0.0016 mm long at the vertex where the chamfer's face narrows to
its surface's end in u (2026-11-20, branch `step-chamfer-one-face`). The note here blamed a piece
of the foot laid on a line of the cone's grid; it was the chamfer's own grid. That grid runs past
the bounds' extent by a margin, a thousandth of the extent, and a point of a bound within
`pointEps` of a cell's side, at most a thousandth of the cell, counts as on it: with one cell
along u the margin is the smaller, so the joint of the foot's two edges at the extreme counted as
on the grid's last line, the bound was broken there, and the cell's walk ran along that line from
the vertex to the cell's corner, an edge no other face has. The margin is now three thousandths
(`layout`, `pkg/persist/step/solid.go`); `TestFilletPivotSeamStep` reads all eight files back as
sound solids of our volume, and the corpus reads as before.

### STEP files of a pocket's whole rims (2026-11-20)

`dishPocket`'s rims x+ and y+ filleted whole, at 1 and 2, written one surface per patch and one
per face: FreeCADCmd reads all four as one valid solid, our reader closed none. Our files were
right; four faults of the reader (`TestFilletDishRimStep`):

- **A corner taken for a pole.** The y+ fillet's end on the crease is a patch whose side along
  the crease starts on two control points at the corner (12, 6, zc): its derivative along that
  side vanishes there, though the side runs on. `faceSurface.point` took any vanishing
  derivative for a pole, and the bound's end at the corner got the parameter of the sample
  before it, a fiftieth of a millimetre up the side; the cell's loop closed round its corner, a
  sliver of two trims 1.7e-4 mm long (the "points 1.7e-4 apart" seen first). A parameter is
  undefined only where the surface stays put as it moves a hundredth of its range one way or the
  other (`collapses`).
- **Two places for the corner in the kernel's parameters.** Read back, that patch made no
  triangles: `MakeEdgesInto`'s search for each trim's points, from the last point found, put the
  corner 6e-5 apart in v for the two trims through it, where a whole range of v is within 1e-9
  mm of it, and the outline didn't close. For a file's surface (`SnapToTrims`) a point the trims
  share now has the one place first found (`makeTrimEdgesInto`); the kernel's own surfaces
  search as before, since nine solid goldens moved in their last bits when they did too.
- **A fold of a sheet's side.** One surface per face, the x+ fillet's sheet ends in the corner
  patch, which folds the sheet's side u = 0 to the corner over its last span. The foot along that
  side ends where the fold begins, on the knot, and the next bound leaves from where it ends: the
  end took the parameter of the sample before it, short of the knot inside a cell, and the cells
  had no way along the fold. The end now goes to where the fold begins (`foldEnd`), and between
  two pieces that meet at one point at different parameters the bound runs along the fold, a
  piece of no curve, cut at the grid's lines (`withFolds`); where both ends lie on their cells'
  sides, as at a pole the grid stops at, the cells go along them as before.
- **A corner of two sides.** At 1, the x+ foot on the cone crosses the dish's seam at y = 0, on
  a knot line of the sheet; the foot lies along the sheet's side u = 1, which the grid's margin
  puts about a side's reach inside the last line. `perimeter` took the first side in reach, the
  right in the cell below the knot and the bottom in the cell above, and the one went round the
  corner between, the other not: three faces on one edge 2e-6 long. A side the point lies on to
  a hundredth of its reach now comes before one it is only near (the "three faces along the
  cone's seam").

## Left

- The corner patches of the strip are joined into the fillets' faces or kept apart (`Faces`)
  differently at different tolerances: 15 faces at 0.02%, 14 at 0.015%.
- Without one sheet for the lathe's quarters (`dishPocket` as the older tests build it), the
  seam is an edge of the solid: a pocket's corner exactly on it is refused "not an edge for a
  rolling ball" (its vertex has three other edges).
