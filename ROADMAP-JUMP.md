# Roadmap: from a SolveSpace port to a modern CAD

Date: 2026-10-01, compacted 2026-10-04 twice. Status: steps 1 to 29 and I7 are merged. Steps
23 and 29 are complete; steps 22 and 24 to 28 are merged with the follow-ups listed under "Open
work", all of them kernel work. 12.4 (CI's first run on GitHub) waits for the user. Read
"Definition of done for every new feature" first.

TeleSAPR began as a Go port of SolveSpace, bit-identical with the C++. Since 2026-10-01 it is a
separate project: no parity or file compatibility with SolveSpace, a Go-native kernel, its own
`.tlspr` format (`docs/tlspr-format.md`), and `.slvs` files read by File > Import only.

This document was compacted on 2026-10-04, twice. The full plan to step 22, with each step's
packages, the estimates by area, the review of 2026-10-03 and the design notes, is this file at
commit `2ec37e4`; the plan of steps 23 to 29, with each package's merge, is this file at
`7a32dca`. Comments in the code that cite a package (as "package 5.1" or "roadmap 24.2") refer
to those versions. User-facing documentation of what was built is in `docs/guide`, `docs/ai.md`
and `docs/done-audit.md`; the kernel's working notes are in `docs/notes`.

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
| 21 | STEP import as an exact solid in Booleans, tessellated STEP | `21abd88` |
| 22 | Offsets of patches (22.1), coaxial cylinder/cone fillets and chamfers (22.2's first case); the rest of 22 is steps 25 and 26 | `ddf8144` |
| 23 | Scripting and MCP fixes: contour problems reported on a sketch and its solid, one undo step per script, the active group after a delete and moves with sources, renders that fit, the interpreter's forms, two-sided groups from scripts, no silent File > New | `d92c5f8` |
| 24 | Booleans on curved surfaces: no edges inside a planar cut, trims along apexes and poles, false intersection starts dropped, grazing ears avoided, no fans on patches ruled one way; cylinder through cylinder 2 to 0 of 54, analytic STEP cuts 10 to 0 of 28, bands over a lathe seam 14 to 0 of 108, twisted lofts meshed within 0.55%; failed Booleans named in `Problem()`; hover in perspective on the grid; crossings counted where an edge meets a triangle | `c4137f3`, `2fcfbae`, `7b3b32a` |
| 25 | Rolling-ball fillets and chamfers between two curved faces (22.2): closed edges (tees of cylinders, the teapot's spout, a rod through a ball), chains ending on a flat face; within 0.5% of OpenCASCADE; sweep 2 of 108 | `8707d1f`, `add3da7` |
| 26 | General fillets (22.3 to 22.5) in part: corner-filling blends that run off onto a flat face; corners of different radii and convex meeting concave in stages; setback corner patches where three or more fillets meet (G1 within about 1e-4 rad); chains between flat faces ending on a curved face; fillets on lofts, sweeps and STEP solids 32 to 44 of 59 sound, volumes against OpenCASCADE; chamfers where a slot meets a cone | `add3da7` to `31e0672` |
| 27 | STEP: knots removed on export where patches join smoothly, edges along another face's seam, degree above 5 rewritten at 5, offset surfaces and conics as solids, poles opened in uv rather than dropped; corpus 5 to 7 of 9 sound | `adc3224`, `ed03d3b`, `7b3b32a` |
| 28 | Chamfers of two distances; drafts of chosen sides about a neutral plane, of an arc or two arcs meeting at a corner; sweeps with mitred corners and along named edges; patterns along a closed path; lofts with end tangency and one or two guide curves | `359cc18`, `fd137dc` |
| 29 | Drawing sheet shown in the window, associative dimensions on isometric, section and detail views and of faces, nested links in the bill of materials, auto-constrain for arcs and circles | `9fcca23` |

Decisions taken along the way: the unit belongs to the document (format 2); files broken by
hand get an error on load and no other safeguard; files with the old `spacesolve` header are
not read.

## Open work and follow-ups

Every item left is kernel work, unsized. A two-day spike on the hardest fixture comes before
any of them is planned as a step. The notes in `docs/notes` hold the attempts that failed and
why.

### Waiting for the user

| Item | From | State |
|---|---|---|
| The remote and CI's first run | 12.4 | The GitHub repository (organisation `telesapr`, for the module path) and the `TAKIGO_TOKEN` secret; then push, watch the workflow and fix what its runner shows. Until then no fresh clone has been shown to build. Effort 0.3 wk |
| What to take back from "Left out" | | Every row there is a proposal. F5 now blocks two items below |
| What the review took in without being asked | | B3 (16.11), G4 (17.4) and S3 (17.5): all done; the user may still disown them |

### Fillets (steps 25 and 26; notes `docs/notes/step-25-rolling-ball.md`, `step-26-blends.md`)

| Item | State |
|---|---|
| Run-offs onto a curved face, and of blends that take material away | refused with the point; the setback patch is the likely means |
| A mitre of two radii over an edge left sharp | refused, naming the radii: the blend would need a crease |
| A rib rounded all over: its top's edges rounded as well as its foot filled | refused. A foot filled all round below sharp edges of the top is made (`fillet-mixed-corners`: the uneven mitre outside the rib). With the top's edges rounded first, the foot's chain meets the arc where the floor cuts a leaning cylinder, which only the rolling ball rounds, and that edge must be picked alone; with the foot first, the top's edge ends on the foot's fillet and the grown top |
| Chamfers of edges between flat faces that end on a curved face | refused; routed like the fillets they left 134 naked edges |
| The loft's and the blob's edges in the 22.5 measurement | refused, for reasons in the notes; 44 of 59 sound |
| Ends on a dome that one degree-5 curve can't follow | the curved-end sweep refuses 8 of 40, radii 1.5 to 2.5 |
| The run-off sweep's last failures | 4 of 36: 2 unfittable, 2 chamfers of 3 crossing the cone by one edge |
| A chamfer of 3 where a slot meets a cone | 6 naked edges where it reaches the slot's floor (sound to 2.75) |
| A band-on-cone fillet at r 1 and 0.001% | 2 crossing mesh edges where it meets the cone tangentially; not checked again since the mesher fix of `7b3b32a` |
| The teapot's handle | its union is unsound before any fillet |
| Sweep allowances set after the first run | the 6:1 setback sweep (12%), the run-off sweep, the curved-end sweep; the rule wants them fixed before. Fix them now and keep them |

### Booleans and meshing (step 24; notes `docs/notes/step-24-spike.md`)

| Item | State |
|---|---|
| A bore whose cap lies in a lathed tip's plane, off axis | fails 40 of 80: coincident contact (F5) |
| A wedge on a plate, its sloping top's foot in the plate's top face, its width leaning back along the foot | the union fails for every lean that way and for none the other way (`TestFilletRibFootSweep` keeps to the one that works): an edge of one solid in a face of the other, likely F5 |
| A twisted swept bar | its flat sides mesh within 1.6%. Grid columns bring it to 0.33% but a bar cell and a thread flank are twisted alike, and columns on both moved thread volumes the wrong way. Untried: fan diagonals alternated so the bias cancels |
| NIST_MBE_PMI_6 as a solid | the topology problem at its poles: the mesh's Euler number 6 off the shell's, where earlier ears used up every other ear |
| Schenkel as a solid | no naked edges since `ed03d3b`, but its shell's Euler number is -34 against the mesh's -6, cause not found; corpus 7 of 9 |

### Solid features (step 28; notes `docs/notes/step-28.md`)

| Item | State |
|---|---|
| Sweep corners next to an arc or curve, on twisted or fitted sweeps | refused; corners between two lines are mitred. Two runs joined by a union fail where they touch, for corners between lines too: coincident contact (F5). The joint must be built in one shell, with surface intersection inside the sweep builder |
| Shells of other curved solids | prisms only; filleted, lofted and revolved solids are refused. With E12 under "Left out"; 22.1's offsets are the first part |

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
