# Performance study, October 2026

Where TeleSAPR spends its time regenerating and exporting models, what was changed on the branch
`perf-study`, and what to do next. It answers `bench-vs-solvespace.md` (2026-10-09). That
benchmark had TeleSAPR 2.5 times slower than SolveSpace on Booleans and 25 times slower on text.

## Summary

- **The 2.5x was mostly not the kernel.** Most of a `telesapr-cli` run on a small model went on
  three things:
  - inflating the bundled Unifont for a text window that a run without a window never shows
    (18 of 27 ms);
  - building the solids twice;
  - and, for text, reading every system font.
- **The text rows measured nothing.** `Gentium-R.ttf` is in no system font directory, and
  solvespace-cli reads no system fonts at all, so both tools drew the missing-font placeholder.
  TeleSAPR read 1184 font files, about 680 MB, to find that out.
- **TeleSAPR is now faster than solvespace-cli on every model of the original corpus**: 0.66 to 0.89
  times SolveSpace's time, against 1.8 to 25.7 times before.
- **At scale it is much faster.** A 16 by 16 step and repeat (a union of 256 holed plates) takes
  264 ms, or 461 ms on one goroutine, against SolveSpace's 2.15 s. TeleSAPR's time grows with the
  number of copies to the power 1.2; SolveSpace's to the power 1.8.
- **Nothing a model makes has changed.** Every fixture regenerates to the same bytes, and exports
  mesh, wireframe, view and STEP to the same bytes. No golden was touched.

## Method

- **Same output.** `scripts/perf-fixture-hashes.sh` regenerates a copy of each of the 276 `.tlspr`
  fixtures and prints the sha256 of each output.
  - Its output before and after each change was diffed.
  - Five fixtures fail to regenerate from the command line before and after alike (a missing
    IDF/STL link, two images, the v1 `fields` file).
  - The export commands were compared the same way, on every fixture and on the scaled corpus.
- **Kernel time.** The benchmarks in `pkg/persist/slvs` measure regeneration in the process:
  - `BenchmarkRegenerateTranslate` (a 12x12 step and repeat);
  - `BenchmarkRegenerateText`;
  - `BenchmarkRegenerateFillet`;
  - `BenchmarkRegenerateLargest` (the triple M12 thread).

  The test binaries before and after a change were run interleaved, six times each, and compared
  with benchstat (`go install golang.org/x/perf/cmd/benchstat@latest`; it is not in `go.mod`,
  because adding it there moved `x/sys`). Running the two binaries one after the other instead gave
  differences of up to 13% that were noise.
- **Process time.** Measured with hyperfine. The binaries were `bin/telesapr-cli` and
  `solvespace/build/bin/solvespace-cli` (Release, mimalloc), on a host with 32 threads.
- **Profiles.**
  - `telesapr-cli` now takes `--cpuprofile`, `--memprofile` and `--trace`, before or after any
    command.
  - The trace has regions for loading, both regeneration passes, each group's solve and solid, the
    mesh Boolean, the stages of the exact Boolean, the merge of coincident surfaces, and saving.
  - Don't take `--memprofile` together with `--cpuprofile`: the finer allocation sampling
    `--memprofile` sets shows up as a third of the CPU profile.

## The original corpus again

Median ms of `regenerate`; the ratio is TeleSAPR now over SolveSpace.

| Model | SolveSpace | TeleSAPR before | TeleSAPR now | Ratio |
|---|---:|---:|---:|---:|
| translate | 17.1 | 38.5 | 15.3 | 0.89 |
| translate_nd | 24.0 | 45.3 | 18.2 | 0.76 |
| boolean_coplanar_union | 14.8 | 34.5 | 11.3 | 0.76 |
| union | 12.1 | 32.3 | 9.8 | 0.80 |
| boolean_tangent_crossing | 13.5 | 32.1 | 8.9 | 0.66 |
| boolean_knife_edge | 12.3 | 30.8 | 8.8 | 0.72 |
| boolean_tangent_spline | 13.0 | 31.8 | 8.8 | 0.68 |
| boolean_tangent_edge | 11.2 | 29.5 | 8.1 | 0.72 |
| boolean_tangent_fillet | 11.5 | 30.5 | 8.6 | 0.75 |
| difference | 10.3 | 29.1 | 7.9 | 0.77 |
| translate_asy | 10.0 | 28.0 | 7.3 | 0.73 |
| ttf_extrude * | 9.1 | 219.9 | 6.0 | 0.66 |
| ttf_extrude_kerning * | 9.0 | 221.4 | 5.9 | 0.66 |

