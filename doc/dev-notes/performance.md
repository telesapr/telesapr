# Performance

How to measure TeleSAPR's speed, where the time goes, what was decided and why, and what is open.
Refreshed 2026-11-03 at `c4e18d1a`; the rolling ball's feet 2026-11-17, its mitred chains 2026-11-18, putting its blends in
and checking the result 2026-11-18, a fillet made once where it was made twice 2026-11-18. Refreshed
2026-11-18 (branch `chamfer-nondeterminism`): a map's order in the kernel (Traps); a corner hole's patches solved faster 2026-11-18.

Two studies came before this note: the command line against solvespace-cli (branch `perf-study`,
2026-10-10, items P1 to P9) and the solver, the kernel's Booleans and the drawing of frames
(branch `perf-core-study`, 2026-10-20, recommendations 1 to 18). Most of what they proposed is
done. Their reports, with every profile and benchstat table, are in git:
`git show 39142006:doc/notes/performance-study.md`, `…:doc/notes/performance-study-2.md` and
`…:doc/notes/perf-core-study/{solver,kernel,drawing}.md`. This note's text before the refresh is
`git show c4e18d1a:doc/dev-notes/performance.md`.

## Measuring

```sh
make build                                            # bin/telesapr-cli, which the script runs
./scripts/perf-fixture-hashes.sh > before.txt         # after the change: again, and diff
go test -run '^$' -bench . -benchmem ./pkg/solver
go test -run '^$' -bench 'Regenerate' -benchmem -count 6 ./pkg/sketch
go test -run '^$' -bench 'Fillet|Shell|Loft|Sweep|Chamfer' ./pkg/sketch ./pkg/topo
go test -run '^$' -bench 'EditBack|UndoRedo' ./pkg/persist/tlspr
go test -run '^$' -bench 'Drag|Paint|Frame|Hover' ./pkg/ui
go test -run '^$' -bench Soft ./pkg/render/soft
TELESAPR_BENCH_MODEL=/path/to/model.tlspr go test -run '^$' -bench RegenerateModel ./pkg/persist/tlspr
cp model.tlspr /tmp/m.tlspr                           # regenerate writes the file back
bin/telesapr-cli regenerate --cpuprofile cpu.out /tmp/m.tlspr
```

- `scripts/perf-fixture-hashes.sh` regenerates a copy of every `.tlspr` under `testdata/` (312
  today, about 6 s) and prints the sha256 of each result. A change meant only to be faster leaves
  the list as it was. `TELESAPR_CLI` names another binary. The renderers' PNG and frame goldens
  hold the pixels.
- Every `telesapr-cli` command takes `--cpuprofile`, `--memprofile`, `--trace` (regions for
  loading, solving, solids and saving) and `--profile` (a JSON summary: time per model, and per
  group for solving, loops and the solid).
- `make bench` runs every benchmark once, to keep them working; the nightly workflow runs it. It
  measures nothing.
- `pkg/sketch`'s `BenchmarkRegenerate{Text,Translate,Fillet,Mesh}` run each model with
  `TELESAPR_PARALLEL` off and on, and with the solid cache off, so the kernel is what is timed.
  `BenchmarkEditBack` and `BenchmarkUndoRedo` time an edit with the cache (`made` and `kept`).
  `Sketch.NoSolidCache` and `Sketch.NoSolverCache` turn the caches off, to measure them.
- benchstat: `go install golang.org/x/perf/cmd/benchstat@latest`. It is not in `go.mod`'s tool
  block: there it pulled a day-old `x/perf` and raised `x/sys`.
- A scaled step and repeat for `TELESAPR_BENCH_MODEL`: copy
  `testdata/tlspr/golden/group/translate_nd/normal.tlspr`, set both `copies = 3` (groups 0x6 and
  0x7) to n, and delete the three `combine difference` lines, so every group is a union. For the
  mesh variant put `force-to-mesh` in their place. n = 16 regenerates in about 0.25 s.

## Where the time goes

- The slowest fixtures are the threads (`thread/face_m8` 0.24 s, `bolt_m8x30` 0.19 s,
  `triple_m12` 0.17 s from the command line) and the ring lofts (`solid/loft-ring-guide*`
  0.16 s). Most fixtures take under 0.1 s.
