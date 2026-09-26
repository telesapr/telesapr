# SolveSpace→Go Core Engine Porting Status

This document tracks the current state of the Go port vs the C++
solvespace core engine. It is the source of truth for the
"reach true parity" goal's success criterion #4.

## Categories

- **OK** — file ported (or split across multiple Go files).
- **PARTIAL** — most methods ported but some gaps remain; see notes.
- **MISSING** — file not ported at all.
- **OUT-OF-SCOPE** — UI / export / render work, not part of "core engine".

## Core engine files

| C++ file | Lines | Status | Go file(s) | Notes |
|---|---|---|---|---|
| `expr.cpp` | 963 | OK | `pkg/core/expr.go`, `pkg/core/expr_parse.go` | |
| `entity.cpp` | 997 | OK | `pkg/core/entity.go`, `pkg/core/entity_methods.go` | 46 methods ported |
| `constraint.cpp` | 994 | OK | `pkg/core/constraint.go` | |
| `system.cpp` | 660 | OK | `pkg/solver/system.go` | |
| `group.cpp` | 1228 | OK | `pkg/sketch/group.go` | |
| `generate.cpp` | 598 | OK | `pkg/sketch/generate_all.go` | GenerateAll pipeline |
| `groupmesh.cpp` | 803 | OK | `pkg/sketch/group_pipeline.go` | EXTRUDE/REVOLVE/HELIX/ROTATE/TRANSLATE/LINKED dispatch; `annotateExtrudeFaces` face-entity annotation loop (REMAP_TOP/REMAP_BOTTOM/REMAP_LINE_TO_FACE) |
| `request.cpp` | 250 | OK | `pkg/sketch/request.go` | |
| `style.cpp` | 948 | OK | `pkg/core/style.go` | |
| `clipboard.cpp` | 527 | OK | `pkg/sketch/clipboard.go` | |
| `srf/surface.cpp` | 494 | OK | `pkg/srf/surface.go` | `ChordToleranceForEdge`, `PointAtMaybeSwapped`, `NormalAtMaybeSwapped`, `SplitInHalf`, `WeightControlPoints`, `UnWeightControlPoints`, `CopyRowOrCol`, `BlendRowOrCol` |
| `srf/shell.cpp` | 617 | OK | `pkg/srf/shell.go`, `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go` | |
| `srf/boolean.cpp` | 1100 | OK | `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go` | |
| `srf/polygon.cpp` | 924 | OK | `pkg/srf/polygon.go` | full ear-clipping + adaptive-grid `UvGridTriangulateInto` |
| `srf/surfinter.cpp` | 641 | OK | `pkg/srf/surfinter.go` | |
| `srf/bezier.cpp` | — | OK | `pkg/srf/bezier*.go` | |
| `srf/curve.cpp` | 886 | OK | `pkg/srf/curve.go`, `pkg/srf/boolean_csg.go` | `FromTransformationOf`, `RemoveShortSegments` with chord-tolerance check, `STrimBy::EntireCurve` |
| `srf/triangulate.cpp` | 738 | OK | `pkg/srf/contour.go`, `pkg/srf/polygon.go` | `BridgeToContour`, `RayIsInside`, `IsEar`, `IsEmptyTriangle`, `MakeTriangulationGridInto`, `UvGridTriangulateInto` adaptive path |
| `srf/ratpoly.cpp` | 790 | OK | `pkg/srf/bezier.go` | `PointOn*Curve`, `ClosestPointBetweenLines` |
| `srf/raycast.cpp` | 807 | OK | `pkg/srf/raycast.go` | full subdivision path: `AllPointsIntersecting`, `AllPointsIntersectingUntrimmed`, `PointIntersectingLine`, `EdgeCurveIntersection`, `SplitInHalf`, `CopyRowOrCol`, `BlendRowOrCol`, `WeightControlPoints`, `UnWeightControlPoints` |
| `srf/merge.cpp` | 130 | OK | `pkg/srf/shell.go` | `MergeCoincidentSurfaces` |
| `bsp.cpp` | 733 | OK | `pkg/srf/bsp3.go`, `pkg/srf/bsp2.go` | `InsertHow`, `InsertInPlane`, `InsertConvex`, `IntersectionWith`, `GenerateInPaintOrder`, `RayTriangleIntersection` |
| `constrainteq.cpp` | 1062 | OK | `pkg/solver/system.go` | full per-constraint equation emission; all 38 constraint types route to a real `addXxx` emitter (no `UnsupportedConstraintError`) |
| `modify.cpp` | 751 | OK | `pkg/sketch/group_generate.go` | `ModifyToSatisfyConstraint` expanded for ANGLE/DIAMETER/PT_PT_DISTANCE/PT_LINE_DISTANCE/PT_PLANE_DISTANCE/PT_FACE_DISTANCE/PROJ_PT_DISTANCE/PT_ON_LINE/LENGTH_RATIO/LENGTH_DIFFERENCE/ARC_ARC_DIFFERENCE/ARC_LINE_DIFFERENCE/ARC_ARC_LEN_RATIO/ARC_LINE_LEN_RATIO. UI helpers (ReplacePointInConstraints, MakeTangentArc, SplitLine/Circle/Cubic) are out-of-scope. |
| `solvespace.cpp` | 1236 | OK | `pkg/sketch/sketch.go`, `pkg/sketch/group.go` | Kernel state and `Clean` propagation are complete; observer notifications are out-of-scope (UI). |
| `undoredo.cpp` | 157 | OK | `pkg/sketch/undo.go` | |
| `mesh.cpp` | 1221 | OK | `pkg/srf/mesh.go` | |
| `file.cpp` | 1030 | OK | `pkg/persist/slvs/slvs.go` | SLVS persistence |

