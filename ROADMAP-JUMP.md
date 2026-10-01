# Roadmap: from a SolveSpace port to a modern CAD

Date: 2026-10-01. Status: roadmap and estimates, nothing here is started.

## Context

spacesolve began as a Go port of SolveSpace, bit-identical with the C++. As of 2026-10-01 it
is a separate project: parity and file compatibility with SolveSpace are no longer goals, and
the kernel stays Go native. The parity record that `PORTING.md` held is in the git history.

This document lists what the program lacks compared with current parametric CAD, sizes each
item against the code as it stood on that date, and gives the order to do them in. It is an
estimate, not an implementation plan. Each item still needs its own design pass before coding.

## How to read the estimates

- **Effort** is focused weeks for one developer who already knows this codebase,
  including tests and UI. The port itself has moved much faster than such numbers
  suggest, so read them as relative weights, not calendar promises.
- **Size**: S under 1 week, M 1 to 3 weeks, L 3 to 8 weeks, XL more than 8 weeks or open ended.
- **Risk** is the chance that the estimate is wrong by more than a factor of two.
- **New code** is a rough count of non-test Go lines.

## What the code base gives us, and what it withholds

Findings from the survey that drive every estimate below.

**Already present, so not a gap in this port**

| Capability | Where |
|---|---|
| Scripting API with console (yaegi Go interpreter) | `pkg/script`, `pkg/ui/scriptconsole.go` |
| STEP export of NURBS shells | `pkg/ui/exportstep.go` |
| STL and OBJ mesh export, DXF/SVG/EPS/PDF vector export | `pkg/ui/export.go`, `pkg/ui/exportvector.go` |
| Section export and hidden-line vector export | `ExportSectionTo` in `pkg/ui/export.go` |
| Interference check, naked edges, volume, centre of mass | `pkg/ui/menuanalyze.go` |
| Free-parameter marking per point and normal | `System.MarkParamsFree` in `pkg/solver/system.go`, drawn in `pkg/ui/drawentity.go` |
| List of constraints to remove when a sketch is overconstrained | `System.FindWhichToRemoveToFixJacobian` |
| Expression parser with an identifier node | `OpVariable`, `NewVariable`, `Eval(ctx, vars)` in `pkg/core/expr.go` |
| Group suppress flag, mirrored mesh transform | `Group.Suppress`, `pkg/srf/mesh.go:344` |
| Threaded Booleans | `pkg/srf/parallel.go` |
| Extension gate that keeps C++ parity tests clean | `SolveSpaceUI.Extensions` |
| Go-only feature precedent with goldens | screw threads, `testdata/thread` |

**Structural limits of the kernel (`pkg/srf`, about 11,000 lines)**

1. **One surface is one rational Bezier patch of degree 3 by 3 at most.** `SSurface` holds a
   fixed `[4][4]` control net and no knot vectors. Planes, extrusions and revolutions fit exactly.
   Rolling-ball fillets on general edges, offsets of cubic curves, and sweeps along space curves
   do not. They need approximation by several patches, and the kernel has no notion of
   a multi-patch face with smooth internal seams.
2. **No explicit topology.** A shell is a list of surfaces plus a list of trim curves. A curve
   knows its two surfaces, so adjacency can be derived, but there are no vertex or edge objects,
   no loops with orientation as first-class data, and no local edit operations. Every feature
   today is "build a new shell, then Boolean it".
3. **Intersection has three exact special cases and one numeric fallback.** Plane with plane,
   plane with extrusion, and coaxial extrusions are exact. Everything else goes through
   `intersectByMarching`, which starts only from points where one surface's boundary pierces
   the other. A closed intersection loop that touches no boundary is missed. Tangent contact
   is not handled.
4. **Classification leans on raycasts with chord-tolerance weaknesses.** The thread work
   documented one concrete failure in `ClassifyEdge` near curved trims and worked around it
   by retrying with rotated sketch planes.
5. **Solid edges have no stable name.** Faces carry a `FaceID` traced to a sketch entity.
   Edges exist as entities only where a group generated them, such as the swept lines of an
   extrusion. Edges born from a Boolean have none. Any feature that asks the user to pick
   an edge of the solid needs this solved first.

