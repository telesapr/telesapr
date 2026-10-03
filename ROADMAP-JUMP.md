# Roadmap: from a SolveSpace port to a modern CAD

Date: 2026-10-01, updated 2026-10-03. Status: steps 1 to 9 are done and merged; step 10 is next. See "Progress" below and the State columns of the implementation plan. Read "Definition of done for every new feature" first.

## Context

spacesolve began as a Go port of SolveSpace, bit-identical with the C++. As of 2026-10-01 it
is a separate project: parity and file compatibility with SolveSpace are no longer goals, and
the kernel stays Go native. The parity record that `PORTING.md` held is in the git history.

This document lists what the program lacks compared with current parametric CAD, sizes each
item against the code as it stood on that date, and gives the order to do them in. The
"Implementation plan" section breaks that order into work packages with the files they touch.
The kernel items of steps 9 to 11 still need a spike and a design pass before coding.

## Progress

| Step | State | Merged into `master` |
|---|---|---|
| 1. Test infrastructure and the thread feature | done 2026-10-02, but for 1.8: the CI workflow has not run on GitHub, since the repository has no remote yet | `c5caea3` |
| 2. The `.spcs` format and `.slvs` import | done 2026-10-02 | `d5b610c` |
| 3. Parameters and sketch feedback | done 2026-10-02, with follow-ups: a faster freedom analysis, a scenario for it, `make lint-new` on the whole branch, no `scale 0` in saves (`a33a1ae`), and the unit as the document's, format version 2 (3.9, `dba44b9`) | `22daacd` |
| 4. History editing | done 2026-10-02 | `fe7cb85` |
| 5. First kernel fixes | done 2026-10-02, with the cylinder-through-cylinder failures (19 of 54, not new) left as a follow-up | `f24fcd9` |
| 6. OpenGL and the section view | done 2026-10-02, with follow-ups listed under the step | `0a96960` |
| 7. First new solid features | done 2026-10-03, with follow-ups listed under the step | `a5829a6` |
| 8. STEP import as a reference mesh | done 2026-10-03, with follow-ups listed under the step | `e556586` |
| 9. Topology and edge naming | done 2026-10-03, on derived topology (9.2 not taken), with follow-ups listed under the step | `6f60438` |
| 10. Fillets, shell and loft | 10.1 (fillet of straight edges between planes), 10.4 (shell, flat faces only) and 10.5 (ruled loft) done 2026-10-03, with follow-ups listed under the step; 10.2 (corners) and 10.3 (circular edges) open | `e1ee516`, 10.4 `2fbc6d4` |

Decisions taken while doing them, beyond those below: the unit of length belongs to the
document and the format went to version 2 for it (2026-10-02, "Format decisions"); files broken
by hand get an error on load and no other safeguard (2026-10-02).

## How to read the estimates

- **Effort** is focused weeks for one developer who already knows this codebase,
  including basic tests and UI. The port itself has moved much faster than such numbers
  suggest, so read them as relative weights, not calendar promises.
- **The tables in "Estimates by area" predate the definition of done below.** They do not
  include scripting support or the full test bar. Add about 20% to every item with a user
  surface. "Totals by tier" and "Recommended order" already include it.
- **Size**: S under 1 week, M 1 to 3 weeks, L 3 to 8 weeks, XL more than 8 weeks or open ended.
- **Risk** is the chance that the estimate is wrong by more than a factor of two.
- **New code** is a rough count of non-test Go lines.

## Definition of done for every new feature

Requirement set by the user on 2026-10-01: a new feature is well tested, has a user interface,
and has scripting support. A feature that lacks one of the three is not done and does not merge.
The three paths, interface, script and file, must reach the same engine entry point in
`pkg/sketch`, so none of them can drift from the others.

### Tests

| Requirement | How it is met here |
|---|---|
| Unit tests for new engine and kernel code | Beside the code, in the package. New packages hold at least 85% statement coverage and no exported function at 0%. Measure with `-coverpkg=./pkg/...`, since `pkg/sketch` and `pkg/core` are mostly exercised from other packages. |
| Geometric invariants on every solid a test builds | The T1 oracle: watertight shell, no naked or self-intersecting edges, face count, and volume and centre of mass against a closed-form value where one exists, such as a filleted box. |
| Golden files | A `.spcs` fixture per feature variant under `testdata`, with a regeneration script. A golden only changes in a commit that says why. |
| Save and load round trip | Load, regenerate, save gives the same bytes. A file from the previous format version still loads. |
| Undo and redo | Every creating and editing action is one undo step and comes back identical on redo. |
| Failure behaviour | Bad input gives a diagnostic on the group, never a panic and never silently wrong geometry. Each documented failure has a test. |
| Robustness sweep, for solid features | A parameter sweep over sizes, angles and placements in the style of the thread sweep, reporting the failure rate. The acceptable rate is fixed before the feature starts. Fuzz targets for parsers, with `.spcs` and STEP first. |
| Concurrency | `go test -race` over the touched packages, and the same result on one goroutine and on eight. |
| Performance | A benchmark for anything on the regeneration path, compared with the previous commit. |
| Interface scenario | At least one script under `testdata/ui/scripts` that drives the feature through menus, mouse and text window on the headless platform. |
| Path equivalence | One test builds the same model through the interface script and through a Go script and requires identical saved files. |

### User interface

- A menu entry in `pkg/ui/menu_table.go`, with an accelerator where one is free, and a toolbar icon for features used while drawing.
- A text-window screen showing every parameter of the feature, editable in place, with expressions accepted wherever a number is.
- Selection-driven creation as the existing groups do it: select, then invoke. A clear message when the selection does not fit, saying what to select.
- Feedback on the canvas: hover and selection highlighting for whatever the feature lets the user pick, and failure marked on the model where it happened.
- An entry in the command search once I5 exists.
- No feature reachable only from the console or the command line.

### Scripting

- A method on `pkg/script/api` to create the feature, to read and change every parameter the text window shows, and to query the result.
- Doc comments on every method, since `help()` and `docs.json` are generated from them. `make generate` is run and `make generate-check` passes.
- A runnable example under `examples/scripts`, executed by a test.
- A test in `pkg/script/api` that covers misuse as well as use, as `TestMisuse` does today.
- Reachable from `spacesolve-cli run`, so the feature works without a window.
- Kernel foundations have no interface of their own, but expose what scripts can use. Topology gives scripts faces, edges and vertices to enumerate and select by. The test oracle gives scripts a `Check()` on a group.

### What existed, and what exists now

| Piece | State on 2026-10-01 | State on 2026-10-02, after step 3 |
|---|---|---|
| Headless interface scripts | 53 scenarios in `testdata/ui/scripts`. Every golden is a `.cppui` file recorded from C++, so a Go-only feature has nothing to compare against on `master`. | 67 scenarios: the 53 compared with C++, 11 Go-only (`ext_*`) and 3 path-equivalence pairs (`path_*`). |
| Go-recorded interface goldens | Exist only on the `threads` branch: one file, `testdata/ui/golden/ext_thread.goui`, with the regeneration switch `SPACESOLVE_REGEN_GOUI`. | 11 `.goui` goldens, `scripts/regen-goui.sh`. The C++ comparisons hide what C++ lacks through test-only switches (`cppComparison`, `goOnlyMenuItems`, `legacySlvsSave`) until package 5.1. |
| Script API | Covers drawing, constraints, export, and the stock groups except helix. TTF text requests are also missing. Its own package tests reach 31% of its statements. | Every group type and creating command has a method (`completeness_test.go`); parameters, freedom, conflicts, commands, measure and units added. Its own package tests reach 55%; more is covered from other packages. |
| Example scripts | One, `examples/scripts/bracket`. | Three, `bracket`, `bolt` and `plate`, each run by a test. |
| Generated symbol check | `make generate-check` exists. | Unchanged. |
| Continuous integration | `make ci` and `.github/workflows/ci.yml` were prepared on 2026-10-01 and pass locally. The workflow has not run on GitHub, since the repository has no remote yet. | Unchanged: passes locally, never run on GitHub. `make lint-new` now lints from the branch's merge base with `master`. |
| Fuzzing | None. | `FuzzExpr`, `FuzzLoad` (`.slvs`) and `FuzzRead` (`.spcs`), `make fuzz`. |
| Geometric invariant checks | Only inside C++ parity tests, as watertight and volume comparisons for Boolean fixtures. | `pkg/check`, Analyze > Check Solid, `Group.Check()`, and a baseline of 352 lines in `testdata/invariants.txt`. |

### Test infrastructure to build first

| Item | Effort | Content |
|---|---|---|
| T1. Invariant oracle | 1 to 1.5 wk | Described under "Decisions taken". One call that checks a group's solid, usable from Go tests, from scripts, and from the Analyze menu. |
| T3. Go-recorded interface goldens | 0.5 wk | Bring the `.goui` mechanism from the `threads` branch to `master`, with a regeneration switch and a rule that regenerated goldens are reviewed as diffs. |
| T4. Script API completeness test | 0.5 wk | Fails when a group type or a creating menu command has no API method, when an exported API method has no doc comment, or when an example does not run. |
| T5. Sweep and fuzz harness | 1 wk | A shared helper for parameter sweeps with a failure-rate report, and fuzz targets wired into `make`. |
| T6. Continuous integration | 0.5 wk | Tests, race run on the engine packages, GUI tests under Xvfb, generated-symbol check, lint, and a coverage report with the 85% rule for new packages. |
| T7. Path equivalence helper | 0.5 wk | Runs an interface script and a Go script and compares the saved files. |