- In a thread, closest points (`ClosestPointToUV`) are half the profile. Most of their Newton
  steps are in `closestPointRestarts`, and nearly all those calls fail: the points a Boolean
  projects (a trim's chord points in `makeCopyTrimAgainst`, `offPatch` and `onSurface` in the
  march) lie further than `ratPolyEps` off the surface, so each tries all its starts. Since
  2026-10-30 a refinement stops when its steps repeat (`closestPointNewton`), which cut the
  steps to 27%; P10 and P11 are what is left.
- Elsewhere in the kernel: the ray tree's walk and the serial part of a Boolean (`fuseVertices`,
  merge classes, the top of the union tree). The parallel speedup on 32 threads was about 3.2x
  (2026-10-20).
- In the window: a model's solids, and the two rank tests of a solve during a drag.
- In a rolling-ball fillet or chamfer, finding the ball's feet (`footNear`, each a
  `nearestOnWithin` on a face either side) was 89% of the time on a pen barrel's 40 band top
  edges (2026-11-17), whose sides are up to 9 patches each. Since then a face whose patch's box
  lies further off than a foot already found within its patch is not projected on, the face
  last found tried first, and the foot is chosen among the rest as before, so the solid is the
  same to the bit: that chamfer 155.8 s to 22.1 s, `BenchmarkFilletWrappedArch` 14.9 s to 4.8 s.
  Since then too the groups of chains a fillet's mitres join, which share nothing but the faces
  they project on, are traced and blended at once, each group's chains in order, with the
  faces' closest-point caches off (`SShell.ProjectIndependently`) so that no projection
  depends on another group's: the barrel's four groups of six chains, 23.0 s to 12.0 s, the
  saved file the same to the byte. Left of those 12 s: the check that the solid is closed
  (`unsound`, 4 s, a mesh of the whole shell), putting the blends in (2.4 s, a linear
  `FindSurface` among them) and the four groups' blends (4 s, one goroutine each). Since
  2026-11-18 the chains of one group are blended side by side as well, every mitre fitted first
  (`blendChainsAtOnce`, `doc/dev-notes/rolling-ball-corners.md`): the four groups' blends 3.6 s
  to 1.6 s, the regeneration about 12 s to 10 s, `BenchmarkFilletWrappedArch` 5.0 s to 2.9 s,
  the same solids to the bit. Refreshed 2026-11-18 (branch `wrapped-put-speed`): what was left
  on one core is on all of them now, or less work, the same solids to the bit. Building a
  topology (`topo.Build`, five times over in a fillet: putting the blends in, the open edges they
  join, the check, the creases turned back, before and after) was three quarters the seam test
  (`smoothSeam`, three closest points on each surface of every edge between patches of one
  sheet, thousands in a fillet's result): the edges are split into groups whose surfaces no
  other group's edges touch and each group told in order on a goroutine (`Topology.seams`),
  and each surface's loops chained and projected at once (`surfaceLoops`). A projection moves
  where the surface's next one starts (`SSurface.Cached`), so every surface is projected on by
  one goroutine in the order one loop would. The check's search for naked and crossing edges
  (`SKdNode.MakeCertainEdgesInto`) runs in chunks of triangles at once (`badEdgesInto`), each
  search marking what it saw in a slice of its own, not in the triangles' tags; the check and
  the look for creases turned back run side by side (`FilletPicksMade`); the blends' patches
  find the open edge they share by its ends, filed by cell (`FaceAdder.along`), and a mitre's
  corners are looked for only on sides whose control points' box holds them
  (`splitAtPoints`). The arches' feet 13 s to 5 to 6 s, the barrel's regeneration with its
  chamfer 9.8 s to 6.2 s, `BenchmarkFilletWrappedArch` 2.9 s to 1.75 s. Left on one core, a few
  tenths of a second each per fillet there: cutting the faces back (`cutBack`, the ball's
  sections), the patches put in (`FaceAdder.Add`) and the check's kd-tree and Euler count.
  Refreshed 2026-11-18 again (branch `fillet-twice-speed`): "FilletSolid makes the fillet twice" was
  so, once per fillet or chamfer added and once per change of its edges, not per regeneration
  (a counter in `topo.FilletPicksMade`: one call adding the arch's foot's fillet, one more at
  the first read, one per regeneration). `NewFillet`, `NewChamfer`, `SetFilletEdges` and
  `SetChamferEdges` try the edit on a copy of the solid so that an edge it can't round is an
  error then (`filletTrial`, `chamferTrial`), and the regeneration that follows made the same
  solid again. The trial now keeps what it made under a key of all it was made from: every field
  of the fresh copy of the solid so far but its pace, the picks, the size, the origin and whether
  it is flat (`pkg/sketch/edit_trial.go`). It is made at the origin the group will have
  (`Sketch.nextGroupHandle`, the handle `AddGroup` gives), where it used a placeholder, and the
  group's regeneration takes it when its own key is the same, else makes it as before. A
  ShellJob's copy of the sketch shares the trials as it shares the solid cache; `NoSolidCache`
  turns both off. `TestEditTrialTaken` compares each taken solid with one made afresh; while
  the work was done each taken solid was also checked against one made again in place, over the
  fillet and chamfer tests of `pkg/sketch` and `pkg/script/api`, and each result's digest
  compared with the code before: the same to the bit. `BenchmarkFilletWrappedArchAdd`, the
  arch's foot added, 3.4 s to 1.45 s. In the kernel, the check (`unsound`) meshes one copy of
  the solid while it builds the topology of another, and finds the mesh's naked and crossing
  edges and its Euler count side by side; its topology is built from the solid's own closest-
  point guesses, not those its mesh left, which tell it nothing else. A kd-tree's two sides are
  built at once above 4096 triangles (`skdNodeFromList`, the same tree), and a fillet's chains
  are traced all at once, not each group's in order (`eachChain`: a chain is traced from its own
  edges and faces, its projections independent of order). `BenchmarkFilletWrappedArch` 1.78 s to
  1.48 s; the digests of every result of the fillet and chamfer tests of `pkg/topo` (1538),
  `pkg/sketch` and `pkg/script/api` are those before, but for `TestChamferConeAsWedges`,
  `TestChamferConeSweep` and `TestChamferCornerOracle`, whose results differ from run to run
  before as after. Left on one core, about 0.1 s of the arch's 1.5 s: cutting the faces back
  (`cutBack`) and the patches put in (`FaceAdder.Add`), which edit the shell in turn and whose
  projections each start where the one before ended (`SSurface.Cached`). The rest is the
  critical path of a group's blend: the first chain fits both its mitres in turn (0.5 s), then
  the slowest chain blends (0.4 s). The two fits of one chain share its last foot found
  (`rollingBall.lastFoot`, which only orders the search) and its faces' (u, v) loops
  (`uvLoops`, filled on first use), so they would race as they are; starting each chain's blend
  as soon as the fits it reads are done gains nothing on the arch, whose last chain reads the
  first chain's fit.