**Project constraints** (the first two were lifted on 2026-10-01, see "Decisions taken")

- Byte-identical parity with C++ must hold with extensions off. Kernel fixes that change
  output need a gate or a mode, as the parallel "exact" and "fast" modes already do.
- New group types and new save-file records make files that upstream SolveSpace cannot open.
  The recent loader change means unknown lines are flagged and the rest loads, so the damage
  is graceful but real. This is one policy decision that covers almost every item below.
- Expressions are evaluated at entry and only the number is stored. `ConstraintBase` has
  `ValA float64` and no expression text.
- Rendering is software only. The OpenGL spike passed but the backend is not built.
- Regeneration runs on the UI thread.

## Estimates by area

### A. Parameters and expressions

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| A1. Named parameter table, expressions kept on dimensions and group values | M | 2 wk | 1,200 | Low |
| A2. Configurations, meaning named sets of parameter values | S | 1 wk after A1 | 400 | Low |
| A3. Dimensions referencing other dimensions by name | S | 0.5 wk after A1 | 200 | Low |

A1 work list: a parameter list on the sketch with save and load records. An expression string
beside `ValA` on constraints and beside `ValA`, `ValB`, `ValC` on groups. Re-evaluation at the start
of `GenerateAll` with cycle detection. A text-window screen to edit the table. Undo coverage.
The parser needs no change. Best ratio of user value to effort in the whole report.

### B. Sketch feedback

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| B1. Colour whole entities by constraint state, not only free points | S | 0.5 wk | 300 | Low |
| B2. Highlight conflicting constraints in the canvas with a one-click jump | S | 0.5 wk | 250 | Low |
| B3. Auto-constrain on draw, with more inference than horizontal and vertical | M | 1.5 wk | 700 | Medium |

The solver already produces the data. B1 has to run `MarkParamsFree` after every solve instead
of on demand, which costs a rank computation per group. That needs measuring on large sketches.

### C. History and the feature tree

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| C1. Reorder groups with dependency validation | M | 1.5 wk | 700 | Medium |
| C2. Suppress a whole group including its sketch, not just the solid | S | 0.5 wk | 250 | Low |
| C3. Broken-reference diagnostics that name the missing entity and the group that owned it | M | 1.5 wk | 600 | Medium |
| C4. Insert a group in the middle of the history | S | 0.5 wk after C1 | 200 | Low |

Handles derive from the group handle, not from position, so reordering is a change to `Order`
plus a check that no group moves ahead of something it references. The risk is in the `Remap`
tables of step-and-repeat and link groups.

### D. Interchange

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| D1. 3MF export | S | 0.5 wk | 350 | Low |
| D2. STEP import as a display mesh with pickable points | L | 3 to 4 wk | 3,000 | Medium |
| D3. STEP import as a NURBS shell usable in Booleans | XL | 8 to 12 wk | 5,000 | High |
| D4. STL and OBJ import as reference mesh | S | 1 wk | 500 | Low |
| D5. STEP export with assembly structure and per-part colours | M | 1 wk | 500 | Low |

D2 breaks down as a Part 21 tokenizer and entity graph, a mapping of the AP203 and AP214
boundary-representation entities, conversion of analytic surfaces and B-splines into something
that can be tessellated, and a link-group style entry point that fills `ImpMesh` and `ImpEntity`.
A standalone tessellator for trimmed STEP faces avoids forcing every surface into the 4 by 4 patch.

D3 is hard because of kernel limit 1. B-spline surfaces must be split at knots into Bezier
patches, and anything above degree 3 must be approximated. Real vendor files are full of
degree 5 surfaces, tiny sliver faces and tolerant edges that this kernel's fixed epsilons will reject.
Expect a long tail of files that import but fail the first Boolean. Recommend D2 first and
treating D3 as research.

