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
| `expr.cpp` | 963 | OK | `pkg/core/expr.go`, `pkg/core/expr_parse.go` | `ExprQuaternion::Rotate`/`RotationU/V/N`/`Times` use the C++ formulas, which differ from a rotation for non-unit quaternions. `PartialWrt` (`Derivative`), `FoldConstants` (including its `Tol` of 0.001 for the x+0, x*1, x*0 rules) and `IsZeroConst` are faithful |
| `entity.cpp` | 997 | OK | `pkg/core/entity.go`, `pkg/core/entity_base.go` | Every `EntityBase` accessor, numeric and symbolic, for all point/normal/face/distance types, plus `GenerateEquations` and `Entity::CalculateNumerical`. The sketch is passed in as a `core.EntitySketch` instead of the global `SK` |
| `constraint.cpp` | 994 | OK | `pkg/core/constraint.go`, `pkg/sketch/constraint.go` | Audited: `DescriptionString`, `DeleteAllConstraintsFor`, `AddConstraint` (as `AddConstraintRemember`), `Constrain`, `TryConstrain`, `ConstrainCoincident` and the three tangent helpers, which return C++'s error message instead of showing it. `MenuConstrain` is user interface |
| `system.cpp` | 660 | OK | `pkg/solver/system.go`, `pkg/solver/qr.go` | `Solve`, `SolveRank`, `SolveBySubstitution`, `WriteJacobian`, `NewtonSolve`, `SolveLeastSquares` (the QR-of-A' form of this checkout), `TestRank`, `FindWhichToRemoveToFixJacobian`, `MarkParamsFree`. Eigen's `SparseQR` is a dense left-looking Householder QR with the same dead-column rule and threshold, but without the COLAMD column order |
| `group.cpp` | 1228 | OK | `pkg/sketch/group.go`, `pkg/sketch/group_generate.go` | `Generate` (all group types), `Clear`, `CopyEntity`, `Remap` (as `RemapEntity`), the `Make*` helpers, `GenerateEquations` and `TransformImportedBy`. LINKED groups copy the `ImpEntity` the loader reads from the linked file. Menu and UI code are out of scope |
| `generate.cpp` | 598 | OK | `pkg/sketch/generate_all.go`, `pkg/sketch/sketch.go` | `GenerateAll` (the `genForBBox` pass, regenerating entities and params, prune-and-retry), `SolveGroup` with `WriteEqSystemForGroup` and `WriteEquationsExceptFor`, `ForceReferences`, `CalculateEntityBBox`. `GenerateAll`'s DIRTY and UNTIL_ACTIVE ranges (the active group is `Sketch.Active`, else the one C++ picks on load), `TestRankForGroup`, `MarkGroupDirty`, the `deleted` counts and their message (through `Sketch.Message`), and `ScheduleGenerateAll`, which `Refresh` serves as C++'s event loop does. Dragged params aren't marked |
| `groupmesh.cpp` | 803 | OK | `pkg/sketch/group_pipeline.go` | `AssembleLoops`/`GenerateLoops`, `GenerateShellAndMesh` (every group type, step and repeat, the face entities, the running-shell combine, and the forced-to-mesh combine with `SnapToMesh`), `GenerateForStepAndRepeat`, `GenerateForBoolean`, `RemapFaces`, `GenerateDisplayItems` (without the render-side polyline join) are faithful; `group/force_to_mesh` saves match C++ byte for byte |
| `drawentity.cpp` (`GenerateBezierCurves`) | — | OK | `pkg/sketch/entity_bezier.go` | Lines, arcs/circles, `ComputeInterpolatingSpline` (C2 cubics, periodic too); TTF text not ported |
| `request.cpp` | 250 | OK | `pkg/sketch/request.go`, `pkg/persist/slvs/linked.go` | Audited: `Generate` (an image request takes its aspect ratio from the image file, loaded as `ReloadLinkedImage` does; a TTF request keeps its saved one, since font loading isn't ported), `DescriptionString`, `IndexOfPoint`, the entity/request table |
| `style.cpp` | 948 | OK | `pkg/core/style.go`, `pkg/sketch/style.go` | Audited: the default styles and `FillDefaultStyle` (factory settings; the engine has no settings store), `Get` (which creates a missing style, so an entity naming one that the file lacks gets a saved "new-custom-style", as in C++), `ForEntity`, the colour, width, text height, exportable and stipple getters (widths in mm take the view scale), `DescriptionString`. Loading starts each style from the defaults. The style screens are user interface |
| `clipboard.cpp` | 527 | OK | `pkg/sketch/clipboard.go`, `pkg/sketch/edit.go` | Audited: the clipboard (`NewEntityFor` was off by one), `CopySelection`, `PasteClipboard` and `DeleteSelection`, taking the selection as an argument, with `GraphicsWindow::AddRequest` and `DeleteTaggedRequests`. The menus and the paste-transformed screen are user interface |
| `srf/surface.cpp` | 494 | OK | `pkg/srf/surface.go`, `pkg/srf/shell.go`, `pkg/srf/raycast_surface.go` | Audited throughout: `FromPlane`, `FromExtrusionOf`, `IsExtrusion`, `IsCylinder`, `FromRevolutionOf`, `FromTransformationOf`, `GetAxisAlignedBounding`, `LineEntirelyOutsideBbox`, `MakeEdgesInto`/`MakeTrimEdgesInto`, `ExactSurfaceTangentAt`, `MakeSectionEdgesInto`, `TriangulateInto`, `Reverse`, `ScaleSelfBy`, `Clear` (trims only), `CoincidentWithPlane`, `SShell::MakeFromCopyOf`/`MakeFromTransformationOf` |
| `srf/shell.cpp` | 617 | OK | `pkg/srf/shell.go` | `MakeFromExtrusionOf`, `MakeFromRevolutionOf`, `MakeFromHelicalRevolutionOf` (including their face entities, through the `ShellGroup` the caller passes in place of C++'s `Group`), `CheckNormalAxisRelationship`, `MakeFirstOrderRevolvedSurfaces`, `MakeSectionEdgesInto` faithful; checked against a C++ harness in `pkg/srf/revolve_test.go` |
| `srf/boolean.cpp` | 1100 | OK | `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go`, `pkg/srf/boolean_trim.go`, `pkg/srf/bsp_uv.go` | `SBspUv`, `MakeFromBoolean`, curve splitting, `MakeCopyTrimAgainst`, `TrimFromEdgeList`, `EdgeNormalsWithinSurface` |
| `srf/polygon.cpp` / `polygon.cpp` | 924 | OK | `pkg/srf/polygon.go`, `pkg/srf/contour.go`, `pkg/srf/edgelist.go`, `pkg/srf/merge.go` | `AssemblePolygon`/`AssembleContour`, contour orientation (C++ sign convention), `FixContourDirections`, `EdgeCrosses`, `CullExtraneousEdges`, and `STriangle` (`pkg/srf/triangle.go`: `MinAltitude`, `ContainsPoint`, `Raytrace`, `IsDegenerate`, `Transform`; `From` leaves the vertex normals zero) |
| `srf/surfinter.cpp` | 641 | OK | `pkg/srf/surfinter.go`, `pkg/srf/surface.go`, `pkg/srf/merge.go` | `AddExactIntersectionCurve`, every `IntersectAgainst` case (plane-plane, plane-extrusion, coaxial extrusions, exact curves in a plane, the marching general case), `CoincidentWith`, `ContainsPlaneCurve`, `MakeCoincidentEdgesInto`. The branches no fixture reaches are pinned to a C++ harness by `TestIntersectionBranchesVsCpp` |
| `srf/curve.cpp` | 886 | PARTIAL | `pkg/srf/curve.go`, `pkg/srf/bezier_loop*.go`, `pkg/srf/bezier_list.go`, `pkg/srf/boolean_csg.go` | Loop assembly (`FromCurves`, `SBezierLoopSet::From`, `FindOuterFacesFrom`, `GetPlaneContainingBeziers`), `IsCircle`, `GetSurfaceA/B`, `RemoveShortSegments`, `AllIntersectionsWith`, `SBezier::TransformedBy`, `SCurve::FromTransformationOf` faithful |
| `srf/triangulate.cpp` | 738 | OK | `pkg/srf/triangulate.go` | Hole bridging, ear clipping, adaptive grid |
| `srf/ratpoly.cpp` | 790 | OK | `pkg/srf/bezier.go`, `pkg/srf/pwl.go`, `pkg/srf/surface.go` | Adaptive pwl (`MakePwlInto`), `PointOnSurfaces`, `PointOnCurve`, `ClosestPointOnThisAndSurface`, `SSurface::ClosestPointTo`/`ClosestPointNewton`, `TangentsAt`, `NormalAt` (unnormalized, as in C++), `SSurface::PointAt` (C++'s product order, so the same bits), `SBezier::PointAt`/`TangentAt`, `SplitAt` (C++'s `Vector4::Blend`), `PointIntersectingLine`, `EdgeCurveIntersection`. Callers pass `mustConverge`/`retry` as the C++ call sites do |
| `srf/raycast.cpp` | 807 | OK | `pkg/srf/raycast.go`, `pkg/srf/raycast_surface.go`, `pkg/srf/surface.go` | Re-audited: `DepartureFromCoplanar` and `SplitInHalf` (with the weighting and row helpers) were stand-ins; the old `SplitInHalf` was wrong, and cancelled out a `PointIntersectingLine` difference on `boolean_tangent_crossing`. `ClassifyEdge` now treats a surface without a BSP as C++ does (outside). `AllPointsIntersecting`, `ClassifyRegion` checked |
| `srf/merge.cpp` | 130 | OK | `pkg/srf/merge.go` | `MergeCoincidentSurfaces` |
| `bsp.cpp` | 733 | OK | `pkg/srf/bsp.go` | `SBsp3` (`FromMesh` with glibc's `rand()` shuffle, `Insert` with the triangle and quad splits, `InsertConvex`, `InsertInPlane`, `GenerateInPaintOrder`) and `SBsp2`, re-ported: the earlier port pushed straddling triangles down both sides unsplit |
| `constrainteq.cpp` | 1062 | OK | `pkg/core/constraint_eq.go`, `pkg/sketch/group_generate.go` | `GenerateEquations` for every type, `ModifyToSatisfy`, `HasLabel`, `Generate` and the static helpers (`Distance`, `PointLineDistance`, `PointPlaneDistance`, `DirectionCosine`, `PointInThreeSpace`, `VectorsParallel3d`) |
| `modify.cpp` | 751 | PARTIAL | `pkg/sketch/edit.go` | `ReplacePointInConstraints`, `RemoveConstraintsForPointBeingDeleted`, `FixConstraintsForRequestBeingDeleted`/`PointBeingDeleted` are ported. The editing tools (tangent arc, splitting lines and curves) aren't |
| `solvespace.cpp` | 1236 | OK | `pkg/sketch/sketch.go`, `pkg/sketch/group.go` | `Refresh`, the unsaved flag; the `Add*` helpers assign handles as `IdList::AddAndAssignId` does (largest plus one; they used the count). Observer notifications are out-of-scope (UI). |
| `undoredo.cpp` | 157 | OK | `pkg/sketch/undo.go` | Re-ported: states hold what defines the sketch (groups without their generated data, group order, requests, constraints, params, styles), stacks are `MAX_UNDO` (100) deep, and a pop reloads linked files and regenerates. The active group is derived from the group order; the unsaved flag and menus are the caller's |
| `mesh.cpp` | 1221 | OK | `pkg/srf/mesh.go`, `pkg/srf/kdnode.go`, `pkg/srf/outline.go` | `SMesh` (the BSP Booleans via `AddAgainstBsp` and `Simplify`, `MakeFromTransformationOf`, `MakeEdgesInPlaneInto`, `CalculateVolume`, `GetCenterOfMass`, ...), `SKdNode` (`From` copies the triangles and shuffles them with glibc's `rand()`; `SnapToMesh`, `FindEdgeOn`, `MakeCertainEdgesInto`, `MakeOutlinesInto`, `OcclusionTestLine`) and `SOutline`. `FirstIntersectionWith` takes the ray, which C++ makes from the view. `PrecomputeTransparency` sorts stably; C++'s `std::sort` may order ties differently, which only affects drawing |
| `file.cpp` | 1030 | OK | `pkg/persist/slvs/slvs.go`, `pkg/persist/slvs/linked.go`, `pkg/sketch/upgrade.go` | Loading follows `LoadFromFile`: the saved entities are dropped, `ReloadAllLinked` reads each linked .slvs file's entities, styles, mesh and shell (`LoadEntitiesFromSlvs`) and forces a mesh-only link to a mesh, and `UpgradeLegacyData` regenerates the sketch (REGEN) and seeds legacy constraint params. Saving follows `SaveToFile` (regenerate, then each entity's export numerics and `Entity::IsVisible` with the default view, then the last group's running mesh and shell), and matches current C++ byte for byte on the fixtures, shells included. A missing linked file counts as declined (C++ asks the user to find it); STL and IDF links aren't ported, nor is loading linked images' pixels |
| `util.cpp` (geometry helpers) | — | OK | `pkg/geom` | `Vector::Equals` uses `LENGTH_EPS`, `OnLineSegment`, `BoundingBoxIntersectsLine`, `BandedMatrix`, plane intersections, `AtIntersectionOfLines`, `DistanceToLine`/`DistanceToPlane`, `DotInToCsys`/`ScaleOutOfCsys`, `WithMagnitude`, `RotatedAbout` (both overloads; Go's went through a quaternion), `DirectionCosineWith`, `ClosestPointOnLine`, `DivProjected`, `ClampWithin`, `Vector4` (`From(w, v)` premultiplies, `Blend`), `Point2d` (`Equals` with `LENGTH_EPS`, `Angle`, `FromPolar`, `DistanceToLineSigned`), `BBox` (`GetOrigin` is the centre; `Overlaps`, `Contains`), `Quaternion::From(axis, angle)`, `Times`, `Inverse`, `Mirror` (was wrong), `ToThe`, `Quaternion::Rotate`/`RotationU/V/N`/`From(u, v)`; `geom.Max`/`Min` are `std::max`/`min` |

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

- 423 tests (334 top-level + subtests) across 6 engine packages (`pkg/core`, `pkg/geom`,
  `pkg/persist/slvs`, `pkg/sketch`, `pkg/solver`, `pkg/srf`).
- `TestSLVSLoadRoundTrip` and `TestPhase7AllFixturesRoundTrip` load
  and save every fixture and require the bytes current C++ writes
  (checked with a C++ harness); the exceptions, with reasons, are in
  `roundTripExclusions`, mostly files older versions wrote.
- `TestParityGroupSolveVsCpp` runs `GenerateAll` on every fixture C++
  can load and requires every group to solve with the degrees of
  freedom C++ reports (`cpp_dof_golden_test.go`), every group's shell
  to carry C++'s face entities (`cpp_faces_golden_test.go`), and every
  constraint's equations to hold.
- `TestArcLengthConstraints` covers the four arc-length constraints no
  fixture uses.
- `TestIntersectionBranchesVsCpp` pins booleans through the surface
  intersection cases no fixture reaches (coaxial cylinders, crossing
  cylinders, a lathe cut on its seam plane) to a C++ harness: volume,
  surface, curve and triangle counts.
- `TestParityBooleanFixtures` requires every boolean fixture's mesh to
  be watertight, as the C++ fixture tests do: `MakeCertainEdgesInto`
  (`NAKED_OR_SELF_INTER`, `coplanarIsInter`) finds no naked or
  self-intersecting edges.
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
  `TestParityGroupSolveVsCpp`.
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
It dumped the chord tolerance, each group's solve result and degrees of
freedom, each constraint's value, and every entity's handle, type,
group, construction flag, point, normal, face and distance. For all
90 fixtures C++ can load, the Go `GenerateAll` output matches exactly
(to 1e-4).

**Save check against C++.** A second harness saved each fixture with
`SaveToFile`, which appends the last group's running shell. Go's save
is byte-identical to it on every fixture, boolean shells included,
except the few that differ in a solved value's last digits
(`same_orientation`, `negative_dia`, `large_dimensions`) and the image
fixtures (C++ relativizes the image path). Getting the shells
identical took these faithful ports:

- `List::AddToBeginning` doesn't insert: its `std::move_backward`
  moves only the first element, to the end, and then overwrites the
  first. `FindChainAvoiding` relies on it, so the order of every
  boolean surface's trims does too; Go reproduces it.
- `SPointList` (the choosing points) matches points with
  `Vector::Equals`, in list order; Go had used a map and a 1e-8
  distance.
- `std::max`/`std::min` keep the first argument for zeros of either
  sign, unlike `math.Max`/`math.Min`; ported code uses `geom.Max`/`Min`.
- `SShell::MakeFromCopyOf` is a transformation by the identity, and
  `SBezier::TransformedBy` always rotates and translates, which turns
  `-0` into `+0`. `SSurface::FromTransformationOf` keeps the handle and
  transforms the trims; `MakeFromAssemblyOf` assigns new handles.
- `Vector::WithMagnitude` returns zero only for an exactly zero vector.

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

**Smaller stand-ins:** none known in the files audited so far. The
entity and save checks cover every fixture (including
`group/force_to_mesh`, whose saves carry C++'s running meshes).

**Not yet re-audited:** the rest of `srf/curve.cpp`. Given what the audits so far found, expect stand-ins
there, especially in code no fixture reaches.
