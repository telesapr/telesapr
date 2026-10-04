# Roadmap: from a SolveSpace port to a modern CAD

Date: 2026-10-01, compacted 2026-10-04. Status: steps 1 to 21 and I7 are done and merged, but
for 12.4 (CI's first run on GitHub), which waits for the user, and 21.5; step 22 is merged in
part. Read "Definition of done for every new feature" first.

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

Decisions taken along the way: the unit belongs to the document (format 2); files broken by
hand get an error on load and no other safeguard; files with the old `spacesolve` header are
not read.

## Open work and follow-ups

Efforts are the plan's where it gave one, "not sized" where it never did.

### Waiting for the user

| Item | From | Effort | State |
|---|---|---|---|
| The remote and CI's first run | 12.4 | 0.3 wk | The GitHub repository (organisation `telesapr`, for the module path) and the `TAKIGO_TOKEN` secret; then push, watch the workflow and fix what its runner shows. Until then no fresh clone has been shown to build |
| What to take back from "Left out" | | | Every row there is a proposal |
| What the review took in without being asked | | | B3 (16.11), G4 (17.4) and S3 (17.5): all done; the user may still disown them |

### The rest of step 22, general fillets

The rolling-ball blend between two faces of any kind: its spine is where the two faces'
offsets by the radius (22.1) meet, marched as the intersector marches; each cross section is a
circular arc between the contact points; the blend is fitted through those arcs by step 20's
fitting, and the two faces are cut along the contact curves (step 21's trimming, since the
faces are often of several patches). Fillets between planes, cylinders and cones stay on
step 10's exact construction.

| Package | Content | Effort | State |
|---|---|---|---|
| 22.2 one edge, two curved faces | The blend along one smooth chain between two faces of any kind, ending where the chain meets a flat face; the faces cut back along the contact curves. Covers a cylinder meeting a cylinder at an angle, spheres, tori, B-spline faces, a chamfer's cone meeting the cylinder it was cut from | 6 wk, less the coaxial case done | coaxial case done |
| 22.3 blends that run off a face | A contact curve that leaves its face: the blend continues onto the next face, or the edit is refused with the point | 4 wk | not started |
| 22.4 vertex blends, and the corners step 10 refuses | Three or more blends at a vertex: a setback corner patch fitted to the blends' ends with G1 seams. Refused today: corners of more than three faces (but a lathe's quarters meeting smoothly), a circular edge meeting another rounded edge at an angle, convex meeting concave, rounded edges of different radii (setbacks, variable radius) | 8 wk | not started |
| 22.5 fillets on lofts, sweeps and imported solids | Against OpenCASCADE where its fillet is exact, the failure rate measured first | 3 wk | not started |

The teapot's spout (a cylinder meeting a cone at 48 degrees) and handle are ready tests for
22.2's general case.

### Kernel

| Item | From | Effort | State |
|---|---|---|---|
| Cylinder through cylinder | step 5, 13.2 | not sized | 2 of 54 in the guard: differences where the crossed cylinder's seam pierces the wall beside a full-height strip triangle |
| A sphere cap too small | step 5, 13.2 | not sized | 1 of 135 in the guard: its triangles lie flat in the face's plane |
| Twisted ruled faces mesh with rows, no columns | 13.1, step 20 | not sized | A triangle loft turned a third of a turn meshes within 2% (`TestLoftSweep`), a twisted sweep's flat sides within about 2%; the surfaces are right. Refinement by sag alone made the loft sweep 7 times slower |
| Booleans on cylinders, cones through their apex, tori, spheres' poles | 21.3 | not sized | 10 of 28 cuts of the analytic STEP set fail on these, whatever made the solid (`solidBooleanMaxFail`); the same limits as the two guards above |
| Tangent and coincident contact in general | F5, 13.4 | | under "Left out"; a cut along a patch's own edge is the case 21.3's cutters avoid |
| Hover in perspective views | 14.4 | not sized | Scans the mesh; orthographic views use the grid |

### Solid features

| Item | From | Effort | State |
|---|---|---|---|
| Chamfers of two distances | step 7, 15.1 | not sized | The edit's corners would need their setback on each face apart |
| Drafts of chosen faces, about another plane, of an arc meeting a line at a corner | 15.4 | not sized | Only an extrusion's sides, about the sketch's plane, arcs tangent to their neighbours |
| Shells of other curved solids | step 10, 15.2 | | Prisms only; filleted, lofted and revolved solids are refused. With E12 under "Left out"; 22.1's offsets are the first part |
| Smooth lofts: tangency set at the end profiles, guide curves | step 20 | not sized | not done |
| Sweeps along a chain of the solid's edges (step 9's names), and paths with corners | 20.5, step 7 | not sized | not done; a corner is a diagnostic |
| Pattern along a closed path | 15.6 | not sized | refused |

### STEP

