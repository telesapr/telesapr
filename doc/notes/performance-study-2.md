# Performance study 2: the geometry core, October 2026

Where TeleSAPR spends its time solving sketches, building solids and drawing frames, what the
branch `perf-core-study` changed, and what is left to gain. It continues
`performance-study.md` (2026-10-10), of whose work packages only a part of P6 had landed since.
The three parts of the work have a note each under `doc/notes/perf-core-study/`: `solver.md`,
`kernel.md` and `drawing.md`, with the profiles, the benchstat tables and the designs; this file
is the view across them.

## Summary

- **Nothing a model makes has changed.** All 277 `.tlspr` fixtures regenerate to the same bytes
  as on `devel` (`scripts/perf-fixture-hashes.sh`), the renderers' PNG goldens and the window's
  frame goldens are untouched, and the OpenGL frames dumped under Xvfb before and after compare
  equal. Every change is measured against `devel` with benchstat over six interleaved runs.
- **The window.** A frame is 27–56% faster across the paint benchmarks (geomean −37%), and
  a frame of a small model went from 9.1 to 5.7 ms: the 8 ms that 80 triangles "took" was
  40 MB of clears and copies per frame and window-sized quads drawn on one goroutine. OpenGL
  frames are 13–50% faster, from one `glDrawArrays` per run instead of a cgo call per vertex.
- **The solver.** Adding a constraint to a sketch of a few hundred segments was quadratic: the
  rank test factored on past the rank, recomputing the elimination tree for each of hundreds
  of dead columns. A rank-only factorisation that stops where the rank cannot grow, the symbolic
  analysis kept across the factorisations of one solve, and the unknowns sorted once make a
  solve of a sketch being dimensioned 82–94% faster and a drag step's solve 54% faster.
- **The kernel.** The ray-cast subdivision of a surface is built once per Boolean and walked per
  ray (P1 of the first study), the mesh BSP no longer allocates a util per node (P3), the
  rolling-ball fillet gets the closest-point guess grids a Boolean has, and the planes of flat
  surfaces are kept on the surface index. The triple M12 thread is 17.5% faster in the process
  and 40% faster from the command line; a 12x12 step and repeat 50% faster on one goroutine; the
  tee fillet 31%; an 8x8 forced-mesh union 25%.
- **Where the time now goes, and what to do next**, ranked below: the fill of the sparse QR,
  which is an Eigen artefact the solver reproduces bit for bit and a row permutation removes
  (86x on the Newton step of a large sketch, goldens); the regeneration's second pass for the
  model's size (half of a display-mode regeneration of a sketch, same bytes in export mode);
  the rasteriser's inner loop and vertex buffers kept across frames; a Jacobian kept between
  solves of a group; arenas for the BSP's nodes.
- **No third-party library is recommended.** The time is in pointer-chasing geometry, symbolic
  bookkeeping and allocation, not in dense arithmetic; the one algorithm worth taking from
  outside, Shewchuk's predicates, is small enough to write in-tree. Go's `simd` package is
  still an experiment in 1.27 and would change the last bits of every sum.

## Method

- **Same output.** `scripts/perf-fixture-hashes.sh` on `devel` and after each change, diffed;
  the five fixtures that fail from the command line (missing IDF/STL links, two images, the v1
  `fields` file) fail before and after alike. For the renderers, the PNG goldens, the frame
  goldens of `pkg/ui`, `TestPaintParallelSameAsSerial`, and `cmp` of the GL frame dumps.
- **Time.** Test binaries before and after, run interleaved six times, compared with benchstat
  (installed to the scratchpad, not in `go.mod`). The host was shared by the three parts of the
  work through most of it, so the intermediate tables in the notes have wide spreads; only
  differences benchstat marks significant are claimed. The final tables were taken with the
  host quiet.
- **Benchmarks added.** `pkg/solver/bench_test.go` (chains of 20–400 segments, open and closed,
  a sketch being dimensioned, a drag step, the Jacobian and the factorisations alone),
  `pkg/ui/drag_bench_test.go` (a point drag end to end, solve and paint apart),
  `pkg/persist/tlspr/model_bench_test.go` (any model from `TELESAPR_BENCH_MODEL`), and the
  rank-alone benchmarks.

## Where the time went

### A drag of a sketch point, in the window

`BenchmarkDrag<n>`: a zigzag of n segments in a 1920x1080 headless window, one mouse step of
a point drag; the solve and regeneration the step makes, and the frame drawn after it,
reported apart. Before this branch:

| n | drag (solve + regen) | paint (software) | step |
|---:|---:|---:|---:|
| 20 | 0.28 ms | 7.5 ms | 7.8 ms |
| 150 | 2.8 ms | 12 ms | 15 ms |
| 400 | 11.4 ms | 21.5 ms | 33 ms |

