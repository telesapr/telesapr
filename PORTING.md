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
| `groupmesh.cpp` | 803 | PARTIAL | `pkg/sketch/group_pipeline.go` | Per-group mesh gen; basic path works |
| `request.cpp` | 250 | OK | `pkg/sketch/request.go` | |
| `style.cpp` | 948 | OK | `pkg/core/style.go` | |
| `clipboard.cpp` | 527 | OK | `pkg/sketch/clipboard.go` | |
| `srf/surface.cpp` | 494 | OK | `pkg/srf/surface.go` | |
| `srf/shell.cpp` | 617 | OK | `pkg/srf/shell.go`, `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go` | |
| `srf/boolean.cpp` | 1100 | OK | `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go` | |
| `srf/polygon.cpp` | 924 | OK | `pkg/srf/polygon.go` | |
| `srf/surfinter.cpp` | 641 | OK | `pkg/srf/surfinter.go` | |
| `srf/bezier.cpp` | — | OK | `pkg/srf/bezier*.go` | |
| `srf/curve.cpp` | 886 | PARTIAL | `pkg/srf/curve.go` | `SCurve::FromTransformationOf` ported |
| `srf/triangulate.cpp` | 738 | PARTIAL | `pkg/srf/contour.go` | `BridgeToContour` ported; ear-clipping exists |
| `srf/ratpoly.cpp` | 790 | PARTIAL | `pkg/srf/bezier.go` | `PointOn*Curve`, `ClosestPointBetweenLines` ported |
| `srf/raycast.cpp` | 807 | MISSING | — | Surface ray-cast; needed for non-trivial CSG |
| `srf/merge.cpp` | 130 | PARTIAL | `pkg/srf/shell.go` | `MergeCoincidentSurfaces` exists |
| `bsp.cpp` | 733 | PARTIAL | `pkg/srf/bsp3.go`, `pkg/srf/bsp2.go` | Most methods; `InsertHow`/`InsertInPlane` not ported |
| `constrainteq.cpp` | 1062 | PARTIAL | `pkg/sketch/group_generate.go` | LINKED/ROTATE/REVOLVE emit equations; others TODO |
| `modify.cpp` | 751 | PARTIAL | — | Geometric modifiers (translate/rotate/skew) partial |
| `solvespace.cpp` | 1236 | PARTIAL | — | Kernel state; `Clean`, observer notifications partial |
| `undoredo.cpp` | 157 | OK | `pkg/sketch/undo.go` | Just added |
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

The Go port's GUI (in `pkg/app/` and `pkg/spacesolve/`) is a
separate Cogent-Core-based SVG editor that does NOT mirror the C++
UI.

## Test counts

- 277 tests across 6 engine packages.
- All 6 boolean fixtures (`boolean_coplanar_union`, `boolean_knife_edge`,
  `boolean_tangent_crossing`, `boolean_tangent_edge`,
  `boolean_tangent_fillet`, `boolean_tangent_spline`) load and
  complete `Sketch::GenerateAll` without panic.
- All 86 non-boolean fixtures load cleanly and round-trip via
  `TestSLVSLoadRoundTrip` and `TestPhase7AllFixturesRoundTrip`.
- 0 TODO markers in engine packages.

## Path to full parity

The remaining work to reach "true parity" (i.e., every C++
engine function has a Go equivalent) is concentrated in:

1. **`srf/raycast.cpp` (807 lines)** — surface ray-cast. Required for
   accurate point-in-mesh tests and BSP degenerate handling.
2. **`constrainteq.cpp` (1062 lines)** — full per-constraint equation
   emission. Currently only LINKED, ROTATE, REVOLVE actually call
   `sys.AddExprEquation`; the other ~15 constraint types emit equations
   via simpler paths.
3. **`modify.cpp` (751 lines)** — full geometric modifiers (rotate,
   scale, mirror, copy-all) needed for interactive constraint
   manipulation.
4. **`srf/triangulate.cpp` (738 lines) full ear-clipping** — current
   `BridgeToContour` covers polygons with holes; full ear-clipping
   for arbitrary non-convex contours remains.
5. **`groupmesh.cpp` (803 lines) full per-group-type mesh gen** —
   current `GenerateShellAndMesh` works for EXTRUDE/REVOLVE/HELIX;
   LINED (step-and-repeat) goes through the assembly path.
