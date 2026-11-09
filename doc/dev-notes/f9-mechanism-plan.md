# F9 Mechanisms: the plan (2026-11-05)

Refreshed 2026-11-09: the degrees of freedom analysis, items 1 and 2, is built ("Built
2026-11-09" under "Degrees of freedom analysis").

The user asked whether a mechanism can be moved: parts linked and joined, one pulled in the
window and the others turning and sliding with it. This is what was found on devel `19f6d08f`,
what is decided, and the order of work. It is the working note of the package: its commits add
what they find here.

## What exists

- A linked part is placed by seven parameters of its group, a move and a quaternion
  (`generateLinked`, `pkg/sketch/group_generate.go`), and every entity of the part is a copy in
  those parameters (`CopyAsNRotTrans`). The one equation of the group is `|q| = 1`, so a part has
  six degrees of freedom and a constraint on any of its points, lines, normals or faces moves it.
- Mates are ordinary constraints made in one step of undo (`pkg/script/api/assembly.go`,
  `mate.go`): Concentric, Flush, Hinge, and Screw, which makes the parameters `turn` and `lead`
  and an axis-angle constraint (`pkg/core/constraint_axis_angle.go`).
- Motion Study drives one parameter through a range, regenerating at each step, and saves
  pictures or meshes (`AssemblyMenu.MotionStudy`). Dimensions take a minimum or a maximum
  (`solveHoldingBounds`, `pkg/sketch/generate_all.go`; `doc/guide/limits.md`).
- Interference, clearance and wall checks over the parts of an assembly.
- Dragging in 3D: points, normals, and a linked part's points through `PointForceTo`, which
  moves the part's translation (`pkg/core/entity_base.go`), with Shift and Ctrl turning it
  (`pkg/ui/mouse.go`).

## What is missing

**One group at a time.** `generateAll` solves each group alone, for the parameters its own
requests, constraints and the group itself make, with every earlier group's values held
(`SolveGroupAndReport`, `writeEqSystemForGroup`). A mate in part B's group moves B and never A;
a loop (a four-bar linkage, a part held to two others) cannot be solved; a constraint whose entity
is in a later group is pruned before that group exists (`PruneRequestsAndConstraints`). This is
the cause, and the rest follows from it.

**Pulling a part.** `MarkDraggedParams` (`pkg/ui/app.go`) lists no case for a linked part's
points (`EntityPointNRotTrans`) or normals (`NormalNRot`), so the solve gives them no preference.
Weighting would not help: `PointForceTo` puts the pulled point under the pointer by moving the
part's translation, and the re-solve can bring a hinge pivot back only through the translation
when the pivot lies at the part's own origin (the pivot's derivative by the quaternion is the
pivot's distance from the origin), so the part snaps back. The pull itself has to move along the
mechanism's freedom.

**Driven joints.** Hinge leaves no angle parameter and Concentric no travel, so Motion Study can
drive only a screw or a dimension made by hand. The axis-angle constraint takes a normal as its
axis, not a line, and `CanLimit` (`pkg/core/constraint_inequality.go`) excludes it from bounds.

## Decided (2026-11-04)

- Scope: the joint solve, pulling in the window and from scripts, and driven Hinge and Concentric
  mates with limits. Not in it: several parameters in one motion study, animation channels in
  glTF, forces and contact; "simulation" stays parked, this is constraint kinematics of rigid
  parts.
- Membership by a flag on a link group, "moves in a mechanism", off by default and saved in the
  file: flagged link groups standing in a row in the group order are one mechanism; unflagged
  links and everything before are the ground, held as today. Old files have no flag and behave
  as before.
- Branch `mechanism` from devel, size M. `make test-fast` after each commit, the documents in
  the same commit, `make ci` before the merge.

## Design

**The block.** `mechanismOf(hg)` is the longest run of consecutive groups of the group order
that are linked, flagged and not suppressed, containing `hg`, and `[hg]` for any other group, so
every caller treats a single group as a block of one. Any other group, a 3D sketch included, ends
a block (documented: datum sketches go before the mechanism or after it). No ground is required:
an ungrounded part keeps six degrees of freedom and the minimum-norm step leaves it where it is.

**The solve** (`pkg/sketch/generate_all.go`). The per-group functions take `[]core.HGroup`,
with the exported single-group names kept as wrappers:

- `writeEqSystem(hgs, params)`: the constraints and equations of every member
  (`WriteEquationsExceptFor` per member); `SuppressDofCalculation` and `AllowRedundant` hold for
  the block when any member sets them; dragged = the window's hook and `sk.pulled` (below).
- `solveHoldingBounds`: the bounds of every member.
- `seedAxisAngles`: only an axis on the same part is left out, not one on another moving member.
- `solveGroups(hgs, params, andFindFree)`: one `Solve`; `Solved.How`, `Dof`, `Remove`, `Timeout`,
  `DofCheckOk` and the diagnostic written to every member alike, so `IsSolvedOkay()` on any
  member answers for the mechanism, as `addUnlessRedundant` and the group list need; the system
  kept under the first member's handle.
- The loop: what is now lines 226 to 289 becomes `prepareGroup` without the prune, and 291 to
  352 `finishGroup`. For a block: `prepareGroup` each member, then `PruneRequestsAndConstraints`
  each (every member's entities now exist), one `solveGroups` if any member is in the range (the
  `resolve` re-solve of the build pass too), then `finishGroup` each in order, and the loop
  steps past the block.
- The range: after `first` and `last` are found, `widenToMechanism` takes them to the block's
  ends, so every way of marking a group dirty reaches the whole mechanism; `MarkGroupDirty` is
  left alone.
- `TestRankForGroup`, the callers of `groupParams`, and `GroupFreedom` (`freedom.go`) build the
  block's system, so `SettleConstraint`, `TryConstrain` and `redundantAsDimension` judge a
  member's constraints against the mechanism.
- `MaxUnknowns` is 2048, seven per part: not a limit anyone reaches.

**The pull** (`pkg/solver/nullspace.go`, `pkg/sketch/mechanism_pull.go`, new).
`System.NullSpace` takes the Jacobian without substitution, reduces it (`rrefFixedColumns` in
`pkg/solver/freedom.go` factored into an `rref` that returns the pivot columns) and reads the null
vectors off the free columns. `pullLinkPoint(he, to)` builds the block's system, the pulled
point's Jacobian from `PointGetExprs` and `Expr.Derivative`, finds the least-squares step in the
null space that moves the point nearest the target, adds it to the parameters and records the
part's seven parameters in `sk.pulled`; the regeneration that follows projects back onto the
constraints with those preferred, so the other members take the correction.
`pullLinkOrientation` does the same with a quaternion residual for the normal drag and the Shift
and Ctrl turns. The entry point is `sk.MovePointTo(he, p)` in `pkg/sketch/move.go`: a member's
`EntityPointNRotTrans` with the block solved and freedom left is pulled; anything else goes
through `PointForceTo` as today. `MovePointsBy`, `api.Entity.MoveTo` and the window's
`UpdateDraggedPoint` all call it; `OrientNormal` routes a member's `NormalNRot` through the
orientation pull. `sk.pulled` is cleared at the end of `generateAll`, which also makes a script's
move weighted as the comment in `move.go` has long claimed. `MarkDraggedParams` gains
`EntityPointNRotTrans` (seven parameters) and `NormalNRot` (four). With no freedom nothing moves.

**Driven joints** (`pkg/script/api/driven.go`, new, after `screw.go`). `Hinge` and `Concentric`
stay as they are. `DrivenMate{Param string; Constraints []Constraint}`;
`AssemblyMenu.HingeDriven(a, b)` makes the hinge, then an axis-angle constraint whose expression
is `angle` plus the angle now (`angle2`... by `freeName`), its point the one of b farthest from the
axis, as Screw picks it; `AssemblyMenu.ConcentricDriven(a, b)` makes the concentric, then a
projected distance along the axis with expression `travel` plus the distance now, Screw's travel
equation. `DrivenMate.SetRange(min, max)` adds two copies of the driving constraint with
`LimitMin` and `LimitMax` (`sk.AddBoundsOf`). In `pkg/core`, the axis-angle constraint accepts a
line segment as its axis: its frame from the two ends, constant local directions turned by the
part's quaternion when both ends are the part's `EntityPointNRotTrans`, numbers when the line is
the ground's; `AxisDirNum` serves `AxisAngleNum`, `seedAxisAngles`, `layoutAxisAngle`
(`pkg/ui/drawconstraint.go`) and `leftout_kind.go`. `CanLimit` admits the axis-angle, and
`Crossed` skips its sign flip for a negative value (a negative angle is not the other side of a
plane). In the window, a bound on a dimension that has an expression is added beside it
(`constrainBound`, `pkg/ui/menuconstrain.go`) rather than converting it, which would drop the
drive. To document: a driven joint has no freedom and does not drag; a motion-study step past a
bound is marked unsolved; on an undriven hinge a bound stops the drag.

**Closing a loop.** A planar four-bar closed with a Hinge (five equations) is redundant in 3D,
its axes parallel, so `addUnlessRedundant` would drop the closing mate. Settled in commit 2:
either the loop closes with a point on a line (two equations, exact) or a member allows
redundancy, which then holds for the block; the guide says which mate closes a loop.

**The file.** `link-moves` as a flag after `link-as-solid` (`pkg/persist/tlspr/encode.go`,
`decode.go`), its entry in `doc/tlspr-format.md`, `fullDocument()` in `decode_test.go` so
`TestEveryFieldRoundTrips` covers it, the undo copy (`pkg/sketch/undo.go`), and the script export
(`export_script_groups.go`) writing `.AsLink().SetMoves(true)`.

**Window and scripts.** The link group's screen (`showLinkedInfo`, `pkg/ui/textscreens.go`) gets
the row `part: ( ) fixed  ( ) moves in a mechanism`, changed through `changeLink` and
`sk.SetLinkMoves` (`group_edits.go`, beside `SetGroupLinkAsSolid`); the group's header a line
`mechanism of N parts, D DOF`. Two menu entries, Mate Hinge, Driven and Mate Concentric, Driven.
In scripts `LinkGroup.Moves()`, `SetMoves(on)`, `Mechanism() []Group` beside `AsSolid`;
`Doc.Describe` names the mechanism; `make generate`.

## Spike first (half a day, nothing committed)

1. The pull. In a test: a ground plate, unflagged, and a bar, flagged, hinged at the plate's edge,
   the bar drawn from its own origin along x. `Entity.MoveTo` on the far end: expect it to snap
   back with `PointForceTo`. Prototype the null-space pull in the same test and see the bar turn.
2. The cost of a frame. Time one `GenerateAll(GenerateDirty)` of ten flagged links of a part of
   about 200 faces, all dirty: `generateLinkedShell` ten times and the assemble. Over about 30 ms
   the leftover "draw by placement matrix while dragging" joins the package.

Done 2026-11-05 (a prototype test, not kept):

1. `PointForceTo` on the far corner of a bar hinged at its origin's upright edge, pulled 30° round
   the hinge: the bar comes back exactly where it was (0.000 mm moved, 31.5 mm from the target),
   as foreseen. The null-space pull (the group's Jacobian reduced to row echelon form, its null
   vectors read off the free columns, the least-squares step in them toward the target, then the
   group solved) reaches the target: one step leaves 0.01 mm at 5°, 2 mm at 30°, 38 mm at 90°;
   two steps 0 at 5° and 0.0006 at 30°; three steps 0.003 at 90°. So the pull iterates, up to
   three steps a frame, each a solve of the block; nine steps in a few milliseconds.
2. Ten links of a 60 x 20 x 4 plate with 96 holes, each moved 0.5 mm, regenerated dirty: 78 ms a
   frame, of which 60% was `MakeFromAssemblyOf` scanning the shell's curves for the next handle
   once per curve added (quadratic). Counting the handles on from the largest, once, as the
   Booleans already do, gives the same handles and 17 ms a frame. Under the 30 ms, so drawing by
   placement matrices stays a leftover.
3. Measured again with the window's drag built (`BenchmarkDragMechanism10`, a chain of ten bars
   of 30 x 6 x 3 at 1920 x 1080): the drag, the pull and the regeneration, 18 ms a frame; the
   paint, 175 ms, of which 86% is triangulating the assembly's shell for display
   (`TriangulateBySurface`, `mendCrossings`), once per frame for the active group, as a drag of any
   linked part in an assembly does. So drawing by placement matrices joins the package after
   all, after the driven joints: each part's own mesh moved by its placement, the assembly's
   mesh made on release.
