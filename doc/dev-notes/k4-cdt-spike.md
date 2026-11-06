# K4: the constrained Delaunay mesher

Refreshed 2026-11-03 at `c4e18d1a`; measured again 2026-11-04 (below). The spike's full results and its tables are in
`git show c4e18d1a:doc/dev-notes/k4-cdt-spike.md`.

## State

- **The CDT is the default** since 2026-11-04 (rd8, `MesherCDT` the zero value of `srf.Mesher`):
  every curved face is meshed by it, flat faces by the ear clipper (their trims' points alone,
  `SSurface.triangulate`). `pkg/srf/triangulate_cdt.go`, on the exact-predicate
  `pkg/internal/cdt` that STEP's tessellation also uses. `srf.Tolerance.WithMesher(MesherEarClip)`
  or `Sketch.Mesher` still asks for the ear clipper, for tests and comparisons; it is not saved,
  shown or scripted.
- The mender (`triangulate_mend.go`, candidates in `triangulate_check.go`) works both ways. A
  face the CDT meshed whose mesh crosses the shell's, or has an edge with no mate there, is meshed
  again by the ear clipper, in rounds until none does (`mendDelaunayCrossings`); then, as before,
  a face the ear clipper meshed badly is meshed again by the CDT.
- The branch `k4-cdt-spike` adds nothing `devel` lacks: its mesher came in as `b33ac6f7`.

## How the mesher works

- **Boundary.** The trim's uv points as they are, so a face shares its vertices with the face
  beyond. Consecutive points are merged only when they are one point on the surface.
- **Chart.** u and v scaled by the surface's mean speed along each, so the chart is in about
  millimetres. On a patch with one side folded to a pole the chart is polar about the pole.
- **Interior points.** Centres of a quadtree's leaves. A cell is halved across x or y where the
  surface sags from that side's chords by more than the chord tolerance, both ways where it twists
  or where the normal turns past the angular tolerance, and near trim segments until it is at most
  twice the shortest of them. No cell is finer than 1/(2 MaxSegments) of the patch.
- **Refinement.** Up to `cdtRefineRounds` (6) rounds of centroids or edge middles where the surface
  strays from a triangle by more than the chord tolerance.
- **Flat faces.** The outline alone. A boundary that runs back over itself goes to the ear clipper.

## Decisions

- **A mender, not the default.** Meshing every candidate by the CDT moved the goldens of threads,
  cone fillets, lofts and spheres. No stray threshold set the bore's bad faces apart from them. So
  the CDT takes a candidate only where its ear mesh is actually unsound.
- **Stray measured to the uv middle's point, not to the surface.** The bore's chords lie on the
  cone within the tolerance. What crosses the trim is the uv image bending away from the chord.
- **No global switch.** The mesher rides on the tolerance, like the chord and the angle.

## Tried on the spike

- No grading near the trim and no refinement: half the triangles, but drafts, fillet sweeps and the
  rolling ball broke.
- An isotropic quadtree: a cylinder was cut along its rulings, 8302 triangles where the ear clipper
  makes 144. Splitting x and y apart took it to 1424.
- A sag from the boundary's turn: at a corner between two curves the turn says nothing of either.
- No floor on the cells: `TestLoftTwist` at chord 0.001% took 282 s against 0.3 s.

## Traps

- Two curves' end points at a corner differ by about 1e-12 in uv and must be one point. But
  `Equals` in uv merged real points of a short trim on a large STEP face (NIST_MBE_PMI_11).
- `splitTrimDiagonals` must not run after the CDT. It splits trim edges the face beyond keeps whole.
- `AssemblePolygon` and the ear clipper consume what they are given. The mender builds the face's
  polygon again through `SSurface.triangulate` rather than reusing it.
- To measure with the CDT on every face, override `Tolerance.Mesher` in the code for the run.
  There is no environment variable, on purpose.

## Measured again, 2026-11-04

At `fc8b6ef7`, with `BenchmarkMeshers` (`pkg/persist/tlspr`): each of the 84 solid and thread
goldens (`testdata/solid`, `testdata/thread`) regenerated with the ear clipper and with the CDT on
every curved face, the active group's mesh checked by `check.Solid`. The machine was shared with
other work (load 20 to 150), so times are CPU seconds where given, and the ratios are what holds.