- Refreshed 2026-11-18 (branch `chamfer-apex-speed`): a corner's hole filled with quintic patches
  (`setbackPatches`, `blendPatch`) is a least-squares solve of some 340 unknowns and 1700 rows of
  a few dozen terms each (`leastSquaresDense`). The `apexsome 1.5 0.5 0 0` line of
  `testdata/topo/chamfer-corners.txt` took 1.3 s, the other lines milliseconds: its corner is
  first tried as a saddle beside a sharp edge (`saddleSideAt`), 6 splits of the hole times 3
  legs of its face curve, each laid out from every corner of its 8 sides, and every one of the 18
  is refused only once solved (12 not tangent to what is round them, 6 folding), before the
  corner is made another way. The search stays; each solve is cheaper, the same to the bit: the
  normal equations are summed over each row's own terms, in the order a sweep of the whole row
  added them; the elimination subtracts the pivot row's nonzero entries only (no entry is ever
  -0, so subtracting f times a zero changes nothing); and the thin-plate rows, alike for every
  patch, are a table computed once (`thinPlate`), not Bernstein polynomials and maps per patch.
  That line 1.30 s to 0.24 s; every line's surfaces the same to the bit, and the fixtures' hashes.
  On the same branch, a fillet's result had its topology built twice, by the check that it is
  closed (`unsound`) and by the look for creases turned back (`turnedBackPicks`), each from a
  copy of the same shell, so the same topology: the check now hands back what it built
  (`unsoundBuilt`) and builds the solid before's beside its mesh, and the look runs after it, only
  where the result is sound. Four builds where there were five; the two ran side by side, so on
  32 idle cores `BenchmarkFilletWrappedArch` is 1.49 s as before (five runs each, medians 1.490 s
  and 1.484 s, `BenchmarkFilletWrappedArchAdd` 1.482 s and 1.478 s), and the processor time
  about 8% less; on 4 cores (`-cpu 4`, three runs each) 2.04 to 2.13 s to 1.89 to 1.90 s, and
  the Add 2.04 to 2.08 s to 1.85 to 1.89 s. The fixtures' hashes are those before.