### E. Solid features

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| E1. Mirror group | M | 1.5 wk | 900 | Medium |
| E2. Pattern along a curve | M | 1.5 wk | 700 | Low |
| E3. Chamfer, straight edges between planar faces | M | 2 wk | 1,000 | Medium |
| E4. Fillet, straight edges between planar faces, constant radius | L | 3 to 4 wk | 1,800 | High |
| E5. Fillet and chamfer on circular edges, plane meeting cylinder or cone | L | 4 to 5 wk | 2,000 | High |
| E6. Fillet on general edges, with vertex blends | XL | 4 to 6 months | 8,000+ | Very high |
| E7. Loft between two profiles, ruled | M to L | 3 wk | 1,500 | Medium |
| E8. Loft through several sections, smooth | L | 5 wk more | 2,000 | High |
| E9. Sweep along a planar path of lines and arcs | M | 2 wk | 1,000 | Medium |
| E10. Sweep along a general 3D curve | L | 4 to 5 wk | 2,000 | High |
| E11. Shell, for solids made of planes and cylinders | L | 5 to 6 wk | 2,500 | High |
| E12. Shell, general | XL | open ended | n/a | Very high |
| E13. Draft angle on extrusion | L | 3 to 4 wk | 1,500 | High |

Notes on the items that matter most.

**E1 Mirror.** The shell transform accepts a scale and the mesh side already handles negative
scale. The missing part is entity copies. There are translated and rotated copy entity types but
no reflected one, so new point and normal entity types and their solver expressions are needed.

**E3 and E4, the first fillets.** Two possible routes.

- *Boolean route.* Build a cutter, a prism for a chamfer or a prism minus a cylinder for a fillet,
  and subtract it. It reuses everything. For a chamfer it is sound. For a fillet the cutter is
  tangent to both faces along its whole length, which is exactly the tangent and coincident case
  the Boolean handles worst. Expect it to work on a cube and fail on real parts.
- *Local edit route.* Trim the two faces back along offset lines, insert a cylinder patch,
  and patch the end faces. It is robust for the supported cases and it is the only route that
  scales to E5 and E6. It needs the derived-topology layer described in F3.

Either route first needs edge picking, which is F4. Recommendation: chamfer by Boolean,
fillet by local edit, and count F3 and F4 as part of the fillet cost. With those included
the first usable fillet is about 7 to 9 weeks, not 3 to 4.

**E6 General fillets.** This is what commercial kernels spent decades on. The blend surface is
not representable in the patch type, so it needs multi-patch approximation, G1 seams, setback
corners where three edges meet, and handling for blends that run off a face or swallow a small one.
I would not commit to it. E4 and E5 cover most machined-part needs.

**E7 Loft.** A ruled loft between two profiles with matching segment counts is exact in the
current patch type, since each face is degree 1 in one direction. The effort is in the UI for
picking two sketches and matching vertices, and in the fact that these faces hit the marching
intersector instead of the exact special cases. Expect Boolean failures that plain extrusions
never show.

**E11 Shell.** Offsetting a plane or a cylinder is exact. The restricted version offsets each
face, re-intersects neighbours, and removes the picked faces. It needs F3. The alternative,
subtracting an offset copy of the solid, pushes all the difficulty into building that offset copy.

### F. Kernel foundations

These are not user-visible features. They are what the E items stand on.

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| F1. Retry or refine raycasts that land within chord tolerance of a curved trim | S to M | 1 wk | 300 | Medium |
| F2. Find closed intersection loops that cross no boundary | M | 2 wk | 600 | Medium |
| F3. Derived topology layer with vertices, edges, loops, adjacency queries and local edit helpers | L | 3 to 4 wk | 2,000 | Medium |
| F4. Stable edge naming and edge picking on the solid | L | 3 wk | 1,200 | High |
| F5. Tangent and coincident contact handled in general | XL | open ended | n/a | Very high |
| F6. Multi-patch faces, or a real B-spline surface type | XL | 8+ wk | 4,000 | High |
| F7. Regeneration off the UI thread, with cancel | M to L | 3 wk | 1,000 | Medium |
| F8. Parallel `MergeCoincidentSurfaces`, the known next bottleneck | M | 1 wk | 400 | Low |

F1 was already identified during the thread work and would remove the rotate-and-retry
workaround. F4 is the topological naming problem. A workable scheme names an edge by the pair
of face identities it separates plus an ordinal along that pair. It is stable for most edits and
degrades visibly, not silently, when a face disappears. F5 is the one that never finishes.
Budget it as a steady stream of fixtures and fixes, not as a project.

