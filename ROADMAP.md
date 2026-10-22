# Roadmap

What is left to build, as of 2026-10-11. Only open work is listed; what is built is described in
`doc/guide`, `doc/ai.md` and `doc/done-audit.md`, and `doc/done-audit.md` holds the definition
of done every feature meets. The plan of steps 1 to 29 and the kernel's working notes were
`ROADMAP-JUMP.md` and `doc/notes/` up to commit `e0004fc` (`git show e0004fc:ROADMAP-JUMP.md`);
code comments that cite "ROADMAP-JUMP.md", "package 5.1" or "roadmap 24.2" refer to that file.

Sizes are focused weeks for one developer who knows the code, the definition of done included:
**S** under a week, **M** one to three, **L** three to eight, **XL** more, **?** not sized. Read
them as relative weights. A kernel item is not sized until a two-day spike on its hardest
fixture has been done; its "Tried" notes say what not to try again.

## Order of work

| # | Item | Kind | Size | Why here |
|---|---|---|---|---|
| 1 | [Fillet leftovers](#fillet-leftovers) | kernel | ? | Each refused case today is a clear message, not bad geometry |
| 2 | [Solid feature leftovers](#solid-feature-leftovers) | kernel | ? | The sweep joint needs the line-arc joint inside the sweep |
| 2a | [Modelling upgrade leftovers](#modelling-upgrade-leftovers) | kernel and feature | ? | What the 2026-10-23 work packages left; each refuses with a message today |
| 3 | [F5 Small UI and CLI conveniences](#f5-small-ui-and-cli-conveniences) | feature | S each | Usability, not capability |
| 4 | [K4 Constrained Delaunay mesher](#k4-constrained-delaunay-mesher) | kernel foundation | M | Since 2026-10-11 it mends the ear clipper's crossing faces; as the default it still costs 3 to 5 times the triangles |
| 5 | [F6 IGES import and export](#f6-iges-import-and-export) | feature | L | Only if asked for |

K3 (contact) and K5 (STEP) are closed: every case they measured is sound, the last ones with K4's
mender. Their notes stay below for the next case. K4, postponed as the default, would open the
mesher's open items that are not crossings; the dependency column in each table below names which.

## Features

### F5 Small UI and CLI conveniences

From the neosolve survey; each is small and independent.

| Item | State |
|---|---|
| The ray-cast oracle (K2) for lofts | extrusions, lathes, revolves, and since 2026-10-10 step and repeats, patterns along a path and mirrors of them (`sketch.Group.CopyPlacements`), with moments of inertia; since 2026-10-11 drafted extrusions (`pkg/check/raycast_draft.go`), helices of a profile in a plane through the axis (`raycast_helix.go`: inside exact, crossings sampled along the ray at steps sized by how fast its image crosses the profile, the samples kept as cuts) and sweeps along lines and arcs, mitred or rounded (`raycast_sweep.go`, from `sketch.Group.SweepSteps`: prisms cut by the mitre planes, revolutions). Refused by name (`check.ErrUnsupported`): lofts, whose surfaces between sections are the kernel's own construction (rebuilding them would repeat it, not check it), sweeps fitted along cubics, twisted or held to a direction, and a profile leaning to a helix's or an arc's axis |
| Dockable property browser | takigo decides; lowest |

### F6 IGES import and export

Neosolve reads and writes IGES. Low priority: STEP covers the exchange; take it when a user asks.

## Kernel foundations

These come before the open defects they unblock. Recommended by the BRL-CAD survey of
2026-10-07 (BRL-CAD checkout surveyed read-only; the libraries are LGPL 2.1, so ideas are
re-implemented in Go, no cgo). Each starts as a two-day spike with a fixed budget.

### K3 Coincident and tangent contact (F5)

Closed 2026-10-11: the bore sweep, the handle on the pot and the leaning wedge fail in none of
their cases, the last of them mended by K4's mesher. What follows is kept for the next case.

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
handle 9 to 5 of 144. Done 2026-10-11 (the spike's second day), the handle 5 to 3, the three left
K4's: where the ring's seam passes over the pot's close to where the curve crosses both, the curves
were split three or four times within 4e-5 mm, one point in the patches' uv, so a trim dropped the
segment and one side's mesh lacked a vertex the other had; vertices of a Boolean's curves within a
hundredth of the chord tolerance (and further apart than the 1e-5 a split at a curve's end already
ignores) are now one point (`SShell.fuseVertices`, `boolean_fuse.go`, a first piece of "fuse first"
below). And where the ring dips into the pot's patch beside the seam by less than the chord
tolerance, the curve there is a chord on the seam: the seam was dropped as inside the ring and the
curve as parallel to it; the curve now stands in for the dropped seam (`SEdgeList.hasSame`).

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

The sweep corners beside an arc, once listed here, are no contact case: they need the joint inside
`MakeFromSweepOf`, not a Boolean of cut runs (see [Solid feature leftovers](#solid-feature-leftovers)).

A last resort, not recommended: BRL-CAD's mesh Boolean with deterministic perturbation of
touching operands. It conflicts with "never silently wrong geometry" and could only be an
explicit, reported "approximate" mode.

### K4 Constrained Delaunay mesher

Postponed 2026-10-10 as the default. A two-day spike built it behind a switch, off by default, on
the branch `k4-cdt-spike`. `doc/notes/k4-cdt-spike.md` has the design, the measurements, what
was tried and what making it the default needs.

Since 2026-10-11 it is merged as a mender (`pkg/srf/triangulate_mend.go`, `triangulate_check.go`):
the ear clipper meshes every face as before, a face whose chords part from their uv segments' images
by more than four chord tolerances is a candidate, and a candidate whose triangles cross the shell's
mesh, or leave an edge with no mate, is meshed again by the CDT, as is a face holding a triangle
another face holds too (K5). A sound mesh is left as it is, so
no golden moved. The bore sweep went from 20 of 80 failing to none, the handle on the pot from 3 of
144 to none, and the run-off chamfers' crossings are gone (2 of 36 left, refused fillets). The
measure alone could not choose the faces: sound faces part as far (a sphere's patches 18 chord
tolerances, a loft's 43), and their goldens moved when it did.

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

As the default it would close these items, which the mender leaves open (none is a crossing):

| Open item | State today, with the ear clipper and the mender |
|---|---|
| A twisted swept bar | flat sides mesh within 1.6% of volume |
| A chamfer of 3 where a slot meets a cone | 6 naked edges, on the slot's flat walls; with K4's mesher none, but one topology problem left |

The band-on-cone fillet at r 1 and chord 0.001%, listed here before, is sound with the ear
clipper at 0.001% to 0.005%.

### K5 STEP repair and accounting rules

Closed 2026-10-11: the STEP corpus reads 9 of 9 as sound solids. NIST_MBE_PMI_6 at its poles and
Schenkel where two faces fan from one corner along the curve they share had the same fault: the ear
clipper put the same sliver triangle in both faces' meshes, which then closed and mated edge for edge
but were off the shell's Euler number. The mender now takes a face holding a triangle another face
holds too (`srf.stackedFaces`) and meshes it by the CDT; no golden moved.

For the next file that fails, from BRL-CAD's `conv/step` (`STEPImportArchitecture.md`,
`STEPBrepSeamRepair.cpp`): accept in model space by the file's declared or a measured tolerance,
never a solver threshold; a ladder of faithful construction, safe repair, tagged inference;
exactly one accounted result per entity, so the report says what was repaired or lost.
`repair_missing_singular_trims` is the approach for poles. STEP's `newtonOnSurface` keeps its own
steps: with the kernel's projection (`srf.ProjectFrom`, K1) NIST_MBE_PMI_6's trims moved and its
pole slivers grew.

## Open defects

### Fillet leftovers

Code: `pkg/topo/rollingball.go`, `rollingend.go`, `rollingrunoff.go`, `rollingmitre.go`,
`rollingchains.go`, `fillet_setback.go`, `fillet_network.go`. Every case below is refused with a message today, except where the State
column says unsound. Measure: `TestFilletOnOtherSolids` (fillets on lofts, sweeps and STEP
solids) at 55 of 59 sound; the 4 left are the loft's creases, which fade to tangent at its top (a fillet
that vanishes along its edge).

| Item | State | Approach / tried |
|---|---|---|
| Run-offs onto a curved face; a run-off of a blend that takes material away; both ends running off | refused, naming the place | The setback patch is the means: where the ball runs off a face, cut the fillet as a setback is cut and fill the hole between it, the next face's fillet (or the face) and the faces round. A run-off taking material away was never reached by any test case (a slot cut from the cone never got that far) |
| Mitres where only some of a rim's chains are picked: a chain mitred at one end, ending on the unpicked chain's wall at the other | refused: "pick it alone" | Per-end mitres: `mitred` and `nearEnd` take both ends alike today; a mitred end beside a curved end (`curvedEndSection`) |
| Chamfers meeting at a crease, of one chain or several | refused: "are not mitred" | The mitre is found from the ball's centres; a chamfer's would be where the two ruled surfaces, carried on, meet |
| Mitres between chains of different radii | refused: "pick it alone" | The height against the other chain's centres less its own radius; the mitre is then no longer in the plane that halves the turn |
| A rim's mitre below about 0.005% chord | refused: "the fillet's sections near its mitre can't be fitted" | At the cap the mitre runs nearly along the chain; tried: starting the fit from chord-length parameters (worse). More spans than two, or sections fitted against the mitre's own parameter |
| A mitre of two radii over an edge left sharp | refused, naming the radii | Needs a crease in the blend, or the setback patch over the upright: the edges' cylinders meet tangentially where both touch the top face |
| A rib rounded all over: its top's edges rounded as well as its foot filled | refused | The foot filled below a sharp top is made (uneven mitre). Top first: the foot's chain meets the arc where the floor cuts a leaning cylinder, which only the rolling ball rounds, and that edge must be picked alone. Foot first: the top's edge ends on the foot's fillet and the grown top |
| Radii 5-6:1 side by side at a setback corner | 4 of 36 refused as folding (`TestFilletSetbackExtremeSweep`) | More patches, or a smaller setback for the small fillets. Tried and given up: one fitted patch over the hole (0.1 rad off at degree 9), a fan of quads, a star for even counts, stronger pulls, larger setbacks, net second differences as fairness |
| Chamfers at setback corners; convex meeting concave at one vertex beyond what the stages round | refused | No setback for chamfers yet |
| Chamfers of edges between flat faces that end on a curved face | refused | Routed like the fillets, they left 134 naked edges and edges run the same way; not traced |
| The vase's foot fillet against OpenCASCADE | ours -2.8% (0.011 mm³) | Not checked further |
| Slot chamfer at 3 | see K4 | The teapot handle and the run-off chamfers' crossings, listed here before, are sound since K4's mender |

Done 2026-10-22: the loft's top ring, four edges between the flat top and side patches that meet
in creases fading out at the ring, refused as "meets the flat face almost along it": a chain now
goes on past a crease whose faces meet tangentially at the vertex (`blendNext`, `joinedAt`,
`smoothAt`), and a foot on a patch's edge there is allowed the sag of `seamCos`. The ring's fillet
of 2 is sound and takes what OpenCASCADE's takes, within 0.01% (`TestFilletLoftTopRing`);
`TestFilletOnOtherSolids` 51 → 55 of 59.

Done 2026-10-22: the loft's foot edges, open chains ending across a crease on the next side
patch, refused as "the ball doesn't touch the face": near the corner the side patch is twisted
and the feet's plain Gauss-Newton steps overshot and stopped off the foot (3.9 from the centre for a
ball of 2). Long steps that take the point further away are now halved (`nearestOnWithin`), and a
section whose feet still don't settle is made again with them projected by
`srf.SSurface.Project`. All four are sound, `TestFilletOnOtherSolids` 47 → 51 of 59. Our fillet's
surface lies within 4e-6 mm of OpenCASCADE's along the run (its own result is not valid there). The
loft's feet take about 20 s each, most of it the retried sections. At chord 0.002% the end is
refused ("can't be fitted … make it smaller").

Done 2026-10-22: rolling-ball edges picked with others whose fillets don't reach them (an edge
between curved faces, or between flat faces ending on a curved one), refused as "pick it alone"
before: the picks step 10 takes are rounded first, together, then each rolling one alone in the
solid as it is then, found again along its line (`pkg/topo/fillet_apart.go`). The lid's four
uprights picked together take what each takes alone, within 0.15% of OpenCASCADE
(`TestFilletEndsOnCurvedFaceApart`); a lens's top and bottom chains, the vertices of each as
alone (`TestFilletChainsNotMitred`). Where a rolling chain's vertex is another pick's, the two
fillets meet and the pick is refused at that vertex.

Done 2026-10-11: rims across a sharp upright edge of a curved wall, blob_extruded's two (each one
closed edge turning 130° where its B-spline wall has a crease): the chain runs from the crease round
to it, and its ends meet in a mitre (`pkg/topo/rollingmitre.go`), where the surfaces the ball sweeps
either way, carried on, meet. Near each end the ball keeps off the wall across the crease; the mitre
is fitted where one swept surface meets the other (a root along the chain on the wall, the nearest
approach on the cap the two share, where they meet tangentially); the sections between it and the
ball's own lie on the swept surface, their arc lengths moved from the mitre's at each point. Its
vertices are OpenCASCADE's and our surfaces lie within 8e-5 mm of its result; r 0.5 and 1 are sound
from chord 0.5% to 0.005%; r 1.5 and 2 are refused, as OpenCASCADE refuses them; below about 0.005%
the sections near the mitre miss their fit (it runs nearly along the chain at the cap). A chain is
closed only through a smooth join (`blendChain`).

Done 2026-10-11: mitres between different chains, a rim that turns sharply at two or more creases of
its wall, each run between two creases a chain of its own, picked together
(`pkg/topo/rollingchains.go`). Each end's mate is the end it meets (`rbEnd.mate`): its height is
measured against the mate chain's centres, the end fitted first fits the mitre and the other copies
it, every chain is blended with split sections or none, and the faces of every chain are cut back
before the solid is checked once. A lens of two cubics, and a profile of three: within 6e-6 of
OpenCASCADE's volume, every point within 2e-4 mm of its fillet, its mitres' vertices
(`TestFilletChainsMitred`).

Done 2026-10-11: the box_hole.step edge whose ball passes a vertex where three of the hole's patches
meet: the scan between two samples now walks every face the ball's point crosses, and a face it is on
for no length is a vertex passed (`rollingBall.crossings`); box_hole fillets all 14 edges.

Done 2026-10-11: fillets ending on a dome round a corner too wide for one quintic (8 of 40 refused,
`TestFilletEndsOnCurvedFaceSweep`, now none): when the end section can't be fitted, the blend is
made again with every section split at its middle into two quintic spans (`rollingBall.splitEnds`),
two rows of patches along the edge, within 0.17% of OpenCASCADE at radius 2.5. A run-off at the
other end of such a fillet is refused.

Done 2026-10-11: the run-off sweep's 2 refused fillets of 3 (`TestRunOffFittedCuts`): along a
curved edge the swept surface is no tube and the end sections' conics strayed from it by 1.6 times
the tolerance however close the sections; when they do, the end sections are refitted as quintics
to where the swept surface meets their planes (`rollingBall.fitCuts`, `planeCutSection`), and a
first try that stalls between conics stops early. And the STEP cylinder at chord 0.002%
(`TestFilletCylinderFineTolerance`): each section put in tipped the span beside it over the
tolerance, one a round round the edge; from the third round spans over three quarters of it are
split too (`refineNear`). Neither moved a golden.

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

Done 2026-10-11: a helix's patches were 2% of a quarter turn's rise off the true helix (0.1 mm on a
helix rising 30 mm a turn), their rise following the arc's rational parameter, not its angle. They
are raised to degree 5 along the turn and their rise fitted to the angle, within 3e-6 of it
(`pkg/srf/helix_rise.go`); `TestOracleHelix` holds the mesh to 0.03 mm at chord 0.01%. Thread
goldens take 17% longer.

| Item | State | Approach / tried |
|---|---|---|
| Mitred sweep corners next to an arc or curve; corners on twisted or fitted sweeps | refused; corners between two lines are mitred, and any corner beside lines or arcs rounded with a corner radius (F4). Closed paths are swept since 2026-10-22 (WP-C4): started where the profile crosses the path, so the closing joint is smooth; exact round planar lines and arcs, mitres and corner radii included; fitted round cubics, twists of whole turns and paths out of a plane, the frames' turn taken out along the path | A plain mitre can't work: on the inside of the turn the halving plane cuts the arc's revolved sides. Tried twice and reverted: runs cut by a half-space box at the mitre plane and unioned (the union leaves a partial coplanar face with curved trims; 188 naked edges with a round profile); runs cut by a bent prism ("bent caps") and unioned (cuts sound, union fails on the touching faces, even between two lines; whole caps from one ellipse unite, `TestMitredBarsUnion`, so it is the cut caps, not contact). What would work: both runs and the joint in one shell with shared exact curves, which needs line-prism against arc-revolve side intersection inside `MakeFromSweepOf`. Fitted sweeps need the end frame of a fitted run and twist split by length. A corner radius (F4, done) rounds corners beside arcs, which is what users usually want |
| Shells of other curved solids (filleted, lofted) | prisms, and since 2026-10-22 (WP-C1) one lathe or revolve of lines and arcs, by its offset profile | Offsets of patches (22.1) are the first part; faces whose offsets do not cross themselves can be sized. The general shell is open-ended (E12, parked). Left from WP-C1: a lathe joined to other solids (needs the profile from the topology), cones made by an extrusion's draft, revolves of more than half a turn with an end kept, curved faces opened at an inside corner |

### Modelling upgrade leftovers

What the work packages of `doc/notes/modelling_upgrades.md` (merged 2026-10-23) left open. Their
working notes are at commit `1e84ca0c` (`git show 1e84ca0c:doc/notes/wrap-emboss.md`, and
`model-to-script.md`, `wp-b1-b2.md`, `wp-b3-b4.md`, `wp-c3.md`, `wp-c4.md`, `wp-d1.md`,
`wp-scripting.md` beside it).

| Item | State | Approach / tried |
|---|---|---|
| Wrap onto a cone, deep (C2) | 14 of 72 text cases refused, all at depth 0.5 of the letter height; cylinders 0 of 72 | At a corner of the profile the face's trim doesn't assemble: marched curves end 1e-7 apart in uv, or a closed curve's two ends miss (`boolean_trim.go`, the curve ends of `surfinter.go`) |
| Wrap: a thin wall under an emboss; loop sets wider than 170° | the wall isn't detected when the overlap crosses it; wide loop sets refused | Split a band wrapped right round into patches in place of refusing |
| Closed lofts; a guided loft along a solid's sides; tangency at a middle profile (C4) | refused | Closed sweeps' periodic frame (`pkg/topo/pathsweep.go`) is the pattern for the first |
| Fillets onto curved faces (C3) | 55 of 59 of `TestFilletOnOtherSolids` sound | Setbacks (`fillet_setback.go`) onto curved end faces, reached by no measured case; picks meeting at a vertex are refused there; chamfers of edges ending on a curved face (`filletApart` is for fillets only). Run-offs are under [Fillet leftovers](#fillet-leftovers) |
| Thread fade-out (run-out); multi-start with every start at one height (B1) | not done | A fade-out needs groups after the helix (a cone cut), so a thread would no longer be four groups; starts at one height need a sketch and a helix per start |
| Cosmetic threads in sections and around other features (B2) | sections draw threads turn by turn; anything inside a thread's band (a cross hole) is dropped with the turns; the iso view shows no root | Hatching to the root and crest in sections |
| Datums (B4) | a datum names an entity, so faces made by no entity (a fillet's, a Boolean's) and edges of the solid can't be named; mating two names in the window is a selection from the datums screen, then the mate | A datum by `topo.Name`; a "mate these names" dialog |
| Clearance and walls (D1) | thin spots between triangle middles are missed; gaps to parts made only of a mesh aren't followed onto surfaces; solids within one linked part aren't checked against each other; the MCP `measure` tool reports neither | |
| Model to script (D5) | 267 of 296 fixtures come back (`scriptRoundTripKnown`, which may only shrink) | Not written: images, splines through more points (`AddSplinePoint` places a point in the nearest span), constraints on four entities (`Constrain.Raw` takes two), request styles, a face's own appearance; the MCP `export` tool doesn't write `.go` |
| Regeneration (D4) | recommendations 3, 6, 12 and 13 and a solid cache taken up, 9 behind a setting that is off, 11 in part (`doc/notes/performance-study-2.md`, "Taken up") | The rest of the list (1, 2, 4, 5, 7, 8, 10, 14 to 18) was not asked for; 2 (a rank test that is not a full QR) is now most of a drag step's solve |

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
| A chat window, MCP resources and prompts, an HTTP transport | "An MCP server, and nothing else" stands until a user needs otherwise. The server's tools grew on 2026-10-22 at the user's asking (`doc/notes/modelling_upgrades.md`, item 16): measure, export, open, list_files, and a section on render; AI scripts' files are kept to allowed folders (`doc/ai.md`). Ideas if it moves further: find-style model search, a machine-readable operation catalogue, shotline reports (K2) |
| `make ai-eval` | Needs a model and an account, outside CI; take it when the primer next changes |
| Tolerances per edge and vertex | 21.2 heals within a file's tolerance; more only if the measured rate asks |
| Generated entities left out of `.tlspr` | Possible behind the version number; no need has come up |
| Threads: tapered, variable pitch, run-out | Never asked for. A thread is edited as one, with shaped crests, and drawn by the convention on drawings, since B1 and B2 of `doc/notes/modelling_upgrades.md` |
| Scripting: a startup script, `--watch`, menu items made by scripts, zoom to fit | Never asked for |
| The bit-exact scaffolding: `pkg/libm`, `CxxSort`, the FMA build rules | Harmless; replacing it moves every golden for nothing |
| A mesh Boolean fallback with perturbation | Conflicts with "never silently wrong geometry"; only as an explicit approximate mode |
| Sheet metal, direct modelling, tolerancing, CAM, simulation | Each an XL product of its own |