## Open

Numbered as before, so references stay good; P10 to P12 are new numbers for items that had none.
"Same bytes": the hash script and the goldens hold. "Goldens": fixtures regenerate otherwise,
and the goldens go in a commit of their own.

| # | Item | State |
|---|---|---|
| P5 | `.tlspr` read and written as streams, numbers parsed in place | Open. `tlspr.LoadFile` reads the whole file; decoding was 15% of a mesh model's allocations. Parked in ROADMAP: no user has waited on it |
| P6 | Sketch bookkeeping (sorts, map rebuilds, linear lookups) | In part: a regeneration sorts only what each group adds, by keys; the curve-id scan is gone. The rest is unmeasured |
| P10 | A closest-point restart only where the first refinement did not settle | Open, goldens. A point off the surface settles where `p - S(u, v)` is square to both tangents; a stall at a fold's edge does not. Most of the remaining steps |
| P11 | `offPatch` and `onSurface` reject a point far from the surface's control-point box before searching | Open, same bytes: a failed search leaves `Cached` alone. How many points it catches is not measured |
| P12 | A text-window benchmark | Worth adding before any work on the text window, which paints a rectangle and a pixmap per character cell |

- **The CDT at fine chord tolerances** (2026-11-04): with it the default, tests at 1e-4 to
  0.00085 mm took 7 to 18 times as long and `make ci` timed out; a budget gives the faces it
  would fill with more than 24 times the ear clipper's grid back to the ear clipper. Since
  2026-11-05 the sag is measured square to the triangles and `FaceAt` keeps each loop's outline
  in its surface's parameters and passes faces whose control points lie farther than one found:
  the wrap round 70 s to 28 (21 under the ear clipper), the drafted boss's shell 15 s to 3.7
  (`doc/dev-notes/k4-cdt-spike.md`, "Fine tolerances").

## Decided against, and why

- OpenGL vertex buffers kept between frames: meshes are under a tenth of a GL frame, and a kept
  buffer needs a generation threaded through `render.Canvas`.
- Arenas for the mesh BSP's nodes: allocations −27%, bytes +8%, no faster. `bspScratch` keeps
  the gain within the rules; a `sync.Pool` of `bspUtil` would be package state written at run
  time.
- Closed-form closest points on cylinders: 6% of a thread, and goldens.
- A drag's solids made off the thread and drawn late: kept behind a setting, off
  (`DragSolidsLate`). Off, a drag makes its solids on the thread, so linked parts follow the
  pointer.
- The loops' curve commits in parallel (recommendation 11): `curveSink.commitTo` and its id calls
  are 0.15% of the 12x12 step and repeat's samples (2026-11-02), and an exact curve's look-up
  among those before it needs them committed first.
- GOGC in the window: 400 saved 0.1 ms a drag step for a heap up to four times the live one.
  `telesapr-cli` runs at 400 unless `GOGC` is set (`gcPercent`): a run lives for one model, and
  it is 15–20% faster for more peak memory.