\* Both tools draw the missing-font placeholder; solvespace-cli cannot read a font file at all, so
text can't be compared with it from the command line.

Peak RSS is now 31 MB for `union`, against SolveSpace's 27 MB, and 24 MB for text, against 26 MB.

Three things in the original corpus mean it measures less than it seems to:

- **`union`, `difference` and `translate` are forced to a mesh.** They go through the triangle-mesh
  BSP Boolean, not the exact one.
- **`translate_nd` combines its copies as a difference.** Its final solid is the same whatever the
  number of copies.
- **`export-mesh` exports the active group**, which in these files is not the last one.

## Scaling

`translate_nd` was rewritten with `valA = n` on both step-and-repeat groups and `meshCombine = 0`
(union), giving n by n plates. The mesh variant adds `forceToMesh = 1`. For n = 4 and 16, both
tools' meshes have the same triangle count, volume and bounding box: 3000 triangles and 7400 mm³ at
16, or 3188 triangles for the mesh variant.

Exact Boolean, median ms ("now" is at 18a519d, before the GC setting and the trim-end index, which
take a further 15-20% off):

| n | SolveSpace | before, all goroutines | before, one | now, all | now, one |
|---:|---:|---:|---:|---:|---:|
| 2 | 16.2 | 37.9 | 37.6 | 15.8 | 15.3 |
| 4 | 37.8 | 52.9 | 63.1 | 26.6 | 29.0 |
| 8 | 188.7 | 113.3 | 249.2 | 68.0 | 88.8 |
| 12 | 734.2 | 250.0 | 876.0 | 134.2 | 215.5 |
| 16 | 2149 | 524.0 | 2518 | 264.2 | 460.6 |

Mesh Boolean (forceToMesh), median ms:

| n | SolveSpace | before | now |
|---:|---:|---:|---:|
| 4 | 19.7 | 42.5 | 21.5 |
| 8 | 42.7 | 73.8 | 46.7 |
| 16 | 138.7 | 193.3 | 146.7 |

The mesh Boolean is now on a par with SolveSpace; GC tuning, below, takes it under.

## What changed

Each line is one commit on `perf-study`. All of them leave every output byte-identical.

| Commit | Change | Effect |
|---|---|---|
| dfac9e0 | `scripts/perf-fixture-hashes.sh` | the check above |
| 8089e6c | profiling options, trace regions | — |
| 0a97294 | Unifont inflates on the first glyph drawn; `BitmapFont.Width` answers from a generated table of Unifont's wide ranges (`scripts/gen-unifont-widths.py`, checked for every code point) | CLI `union` 32.8 → 13.4 ms |
| c4932e1 | `FontList.LoadFont` opens only the files of the basename asked for | text 225 → 7 ms, with the next |
| 691ac0d | `regenerate` and `export-*` load with solids deferred, since they rebuild them at the export tolerance | `translate_nd` 44.7 → 21.4 ms |
| 6bd5152 | bounding-volume tree over a shell's surfaces while a Boolean classifies (`srf/surfindex.go`), for `ClassifyEdge`, `raycastClass` and `AllPointsIntersecting`; candidates go through the same box test, in order | 12x12: 391 → 188 ms on one goroutine |
| 51987b2 | `FindCurve` / `FindSurface` look at index h−1 first | 12x12: 187 → 126 ms; text −5% |
| 4d7bf07 | uv BSP insertion works out the tangent scale once per edge end | text 21.2 → 12.2 ms on one goroutine |
| 18a519d | closest-point guess grid evaluated once per Boolean loop (`srf/closest_guess.go`, `sync.Once` per surface) | thread 136 → 108 ms |
| 725ce48 | `telesapr-cli` runs at GOGC=400 unless `GOGC` is set | large models −14–19%, peak memory +20–65% (138 MB at most) |
| ef765ff | STEP export finds aliases through a grid of cells and maps | STEP of the 16x16: 793 → 455 ms |
| 9f13fb3 | trim ends sorted by x in the surface index, for `findVertsOnCurve` | 12x12: 123 → 103 ms on one goroutine |