The frame dominates at every size; the solve is a quarter to a third at 400 segments. The paint
grows with the sketch because every segment is an antialiased line through the rasteriser, and
every constraint a label. After the branch (same benchmark, same host):

| n | drag | paint | step |
|---:|---:|---:|---:|
| 20 | 0.34 ms | 4.9 ms | 5.2 ms |
| 150 | 1.7 ms | 6.3 ms | 7.9 ms |
| 400 | 4.7 ms | 9.9 ms | 14.6 ms |

A step of the 400-segment drag went from 33 to 14.6 ms; the solve's half of the gain is the
symbolic analysis kept across the factorisations and the rank test stopping at the rank, the
frame's half the renderer's fixed cost gone. The sketch is still regenerated twice per step
(next section) and drawn line by line.

### A regeneration in display mode solves everything twice

`generateAll` in display mode first runs itself with `genForBBox` to find the model's size,
from which the chord tolerance follows, then runs again to build. In the profile of
`BenchmarkRegen150` the inner pass is 24.7 of the outer's 50.2 s cumulative: half of a
regeneration of a sketch, every group solved twice. In export mode there is no such pass. See
recommendation 3.

### The solver, corrected

The first profile of `BenchmarkRegen150` put 84% of the solve in the rank test. That was the
benchmark's setup, not its loop: building the 150-segment zigzag one constraint at a time took
7 s, 25 ms per constraint by the end, because each addition regenerates with a rank test of up
to 98 equations against 600 unknowns, with 500 dead columns and a column elimination tree
computed again for each. The timed regeneration of the finished sketch takes 2.5 ms and solves
by substitution (every equation is `a − b`, so after substitution there are no equations left).
What was slow is the interaction of drawing and constraining a large sketch, and that is what
the rank-only factorisation fixes: 4.4 → 0.1 ms per factorisation.

For a sketch that is dimensioned rather than drawn, the sparse QR is the cost. On a closed
chain of 400 segments (802 unknowns) the factors hold about n²/8 entries where a banded structure
would give 2n:

| n | A | nnz(A) | nnz(Q) | nnz(R) | time |
|---:|---|---:|---:|---:|---:|
| 100 | 202x202 | 402 | 5 452 | 10 552 | 1.5 ms |
| 400 | 802x802 | 1 602 | 81 802 | 162 202 | 71 ms |
| 400, transposed (Newton step) | 802x400 | 1 602 | 81 000 | 40 599 | 28.5 ms |

The cause is the row order: Eigen's `coletree` assumes row i has its diagonal entry in column
i, and the rows are equations in handle order, unrelated to the columns' params. A greedy row
matching before the factorisation cuts the Newton step's 28.5 ms to 0.33 ms, with nnz(Q) 81 000
→ 800. It changes the bytes (the Householder sums run in another order), so it is
recommendation 1, not landed. `Expr.Eval` is not a cost: the 1 030 expressions of the 100-chain's
Jacobian evaluate in 25 µs; a flat tape gave 18 µs.

### Solids

The triple M12 thread, before: `AllPointsIntersectingUntrimmed` 27% (the surface split in halves
down to flat pieces for every ray), `ClosestPointToUV` 27% (Newton through `TangentsAt` and
`PointAt`, each a Bernstein pass), `geom.Vector` arithmetic 15% flat, allocation and GC 13%.
The 12x12 step and repeat on one goroutine: `coplanarTrimCrossings` recomputing the planes of
flat surfaces, trim building, `AxisAlignedBounding`. The tee fillet: the rolling ball projecting
thousands of points on surfaces whose guess grids were off, so each projection evaluated the
surface at 400 grid points. The forced-mesh union: a `bspUtil` and five slices allocated at
every BSP node an insertion passes, 36% of all allocations.

After the kernel's changes the thread's time is in `ClosestPointToUV` with `mustConverge` on
cylinders during the loops (9%, the caches being off there by design), the ray tree's walk, the
serial commits of the curve loops, and `fuseVertices`. The parallel speedup on 32 threads is
3.1–3.3x: about a quarter of a Boolean is still serial (the loop commits, curve-id assignment,
`fuseVertices`, merge class finding, the top of the pairwise union tree).

### Frames

A frame of the 80-triangle Boolean model took 8 ms in software: the depth buffer was cleared
four times, 8 MB each, the persistent layer copied, 16 MB, on every orbit frame, and the
window-sized background quads were drawn on one goroutine while the triangles went in bands.
The lit mesh frames were in `rasterTriangleRows` (48% flat), with a `[4]float64` colour per
pixel rounded and clamped per channel, and clip planes as function values that made every
clipped vertex escape. In OpenGL every vertex was a cgo call (`gl.Vertex3d`, `gl.Normal3d`:
7% in `cgocall`). Allocations per frame: 11 832 for the bolt in software, 25 613 for the ball
mill, 4 225 for the text model's paint; now 646, 3 200 and 258.

