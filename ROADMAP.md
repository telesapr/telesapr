# Roadmap

What is left to build, as of 2026-10-22. Only open work is listed; what is built is described in
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
| 2 | [Solid feature leftovers](#solid-feature-leftovers) | kernel | ? | Shells of filleted and lofted solids; corners on twisted and fitted sweeps |
| 2a | [Modelling upgrade leftovers](#modelling-upgrade-leftovers) | kernel and feature | ? | What the 2026-10-22 work packages left; each refuses with a message today |
| 2b | [Thread leftovers](#thread-leftovers) | kernel and feature | ? | What the thread improvements of 2026-10-23 left: ACME classes, buttress classes and rounded crests |
| 3 | [F5 Small UI and CLI conveniences](#f5-small-ui-and-cli-conveniences) | feature | S each | Usability, not capability |
| 4 | [K4 Constrained Delaunay mesher](#k4-constrained-delaunay-mesher) | kernel foundation | M | Since 2026-10-11 it mends the ear clipper's crossing faces; as the default it still costs 3 to 5 times the triangles |
| 5 | [F6 IGES import and export](#f6-iges-import-and-export) | feature | L | Only if asked for |
| 6 | [F8 TeleSAPR in a browser](#f8-telesapr-in-a-browser) | feature | ? | A spike is planned; sized after it |

K3 (contact) and K5 (STEP) are closed: every case they measured is sound, the last ones with K4's
mender. Their notes stay below for the next case. K4, postponed as the default, would open the
mesher's open items that are not crossings; the dependency column in each table below names which.

## Features

### F5 Small UI and CLI conveniences

From the neosolve survey; each is small and independent.

| Item | State |
|---|---|
| The ray-cast oracle (K2) for lofts | extrusions, lathes, revolves, and since 2026-10-10 step and repeats, patterns along a path and mirrors of them (`sketch.Group.CopyPlacements`), with moments of inertia; since 2026-10-11 drafted extrusions (`pkg/check/raycast_draft.go`), helices of a profile in a plane through the axis (`raycast_helix.go`: inside exact, crossings sampled along the ray at steps sized by how fast its image crosses the profile, the samples kept as cuts) and sweeps along lines and arcs, mitred or rounded (`raycast_sweep.go`, from `sketch.Group.SweepSteps`: prisms cut by the mitre planes, revolutions); since 2026-10-29 fitted sweeps, along cubics, twisted, held to a direction, and round closed paths out of a plane (`raycast_pathsweep.go`, from `sketch.Group.FittedSweepPath`: the profile placed along the path's own curves by frames the oracle turns itself, the least rotation from each sample's tangent to the next's where the kernel reflects twice, the twist by arc length, a closed path's turn taken out; inside where the profile, moved back from a plane through the point, holds it; crossings sampled as a helix's; a straight piece beside a sharp corner a prism cut by the mitre, `raycast_pathcorner.go`), with moments of inertia: Pappus's volumes, the twisted bar's and ring's moments, and held frames giving the rigid steps' solid ray for ray (`TestOracleFittedSweep`, `TestOracleFittedHeldAsRigid`). Refused by name (`check.ErrUnsupported`): lofts, whose surfaces between sections are the kernel's own construction (rebuilding them would repeat it, not check it), and a profile leaning to a helix's or an arc's axis. A fitted sweep's rays cost about ten times a rigid one's: a Ray-Cast Check of one takes tens of seconds |
| Dockable property browser | takigo decides; lowest |

### F6 IGES import and export

Neosolve reads and writes IGES. Low priority: STEP covers the exchange; take it when a user asks.

### F8 TeleSAPR in a browser

Go to WebAssembly: the kernel and the software renderer are pure Go, and `pkg/ui` is written
against `pkg/platform`, so a `web` platform would sit beside `tkgui` and `headless`. The spike is
planned, not started (`doc/notes/browser-wasm-spike.md`): compile the kernel and `pkg/render/soft`
for js/wasm, run yaegi there (the highest risk), measure a frame loop in a canvas, and settle fonts.

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

The sweep corners beside an arc, once listed here, were no contact case: they are joined inside
`MakeFromSweepOf` since 2026-10-29, not by a Boolean of cut runs (see [Solid feature leftovers](#solid-feature-leftovers)).

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
solids) at 59 of 59 sound since 2026-10-22. Chamfers: `TestChamferOnOtherSolids` at 59 of 59 since
2026-10-28.

| Item | State | Approach / tried |
|---|---|---|
| A run-off of a blend that takes material away; both ends running off where their cuts share a patch; a run-off onto a curved face only just past its edge, or with a curved end face; a chamfer's end running off both its faces | refused, naming the place | Run-offs onto a curved face are made since 2026-10-29 (below), and chamfers ending on a curved face the cone goes on past since 2026-10-29 (rd5, below). Both ends running off onto flat faces are made where the fillet is long enough that their cuts don't share a patch: a plate with a gabled top on a rod, its side's edges with the ends 0.58 high, fillets and chamfers of 1 to 2 run off at both ends, sound, on plates 2 to 20 long (probed 2026-10-29, not kept as a test); shorter ones are refused as "the fillet runs off the face past the next one too". For the rest the setback patch is the means: where the ball runs off a face, cut the fillet as a setback is cut and fill the hole between it, the next face's fillet (or the face) and the faces round. A run-off taking material away was never reached by any test case (a slot cut from the cone never got that far; a band raised off the cone's base, its chamfer's end on the band's bottom, is no run-off). "Only just runs off" is a sliver of the next face thinner than the chord tolerance; snapping p1 to w as on a flat face would move the curves' ends by up to the chord tolerance, past an edge's |
| A rib rounded all over, foot first | the top first is made (below), all 24 cases sound since 2026-10-29 (below) | Foot first, the top's edge ends on the foot's fillet and the grown top. Tried 2026-10-29, after run-offs onto curved faces were made: on a rib whose foot is filled, the edge across the back rounds alone, sound; the uprights at the back are refused as fillets meeting above an edge left sharp; the slope's edges are refused before their ends, the ball leaving the side face for the foot's fillet (three fillets meeting at a vertex: a vertex blend, not a run-off). Not needed while the stages take the top first. The blend patch of 2026-10-29 (two fillets above an edge left sharp, flat faces) is not that: there the three fillets meet on curved faces |
| Convex meeting concave at one vertex beyond what the stages round, fillets or chamfers | refused | Chamfer corners where every edge is chamfered are made since 2026-10-28 (`chamferCornerAt`: OpenCASCADE's pivot and square cuts, plane or bilinear patch), and so are two chamfers of any distances above a third edge left sharp (`chamferPairAt`). Since 2026-10-29 a chamfer's edge takes a distance and a second distance of its own (`EdgeRadii`, `EdgeSeconds`, `pkg/sketch/chamfer_edge_distances.go`), from its screen and from scripts. Since 2026-10-29 (below) a corner of three of both kinds, every edge chamfered, takes OpenCASCADE's pivot too, and a corner of four or more with some edges sharp is cut by the chamfers' planes. Since 2026-10-29 (below) a convex and a concave chamfer with the third edge sharp meet in a three-sided face, and a convex and a concave fillet above a third edge left sharp are blended by one patch, as OpenCASCADE's are. Since 2026-10-29 (below) that blend is made whichever radius is the larger, fillets at corners of three of both kinds whose two of one kind differ in radius are made, the radius varying along the arc between them, and fillets at corners of four or more of both kinds, every edge rounded, are set back. Left: at a corner of four or more of both kinds, fillets with an edge left sharp (OpenCASCADE makes the saddle's with one vertex blend whatever is picked) and chamfers (OpenCASCADE: one B-spline patch; our cone cut takes convex cones only, and the setback's patches are tangent to fillets, not to a chamfer's plane) |
| Slot chamfer at 3 | see K4 | The teapot handle and the run-off chamfers' crossings, listed here before, are sound since K4's mender |

Done 2026-10-29: the vase's foot fillet, listed as 2.8% short of OpenCASCADE, is right: its surface
lies on the balls' torus within 1e-5 mm, and the section it takes, integrated by Pappus, is 0.395502
mm³ against OpenCASCADE's 0.395511. The shortfall is the measure's: under the cap of 20 segments a
curve the STEP solid's side is meshed in rows 0.13 mm apart, whatever the chord tolerance, and their
chords miss 0.013 mm³ across the band the fillet takes, which the fillet's own finer mesh does not.
With 1000 segments the foot takes 0.3972 mm³ at a chord of 0.002% and 0.3959 at 0.0005%
(`TestFilletOnOtherSolidsVolumes` keeps 20 and says why).

Done 2026-10-29: the rib rounded all over, width leaning -4, r 0.5, h 5 and 10, unsound at chord
0.015%. The mesh edges crossed on the slope itself, not on the fillets: the foot's fillet meets the
flat slope along a straight line, and the ear clipper, taking any ear on a plane, clipped slivers 9 mm
long and under a thousandth of that thick from three of its points, which lie off the line in their last
digits; a sliver's plane is turned anyhow, and the slope's own edges beside its end crossed it. A
plane's ear-clipped mesh holding a sliver thinner than a thousandth of its length is now a candidate
for K4's mender (`earMeshSlivers`, `pkg/srf/triangulate_check.go`), which meshes it again by the CDT
where it crosses the shell's mesh; no golden moved. Foot first stays as the row says: the vertex blend
of three fillets is not begun.

Done 2026-10-29 (rd5): chamfers ending on a curved face the cone goes on past (the narrowest band's
rounded top, sizes 1.5 to 2.5), refused as "the fillet's surface can't be fitted". Two faults: at the top
the conic end sections strayed, as along any curved edge, so the blend was made again with its end
sections fitted to their cuts (`fitCuts`); and then at the base, a flat end, the surface's misfit stayed at
6e-4 mm however close the sections, because near a flat end a chamfer's surface was checked against the
straight line between its edges, and the end section fitted to the base plane's cut is curved across
(the chamfer's rulings along a curved edge lean out of that plane). Under `fitCuts` the surface near a flat
end now keeps to the nearest ruling, as near a curved end (`rollingBall.misfit`). All four sizes are sound,
adding 8.6, 19.0, 33.1 and 50.5 mm³ at 1, 1.5, 2 and 2.5. Seen on the way and left (the mesher's, K4): a
fillet of 2 between a plate's side and a rod of 8 (its foot on the rod at y = 4, along a row of the rod's
grid within 0.03 mm) has a mesh edge from the foot to the rod's end that crosses, at chord 0.002% and
0.003% (sound at 1.9, 2.1, and at 0.005%); and a chamfer of 2.5 or 3 of a band raised 5 off the cone's base
has naked and crossing edges on the cone along its foot, mid-edge.

Done 2026-10-29: fillets at corners of three edges of both kinds, all rounded, whose two of one kind
differ in radius: a ledge's two top edges at 1 and 2 over its concave upright, a rib's two foot edges
under its convex upright, refused before as "a convex edge's fillet meets a concave one's here" (the
stages round one radius and kind at a time, and the top's chain would have had to change radius round
the upright's arc). Where the stages by radius fail, a stage of each kind is tried with every pick at
its own radius; the edge of the kind there is one of is rounded first, and the other two, picked in
one stage, go on round the arc it leaves in their face with a radius going from one's to the other's
(`pkg/topo/fillet_vary.go`): faces at such an arc are ruled in the half plane through its axis (a plane
and a cylinder), so the section of radius ρ is that of radius 1 scaled by ρ about the edge, and the
face is an exact rational patch of degree 5 along the arc (the arc's 2, the cubic law's 3, flat at its
ends so it meets each cylinder tangent) and 2 across, tangent to both faces at every point. Against
OpenCASCADE's makeFillet (`scripts/fillet-mixed-fixtures.py`, `TestFilletMixedCornerOracle`), ledges
of 60 to 120 degrees and ribs, 36 lines: its count of faces and its volume within 0.05 mm³ (OpenCASCADE
fills the arc with a B-spline of its own law); `TestFilletMixedCornerSweep`, 180 ledges and ribs up to
150 degrees and radii 0.5 to 3 at two tolerances, none refused or unsound. And fillets at a corner of
four or more edges of both kinds, all rounded, the middle of a saddle where two ridges and two valleys
meet: the setback takes edges of both kinds (`setbackCornerAt`), each face's tangent lines lying in it
whichever way its edges go; within 0.05 r³ of OpenCASCADE's volume on 15 saddles (its one patch against
our four; `TestFilletSaddleSweep`, 36, none failing). Not open to the ledge: its two convex edges
first, which are two fillets of unlike radii above an edge left sharp, a corner nothing here makes.


Done 2026-10-29: the mitres of two radii left (`pkg/topo/rollingkink.go`, `TestFilletChainsRadii`,
`TestChamferChainsRadii`). A chain that meets smaller ones at both its ends is kinked at both, its
sections split at each; a chain that meets a larger one at one end and a smaller at the other copies the
larger's first span and is kinked at its own other end; a mitre of one radius beside one of two radii
has its two chains take each other's kinks, as they share the mitre's curve as it is. The two sides of
a mitre of two radii are no longer split alike: the copy is the larger's span from the shared face to
its kink found whole again (`spanOf`) where the larger's other kink splits it, and the patch sides along
the mitre are split where a corner of a patch across it lies (`splitAtMitres`), as the smaller's wall is
along the larger's pieces (`mateWallSpans`). Tried first and dropped: splitting each chain's sections at
its mates' kinks carried across the mitres, which round a ring of chains (the three cubics) came back
each time somewhere new and never closed. Chamfers of two distances mitre as fillets do, the kink where
the cut leaves the smaller's line between its feet; at a wide corner their mitre lies past the larger's
end further than a fillet's, and is looked for there. Against OpenCASCADE (Part::Fillet and
Part::Chamfer with each edge's own radius): the three cubics at 0.5, 1 and 0.75 and at 1, 1 and 0.5,
and the lens, four and three cubics chamfered at two distances, every vertex of its within 2.3e-6 mm of
ours, but for the four cubics' chamfer mitres' feet on the top, past the larger's edge's end, 2.2e-4 mm
off (the edge carried on along its tangent, `rollingBall.along`).

Done 2026-10-29: a convex and a concave chamfer meeting above a third edge left sharp, a ledge's top
edge with the upright below it, a rib's upright with one foot edge, made at once
(`pkg/topo/chamfer_mixed_pair.go`), where the stages made the convex first and ran its plane on over the
concave one's fill. The strips cross on the face they share; of their lines on their other faces one
meets the sharp edge past the vertex, and that strip runs to it, and the other's meets its line behind
the vertex, and that strip is cut square to its edge. The face the sharp edge bounds takes a parabola
from the square cut's end, along its strip's line, round onto the edge, through where the two lines
meet; the hole is filled by the hyperbolic paraboloid through the two ends and the parabola, the bilinear
patch whose diagonal it is. Where the walls meet at an obtuse angle and the lines meet level with the
cut's end or past it, the parabola is the line it becomes there, and the hole a triangle. Against
OpenCASCADE's makeChamfer (`scripts/chamfer-setback-fixtures.py`, `ledgetwo`, `ribtwo`), the corners and
counts of faces of 9 lines are its own; its plate, of degree 8, bulges less on that face than the
parabola, 0.04 to 1.1 mm³ (within 0.15 d³). Where the ledge's top chamfer is the wider (3 lines) its
corner has no patch and takes more than the top chamfer alone does. Of the rib's, it gives 6 of 9 as
not valid. `TestChamferMixedPairSweep` (324 cases: ledges and ribs whose walls meet at 60° to 135°,
distances 4 to 1 apart, one distance or two to an edge) has none failing, each made at once.

Done 2026-10-29: blends where the convex fillet's radius is several times how far the concave one
reaches along the face, 16 of 80 refused as folding in `TestFilletBlendSweep` (from 6 to 1 at a right
angle, 1.7 to 1 at 120 degrees), now none. Two faults, neither the sliver's shape. The concave fillet's
cut climbs nearly along its axis to where the sharp edge ends, so beside that point the patch's normal
turns on the sign of the last row's lean along the axis alone, and fairness pulled it under the cut
(by 0.02 of the hole at 105 degrees, 3 and 0.5): the row beside the cut is now held to its lean
harder than the rest when the first tries fold (`blendLeans`), and the other rows held less, then
more, for with all of them held hard the face's curve folded. And that curve, swinging out round the
sharp edge's vertex and back onto the convex fillet's tangent line, turns by more than a right angle,
and its rows' lean was turned over wherever it faced away from the first one's: it now keeps the turn's
hand (`blendPatch`). The ledges at 120 degrees and every ledge at 3 and 0.5, and the rib at 0.5 and 3,
are in `fillet-blend-corners.txt` now: OpenCASCADE's count of faces, its volume within 0.25 mm³ (ours
the less by 0.24 at 105 degrees, 3 and 0.5, the longest hole); every new corner's surface within 0.24
mm of OpenCASCADE's both ways, about as near as the corners made before (up to 0.22 at 90 and 105
degrees, 3 and 3): its end cut is its own, so the two fills differ. The first tries are as before, so of the corners made before only those whose lean had turned
over moved: nine ledges at 60 and 75 degrees, by up to 0.008 mm³.

Done 2026-10-29: a convex fillet and a concave one meeting at a corner of three flat faces whose
third edge is left sharp, a ledge's top edge with the inside upright below it or a rib's upright
with one of its foot's edges, refused before as "a convex edge's fillet meets a concave one's here"
(`pkg/topo/fillet_blend.go`). The two fillets touch at one point only, where their tangent lines
cross on the face they share, and the sharp edge has to end in the corner, so one patch fills it,
as OpenCASCADE's does: a quintic with one side folded to the point where the sharp edge ends, P,
tangent to the face along one side of P and to the fillet beside it along the other. The fillet
whose tangent line meets the sharp edge past the corner is cut by the plane through the crossing,
P and the other fillet's axis; the other by the plane through the crossing holding the first one's
axis, square to its edge where the edges are square; the face's curve reaches the second's tangent
line along it. Each choice is what lets the patch's twist at a corner of the hole be tangent both
ways: with the first cut square to the edge, or z rising evenly round it as OpenCASCADE's does, the
patch strayed from the fillets by 0.02 rad beside the crossing; and the folded side holds the lean
of the row beside the cut at nothing, or the normal turned off the cylinder by 0.47 rad short of P.
A fold is told by the normal turning over between neighbouring points or along a side, not by a
direction the corner faces, which refused every corner of 60 degrees. Against makeFillet
(`scripts/fillet-blend-fixtures.py`), ledges of 60 to 105 degrees and the rib at radii from 0.5 to
3, 30 lines: OpenCASCADE's count of faces, its volume within 0.15 mm³; the face's curve nearer its
chord than the parabola, as OpenCASCADE's is (the parabola took up to 0.17 mm³ more on a ledge at
radius 2). `TestFilletBlendSweep` refuses 16 of 80 by name, none unsound.

Done 2026-10-29: chamfers at more corners (`pkg/topo/chamfer_cone.go`, `chamfer_corner.go`). Where
convex and concave edges meet at a corner of three, every edge chamfered (a ledge's top above the
concave edge between its walls, a rib's foot below its upright), the edge of the kind there is one
of is the pivot and the other two are cut square where they cross its lines; the face between those
two, whose lines cross outside it, takes the line from one cut to the other, and a plane or bilinear
patch fills the hole: OpenCASCADE's corner, its volume within 0.005 mm³ and its count of faces on 12
lines of `testdata/topo/chamfer-corners.txt`. Before, the ledge was refused and the rib made in
stages, a cubic millimetre off. Where every face at a corner is flat and its edges all convex, or all
concave, and some of four or more stay sharp, the solid's cone (or the outside's) is cut by the
chamfers' planes (`chamferConeAt`): each face's wedge and each chamfer's strip clipped by the others'
planes, their chains sharing their points. That is what the chamfers' wedges cut, to 1e-6 mm³
(`TestChamferConeAsWedges`), and OpenCASCADE's apex where one chamfer reaches it, or two of one
distance across it (`apexsome`, `pocketsome`); where two meet, OpenCASCADE fills a plate between
them and takes up to 0.3 mm³ more, 14 of 18 lines. A corner of three chamfered edges that
`chamferCornerAt` can't close falls back to the same cut. And a convex chamfer whose end face lies
past an inside corner, its other edge on the face running back along it, stops where the end face's
plane cuts it, the face growing over its end (`pointAlong` back, as a concave fillet's): an L-shaped
block's inside top edge, OpenCASCADE's to the digit (`ledgeone`). The chamfer-solid-edge golden moved
with it: its edge ends at a notch's inside corner, where the wedge went 2 mm on past it into the
block (1.5 mm³). `TestChamferConeSweep` (504 cases, none failing) covers the new corners.

Done 2026-10-29: a rim's mitre at fine chord tolerances, refused below about 0.005% ("the
fillet's sections near its mitre can't be fitted"). The sections near a mitre run along the chain
across many of the dense edge's points (a tenth of a millimetre apart on the blob's rim), and the
edge's tangent was interpolated linearly between them, so the planes the sections lie in turned with
a corner at each: the points a section is fitted through had a kink of a few hundred-thousandths of a
millimetre at every one, and the misfit stayed there whatever the fit. On a chain with a mitre the
edge is now cubic Hermite pieces, the tangent's with its turn at the points (`rollingBall.along`);
other chains keep straight pieces, since with Hermite pieces everywhere fillet-tee's golden moved in
its last digits and a rib's foot at r 1, leaning -4, came out with two crossing triangles. Smooth, the
sections take four spans where two and one miss (`mitreSpans`, tried last, so what two or one made
stays). The blob's rims at r 0.5 and 1 are sound from 0.01% to 0.0005% (`TestFilletRimMitreFine`);
r 1.5 stays refused, as OpenCASCADE refuses it. Tried first: four spans alone (no better: the kinks
were the misfit), more centres for the mate's swept surface (`mitreSamples` 32 to 128, no change), a
denser edge (smaller kinks, the misfit the same).

Done 2026-10-29: setback corners of radii far apart, 4 of 36 refused as folding with radii 5 or 6 to
1 side by side (`TestFilletSetbackExtremeSweep`). Every fold was the check just inside a face's side:
the face's curve, the parabola through where the two fillets' tangent lines cross, has legs as unequal
as the radii, and the patches along it, slow at one end and fast at the other, turned beside it. Where
they do, the curves are cubics whose legs are 0.4, then 0.2, of the shorter leg, nearer the chord
(`setbackLegs`, `setbackFaceCurve`); a corner the parabolas fill is made as before. The sweep, its
radii taken down to 0.25 (12 to 1), refuses none of 96, each sound and tangent all round; to 20 to 1
none of 36 either. Tried and not taken: setbacks of each fillet's own radius, of the radii's
geometric mean, twice and half the largest's (each still folds all four). Of the curves: legs of
0.6 the shorter leg and over fill the apexes, not the boxes; 0.2 and 0.4 of the longer fill all four,
0.6 and over none; the parabola's legs scaled alike, 0.3 all four, 0.5 not the boxes.

Done 2026-10-29: blends that run off their face onto a curved one, as a fillet of a band's side on
the cone past the band's rounded top (`TestFilletRunsOffOntoCylinder`). The run-off's plane is
the next face's surface, carried on past its edge with the side as its polynomial runs
(`srf.SSurface.Sub`, `growSurface`), the fillet's cut fitted to where it meets that surface and
the next face grown over the fillet's end; the end face and the next face share the line where
they meet, or where it is not a line (a cylinder about an axis across the end face) cubics where
the end face's plane meets the surface (`sharedCurve`). The same carried-on surface ends a
fillet on a curved face that a side's face goes on past (the narrowest band, whose edge ends on
its top with the cone going on above it), refused before as "the fillet's end would stand out of
it" (`planeCut`, `closeEnd`). OpenCASCADE's makeFillet makes the narrowest band's, which ours
match within 0.8% of what they add; it refuses every run-off onto the top, which ours make sound,
adding more as the radius grows. Refused: a run-off whose end section crosses the curved face
within the chord tolerance of the side's edge (the face would grow by a sliver its mesh leaves
open), and run-offs from a curved end face onto a curved face whose shared edge is no line.
The setback patch the row proposed was not needed for a run-off onto one face. A band whose top
touches the cone where it meets the band's end (the x band at 21 dipping 1) is unsound with or
without a run-off: a contact case, left out of the fixtures.

Done 2026-10-28: two chamfers of any distances meeting at a corner of three flat faces whose
third edge stays sharp (`pkg/topo/chamfer_pair.go`). Each chamfer is a plane, so the corner is
where the planes cross: the mitre runs from the face both share to the first side face it reaches.
The chamfer that reaches further down the sharp edge goes on past the mitre, along the line where
it cuts the other's side face. That is the uneven fillet mitre's layout (`unevenMitre`) with lines
in place of its conic. Where the chamfers fill below a convex edge, as at a rib's foot, or cut above
a concave one, as at a ledge, the side face grows out to the mitre. Against OpenCASCADE's makeChamfer
(`scripts/chamfer-setback-fixtures.py`), 48 corners match its volumes within 0.005 mm³ and its counts
of faces: a box's corner, a pocket's, a rib's foot, a ledge, and a prism's foot at 60° and 120°.
`TestChamferPairSweep` covers 45° to 135°. Before, the top corner was cut by wedges and the filled
ones were refused.

Done 2026-10-29: a rib rounded all over, its top's five edges and its foot's four picked together
(`TestFilletRibAllOverOracle`). The stages round the top first, which leaves the foot one closed chain
going round from the flat sides onto the uprights' fillets and the slope edges' leaning ones; the
rolling ball fills it whole (`pkg/topo/fillet_onechain.go`), as it does picked in an edit after the top
(its eight edges, or one). Three faults of closed rolling-ball chains showed on the way: a foot was
taken on a flat face's patch far past its trims (the upright side past the slope edge's fillet), and
is now taken on a face that holds it, within the tolerance (`trimmedFoot`); a chain closed where the
seam it crosses leans along it (the leaning fillet's) left the trims open there, so a closed chain now
begins at the vertex whose seams stand most square to it (`closeAtSquareSeam`), and the crossing found
within the tolerance of its start is put at it. Against OpenCASCADE the foot's volume is within 0.03
mm³ for upright widths; for leaning ones OpenCASCADE's fillet lies inside the balls' envelope beside
the front corners (up to 0.009 mm at r 1, h 40, its volume 1.5% off), so the heights are checked
against the balls themselves (`ballHeight`), which ours meet within 4 chord tolerances. OpenCASCADE
refuses h 5 with the width leaning -4, which we make, checked by the balls alone.

Done 2026-10-28: mitres of two radii, chains of a rim and edges at a corner whose third edge stays
sharp (`pkg/topo/rollingkink.go`). The smaller end is a mitre against the larger's swept surface
(`mateSurface` takes the mate's radius), ending on its own wall at its depth. The larger end is
kinked as OpenCASCADE's is: each arc point is cut by the smaller's swept surface while the cut lies
on the smaller's fillet, between its feet (`onMateArc`; past its foot on the wall the two surfaces
still meet, on its ball), and by the smaller's wall after; every section of the chain has a full
knot at the kink's arc parameter, its patches two rows, and the mitre's sections are fitted in two
spans. The kinked ends are fitted before any chain is blended, the smaller end copies the first
span reparametrized to its own arcs (weights scaled by cⁱ, `matchParams`), keeps its sections
unsplit so the edge is shared whole, and is cut back after the larger, along the second span on its
wall. The lens at 1 and 0.5, 0.7 and 0.25, the four cubics alternating 1 and 0.5, and the lid's
upright with its bottom edge either way round: OpenCASCADE's vertices within 6.4e-6 mm, its surfaces
within 5.3e-4 (`TestFilletChainsRadii`, `TestFilletCornerRadii`).

Done 2026-10-28: chamfers meeting in mitres, as fillets do: at a crease of a rim's wall, of one
chain (the blob's 2 rims) or several, all picked or some, and at a corner whose third edge stays
sharp (the lid's upright with the bottom edge it meets). A point's height over the mate's swept
surface is over its line in the section through the point (`rollingBall.mateSurface`), and the
mitre's foot on the face both share is a root there, where the lines cross it, as on the creased
side. Two older faults of chamfers showed on the way: a chamfer's misfit between sections was its
distance from the line at the span's arc length, not from its nearest line, so a surface slid along
the edge was refused at fine tolerances (the blob's rims at chord 0.01%); and past an open end its
distance was measured square to the tangent line, not from where the faces, carried on as their
surfaces, meet (a chamfer ending on the next cubic's wall 0.046 mm off OpenCASCADE's, now 4.5e-4).
Against OpenCASCADE's makeChamfer the mitres' vertices are within 4e-6 mm and the surfaces within
2.4e-4 (`TestChamferChainsMitred`, `TestChamferRimMitre`).

Done 2026-10-28: some of a rim's chains picked, a chain mitred where it meets another picked one
and ending on the unpicked one's wall at its other end. They went the way of picks meeting at a
corner (`filletCornerPicks`, `linkMitresAt` partial) since 2026-10-22, but that end's conics along
a curved edge strayed from the swept surface as a lone chain's do, and the mitred chains' blend
never tried the sections fitted to their cuts (`fitCuts`): two of four cubics' top edges at r 1
were refused at chord 0.002%. Every vertex of OpenCASCADE's makeFillet is ours within 7e-6 mm
(`TestFilletChainsPartlyMitred`).

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
the sections near the mitre missed their fit (made since 2026-10-29, above). A chain is
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
| Sharp corners on twisted sweeps, next to a cubic, or beside an arc on a fitted sweep; corners beside an arc too sharp to join | refused; corners between two lines are mitred, on fitted sweeps too since 2026-10-29 (below), corners beside an arc joined since 2026-10-29 (below), and any corner beside lines or arcs rounded with a corner radius (F4), on fitted and twisted sweeps too. Closed paths are swept since 2026-10-22 (WP-C4): started where the profile crosses the path, so the closing joint is smooth; exact round planar lines and arcs, mitres, joints and corner radii included; fitted round cubics, twists of whole turns and paths out of a plane, the frames' turn taken out along the path | A twist turns the frames along a line, so its sides are no prism and the two sides' cuts by the mitre plane differ (a reflection in the plane turns the twist's hand): a twisted corner would need the profile's paths through it followed, as `sweep_joint.go` does beside an arc, or the twist held still through the corner; a corner radius rounds it today. A cubic's side at a corner is no prism either, nor is its circle's offset a line or a circle, so a corner radius can't round it (`pathChainAny` refuses it). A corner beside an arc is refused where a point's paths along the two pieces don't cross: the line's wall, offset from the path, passes outside the arc's (a bar 2 wide round an arc of 12 at 150°), so the arc's wall would leave through the line's start cap; joining there would take that cap's piece as a face of the joint. A corner radius rounds it |
| Shells of other curved solids (filleted, lofted) | prisms, and since 2026-10-22 (WP-C1) one lathe or revolve of lines and arcs, by its offset profile; since 2026-10-29 unions of lathes, revolves and extrusions, and drafted cones; since 2026-10-29 such parts taken away or intersected with too | Offsets of patches (22.1) are the first part; faces whose offsets do not cross themselves can be sized. The general shell is open-ended (E12, parked). Left from WP-C1: curved faces opened at an inside corner; a drafted extrusion joined to others, or open at a side; a revolve of less than a whole turn taken away (its ends would have to move out, which `revolvedInside`'s end cuts don't do); a face taken away on a lathe taken away or intersected with (refused by name: the open face is found on flat parts by the solid's open faces' points, not on revolutions); in a union, a prism's side partly inside another part and partly a wall, and a revolve's end lying inside another part (refused by name: a prism's sides move by one distance, and an end moved out would touch the other part's inside). Since 2026-10-29 parts taken away and intersected with (`pkg/topo/shelljoined.go`, `ShellPart.Combine`, `sketch.unionPartsAt`): the inside is the parts' insides combined in order, a part taken away grown (every face moved out, a prism's sides all alike), a part intersected with moved in, as the solid moved in less B is A moved in less B moved out; a union part's face is moved out only where it lies on no face of the union parts' own union, not where a later part cut it away. A face taken away lying on a part taken away runs into it, on one intersected with out of it, so the inside's face doesn't lie on the solid's (the ball cut flat had 220 naked edges where it did). The sides of a cut part run on past the solid's faces, so the open faces are found from the open faces' triangle middles on the part's faces, with the normals alike, not from the part's points on the solid (a pocket's side touched the open top at its rim). Coaxial lathes are joined by profiles only before any part is taken away. Trap: a cutter flush with a face (a pocket extruded from the block's bottom) leaves any later Boolean naked edges round the pocket's mouth at fine chord tolerances (0.002 mm, maxSegments large; sound at 0.02), the shell's or another cut's alike: the kernel's, a contact case for K3; since 2026-10-29 a part's face moved out into a wall thinner than the thickness, where the part it sinks into is thinner there than twice the thickness and yet the inside stays in the solid, is refused as one coming out of the solid is: the check's insides move such faces out by twice the thickness (`probedFace`), and what of them lies outside the solid is where the wall beyond would be thin. Since 2026-10-29 unions of lathes, revolves and extrusions, each a part a shell hollows alone (`pkg/topo/shelljoined.go`: the parts' insides joined, a part's face lying inside the others moved out by the thickness instead of in, so the insides meet; each face's mode found by where its triangles' middles lie on the solid; an inside moved out of the solid refused by a Boolean of the parts' insides with every face a wall), and drafted extrusions of arcs (`shelldraft.go`: the same draft of the profile offset by t/cos α). Since 2026-10-28 revolves of more than half a turn with an end kept (`pkg/topo/shellrevolve_reflex.go`) and lathes about one axis joined (`shellrevolve_joined.go`: the profiles joined in 2D, each curve split where others meet it and kept where the parts' insides combined differ on its sides; the kernel's Boolean of the profiles' prisms leaves faces that touch, and taking the inside from the joined solid fails where taking it from that solid turned again from the profile does not, `shellrevolve_again.go`) |

Done 2026-10-29: corners beside an arc swept with no corner radius, refused before ("a mitre can't
join an arc's sides"). A plane through the corner cuts the arc's turned sides along curves the
line's sides don't pass through. Tried before and reverted: runs cut by a half-space box at the
mitre plane and unioned (a partial coplanar face with curved trims; 188 naked edges with a round
profile), and runs cut by a bent prism ("bent caps") and unioned (the union failed on the touching
faces, even between two lines).
Each point of the profile runs along its own path through the corner, a line along a straight piece
and a circle about an arc's axis, all in the plane square to the corner's axis (which must be the
arc's); carried on past the corner, a point's two paths cross where the solid's sides meet, on the
outside of the turn both pieces going on past the corner to fill it, on the inside each stopping
where the other's side cuts it, as a mitre between lines does (`pkg/srf/sweep_joint.go`). Inside
`MakeFromSweepOf`, each curve of the profile has a joint, the polyline of those crossings (at the
turn's grid, refined to the chord tolerance where it bows, as on a cone), shared by the two pieces'
surfaces, each carried on as far as the joint needs: an arc's surface turned on past its end, a
line's extruded back; the seams run to where the curves' starts cross. An arc beside a joint is
split so that its section there holds the joint within a quarter turn. No Boolean, so no contact
case. `CheckSweepCorners` refuses by name the corners it can't join (`ErrCornerNoJoint`,
`ErrCornerTooLong`, `ErrCornerTooWide`, `ErrCornerOutOfPlane`), which the sweep reports with the
corner radius to set. Volumes against the region each piece and the overlap of both carried on
sweep, integrated by rows and slices in the test: square, round, diamond and ring profiles, an arc
before or after a line, arcs into arcs, an arc between two lines, within 2.5e-5 at a chord of
1e-4 mm. `TestSweepJointedCornerSweep`: 240 points, none unsound, 96 refused (the corners of 150°
and 170°, by the rule above). Closed paths take them too (a D). No golden moved; corners between
lines keep the plane. Not tried: corners beside cubics, and twisted or fitted sweeps' corners.

Done 2026-10-29: sharp corners between two lines on fitted sweeps, along cubics or held to a
direction, refused before ("a twisted sweep, one held to a direction, or one along a curve takes no
corners"). Along a line with no twist the frames don't turn, so the sweep there is a prism: a
straight piece beside a corner is skinned from its start section to its end one, each carried along
the line onto the plane halving the corner (`pkg/topo/pathsweep_corner.go`, `SweepPath.prismSkins`),
which both sides share, so the shell is one with no Boolean. The frame on past the corner is the
one before turned about the corner's axis, which the double reflection gives across the tangent's
jump; held to a direction, the frames must turn so too, so the direction must be the corner's axis
(`ErrPathCornerHeld`). A twisted sweep, and a fitted one with a corner beside an arc, take a corner
radius, the arc then fitted with the rest. The ray-cast oracle takes each straight piece beside a
corner as a prism cut by the planes at its ends (`pkg/check/raycast_pathcorner.go`): held to the
corner's axis it gives the rigid mitred sweep's solid ray for ray (`TestOracleFittedCornerAsRigid`),
and after a cubic out of the plane the mesh's, within 2e-3 of the volume (`TestOracleFittedCorner`).
`TestPathSweepCorner`: corners of 30° to 120°, in the cubics' plane and leaning out of it, three
profiles, none unsound. No golden moved.

### Modelling upgrade leftovers

What the work packages of `doc/notes/modelling_upgrades.md` (merged 2026-10-22) left open. Their
working notes are at commit `1e84ca0c` (`git show 1e84ca0c:doc/notes/wrap-emboss.md`, and
`model-to-script.md`, `wp-b1-b2.md`, `wp-b3-b4.md`, `wp-c3.md`, `wp-c4.md`, `wp-d1.md`,
`wp-scripting.md` beside it).

| Item | State | Approach / tried |
|---|---|---|
| Lofts (C4) | closed lofts, a guided loft along a solid's sides and tangency at a profile between the ends done 2026-10-22 (branch `lo-loft`); a closed loft refuses profiles more than a third of a turn apart round the ring (a quarter before 2026-10-29). Since 2026-10-28 a closed loft is held square to profiles between its first and last (`topo.ClosedLoftSurfaces`: a piece from each held profile round to the next, past the seam) or by closed guides going round it (`topo.ClosedGuidedLoftSurface`: the open loft's moved sections round the ring, the seam's slope across from the moved sections either side), within 1e-3 mm of its guides. Since 2026-10-29 a loft, open or closed, is held by guides and square at once (`topo.GuidedLoftThroughSurfaces`, `topo.ClosedGuidedLoftSurfaces`, `pkg/topo/guidedheld.go`): the guides move the sections of the loft held square, which is skinned piece by piece, each leaving a held profile square to it; a guide crossing a held profile slanting is turned onto within the twelfth of the way to the next section | Left: a closed loft's first and last profiles held square (a ring has no ends, so the loft's end tangencies stay refused; drawing the profiles starting elsewhere does it). Since 2026-10-29 the sections between a ring's profiles are turned with the ring's frame (`pkg/topo/closedloft_frame.go`), so a ring of few profiles is as full as its profiles: four circles 8 across round a circle of 25 were 7.33 across and their middle 2.1 mm in, 5% short of the torus, now within 0.01 mm and 0.2%; a saddle ring of six 4.4% short of its tube, now 1.1% |
| Datums (B4) | since 2026-10-22 any face or edge of the solid no entity makes is named by a workplane that follows it (`Datums.NameFace`, `NameEdge`): a plane, an axis, a line, or on other faces and edges the plane touching the face or the line along the edge where it was picked (since 2026-10-28, `Datums.NameFaceAt`, `NameEdgeAt`, `topo.FacePlace`, `EdgePlace`: kept as shares of the face's uv extent or of the edge's length, so it follows stretching; at the middle when picked from a list); mates refuse a flat face as an axis and an edge as a plane; the Assembly menu mates selected datums, the datums screen two names. Since 2026-10-29 a place on an edge keeps its end when an edit turns the edge round (a profile's curves bent the other way): of the place and its mirror along the edge, the one nearer where the datum was last found is taken (`topo.EdgeFrameNear`); the line's direction still follows the edge's, as a straight edge's line does. Since 2026-10-29 a datum on the solid in a group after the active one follows its face at every regeneration: the groups up to the last such datum are made, hidden as before (`Sketch.lastSolidDatum`) | |
| Fillets onto curved faces (C3) | since 2026-10-22 `TestFilletOnOtherSolids` 59 of 59 sound and `TestChamferOnOtherSolids` 57 of 59 (59 since 2026-10-28), none unsound: picks meeting at a corner whose third edge stays sharp, one of them the rolling ball's, are mitred there (`filletCornerPicks`, a chain mitred at one end only); a fillet or chamfer ends where the crease it rounds fades out (`rbEnd.fade`, the loft's 4 creases, which OpenCASCADE refuses); a chamfer's end on a doubly curved face takes its ways from each foot's own ruling and its sections there are fitted to their cuts (the loft's 4 foot edges) | Left: a crease that fades out is rounded only up to where it does, never past where it turns the other way (the loft's creases turn back above the first ring by up to 5°); run-offs and the rest under [Fillet leftovers](#fillet-leftovers) |
| Clearance and walls (D1) | since 2026-10-22 walls are measured at points no further apart than half the thickness asked about, finer about the thinnest, and followed onto the surfaces; gaps to mesh parts are followed onto the other's surfaces; `ClearancesWithin` takes a part by its solids; MCP `measure` reports both. Since 2026-10-29 the corners of where the two triangles a ray joined cover each other, seen along it, are measured too, the ray's length being linear in its start there (`pkg/sketch/wall_corners.go`), so the mesh's thinnest between the points is found (a V-groove's tip 0.2 over the floor, where the points measured 0.65; sharp edges, as a thread's ridge running out of its end face, measure 0); and a mesh part is taken apart by its pieces, its triangles joined by shared corners, for gaps within parts (`meshPieces`). Since 2026-10-29 walls are measured by solids, and a part's pieces are grouped into solids with each hollow's inside kept with the piece round it (`pkg/sketch/clearance_solids.go`, `solidsOf`), for walls and `ClearancesWithin` alike: two solids of a linked part standing on each other measure each alone (a ray up through the lower stopped where it met the upper, 4512 of 37480 rays unmeasured), and a hollow is no gap to its own solid. Left: a wall between two triangles no measured ray joined can still be missed; a group forced to a mesh with a part assembled after it is no part to `Clearances` | |
| Model to script (D5) | 300 of 301 fixtures come back (`scriptRoundTripKnown`, which may only shrink); images, splines of any number of points, constraints on four entities, styles, kerned text, faces' own appearances and datum points in a group that isn't a sketch are written since 2026-10-22, and the MCP `export` tool writes `.go`; what the DXF import draws as the window can't (`Sketch.ArcIn3D`, `Sketch.NewWorkplaneIn`, `Constrain.Again`) and an image whose file is missing (`Sketch.ImageFrame`) since 2026-10-22 | Left: `tlspr/v1/fields.tlspr`, a file of every field set whose references lead nowhere, which opens as no model. Looked at 2026-10-29 and kept on the list: it is the codec's fixture of the frozen v1 corpus (`TestCorpus` decodes and upgrades it), it has none of the reference workplanes, and regenerating it stops at the first (`Sketch.ForceReferences`), whose `MustEntity` panic `tlspr.load` turns into the error "can't regenerate". Mending the file would end what it tests; making it open would mean every dangling handle a file can hold becomes a diagnostic on its group rather than an error at load, about 250 `Must*` lookups in `pkg/sketch`, an item of its own if a damaged file is ever to open in part |
| Regeneration (D4) | recommendations 1 to 4, 6, 10 and 12 to 14 and a solid cache taken up, 9 behind a setting that is off, 11 in part (`doc/notes/performance.md`) | 5, 7, 8 and 15 to 18 measured and not taken, each with why in the same note; 11's parallel commits (curve ids reserved in advance) left |

### Thread leftovers

What the thread improvements of 2026-10-23 (`doc/notes/thread-evaluation.md`, branch
`thread-improvements`) left open. Code: `pkg/sketch/thread*.go`, `pkg/thread`, `pkg/ui/thread*.go`.

| Item | State | Approach / tried |
|---|---|---|
| ACME tolerance classes (2G to 5G) | read from a designation and dropped; an ACME thread takes B1.5's crest clearance and a typed-in clearance | B1.5's pitch diameter allowances and tolerances were not to hand; metric, trapezoidal and unified classes follow their tables since 2026-10-28 (`pkg/thread/class_iso.go`, `class_unified.go`) |
| Buttress threads, what was left | DIN 513 (S) and ASME B1.9 (BUTT) made since 2026-10-28, B1.9's rounded root and PUSH-/PULL-BUTT since 2026-10-29 (below); no classes, no rounded crest (`ErrButtressRounded`); a rounded root refused at chord tolerances coarser than a fifth of its radius (`ErrRootRoundCoarse`) | DIN 513-2's and B1.9's tolerance classes; a rounded crest needs the arc tangent to flanks at different angles, its centre off the ridge's middle. A rounded root at coarse tolerances: grooves cut from a blank at the crest (the groove's arc then meets nothing) were sound for every nut at 0.5% in a spike, but screws turned down failed at 0.2% and 0.5%, and the cut would need its own chamfer and run-out |

Done 2026-10-29: chamfered threads at 0.5% chord, the last unsound row of `TestThreadChamferSweep`
(a 1/2 BSF nut, 1 of 660, now none) and 4 of the rounded-crest chamfers' 114 (6 of 120 when last
counted; the Tr12x3 screw and stud are refused by name). It was not the mesher: the faces whose edges
crossed were no candidates of K4's mender (their ear-clipped chords part from the surface by less than
four chord tolerances), and meshed by the CDT they crossed as much, since the crossing edges are the
trims' own segments. Where a ridge's foot crosses the circle at which the bore turns into the
chamfer's cone, the two curves leave their vertex 2° apart on the bore, each chorded to the tolerance
on its own, and the circle's chord sagged into the hole by twice as far as the curves were apart,
through the ridge's flank; with rounded crests the cone's curves across a crest and the flank it runs
into smoothly leave their vertex along each other, and sag through the flank the same way. The Boolean
now halves a curve's segments beside a vertex where two curves on one surface leave it at under 30°,
until each sags off that surface by less than half of how far the two are apart at its middle, or by
less than an eighth of the chord tolerance (`srf.SShell.refineGrazingMeets`, `pkg/srf/boolean_graze.go`).
No golden moved: a sag along the surface (a cubic leaving a line tangentially in a plane), curves lying
on each other, and curves along which their surfaces meet tangentially are left alone, and the
projections leave the operands' closest-point caches as they were; each of those moved goldens when it
wasn't. `TestThreadChamferCoarse`.

Done 2026-10-29: ASME B1.9's rounded roots, asked for by **rounded root** (`RoundRoot`), and the
load's way in a B1.9 designation (`1-12 PUSH-BUTT`, `1-12 PULL-BUTT`, `thread.Spec.Load`), which
sets `LoadFlankBack` when the thread is made or edited (`ThreadOptions.LoadStated`). The standard's
arc, of 0.07141 P tangent to both flanks, touches the root at its bottom: drawn so it would lie on
the turned surface along a helix. Each side keeps the arc from its flank to 80° short of the
bottom, or nine tenths of the way beside the 45° flank, and goes on along its tangent into the part
(`thread.Profile.RootArcs`, `pkg/thread/root.go`), within a quarter of the radius of the standard's
arc; the flat root's corners were 1.6 radii off it. The ridge's sketch has two arcs and two tangents
more, 0 dof. Tried first: one arc per flank moved along it to cross the root at 18° (unsound at
the default tolerance for every nut), and turning 60° short (sound only up to 0.12 of the radius,
where these are to 0.2). Every B1.9 size tried (1/2 to 4 inches, 4 to 20 tpi) is sound at a chord
tolerance of 0.17 of the radius and most at 0.24; above 0.2 it is refused, naming the tolerance to
set. The flat root stays the default: at 0.5% and 0.2% the rounded root's flat between the arcs is
narrower than the chord tolerance, and `TestThreadSweep` holds every thread sound there.

Done 2026-10-29: a phase with a run-out on a modelled thread, refused before because the run-out
keeps the ridges' end off the circle where the root turns into its cone and a phase moves the end.
The end is now cleared of the circle where the phase has moved it (`ThreadOptions.phaseShift`,
`clearOfRunOut`), and the planes tried for the ridges take the moved flanks' crossings
(`threadDrawer.crossings`). `TestThreadPhasedRunOutSweep`, ten threads at six phases and three
run-outs, is sound in all 180.

Done 2026-10-28: buttress threads, DIN 513's metric S and ASME B1.9's inch BUTT (`pkg/thread/buttress.go`).
The profile has a flank angle of its own on each side (`thread.Profile.UpperFlank`): the ridge is drawn
with the angle between its flanks and the upper flank's to its foot in place of their equal lengths, its
crest off its foot's middle (`Profile.MiddleShift`), and a screw mate fits ridges by their middles at the
pitch line, where a nut's lies in the screw's groove. The load flank faces along a screw's thread and
back in a nut's; `LoadFlankBack` turns it, and `ThreadFitMove` refuses two whose load flanks face the
same way (`ErrLoadFlanksSameWay`).

Done 2026-10-29: the chamfered M8 stud 30 mm long, the sweep's unsound cases at 0.5% from 4 of
660 to 1, and chamfers on rounded crests, refused before (`ErrChamferRounded`, gone). All three were
the kernel's, where a chamfer's cone meets a ridge's flank just past the crest at a shallow angle:
the curve leaves the crest's edge nearly along it, the march from that seed took the wrong side,
ran off along the surfaces' extensions and was dropped (`srf.marchFrom` now marches the other way
when the first finds no seed); a long chord across a rounded crest was given the wrong side from
the normals at its start (now its middle); and a sliver of the cut shorter than the trim's assembly
tells from a point was left alone (culled). No golden moved. 28 and 31 mm, which needed later ways
of making the chamfer, are sound at the first. Tried for the rounded crest and dropped: taking the
seed and the split where the chord meets the cone, unrefined, wherever a curve's surfaces meet
smoothly (it moved the last digits of every thread golden and fixed nothing).

Done 2026-10-27: a full-length thread on a face follows the face's length at regeneration: the
thread's datum, finding the face again, measures it and moves the dimensions that carry the
length (`GroupThread.LengthDims`, `pkg/sketch/thread_follow.go`) by the difference before the
thread's groups are solved, as a length's expression moves them. A face come to reach the part's
end or to stop short of it changes the blank's shape, which is reported on the thread's group and
taken up at the next edit, as is a face too short for the thread.

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
| Threads: tapered (NPT, BSPT), variable pitch; a fastener library; a thread attribute in STEP | Never asked for. Since the thread evaluation (`doc/notes/thread-evaluation.md`) a thread starts on a face, is chosen from ISO, unified, Whitworth, pipe, trapezoidal and ACME tables, takes a class, a phase, a chamfer and a cosmetic form, and folds into one row of the history |
| Scripting: a startup script, `--watch`, menu items made by scripts, zoom to fit | Never asked for |
| The bit-exact scaffolding: `pkg/libm`, `CxxSort`, the FMA build rules | Harmless; replacing it moves every golden for nothing |
| A mesh Boolean fallback with perturbation | Conflicts with "never silently wrong geometry"; only as an explicit approximate mode |
| Sheet metal, direct modelling, tolerancing, CAM, simulation | Each an XL product of its own |
| Dual contouring (Ju et al. 2002) | Analysed 2026-10-21 (`git show 39142006:doc/notes/dual-contouring-fit.md`): a mesher for sampled fields, not for an exact B-rep; it would make STL less exact and regeneration no faster. Only its quadric simplification with the topology-safety test fits, as an LOD for large STL and assemblies, if one is ever asked for |
| `.tlspr` read and written as streams | P5 of `doc/notes/performance.md`; decoding is 15% of a mesh model's allocations, and no user has waited on it |
