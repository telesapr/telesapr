# Roadmap: from a SolveSpace port to a modern CAD

Date: 2026-10-01, compacted 2026-10-04. Status: steps 1 to 21, 23, 29 and I7 are done and merged, but
for 12.4 (CI's first run on GitHub), which waits for the user, and 21.5; step 22 is merged in
part. The open work is planned as steps 23 to 29. Read "Definition of done for every new
feature" first.

TeleSAPR began as a Go port of SolveSpace, bit-identical with the C++. Since 2026-10-01 it is a
separate project: no parity or file compatibility with SolveSpace, a Go-native kernel, its own
`.tlspr` format (`docs/tlspr-format.md`), and `.slvs` files read by File > Import only.

This document was compacted on 2026-10-04. The full plan, with each step's packages, the
estimates by area, the review of 2026-10-03 and the design notes, is this file at commit
`2ec37e4`; comments in the code that cite a package (as "package 5.1") refer to that version.
User-facing documentation of what was built is in `docs/guide`, `docs/ai.md` and
`docs/done-audit.md`.

## Done

| Step | Content | Merged |
|---|---|---|
| 1 | Test infrastructure (invariant oracle `pkg/check`, `.goui` goldens, path pairs, API completeness, sweeps and fuzzing, `make ci`) and the screw thread | `c5caea3` |
| 2 | The `.tlspr` format and `.slvs` import | `d5b610c` |
| 3 | Named parameters and expressions, document units (format 2), freedom colouring, conflicts, command search, measure | `22daacd` |
| 4 | History editing: reorder with dependency checks, suppress whole groups, broken references | `fe7cb85` |
| 5 | Raycast fix, closed intersection loops, C++ comparison frozen | `f24fcd9` |
| 6 | OpenGL renderer, section view | `0a96960` |
| 7 | Mirror, chamfer, sweep along lines and arcs | `a5829a6` |
| 8 | STEP import as a reference mesh | `e556586` |
| 9 | Derived topology, edge names, chamfer of any edge | `6f60438` |
| 10 | Fillets of straight and circular edges with corners, shell of flat faces, ruled loft | `47ff6e0` (last) |
| 11 | General surface type: multi-patch faces, B-splines with knots, degree up to 5, STEP export per face | `a8a973b` |
| I7 | Drawing with an AI model: an MCP server in `telesapr-cli mcp` running Go scripts | `baaec8d` |
| Name | Rename to TeleSAPR, module `github.com/telesapr/telesapr`, `.tlspr` | `74ff70b` |
| 12 | Licence (GPL-3.0-or-later), version 0.1.0, icons, README, lint at 0, audit, user's guide; not 12.4 | `f5c6971` |
| 13 | Kernel soundness: seam splits, fold check, twist rows, untrimmed surfaces dropped, symmetric mirror, STEP T-junctions | `101f59e` |
| 14 | Every face picked, names that survive, non-manifold edges reported, hover grid, outward normals for Measure | `f512ed1` |
| 15 | Chamfer by local edit, shell of curved prisms, four-face apexes, draft angle, mirror of the whole solid, pattern along a path | `fb151c3` |
| 16 | Dimensions by name, configurations, motion study, insert mid-history, 3MF, text STL/OBJ, BOM, STEP assemblies, mates, interference by shells, auto-constrain, section hatch, STEP links in place, AI hover and render | `5e2d5be` |
| 17 | One parallel mode, parallel merge of coincident planes, regeneration off the UI thread, link and STEP caches, `.tlsprz` packs | `978d25f` |
| 18 | Drawing sheets: standard views, title block, section and detail views, associative dimensions | `2c41d13` |
| 19 | Shading: GLSL lighting, appearances | `6462cc3` |
| 20 | Smooth lofts through any number of profiles, sweeps along 3D cubics with frames and twist | `dda4e0b` |
| 21 | STEP import as an exact solid in Booleans, tessellated STEP; not 21.5 | `21abd88` |
| 22 (part) | Offsets of patches (22.1), coaxial cylinder/cone fillets and chamfers (22.2's first case) | `ddf8144` |
| 23 | Scripting and MCP fixes: contour problems reported on a sketch and its solid, one undo step per script, the active group after a delete and moves with sources, renders that fit, the interpreter's forms, two-sided groups from scripts, no silent File > New | `d92c5f8` |
| 29 | Drawing sheet shown in the window, associative dimensions on isometric, section and detail views and of faces, nested links in the bill of materials, auto-constrain for arcs and circles | `9fcca23` |

Decisions taken along the way: the unit belongs to the document (format 2); files broken by
hand get an error on load and no other safeguard; files with the old `spacesolve` header are
not read.

## Open work: steps 23 to 29

Planned on 2026-10-04 from the open items left at the compaction. The order: the scripting and
MCP fixes first (small, found in real use, and they ease every later step's scripts); then the
Booleans on curved surfaces, since the rolling ball of step 25 needs the same intersections and
trims they fail on; then the rest of step 22; then STEP, solid features and drawings, which do
not depend on each other. Efforts are the earlier plan's where it gave one; the others, marked
"~", are first guesses of 2026-10-04. Each step is one branch as the rules below say.

### Waiting for the user

| Item | From | Effort | State |
|---|---|---|---|
| The remote and CI's first run | 12.4 | 0.3 wk | The GitHub repository (organisation `telesapr`, for the module path) and the `TAKIGO_TOKEN` secret; then push, watch the workflow and fix what its runner shows. Until then no fresh clone has been shown to build. Best done before the next merge, blocks nothing |
| What to take back from "Left out" | | | Every row there is a proposal |
| What the review took in without being asked | | | B3 (16.11), G4 (17.4) and S3 (17.5): all done; the user may still disown them |

### Step 24: Booleans on curved surfaces (~6 wk), branch `step-24-kernel-robustness`

Begins with a two-day spike on the hardest fixture, to learn whether these share a cause (seams
and poles in the intersector and in trim splitting). The failure counts to reach are fixed
before the work starts.

| Package | From | Content | State |
|---|---|---|---|
| 24.1 cylinder through cylinder, sphere cap | step 5, 13.2 | 2 of 54 in the guard: differences where the crossed cylinder's seam pierces the wall beside a full-height strip triangle. 1 of 135: a sphere cap too small, its triangles lie flat in the face's plane | not started |
| 24.2 cylinders, cones through their apex, tori, spheres' poles | 21.3 | 10 of 28 cuts of the analytic STEP set fail on these, whatever made the solid (`solidBooleanMaxFail`); the same limits as 24.1 | not started |
| 24.3 lathe seams and pole discs | MCP notes | A union fails where an extruded band's arcs cross a lathed cone at a shallow angle, over the lathe's seam (a line 0.76 or 1.0 mm inside a 3 mm wall, arcs of 6 to 8 mm; the angle alone doesn't separate failing from working); `Problem()` was "" though the Boolean failed. A cut whose end cap lies in a lathed end face's plane (a disc of four patches with a pole) leaves naked edges: a bore along a lathed spout's axis ending at its tip, 32 naked edges, not flagged; 3 mm past the tip is sound, and the same with a swept spout was sound. Both must also set `Problem()` | not started |
| 24.4 twisted ruled faces mesh with rows, no columns | 13.1, step 20 | A triangle loft turned a third of a turn meshes within 2% (`TestLoftSweep`), a twisted sweep's flat sides within about 2%; the surfaces are right. Refinement by sag alone made the loft sweep 7 times slower; the benchmark gates the fix | not started |
| 24.5 hover in perspective views | 14.4 | Scans the mesh; orthographic views use the grid | not started |

Tangent and coincident contact in general (F5, 13.4) stays under "Left out"; a cut along a
patch's own edge is the case 21.3's cutters avoid.

### Steps 25 and 26: the rest of step 22, general fillets, branch `step-22-general-fillets`

The rolling-ball blend between two faces of any kind: its spine is where the two faces'
offsets by the radius (22.1) meet, marched as the intersector marches; each cross section is a
circular arc between the contact points; the blend is fitted through those arcs by step 20's
fitting, and the two faces are cut along the contact curves (step 21's trimming, since the
faces are often of several patches). Fillets between planes, cylinders and cones stay on
step 10's exact construction. Step 25 is 22.2; step 26 is 22.3 to 22.5, on the same branch or,
once 22.2 is merged, a new one.

| Step | Package | Content | Effort | State |
|---|---|---|---|---|
| 25 | 22.2 one edge, two curved faces | The blend along one smooth chain between two faces of any kind, ending where the chain meets a flat face; the faces cut back along the contact curves. Covers a cylinder meeting a cylinder at an angle, spheres, tori, B-spline faces, a chamfer's cone meeting the cylinder it was cut from. The teapot's spout (a cylinder meeting a cone at 48 degrees) and handle are ready tests | 6 wk, less the coaxial case done | coaxial case done (`ddf8144`) |
| 26 | 22.3 blends that run off a face | A contact curve that leaves its face: the blend continues onto the next face, or the edit is refused with the point | 4 wk | not started |
| 26 | 22.4 vertex blends, and the corners step 10 refuses | Three or more blends at a vertex: a setback corner patch fitted to the blends' ends with G1 seams. Refused today: corners of more than three faces (but a lathe's quarters meeting smoothly), a circular edge meeting another rounded edge at an angle, convex meeting concave, rounded edges of different radii (setbacks, variable radius) | 8 wk | not started |
| 26 | 22.5 fillets on lofts, sweeps and imported solids | Against OpenCASCADE where its fillet is exact, the failure rate measured first | 3 wk | not started |

### Step 27: STEP (~5 wk), branch `step-27-step`

| Package | From | Content | Effort | State |
|---|---|---|---|---|
| 27.1 export: knots removed, one surface per face | 21.5, step 11 | Knots removed where patches join smoothly, faces whose patches make no grid as one surface. The export reads back exact as it is; OpenCASCADE's volume of a face written at full multiplicity is off by up to 0.3% | 1.5 wk | not started |
| 27.2 degree above 5 | 21.2, step 11 | Surfaces and curves of degree above 5 are refused, so the file links as a mesh; needs degree reduction or approximation | ~1 wk | not started |
| 27.3 one face's edge along another face's seam | 21.3 | NIST_MBE_PMI_5 and _6 fail to close for it and link as meshes | ~0.5 wk | not started |
| 27.4 edges further from their faces than the file's tolerance | 21.3 | Schenkel and the OpenCASCADE screw leave mesh gaps or crossings and link as meshes; 5 of the 9 vendor files read as sound solids | ~1 wk | not started |
| 27.5 offset surfaces and conics as solids | 21.2 | Offset surfaces (by 22.1) and parabolas, hyperbolas are refused; the file links as a mesh | ~1 wk | not started |

### Step 28: solid features (~5 wk), branch `step-28-solid-features`

| Package | From | Content | State |
|---|---|---|---|
| 28.1 chamfers of two distances | step 7, 15.1 | The edit's corners need their setback on each face apart | not started |
| 28.2 drafts of chosen faces, about another plane, of an arc meeting a line at a corner | 15.4 | Only an extrusion's sides, about the sketch's plane, arcs tangent to their neighbours | not started |
| 28.3 smooth lofts: tangency set at the end profiles, guide curves | step 20 | not done | not started |
| 28.4 sweeps along a chain of the solid's edges (step 9's names), and paths with corners | 20.5, step 7 | A corner is a diagnostic today | not started |
| 28.5 pattern along a closed path | 15.6 | refused | not started |

Shells of other curved solids (step 10, 15.2: prisms only; filleted, lofted and revolved solids
are refused) stay with E12 under "Left out"; 22.1's offsets are the first part.

## Definition of done for every new feature

Requirement set by the user on 2026-10-01: a new feature is well tested, has a user interface,
and has scripting support. A feature that lacks one of the three is not done and does not merge.
The three paths, interface, script and file, must reach the same engine entry point in
`pkg/sketch`, so none of them can drift from the others. `docs/done-audit.md` holds the table
of every feature against this list.

### Tests

| Requirement | How it is met here |
|---|---|
| Unit tests for new engine and kernel code | Beside the code, in the package. New packages hold at least 85% statement coverage and no exported function at 0% (`scripts/coverage-packages.txt`). Measure with `-coverpkg=./pkg/...`, since `pkg/sketch` and `pkg/core` are mostly exercised from other packages. |
| Geometric invariants on every solid a test builds | The oracle `pkg/check`: watertight shell, no naked or self-intersecting edges, face count, and volume and centre of mass against a closed-form value where one exists. |
| Golden files | A `.tlspr` fixture per feature variant under `testdata`, with a regeneration script. A golden only changes in a commit that says why. |
| Save and load round trip | Load, regenerate, save gives the same bytes. A file from the previous format version still loads. |
| Undo and redo | Every creating and editing action is one undo step and comes back identical on redo. |
| Failure behaviour | Bad input gives a diagnostic on the group, never a panic and never silently wrong geometry. Each documented failure has a test. |
| Robustness sweep, for solid features | A parameter sweep over sizes, angles and placements (`pkg/internal/sweep`), reporting the failure rate. The acceptable rate is fixed before the feature starts. Fuzz targets for parsers. |
| Concurrency | `go test -race` over the touched packages, and the same result on one goroutine and on eight. |
| Performance | A benchmark for anything on the regeneration path, compared with the previous commit. |
| Interface scenario | At least one script under `testdata/ui/scripts` that drives the feature through menus, mouse and text window on the headless platform. |
| Path equivalence | One test builds the same model through the interface script and through a Go script and requires identical saved files. |

### User interface

- A menu entry in `pkg/ui/menu_table.go`, with an accelerator where one is free, and a toolbar icon for features used while drawing.
- A text-window screen showing every parameter of the feature, editable in place, with expressions accepted wherever a number is.
- Selection-driven creation as the existing groups do it: select, then invoke. A clear message when the selection does not fit, saying what to select.
- Feedback on the canvas: hover and selection highlighting for whatever the feature lets the user pick, and failure marked on the model where it happened.
- An entry in the command search.
- No feature reachable only from the console or the command line.

### Scripting

- A method on `pkg/script/api` to create the feature, to read and change every parameter the text window shows, and to query the result.
- Doc comments on every method, since `help()` and `docs.json` are generated from them. `make generate` is run and `make generate-check` passes.
- A runnable example under `examples/scripts`, executed by a test.
- A test in `pkg/script/api` that covers misuse as well as use, as `TestMisuse` does.
- Reachable from `telesapr-cli run`, so the feature works without a window.
- Kernel foundations have no interface of their own, but expose what scripts can use.

## Rules for every package

- **One branch per step**, cut from `master` and named after the step, such as `step-22-general-fillets`.
  A package is one commit or a short series, and `make test` passes after each. When the step is
  finished, `make ci` passes on the branch and it is merged into `master`. Steps never stack on
  each other's unmerged branches.
- **Engine first.** The order inside a feature is: entry point in `pkg/sketch`, save and load,
  interface, script API, path equivalence test.
- **Undo is a field, not code.** Document state added by a feature goes into `UndoState` in
  `pkg/sketch/undo.go`, or it is lost on undo.
- **A golden changes only on purpose.** A commit that regenerates goldens runs the
  regeneration script, contains nothing else, and says why in its message.
- **A step is finished** when every feature in it passes the definition of done, this document
  is updated, and its branch is merged into `master`.
- **Estimates** are focused weeks for one developer who knows the code base, the definition of
  done included. Read them as relative weights: steps 1 to 21 were sized at about 175 weeks
  and were merged between 2026-10-01 and 2026-10-04. For a kernel item, a two-day spike on the
  hardest fixture is cheaper than trusting the table.
- **takigo is a sibling checkout**, replaced in `go.mod`. CI pins its commit in `TAKIGO_REF`,
  which has to move whenever `pkg/platform/tkgui` follows a newer takigo.

## Left out

Proposed on 2026-10-03; the user decides. Each stays out until its reason stops holding.

| Item | Reason |
|---|---|
| E12 general shell | Open ended. With 22.1's offsets of curved surfaces, a part of it (faces whose offsets do not cross themselves) can be sized. |
| F5 tangent and coincident contact in general | It never finishes. 13.4 took the first case; each later one enters as a fixture with a sweep. |
| Topology made by the kernel (9.2) | Derived topology costs 7 ms for 270 faces, and no feature has needed identities kept through a Boolean. |
| OpenGL on Windows and macOS | TeleSAPR has been built and tested on Linux only. takigo has backends for both; a port is a project of its own. |
| The section saved in files | It is view state. It goes with saved views, which nothing plans. |
| Open thread ends in STEP | The file is open there; closing it would invent geometry. |
| CSG and swept solids in STEP | Reported by name. No vendor file in the corpus has them. |
| A chat window, MCP resources and prompts, an HTTP transport, a sandbox | "An MCP server, and nothing else" stands until a user needs otherwise. |
| `make ai-eval` | It needs a model and an account, outside CI. Worth taking when the primer is next changed. |
| Tolerances per edge and vertex | 21.2 heals within a file's tolerance. More only if 21.3's measured rate asks for it. |
| Generated entities left out of `.tlspr` | Possible behind the version number; no need has come up. |
| Threads: tapered and variable pitch, run-out, cosmetic threads, one parametric group | Never asked for. |
| Scripting: a startup script, `--watch`, menu items made by scripts, zoom to fit | Never asked for. |
| The bit-exact scaffolding: `pkg/libm`, `CxxSort`, the FMA build rules | Harmless where it is. Replacing it moves every golden for nothing. |
| Sheet metal, direct modelling, tolerancing, CAM, simulation | Each is an XL product in its own right. |