### GC

The window binary runs at the default `GOGC=100`; `telesapr-cli` sets 400. On the 150-segment
regeneration (in process, host quiet) 400 gave −13% (2.89 → 2.52 ms). On the solids' benchmarks
the comparison was taken with the host loaded and is not reliable; it is worth redoing before
setting anything in the window, where memory is held for a session rather than one model (the
first study measured +20–65% peak memory on the CLI).

## What changed

Each line is one commit; all leave every output byte-identical. The branch merges the three
tracks' branches (`perf-core-solver`, `perf-core-kernel`, `perf-core-drawing`), then adds the
drag benchmark and the regenerated script symbols.

### Solver (`pkg/solver`)

| Commit | Change | Effect |
|---|---|---|
| 72701800 | benchmarks of the solve, the Jacobian and the sparse QR | — |
| 47d45bae | `sparseQRRank`: the rank test stops factoring once the rank cannot grow; randomised test against the full factorisation | rank of a wide system 4.4 ms → 0.07 ms |
| eb6091d3 | unknowns sorted once, lazily, not on every `AddParam` | — |
| 098f9c03 | benchmarks of the rank alone and of a sketch being dimensioned | — |
| a58cf4e1 | COLAMD ordering and column elimination tree kept per sparsity pattern across the factorisations of one solve; test holds the factors identical | FactorWide −25% |
| a01676bd | one backing array for the Jacobian's values; one allocation for a sparse matrix and its transpose; `WriteJacobian` reuses its param set | bytes −10 to −50% |
| 359e5b79 | `Expr.Eval` against a flat tape, benchmark only | measured, see below |
| 04a6ca03 | the elimination tree of a dead column reuses its arrays; the sparse Jacobian built from the symbolic pattern | SolveDimensioning/50 −17% |

benchstat, whole branch against `devel`:

```
SolveChain/400           2103.1µ → 395.9µ   -81%
SolveClosed/400          37.30m  → 32.57m   -13%
SolveDimensioning/10     10.65m  → 0.60m    -94%
SolveDimensioning/50     12.77m  → 2.24m    -82%
SolveDrag                204.7µ  → 93.9µ    -54%
RankWide/50              4676µ   → 328µ     -93%
RankSquare/400           75.40m  → 69.30m   -8%
geomean                  2.054m  → 921.0µ   -55%;  B/op -77%
```

### Kernel (`pkg/srf`, `pkg/topo`)

| Commit | Change | Effect |
|---|---|---|
| d9969357 | P1: the ray-cast subdivision tree of a surface kept between rays (`raycast_tree.go`), built lazily per Boolean as the guess grid is | thread −12% |
| 75e4ebe8 | planes of flat surfaces kept on the surface index for `coplanarTrimCrossings` | 12x12 off −45% |
| 8bf0f929 | the plane of a trim's surface found by handle in `fuseCoplanarSeam` | — |
| 80602213 | `PointAndTangentsAt`: point and both tangents in one Bernstein pass, the same to the bit | thread −5% |
| ee9a82bc | `KeepClosestGuesses`: the guess grids kept through `topo.Build` and the rolling-ball fillet | FilletTee −31% |
| ac8af011 | P3: the mesh BSP's `bspScratch`, one util per depth per insertion; `BenchmarkRegenerateModel` | 8x8 mesh union −25%, allocs −76% |

```
RegenerateTranslate/off   351.0m → 175.8m   -50%
RegenerateTranslate/on    105.2m → 87.9m    -16%
RegenerateLargest         155.6m → 128.3m   -18%
FilletTee                 93.0m  → 64.0m    -31%
PathSweep                 136.8m → 122.0m   -11%
SmoothLoft                5.94m  → 5.62m    -5.5%
RegenerateModel (8x8)     17.51m → 13.04m   -25%;  B/op -50%, allocs -76%
telesapr-cli, triple M12 thread (hyperfine): 228/742 ms on/off → 138/425 ms
```

