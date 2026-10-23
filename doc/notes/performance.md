# Performance, October 2026

Where TeleSAPR spends its time, what was changed, what was measured and left, and how to measure
again. It compacts two studies and the work that followed them:

- **The first study** (branch `perf-study`, 2026-10-10): the command line against solvespace-cli.
  Its work packages are P1 to P9 below.
- **The second study** (branch `perf-core-study`, 2026-10-20): the solver, the kernel's Booleans
  and the drawing of frames, one track each. Its recommendations are 1 to 18 below. The tracks'
  letters (S, K, D) refer to their own notes.
- **The work that took the recommendations up** (branches `wp-d4-incremental`, `lo-perf` and
  `lo2-perf`, 2026-10-22).

The full notes, with every profile, benchstat table and commit list, are in git:
`git show 39142006:doc/notes/performance-study.md`, `…:doc/notes/performance-study-2.md` and
`…:doc/notes/perf-core-study/{solver,kernel,drawing}.md`.

## Where it stands

- **Nothing a model makes changed without a commit of its own saying so.** Each change was held
  to `scripts/perf-fixture-hashes.sh` (every `.tlspr` fixture regenerated, and its exports, by
  sha256) and to the renderers' PNG and frame goldens. Three changes moved goldens on purpose:
  the components (12), the rows matched (1, 2), each by at most 1.2e-14 in params.
- **Against SolveSpace**, every model of the original benchmark corpus is faster: 0.66 to 0.89
  times solvespace-cli's time, against 1.8 to 25.7 times before. Most of the old gap was not the
  kernel: inflating Unifont for a text window a command-line run never shows, building the solids
  twice, and reading 1184 system font files to find one that wasn't there. At scale the exact
  Boolean grows with the number of copies to the power 1.2, SolveSpace's to 1.8: a 16x16 step and
  repeat takes 264 ms against 2.15 s.
- **The solver.** A rank test that stops where the rank cannot grow, the symbolic analysis kept
  across a solve's factorisations, the rows matched to the columns before the sparse QR (the fill
  was an artefact of Eigen's row order: the Newton step of a 400-chain 28.4 → 0.29 ms), the system
  kept between solves of a group, connected components solved apart, and a drag's steps taken with
  a kept factorisation.
- **The kernel.** Bounding-volume trees over a shell's surfaces, the ray-cast subdivision built
  once per Boolean, guess grids kept through the topology and the rolling ball, the mesh BSP
  without an allocation per node, planes and trims of coplanar crossings kept per surface.
- **Regeneration.** One pass where the model's size keeps the tolerance (the second pass for the
  size was half a sketch's regeneration in the window), and solids kept by what they were made
  from (`pkg/sketch/solid_cache.go`): the rover regenerated with nothing changed 1.01 → 0.13 s,
  an undo and redo of the triple M12 thread 267 → 47 ms.
- **Frames.** A software frame 27–56% faster (lazy clears, bands, one colour conversion per
  triangle, rows tested at their ends, hover restoring only the rows it touched); OpenGL 13–50%
  faster with one `glDrawArrays` per run in place of a cgo call per vertex.

Where the time goes now: a model's solids (the rover's 33 G instructions are its solids'), the
two rank tests of a solve during a drag, and in the kernel `ClosestPointToUV`, the ray tree's
walk and the serial quarter of a Boolean (the loops' commits, `fuseVertices`, merge classes, the
top of the union tree). The parallel speedup on 32 threads is about 3.2x.

## Every recommendation and what became of it

"Same bytes": held by the hash script and the goldens. "Goldens": fixtures regenerate otherwise,
and the goldens go in a commit of their own.

### The first study