All of F1, F2 and F5 change Boolean output. With parity dropped they ship on by default,
but they need the invariant-based test oracle (T1) in place first.

### G. Assemblies

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| G1. Mate shortcuts built from existing constraints, such as concentric, flush, hinge | M | 2 wk | 900 | Low |
| G2. Bill of materials from link groups | S | 0.5 wk | 250 | Low |
| G3. Interference check between linked parts using shells, not triangles | S | 1 wk | 400 | Medium |
| G4. Large-assembly performance through lightweight links and cached meshes | L | 4 wk | 1,500 | Medium |
| G5. Motion study, driving one dimension through a range | M | 1.5 wk | 600 | Low |

The solver already positions linked parts with ordinary constraints, so G1 is user interface
over what exists. G5 becomes almost free once A1 lands, since it is a parameter sweep.

### H. Drawings

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| H1. Sheet with standard views placed automatically, exported to PDF and DXF | M to L | 3 wk | 1,500 | Medium |
| H2. Associative dimensions placed on drawing views | L | 4 to 5 wk | 2,500 | High |
| H3. Section and detail views on the sheet, with hatching | M | 2 wk after H1 | 900 | Medium |
| H4. Title block and templates | S | 1 wk | 400 | Low |

Hidden-line projection, section cuts and vector writers exist. H1 composes them. H2 is the
expensive part because a dimension must attach to projected geometry and survive regeneration,
which is the edge naming problem again, in 2D.

### I. Rendering and user experience

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| I1. OpenGL backend, fixed-function, as spiked | M | 1.5 wk | 1,000 | Low |
| I2. Shader-based backend with smooth shading, materials and environment lighting | M to L | 3 wk after I1 | 1,500 | Medium |
| I3. Live section view with capped cut | M | 1.5 wk | 600 | Medium |
| I4. Measure mode | S | 0.5 wk | 300 | Low |
| I5. Command search palette | S | 0.5 wk | 300 | Low |
| I6. Per-face and per-part appearance beyond a colour | S | 1 wk after I2 | 400 | Low |

I5 is cheap because the menu is already a data table in `pkg/ui/menu_table.go`.
I3 in the software renderer is possible but slow. It belongs after I1.

### J. Out of scope

Sheet metal, direct push-pull modelling, tolerancing annotations, CAM and simulation.
Each is an XL product in its own right and none is needed to count as a modern part modeller.

## Totals by tier

| Tier | Items | Effort |
|---|---|---|
| 1. Cheap, high value, no kernel work | A1 to A3, B1, B2, C1 to C4, D1, D4, G2, I4, I5 | about 10 wk |
| 2. Moderate, bounded | E1, E2, E3, E9, D2, D5, G1, G3, G5, H1, H4, I1, I3, F1, F8 | about 24 wk |
| 3. Kernel-dependent features | F2, F3, F4, E4, E5, E7, E11, E13, F7, H2, H3, I2, I6 | about 40 wk |
| 4. Research grade | E6, E8, E10, E12, D3, F5, F6, G4 | more than a year, no firm bound |

Tiers 1 to 3 together are roughly 18 months of one developer at conventional pace.
Tier 4 is where the estimate stops being meaningful.

## Decisions taken (2026-10-01)

The user decided all three open questions.

- **The kernel stays Go native.** No binding to OpenCASCADE or any other C++ kernel.
- **Parity with C++ SolveSpace may be dropped.** Kernel output no longer has to match bit for bit.
- **File compatibility with upstream SolveSpace may be dropped.** Reading old `.slvs` files stays worthwhile. Writing files upstream can open is no longer a goal.

### What the decisions change

**Costs that disappear**

- No gating. Kernel fixes F1, F2 and F5 ship on by default, with no mode switch and no second code path. That removes roughly a fifth of the effort and most of the test burden from each.
- New group types, parameter tables and expression strings go straight into the save format. No sidecar files, no degrading to stock groups.
- The parallel "fast" mode can become the only mode, and F8 no longer has to reproduce serial ordering.
- Bit-exact scaffolding stops constraining design: `CxxSort`, the ported libm, the FMA build rules. It can stay where it is harmless, but new code need not respect it.

