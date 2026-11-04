# K4: the constrained Delaunay mesher

Refreshed 2026-11-03 at `c4e18d1a`; measured again 2026-11-04 (below). The spike's full results and its tables are in
`git show c4e18d1a:doc/dev-notes/k4-cdt-spike.md`.

## State

- The mesher is in `devel`: `pkg/srf/triangulate_cdt.go`, on the exact-predicate `pkg/internal/cdt`
  that STEP's tessellation also uses. It is chosen by `srf.Tolerance.WithMesher(srf.MesherCDT)` or
  `Sketch.Mesher`, which the sketch carries into the tolerance at each regeneration. The default is
  the ear clipper.
- It runs as a mender (`triangulate_mend.go`, candidates in `triangulate_check.go`). The ear
  clipper meshes every face; `mendCrossings` meshes again by the CDT a face that strays from its
  surface by more than four chord tolerances, folds, or holds a sliver (on a plane or a curved
  face), and is unsound in the shell's mesh, and every face that shares a triangle with another.
  No golden moved when each of these was added.
- The branch `k4-cdt-spike` adds nothing `devel` lacks: its mesher came in as `b33ac6f7`.
- Making the CDT the default is postponed. It would close the twisted swept bar's 1.6% volume
  error, the one open item no mender reaches.

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

## Measurements (spike, 2026-10-10, CDT on every face)

| | CDT | Ear clipper |
|---|---|---|
| Twisted swept bar, 45 degrees, volume error | 0.003% | 1.6% |
| Crossing cylinders at 0.1 mm, triangles | 1738 | 364 |
| `pkg/topo` tests | 260 s | 77 s |

The quadtree alone is 1.7 times the ear clipper's triangles; grading near the trim and refinement
are each about half of the rest. The times predate the test suite review; the ratio is what holds.

## To make it the default

- **Cost.** 1.43 times the triangles and 1.27 times `pkg/topo`'s CPU since 2026-11-04, for a mesh
  that meets the chord tolerance where the ear clipper's does not in a sixth of its triangles.
- **The rib rounded all over.** No self-intersecting edge on its short case since the evening of
  2026-11-04; its test does not finish in 10 minutes with the CDT on every face, the cost being
  the test's walks of 151 500 triangles. Run it in full, its checks made cheaper.
- **Goldens.** Every `.tlspr`, STL, PNG and pinned count with a curved face moves: one commit per
  regen script.
- **Tests to restate.** `TestIntersectionBranches` and `TestNearContactSweep` are, 2026-11-04. On
  the spike also
  `TestSphereThroughFace` (0.1% short), one `TestFilletRibFootSweep` point and
  `TestOracleBooleans`' hole check (should drop a whole face) failed. Run them again.
- **Slot chamfer on the cone at 3 and 3.25.** One topology problem and one crossing under the CDT,
  not traced.
- **A setting.** `Sketch.Mesher` is not saved, shown or scripted. It is needed only if both meshers
  stay.
