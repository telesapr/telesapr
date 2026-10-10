# Roadmap

What is left to build, as of 2026-10-10. Only open work is listed; what is built is described in
`docs/guide`, `docs/ai.md` and `docs/done-audit.md`, and `docs/done-audit.md` holds the definition
of done every feature meets. The plan of steps 1 to 29 and the kernel's working notes were
`ROADMAP-JUMP.md` and `docs/notes/` up to commit `e0004fc` (`git show e0004fc:ROADMAP-JUMP.md`);
code comments that cite "ROADMAP-JUMP.md", "package 5.1" or "roadmap 24.2" refer to that file.

Sizes are focused weeks for one developer who knows the code, the definition of done included:
**S** under a week, **M** one to three, **L** three to eight, **XL** more, **?** not sized. Read
them as relative weights. A kernel item is not sized until a two-day spike on its hardest
fixture has been done; its "Tried" notes say what not to try again.

## Order of work

| # | Item | Kind | Size | Why here |
|---|---|---|---|---|
| 1 | [F2 Inequality and point-on-segment constraints](#f2-inequality-and-point-on-segment-constraints) | feature | M | Solver gap against neosolve; no kernel risk |
| 2 | [K1 Robust point projection](#k1-robust-point-projection) | kernel foundation | S–M | Smallest kernel item with the widest effect (fillets, pullback, picking, STEP) |
| 3 | [K2 Ray-cast CSG oracle](#k2-ray-cast-csg-oracle) | test foundation | M | An in-tree volume and Boolean oracle for every later kernel change, no FreeCAD needed |
| 4 | [K3 Coincident and tangent contact (F5)](#k3-coincident-and-tangent-contact-f5) | kernel foundation | ? (L) | Blocks four open items below |
| 5 | [K4 Constrained Delaunay mesher](#k4-constrained-delaunay-mesher) | kernel foundation | ? (L) | Retires the ear-clipper heuristics; blocks five open items below |
| 6 | [F4 Corner radius on path sweeps](#f4-corner-radius-on-path-sweeps) | feature | M | The sound joint users usually want where 28.4's mitre fails |
| 7 | [F5 Small UI and CLI conveniences](#f5-small-ui-and-cli-conveniences) | feature | S each | Usability, not capability |
| 8 | [K5 STEP repair and accounting rules](#k5-step-repair-and-accounting-rules) | kernel | ? (M) | Corpus 7 of 9 sound; NIST_6 and Schenkel |
| 9 | [Fillet leftovers](#fillet-leftovers) | kernel | ? | Each refused case today is a clear message, not bad geometry |
| 10 | [Solid feature leftovers](#solid-feature-leftovers) | kernel | ? | Wait for K3 |
| 11 | [F6 IGES import and export](#f6-iges-import-and-export) | feature | L | Only if asked for |

Items 4 and 5 open most of the rest: the dependency column in each table below names which.

## Features

### F2 Inequality and point-on-segment constraints

From the neosolve survey (2026-10-07). Neosolve has them; TeleSAPR has neither.

- **Minimum and maximum distance** (≥, ≤): two slack equations per constraint, as the solver's
  other inequality mechanisms do.
- **Point on segment**: a point held to a line's parameter t in [0, 1]; today `PtOnLine` is on
  the infinite line. Same slack mechanism.

Both need the usual menu entries, text-window rows, `pkg/script/api` methods and freedom/conflict
reporting.

### F4 Corner radius on path sweeps

A path sweep's corner rounded on the path, not on the solid: the straight runs trimmed back by
`radius · tan(θ/2)` and joined by the profile revolved about the bend's axis, so each run's end
section is the bend's end section and no surface intersection is needed (BRL-CAD's pipe
primitive, `pipe.c`). Refuse bends that overlap. Works for any profile and for corners beside
arcs, which the mitre cannot do (see [Solid feature leftovers](#solid-feature-leftovers)).

### F5 Small UI and CLI conveniences

From the neosolve survey; each is small and independent.

| Item | State |
|---|---|
| Arrow keys nudge the selected entities | not found in `pkg/ui` |
| Directional marquee: window left to right, crossing right to left | check what the selection does today |
| Separate visibility toggles for reference constraints and comments | check what View > Show offers |
| `telesapr-cli` `--profile` with a JSON dump | benchmarks exist, no flag |
| `telesapr-cli export --group` | export takes the whole model |
| Dockable property browser | takigo decides; lowest |

### F6 IGES import and export

Neosolve reads and writes IGES. Low priority: STEP covers the exchange; take it when a user asks.

## Kernel foundations

These come before the open defects they unblock. Recommended by the BRL-CAD survey of
2026-10-07 (BRL-CAD checkout surveyed read-only; the libraries are LGPL 2.1, so ideas are
re-implemented in Go, no cgo). Each starts as a two-day spike with a fixed budget.

### K1 Robust point projection

`SSurface.ClosestPointTo` / `ClosestPointToUV` (`pkg/srf/surface.go`) can stop short: a point
2 mm off a cylinder came back 6 µm short, which is why `nearestOn` in `pkg/topo/rollingball.go`
polishes by Gauss-Newton after it. Port the design of BRL-CAD's
`surface_closest_point_seeded` (`PullbackCurve.cpp` ~1091-1240): damped Gauss-Newton with an
explicit 2×2 solve; a per-direction step cap and up to 12 halvings, a step taken only when the 3D
distance drops; periodic directions evaluated wrapped, open ones clamped; best point kept;
success judged by the 3D distance against the caller's tolerance, never by step size; fallbacks
of three seeds per knot span and a branch-and-bound over Bézier span boxes.

Keep the old path behind `mustConverge` until the fillet and STEP suites pass, then drop
`nearestOn`'s polish. About 500 lines.

### K2 Ray-cast CSG oracle

An independent oracle in `pkg/check` that needs no surface intersection: each leaf solid returns
in/out segments along a ray, the segments are woven and the group tree evaluated per segment
(BRL-CAD `rt_boolweave` / `bool_eval`). Analytic leaves (plane, cylinder, cone, sphere, torus),
extrusions by 2D parity, revolutions. Volume, centroid and inertia by three orthogonal grids,
halved until the three agree; a raydiff against the B-rep mesh says where they disagree. Gives the
sweeps a failure signal without FreeCAD or OpenCASCADE, and an MCP "probe along this line" tool.

Also from BRL-CAD, for `pkg/check`: a one-pass sorted half-edge closed-manifold check
(`bg_trimesh_solid2`: unmatched, excess, misoriented, degenerate in O(n log n)).

### K3 Coincident and tangent contact (F5)

F5 in general never finishes, so it is taken case by case, each as a fixture with a sweep.
`MakeCoincidentEdgesInto` (`pkg/srf/surfinter.go`) handles coincident planes only; the failures
are an edge of one solid in a face of the other, a cap coincident with several flat patches at
once, and two coplanar opposite faces with equal outlines.

Approach (BRL-CAD libnmg): **fuse first, then classify by identity.** Before intersecting, fuse
the two shells' vertices within tolerance, split edges at vertices lying on them, make coplanar
faces share one plane, fuse edges. After the cut an element is ON when the other shell uses the
same vertex or edge; split ON into on-shared (same direction) and on-anti (opposite) and decide
each Boolean by a table: union keeps shared, drops anti; difference keeps anti; intersection
keeps shared. Edges with both ends ON are classified at 0.5, then 0.1, then 0.9. Extend
`keepEdge` / `keepRegion` (`pkg/srf/boolean_trim.go`) with the split. For curved multi-patch
overlaps (libbrep): the overlap's boundary lies on isocurves at the patches' knots; intersect
those with the other surface and chain them into loops. Point-in-solid by several rays with
explicit valid / grazing / coincident / failed results instead of a vote.

Spike order:

1. the leaning wedge on a plate (all planes; simplest);
2. the off-axis bore whose cap lies in a lathed tip's plane (isocurve overlap boundary).

Unblocks:

| Open item | State today |
|---|---|
| A wedge on a plate, its sloping top's foot in the plate's top face, its width leaning back along the foot | the union fails for every lean that way, none the other; `TestFilletRibFootSweep` keeps to the one that works |
| A bore whose cap lies in a lathed tip's plane, off axis | fails 40 of 80: coincident contact across the tip's four flat patches |
| Sweep corners beside an arc (see [Solid feature leftovers](#solid-feature-leftovers)) | two runs joined by a union fail where they touch, for corners between lines too |
| The teapot's handle (a torus joined to a cone) | unsound before any fillet, at 0.005 and 0.02 chord tolerance |

A last resort, not recommended: BRL-CAD's mesh Boolean with deterministic perturbation of
touching operands. It conflicts with "never silently wrong geometry" and could only be an
explicit, reported "approximate" mode.

### K4 Constrained Delaunay mesher

`UvTriangulateInto` (`pkg/srf/triangulate.go`) ear-clips a uv outline against grid points, and
every step 24 to 26 fix was an ear-ordering rule (`grazed`, `folded`, `cutOff.crosses`,
`poleSliver`, `AvoidFlatEars`, `simple`). Its weakness is fans: a patch ruled one way, or a
trimmed corner, has few interior points, so the clipper fans long chords from one trim point and
a chord, straight in xyz where its uv image bends by its sag, passes a trim point on the wrong
side.

Approach (BRL-CAD `ON_Brep_CDT_Tessellate`): discretise each edge once, in 3D, shared by both
faces (watertight by construction); interior uv points by recursive quadsection sized from the
boundary's segments, finer near trims; uv rescaled to unit aspect; a constrained Delaunay with
holes (a Go port of detria, or earcut with Steiner points); singular trims split in 2D at poles
and the degenerate 3D triangles dropped; then validate and repair (edges used more than twice,
normals against the surface, misoriented pairs; a bad patch regrown and retriangulated).

Spike: one patch type behind a flag (the twisted ruled face, then a pole patch), measured against
the current mesher on the fixtures below.

Unblocks:

| Open item | State today | Tried |
|---|---|---|
| A twisted swept bar | flat sides mesh within 1.6% of volume | One inner column across the rulings: 0.33% (132 to 452 triangles); four lines 0.25%. But a bar cell and a thread flank's cell have the same twist (diagonal sag ~0.05 mm), so no rule by aspect or sag gives the bar columns and not the threads; columns everywhere doubled thread meshes and moved 2 of 3 thread volumes the wrong way. Sag-driven cell refinement ran for minutes. Untried: fan diagonals alternated so the twist's bias cancels |
| NIST_MBE_PMI_6 as a solid | mesh Euler number 6 off the shell's | At two poles a trim runs up a meridian into the pole and earlier ears fanned the last grid row to one trim point, leaving only pole slivers; `poleSliver` as a fallback ear mended one pole. Needs grid points in the band below the pole, or an ear order that keeps a row point for the meridian's top |
| Run-off chamfers crossing the cone (`TestRunOffSweep`) | 2 of 36 cross by one edge each | `cutOff.crosses` took 10 to 4 failing. Grid lines across rulings of trimmed ruled patches: 8 to 2, but 2x meshes at default tolerance, 5-7x at fine. Demoting ears leaning off the tangent plane: no change |
| A band-on-cone fillet at r 1, chord 0.001% | 2 crossing mesh edges where the fillet meets the cone tangentially | It is two tangent faces' meshes crossing, not the fit; not re-checked since the mesher fix of `7b3b32a` |
| A chamfer of 3 where a slot meets a cone | 6 naked edges where it reaches the slot's floor at (6, 3, 0); sound to 2.75, 10 naked at 3.25 | The topology lists every edge, so they are the meshes' (the end wall's, or the side wall's thin triangle (6,3,0)-(6.47,3,0)-(6,3,2.25)); not traced |

### K5 STEP repair and accounting rules

STEP corpus: 7 of 9 read as sound solids. From BRL-CAD's `conv/step` (`STEPImportArchitecture.md`,
`STEPBrepSeamRepair.cpp`): accept in model space by the file's declared or a measured tolerance,
never a solver threshold; a ladder of faithful construction, safe repair, tagged inference;
exactly one accounted result per entity, so the report says what was repaired or lost. Try
`repair_missing_singular_trims`' approach on the two below.

| Open item | State today |
|---|---|
| NIST_MBE_PMI_6 | the pole topology problem under K4 |
| Schenkel | mesh closed and not self-crossing (`TestSolidSchenkelClosed`), no naked edges since `ed03d3b`, but the shell's Euler number is -34 against the mesh's -6; cause not found, so not counted sound |

## Open defects

### Fillet leftovers

Code: `pkg/topo/rollingball.go`, `rollingend.go`, `rollingrunoff.go`, `fillet_setback.go`,
`fillet_network.go`. Every case below is refused with a message today, except where the State
column says unsound. Measure: `TestFilletOnOtherSolids` (fillets on lofts, sweeps and STEP
solids) at 44 of 59 sound.

| Item | State | Approach / tried |
|---|---|---|
| Run-offs onto a curved face; a run-off of a blend that takes material away; both ends running off | refused, naming the place | The setback patch is the means: where the ball runs off a face, cut the fillet as a setback is cut and fill the hole between it, the next face's fillet (or the face) and the faces round. A run-off taking material away was never reached by any test case (a slot cut from the cone never got that far) |
| Open chains ending on a curved face (the loft's foot edges, across a crease) | refused: "the ball doesn't touch the face" | Same patches, the end face's curve no longer planar: its side rule becomes the curved face's tangent plane along the curve, exact in the tau/sigma form only for ruled faces, fitted otherwise. Tried: the end section as the conic through the roots on the curved face, accepted within half the chord tolerance: no measured case gained |
| The loft's top ring, where creases fade to tangent | "meets the flat face almost along it" | The chain should go on round the ring across a crease that is smooth at that vertex; `blendNext` takes only edges smooth all along |
| A mitre of two radii over an edge left sharp | refused, naming the radii | Needs a crease in the blend, or the setback patch over the upright: the edges' cylinders meet tangentially where both touch the top face |
| A rib rounded all over: its top's edges rounded as well as its foot filled | refused | The foot filled below a sharp top is made (uneven mitre). Top first: the foot's chain meets the arc where the floor cuts a leaning cylinder, which only the rolling ball rounds, and that edge must be picked alone. Foot first: the top's edge ends on the foot's fillet and the grown top |
| Radii 5-6:1 side by side at a setback corner | 4 of 36 refused as folding (`TestFilletSetbackExtremeSweep`) | More patches, or a smaller setback for the small fillets. Tried and given up: one fitted patch over the hole (0.1 rad off at degree 9), a fan of quads, a star for even counts, stronger pulls, larger setbacks, net second differences as fairness |
| Chamfers at setback corners; convex meeting concave at one vertex beyond what the stages round | refused | No setback for chamfers yet |
| Chamfers of edges between flat faces that end on a curved face | refused | Routed like the fillets, they left 134 naked edges and edges run the same way; not traced |
| Ends on a dome that one degree-5 curve can't follow | 8 of 40 refused at r 1.5 to 2.5 (`TestFilletEndsOnCurvedFaceSweep`) | The dome's height round the corner is no quintic within tolerance (degree 5 misses by 3e-4, 6 by 2e-5); the end section should be several Béziers |
| A fillet of 3 on the narrowest band (run-off sweep) | 2 of 36 refused, cannot be fitted | |
| The vase's foot fillet against OpenCASCADE | ours -2.8% (0.011 mm³) | Not checked further |
| One box_hole.step edge | the ball skips a face cut from a shared patch between two scan samples (faces 7, 9, 8 round the hole) | |
| STEP cylinder fillet at chord 0.002% | the fit's misfit floors at 2.6e-5 against 2.4e-5 | More rounds change nothing; cause not found |
| Teapot handle, slot chamfer at 3, run-off chamfer crossings, band-on-cone r 1 | see K3 and K4 | |

Traps kept from the notes: OpenCASCADE is not the truth everywhere (on a hole of 5 in a pipe of
10, r 2, its fillet face lies up to 0.093 mm inside the balls' envelope; ours within 0.00024).
OCC's own Volume of our STEP files is off by up to 5% of a fillet: integrate slices in FreeCAD
instead (`scripts/fillet-step-volumes.py`). Choosing the ball's foot by trims among every patch
holding it, not only those sharing the nearest one's surface, made a sphere_bored fillet unsound.

### Solid feature leftovers

| Item | State | Approach / tried |
|---|---|---|
| Sweep corners next to an arc or curve; corners on twisted or fitted sweeps; closed sweeps | refused; corners between two lines are mitred | A plain mitre can't work: on the inside of the turn the halving plane cuts the arc's revolved sides. Tried twice and reverted: runs cut by a half-space box at the mitre plane and unioned (the union leaves a partial coplanar face with curved trims; 188 naked edges with a round profile); runs cut by a bent prism ("bent caps") and unioned (cuts sound, union fails on the touching faces, even between two lines: F5). What would work: both runs and the joint in one shell with shared exact curves, which needs line-prism against arc-revolve side intersection inside `MakeFromSweepOf`. Fitted sweeps need the end frame of a fitted run and twist split by length. F4 is the alternative users usually want |
| Shells of other curved solids (filleted, lofted, revolved) | prisms only | Offsets of patches (22.1) are the first part; faces whose offsets do not cross themselves can be sized. The general shell is open-ended (E12, parked) |

## Waiting for the user

| Item | State |
|---|---|
| What to take back from "Parked" | Every row there is a proposal; F5 contact (now K3) blocks items above |
| What the review took in without being asked | B3 (16.11), G4 (17.4) and S3 (17.5) are built; the user may still disown them |

## Parked

Proposed on 2026-10-03; each stays out until its reason stops holding.

| Item | Reason |
|---|---|
| E12 general shell | Open-ended. With 22.1's offsets, faces whose offsets do not cross themselves can be sized |
| Topology made by the kernel (9.2) | Derived topology costs 7 ms for 270 faces; no feature has needed identities kept through a Boolean |
| OpenGL on Windows and macOS | takigo has backends for both; a port is a project of its own |
| The section view saved in files | View state; goes with saved views, which nothing plans |
| Open thread ends in STEP | The file is open there; closing it would invent geometry |
| CSG and swept solids in STEP | No vendor file in the corpus has them. The export half (the group tree as CSG_SOLID / BOOLEAN_RESULT, B-rep operands as fallback, as BRL-CAD's `g-step --native-csg`) is cheap once the tree is walkable |
| A chat window, MCP resources and prompts, an HTTP transport, a sandbox | "An MCP server, and nothing else" stands until a user needs otherwise. Ideas if it moves: find-style model search, a machine-readable operation catalogue, shotline reports (K2) |
| `make ai-eval` | Needs a model and an account, outside CI; take it when the primer next changes |
| Tolerances per edge and vertex | 21.2 heals within a file's tolerance; more only if the measured rate asks |
| Generated entities left out of `.tlspr` | Possible behind the version number; no need has come up |
| Threads: tapered, variable pitch, run-out, cosmetic, one parametric group | Never asked for |
| Scripting: a startup script, `--watch`, menu items made by scripts, zoom to fit | Never asked for |
| The bit-exact scaffolding: `pkg/libm`, `CxxSort`, the FMA build rules | Harmless; replacing it moves every golden for nothing |
| A mesh Boolean fallback with perturbation | Conflicts with "never silently wrong geometry"; only as an explicit approximate mode |
| Sheet metal, direct modelling, tolerancing, CAM, simulation | Each an XL product of its own |