The thread allocates more than before (56 → 81 MiB per regeneration: the ray tree's nodes
and the frontier's control nets) for the time it saves; its GC share is unchanged at 6%.

### Drawing (`pkg/render/soft`, `pkg/render/gl`, `pkg/ui`)

| Commit | Change | Effect |
|---|---|---|
| 667b0b54 | soft: depth cleared lazily, big polygons drawn in bands, opaque colour converted once per triangle, per-row edge terms hoisted in the same expression, clip planes a switch | boolean frame 9.1 → 5.7 ms; lines −10%, allocs 100k → 0 |
| 8d2aa7da | gl: each primitive run collected into client arrays and sent with one `glDrawArrays` | opengl frames −13 to −50% |
| eb9d01e0 | gl: arrays stay enabled between runs, pointers set only when a slice moved; points of a stroke one run | — |
| 273896ec | ui: the persistent layer saved only when the frame before had the same key | PaintHover −56% |

```
Frame/boolean/software    9.145m → 5.654m   -38%
Frame/bolt/software       26.65m → 19.45m   -27%
Frame/ball_mill/opengl    6.640m → 3.310m   -50%
PaintText                 7.996m → 5.071m   -37%
PaintBoolean              9.269m → 5.890m   -37%
PaintHover                10.96m → 4.810m   -56%
geomean                                     -37%
```

## Measured and not kept

- **Row matching before the sparse QR**: 86x on the Newton step of a 400-chain, but other
  bytes; recommendation 1.
- **A flat tape for `Expr.Eval`**: 25 → 18 µs for a whole Jacobian; nothing against the
  factorisation. The prototype lives in a benchmark only.
- **Clearing `mark` and `tval` only where touched** in the factorisation: they are `memclr`s
  already, under 1%; and `tval` is touched outside the pattern, so a partial clear is unsafe.
- **Incremental edge functions** in the rasteriser (adding dx per pixel): the edge pixels'
  bits change; goldens.
- **The ray tree with control nets remade from the root**: no gain, a thread's rays visit most
  pieces once anyway.
- **Pre-sizing trim edge lists with `slices.Grow`**: time unchanged, bytes −12% on the thread
  and +19% on the 12x12.
- **PGO, a `sync.Pool` for `bspUtil`, benchstat in `go.mod`**: see the first study; the pool's
  gain is now had within the rules by `bspScratch`.

## Recommendations

Ordered by expected gain over cost across the three parts. "Same bytes" means the hash script
and the goldens hold it; "goldens" means fixtures regenerate differently and go in a commit of
their own, with `pkg/check` passing. The letters refer to the notes' own lists (S for
`solver.md`, K for `kernel.md`, D for `drawing.md`).

1. **Row ordering of the Jacobian before the sparse QR** (S1). Goldens; 2 days. Match each
   row to a column with an entry in it (greedy in COLAMD order, or a maximum bipartite
   matching as in Dulmage–Mendelsohn) and factor the permuted matrix. Measured 28.5 → 0.33 ms
   on the Newton step of a 400-chain; every sketch past a few dozen constraints gains, small
   ones are unchanged. The square rank test gains less (71 → 57 ms) and needs 2.
2. **A rank test that is not a full QR** (S2). Goldens only where the threshold decides a dof
   count differently; 3 days. The rank is the integer the dof is counted from; Q and R are
   thrown away. QR of the transposed matrix with rows matched (0.33 ms against 71 ms for the
   square 400-chain), or a sparse LU with partial pivoting.
3. **Skip the regeneration's second pass for the model's size** (P4 of the first study). Same
   bytes in export mode, and in display mode wherever the size's step (`tolStep`) does not
   change; 2–3 days. Take the size from the previous regeneration's box and run the second
   pass only when it moved past a step; or keep the first pass's solved params, which the
   second pass today solves again from the converged values. Halves a sketch's regeneration
   in the window, which every drag step pays.
4. **Software rasteriser: the inner loop** (D1). Same bytes; 1–2 days. A fast path for a row
   span whose ends are both inside all three edges (the per-pixel edge test only skipped, so
   the bits are the same), and a depth-only pass when colour is not written. −20–30% on
   mesh-heavy frames, which are what a drag of a solid's dimension repaints.
5. **OpenGL vertex buffers kept across frames** (D2, D3). Same pixels; 3–4 days. A VBO per
   `*srf.SMesh` keyed on the sketch generation, and an edge array per outline list, so an orbit
   frame of the ball mill is a handful of calls; the shaded program in `shaded.go` is the
   place to start.
6. **Keep the written Jacobian between solves of the same group** (S3). Same bytes; 1 week.
   `WriteJacobian` is 20% of a dimensioned solve and most of its allocation. Cache the copied,
   bound, folded equations and their partials per group, invalidated when any param outside the
   group changes value (folding bakes known values in). Then the solve of a drag step is the
   factorisation alone, and with 1 that is small.
7. **Arenas for the mesh BSP's nodes** (K1). Same bytes; 1 day. With the utils gone, what a
   mesh Boolean allocates is the nodes themselves and the `More` chains; a slice of nodes per
   tree indexed instead of pointers is one allocation per tree.
8. **Closest points on cylinders and planes during a Boolean's loops** (K2). Same bytes if
   Newton converges to the same root, else goldens, fixture by fixture; 1–2 days. 9% of the
   thread searches a 400-point grid for a first guess on surfaces `IsCylinder` describes in
   closed form.
