# Performance

How to measure TeleSAPR's speed, where the time goes, what was decided and why, and what is open.
Refreshed 2026-11-03 at `c4e18d1a`.

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
- CDT as the mesher on every face: 1.43 times the ear clipper's triangles and 1.64 times its
  regeneration time with the chart scaled by curvature, 2.00 and 1.85 by speed, over the 84 solid
  and thread goldens (`BenchmarkMeshers`, 2026-11-04; `k4-cdt-spike.md`). It mends the faces the
  ear clipper gets wrong.
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
- One fixture fails in the hash script, before and after alike: `tlspr/v1/fields.tlspr`, whose
  linked file is not in `testdata/`.
