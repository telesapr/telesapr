# Audit against the definition of done

Roadmap package 12.8, made on 2026-10-03 on branch `step-12-close`. Each feature that steps 1 to
11 and I7 added is a row; each column is a requirement of "Definition of done for every new
feature" below. A cell names what meets the requirement, or says why it does not apply ("n/a"
and the reason). No cell is empty.

## Definition of done for every new feature

Requirement set by the user on 2026-10-01: a new feature is well tested, has a user interface,
and has scripting support. A feature that lacks one of the three is not done and does not merge.
The three paths, interface, script and file, must reach the same engine entry point in
`pkg/sketch`, so none of them can drift from the others. The conventions of the code itself, and
the environment variables the tests read, are in `AGENTS.md`.

The order inside a feature is: entry point in `pkg/sketch`, save and load, interface, script API,
path equivalence test. Document state a feature adds goes into `UndoState` in
`pkg/sketch/undo.go`, or it is lost on undo.

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

## The audit

Where things are:

- **Scenario**: an interface script in `testdata/ui/scripts`, with its golden in
  `testdata/ui/golden/<name>.goui` (`TestGoScripts`).
- **Path pair**: `testdata/ui/scripts/path_<name>.script` and `.go`, compared by
  `pkg/ui/pathequiv_test.go`.
- **Example**: `examples/scripts/<name>`, each run by `TestExamplesRun` in `cmd/telesapr-cli`.
- **Golden**: files compared byte for byte, regenerated only by `scripts/regen-golden.sh` and
  `scripts/regen-goui.sh`. `testdata/solid/*.tlspr` are `TestSolidGoldens` in `pkg/sketch`.
- **Sweep**: a `sweep.Grid` test; its allowance and what the whole grid gave are in the test.
  Parsers have fuzz targets instead (`make fuzz`).
- **Benchmark**: a `Benchmark*` function on the regeneration path.
- **Undo**: a test that the feature's action is one undo step that redo restores; in
  `pkg/sketch` the `undoRedo` helper compares the saved files.
- **Round trip**: save, open, save gives the same bytes; in `pkg/sketch` the `roundTrip` helper.
  `TestEveryFieldRoundTrips` in `pkg/persist/tlspr` round-trips a document that sets every field
  of the format.