| | Ear clipper | CDT, chart by speed | CDT, chart by curvature |
|---|---|---|---|
| Triangles, 84 models | 157 454 | 315 528 (2.00) | 225 220 (1.43) |
| Per model, median ratio | | 2.81 | 1.48 |
| Regeneration time, ratio | | 1.85 | 1.64 |
| Curved triangles straying past the chord tolerance | 16.7% | 0.8% | 1.8% |
| ... past four times it | 0.84% | 0.14% | 0.20% |
| Models unsound | 0 | 0 | 0 |
| `pkg/topo` tests, CPU, CDT on every face | 2022 s | 6655 s and a 10-minute timeout | 2576 s (1.27) |
| `pkg/topo` tests failing | 0 | 4 and the timeout | 1 |

- **The ear clipper does not meet its tolerance.** A sixth of its curved triangles stray from the
  surface by more than the chord tolerance at their middle or an edge's middle: its grid is sized
  by the parameter lines' chords and the ring it clips along the trims has no inside points. So
  "the CDT costs N times the triangles" compares a mesh within the tolerance with one that is not.
- **Where the CDT's points went.** On the goldens, 106 544 trim points (the ear clipper's too),
  89 757 from the quadtree and 28 429 from refinement. Taking refinement out entirely left 1.66
  times the ear clipper's triangles; grading near the trim off, 1.88; cutting a sagging cell into
  as many parts as its sag needs instead of halving it, 1.95. None was the cost.
- **The cost was the chart.** Scaled by the surface's speed, the chart is square on the surface,
  and Delaunay triangles in it are about equilateral on the surface: a fillet or a cylinder, curved
  one way, was sampled as finely along its rulings as round it (`fillet-radii`: 132 triangles by
  the ear clipper, all within the tolerance; 2156 by the CDT). `curvatureScale` shortens the chart
  along the less bent parameter by the square root of the ratio of the bends, at least 0.1: the
  cells then sag alike both ways (`fillet-radii` 420, `sweep-corner` 2024 → 508). A least scale
  of 0.05 gave 1.30 but 4.3% of the curved triangles past the tolerance.
- **What fails with the CDT on every face**, beyond the goldens:
  - `TestFilletRibAllOverOracle` (`pkg/topo`): 1 to 4 self-intersecting edges on the rib rounded
    all over, every case. The unscaled chart timed out there instead. A real defect of the CDT on
    those fillets; not traced.
  - `TestIntersectionBranches` (`pkg/srf`, pinned triangle counts) and `TestNearContactSweep`
    (its volume within a tolerance of a reference itself meshed): both restate, as the list below
    says. With the unscaled chart also `TestChamferCornerOracle` (mesh corners counted against
    OpenCASCADE's), `TestFilletRibFootSweep` (one point 0.13 mm³ off), `TestFilletRibFootAfterTop`
    (one self-intersecting edge) and `TestFilletMixedCornerOracle` (0.05 mm³ off): all pass scaled.
  - The twisted swept bar (`TestSweepTwistAndUp`): within 0.05% either way.
- **Mergeable.** `curvatureScale` runs only when the tolerance asks for the CDT on every face;
  the mender keeps the chart it had, so the meshes of a default model are byte for byte what they
  were (all 84 goldens compared through `BenchmarkMeshers`).

## The rib and the pinned tests, 2026-11-04 (later)

At `7de6e79b`, the CDT forced on every face for the run only (the switch is in the code, not
committed; see Traps).

- **The rib rounded all over is no longer known to self-intersect.** Its short case (h 10, lean 2,
  r 1) meshes to 151 500 triangles with no self-intersecting edge by the kernel's own check
  (`MakeCertainEdgesInto`, `EdgeSelfInter`), made in 43 s. But `TestFilletRibAllOverOracle` itself
  did not finish in its 10 minutes (274 s of processor): the time goes after the mesh, in the
  test's checks, which walk every triangle for each of a few hundred points (`firstHeight`,
  `ballHeight`) and run `check.Solid` on it. Whether its 1 to 4 self-intersecting edges of the
  morning were the merges since (K3's fuse, the mender's open-edge rule) or are still there in the
  other cases is not known: the full test must be run with the CDT on every face, given the time,
  and its checks made to cost less than one walk of the mesh per point.
