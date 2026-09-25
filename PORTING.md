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