4. Most of that triangulation was the mend of crossing faces (`mendCrossings`) searching every
   part's triangles against all the others', which touch along their hinges. An assembly now
   records where its parts' surfaces start (`SShell.parts`, set by `MakeFromAssemblyOf`, kept by
   copies, cleared by a Boolean), and the display mesh mends each part alone: paint 177 → 118
   ms, the drag 19 ms, no golden moved. Splitting a shell by the curves its surfaces share was
   tried first and moved single solids' meshes (a two-sided draft, mitred bars, a loft), whose
   surfaces are not always trimmed by one curve: the record is exact. What is left, about 9 ms
   a part, is each part's own triangulation and mend, which only meshes kept per part and moved
   by their placement while dragging would save.
5. Built (`pkg/sketch/display_parts.go`): a link group keeps its display mesh and outlines in
   its part's own frame, made again when the linked shell, the scale, the tolerance or the
   outlines shown change; while the window says a drag is on (`DragWatcher`), the display of an
   assembly of linked parts only, each read as a shell and assembled, is those meshes moved by
   each part's placement, and the drag's end makes it again as ever
   (`RemakeMovedDisplays`). Paint 118 → 18 ms, a frame 36 ms in all with the drag. The moved
   mesh isn't the assembly's own: a part meshed turned can be cut otherwise (1700 triangles
   against 1684 for four bars, the volumes 7e-6 apart), so it is only drawn, during the drag.
   Left: drawing each part by a model matrix in GL, which would save the moving too.

