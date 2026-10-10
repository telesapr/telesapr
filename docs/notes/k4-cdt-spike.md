# K4 spike: a constrained Delaunay mesher, October 2026

What the two-day spike of roadmap K4 built, what it measured against the ear clipper, and what it
would take to make it the default. K4 is postponed; the code is kept, unmerged, on the branch
`k4-cdt-spike` (commits `5d8923c`, `2d91e8b`, `e372c4c`, cut from `devel` at `ce7dde1`).

## Summary

- **It is sound where the ear clipper is not.** The off-axis bore, the run-off chamfer crossings
  and the twisted bar's volume bias all go away, and the STEP corpus reads 9 of 9 files as sound
  solids, NIST_MBE_PMI_6 and Schenkel (K5's two open files) included.
- **It costs 3 to 5 times the triangles.** `pkg/topo`'s tests take 260 s against 77 s.
- **It is off by default on the branch.** No golden moved. Making it the default means bringing
  the cost down, then regenerating every golden with a curved face.

## What was built

`pkg/srf/triangulate_cdt.go`, on the exact-predicate `pkg/internal/cdt` that STEP's tessellation
(`pkg/persist/step/tessellate.go`) already used.

- **Selection.** `srf.Tolerance.WithMesher(srf.MesherCDT)`, `srf.SetMesher` for tests, and
  `Sketch.Mesher`, carried into the tolerance at each regeneration. There is no global flag.
  `SSurface.TriangulateInto` (`shell.go`) gives a face to the ear clipper when the CDT refuses it.
- **Boundary.** The trim's uv points as they are, so a face shares its vertices with the face
  beyond. Consecutive points are merged when they are one point on the surface.
- **Chart.** u and v scaled by the surface's mean speed along each, so the chart is in about
  millimetres. On a patch with exactly one side folded to a pole, the chart is polar about the
  pole, and the whole side is one point.
- **Interior points.** The centres of a quadtree's leaves:
  - a cell is halved across x where the surface sags from its sides' chords along x by more than
    the chord tolerance, across y likewise, and both ways where it twists from both diagonals;
  - where trim segments cross a cell, it is halved until it is at most twice the shortest of them;
  - no cell is cut finer than 1/(2 MaxSegments) of the patch;
  - a point is kept only if it is clear of the boundary by half its cell, and, in xyz, clear of
    each trim chord by that segment's sag (twice the surface's rise over the chord's middle, at
    least the chord tolerance).
- **Refinement.** Up to six rounds. Each inserts a triangle's centroid, or the middle of an edge
  that is not the boundary's, where the surface strays from it by more than the chord tolerance.
  Points nearer the boundary than a fifth of the triangle's shortest side are left out.
- **Flat faces.** The CDT of their outline alone, with no interior points.
- **Fallback.** A boundary that runs back over itself (a spike of no width, as screw.step's
  seams) goes to the ear clipper, since the even-odd rule cancels edges the face beyond keeps.

Tests on the branch: `pkg/srf/triangulate_cdt_test.go` (lathed shapes, the bore and handle sweeps,
a plane with a hole, the spike fallback, the polar chart), the twisted bar in
`pkg/sketch/sweep_test.go`, and `TestSolidCorpusCDT` in `pkg/persist/step/solid_test.go`.

## Measurements

With the CDT mesher used for every face. The measurements were taken with a temporary environment
override in `Tolerance.Mesher`, which was removed before the commit; put it back the same way to
measure again.

| Fixture | CDT | Ear clipper |
|---|---|---|
| Twisted swept bar, 45 degrees, volume error | 0.003% | 1.6% |
| `TestBoreToLathedTipSweep` (full grid) | 0 of 80 fail | 20 |
| `TestHandleOnPotSweep` (full grid) | 6 of 144 fail | 9 |
| `TestRunOffSweep` (full grid) | 2 of 36 fail, both fillet fits refused, no crossing | 4 |
| Slot chamfer on the cone at 3 / 3.25 | no naked edges; 1 topology problem / 1 crossing | 6 / 10 naked edges |
| STEP corpus as solids | 9 of 9 sound | 7 |
| Band-on-cone fillet, r 1, chord 0.001% to 0.005% | sound | sound |

Everything else in `pkg/srf`, `pkg/topo` and `pkg/check` passes, except four tests:

- `TestIntersectionBranches` pins triangle counts.
- `TestSphereThroughFace` comes out 0.1% short. The mesh sits at the chord tolerance, where the
  ear grid is denser on those patches.
- One `TestFilletRibFootSweep` point adds 36.09 against 35.95 within 0.125.
- `TestOracleBooleans`' hole check drops two triangles, now too small for its rays to find.

Cost:

| | CDT | Ear clipper |
|---|---|---|
| Crossing cylinders at 0.1 mm, triangles | 1738 | 364 |
| `pkg/topo` tests | 260 s | 77 s |
| `TestRunOffSweep`, full grid | 84 s | 23 s |

Where the extra triangles come from, on the crossing cylinders: the quadtree alone is 1.7 times
the ear clipper's; the grading near the trim and the refinement rounds are each about half of the
rest.

## Tried

- **No grading near the trim and no refinement.** This halves the triangles, but it broke drafts,
  fillet sweeps and the rolling ball. Grading to 8 segments rather than 2 changed no sweep.
- **An isotropic quadtree.** A cylinder was cut along its rulings too: 8302 triangles on a rod
  the ear clipper meshes in 144. Splitting x and y separately took it to 1424.
- **A sag taken from the boundary's turn** (as `gridNearTrim` does). At a corner between two
  curves the turn says nothing of either curve. It kept points 2.5 mm off a straight seam, and
  the cylinders came out short.
- **No floor on the cells.** `TestLoftTwist`, at chord 0.001%, took 282 s against 0.3 s.

## Traps

- Two curves' end points at a corner differ by about 1e-12 in uv and must be one point, or the
  CDT gets a segment it can't keep. But `Equals` in uv (LengthEps) merged real points of a short
  trim on a large STEP face (NIST_MBE_PMI_11). Merge points only when they are one point on the
  surface.
- `splitTrimDiagonals` must not run after the CDT. It split trim edges that the face beyond kept
  whole.
- `AssemblePolygon` consumes the edge list it is given.

## To make it the default

| Item | State |
|---|---|
| Cost | 3 to 5 times the triangles. First candidates: coarser grading, refinement only on edges, a looser refinement bound |
| Goldens | Every `.tlspr`, STL, PNG and pinned count with a curved face moves, one commit per regen script |
| Slot chamfer at 3 and 3.25 | One topology problem and one crossing, not traced |
| The four failures above | Bounds to restate for the new mesh; the oracle's hole test should drop a whole face |
| A setting | `Sketch.Mesher` is not saved, shown or scripted. Needed only if both meshers stay |