**Costs that appear**

- **T1. A new test oracle, 1 to 1.5 weeks.** Today correctness means "same bytes as C++". Once the kernel diverges, the goldens become frozen regression baselines that get regenerated on purpose. Before the first divergent fix, the suite needs invariant checks on every fixture: watertight shell, no naked edges, volume and centre of mass within tolerance of the last C++-verified values, and face count. Without this, a kernel change has no judge.
- **T2. A save-format version marker, 2 days.** So a file with new records announces itself, and old readers fail clearly.

**What becomes possible**

Staying native with no parity constraint means the kernel's structural limits can be removed
instead of worked around. That reshapes tier 3.

- F3 was sized as a *derived* topology layer on top of the surface and curve lists, to leave C++-shaped data alone. It can now be the primary representation. That is more work at first, about 5 to 6 weeks instead of 3 to 4, and it makes every later feature cheaper.
- F6, a general surface type, moves from research to a planned investment. Replacing the fixed `[4][4]` net with a B-spline surface of arbitrary degree and knot vectors touches evaluation, subdivision, intersection, triangulation and STEP export. About 8 to 10 weeks. It is what makes general fillets, smooth lofts, offsets and Boolean-capable STEP import representable at all.
- Tolerant modelling becomes an option. Per-edge and per-vertex tolerances in place of global epsilons is how commercial kernels survive imported geometry. Open ended, but it can now be done incrementally.

Since OpenCASCADE is off the table, tier 4 is no longer "decide later". It is the long road
this project is choosing, so the foundations should come earlier than first recommended.

## Own file format `.spcs` (proposed by the user as `.sps`, 2026-10-01)

Proposal: spacesolve saves only its own format, `.spcs`. Upstream `.slvs` files are opened
through File, Import, never through Open, and never written. This replaces T2.

**Assessment: agreed, and it is the cleaner option.** A format that is "slvs plus records
upstream cannot read" would mislead users into thinking files round-trip. A separate extension
states the truth. Import-only also means the importer is a one-way converter, so it never has
to preserve upstream quirks on write.

**Current state**

- All persistence is `pkg/persist/slvs`, about 2,300 lines: `slvs.go` reads and writes with one hand-written function per record type, `linked.go` resolves link groups and images, `idf.go` and `stl.go` are link importers.
- `.slvs` is referenced in the file dialogs (`pkg/ui/menufile.go`, `pkg/platform/filters.go`), the backup extension `slvs~`, the command-line tool's default output pattern, and the script runner.
- 113 `.slvs` fixtures in `testdata`. The C++ comparison harness needs the `.slvs` writer to hand files to C++.

**Design points**

| Point | Recommendation |
|---|---|
| Text or binary | Text, line oriented, stable ordering. Diffable saves are a real advantage over every competitor and should be kept. |
| Header | Magic line with a format version and the writing application version. Reader refuses a newer major version with a clear message. |
| Grammar | A new one built for extension, with nested blocks and lists, in place of the flat `Group.field=value` records. Parameter tables, stored expressions, per-face appearance and later feature data all need nesting. |
| What is stored | Requests, constraints, groups, styles, parameters, plus what `.slvs` lacks: expression text, the named parameter table, units, document settings. Solved parameter values stay, since they are the solver's starting guess and pick the solution branch. |
| Generated entities | Keep them in version 1. Dropping them is tempting but the loader relies on them, and it can be done later behind the version number. |
| Embedded assets | Not in version 1. When STEP import lands, add a packed variant, a zip holding the text model plus imported geometry and linked parts. Deciding the name now avoids a second migration. |
| Import of `.slvs` | File, Import, SolveSpace model. A file with no links produces an untitled document. An assembly asks for a destination first. The `.slvs` file is never modified. |
| Link groups | Point at `.spcs` files only. Importing an assembly converts every linked `.slvs` recursively, see below. |
| `.slvs` writer | Leaves the product. It stays in the test tree until the C++ comparison tests are frozen under T1, then it is deleted. |
| Fixtures | The 113 `.slvs` files stay as the import test corpus. New goldens are `.spcs`. |

