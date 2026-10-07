# Neosolve vs TeleSAPR Feature Comparison

Date: 2026-10-07

Source: https://github.com/dodox1/neosolve (a modern SolveSpace fork with OpenCASCADE)

---

## Executive Summary

TeleSAPR has **most** of neosolve's features, and many that neosolve doesn't. TeleSAPR uses its own
Go-native B-spline kernel (degree up to 5) rather than OpenCASCADE, with comparable solid modeling
capabilities.

---

## What TeleSAPR Already Has (Equal or Better)

### Solid Modeling Operations

| Operation | Neosolve (OCC) | TeleSAPR (Native) |
|-----------|-----------------|--------------------|
| Extrude | Yes | Yes (with draft angles, two-sided, selected sides) |
| Lathe | Yes | Yes (full 360° revolution) |
| Revolve | Yes | Yes (partial angle, two-sided) |
| Helix | - | Yes (coil springs) |
| Sweep | Yes | Yes (along lines/arcs/cubics, with twist, RMF, mitres, named edges) |
| Loft | Yes | Yes (ruled or smooth B-spline; end tangency, twist offsets, 1-2 guide curves) |
| Mirror | Yes | Yes (source or whole solid; union optional) |
| Chamfer | Yes | Yes (single distance or two distances; named edges) |
| Fillet | Yes | Yes (multiple radii, setback corners, uneven mitres, rolling-ball between curved faces) |
| Shell | Yes | Yes (flat faces + curved prisms; general curved solids = item 22.1 roadmap) |
| Draft | - | Yes (neutral plane, selected sides) |
| Pattern/Step | Yes | Yes (step translating, step rotating, pattern along path including closed paths) |
| Screw Thread | - | Yes (metric M, unified UNC/UNF, trapezoidal Tr; external/internal, multi-start) |

### Import/Export

| Format | Neosolve | TeleSAPR |
|--------|----------|----------|
| STEP | Import/Export | Import (exact solid + reference mesh), Export (assemblies, per-face B-spline) |
| STL | - | Import (link), Export (binary/text) |
| 3MF | - | Export (with colors) |
| OBJ | - | Import (link), Export (with materials/.mtl) |
| VRML | - | Export |
| DXF | - | Export (2D views/drawings) |
| SVG | - | Export |
| PDF | - | Export (drawings) |
| EPS/PS | - | Export |
| HPGL | - | Export |
| PNG | - | Export (rendered views from any direction) |
| BOM CSV | - | Export (nested links counted properly) |
| G-code | Export (with G02/G03 arcs) | Export (facets to G01 lines only) |
| IGES | Import/Export | **Not supported** |
| BREP | Import/Export | Via STEP |

### Advanced Features

- **Named parameters and expressions**: Both have it. TeleSAPR additionally has configurations.
- **Motion study**: TeleSAPR can drive parameters through ranges and export meshes/images for animation.
- **History editing**: TeleSAPR can reorder groups with dependency checking, suppress groups, insert mid-history.
- **Assembly links**: TeleSAPR can link `.tlspr`, `.stl`, `.obj`, `.step` files; has mates (concentric/flush/hinge),
  interference detection, bill of materials with nested counting.
- **Drawing sheets**: TeleSAPR has full 2D documentation: standard views, section/detail views, associative
  dimensions on isometric/section/detail views, title block.
- **AI/MCP scripting**: TeleSAPR has an embedded Go interpreter (yaegi) and an MCP server for AI-driven modeling.
  Neosolve has no equivalent.
- **Dual kernels**: TeleSAPR's native kernel operates independently; neosolve falls back to OpenCASCADE when its
  native kernel can't handle an operation.

---

## What TeleSAPR Is Missing

### Constraint Solver Additions (Highest Priority)

| Feature | Neosolve | TeleSAPR | Notes |
|---------|----------|----------|-------|
| **Inequality Constraints** | Min/max distance (≥ and ≤) | Only exact equality | Needs 2 slack equations per constraint, like existing inequality mechanisms |
| **Point-on-Segment** | Bounded parameter t ∈ [0,1] | Only PtOnLine (infinite line) | Also needs slack equations; equations exist per neosolve notes |

### G-code Export (Localized Change)

| Feature | Neosolve | TeleSAPR | Location |
|---------|----------|----------|----------|
| **Arc Commands** | G02/G03 for arcs | All curves faceted to G01 lines | `pkg/ui/exportvector.go:654-713` |

Neosolve approximates Bezier curves to biarcs for G-code export. TeleSAPR's `gcodeFileWriter.Bezier()` only
handles degree-1 curves (lines) and falls back to piecewise-linear for everything else.

### Format Support

| Format | Neosolve | TeleSAPR | Effort |
|--------|----------|----------|--------|
| IGES | Import/Export | Not supported | Low if already handling STEP; low priority |

### UI Conveniences (Neosolve "Improvements")

These are quality-of-life UI features, not kernel capabilities:

| Feature | Neosolve | TeleSAPR |
|---------|----------|----------|
| Dockable property browser | Yes (GTK) | Fixed browser |
| Directional marquee selection | Window (left→right) vs Crossing (right→left) | Behavior unknown; check selection code |
| Arrow key nudging | Move selected entities with arrow keys | Not found in search |
| Double-click text editing | Inline TTF text editing | Unknown |
| Independent visibility toggles | Separate controls for REF constraints and comments | Unknown |
| Gruvbox-inspired themes, Qt dark theme | Yes | Has appearances (matte/plastic/glossy/metal); theme system? |

### CLI and Tooling

| Feature | Neosolve | TeleSAPR |
|---------|----------|----------|
| Built-in profiler | `--profile` with JSON export | Benchmarks exist (`BenchmarkFillet*` etc) but no CLI flag |
| Targeted group export | `--group` option | `telesapr-cli` export; check if it filters groups |

---

## Neosolve Design Philosophies Not Applicable to TeleSAPR

Neosolve frames itself as "OCC as a bridge while native kernel matures." TeleSAPR already has a mature
Go-native kernel that handles fillets between curved faces (rolling ball), smooth lofts with guide curves,
sweeps with twist and rotation-minimizing frames, etc. The comparison is not "when will TeleSAPR get OCC?"
but "what small solver/export features are missing?"

Neosolve's dual-kernel approach (native + OCC fallback) solves a SolveSpace limitation that TeleSAPR
does not have.

---

## Open Work / Follow-up Candidate Items

From `ROADMAP-JUMP.md#Open work` and this survey:

1. **Inequality constraints** (≥ and ≤ distance) - Solver addition, highest value per neosolve
2. **Point-on-segment constraint** (bounded t ∈ [0,1]) - Related to above
3. **G02/G03 arc export in G-code** - Localized to `exportvector.go`
4. **IGES support** - Low priority unless requested
5. **UI conveniences** - Marquee direction, arrow nudge, docking - usability but not capability

---

## References

- Neosolve README: https://github.com/dodox1/neosolve (forked from 0xSeren/neosolve)
- Neosolve wishes: `wishlist.txt` in their repo (not examined)
- TeleSAPR roadmap: `ROADMAP-JUMP.md`
- TeleSAPR existing notes pattern: `docs/notes/brlcad-survey.md`
- G-code export location: `pkg/ui/exportvector.go:654` ("G code: slightly complicated...")