9. **Draw while regenerating** (D4). Behavioural; 3–5 days. The window paints after the
   regeneration the event loop schedules; painting the previous generation's meshes while the
   new one builds off-thread (there is a `ShellJob` already) keeps the window moving during a
   drag of a solid's dimension.
10. **Bucket the planes for coplanar trim crossings** (K3). Same bytes; 1 day.
11. **Curve ids reserved in advance** (K4). Same bytes if the ids match; 3–4 days. Makes the
    commits of a Boolean's two curve loops parallel, the largest of the serial quarter.
12. **Split the system into connected components** (S4). Goldens (COLAMD orders the whole
    matrix and the Householder sums span blocks); 1 week. A sketch with several unconnected
    profiles solves each alone. With 1 this is where the fill ends.
13. **Incremental solving during a drag** (S5). Goldens; 2 weeks. Keep the factorisation
    between mouse moves while the pattern is unchanged, or take one Newton step per frame from
    the previous solution. Bounded by 1 and 6.
14. **The hover frame** (D7). Same bytes; half a day. Restore only the rows the hover's box
    touches, instead of 16 MB.
15. **GOGC in the window.** Half a day to measure properly, with the host quiet: −13% on a
    sketch regeneration in process, against memory held for a session.
16. **P7, constrained Delaunay triangulation of trimmed faces** (K5). Goldens; 1 week. Ear
    clipping with holes bridged in is O(n²) to O(n³) in a face's vertices; `pkg/internal/cdt`
    exists. Not in any profile here (text's regeneration spends it in the Boolean, not the
    triangulation), so for scaling and triangle quality rather than for a measured cost.
17. **P8, robust predicates for the mesh BSP** (K6). Goldens where it fixes something; 3–4
    days. Robustness first.
18. **A tiled software rasteriser** (D5), **SIMD** (D6, K8), **`GOAMD64=v3`** (K7, P9), **a BVH
    for mesh Booleans** (K9): all goldens, all for after the above; the profiles are bound by
    pointer chasing, branches and allocation, not by arithmetic width.

A text-window benchmark (D8) is worth adding before any work on it: it paints a rectangle and
a pixmap per character cell and is in none of the frame benchmarks.

## Taken up, 2026-10-22

On branch `wp-d4-incremental` (item 17 of `modelling_upgrades.md`), same bytes throughout: a
display-mode digest of all 296 fixtures (params, entities, shells and meshes, after loading,
after groups are made dirty, after dimensions are changed and changed back, after a full
regeneration) is identical before and after, and `TestSolidCacheSameAsMaking` holds it.

- **Recommendation 3, one pass.** After the first regeneration the tolerance is known: the
  groups are solved, their loops made and their solids built at it in one pass (`passOne` in
  `generate_all.go`). Only where the size that pass finds moves the tolerance across a step, or
  a new group follows solids made at another, does the build pass run again as before. The
  params a solve took as known are put back as the build pass left them. `BenchmarkDrag20` drag
  0.41 → 0.23 ms (−45%), `BenchmarkDrag150` 1.72 → 1.33 ms (−23%); the rover regenerated in full
  1.01 → 0.96 s, its solids being most of it.
- **Solids kept by what they were made from** (`solid_cache.go`), not in the list above. A
  group's solid is a function of its definition, its entities and params, those of the groups
  before it, the solid before it and the tolerance; those are hashed group by group into a
  key, every solid made is kept as an exact copy under its key (an LRU of 256 MB and 4096
  solids), and a group whose key is kept takes a copy in place of making it. The window's
  `ShellJob`, which regenerates a copy of the sketch in full for every edit, shares the cache.
  Three-thread screw (`testdata/thread/triple_m12.tlspr`): an edit of the first sketch and back,
  131 → 45 ms a regeneration (`BenchmarkEditBack`); an undo and a redo, 267 → 47 ms
  (`BenchmarkUndoRedo`); the rover regenerated with nothing changed, 1.01 → 0.13 s. No key is
  made while something is dragged, when none would come again: drags are as with one pass alone.
- **Recommendation 9, not taken.** Drawing the last solids while a drag's are made off the
  thread goes against 151fda16, where a drag makes its solids on the thread so that linked
  parts follow the pointer; a variant that shows a job's solids a frame or two late is the
  user's call.
- **Recommendation 6, not done.** The Jacobian kept between solves of a group stays open, as do
  11–13, which move goldens.