**Effort**

| Variant | Effort | Notes |
|---|---|---|
| S1. Same record grammar, new header and extension | about 1 wk | Fork of the existing reader and writer. Cheap, but inherits a flat grammar that every later feature will fight. |
| S2. New extensible text grammar | 2 to 2.5 wk | Reader, writer, version handling, import path, dialogs, backup and autosave names, command-line tool, script runner, fixture conversion, round-trip tests. Recommended. |
| S3. Packed container variant | 1 wk more | Can wait until D2 STEP import. |

**Format decisions taken by the user (2026-10-01)**

- **Extension is `.spcs`.** The first idea, `.sps`, clashes with SPSS syntax files.
- **Grammar is the new extensible text form (S2).** Version header, named blocks with braces, `name = expression` for values that keep their expression. Sketch of the shape agreed:

  ```
  spacesolve 1
  units mm

  param wall = 2.5
  param bore = wall*4

  group 3 extrude "body" {
    source 2
    depth = wall*10
    combine union
  }
  ```

- **Linked `.slvs` files are converted recursively on import.** Link groups never read `.slvs`. Consequences to design for:
  - Import walks the link tree first and shows the list of files it will write before writing any.
  - Each linked `a.slvs` becomes a sibling `a.spcs`. If one already exists, the user chooses reuse or overwrite, once for the whole import.
  - The top-level document is written too, since relative link paths need it to have a location. Import therefore asks for a destination up front instead of producing an untitled document.
  - A read-only directory or a missing linked file falls back to the existing locate-missing-file prompt.
  - Link cycles are detected and reported.
  - IDF and STL links are unaffected.
  - The command-line tool gets a batch `convert` command that does the same without prompts.
  - Adds about half a week to S2, so the format work is about 2.5 to 3 weeks.

## Recommended order (revised)

1. **T1 test oracle, then the `.spcs` format (S2) with `.slvs` import.** About 3.5 weeks. Everything after this may change kernel output and add records to the format.
2. **A1 named parameters, then B1, B2, I5, I4.** Three to four weeks. No kernel work, large change in feel.
3. **C1 to C3 history editing.** Three to four weeks.
4. **F1 raycast fix and F2 closed intersection loops, on by default.** About 2.5 weeks. Remove the rotate-and-retry workaround in the thread feature afterwards.
5. **I1 OpenGL backend, then I3 section view.** Three weeks.
6. **E1 mirror, E3 chamfer by Boolean, E9 sweep along lines and arcs.** About 5 weeks. First new solid features, all on existing machinery.
7. **D2 STEP import as reference mesh.** Three to four weeks.
8. **F3 topology as the primary representation, then F4 edge naming.** About 8 to 9 weeks. The first large kernel investment.
9. **E4 and E5 fillets on straight and circular edges, E11 restricted shell, E7 ruled loft.** About 14 weeks.
10. **F6 general surface type.** About 8 to 10 weeks. Then E6, E8, E10 and D3 become ordinary hard features instead of impossible ones.

Steps 1 to 7 are about 22 weeks at conventional pace and deliver most of the visible gap.
Steps 8 to 10 are about 8 months and are where the native kernel is actually built out.

## Decisions still open

- **Reading upstream files.** Settled in principle: `.slvs` is import only. The importer is kept indefinitely as the migration path.
- **Grammar, extension and linked-file handling.** Settled, see "Format decisions taken by the user".
- **What happens to the C++ comparison tests.** Recommended: keep them running until step 4 lands, then freeze their outputs as Go goldens under T1.
- **Topology first or surface type first** at step 8. Recommended: topology first, since fillets on planes and cylinders need it and do not need the new surface type.

## How to check an estimate before committing to it

For each kernel-dependent item, a two-day spike on the hardest fixture is cheaper than trusting
the table. Suggested spikes: a Boolean-route fillet on one edge of a box with a hole near the edge
for E4, a ruled loft from square to circle subtracted from a block for E7, and one real vendor STEP
file of a fastener and one of a connector for D2. `TestThreadSweep` is the model for a sweep-style
robustness test, and the naked-edge check in `pkg/ui/menuanalyze.go` is the pass criterion.