About 4 to 4.5 weeks in total, and it is the first step of the order below.

### The screw thread feature

Done in step 1 (package 1.9): merged with `Doc.Thread`, `examples/scripts/bolt` and a path
equivalence test. What follows is the state before it.

It is the only Go-only feature so far and it is not on `master`. It sits on the `threads` branch,
two commits ahead and six behind. It has an interface screen and goldens but no script API.
Under this definition it merges after it gains `doc.Thread(...)`, an example, an API test and a
path equivalence test. About half a week, and a good first trial of the checklist.

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
| Free-parameter marking per point and normal | `System.MarkParamsFree` in `pkg/solver/system.go`, drawn in `pkg/ui/drawentity.go`; since step 3 also `System.FreeParams`, one pass for every solve |
| List of constraints to remove when a sketch is overconstrained | `System.FindWhichToRemoveToFixJacobian` |
| Expression parser with an identifier node | `OpVariable`, `NewVariable`, `Eval(ctx, vars)` in `pkg/core/expr.go` |
| Group suppress flag, mirrored mesh transform | `Group.Suppress`, `pkg/srf/mesh.go:344` |
| Threaded Booleans | `pkg/srf/parallel.go` |
| Go-only feature precedent with goldens and an extension gate | Screw threads, merged in step 1 without the gate |

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
  `ValA float64` and no expression text. (Lifted in step 3: constraints and groups keep their
  expression, and the sketch has a parameter table.)
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

A1 was done in step 3 (packages 3.1 to 3.4, and 3.9 for units). The parser did need two fixes:
trigonometry in degrees, and one operator character per token.

### B. Sketch feedback

| Item | Size | Effort | New code | Risk |
|---|---|---|---|---|
| B1. Colour whole entities by constraint state, not only free points | S | 0.5 wk | 300 | Low |
| B2. Highlight conflicting constraints in the canvas with a one-click jump | S | 0.5 wk | 250 | Low |
| B3. Auto-constrain on draw, with more inference than horizontal and vertical | M | 1.5 wk | 700 | Medium |

The solver already produces the data. B1 has to run `MarkParamsFree` after every solve instead
of on demand, which costs a rank computation per group. That needs measuring on large sketches.

B1 and B2 were done in step 3 (3.5, 3.6). B1 does not run `MarkParamsFree`: a one-pass analysis
of the Jacobian at the solved values costs 1.5 ms at about 600 unknowns, below the solve.

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

Either route needs a way to pick an edge. Edges that a group generated already exist as
entities: the copied sketch segments at both ends of an extrusion and its swept lines. E3 in
step 7 is limited to those, so it does not wait for F4. Edges born from a Boolean need F4, and
the chamfer gains them in step 9. Recommendation: chamfer by Boolean, fillet by local edit, and
count F3 and F4 as part of the fillet cost. With those included the first usable fillet is
about 7 to 9 weeks, not 3 to 4.

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
I4 and I5 were done in step 3 (3.8, 3.7).
I3 in the software renderer is possible but slow. It belongs after I1.

### J. Out of scope

Sheet metal, direct push-pull modelling, tolerancing annotations, CAM and simulation.
Each is an XL product in its own right and none is needed to count as a modern part modeller.

## Totals by tier

| Tier | Items | Effort |
|---|---|---|
| 0. Test infrastructure | T1, T3 to T7, thread feature brought to the bar | about 5 wk |
| 1. Cheap, high value, no kernel work | A1 to A3, B1, B2, C1 to C4, D1, D4, G2, I4, I5 | about 14 wk |
| 2. Moderate, bounded | E1, E2, E3, E9, D2, D5, G1, G3, G5, H1, H4, I1, I3, F1, F8 | about 30 wk |
| 3. Kernel-dependent features | F2, F3, F4, F6, E4, E5, E7, E11, E13, F7, H2, H3, I2, I6 | about 63 wk |
| 4. Research grade | E6, E8, E10, E12, D3, F5, G4 | more than a year, no firm bound |

These totals include the definition of done: the full test bar, interface and scripting for
every feature. Each is the sum of the table efforts, taken at the middle of a range, plus 20%
on every item except F1, F2 and F8. Tier 3 counts F3 at its revised 5 to 6 weeks and F6 at
8 to 10, both from "Decisions taken". F6 sat in tier 4 before those decisions.
Tiers 0 to 3 together are about 112 weeks, roughly 26 months of one developer at conventional
pace. Tier 4 is where the estimate stops being meaningful.

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

**State before step 2.** The format as built is described in `docs/spcs-format.md`; it is at
version 2 since package 3.9.

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

- ~~**Stored expressions are in millimetres and degrees** (decided 2026-10-01). The file does not depend on the view unit, which is an interface preference. The interface converts when it shows and when it reads an expression typed in view units. The `units` line records the unit the document was authored in, for display only.~~
- **Revised 2026-10-02: the unit belongs to the document** (format version 2, package 3.9). The `units` line is the document's unit; lengths written with `=` are in it, angles in degrees, and parameters are plain numbers that take the unit of what they drive. Typed text other than a plain number is always kept. Everything else in the file stays in millimetres. Unit suffixes in expressions (`1/4 in`) were considered and not taken.
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
- **The `.slvs` importer is kept indefinitely** as the migration path for upstream files.

## Recommended order (revised)

Efforts here include the definition of done. Every step ends with its features reachable from
the interface and from scripts, and with the tests listed in that section. Each step is
developed in its own branch and merged into `master` when it is done, see "Rules for every package".

1. **Done 2026-10-02.** **Test infrastructure T1 and T3 to T7, then the thread feature brought to the bar and merged.** About 5 weeks. Nothing else can be called done without it.
2. **Done 2026-10-02.** **The `.spcs` format (S2) with `.slvs` import.** About 3 weeks, with a fuzz target for the reader and `Save`, `Open` and `Import` in the script API. Everything after this may add records to the format.
3. **Done 2026-10-02, with document units as a follow-up.** **A1 named parameters, then B1, B2, I5, I4.** About 4.5 weeks. No kernel work, large change in feel. Scripts get parameters by name, constraint state per entity, and measurements.
4. **C1 to C3 history editing.** About 4 weeks. Scripts get reorder, suppress and the list of broken references.
5. **F1 raycast fix and F2 closed intersection loops, on by default.** About 3 weeks, most of the addition being sweeps. Remove the rotate-and-retry workaround in the thread feature afterwards.
6. **Done 2026-10-02.** **I1 OpenGL backend, then I3 section view.** About 3.5 weeks. Goldens stay on the software renderer. Scripts get the section plane and a rendered image.
7. **E1 mirror, E3 chamfer by Boolean, E9 sweep along lines and arcs.** About 6 weeks. First new solid features, all on existing machinery, each with a robustness sweep. The chamfer takes only edges that exist as entities until step 9.
8. **Done 2026-10-03.** **D2 STEP import as reference mesh.** About 4.5 weeks, with a corpus of real files and a fuzz target for the reader.
9. **Done 2026-10-03, on derived topology.** **F3 topology as the primary representation, then F4 edge naming.** About 10 weeks. The first large kernel investment. Scripts get faces, edges and vertices to enumerate and select by, which later features need for their own API. The chamfer is extended to any edge of the solid.
10. **E4 and E5 fillets on straight and circular edges, E11 restricted shell, E7 ruled loft.** About 20 weeks.
11. **F6 general surface type.** About 10 to 12 weeks. Then E6, E8, E10 and D3 become ordinary hard features instead of impossible ones.

Steps 1 to 8 are about 34 weeks at conventional pace and deliver most of the visible gap.
Steps 9 to 11 are about 41 weeks, between 9 and 10 months, and are where the native kernel is
actually built out.

### Items outside the numbered steps

The steps above cover about 75 of the 112 weeks in tiers 0 to 3. The remaining items are below.
None blocks a step. Each can be taken between steps once its prerequisite has landed.
Efforts include the 20%.

