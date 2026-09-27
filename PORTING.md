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
| `expr.cpp` | 963 | OK | `pkg/core/expr.go`, `pkg/core/expr_parse.go` | `ExprQuaternion::Rotate`/`RotationU/V/N`/`Times` use the C++ formulas, which differ from a rotation for non-unit quaternions |
| `entity.cpp` | 997 | OK | `pkg/core/entity.go`, `pkg/core/entity_base.go` | Every `EntityBase` accessor, numeric and symbolic, for all point/normal/face/distance types, plus `GenerateEquations` and `Entity::CalculateNumerical`. The sketch is passed in as a `core.EntitySketch` instead of the global `SK` |
| `constraint.cpp` | 994 | OK | `pkg/core/constraint.go` | |
| `system.cpp` | 660 | UNAUDITED | `pkg/solver/solve.go`, `pkg/solver/system.go` | Not a port. The Go solver is a damped least-squares Newton with its own substitution and staging passes; it has no rank test, and its DOF differs from C++. `System.Known` gives the per-group solve its fixed params |
| `group.cpp` | 1228 | OK | `pkg/sketch/group.go`, `pkg/sketch/group_generate.go` | `Generate` (all group types), `CopyEntity`, `Remap` (as `RemapEntity`), the `Make*` helpers and `GenerateEquations`. LINKED groups copy `ImpEntity`, which nothing populates yet. Menu and UI code are out of scope |
| `generate.cpp` | 598 | OK | `pkg/sketch/generate_all.go`, `pkg/sketch/sketch.go` | `GenerateAll` (the `genForBBox` pass, regenerating entities and params, prune-and-retry, per-group `SolveGroup`), `ForceReferences`, `CalculateEntityBBox`. With no active group or dirty tracking, DIRTY and UNTIL_ACTIVE regenerate everything like ALL |
| `groupmesh.cpp` | 803 | PARTIAL | `pkg/sketch/group_pipeline.go` | `AssembleLoops`/`GenerateLoops`, the running-shell boolean combine and `MergeCoincidentSurfaces` placement are faithful. LATHE/REVOLVE/HELIX shells follow C++ (axis from `predef`, angle `param(3)*4`, helix distance `param(7)*2`), but faces aren't remapped (`REMAP_LINE_TO_FACE`, `REMAP_LATHE_START/END`): `SBezier` carries no source entity |
| `drawentity.cpp` (`GenerateBezierCurves`) | — | OK | `pkg/sketch/entity_bezier.go` | Lines, arcs/circles, `ComputeInterpolatingSpline` (C2 cubics, periodic too); TTF text not ported |
| `request.cpp` | 250 | OK | `pkg/sketch/request.go` | |
| `style.cpp` | 948 | OK | `pkg/core/style.go` | |
| `clipboard.cpp` | 527 | OK | `pkg/sketch/clipboard.go` | |
| `srf/surface.cpp` | 494 | PARTIAL | `pkg/srf/surface.go`, `pkg/srf/shell.go`, `pkg/srf/raycast_surface.go` | `FromPlane`, `FromExtrusionOf`, `IsExtrusion`, `IsCylinder`, `Reverse`, `MakeEdgesInto`/`MakeTrimEdgesInto`, `TriangulateInto`, `MakeFromExtrusionOf`, `FromRevolutionOf` are faithful. `MakeSectionEdgesInto` is a stub |
| `srf/shell.cpp` | 617 | OK | `pkg/srf/shell.go` | `MakeFromExtrusionOf`, `MakeFromRevolutionOf`, `MakeFromHelicalRevolutionOf`, `CheckNormalAxisRelationship`, `MakeFirstOrderRevolvedSurfaces` faithful; checked against a C++ harness in `pkg/srf/revolve_test.go` |
| `srf/boolean.cpp` | 1100 | OK | `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go`, `pkg/srf/boolean_trim.go`, `pkg/srf/bsp_uv.go` | `SBspUv`, `MakeFromBoolean`, curve splitting, `MakeCopyTrimAgainst`, `TrimFromEdgeList`, `EdgeNormalsWithinSurface` |
| `srf/polygon.cpp` / `polygon.cpp` | 924 | OK | `pkg/srf/polygon.go`, `pkg/srf/contour.go`, `pkg/srf/edgelist.go`, `pkg/srf/merge.go` | `AssemblePolygon`/`AssembleContour`, contour orientation (C++ sign convention), `FixContourDirections`, `EdgeCrosses`, `CullExtraneousEdges` |
| `srf/surfinter.cpp` | 641 | UNAUDITED | `pkg/srf/surfinter.go` | Not yet compared line by line against C++; `AddExactIntersectionCurve` now uses the faithful curve split, and every branch passes `agnstA`/`agnstB` through as in C++. The general case is a Go marching-squares stand-in |
| `srf/bezier.cpp` | — | OK | `pkg/srf/bezier*.go` | |
| `srf/curve.cpp` | 886 | PARTIAL | `pkg/srf/curve.go`, `pkg/srf/bezier_loop*.go`, `pkg/srf/bezier_list.go`, `pkg/srf/boolean_csg.go` | Loop assembly (`FromCurves`, `SBezierLoopSet::From`, `FindOuterFacesFrom`, `GetPlaneContainingBeziers`), `IsCircle`, `GetSurfaceA/B`, `RemoveShortSegments` faithful. `SBezier::AllIntersectionsWith` is a sampling approximation |
| `srf/triangulate.cpp` | 738 | OK | `pkg/srf/triangulate.go` | Hole bridging, ear clipping, adaptive grid |
| `srf/ratpoly.cpp` | 790 | OK | `pkg/srf/bezier.go`, `pkg/srf/pwl.go`, `pkg/srf/surface.go` | Adaptive pwl (`MakePwlInto`), `PointOnSurfaces`, `PointOnCurve`, `ClosestPointOnThisAndSurface` |
| `srf/raycast.cpp` | 807 | OK | `pkg/srf/raycast.go`, `pkg/srf/raycast_surface.go` | `ClassifyEdge` and trimmed `AllPointsIntersecting` (plane, cylinder, subdivision) |
| `srf/merge.cpp` | 130 | OK | `pkg/srf/merge.go` | `MergeCoincidentSurfaces` |
| `bsp.cpp` | 733 | OK | `pkg/srf/bsp3.go`, `pkg/srf/bsp2.go` | |
| `constrainteq.cpp` | 1062 | PARTIAL | `pkg/solver/system.go`, `pkg/sketch/group_generate.go` | Every constraint type has an `addXxx` emitter, but not in the C++ form. PT_PT_DISTANCE squares both sides; PARALLEL is three equations on normalized vectors instead of `VectorsParallel3d`/the pseudo-scalar product. `ModifyToSatisfy` is computed numerically per type. `HasLabel` and `Generate` are faithful |
| `modify.cpp` | 751 | OK | `pkg/sketch/group_generate.go` | UI helpers (ReplacePointInConstraints, MakeTangentArc, SplitLine/Circle/Cubic) are out-of-scope. |
| `solvespace.cpp` | 1236 | OK | `pkg/sketch/sketch.go`, `pkg/sketch/group.go` | Observer notifications are out-of-scope (UI). |
| `undoredo.cpp` | 157 | OK | `pkg/sketch/undo.go` | |
| `mesh.cpp` | 1221 | OK | `pkg/srf/mesh.go` | |
| `file.cpp` | 1030 | OK | `pkg/persist/slvs/slvs.go` | SLVS persistence |
| `util.cpp` (geometry helpers) | — | OK | `pkg/geom` | `Vector::Equals` uses `LENGTH_EPS`, `OnLineSegment`, `BoundingBoxIntersectsLine`, `BandedMatrix`, plane intersections, `Quaternion::Rotate`/`RotationU/V/N`/`From(u, v)` |

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