Traps: a key must leave out what a group keeps from an earlier state that nothing reads (the
profile's error point once it is good, empty slices by where their array is), or a solid
made again from the same inputs gets another key; a group with no solid passes one key down
whatever tolerance it was made at, or the references group's first tolerance would split the
keys of a sketch and its `ShellJob` copy. A test reading a script's sketch through `Doc.With`
records an undo step and marks the active group dirty: read through the host.

## Taken up, 2026-10-23

On branch `lo-perf`, the leftovers of the list above (ROADMAP, "Modelling upgrade leftovers").
The host was shared by several other branches' tests throughout (load 150 on 32 threads), so
times are not comparable; costs are counted in instructions (`perf stat -e instructions:u`, a
benchmark run at two counts and the difference taken), which the load does not move.

- **Recommendation 6, the Jacobian kept.** The sketch keeps the system that solved a group
  (`solver_cache.go`), reseeded from the sketch at the next solve of the same params, and the
  system keeps what `WriteJacobian` wrote under what it was written from, written out in full:
  the tagged equations, which params are unknowns, and the value of every known param that
  folding bakes in (`jacobian_memo.go`). Same bytes: the fixture hashes, and
  `TestSolverCacheSameAsWriting` over every fixture edited and its points moved. Instructions
  per solve, new system against kept: closed 400-chain 330 M → 252 M, `SolveDimensioning/50`
  24.7 M → 14.4 M, `SolveDrag` 26.1 M → 13.8 M; an open 400-chain, all solved by substitution,
  5.8 M → 6.0 M (the key costs more than the little it saves).
- **Recommendation 9, behind a setting, off.** "while dragging, make solids off the thread (they
  lag)", the configuration screen's last box (`DragSolidsLate`): a drag's frames leave the
  solids to a shell job that the next frames don't stop, whose solids go in when it ends
  (`Sketch.ApplyLate`), still out of date, and the next job starts for where the sketch is then.
  Off, a drag makes them on the thread as 151fda16 has it.
- **Recommendation 11, in part.** The scan for the next curve handle at every curve added is
  gone from the Boolean's loops (`nextCurveID`, counted on); same bytes. The commits stay in
  order: on one goroutine they are 2% of the 12x12 step and repeat. Instructions per
  regeneration, every solid made again: 12x12 1.627 G → 1.590 G, text 125 M → 121 M.
- **Recommendation 12, components.** The leftovers of a solve are split into connected
  components, each solved alone (`components.go`); the rank test still counts the whole.
  Sixteen profiles of 50 dimensioned segments from off their dimensions: 9.80 G → 1.26 G
  instructions; four, 258 M → 144 M. Goldens: two fixtures of 296 regenerate otherwise, and the
  DXF import goldens and four interface goldens, by at most 1.3e-14 in params that are zero or
  whole.
- **Recommendation 13, a drag's steps with a kept factorisation.** While something is dragged
  the Newton step's factorisation is kept by the Jacobian it was made for, and steps are taken
  with it (a chord method) for as long as each halves what is left (`drag_step.go`). A dragged
  sketch comes to rest at another point of its solutions where it has freedom, within a 500th
  of the move; no file regenerates otherwise and no golden moved. A drag of a 300-chain with 50
  lengths: 25.2 M instructions a move with a new system, 16.1 M kept without the kept steps,
  13.4 M with them.

The rover regenerated from the command line is the same file and the same 33.0 G instructions:
its time is the solids'. The window's drag of a zigzag held by horizontals and verticals, which
substitution solves whole (`BenchmarkDrag150`, host quiet, 1000 steps, six runs of each
interleaved), was 1.30 ms a step on `devel`, then 1.60 ms (+23%) with every group's system kept:
the solve itself cost about the same, the rest was the collector's, scanning the systems kept.
A group whose solve leaves nothing to solve together after substitution and the equations
solved alone has no Jacobian worth keeping, so its system is let go (`Sketch.keepSystem`,
`System.Unsolved`): 1.27 ms (−1.9% against `devel`), allocations −5.5%; the fixture hashes
the same. Left: recommendations 1, 2, 4, 5, 7, 8, 10 and 14 to 18, none of
which the modelling upgrades asked for; the two rank tests of each solve are now most of a drag
step's solve, and 2 is what would take them down.

## Taken up, 2026-10-23 (the rest of the list)

On branch `lo2-perf`, the recommendations left above. Costs are instructions (`perf stat -e
instructions:u`, two benchmark counts and the difference), the host being shared; times where
instructions don't show the cost (copies), with the runs of each build interleaved.

- **Recommendations 1 and 2, rows matched.** The rows of the matrix factored are renumbered so
  that row i has an entry in the i-th column ordered (`rowmatch.go`, kept per pattern with the
  analysis): the Newton step's factorisation of A', and the rank test, which now factors A'
  where A is wide or square, the rank being the same. The rank of a square 400-chain 988 M →
  3.5 M instructions; of the wide 50-of-300 sketch 3.8 M → 0.2 M; the Newton step's
  factorisation of a 400-chain 28.4 → 0.29 ms. Solves: closed 400-chain kept 260 M → 51 M,
  `SolveDimensioning/50` kept 14.4 M → 3.4 M, the dimensioned drag kept 12.2 M → 5.2 M, sixteen
  profiles 9.8 G → 0.5 G. Goldens: 24 files of the fixtures and the interface, numbers only, by
  at most 1.2e-14 in params (1.7e-13 relative on `large_dimensions`); no rank, dof or structure
  changed; the solver's least-squares cases keep every rank. The window's drag (host at load
  5 to 30, three interleaved runs): `BenchmarkDrag150` about 1.3 → 1.1 ms a step,
  `BenchmarkDrag400` about 4.0 → 3.2 ms.