| Item | Earliest slot | Prerequisite | Effort |
|---|---|---|---|
| A2 configurations | after step 3 | A1 | 1.2 wk |
| A3 dimensions referenced by name | after step 3 | A1 | 0.6 wk |
| G5 motion study | after step 3 | A1 | 1.8 wk |
| C4 insert a group mid-history | after step 4 | C1 | 0.6 wk |
| D1 3MF export | after step 2 | none | 0.6 wk |
| D4 STL and OBJ reference import | after step 2 | none | 1.2 wk |
| D5 STEP export with assembly structure | after step 2 | none | 1.2 wk |
| G2 bill of materials | after step 2 | none | 0.6 wk |
| G1 mate shortcuts | after step 2 | none | 2.4 wk |
| G3 shell-based interference check | after step 5 | F1 | 1.2 wk |
| F8 parallel `MergeCoincidentSurfaces` | after step 5 | goldens frozen | 1 wk |
| F7 regeneration off the UI thread | after step 5 | goldens frozen | 3.6 wk |
| E2 pattern along a curve | after step 7 | E9 path handling | 1.8 wk |
| H1 drawing sheet | after step 6 | none | 3.6 wk |
| H4 title block and templates | after H1 | H1 | 1.2 wk |
| H3 section and detail views | after H1 | H1 | 2.4 wk |
| I2 shader backend | after step 6 | I1 | 3.6 wk |
| I6 appearance beyond colour | after I2 | I2 | 1.2 wk |
| E13 draft angle | after step 9 | F3 | 4.2 wk |
| H2 associative drawing dimensions | after step 9 | F4, H1 | 5.4 wk |

About 39 weeks in total. A2, A3, C4, D1, D4 and G2 are each under a week and a half and are the
natural fillers between the long steps.

## Implementation plan

This section turns the recommended order into work packages. A package is numbered by step,
so 3.2 is the second package of step 3. Paths and symbols were checked against `master` at
commit `ce5bfe4` on 2026-10-01. Steps 1 to 5 are planned to the file level. Steps 6 to 8 are
planned to the package level. Steps 9 to 11 are planned as phases, each opened by a spike.

### Rules for every package

- **One branch per step**, cut from `master` and named after the step, such as `step-3-parameters`.
  The whole step is developed on that branch and nothing from it lands on `master` earlier.
  A package is one commit or a short series, and `make test` passes after each. When the step is
  finished, `make ci` passes on the branch and it is merged into `master`. The next step is cut
  from the updated `master`, so steps never stack on each other's unmerged branches. The
  `threads` branch is step 1's package 1.9 and follows the same rule.
- **Engine first.** The order inside a feature is: entry point in `pkg/sketch`, save and load,
  interface, script API, path equivalence test. The interface and the script API call the same
  entry point, as `ss.addNewGroup` and `SketchHost.AddGroup` both reach
  `Sketch.AddGroupAfterActive` today.
- **Undo is a field, not code.** Document state added by a feature goes into `UndoState` in
  `pkg/sketch/undo.go`, or it is lost on undo.
- **A golden changes only on purpose.** A commit that regenerates goldens runs the
  regeneration script, contains nothing else, and says why in its message.
- **A step is finished** when every feature in it passes the definition of done, the
  "State" column below is updated, and its branch is merged into `master`.

### Step 1. Test infrastructure and the thread feature

| Package | Content | State |
|---|---|---|
| 1.1 Go-recorded interface goldens (T3) | Bring the `.goui` comparison from `threads` commit `614b2d4` into `pkg/ui/uiscript_test.go`: a test beside `compareUIScripts` that reads `testdata/ui/golden/<name>.goui` and rewrites it when `SPACESOLVE_REGEN_GOUI=1`. Scripts with the prefix `ext_` are Go-only. Add `scripts/regen-goui.sh`. | done 2026-10-02: `TestGoScripts`, `scripts/regen-goui.sh` |
| 1.2 Invariant oracle (T1) | New package `pkg/check`. `check.Solid(shell, mesh)` returns a report: naked edge count, self-intersecting edge count, face count, volume, centre of mass, bounding box, and the group's `BooleanFailed` flag. It reuses `SKdNode.MakeCertainEdgesInto` and `SMesh.CalculateVolume`. The centre of mass code moves out of `pkg/ui/menuanalyze.go` into the package. | done 2026-10-02. Centre of mass was already `SMesh.GetCenterOfMass`; `check.Group(sk, g)` takes the shell of the last group that made solid |
| 1.3 Invariant baseline | `TestInvariants` in `pkg/persist/slvs` loads each of the 113 fixtures, regenerates, and compares the report with `testdata/invariants.txt`, one line per fixture and group. Counts must match exactly. Volume and centre of mass must match to a relative 1e-9 while parity holds. The tolerance is loosened to 1e-6 in package 5.1. `SPACESOLVE_REGEN_INVARIANTS=1` rewrites the file. The baseline is recorded now, while the output is still verified against C++. | done 2026-10-02: 287 lines from 113 fixtures, then 65 more for the thread goldens. The one invalid fixture is recorded with its load error |
| 1.4 Oracle in the interface and scripts | Analyze menu entry "Check Solid" in `pkg/ui/menu_table.go`, reporting through the existing message path and filling `ss.NakedEdges` for display. `Group.Check()` in `pkg/script/api/group.go` returns the same report. | done 2026-10-02: Analyze > Check Solid (Ctrl+Shift+O), `Group.Check()`, first scenario `ext_check_solid` |
| 1.5 Path equivalence helper (T7) | `requireSamePaths(t, uiScript, goScript)` in a new `pkg/ui/pathequiv_test.go`. It runs the interface script on `uiScriptRunner`, runs the Go script through `script.Engine` on an `api.SketchHost`, and compares `Document.SaveRegenerated()` bytes. First user: one extrusion built both ways. | done 2026-10-02: pairs `testdata/ui/scripts/path_*.script` and `.go`, compared byte for byte. It found that `Doc.Rect` built another rectangle than the tool; fixed. An interface script must not save, since a save regenerates and the solver is not a fixed point |
| 1.6 Script API completeness (T4) | `pkg/script/api/completeness_test.go`, as an external test package so it can import `pkg/ui`. It fails for a `sketch.GroupType` with no creating method, for a creating menu command with no method, for an exported method with no doc comment (parsed with `go/doc`), and for an example under `examples/scripts` that does not run. It will fail at once on helix and TTF text, so the package adds `Doc.Helix` and `Doc.Text`. | done 2026-10-02. Added `Doc.Helix`, `Text`, `Image`, `TangentArc`, `Split`, `Constraint.SetOther`; fixed `Doc.Revolve`, which turned four times too far, and the builtin font name, which matched no font. The example check lives in `cmd/spacesolve-cli`, where `run` is |
| 1.7 Sweep and fuzz harness (T5) | `pkg/internal/sweep`: runs a function over a parameter grid, collects failures, and fails the test above a stated rate. A small sample runs by default and the full grid under `SPACESOLVE_SWEEP=1`. Fuzz targets `FuzzLoad` over `Document.parse` in `pkg/persist/slvs` and `FuzzExpr` over `core.NewFromString`. `make fuzz` runs each for 30 seconds. | done 2026-10-02: `pkg/internal/sweep` (with a per-sample allowance), `FuzzExpr`, `FuzzLoad`, `make fuzz`. No findings in the first 30 s of each |
| 1.8 Continuous integration (T6) | `make ci` runs `fmt-check`, `vet`, `build`, `generate-check`, `coverage`, `test-race`, `test-gui` and `lint-new`. `scripts/coverage-check.sh` applies the 85% rule to the packages listed in `scripts/coverage-packages.txt`, which is empty until the first new package. `.github/workflows/ci.yml` runs the same targets and checks out takigo beside the repository at a pinned commit, because `go.mod` replaces it with `../takigo`. The existing code has about 100 lint findings, so only changed lines are linted. Left to do: push, add the `TAKIGO_TOKEN` secret, and see the first run pass. | prepared 2026-10-01, not yet run on GitHub |
| 1.9 Thread feature to the bar | Rebase `threads` onto `master`. Add `Doc.Thread` over `Sketch.AddThread`, with setters for what the thread screen edits. Add `examples/scripts/bolt`, a misuse test, a path equivalence test against `ext_thread.script`, and `Check()` on every thread golden. Move `TestThreadSweep` onto `pkg/internal/sweep`. Remove the `Extensions` gate, since there is no C++ comparison to protect. Merge. | done 2026-10-02 on the step branch: gate removed, `Doc.Thread`, `examples/scripts/bolt`, `path_thread`, golden checks, sweep at 24 of 400 unsound (none at the default tolerance). `thread/bolt_m8x30.slvs` is not a C++ save and is out of the round trip |

Order: 1.1, 1.2, 1.3 first, since 1.3 must be recorded before anything changes output. Then 1.4
to 1.8 in any order. 1.9 last, since it uses all of them.

Done when: `make ci` passes on `master` with the thread feature merged, and T4 passes with no
exclusion list.

### Step 2. The `.spcs` format

