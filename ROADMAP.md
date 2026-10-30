# Roadmap

What is left to build, as of 2026-10-30. Only open work is listed. What is built is described in
`doc/guide`, `doc/ai.md` and `doc/done-audit.md`; `doc/done-audit.md` also holds the definition of
done. The dated "Done" and "Tried" notes of every item, cut from this file on 2026-10-30, are in
`git show af848717:ROADMAP.md`; read an item's there before taking it up, so as not to try again
what failed. Code comments that cite "ROADMAP-JUMP.md", "package 5.1" or "roadmap 24.2" refer to
`git show e0004fc:ROADMAP-JUMP.md`; "ROADMAP F1" and "ROADMAP F7" name features since built.

Sizes are focused weeks for one developer who knows the code, the definition of done included:
**S** under a week, **M** one to three, **L** three to eight, **XL** more, **?** not sized. A kernel
item is sized only after a two-day spike on its hardest fixture.

## Order of work

| # | Item | Kind | Size | Why here |
|---|---|---|---|---|
| 1 | [Fillet leftovers](#fillet-leftovers) | kernel | ? | One case is unsound, not refused; the rest refuse with a message |
| 2 | [Solid feature leftovers](#solid-feature-leftovers) | kernel | ? | Sweep corners next to a cubic; shells of the last joined cases |
| 3 | [Modelling upgrade leftovers](#modelling-upgrade-leftovers) | kernel and feature | ? | Small ends of D1, D4, D5 and lofts |
| 4 | [Thread leftovers](#thread-leftovers) | feature | S each | DIN 513's nut deviation, B1.9's pitches and multi-start allowances wait for sources; chamfered nuts at coarse tolerances |
| 5 | [Test infrastructure](#test-infrastructure) | tooling | S | The race stage is within minutes of its limit |
| 6 | [K3 contact](#k3-coincident-and-tangent-contact) | kernel foundation | ? | One pinch case and the "fuse by identity" rework |
| 7 | [K4 mesher as default](#k4-constrained-delaunay-mesher) | kernel foundation | M | Costs 3 to 5 times the triangles today |
| 8 | [F5 small conveniences](#f5-small-ui-and-cli-conveniences) | feature | S each | Usability, not capability |
| 9 | [F8 TeleSAPR in a browser](#f8-telesapr-in-a-browser) | feature | L | Spiked; needs a WebGL renderer and a `web` platform |
| 10 | [F6 IGES](#f6-iges-import-and-export) | feature | L | Only if asked for |

## Open defects

### Fillet leftovers

Code: `pkg/topo/rolling*.go`, `fillet_*.go`, `chamfer_*.go`. Measures: `TestFilletOnOtherSolids`
and `TestChamferOnOtherSolids`, 59 of 59 sound. Each case below is refused with a message unless it
says unsound.

| Item | State | Approach |
|---|---|---|
| Closed fillet round a rod through a plate, foot reaching the plate's sides (rod r 4, plate 12 × 4, fillet r ≥ 2) | **unsound** (69 naked edges at r 2), not refused | At least refuse it; then find where the foot leaves the plate's face |
| Run-offs left: both ends running off where their cuts share a patch (short fillets); onto a curved face only just past its edge (a sliver under the chord tolerance); from a curved end face onto a curved face whose shared edge is no line; a chamfer's end running off both its faces; a run-off taking material away (no test reaches it) | refused | The setback patch: cut the fillet as a setback is cut, fill the hole between it, the next face and the faces round |
| Saddle corner (four or more edges of both kinds) with an edge left sharp, fillets | refused | OpenCASCADE uses one vertex blend. A setback folded to a point on the sharp edge turned over in every layout; next try: one fillet run up to the sharp edge as in `fillet_blend.go`, a seven-sided hole |
| The same, chamfers | refused, not begun | OpenCASCADE: one B-spline patch; our cone cut takes convex cones only |
| A crease that fades out and turns back (C3) | rounded only up to where it fades | The loft's creases turn back by up to 5° above the first ring |
| A rib rounded all over, foot first | refused (top first is made, all 24 cases) | A vertex blend of three fillets; not needed while the stages take the top first |

Traps: OpenCASCADE is not the truth everywhere (a hole of 5 in a pipe of 10, r 2: its face lies up to
0.093 mm inside the balls' envelope); its Volume of our STEP files is off by up to 5% of a fillet,
integrate slices instead (`scripts/fillet-step-volumes.py`). `srf.ProjectFrom` is not for the rolling
ball's probes: four times slower, no fillet gained.

### Solid feature leftovers

| Item | State | Approach |
|---|---|---|
| Sharp sweep corner next to a cubic | refused; a corner radius can't round it either | In a plane the profile's paths still cross, so `sweep_joint.go`'s joint carries over with offset tracks; the cubic's fitted skin would have to run past the corner and be trimmed by the joint, which `FaceAdder`'s untrimmed patches can't hold |
| Corner beside an arc too sharp to join (a bar 2 wide round an arc of 12 at 150°) | refused, names the corner radius | The arc's wall would leave through the line's start cap |
| A twisted sweep with nowhere to twist (every piece a line or arc beside a corner) | refused (`ErrPathCornerTwisted`) | Corner radius |
| A closed path out of a plane with sharp corners | refused early in `pkg/sketch/sweep.go` | Likely allowed now that the twist is held still beside corners |
| Shell: curved faces opened at an inside corner | refused (`outsideCorners`) | The inside needs a step along the neighbour's plane touching the solid's edge |
| Shell: a revolve of more than half a turn with an end inside another part; a half turn whose two ends move unlike | refused by name | Its ends lie in one plane |
| Shell: a union opened at a top face two parts share (a post as high as the block it is sunk in) | **fails** at the final Boolean | The inside's top is two coplanar faces merged |
| Shells of filleted and lofted solids | refused | Patch offsets (22.1) size faces whose offsets don't cross themselves; the general shell is E12, parked |

### Modelling upgrade leftovers

Left by the work packages of `doc/notes/modelling_upgrades.md` (notes at `1e84ca0c`).

| Item | Left |
|---|---|
| Lofts (C4) | A solid's sides at a closed loft's ends (a ring has none); profiles more than a third of a turn apart round a ring (draw more) |
| Clearance and walls (D1) | A wall between two triangles no measured ray joined can be missed; a group forced to a mesh with a part assembled after it is no part to `Clearances` |
| Model to script (D5) | `tlspr/v1/fields.tlspr` (dangling references) opens as no model. Opening damaged files in part would turn about 250 `Must*` lookups in `pkg/sketch` into diagnostics on their groups: an item of its own |
| Regeneration (D4) | Recommendation 11's parallel commits (curve ids reserved in advance); the rest taken or measured and declined (`doc/notes/performance.md`) |

### Thread leftovers

Code: `pkg/thread`, `pkg/sketch/thread*.go`, `pkg/ui/thread*.go`. Sources checked for the classes:
DIN 513-3:2020 (free text), NBS H28 (1957) Part III, Machinery's Handbook, Engineers Edge's ACME rows.

| Item | State | Approach |
|---|---|---|
| DIN 513-3's nut deviation | 7H, 8H, 9H taken as basic | EI = 3.1758·a (2/(tan 3° + tan 30°) times DIN 513-1's axial play a, not found free); with a, `buttressClearance` adds it |
| B1.9's preferred pitches | `buttressInchTPI` follows H28's ranges | amesweb's calculator lists three a diameter (B1.9 Table 1?); B1.9 itself would settle it, and the default pitch per size |
| Rounded B1.9 root at coarse chord tolerances | refused coarser than a fifth of its radius (`ErrRootRoundCoarse`) | Grooves cut from a blank: nuts sound at 0.5%, screws failed at 0.2% and 0.5% |
| Chamfered nuts at coarse chord tolerances | Buttress nuts at 0.2% and coarser (a 1-12 BUTT nut 20 long, a 2-8 BUTT nut), and ISO, UNC and trapezoidal nuts 16 long in parts of twice their diameter at 0.5% (M20, M36) or 1% (M8, 1/2-13 UNC, Tr40x7): unsound, flat or rounded crest, after every one of `chamferTries` | Naked edges where the chamfer's cone meets the ridges near its quarters' seams and the end's plane. Sweeping the ridges or the chamfer finer than the sketch moves which cases fail, so it is the Boolean of the cone with the ridges, not their chords |
| Multi-start allowances | left out for B1.5 (appendix B) and B1.9; DIN 513-3's factors are in | H28 (1957) has none for ACME, only its gauges' lead tolerance times 1.5. Machining Doctor says B1.5 adds K × es to a nut's diameters by its number of starts; K not found free |

### Test infrastructure

| Item | State |
|---|---|
| Race stage of `make ci` | About 1450 to 1560 s a package against the 1800 s limit (2026-10-30). Next to thin under `-race`: the single fillet tests of 500 to 640 s (`TestChamferEndsOnCurvedFace`, `TestFilletEndsOnCurvedFace*`, `TestFilletRimMitreFine/1`, the loft's edges) |

## Kernel foundations

From the BRL-CAD survey of 2026-10-07 (ideas re-implemented in Go, no cgo). Each starts as a two-day
spike with a fixed budget.

### K3 Coincident and tangent contact

Taken case by case, each a fixture with a sweep (`pkg/srf/contact_test.go`, `contact_flush_test.go`,
`latheseam_test.go`); every case measured so far is sound but one.

- **Left:** a band's top touching a cone at one point at its end face (the x band at 21 dipping 1).
  The end face is two lobes meeting at the point: the trims make one loop through it twice,
  `topo.chain` picks a turn of ±π by rounding, and the ear clipper leaves 6 naked edges. Splitting
  the contour (`SPolygon.splitPinches`) closes the mesh, but `topo` must split the loop too or the
  Euler check fails.
- **Fragile:** a flush cutter's orphaned edge keeps its dropped cap's surface handle, which a later
  Boolean can map onto another surface. Merging the trims onto the kept curve (one edge) is sound
  but moves the trims' curve handles in several goldens; it waits for a golden regeneration of its own.
- **Approach for what is left** (BRL-CAD libnmg): fuse first, then classify by identity. Fuse the
  shells' vertices within tolerance, split edges at vertices on them, share coplanar faces' planes;
  after the cut, ON elements split into on-shared and on-anti, kept by a table per Boolean (extend
  `keepEdge` / `keepRegion`, `pkg/srf/boolean_trim.go`). Curved overlaps: boundaries on isocurves at
  the patches' knots. Point-in-solid by several rays with explicit valid / grazing / coincident /
  failed results.
- Not recommended: a mesh Boolean with perturbation (only as an explicit "approximate" mode).

### K4 Constrained Delaunay mesher

Merged as a mender (`pkg/srf/triangulate_mend.go`, `triangulate_check.go`): the ear clipper meshes
every face, and a face that crosses the shell's mesh, leaves an edge unmated, shares a triangle with
another face, or holds a sliver is meshed again by the CDT. As the default (`k4-cdt-spike` branch,
`doc/notes/k4-cdt-spike.md`) it costs 3 to 5 times the triangles and makes `pkg/topo`'s tests 3.4
times slower. To take it up: bring the cost down, then regenerate every golden with a curved face.
It would close the twisted swept bar's 1.6% volume error, the only open item no mender reaches.

### K5 STEP repair and accounting rules

Closed: the STEP corpus reads 9 of 9 as sound. For the next file that fails (BRL-CAD `conv/step`):
accept in model space by the file's tolerance, a ladder of faithful construction, safe repair,
tagged inference, one accounted result per entity; `repair_missing_singular_trims` for poles.
STEP's `newtonOnSurface` keeps its own steps (`srf.ProjectFrom` grew NIST_MBE_PMI_6's slivers).

## Features

### F5 Small UI and CLI conveniences

| Item | State |
|---|---|
| The ray-cast oracle (K2) for lofts | Refused by name (`check.ErrUnsupported`): a loft's surfaces are the kernel's own construction, so rebuilding them would repeat it, not check it. Also refused: a profile leaning to a helix's or an arc's axis |
| Dockable property browser | takigo decides; lowest |

### F6 IGES import and export

Neosolve reads and writes IGES. STEP covers the exchange; take it when a user asks.

### F8 TeleSAPR in a browser

Spike done 2026-10-29 (`doc/notes/browser-wasm-spike.md`, `cmd/telesapr-wasm`): every package but
the takigo window builds for js/wasm, output is byte-identical to native, regeneration 2.5 to 5 times
slower than one native thread, 12 MB gzipped, the soft renderer 6 to 18 fps (under the 15 fps gate).

| Item | Size |
|---|---|
| A WebGL 2 `render.ViewportCanvas` (`pkg/render/webgl`) after `pkg/render/gl` | M |
| `pkg/platform/web`: canvases, HTML menus and dialogs, file dialogs over memfs, timers, settings; its event loop on its own goroutine so modal dialogs block it, not the page | M |
| `cmd/telesapr-web`, the page, memfs kept in IndexedDB, `make wasm` | S |
| Regeneration off the event goroutine with progress; module size | S, then open |

Not in it: threads in the browser, the MCP agent.

## Waiting for the user

| Item | State |
|---|---|
| DIN 513-1's axial play a, ASME B1.9's Table 1, and ASME B1.5's appendix B | Each settles a row of [Thread leftovers](#thread-leftovers) |
| What to take back from "Parked" | Every row there is a proposal |
| What the review took in without being asked | B3 (16.11), G4 (17.4) and S3 (17.5) are built; the user may still disown them |

## Parked

Each stays out until its reason stops holding.

| Item | Reason |
|---|---|
| E12 general shell | Open-ended; patch offsets size faces whose offsets don't cross themselves |
| Topology made by the kernel (9.2) | Derived topology costs 7 ms for 270 faces; no feature needs identities kept through a Boolean |
| OpenGL on Windows and macOS | takigo has backends for both; a port is a project of its own |
| The section view saved in files | View state; goes with saved views, which nothing plans |
| Open thread ends in STEP | Closing them would invent geometry |
| CSG and swept solids in STEP | No vendor file in the corpus has them; the export half is cheap once the group tree is walkable |
| A chat window, MCP resources and prompts, an HTTP transport | "An MCP server, and nothing else" until a user needs more (`doc/ai.md`) |
| `make ai-eval` | Needs a model and an account, outside CI |
| Tolerances per edge and vertex | Healing within a file's tolerance suffices so far |
| Generated entities left out of `.tlspr` | Possible behind the version number; no need yet |
| Threads: tapered (NPT, BSPT), variable pitch, a fastener library, a thread attribute in STEP | Never asked for |
| Scripting: a startup script, `--watch`, menu items made by scripts, zoom to fit | Never asked for |
| The bit-exact scaffolding: `pkg/libm`, `CxxSort`, the FMA build rules | Replacing it moves every golden for nothing |
| A mesh Boolean fallback with perturbation | Conflicts with "never silently wrong geometry" |
| Sheet metal, direct modelling, tolerancing, CAM, simulation | Each an XL product of its own |
| Dual contouring | A mesher for sampled fields, not an exact B-rep (`git show 39142006:doc/notes/dual-contouring-fit.md`) |
| `.tlspr` read and written as streams | P5 of `doc/notes/performance.md`; no user has waited on it |