- The CDT as the mesher of every curved face, the default since 2026-11-04: 1.44 times the ear
  clipper's triangles over the 87 solid and thread goldens, their regeneration 0.99 times its
  time, one model at a time (`BenchmarkMeshers`; `k4-cdt-spike.md`). A run earlier that day, under
  other load and before the changes of the switch, measured 1.43 and 1.64.
- Robust predicates in the mesh BSP: no failing case, no cost in a profile. Shewchuk's are a few
  hundred lines to port if ever needed.
- `GOAMD64=v3`: 91 of 296 fixtures regenerated otherwise (FMA, `pkg/libm` included), with the
  thread no faster. Tiled rasteriser, SIMD (`simd/archsimd` is behind `GOEXPERIMENT` in Go 1.27,
  and no profile has a wide loop over contiguous floats), a BVH for mesh Booleans: not taken.
- PGO from the corpus's profiles: −7% to +4% by model.
- Tried and no gain, or bits changed: clearing the factorisation's `mark` and `tval` only where
  touched (they are `memclr`s, and `tval` is touched outside the pattern); incremental edge
  functions in the rasteriser (edge pixels change); the ray tree's control nets remade from the
  root; trim edge lists pre-sized with `slices.Grow`.
- A flat tape for `Expr.Eval`: 25 → 18 µs for a whole Jacobian, kept only as a benchmark
  (`pkg/solver/tape_bench_test.go`). The rest of P2 was done as the Jacobian kept between
  solves (`solver/jacobian_memo.go`) and components solved apart.
- Libraries: none recommended. The time is in pointer-chasing geometry, symbolic bookkeeping and
  allocation, not dense arithmetic. gonum has no sparse QR, SuiteSparseQR and CHOLMOD need cgo,
  and the QR's fill was fixed by matching rows to columns (`solver/rowmatch.go`). Bézier
  clipping for ray casts would change which point is found first. Pure-Go mesh Booleans are the
  same BSP; Manifold has no Go bindings.

## Traps

- Two test binaries run one after the other differ by up to 13% from noise: run them
  interleaved, six times each, and claim only what benchstat marks significant. On a shared host,
  count instructions (`perf stat -e instructions:u`, a benchmark at two counts, the difference).
- Don't take `--memprofile` with `--cpuprofile`: its finer sampling is a third of the CPU profile.
- `telesapr-cli regenerate` writes the model back over its input. Profile a copy, never a user's
  design or a fixture in the tree.
- A solid cache key (`pkg/sketch/solid_cache.go`) must leave out what a group keeps from an
  earlier state that nothing reads (the profile's error point once good, empty slices by their
  array), or the same inputs get another key. A group with no solid passes one key down whatever
  its tolerance.
- A group with nothing left after substitution keeps no solver system: the collector scanning
  kept systems cost 23% on a drag.
- A test reading a script's sketch through `Doc.With` records an undo step and dirties the active
  group: read through the host.
- A loop over a map whose order reaches the arithmetic gives a solid that differs from run to
  run, with the parallel mode off too, since Go changes a map's order every time. The chamfer
  corner cut from its cone (`chamferConeAt`, `pkg/topo/chamfer_cone.go`) clipped each face's
  wedge and each chamfer's strip by its planes in the order of two maps: the same polygon in exact
  arithmetic, other last bits in floating point, so one or more of four builds of 9 of the 198
  lines of `testdata/topo/chamfer-corners.txt` (all `apexsome` and `pocketsome`, an apex with edges
  left sharp) came out different. They are clipped in the order of the planes' keys now
  (`clipAll`); `TestChamferConeSameEveryRun` builds five of them six times at once. A hash of
  the shell (surfaces' control points and trims, curves' points, the mesh) over every line and
  the 234 points of the cone sweep's grid, three processes with the parallel mode on and two off,
  was the same each time afterwards, and the 189 lines that had not differed kept their hashes.
  The other loops over maps in `pkg/topo`, `pkg/srf` and `pkg/geom` fill sets, write each key's
  own entry, keep a maximum or sort what they gather, and were left.
- One fixture fails in the hash script, before and after alike: `tlspr/v1/fields.tlspr`, whose
  linked file is not in `testdata/`.
