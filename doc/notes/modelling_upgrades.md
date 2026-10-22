# TeleSAPR upgrade list

Generic improvements to TeleSAPR. Size is a rough effort estimate under TeleSAPR's definition of
done (test, UI, scripting method, example, goldens). The list was checked against the code on
2026-10-22: "Exists" says what is built already, "Missing" what the item still asks for.

## The items

| # | Improvement | Exists | Missing | Size |
|---|---|---|---|---|
| 1 | **Wrap or emboss a sketch onto cylinders and cones** | Flat text (`core.EntityTTFText`, `srf.OutlineInto`); sketch on a flat face (`sketch.NewSketchOnFace`); projection onto a surface (`srf.ProjectFrom`); exact helix patches (`srf/helix_rise.go`) | The mapping from a tangent plane onto the face, the sketch on a curved face, raised and sunk bands | L |
| 2 | **Named datums** (points, workplanes, faces with persistent names) | Group names, named dimensions (`sketch/dimname.go`), derived edge and face names (`topo.Name`, `srf.FaceID`), `Solid.Face`, `Edge`, `FaceAt` in scripts | Names chosen by the user on points, faces and axes; names seen through a link; mates by name | M |
| 3 | **Parametric links and relink** | `Group.LinkFile`, link cache, configurations (`sketch/config.go`); relink designed in `feature-inputs-and-relink.md` | Relink; a configuration or parameter values passed into a linked part, with the cache keyed by them | M |
| 4 | **Thread as one parametric group** | `thread.Spec` (starts, hand), `Sketch.AddThread` writing four plain groups | The four groups held as one thread: pitch, length, clearance editable after creation; fade-out; rounded or truncated crests | M |
| 5 | **Script forms** | Script > Run Script, `script.RunFile` with arguments, `ParamsMenu` | A declaration of a script's inputs, the dialog, the values handed to the script and to MCP | M |
| 6 | **Script library imports** | Precompiled symbol tables only | A library folder read through yaegi's `GoPath` and `SourcecodeFilesystem` by console, `run` and MCP | S |
| 7 | **Shells of curved solids** | Flat solids (`topo.ShellSolid`), prisms of lines and arcs (`shellPrism`); `srf.OffsetSurface`, tested, used nowhere | Revolved and coned bodies, by offsetting the profile (exact); the general case is E12 | M, then L |
| 8 | **Sweeps along closed paths, lofts tangent to a face** | Loft end tangency (`topo.LoftEnds`, saved, in the window); guided lofts | Closed smooth sweeps (periodic frame, no caps); G1 to an adjacent face; closed lofts. Corners on a closed path stay refused (ROADMAP, solid feature leftovers) | M |
| 9 | **Fillets running onto curved faces** | Rolling ball for one picked edge ending on a curved face; arcs between a plane and a cylinder or cone; `TestFilletOnOtherSolids` 47 of 59 | Several picks with curved ends; run-offs; setback patches onto curved faces | M, open tail |
| 10 | **Exact features after a mesh body** | STL and OBJ links, mesh Booleans, a model switched to meshes from the first mesh group (`IsForcedToMesh`) | Fillet, shell, chamfer and thread refuse mesh groups. Only as an explicit approximate mode (ROADMAP, parked: mesh Boolean fallback) | hold |
| 11 | **Screw mate** | Concentric, flush, hinge; `MotionStudy` | Concentric plus angle and distance tied by `distance = angle·lead/360` | S, after 12 |
| 12 | **Expressions on more group values** | Fillet radius, chamfer, shell thickness, helix pitch, draft, sweep twist, link scale (`sketch/parameter.go`) | Extrude depth and revolve angle (solver parameters, not group values); thread length (with 4) | S |
| 13 | **Density per group** | Colour, transparency and appearance per group and per face | Density; mass, centre of mass and inertia per part; a mass column in the BOM | S |
| 14 | **Minimum wall and clearance analysis** | Interference of parts (`sketch/interference.go`), ray oracle (`check.Oracle.Probe`), `Sketch.Measure` | Minimum gap between parts; wall thickness | M |
| 15 | **Model to script export** | `Doc.Describe`; `.tlspr` is text | An emitter for every group and constraint kind, checked by a round trip over the fixtures | L |
| 16 | **More MCP tools and a filesystem allowlist** | `run_script`, `get_drawing`, `render`, `api_help`; export and measure reachable through `run_script`; scripts have the whole of `os` | Measure, section render, export, open and list as tools (S); an allowlist over every file-touching symbol and `api.File` path (M) | S + M |
| 17 | **Incremental regeneration** | Regeneration from the first dirty group, `ShellJob` off the window's thread, link and STEP caches | Recommendations 3, 6 and 9 of `performance-study-2.md` (same bytes); a cache per group keyed by its inputs | L |
| 18 | **glTF export** | STL, 3MF, OBJ, three.js, VRML (`pkg/export/mesh.go`) | A binary `.glb` writer with colours as materials | S |
| 19 | **Cosmetic thread convention on 2D drawings** | Drawings with hidden lines (`pkg/ui/drawing.go`) | Thread data on the group (from 4); root and crest lines and the three-quarter circle in place of the helix edges | S, after 4 |

