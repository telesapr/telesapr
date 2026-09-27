# SolveSpace→Go Core Engine Porting Status

This document tracks the current state of the Go port vs the C++
solvespace core engine. It is the source of truth for the
"reach true parity" goal's success criterion #4.

## Categories

- **OK** — file ported (or split across multiple Go files).
- **PARTIAL** — most methods ported but some gaps remain; see notes.
- **MISSING** — file not ported at all.
- **UNAUDITED** — ported, but not yet compared line by line against the C++.
- **OUT-OF-SCOPE** — UI / export / render work, not part of "core engine".

## Core engine files

| C++ file | Lines | Status | Go file(s) | Notes |
|---|---|---|---|---|
| `expr.cpp` | 963 | OK | `pkg/core/expr.go`, `pkg/core/expr_parse.go` | |
| `entity.cpp` | 997 | PARTIAL | `pkg/core/entity.go`, `pkg/core/entity_methods.go` | `EntityBase::ArcGetAngles` is a stub (always a full circle); `pkg/sketch/entity_bezier.go` computes arc angles itself |
| `constraint.cpp` | 994 | OK | `pkg/core/constraint.go` | |
| `system.cpp` | 660 | OK | `pkg/solver/system.go` | |
| `group.cpp` | 1228 | OK | `pkg/sketch/group.go` | |
| `generate.cpp` | 598 | PARTIAL | `pkg/sketch/generate_all.go` | Solves all groups in one Newton system instead of per group; `chordTolCalculated` is ported, but `Sketch.CalculateEntityBBox` disagrees with C++ on some models (see §"Boolean-CSG parity") |
| `groupmesh.cpp` | 803 | PARTIAL | `pkg/sketch/group_pipeline.go` | `AssembleLoops`/`GenerateLoops`, the running-shell boolean combine and `MergeCoincidentSurfaces` placement are faithful. REVOLVE/HELIX shells are not (see `srf/surface.cpp`) |
| `drawentity.cpp` (`GenerateBezierCurves`) | — | OK | `pkg/sketch/entity_bezier.go` | Lines, arcs/circles, `ComputeInterpolatingSpline` (C2 cubics, periodic too); TTF text not ported |
| `request.cpp` | 250 | OK | `pkg/sketch/request.go` | |
| `style.cpp` | 948 | OK | `pkg/core/style.go` | |
| `clipboard.cpp` | 527 | OK | `pkg/sketch/clipboard.go` | |
| `srf/surface.cpp` | 494 | PARTIAL | `pkg/srf/surface.go`, `pkg/srf/shell.go`, `pkg/srf/raycast_surface.go` | `FromPlane`, `FromExtrusionOf`, `IsExtrusion`, `IsCylinder`, `Reverse`, `MakeEdgesInto`/`MakeTrimEdgesInto`, `TriangulateInto`, `MakeFromExtrusionOf` are faithful. `MakeFromRevolutionOf`/`MakeFromHelixOf` build no trims (their surfaces render untrimmed, `SSurface.TrimBuilt=false`); `MakeSectionEdgesInto` is a stub |
| `srf/shell.cpp` | 617 | PARTIAL | `pkg/srf/shell.go` | Extrusion faithful; revolution/helix as above |
| `srf/boolean.cpp` | 1100 | OK | `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go`, `pkg/srf/boolean_trim.go`, `pkg/srf/bsp_uv.go` | `SBspUv`, `MakeFromBoolean`, curve splitting, `MakeCopyTrimAgainst`, `TrimFromEdgeList`, `EdgeNormalsWithinSurface` |
| `srf/polygon.cpp` / `polygon.cpp` | 924 | OK | `pkg/srf/polygon.go`, `pkg/srf/contour.go`, `pkg/srf/edgelist.go`, `pkg/srf/merge.go` | `AssemblePolygon`/`AssembleContour`, contour orientation (C++ sign convention), `FixContourDirections`, `EdgeCrosses`, `CullExtraneousEdges` |
| `srf/surfinter.cpp` | 641 | UNAUDITED | `pkg/srf/surfinter.go` | Not yet compared line by line against C++; `AddExactIntersectionCurve` now uses the faithful curve split |
| `srf/bezier.cpp` | — | OK | `pkg/srf/bezier*.go` | |
| `srf/curve.cpp` | 886 | PARTIAL | `pkg/srf/curve.go`, `pkg/srf/bezier_loop*.go`, `pkg/srf/bezier_list.go`, `pkg/srf/boolean_csg.go` | Loop assembly (`FromCurves`, `SBezierLoopSet::From`, `FindOuterFacesFrom`, `GetPlaneContainingBeziers`), `IsCircle`, `GetSurfaceA/B` faithful. `SBezier::AllIntersectionsWith` is a sampling approximation |
| `srf/triangulate.cpp` | 738 | OK | `pkg/srf/triangulate.go` | Hole bridging, ear clipping, adaptive grid |
| `srf/ratpoly.cpp` | 790 | OK | `pkg/srf/bezier.go`, `pkg/srf/pwl.go`, `pkg/srf/surface.go` | Adaptive pwl (`MakePwlInto`), `PointOnSurfaces`, `ClosestPointOnThisAndSurface` |
| `srf/raycast.cpp` | 807 | OK | `pkg/srf/raycast.go`, `pkg/srf/raycast_surface.go` | `ClassifyEdge` and trimmed `AllPointsIntersecting` (plane, cylinder, subdivision) |
| `srf/merge.cpp` | 130 | OK | `pkg/srf/merge.go` | `MergeCoincidentSurfaces` |
| `bsp.cpp` | 733 | OK | `pkg/srf/bsp3.go`, `pkg/srf/bsp2.go` | |
| `constrainteq.cpp` | 1062 | OK | `pkg/solver/system.go` | all 38 constraint types route to a real `addXxx` emitter |
| `modify.cpp` | 751 | OK | `pkg/sketch/group_generate.go` | UI helpers (ReplacePointInConstraints, MakeTangentArc, SplitLine/Circle/Cubic) are out-of-scope. |
| `solvespace.cpp` | 1236 | OK | `pkg/sketch/sketch.go`, `pkg/sketch/group.go` | Observer notifications are out-of-scope (UI). |
| `undoredo.cpp` | 157 | OK | `pkg/sketch/undo.go` | |
| `mesh.cpp` | 1221 | OK | `pkg/srf/mesh.go` | |
| `file.cpp` | 1030 | OK | `pkg/persist/slvs/slvs.go` | SLVS persistence |
| `util.cpp` (geometry helpers) | — | OK | `pkg/geom` | `Vector::Equals` uses `LENGTH_EPS`, `OnLineSegment`, `BoundingBoxIntersectsLine`, `BandedMatrix`, plane intersections |