- **Recommendation 12, again.** With the rows matched, the components cost more than they saved
  (four profiles 108 M against 74 M whole): each was written again. A component's system is now
  the whole Jacobian's rows and columns for it (`components.go`): four profiles 67 M, sixteen
  502 M against 501 M whole. Same bytes.
- **Recommendation 4.** A triangle's fragments are tested for depth before their colour is
  made, and a row's pixels are tested at the ends of their run only, the edge functions being
  monotonic as computed. Same pixels (`TestRasterTriangleSameAsFullScan`). Frames: bolt
  1175 M → 994 M, ball mill 1455 M → 1193 M, `PaintTranslate` 418 M → 367 M.
- **Recommendation 14.** Clears are kept per row and done at a row's first fragment, and a frame
  put back from the layer it was last saved to or restored from copies only the rows drawn on
  since (`rowstate.go`). `BenchmarkPaintHover` 7.8 → 7.3 ms. Same pixels
  (`TestRestoreLayerRowsSameAsWhole`).
- **Recommendation 10, by surface.** The trims `coplanarTrimCrossings` tests are kept per
  surface split against, in the shell's index: the 12x12 step and repeat on one goroutine
  1572 M → 1313 M (−16%). Same bytes. Bucketing the planes was not needed once the candidates
  are found once per surface.
- **Found on the way:** `sortTailStable` compared elements passed by value to a key taking a
  pointer, two heap copies of an entity a comparison: half the allocations of a mesh
  regeneration (`BenchmarkRegenerateMesh`, 43k → 22k, 27.8 → 19.8 MB). Same order.

Not taken:

- **7, arenas for the BSP's nodes.** Built (chunks of nodes in the insertion's scratch) and
  measured on `BenchmarkRegenerateMesh`: allocations 30k → 22k, bytes +8%, time and
  instructions no better (169 M against 171 M). Reverted.
- **5, GL vertex buffers kept.** A GL frame of the ball mill is 3.6 ms, of which the meshes are
  under a tenth; the entities and their lines are the rest. `DisplayMesh` is rebuilt in place,
  so a buffer kept needs a generation threaded through `render.Canvas` to know when it is stale:
  not worth it at that share.
- **8, a closed-form start on cylinders.** The guess grid is about 6% of the thread on one
  goroutine; another start ends Newton elsewhere in the last bits, so goldens for that 6%.
- **15, GOGC.** `BenchmarkDrag150` at GOGC 100, 200, 400: 1.15, 1.10, 1.05 ms a step, paint
  6.9, 6.8, 6.4 ms, against a heap up to four times the live one for the session; a soft memory
  limit would be the way, and the binaries keep the default.
- **16, CDT as the mesher.** K4 meshes the faces the ear clipper gets wrong; as the default it
  costs 3 to 5 times the triangles (`doc/notes/k4-cdt-spike.md`), and no profile here has the
  triangulation in it.
- **17, robust predicates for the mesh BSP.** For robustness, with no failing case to fix and no
  cost in a profile.
- **18.** `GOAMD64=v3` measured: 91 of 296 fixtures regenerate differently (fused multiply-adds,
  `pkg/libm` included) and the thread regenerates in 0.13–0.14 s either way. The tiled
  rasteriser, SIMD and a BVH remain goldens for gains the profiles don't point at.

## Libraries and algorithms surveyed

- **Sparse linear algebra.** gonum's `mat` has a dense QR, O(n³) and slower than the sparse one
  at n = 800, and no sparse QR; SuiteSparseQR and CHOLMOD need cgo, which the kernel avoids;
  Quern (Bridson) is C, Givens-based and not rank-revealing. The fill here is a property of the
  symbolic analysis, not of the arithmetic, and the fix is a permutation (1, 2). The
  `simdblas` package (AVX-512/NEON BLAS behind `blas64.Use`, Go 1.25+) only matters for dense
  kernels, which the solver has none of. Verdict unchanged: no library.
- **Geometric constraint decomposition.** Dulmage–Mendelsohn and the matching-based plans of
  Serrano, Hoffmann–Lomonosov–Sitharam, and the tree decompositions of the graph-based
  solvers: the structural part (a maximum bipartite matching, then strongly connected
  components in topological order) is exactly what 1 and 12 need, and it is linear in the
  matching. The first study's SolveSpace heritage has none of it. FreeCAD's "improve solving
  while dragging" is a reminder that a different method during a drag can also cost time on
  large sketches. DeltaBlue-style incremental solvers exploit that a drag changes the solution
  locally; that is 13.
