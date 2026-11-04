# Roadmap

What is left to build, as of 2026-11-04. Only open work is listed. What is built is described in
`doc/guide`, `doc/guide/ai.md` and `doc/dev-notes/done-audit.md`; `doc/dev-notes/done-audit.md` also holds the definition of
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
| 1 | [Fillet leftovers](#fillet-leftovers) | kernel | ? | Every case left refuses with a message |
| 2 | [Solid feature leftovers](#solid-feature-leftovers) | kernel | ? | Round and diamond profiles at a cubic corner; fitted and closed paths crossing themselves; shells beside curved faces |
| 3 | [Modelling upgrade leftovers](#modelling-upgrade-leftovers) | kernel and feature | ? | The small end of D5 |
| 4 | [Thread leftovers](#thread-leftovers) | feature | S each | DIN 513's nut deviation, B1.9's pitches and multi-start allowances wait for sources |
| 5 | [Test infrastructure](#test-infrastructure) | tooling | S | `pkg/topo` takes 1409 s of the race stage's 1800 |
| 6 | [K3 contact](#k3-coincident-and-tangent-contact) | kernel foundation | S–M | A ball in a seat of its radius; slivers where curved faces sink a hair |
| 7 | [K4 mesher as default](#k4-constrained-delaunay-mesher) | kernel foundation | M | 1.43 times the triangles since 2026-11-04, within the tolerance; the rib rounded all over run in full first |
| 8 | [F5 small conveniences](#f5-small-ui-and-cli-conveniences) | feature | S each | Usability, not capability |
| 9 | [F8 TeleSAPR in a browser](#f8-telesapr-in-a-browser) | feature | S each | Built; progress within a slow group, module size |
| 10 | [F6 IGES](#f6-iges-import-and-export) | feature | L | Only if asked for |

## Open defects

### Fillet leftovers

Code: `pkg/topo/rolling*.go`, `fillet_*.go`, `chamfer_*.go`. Measures: `TestFilletOnOtherSolids`
and `TestChamferOnOtherSolids`, 59 of 59 sound. Each case below is refused with a message.

| Item | State | Approach |
|---|---|---|
| A circular fillet's foot that only just reaches a flat side of its face (a rod of r 4 in a plate 12 wide, r within about a hundredth of 2), or runs off its face past a side where the solid goes on beyond it, or into a side that is curved, not square to the face, or has holes | refused (`contactsClear`, `filletPastSides`, `TestFilletRodThroughPlate`). Since 2026-11-04 a foot running past flat sides square to its face is made: the face widened past them by slabs, the fillet made, the slabs and the fillet beyond cut away by the sides' planes; tried on a rod's foot and a hole's rim, fillets and chamfers, r 2.01 to 3.5, volume within 2e-4 | Just reaching: the face pinched to a point where the side meets the foot, two faces touching there (as `topo`'s pinched loops, `loop_pinch.go`); the cut that only touches the torus leaves naked edges |
| Run-offs left: both ends running off where their cuts overlap along the edge (shorter fillets still; where the cuts lie apart but fell on one patch, a section is put between them since 2026-11-04, `runOffApart`, `TestChamferRunsOffBothEnds`); one only just past a curved face's edge is taken to its corner since 2026-11-04 (`runOffSliver`, `TestFilletRunsOffOntoCylinderBetween`); from a curved end face onto a curved face whose shared edge is no line; a chamfer's end running off both its faces; a run-off taking material away (no test reaches it) | refused | The setback patch: cut the fillet as a setback is cut, fill the hole between it, the next face and the faces round |
| Saddle corner (four or more edges of both kinds) with an edge left sharp, fillets | three of a saddle's four rounded, the fourth sharp, made since 2026-11-04 where the two fillets beside the sharp edge reach it at one point (`saddleSharpAt`: both cut up to it, the third set back, a star of three patches with a side folded there; `TestFilletSaddleSharp` 84 of 108, `TestFilletSaddleSharpOracle` all 58 of OpenCASCADE's lines within 0.056 r³). Refused: steep saddles (24 of the 108, each slope 0.5 or more and the sharp edge's 0.8: the folded patch turns over), the two fillets reaching the sharp edge at two points (unlike radii, a saddle not symmetric about it), and one or two of the four rounded | Two points: the hole has a seventh side, the sharp edge's face's curve between them, which a star doesn't take; split a side at a rung's end for a ladder of three. One or two rounded: a fillet's end at a saddle's vertex has no face to end on, OpenCASCADE's vertex blend |
| The same, chamfers | three of the four chamfered, the fourth sharp, made since 2026-11-04 as the fillets are (`saddleSharpAt` takes chamfers too; `TestChamferSaddleSharp` 84 of 108, `TestChamferSaddleSharpOracle` 48 of OpenCASCADE's 60, the volumes up to 2.3 d² apart, our corner set back further); refused where the fillets are, and with fewer than three chamfered | OpenCASCADE: one B-spline patch, set back less than ours; a flatter corner would be planes cut where the chamfers' planes meet |
| A crease that fades out and turns back (C3) | rounded only up to where it fades | The loft's creases turn back by up to 5° above the first ring |
| A rib rounded all over, foot first | refused (top first is made, all 24 cases) | A vertex blend of three fillets; not needed while the stages take the top first |

Traps: OpenCASCADE is not the truth everywhere (a hole of 5 in a pipe of 10, r 2: its face lies up to
0.093 mm inside the balls' envelope); its Volume of our STEP files is off by up to 5% of a fillet,
integrate slices instead (`scripts/fillet-step-volumes.py`). `srf.ProjectFrom` is not for the rolling
ball's probes: four times slower, no fillet gained.

### Solid feature leftovers

| Item | State | Approach |
|---|---|---|
| Sharp sweep corner next to a cubic | Made since 2026-11-04 of Booleans on an open path with no twist, held frames or corner radius (`pkg/sketch/sweep_cubic_corner.go`): the path cut at the corner into runs carried a little past it along their tangents, exact along lines and arcs, fitted along cubics with their flat patches made planes (`srf.SShell.FlattenPlanarPatches`), and the corner's fill where both sides carried on overlap, joined fill, fitted, exact, the other orders after. Refused by name where no order joins into a sound solid: round and diamond profiles, whose tubes or slanted faces meet the fitted skin nearly tangent where the runs overlap, and twisted, held, rounded or closed paths | The joins of a fitted skin overlapping an exact solid: K3's coincident and tangent contact for B-spline patches, or the kernel's own mitre with the cubic's skin run past the corner and trimmed, which `FaceAdder`'s untrimmed patches can't hold |
| Corner beside an arc whose pieces' outer sides never meet past it (the arc turning against a corner of 150° or more) | refused, names the corner radius (`errCornerFillOpen`); the corner too sharp to join whose outer sides do meet is made since 2026-11-04 by Booleans, the pieces swept apart and the outside filled by their overlap carried on (`pkg/sketch/sweep_corner_fill.go`, `TestSweepSharpCornerBesideArc*`: of 64 rectangles at radius 4 and 12, 48 made, 16 refused) | The overlap runs on as far as the pieces are carried: no finite joint closes the corner, so only a corner radius makes a solid. Closed paths still refuse every such corner (the fill is only on open paths) |
| A sweep whose path comes back within the profile's reach of where it has been (an arc past a sharp corner turning back over the line before it, or looping round across it) | Since 2026-11-04 never made crossing itself (`pkg/sketch/sweep_cross.go`): a path coming back within twice the profile's reach of itself, a quarter turn or more between, has its shell meshed and searched for edges through a triangle. Round lines and arcs, open, the sweep is then made in stretches each clear of itself (cut inside a line or an arc, never at a corner) and joined by a Boolean, the union checked as the group will hold it; otherwise `ErrSweepCrossesItself`. `TestSweepCrossesItselfSweep`: of 1440 line-and-arc paths, 160 crossed themselves on devel; 81 are made sound, 79 refused | Refused still: fitted sweeps (twisted, held, cubic), closed paths, and stretches that can't be cut clear beside a corner's joint or whose union opens edges. Fitted: cut the `topo.SweepPath` the same way; closed: open it at a smooth point first |
| A twisted sweep with nowhere to twist (every piece held: an arc beside a corner, a line within its corner's mitre or joint) | refused (`ErrPathCornerTwisted`), naming the corner radius. Since 2026-11-04 a line at a rigid run's end is held only as far as its joint reaches (`lineHold`, `srf.JointReach`) and twists past it (`TestPathSweepTwistedRun`, `TestPathSweepTwistedRunSweep`); since 2026-11-04 too, where that leaves nowhere to twist, an arc beside a corner is split where its joint's reach ends and twists past it (`arcHold`, `splitArc`, `alongCircle` stops the run growing into it; `TestPathSweepTwistedArcs`, `TestPathSweepTwistedArcsSweep`); what is left is a twist arriving turned at a joint it no longer fits ("a piece is too short for the joint"), and arcs a joint takes whole (corners of 150°, profiles wide against the arc). The arcs-only twisted sweep has its fixture (`sweep-twisted-arcs`), path pair and example (`zigzag`) | A corner radius |
| Shell: curved faces opened at an inside corner beside a curved or sloping face (a cylinder standing on a cone), or a flat face narrower than the thickness | refused by name; beside a flat face square to the axis made since 2026-11-04 (`pkg/topo/shellrevolve_step.go`, `TestShellRevolvedInsideCornerStep`); a drafted extrusion's round side still only between outside corners (`outsideCorners`) | The step runs along the neighbour's surface, which for a cone or torus touches it along a curved face, not a plane: needs K3's coincident curved contact |
| Shell: a union whose parts' insides touch along a line or at a point (a round post as high as the block, sunk into its top beside its side, at 21 of a side at 20) | refused, saying what touches (`TestShellJoinedFlushPostRefused`); was made unsound before 2026-11-03 | The true result is no manifold: the walls either side of the touch meet with no thickness, as with the prism's pinch below. The step against the block's side (the post at 18) is made since 2026-11-04 (`TestShellJoinedFlushPostStep`): the post's foot is kept as a wall where moving it out would carry the inside to the solid's face |
| Shell: a prism (every part extruded from one plane, or one sketch) whose inside's outline touches itself, as a block with a round post at 21 beside its side at 20 | refused since 2026-11-03 (`ShellSolid` checks its result as `ShellJoined` does; `TestShellPrismInsideTouches`, `TestShellBlockAndPostInsideTouches`); was made unsound | The true result is not a manifold: the walls either side of the touch meet along a line, so splitting the outline at the pinch would leave that line. Only a different solid (the touch moved apart) can be made |
| Shells of filleted and lofted solids | refused | Patch offsets (22.1) size faces whose offsets don't cross themselves; the general shell is E12, parked |

### Modelling upgrade leftovers

Left by the work packages of `doc/dev-notes/modelling_upgrades.md`, which holds their decisions.

| Item | Left |
|---|---|
| Model to script (D5) | Damaged files open in part since 2026-11-02 (`TestOpenDamaged`); what they leave out is kept in the file, edges dropped from a group included, and mended from Broken References or a script by giving a point, a workplane or a curve in place of what the file lacks, since 2026-11-04 (`Sketch.LeftOut`, `MendLeftOut`). Left: a missing group (a request's group, an operand) or a constraint's line, circle or face can be given only in the file's text, the window having no way to pick a group or to know what kind the missing entity was |

### Thread leftovers

Code: `pkg/thread`, `pkg/sketch/thread*.go`, `pkg/ui/thread*.go`. Sources checked for the classes:
DIN 513-3:2020 (free text), NBS H28 (1957) Part III, Machinery's Handbook, Engineers Edge's ACME rows.

| Item | State | Approach |
|---|---|---|
| DIN 513-3's nut deviation | 7H, 8H, 9H taken as basic | EI = 3.1758·a (2/(tan 3° + tan 30°) times DIN 513-1's axial play a, not found free); with a, `buttressClearance` adds it |
| B1.9's preferred pitches | `buttressInchTPI` follows H28's ranges | amesweb's calculator lists three a diameter (B1.9 Table 1?); B1.9 itself would settle it, and the default pitch per size |
| Rounded B1.9 root at coarse chord tolerances | refused coarser than a fifth of its radius (`ErrRootRoundCoarse`) | Grooves cut from a blank: nuts sound at 0.5%, screws failed at 0.2% and 0.5% |
| Multi-start allowances | left out for B1.5 (appendix B) and B1.9; DIN 513-3's factors are in | H28 (1957) has none for ACME, only its gauges' lead tolerance times 1.5. Machining Doctor says B1.5 adds K × es to a nut's diameters by its number of starts; K not found free |

### Test infrastructure

| Item | State |
|---|---|
| Race stage of `make ci` | 2026-11-04, `make ci` with nothing beside it (26 min): `pkg/topo` 1409 s, `pkg/script/api` 1317, `pkg/sketch` 1245, `pkg/ui` 889, `pkg/check` 529, against the 1800 s limit. Thinned under `-race` on 2026-11-03 and 04 (the normal run keeps every case): the rim mitres, lid and other-solid fillet tests of `pkg/script/api`; the fitted sweeps' oracle tests leave out `Oracle.Measure`; the thread, rolling-ball chain, rib and closed-loft tests take a part of their cases; `pkg/ui` keeps one thread among its scripts and cached models (`raceLeavesOut`). Next in `pkg/topo`: the loft sweeps (`TestSmoothLoftSweep` 595 s, `TestClosedLoftSweep` 564 under load) and `TestFilletChainsPartlyMitred` (431) |

### Guide tooling

| Item | State |
|---|---|
| Menus, popup menus and dialogs in pictures | The real window under an Xvfb of its own since 2026-11-04 (`TestDocWindowShots`, `scripts/regen-doc-window-images.sh`): the File menu, an edge's context menu and the Triangle Mesh dialog. Left: more of them where a page would gain (the Fillet and Thread dialogs, a submenu open), and the fonts are the machine's, so the pictures change with the machine that regenerates them |
| Clicks by name in the rest of the scripts | `click-entity` and `click-tw` since 2026-11-04 (`doc/guide/scenarios/README.md`), used by the sweep path pairs, `path_chamfer_corner` and `ext_fillet_concave`; the guide's scenarios and the other scripts still click by coordinates, and a solid's edges and faces have no command of their own (`click-edge`) |
| Video | Frames of each `settle()` with the cursor tweened, captions, ffmpeg; longer demos from the real window under Xvfb |

## Kernel foundations

From the BRL-CAD survey of 2026-10-07 (ideas re-implemented in Go, no cgo). Each starts as a two-day
spike with a fixed budget.

### K3 Coincident and tangent contact

Taken case by case, each a fixture with a sweep (`pkg/srf/contact_test.go`, `contact_flush_test.go`,
`latheseam_test.go`, `pkg/topo/loop_pinch_test.go`, `contact_near_test.go`); every case of those
sweeps is sound, and the curved-contact probe below finds what is not. A flush
cutter's side and the face it lies on meet on one curve since 2026-11-04: the trims on the edge the
side kept with its dropped cap move onto the face's curve along the same points (`moveOrphanTrims`,
`TestFlushCutterCurveHandles`), so no curve names a dropped surface.

- **Spike of 2026-11-04** (`doc/dev-notes/k3-fuse-spike.md`): a Boolean fuses the second operand's
  planes and coaxial cylinders onto the first's within a tenth of the chord tolerance
  (`snapToPlanes`, `snapToCylinders`), and a curved face lying on the classified patch across a
  smooth edge is coincident with it, not inside (`classifyEdgeOn`). Solids touching, standing off or
  sunk a hair (`TestNearContactSweep`) 150 of 816 wrong → 0; a pin in a bore of its size
  (`TestPinInBoreSweep`, `TestCurvedContactProbe`) 77 of 108 → 0. No golden moved.
- **Cones and balls, 2026-11-04:** a cone in a countersink of its angle, touching or a hair off, is
  sound by every Boolean (`TestCoaxialContactSweep`, 96 of 96 wrong before): the fuse takes coaxial
  cones and concentric spheres too (`snapToCones`, `snapToSpheres`), a side running onto a face of
  the other solid lying on the classified patch is coincident where the other solid's chorded edge
  misses the true one (`classifyOnCoincident`), and a plane meeting a lathed face adopts the plane's
  own exact outline lying on it instead of marching another. A ball in a seat of its radius
  (`TestCoaxialSeatProbe`, `TELESAPR_K3_PROBE=1`) is 50 of 96 wrong (88 before): the seat's rim is
  the marched curve of the Boolean that made the seat, and the ball's crossing of the block's top is
  marched again through other points; an exact circle for a plane across a sphere would close it, at
  the cost of moving goldens with such a cut.
- **Tangent contact, 2026-11-04:** touching at a point or along a line, the union has the right
  volume and is not a manifold there. Check Solid now says so (`Report.Touches`, `TouchAt`,
  `TestCheckPartsTouching`) where every problem edge comes within the chord tolerance of both the
  group's part and the solid before it and the volume is theirs added. It is a report of Check
  Solid, not a problem on the group: finding it needs the mesh checked, which regeneration does not.
- **Left, S to M:** the ball in its seat (above). Sunk a hair, two curved faces meet in a sliver
  under the chord tolerance (`ball`, `lying` in the probe), as do sides closer than it but more
  than a tenth of it: those are the mesher's (K4).
- Not recommended: a mesh Boolean with perturbation (only as an explicit "approximate" mode).

### K4 Constrained Delaunay mesher

Merged as a mender (`pkg/srf/triangulate_mend.go`, `triangulate_check.go`): the ear clipper meshes
every face, and a face that crosses the shell's mesh, leaves an edge unmated, shares a triangle with
another face, or holds a sliver is meshed again by the CDT. It can mesh every face
(`Tolerance.WithMesher(MesherCDT)`, `doc/dev-notes/k4-cdt-spike.md`). Measured again 2026-11-04
(`BenchmarkMeshers`, the 84 solid and thread goldens): with its chart scaled by the surface's
curvature since then, it makes 1.43 times the ear clipper's triangles and `pkg/topo`'s tests take
1.27 times the CPU, where the spike's chart made 2.0 and over 3.2. The ear clipper does not meet
its own tolerance: 16.7% of its curved triangles stray past the chord tolerance, the CDT's 1.8%.
Left before it can be the default (M): the rib rounded all over (`TestFilletRibAllOverOracle`) had
1 to 4 self-intersecting edges under it; its short case has none since the evening of 2026-11-04,
but the test does not finish in 10 minutes over the CDT's 151 500 triangles, so its checks must cost
less and it must run in full. The two tests that pinned mesh counts are restated (2026-11-04). Then
every golden with a curved face is regenerated, a commit per regen script.
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

Built: `cmd/telesapr-web` on `pkg/platform/web` and a WebGL 2 renderer, `make wasm`, the site's
"Try in browser" page (`doc/dev-notes/browser.md`). Left:

| Item | Size |
|---|---|
| Progress within a group: since 2026-11-04 the page draws and shows how long between a regeneration's groups (`platform.Yielder`), but one slow group (a thread's Boolean) still holds it for its length. Needs yield points inside the kernel's long loops, given to it rather than global | S |
| Module size: 39 MB after `wasm-opt`, 11.5 MB gzipped, since 2026-11-04 without package `net` (1.2 MB). The rest is code (yaegi 2.3 MB, then `pkg/ui`, `topo`, `sketch` 2 MB each) and Go's tables; the icons are 0.24 MB. Left, each costing a format or a feature: `encoding/gob` for the STEP cache (0.3 MB), `docs.json` gzipped (0.65 MB raw, nothing over the wire); `doc/dev-notes/browser.md` Size | open |
| The agent (MCP) through a postMessage bridge | The bridge is built (2026-11-04: `socket_js.go`, `agent-client.js`), carrying the agent's protocol; left: carrying MCP to it from a client outside the browser, e.g. an extension relaying `telesapr-cli mcp` by native messaging (S–M). The MCP server in the page would bring `net/http` back |

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
| A chat window, MCP resources and prompts, an HTTP transport | "An MCP server, and nothing else" until a user needs more (`doc/guide/ai.md`) |
| `make ai-eval` | Needs a model and an account, outside CI |
| Tolerances per edge and vertex | Healing within a file's tolerance suffices so far |
| Generated entities left out of `.tlspr` | Possible behind the version number; no need yet |
| Threads: tapered (NPT, BSPT), variable pitch, a fastener library, a thread attribute in STEP | Never asked for |
| Scripting: a startup script, `--watch`, menu items made by scripts, zoom to fit | Never asked for |
| The bit-exact scaffolding: `pkg/libm`, `CxxSort`, the FMA build rules | Replacing it moves every golden for nothing |
| A mesh Boolean fallback with perturbation | Conflicts with "never silently wrong geometry" |
| Sheet metal, direct modelling, tolerancing, CAM, simulation | Each an XL product of its own |
| Dual contouring | A mesher for sampled fields, not an exact B-rep (`git show 39142006:doc/notes/dual-contouring-fit.md`) |
| `.tlspr` read and written as streams | P5 of `doc/dev-notes/performance.md`; no user has waited on it |
| A wasm build in CI (F8) | Postponed by the user 2026-11-04. Nothing builds or tests the js/wasm target, so a change can break it unseen until `make wasm` is run by hand |