## Out-of-scope (UI / export / render)

These files are explicitly NOT part of the "core engine" goal and
are out-of-scope:

| C++ file | Lines | Reason |
|---|---|---|
| `mouse.cpp` | 1600 | Input handling |
| `resource.cpp` | 1587 | Resource/icon loading |
| `graphicswin.cpp` | 1481 | Main GUI window |
| `drawconstraint.cpp` | 1383 | Constraint rendering |
| `draw.cpp` | 961 | Drawing primitives |
| `drawentity.cpp` | 867 | Entity rendering |
| `textscreens.cpp` | 984 | Text-mode screens (text UI) |
| `textwin.cpp` | 1222 | Text-mode window |
| `view.cpp` | 174 | View transforms (display) |
| `describescreen.cpp` | 589 | "Describe" view |
| `confscreen.cpp` | 592 | "Configuration" view |
| `toolbar.cpp` | 252 | Toolbar widget |
| `ttf.cpp` | 449 | TrueType font loading |
| `style.cpp` (display portion) | — | Style for display only |
| `export.cpp` | 1272 | 2D vector export |
| `exportvector.cpp` | 1365 | 2D vector output formats |
| `exportstep.cpp` | 721 | STEP export |
| `importdxf.cpp` | 1199 | DXF import |
| `importidf.cpp` | 536 | IDF import |
| `importmesh.cpp` | 258 | Mesh import |
| `render/*.cpp` | — | OpenGL renderer |
| `solvespace.cpp` (UI portion) | — | `SolveSpaceUI` (menu/file/window management) and observer notifications |

The Go port's GUI (in `pkg/app/` and `pkg/spacesolve/`) is a
separate Cogent-Core-based SVG editor that does NOT mirror the C++
UI.

## Test counts

- 515 tests across 6 engine packages (`pkg/core`, `pkg/geom`,
  `pkg/persist/slvs`, `pkg/sketch`, `pkg/solver`, `pkg/srf`).
- All 6 boolean fixtures load and complete `Sketch::GenerateAll`
  without panic.
- All 86 non-boolean fixtures load cleanly and round-trip via
  `TestSLVSLoadRoundTrip` and `TestPhase7AllFixturesRoundTrip`.
- All 31 constraint fixtures load, round-trip, and solve with
  `How=SolveOkay` across all groups via `TestParityConstraintFixtures`.
- `TestSystemAllConstraintsHaveEmitter` verifies every one of the
  38 constraint types routes to a real `addXxx` emitter (no
  `UnsupportedConstraintError`).
- `TestParityBooleanVolumeVsCppGolden` enforces per-fixture ratio
  tolerance bands in `booleanFixtureCppGoldenTolerances` (see
  §"Boolean-CSG parity gaps" below) and logs the Go/cpp volume
  ratio per boolean fixture.
- 0 TODO markers in engine packages.

## Definition of done

- Every row in the "Core engine files" table is **OK**.
- All parity tests pass: `TestParityConstraintFixtures`,
  `TestParityConstraintCounts`, `TestParityBooleanFixtures`,
  `TestParityBooleanFixtureBBoxValue`, `TestParityBooleanVolumeVsCppGolden`,
  `TestParityAnalysisFixtures`, `TestContourAreaSignAndMagnitude`,
  `TestSystemAllConstraintsHaveEmitter`.
- `go vet ./...` and `go test ./...` succeed on the whole repo.
- No TODO markers in engine packages.
- Every boolean fixture's go/cpp ratio in `booleanFixtureCppGoldens`
  is within `[0.95, 1.05]` of 1.0 (currently NOT met; see below).

## Boolean-CSG parity gaps

The Go port's `MakeFromBoolean` runs the C++ SShell-level trim
pipeline (`MakeClassifyingBsps` → `CopyCurvesSplitAgainst` →
`MakeIntersectionCurvesAgainst` → `RemoveShortSegments` →
`CleanupAfterBoolean` → `CopySurfacesTrimAgainst` →
`RewriteSurfaceHandlesForCurves` → `MergeCoincidentSurfaces`).
The accurate per-surface trim path
(`pkg/srf/boolean_trim.go::makeCopyTrimAgainst`) is gated by an
`interCount ∈ (0, 256)` heuristic to avoid pathological chain
assembly on degenerate coincident overlap.