6. **`srf/curve.cpp` (886 lines) full SCurve ops** — only
   `FromTransformationOf` and `GetSurfaceA/B` of the C++ API are
   ported; `RemoveShortSegments`, `ClosestPoint`, etc. need their
   Go equivalents.
7. **`solvespace.cpp` (1236 lines) full kernel state** — observer
   notifications and dirty-flag propagation are partial.

Estimated effort: another ~3-4k lines of Go code, depending on test
depth. None of this work is blocked — the existing engine handles
all 277 tests, including the 6 boolean fixtures.

## Recent milestones (Phase E boolean + constraint + analysis parity)

### SLVS parser entity-group association (commit de5354b)

The SLVS file format does not store `Entity.group.v`. The Go port's
parser used to leave every entity's `Group` field at the zero value,
so `AssembleLoops`'s filter `ent.Group == g.H` never matched and
every mesh group produced empty meshes. The fix is a second-pass
method `associateEntitiesWithGroups` that maps each entity's
`Entity.workplane.v` to the group whose `Group.activeWorkplane.v`
matches. With this fix:

- All 6 boolean_* fixtures produce non-empty meshes:
  - boolean_coplanar_union:   12288 triangles
  - boolean_knife_edge:        6656
  - boolean_tangent_crossing: 5120
  - boolean_tangent_edge:     6656
  - boolean_tangent_fillet:    4608
  - boolean_tangent_spline:    5120

- Phase 5 of `GenerateAll` was updated to walk `g.OpA` lineage for
  each mesh group, running `AssembleLoops` on every upstream group
  so `BezierLoops` propagate from the source Drawing down to the
  mesh group.

- Round-trip parity is preserved by recording which entities had
  an explicit `Entity.group.v` in the source file and only emitting
  the field on save when it was originally present.

### Phase E parity tests added (commits 76fa83e, 77ff521, 203f3a8, 1c2af44)

- `TestParityConstraintFixtures`: 31/31 constraint fixtures load,
  round-trip, and solve with `How=SolveOkay` across all groups.
- `TestParityConstraintCounts`: structural parity gate asserting each
  constraint fixture's group/entity/constraint counts.
- `TestParityBooleanFixtures`: asserts each boolean fixture produces
  the recorded triangle count.
- `TestParityBooleanFixtureBBoxValue`: asserts each boolean fixture
  produces the recorded bounding box.
- `TestParityAnalysisFixtures`: structural parity gate for the
  analysis fixture family.
- `TestContourAreaSignAndMagnitude`: computes the shoelace area of
  the contour_area fixture (400 = 20x20 square, exactly correct).

### Phase C unit tests added (commit 0cabadd)

- `TestSystemAtMidpointPointOnLineConstraint`: verifies the
  at-midpoint equation reads `point - 0.5*(start + end) = 0`.
- `TestSystemSameOrientationConstraint`: verifies parallel line
  segments with parallel plane normals emit zero residuals.
- `TestSystemArcArcLenRatioConstraint`: verifies the arc-arc
  length ratio equation for two concentric circles with known
  radius ratio.

### Test counts (post-Phase E)

- Before Phase E: 277 tests
- After Phase E: 290 tests (+13)

### Remaining for full parity

Still pending for true C++ parity:

1. **Phase B**: `srf/raycast.cpp` (807 lines), `srf/triangulate.cpp`
   full ear-clipping (738 lines), `srf/curve.cpp` full SCurve ops
   (886 lines), `bsp.cpp` `InsertHow`/`InsertInPlane` (~300 lines).
2. **Phase C**: `modify.cpp` full `Constraint::ModifyToSatisfy` for
   `PT_PT_DISTANCE`, `PT_ON_LINE`, `ARC_ARC_DIFFERENCE`, etc. —
   currently stubbed because the Go port's solver recomputes valA
   every GenerateAll, so reference dimensions remain accurate
   without the per-constraint modify path.
3. **Phase C**: `groupmesh.cpp` full per-group-type mesh gen paths
   (currently using simplified helpers that handle the common
   EXTRUDE/LATHE/REVOLVE/HELIX cases).
4. **Phase E**: golden values from a C++ binary for true parity
   (the current recorded baselines are Go-port self-consistency
   checks, not C++ reference values).