- **Ray–surface intersection.** Bézier clipping converges in about half the iterations of
  midpoint subdivision but each costs more and needs rotations that add numerical error; the
  geometric-interval method of the 2007 BYU thesis is reported 25% faster than clipping on
  ray–surface tests. Our subdivision now runs over a tree built once per Boolean (P1), which is
  where its repeated cost was; the leaf test and the order of the results are those of the loop
  it replaced, so the goldens hold. A clipping method would change which point is found and in
  what order: goldens, and nothing measured here says it would win.
- **Robust predicates.** Shewchuk's adaptive `orient2d`/`orient3d`/`insphere` have ports in
  JavaScript (`robust-predicates`), Rust (`geometry-predicates`, `robust`) and one Go package
  on Gitee (`gitee.com/toy80/predicates`); none is a dependency to take over an in-tree port of
  a few hundred lines (17).
- **Mesh Booleans.** The pure-Go options (`reactivego/csg`, `vibrantgio/csg`) are the same BSP
  algorithm this kernel has, without its parity. Manifold has no Go bindings; `boolmesh` is a
  Rust reimplementation of it. A different mesh Boolean gives a different mesh: goldens, and
  the mesh Boolean is the fallback here, not the main path.
- **B-rep kernels.** No pure-Go B-rep or NURBS Boolean kernel exists; the current ones in that
  space are Rust (`brepkit`, `BREP_kernel`, both 2026) and OCCT in C++. Not an option for this
  project, which is its own kernel by decision (`doc/adr/0001`).
- **SIMD in Go.** `simd/archsimd` is behind `GOEXPERIMENT=simd` in Go 1.26 and 1.27, with a
  portable size-agnostic package added in 1.27 and breaking changes still landing. Nothing in
  the profiles is a wide loop over contiguous floats; the Bernstein sums are at most 36 terms
  and their order is what the goldens record. Revisit with D5.
- **FMA.** The Go compiler may fuse `x*y+z` where the target guarantees FMA (arm64, and amd64
  at `GOAMD64=v3`), and applications cannot assume the same results across architectures; the
  OS skip lists already carry an arm64 FMA case. P9 stays a measurement, and the fix for
  reproducibility, where it is wanted, is explicit `float64()` conversions.
- **Go 1.27.** Size-specialised allocation routines cut small allocations by up to 30%; the
  kernel's and the renderer's allocation counts above are what that applies to. The Green Tea
  collector is on by default since 1.26.

## Reproducing

```sh
make build
./scripts/perf-fixture-hashes.sh > after.txt       # diff against devel's
go test -run '^$' -bench . -benchmem ./pkg/solver
go test -run '^$' -bench 'Drag|Paint|Frame' ./pkg/ui
(cd pkg/persist/slvs && go test -run '^$' -bench Regenerate -benchmem .)
go test -run '^$' -bench 'Fillet|Shell|Loft|Sweep' ./pkg/sketch ./pkg/topo
TELESAPR_BENCH_MODEL=/path/to/model.tlspr go test -run '^$' -bench RegenerateModel ./pkg/persist/tlspr
```

The notes under `doc/notes/perf-core-study/` have the benchstat method, the scaled models and
the profiles for each part.

Sources for the survey: [Go SIMD blog](https://go.dev/blog/archsimd), [tracking issue
73787](https://github.com/golang/go/issues/73787), [Go 1.27 release
notes](https://go.dev/doc/go1.27), [Quern](https://www.cs.ubc.ca/~rbridson/quern),
[simdblas](https://pkg.go.dev/github.com/sebishogun/simdblas), [A review on geometric constraint
solving](https://arxiv.org/pdf/2202.13795), [Hoffmann, geometric constraint
decomposition](https://www.cs.purdue.edu/cgvlab/www/resources/papers/Hoffmann-Geometric_constraint_solving_and_Applications-1998-Geometric_constraint_decomposition.pdf),
[DeltaBlue](https://sites.cc.gatech.edu/classes/AY2002/cs4470_fall/p54-freeman-benson.pdf),
[ray tracing subdivision surfaces, Bézier clipping vs subdivision](https://arxiv.org/pdf/1811.03510),
[BYU geometric intervals thesis](https://scholarsarchive.byu.edu/etd/1207/),
[robust-predicates](https://www.npmjs.com/package/robust-predicates),
[toy80/predicates](https://pkg.go.dev/gitee.com/toy80/predicates),
[boolmesh](https://docs.rs/crate/boolmesh/latest), [brepkit](https://docs.rs/crate/brepkit-io/3.0.0),
[Go FMA discussion](https://golang.org/issue/52293).