Items 4, 19 and the allowlist of 16 are rows of ROADMAP "Parked" ("never asked for", "an MCP
server, and nothing else"). This list asks for them: each leaves "Parked" in the branch that
builds it.

## Order of work

Each work package is a branch from `devel`, merged back into `devel`. Foundations come first,
and small items early.

| Wave | Work package | Item | After |
|---|---|---|---|
| A | A1 extrude depth and revolve angle as group values | 12 | |
| A | A2 glTF export | 18 | |
| A | A3 density and mass properties | 13 | |
| A | A4 script library folder | 6 | |
| A | A5 screw mate | 11 | A1 |
| B | B1 thread as one group | 4 | A1 |
| B | B2 cosmetic threads in drawings | 19 | B1 |
| B | B3 relink, then parametric links | 3 | |
| B | B4 named datums | 2 | B3 |
| B | B5 script forms | 5 | A4 |
| C | C1 shells of revolved and coned bodies | 7 | |
| C | C2 wrap and emboss, cylinder then cone | 1 | |
| C | C3 fillets with curved ends, several picks | 9 | |
| C | C4 closed smooth sweeps, lofts tangent to a face | 8 | |
| D | D1 clearance and wall thickness | 14 | |
| D | D2 MCP tools, D3 file allowlist | 16 | |
| D | D4 incremental regeneration | 17 | |
| D | D5 model to script | 15 | |
| — | held: exact features after a mesh, general shell (E12) | 10, 7 | |

Waves A and B change the model and its tools; C extends the kernel and is measured by
`pkg/check` (watertight, no naked edges, closed-form volume) and an OCC check of the STEP; D is
analysis, tooling and speed.

## Notes per package

- **A1.** New `GroupValue`s that seed and hold the extrude translation (params 0–2) and the revolve
  angle (param 3), as `GroupPitch` does for the helix. A driven value must not show as a freedom.
- **A2.** `pkg/export/gltf.go`, modelled on `WriteThreeJS`; a filter in `pkg/platform/filters.go`.
- **A3.** `Group.Density`, saved; `Sketch.MassProperties` in `pkg/sketch/analyze.go` per solid
  group and per link part. After a Boolean, volumes are no longer per feature, so mass is per part.
- **A4.** In `newInterpreter` (`pkg/script/engine.go`); yaegi wants `src/<import path>` under
  `GoPath`. Library code has the same `os` access as a script (see D3).
- **A5.** A macro over existing constraints, not a new solver constraint (which would be nonlinear
  in the quaternion).
- **B1.** Keep the four groups; record `ThreadOptions` and `Spec` on the helix group and drive the
  generated dimensions from them, so one thread screen edits them all. Check how `AddThread` lays
  out several starts before adding the angular offset.
- **B2.** In `viewEdges`, leave out the thread's helix edges and draw the convention. Drawing
  goldens move in a commit of their own.
- **B3.** Relink as `feature-inputs-and-relink.md` designs it; then `LinkConfig` and `LinkParams`
  on the link group, a regeneration of the part per set of values, the cache key including them.
- **B4.** A name table on `Sketch` beside `dimname.go`, faces named by `srf.FaceID`; names carried
  through links with the remapped handles.
- **B5.** A top-level `var Form = api.Form{...}` read with `go/parser` before the run, as
  `isProgram` reads a program; the headless platform answers the dialog in UI scripts.
- **C1.** As `shellPrism` offsets a cap: offset the closed revolve profile inward, revolve it,
  subtract.
- **C2.** On a cylinder the wrap is an isometry, x to r·θ: lines along or across the axis map
  exactly, others are fitted as `helix_rise.go` fits. A cone by its flat development. The risk is
  many small patches in a Boolean on one curved face.
- **C3.** Lift the one-pick limit of the rolling-ball fallback; extend `fillet_setback.go` to
  curved end faces. Stop when `TestFilletOnOtherSolids` stops moving.
- **C4.** A periodic frame in `pkg/topo/pathsweep.go`, its twist closed over the loop.
- **D1.** Broad phase from `interference.go`, closest points on the meshes refined with
  `ClosestPointOnThisAndSurface`; thickness by inward rays. The report states its tolerance.
- **D3.** Not done until every file-touching symbol exposed to scripts is covered by a test.
- **D4.** Done 2026-10-22 on branch `wp-d4-incremental`: recommendation 3 (one pass over the
  groups where the size keeps the tolerance) and a cache of solids by what they were made from,
  which makes undo, redo, an edit taken back and an unchanged regeneration cost the solve and
  copies (numbers in `performance-study-2.md`, "Taken up"). Not done: recommendation 6 (the
  Jacobian kept between solves) and 9 (drawing the last solids while a drag's are made off the
  thread, which goes against 151fda16: a drag makes its solids on the thread so linked parts
  follow the pointer); 11–13 move goldens.
- **D5.** Done 2026-10-23 on branch `wp-d5-model-script`:
  `Doc.Script` and `File.ExportScript` beside `describe.go`, File > Export > Script, and
  `telesapr-cli export-script`. `TestScriptRoundTrip` runs every fixture's script on a new
  document and compares groups, counts, DOF, volumes and bounds: 267 of 296 come back; the
  rest (DXF imports drawn as no script draws, splines through more points, images, constraints
  on four entities, fonts beside a fixture) are listed with why.

What the packages left open is in ROADMAP, "Modelling upgrade leftovers".