- **The two tests that pinned the ear clipper's mesh are restated.** `TestIntersectionBranches`
  pins the kernel's surfaces and curves, checks the mesh closed, and the volume against the exact
  solid's (the lens of the coaxial cylinders, the crossing ones' integral, half the tube) within
  the chord tolerance times the curved area. `TestNearContactSweep`'s lathed cylinder allows twice
  the share of b's volume its own mesh loses, which the CDT can lose again on a piece of b meshed
  apart. Both pass the whole grid with either mesher.

## The rib in full, 2026-11-04 (rd8)

- **The test's cost was its own.** `ballHeight` walked every triangle near a point for each of the
  few thousand balls it tries there, some 90 000 triangles under the CDT: 56 of the short case's
  63 s. The triangles a ball resting on the plate can touch are now sorted once per mesh into
  cubic cells of half the radius (`triGrid`), each looked at only where its box comes within the
  ball: the same answers, the short case 69 s → 19 s with the CDT.
- **The 1 to 4 self-intersecting edges were the plate's.** With the mesher set to the CDT, flat
  faces were meshed by it too, though `Mesher` is documented for curved surfaces. A plane's mesh
  is its trims' points alone; along the foot of a fillet tangent to the plate, the fitted trim's
  points lie a few micrometres either side of one line, and Delaunay joined three of them into a
  sliver whose chord passed a micrometre over the fillet's first row of triangles, crossing it
  (h 5, lean 0, r 0.5: three such edges, at the corners where the foot's fillet meets the
  uprights'). The fillet refused it as not closed in 9 of the 24 cases. Planes are now meshed by
  the ear clipper whatever the mesher (`SSurface.triangulate`; `TestCDTLeavesPlanes`), and by the
  CDT only when the mender asks, as before. The counts above are unchanged by it: a plane's mesh
  of its trim points has as many triangles either way.
- **Result.** `TestFilletRibAllOverOracleCDT` (the oracle test with the CDT on every curved face)
  passes all 24 cases: no self-intersecting or naked edge, the volumes and the balls' heights
  within the ear clipper's allowances. It is in `HEAVY`. Since the CDT became the default it is
  the main test of the rib (12 of the 24 rows unless `TELESAPR_SWEEP=1`, one under `-short` and the
  race detector), and the ear clipper's, then `TestFilletRibAllOverOracle`, is
  `TestFilletRibAllOverOracleEarClip` (4 rows, all under `TELESAPR_SWEEP=1`).
- **Cost under the CDT, beyond the mesh.** In a case of radius 0.5, a third of the time is the
  fillet's own check of its result (`topo.unsound`: `mendCrossings` and the edge search over
  some 120 000 triangles), which every fillet pays at regeneration once the CDT is the default.

## Measurements (spike, 2026-10-10, CDT on every face)

| | CDT | Ear clipper |
|---|---|---|
| Twisted swept bar, 45 degrees, volume error | 0.003% | 1.6% |
| Crossing cylinders at 0.1 mm, triangles | 1738 | 364 |
| `pkg/topo` tests | 260 s | 77 s |

The quadtree alone is 1.7 times the ear clipper's triangles; grading near the trim and refinement
are each about half of the rest. The times predate the test suite review; the ratio is what holds.

## Made the default, 2026-11-04 (rd8)

- **Cost.** `BenchmarkMeshers` over the 87 solid and thread goldens: 228 066 triangles against
  the ear clipper's 158 406 (1.44, median per model 1.41), regeneration 0.99 times the ear
  clipper's time (median per model 0.88), no model unsound either way. The largest: `loft-tangency` 116 → 1900 triangles
  (the volume moves in the fourth digit), `screw_bsw` 2686 → 10 116 and three times the time.
  `pkg/topo`'s light tests: 2797 s of processor against 2347 at the commit before (1.19), with
  other work on the machine.
