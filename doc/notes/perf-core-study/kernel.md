# Kernel track of the core performance study, October 2026

What was measured and changed in the geometry kernel (`pkg/srf`, with the
callers in `pkg/topo`) on the branch `perf-core-kernel`, and what to do
next. It follows `doc/notes/performance-study.md` (the October 10 study),
whose P1 (cache the ray subdivision) and P3 (mesh BSP without an
allocation per node) were still open; nothing else of its P1–P9 had
landed but a part of P6.

Every change here leaves every model byte-identical: the 277 fixtures
regenerate to the same bytes before and after each commit
(`scripts/perf-fixture-hashes.sh`, 5 FAIL before and after alike, as in
the earlier study).

## Summary

| Benchmark | Before | After | Change |
|---|---:|---:|---:|
| RegenerateLargest (triple M12 thread, all goroutines) | 155.6 ms | 128.3 ms | −17.5% |
| RegenerateTranslate/off (12x12 step and repeat, one goroutine) | 351.0 ms | 175.8 ms | −49.9% |
| RegenerateTranslate/on | 105.2 ms | 87.9 ms | −16.5% |
| FilletTee (rolling-ball fillet round a tee) | 93.0 ms | 64.0 ms | −31% |
| PathSweep | 137 ms | 122 ms | −11% |
| SmoothLoft | 5.94 ms | 5.62 ms | −5.5% |
| RegenerateText, RegenerateFillet, the other Fillet*, Shell*, Loft*, Sweep*, Chamfer*, Mirror | | | unchanged |
| RegenerateModel (8x8 forced-mesh union, new; P3 alone) | 17.51 ms | 13.04 ms | −25.5% |

The first three rows and the last are the final six-run comparisons
(spreads ±1–4%, p = 0.002). The host was shared with two other agents'
test runs through most of the work, so the intermediate comparisons
quoted below have wide spreads (up to ±50% on the thread, ±200% on
single runs of the topo set); only differences benchstat marks
significant (p ≤ 0.05) are claimed anywhere here.

## Method

- Baseline test binaries of `pkg/persist/slvs`, `pkg/sketch`, `pkg/srf`,
  `pkg/topo` built at devel (50409a4e) and kept; after each change the
  new binary and the baseline were run interleaved six times and
  compared with benchstat (`go install golang.org/x/perf/cmd/benchstat`
  into the scratchpad; not in `go.mod`).
- CPU profiles of FilletTee, PathSweep, RegenerateTranslate/off and
  RegenerateLargest at the baseline, and again after each change, with
  `-cpuprofile`; allocations with `-memprofile`.
- Same bytes: `make build` equivalent (`go build ./cmd/telesapr-cli`) and
  the hash script, diffed against the baseline's output, after each
  change.
- A scaled model for the mesh Boolean: `testdata/group/translate_nd/
  normal.slvs` with both `Group.valA` at 8, `meshCombine` 0 and
  `forceToMesh=1` on the two step-and-repeat groups, converted with
  `telesapr-cli convert`. It lives in the scratchpad, as the earlier
  study's did; `BenchmarkRegenerateModel` in `pkg/persist/tlspr` takes
  its path from `TELESAPR_BENCH_MODEL` and is skipped without it.
  (Converting the `.slvs` gave the step groups `combine difference`
  for `meshCombine=0`, which the sketch's constants say is union; the
  `.tlspr` was set to `combine union` by hand. Worth a look by whoever
  owns the loader.)

## Baseline profiles (top of the cumulative list, kernel entries)

**RegenerateLargest, 154 ms, all goroutines.** `allPointsIntersecting`
(ray against surface) 47.6%, of which `intersectByMarching` is the
caller for 44%; `ClosestPointToUV` 34% in all; `AllPointsIntersecting-
Untrimmed` (the subdivision) 33.5%; `MakeCopySplitAgainst` 28%;
`makeCopyTrimAgainst` 27%; `ClassifyEdge` 19.6%; `TangentsAt` 18.5%.
Within `ClosestPointTo`'s 4.28 s: 1.30 s from the subdivision's leaves,
1.28 s from the cylinder/plane branch of `allPointsIntersecting`
(a closest point with `mustConverge` on a cylinder searches the whole
guess grid while the caches are off), 1.19 s from
`ClosestPointOnThisAndSurface` (marching).

**RegenerateTranslate/off, 359 ms.** `MakeFromBoolean` 84.5%;
`CopyCurvesSplitAgainst` 63%; `MakeCopySplitAgainst` 62%;
`coplanarTrimCrossings` 59%, of which `TangentsAt` via `NormalAt` 37%:
for every segment of every curve split, the normal of every flat
surface of the shell was taken again.

