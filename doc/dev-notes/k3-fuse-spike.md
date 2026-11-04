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

`TestCurvedContactProbe` (`TELESAPR_K3_PROBE=1`) takes what the planes leave, 108 Booleans each
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
  (`TestCoaxialContactSweep`). A ball in a seat of its radius is not yet: 50 of 96 within the fuse
  still leave the mesh open at the seat's rim, where the seat's marched curve and the ball's do
  not meet (`TestCoaxialSeatProbe`, under `TELESAPR_K3_PROBE=1`).
- `shell-step` was regenerated on devel with the rest of the round.

The libnmg plan's other parts were not needed by any probe case: vertex-to-vertex fusing past
`fuseVertices`, splitting edges at vertices on them, and point-in-solid by several rays with explicit
grazing results (`raycastClass` already tries 16 directions).
