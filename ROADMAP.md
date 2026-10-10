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
| 1 | [K3 Coincident and tangent contact (F5)](#k3-coincident-and-tangent-contact-f5) | kernel foundation | ? (M) | Four cases done 2026-10-10 and 11; two of a ring touching a pot left, and three for K4 |
| 2 | [F5 Small UI and CLI conveniences](#f5-small-ui-and-cli-conveniences) | feature | S each | Usability, not capability |
| 3 | [K5 STEP repair and accounting rules](#k5-step-repair-and-accounting-rules) | kernel | ? (M) | Corpus 7 of 9 sound; NIST_6 and Schenkel, both sound with K4's mesher |
| 4 | [Fillet leftovers](#fillet-leftovers) | kernel | ? | Each refused case today is a clear message, not bad geometry |
| 5 | [Solid feature leftovers](#solid-feature-leftovers) | kernel | ? | The sweep joint is not a contact case after all |
| 6 | [K4 Constrained Delaunay mesher](#k4-constrained-delaunay-mesher) | kernel foundation | M | Postponed 2026-10-10 after its spike: sound where the ear clipper isn't, at 3 to 5 times the triangles |
| 7 | [F6 IGES import and export](#f6-iges-import-and-export) | feature | L | Only if asked for |

K3 opens what touches. K4, postponed, would open most of the mesher's open items; the dependency
column in each table below names which.

## Features

### F5 Small UI and CLI conveniences

From the neosolve survey; each is small and independent.

| Item | State |
|---|---|
| The ray-cast oracle (K2) for more groups: helices, sweeps, lofts | extrusions, lathes, revolves, and since 2026-10-10 step and repeats, patterns along a path and mirrors of them (`sketch.Group.CopyPlacements`), with moments of inertia; since 2026-10-11 drafted extrusions (`pkg/check/raycast_draft.go`: the outline at each height, per-side angles, a neutral plane, arcs to cones); the rest are refused by name (`check.ErrUnsupported`). Helices, sweeps and lofts need a ray against a curved swept boundary |
| Dockable property browser | takigo decides; lowest |

### F6 IGES import and export

Neosolve reads and writes IGES. Low priority: STEP covers the exchange; take it when a user asks.

## Kernel foundations

These come before the open defects they unblock. Recommended by the BRL-CAD survey of
2026-10-07 (BRL-CAD checkout surveyed read-only; the libraries are LGPL 2.1, so ideas are
re-implemented in Go, no cgo). Each starts as a two-day spike with a fixed budget.

### K3 Coincident and tangent contact (F5)

F5 in general never finishes, so it is taken case by case, each as a fixture with a sweep
(`pkg/srf/contact_test.go`, `latheseam_test.go`). Done 2026-10-10 (`a5bf482`): a sheared flat
patch clipped a plane-plane line short (the leaning wedge, 108 of 252 to none; the rib fillet
sweep takes leans either way); a face lying on another's coplanar patches is split at their seams
(`fuseCoplanarSeam`; the off-axis bore 40 to 20 of 80); a march from where seams of both operands
cross no longer follows a patch past its edge (`onBothPatches`), and stops at the first seed it
reaches (a ring joined to a lathed pot, `TestHandleOnPotSweep`, 15 to 9 of 144). Caps meeting
caps with equal curved outlines unite (`TestMitredBarsUnion`); the sweep corners' failure was the
half-space and bent-cap cuts, not the contact. Done 2026-10-11 (a two-day spike's first day): where
a ring's patch corner lies on the pot's seam and the curve runs from it along the ring's seam within
the chord tolerance, the march on the patch beside the curve's own copied it and used up the seed
the rest of that patch's curve starts from (`marchOnBothPatches`: a march's points between its seeds
must lie on both patches within LengthEps); and on its own patch the curve's chord lay on the seam,
trimming a sliver of nothing (`SEdgeList.hasReversed`, `SSurface.offChord`: the two cancel). The
handle 9 to 5 of 144.

Tried: fusing every curve vertex within LengthEps before trimming changed no sweep. Splitting the
operands' trims at the intersection curves' ends that lie within the chord tolerance of them (the
seam's chord missing the pot) fixed no handle case once the two rules above were in, and split
coincident outlines on one side only; marching a seed left alone to a used one, and dropping
marches that reach no seed, gained nothing either.

Approach for what is left (BRL-CAD libnmg): **fuse first, then classify by identity.** Before
intersecting, fuse the two shells' vertices within tolerance, split edges at vertices lying on
them, make coplanar faces share one plane, fuse edges. After the cut an element is ON when the
other shell uses the same vertex or edge; split ON into on-shared (same direction) and on-anti
(opposite) and decide each Boolean by a table: union keeps shared, drops anti; difference keeps
anti; intersection keeps shared. Edges with both ends ON are classified at 0.5, then 0.1, then
0.9. Extend `keepEdge` / `keepRegion` (`pkg/srf/boolean_trim.go`) with the split. For curved
multi-patch overlaps (libbrep): the overlap's boundary lies on isocurves at the patches' knots;
intersect those with the other surface and chain them into loops. Point-in-solid by several rays
with explicit valid / grazing / coincident / failed results instead of a vote.

| Open item | State today |
|---|---|
| A ring on a pot, tilted, a patch's corner on the pot's seam | 2 of 144 (`TestHandleOnPotSweep`; 3 more are K4's, trims meshed crossing themselves): a curve dips into the pot's patch beside its seam deeper than the classification's probe (the chord tolerance), so the seam reads as inside the ring on both sides; and a march ends 1e-5 from the seam's vertex, under the chord tolerance but over LengthEps, leaving an edge of 1e-5 the outline can't assemble |
| Sweep corners beside an arc (see [Solid feature leftovers](#solid-feature-leftovers)) | needs the joint inside `MakeFromSweepOf`, not a Boolean of cut runs |

A last resort, not recommended: BRL-CAD's mesh Boolean with deterministic perturbation of
touching operands. It conflicts with "never silently wrong geometry" and could only be an
explicit, reported "approximate" mode.

### K4 Constrained Delaunay mesher

Postponed 2026-10-10. A two-day spike built it behind a switch, off by default, on the branch
`k4-cdt-spike`, which is kept unmerged for reference. `docs/notes/k4-cdt-spike.md` has the
design, the measurements, what was tried and what making it the default needs.

`UvTriangulateInto` (`pkg/srf/triangulate.go`) ear-clips a uv outline against grid points, and
every step 24 to 26 fix was an ear-ordering rule (`grazed`, `folded`, `cutOff.crosses`,
`poleSliver`, `AvoidFlatEars`, `simple`). Its weakness is fans: a patch ruled one way, or a
trimmed corner, has few interior points, so the clipper fans long chords from one trim point and
a chord, straight in xyz where its uv image bends by its sag, passes a trim point on the wrong
side.

The spike's mesher is a constrained Delaunay triangulation on `pkg/internal/cdt`. It uses a chart
in millimetres (polar about a pole), quadtree points sized by the surface's sag and the trim's
segments, and refinement rounds. With it on every face, against the ear clipper:

- the twisted bar's volume error is 0.003% (1.6%);
- the off-axis bore sweep fails 0 of 80 (20);
- the handle on the pot fails 6 of 144 (9);
- the run-off sweep has no crossing (2 of 36 crossed);
- the STEP corpus reads 9 of 9 as sound (7).

It costs 3 to 5 times the triangles, and `pkg/topo`'s tests take 260 s against 77 s. To take it
up again: bring the cost down, then regenerate every golden with a curved face.

It would unblock these items, which stay open with the ear clipper:

| Open item | State today, with the ear clipper |
|---|---|
| A twisted swept bar | flat sides mesh within 1.6% of volume |
| NIST_MBE_PMI_6 as a solid | mesh Euler number 6 off the shell's (see K5) |
| Run-off chamfers crossing the cone (`TestRunOffSweep`) | 2 of 36 cross by one edge each |
| A bore off a lathed tip's axis that breaks out through its side (`TestBoreToLathedTipSweep`) | 20 of 80 |
| A chamfer of 3 where a slot meets a cone | 6 naked edges, on the slot's flat walls; with K4's mesher none, but one topology problem left |

The band-on-cone fillet at r 1 and chord 0.001%, listed here before, is sound with the ear
clipper at 0.001% to 0.005%.

### K5 STEP repair and accounting rules

STEP corpus: 7 of 9 read as sound solids. From BRL-CAD's `conv/step` (`STEPImportArchitecture.md`,
`STEPBrepSeamRepair.cpp`): accept in model space by the file's declared or a measured tolerance,
never a solver threshold; a ladder of faithful construction, safe repair, tagged inference;
exactly one accounted result per entity, so the report says what was repaired or lost. Try
`repair_missing_singular_trims`' approach on the two below.

| Open item | State today |
|---|---|
| NIST_MBE_PMI_6 | the pole topology problem under K4; sound with the spike's mesher (polar chart). STEP's `newtonOnSurface` keeps its own steps: with the kernel's projection (`srf.ProjectFrom`, K1) the trims moved and the pole slivers grew, the mesh's Euler number off by 10, not 6; switch it after K4 |
| Schenkel | mesh closed and not self-crossing (`TestSolidSchenkelClosed`), no naked edges since `ed03d3b`, but the shell's Euler number is -34 against the mesh's -6; cause not found, so not counted sound; sound with K4's mesher, so likely the mesh's |

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
| Teapot handle (tilted, touching), slot chamfer at 3, run-off chamfer crossings | see K3 and K4 | |

The robust projection of K1 (`srf.SSurface.Project`, `srf.ProjectFrom`) is not what the rolling
ball's feet use: most points it projects are on faces far from the ball, where plain Gauss-Newton
steps don't settle and the answer is discarded, and refining them made `TestFilletOnOtherSolids`
four times slower (7.4 million projections against 1.5) with no fillet gained. Use it where a
projection must be right, not where it is a probe.

Traps kept from the notes: OpenCASCADE is not the truth everywhere (on a hole of 5 in a pipe of
10, r 2, its fillet face lies up to 0.093 mm inside the balls' envelope; ours within 0.00024).
OCC's own Volume of our STEP files is off by up to 5% of a fillet: integrate slices in FreeCAD
instead (`scripts/fillet-step-volumes.py`). Choosing the ball's foot by trims among every patch
holding it, not only those sharing the nearest one's surface, made a sphere_bored fillet unsound.

### Solid feature leftovers

| Item | State | Approach / tried |
|---|---|---|
| Mitred sweep corners next to an arc or curve; corners on twisted or fitted sweeps; closed sweeps | refused; corners between two lines are mitred, and any corner beside lines or arcs rounded with a corner radius (F4) | A plain mitre can't work: on the inside of the turn the halving plane cuts the arc's revolved sides. Tried twice and reverted: runs cut by a half-space box at the mitre plane and unioned (the union leaves a partial coplanar face with curved trims; 188 naked edges with a round profile); runs cut by a bent prism ("bent caps") and unioned (cuts sound, union fails on the touching faces, even between two lines; whole caps from one ellipse unite, `TestMitredBarsUnion`, so it is the cut caps, not contact). What would work: both runs and the joint in one shell with shared exact curves, which needs line-prism against arc-revolve side intersection inside `MakeFromSweepOf`. Fitted sweeps need the end frame of a fitted run and twist split by length. A corner radius (F4, done) rounds corners beside arcs, which is what users usually want |
| Shells of other curved solids (filleted, lofted, revolved) | prisms only | Offsets of patches (22.1) are the first part; faces whose offsets do not cross themselves can be sized. The general shell is open-ended (E12, parked) |

## Waiting for the user

| Item | State |
|---|---|
| What to take back from "Parked" | Every row there is a proposal |
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