**FilletTee, 91 ms.** `rollingBallFillet` 84%; `ClosestPointToUV` 67%,
called from `footNear` → `nearestOnWithin` (64% of it) and
`surfaceLoopUV` (26%). Inside: the guess-grid search by `PointAt` 26%,
`closestPointNewton` 28%, `closestPointRestarts` 28%. Outside a
Boolean's loops no surface had a guess grid, so every closest point
evaluated the surface at all 400 grid points.

**PathSweep, 130 ms.** `TriangulateInto` 40% (`uvGridTriangulateInto`
26.5%, `ClosestPointToUV` 26%); `Build` (topology) 27%, of which
`findSheets` → `isSeam` 23%, again closest points without a grid.

**Allocations, RegenerateLargest (alloc_space).** `SEdgeList.AddEdge`
0.51 GB (33%), from `makeTrimEdgesInto` appending an edge at a time;
`newGuessGrid` 0.13 GB (8.6%), the grids made again for each of a
Boolean's three loops; `core.Expr.clone` 0.09 GB (the solver, another
track's).

## What changed

Each section is one commit on `perf-core-kernel` (d9969357, 75e4ebe8,
8bf0f929, 80602213, ee9a82bc, ac8af011).

### P1: the ray subdivision kept between rays (`srf/raycast_tree.go`)

`AllPointsIntersectingUntrimmed` split a curved surface in halves by u
and v in turn down to pieces flat within the chord tolerance, and
projected the centre of each flat piece on the surface for the Newton
guess, for every ray. The pieces' boxes, which are flat, and their
guesses depend only on the surface, so the first ray to reach a piece
now makes its node of a per-surface tree and the rest read it; the walk
keeps the recursion's order, box test and cap of 2000 pieces, so it
finds the same hits in the same order (a randomised test holds it to the
recursion, also from goroutines at once).

Three designs were measured:

1. Nodes with no control net, the net made again from the root by the
   splits on the way down when a node is first split: **no gain** (the
   remaking was 13% of the thread). Most nodes of a thread's surfaces
   are visited by one or two rays only, so what is cached must cost no
   more to make than the recursion did.
2. Nodes keeping their net (one `[]float64` per unsplit node, let go once
   it is split; a flat node keeps only the centre of its corners), the
   guess made lazily by the first ray to reach the leaf: −9.3% on the
   thread (159 → 145 ms), but +74% bytes and +153% allocations.
3. As 2, with the guess published through a state word rather than a
   pointer, and the cells for the tree and the guess grid living through
   the three loops of a Boolean (`makeGuessGrids` makes a cell only
   where there is none; `CleanupAfterBoolean` takes them away) instead
   of being made for each loop: −13% on the thread, +64% bytes, +97%
   allocations. Kept.

A leaf is now projected once instead of at every visit: 96.7k guesses
for 152k leaf visits in a thread regeneration (36% fewer). The guess
grids, made once per Boolean instead of three times, are where the
bytes allocated by the thread fall again by 8%.

### Planes for coplanar trim crossings (`surfindex.go`, `boolean_csg.go`)

`coplanarTrimCrossings` tested every flat surface of the shell for every
segment of every curve split, taking its normal at the middle each time.
The shell's surface index now keeps each surface's plane (unit normal at
the middle, first corner), made by the first call, and a map from handle
to place for the planes `fuseCoplanarSeam` asks for by handle; the
arithmetic and the order of the loop are as they were. The 12x12 step
and repeat is −24% on one goroutine (377 → 287 ms) and −12% on all.
Before the by-handle map, `fuseCoplanarSeam`'s `planeOf` had become 28%
of the parallel run on its own.

The loop is still linear in the shell's surfaces per segment, with a
dot product and a plane distance each. It is no longer in the profile's
top, but for shells of several thousand flat patches the next step is
to bucket the planes by normal and distance with a tolerance-aware
search (see below).

### Fused point and tangents (`PointAndTangentsAt`)

Every Newton step in the kernel evaluated `PointAt(u, v)` and then
`TangentsAt(u, v)`, which sums the same Bernstein products again to
make the tangents' quotients. `PointAndTangentsAt` gives the point from
that pass, the same to the bit (same products in the same order; the
point is `num.ScaledBy(1/den)` as in `PointAt`), and `TangentsAt` is a
wrapper of it. Used in `closestPointNewton`, `PointOnCurve`,
`PointIntersectingLine`, the two- and three-surface Newton loops, and
`topo`'s `nearestOnWithin` and `rollingrunoff`. Measured together with
the next two, below.