| Feature (step) | Scenario | Path pair | Example | Golden | Sweep | Benchmark | Undo | Round trip |
|---|---|---|---|---|---|---|---|---|
| Screw thread (1.9) | `ext_thread` | `path_thread` | `bolt` | `testdata/thread/*.tlspr` | `TestThreadSweep`, 0 of 400 | `BenchmarkBuildThread` | `ext_thread` (undo takes the thread away at once) | `TestThreadGoldens` (save, open, save) |
| Check Solid (1.2 to 1.4) | `ext_check_solid` | n/a: changes nothing in the file | every solid example calls `Check` | `testdata/invariants.txt` (`TestInvariants`) | n/a: it is the oracle the sweeps use | n/a: not on the regeneration path | n/a: changes nothing | n/a: saves nothing |
| The `.tlspr` format (2.1 to 2.5) | `ext_save_open` | n/a: both paths save through `tlspr.Encode`, which every pair compares | every example that writes saves `.tlspr` | `testdata/tlspr/v1`, `v2` (frozen), `testdata/tlspr/golden` | `FuzzRead` | n/a: reading is not regeneration; `BenchmarkRegenerateLargest` loads first | n/a: opening makes a new document | `TestEveryFieldRoundTrips`, `TestFixturesRoundTrip`, `TestParsePrintRoundTrip` |
| `.slvs` import (2.6 to 2.8) | `ext_import_slvs` | n/a: the interface and `api.Import` both call `convert.Run`, whose output is the golden | n/a: it needs a SolveSpace file beside it; `telesapr-cli convert` is the command-line path | `testdata/tlspr/golden/*.tlspr`, one per fixture | `FuzzLoad` | n/a: once per file | n/a: makes new files | `TestFixturesRoundTrip` (import, save, open, save) |
| Parameters (3.1 to 3.4) | `ext_parameters`, `ext_parameter_copies` | `path_parameters` | `plate`, `edges` | `ext_parameters.goui`, `testdata/solid/chamfer-parameter.tlspr` | `FuzzExpr`; no solid of its own | n/a: a few expressions per regeneration, inside every `BenchmarkRegenerate*` | `TestParameterUndo`, `TestUndoRememberFor` | `TestEveryFieldRoundTrips`, `TestExpressions` |
| Freedom colouring (3.5) | `ext_freedom` | n/a: shown, not saved | `feedback` (`Entity.Free`) | `ext_freedom.goui` | n/a: no solid | `BenchmarkGroupFreedom`, `BenchmarkFreedom150` | n/a: changes nothing | n/a: not saved |
| Conflicts (3.6) | `ext_conflicts` | n/a: shown, not saved | `feedback` (`Group.Conflicts`) | `ext_conflicts.goui` | n/a: no solid | n/a: found by the solve that fails | n/a: changes nothing | n/a: not saved |
| Command search (3.7) | `ext_find_command` | n/a: it runs other commands, whose own rows apply | `feedback` (`Commands`, `RunCommand`) | `ext_find_command.goui` | n/a | n/a: not on the regeneration path | n/a: the command run makes its own step | n/a: not saved |
| Measure (3.8) | `ext_measure` | n/a: adds nothing to the file | `feedback` (`Doc.Measure`) | `ext_measure.goui` | n/a: no solid | n/a: not on the regeneration path | n/a: changes nothing | n/a: not saved |
| Document units (3.9) | `ext_units` | `path_units` (added by 12.8) | `feedback` (`Doc.SetUnits`) | `ext_units.goui` | n/a: no solid | n/a: not on the regeneration path | `ext_units` (a change of unit is one step) | `TestEveryFieldRoundTrips`, `TestUnitsInFiles` |
| Move a group (4.2) | `ext_move_group` | `path_move_group` | `history` | `ext_move_group.goui` | n/a: no solid of its own | n/a: renumbers, then the usual regeneration | `ext_move_group`, `TestMoveGroup` (api) | `TestEveryFieldRoundTrips` (the order) |
| Suppress whole (4.3) | `ext_suppress_all` | `path_suppress_all` | `history`, `step` | `ext_suppress_all.goui` | n/a: no solid of its own | n/a: skips work | `TestSuppressAllUndo`, `ext_suppress_all` | `TestEveryFieldRoundTrips` |
| Broken references (4.4) | `ext_broken_references` | `path_delete_group` | `history` | `ext_broken_references.goui` | n/a: no solid of its own | n/a: a report from pruning | `ext_broken_references` | n/a: a report, not saved |
| OpenGL renderer (6.1 to 6.4) | `ext_renderer` | n/a: a setting of the window, not of the file | n/a: scripts draw in no window; the setting is the configuration screen's "draw with OpenGL" box and `--renderer` | `TestGLWindowRendersFace` (read-back pixels) | n/a: no solid | n/a: frame times under Xvfb are Mesa's, not the code's; `BenchmarkFrame` times the software path | n/a: a setting | n/a: not saved |
| Section view (6.5) | `ext_section` | `path_section` | `section` | `ext_section.goui` | n/a: cuts a mesh for display | n/a: drawn, not regenerated; `BenchmarkPaint*` | n/a: view state, by decision ("Left out") | n/a: not saved, by decision ("Left out") |
| Mirror (7.1, 7.2) | `ext_mirror` | `path_mirror` | `mirror` | `testdata/solid/mirror-*.tlspr` | `TestMirrorSweep`, 0 of 144 (added by 12.8) | `BenchmarkMirror` | `TestMirrorArcRoundTripUndo` | `TestMirrorArcRoundTripUndo` |
| Chamfer (7.3, 9.6) | `ext_chamfer`, `ext_chamfer_solid_edge` | `path_chamfer`, `path_chamfer_solid_edge` | `chamfer`, `edges`, `knob`, `spinner` | `testdata/solid/chamfer-*.tlspr` | `TestChamferSweep`, `TestNamingSweep` | `BenchmarkChamfer` | `TestChamferOneEdge` | `TestChamferOneEdge` |
| Chamfer of two distances (28.1) | `ext_chamfer_two` | `path_chamfer_two` | `bevel` | `testdata/solid/chamfer-two-distances*.tlspr` | `TestChamferTwoDistancesSweep` (no failures allowed) | `BenchmarkChamferTwo` | `TestChamferTwoDistances` | `TestChamferTwoDistances` |
| Pattern round a closed path (28.5) | `ext_pattern_ring` | `path_pattern_ring` | `flange` | `testdata/solid/pattern-ring.tlspr` | `TestPathPatternClosedSweep` (no failures allowed) | `BenchmarkPathPatternRing` | `TestPathPatternCircle` | `TestPathPatternCircle` |
| Draft of chosen sides, neutral plane, sharp arc corners (28.2) | `ext_draft_sides` | `path_draft_sides` | `parting` | `testdata/solid/draft-sides.tlspr`, `draft-sharp-arc.tlspr`, `draft-lens.tlspr` | `TestDraftSidesSweep`, `TestDraftAnglesSweep`, `TestDraftSharpArcsSweep` (none allowed) | `BenchmarkDraftSharpArc` | `TestDraftChosenSides`, `TestDraftNeutral`, `TestDraftSharpArcs` | `TestDraftChosenSides`, `TestDraftSharpArcs` |
| Sweep with corners and along solid edges (28.4) | `ext_sweep_corner` | `path_sweep_corner` | `bead` | `testdata/solid/sweep-corner.tlspr`, `sweep-solid-edges.tlspr` | `TestSweepCornerSweep` (none allowed) | `BenchmarkSweepCorners` | `TestSweepCorner`, `TestSweepSolidEdges` | `TestSweepCorner`, `TestSweepSolidEdges` |
| Loft tangency at the end profiles (28.3, in part) | `ext_loft_tangency` | `path_loft_tangency` | `nozzle` | `testdata/solid/loft-tangency.tlspr` | `TestSmoothLoftEndsSweep` (none allowed) | `BenchmarkLoftTangency` | `TestLoftTangency` | `TestLoftTangency` |
| Loft guide curves (28.3) | `ext_loft_guides` | `path_loft_guides` | `horn` | `testdata/solid/loft-guides.tlspr` | `TestGuidedLoftSweep` (none allowed) | `BenchmarkLoftGuides` | `TestLoftGuide` | `TestLoftGuide` |
| Sweep along a path (7.4) | `ext_sweep` | `path_sweep` | `sweep` | `testdata/solid/sweep-*.tlspr` | `TestSweepGrid`, 0 of 72 | `BenchmarkSweep` | `TestSweepPipe` | `TestSweepPipe` |
| STEP link (8.1 to 8.5) | `ext_link_step` | `path_link_step` | `step` | `ext_link_step.goui`, `testdata/step` | `FuzzStep` | `BenchmarkReadNIST5` | `TestLinkStepUndo` (added by 12.8) | `TestEveryFieldRoundTrips` (the link's path); the mesh is read again |
| Edge names and picking (9.1, 9.3 to 9.5) | `ext_chamfer_solid_edge`, `ext_fillet_concave` | `path_chamfer_solid_edge`, `path_fillet_rim` | `edges` | `testdata/topo/*.tlspr`, `testdata/solid/*-solid-edge.tlspr` | `TestNamingSweep` | inside `BenchmarkChamfer` and `BenchmarkFillet*`, which derive the topology | n/a: names are kept in the chamfer's and fillet's groups, whose rows apply | `TestEveryFieldRoundTrips` (solid edges), the solid goldens |
| Fillet (10.1 to 10.3, 10.6 to 10.10) | `ext_fillet`, `ext_fillet_circle`, `ext_fillet_concave`, `ext_fillet_cone`, `ext_fillet_outline`, `ext_fillet_radii`, `ext_fillet_uneven` | `path_fillet`, `path_fillet_circle`, `path_fillet_concave`, `path_fillet_cone`, `path_fillet_mitre`, `path_fillet_outline`, `path_fillet_radii`, `path_fillet_rim`, `path_fillet_uneven` | `fillet`, `knob`, `plinth`, `ramp`, `roundbox`, `soap`, `spinner`, `tray` | `testdata/solid/fillet-*.tlspr` | the sweeps of `pkg/topo/fillet*_test.go` and `pkg/sketch/fillet_test.go`, all at 0 | `BenchmarkFillet*`, `BenchmarkRegenerateFillet` | `TestFilletOneEdge` and seven more `TestFillet*` | the same tests |
| Fillet between curved faces, rolling ball (22.2) | the fillet's scenarios: the same menu, screen and picking | n/a: the fillet's path pairs, through the same `FilletPicks` | `tee` | `testdata/solid/fillet-tee.tlspr` | `TestRollingBallSweep`, 2 of 108 (10% fixed before) | `BenchmarkFilletTee` | `TestFilletBetweenCylinders` | `TestFilletBetweenCylinders`, `TestSolidGoldens` |
| Fillet and chamfer between curved faces ending at flat faces (22.2) | the fillet's and chamfer's scenarios: the same menus, screens and picking | n/a: through the same `FilletPicks` | `bandfillet` | n/a: the fillet's goldens | `TestRollingBallOpenEdge` (exact volumes, square and leaning ends) | `BenchmarkFilletTee` | `TestFilletFlatOnConeOffAxis` | `TestFilletFlatOnConeOffAxis` |
| Fillet and chamfer ends that run off onto the next flat face (22.3, filling a corner) | the fillet's and chamfer's scenarios: the same menus, screens and picking | n/a: through the same `FilletPicks` | `bandfillet` (its chamfer of 2) | n/a | `TestRunOffSweep`, 4 of 36 (2 refused, 2 chamfers whose meshes cross at the cone: the mesher's; set after the first run, fixed at 4 on 2026-10-10) | `BenchmarkFilletTee` | `TestChamferRunsOffBand` (save, load, undo) | `TestChamferRunsOffBand` |
| Fillets on lofts, sweeps and STEP solids (22.5, measured) | the fillet's scenarios | n/a: through the same `FilletPicks` | `bandfillet`, `tee` | n/a | `TestFilletOnOtherSolids`, 44 of 59 edges, smooth edges left out (counts may only rise); `TestFilletOnOtherSolidsVolumes` against exact values and OpenCASCADE | `BenchmarkFilletTee` | `TestFilletStepCylinder` | n/a |
| Fillets of edges between flat faces that end on a curved face (22.5) | the fillet's scenarios: the same menu, screen and picking | n/a: through the same `FilletPicks` | n/a: the lid file's edges in the tests | n/a | `TestFilletEndsOnCurvedFaceSweep`, 8 of 40 refused (0 fixed before, missed: a quintic end can't follow the dome; fixed at 8 on 2026-10-10), none unsound; `TestFilletEndsOnCurvedFace` against OpenCASCADE within 0.5% | `BenchmarkFilletTee` | `TestFilletEndsOnCurvedFace` | `TestFilletEndsOnCurvedFace` (save, load, undo) |
| Fillet corners of different radii, or convex and concave, in stages (22.4) | the fillet's scenarios: the same menu, screen and "[own radius]" links (`ext_fillet_radii`) | n/a: through the same `FilletPicks` (`path_fillet_radii`) | `casing` | n/a: the fillet's goldens | `TestFilletStagesSweep`, 0 of 18 (0 fixed before) | `BenchmarkFillet*` (one stage unless a corner is mixed) | `TestFilletStagesScript` | `TestFilletStagesScript` |
| Fillets that meet below an edge of the other kind left sharp, as round a rib's foot (22.4) | the fillet's scenarios: the same menu, screen and picking | n/a: through the same `FilletPicks` | `rib` | `testdata/solid/fillet-rib.tlspr` | `TestFilletRibFootSweep`, 0 of 108 (0 fixed before), volumes against the height summed over the plate; `TestFilletGrownFaceCut` | `BenchmarkFilletRib` | `TestFilletRibFoot` | `TestFilletRibFoot`, `TestSolidGoldens` |
| Fillet corners set back where radii differ or more than three edges meet (22.4) | the fillet's scenarios: the same menu, screen and "[own radius]" links (`ext_fillet_radii`) | n/a: through the same `FilletPicks` (`path_fillet_radii`) | `setback` | n/a: the fillet's goldens | `TestFilletSetbackSweep`, 0 of 36 up to 3 to 1 (0 fixed before); `TestFilletSetbackExtremeSweep`, 4 of 36 refused up to 6 to 1 (set after the first run, fixed at 4 on 2026-10-10) | `BenchmarkFillet*` (no setback unless a corner needs one) | `TestFilletStagesScript` | `TestFilletStagesScript` |
| Shell (10.4) | `ext_shell` | `path_shell` | `shell` | `testdata/solid/shell-*.tlspr` | `pkg/sketch/shell_test.go`, `pkg/topo/shell_test.go`, at 0 | `BenchmarkShell` | `TestShellOpenBox` | `TestShellOpenBox` |
| Sketch on a face (F1) | `ext_sketch_on_face`, `ext_sketch_on_face_context` | `path_sketch_on_face` | `pedestal` | `testdata/solid/sketch-on-face.tlspr`, `sketch-on-chamfer-face.tlspr`, `testdata/tlspr/v2/sketch_on_face.tlspr` | `TestSketchOnFaceSweep`, 0 of 48 (0 fixed before) | `BenchmarkSketchOnFace`, `BenchmarkSketchOnFaceMoved` | `TestSketchOnFaceFollowsFace` | `TestSketchOnFaceFollowsFace`, `TestEveryFieldRoundTrips` (`at`) |
| Minimum and maximum dimensions, point on a segment (F2; any dimension 2026-10-10) | `ext_distance_bounds`, `ext_dimension_bounds` | `path_distance_bounds` | `limits` | `testdata/tlspr/v2/limits.tlspr`; `ext_distance_bounds.goui` | `TestDistanceBoundsSweep`, 0 of 56 (0 fixed before) | `BenchmarkDistanceBounds` | `TestDistanceBoundsSavedAndUndone` | `TestDistanceBoundsSavedAndUndone`, `TestCorpus` and `TestDistanceBoundsOldFile` (`limits.tlspr`) |
| Ray-cast check (K2) | `ext_ray_check` | n/a: changes nothing in the file | `raycheck` | `ext_ray_check.goui` | n/a: it is an oracle; `TestOracleBooleans` and `TestMeshManifold` break meshes on purpose and it finds them | n/a: an analysis, not a regeneration | n/a: changes nothing | n/a: changes nothing |
| Ruled loft (10.5) | `ext_loft` | `path_loft` | `loft` | `testdata/solid/loft-*.tlspr` | `pkg/sketch/loft_test.go`, at 0 | `BenchmarkLoft` | `TestLoftFrustum` | `TestLoftFrustum` |
| General surfaces (11.1 to 11.4) | n/a: no user surface yet; faces of several patches are seen through the features that make them | n/a: as for the scenario | n/a: as for the scenario | the STEP exports in `testdata/export` | n/a: `TestBSpline*` in `pkg/topo` and `pkg/srf` check exactly; 21.x and 22.x bring the features to sweep | `BenchmarkBSplineLid` | n/a: no action of its own | n/a: not saved; the patches are regenerated |
| AI connections (I7) | `ext_ai_connections` | n/a: the MCP tools run scripts, which are the script path | `bracket`, driven through MCP by `TestMCPBracket` in `cmd/telesapr-cli`; `docs/ai.md` | `ext_ai_connections.goui`, `testdata/mcp` | n/a: no solid | n/a: not on the regeneration path | `TestSession*` in `pkg/script/agent` (a script run is one step), `pkg/ui/agent_test.go` | n/a: the scripts' own rows apply |
| Nested bill of materials (29.3) | `ext_bill_of_materials` (the screen; the nested counts are `TestBillOfMaterialsNested`) | n/a: counts, adds nothing to the file | `bom` (a bracket linked twice) | `ext_bill_of_materials.goui` | n/a: no solid | n/a: a linked model's file is read when the links load, not at each regeneration | `TestBillOfMaterialsNested` (the bill after undo) | n/a: not saved, filled when the links load |
| Auto-constrain arcs and circles (29.4) | `ext_auto_constrain_arcs` | `path_auto_constrain` | `autoconstrain` | `ext_auto_constrain_arcs.goui` | n/a: no solid | n/a: while drawing, not on the regeneration path | `TestAutoConstrain` (api) | n/a: ordinary constraints, `TestEveryFieldRoundTrips` |
| Drawing dimensions on more views and of faces (29.2) | `ext_drawing_faces` | `path_drawing_faces` | `drawing` (faces, the isometric) | `ext_drawing_faces.goui` | n/a: measures, makes no solid | n/a: not on the regeneration path; measured when the sheet is made | `ext_drawing_faces` (each dimension one step, as 18.4's) | `TestDrawingDimensionsViewsFaces` (faces and letters saved and read back), `TestEveryFieldRoundTrips` |
| Drawing sheet in the window (29.1) | `ext_drawing_show` | n/a: view state, not saved; `TestShowDrawingSheet` drives the menu and the script call alike | `drawing` (`sheet.png`) | `ext_drawing_show.goui` | n/a: no solid | n/a: the sheet is made again only when the model or drawing changes (cached by generation and drawing) | n/a: changes nothing in the document | n/a: not saved |
| Menus in the order of the work, View > Show, former labels (UX) | `menu_states`, `menu_newfile` (the menus), `ext_find_command` | n/a: the same commands; `TestFormerCommandLabels` runs the old labels | `feedback` (`RunCommand`) | the `.goui` menu dumps | n/a: no solid | n/a: not on the regeneration path | n/a: changes nothing; the commands run make their own steps | n/a: not saved |
| Greyed entries, idle keys, search reasons (UX) | the `.goui` menu dumps' `en=` flags | n/a: shown, not saved | n/a: `RunCommand` says why (`TestMenuEnablement`) | `menu_states` | n/a | n/a | n/a: changes nothing | n/a |
| Context menu for edges, faces, dimensions, groups (UX) | `ext_context_edge`, `act_context_menu` | n/a: each entry runs its menu entry's handler, whose pair applies | the features' own | `ext_context_edge.goui` | n/a: the features' sweeps | n/a | `TestContextMenuSolid` (suppress undone) | the features' own |
| Spline points and coincidences (UX) | `TestContextMenuSpline` | n/a: the position comes from the pointer, the script's from the nearest span | `spline` | n/a: no solid | n/a | n/a | `TestSplinePoints`, `TestContextMenuSpline` | n/a: a sketch edit, saved as any |
| Toolbar from its table (UX) | the hover grids beside it | n/a: buttons run menu commands | n/a: the window's | `testdata/draw` (the axes beside it) | n/a | n/a | n/a | n/a |
| Breadcrumbs and a group's actions (UX) | every `.goui` with the property browser | n/a: the actions are the group list's | n/a: the window's | `testdata/textwin`, `.goui` | n/a | n/a | n/a: the actions' own | n/a |
| Copy, paste, length, areas, centre of mass from scripts (UX) | the Edit and Analyze menus' scenarios | n/a: one sketch call each, `TestEveryMenuCommandHasAMethod` | `measure` | n/a | n/a | n/a | `TestCopyPaste` | n/a: pastes are sketch edits |

## What the audit changed

- `examples/scripts/feedback`: the first example to call `Doc.SetUnits`, `Doc.Measure`,
  `Entity.Free`, `Group.Conflicts`, `Commands` and `Doc.RunCommand`. The doc comment of
  `RunCommand` was wrong: `telesapr-cli run` runs scripts on the interface without a window,
  where commands run.
- `path_units`: the unit set through the menu and through `Doc.SetUnits` saves the same file.
- `TestMirrorSweep`: 0 of 144. A mirror reflects its source group, not the solid so far, so a
  hole cut by a later group is not reflected (15.5); the sweep puts the hole in the profile.
- `TestLinkStepUndo`: linking a STEP file is one undo step.
