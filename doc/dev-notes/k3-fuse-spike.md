# K3 spike: fuse first (2026-11-04)

The roadmap's rule for a kernel item: a spike on its hardest fixture before it is sized. This is
the spike for what was left of K3, "fuse first, then classify by identity" (BRL-CAD libnmg), run on
2026-11-04 from devel `3b4c15d6` on branch `rd7-k3-fuse-spike`.

## The fixture

Every contact sweep in `pkg/srf` passed before the spike, with no failures allowed, so none of
them was the hardest case. The spike built one instead: the regime fuse-first exists for. Two
solids touch, stand off by a hair, or sink in by a hair, at 17 offsets from 3e-7 to 3e-2 mm on each
side and at 0, turned off every axis (0° and 35°), at chord tolerances of 0.1 and 0.01 mm, by
union, difference and intersection: 816 Booleans. The fixtures are in
`pkg/srf/contact_near_test.go`:

| Fixture | Contact |
|---|---|
| `stack` | A block standing on another, partly over its edge: face on face |
| `side` | A block beside another: face on face, sideways |
| `flushtop` | A block inside another, its top at the other's top: coplanar faces, with walls touching |
| `cylinder` | A lathed cylinder standing on a block: a circle's disc on a face |

A result counts as right if it is made, its mesh is closed where it has volume, and its volume is
the exact one. Where the offset is within the chord tolerance, a volume off by up to the contact's
area times the offset is also right: that is the result of taking the offset for none.

## What the probe found on devel

150 of the 816 were wrong:

| Outcome | Count | What a user sees |
|---|---|---|
| Boolean failed | 114 | A problem on the group |
| Mesh not closed | 33 | A problem on the group (naked edges) |
| Volume off, mesh closed | 3 | Nothing: silently wrong |

Every wrong case was at an offset under the chord tolerance and above zero: from 3e-7 up to 1e-3
at the 0.1 mm tolerance, and in the band 1e-6 to 1e-4 a block's thin protrusion (`flushtop`)
failed every time. Exact contact (offset 0) was right in all 48 cases, as K3's earlier work made
it. At offsets of 3e-3 and more, nothing failed.

The probe also found a defect that has nothing to do with contact: a turned lathed cylinder and a
block 4 mm apart failed to unite at 35° and 90° (`TestDisjointTurnedCylinder`). The plane of the
block's top crosses the cylinder's surface where it extends past the patch, so the circle there is
an intersection of the two surfaces but on neither solid. `addExactIntersectionCurve` keeps a curve
if one of its points has its closest point within each patch's parameters, and the closest point
of a point off the patch's end came back within them. The cylinder's side then took the circle as
an edge along its own end, the two cancelled, and its trim did not close.

## What was built

These and the curved work below are on the branch and pass every test in the repository but one
golden that fails on devel without them (below).

1. **On-patch check** (`pkg/srf/surfinter.go`): a point of an exact intersection curve counts as
   within a patch only if the patch's point at its closest parameters is within twice the chord
   tolerance of it. `TestDisjointTurnedCylinder` fails without it at 35°.
2. **Fuse planes first** (`pkg/srf/boolean_snap.go`, `snapToPlanes`, called first in
   `MakeFromBoolean`): where a planar patch of the second operand is parallel to one of the
   first's, their boxes overlap, and it lies more than 1e-9 and less than a tenth of the chord
   tolerance off it, every point of the second operand within twice that of the first's plane
   (patch control points, curves' control and pwl points, trims' ends) moves onto the plane. The
   second operand is copied first; nothing moves if no plane is near. The first operand never
   moves, so the model before a feature keeps its geometry.

With both, the probe has 0 of 816 wrong (`TestNearContactSweep`, all 816 under `TELESAPR_SWEEP=1`,
120 sampled otherwise). Without the fuse it has 150 wrong. The fuse distance was chosen from the
probe:

| Snap within | Wrong of 528 (first probe, offsets to 1e-3) |
|---|---|
| none | 150 |
| chord / 100 | 18 |
| chord / 10, any point near a plane | 12 |
| chord / 10, matched planes only, points within 2 × that | 0 |
| chord | 0 |

A tenth of the chord tolerance keeps a gap a user drew if it is wider than a tenth of what the
mesh can show, and closes one the mesh could not show anyway. Moving single points near any plane
of the first operand, as the first prototype did, bent a plane of the second where only part of
it was near (a block partly over the edge): the second operand's planes are matched whole, and all
their points follow, the turned ones a little off by rounding.