## Commits

1. `sketch, tlspr, api: a link group that moves in a mechanism`. `LinkMoves`, `SetLinkMoves`,
   `mechanism.go` (`mechanismOf`, `mechanismParams`), undo, the file, `Moves`/`SetMoves`/
   `Mechanism`, the script export, `make generate`. Tests: the round trip, `TestSetLinkMoves`
   refused on a sketch group, `TestMechanismOf` (runs, split by another group, a suppressed
   member). No changelog line yet.
2. `sketch: solve the parts of a mechanism as one system`. `generate_all.go` as above,
   `axis_angle_seed.go`, `freedom.go`. `mechanism_test.go` with `mechanismParts(t)`, a bar
   60 x 10 x 4 with holes at both ends drawn from the origin, a frame unflagged and N bars
   flagged, as `matePart` in `mate_test.go`. `TestFourBarSolves` (every member solved, one degree
   of freedom, the loop closed to 1e-6), `TestCrankSlider`, `TestMechanismDirtyFromAnyMember`,
   `TestUnflaggedFourBarAsBefore`; `TestMates` and `TestScrewMate` untouched. A Mechanisms
   section in `doc/guide/bill-of-materials.md`, the changelog, the F9 rows of `ROADMAP.md`.
3. `solver: the null space of a group's Jacobian`. `nullspace.go`, `rref` factored from
   `freedom.go`; tests on made-up systems (six equations in seven unknowns give one vector with
   J·N near 0; full rank gives none; agrees with `TestRank`'s count).
4. `sketch: pulling a point of a mechanism's part moves it along its freedom`.
   `mechanism_pull.go`, `move.go` (`MovePointTo`, `pulled`), `orient.go`, `generate_all.go`,
   `api/entity.go`. `TestPullHingedBar` (ten pulls of 1 mm: the angle grows each time, the pivot
   stays to 1e-6, the end within 0.05 mm of the arc), `TestPullFourBar`, `TestPullStopsAtBound`,
   `TestPullFixedStays`. The dragging paragraph of the Mechanisms section and of
   `doc/guide/interface.md`; this note gets the spike's numbers; the changelog.
5. `ui: dragging a mechanism's part`. `mouse.go`, `app.go`, `drawentity.go` where `ptCanDrag` or
   `entityCanBeDragged` refuse a member's point. Fixture `testdata/ui/mechanism/fourbar.tlspr`
   with `bar.tlspr` beside it, linked by a relative path as `testdata/linkparams`. Scenario
   `ext_mechanism.script` (load, dump, drag the coupler, dump, params, freedom, undo, dump) and
   its golden; `BenchmarkDragMechanism10` in `drag_bench_test.go`; the done-audit row. Run with
   `DragSolidsLate` on and off.
6. `core: an axis-angle about a line, and bounds on it`. `constraint_axis_angle.go`,
   `constraint_inequality.go`, `drawconstraint.go`, `leftout_kind.go`, `axis_angle_seed.go`,
   tests, `doc/guide/limits.md` ("an angle or a turn").
7. `api, ui: driven hinge and concentric mates`. `driven.go`, `AddBoundsOf`, the menu entries,
   `menuconstrain.go`. `TestDrivenHinge` (0 to 90, no freedom, one step of undo and back as
   `TestScrewMate`), `TestDrivenConcentric`, `TestDrivenHingeLimited` (a study 0 to 180 with a
   maximum of 90: the frames past it unsolved), `TestDrivenCrankFourBar` (twelve steps round, all
   solved). Scenario `ext_mate_driven.script` and golden; path pair `path_mechanism`; example
   `examples/scripts/mechanism` (a four-bar, the crank driven, frames by Motion Study) run by
   `cmd/telesapr-cli`'s examples test. The Mechanisms section, the motion study of
   `parameters-and-units.md`, the Assembly rows of `menus.md`, `ai.md`, the done-audit row, the
   changelog, `make generate`.
8. `ui: the mechanism on the link and group screens`. `link_screen.go`, `textscreens.go`,
   `api/describe.go`; the scenario toggles the flag and undoes it.
9. `Goldens (ui): the link screen's part row`: the link-screen goldens the new row shifts
   (`ext_link_step_solid`, `ext_link_params`, `act_rotate_linked`, ...), by
   `scripts/regen-golden.sh`, a commit of its own.

## Checking it

- `go test ./pkg/sketch -run 'Mechanism|FourBar|Pull|Driven|Mates|Screw'`, `./pkg/solver -run
  NullSpace`, `./pkg/core -run 'AxisAngle|Inequality'`, `./pkg/persist/tlspr -run
  EveryFieldRoundTrips`.
- `TELESAPR_UI_SCRIPT=ext_mechanism go test ./pkg/ui -run TestUIScript`, the path pair,
  `make generate-check`.
- In the running window through the MCP server: link the bar four times, flag three, hinge them,
  move the coupler's end by `Entity.MoveTo` in `run_script`, render before and after and see the
  three bars turned; `Assembly.MotionStudy("angle", 0, 360, 12, "f%02d.png")`.
- `make test` after each commit, `make ci` before the merge into devel.

## Risks

- The pull is the one new algorithm. If large pointer jumps misbehave, step and project twice a
  frame before falling back to `PointForceTo`. If `WriteJacobian` wants the substitution's tags,
  substitute first and map through `subMap` as `FreeParams` does.
- The closing joint's redundancy, above.
- The link screen's new row moves text-window goldens: their regeneration is commit 9 alone.
- The first pass's `unknown` bookkeeping and the `DeferShells` path must both run over a block.

## Leftovers, for the roadmap when the package is done

A drag drawing the parts by their placement matrices, the solids remade on release; a 3D sketch
as a member; a drag written back into a driving parameter; several driven parameters in one
motion study; glTF animation channels from a study.

## Degrees of freedom analysis (Kramer 1992), read 2026-11-08

Glenn Kramer, *Solving Geometric Constraint Systems: A Case Study in Kinematics* (MIT Press, 1992;
the short form is his AAAI 1990 paper of the same title) solves exactly what a mechanism is here:
rigid parts joined by mates, each part with six freedoms. It reasons about the parts' freedom
instead of about the equations, and would answer several things the block solve and the pull
above leave open. Nothing of it is built; the ROADMAP's F9 table carries the items.

**The method.** A part has a translational freedom count (0 to 3) and a rotational one (0 to 3),
and an *invariant*: nothing, a fixed point, a fixed axis or a fixed plane. Mates are between
*markers*, points with a frame, and seven kinds cover every lower-pair joint: coincident, in-line,
in-plane, parallel-z, offset-z, offset-x and helical. A mate added to a part consumes a known
number of freedoms given the part's current counts and invariant; a table of *plan fragments*,
indexed by (counts, invariant, mate kind), gives the rigid motion (a translation, a turn about a
point or about an axis) that satisfies the new mate and keeps the earlier ones. Satisfying the
mates one after the other is *incremental assembly*, and every part a chain of mates reaches from
the ground is placed in closed form. Where a marker is confined to a locus (line, circle, sphere,
plane, helix) the new mate is met at a locus intersection, also closed form, and of two roots the
one nearest the previous configuration is taken. A closed loop is cut: the open chain is placed
symbolically and the closing mate leaves one scalar unknown, found by a 1-D search. The plan is
compiled once and run per step, so a simulation is a few dozen rigid transforms a frame, no
Newton. The final freedom status of a part does not depend on the order the mates are taken in
(proved), and redundancy and inconsistency are found by counting, before any numerics.

**What it would change here**, against what the guide's "Closing a loop", "Dragging" and "Not
yet" describe:

1. *Exact freedom with a meaning.* The group screen's DOF is `unknowns - rank` at the current
   position (`TestRank`, `FreeParams`): first order, and wrong at a singular position such as full
   stretch. Counting gives each part a status, "turns about this axis", "slides along this line
   and turns about it", "fixed", that the group screen, the mate's dimension and the drag can show.
   The seven marker kinds also give the mates a model: Hinge is in-line + in-plane, Concentric
   in-line, Flush in-plane, Screw helical; ball (coincident), cylindrical (in-line alone), planar
   (in-plane alone) and slot (point in-plane + in-line) are the joints missing, each with a known
   freedom cost.
2. *The closing hinge accepted.* A planar four-bar closed with a Hinge is redundant in 3D and
   `addUnlessRedundant` refuses it, so the guide tells the user to close with a point on a line or
   allow redundancy. Counting knows the last part has one freedom left and a hinge wants more: the
   extra conditions (axes parallel) are checked for consistency instead of failing the rank test,
   and the mate the user meant is kept as redundant but consistent.
3. *No start position.* Tree-connected parts are placed in closed form, and the loop-closing mate
   is a 1-D search over the remaining freedom (the crank's angle), which finds its root from any
   start on a bounded interval. Dead-centre configurations stop being special.
4. *Drag and motion study as plan execution.* The pull is up to three null-space steps a frame
   (2 mm of error at 30° in one step), and a motion study solves Newton at every step. With a
   compiled plan, a driven angle places every part in closed form, a drag maps the pointer to the
   one freedom and runs the same plan, and so writes back into the driving parameter, one of the
   "Not yet" items.
5. *Root choice by continuity.* The crank-rod loop has two assemblies for one crank angle; Newton
   takes whichever it converges to, which is what makes a mechanism flip during a drag. Nearest
   previous configuration is the rule, and fits the existing pull as it is.

**What it does not change.** Nothing for the 2D sketch solver: the book has no tangency, free
radii or dimension-driven curves, and Newton with the rank test stays right there (sketch
decomposition is Owen and Fudos–Hoffmann, not this). The plan fragment table is hand-written,
the method's own weakness (Bhansali, Kramer and Hoar, JAIR 4, 1996, synthesise the fragments from
first principles): TeleSAPR would write the entries for its mate kinds and the four invariants, a
few dozen short closed-form motions, and keep `solveBlock` as the fallback for whatever the table
does not cover. Kramer's patents on the method (US 5,410,496 and 5,452,238) have expired.

The order that pays first: item 1 (counting and status, no new solving), then 3 and 4 (serial
placement, the compiled plan), then 2 and 5.

**The alternative without the table** (`solver-literature.md`, read 2026-11-08): body-and-cad
rigidity (Haller, Lee-St.John, Sitharam, Streinu, White, CGTA 45, 2012; Lee-St.John and Sidman,
CAD 45, 2013) counts the generic freedom and the dependencies of rigid bodies joined by
coincidence, angular and distance constraints combinatorially, by a pebble game over the
constraint graph, with a proved characterisation for 20 of its 21 constraint types (all but
point–point coincidence, which no mate here is). That gives items 1 and 2 with no hand-written
fragments and is right at full stretch, where the rank is not; positions would still come from
`solveBlock` and the root from continuity. Lee-St.John's joint recognition (CCCG 2012) names the
revolute and prismatic joints a pair of mates implies, which is the status text of item 1.

**Built 2026-11-09: the count, numerically, and the closing hinge.** Items 1 and 2, on branch
`rd12-dof`.

*The decision: the null space, not the table.* Kramer's fragments are a hand-written table, one
entry per (counts, invariant, mate kind), and know only the mate kinds written into it; a user's
own constraints between parts (a point on a line, a parallel, a workplane-projected slide, as the
engine's piston) would each need an entry. The mechanism's Jacobian already holds every
constraint there is, and `System.NullSpace` gives every first-order motion of the block. Each
null vector, read through a part's seven placement parameters, is a twist of the part: its turn
ω, read off R'·Rᵀ by a central difference of the quaternion's rotation (exact: the rotation is
quadratic in q), and the speed of its origin. The span of a part's twists is its freedom: the
rank of the turns is `Turns`, the rest `Slides`, and the invariant is what every twist leaves
still: the screw axis of a single turn (pitch zero a hinge, else a screw with its lead), the
common point of two or three turns (a ball), the normal of two slides with or without a turn
about it (a plane), the direction of one slide. Body-and-cad counting (`solver-literature.md`)
would be generic too and right at full stretch, but it is a second, combinatorial model of the
constraints to keep in step with the first; the numeric count is a few dozen lines on top of
what the pull already uses. Its weakness is the rank's: at a singular position it counts what
the rank there gives, as the group's DOF does, which the guide says.

*The invariant made reliable.* A null space is first order: a four-bar's coupler turns, at each
position, about its instant centre, which first order can't tell from a fixed pin. So each
invariant found is checked at a second position, a step of some 0.1° along each motion of the
block, solved on a system of its own (`freeSystem`), and the params put back: an invariant that
moved more than 1e-4 there is none, and the part is `Moving` ("turns about an axis that moves
with the mechanism"). The crank's and rocker's axes stay put to round-off, the coupler's centre
moves some 0.1 mm.

*Drives and bounds.* A driven joint leaves no freedom, so a count with it would say every
driven part is fixed. The count is of the mates' freedom instead: the bounds are let go, and each
parameter that is all of some dimension's expression (`angle` of "angle + 12.5", `turn` of a
screw's "turn - 18.4") is a drive whose motion, by a central difference of ±0.001 in it with the
dimensions it drives re-evaluated, joins the null space; `DrivenBy` names the drives that add to
what the null space gives the part. A screw's `lead` appears only beside `turn` (turn·lead/360)
and so isn't a drive: the screw counts helical, not cylindrical.

*The closing hinge* (`mechanism_implied.go`). A solve that converges with a rank deficit is
consistent; the rank test refused it all the same. In a mechanism, when every constraint of every
dependency the solver names (`Solved.Groups`) is a mate's, an incidence with no value or a
distance of zero with no expression, the block is solved, `Solved.Implied`, and the group's
screen marks those constraints "(implied)". A dimension in a dependency is refused as before
(`TestFourBarRedundancyRefused`), and a loop that can't close doesn't converge. One consequence
had to be met: in a mechanism already redundant this way, `SettleConstraint` found every
dimension added after redundant and made it a reference, which broke the engine's flush and
concentric mates (4 DOF left, no stroke); it now compares the rank with and without the
dimension there (`addsRank`).

*Left*: item 1's marker model and the missing joints (ball, cylindrical, planar and slot as mates
of their own; a point on a point already makes a ball joint); items 3 to 5. In `ROADMAP.md`.

## The examples (2026-11-06)

The first examples, bars hinged by their upright edges (`hinge`, and a four-bar as `mechanism`),
were replaced at the user's asking by things one recognises: `examples/scripts/lid`, a box's lid
on a driven hinge kept between 0 and 110°, and `examples/scripts/engine`, a slider-crank of a
frame, a crank, a connecting rod and a piston, each joint a pin in its hole (Concentric and Flush;
`HingeDriven` takes two circles since, a pin and its hole) and the piston's slide a point on the
cylinder's axis and a line parallel to it, both made in the XY workplane, where the stacked parts
hold in projection: that closes the loop with no redundancy, one freedom left, the crank's. Its
stroke read every 5° is 23.99 against 24. The guide page follows the lid, its fixtures
`testdata/ui/mechanism/box.tlspr` and `lid.tlspr`, the lid drawn beside the box at the rim's
height: drawn on the box, its back edge lay on the box's and the one of the active group is the
only one a click picks; drawn at the box's bottom, the hinge's solve tilted it on the way up,
turning being the cheaper change of its unknowns.