### Guess grids for the topology and the rolling ball

`(*SShell).KeepClosestGuesses(keep bool)` gives a shell's curved
surfaces the guess grid a Boolean's loops have, or takes it away.
`topo.Build` keeps them while it runs (seams and loops project many
points) and `filletPicksWith` while the ball rolls. The points found are
the same, since a grid holds `PointAt` at the same parameters the search
evaluated. FilletTee −31%, PathSweep −11%, SmoothLoft −5.5%.

### P3: mesh BSP without a util per node (`bsp.go`, `mesh.go`)

A `bspScratch` holds a `bspUtil` per depth of one insertion (a node's
split pieces go in below it one after the other, so the util of a depth
is in use while those below come and go); the utils reuse their flag and
vertex slices. One scratch serves a whole `SBsp3FromMesh` and a whole
`AddAgainstBsp`; the exported `Insert`, `InsertHow`, `InsertConvex`,
`InsertConvexHow` and `SBsp3InsertOrCreate` make one for the call. The
earlier study's pool measured −13% on a 16x16 mesh model and was
rejected for being a package variable; this is the same saving within
the rules. On the 8x8 mesh model, this change alone: −25.5% time
(17.51 → 13.04 ms), −50% bytes (37.1 → 18.5 MiB), −76% allocations
(174.6k → 41.1k). The exact Boolean (12x12, Largest) is untouched by
it, as expected.

## Allocations

Before and after the branch (six-run comparisons):

| Benchmark | B/op before | B/op after | allocs/op before | allocs/op after |
|---|---:|---:|---:|---:|
| RegenerateLargest | 56.3 MiB | 81.5 MiB | 187k | 358k |
| RegenerateTranslate/off | 116.3 MiB | 138.9 MiB | 457k | 457k |
| RegenerateModel (mesh, P3 alone) | 37.1 MiB | 18.5 MiB | 175k | 41k |
| FilletTee | 5.70 MiB | 5.29 MiB | 20.7k | 19.9k |
| PathSweep | 48.2 MiB | 50.4 MiB | 91.0k | 91.4k |

The thread allocates more: the ray tree's nodes and the control nets
at its frontier (one `[]float64` per unsplit piece, dropped when it
splits). The time saved is in computing, not in allocating, and the
GC share of the thread profile is the same before and after (about
6%). The 12x12's extra bytes are the plane and handle tables of its
indexes, kept per Boolean. The first-visit guess state (`guessState`)
rather than a pointer, and the one array per net, took the thread's
allocations from +153% to +91% over the baseline; what is left is one
allocation per split (the pair of halves) and one per unsplit piece.

The mesh BSP scratch is the one change that takes allocations away in
bulk: three quarters of a mesh Boolean's.

## Concurrency: what is still serial in a Boolean

Measured with the CLI on the triple M12 thread (`hyperfine`, 10 runs):

| | all goroutines | one goroutine | ratio |
|---|---:|---:|---:|
| devel | 235 ms | 773 ms | 3.3x |
| this branch | 138 ms | 425 ms | 3.1x |

On 32 hardware threads a ratio of 3.3 means about a quarter of the
work is serial, by Amdahl, or is in the parallel loops' commits. What
runs on one goroutine today, per Boolean (`MakeFromBoolean`):

- The commits of the three loops: `CopyCurvesSplitAgainst` splits the
  curves at once but adds them in order (`commitTo`, `addAndAssign-
  CurveID`, which scans for the largest id on each add);
  `CopySurfacesTrimAgainst` copies at once, then assigns handles in
  order. These are serial by design, so the result does not depend on
  the goroutines' timing.
- `RewriteSurfaceHandlesForCurves`, `fuseVertices`, and the parts of
  `MergeCoincidentSurfaces` outside its `par.For` over classes (finding
  the classes).
- `MakeClassifyingBsps` runs at once, but each shell's `newSurfaceIndex`
  (and now its planes) is built on the calling goroutine.
- In the thread, the union of the ridges is a pairwise tree
  (`group_pipeline.go`): the unions of a level run at once, but the top
  of the tree is one Boolean of two large shells on which only the
  inner loops are parallel.

The next parallelisable stage is the per-surface work of the top-level
union itself (already parallel) against its serial commits: making the
curve ids assignable in advance (count the curves each surface will
add, reserve id ranges, then add at once) would make `commitTo` an
append of pre-numbered curves. It keeps the result the same if the ids
are the ones the serial order would give. Not done here: it touches the
handle assignment of every curve and the goldens record the ids.

## Measured and not kept

