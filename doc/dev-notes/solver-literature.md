# The constraint-solver literature against TeleSAPR's solver (read 2026-11-08)

A check of what the research on geometric constraint solving offers this program: what the
field has settled, what it calls open, and which of it maps onto a weakness of `pkg/solver` or
`pkg/sketch` as they stand. The last section is the work this suggests, in the order that pays.

## How the search was made

Google Scholar sits behind a bot check and Semantic Scholar's open API rate-limits an anonymous
sweep, so the survey went to the primary sources directly: arXiv (the 2022 review and the papers
it cites), ACM and Elsevier abstracts, Hoffmann's Purdue pages, the CAD'24 proceedings, and two
engineering threads from 2026 that report measurements of a shipping sketch solver: the FreeCAD
Project Association grant proposal on planegcs's redundancy diagnosis and SolidCraft's issue on
constructive solving. The reading list is at the end.

What the field published in 2023 to 2026 is learning-based constraint *generation* for sketches
and reviews. Nothing in that span changes the numerical core; the results that matter here are
from 1991 to 2019, plus the two engineering threads.

## The solver as it stands

`pkg/solver` is a port of SolveSpace's `system.cpp`. The pipeline of `System.Solve`
(`system.go`): write the block's equations; substitute away every `a - b = 0` between two
unknowns, keeping a dragged parameter (`SolveBySubstitution`); solve each one-unknown equation
on its own; build the Jacobian symbolically (`WriteJacobian`, memoised, `jacobian_memo.go`);
test the rank with the rank-revealing sparse QR (`qr.go`, Eigen 3.4's SparseQR with COLAMD and
row matching, pivot threshold `20·(m+n)·max‖col‖·eps`); run Newton on each connected component
of the Jacobian's pattern (`components.go`); test the rank again after the solve, and if it is
deficient call the result `SolveRedundantOkay` and name the constraints to blame.

The Newton step (`NewtonSolve`) is the minimum-norm least-squares solution of `J·dx = F`, with
dragged parameters scaled 1/20 so the step moves them more. There is no line search and no
damping, only an abort on NaN or a value past 1e11; 50 iterations; convergence when every
residual is under `LengthEps/100`. During a drag the factorisation is reused while each step
halves the residual (`drag_step.go`).

Redundant constraints are found by `FindWhichToRemoveToFixJacobian` (`system.go`): for each
constraint in turn the system is written without it and the rank re-tested; point-coincident
constraints are tried last; the search has a 1 s timeout (`pkg/sketch/generate_all.go`,
`timeoutRedundantConstr`) and names one constraint. `TryConstrain` (`pkg/sketch/constraint.go`)
drops a new constraint when the rank test says it made the group redundant; `SettleConstraint`
turns a redundant dimension into a reference. Degrees of freedom are `unknowns - rank` at the
current position (`TestRank`); which parameters are free comes from `FreeParams`
(`freedom.go`, union-find on `a = b` rows, propagation, then dense RREF); the mechanism pull
uses an orthonormal null space (`nullspace.go`). All of it is first order at the current
configuration.

Mechanisms (F9) are the same Newton on a block of consecutive linked groups
(`pkg/sketch/mechanism.go`); the pull (`mechanism_pull.go`) takes up to 32 null-space steps and
re-solves after each. `f9-mechanism-plan.md` ("Degrees of freedom analysis") proposes Kramer's
counting for mechanisms and names its weakness, the hand-written plan-fragment table.

The performance study of 2026-10-20 (`performance.md`) found the sparse QR, not the Jacobian, to
be the solver's cost, and row matching plus early exit from the rank-only factorisation brought a
400-segment chain's Newton-step factorisation from 28.5 ms to 0.33 ms.

## What the literature says, against the code

### Redundancy: name the group, not one constraint

Zou and Feng, *On limitations of the witness configuration method* (arXiv 1904.00526, 2019),
section 3.3, and their review (arXiv 2202.13795, 2022), section 6.4: every published detector
of over-constrained parts is greedy. It finds a maximal independent set of equations from a
seed and then declares each remaining equation dependent on that set. The result depends on the
seed and can miss the smaller group. Their example, five linear equations in three unknowns:

    E1: x + y + z = 0      E4: x + 2y + 3z = 1
    E2: 2x + y + z = 1     E5: 2x + 4y + 6z = 2
    E3: 3x + 2y + z = 1