- **The segment limit.** The quadtree never cuts a cell smaller than a 2 MaxSegments'th of the
  patch (`minCell`); refinement did not stop there. Where the limit holds the cells wider than the
  tolerance would have them, no triangle can meet it, and every round added points: a cylinder
  of radius 10 at 0.001 mm and 7 segments made 4816 triangles, more than with no limit (1836).
  A triangle is now refined only where it strays by more than the tolerance and by more than a
  cell of the least size sags there (`refinePoints`): 342. Twice the least cell, at twice its
  sag, was tried first and left seven fillet, chamfer, draft and shell tests of `pkg/topo` past
  their volume tolerances.
- **Wall thickness.** Along a cylinder the CDT's triangles beside a trim lean by up to a few
  tenths of a degree (its points inside the face aren't in line with the trim's), so at a cup's
  rim, inside meeting top at 90°, a ray square to a side triangle left through the top at once,
  and the thinnest wall was 0. A ray whose two surfaces don't face each other by more than a
  degree there measures the mesh's fold and is left out (`wallFold`, `pkg/sketch/wall.go`).
- **Tests restated** for a mesh that is no longer the same however a solid is moved or meshed
  again: the CDT's points inside a face fall by rounding, so copies of a pattern, a cylinder
  meshed again in a union, a fitted sweep against a rigid one, a section's cap, and a nut bored
  twice now agree within the chord tolerance over the curved area, not to a millionth
  (`TestPathPattern*`, `TestDisjointTurnedCylinder`, `TestSweepFittedCornerBesideArc`,
  `TestSectionThroughAxis`, `TestThreadCosmeticNut`, `TestSphereThroughFace`, the lathe shell's
  path pair). `TestMendCrossings` asks for the ear clipper, which the mender is for. The
  twisted bar (`TestSweepTwistAndUp`) holds 0.05% by default, 2% with the ear clipper.
- **Faces within the tolerance of each other.** The sides of a triangle lofted a third of a turn
  round come within the chord tolerance of each other near its corners, and their meshes, each
  within it of its own surface, crossed: 2 to 46 self-intersecting edges in 4 of the 5 triangle
  cases `TestLoftSweep` samples, where the ear clipper's rows, one per trim point, keep clear. The
  mender takes such faces back to the ear clipper (above): every case sound, no golden's mesh
  changed by it. Checking every CDT face for it costs a walk of the shell's mesh per
  triangulation; with each face also checked for being pierced by the others' edges, as the ear
  clipper's mender does, regeneration took 2.2 times the ear clipper's, and that check was left
  out: a crossing is found from the other face's side.
- **Pattern copies differ.** Copies of a post along a path have 110 or 111 triangles to the
  post's 108: rounding, not the copies' geometry. Not traced further.

## Fine tolerances: the budget (2026-11-04, rd8-cdt-speed)

The default's measurements (`BenchmarkMeshers`, the goldens' regeneration at 0.99 times) were
all at the goldens' chord tolerances. At fine ones the CDT made far more triangles than the ear
clipper, and `make ci` timed out at 30 minutes in four packages. Alone, without the race
detector, on a quiet machine:

| Test | base bf905a66 (ear clipper) | CDT default 20e8502d | with the budget |
|---|---|---|---|
| `TestBandEndOnCone` (0.00085 mm) | 1.6 s | 27 s | 3.7 s |
| `TestWrapRectangleOnCone` (1e-4 mm) | 7.3 s | 85 s | 17 s |
| `TestWrapRoundMostOfTheWay` (1e-4 mm) | 21 s | 142 s | 70 s |
| `TestShellBlockAndDraftedBoss` (0.005) | 1.1 s | 15 s | 15 s |
| `TestFilletRunsOffOntoCylinder` | 78 s | 487 s | 122 s |

A band on a quartered frustum at 0.00085 mm: 40 084 triangles against the ear clipper's 1394,
each quarter of the cone 5096 points inside from `quadPoints`. Two causes, both in how the CDT
measures sag, not in the triangulation:

- **Parametric middles.** `cellSag`'s diagonal term and `refinePoints` compare the surface at a
  cell's or triangle's middle in the chart with the middle of its corners. Between two rulings of
  a frustum the cell is flat but no parallelogram, and the two middles lie apart along it: it
  counts as sagging, though the triangles cover the surface there. Measured square to the
  triangles instead, the band's mesh falls to 6086 triangles, `TestWrapRoundMostOfTheWay` to
  27 s, the drafted boss to 8 s; but 31 golden cases move (display outlines, STL, solid and
  script goldens), so it is not in this branch. The patch is kept with the round's notes.
- **Square cells in an unrolled chart.** A cone to its apex is charted polar, unrolled, and the
  quadtree's cells are about square there; a straight line of the unrolled cone is a geodesic
  that sags off its chord, so at 1e-4 mm the cells are a sag's width each way, where the ear
  clipper's grid runs long along the rulings: up to 174 times the points of its grid's cells.

The budget (`cdtBudgetPerCell`, `cdtTriangulateBudgetInto`): a curved face meshed by the
tolerance's mesher is given back to the ear clipper where the CDT's points inside pass 24 times
the cells of the grid the ear clipper would mesh the patch on (`MakeTriangulationGridInto`), and
is then told to the mender as an ear-clipped face that strays, as before the CDT was the default
(without that, a chamfer on a raised band, `TestChamferRaisedBand`, came out open). The mender's
own call for the CDT has no budget. Over the solid, thread, display, STL, export and script
goldens no face passed 19, so no golden moves. At 32, `TestWrapRoundMostOfTheWay` kept faces of
24 to 32 and took 2.2 times the base. The drafted boss is untouched by it: its shell's
`faceModeOf` asks `FaceAt` for every triangle's middle of each face, and the frustum's
parametric-middle triangles are under the budget.

## Fine tolerances: sag square to the triangles (2026-11-05, rd9-cdt-sag)

The parametric-middle measure is gone. `cellSag`'s term across a cell's middle is now the
distance of the surface's middle from the two triangles either diagonal makes, measured square
to them, and `refinePoints` measures a triangle's middle and its edges' middles square to the
triangle's plane: a point of the surface that the chart puts off the triangle's middle along
the triangle, as between a frustum's rulings, is covered by the triangle and is no error. The
budget stays, for the cone charted polar.

`FaceAt`, which a joined shell's `faceModeOf` asks at every triangle of every face, projected
each loop's points onto its surface again for every face it tried. It now keeps the outlines
per topology (`loop_uv_cache.go`) and passes a face whose surface's control points lie farther
from the point than both the faces already found, which the patch can't be nearer than. A pass
skips projections, so a surface's last closest point (`SSurface.Cached`, the next search's
first guess) can differ from before; no result moved by it.

Alone, without the race detector, at loads of 20 to 56 from other work (base: the ear clipper,
bf905a66):

| Test | base | budget only (bac83ed3) | now |
|---|---|---|---|
| `TestBandEndOnCone` | 1.6 s | 3.7 s | 3.7 s |
| `TestWrapRectangleOnCone` | 7.3 s | 17 s | 13 s |
| `TestWrapRoundMostOfTheWay` | 21 s | 70 s | 28 s |
| `TestShellBlockAndDraftedBoss` | 1.1 s | 15 s | 3.7 s |
| `TestFilletRunsOffOntoCylinder` | 78 s | 122 s | 138 s (loaded) |

What moved: the meshes of curved faces whose cells were refined for the parametric gap, in the
display outlines, the draw calls, the text window, the `.tlspr` round trip and 25 interface
scripts' triangle counts and volumes (regenerated in commits of their own). The solid, thread,
STL and export goldens did not move. `TestChamferCornerOracle`'s box corners, where ours is a
setback and OpenCASCADE's a triangle, came out up to 0.002 mm³ past the half cubic millimetre
it allowed: the curved setback is meshed coarser, within the tolerance, and the test allows
the mesh's 0.005 as its OpenCASCADE-volume case does.

Under `-race`, the four packages that time out in `make ci` run together, the machine loaded 87
to 110 by other work, in processor seconds before (bac83ed3, alone on the machine) and after:
`pkg/srf` 1334 to 888, `pkg/sketch` 9401 to 6994, `pkg/topo` 8602 to 6467, `pkg/script/api`
10 284 to 8652; all pass. The wall times (414, 1388, 1557, 1650 s) were taken under that load.