| Package | Content | State |
|---|---|---|
| 2.1 Grammar | `docs/spcs-format.md` with the grammar in EBNF and one example per block kind. Stored expressions are in millimetres and degrees. Decide here: string escaping, comment syntax, and how handles are written. | done 2026-10-02: Go string literals, `#` comments, handles as `0x` hex without leading zeros; group values that will take expressions are written `copies = 3` already |
| 2.2 Syntax tree | New package `pkg/persist/spcs`. `lex.go` and `parse.go` produce a generic tree of blocks, fields and lists with line and column on every node. `print.go` writes a tree back. No knowledge of the sketch at this layer, so an unknown block can be reported by position and skipped. | done 2026-10-02 |
| 2.3 Model mapping | `encode.go` and `decode.go` map the tree to `sketch.Sketch`: groups, requests, entities, constraints, styles, solved parameter values, units and document settings. Stable order: groups by `Order`, everything else by handle. Floats are written in the shortest form that reads back to the same bits. | done 2026-10-02. `Document.Encode` re-emits a decoded file unchanged; an entity's workplane is a plain handle, since copied entities have 0, not free-in-3D |
| 2.4 Version handling | Header `spacesolve <major>`. A newer major is refused with a message naming both versions. `testdata/spcs/v1` holds files that every later reader must load. | done 2026-10-02: `VersionError`; nine files in `testdata/spcs/v1`, one of them (`fields.spcs`) setting every field |
| 2.5 Link loading | Move `ReloadAllLinked`, `RebaseLinks` and `LoadLinkedImage` from `pkg/persist/slvs/linked.go` into a format-neutral `pkg/persist/link`, which picks a loader by extension: `.spcs`, `.emn`, `.stl`. `.slvs` is not among them. | done 2026-10-02: formats register a loader by extension (`link.Register`), so `spcs` registers itself; `slvs.LoadFile` passes its own `.slvs` loader as a fallback for the C++ tests |
| 2.6 Import | `pkg/persist/convert`. `Plan(root)` walks the link tree, detects cycles, and returns the files it would write. `Run(plan, policy)` writes them, with reuse or overwrite chosen once. The `.slvs` reader stays where it is and gains nothing. | done 2026-10-02: `MakePlan`, `Run`, and `File` for batch use. A linked copy that exists is reused unless overwritten; the root is always written |
| 2.7 Product switch | `pkg/ui/menufile.go`: `backupExt` becomes `spcs~`, Open and Save use `.spcs`, and File, Import gains "SolveSpace model" with the plan shown before writing. `pkg/platform/filters.go`: new filter lists. `cmd/spacesolve-cli/main.go`: default output pattern `%.spcs` and a `convert` command. `cmd/spacesolve-cli/script.go`: the sketch-argument test accepts `.spcs`. `pkg/script/engine.go`: help text. | done 2026-10-02. Opening a `.slvs` file points at File > Import; a test-only hook, `legacySlvsSave`, keeps the C++ interface scenarios on `.slvs` until 5.1 |
| 2.8 Script API | `Doc.Save` and `api.Open` work on `.spcs`. New `api.Import(src, dst)` over `convert`. Saving to a `.slvs` path is an error that names the reason. | done 2026-10-02. `api.Import` reuses linked copies and refuses a plan with missing files; `Open` refuses a file with parts it does not know, which a save would lose. Scripts import `persist/spcs` and `persist/convert` in place of `persist/slvs` |
| 2.9 Retire the `.slvs` writer | Move `Document.Save`, `SaveRegenerated` and the `write*` functions to `pkg/internal/slvswrite`. A test fails if a non-test file imports it. It is deleted in package 5.1. | done 2026-10-02: `slvswrite.Save` and `SaveRegenerated`, guarded by `TestOnlyTestsImport` |
| 2.10 Tests | For each of the 113 fixtures: import, save, load, save again, and require the same bytes and the same invariant report as 1.3. `FuzzRead` over the parser and the decoder. A golden `.spcs` for each fixture directory. An interface scenario for Import and one for Save and Open. A `convert` test on an assembly with a cycle, a missing link and a read-only directory. | done 2026-10-02: `TestFixturesRoundTrip` (decode and encode is the identity on every fixture; load, regenerate and save is, except 15 fixtures the solver never settles, with or without the format), 61 directory goldens, `ext_import_slvs` and `ext_save_open`, `FuzzRead` 10 minutes with no finding. It found an order dependence in the 1.3 baseline, fixed in its own commit |

Order: 2.1 to 2.4, then 2.5 and 2.6, then 2.7 to 2.9, with 2.10 growing alongside.

Done when: the product writes no `.slvs`, all fixtures round trip, and the fuzz target has run
ten minutes without a finding.

### Step 3. Parameters and sketch feedback

| Package | Content | State |
|---|---|---|
| 3.1 A1 engine | `sketch.Parameter{Name, Expr, Value}` and `Sketch.Parameters`, added to `UndoState`. Expressions and values are in millimetres, as in the script facade. An expression string beside `ValA` in `core.ConstraintBase` and beside `ValA`, `ValB`, `ValC` in `sketch.Group`. `Sketch.EvaluateParameters` runs at the top of `generateAll`: it parses with `core.NewFromString`, orders parameters by dependency, evaluates with `Expr.Eval(ctx, vars)`, and writes the numbers into the `Val` fields. A cycle or an unknown name leaves the last good value and sets a diagnostic that names the chain. | done 2026-10-02: `sketch.Parameter`, `EvaluateParameters` at the top of `generateAll`, `SetConstraintText` and `SetGroupText` as the one entry point for typed values, `UndoRememberFor`. A plain expression is evaluated once in view units as before; only one that names a parameter is kept. Found and fixed two parser bugs on the way: trigonometry took radians, and `2*(3)` did not parse |
| 3.2 A1 persistence | `param name = expr` lines, and `field = expr` wherever a stored expression exists, as in the agreed sketch of the format. A file without expressions is byte-identical to before. | done 2026-10-02: also `scale = expr`. What cannot be evaluated is kept and reported apart from skipped parts (`Document.ExprErrors`); `testdata/spcs/v1/parameters.spcs` added |
| 3.3 A1 interface | A "Parameters" screen in `pkg/ui/textscreens.go`, linked from the group list: add, rename, edit, delete, with the evaluated value beside each. Renaming rewrites the expressions that use the name. The dimension editor in `pkg/ui/mouse.go` and the group value editors keep the typed text when it is not a plain number. A dimension that has an expression shows it in the editor and the value on the canvas. | done 2026-10-02: Edit > Parameters (Ctrl+Shift+M) and a link under the group list; the group screen shows `(= expr)`; scale and pitch edits became undo steps. The C++ comparisons hide the Go-only links (`cppComparison`). Scenarios `ext_parameters`, `ext_parameter_copies` |
| 3.4 A1 scripting | `Doc.Parameters`, `Doc.Parameter(name)`, `Doc.SetParameter(name, expr)`, `Doc.DeleteParameter(name)`. The existing `api.Param` type is the solver's parameter, so the new type is named `Parameter` and the doc comments say which is which. `Constraint.SetExpr` now stores the text. `Constraint.Expr` and the group equivalents read it. | done 2026-10-02: `Doc.SetParameter` adds or changes; also `RenameParameter`, `ParameterUsers`, `ExprErrors`, `Group.SetExpr/Expr/Value`. `examples/scripts/plate`; `path_parameters`, for which a new distance's label offset became the engine's (`Sketch.DistanceLabelOffset`) |
| 3.5 B1 entity state colouring | `SolveGroupAndReport` already takes `andFindFree`. Turn it on for the active group after every solve, behind a setting, after a benchmark on the largest sketch in `testdata` shows the cost. `pkg/ui/drawentity.go` colours an entity by whether all, some or none of its parameters are free. `Entity.Free()` in the API. | done 2026-10-02 without a second solve: `Solver.FreeParams` finds all free unknowns in one reduction to row echelon form at the solved values, so it cannot move a save; checked against the rank test on random systems and against Show DOF on every fixture. Configuration setting, on by default. Equalities are substituted and one-unknown rows dropped before the dense step: 1.5 ms at about 600 unknowns (regeneration 2.0 ms) and 7.2 ms at about 1,600 (regeneration 9.8 ms). Scenario `ext_freedom`, with a `freedom` dump command |
| 3.6 B2 conflict highlighting | `Group.Solved.Remove` holds the list and `ShowGroupSolveInfo` prints it. Add canvas highlighting in `pkg/ui/drawconstraint.go` and a jump that zooms to the constraint. `Group.Conflicts()` in the API. | done 2026-10-02: red, `[show]` on the failure screen, scenario `ext_conflicts` |
| 3.7 I5 command search | A palette over the `menuEntry` table: type to filter, Enter to run the entry's handler. One accelerator. `api.Commands()` lists entries and `Doc.RunCommand(label)` runs one. On a host with no window it returns an error. | done 2026-10-02: Edit > Find Command (Ctrl+K). The editor reports only Enter, so Enter lists and a second Enter runs the first match. Scenario `ext_find_command` |
| 3.8 I4 measure | A mode that shows distance, angle or radius for the selection on the canvas without creating a constraint, reusing the computation in `pkg/ui/describescreen.go`. `Doc.Measure(a, b)` returns the same numbers. | done 2026-10-02: `Sketch.Measure`; Analyze > Measure Selection (Ctrl+Shift+E) labels the selection on the canvas. Scenario `ext_measure` |
| 3.9 Document units (follow-up) | The `units` line becomes the document's unit: typed lengths and `=` lengths in the file are in it, so typed text can always be kept. Format version 2; version 1 files read as before and are upgraded on save. View > Dimension Units sets the document's unit, keeping sizes, as an undo step. | done 2026-10-02: `Sketch.Units`, `Sketch.SetUnits` (wraps length expressions in the factor and lists the parameters to check), `Doc.Units`/`SetUnits`, `testdata/spcs/v2`, scenario `ext_units`. In the C++ comparisons the unit stays the view's only. A plain length in a non-mm document moves by an ulp on its first reload, then is stable |