- The ray tree with nets remade on the way down (design 1 above): no
  gain, since a thread's rays reach most pieces once.
- `slices.Grow(sel.Edges, len(sc.Points))` before `makeTrimEdgesInto`'s
  loop, which added an edge at a time (the growth copies were a third
  of the bytes a thread allocated): time unchanged on the thread and
  the 12x12 (p > 0.1), bytes −12% on the thread but +19% on the 12x12,
  whose edge lists keep the room reserved for whole curves of which a
  trim uses a part. Not kept; an exact count of a trim's points would
  make it a saving on both.
- Keeping guess grids across `topo.Build` for all callers permanently
  (not dropped at the end): the first quick numbers showed SmoothLoft
  and FilletRib slower, which the six-run comparison then showed to be
  noise; kept as drop-at-the-end anyway, so a shell's surfaces do not
  carry 10 KB of grid each for the life of the group.

## Recommendations, ranked by gain over cost

1. **The mesh BSP's remaining allocations (same bytes, 1 day).** With
   the utils gone, what a mesh Boolean allocates is the nodes
   themselves (`SBsp3`, one per triangle kept, with the triangle copied
   into it), the `More` chain of coplanar triangles and the `SBsp2`
   edge nodes; an arena per tree (a slice of nodes indexed instead of
   pointers) would be one allocation per tree. Also `SBsp3FromMesh`
   copies the whole mesh before shuffling it.
2. **`ClosestPointToUV` on cylinders and planes during the loops (same
   bytes, 1–2 days).** 9% of the thread is closest points with
   `mustConverge` on cylinders, searching the 400-point grid because the
   caches are off. A closed-form first guess for a surface
   `IsCylinder` says is one (project on the axis, take the angle)
   followed by the same Newton would give the same point when Newton
   converges to the same root, which must be checked fixture by fixture:
   goldens if it does not.
3. **Bucket the planes for coplanar trim crossings (same bytes, 1 day).**
   Group the index's planes by rounded normal and distance in a map,
   with the neighbours of a bucket searched too, so that the candidates
   are a handful instead of all the shell's surfaces; test every
   candidate with the exact test in surface order.
4. **Curve ids reserved in advance (same bytes if the ids match,
   3–4 days).** See the concurrency section; makes the commits of the
   two curve loops parallel and takes `addAndAssignCurveID`'s scan away.
5. **P7, constrained Delaunay triangulation of trimmed surfaces
   (goldens, 1 week).** Unchanged from the earlier study: ear clipping
   with holes bridged in is O(n²) to O(n³) in a face's vertices;
   `pkg/internal/cdt` exists (STEP faces, the K4 mender). Every mesh
   golden moves.
6. **P8, robust predicates for the mesh BSP (goldens where it fixes
   something, 3–4 days).** Shewchuk's adaptive `orient3d` in-tree; for
   robustness first.
7. **P9, `GOAMD64=v3` (goldens, half a day to measure).** Lets the
   compiler fuse multiply-adds; changes the last bits everywhere. The
   kernel is bound by pointer chasing and branches more than by
   arithmetic (the profiles above), so expect little.
8. **SIMD Bézier evaluation.** Not worth it: `PointAndTangentsAt` is
   degree-dependent loops over at most 36 control points, already the
   fused kind; a 4-wide version would change summation order, so
   goldens, for a part of the profile that is now under 20%.
9. **A BVH for mesh Booleans instead of the BSP.** The BSP is the
   algorithm the goldens' meshes record (which triangle is split where);
   a BVH-based Boolean is a different mesh. Only with goldens, and only
   if mesh Booleans matter more than they do (they are the fallback and
   `forceToMesh`).
10. **Caching `AxisAlignedBounding` on the surface.** The index keeps
    the boxes of a shell's surfaces (earlier study), and the ray tree
    keeps those of the pieces; the remaining calls are on pieces made
    and dropped at once. No longer in the profile's top.

## Reproducing

```sh
go test -c -o base.test ./pkg/persist/slvs      # at devel, and again after
(cd pkg/persist/slvs && for i in 1 2 3 4 5 6; do
  ../../../base.test -test.run '^$' -test.bench 'Regenerate' -test.benchmem -test.count 1 >> base.txt
  ../../../new.test  -test.run '^$' -test.bench 'Regenerate' -test.benchmem -test.count 1 >> new.txt
done); benchstat base.txt new.txt
TELESAPR_BENCH_MODEL=/path/to/tu8.tlspr go test -run '^$' -bench RegenerateModel ./pkg/persist/tlspr
./scripts/perf-fixture-hashes.sh > after.txt && diff before.txt after.txt
```