**Golden movement:** none. The floor of 1e-9 keeps points that lie on a plane but for rounding
where they are, so every exactly coincident model regenerates to the same bits. `make test-fast`
and `make test` on the branch fail only at `TestSolidGoldens/shell-step`, which fails the same way
on devel `3b4c15d6` without the spike: `rd7-shell-inside-corner` wrote that golden before
`rd7-k3-flush-trims` moved the trims of flush cutters, and the two were merged in that order. It
needs a regeneration of its own on devel.

**Cost:** one pass over both operands' planar patches, pairs pruned by their boxes, and one over
the second's points if a plane matched. It is not measurable in `pkg/srf`'s tests.

## Curved contact

`TestCurvedContactSweep` (`TestCurvedContactProbe` until 2026-11-05, behind `TELESAPR_K3_PROBE=1`) takes what the planes leave, 108 Booleans each
(offsets from -1e-2 to 1e-2, two tolerances, two turns, three Booleans), counting those that fail
or leave the mesh open:

| Fixture | devel 2026-11-03 | With the on-patch check | With coaxial fuse and identity |
|---|---|---|---|
| `pin`: a lathed rod in a lathed tube of its bore | 84 | 77 (all 12 at offset 0) | 0 |
| `ball`: a ball resting on a plate | 14 | 12 | 12 |
| `lying`: a cylinder lying on a plate | 30 | 30 | 30 |

An extruded pin in a bore drilled through a block, the way a model is drawn, failed every Boolean
at offset 0 and at every offset under the chord tolerance; `TestPinInBoreSweep` holds it and the
lathed pair, 216 Booleans, none failing now.

Two causes, two changes, both on the branch:

1. **Classify by identity** (`classifyEdgeOn` in `pkg/srf/raycast.go`): the bore's side was
   classified at a point on the pin's seam, where two of the pin's patches meet. The edge's normal
   in the bore's surface is taken along a chord in the patch's parameters, and on a cylinder it
   dips off the surface by its curvature (a direction cosine of -0.017 with the pin's normal, past
   `DotProductTol`), so the pin's two faces read as both on one side, and the bore's side was kept
   inside the pin. Now, where the classified patch is known (`makeCopyTrimAgainst` passes it), the
   two faces at the edge are smooth across it, and the one that runs on into the classified patch
   lies on it (a probe a quarter of the edge into it, within a tenth of the chord tolerance), both
   sides are coincident, same or opposite by the normals. This is the "on-shared and on-anti by
   identity" of the roadmap, for curved faces, done at the one place the probe needed it.
2. **Fuse coaxial cylinders** (`snapToCylinders` in `pkg/srf/boolean_snap.go`, after
   `snapToPlanes`): a circular side of the second operand coaxial with one of the first's within a
   tenth of the chord tolerance, its radius off by more than 1e-9 and less than that, is scaled
   about the axis onto the first's radius, with every point of the operand about that axis from a
   little inside its radius to a little past its arcs' middle control points. The points move by
   the exact difference: `Equals` takes a move of 1e-6 for none.

Not done: tangent contact, and it is less than it looks. At exact touching, the 6 cases that "fail"
are unions with the right volume (the ball's and the plate's, to 1e-9) whose mesh `meshIsSound`
refuses: a ball touching a plate at a point, or a cylinder along a line, unite into a solid that is
not a manifold there, so the open mesh is the truth, not a defect. Sunk a hair (1e-6 to 1e-3, the
other 36), they meet in a disc or a strip of width 2·√(2rε): 0.08 mm wide for a ball of radius 3
sunk 1e-3, under the chord tolerance of 0.1 mm, so the mesh cannot show it. Fusing them to touch
would give the non-manifold union instead. Neither is the Boolean's to mend: the first is a
question of what to report (a problem on the group saying the parts touch at a point or a line),
the second the mesher's (K4).

## Sizing and plan

Done on the branch, mergeable: the planar fuse, the coaxial fuse, classification by identity at a
smooth edge, and the on-patch check (sizes S each, together about a day of this spike). What is left
of K3 is **S to M**:

1. Tangent contact (`ball`, `lying`): report a union that touches at a point or along a line as such,
   a problem on the group, instead of "naked edges". A sliver under the chord tolerance where they
   sink a hair is K4's.