| # | Item | Outcome |
|---|---|---|
| P1 | The subdivision of a surface for ray casts kept | Done in the second study (`srf/raycast_tree.go`): thread −12% |
| P2 | The solver's equations and Jacobian kept; a flat tape for `Expr.Eval`; components | Done as 6 and 12 below. The tape measured 25 → 18 µs for a whole Jacobian, a prototype in a benchmark only |
| P3 | Mesh BSP without a util per node | Done (`bspScratch`): 8x8 mesh union −25%, allocations −76% |
| P4 | Skip the regeneration's pass for the model's size | Done as 3 below |
| P5 | `.tlspr` read and written as streams, numbers parsed in place | Open; decoding was 15% of a mesh model's allocations |
| P6 | Sketch bookkeeping (sorts, map rebuilds, linear lookups) | In part: the curve-id scan (11), a tail sort copying entities (half a mesh regeneration's allocations) |
| P7 | CDT of trimmed faces | As 16 below |
| P8 | Robust predicates in the mesh BSP | As 17 below |
| P9 | `GOAMD64=v3` | As 18 below |

### The second study

| # | Item | Outcome |
|---|---|---|
| 1 | Rows of the Jacobian matched to columns before the sparse QR (S1) | Done (`solver/rowmatch.go`). Goldens, 24 files by numbers only. Closed 400-chain solve 260 M → 51 M instructions |
| 2 | A rank test that is not a full QR (S2) | Done with 1: the rank test factors A' where A is wide or square. Square 400-chain 988 M → 3.5 M instructions |
| 3 | One regeneration pass for the model's size | Done (`passOne`, `generate_all.go`). Same bytes. `BenchmarkDrag20` −45% |
| 4 | Software rasteriser inner loop (D1) | Done: depth before colour, row ends tested. Same pixels. Bolt frame −15% instructions |
| 5 | OpenGL vertex buffers kept (D2, D3) | Not taken: meshes are under a tenth of a GL frame; a buffer kept needs a generation threaded through `render.Canvas` |
| 6 | The Jacobian kept between solves of a group (S3) | Done (`solver_cache.go`, `jacobian_memo.go`). Same bytes. A group with nothing left after substitution keeps no system: the collector scanning kept systems cost +23% on a drag |
| 7 | Arenas for the BSP's nodes (K1) | Built and reverted: allocations −27%, bytes +8%, time no better |
| 8 | Closed-form closest points on cylinders (K2) | Not taken: 6% of the thread, goldens |
| 9 | Draw the last solids while a drag's are made off the thread (D4) | Behind a setting, off (`DragSolidsLate`). Off, a drag makes its solids on the thread so linked parts follow the pointer (151fda16) |
| 10 | Coplanar trim crossings bucketed (K3) | Done per surface split against, in the shell's index: 12x12 −16% |
| 11 | Curve ids reserved in advance (K4) | In part: the scan for the next id is gone, same bytes. The loops' commits stay serial (2% of the 12x12 on one goroutine) |
| 12 | Connected components solved apart (S4) | Done (`components.go`), a component's system taken from the whole Jacobian. Goldens by 1.3e-14 |
| 13 | A drag's steps with a kept factorisation (S5) | Done (`drag_step.go`), a chord method while each step halves what is left. No golden moved |
| 14 | The hover frame restores only its rows (D7) | Done (`rowstate.go`): `PaintHover` 7.8 → 7.3 ms |
| 15 | GOGC in the window | Not taken: 1.15 → 1.05 ms a drag step at GOGC 400, against a heap up to four times the live one for a session. `telesapr-cli` runs at 400 unless `GOGC` is set (first study: −14–19%, peak memory +20–65%) |
| 16 | CDT as the mesher (K5) | Not taken as the default: 3 to 5 times the triangles (`k4-cdt-spike.md`); it mends the faces the ear clipper gets wrong |
| 17 | Robust predicates for the mesh BSP (K6) | Not taken: no failing case, no cost in a profile |
| 18 | Tiled rasteriser, SIMD, `GOAMD64=v3`, a BVH for mesh Booleans | Not taken. `v3` measured: 91 of 296 fixtures regenerate otherwise (FMA, `pkg/libm` included), the thread 0.13–0.14 s either way |

Not in either list: a text-window benchmark (D8) is worth adding before any work on the text
window, which paints a rectangle and a pixmap per character cell.

## Measured and not kept

- PGO from the corpus's profiles: −7% to +4% by model, same bytes.
- A `sync.Pool` of `bspUtil`: −13%, but a package-level variable written at run time; `bspScratch`
  has the gain within the rules.
- Clearing the factorisation's `mark` and `tval` only where touched: they are `memclr`s, and
  `tval` is touched outside the pattern.
- Incremental edge functions in the rasteriser: the edge pixels' bits change.
- The ray tree with control nets remade from the root: no gain.
- Pre-sizing trim edge lists with `slices.Grow`: time unchanged, bytes −12% and +19%.
- benchstat in `go.mod`'s tool block: it pulled a day-old `x/perf` and raised `x/sys`. Install it
  with `go install golang.org/x/perf/cmd/benchstat@latest` instead.

## Libraries surveyed

None is recommended. The time is in pointer-chasing geometry, symbolic bookkeeping and allocation,
not dense arithmetic.

- Sparse linear algebra: gonum has no sparse QR; SuiteSparseQR and CHOLMOD need cgo; the fill was
  a permutation's to fix (1).
- Constraint decomposition (Dulmage–Mendelsohn, matching-based plans): the structural part is
  what 1 and 12 took.
- Ray–surface intersection: Bézier clipping or geometric intervals would change which point is
  found first, so goldens, with nothing measured saying they would win.
- Robust predicates: Shewchuk's are a few hundred lines to port in-tree if ever needed (17).
- Mesh Booleans: the pure-Go ones are the same BSP; Manifold has no Go bindings. No pure-Go B-rep
  kernel exists, and this project is its own kernel.
- SIMD: `simd/archsimd` is still behind `GOEXPERIMENT` in Go 1.27, and no profile has a wide loop
  over contiguous floats. FMA is fused by the compiler on arm64 and at `GOAMD64=v3`, which the OS
  skip lists already meet.

## Traps

- Two test binaries run one after the other differ by up to 13% from noise: run them
  interleaved, six times each, and claim only what benchstat marks significant. On a shared host,
  count instructions (`perf stat -e instructions:u`, a benchmark at two counts, the difference).
- Don't take `--memprofile` with `--cpuprofile`: its finer sampling is a third of the CPU profile.
- A solid cache key must leave out what a group keeps from an earlier state that nothing reads
  (the profile's error point once good, empty slices by their array), or the same inputs get
  another key. A group with no solid passes one key down whatever its tolerance.
- A test reading a script's sketch through `Doc.With` records an undo step and dirties the active
  group: read through the host.
- Five fixtures fail to regenerate from the command line before and after alike (a missing IDF/STL
  link, two images, the v1 `fields` file).

## Reproducing

```sh
make build
./scripts/perf-fixture-hashes.sh > before.txt         # and after a change: diff
go test -run '^$' -bench . -benchmem ./pkg/solver
go test -run '^$' -bench 'Drag|Paint|Frame' ./pkg/ui
(cd pkg/persist/slvs && go test -run '^$' -bench Regenerate -benchmem -count 6 .)
go test -run '^$' -bench 'Fillet|Shell|Loft|Sweep' ./pkg/sketch ./pkg/topo
TELESAPR_BENCH_MODEL=/path/to/model.tlspr go test -run '^$' -bench RegenerateModel ./pkg/persist/tlspr
bin/telesapr-cli regenerate --cpuprofile cpu.out model.tlspr   # or --trace trace.out
```

The scaled step-and-repeat models are `testdata/group/translate_nd/normal.slvs` with both
`Group.valA` (lines 212 and 377) set to n and each `Group.meshCombine` set to 0
(`Group.forceToMesh=1` after each for the mesh variant), converted with `telesapr-cli convert`.
