# Roadmap

What is left to build, as of 2026-11-05. Only open work is listed. What is built is described in
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
| 2 | [Solid feature leftovers](#solid-feature-leftovers) | kernel | ? | Round and diamond profiles at a cubic corner; fitted and closed paths crossing themselves; shells beside a rise within the thickness |
| 3 | [Thread leftovers](#thread-leftovers) | feature | S each | DIN 513's nut deviation, B1.9's pitches and multi-start allowances wait for sources |
| 4 | [Test infrastructure](#test-infrastructure) | tooling | S | The race stage beside `test` and `test-heavy` in `make ci`: `pkg/sketch`, `pkg/topo` and `pkg/script/api` 17 to 19 min under `-race` together alone, of 30 |
| 5 | [K3 contact](#k3-coincident-and-tangent-contact) | kernel foundation | S | Sides closer than the chord tolerance, more than a tenth of it |
| 6 | [F9 Mechanisms](#f9-mechanisms) | feature | M | Asked for 2026-11-04: pull a part and the parts joined to it move; planned, not begun |
| 7 | [F5 small conveniences](#f5-small-ui-and-cli-conveniences) | feature | S each | Usability, not capability |
| 8 | [F8 TeleSAPR in a browser](#f8-telesapr-in-a-browser) | feature | S each | Built; publishing the extension |
| 9 | [F6 IGES](#f6-iges-import-and-export) | feature | L | Only if asked for |

## Open defects

### Fillet leftovers

Code: `pkg/topo/rolling*.go`, `fillet_*.go`, `chamfer_*.go`. Measures: `TestFilletOnOtherSolids`
and `TestChamferOnOtherSolids`, 59 of 59 sound. Each case below is refused with a message.

| Item | State | Approach |
|---|---|---|
| A circular fillet's foot that runs off its face past a side where the solid goes on beyond it, or into a side that is curved, not square to the face, or has holes | refused (`contactsClear`, `filletPastSides`). Since 2026-11-04 a foot running past flat sides square to its face is made: the face widened past them by slabs, the fillet made, the slabs and the fillet beyond cut away by the sides' planes; tried on a rod's foot and a hole's rim, fillets and chamfers, r 2.01 to 3.5, volume within 2e-4. A foot that just reaches the sides is made, the top pinched to a point at each (`assembleTrimPolygon`, `splitTouchingHoles`, `touchingSize`); since 2026-11-05 one just past them, whose arc stands above the top at their planes by under LengthEps, is made at the nearest size the kernel can cut within the chord tolerance of the one asked for (`nearSize`), said on the group (`Group.Note`, "(note)" in the list of groups): swept 1.997 to 2.006 by 0.0001, all 91 fillets and 91 chamfers (`TestFilletRodJustPastSides`) | Past a side the solid goes on beyond: cut the fillet's patches at the side's line directly, as the foot's own trim, rather than by a Boolean |
| Run-offs left: a chamfer's end running off both its faces (`TestFilletRunsOffOntoCylinder`'s x band chamfers of 2.5: the end section lies wholly past the band's curved top, so the chamfer would end on the top, not the end face, and the top grow over the cone too); a run-off taking material away (no test reaches it: the convex edges tried, a gabled plate's foot, are box fillets whose end face is narrower than the radius, refused before the rolling ball). Made since 2026-11-04: both ends' cuts overlapping, the blend reaching over the side all along, takes the side whole (`consumedRunOff`, `TestBlendTakesSideFaceWhole`, 20 of 20 since 2026-11-05, against an integral: OpenCASCADE refuses them all; an end section above the far slope carried past the ridge is above the near one there too, which cuts it first); from a curved end face onto a curved face whose shared edge is no line (`sharedCurve`, `TestChamferRunsOffCurvedEndOntoCurvedTop`); where the cuts lie apart but fell on one patch, a section is put between them (`runOffApart`); one only just past a curved face's edge is taken to its corner (`runOffSliver`) | refused | The setback patch: cut the fillet as a setback is cut, fill the hole between it, the next face and the faces round |
| Saddle corner (four or more edges of both kinds) with an edge left sharp, fillets | three of a saddle's four rounded, the fourth sharp, made since 2026-11-04 (`saddleSharpAt`: the two beside the sharp edge cut up to it, the third set back, a star of three patches with a side folded there; steep saddles and two fillets reaching the sharp edge at two points, unlike radii, by a ladder of three, a face's curve split where a rung ends, the nearer of the two cut short of the edge, `saddleSharpBack`). `TestFilletSaddleSharp` 108 of 108; against OpenCASCADE, `TestFilletSaddleSharpOracle` 106 of 106 within 0.0643 r³ and `TestFilletSaddleSharpUnlike` 112 of 112 (one OpenCASCADE line differs from its own mirror image by 6.3 mm³ and is checked against the mirror's). One rounded alone, or two opposite, made since 2026-11-04 (`pkg/topo/fillet_crease_end.go`: one ends where its surface meets the planes of the two faces across, its section projected along the edge onto them, `TestFilletSaddleOne` 84 of OpenCASCADE's 84 within the chords; two opposite meet in a mitre in the plane of the two sharp edges, `TestFilletSaddlePair` 42 of 42; chamfers likewise, 84 and 42, to 1e-6). A ridge and a valley side by side, the other two sharp, made since 2026-11-04 (`pkg/topo/fillet_saddle_side.go`: each cut obliquely up to the sharp edge beside it, the hole between the two cuts, the two sharp edges' pieces, which stay sharp, and the shared face's curve filled by a ladder of three patches, a rung leaving the corner, wider than a straight angle there; `TestFilletSaddleSide` 84 of OpenCASCADE's 84 within 0.051 r³, unlike radii too, `TestFilletSaddleSideUnlike`). Two opposite of unlike radii made since 2026-11-05 (`creaseStep`: each ends in the plane of the two sharp edges, the plane between their ends a flat step face; `TestFilletSaddlePairUnlike` 54 of OpenCASCADE's 54, at the two pairs' mean within the chords and within 0.12 r³ of OpenCASCADE, whose B-spline blends the two). Refused: two opposite whose ends cross in that plane (only where the corner is not symmetric about it; no test reaches it) | A blend in place of the step, as OpenCASCADE's, would smooth the corner; nothing needs it yet |
| The same, chamfers | three of the four chamfered, the fourth sharp, made since 2026-11-04 as the fillets are (`saddleSharpAt` takes chamfers too; `TestChamferSaddleSharp` 108 of 108, the steep ones whose ladder folds made in two stages, each edge ending on the faces across, `fillet_crease_end.go`; `TestChamferSaddleSharpOracle` 108 of OpenCASCADE's 108 since all nine slope pairs are in it, the volumes up to 2.3 d² apart, 3.5 d² for the staged steepest, our corner set back further; unlike distances reaching the sharp edge at two points since, `TestChamferSaddleSharpUnlike` 112 of 112). One alone, two opposite and a ridge and a valley side by side are made since 2026-11-04 (`TestChamferSaddleOne`, `TestChamferSaddlePair`, `TestChamferSaddleSide` 84 of 84, up to 1.47 d² from OpenCASCADE, its patch flatter); two opposite of unlike distances since 2026-11-05 with a flat step between their ends (`TestChamferSaddlePairUnlike` 56 of 56, within 0.049 d³ of OpenCASCADE) | OpenCASCADE: one B-spline patch, set back less than ours; a flatter corner would be planes cut where the chamfers' planes meet |
| A rib rounded all over, foot first | refused: the top's edges rounded in an edit after the foot was filled (picked with the foot in one edit, or the top first and the foot after, made, all 24 cases). The uprights say "round that edge too" at the crease where the foot's two fillets meet in a mitre below them, and the slope's edges that the ball runs off the side face onto the foot's fillet | Tried 2026-11-04 (rd8-rib-foot): the upright runs on tangent into that crease, which `blendChain` takes as one chain down to the plate. With a chain mitred at its far end tracing from where the crease fades in at its start (`fadeIn` without reversing), it traces, but the crease turns sharply almost to the plate and fades only at its very end, so just above the plate the ball's feet run off the foot's fillets onto the plate: a blend of three surfaces, the two fillets and the plate, not a fillet of the crease. OpenCASCADE makes it, valid in all 12 cases tried (h 5 and 10, lean −4, 0, 2, r 0.5 and 1), with 6 B-spline faces: four-sided patches at each upright's foot (bbox r about the corner, z 0 to r) and at the front where the slope's edges meet the foot's fillets; its volumes differ from top first by up to 1.5 mm³ at r 1 (h 10, lean 0: 3653.98 against 3652.44). Needs an n-sided patch from the three fillets' end sections, as `fillet_setback.go`'s star does for flat faces, but with the plate's curve and the two crease halves as its sides |

Traps: OpenCASCADE is not the truth everywhere (a hole of 5 in a pipe of 10, r 2: its face lies up to
0.093 mm inside the balls' envelope); its Volume of our STEP files is off by up to 5% of a fillet (and took 0.28 mm³ for the turned-back chamfer of `TestFilletCreaseFades`, whose slices and our meshes give 4.85),
integrate slices instead (`scripts/fillet-step-volumes.py`). `srf.ProjectFrom` is not for the rolling
ball's probes: four times slower, no fillet gained.

### Solid feature leftovers

| Item | State | Approach |
|---|---|---|
| Sharp sweep corner next to a cubic | Made since 2026-11-04 of Booleans on an open path with no twist, held frames or corner radius (`pkg/sketch/sweep_cubic_corner.go`): the path cut at the corner into runs carried a little past it along their tangents, exact along lines and arcs, fitted along cubics with their flat patches made planes (`srf.SShell.FlattenPlanarPatches`), and the corner's fill where both sides carried on overlap, joined fill, fitted, exact, the other orders after. Refused by name where no order joins into a sound solid, and twisted, held, rounded or closed paths. Of 60 corners each (both orders, corners ±30° to ±150°, bends −90° to 90°, lead 10): rectangles either way and a skewed quadrilateral 60, a diamond 12 (the cubic straight), a circle 0. What decides it is where the turn's axis through the corner crosses the profile: the two sides' skins pass through those points with the same tangent plane, coplanar for a rectangle's side (joined), crossing for a skewed edge, but tangent for a circle and meeting at a vertex for a diamond | Tried 2026-11-04 (rd8-sweep-round-corner, not kept): each side carried to the mitre plane and cut back to it by a box, then joined flat on it. With a circle's arcs re-split off the axis and a line's lead swept as the same line (no seam at the corner), both cuts are made, but their union still fails where the two tubes touch tangent at the ends of the mitre's ellipse; a diamond's vertex on the axis meets the fitted skin's seam at the corner exactly where the plane crosses it, and the cut fails. Rectangles mitred so are sound, but lose up to 0.5% where the cubic curves away (the fill keeps it). Needs the union to take two faces meeting tangent at a point on their shared edge (K3), or the mitre built as the kernel's own, the cubic's skin run past the corner and trimmed, which `FaceAdder`'s untrimmed patches can't hold. Probed 2026-11-05 (rd9-tangent-point): the join that fails is the fitted run's with the exact one (each joins the fill), and the same failure is had with two exact tubes of one radius crossing, any lead: the kernel's march turned back where they touch (K3 above: exact tubes are mended since 2026-11-05, a fitted one against an exact one is not). Beyond that, the runs' leads, 0.05 of the reach, end within a grazing distance of the points of contact, where an edge's chord misses the crossing; carried a reach or more, the fitted run's skin meets the exact one off by more than a touch allows, and the join leaves edges there |
| Corner beside an arc whose pieces' outer sides never meet past it (the arc turning against a corner of 150° or more) | refused, names the corner radius (`errCornerFillOpen`); the corner too sharp to join whose outer sides do meet is made since 2026-11-04 by Booleans, the pieces swept apart and the outside filled by their overlap carried on (`pkg/sketch/sweep_corner_fill.go`, `TestSweepSharpCornerBesideArc*`: of 64 rectangles at radius 4 and 12, 48 made, 16 refused) | The overlap runs on as far as the pieces are carried: no finite joint closes the corner, so only a corner radius makes a solid. Closed paths take the fill too since 2026-11-04 (`TestSweepClosedSharpCornerBesideArc*`: of 57 closed paths of a line, a sharp corner, an arc and two lines back, 17 made, 8 of them before, 40 refused). Checked 2026-11-05 (`TestSweepCornerFillNear`): every corner made of both grids with the bars, 291, is the region whose corners are filled only by the piece of their overlap at the corner; in 60 the overlap carried a quarter turn has a second piece far from the corner (as at 150° beside an arc of r 20 turning −30°, which the kernel joins itself, no fill), and none holds it. A fill of more than one piece keeps only the one at the corner (`nearestPiece`) |
| A sweep whose path comes back within the profile's reach of where it has been (an arc past a sharp corner turning back over the line before it, or looping round across it) | Never made crossing itself (`pkg/sketch/sweep_cross.go`): a path coming back within twice the profile's reach of itself, a quarter turn or more between, has its shell meshed and searched for edges through a triangle. Round lines and arcs, open or (since 2026-11-04) closed, the sweep is made in stretches each clear of itself (cut inside a line or an arc, never at a corner; round a closed path cut at least once, its first and last stretches meeting where the profile is) and joined by a Boolean, the union checked as the group will hold it; a fitted sweep, since 2026-11-04, in spans of its `topo.SweepPath` the same way (`topo.MakePathSweepSpan`, `SweepPath.CutAt`, `pkg/sketch/sweep_cross_fitted.go`), each swept with the whole path's frames; otherwise `ErrSweepCrossesItself`. `TestSweepCrossesItselfSweep`: of 1440 line-and-arc paths, 160 crossed themselves; 81 are made sound, 79 refused. `TestSweepCrossesItselfFittedSweep` (hairpins, rings and a looping cubic, rigid, held or twisted, three profiles): of 54 points, 33 made sound and 21 refused, where before 21 were made and 33 refused (2026-11-04) | Refused still: fitted skins that meet where the path comes back and fail to unite (a round profile held to a direction at a fine tolerance, a square held with its sides on both legs in one plane), a twisted ring whose spans' joins stay open, paths with a run swept rigidly beside a corner at an arc (`topo.ErrSpanRigid`), a sharp corner beside a cubic (`sweep_cubic_corner.go`) or a filled corner (`sweep_corner_fill.go`), and stretches that can't be cut clear beside a corner's joint or whose union opens edges. Fitted skins lying on or crossing each other are the Boolean's: K3's coincident and tangent contact for B-spline patches |
| A twisted sweep with nowhere to twist (every piece held: an arc beside a corner, a line within its corner's mitre or joint) | refused (`ErrPathCornerTwisted`), naming the corner radius. Since 2026-11-04 a line at a rigid run's end is held only as far as its joint reaches (`lineHold`, `srf.JointReach`) and twists past it (`TestPathSweepTwistedRun`, `TestPathSweepTwistedRunSweep`); since 2026-11-04 too, where that leaves nowhere to twist, an arc beside a corner is split where its joint's reach ends and twists past it (`arcHold`, `splitArc`, `alongCircle` stops the run growing into it; `TestPathSweepTwistedArcs`, `TestPathSweepTwistedArcsSweep`); what is left is a twist arriving turned at a joint it no longer fits ("a piece is too short for the joint"), and arcs a joint takes whole (corners of 150°, profiles wide against the arc). The arcs-only twisted sweep has its fixture (`sweep-twisted-arcs`), path pair and example (`zigzag`) | A corner radius |
| Shell: a revolve's curved face opened where the solid comes back within an eighth of the thickness outside it (a slot that narrow), and a drafted extrusion's round side opened at an inside corner beside a round side kept | refused by name. Since 2026-11-04 a revolve's curved face opens at an inside corner beside a flat face square to the axis, beside a cone, sphere or torus kept or taken away, or beside a step narrower than the thickness (`pkg/topo/shellrevolve_step.go`: the step, `cornerOut` into the empty wedge, `srf.joinOrphans`); where the solid comes back within the thickness outside the face (a slot, a step that rises again), the run-out past it is shortened to a half, a quarter or an eighth of the thickness (`runOutLoop`, rd9; `TestShellRevolvedInsideCornerRunOut`, 9 of 9 sound and within 0.3% of Pappus's volume). A drafted extrusion's round side opens at an inside corner beside a flat side, kept or taken away, since 2026-11-05 (`pkg/topo/shelldraft_corner.go`, rd9): the wall under the flat side ends short of the corner, which moves along a conic, on a drafted plane following its chord set back by its sag and a margin of a fiftieth of the thickness; `TestShellDraftedInsideCorner`, 16 of 16 sound and within 2e-4 of the stepped region's integral, drafts 0.5° to 10°. Found then: a two-sided drafted lobed block isn't shelled at all, even at its top, by the drafted shell (it falls through to the prism shell's refusal) | Drafted beside a round side kept: the step's plane would have to follow two cones' meeting; the same chord and set-back on a drafted plane square to neither may do. Exact walls to the corner: each section stepped and lofted |
| Shell: a union whose parts' insides touch along a line or at a point (a round post as high as the block, sunk into its top beside its side, at 21 of a side at 20) | refused, saying what touches (`TestShellJoinedFlushPostRefused`); was made unsound before 2026-11-03 | The true result is no manifold: the walls either side of the touch meet with no thickness, as with the prism's pinch below. The step against the block's side (the post at 18) is made since 2026-11-04 (`TestShellJoinedFlushPostStep`): the post's foot is kept as a wall where moving it out would carry the inside to the solid's face |
| Shell: a prism (every part extruded from one plane, or one sketch) whose inside's outline touches itself, as a block with a round post at 21 beside its side at 20 | refused since 2026-11-03 (`ShellSolid` checks its result as `ShellJoined` does; `TestShellPrismInsideTouches`, `TestShellBlockAndPostInsideTouches`); was made unsound | The true result is not a manifold: the walls either side of the touch meet along a line, so splitting the outline at the pinch would leave that line. Only a different solid (the touch moved apart) can be made |
| Shells of filleted and lofted solids | refused | Patch offsets (22.1) size faces whose offsets don't cross themselves; the general shell is E12, parked |

### Thread leftovers

Code: `pkg/thread`, `pkg/sketch/thread*.go`, `pkg/ui/thread*.go`. Sources checked for the classes:
DIN 513-3:2020 (free text), NBS H28 (1957) Part III, Machinery's Handbook, Engineers Edge's ACME rows.

| Item | State | Approach |
|---|---|---|
| DIN 513-3's nut deviation | 7H, 8H, 9H taken as basic | EI = 3.1758·a (2/(tan 3° + tan 30°) times DIN 513-1's axial play a, not found free); with a, `buttressClearance` adds it |
| B1.9's preferred pitches | `buttressInchTPI` follows H28's ranges | amesweb's calculator lists three a diameter (B1.9 Table 1?); B1.9 itself would settle it, and the default pitch per size |
| Multi-start allowances | left out for B1.5 (appendix B) and B1.9; DIN 513-3's factors are in | H28 (1957) has none for ACME, only its gauges' lead tolerance times 1.5. Machining Doctor says B1.5 adds K × es to a nut's diameters by its number of starts; K not found free |

### Test infrastructure

| Item | State |
|---|---|
| Race stage of `make ci` | 2026-11-04, `make ci` with nothing beside it (26 min): `pkg/topo` 1409 s, `pkg/script/api` 1317, `pkg/sketch` 1245, `pkg/ui` 889, `pkg/check` 529, against the 1800 s limit. Thinned under `-race` on 2026-11-03 and 04 (the normal run keeps every case): the rim mitres, lid and other-solid fillet tests of `pkg/script/api`; the fitted sweeps' oracle tests leave out `Oracle.Measure`; the thread, rolling-ball chain, rib and closed-loft tests take a part of their cases; `pkg/ui` keeps one thread among its scripts and cached models (`raceLeavesOut`). Since 2026-11-04 `pkg/topo` too: the loft, path and B-spline sweeps take a fifth of their cases (`fixtures.Cases`, the random numbers drawn for all), the closed loft sweep rings of four and five and the wide one its circle path, the saddle oracles a fifth of their lines, the twisted corner and run one profile of three, the partly mitred chains a coarser chord tolerance, and the rib's foot after its top is skipped (no coarser tolerance makes it). `go test -race ./pkg/topo/` took 8292 s of processor before, 4921 after (the wall times, 712 s and 1242, measured beside other work at loads of 8 to 200, say nothing). On 2026-11-05, after the CDT default and the round's new tests put `pkg/srf`, `pkg/sketch`, `pkg/topo` and `pkg/script/api` past 30 min in the stage, more under `-race` only: the wrap tests' cone bands and unions, the closed and self-crossing fitted sweeps (the ring, frame and one fitted case), the partway sweep's oracle on fitted paths, one thread of the crest test, the rib all over its short row, the saddle tests three lines (`saddleLines`), two joined shells of a block and boss, the rib foot's comparison fillets, and in `pkg/script/api` the crease, corner radii, lid uprights, buttress, band run-off, fitted cuts, off-axis, rod row, fine cylinder and fillet stages' cases. The four run together under `-race` alone on the machine, before and after: `pkg/srf` 12.5 min (1680 s of processor) to 6.0 (1334), `pkg/sketch` 22.0 (10 190) to 18.9 (9401), `pkg/topo` 20.6 (9773) to 16.7 (8602), `pkg/script/api` 21.0 (13 130) to 17.9 (10 284). Inside `make ci`, beside `test` and `test-heavy`, that may still not fit: the rest of the cost is spread over hundreds of tests (the slowest of `pkg/sketch` under `-race -parallel 1` is 78 s). Next: the race stage beside the other stages (run it after them, or leave the HEAVY tests to the nightly race run), or thin further by package |

### Guide tooling

| Item | State |
|---|---|
| Menus, popup menus and dialogs in pictures | The real window under an Xvfb of its own since 2026-11-04 (`TestDocWindowShots`, `scripts/regen-doc-window-images.sh`): the File menu and its Export cascade, an edge's context menu, the Triangle Mesh dialog, and the messages of Fillet Edges and Screw Thread... with nothing selected. Fillet and Thread have no dialog of their own: their screens are in the property browser, which the scenarios picture. Left: the fonts are the machine's, so the pictures change with the machine that regenerates them |
| Clicks by name in the rest of the scripts | `click-entity`, `click-tw`, and since 2026-11-04 `click-edge` and `click-face` for a solid's edges and faces, each with `right` for a context menu (`doc/guide/scenarios/README.md`); every click by coordinates on a solid's face or edge in the guide's scenarios and the interface scripts is by name, but `path_datum_freeform`'s, whose datum is placed where it is clicked. Left: clicks on sketch entities and empty space by coordinates in the older scripts (about 300 `down` lines) |
| Video | A scenario records a video between `video start` and `video end` since 2026-11-04 (`pkg/ui/docvideo_test.go`, `scripts/regen-guide-videos.sh`): a frame after each step, the pointer gliding between clicks, a ring where it presses, the keys pressed and a caption, through ffmpeg into WebM (VP9); `getting-started-plate` (36 s, 0.44 MB) and `fillet-edges` (32 s, 0.44 MB) on their pages. Left: longer demos from the real window under Xvfb (menus and dialogs, which the headless frames don't draw), an MP4 beside the WebM if a browser in use can't play VP9 |

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
  (`TestPinInBoreSweep`, `TestCurvedContactSweep`) 77 of 108 → 0. No golden moved.
- **Cones and balls, 2026-11-04:** a cone in a countersink of its angle, touching or a hair off, is
  sound by every Boolean (`TestCoaxialContactSweep`, 96 of 96 wrong before): the fuse takes coaxial
  cones and concentric spheres too (`snapToCones`, `snapToSpheres`), a side running onto a face of
  the other solid lying on the classified patch is coincident where the other solid's chorded edge
  misses the true one (`classifyOnCoincident`), and a plane meeting a lathed face adopts the plane's
  own exact outline lying on it instead of marching another.
- **A ball in a seat of its radius, 2026-11-04:** sound by every Boolean (`TestCoaxialSeatSweep`,
  50 of 96 wrong before; no golden moved): a plane meets a sphere in an exact circle, cut where it
  crosses each patch's sides and its face's outline (`intersectPlaneAndSphere`,
  `pkg/srf/surfinter_sphere.go`), so the seat's rim and
  the ball's crossing of the block's top are one curve; an edge whose chord dips off a sphere by
  its curvature is taken as coincident where it runs onto a face lying on the classified sphere
  (`classifyEdgeOn`), and the true rim, a sag outside the sphere's chorded trim, still finds that
  face (`classifyOnCoincident`). Both rules hold on spheres only: on every curved face they broke
  threads and wrapped text.
- **Tangent contact, 2026-11-04:** touching at a point or along a line, the union has the right
  volume and is not a manifold there. Check Solid now says so (`Report.Touches`, `TouchAt`,
  `TestCheckPartsTouching`) where every problem edge comes within the chord tolerance of both the
  group's part and the solid before it and the volume is theirs added. It is a report of Check
  Solid, not a problem on the group: finding it needs the mesh checked, which regeneration does not.
- **Resting, 2026-11-05:** a cylinder's side parallel to a plane of the other solid, or a ball,
  within a tenth of the chord tolerance of touching it is moved, the whole second operand rigidly,
  until it touches (`snapTangent`, `pkg/srf/boolean_snap_tangent.go`). Sunk a hair, the two had cut
  each other in lines closer than the mesh tells apart and the Boolean failed (cylinder lying on a
  plate 26 of 108, ball 3). The probe is now `TestCurvedContactSweep`: every Boolean sound, but for
  the union of the cylinder resting on the plate, joined along a line as solids that touch are.
- **Equal pipes crossing, 2026-11-05:** two exact tubes of one radius whose axes cross (a tee or
  cross of equal pipes) failed every Boolean: they meet in two ellipses crossing where the tubes
  touch, and the march turned back there. Their intersection is now exact, not marched
  (`intersectEqualCylinders`, `pkg/srf/surfinter_tee.go`): each ellipse is the first tube's circle
  moved along its axis onto a plane halving the axes' angle, rational quadratic arcs, cut at the
  patches' sides, their outlines and the two touching points; the operands' own edges through a
  touching point (a tee's branch ending at the main pipe's axis) get a vertex there (`touchesOn`).
  `TestBooleanEqualCylindersCrossing`: crosses and tees at 90°, 75°, 60° and 45°, seams on and off
  the points, every union, difference and intersection sound and within the chords' bound of the
  Steinmetz volume but 3 of the 12 tee differences at 45°, whose branch's end rim touches the pipe
  tangent there and lies nearer it than the pipe's chords: those meshes cross, and a Boolean of
  equal crossing tubes whose mesh crosses itself or opens is failed, the group saying so
  (`failCrossingTouch`; the mesher's, below).
  `TestEqualPipeTee` (`pkg/script/api`) from sketches. The march's own rule, tried first
  (rd9-tangent-point), found touches on threads and hung them; this takes only exact tubes, and the
  thread tests keep their time. Left for the sweep corner (a fitted tube against an exact one): a
  tube's flat end within a grazing distance of the point (0.05 of the reach) loses a seed, and a
  fitted tube's skin touching an exact one is off by more than the tenth.
- **Left, S:** sides closer than the chord tolerance but more than a tenth of it, which the fuse
  leaves as drawn: those are the mesher's (`doc/dev-notes/k4-cdt-spike.md`). The coincident rules
  of the seat (`classifyEdgeOn`, `classifyOnCoincident`) stay on spheres: no case found on 2026-11-05
  that they would fix on another curved face, and on every curved face they broke threads and
  wrapped text.
- Not recommended: a mesh Boolean with perturbation (only as an explicit "approximate" mode).

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

### F9 Mechanisms

Planned 2026-11-05, nothing built: `doc/dev-notes/f9-mechanism-plan.md` has what was found, the
design and the commits. Today each group is solved alone with every earlier group held, so a mate
in part B's group moves only B, a loop of parts cannot be solved, and a linked part pulled in the
window snaps back. Branch `f9-mechanism`, after a half-day spike on the pull and the cost of a
drag frame.

| Item | State | Size |
|---|---|---|
| A link group flagged "moves in a mechanism" (`link-moves` in the file), flagged links in a row solved as one system, unflagged ones and everything before them the ground; old files unchanged | planned | S |
| Pulling a part along the mechanism's freedom: a null-space step shared by the window's drag and `Entity.MoveTo`, through one `pkg/sketch` entry point | planned | S |
| Driven hinge and concentric mates making `angle` and `travel` parameters, as Screw makes `turn`; the axis-angle constraint about a line and under a minimum or maximum | planned | S |
| Left out until asked for: a drag drawn by placement matrices, a 3D sketch as a member, a drag written back into its driving parameter, several parameters in one motion study, glTF animation channels | not planned | |

### F8 TeleSAPR in a browser

Built: `cmd/telesapr-web` on `pkg/platform/web` and a WebGL 2 renderer, `make wasm`, the site's
"Try in browser" page (`doc/dev-notes/browser.md`). Left:

| Item | Size |
|---|---|
| Module size: 39.0 MB after `wasm-opt`, 11.47 MB gzipped, since 2026-11-04 without package `net` (1.2 MB) or `encoding/gob` (the STEP cache's own format, 0.46 MB). What is left is the browser's features (scripting 3 MB of code, JSON, fonts, packs) and Go's tables; `docs.json` gzipped saves nothing over the wire. Further cuts would cost a feature (`doc/dev-notes/browser.md` Size) | open, nothing cheap left |
| The AI relay extension in a store, shipped with a release | Built 2026-11-04 (`cmd/telesapr-web/extension`, `telesapr-cli browser-relay`): an MCP client reaches the page through it, loaded unpacked from the sources. Left: publishing it (Chrome Web Store, addons.mozilla.org) and a zip of it with the site or the release | S |

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
| Sheet metal, direct modelling, tolerancing, CAM, simulation (forces, contact, dynamics) | Each an XL product of its own; [F9 Mechanisms](#f9-mechanisms) is constraint kinematics of rigid parts, not simulation |
| Dual contouring | A mesher for sampled fields, not an exact B-rep (`git show 39142006:doc/notes/dual-contouring-fit.md`) |
| `.tlspr` read and written as streams | P5 of `doc/dev-notes/performance.md`; no user has waited on it |
| A wasm build in CI (F8) | Postponed by the user 2026-11-04. Nothing builds or tests the js/wasm target, so a change can break it unseen until `make wasm` is run by hand |