Order: 3.1 to 3.4 in sequence, then the rest in any order.

Done when: a model driven by three parameters can be built, saved, reopened and changed
through the screen and through a script, with identical files.

### Step 4. History editing

| Package | Content | State |
|---|---|---|
| 4.1 Dependency query | `Sketch.GroupDependencies(h)`: the groups a group refers to through `OpA`, `OpB`, `ActiveWorkplane`, `Predef`, the entities its requests and constraints name, and the sources in `Remap`. Tested on every fixture by checking that each group depends only on earlier ones. | done 2026-10-02: each dependency carries the first reason found ("operand", "constraint c012"); worked out from the stored model, so it holds before a regeneration; a linked group's remap is left out, since it names entities of its file. `GroupDependents` for the other direction |
| 4.2 C1 reorder | `Sketch.MoveGroup(h, before)` renumbers `Order` and refuses a move that puts a group ahead of a dependency, naming it. Up and down links in `ShowListOfGroups`. `Group.MoveBefore(other)` and `Group.DependsOn()` in the API. Sweep: every legal move on every fixture, then move back, and require the original bytes. | done 2026-10-02: the groups keep their set of order numbers, so a move back restores the file; arrows in the group list and Edit > Move Group Earlier/Later; `MoveToEnd`, `UsedBy`. `TestMoveGroupSweep`: 192 moves allowed, 908 refused, every allowed one regenerates without pruning. Scenario `ext_move_group`, pair `path_move_group` |
| 4.3 C2 full suppress | A second flag beside `Group.Suppress` that also skips the group's solve and entities. Groups that depend on it get a diagnostic instead of a pruned reference. Checkbox in `ShowGroupInfo`. `Group.SetSuppress` gains a mode argument. | done 2026-10-02: `Group.SuppressAll`, `.spcs` field `suppress-all` (added within version 2); groups that use it are left out with it (`SuppressedBy`), keeping their parameter values; the active group moves to the nearest one still in. `SetSuppress(Unsuppressed/SuppressSolid/SuppressWhole)`, `Suppression`, `LeftOut`. `TestSuppressAllSweep`: nothing pruned, and the 176 fixed-point fixtures save the same once brought back. Scenario `ext_suppress_all`, pair `path_suppress_all` |
| 4.4 C3 broken references | `PruneOrphans` and the pruning in `generateAll` record what they remove: the entity's description and the group that owned it. The record goes into `SketchDiagnostic` and into a screen that lists each break with a jump to the affected group. `Doc.BrokenReferences()` in the API. Since step 3 a deletion can also break stored expressions; the screen lists `Sketch.ExprErrors` beside the pruned references. | done 2026-10-02, in `Sketch.BrokenReferences` rather than `SketchDiagnostic`, which each solve clears: the list stays until the next deletion, an undo or `ClearBrokenReferences`. Names come from the last regeneration, so a deleted request or group is still named; what belonged to a deleted group is not listed. Edit > Broken References, a home-screen link while the list isn't empty. Scenario `ext_broken_references`, pair `path_delete_group`, `examples/scripts/history` |

Done when: reordering, suppressing and deleting an upstream group each produce a named
diagnostic, never a silent deletion.

### Step 5. First kernel fixes