An **OK** row means the functions listed in its notes (or the whole
file, when none are listed) were compared line by line against the C++.
The 2026-09-27 audit showed that earlier OK markings were often wrong:
several "ported" functions were simplified stand-ins. Treat rows not
re-audited since then with suspicion.

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

- 515 tests (422 top-level + subtests) across 6 engine packages (`pkg/core`, `pkg/geom`,
  `pkg/persist/slvs`, `pkg/sketch`, `pkg/solver`, `pkg/srf`).
- All 86 non-boolean fixtures load cleanly and round-trip via
  `TestSLVSLoadRoundTrip` and `TestPhase7AllFixturesRoundTrip`.
- All 31 constraint fixtures load, round-trip, and solve with
  `How=SolveOkay` across all groups via `TestParityConstraintFixtures`.
- `TestParityBooleanVolumeVsCppGolden` checks every boolean fixture's
  volume against the C++ testsuite golden (see below);
  `TestBooleanTwoBoxesVolume` checks union/difference/intersection of
  two overlapping boxes exactly.
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
- Every boolean fixture's go/cpp volume ratio is within `[0.95, 1.05]`
  (met; five of six are within the C++ testsuite's own 1e-6).

## Boolean-CSG parity

| Fixture | go/cpp volume ratio |
|---|---|
| `boolean_coplanar_union`   | 1.000000 |
| `boolean_knife_edge`       | 1.000000 |
| `boolean_tangent_crossing` | 1.000000 |
| `boolean_tangent_edge`     | 1.000000 |
| `boolean_tangent_fillet`   | 1.000000 |
| `boolean_tangent_spline`   | 0.999907 |

None of the fixtures sets `BooleanFailed`. `booleanFixtureCppGoldenTolerances`
(`pkg/persist/slvs/bool_generate_test.go`) enforces
`|ratio - 1| < 1e-6` (the C++ `CHECK_EQ_EPS` criterion) for all but
`boolean_tangent_spline`.

`boolean_tangent_spline`'s residual is not in the CSG. At equal chord
tolerance, the Go mesh volume matches C++ to STL float precision:
`solvespace-cli export-mesh --chord-tol 0.12` gives 204619.494, and
Go gives 204619.494. The C++ testsuite golden (204638.627) is what
C++ produces at chord tolerances of about 0.07–0.1 mm. Go's
`chordTolCalculated` is 0.12 mm, because `Sketch.CalculateEntityBBox`
reports a 120 mm extent (entity points at z = ±60) where the C++
evidently sees about 71 mm. See remaining work item 2.

Parity came from replacing simplified stand-ins with faithful ports,
not from tuning. The main fixes:

- **Operand geometry.**
  - `FromPlane` had u/v swapped, which flipped every plane's normal and transposed its parameterization.
  - `Group.AssembleLoops` put every curve into one unchained "loop".
  - `FindOuterFacesFrom` was a stub: no orientation fixing, no holes.
  - Contour orientation used the opposite sign convention from C++.
  - `GenerateEntityBeziers` drew arcs as full circles, split circles into degenerate 180° arcs, and read cubics' points off by one.
  - `MakeFromExtrusionOf` of a multi-loop sketch kept only the last loop.
- **Classification primitives.**
  - `SBspUv` never returned `EDGE_PARALLEL`/`ANTIPARALLEL` and didn't split straddling edges.
  - `SShell.AllPointsIntersecting` raytraced a cached mesh and returned zero normals, so the `ClassifyEdge` raycast always said OUTSIDE.
  - `LineEntirelyOutsideBbox` ignored `asSegment`.
  - The internal `lengthEps` was 1e-12 instead of `LENGTH_EPS` (1e-6).
  - `EdgeNormalsWithinSurface` used xyz offsets as uv.
- **Trim pipeline.**
  - Split curves kept their original handles, so A's and B's curves collided.
  - The `orig` trim edges were straight lines, double-reversed.
  - Inter edges sampled `Exact` instead of the split polyline.
  - `SSurface.Reverse` was a no-op.
  - `AssemblePolygon` reported success on open chains.
  - `CullExtraneousEdges` didn't cull anti-parallel pairs.
  - `MergeCoincidentSurfaces` kept one surface's trims instead of merging them.
  - The running shell never accumulated across groups.
- **Triangulation.** Contours were ear-clipped one by one, with no hole bridging.

The Go-only heuristics these had been masking were removed: the
`interCount` gate, the centroid classifier fallback, the op-aware
`TrimBuilt` grid fallback, and the "fall back to the untrimmed grid
when the trim polygon doesn't assemble" path.

## Remaining work

In priority order:

1. **`MakeFromRevolutionOf` / `MakeFromHelixOf` trims** (`srf/surface.cpp`).
   Port the C++ constructors, which build trim curves the way
   `MakeFromExtrusionOf` does. Until then, their surfaces render over
   the full parameter range (`TrimBuilt=false`) and can't take part
   in booleans correctly. Add a revolve-vs-box boolean test with an
   analytic volume.
2. **`chordTolCalculated`.** Make `Sketch.CalculateEntityBBox` and the
   generate order match C++ `SolveSpaceUI::GenerateAll`
   (`genForBBox` pass, `generate.cpp:219`). That closes
   `boolean_tangent_spline`'s 1e-4 residual, and the band can then
   tighten to 1e-6.
3. **Audit `srf/surfinter.cpp`** (`IntersectAgainst` and its per-case
   helpers) line by line, as was done for `boolean.cpp`/`raycast.cpp`.
4. **Watertightness check.** The C++ fixture tests also assert that
   `SKdNode::MakeCertainEdgesInto(NAKED_OR_SELF_INTER)` finds no
   naked or self-intersecting edges. Port that check and assert it in
   `TestParityBooleanFixtures`.
5. **Smaller stand-ins found but not yet ported:**
   - `EntityBase::ArcGetAngles` in `pkg/core`;
   - `SBezier::AllIntersectionsWith`;
   - `SSurface::MakeSectionEdgesInto`;
   - per-group solving in `GenerateAll`.