The greedy methods seeded at E1 report {E1, E2, E3, E4} and {E1, E2, E3, E5}; the minimal group
is {E4, E5}. Hu, Kleiner and Pernot (CAD 90, 2017) combine structural decomposition with
numerical rank analysis for the same problem and separate redundant from conflicting. The
FreeCAD proposal (FPA-grant-proposals #106, closed September 2026) reaches the same conclusion
for planegcs: a rank-deficient system is a *group* of constraints that jointly over-determine a
freedom, so report the QR null-space group rather than one arbitrary "redundant" constraint, and
keep three notions apart: the structural count, the positional Jacobian rank, and feasibility.

Here: `FindWhichToRemoveToFixJacobian` is the greedy one-at-a-time method, implemented as O(n)
refactorisations with a timeout, and it names one constraint. The left null space of `J` (the
null space of `Jᵀ`, or the trailing rows of `Q` from the rank-revealing QR already computed)
gives every dependency group in one factorisation and no timeout. The user-facing text then
names the group: the user sees which constraints over-determine each other and chooses which to
drop, instead of being told one that the search order happened to pick.

### The witness configuration: where the rank is taken matters

Michelucci and Foufou (CAD 38, 2006), Thierry et al. (CAD 43, 2011), Foufou and Michelucci
(Information and Computation 216, 2012): the Jacobian's rank is only generic at a *witness*, a
configuration that already satisfies the degenerate constraints (incidence, parallelism,
perpendicularity, tangency). At an arbitrary configuration a bilinear constraint's rows can look
independent or dependent wrongly. Zou and Feng's 2019 paper shows the method itself is not
complete (the degree-of-rigidity patch depends on how an entity is represented, their Table 1),
but the witness requirement is the point that applies to any Jacobian-rank test.

Here: the rank test *after* the solve is at a witness, since the solve satisfied the constraints.
The test *before* the solve runs at the previous configuration, and a constraint just added
(parallel between two lines not yet parallel, point-on-line with the point off the line) is not
yet satisfied there. Whether `TryConstrain` decides on that pre-solve rank is to be read; if it
does, a false "redundant" is possible on a constraint the solve would have satisfied. A test:
two lines at an angle, add Parallel, expect it kept.

The rank tolerance is already scale-relative, which is what the FreeCAD proposal asks for; no
action there.

### Root selection: the flips

Hoffmann, *Summary of basic 2D constraint solving* (J. PLM 1, 2004): of a subproblem's up to
four roots a solver takes the one with the same *order type* as the sketch (the orientation of
each triple of points), on the hypothesis that the sketch is the intent; the rule is invariant
under decomposition order; it fails when a dimension edit needs a different order type, and no
automatic rule of general use exists (a non-self-intersecting polygon is NP-hard). Bettig and
Hoffmann's survey (JCISE 11, 2011), section 4.3, lists root identification among the open
problems. Fudos and Stamati (CAD'24, pp. 288-294) propose a neural network to recommend the root
of each construction step; a proposal, no results.

SolidCraft issue #203 (closed 2026-10-14) measured a Levenberg–Marquardt sketch solver that
starts from the current geometry: a fan-triangulated 12-gon with dimensions edited by 0.5 to
2.5× mirrored a triangle in 407 of 500 cases; a two-link arm dragged in one step flipped its
elbow in 356 of 1000; an arc end dragged past its start went from a 15° to a 345° sweep. Two
measures took every count to zero: construction of what two constraints determine, and
*continuation*, stepping an edited dimension from its old to its new value and splitting a long
drag into steps, each solved from the previous positions; arc sweeps never wrap. Edits that would
still fold are refused with the constraints named.

Here: Newton takes whichever root it reaches, in sketches and in mechanisms; the Kramer note's
gain 5 asks for the nearest root. The mechanism pull already steps; dimension edits and 2D drags
do not. Continuation and an order-type check are numerical, cheap, and need no constructive
solver. The order-type check compares the sign of the oriented area of each constrained triple
(and each arc's sweep) with the previous solution; on a flip the solve is retried from the seed
reflected across the constraint's line, and if it still flips the result is kept with a
diagnostic on the group, never refused.

### Constructive solving

Owen (Solid Modeling 1991, triconnected decomposition), Bouma, Fudos, Hoffmann, Cai, Paige
(CAD 27, 1995), Fudos and Hoffmann (ACM TOG 16, 1997, triangle decomposition with a correctness
proof), Hoffmann, Lomonosov and Sitharam (J. Symbolic Computation 31, 2001, decomposition plans
and the frontier algorithm that finds every solvable subgraph). Graph-constructive solving is
what the commercial 2D solvers do: decompose into clusters solvable in closed form, merge three
clusters sharing elements pairwise, and the equations are univariate quadratics. Fudos, Hoffmann
and Joan-Arinyo's handbook chapter (arXiv 1608.05205) is the current statement. Zou and Feng's
review, section 1: the whole solve's cost is the largest subsystem's.

Here: not now. It is a second solver with its own competence limits (variable-radius circles and
tangency need extra patterns, Hoffmann 2004; 3D has no complete pattern set). The reason the
literature gives for decomposition, speed, does not bite: the sparse QR with row matching
already makes a 400-segment chain's step 0.33 ms. Its other gift, order-type-preserving roots,
is what continuation and the order-type check approximate. It stays the long-term option if the
flips survive those.

### Damping and line search

planegcs offers DogLeg, Levenberg–Marquardt, BFGS and SQP; the neoscad solver runs
Levenberg–Marquardt with a fixed damping schedule then polishes with Gauss–Newton; the review,
section 3.2, calls optimisation methods less sensitive to the start than Newton.

Here: undamped Newton with a NaN guard. A backtracking line search on ‖F‖, engaged only when the
full step raises it, would help large edits and change nothing where Newton already converges
monotonically. It may still move goldens (the last bits of a solved sketch are what the
fixtures record), so it is measured before it is adopted, and last.

### Degrees of freedom of a mechanism without a hand-written table

Haller, Lee-St.John, Sitharam, Streinu and White, *Body-and-cad geometric constraint systems*
(arXiv 1006.1126; Computational Geometry: Theory and Applications 45, 2012): rigid bodies in 3D
joined by pairwise coincidence, angular and distance constraints, 21 constraint types in all,
with the infinitesimal rigidity theory, a counting condition (*nested sparsity*, necessary but
not sufficient) and a pebble-game algorithm for it. Lee-St.John and Sidman, *Combinatorics and
the rigidity of CAD systems* (arXiv 1210.0451; CAD 45, 2013): a combinatorial characterisation
of generic minimal rigidity for 20 of the 21 types (all but point–point coincidence), stated as
a partition into edge-disjoint spanning trees, with deterministic algorithms that use no
numerics. Lee-St.John, *Kinematic joint recognition in CAD constraint systems* (CCCG 2012):
recognising the revolute and prismatic joints that pairwise constraints between two bodies
imply.

Here: this is the alternative to Kramer's plan-fragment table for the F9 note's gains 1 and 2,
an exact freedom count with a meaning ("turns about this axis") and a consistent redundancy
accepted (the closing hinge of a planar four-bar). The count is generic and combinatorial, so it
is right at full stretch where the Jacobian rank is not; positions still come from Newton and
roots from continuation. Point–point coincidence, the one type left out, is not a mate here.
Recorded in `f9-mechanism-plan.md`; not planned.

### What the field calls open, and that this program lives with

No complete combinatorial characterisation of rigidity in 3D exists (the double banana; Fudos
and Stamati's CAD'24 Conjecture 1 is unproved); valid parameter ranges of a dimensioned sketch
are a high-dimensional, disconnected set that only small subproblems map (Bettig and Hoffmann
4.2.4); the witness method has no proof (Zou and Feng). So the numerical rank test at the solved
configuration stays the final word on redundancy, with the combinatorial count, if ever built,
as the first word.

### Not relevant to the solver

Under-constrained sketches and dragging: the minimum-norm step with scaled dragged parameters is
the standard numerical answer (planegcs's "temporary constraints" are the same idea); van der
Meiden's non-rigid cluster rewriting and the handbook chapter add nothing a numerical solver
needs. Variable-radius circles by Laguerre maps, GPU subproblem solving: constructive-solver
concerns. Learning-based work (Vitruvion, ICLR 2022; Autodesk's *Aligning constraint generation
with design intent*, arXiv 2504.13178, 2025, which uses Fusion's solver as the reward; CAD-
Assistant, arXiv 2412.13810) generates constraints for a sketch; it belongs with the MCP side,
which already reports solve status, and asks nothing of `pkg/solver`.

## The work this suggests

Branch `solver-literature` from `devel`, in this order.

1. **Dependency groups from the null space.** A `DependentRows` in `pkg/solver` beside
   `nullspace.go`: the null space of `Jᵀ` (or the trailing rows of `Q`), each null vector's
   non-zero rows one group, mapped to constraints as `FindWhichToRemoveToFixJacobian` maps them.
   `System.Solve` reports the groups on a deficient post-solve rank; the brute-force search
   stays only as a fallback and its timeout goes if unreachable. The group's diagnostic text in
   `pkg/sketch` names every constraint of the group; the text window lists and highlights them;
   the script accessor returns the group; `make generate`. Tests: the five-equation example above
   (expects {E4, E5}); the redundant fixtures under `testdata/`; a UI scenario and the
   path-equivalence test. While there, read `TryConstrain` for the witness point and add the
   two-lines-then-Parallel test; fix only if it fails. A line in `CHANGELOG.md`.
2. **Continuity on dimension edits and drags.** In `pkg/sketch`, when a dimension changes by
   more than a fraction of its magnitude (start at 25%) or a dragged point moves more than a
   fraction of the sketch's extent, step from old to new in `k` steps solving at each, reusing
   the clamping of `mechanism_pull.go`; keep `drag_step.go`'s reuse of the factorisation inside
   a step. The order-type check in a `flip.go` as described above. Tests: a `sweep.Grid`
   modelled on SolidCraft's cases (the 12-gon with edits 0.5 to 2.5×, the two-link arm, the
   arc dragged past its start) counting flips before and after; and a test over every fixture
   that regeneration never engages continuation, so no golden moves. UI and script reach the
   same dimension-edit and drag entry points; scenario and example. A line in `CHANGELOG.md`.
3. **Recorded, not done.** The line search (measure golden moves first); the constructive
   pre-pass if flips survive item 2; body-and-cad counting and joint recognition for F9.

`make test-fast` after each commit, `make ci` before the merge; `TestDocScenarios` and
`TestReferencePages` for the guide; grep `doc/` and `ROADMAP.md` for "redundant constraint"
wording that item 1 turns into a group.

## Reading list

- Zou, Feng. A review on geometric constraint solving. arXiv 2202.13795 (2022).
  https://arxiv.org/abs/2202.13795
- Zou, Feng. On limitations of the witness configuration method for geometric constraint solving
  in CAD modeling. arXiv 1904.00526 (2019). https://arxiv.org/abs/1904.00526
- Michelucci, Foufou. Geometric constraint solving: the witness configuration method. CAD 38(4),
  2006. Thierry, Schreck, Michelucci, Fünfzig, Génevaux. Extensions of the witness method. CAD 43,
  2011. Foufou, Michelucci. Interrogating witnesses. Information and Computation 216, 2012.
- Hu, Kleiner, Pernot. Over-constraints detection and resolution in geometric equation systems.
  CAD 90, 2017. https://doi.org/10.1016/j.cad.2017.05.007
- Hoffmann. Summary of basic 2D constraint solving. J. Product Lifecycle Management 1, 2004.
  https://www.cs.purdue.edu/cgvlab/www/resources/papers/Hoffmann-IJPLM-2006-Summary_of_Basis_2D_Constaint_Solving.pdf
- Bettig, Hoffmann. Geometric constraint solving in parametric computer-aided design. JCISE 11,
  2011, section 4.3 "Open problems".
  https://www.cs.purdue.edu/cgvlab/www/resources/papers/Bettig-Comp_and_Info_Sci_in_Eng-2011-Geometric_Constraint_Solving_In_Parametric_CAD.pdf
- Owen. Algebraic solution for geometry from dimensional constraints. Solid Modeling 1991.
  Fudos, Hoffmann. A graph-constructive approach to solving systems of geometric constraints.
  ACM TOG 16(2), 1997. Hoffmann, Lomonosov, Sitharam. Decomposition plans for geometric
  constraint systems, parts I and II. J. Symbolic Computation 31, 2001.
- Fudos, Hoffmann, Joan-Arinyo. Tree-decomposable and underconstrained geometric constraint
  problems. Handbook of Geometric Constraint Systems Principles, 2017. https://arxiv.org/abs/1608.05205
- Haller, Lee-St.John, Sitharam, Streinu, White. Body-and-cad geometric constraint systems.
  CGTA 45, 2012. https://arxiv.org/abs/1006.1126
- Lee-St.John, Sidman. Combinatorics and the rigidity of CAD systems. CAD 45, 2013.
  https://arxiv.org/abs/1210.0451
- Lee-St.John. Kinematic joint recognition in CAD constraint systems. CCCG 2012.
  https://2012.cccg.ca/papers/paper61.pdf
- Kramer. Solving Geometric Constraint Systems: A Case Study in Kinematics. MIT Press, 1992
  (in `f9-mechanism-plan.md`).
- Fudos, Stamati. Graph constructive geometric constraint solving: challenges and machine
  learning. CAD'24, 288-294. https://www.cad-conference.net/files/CAD24/CAD24_288-294.pdf
- Casey et al. (Autodesk Research). Aligning constraint generation with design intent in
  parametric CAD. arXiv 2504.13178 (2025).
- FreeCAD FPA grant proposal #106, Make Sketcher constraint redundancy diagnosis reliable (2026).
  https://github.com/FreeCAD/FPA-grant-proposals/issues/106
- SolidCraft issue #203, Sketch solver: constructive solving so edits and drags keep their
  solution (2026). https://github.com/bherbruck/solidcraft/issues/203