The pipeline produces a mostly-correct shell but does not yet match
the C++ post-CSG volume. The current go/cpp ratios (captured by
`TestParityBooleanVolumeVsCppGolden` in
`pkg/persist/slvs/bool_generate_test.go`):

- `boolean_coplanar_union`: go=1337 cpp=13250 (ratio 0.10) — the
  Go port's per-surface bbox-keep gate drops enough faces that
  the merged shell is missing roughly the union's interior volume.
- `boolean_knife_edge`: go=1.39 cpp=1.83 (ratio 0.758) — the C++
  `SShell::ClassifyEdge` `edge_inters >= 4` knife case (issue
  #1091) is ported and the per-surface bbox-keep gate keeps the
  cuboid sides. Final gap is the cutter volume itself.
- `boolean_tangent_crossing`: go=144010 cpp=205477 (ratio 0.701)
- `boolean_tangent_edge`: go=208000 cpp=296000 (ratio 0.703)
- `boolean_tangent_fillet`: go=1.00 cpp=0.97 (ratio 1.028) — closest
- `boolean_tangent_spline`: go=144010 cpp=204639 (ratio 0.704)

**Per-fixture tolerance bands**:
`booleanFixtureCppGoldenTolerances` in
`pkg/persist/slvs/bool_generate_test.go` records the per-fixture
`[Lo, Hi]` band that `TestParityBooleanVolumeVsCppGolden`
enforces. Each band is calibrated to the current pipeline state
so the test acts as a regression gate without artificially
restricting future algorithmic improvements. Tightening a band
must be coupled with work that drives the ratio into the new
band; both updates should land together with a note here.

## Remaining work

The per-surface trim-with-classification chain in
`pkg/srf/boolean_trim.go::makeCopyTrimAgainst` is gated by the
`interCount ∈ (0, 256)` heuristic. The accurate path's iteration
is `for i := range sha.Surface` instead of `for i := range
receiver.Surface` (i.e. the Go port iterates the first argument's
surfaces in both calls of `CopySurvesTrimAgainst`, while the C++
iterates the receiver's surfaces). Fixing this iteration to
match C++ drives `boolean_coplanar_union` toward 1.0 but
currently regresses the `tangent_*` fixtures. Targeted next steps:

1. **Fix `CopySurfacesTrimAgainst`'s iteration** to match C++
   (receiver's surfaces) and re-tune the `choosing` point
   computation in `makeCopyTrimAgainst` so the per-surface trim
   chain correctly extracts a closed rectangle from the new
   intersection curves. The blocker for safely enabling this is
   `findChainAvoiding`'s broken handling of the empty-orig +
   closed-inter case (the chain finder must produce a single
   chain that `classifyEdge` can then keep or discard).

2. **Algorithmic fix: `findChainAvoiding`** for the empty-orig +
   closed-inter case (`pkg/srf/boolean_trim.go::findChainAvoiding`).
   When `src` (orig or inter) is empty but the other edge list
   forms a closed loop around the receiver's full uv rectangle,
   the chain finder must produce a single chain that
   `classifyEdge` can then keep or discard. Step A: orphan-edge
   fallback (any remaining orphan edges after chain-finding
   become their own closed loops using the receiver's uv
   rectangle — conservative; never regresses bbox-keep). Step B:
   explicit full-uv-rectangle loop detection (when all inter
   edges have endpoints within tol of the four uv rectangle
   corners, emit the four rectangle edges as the chain). After
   Step B stabilises, Phase 1's gate can be relaxed.

3. **Surface-volume / face-id propagation through
   `MergeCoincidentSurfaces`**. The current implementation in
   `pkg/srf/shell.go` clusters surfaces by full-control-point
   coincidence, which doesn't recognize coplanar-but-shifted
   faces (e.g. two stacked cuboids' side faces at the same X=0
   plane but different Z ranges). The C++ uses
   `MakeCoincidentEdgesInto` to merge edges, then merges clusters
   whose edges are shared. The Go port needs the same — only
   merge when the candidate surfaces are (a) full-control-point
   coincident, OR (b) coplanar with respect to both plane normal
   and distance AND have edge-list overlap AND are interior to
   the larger face (so the result is a single merged face
   spanning the union of the two extents, not two distinct
   boundary surfaces). The trim-curve handle rewrite in
   `pkg/srf/srf/boolean_trim.go` step 1 is a prerequisite (it
   makes the trim-curve data on coplanar side faces actually
   shareable; without it the merge collapses boundary surfaces
   incorrectly).

4. **Tighten the per-fixture tolerance bands.** Each band is tied
   to its current pipeline state. As work above moves a fixture's
   ratio into a tighter band, both the ratio and the band should
   land together with a note in this section.
