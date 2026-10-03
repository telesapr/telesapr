# Roadmap: from a SolveSpace port to a modern CAD

Date: 2026-10-01, updated 2026-10-03. Status: steps 1 to 14 and I7 are done and merged, step 12 but for 12.4 (CI's first run on GitHub), which waits for the user. What steps 1 to 11 left open, the items that never had a step and the tier 4 items are ordered as steps 12 to 22 under "Finishing the roadmap"; 15 to 22 are not started. See "Progress" below and the State columns of the implementation plan. Read "Definition of done for every new feature" first.

## Context

TeleSAPR began as a Go port of SolveSpace, bit-identical with the C++. As of 2026-10-01 it
is a separate project: parity and file compatibility with SolveSpace are no longer goals, and
the kernel stays Go native. The parity record that `PORTING.md` held is in the git history.

This document lists what the program lacks compared with current parametric CAD, sizes each
item against the code as it stood on that date, and gives the order to do them in. The
"Implementation plan" section breaks that order into work packages with the files they touch.
Steps 1 to 11 of that order are done. "Finishing the roadmap" orders what is left.

## Progress

| Step | State | Merged into `master` |
|---|---|---|
| 1. Test infrastructure and the thread feature | done 2026-10-02, but for 1.8: the CI workflow has not run on GitHub, since the repository has no remote yet | `c5caea3` |
| 2. The `.tlspr` format and `.slvs` import | done 2026-10-02 | `d5b610c` |
| 3. Parameters and sketch feedback | done 2026-10-02, with follow-ups: a faster freedom analysis, a scenario for it, `make lint-new` on the whole branch, no `scale 0` in saves (`a33a1ae`), and the unit as the document's, format version 2 (3.9, `dba44b9`) | `22daacd` |
| 4. History editing | done 2026-10-02 | `fe7cb85` |
| 5. First kernel fixes | done 2026-10-02, with the cylinder-through-cylinder failures (19 of 54, not new) left as a follow-up | `f24fcd9` |
| 6. OpenGL and the section view | done 2026-10-02, with follow-ups listed under the step | `0a96960` |
| 7. First new solid features | done 2026-10-03, with follow-ups listed under the step | `a5829a6` |
| 8. STEP import as a reference mesh | done 2026-10-03, with follow-ups listed under the step | `e556586` |
| 9. Topology and edge naming | done 2026-10-03, on derived topology (9.2 not taken), with follow-ups listed under the step | `6f60438` |
| 10. Fillets, shell and loft | done 2026-10-03: 10.1 fillets of straight edges, 10.2 corners (spheres, mitres), 10.3 circular edges (tori, cones), 10.4 shell (flat faces only), 10.5 ruled loft; follow-ups 10.6 concave fillets, 10.7 smooth chains, 10.8 uneven mitres, 10.9 plane meeting cone, 10.10 a radius per edge; the rest listed under the step | `e1ee516`, 10.4 `2fbc6d4`, 10.2 and 10.3 `8461b2b`, 10.6 `54b6cb0`, 10.7 `5a2239e`, 10.8 `f1cf92e`, 10.9 `e86d002`, 10.10 `47ff6e0` |
| 11. General surface type | done 2026-10-03: 11.1 faces of several patches, 11.2 B-spline surfaces with knots, 11.3 degree up to 5, 11.4 STEP export of a face as one surface, 11.5 the tier 4 items sized and planned; follow-ups listed under the step | `a8a973b` |
| I7. Drawing with an AI model (outside the steps) | done 2026-10-03; follow-ups listed under the item | `baaec8d` |
| The name TeleSAPR (outside the steps) | done 2026-10-03: the project, until then spacesolve, is TeleSAPR; the module is `github.com/telesapr/telesapr`, the programs `telesapr` and `telesapr-cli`, the format `.tlspr` with the header `telesapr <version>`. It took the parts of 12.2 that are about the name | `74ff70b` |
| 12. Close the first eleven steps | done 2026-10-03 but for 12.4, which waits for the user: licence and notices, TeleSAPR 0.1.0 with its own icons, README, `TestMenuBarKeyboard` mended in the code, the guards set and kept from going stale, lint at 0 and run on everything, the audit in `docs/done-audit.md`, the user's guide in `docs/guide` | `f5c6971` |
| 13. Kernel soundness | done 2026-10-03: curves split at the seams of flat faces, ears that fold over skipped, 32 rows of twist, untrimmed surfaces dropped after a Boolean, a symmetric part its own mirror image, STEP meshes mended at T-junctions; the sphere and cylinder guards at 1 and 2, with their causes named | `101f59e` |
| 14. Names, faces and picking | done 2026-10-03: faces with only an Origin drawn apart and picked, a circle's fillet one face, names that count their edges and survive a longer thread, non-manifold edges reported where they are, hover on large meshes by a grid, Measure by outward normals | `f512ed1` |
| 15. Solid features completed | done 2026-10-03: chamfers by the fillet's edit (concave edges, chains; two distances not done), shells of prisms with round sides, apexes of four faces split, the draft angle on extrusions, the mirror of the whole solid, the pattern along a path, the stopped chain's message | MERGE15 |
| 15 to 22. Finishing the roadmap | planned 2026-10-03 from a review of every follow-up; not started | |

Decisions taken while doing them, beyond those below: the unit of length belongs to the
document and the format went to version 2 for it (2026-10-02, "Format decisions"); files broken
by hand get an error on load and no other safeguard (2026-10-02); with the name TeleSAPR the
format's extension went from `.spcs` to `.tlspr` and its header from `spacesolve` to `telesapr`,
and files with the old header are not read, the frozen corpus being renamed with the rest
(2026-10-03).

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
| Golden files | A `.tlspr` fixture per feature variant under `testdata`, with a regeneration script. A golden only changes in a commit that says why. |
| Save and load round trip | Load, regenerate, save gives the same bytes. A file from the previous format version still loads. |
| Undo and redo | Every creating and editing action is one undo step and comes back identical on redo. |
| Failure behaviour | Bad input gives a diagnostic on the group, never a panic and never silently wrong geometry. Each documented failure has a test. |
| Robustness sweep, for solid features | A parameter sweep over sizes, angles and placements in the style of the thread sweep, reporting the failure rate. The acceptable rate is fixed before the feature starts. Fuzz targets for parsers, with `.tlspr` and STEP first. |
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
- Reachable from `telesapr-cli run`, so the feature works without a window.
- Kernel foundations have no interface of their own, but expose what scripts can use. Topology gives scripts faces, edges and vertices to enumerate and select by. The test oracle gives scripts a `Check()` on a group.

### What existed, and what exists now

| Piece | State on 2026-10-01 | State on 2026-10-02, after step 3 |
|---|---|---|
| Headless interface scripts | 53 scenarios in `testdata/ui/scripts`. Every golden is a `.cppui` file recorded from C++, so a Go-only feature has nothing to compare against on `master`. | 67 scenarios: the 53 compared with C++, 11 Go-only (`ext_*`) and 3 path-equivalence pairs (`path_*`). |
| Go-recorded interface goldens | Exist only on the `threads` branch: one file, `testdata/ui/golden/ext_thread.goui`, with the regeneration switch `TELESAPR_REGEN_GOUI`. | 11 `.goui` goldens, `scripts/regen-goui.sh`. The C++ comparisons hide what C++ lacks through test-only switches (`cppComparison`, `goOnlyMenuItems`, `legacySlvsSave`) until package 5.1. |
| Script API | Covers drawing, constraints, export, and the stock groups except helix. TTF text requests are also missing. Its own package tests reach 31% of its statements. | Every group type and creating command has a method (`completeness_test.go`); parameters, freedom, conflicts, commands, measure and units added. Its own package tests reach 55%; more is covered from other packages. |
| Example scripts | One, `examples/scripts/bracket`. | Three, `bracket`, `bolt` and `plate`, each run by a test. |
| Generated symbol check | `make generate-check` exists. | Unchanged. |
| Continuous integration | `make ci` and `.github/workflows/ci.yml` were prepared on 2026-10-01 and pass locally. The workflow has not run on GitHub, since the repository has no remote yet. | Unchanged: passes locally, never run on GitHub. `make lint-new` now lints from the branch's merge base with `master`. |
| Fuzzing | None. | `FuzzExpr`, `FuzzLoad` (`.slvs`) and `FuzzRead` (`.tlspr`), `make fuzz`. |
| Geometric invariant checks | Only inside C++ parity tests, as watertight and volume comparisons for Boolean fixtures. | `pkg/check`, Analyze > Check Solid, `Group.Check()`, and a baseline of 352 lines in `testdata/invariants.txt`. |

On 2026-10-03, after step 11: 107 scenarios, 31 of them Go-only (`ext_*`) and 23 path pairs
(`path_*`), with 84 `.goui` goldens; 21 example scripts, each run by a test; four fuzz targets
(`FuzzExpr`, `FuzzLoad`, `FuzzRead`, `FuzzStep`); 34 benchmarks; 13 packages held to 85% in
`scripts/coverage-packages.txt`. Continuous integration is as the table says: it passes
locally and has never run on GitHub (package 12.4).

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
B3 was in no tier and no step; it is package 16.11.

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

## Own file format `.tlspr` (proposed by the user as `.sps`, 2026-10-01; `.spcs` until 2026-10-03)

Proposal: TeleSAPR saves only its own format, `.tlspr`. Upstream `.slvs` files are opened
through File, Import, never through Open, and never written. This replaces T2.

**Assessment: agreed, and it is the cleaner option.** A format that is "slvs plus records
upstream cannot read" would mislead users into thinking files round-trip. A separate extension
states the truth. Import-only also means the importer is a one-way converter, so it never has
to preserve upstream quirks on write.

**State before step 2.** The format as built is described in `docs/tlspr-format.md`; it is at
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
| Link groups | Point at `.tlspr` files only. Importing an assembly converts every linked `.slvs` recursively, see below. |
| `.slvs` writer | Leaves the product. It stays in the test tree until the C++ comparison tests are frozen under T1, then it is deleted. |
| Fixtures | The 113 `.slvs` files stay as the import test corpus. New goldens are `.tlspr`. |

**Effort**

| Variant | Effort | Notes |
|---|---|---|
| S1. Same record grammar, new header and extension | about 1 wk | Fork of the existing reader and writer. Cheap, but inherits a flat grammar that every later feature will fight. |
| S2. New extensible text grammar | 2 to 2.5 wk | Reader, writer, version handling, import path, dialogs, backup and autosave names, command-line tool, script runner, fixture conversion, round-trip tests. Recommended. |
| S3. Packed container variant | 1 wk more | Can wait until D2 STEP import. Not taken with step 8; it is package 17.5. |

**Format decisions taken by the user (2026-10-01)**

- ~~**Stored expressions are in millimetres and degrees** (decided 2026-10-01). The file does not depend on the view unit, which is an interface preference. The interface converts when it shows and when it reads an expression typed in view units. The `units` line records the unit the document was authored in, for display only.~~
- **Revised 2026-10-02: the unit belongs to the document** (format version 2, package 3.9). The `units` line is the document's unit; lengths written with `=` are in it, angles in degrees, and parameters are plain numbers that take the unit of what they drive. Typed text other than a plain number is always kept. Everything else in the file stays in millimetres. Unit suffixes in expressions (`1/4 in`) were considered and not taken.
- **Extension is `.tlspr`.** The first idea, `.sps`, clashes with SPSS syntax files.
- **Grammar is the new extensible text form (S2).** Version header, named blocks with braces, `name = expression` for values that keep their expression. Sketch of the shape agreed:

  ```
  telesapr 1
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
  - Each linked `a.slvs` becomes a sibling `a.tlspr`. If one already exists, the user chooses reuse or overwrite, once for the whole import.
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
2. **Done 2026-10-02.** **The `.tlspr` format (S2) with `.slvs` import.** About 3 weeks, with a fuzz target for the reader and `Save`, `Open` and `Import` in the script API. Everything after this may add records to the format.
3. **Done 2026-10-02, with document units as a follow-up.** **A1 named parameters, then B1, B2, I5, I4.** About 4.5 weeks. No kernel work, large change in feel. Scripts get parameters by name, constraint state per entity, and measurements.
4. **Done 2026-10-02.** **C1 to C3 history editing.** About 4 weeks. Scripts get reorder, suppress and the list of broken references.
5. **Done 2026-10-02.** **F1 raycast fix and F2 closed intersection loops, on by default.** About 3 weeks, most of the addition being sweeps. Remove the rotate-and-retry workaround in the thread feature afterwards.
6. **Done 2026-10-02.** **I1 OpenGL backend, then I3 section view.** About 3.5 weeks. Goldens stay on the software renderer. Scripts get the section plane and a rendered image.
7. **Done 2026-10-03.** **E1 mirror, E3 chamfer by Boolean, E9 sweep along lines and arcs.** About 6 weeks. First new solid features, all on existing machinery, each with a robustness sweep. The chamfer takes only edges that exist as entities until step 9.
8. **Done 2026-10-03.** **D2 STEP import as reference mesh.** About 4.5 weeks, with a corpus of real files and a fuzz target for the reader.
9. **Done 2026-10-03, on derived topology.** **F3 topology as the primary representation, then F4 edge naming.** About 10 weeks. The first large kernel investment. Scripts get faces, edges and vertices to enumerate and select by, which later features need for their own API. The chamfer is extended to any edge of the solid.
10. **Done 2026-10-03, the shell for flat faces only.** **E4 and E5 fillets on straight and circular edges, E11 restricted shell, E7 ruled loft.** About 20 weeks.
11. **Done 2026-10-03.** **F6 general surface type.** About 10 to 12 weeks. Then E6, E8, E10 and D3 become ordinary hard features instead of impossible ones; their plans are under step 11.

Steps 1 to 8 are about 34 weeks at conventional pace and deliver most of the visible gap.
Steps 9 to 11 are about 41 weeks, between 9 and 10 months, and are where the native kernel is
actually built out.

The order goes on with steps 12 to 22, under "Finishing the roadmap".

### Items outside the numbered steps

The steps above cover about 75 of the 112 weeks in tiers 0 to 3. The remaining items are below.
None blocks a step. Each can be taken between steps once its prerequisite has landed.
Efforts include the 20%. None was taken between steps 1 and 11; the last column is the
package that now holds each, under "Finishing the roadmap".

| Item | Earliest slot | Prerequisite | Effort | Package |
|---|---|---|---|---|
| A2 configurations | after step 3 | A1 | 1.2 wk | 16.2 |
| A3 dimensions referenced by name | after step 3 | A1 | 0.6 wk | 16.1 |
| G5 motion study | after step 3 | A1 | 1.8 wk | 16.3 |
| C4 insert a group mid-history | after step 4 | C1 | 0.6 wk | 16.4, since it exists but for its tests |
| D1 3MF export | after step 2 | none | 0.6 wk | 16.5 |
| D4 STL and OBJ reference import | after step 2 | none | 1.2 wk | 16.6, binary STL being there |
| D5 STEP export with assembly structure | after step 2 | none | 1.2 wk | 16.8 |
| G2 bill of materials | after step 2 | none | 0.6 wk | 16.7 |
| G1 mate shortcuts | after step 2 | none | 2.4 wk | 16.9 |
| G3 shell-based interference check | after step 5 | F1 | 1.2 wk | 16.10 |
| F8 parallel `MergeCoincidentSurfaces` | after step 5 | goldens frozen | 1 wk | 17.2 |
| F7 regeneration off the UI thread | after step 5 | goldens frozen | 3.6 wk | 17.3 |
| E2 pattern along a curve | after step 7 | E9 path handling | 1.8 wk | 15.6 |
| H1 drawing sheet | after step 6 | none | 3.6 wk | 18.1 |
| H4 title block and templates | after H1 | H1 | 1.2 wk | 18.2 |
| H3 section and detail views | after H1 | H1 | 2.4 wk | 18.3 |
| I2 shader backend | after step 6 | I1 | 3.6 wk | 19.1 |
| I6 appearance beyond colour | after I2 | I2 | 1.2 wk | 19.2 |
| E13 draft angle | after step 9 | F3 | 4.2 wk | 15.4 |
| H2 associative drawing dimensions | after step 9 | F4, H1 | 5.4 wk | 18.4 |

About 39 weeks in total. A2, A3, C4, D1, D4 and G2 are each under a week and a half and are the
natural fillers between the long steps.

#### I7. Drawing with an AI model (added 2026-10-03)

Asked for by the user on 2026-10-03: say in plain English what to draw. It was not in the
estimates above. Decisions taken that day:

- **An MCP server, and nothing else.** TeleSAPR has no model client, API key or chat window.
  It offers the drawing to any Model Context Protocol client, which brings the model; that is
  also what keeps it independent of a provider.
- **The model writes scripts.** Its tools run Go against `pkg/script/api`, so every feature
  that meets the definition of done is available to it with nothing added, and no second
  command vocabulary exists to drift from the first.
- **Applied at once, one undo step each**, with a setting that has the user asked first. A
  script that fails is rolled back.
- **No sandbox.** A model's script can do what a user's can.
- **The protocol lives in `telesapr-cli`.** The client starts `telesapr-cli mcp`, which
  passes each call to the application over a per-user Unix socket. The application does not
  link the MCP SDK, and the connection is made at each call, so the two may start in either
  order.

Developed on the branch `mcp-server`, in a worktree, while step 11 was in progress on its own
branch, and merged into `master` after step 11 as `baaec8d` on 2026-10-03. `docs/ai.md` is the
user's guide.

| Package | Content | State |
|---|---|---|
| I7.1 what a model needs of the scripting engine | `Doc.Describe` and `Group.Describe`, the drawing in words with the names the lookups take. `Engine.Run`, several statements as one undo step on a console's state. `script.Help` by name. `Rollback` on both hosts, which takes a step back with nothing left to redo. | done 2026-10-03 (`91c5a73`). Example `examples/scripts/describe` |
| I7.2 `pkg/script/agent` | `Backend` (run, describe, render, help), `Session` on an engine with the output captured and a failed script rolled back, the primer and the instructions a model reads, and `Serve`, `Remote`, `Listen`, `Connect` for the socket: JSON lines, a cancel request, a protocol version. The socket's directory is per platform. | done 2026-10-03 (`ce6a24f`). 91% covered, in `scripts/coverage-packages.txt` |
| I7.3 `pkg/mcpserver` and `telesapr-cli mcp` | Four tools, `run_script`, `get_drawing`, `render`, `api_help`, on `github.com/modelcontextprotocol/go-sdk` v1.6.0; instructions and each description under 2 KB, since clients cut them. `mcp` reaches the application's socket; `mcp --headless [sketch.tlspr]` serves a drawing of its own. | done 2026-10-03 (`d3ad119`). `testdata/mcp/bracket.jsonl`, a recorded conversation with a mistake in it, saves the bytes `examples/scripts/bracket` saves |
| I7.4 the application | Script > Allow AI Connections (setting `AgentAllowed`, off by default), `telesapr --agent` for one run, scripts run through the script console with an `ai>` prompt and are stopped by Script > Stop, "ask before an AI program's script runs" on the configuration screen (`AgentConfirm`). | done 2026-10-03 (`11d176a`, goldens `9adeb82`). Scenario `ext_ai_connections`; `pkg/ui/agent_test.go` on a platform with an event loop |

No test calls a model: a conversation is a recorded list of tool calls.

Follow-ups, none started:

- [left out] A chat window inside the application, on the same `Backend`, for users without an MCP client.
- [16.14] `render` of the view as the window shows it, and of a chosen group.
- [left out] A suite of English prompts run against a real model and judged by the invariants
  (`make ai-eval`, outside CI), to measure whether a change to the primer helps.
- [16.14] A tool, or a field of `get_drawing`, for what the mouse hovers, so that "this edge" works
  without a selection.
- [left out] MCP resources and prompts, and an HTTP transport for clients that take only a URL.
- [left out] A sandboxed engine for a model's scripts, if the decision above is revisited.

## Implementation plan

This section turns the recommended order into work packages. A package is numbered by step,
so 3.2 is the second package of step 3. Paths and symbols were checked against `master` at
commit `ce5bfe4` on 2026-10-01. Steps 1 to 5 are planned to the file level. Steps 6 to 8 are
planned to the package level. Steps 9 to 11 are planned as phases, each opened by a spike.
Under each step's follow-ups, square brackets hold the package of "Finishing the roadmap"
that takes the follow-up, `[left out]` or `[closed]`, as the review of 2026-10-03 placed them.

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
| 1.1 Go-recorded interface goldens (T3) | Bring the `.goui` comparison from `threads` commit `614b2d4` into `pkg/ui/uiscript_test.go`: a test beside `compareUIScripts` that reads `testdata/ui/golden/<name>.goui` and rewrites it when `TELESAPR_REGEN_GOUI=1`. Scripts with the prefix `ext_` are Go-only. Add `scripts/regen-goui.sh`. | done 2026-10-02: `TestGoScripts`, `scripts/regen-goui.sh` |
| 1.2 Invariant oracle (T1) | New package `pkg/check`. `check.Solid(shell, mesh)` returns a report: naked edge count, self-intersecting edge count, face count, volume, centre of mass, bounding box, and the group's `BooleanFailed` flag. It reuses `SKdNode.MakeCertainEdgesInto` and `SMesh.CalculateVolume`. The centre of mass code moves out of `pkg/ui/menuanalyze.go` into the package. | done 2026-10-02. Centre of mass was already `SMesh.GetCenterOfMass`; `check.Group(sk, g)` takes the shell of the last group that made solid |
| 1.3 Invariant baseline | `TestInvariants` in `pkg/persist/slvs` loads each of the 113 fixtures, regenerates, and compares the report with `testdata/invariants.txt`, one line per fixture and group. Counts must match exactly. Volume and centre of mass must match to a relative 1e-9 while parity holds. The tolerance is loosened to 1e-6 in package 5.1. `TELESAPR_REGEN_INVARIANTS=1` rewrites the file. The baseline is recorded now, while the output is still verified against C++. | done 2026-10-02: 287 lines from 113 fixtures, then 65 more for the thread goldens. The one invalid fixture is recorded with its load error |
| 1.4 Oracle in the interface and scripts | Analyze menu entry "Check Solid" in `pkg/ui/menu_table.go`, reporting through the existing message path and filling `ss.NakedEdges` for display. `Group.Check()` in `pkg/script/api/group.go` returns the same report. | done 2026-10-02: Analyze > Check Solid (Ctrl+Shift+O), `Group.Check()`, first scenario `ext_check_solid` |
| 1.5 Path equivalence helper (T7) | `requireSamePaths(t, uiScript, goScript)` in a new `pkg/ui/pathequiv_test.go`. It runs the interface script on `uiScriptRunner`, runs the Go script through `script.Engine` on an `api.SketchHost`, and compares `Document.SaveRegenerated()` bytes. First user: one extrusion built both ways. | done 2026-10-02: pairs `testdata/ui/scripts/path_*.script` and `.go`, compared byte for byte. It found that `Doc.Rect` built another rectangle than the tool; fixed. An interface script must not save, since a save regenerates and the solver is not a fixed point |
| 1.6 Script API completeness (T4) | `pkg/script/api/completeness_test.go`, as an external test package so it can import `pkg/ui`. It fails for a `sketch.GroupType` with no creating method, for a creating menu command with no method, for an exported method with no doc comment (parsed with `go/doc`), and for an example under `examples/scripts` that does not run. It will fail at once on helix and TTF text, so the package adds `Doc.Helix` and `Doc.Text`. | done 2026-10-02. Added `Doc.Helix`, `Text`, `Image`, `TangentArc`, `Split`, `Constraint.SetOther`; fixed `Doc.Revolve`, which turned four times too far, and the builtin font name, which matched no font. The example check lives in `cmd/telesapr-cli`, where `run` is |
| 1.7 Sweep and fuzz harness (T5) | `pkg/internal/sweep`: runs a function over a parameter grid, collects failures, and fails the test above a stated rate. A small sample runs by default and the full grid under `TELESAPR_SWEEP=1`. Fuzz targets `FuzzLoad` over `Document.parse` in `pkg/persist/slvs` and `FuzzExpr` over `core.NewFromString`. `make fuzz` runs each for 30 seconds. | done 2026-10-02: `pkg/internal/sweep` (with a per-sample allowance), `FuzzExpr`, `FuzzLoad`, `make fuzz`. No findings in the first 30 s of each |
| 1.8 Continuous integration (T6) | `make ci` runs `fmt-check`, `vet`, `build`, `generate-check`, `coverage`, `test-race`, `test-gui` and `lint-new`. `scripts/coverage-check.sh` applies the 85% rule to the packages listed in `scripts/coverage-packages.txt`, which is empty until the first new package. `.github/workflows/ci.yml` runs the same targets and checks out takigo beside the repository at a pinned commit, because `go.mod` replaces it with `../takigo`. The existing code has about 100 lint findings, so only changed lines are linted. Left to do: push, add the `TAKIGO_TOKEN` secret, and see the first run pass. | prepared 2026-10-01, not yet run on GitHub |
| 1.9 Thread feature to the bar | Rebase `threads` onto `master`. Add `Doc.Thread` over `Sketch.AddThread`, with setters for what the thread screen edits. Add `examples/scripts/bolt`, a misuse test, a path equivalence test against `ext_thread.script`, and `Check()` on every thread golden. Move `TestThreadSweep` onto `pkg/internal/sweep`. Remove the `Extensions` gate, since there is no C++ comparison to protect. Merge. | done 2026-10-02 on the step branch: gate removed, `Doc.Thread`, `examples/scripts/bolt`, `path_thread`, golden checks, sweep at 24 of 400 unsound (none at the default tolerance). `thread/bolt_m8x30.slvs` is not a C++ save and is out of the round trip |

Order: 1.1, 1.2, 1.3 first, since 1.3 must be recorded before anything changes output. Then 1.4
to 1.8 in any order. 1.9 last, since it uses all of them.

Done when: `make ci` passes on `master` with the thread feature merged, and T4 passes with no
exclusion list.

### Step 2. The `.tlspr` format

| Package | Content | State |
|---|---|---|
| 2.1 Grammar | `docs/tlspr-format.md` with the grammar in EBNF and one example per block kind. Stored expressions are in millimetres and degrees. Decide here: string escaping, comment syntax, and how handles are written. | done 2026-10-02: Go string literals, `#` comments, handles as `0x` hex without leading zeros; group values that will take expressions are written `copies = 3` already |
| 2.2 Syntax tree | New package `pkg/persist/tlspr`. `lex.go` and `parse.go` produce a generic tree of blocks, fields and lists with line and column on every node. `print.go` writes a tree back. No knowledge of the sketch at this layer, so an unknown block can be reported by position and skipped. | done 2026-10-02 |
| 2.3 Model mapping | `encode.go` and `decode.go` map the tree to `sketch.Sketch`: groups, requests, entities, constraints, styles, solved parameter values, units and document settings. Stable order: groups by `Order`, everything else by handle. Floats are written in the shortest form that reads back to the same bits. | done 2026-10-02. `Document.Encode` re-emits a decoded file unchanged; an entity's workplane is a plain handle, since copied entities have 0, not free-in-3D |
| 2.4 Version handling | Header `telesapr <major>`. A newer major is refused with a message naming both versions. `testdata/tlspr/v1` holds files that every later reader must load. | done 2026-10-02: `VersionError`; nine files in `testdata/tlspr/v1`, one of them (`fields.tlspr`) setting every field |
| 2.5 Link loading | Move `ReloadAllLinked`, `RebaseLinks` and `LoadLinkedImage` from `pkg/persist/slvs/linked.go` into a format-neutral `pkg/persist/link`, which picks a loader by extension: `.tlspr`, `.emn`, `.stl`. `.slvs` is not among them. | done 2026-10-02: formats register a loader by extension (`link.Register`), so `tlspr` registers itself; `slvs.LoadFile` passes its own `.slvs` loader as a fallback for the C++ tests |
| 2.6 Import | `pkg/persist/convert`. `Plan(root)` walks the link tree, detects cycles, and returns the files it would write. `Run(plan, policy)` writes them, with reuse or overwrite chosen once. The `.slvs` reader stays where it is and gains nothing. | done 2026-10-02: `MakePlan`, `Run`, and `File` for batch use. A linked copy that exists is reused unless overwritten; the root is always written |
| 2.7 Product switch | `pkg/ui/menufile.go`: `backupExt` becomes `tlspr~`, Open and Save use `.tlspr`, and File, Import gains "SolveSpace model" with the plan shown before writing. `pkg/platform/filters.go`: new filter lists. `cmd/telesapr-cli/main.go`: default output pattern `%.tlspr` and a `convert` command. `cmd/telesapr-cli/script.go`: the sketch-argument test accepts `.tlspr`. `pkg/script/engine.go`: help text. | done 2026-10-02. Opening a `.slvs` file points at File > Import; a test-only hook, `legacySlvsSave`, keeps the C++ interface scenarios on `.slvs` until 5.1 |
| 2.8 Script API | `Doc.Save` and `api.Open` work on `.tlspr`. New `api.Import(src, dst)` over `convert`. Saving to a `.slvs` path is an error that names the reason. | done 2026-10-02. `api.Import` reuses linked copies and refuses a plan with missing files; `Open` refuses a file with parts it does not know, which a save would lose. Scripts import `persist/tlspr` and `persist/convert` in place of `persist/slvs` |
| 2.9 Retire the `.slvs` writer | Move `Document.Save`, `SaveRegenerated` and the `write*` functions to `pkg/internal/slvswrite`. A test fails if a non-test file imports it. It is deleted in package 5.1. | done 2026-10-02: `slvswrite.Save` and `SaveRegenerated`, guarded by `TestOnlyTestsImport` |
| 2.10 Tests | For each of the 113 fixtures: import, save, load, save again, and require the same bytes and the same invariant report as 1.3. `FuzzRead` over the parser and the decoder. A golden `.tlspr` for each fixture directory. An interface scenario for Import and one for Save and Open. A `convert` test on an assembly with a cycle, a missing link and a read-only directory. | done 2026-10-02: `TestFixturesRoundTrip` (decode and encode is the identity on every fixture; load, regenerate and save is, except 15 fixtures the solver never settles, with or without the format), 61 directory goldens, `ext_import_slvs` and `ext_save_open`, `FuzzRead` 10 minutes with no finding. It found an order dependence in the 1.3 baseline, fixed in its own commit |

Order: 2.1 to 2.4, then 2.5 and 2.6, then 2.7 to 2.9, with 2.10 growing alongside.

Done when: the product writes no `.slvs`, all fixtures round trip, and the fuzz target has run
ten minutes without a finding.

### Step 3. Parameters and sketch feedback

| Package | Content | State |
|---|---|---|
| 3.1 A1 engine | `sketch.Parameter{Name, Expr, Value}` and `Sketch.Parameters`, added to `UndoState`. Expressions and values are in millimetres, as in the script facade. An expression string beside `ValA` in `core.ConstraintBase` and beside `ValA`, `ValB`, `ValC` in `sketch.Group`. `Sketch.EvaluateParameters` runs at the top of `generateAll`: it parses with `core.NewFromString`, orders parameters by dependency, evaluates with `Expr.Eval(ctx, vars)`, and writes the numbers into the `Val` fields. A cycle or an unknown name leaves the last good value and sets a diagnostic that names the chain. | done 2026-10-02: `sketch.Parameter`, `EvaluateParameters` at the top of `generateAll`, `SetConstraintText` and `SetGroupText` as the one entry point for typed values, `UndoRememberFor`. A plain expression is evaluated once in view units as before; only one that names a parameter is kept. Found and fixed two parser bugs on the way: trigonometry took radians, and `2*(3)` did not parse |
| 3.2 A1 persistence | `param name = expr` lines, and `field = expr` wherever a stored expression exists, as in the agreed sketch of the format. A file without expressions is byte-identical to before. | done 2026-10-02: also `scale = expr`. What cannot be evaluated is kept and reported apart from skipped parts (`Document.ExprErrors`); `testdata/tlspr/v1/parameters.tlspr` added |
| 3.3 A1 interface | A "Parameters" screen in `pkg/ui/textscreens.go`, linked from the group list: add, rename, edit, delete, with the evaluated value beside each. Renaming rewrites the expressions that use the name. The dimension editor in `pkg/ui/mouse.go` and the group value editors keep the typed text when it is not a plain number. A dimension that has an expression shows it in the editor and the value on the canvas. | done 2026-10-02: Edit > Parameters (Ctrl+Shift+M) and a link under the group list; the group screen shows `(= expr)`; scale and pitch edits became undo steps. The C++ comparisons hide the Go-only links (`cppComparison`). Scenarios `ext_parameters`, `ext_parameter_copies` |
| 3.4 A1 scripting | `Doc.Parameters`, `Doc.Parameter(name)`, `Doc.SetParameter(name, expr)`, `Doc.DeleteParameter(name)`. The existing `api.Param` type is the solver's parameter, so the new type is named `Parameter` and the doc comments say which is which. `Constraint.SetExpr` now stores the text. `Constraint.Expr` and the group equivalents read it. | done 2026-10-02: `Doc.SetParameter` adds or changes; also `RenameParameter`, `ParameterUsers`, `ExprErrors`, `Group.SetExpr/Expr/Value`. `examples/scripts/plate`; `path_parameters`, for which a new distance's label offset became the engine's (`Sketch.DistanceLabelOffset`) |
| 3.5 B1 entity state colouring | `SolveGroupAndReport` already takes `andFindFree`. Turn it on for the active group after every solve, behind a setting, after a benchmark on the largest sketch in `testdata` shows the cost. `pkg/ui/drawentity.go` colours an entity by whether all, some or none of its parameters are free. `Entity.Free()` in the API. | done 2026-10-02 without a second solve: `Solver.FreeParams` finds all free unknowns in one reduction to row echelon form at the solved values, so it cannot move a save; checked against the rank test on random systems and against Show DOF on every fixture. Configuration setting, on by default. Equalities are substituted and one-unknown rows dropped before the dense step: 1.5 ms at about 600 unknowns (regeneration 2.0 ms) and 7.2 ms at about 1,600 (regeneration 9.8 ms). Scenario `ext_freedom`, with a `freedom` dump command |
| 3.6 B2 conflict highlighting | `Group.Solved.Remove` holds the list and `ShowGroupSolveInfo` prints it. Add canvas highlighting in `pkg/ui/drawconstraint.go` and a jump that zooms to the constraint. `Group.Conflicts()` in the API. | done 2026-10-02: red, `[show]` on the failure screen, scenario `ext_conflicts` |
| 3.7 I5 command search | A palette over the `menuEntry` table: type to filter, Enter to run the entry's handler. One accelerator. `api.Commands()` lists entries and `Doc.RunCommand(label)` runs one. On a host with no window it returns an error. | done 2026-10-02: Edit > Find Command (Ctrl+K). The editor reports only Enter, so Enter lists and a second Enter runs the first match. Scenario `ext_find_command` |
| 3.8 I4 measure | A mode that shows distance, angle or radius for the selection on the canvas without creating a constraint, reusing the computation in `pkg/ui/describescreen.go`. `Doc.Measure(a, b)` returns the same numbers. | done 2026-10-02: `Sketch.Measure`; Analyze > Measure Selection (Ctrl+Shift+E) labels the selection on the canvas. Scenario `ext_measure` |
| 3.9 Document units (follow-up) | The `units` line becomes the document's unit: typed lengths and `=` lengths in the file are in it, so typed text can always be kept. Format version 2; version 1 files read as before and are upgraded on save. View > Dimension Units sets the document's unit, keeping sizes, as an undo step. | done 2026-10-02: `Sketch.Units`, `Sketch.SetUnits` (wraps length expressions in the factor and lists the parameters to check), `Doc.Units`/`SetUnits`, `testdata/tlspr/v2`, scenario `ext_units`. In the C++ comparisons the unit stays the view's only. A plain length in a non-mm document moves by an ulp on its first reload, then is stable |

Order: 3.1 to 3.4 in sequence, then the rest in any order.

Done when: a model driven by three parameters can be built, saved, reopened and changed
through the screen and through a script, with identical files.

### Step 4. History editing

| Package | Content | State |
|---|---|---|
| 4.1 Dependency query | `Sketch.GroupDependencies(h)`: the groups a group refers to through `OpA`, `OpB`, `ActiveWorkplane`, `Predef`, the entities its requests and constraints name, and the sources in `Remap`. Tested on every fixture by checking that each group depends only on earlier ones. | done 2026-10-02: each dependency carries the first reason found ("operand", "constraint c012"); worked out from the stored model, so it holds before a regeneration; a linked group's remap is left out, since it names entities of its file. `GroupDependents` for the other direction |
| 4.2 C1 reorder | `Sketch.MoveGroup(h, before)` renumbers `Order` and refuses a move that puts a group ahead of a dependency, naming it. Up and down links in `ShowListOfGroups`. `Group.MoveBefore(other)` and `Group.DependsOn()` in the API. Sweep: every legal move on every fixture, then move back, and require the original bytes. | done 2026-10-02: the groups keep their set of order numbers, so a move back restores the file; arrows in the group list and Edit > Move Group Earlier/Later; `MoveToEnd`, `UsedBy`. `TestMoveGroupSweep`: 192 moves allowed, 908 refused, every allowed one regenerates without pruning. Scenario `ext_move_group`, pair `path_move_group` |
| 4.3 C2 full suppress | A second flag beside `Group.Suppress` that also skips the group's solve and entities. Groups that depend on it get a diagnostic instead of a pruned reference. Checkbox in `ShowGroupInfo`. `Group.SetSuppress` gains a mode argument. | done 2026-10-02: `Group.SuppressAll`, `.tlspr` field `suppress-all` (added within version 2); groups that use it are left out with it (`SuppressedBy`), keeping their parameter values; the active group moves to the nearest one still in. `SetSuppress(Unsuppressed/SuppressSolid/SuppressWhole)`, `Suppression`, `LeftOut`. `TestSuppressAllSweep`: nothing pruned, and the 176 fixed-point fixtures save the same once brought back. Scenario `ext_suppress_all`, pair `path_suppress_all` |
| 4.4 C3 broken references | `PruneOrphans` and the pruning in `generateAll` record what they remove: the entity's description and the group that owned it. The record goes into `SketchDiagnostic` and into a screen that lists each break with a jump to the affected group. `Doc.BrokenReferences()` in the API. Since step 3 a deletion can also break stored expressions; the screen lists `Sketch.ExprErrors` beside the pruned references. | done 2026-10-02, in `Sketch.BrokenReferences` rather than `SketchDiagnostic`, which each solve clears: the list stays until the next deletion, an undo or `ClearBrokenReferences`. Names come from the last regeneration, so a deleted request or group is still named; what belonged to a deleted group is not listed. Edit > Broken References, a home-screen link while the list isn't empty. Scenario `ext_broken_references`, pair `path_delete_group`, `examples/scripts/history` |

Done when: reordering, suppressing and deleting an upstream group each produce a named
diagnostic, never a silent deletion.

### Step 5. First kernel fixes

| Package | Content | State |
|---|---|---|
| 5.1 Freeze the C++ comparison | Every test that compares against a C++ recording keeps its recorded file as a Go golden, and its regeneration script calls Go. The `scripts/cpp-*-harness.cpp` files, `scripts/regen-cpp-*.sh` and `pkg/internal/slvswrite` are deleted. So are the test-only switches that keep the C++ recordings valid: `legacySlvsSave`, `cppComparison` (Go-only links and colours, the unit as the view's only) and `goOnlyMenuItems`; the scenarios they affect are regenerated in their own commit. The invariant tolerance in 1.3 is loosened to 1e-6. | done 2026-10-02: `golden.Check`, `CheckDump` and `CheckDir` rewrite a golden under `TELESAPR_REGEN_GOLDEN=1`, which `scripts/regen-golden.sh [areas]` and `scripts/regen-goui.sh [names]` set; the hex-float goldens were rewritten once in Go's spelling, values unchanged. The C++ scenarios became `.goui` goldens, regenerated without the switches in their own commit. With no `.slvs` writer left, what was compared with a C++ save is now a `.tlspr` golden Go wrote from the same state: an import golden per fixture under `testdata/tlspr/golden`, the new file, the modify tools, the thread goldens (`testdata/thread/*.tlspr`) and the DXF imports. Tests open `.slvs` fixtures through a test-only `loadSlvs`. `check.BaselineRelTol` is 1e-6. The glibc and Eigen references stay, since they pin libraries |
| 5.2 F1 raycast | The fallback at the end of `SShell.ClassifyEdge` in `pkg/srf/raycast.go` retries with another direction only when the nearest hit has `OnEdge` set, and `OnEdge` means within `lengthEps` of a trim. Widen it: `SSurface.AllPointsIntersecting` in `pkg/srf/raycast_surface.go` also marks a hit that lies within the chord tolerance of a piecewise-linear edge of a curved trim. Raise the retry limit from 5 and pick the ray with the largest clearance if all are marked. | done 2026-10-02, in `ClassifyEdge`'s raycast rather than for every caller of `AllPointsIntersecting`: each surface keeps its curved trims' xyz edges, hits within the chord tolerance of them are untrusted on either side of the BSP (the BSP also drops hits inside the true rim but outside the chords, which sent the ray on to a farther surface), up to 16 directions, the farthest from a trim edge wins. A hit clamped onto the patch boundary is off the ray and not one. No fixture output changed |
| 5.3 F1 proof | Run the thread sweep with the plane turning disabled. Target fixed before coding: no unsound solid in the full grid, at the default and at a five times coarser chord tolerance. Then delete the turning from `pkg/sketch/thread.go`. | done 2026-10-02: 3 of 400 unsound generated once (at 0.03% and 0.01%), none at 0.1% or 0.5%; it was 68 without the fix and 24 with seven placements. `AddThread` generates once; `ThreadResult.Tries` is gone. Two thread goldens moved with the fix, in their own commit |
| 5.4 F2 closed loops | After the boundary-seeded pass in `SSurface.intersectByMarching`, search for seeds inside: sample each surface on a grid, project each sample onto both surfaces with `PointOnSurfaces`, drop seeds that lie on a curve already found, and march each remaining seed until it returns to its start. Fixture: a sphere bulging through one face of a box, so the circle lies inside one patch and inside the face. | done 2026-10-02, with seeds from a recursive subdivision of both surfaces instead of a grid (a grid made the thread imports four times slower; this adds about a sixth). A loop goes in as four curves, since a trim can't start and finish at one point. The grid triangulation now leaves out grid points within a trim edge's sag, which otherwise put a hole's rim through the face around it. `TestSphereThroughFace` in `pkg/srf`. No golden changed |
| 5.5 Sweeps | A sweep of that fixture over radius, depth and position, and a sweep of cylinder through cylinder at angles. Each reports its failure rate through `pkg/internal/sweep`. Goldens regenerated in their own commit. | done 2026-10-02 at a 0.01 mm chord tolerance: sphere 10 of 135 (99 before), cylinders 19 of 54, the same points before and after the step, kept as a guard. Follow-ups: the cylinder failures, and the sphere cap that crosses the seams at the largest bulge |

Order: 5.1 first, in a commit that changes no output. Then 5.2 and 5.3, then 5.4 and 5.5.

Done when: both sweeps meet their rates and the invariant baseline shows no fixture got worse.

Follow-ups from step 5, each a guard in its sweep's allowed rate until looked into:

- **Cylinder through cylinder** [13.2] (`TestCylinderThroughCylinderSweep`, `pkg/srf/closedloop_test.go`):
  19 of 54 fail, the same points before and after the step; mostly unions and steep crossings,
  as failed Booleans or leaky meshes. The curves come from the boundary-seeded marching.
  On 2026-10-03, after step 11, 11 of 54 fail; the guard still allows 19 (12.6).
- **Sphere cap across the seams** [13.2] (`TestSphereThroughFaceSweep`): the intersection at the largest
  bulge (0.4 of the radius, radius 7) leaks at every position, nine of the ten failures; it failed
  before the step too. The tenth is a cap so small that it triangulates flat against the face.
- **Threads at fine chord tolerance** [closed] (`TestThreadSweep`, `pkg/persist/slvs/thread_test.go`): 3 of
  400 unsound, `nut_m6_down_lh` at 0.03% and `M3 tapped`, `M4 tapped` at 0.01%. Since `AddThread`
  no longer turns its planes, such a thread is reported unsound to the user. On 2026-10-03,
  after step 11, none of the 400 is unsound; the guard still allows 3 (12.6).

### Step 6. OpenGL and the section view

| Package | Content | State |
|---|---|---|
| 6.1 Renderer choice | `Window` gains `CreateRenderer()` so the platform picks the software or the GL renderer. Headless always picks software, so goldens do not move. | done 2026-10-02: `Window.CreateRenderer(useGL)`, and `PresentFrame` takes the canvas, so GL presents by swapping buffers. Headless returns software |
| 6.2 I1 GL renderer | New `pkg/render/gl` on `go-gl/gl/v2.1`, implementing the canvas interface of `pkg/render`. Polygons go through the Go triangulator. | done 2026-10-02: a port of `rendergl1.cpp` in `pkg/render/gl`, contexts from the new `pkg/render/gl/glx` (a window's, or an offscreen pixel buffer for tests). Without cgo or with the `nogl` tag both build and return `ErrUnavailable`. Two scenes drawn on both renderers differ in 3 and 190 of 43200 pixels under Xvfb |
| 6.3 I1 GLX window | A build-tagged file in `pkg/platform/tkgui` that makes a GLX context on the existing window from a second X connection, as the spike of 2026-09-29 did. `cmd/telesapr` locks the OS thread. A setting and a command-line flag choose the renderer, and software stays the fallback when GLX fails. | done 2026-10-02, in `pkg/platform/tkgui/gl.go` on `glx` rather than a build-tagged file there (the stubs live in `glx`). Setting: Configuration > "draw with OpenGL" (on by default), switching both windows live; flag `--renderer opengl\|software` for one run. Failure logs once and draws in software |
| 6.4 I1 tests | Smoke tests under `make test-gui`: a frame renders, and a read-back pixel inside a known face has the face colour. No pixel goldens for GL. | done 2026-10-02: `pkg/platform/tkgui/gl_test.go` reads a face's colour back from a GL window and switches renderers; `pkg/render/gl` tests run in `make test-gui`, and `make coverage` runs under Xvfb so both GL packages are held to 85% |
| 6.5 I3 section view | A clipping plane taken from a selected workplane or face. The cap is the section polygon that `ExportSectionTo` already computes, filled. View menu entry and a screen for offset and flip. `Doc.SetSection(plane, offset)` and `Doc.ClearSection()`, and `Doc.Render` honours it on both renderers. | done 2026-10-02: `Sketch.SetSection(plane, offset, flip)`, with the offset typed in the document's unit (expressions and parameters allowed) and a face's normal taken out of the solid. View state: not saved, not undone. The cap is the section of the display mesh (`SMesh.MakeSectionPolygonInto`) rather than of the shell, so it meets the clipped mesh exactly. View > Section View (Ctrl+Shift+X), screen with offset, flip and off. `Doc.SetSection(plane, offset, flip)`, `ClearSection`, `Section` (with the cap's area), `Group.Faces`. Scenario `ext_section`, pair `path_section`, example `examples/scripts/section` |

Done when: the same model looks the same on both renderers by eye, and the section scenario
passes on the software renderer. Both hold: the application under Xvfb draws the sectioned
example the same with `--renderer opengl` and `software`.

Follow-ups from step 6:

- **Cap colour** [16.12]: a fixed colour (`sectionCapColor` in `pkg/ui/section.go`); a style for it,
  or a hatch, would let the user change it.
- **Section in files** [left out]: the section is view state, as the camera is. If views are ever saved,
  it belongs with them.
- **Face normals** [14.5]: extrusion side faces carry normals that point into the solid. The section
  turns them out using the mesh; anything else that relies on a face's normal may need the same.
- **GL on other platforms** [left out]: only GLX exists; the takigo backends for Windows and macOS would
  need their own context code.

### Step 7. First new solid features

| Package | Content | State |
|---|---|---|
| 7.1 E1 mirror entities | New entity types beside `EntityPointNTrans` in `pkg/core/entity.go` for a reflected point, normal and face, with solver expressions for reflection in a plane whose parameters can move. `Group.CopyEntity` in `pkg/sketch/group_generate.go` learns the reflected kind. | done 2026-10-03, without new entity types: a mirror copies as a link group with a scale of -1 does (point inversion, then a half turn about the plane's normal, then a translation), so its copies are the existing rotate-and-translate points, normals and faces made at scale -1 (`copyEntityScaled`). The group's seven parameters, the translation 2(o.n)n and the half turn (0, n), are tied to the plane by equations, so the copies follow the plane when it moves |
| 7.2 E1 mirror group | New group type with the mirror plane in `Predef`. The solid comes from `SShell.MakeFromTransformationOf` with a negative scale, as linked groups already do. Options: keep the original, and combine as union. Menu entry, screen, `Doc.Mirror(plane, name)`. | done 2026-10-03: `GroupMirror` (`mirror` in the file), plane in `Predef.EntityB`, a workplane or a planar face, `SkipFirst` for the reflection alone; it reflects its source group as step and repeat copies it, so a mirrored pocket is cut too and the combine is the source's. New Group > Mirror (Shift+Y) with its screen; `Sketch.NewMirror`, `Doc.Mirror`, `KeepOriginal`, `SetKeepOriginal`, `Plane`. Scenario `ext_mirror`, pair `path_mirror`, example `examples/scripts/mirror`, goldens `testdata/solid/mirror-*.tlspr` |
| 7.3 E3 chamfer | New group type holding a list of edge entities and one or two distances. For each edge it builds a prism cutter from the two face normals and subtracts it. It accepts only edges that have entities. Selecting anything else gives a message that says so. Sweep over edge length, distance and dihedral angle. `Doc.Chamfer(edges, distance, name)`. | done 2026-10-03 with one distance: two would need a way to name which face of each edge takes which, left for step 9. `GroupChamfer` (`chamfer`, fields `edges` and `distance`), `Group.Edges`. Each edge's two faces are found from the triangles along it and its convexity by winding-number tests (`SMesh.WindingNumber`); the cutter is a wedge that reaches past both faces and twice the distance past the edge's ends (`SShell.MakePrism`), the wedges joined and taken away. An edge that isn't one, is concave, or is narrower than the distance gives `Group.Problem`, marked on the model and on the screen, and no cut. New Group > Chamfer Edges (Shift+Q), distance edited on the screen with expressions; `Sketch.NewChamfer`, `Doc.Chamfer`, `Distance`, `SetDistance`, `Edges`, `Problem`. `TestChamferSweep`: 84 of 84 sound with the exact volume (angle 20 to 165 degrees, distance, length). Scenario `ext_chamfer`, pair `path_chamfer`, example `examples/scripts/chamfer` |
| 7.4 E9 sweep | New group type with a profile group and a path made of lines and arcs, tangent continuous in the first version. Each line gives extrusion surfaces and each arc gives revolution surfaces, built with the code behind `MakeFromExtrusionOf` and `MakeFromRevolutionOf`. The segments are assembled into one shell with caps only at the two ends, so no Boolean between segments is needed. A path with a corner gives a diagnostic. `Doc.Sweep(profile, path, name)`. | done 2026-10-03: `GroupSweep` (`sweep`, `edges` the path), `SShell.MakeFromSweepOf` in `pkg/srf/sweep.go`: per curve and piece an extrusion or a surface of revolution per quarter turn, the curves at the joints shared, caps at the ends, no Boolean. The chain is ordered from the selection in any order and starts at the end nearest the profile's plane; branches, gaps, corners, a closed path, a path in the profile's plane and a bend tighter than the profile are each a diagnostic. Copies of the profile at both ends and the two cap faces are entities. New Group > Sweep Along Path (Shift+U); `Sketch.NewSweep`, `Doc.Sweep`. `TestSweepGrid`: 7 of 72 flagged, all a thin tube on a tight bend whose turn triangulates exactly as a Revolve's and which a Revolve shows at some chord tolerances too. Scenario `ext_sweep` (from `testdata/ui/sweep_path.tlspr`), pair `path_sweep`, example `examples/scripts/sweep` |

Each package ends with goldens, an interface scenario, an example script and a path
equivalence test. Order: 7.1, 7.2, then 7.3 and 7.4 in either order.

Done: every package has its goldens (`scripts/regen-golden.sh solid`), scenario, example and
path pair, and the three keep one file across a save and load, an undo and redo, and one or
eight goroutines. Their group values and edges went into format version 2 without a bump.

Follow-ups from step 7:

- **Mirror of the whole solid** [15.5]: a mirror reflects its source group, as step and repeat does;
  mirroring a body made of several groups means mirroring each, or a group of its own.
- **A solid mirrored onto itself** [13.4]: a part symmetric about the plane, mirrored with the
  original kept, puts coincident faces through the Boolean, which fails (reported as a failed
  Boolean). The case is common enough to want handling.
- **Two chamfer distances** [left by 15.1; not placed], once step 9 names faces: the edit's
  corners, where three chamfered edges meet, would need the setback on each face apart.
- **Thin tori** [closed]: the solid check finds triangles of a thin tube's bend crossing their
  neighbours by about 2e-4 mm, in a sweep and in a Revolve alike (`sweepGridMaxFailRate`).
  On 2026-10-03 `TestSweepGrid` has 0 of 72; the guard still allows 7 (12.6).
- **Benchmarks** [left out]: `BenchmarkRegen150` reads about 6% slower on this branch, all of it inside
  the solver's QR, which the step doesn't touch; adding one file the benchmark never runs moves
  it by 3%, so it is code layout, not work.
- **Untrimmed surfaces after a difference** [13.3]: a Boolean keeps the cutter's surfaces it removed
  whole, with no trims, so a chamfered block's shell has 11 surfaces for 7 faces.

### Step 8. STEP import as a reference mesh

| Package | Content | State |
|---|---|---|
| 8.1 Part 21 reader | New package `pkg/persist/step`. A tokenizer and an entity table keyed by instance number, with typed access to parameters. `FuzzStep` from the first day. | done 2026-10-03: `step.Parse` (complex instances, `\X2\` and the other string escapes, typed values, comments, ANCHOR and SIGNATURE sections skipped), `FuzzStep` over parsing, mapping and tessellating, in `make fuzz`; three minutes, seven million inputs, no finding |
| 8.2 Boundary representation | Map the AP203 and AP214 entities for solids, shells, faces, loops, edges and vertices, and the surface kinds plane, cylinder, cone, sphere, torus and B-spline, into the package's own structures. Unknown surface kinds are counted and reported, and their faces are skipped. | done 2026-10-03: `step.ReadModel`; also AP242, faceted and shell-based models, voids, surfaces of extrusion and revolution, offset and rectangular-trimmed surfaces, rational and uniform B-splines and Bezier patches, conics, polylines, trimmed and surface curves. Units per representation context (SI prefixes and conversion-based, angles too), assembly placements through `NEXT_ASSEMBLY_USAGE_OCCURRENCE` in both orders of `rep_1` and `rep_2` (CAx-IF and SolidWorks), mapped items and mirroring operators, surface colours. What can't be read is counted by entity and named, with the first instance (`Report.Problem`, `Details`) |
| 8.3 Tessellation | Evaluate each surface in its own parameter space, trim by the face loops, and triangulate with `SPolygon.TriangulateInto`. This does not go through `SSurface`, so degree is not limited. | done 2026-10-03, with a new constrained Delaunay triangulator (`pkg/internal/cdt`, exact predicates) instead of `TriangulateInto`, which is bound to `SSurface` and adds no interior points: each face gets a staggered grid of points as dense as its curvature needs. Edges are sampled once and shared, so the mesh is closed. Seams, poles (a polar chart about a pole, the development of a cone), loops round a closed surface (two joined by a cut, one closed at its pole or, as OpenCASCADE does, straight across) and faces with no bounds. Faces side by side on all cores, the result the same on one |
| 8.4 Link entry | `pkg/persist/link` dispatches `.step` and `.stp`. The result fills `Group.ImpMesh` and gives a point entity per vertex in `ImpEntity`, so constraints can attach. Units and assembly placement are applied. `Doc.Link` needs no change. | done 2026-10-03: also a point at each circle's centre; the tessellation cached by file and chord tolerance (the sketch's `ChordTolPct` of the file's size), since undo reloads links. Loaders may report (`link.RegisterReporting`): `Group.LinkInfo` (protocol, unit, shapes, faces, triangles, tolerance) and `Group.LinkProblem`, on the group's screen and as a warning in the group list; a file that holds nothing readable links empty with the reason, a missing one asks to be found. `Group.LinkInfo`, `LinkProblem`, `examples/scripts/step`, scenario `ext_link_step`, pair `path_link_step` |
| 8.5 Corpus | The exporter is the first oracle. `testdata/export` holds 33 `.step` files, most of them from sketches without a solid, so the corpus adds exports of the solid fixtures: export, import, and require the volume to match within the chord tolerance. Then vendor files, starting with the fastener and the connector of the spike. | done 2026-10-03: 14 analytic shapes written by FreeCAD (`testdata/step/shapes`, `scripts/step-fixtures.py`) against worked-out volumes at three tolerances; 11 files kept in `testdata/step/corpus` (NIST MBE PMI, OpenCASCADE's screw, FreeCAD, Gmsh, BRL-CAD) and 16 more listed in `external.txt` and checked under `TELESAPR_STEP_CORPUS` (SolidWorks assembly and parts, knurled bolt and nut, more NIST), all against FreeCAD's volumes within area times tolerance. Every solid fixture exported by our writer and read back came out closed with its volume, but the knife-edge one (below). No connector file was at hand |

Done: every corpus file imports, closed and within the tolerance of the reference volume, or
fails with a message that names the unsupported entities (the CSG and swept-solid files), and
none panics.

Follow-ups from step 8:

- **Knife edges** [13.5]: a solid whose faces meet along an edge four times (the
  `boolean_knife_edge` fixture, exported and read back) gives six naked edges.
- **Open thread ends** [left out]: the knurled bolt's thread ends are faces bounded by a single loop
  round a cylinder; they are closed straight across, as OpenCASCADE does, which leaves
  holes, as the file does (`external.txt` marks it open).
- **Faces through both poles** [13.5], such as a whole sphere or a lune from pole to pole, are
  triangulated in their parameters; only faces at one pole get the polar chart. They pass
  the tests, but a split at the equator would be safer.
- **Tessellated STEP** [21.6] (AP242 `TRIANGULATED_FACE` and the like) and CSG and swept solids
  [left out] are reported, not read.
- **Self-intersections in tiny parts** [13.5]: `linkrods.step` (5 mm across) passes the volume check,
  but Check Solid finds 24 crossing edges at the default tolerance.
- **Placement of a new link** [16.13]: a linked part is placed at the middle of the view, as every
  link is; a STEP file is usually wanted where it was drawn.
- **Performance** [13.5]: the triangulator's adjacency is a map; a 10 MB file reads in about a
  second, of which the triangulator is most.

### Step 9. Topology and edge naming

A two-day spike comes first. It derives vertices, edges and loops from the shell of every
fixture and checks Euler's formula on each. What fails there decides the design.

| Phase | Content | State |
|---|---|---|
| 9.1 Derived topology | New package `pkg/topo`: solid, face, loop, co-edge, edge, vertex, built from `SShell` by walking `SCurve.SurfA`, `SurfB` and each surface's `Trim` list. A validator: every edge has two co-edges of opposite sense, loops close, Euler's formula holds. The validator joins `check.Solid`. | done 2026-10-03. The spike found what the design needed: the two sides of an edge can trim by two coincident curves after a Boolean, so edges are matched by their vertices and points, not by curve; a Boolean can leave a T-junction, a trim running past a vertex where two faces meet on its other side, so edges are split at vertices inside them; outer loops run clockwise in SolveSpace's parameters. Every fixture solid validates and has its mesh's Euler characteristic, but the knife edge's, which is non-manifold on purpose. `check.Report` gains `TopologyProblems` and `Euler` (`topo=`, `euler=` in `testdata/invariants.txt`); problems make a solid unsound |
| 9.2 Topology as output of the kernel | `MakeFromBoolean` and the shell constructors produce topology directly instead of having it derived afterwards. `SShell` keeps its lists as storage. The trim handling in `pkg/srf/boolean_trim.go` becomes loop editing. | not taken, as the risk note allows: derivation works on every manifold fixture and costs about 7 ms for a threaded bolt of 270 faces (`BenchmarkBuildThread`); it is built lazily, per regeneration (`Group.Topology`) |
| 9.3 Local edit helpers | Split an edge, split a face along a curve, remove a face and heal, replace a face's surface. Each keeps the validator green. Steps 10 and 11 are built from these. | done 2026-10-03: `SplitEdge`, `SplitFace`, `RemoveFaces`, `AddFace` (taking the open edges it runs along, the surface turned to suit), `ReplaceSurface`; a box edge chamfered with them alone validates with the exact volume (`TestLocalChamfer`) |
| 9.4 F4 edge names | An edge name is the two `FaceID` values it separates and an ordinal. The ordinal comes from a geometric order along the face pair, not from creation order. A name that no longer resolves gives a diagnostic naming both faces. Written to `.tlspr` as a reference kind. | done 2026-10-03: `topo.Name` ("edge A B N"), ordered by midpoints along the direction the edges between the pair spread most. Surfaces no entity names get `SSurface.Origin` from the group that made them (`srf.OriginFor`, bit 30), carried through Booleans, so all 4445 manifold edges of the fixtures have names, each resolving to itself. `solid-edge` fields in `.tlspr` (version 2) |
| 9.5 Picking and scripting | Hover and selection of solid edges in the graphics window through the outline list, and a selection kind for them. `Group.Faces()`, `Edges()`, `Vertices()`, adjacency queries, and filters such as edges between two faces or edges parallel to a direction. | done 2026-10-03: `Selection.Edge`, hovered within six pixels of the projected edge and not hidden behind the solid, below entities and constraints and above faces; described in the text window. `Group.Solid()` with `Faces`, `Edges`, `Vertices`, `Edge(name)`, `EdgeAt`, `EdgesBetween`, `EdgesAlong`, `Problems`, `Euler`, and `SolidFace`, `SolidEdge`, `SolidVertex` with adjacency (`Group.Faces` and `Group.Edges` already meant other things). `examples/scripts/edges` |
| 9.6 Chamfer on any edge | E3 accepts named edges. Naming sweep: change an upstream dimension through its range and require every chamfer to stay on its edge or report. | done 2026-10-03: `Group.SolidEdges`, Chamfer Edges takes picked edges, `Doc.ChamferSolid`. `TestNamingSweep` moves a notch across the block in 23 steps: the chamfer of the second piece of the front's top edge stays on it, with the exact volume, while the notch splits the front, and reports otherwise; none moves silently. Scenario `ext_chamfer_solid_edge`, pair `path_chamfer_solid_edge`, goldens `chamfer-solid-edge`, `chamfer-of-chamfer` |

Done: the validator passes on every fixture and golden but the knife edge, and the naming sweep
reports no silent move of a chamfer to another edge.

Follow-ups from step 9:

- **9.2** [left out], topology made by the kernel, if derived topology ever costs too much or a feature
  needs to keep identities through a Boolean.
- **Origins shift** [14.2]: a surface's `Origin` is its place among its group's surfaces, so an edit
  that changes how many surfaces a group makes (a thread's length) renames the later ones.
- **A face picked by the mouse** [14.1] is still the entity of its `FaceID`; faces that only have an
  `Origin` can't be selected as faces, only their edges.
- **Hidden edges** [14.4] are tested against the active group's display mesh with a ray per hover;
  a large mesh makes hovering slower.
- **Non-manifold edges** [14.3] (the knife edge) are reported as topology problems, not modelled.
- **Ordinals** [14.2] follow geometric order; an edit that reorders the edges between two faces along
  their spread can move a name, as the sweep's design keeps from happening but does not forbid.
- **Loops through a vertex twice** [14.3] choose the sharpest left turn, which no fixture exercises.

### Step 10. Fillets, shell and loft

Spikes first, as listed under "How to check an estimate before committing to it".

| Phase | Content | State |
|---|---|---|
| 10.1 E4 fillet, straight edge between planes | Local edit: split both faces along the offset lines, remove the strip, insert a cylinder patch, which the rational patch represents exactly, and rebuild the end faces. Chains of tangent edges are taken as one. | done 2026-10-03, but for chains: `topo.FilletEdge` cuts both faces along the cylinder's tangent lines and the end faces along its arcs (leaning end faces get the cross section projected along the edge), takes out the strip and puts in an exact cylinder patch; `FilletEdges` refuses edges that meet, as a corner is 10.2. The fillet group (Sketch -> Fillet Edges, Shift+F) edits the solid so far with no Boolean; entity edges or named ones, the radius an expression. `TestFilletSweep` in `pkg/topo`, 126 prisms over angle, radius, length and lean, and in `pkg/sketch`, 63 points, rate 0 for both, set before. Scenario `ext_fillet`, pair `path_fillet`, `examples/scripts/fillet`, goldens `fillet-edges`, `fillet-solid-edge` |
| 10.2 E4 corners | Three filleted edges meeting at a convex vertex of a box get a sphere patch. Other vertex cases give a diagnostic in this version. | done 2026-10-03, at any convex corner of three flat faces: `topo.FilletNetwork` rounds all the edges as one edit, building every face along them again from its own loops with the corners moved. Three edges get a sphere, a patch of revolution about an axis square to the corner so that no pole lies on it; two get a mitre, their cylinders meeting along an ellipse in the plane that halves them, when the faces lie alike about them (else a diagnostic). Volumes by a formula that matches OpenCASCADE to six places (`TestFilletCorners`), and `TestFilletCornerSweep`, 90 prisms of polygons with top edges, a corner or all edges against OpenCASCADE's volumes (`testdata/topo/fillet-corners.txt`, `scripts/fillet-fixtures.py`), rate 0, set before. Fillet Edges takes edges that meet; scenario `ext_fillet` (a mitre), pair `path_fillet_mitre`, `examples/scripts/roundbox`, goldens `fillet-mitre`, `fillet-corner-sphere`. Found on the way: the solid check's crossing test had a tolerance relative to the triangles' size, and scripts' chord tolerance was overwritten by the application's (`Doc.SetChordTolerance` now) |
| 10.3 E5 circular edges | Plane meeting cylinder gives a torus patch, and a chamfer there gives a cone. Both are exact in the patch type. Plane meeting cone follows. | done 2026-10-03 for plane meeting cylinder, a boss's top or a hole's rim; plane meeting cone not yet. The same edit with the edge's cross section turned about the axis, a patch per arc; picking one arc takes its circle. Fillet Edges and Chamfer Edges take circles and arcs, entities or solid edges; the chamfer makes the solid itself then, less the wedges of any straight edges. Against Pappus, which matches OpenCASCADE to six places; `TestFilletCircleSweep`, 72 points over boss or hole, fillet or chamfer, radius, height and size, rate 0, set before. Scenario `ext_fillet_circle`, pair `path_fillet_circle`, `examples/scripts/knob`, goldens `fillet-circle`, `chamfer-circle-and-edge` |
| 10.4 E11 restricted shell | Offset every plane and cylinder inward, intersect neighbours again through the topology, drop the picked faces, and join inner and outer shells at the openings. A thickness that makes a face vanish gives a diagnostic. | done 2026-10-03 for planes; cylinders not yet. `topo.ShellSolid` builds the inner solid on the outer's topology, each face moved in but those taken away, which stay, each vertex where its faces' moved planes meet, and takes it away by a Boolean; an edge turned round says the walls meet; a vertex of more than three faces must stay one point. The shell group (Sketch -> Shell Solid, Shift+G) takes picked faces (`solid-face` in the file), the thickness an expression; `Doc.Shell`, `Doc.ShellSolid`, `Group.Thickness`, `SetThickness`, and `Solid.FaceAt`. `TestShellSweep` (180 prisms of polygons and an L over opening, thickness and height, against the moved cross-section's area) and `TestShellGroupSweep` (12 boxes), rate 0 for both, set before. Scenario `ext_shell`, pair `path_shell`, `examples/scripts/shell`, goldens `shell-open-box`, `shell-holed-block` |
| 10.5 E7 ruled loft | Two profile groups with equal segment counts and a vertex matching the user can rotate. One patch per segment pair, degree 1 across. | done 2026-10-03: `topo.MakeRuledLoft` builds the shell face by face; flat sides are planes trimmed to their trapezoid, twisted ones patches of degree 2 along the curves. The loft group (Sketch -> Loft To Selected Profile, Shift+O) joins the active sketch's one closed loop to that of the sketch an entity is picked from, a circle counting four arcs; the screen turns the second by a curve either way (`turn` in the file); it combines as an extrusion does. `Doc.Loft`, `Group.Turn`, `SetTurn`. Volumes by formula, h (A0/3 + A1/3 + K/6) with K the loops' mixed area; `TestLoftSweep` (81 points over polygon, circle, scale, height and turn, and 9 untwisted triangles, rate 0, set before); `TestLoftOnBlock` joins lofts to a block on, in, under and above it. Scenario `ext_loft`, pair `path_loft`, `examples/scripts/loft`, goldens `loft-on-block`, `loft-circle-to-square`. Found on the way: the grid mesher dropped its grid near any trim corner, shells built by `AddFace` met no Boolean (planes on a 1 mm square, curves marked as a Boolean's), and new requests in 3d groups saved differently after a reload |
| 10.6 Concave fillets (follow-up) | Edges that go in are filled: straight edges, a boss's foot and a blind hole's floor (tori), pocket corners (spheres) and mitres. The rate for the sweep, 0, set before. | done 2026-10-03: the same edit, the centres outside the solid; a concave edge ending on the outside of a corner (a boss's foot at its side) runs on to that face's plane and caps there; convex and concave fillets meeting at a corner are refused. `TestFilletConcaveSweep`, 90 points over wedges, pockets, foot edges, round bosses and blind holes against volumes by formula, rate 0. Scenario `ext_fillet_concave`, pair `path_fillet_concave`, `examples/scripts/tray`, goldens `fillet-pocket`, `fillet-boss-foot`, `BenchmarkFilletPocket` |
| 10.7 Circular and straight edges meeting (follow-up) | A circular edge's fillet meets a straight one's where the arc meets the line smoothly, as round a rounded rectangle; picking one edge takes the chain. The rate for the sweep, 0, set before. | done 2026-10-03: the seam of two pieces of a circle generalised to any two rounded edges that share a face and have one cross section at the vertex; the third edge there is cut back to its point. A circular edge meeting another at an angle is refused. `TestFilletOutlineSweep`, 36 points over rectangles with one, two or four rounded corners, top or pocket floor, against the cross section along its centroid's path, rate 0. Scenario `ext_fillet_outline`, pair `path_fillet_outline`, `examples/scripts/soap`, golden `fillet-outline`, `BenchmarkFilletOutline` |
| 10.8 Uneven mitres (follow-up) | Two rounded edges whose faces don't lie alike about them meet as a mitre too. The rate for the sweep, 0, set before. | done 2026-10-03: the cylinders still meet along an ellipse in the plane that halves the edges; the strip that turns further goes on past it, cut by the other's side face along a conic, as OpenCASCADE builds it. `TestFilletUnevenMitreSweep`, 96 leaning prisms against OpenCASCADE (`testdata/topo/fillet-mitres.txt`, `scripts/fillet-mitre-fixtures.py`), rate 0; OpenCASCADE is exact for 62, its spline between the cylinders straying by up to 0.15 mm or its shape invalid in the rest, which are checked for soundness only (where it is exact, these solids exported to STEP measure as its to 1e-4 mm³). Scenario `ext_fillet_uneven` on `testdata/topo/ramp.tlspr`, pair `path_fillet_uneven`, `examples/scripts/ramp`, golden `fillet-uneven`, `BenchmarkFilletUneven` |
| 10.9 Plane meeting cone (follow-up, 10.3's last part) | Where a flat face meets a cone about the circle's axis: a torus, or a chamfer's cone. The rate for the sweep, 0, set before. | done 2026-10-03: the other face's generator at the edge, found from the face and checked against its whole surface, gives the section's angle; the tangent circles turn to new radii and move along the axis; a chamfer cuts at its distance along both faces (on a cylinder as before). A lathe's flat faces (quarters of a disc, of degree 2) count as flat, and two rounded edges meet smoothly across them at a vertex of four edges. A Boolean's plane-cone curve, not exact, is taken as the arc through its points. `TestFilletConeSweep`, 96 points over a lathed frustum's top or foot, a countersink's rim and a frustum's foot on a plate (filled), fillet or chamfer, slope and radius, against Pappus, rate 0. Scenario `ext_fillet_cone` on `testdata/topo/frustum.tlspr`, pair `path_fillet_cone`, `examples/scripts/spinner`, golden `fillet-cone`, `BenchmarkFilletCone` |
| 10.10 A radius per edge (follow-up) | A fillet's edge may have a radius of its own. The rate for the sweep, 0, set before. | done 2026-10-03: `Group.EdgeRadii`, `edge-radius <n> = <expression>` in the file, a number or an expression in the document's unit, kept by parameters, units, undo and pruning; the group's screen shows it under its edge with [change] and [the fillet's], or [own radius]; `Group.EdgeRadius`, `SetEdgeRadius`, `SetEdgeRadiusExpr`. Edges an edge takes on through smooth joins inherit its radius; rounded edges meeting at a corner need one radius (a diagnostic otherwise), an edge ending where no other rounded edge does may have any. `TestFilletRadiiSweep`, 18 points over a box's top and foot edges and a cylinder's two circles at two radii, rate 0. Scenario `ext_fillet_radii`, pair `path_fillet_radii`, `examples/scripts/plinth`, golden `fillet-radii`, `BenchmarkFilletRadii` |

Each phase carries a sweep whose acceptable failure rate is written into this table before the
phase starts.

Follow-ups from step 10:

- **Corners left** [22.4, and 14.3 for the T-junction]: corners of more than three faces (but the four where a lathe's quarters
  meet along a smooth join), a circular edge meeting another rounded one at an angle, edges
  that go on straight through a vertex (a T-junction a Boolean left), convex and concave
  fillets meeting at a corner, and rounded edges of different radii meeting at a corner
  (variable-radius blends, setbacks) are refused.
- **Chamfer group** [done in 15.1]: it cuts concave straight edges by its Boolean, which refuses them, and
  takes no chains through smooth joins, its straight edges being cut by wedges.
- **A chain stopped by its radius** [15.7]: a fillet's radius at least a rounded outline's corner
  radius stops the chain there, and the message speaks of the curved face it ends on rather
  than of the radius.
- **Fillets between two curved faces** [22.2] (a cylinder meeting a cone, a chamfer's cone meeting the
  cylinder it was cut from).
- **Small spheres at fine tolerances** [13.1]: with the chord tolerance below about a thousandth of a
  corner's radius, the sphere's grid stops refining while its trims go on, and the strips
  between are ear-clipped into slivers that cross; tests set the tolerance from the radius.
- **Strong twists** [13.1]: a ruled face turned by a third of a turn or more meshes with false
  self-intersections and about 1% error in volume; the mesher's grid stops refining a twist at
  16 rows (`MakeTriangulationGridInto`'s depth), and the rows along the trim are ear-clipped.
  A deeper cap fixes the triangles in the sweep but makes it four times slower and changes the
  meshes of cones.
- **Chord loss after a Boolean** [13.3]: a filleted block cut afterwards loses about 1 mm³ more than
  the fillet's own mesh, the cylinder's trim being cut into coarser chords.
- **Loft matching** [20.1]: profiles must have as many curves; splitting curves to match counts, and
  lofts through more than two profiles, are not done.
- **Faces with only an Origin** [14.1] (a loft's, a fillet's) can't be picked as faces, as for step 9;
  scripts reach them through `Solid.FaceAt` and `Doc.ShellSolid`.
- **Shell of curved faces** [15.2 for prisms; the rest left out with E12]: prisms whose sides are
  flat or round, open at their ends, are hollowed since 15.2; other curved faces (filleted,
  lofted, revolved solids, a cylinder's side taken away) are refused.
- **Shell vertices of four faces** [done in 15.3] that don't meet in one point once moved (an off-centre
  pyramid's apex) are refused; the shell would need a new edge there.
- **A Boolean's leftover surface** [13.3]: a shell's inner face flush with a face taken away stays in
  the shell untrimmed, which the solid check counts as a face (Check Solid says 12 faces for an
  open box of 11), as step 7 noted for differences.

### Step 11. General surface type

Spike first (2026-10-03), on the two questions that decided the design. Raising the patch's
fixed `[4][4]` net to `[8][8]` (degree 7) made the regeneration benchmarks 3.7% slower in the
geometric mean, all of it in copying and clearing whole surfaces; `[6][6]` (degree 5) cost
under 1% once two whole-surface copies were taken out of the Boolean. Slices, which the table
below proposed, were not taken: a copied surface would share its control net with the
original, and `Reverse`, `ScaleSelfBy` and `WeightControlPoints` change it in place. The
second question, whether the Boolean survives faces of several patches, was answered by the
B-spline lids of 11.2: it did not, in three places, all fixed there.

| Phase | Content | State |
|---|---|---|
| 11.1 Multi-patch faces | A face in `pkg/topo` may own several patches joined by seams marked smooth. Seams are hidden from display, picking and naming. This alone makes approximated blends and offsets representable. | done 2026-10-03: surfaces carry a sheet key (`srf.SSurface.Sheet`), the sketch entity of the curve they were swept from, kept by extrusions, revolutions, sweeps, copies and Booleans and saved as `sheet` in `.tlspr` shells; an edge between two patches of one key, or one identity, whose normals agree along it is a seam (`topo.Edge.Seam`), and `topo.Topology.Sheets` are the faces they make. A chain is an edge as a person sees it, the pieces on either side of vertices that only seams touch. Seams are not picked or named (names still count them, so names saved before keep resolving); hovering any piece picks the chain by its lead piece and draws all of it. The script API works in faces and edges as a person sees them, with `SolidFace.Patches` and `SolidEdge.Pieces`; Check Solid counts faces, and patches when they differ. Over every fixture solid 2272 patches make 935 faces. Pair `path_fillet_rim` on `testdata/topo/boss.tlspr`, `examples/scripts/edges`, `TestSolidSheets` |
| 11.2 B-spline surface with knots | A surface type with arbitrary degree and knot vectors for storage, evaluation and exchange. It is cut into Bezier patches at its knots for the existing intersection and triangulation code. | done 2026-10-03: `srf.BSplineSurface` and `srf.BSplineCurve`, rational or not, clamped or not, any degree; `PointAt`, knot insertion, `Patches` and `Beziers` (degree up to `srf.MaxDegree`); `topo.AddBSplineFace` puts one into a shell as one face of its patches, `topo.AddPlanarFace` a flat face. The Boolean needed three fixes across seams: a split at a segment's own end, where the three-surface refinement can't converge on two tangent patches; a split off the curve, where a raycast stops at a patch's edge; and the ear clipper culling triangles thin in uv that are not in xyz, as along a seam. `TestBSplineLid` (degrees 2x3 to 5x4 against a closed form), `TestBSplineSweep`, 96 lids holed or cut by blocks against Gauss quadrature, rate 0, set before. No user surface yet: D3 and E8 below are the first features to make these faces |
| 11.3 Degree above 3 | Replace the fixed `[4][4]` arrays in `SSurface` with slices and lift the degree limit in evaluation, subdivision and the Bernstein tables of `pkg/srf/bernstein.go`. Benchmarks guard the regeneration path, since this touches the hottest loops. | done 2026-10-03 with fixed arrays of `srf.MaxDegree` 5 (see the spike above), for patches and curves; de Casteljau splitting for any degree, degree elevation, Bernstein polynomials of every degree by one Horner form whose results for degrees up to 3 are bit for bit those of before; `.tlspr` takes degrees up to 5, DXF writes knots for any degree. `TestElevatedSolids`: boxes, tubes and their Boolean rebuilt from patches of degree 4 and 5, against their volumes. Found on the way: the ear clipper culled real slivers by comparing an area with a squared length. Against `master` the regeneration benchmarks are 3.6% faster in the geometric mean, none slower, since `PointAt` and `TangentsAt` now work out each basis function once |
| 11.4 STEP export | `pkg/ui/exportstep.go` writes one B-spline surface per face instead of one per patch. | done 2026-10-03: `topo.SheetSpline` writes a face's patches as one B-spline where they make a grid, in any turning, closing into a ring along u (a lathe's along v, turned), knots of full multiplicity spaced by the patches' lengths; all 101 faces of several patches over the fixtures are grids. Export joins the face's trims but its seams into loops, and on a ring joins the two loops that go round it along a seam edge, as OpenCASCADE writes a cylinder. Faces whose patches make no grid are written a patch at a time, as before. `TestExportStepSheets` reads ten exports back with the step 8 reader (one face per face, sound, the volume of the same solid written a patch at a time); FreeCAD finds every file valid and the surfaces within 1e-14 mm of the patches |
| 11.5 Open the tier 4 items | E6, E8, E10 and D3 each get their own estimate and plan at this point. | done 2026-10-03, below |

Follow-ups from step 11:

- **Knot removal on export** [21.5]: knots between patches are written at full multiplicity, so the
  surface is C0 at each in its parameters though smooth in space. OpenCASCADE's volume of such
  a face is off by up to 0.3% (its meshes agree with ours to 2.5e-4); removing the knots where
  the patches join with matching derivatives would make the surfaces C1 or C2.
- **Degree above 5** [21.2]: `srf.MaxDegree` is 5, and the Horner form in `bernstein.go` is written
  out for it (a compile-time check fails for any other). STEP files with degree 6 to 9 surfaces
  need degree reduction or approximation in D3.
- **Faces whose patches make no grid** [21.5]: a whole torus (closed both ways), a ring of two patches,
  and a face some of whose patches a Boolean removed entirely are written a patch at a time.
- **A fillet's patches are faces of their own** [14.1]: tori and cylinders built by `FilletNetwork` get
  identities but no sheet key, so a circle's fillet is four faces, each with an edge between.
- **Edges between faces with no entity** [14.1] (a fillet's, a Boolean tool's) are hidden in the
  display mesh, which tells faces by entity (`STriMeta.Face` is 0 for both); the topology knows
  them.
- **A face's name** [14.1] is its first patch's identity; a script can't name a face of several
  patches by another patch's.

### 11.5: the tier 4 items opened

What step 11 leaves in place, which none of these had before: faces of any number of patches,
B-spline surfaces and curves with knots and knot insertion, patches up to degree 5, the
Boolean and the mesher sound across seams (a sweep of 96 lids at rate 0), and STEP export of a
face as one surface. What none of them has yet: surface fitting (interpolation or least squares
of a grid of points by a B-spline), trimming a face of several patches by a loop that crosses
its knot lines, offsets of curved surfaces, and per-edge tolerances. The estimates below are for
one developer who knows the code base, with the definition of done included, as elsewhere in
this document.

| Item | Size | Effort | New code | Risk | Depends on |
|---|---|---|---|---|---|
| D3. STEP import as a NURBS shell usable in Booleans | L | 8 to 10 wk | 4,000 | High | the trimmed-sheet builder (D3.1) |
| E8. Loft through several sections, smooth | L | 4 to 5 wk | 2,000 | Medium | surface interpolation (E8.2) |
| E10. Sweep along a general 3D curve | L | 4 to 6 wk | 2,000 | High | surface fitting from E8 |
| E6. Fillet on general edges, with vertex blends | XL | 5 to 7 months | 8,000+ | Very high | offsets and fitting from E10, D3's trimming |

Recommended order: E8, then E10, then D3, then E6. E8 is the cheapest and is exact in the new
surface type; it builds surface interpolation, which E10 and E6 reuse. D3 builds the trimmed
sheet, which E6 needs where a blend runs into a face of several patches.

**E8 smooth loft.** Skinning: the sections' curves are made compatible (same degree by
elevation, same knot vector by insertion of each other's knots), then interpolated across the
sections by cubic B-splines in the other direction. The surface is exact B-spline output, so no
tolerance enters; it goes in by `topo.AddBSplineFace` with caps by `topo.AddPlanarFace`.

| Phase | Content | Effort |
|---|---|---|
| E8.1 Compatible sections | Each section's closed loop as one B-spline curve (`srf.BSplineCurve` from its Beziers, a knot per joint), degrees raised to one, knot vectors merged; the vertex matching and turn of the ruled loft (step 10.5) choose where each starts | 1 wk |
| E8.2 Interpolation across | A cubic B-spline through the sections' control points by chord-length parameters, solved by the banded solver `pkg/sketch` uses for splines; rational sections interpolated in homogeneous coordinates | 1.5 wk |
| E8.3 Group, file, interface, scripts | The loft group takes more than two sections, in order; `Doc.Loft` takes a list; `ext_loft_smooth`, a path pair, an example, goldens | 1.5 wk |
| E8.4 Sweep | Lofts through 3 to 6 sections of polygons, circles and splines against volumes by Gauss quadrature of the surface (as `TestBSplineSweep`), joined to blocks; rate 0 set before | 1 wk |

**E10 sweep along a 3D curve.** The swept surface of a profile along a space curve is not a
B-spline, so it is fitted: frames by rotation-minimising transport along the path (the step 7
sweep uses fixed frames along lines and arcs), the profile placed at enough frames that the
fitted surface stays within a tenth of the chord tolerance of the true sweep, then E8's
interpolation through those placed profiles. The error is measured, and sections added where it
is too large.

| Phase | Content | Effort |
|---|---|---|
| E10.1 Paths and frames | A path of any sketch curves in 3D (cubics, helices, a step 9 edge chain); rotation-minimising frames with an optional twist and a fixed binormal mode | 1.5 wk |
| E10.2 Fitting with an error bound | Sections placed and interpolated; the distance from the fitted surface to the true sweep sampled between sections, sections added until it is within tolerance; the profile's caps as planar faces | 2 wk |
| E10.3 Group, file, interface, scripts | The sweep group takes a 3D path; the existing sweep's lines and arcs keep their exact surfaces | 1 wk |
| E10.4 Sweep | Profiles along helices and cubics against volumes by Pappus where the path is planar and by quadrature of the fitted surface where not, joined to blocks, rate 0 set before | 1 wk |

**D3 STEP import as a solid.** The step 8 reader parses every surface and curve a solid needs
and meshes them; D3 builds a shell from them instead. Analytic surfaces are exact as rational
patches (planes, cylinders, cones, spheres, tori), B-splines by `Patches`. The hard part is a
face's trim: a STEP face is a B-spline trimmed by edge loops that cross its knot lines, and the
kernel trims patches, so each loop has to be cut at the knot lines and given to the patches it
crosses, which nothing does yet. Then vendor files: edges that miss their surfaces by up to a
hundredth of a millimetre, surfaces of degree 7 and above, sliver faces, all of which the
kernel's 1e-6 mm epsilon rejects.

| Phase | Content | Effort |
|---|---|---|
| D3.1 Trimmed sheets | `topo.AddTrimmedSheet`: a B-spline face bounded by any loops of curves on it, the loops cut at the knot lines and handed to the patches; patches no loop reaches dropped or kept whole by a point test. Tested on the lids of 11.2 with holes and outlines put in by the builder instead of a Boolean, against the Boolean's result | 3 wk |
| D3.2 From STEP to a shell | Each STEP face to a sheet (analytic or B-spline), each edge to a curve shared by its two faces; healing: vertices within the file's tolerance merged, edges projected onto both faces' surfaces, seams and poles of closed surfaces as step 8 found them; degree above 5 by reduction where the error allows, else the face is refused and the import falls back to a reference mesh | 3 wk |
| D3.3 Booleans on imported solids | A corpus (the NIST files, the SolidWorks gripper, our own exports) imported, checked sound, and cut by a block and a cylinder; the failure rate is measured first and the target set from it, as for the threads | 2.5 wk |
| D3.4 Interface and scripts | The link group of a STEP file gets "as a solid" beside "as a reference mesh"; `Doc.LinkStep` takes the choice; Check Solid on the result; a scenario and an example | 1 wk |

**E6 general fillets.** The rolling-ball blend between two faces of any kind: its spine is
where the two faces' offsets by the radius meet, marched as the intersector marches; each
cross section is a circular arc between the contact points; the blend is fitted through those
arcs by E10's fitting, and the two faces are cut along the contact curves (D3.1's trimmed
sheets, since the faces are often of several patches). Fillets between planes, cylinders and
cones stay on step 10's exact construction; E6 is for the rest.

| Phase | Content | Effort |
|---|---|---|
| E6.1 Offsets | The offset of a patch by a distance, fitted with an error bound (E10.2's method), for the spine | 3 wk |
| E6.2 One edge, two curved faces | The blend along one smooth chain between two faces of any kind, ending where the chain meets a flat face; the faces cut back along the contact curves; the curved-curved follow-up of step 10 (a cylinder meeting a cone) is its first test | 6 wk |
| E6.3 Blends that run off a face | A contact curve that leaves its face: the blend continues onto the next face, or the edit is refused with the point | 4 wk |
| E6.4 Vertex blends | Three or more blends meeting at a vertex: a setback corner patch fitted to the three blends' ends with G1 seams, as commercial kernels do; the step 10 corner cases (spheres, mitres) stay exact | 8 wk |
| E6.5 Sweeps | Fillets on lofts, sweeps and imported solids, against OpenCASCADE's where its fillet is exact (step 10's oracle), the failure rate measured first | 3 wk |

E6 is 24 weeks of phases plus the interface and scripts, which the fillet group already has;
the range of 5 to 7 months allows for E6.4, which is the part nobody gets right the first time.
Of tier 4's other items, G4 is package 17.4 and a first part of F5 is 13.4; E12 general shell
and F5 in general are under "Left out". The four planned here are steps 20 to 22.

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

## Finishing the roadmap (plan of 2026-10-03)

Steps 1 to 11 and I7 are merged. This section gathers everything the document still holds
open: the follow-ups listed under each step, the items that never had a step, the tier 4 plans
of 11.5, and what a review of `master` found that no list held. It orders them as steps 12 to
22, under the rules of steps 1 to 11 ("Rules for every package") and the same definition of
done.

**The roadmap is finished** when every package below is done, or has been moved to "Left out"
by a decision recorded there. Nothing else in this document is open: a follow-up above carries
in square brackets the package that takes it, `[left out]`, or `[closed]`.

### What the review found

Every follow-up was checked against `master` at `4383cbd` on 2026-10-03.

**The state of `master`.** `make ci` passes stage by stage. In the run of all stages at once
`TestMenuBarKeyboard` (`pkg/platform/tkgui`) failed in the coverage stage, with the sweeps
below running beside it; the stage alone passes. The test is sensitive to load, which steps 6 to 10 met too, and is package 12.5.

**The guarded sweeps, run over their whole grids** (`TELESAPR_SWEEP=1`). Three of the four
guards allow more than now fails, so three follow-ups had mended without anybody noticing:

| Sweep | Guard allows | Fails on 2026-10-03 | Follow-up |
|---|---|---|---|
| `TestCylinderThroughCylinderSweep` | 19 of 54 | 11 of 54: ten unions that fail at 45 degrees and steeper, one difference that leaks | step 5, still open, 13.2 |
| `TestSphereThroughFaceSweep` | 10 of 135 | 10 of 135, the same points: nine at the largest bulge, one tiny cap | step 5, still open, 13.2 |
| `TestThreadSweep` | 3 of 400 | 0 of 400 | step 5, closed |
| `TestSweepGrid` (thin tori) | 7 of 72 | 0 of 72 | step 7, closed |

Which commit mended them was not looked for. The guards are tightened in 12.6, which also
keeps a guard from going stale again.

**Everything else listed as a follow-up is still open**: none of it was found done in the code.

**Statements that had gone stale**, corrected in place: the Context said steps 9 to 11 still
needed a spike; the recommended order did not mark steps 4, 5, 7 and 10 as done; "Decisions
still open" asked a question step 9 had answered; 11.5 called G4 unplanned.

**What no list held:**

- **No licence file.** The About box says GPL version 3 or later, as SolveSpace's does; the
  repository has no `LICENSE`.
- **The program has no version of its own.** `PackageVersion` in `pkg/ui/menuanalyze.go` is
  SolveSpace's `3.2~cbff7a91`. Since the rename the About box, `telesapr-cli version` and the
  VRML export say "TeleSAPR, based on SolveSpace version"; the help links still open
  `solvespace.com`.
- **The icons are SolveSpace's.** `make install` takes them from the ignored `solvespace/`
  checkout, so it fails in a fresh clone, and the window icon is `solvespace-48x48.png`. The
  desktop entry and the MIME type (`application/x-telesapr` for `*.tlspr`) were set right with
  the rename.
- **The README** lists `pkg/persist/slvs` as the file format, runs scripts on `sketch.slvs`,
  and lacks the packages added since (`topo`, `persist/tlspr`, `persist/step`, `persist/convert`,
  `persist/link`, `render/gl`, `script/agent`, `internal`).
- **Lint debt.** `make lint` reports 99 findings (38 unchecked errors, 36 from staticcheck, 21
  unused, 4 ineffectual assignments), all in code older than 2026-10-01; CI lints new lines only.
- **No remote**, so package 1.8 is where it was: the workflow has never run. `TAKIGO_REF` in
  it is takigo's head today (`e3bad2c`).
- **B3, auto-constrain on draw**, is in the estimates and was in no tier and no step.
- **S3, the packed container**, was to follow STEP import. Step 8 links a STEP file by path and
  nothing is packed.
- **C4 and half of D4 exist since the port.** A new group goes in after the active one, with
  the later ones moved down (`Sketch.AddGroupAfterActive`), which is insertion in the middle of
  the history. Binary STL files link as reference meshes (`pkg/persist/link/stl.go`); text STL
  and OBJ do not.
- **Two parallel modes remain.** `ParallelExact` is the default (`pkg/srf/parallel.go`), kept
  for bits that no longer have a C++ to match.
- **No user's guide.** `docs` holds the file format and the AI guide; the features of steps 3
  to 10 are documented by their example scripts only. The examples cover no measurement,
  freedom, conflict or unit call.
- **Notes kept outside this document**, from before the roadmap: for threads, tapered and
  variable pitch, run-out through a face, cosmetic threads, one parametric group in place of
  four; for scripting, a startup script, `--watch`, menu items made by scripts, zoom to fit.
  They are under "Left out".

### The order

| Step | Content | Effort | Risk | Needs |
|---|---|---|---|---|
| 12. Close the first eleven steps | licence, version, icons, README, CI's first run, guards, lint, an audit, a guide | 4.5 wk | Low | the user, for 12.1 and 12.4 |
| 13. Kernel soundness | the mesher's grid, curved surfaces crossing, what a Boolean leaves, coincident faces, STEP meshes | 10 wk | High | 12.6 |
| 14. Names, faces and picking | every face picked and seen, names that stay, topology's edge cases | 6 wk | Medium | none |
| 15. Solid features completed | chamfer by local edit, shell with cylinders, draft, mirror of a body, pattern along a curve | 13.5 wk | High | 14.1, 14.2 |
| 16. Small features | A2, A3, G5, C4, D1, D4, G2, D5, G1, G3, B3 and four follow-ups | 13.5 wk | Low | none |
| 17. Regeneration | one parallel mode, F8, F7, G4, S3 | 11 wk | Medium | none |
| 18. Drawings | H1, H4, H3, H2 | 12.5 wk | High for H2 | 14.2 for H2 |
| 19. Shading | I2, I6 | 5 wk | Medium | none |
| 20. Smooth lofts and sweeps | E8, E10 | 11 wk | High | 13 |
| 21. STEP import as a solid | D3, and the export follow-ups of step 11 | 12 wk | High | 13 |
| 22. General fillets | E6 | 24 to 30 wk | Very high | 20, 21 |

About 126 weeks at the pace the estimates assume, with E6 at its middle. They are weights, as
before: steps 1 to 11 were sized at about 75 weeks and were merged between 2026-10-01 and
2026-10-03.

Why this order. Step 12 is cheap, and nothing can be shown to anybody else before it. Step 13
comes before any new kernel feature, since D3.3 and E6.5 measure their failure rates on the
Boolean as it is, and a Boolean that fails 11 of 54 cylinder crossings would be measured into
them. Steps 14 and 15 mend what users of the shipped features meet today. Steps 16 to 19 do not
depend on each other or on the kernel, and may be taken in any order, or between the others,
as the original plan meant its fillers to be. Steps 20 to 22 keep the order 11.5 gave: E8, E10,
D3, E6.

The estimates of steps 13 to 15 come from no spike. Each package of step 13 opens with a day
of finding the cause, and its estimate is written into the table again after that day.

### Step 12. Close the first eleven steps

| Package | Content | Effort | State |
|---|---|---|---|
| 12.1 Licence and notices | A `LICENSE` file: GPL version 3 or later, which the port inherits and the About box already states. A `NOTICE` for code ported from other libraries (`pkg/libm`, the QR of `pkg/solver`, `pkg/ttf`), each checked against its source's licence. Needs the user's word, see "Decisions still open". | 0.2 wk | done 2026-10-03 (`7a7dd1f`): the user chose GPL version 3 or later. `NOTICE` credits glibc's libm and fdlibm, Eigen's SparseQR and COLAMD, FreeType, the fonts, three.js and Hammer.js |
| 12.2 Version and icons | A version of TeleSAPR's own in one place, printed by the About box, the exports and `telesapr-cli version` in place of "based on SolveSpace version"; the origin and its authors kept. Help links that do not open `solvespace.com`. Icons of TeleSAPR's own in `dist/freedesktop` and for the window, so `make install` works in a clone without `solvespace/`. The name, the MIME type for `*.tlspr` and `.slvs` no longer claimed were done with the rename (`74ff70b`). | 0.4 wk | done 2026-10-03 (`f48a98c`, goldens `e67f719`): `pkg/version`, 0.1.0 as the user chose; Help > User's Guide and Go to GitHub open the repository; icons in `dist/freedesktop`, and `make install` passed in a tree without `solvespace/` |
| 12.3 README | The project structure as it is, `.tlspr` throughout, what the program does that SolveSpace does not, and how to build beside a takigo checkout. | 0.2 wk | done 2026-10-03 (`43b3c2a`) |
| 12.4 Remote and CI's first run (1.8) | The user makes the GitHub repository and the `TAKIGO_TOKEN` secret. Then push, see the workflow run, and fix what the runner shows; the font package and Go 1.27 there are unverified. | 0.3 wk | open: waits for the user to make the repository and the secret |
| 12.5 `TestMenuBarKeyboard` under load | The test waits on timing and fails when the machine is busy. It waits for the event instead, so that `make ci` passes whatever runs beside it. | 0.3 wk | done 2026-10-03 (`120fa55`): the cause was in the code, a menu opened by key could lose its first entry to a late Enter of its grab; 40 runs in a row passed with twice as many busy loops as cores |
| 12.6 Guards as they are | The allowances set to what 2026-10-03 measured: cylinders 11 of 54, threads 0, sweep along a path 0. A run of a whole grid also fails when the grid does better than its allowance, so a guard cannot go stale. | 0.2 wk | done 2026-10-03 (`dc751ed`): sphere 10 of 135, cylinders 11, threads 0, sweep along a path 0; `Report.Require` fails a whole grid that does better |
| 12.7 Lint debt | The 99 findings fixed, or annotated with the reason, in commits that change no golden. `make ci` and the workflow then run `lint` on everything in place of `lint-new`. | 1 wk | done 2026-10-03 (`96205a7`): golangci-lint's limits had hidden most of it, about 220 findings in all; fixed, or excluded in `.golangci.yml` with the reason (fmt.Fprint* in errcheck, ST1005, QF1001, QF1003); `make ci` and the workflow run `lint` |
| 12.8 Audit against the definition of done | One table, a feature of steps 1 to 11 by a row of the checklist: scenario, path pair, example, golden, sweep, benchmark, undo and round trip. Known gaps to start from: no example calls `Doc.Measure`, `Entity.Free`, `Group.Conflicts` or `Doc.SetUnits`. A gap is filled, or recorded as not applying, as for 11.2's B-spline faces, which have no user surface yet. | 0.8 wk | done 2026-10-03 (`9437d39`): `docs/done-audit.md`, no empty cell; filled on the way: example `feedback`, `path_units`, `TestMirrorSweep` (0 of 144), `TestLinkStepUndo` |
| 12.9 User's guide | `docs/guide`, a page per area added since the port: parameters and units, history, section view, mirror, chamfer, fillet, shell, loft, sweep, threads, STEP links, Check Solid. Each is built on its example script and linked from the README. | 1 wk | done 2026-10-03 (`43b3c2a`): `docs/guide`, twelve pages and an index |

Done when: the workflow passes on GitHub, `make ci` passes on `master` with `lint` in it, a
fresh clone builds and installs, and the audit's table has no empty cell.

State, 2026-10-03: done but for 12.4, which waits for the user; merged into `master` from
branch `step-12-close`. `make ci` passes with `lint` in it, `make install` works without
`solvespace/`, and the audit has no empty cell. Whether a fresh clone builds is 12.4's to show,
since takigo is cloned beside it.

### Step 13. Kernel soundness

Each package ends with its sweep's allowance at 0, or with the points that remain listed in
the test with their cause. Goldens that move are regenerated in their own commit.

| Package | Content | Effort | State |
|---|---|---|---|
| 13.1 The mesher's grid | The grid of a curved patch stops refining at a fixed depth while its trims go on, and the strips between are ear-clipped into slivers: small spheres at fine tolerances and strong twists (step 10). Refinement by the sag in space in place of a cap of 16 rows, with the regeneration benchmarks as the guard, since a deeper cap alone made the loft sweep four times slower and moved the meshes of cones. | 2 wk | done 2026-10-03 (`12c98c2`, golden `55e53f9`): the grid follows a twist to 32 rows, not 16, at under 3% in the regeneration benchmarks; a bilinear patch whose corners are not in a plane is meshed as curved. Refinement by sag without the 1/20 floor was tried and made the loft sweep 7 times slower, so it was not taken. The small spheres were mended by 13.2's fold check: `TestFilletCorners` runs at a five-thousandth of the radius. Left: a strongly twisted side has four columns along its straight edges, so a triangle turned a third of a turn meshes within 2% (`TestLoftSweep`), and the two cylinder crossings of 13.2 |
| 13.2 Curved surfaces crossing | Cylinder through cylinder, 11 of 54, and the sphere cap across the seams, 10 of 135 (step 5). Both come from curves the marching intersector seeds at boundaries. | 3 wk | done 2026-10-03 (`b4aa90f`, `7e22384`, golden `7487f06`): cylinders 11 → 2 of 54, a curve lying in a flat face is now split at the seams between its patches; spheres 10 → 1 of 135, the ear clipper of curved patches skips ears that fold over in xyz. Left, named in the guards: two differences where the crossed cylinder's seam pierces the wall beside a full-height strip triangle (`2bffa08`), and a cap so small its triangles lie flat in the face's plane |
| 13.3 What a Boolean leaves | A difference keeps the cutter's removed surfaces whole, with no trims, so a chamfered block's shell has 11 surfaces for 7 faces, and Check Solid says 12 faces for an open box of 11 (steps 7 and 10); they are dropped. A cut through a fillet loses about 1 mm³ to coarser chords of the cylinder's trim (step 10). | 1.5 wk | done 2026-10-03 (`049f506`, goldens `1f34d3a`): a Boolean drops the surfaces it leaves with no trim, and the curves on them no trim uses; Check Solid counts 11 faces and 11 patches for the open box. The cut through a fillet loses 0.34 mm³ more at the default tolerance and nothing at a fine one, chord error rather than a fault (`f72f829`, `TestFilletCutChords`) |
| 13.4 Coincident faces, a first part of F5 | A part symmetric about a plane, mirrored with the original kept, puts coincident faces through the Boolean, which fails (step 7). That case and a sweep of it; further cases enter as they are found, each with a fixture. | 2 wk | done 2026-10-03 (`73a5f88`): a part symmetric about the mirror plane is its own reflection (`srf.SameShell`), and the mirror keeps it as it is; `TestMirrorSymmetricSweep` 0 of 48, 10 of its sample of 12 failing before. Flat coincident faces came through already; curved ones in general stay F5's |
| 13.5 STEP reference meshes | Six naked edges at a knife edge, faces through both poles split at the equator, 24 crossing edges in a part 5 mm across, and the triangulator's adjacency as a map (step 8). | 1.5 wk | done 2026-10-03 (`0167926`): the mesh is mended at T-junctions on its naked edges, so the knife edge read back has none; linkrods.step had no crossing edge before this step; faces through both poles fail nowhere and stay as they are; NIST_MBE_PMI_5 reads in 50 ms, as before, so the map stays |

Done when: the four guards of 12.6 and the sweeps of steps 7 and 10 stand at 0, or name what
is left.

State, 2026-10-03: done, on branch `step-13-kernel-soundness`. Run over their whole grids, every
sweep stands at 0 but two guards, which name what is left: the sphere through a face 1 of 135
(10 before) and the cylinder through a cylinder 2 of 54 (11 before). Threads 0 of 400, the sweep
along a path 0 of 72, and every sweep of steps 7 and 10 at 0; new sweeps: the mirror 0 of 144,
the symmetric mirror 0 of 48, triangle lofts turned 0 of 27. The estimate of 10 weeks was not
re-sized after the first day, as planned, since each package's cause was found within it.

### Step 14. Names, faces and picking

| Package | Content | Effort | State |
|---|---|---|---|
| 14.1 Every face can be picked and seen | A face that only has an `Origin` (a loft's, a fillet's, a Boolean tool's) is picked by the mouse as other faces are (steps 9 and 10). The display mesh tells such faces apart, so the edges between them are drawn (step 11). The patches of one fillet get a sheet key, so a circle's fillet is one face and not four (step 11). A face of several patches is named by any of them (step 11). | 2.5 wk | done 2026-10-03 (`f055ba9`, `7b45aef`, `db3a4b7`, goldens `85429df`, `1cc89cf`): triangles of faces with only an Origin carry it, so the display draws the edges between faces where they meet smoothly too (`EdgeSharpOrFace`) and the mouse picks them (`Selection.Face`, which Shell Solid takes); a circle's fillet is one face (a sheet key per edge); `Solid.Face` finds a face by any patch's identity. `TestEveryFacePicked`: every face of every solid golden is picked, the bent pipe's inside through the section view |
| 14.2 Names that stay | A surface's `Origin` comes from what made it and not from its place among its group's surfaces, so a thread made longer does not rename what follows (step 9). A sweep that reorders the edges between two faces along their spread: a name reports and never moves (step 9). | 1.5 wk | done 2026-10-03 (`0ff7354`, golden `4e9aaee`): a helix's surfaces take a local origin from curve and section, so a longer thread renames none (other groups' counts follow only their sketch and keep their places, which saved names use); names count the edges between their faces, `edge A B N of M`, and report rather than move when that count changes (format 2 gains `of <m>`, names without it read as before) |
| 14.3 Topology's edge cases | A non-manifold edge is refused at its place or modelled (step 9, the knife edge). A fixture for a loop through a vertex twice (step 9). A fillet takes an edge that goes on straight through a vertex a Boolean left (step 10). | 1 wk | done 2026-10-03 (`de895d1`): a non-manifold edge is reported with where it lies and marked by Check Solid; two holes touching at a corner are the fixture, and of a loop through a vertex twice. A fillet along an edge that runs into a vertex where other edges meet is refused with a message naming the five edges, as 22.4's corners are; no case was found of an edge going on straight through a vertex with nothing else at it |
| 14.4 Hover on large meshes | The hidden-edge test casts a ray at the display mesh on every hover (step 9); it gets a tree, or the answer is kept for the view. | 0.5 wk | done 2026-10-03 (`43d7ab1`): a grid of the display mesh's triangles by screen cell, kept for the view; 104 ns a hover on the triple-start thread against 3.8 µs, same answers (`TestHoverGridSameAsScan`); perspective views still scan |
| 14.5 Face normals point out | The side faces of an extrusion carry normals that point into the solid; the section view turns them by the mesh (step 6). They are turned once, where the faces are made, for everything that reads them. | 0.5 wk | done 2026-10-03 (`62d59bb`), by another route: the face entities keep their sketch's normal, since a distance constraint on a face keeps its sign in the file and turning the normal would move its point in every saved model; `Sketch.OutwardFaceNormal`, the section's, is what Measure uses now |

Done when: every face of every golden solid is picked by a click in a scenario, and the naming
sweep of 9.6 passes with a thread upstream whose length changes.

State, 2026-10-03: done, on branch `step-14-names-faces`. `TestEveryFacePicked` hovers every
face of every solid golden into the selection, and `TestNamesThroughThreadLength` resolves every
edge name of a threaded part after its thread is doubled, but for the four whose faces no longer
meet, which say so.

### Step 15. Solid features completed

| Package | Content | Effort | State |
|---|---|---|---|
| 15.1 Chamfer by local edit | Straight edges go through the fillet's edit, as circular ones do since 10.3, in place of wedges and a Boolean. That brings concave edges, chains through smooth joins (step 10), and two distances, with the face that takes the first named (step 7). | 2.5 wk | done 2026-10-03 but for two distances (`48cba44`, goldens `3e82b81`): the chamfer tries the edit first, flat, for every edge, and falls back to wedges where it can't make a corner (a concave edge at a notch) or on a solid forced to a mesh; concave edges are filled and chains go round smooth joins by cones; a mirror of a chamfer reflects the solid so far. Sweep 0 of 84. Two distances would need the edit's corners to set back on each face apart, and are left |
| 15.2 Shell with cylinders | E11 as it was sized: a cylinder's offset is a cylinder, and a vertex is where the offset surfaces meet, no longer three planes (step 10). Filleted and lofted solids stay refused until 22.1 gives offsets of other surfaces. | 3 wk | done 2026-10-03 for prisms (`e3d41f9`, golden `8aac3dc`): a solid whose sides are flat or extrusions of arcs, with flat ends, is hollowed by offsetting an end's outline (lines moved, arcs to a smaller or larger radius, an arc thinner than the walls to a sharp corner), extruding it between the ends moved in and taking it away; sides can't be opened, other curved faces are refused. Path pair `path_shell_cup`, example `pot`, sweep 0 of 108 at a chord tolerance of 0.001; at the default 0.1, walls thinner than about twice it beside a round hole come out unsound (in the guide) |
| 15.3 Shell vertices of four faces | An apex whose faces do not meet in one point once moved gets a new edge (step 10). | 1 wk | done 2026-10-03 (`616528d`, sweep `28a0c33`): the vertex splits into two points, each on three of the planes, joined by a new edge between the two opposite faces; five faces or more must still meet in one point. Sweep of apexes 0 of 108 |
| 15.4 E13 draft angle | Side faces of an extrusion tilted about a neutral plane, by `ReplaceSurface` on flat faces; cylinders become cones. | 4.2 wk | done 2026-10-03 by another route (`ecb2de2`, goldens `9f53558`): the far end is the profile offset as 15.2 offsets an end, and each piece is joined to its copy by a ruled patch, a plane or an exact cone, with no `ReplaceSurface` and no Boolean. The extrusion's `draft` (ValC, degrees) on its group screen and `Group.SetDraft`; the sketch's plane is the neutral one, a two-sided extrusion leans both ways; arcs must meet their neighbours tangent; the far end's entities are hidden, not moved. Scenario `ext_draft`, path pair `path_draft`, example `draft`, sweep 0 of 150 |
| 15.5 Mirror of a body | A mirror reflects its source group; an option reflects the solid so far (step 7). | 1 wk | done 2026-10-03 (`3d73b15`, `bc01560`): `Group.MirrorSolid`, file field `mirror-solid`, the screen's "the whole solid so far", `Doc`/`Group.SetMirrorSolid`; the mirror example mirrors a bracket of two groups |
| 15.6 E2 pattern along a curve | Step and repeat along the path the sweep of 7.4 takes. | 1.8 wk | done 2026-10-03 (`606d643`, goldens `9985efc`): group `pattern-path`, New Group > Pattern Along Path, `Doc.PatternAlongPath`, example `pattern`; sweep 0 of 36. The solid is repeated, not the entities; a closed path is refused |
| 15.7 The message of a stopped chain | A fillet whose radius stops its chain at a rounded corner says so, in place of naming the curved face (step 10). | 0.1 wk | done 2026-10-03 (`519f19d`) |

Done when: each has its sweep at a rate set before it starts, as in step 10.

State, 2026-10-03: done, on branch `step-15-solid-features`, each package's sweep at 0 over its
whole grid (the chamfer 0 of 84, the curved shell 0 of 108, the apexes 0 of 108, the draft 0 of
150, the pattern 0 of 36). Left: two chamfer distances; drafts of chosen faces, about another
plane, or of an arc meeting a line at a corner; shells of other curved solids (22.1).

### Step 16. Small features

None needs the kernel. Efforts include the 20%, as in "Items outside the numbered steps".

| Package | Content | Effort | State |
|---|---|---|---|
| 16.1 A3 dimensions by name | A constraint's value is read in an expression by the constraint's name. | 0.6 wk | not started |
| 16.2 A2 configurations | Named sets of parameter values, one active. | 1.2 wk | not started |
| 16.3 G5 motion study | One parameter driven through a range, with the frames exported. | 1.8 wk | not started |
| 16.4 C4 insert a group mid-history | Exists since the port. Left: a scenario, a path pair, and a test that an inserted group can't be referred to by an earlier one, by step 4's dependency check. | 0.2 wk | not started |
| 16.5 D1 3MF export | | 0.6 wk | not started |
| 16.6 D4 text STL and OBJ as reference meshes | Binary STL links already. | 0.6 wk | not started |
| 16.7 G2 bill of materials | From link groups, to the text window and a CSV file. | 0.6 wk | not started |
| 16.8 D5 STEP export with assembly structure | A product per linked part, with its colour. | 1.2 wk | not started |
| 16.9 G1 mate shortcuts | Concentric, flush and hinge, from existing constraints. | 2.4 wk | not started |
| 16.10 G3 interference by shells | The check between linked parts uses their shells, not their triangles. | 1.2 wk | not started |
| 16.11 B3 auto-constrain on draw | More inference than horizontal and vertical: coincident, tangent, parallel, perpendicular, equal. | 1.8 wk | not started |
| 16.12 The section's cap | A style for it, with a hatch (step 6). | 0.4 wk | not started |
| 16.13 A linked STEP file's place | Placed where it was drawn, not at the middle of the view (step 8). | 0.3 wk | not started |
| 16.14 What an AI program sees | `render` of the view as the window shows it and of a chosen group; what the mouse hovers, in `get_drawing` (I7). | 0.6 wk | not started |

### Step 17. Regeneration

| Package | Content | Effort | State |
|---|---|---|---|
| 17.1 One parallel mode | The fast mode becomes the only one; `ParallelExact` goes, and the tests that compare the two. Goldens that move in their last bits are regenerated in their own commit. | 0.3 wk | not started |
| 17.2 F8 parallel `MergeCoincidentSurfaces` | No longer bound to the serial order. | 1 wk | not started |
| 17.3 F7 regeneration off the UI thread | With cancel, and the window drawing the last good solid meanwhile. | 3.6 wk | not started |
| 17.4 G4 large assemblies | Links that load a mesh and its points without regenerating the part, and meshes kept between sessions. It was in tier 4 unplanned; at L and medium risk it is sized like its neighbours. | 4.8 wk | not started |
| 17.5 S3 packed container | A zip of the text model with its linked parts and imported geometry, which is also where 17.4's meshes are kept. | 1.2 wk | not started |

### Step 18. Drawings

| Package | Content | Effort | State |
|---|---|---|---|
| 18.1 H1 sheet with standard views | | 3.6 wk | not started |
| 18.2 H4 title block and templates | | 1.2 wk | not started |
| 18.3 H3 section and detail views | With the hatch of 16.12. | 2.4 wk | not started |
| 18.4 H2 associative dimensions | A dimension holds the names of the edges it measures (step 9's names, made stable by 14.2). | 5.4 wk | not started |

### Step 19. Shading

| Package | Content | Effort | State |
|---|---|---|---|
| 19.1 I2 shader backend | | 3.6 wk | not started |
| 19.2 I6 appearance beyond a colour | | 1.2 wk | not started |

### Steps 20 to 22. The tier 4 items

Their plans are under "11.5: the tier 4 items opened" and are not repeated. What the review
adds to them:

| Step | Packages | Added from the follow-ups | Effort |
|---|---|---|---|
| 20. Smooth lofts and sweeps | 20.1 to 20.4 are E8.1 to E8.4; 20.5 to 20.8 are E10.1 to E10.4 | 20.1 also splits curves so that profiles of unequal counts match (step 10, loft matching); lofts through more than two profiles are E8 itself, 0.5 wk | 11 wk |
| 21. STEP import as a solid | 21.1 to 21.4 are D3.1 to D3.4 | 21.2 takes degree above 5 (step 11). 21.5: on export, knots removed where patches join with matching derivatives, and faces whose patches make no grid written as one surface where they can be (step 11), 1.5 wk. 21.6: tessellated STEP read as a reference mesh (step 8), 1 wk | 12 wk |
| 22. General fillets | 22.1 to 22.5 are E6.1 to E6.5 | 22.2 closes fillets between two curved faces, and 22.4 the corners step 10 refuses: more than three faces, convex meeting concave, a circular edge meeting a rounded one at an angle, unequal radii | 24 to 30 wk |

### Left out

Proposed on 2026-10-03; the user decides. Each stays out until its reason stops holding.

| Item | From | Reason |
|---|---|---|
| E12 general shell | tier 4 | Open ended. After 22.1 has offsets of curved surfaces, a part of it (faces whose offsets do not cross themselves) can be sized. |
| F5 tangent and coincident contact in general | tier 4 | It never finishes. 13.4 takes the first case; each later one enters as a fixture with a sweep. |
| 9.2 topology made by the kernel | step 9 | Derived topology costs 7 ms for 270 faces, and no feature has needed identities kept through a Boolean. |
| OpenGL on Windows and macOS | step 6 | TeleSAPR has been built and tested on Linux only. takigo has backends for both; a port is a project of its own, and the GL context a small part of it. |
| The section saved in files | step 6 | It is view state. It goes with saved views, which nothing plans. |
| Open thread ends in STEP | step 8 | The file is open there; closing it would invent geometry. |
| CSG and swept solids in STEP | step 8 | Reported by name. No vendor file in the corpus has them. |
| Benchmarks moving with code layout | step 7 | An observation, not work. |
| A chat window, MCP resources and prompts, an HTTP transport, a sandbox | I7 | "An MCP server, and nothing else" stands until a user needs otherwise. |
| `make ai-eval` | I7 | It needs a model and an account, outside CI. Worth taking when the primer is next changed. |
| Tolerances per edge and vertex | "Decisions taken" | 21.2 heals within a file's tolerance. More only if 21.3's measured rate asks for it. |
| Generated entities left out of `.tlspr` | the format's design | Possible behind the version number; no need has come up. |
| Threads: tapered and variable pitch, run-out, cosmetic threads, one parametric group | notes of 2026-09-30 | Never asked for. |
| Scripting: a startup script, `--watch`, menu items made by scripts, zoom to fit | notes of 2026-10-01 | Never asked for. |
| The bit-exact scaffolding: `pkg/libm`, `CxxSort`, the FMA build rules | "Decisions taken" | Harmless where it is. Replacing it moves every golden for nothing. |
| Sheet metal, direct modelling, tolerancing, CAM, simulation | J | Unchanged. |

## Decisions still open

Taken on 2026-10-03: the licence is GPL version 3 or later (12.1), and the version 0.1.0 (12.2).

Topology first or surface type first, the one question here before, was taken as recommended:
topology in step 9, the surface type in step 11. What is open now is the user's, and all of it
comes from "Finishing the roadmap":

- **The remote (12.4).** Where the repository goes and whether it is public. CI was prepared
  for GitHub, with takigo private beside it. The module path is `github.com/telesapr/telesapr`,
  so the repository belongs in a GitHub organisation `telesapr`, which the user has to hold.
- **"Left out".** Every row is a proposal; a row the user wants back gets a package.
- **What the review took in without being asked**: B3 (16.11), G4 (17.4) and S3 (17.5).

## How to check an estimate before committing to it

For each kernel-dependent item, a two-day spike on the hardest fixture is cheaper than trusting
the table. Suggested spikes: a Boolean-route fillet on one edge of a box with a hole near the edge
for E4, a ruled loft from square to circle subtracted from a block for E7, and one real vendor STEP
file of a fastener and one of a connector for D2. `TestThreadSweep`, in `pkg/persist/slvs/thread_test.go`, is the model for a sweep-style
robustness test, and the naked-edge check in `pkg/ui/menuanalyze.go` is the pass criterion.