- 505 tests (412 top-level + subtests) across 6 engine packages (`pkg/core`, `pkg/geom`,
  `pkg/persist/slvs`, `pkg/sketch`, `pkg/solver`, `pkg/srf`).
- All 86 non-boolean fixtures load cleanly and round-trip via
  `TestSLVSLoadRoundTrip` and `TestPhase7AllFixturesRoundTrip`.
- All 31 constraint fixtures load, round-trip, and solve with
  `How=SolveOkay` across all groups via `TestParityConstraintFixtures`.
- `TestSolverAllSLVSFixtures` runs `GenerateAll` on every fixture and
  requires every group to solve, except `constraint/large_dimensions`
  (see remaining work) and one hand-made file C++ can't load either.
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
- Every boolean fixture's go/cpp volume ratio is within the C++
  testsuite's own 1e-6 (met).

## Boolean-CSG parity

| Fixture | go/cpp volume ratio |
|---|---|
| `boolean_coplanar_union`   | 1.000000 |
| `boolean_knife_edge`       | 1.000000 |
| `boolean_tangent_crossing` | 1.000000 |
| `boolean_tangent_edge`     | 1.000000 |
| `boolean_tangent_fillet`   | 1.000000 |
| `boolean_tangent_spline`   | 1.000000 |

None of the fixtures sets `BooleanFailed`. `TestParityBooleanVolumeVsCppGolden`
(`pkg/persist/slvs/bool_generate_test.go`) enforces `|ratio - 1| < 1e-6`,
the C++ `CHECK_EQ_EPS` criterion, for all six.