2. Coaxial cones and spheres on spheres, by the same pattern as the cylinders, each with a probe
   like `TestPinInBoreSweep` before code.
3. Offsets past a tenth of the chord tolerance and under it (`TestPinInBoreSweep` leaves them out):
   two sides that close cross within their chords. That is a mesher's problem (K4: a mesh shared
   along the meeting curve), not the Boolean's.
4. Fuse the first operand onto the second's planes too, only if a case turns up where the feature
   is the exact one and the model before it is off by a hair.

Since the spike (merged in `a68c4c8b`):

- Item 1's report is built: Check Solid, and `Analyze().Check()`, say where a group's part and the
  solid before it touch only at a point or along a line (`touchAt`, `pkg/check/touch.go`;
  `Report.Touches`, `Report.TouchAt`; `TestCheckPartsTouching`).
- Item 2 is built: `snapToCones` and `snapToSpheres` (`pkg/srf/boolean_snap_revolved.go`), after
  `snapToCylinders`. A cone in a countersink of its angle is right in all 96 cases within the fuse
  (`TestCoaxialContactSweep`). A ball in a seat of its radius was not: 50 of 96 within the fuse
  left the mesh open at the seat's rim, where the seat's marched curve and the ball's did not
  meet.
- The ball in its seat is right in all 96 since 2026-11-04 (rd8; `TestCoaxialSeatSweep`, the probe
  made a test). Three changes, each needed:
  1. A plane meets a patch on a sphere in an exact circle (`intersectPlaneAndSphere`,
     `pkg/srf/surfinter_sphere.go`, tried before marching): the circle is cut where it crosses the
     patch's four sides and its face's outline (found by bisection along them; a face trimmed by
     a curve that isn't exact is left to the marcher) and kept where it lies on the face, in
     rational quadratic arcs of a quarter turn or less. The seat's rim and the ball's crossing of
     the block's top are then the same exact curve, and the second Boolean follows the first's
     points (`findExact`). Alone, this made it worse (96 of 108): the classification then saw
     the rim edge on both shells and misread it.
  2. Edge on edge (`classifyEdgeOn`): where one of the two faces at the other shell's edge is
     curved and lies on the classified surface, and the faces aren't smooth across it (the seat's
     sphere and the block's top), the side that runs onto that face is coincident and the other is
     classified against the other face. Taken as the tangent case, the edge's normal, along a
     chord, dips off a sphere by its curvature (a direction cosine of about chord/2r, against
     `DotProductTol` of 1e-5) and both sides read as outside.
  3. Edge on surface: the true rim lies a sag outside the seat's chorded trim, so
     `classifyOnCoincident` takes a curved face whose trim misses the point by less than the
     chord tolerance; and where the classified surface and the face it lies on are both curved, a
     side that runs onto the face within its trim is coincident although `ClassifyRegion`'s
     normals, dipped the same way, said inside or outside (both sides of a seat's rim at chord
     0.1).
  Changes 2 and 3 are taken for patches on spheres only (`isSpherePatch`): for every curved face
  they moved results that were right, threads, text wrapped on cylinders and cones, and a chamfer
  running off a curved end (`make test-fast` on the branch). The chord's dip is the same on any
  curved face, so the narrowing is a limit of these rules, not of the cause.
  No golden moved (`make test` and `make test-heavy` on the branch). A first version cut the
  circle only at the patch's sides and broke a hemisphere read from STEP cut by a block
  (`TestSolidBooleans`, `sphere_dome`): its patches are trimmed along a meridian inside them.
- Sunk a hair under the CDT (`TestCurvedContactProbe`, now `TestCurvedContactSweep`): ball 3 of 108 (12 under the ear clipper, 4
  on devel with the CDT before the changes above), cylinder lying on a plate 30, as before. Those
  are slivers under the chord tolerance, the mesher's.
- `shell-step` was regenerated on devel with the rest of the round.
- A shell's inside leaving a revolve's edge at an inside corner beside a cone, sphere or torus
  (rd8, `pkg/topo/shellrevolve_step.go`, `cornerOut`) does not lie on the face beside, so needs
  no coincident curved contact: it leaves the corner at an eighth of a turn from that face into
  the empty wedge, and touches the solid only along the corner's circle. The Boolean then keeps
  two edges there, each with one kept face: the solid's face with the face taken away, the
  inside's wall end with the part of the inside outside the solid. Both are exact circles, but
  their points can differ (a cove's torus edge takes one more near its seam), and the mesh was
  open along the corner. `joinOrphans` (`pkg/srf/boolean_orphan_join.go`, after
  `moveOrphanTrims`) joins two such curves, running between the same ends along one exact curve,
  into one between the two kept faces along the points of both.

## Resting on a plane (rd9, 2026-11-05)

What the probe still had wrong, measured again after the CDT's sag change: ball 3 of 108, cylinder
lying on a plate 30. Broken down, nearly all were failed Booleans, not slivers in a sound mesh:
sunk 1e-5 below the plate, its plane cuts the cylinder in two lines 0.0126 mm apart, along both
sides of the patch seam at its lowest line, and the strips of side and cap between them trim to
nothing that closes (`AssemblePolygon` fails on edges of 0.003 in u). The cure is the fuse's, for
the case the coaxial snaps leave: `snapTangent` (`pkg/srf/boolean_snap_tangent.go`) moves the
second operand whole, rigidly, along a plane's normal, where a cylinder's side parallel to that
plane, or a ball, is off touching it by more than `snapPlaneFloor` and less than a tenth of the
chord tolerance, either operand's round on the other's plane; every match must ask the same move.
After it the probe, now `TestCurvedContactSweep`, has every Boolean of the pin and the ball sound,
and of the cylinder all but the unions within the snap's reach: two solids touching along a line,
their volumes added, no manifold there, as the true union is (Check Solid's `Touches`). Standing a
hair off, within the reach, now unions to the same touching pair where it gave two apart: as a
block a hair off another's top stands on it.

The seat's coincident rules (changes 2 and 3 above) were not widened past spheres: the sweep has no
case left they would change, and on every curved face they moved threads and wrapped text.

The libnmg plan's other parts were not needed by any probe case: vertex-to-vertex fusing past
`fuseVertices`, splitting edges at vertices on them, and point-in-solid by several rays with explicit
grazing results (`raycastClass` already tries 16 directions).

## Touching tangent at a point (rd9, 2026-11-05, tried, not kept)

Two exact tubes of one radius whose axes cross square meet in two ellipses that cross at the two
points where the tubes touch tangent. Every Boolean of them fails, whatever the lengths past the
crossing; tubes of radii 1 and 0.999 are sound. The march along a branch reaches a point where
the normals' cross product, its direction, shrinks to nothing and turns over, and followed on it
goes back the way it came; the loop finder (`marchLoop`) took such a there-and-back for a loop,
its step growing near the point (the surfaces lie so close there that a long step projects
anywhere along them) until it jumped back to its start.

Tried, on a branch:
- `marchFrom` ended a curve where the sine between the normals would fall to nothing within the
  next step and the surfaces lay within a tenth of the chord tolerance with parallel normals there,
  every branch ending on the same point; a curve ending so takes one seed, so a seed left alone
  was still marched. `marchLoop` refused a loop through such a point, and steps projected more than
  60° off their way. The Boolean was marked, and its difference (two pieces meeting at the point)
  refused.
- Union and intersection came out sound for 32 of 32 pairs with the patches' seams away from the
  points (more than about a tenth of a radian), the intersection the Steinmetz solid's 16/3 r³
  within the chords' bound. A seam through a point still fails: the seeds there are both
  patches' corners.
- Not kept: the rule also ended curves on thread models (`TestOpenDamaged/thread/face_m8`,
  `screw_bsw`), at points on the rod where a ridge's surface comes tangent to it, and the
  regeneration hung (over 30 minutes; 10 s without the rule). Requiring the normals to part past
  the point at the rate they came together did not stop it.

Next: take the touch only between surfaces known to be quadrics (circles extruded or lathed),
or find the point by Newton on both surfaces with parallel normals rather than by extrapolating
the march; then look at the sweep's case, where a lead's flat end lies a grazing distance from the
point and a fitted tube's skin is off an exact one by more than the touch allows.

## Equal pipes crossing, exact (rd9, 2026-11-05)

On two cylinders of one radius r whose axes a and b cross at o, a point's distance from either axis
is r, so (p−o)·a = ±(p−o)·b: the intersection lies in the two planes through o with normals a ± b,
and on each it is the first cylinder's circle moved along a onto the plane, an affine image of a
circle, exact as rational quadratic arcs (the map moves control points and keeps weights). Both
ellipses pass through o ± r (a×b)/|a×b|, where the cylinders touch tangent. `intersectEqualCylinders`
(`pkg/srf/surfinter_tee.go`) takes that case before the marcher, for patches `IsExtrusion` of a
circle (`IsCircle`) of radii within `lengthEps` and axes within `lengthEps` of crossing: it cuts
each ellipse at the touching points, at the crossings of both patches' sides and their faces'
outlines (a side lies on its cylinder, and a cylinder meets the plane in the same ellipse, so the
plane's sign along the side finds them), and keeps the spans whose middle lies on both patches.
An operand's own exact edge passing through a touching point is split there in
`MakeCopySplitAgainst` (`touchesOn`): a tee's branch ending at the main pipe's axis has its end's
rim through both points, and without a vertex there its chords crossed the pipe's mesh beside them.

`TestBooleanEqualCylindersCrossing`: 48 pairs (crosses and tees, 90° to 45°, the circles' seams on
the touching points and off), every Boolean sound and within the chords' bound of the Steinmetz
16 r³ / (3 sin θ), but 3 of the 12 tee differences at 45°: the branch's end rim lies inside the pipe
but nearer it than the pipe's chords within a chord tolerance's reach of the touching point, and
the two meshes cross there (faces closer than the chord tolerance, the mesher's). Regeneration
checks no mesh, so `failCrossingTouch` meshes the result of any Boolean of equal crossing tubes and
fails it if the mesh crosses itself or opens: the group says the Boolean failed, as before the
change, instead of showing a solid that isn't sound. It costs one more triangulation of such a
Boolean. Threads are not
touched: the rule asks for two cylinders of one radius, which a thread's ridges and rod are not
(`TestOpenDamaged` and `pkg/sketch`'s thread tests keep their times).


## Between a tenth and the tolerance (rd10, 2026-11-05)

What the fuses left: faces off each other by more than a tenth of the chord tolerance and less
than it. `TestBetweenContactSweep` (`pkg/srf/contact_between_test.go`) takes the curved fixtures
of the sweeps above (a lathed pin in a tube, a pin in a drilled bore, a cone in a countersink, a
ball in a seat, a ball on a plate, a cylinder lying on one) at 0.15, 0.3, 0.6 and 0.9 of the
tolerance either way, two tolerances, two turns, three Booleans: 576. A probe over the planar
fixtures of `TestNearContactSweep` (stack, side, flushtop, a cylinder's end on a block) at the
same offsets had all 384 right, so planes are left as they are.

Before: 143 failed and 31 left the mesh open (pin 32, drilled 32, cone 53, seat 51, ball 6), all
coaxial pairs within 0.6 of the tolerance and balls sunk into the plate. Regeneration meshes no
group, so the open ones showed as solids.

The fuse can't simply reach further: a gap a user drew wider than a tenth of the tolerance was to
stay. So it reaches further only where the result as drawn is bad. `snapBetween`
(`pkg/srf/boolean_snap_between.go`) runs the coaxial and tangent fuses with their radius or gap
window widened to the chord tolerance (the axes still within the tenth); if that moves nothing,
nothing more is done and no mesh is made. If it moves something, the Boolean is made as drawn
first, and if it failed or its mesh has naked or self-intersecting edges, made again from the
fused operand (`retryFused`). Kept if sound, with `SShell.Fused` the largest move of a grid over
b's curved patches (control points move further: an arc's middle one by √2; a flat cap's
corners, outside its trim, likewise); `Group.noteFused` puts it on the group's note. Where neither
is sound and both operands are, the Boolean fails rather than show an open mesh.

After: 0 of 576 wrong. A pin over its bore is made to fill it, and its volume is the two's taken
as touching, off by at most the contact's area times the offset. A ball sunk 0.9 of the tolerance
into a plate moves up to touch it: the union then touches at a point, which meshes closed.

In the repository's tests (`make test-fast`, with a log of each trigger) the widened fuse moved
something in one Boolean: `TestThreadOnHoleAtRoot`, an M10 nut thread in a hole of 10, whose
thread's bore is 0.029 mm off the hole's at that model's tolerance. Its first Boolean as drawn
failed, and the group came out sound before too, by the thread's later tries; now that Boolean
is made, fused, and the group carries the note. No
golden moved.