| Item | From | Effort | State |
|---|---|---|---|
| Export: knots removed where patches join smoothly, faces whose patches make no grid as one surface | 21.5, step 11 | 1.5 wk | not done; the export reads back exact as it is. OpenCASCADE's volume of a face written at full multiplicity is off by up to 0.3% |
| Surfaces and curves of degree above 5 | 21.2, step 11 | not sized | refused, so the file links as a mesh; needs degree reduction or approximation |
| One face's edge along another face's seam | 21.3 | not sized | NIST_MBE_PMI_5 and _6 fail to close for it and link as meshes |
| Edges further from their faces than the file's tolerance | 21.3 | not sized | Schenkel and the OpenCASCADE screw leave mesh gaps or crossings and link as meshes; 5 of the 9 vendor files read as sound solids |
| Offset surfaces and conics (parabola, hyperbola) as solids | 21.2 | not sized | refused; the file links as a mesh |

### Drawings and small features

| Item | From | Effort | State |
|---|---|---|---|
| The drawing sheet shown in the window | 18.1 | not sized | written to files only |
| Associative dimensions on section, detail and isometric views, and of faces | 18.4 | not sized | edges only, on the orthographic views |
| Nested links in the bill of materials | 16.7 | not sized | a link inside a linked file is not counted |
| Auto-constrain while drawing arcs and other entities | 16.11 | not sized | lines only |

### Scripting and the MCP server

From two notes of 2026-10-03, written while drawing a teapot through the MCP server. None was
checked again since, but the first, which step 20's example scripts met again. Suggested order:
the silent no-solid first, then the rollback and the active group after a delete, then the
render fit, then the interpreter, then the kernel's two.

| Item | Effort | State |
|---|---|---|
| A float literal beside a library constant fails: `2.0 * math.Pi`, `math.Pi + 0.5` give "mismatched types untyped float and untyped float64"; `math.Pi / 180.0`, `12 * math.Pi`, a variable and `float64(math.Pi)` work | not sized | Read from the error, not traced: yaegi v0.16.1's stdlib tables make `math`'s constants "untyped float64". Ways to weigh: a newer yaegi; our own symbols for `math`'s untyped constants; else rewrite the error in `pkg/script/engine.go` ("write 2 * math.Pi or float64(math.Pi)") and say it in the primer's "What the interpreter can't do". Tests: a case per failing form in `engine_test.go` |
| `eval` of code that starts with `const` or `var` and goes on with a statement fails to parse ("expected declaration") | not sized | Taken for a program's top level; check `run_script` too |
| A sketch that crosses itself makes no solid and nothing says so | not sized | The status, `get_drawing`, `Group.Problem()` and `Check().Sound()` (true, with no faces) all say well; only `PolyError` knows. Wanted: a sketch's `PolyError` (open, not coplanar, crossing, zero length) with its point in `Problem()`, `get_drawing` and the status; a solid group whose source has one says why it made no solid; `Sound()` false for a solid group with no solid |
| A failed script that called `TangentArc` is not all taken back | not sized | The arc and its trimmed lines stayed though the message said everything was taken back; guess: `TangentArc` pushes an undo step of its own inside the script |
| Deleting the active group puts later new groups after the first sketch; `MoveToEnd` moved an extrusion but not its two sketch groups, silently | not sized | Wanted: the last remaining group active after such a delete, or the script told which; `MoveToEnd` moves or says why not |
| `render` with no view (isometric) or "front" doesn't fit the model: zoomed in and pushed down, at every picture size | not sized | "window" after Zoom To Fit is right. Entry: `Doc.Render`, `pkg/script/api/doc.go` |
| yaegi: `a, b, c := s[0], s[1], s[2]` fails as "constant definition loop" | not sized | One per line works; the smallest failing form not found. Mend, or put it in the primer |
| yaegi: names kept between `run_script` calls collide with a block's `:=` of another type (panic in `reflect.Set`), and `p, q := e.Ends()` in a loop left `p` undefined | not sized | Wanted: a block's `:=` shadows as in Go; the primer says whether a new file or MCP session clears the names |
| A union fails where an extruded band's arcs cross a lathed cone at a shallow angle, over the lathe's seam | not sized | A line 0.76 or 1.0 mm inside a 3 mm wall, arcs of 6 to 8 mm; the angle alone doesn't separate failing from working. `Problem()` was "" though the Boolean failed |
| A cut whose end cap lies in a lathed end face's plane (a disc of four patches with a pole) leaves naked edges | not sized | A bore along a lathed spout's axis ending at its tip: 32 naked edges, not flagged; 3 mm past the tip is sound, and the same with a swept spout was sound |
| A fillet where a flat band meets a cone is refused unhelpfully | in 22.2 | The refusal could say that the edge is a flat face meeting a cone off its axis |
| Texts: `fmt.Println(g.Bounds())` fails plainly only in Go's words; a `Radius` constraint reads `c085-diameter = 6` with value 6 the radius | not sized | The constraint's name and value disagree |
| A two-sided extrusion from scripts | not sized | The group has the subtype; `Doc.Extrude` goes one way only |
| `render` "window" right after Align View to Workplane catches the view mid-animation | not sized | Wait for the animation or finish it |
| `doc.RunCommand("File > New")` on unsaved changes returned nil and discarded them, as far as a script could see | not sized | Check a script or the MCP server can't drop unsaved work without the question (`OkayToStartNewFile`) |
| A group's volume changes when later groups make the model larger | | Expected, the chord tolerance being a share of the model's size; worth a line in the guide |

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