`boolean_tangent_spline` needed the chord tolerance, not the CSG. Go's
`Group::Generate` was a stand-in that emitted copies of the extruded
points with made-up handles at z = ±60. That gave a 120 mm bounding box
where C++ has 70.96 mm, so the chord tolerance was 0.12 mm instead of
0.07096 mm. With `Group::Generate` and `GenerateAll` ported, Go
produces the same entities as C++ and the same volume.

**Entity check against C++.** A throwaway C++ harness loaded each
fixture the way the testsuite does (`LoadFromFile` + `AfterNewFile`).
It dumped every entity's handle, type, group, construction flag, point,
normal, face and distance, the chord tolerance, and each group's solve
status. For 86 of the 89 fixtures C++ can load, the Go `GenerateAll`
output matches exactly (to 1e-4). The three that differ:

- `constraint/large_dimensions`: the Go solver doesn't converge.
- `constraint/pt_on_circle/negative_dia`: the Go constraint equations
  settle on a different solution.
- `request/image/linked`: LINKED import isn't implemented.

The first two are `constrainteq.cpp`/`system.cpp` differences.

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
  - Plane-plane and same-axis extrusion intersection curves weren't split against the operand shells (`nil` passed for `agnstA`/`agnstB`).
  - `SSurface.PointOnCurve` ignored its initial uv guess, so a circle cut by a plane could refine to the wrong root.
  - `SCurve.RemoveShortSegments` lacked the exact-curve t-spacing check, so it thinned circles below C++'s pwl.
- **Triangulation.** Contours were ear-clipped one by one, with no hole bridging.

The Go-only heuristics these had been masking were removed: the
`interCount` gate, the centroid classifier fallback, the op-aware
`TrimBuilt` grid fallback, and the "fall back to the untrimmed grid
when the trim polygon doesn't assemble" path.

## Remaining work

In priority order:

1. **Audit `constrainteq.cpp` and `system.cpp`.** Port
   `ConstraintBase::GenerateEquations` line by line onto the `core`
   `EntityBase` accessors, replacing the `addXxx` builders. Then compare
   the solver with `System::Solve` (substitution, rank test, DOF). That
   fixes `large_dimensions` and `negative_dia`, and makes
   `ModifyToSatisfy` the C++ one (`valA += eq.Eval()`).
2. **Audit `srf/surfinter.cpp`** (`IntersectAgainst` and its per-case
   helpers) line by line, as was done for `boolean.cpp`/`raycast.cpp`.
3. **Watertightness check.** The C++ fixture tests also assert that
   `SKdNode::MakeCertainEdgesInto(NAKED_OR_SELF_INTER)` finds no
   naked or self-intersecting edges. Port that check and assert it in
   `TestParityBooleanFixtures`.
4. **Smaller stand-ins found but not yet ported:**
   - `SBezier::AllIntersectionsWith`;
   - `SSurface::MakeSectionEdgesInto`;
   - the face remapping of LATHE/REVOLVE/HELIX shells, which needs
     `SBezier::entity`;
   - the load-time `GenerateAll(REGEN)` in `UpgradeLegacyData`, so the
     first bbox pass pwl's loops at the file's own chord tolerance;
   - LINKED import (`impEntity`, `impShell`).
5. **GUI.** `pkg/app` adds constraints over the entities as loaded,
   without `GenerateAll`. Group-generated entities saved in a file
   carry no params, so the faithful accessors report errors for them.
   The app should regenerate like C++ `AfterNewFile`.