Each index keeps the order and the exact test of the loop it replaces, and has a randomised test
holding it to that loop.

## Measured and not kept

- **PGO** (`default.pgo` from the corpus's profiles): −7% to +4% by model, no clear gain. It gave
  the same bytes, so it can be tried again when the profile is more representative.
- **A `sync.Pool` of the mesh BSP's `bspUtil`**: −13% on the 16x16 mesh model. But it is a
  package-level variable written at run time, which CLAUDE.md rules out. See P3 for a way within
  the rules.
- **Backing the BSP flags with a small array**: −2%. The struct still escapes to the heap, because
  its slices point into itself.
- **Adding benchstat to `go.mod`'s tool block**: it pulled a pseudo-version of `x/perf` a day old
  and raised `x/sys`. Install it with `go install` instead.

## Where the time goes now

- **12x12 step and repeat, 103 ms on one goroutine.**
  - Trim building in `makeCopyTrimAgainst` and `MakeEdgesInto`.
  - `AxisAlignedBounding` from the remaining per-surface loops.
  - `addAndAssignCurveID`, which scans for the maximum id on every add.
- **Thread, 108 ms on all goroutines.**
  - Ray–surface subdivision, `AllPointsIntersectingUntrimmed`: 39%. Every ray splits the same
    patches again.
  - `ClosestPointToUV` with Newton: 26%.
- **Fillet model, 2 ms.**
  - Solver: 23%, mostly `WriteJacobian`, which deep-copies every equation and takes symbolic
    derivatives on each solve.
  - Sorting the entity list twice per group.
  - GC.
- **Mesh Boolean.**
  - `SBsp3.Insert` makes 36% of all allocations: a `bspUtil` and five small slices at every node an
    insertion passes.
  - Decoding the `.tlspr` (`Parse`, `lexLine`) makes another 15%.
- **STEP export.** `HasBSplineCurveAnAlias` still scans every curve, with a match test that isn't
  set equality, so it can't simply be hashed.

## Recommended work packages

Ordered by expected gain over cost. "Same bytes" means the change can be held to the hash script;
"goldens" means it changes output, and the regenerated goldens then go in a commit of their own,
with `pkg/check` passing.

1. **P1: cache the subdivision of a surface for ray intersection.** Same bytes; about 2 days.
   - `AllPointsIntersectingUntrimmed` splits a surface in halves, alternating u and v, down to
     flat pieces, for every ray, and that is 39% of the thread.
   - The tree of pieces and their boxes depends only on the surface. Build it once per Boolean
     loop, as the guess grid is, and walk it for each ray.
   - Expected −25–35% on curved Booleans: threads, fillets, lathes.
2. **P2: keep the solver's equations and Jacobian between regenerations.** Same bytes if the
   symbolic forms are reused exactly; 1–2 weeks.
   - Today every solve deep-copies each equation, takes symbolic derivatives, folds constants
     and builds a dense `[][]*Expr` (`solver/system.go:187`).
   - Keyed on the group's constraint and param handles, the copied equations, their derivatives
     and the COLAMD ordering can be kept while only values change, as in a drag or an edited
     dimension.
   - Then compile `core.Expr` to a flat tape, so evaluation is a loop instead of a recursive
     walk.
   - Later: split the system into connected components, so independent sub-sketches are solved
     apart. SolveSpace doesn't do this.
   - Expected gain: the 23% of a small model's regeneration, and most of the time of a drag.
3. **P3: mesh BSP without an allocation per node.** Same bytes; 1 day.
   - Give `bspUtil` fixed arrays for the flags, with a slice only for a polygon of more than 8
     vertices, behind accessor methods (54 uses), so that it no longer points into itself.
   - Then `var u bspUtil` stays on the stack.
   - The pool measured the gain at −13%.
4. **P4: skip the bounding-box pass.** Same bytes in export mode, where there is no such pass;
   goldens in display mode if the tolerance changes; 2–3 days.
   - Display regeneration solves every group twice: once to find the model's size, for the chord
     tolerance, and once to build the solids.
   - Take the size from the previous regeneration's box, falling back to the second pass only
     when it moved by more than a margin.
   - This is for the window: interaction regenerates in display mode.
5. **P5: `.tlspr` reading and writing.** Same bytes; 2–3 days.
   - Decoding builds a token and a `Node` tree per line, with strings for every number.
   - Encoding builds a whole `Node`/`Value` tree, with 13 `strconv` values per mesh triangle,
     before printing.
   - Instead, stream both ways, and parse numbers from the bytes in place.
   - This matters for linked parts and large meshes: decoding is 15% of the mesh model's
     allocations.
6. **P6: sketch bookkeeping.** Same bytes; 1 day.
   - `sortByHandle` stable-sorts the entity and param lists twice per group, even when they are
     sorted already: check with `slices.IsSortedFunc` first.
   - `Sketch.Entity` and `Sketch.Param` rebuild their whole map on any miss.
   - `Group`, `Request`, `Constraint` and `Style` are linear scans, and `Group` runs inside the sort
     comparator of `generateAll`.
   - `prevGroupParams` is rebuilt per group.
   - `addAndAssignCurveID` scans for the largest id on every add.
7. **P7: constrained Delaunay triangulation of trimmed surfaces.** Goldens; about 1 week.
   - Ear clipping, with holes bridged in first (`triangulate.go:134,450`), is O(n²) to O(n³) in the
     vertices of a face.
   - `pkg/internal/cdt` is already used for STEP faces.
   - It changes every mesh golden, but scales on faces with many holes, such as text on a plate,
     and gives better triangles.
8. **P8: robust predicates in the mesh BSP.** Goldens where it fixes something; 3–4 days.
   - Use Shewchuk-style adaptive `orient3d` for which side of a plane a vertex is on.
   - This is robustness first; speed only by avoiding slivers.
9. **P9: `GOAMD64=v3`.** Goldens; half a day to measure.
   - It lets the compiler fuse `x*y+z` into FMA, which changes the last bits of the kernel's
     arithmetic, so every golden.
   - Worth a measurement, but the gain on code bound by memory and branches is likely small.
   - The released binaries would then need a CPU from 2013 or later.

## Third-party math libraries

None is recommended now.

- **What dominates is not dense arithmetic.** The profiles show the time in pointer-chasing
  geometry (BSPs, trims, subdivisions), allocation, and repeated work that can be cached. A faster
  library would speed up a small part.
- **gonum** (`mat`, pure Go) could take the dense RREF of `solver/freedom.go`, but the matrices are
  small and the code is not hot. It has no sparse QR.
- **SuiteSparse SPQR or CHOLMOD** would replace the Eigen-derived sparse QR, but only through cgo,
  which the kernel avoids. The QR is not where the time is (P2 is).
- **SIMD for `geom.Vector`** (avo, or the `simd` package when Go's is stable): vectors are three
  `float64`s used one at a time across pointers. No loop found is wide enough to vectorise
  profitably.
- **`pkg/libm`** (glibc's sin, cos, atan2 and so on, bit for bit) is cold except in revolves and
  offset contours. Swapping it for `math` would change goldens for no measurable gain.
- **A Go port of exact geometric predicates** (P8) is the one external algorithm worth taking, and
  it is small enough to write in-tree.

## Reproducing

```sh
make build
./scripts/perf-fixture-hashes.sh > before.txt   # and after a change: diff
go test -c -o slvs.test ./pkg/persist/slvs && (cd pkg/persist/slvs &&
  ../../../slvs.test -test.run '^$' -test.bench 'Regenerate' -test.count 6)
bin/telesapr-cli regenerate --cpuprofile cpu.out model.tlspr   # or --trace trace.out
go tool pprof -top bin/telesapr-cli cpu.out
```

The scaled models are `testdata/group/translate_nd/normal.slvs` rewritten as follows:

- lines 212 and 377 (the two `Group.valA`) set to n;
- each `Group.meshCombine` set to 0;
- for the mesh variant, `Group.forceToMesh=1` added after each `meshCombine`.

Convert the result with `telesapr-cli convert`.