| Package | Content | State |
|---|---|---|
| 5.1 Freeze the C++ comparison | Every test that compares against a C++ recording keeps its recorded file as a Go golden, and its regeneration script calls Go. The `scripts/cpp-*-harness.cpp` files, `scripts/regen-cpp-*.sh` and `pkg/internal/slvswrite` are deleted. So are the test-only switches that keep the C++ recordings valid: `legacySlvsSave`, `cppComparison` (Go-only links and colours, the unit as the view's only) and `goOnlyMenuItems`; the scenarios they affect are regenerated in their own commit. The invariant tolerance in 1.3 is loosened to 1e-6. | done 2026-10-02: `golden.Check`, `CheckDump` and `CheckDir` rewrite a golden under `SPACESOLVE_REGEN_GOLDEN=1`, which `scripts/regen-golden.sh [areas]` and `scripts/regen-goui.sh [names]` set; the hex-float goldens were rewritten once in Go's spelling, values unchanged. The C++ scenarios became `.goui` goldens, regenerated without the switches in their own commit. With no `.slvs` writer left, what was compared with a C++ save is now a `.spcs` golden Go wrote from the same state: an import golden per fixture under `testdata/spcs/golden`, the new file, the modify tools, the thread goldens (`testdata/thread/*.spcs`) and the DXF imports. Tests open `.slvs` fixtures through a test-only `loadSlvs`. `check.BaselineRelTol` is 1e-6. The glibc and Eigen references stay, since they pin libraries |
| 5.2 F1 raycast | The fallback at the end of `SShell.ClassifyEdge` in `pkg/srf/raycast.go` retries with another direction only when the nearest hit has `OnEdge` set, and `OnEdge` means within `lengthEps` of a trim. Widen it: `SSurface.AllPointsIntersecting` in `pkg/srf/raycast_surface.go` also marks a hit that lies within the chord tolerance of a piecewise-linear edge of a curved trim. Raise the retry limit from 5 and pick the ray with the largest clearance if all are marked. | done 2026-10-02, in `ClassifyEdge`'s raycast rather than for every caller of `AllPointsIntersecting`: each surface keeps its curved trims' xyz edges, hits within the chord tolerance of them are untrusted on either side of the BSP (the BSP also drops hits inside the true rim but outside the chords, which sent the ray on to a farther surface), up to 16 directions, the farthest from a trim edge wins. A hit clamped onto the patch boundary is off the ray and not one. No fixture output changed |
| 5.3 F1 proof | Run the thread sweep with the plane turning disabled. Target fixed before coding: no unsound solid in the full grid, at the default and at a five times coarser chord tolerance. Then delete the turning from `pkg/sketch/thread.go`. | done 2026-10-02: 3 of 400 unsound generated once (at 0.03% and 0.01%), none at 0.1% or 0.5%; it was 68 without the fix and 24 with seven placements. `AddThread` generates once; `ThreadResult.Tries` is gone. Two thread goldens moved with the fix, in their own commit |
| 5.4 F2 closed loops | After the boundary-seeded pass in `SSurface.intersectByMarching`, search for seeds inside: sample each surface on a grid, project each sample onto both surfaces with `PointOnSurfaces`, drop seeds that lie on a curve already found, and march each remaining seed until it returns to its start. Fixture: a sphere bulging through one face of a box, so the circle lies inside one patch and inside the face. | done 2026-10-02, with seeds from a recursive subdivision of both surfaces instead of a grid (a grid made the thread imports four times slower; this adds about a sixth). A loop goes in as four curves, since a trim can't start and finish at one point. The grid triangulation now leaves out grid points within a trim edge's sag, which otherwise put a hole's rim through the face around it. `TestSphereThroughFace` in `pkg/srf`. No golden changed |
| 5.5 Sweeps | A sweep of that fixture over radius, depth and position, and a sweep of cylinder through cylinder at angles. Each reports its failure rate through `pkg/internal/sweep`. Goldens regenerated in their own commit. | done 2026-10-02 at a 0.01 mm chord tolerance: sphere 10 of 135 (99 before), cylinders 19 of 54, the same points before and after the step, kept as a guard. Follow-ups: the cylinder failures, and the sphere cap that crosses the seams at the largest bulge |

Order: 5.1 first, in a commit that changes no output. Then 5.2 and 5.3, then 5.4 and 5.5.

Done when: both sweeps meet their rates and the invariant baseline shows no fixture got worse.

Follow-ups from step 5, each a guard in its sweep's allowed rate until looked into:

- **Cylinder through cylinder** (`TestCylinderThroughCylinderSweep`, `pkg/srf/closedloop_test.go`):
  19 of 54 fail, the same points before and after the step; mostly unions and steep crossings,
  as failed Booleans or leaky meshes. The curves come from the boundary-seeded marching.
- **Sphere cap across the seams** (`TestSphereThroughFaceSweep`): the intersection at the largest
  bulge (0.4 of the radius, radius 7) leaks at every position, nine of the ten failures; it failed
  before the step too. The tenth is a cap so small that it triangulates flat against the face.
- **Threads at fine chord tolerance** (`TestThreadSweep`, `pkg/persist/slvs/thread_test.go`): 3 of
  400 unsound, `nut_m6_down_lh` at 0.03% and `M3 tapped`, `M4 tapped` at 0.01%. Since `AddThread`
  no longer turns its planes, such a thread is reported unsound to the user.

### Step 6. OpenGL and the section view

| Package | Content | State |
|---|---|---|
| 6.1 Renderer choice | `Window` gains `CreateRenderer()` so the platform picks the software or the GL renderer. Headless always picks software, so goldens do not move. | done 2026-10-02: `Window.CreateRenderer(useGL)`, and `PresentFrame` takes the canvas, so GL presents by swapping buffers. Headless returns software |
| 6.2 I1 GL renderer | New `pkg/render/gl` on `go-gl/gl/v2.1`, implementing the canvas interface of `pkg/render`. Polygons go through the Go triangulator. | done 2026-10-02: a port of `rendergl1.cpp` in `pkg/render/gl`, contexts from the new `pkg/render/gl/glx` (a window's, or an offscreen pixel buffer for tests). Without cgo or with the `nogl` tag both build and return `ErrUnavailable`. Two scenes drawn on both renderers differ in 3 and 190 of 43200 pixels under Xvfb |
| 6.3 I1 GLX window | A build-tagged file in `pkg/platform/tkgui` that makes a GLX context on the existing window from a second X connection, as the spike of 2026-09-29 did. `cmd/spacesolve` locks the OS thread. A setting and a command-line flag choose the renderer, and software stays the fallback when GLX fails. | done 2026-10-02, in `pkg/platform/tkgui/gl.go` on `glx` rather than a build-tagged file there (the stubs live in `glx`). Setting: Configuration > "draw with OpenGL" (on by default), switching both windows live; flag `--renderer opengl\|software` for one run. Failure logs once and draws in software |
| 6.4 I1 tests | Smoke tests under `make test-gui`: a frame renders, and a read-back pixel inside a known face has the face colour. No pixel goldens for GL. | done 2026-10-02: `pkg/platform/tkgui/gl_test.go` reads a face's colour back from a GL window and switches renderers; `pkg/render/gl` tests run in `make test-gui`, and `make coverage` runs under Xvfb so both GL packages are held to 85% |
| 6.5 I3 section view | A clipping plane taken from a selected workplane or face. The cap is the section polygon that `ExportSectionTo` already computes, filled. View menu entry and a screen for offset and flip. `Doc.SetSection(plane, offset)` and `Doc.ClearSection()`, and `Doc.Render` honours it on both renderers. | done 2026-10-02: `Sketch.SetSection(plane, offset, flip)`, with the offset typed in the document's unit (expressions and parameters allowed) and a face's normal taken out of the solid. View state: not saved, not undone. The cap is the section of the display mesh (`SMesh.MakeSectionPolygonInto`) rather than of the shell, so it meets the clipped mesh exactly. View > Section View (Ctrl+Shift+X), screen with offset, flip and off. `Doc.SetSection(plane, offset, flip)`, `ClearSection`, `Section` (with the cap's area), `Group.Faces`. Scenario `ext_section`, pair `path_section`, example `examples/scripts/section` |

Done when: the same model looks the same on both renderers by eye, and the section scenario
passes on the software renderer. Both hold: the application under Xvfb draws the sectioned
example the same with `--renderer opengl` and `software`.

Follow-ups from step 6:

- **Cap colour**: a fixed colour (`sectionCapColor` in `pkg/ui/section.go`); a style for it,
  or a hatch, would let the user change it.
- **Section in files**: the section is view state, as the camera is. If views are ever saved,
  it belongs with them.
- **Face normals**: extrusion side faces carry normals that point into the solid. The section
  turns them out using the mesh; anything else that relies on a face's normal may need the same.
- **GL on other platforms**: only GLX exists; the takigo backends for Windows and macOS would
  need their own context code.

### Step 7. First new solid features

| Package | Content | State |
|---|---|---|
| 7.1 E1 mirror entities | New entity types beside `EntityPointNTrans` in `pkg/core/entity.go` for a reflected point, normal and face, with solver expressions for reflection in a plane whose parameters can move. `Group.CopyEntity` in `pkg/sketch/group_generate.go` learns the reflected kind. | done 2026-10-03, without new entity types: a mirror copies as a link group with a scale of -1 does (point inversion, then a half turn about the plane's normal, then a translation), so its copies are the existing rotate-and-translate points, normals and faces made at scale -1 (`copyEntityScaled`). The group's seven parameters, the translation 2(o.n)n and the half turn (0, n), are tied to the plane by equations, so the copies follow the plane when it moves |
| 7.2 E1 mirror group | New group type with the mirror plane in `Predef`. The solid comes from `SShell.MakeFromTransformationOf` with a negative scale, as linked groups already do. Options: keep the original, and combine as union. Menu entry, screen, `Doc.Mirror(plane, name)`. | done 2026-10-03: `GroupMirror` (`mirror` in the file), plane in `Predef.EntityB`, a workplane or a planar face, `SkipFirst` for the reflection alone; it reflects its source group as step and repeat copies it, so a mirrored pocket is cut too and the combine is the source's. New Group > Mirror (Shift+Y) with its screen; `Sketch.NewMirror`, `Doc.Mirror`, `KeepOriginal`, `SetKeepOriginal`, `Plane`. Scenario `ext_mirror`, pair `path_mirror`, example `examples/scripts/mirror`, goldens `testdata/solid/mirror-*.spcs` |
| 7.3 E3 chamfer | New group type holding a list of edge entities and one or two distances. For each edge it builds a prism cutter from the two face normals and subtracts it. It accepts only edges that have entities. Selecting anything else gives a message that says so. Sweep over edge length, distance and dihedral angle. `Doc.Chamfer(edges, distance, name)`. | done 2026-10-03 with one distance: two would need a way to name which face of each edge takes which, left for step 9. `GroupChamfer` (`chamfer`, fields `edges` and `distance`), `Group.Edges`. Each edge's two faces are found from the triangles along it and its convexity by winding-number tests (`SMesh.WindingNumber`); the cutter is a wedge that reaches past both faces and twice the distance past the edge's ends (`SShell.MakePrism`), the wedges joined and taken away. An edge that isn't one, is concave, or is narrower than the distance gives `Group.Problem`, marked on the model and on the screen, and no cut. New Group > Chamfer Edges (Shift+Q), distance edited on the screen with expressions; `Sketch.NewChamfer`, `Doc.Chamfer`, `Distance`, `SetDistance`, `Edges`, `Problem`. `TestChamferSweep`: 84 of 84 sound with the exact volume (angle 20 to 165 degrees, distance, length). Scenario `ext_chamfer`, pair `path_chamfer`, example `examples/scripts/chamfer` |
| 7.4 E9 sweep | New group type with a profile group and a path made of lines and arcs, tangent continuous in the first version. Each line gives extrusion surfaces and each arc gives revolution surfaces, built with the code behind `MakeFromExtrusionOf` and `MakeFromRevolutionOf`. The segments are assembled into one shell with caps only at the two ends, so no Boolean between segments is needed. A path with a corner gives a diagnostic. `Doc.Sweep(profile, path, name)`. | done 2026-10-03: `GroupSweep` (`sweep`, `edges` the path), `SShell.MakeFromSweepOf` in `pkg/srf/sweep.go`: per curve and piece an extrusion or a surface of revolution per quarter turn, the curves at the joints shared, caps at the ends, no Boolean. The chain is ordered from the selection in any order and starts at the end nearest the profile's plane; branches, gaps, corners, a closed path, a path in the profile's plane and a bend tighter than the profile are each a diagnostic. Copies of the profile at both ends and the two cap faces are entities. New Group > Sweep Along Path (Shift+U); `Sketch.NewSweep`, `Doc.Sweep`. `TestSweepGrid`: 7 of 72 flagged, all a thin tube on a tight bend whose turn triangulates exactly as a Revolve's and which a Revolve shows at some chord tolerances too. Scenario `ext_sweep` (from `testdata/ui/sweep_path.spcs`), pair `path_sweep`, example `examples/scripts/sweep` |

Each package ends with goldens, an interface scenario, an example script and a path
equivalence test. Order: 7.1, 7.2, then 7.3 and 7.4 in either order.

Done: every package has its goldens (`scripts/regen-golden.sh solid`), scenario, example and
path pair, and the three keep one file across a save and load, an undo and redo, and one or
eight goroutines. Their group values and edges went into format version 2 without a bump.

Follow-ups from step 7:

- **Mirror of the whole solid**: a mirror reflects its source group, as step and repeat does;
  mirroring a body made of several groups means mirroring each, or a group of its own.
- **A solid mirrored onto itself**: a part symmetric about the plane, mirrored with the
  original kept, puts coincident faces through the Boolean, which fails (reported as a failed
  Boolean). The case is common enough to want handling.
- **Two chamfer distances**, once step 9 names faces.
- **Thin tori**: the solid check finds triangles of a thin tube's bend crossing their
  neighbours by about 2e-4 mm, in a sweep and in a Revolve alike (`sweepGridMaxFailRate`).
- **Benchmarks**: `BenchmarkRegen150` reads about 6% slower on this branch, all of it inside
  the solver's QR, which the step doesn't touch; adding one file the benchmark never runs moves
  it by 3%, so it is code layout, not work.
- **Untrimmed surfaces after a difference**: a Boolean keeps the cutter's surfaces it removed
  whole, with no trims, so a chamfered block's shell has 11 surfaces for 7 faces.

### Step 8. STEP import as a reference mesh

| Package | Content | State |
|---|---|---|
| 8.1 Part 21 reader | New package `pkg/persist/step`. A tokenizer and an entity table keyed by instance number, with typed access to parameters. `FuzzStep` from the first day. | done 2026-10-03: `step.Parse` (complex instances, `\X2\` and the other string escapes, typed values, comments, ANCHOR and SIGNATURE sections skipped), `FuzzStep` over parsing, mapping and tessellating, in `make fuzz`; three minutes, seven million inputs, no finding |
| 8.2 Boundary representation | Map the AP203 and AP214 entities for solids, shells, faces, loops, edges and vertices, and the surface kinds plane, cylinder, cone, sphere, torus and B-spline, into the package's own structures. Unknown surface kinds are counted and reported, and their faces are skipped. | done 2026-10-03: `step.ReadModel`; also AP242, faceted and shell-based models, voids, surfaces of extrusion and revolution, offset and rectangular-trimmed surfaces, rational and uniform B-splines and Bezier patches, conics, polylines, trimmed and surface curves. Units per representation context (SI prefixes and conversion-based, angles too), assembly placements through `NEXT_ASSEMBLY_USAGE_OCCURRENCE` in both orders of `rep_1` and `rep_2` (CAx-IF and SolidWorks), mapped items and mirroring operators, surface colours. What can't be read is counted by entity and named, with the first instance (`Report.Problem`, `Details`) |
| 8.3 Tessellation | Evaluate each surface in its own parameter space, trim by the face loops, and triangulate with `SPolygon.TriangulateInto`. This does not go through `SSurface`, so degree is not limited. | done 2026-10-03, with a new constrained Delaunay triangulator (`pkg/internal/cdt`, exact predicates) instead of `TriangulateInto`, which is bound to `SSurface` and adds no interior points: each face gets a staggered grid of points as dense as its curvature needs. Edges are sampled once and shared, so the mesh is closed. Seams, poles (a polar chart about a pole, the development of a cone), loops round a closed surface (two joined by a cut, one closed at its pole or, as OpenCASCADE does, straight across) and faces with no bounds. Faces side by side on all cores, the result the same on one |
| 8.4 Link entry | `pkg/persist/link` dispatches `.step` and `.stp`. The result fills `Group.ImpMesh` and gives a point entity per vertex in `ImpEntity`, so constraints can attach. Units and assembly placement are applied. `Doc.Link` needs no change. | done 2026-10-03: also a point at each circle's centre; the tessellation cached by file and chord tolerance (the sketch's `ChordTolPct` of the file's size), since undo reloads links. Loaders may report (`link.RegisterReporting`): `Group.LinkInfo` (protocol, unit, shapes, faces, triangles, tolerance) and `Group.LinkProblem`, on the group's screen and as a warning in the group list; a file that holds nothing readable links empty with the reason, a missing one asks to be found. `Group.LinkInfo`, `LinkProblem`, `examples/scripts/step`, scenario `ext_link_step`, pair `path_link_step` |
| 8.5 Corpus | The exporter is the first oracle. `testdata/export` holds 33 `.step` files, most of them from sketches without a solid, so the corpus adds exports of the solid fixtures: export, import, and require the volume to match within the chord tolerance. Then vendor files, starting with the fastener and the connector of the spike. | done 2026-10-03: 14 analytic shapes written by FreeCAD (`testdata/step/shapes`, `scripts/step-fixtures.py`) against worked-out volumes at three tolerances; 11 files kept in `testdata/step/corpus` (NIST MBE PMI, OpenCASCADE's screw, FreeCAD, Gmsh, BRL-CAD) and 16 more listed in `external.txt` and checked under `SPACESOLVE_STEP_CORPUS` (SolidWorks assembly and parts, knurled bolt and nut, more NIST), all against FreeCAD's volumes within area times tolerance. Every solid fixture exported by our writer and read back came out closed with its volume, but the knife-edge one (below). No connector file was at hand |

Done: every corpus file imports, closed and within the tolerance of the reference volume, or
fails with a message that names the unsupported entities (the CSG and swept-solid files), and
none panics.

Follow-ups from step 8:

- **Knife edges**: a solid whose faces meet along an edge four times (the
  `boolean_knife_edge` fixture, exported and read back) gives six naked edges.
- **Open thread ends**: the knurled bolt's thread ends are faces bounded by a single loop
  round a cylinder; they are closed straight across, as OpenCASCADE does, which leaves
  holes, as the file does (`external.txt` marks it open).
- **Faces through both poles**, such as a whole sphere or a lune from pole to pole, are
  triangulated in their parameters; only faces at one pole get the polar chart. They pass
  the tests, but a split at the equator would be safer.
- **Tessellated STEP** (AP242 `TRIANGULATED_FACE` and the like) and CSG and swept solids are
  reported, not read.
- **Self-intersections in tiny parts**: `linkrods.step` (5 mm across) passes the volume check,
  but Check Solid finds 24 crossing edges at the default tolerance.
- **Placement of a new link**: a linked part is placed at the middle of the view, as every
  link is; a STEP file is usually wanted where it was drawn.
- **Performance**: the triangulator's adjacency is a map; a 10 MB file reads in about a
  second, of which the triangulator is most.

### Step 9. Topology and edge naming

A two-day spike comes first. It derives vertices, edges and loops from the shell of every
fixture and checks Euler's formula on each. What fails there decides the design.

| Phase | Content | State |
|---|---|---|
| 9.1 Derived topology | New package `pkg/topo`: solid, face, loop, co-edge, edge, vertex, built from `SShell` by walking `SCurve.SurfA`, `SurfB` and each surface's `Trim` list. A validator: every edge has two co-edges of opposite sense, loops close, Euler's formula holds. The validator joins `check.Solid`. | done 2026-10-03. The spike found what the design needed: the two sides of an edge can trim by two coincident curves after a Boolean, so edges are matched by their vertices and points, not by curve; a Boolean can leave a T-junction, a trim running past a vertex where two faces meet on its other side, so edges are split at vertices inside them; outer loops run clockwise in SolveSpace's parameters. Every fixture solid validates and has its mesh's Euler characteristic, but the knife edge's, which is non-manifold on purpose. `check.Report` gains `TopologyProblems` and `Euler` (`topo=`, `euler=` in `testdata/invariants.txt`); problems make a solid unsound |
| 9.2 Topology as output of the kernel | `MakeFromBoolean` and the shell constructors produce topology directly instead of having it derived afterwards. `SShell` keeps its lists as storage. The trim handling in `pkg/srf/boolean_trim.go` becomes loop editing. | not taken, as the risk note allows: derivation works on every manifold fixture and costs about 7 ms for a threaded bolt of 270 faces (`BenchmarkBuildThread`); it is built lazily, per regeneration (`Group.Topology`) |
| 9.3 Local edit helpers | Split an edge, split a face along a curve, remove a face and heal, replace a face's surface. Each keeps the validator green. Steps 10 and 11 are built from these. | done 2026-10-03: `SplitEdge`, `SplitFace`, `RemoveFaces`, `AddFace` (taking the open edges it runs along, the surface turned to suit), `ReplaceSurface`; a box edge chamfered with them alone validates with the exact volume (`TestLocalChamfer`) |
| 9.4 F4 edge names | An edge name is the two `FaceID` values it separates and an ordinal. The ordinal comes from a geometric order along the face pair, not from creation order. A name that no longer resolves gives a diagnostic naming both faces. Written to `.spcs` as a reference kind. | done 2026-10-03: `topo.Name` ("edge A B N"), ordered by midpoints along the direction the edges between the pair spread most. Surfaces no entity names get `SSurface.Origin` from the group that made them (`srf.OriginFor`, bit 30), carried through Booleans, so all 4445 manifold edges of the fixtures have names, each resolving to itself. `solid-edge` fields in `.spcs` (version 2) |
| 9.5 Picking and scripting | Hover and selection of solid edges in the graphics window through the outline list, and a selection kind for them. `Group.Faces()`, `Edges()`, `Vertices()`, adjacency queries, and filters such as edges between two faces or edges parallel to a direction. | done 2026-10-03: `Selection.Edge`, hovered within six pixels of the projected edge and not hidden behind the solid, below entities and constraints and above faces; described in the text window. `Group.Solid()` with `Faces`, `Edges`, `Vertices`, `Edge(name)`, `EdgeAt`, `EdgesBetween`, `EdgesAlong`, `Problems`, `Euler`, and `SolidFace`, `SolidEdge`, `SolidVertex` with adjacency (`Group.Faces` and `Group.Edges` already meant other things). `examples/scripts/edges` |
| 9.6 Chamfer on any edge | E3 accepts named edges. Naming sweep: change an upstream dimension through its range and require every chamfer to stay on its edge or report. | done 2026-10-03: `Group.SolidEdges`, Chamfer Edges takes picked edges, `Doc.ChamferSolid`. `TestNamingSweep` moves a notch across the block in 23 steps: the chamfer of the second piece of the front's top edge stays on it, with the exact volume, while the notch splits the front, and reports otherwise; none moves silently. Scenario `ext_chamfer_solid_edge`, pair `path_chamfer_solid_edge`, goldens `chamfer-solid-edge`, `chamfer-of-chamfer` |

Done: the validator passes on every fixture and golden but the knife edge, and the naming sweep
reports no silent move of a chamfer to another edge.

Follow-ups from step 9:

- **9.2**, topology made by the kernel, if derived topology ever costs too much or a feature
  needs to keep identities through a Boolean.
- **Origins shift**: a surface's `Origin` is its place among its group's surfaces, so an edit
  that changes how many surfaces a group makes (a thread's length) renames the later ones.
- **A face picked by the mouse** is still the entity of its `FaceID`; faces that only have an
  `Origin` can't be selected as faces, only their edges.
- **Hidden edges** are tested against the active group's display mesh with a ray per hover;
  a large mesh makes hovering slower.
- **Non-manifold edges** (the knife edge) are reported as topology problems, not modelled.
- **Ordinals** follow geometric order; an edit that reorders the edges between two faces along
  their spread can move a name, as the sweep's design keeps from happening but does not forbid.
- **Loops through a vertex twice** choose the sharpest left turn, which no fixture exercises.

### Step 10. Fillets, shell and loft

Spikes first, as listed under "How to check an estimate before committing to it".

| Phase | Content | State |
|---|---|---|
| 10.1 E4 fillet, straight edge between planes | Local edit: split both faces along the offset lines, remove the strip, insert a cylinder patch, which the rational patch represents exactly, and rebuild the end faces. Chains of tangent edges are taken as one. | done 2026-10-03, but for chains: `topo.FilletEdge` cuts both faces along the cylinder's tangent lines and the end faces along its arcs (leaning end faces get the cross section projected along the edge), takes out the strip and puts in an exact cylinder patch; `FilletEdges` refuses edges that meet, as a corner is 10.2. The fillet group (Sketch -> Fillet Edges, Shift+F) edits the solid so far with no Boolean; entity edges or named ones, the radius an expression. `TestFilletSweep` in `pkg/topo`, 126 prisms over angle, radius, length and lean, and in `pkg/sketch`, 63 points, rate 0 for both, set before. Scenario `ext_fillet`, pair `path_fillet`, `examples/scripts/fillet`, goldens `fillet-edges`, `fillet-solid-edge` |
| 10.2 E4 corners | Three filleted edges meeting at a convex vertex of a box get a sphere patch. Other vertex cases give a diagnostic in this version. | not started |
| 10.3 E5 circular edges | Plane meeting cylinder gives a torus patch, and a chamfer there gives a cone. Both are exact in the patch type. Plane meeting cone follows. | not started |
| 10.4 E11 restricted shell | Offset every plane and cylinder inward, intersect neighbours again through the topology, drop the picked faces, and join inner and outer shells at the openings. A thickness that makes a face vanish gives a diagnostic. | done 2026-10-03 for planes; cylinders not yet. `topo.ShellSolid` builds the inner solid on the outer's topology, each face moved in but those taken away, which stay, each vertex where its faces' moved planes meet, and takes it away by a Boolean; an edge turned round says the walls meet; a vertex of more than three faces must stay one point. The shell group (Sketch -> Shell Solid, Shift+G) takes picked faces (`solid-face` in the file), the thickness an expression; `Doc.Shell`, `Doc.ShellSolid`, `Group.Thickness`, `SetThickness`, and `Solid.FaceAt`. `TestShellSweep` (180 prisms of polygons and an L over opening, thickness and height, against the moved cross-section's area) and `TestShellGroupSweep` (12 boxes), rate 0 for both, set before. Scenario `ext_shell`, pair `path_shell`, `examples/scripts/shell`, goldens `shell-open-box`, `shell-holed-block` |
| 10.5 E7 ruled loft | Two profile groups with equal segment counts and a vertex matching the user can rotate. One patch per segment pair, degree 1 across. | done 2026-10-03: `topo.MakeRuledLoft` builds the shell face by face; flat sides are planes trimmed to their trapezoid, twisted ones patches of degree 2 along the curves. The loft group (Sketch -> Loft To Selected Profile, Shift+O) joins the active sketch's one closed loop to that of the sketch an entity is picked from, a circle counting four arcs; the screen turns the second by a curve either way (`turn` in the file); it combines as an extrusion does. `Doc.Loft`, `Group.Turn`, `SetTurn`. Volumes by formula, h (A0/3 + A1/3 + K/6) with K the loops' mixed area; `TestLoftSweep` (81 points over polygon, circle, scale, height and turn, and 9 untwisted triangles, rate 0, set before); `TestLoftOnBlock` joins lofts to a block on, in, under and above it. Scenario `ext_loft`, pair `path_loft`, `examples/scripts/loft`, goldens `loft-on-block`, `loft-circle-to-square`. Found on the way: the grid mesher dropped its grid near any trim corner, shells built by `AddFace` met no Boolean (planes on a 1 mm square, curves marked as a Boolean's), and new requests in 3d groups saved differently after a reload |

Each phase carries a sweep whose acceptable failure rate is written into this table before the
phase starts.

Follow-ups from step 10:

- **Tangent chains and corners**: edges that meet are refused, both at a corner (10.2) and
  where they continue each other tangentially.
- **Strong twists**: a ruled face turned by a third of a turn or more meshes with false
  self-intersections and about 1% error in volume; the mesher's grid stops refining a twist at
  16 rows (`MakeTriangulationGridInto`'s depth), and the rows along the trim are ear-clipped.
  A deeper cap fixes the triangles in the sweep but makes it four times slower and changes the
  meshes of cones.
- **Chord loss after a Boolean**: a filleted block cut afterwards loses about 1 mm³ more than
  the fillet's own mesh, the cylinder's trim being cut into coarser chords.
- **Loft matching**: profiles must have as many curves; splitting curves to match counts, and
  lofts through more than two profiles, are not done.
- **Faces with only an Origin** (a loft's, a fillet's) can't be picked as faces, as for step 9;
  scripts reach them through `Solid.FaceAt` and `Doc.ShellSolid`.
- **Shell of curved faces**: cylinders, and so filleted or lofted solids, are refused; a
  cylinder's offset is a cylinder, but its vertices are no longer where three planes meet.
- **Shell vertices of four faces** that don't meet in one point once moved (an off-centre
  pyramid's apex) are refused; the shell would need a new edge there.
- **A Boolean's leftover surface**: a shell's inner face flush with a face taken away stays in
  the shell untrimmed, which the solid check counts as a face (Check Solid says 12 faces for an
  open box of 11), as step 7 noted for differences.

### Step 11. General surface type

| Phase | Content | State |
|---|---|---|
| 11.1 Multi-patch faces | A face in `pkg/topo` may own several patches joined by seams marked smooth. Seams are hidden from display, picking and naming. This alone makes approximated blends and offsets representable. | not started |
| 11.2 B-spline surface with knots | A surface type with arbitrary degree and knot vectors for storage, evaluation and exchange. It is cut into Bezier patches at its knots for the existing intersection and triangulation code. | not started |
| 11.3 Degree above 3 | Replace the fixed `[4][4]` arrays in `SSurface` with slices and lift the degree limit in evaluation, subdivision and the Bernstein tables of `pkg/srf/bernstein.go`. Benchmarks guard the regeneration path, since this touches the hottest loops. | not started |
| 11.4 STEP export | `pkg/ui/exportstep.go` writes one B-spline surface per face instead of one per patch. | not started |
| 11.5 Open the tier 4 items | E6, E8, E10 and D3 each get their own estimate and plan at this point. | not started |

### Risks to the plan itself

- **Step 1 is the only step with no user-visible result.** The pressure to skip parts of it
  will be highest there. Package 1.3 cannot be skipped or done later, since the baseline is
  only trustworthy while output still matches C++.
- **Package 9.2 is the largest single change in the plan.** If the spike shows the Boolean
  cannot produce topology directly without a rewrite, stay on derived topology, accept the
  cost of deriving after every Boolean, and move on. Steps 10 and 11 work on either.
- **takigo is a sibling checkout**, replaced in `go.mod`. Continuous integration and any
  contributor need both repositories at matching commits. The workflow pins takigo's commit in
  `TAKIGO_REF`, which has to move whenever `pkg/platform/tkgui` follows a newer takigo.

## Decisions still open

- **Topology first or surface type first** at step 9. Recommended: topology first, since fillets on planes and cylinders need it and do not need the new surface type.

## How to check an estimate before committing to it

For each kernel-dependent item, a two-day spike on the hardest fixture is cheaper than trusting
the table. Suggested spikes: a Boolean-route fillet on one edge of a box with a hole near the edge
for E4, a ruled loft from square to circle subtracted from a block for E7, and one real vendor STEP
file of a fastener and one of a connector for D2. `TestThreadSweep`, in `pkg/persist/slvs/thread_test.go`, is the model for a sweep-style
robustness test, and the naked-edge check in `pkg/ui/menuanalyze.go` is the pass criterion.
