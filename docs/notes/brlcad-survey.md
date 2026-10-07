# BRL-CAD as a source of algorithms and ideas for TeleSAPR

Date: 2026-10-07. Checkout surveyed: `/home/msorc/install/design/brlcad/brlcad`
(release line 7.44/7.46, last commit 2026-08-28). Read-only investigation; nothing changed.

## Context

TeleSAPR's open work is all kernel work (ROADMAP-JUMP.md "Open work"): coincident and
tangent contact in Booleans (F5: off-axis cap bore 40 of 80, the leaning wedge on a plate,
the 28.4 sweep joint), meshing leftovers (twisted bar 1.6%, NIST_6 pole Euler number,
Schenkel Euler), and the rest of the fillet work (run-offs onto curved faces, mixed radii,
chamfers ending on curved faces). The question was whether BRL-CAD holds algorithms,
approaches or ideas that would help. Standing decisions that frame the answer: the kernel
stays Go-native with no cgo (no binding of BRL-CAD or OpenNURBS), kernel topology (9.2) is
left out, and no feature ships silently wrong geometry.

## Verdict

BRL-CAD is useful as a **reference for a handful of specific techniques**, not as a kernel
to adopt and not for fillets. In order of value to TeleSAPR's open items:

1. its treatment of **coincident and tangent contact** in two different Booleans (the NURBS
   one in libbrep and the polygonal one in libnmg) maps straight onto F5;
2. its **robust point projection** design (damped Gauss-Newton with trust region, line
   search, periodic wrap, acceptance by 3D error not by step) fixes the class of problem
   behind `ClosestPointToUV` leaving points 6 µm short;
3. its **watertight tessellation** (edges discretised once in 3D, constrained Delaunay in a
   rescaled uv, validate-and-repair) replaces the ear-clipper heuristics that every step 24
   fix patched (`grazed`, `folded`, `cutOff.crosses`, `poleSliver`);
4. its **ray-cast CSG evaluator** is an independent oracle for Booleans and mass properties
   that needs no surface intersection, usable in `pkg/check` and as an AI probe;
5. its **STEP import repair ladder** and accounting rules match step 27's leftovers.

What BRL-CAD does not have: any fillet, blend, offset-surface or rolling-ball code (searched
libbrep, librt, libged: none), a sketch constraint solver (libpc is an abandoned Gecode
experiment), or a NURBS Boolean better than TeleSAPR's. Its NURBS Boolean does not stitch
topology between result faces and uses a weaker intersection tracer than
`intersectByMarching`; its own guide lists "may produce incorrect output due to unhandled
intersection cases" first among the limitations.

## What is where (sizes by `wc -l`)

| Library | Lines | What it is | Relevance |
|---|---|---|---|
| libbrep | ~52k | NURBS SSI/CSI, Boolean, pullback/projection, CDT tessellation, shape recognition; on top of OpenNURBS | High: projection, overlap handling, CDT |
| libnmg | ~65k | Weiler radial-edge polygonal B-rep with Booleans (Muuss/Butler 1991) | Medium: fuse-first and the 8-way classification; the structure itself is not wanted |
| libbg | ~31k (19k vendored) | Geometry algorithms: trimesh checks/repair, tri-tri, CDT (detria), earcut, Clipper, hulls, GTE | Medium: trimesh validation, CDT choice |
| librt | ~236k | Ray tracer, CSG evaluation (boolweave/booleval), 40 primitives, db5 model | Medium: the CSG oracle, pipe bends, tolerance policy |
| libanalyze | 17k | Volume/mass/centroid/moments by ray grids with 3-view convergence, raydiff, overlaps, nirt | Medium: oracle and AI probe |
| conv/step | 125k | STEPcode-based AP203/214/242 import with a repair ladder; g-step export incl. native CSG | Medium: repair ideas, export of the feature tree |
| libged | 188k | Commands: facetize (Manifold mesh Booleans + perturbation planner), lint, gqa, search, edit | Low-medium: ideas for MCP tools |
| libpc | 0.9k | Abandoned constraint experiments | None |

Licence: the libraries are LGPL 2.1; TeleSAPR is GPL-3.0-or-later. Re-implementing ideas in
Go is unrestricted. Copying code lines is allowed under LGPL 2.1 §3 (conversion to GPL) with
the notices kept; prefer re-implementation, since the Go code is structured differently
anyway. Vendored third-party parts of libbg (detria, earcut, Clipper, GTE, geogram) are
permissive.

## Open TeleSAPR problems mapped to BRL-CAD techniques

### F5: coincident and tangent contact (cap bore 40/80, leaning wedge, 28.4 joint, teapot handle)

TeleSAPR today: `MakeCoincidentEdgesInto` (pkg/srf/surfinter.go) handles coincident
*planes* with a sameNormal flag; the classifier `ClassifyEdge`/`raycastClass`
(pkg/srf/raycast.go) decides in/out/on by ray casts with tangent probing. The F5 failures
are an edge of one solid lying in a face of the other, a cap coincident with several flat
patches at once, and two coplanar opposite faces with equal outlines.

BRL-CAD offers two complementary treatments:

- **libnmg, fuse first then classify by identity** (`nmg_model_fuse`, src/libnmg/fuse.c:1894,
  `nmg_bool`, bool.c:601). Before any intersection: vertices fused within `tol.dist`
  (sort by X, sweep), edges broken on vertices that lie within tolerance of their interior
  (T-junctions), coplanar faces made to share one plane record, edges fused. After the cut,
  an element is "ON" when a *use* of the same vertex or edge belongs to the other shell,
  which is an identity test and not a fresh epsilon test. ON is split into **on-shared**
  (same direction and same left vector) and **on-anti** (opposite), and each Boolean is a
  table over 8 classes (eval.c:143-180): union keeps shared and kills anti, subtraction
  (B inverted) keeps anti, intersection keeps shared. Edges with both ends ON are classified
  by the point at 0.5, then 0.1, then 0.9 (class.c). The wedge-on-plate case (an edge in a
  face) and the 28.4 joint (two opposite coplanar faces plus an edge on an edge) are exactly
  the cases this table was built for.
- **libbrep, overlap regions found by isocurves** (`find_overlap_boundary_curves`,
  src/libbrep/intersect.cpp ~3538; algorithm comment ~3645). The overlap between two Bezier
  patches is bounded by patch boundaries, so the boundary of a coincident region lies on
  isocurves at the Bezier knots. Those isocurves are intersected with the other surface
  (CSI), kept where the surfaces coincide on one side only, split against each other, and
  chained into closed loops: `ssx_overlap` events. Subsurfaces inside an overlap are pruned
  from the transverse search, and in the Boolean a coincident face is settled by one sample
  point and a normal comparison (boolean.cpp ~3790). TeleSAPR's cap-over-four-flat-patches
  case is a multi-patch overlap region of this kind; the isocurve rule gives its boundary
  without guessing.
- **The lessons file** `src/librt/brep_csg_boolean.txt` (2026-08-01) §6.6: point-in-solid by
  several rays with explicit VALID / GRAZING / COINCIDENT / FAILED results instead of a
  majority vote; tight bounding boxes of *trimmed* faces; residual check after a uv
  projection is clamped; "resource exhaustion is not non-intersection".

Recommendation: a two-day spike on the wedge-on-plate fixture (the simplest F5 case: all
planes) that adds a pre-Boolean fuse of the two shells' coincident vertices, edges and
planes and extends `keepEdge`/`keepRegion` (pkg/srf/boolean_trim.go) to the shared/anti
split. The cap bore comes next with the isocurve overlap boundary.

### Projection accuracy (`ClosestPointToUV` 6 µm short, `nearestOn` polish, rolling-ball `foot`)

`PullbackCurve.cpp` `surface_closest_point_seeded` (~1091-1240) is the design to copy into
`SSurface.ClosestPointTo` (pkg/srf/surface.go:384):

- Gauss-Newton on the residual with a tiny damping term, explicit 2×2 solve;
- a per-direction step cap (trust region) and up to 12 halvings of backtracking, a step
  accepted only when the 3D distance strictly drops;
- periodic directions kept on an unwrapped branch but *evaluated wrapped*, open directions
  clamped; the best point seen is kept;
- success decided by the 3D lift (does S(u,v) come within the caller's tolerance of P), never
  by the step size; search limits (iterations, nodes) separate from acceptance;
- fallbacks: `surface_closest_point_multiseed` (3 seeds per knot span, refine the nearest
  few) and a branch-and-bound over Bezier span boxes with min/max distance per box.

This is about 500 lines of logic and removes the need for callers to polish
(`nearestOn` in pkg/topo/rollingball.go). It is the cheapest item with the widest effect
(fillets, pullback of edge curves, picking, STEP import).

### Meshing leftovers (twisted bar 1.6%, NIST_6 Euler −6, Schenkel Euler −34, grazed ears)

TeleSAPR's `UvTriangulateInto` (pkg/srf/triangulate.go) ear-clips a uv outline against grid
points; each step 24 fix is an ear-ordering heuristic. BRL-CAD's `ON_Brep_CDT_Tessellate`
(src/libbrep/cdt/cdt.cpp:351, edge.cpp, surf.cpp, mesh.cpp) does it differently:

1. **each B-rep edge is discretised once, in 3D**, and both faces reuse the same points
   (watertight by construction, no T-junction snapping afterwards);
   segment length from chord sag, normal and tangent change (`tol_need_split`), the median
   of neighbouring segment lengths (`curved_edges_refine`), and "too close in 2D for their
   length" refinement with an R-tree (`refine_close_edges`);
2. interior uv points sampled by recursive quadsection, sized from the face's own boundary
   segment lengths, finer near trims, points too close to trims filtered;
3. **uv rescaled to unit aspect** before a **constrained Delaunay with holes**
   (`bg_nested_poly_triangulate`, detria), which never produces the strip and sliver ears
   the ear clipper does (the twisted bar's outer-cell fans are exactly that);
4. poles: singular trims split in 2D, the degenerate 3D triangles dropped afterwards, vertex
   normals averaged (NIST_6's pole problem is a trim running into the pole with no row point
   left, which a CDT with interior points does not have);
5. **validate and repair**: edges used by more than two triangles, triangles whose normal
   disagrees with the surface, misoriented pairs; a bad patch is grown, reprojected to a
   best-fit plane or flattened by LSCM, retriangulated (`cdt_mesh_t::repair`, mesh.cpp:3629);
   then a whole-mesh closed-manifold check (`bg_trimesh_solid2`, one sorted half-edge pass:
   unmatched, excess, misoriented, degenerate in O(n log n), a good oracle for `pkg/check`).

Recommendation: a spike that triangulates one patch type (the twisted ruled face, then a
pole patch) with a Go constrained Delaunay (a port of detria or earcut-with-Steiner-points;
no cgo) behind a flag, measured against the current mesher on the three fixtures above.
This is the largest item; it is also the one that retires a whole class of fixes.

### Booleans: an independent oracle, and a last-resort fallback

- **Ray-cast CSG** (`rt_boolweave`/`bool_eval`/`rt_boolfinal`, src/librt/bool.c:150,
  1275, 1423; `rt_shootray`, shoot.c:531): each solid returns in/out segments along a ray,
  segments are woven into partitions, and the group tree is evaluated per partition. No
  surface intersection, so it is right whenever the leaf solids are. With analytic leaves
  (plane, cylinder, cone, sphere, torus quartic; extrusion of a sketch by 2D parity as
  `rt_extrude_shot` does, extrude.c:857; revolution as `rt_revolve_shot`) a Go version gives
  volume, centroid and inertia of *any* feature tree by `libanalyze`'s rule: three
  orthogonal grids, halve the spacing until the three results agree within tolerance
  (api.c:585). `raydiff` (src/libanalyze/raydiff.c:191) then measures where the B-rep mesh
  and the implicit tree disagree, which is a Boolean oracle that needs neither OpenCASCADE
  nor FreeCAD. `rt_plot_cell` (shoot.c:454) style per-ray dumps are a debugging aid worth
  having. Fits `pkg/check` and the sweeps directly; also an MCP "probe along this line"
  tool as `nirt` is.
- **Mesh Boolean fallback with deterministic perturbation** (libged facetize: plan.cpp and
  Manifold; NEWS 7.38.0: success 88.9% to 98.9% and 7× faster on the example assemblies).
  Operands that touch coplanar are perturbed by 1.0-1.9× tolerance, deterministically per
  tree slot, subtracted tools grown so they leave no slivers, with per-method time limits.
  For TeleSAPR this conflicts with "never silently wrong geometry", so it can only be an
  explicit, reported mode ("approximate" on the group), and there is no Go Manifold; noted
  as an option, not recommended now.

### STEP leftovers (NIST_6 poles, Schenkel's Euler number, tolerance rules)

`src/conv/step/STEPImportArchitecture.md` and the repair units:
`STEPBrepSeamRepair.cpp` (`repair_closed_surface_seam_crossings`,
`repair_missing_singular_trims` for poles of spheres and cones, `repair_invalid_open_pcurves`),
`STEPBrepPullback.cpp` (`refine_surface_pullback_seeded`, `relocate_closed_surface_loop_seam`),
`STEPBrepPeriodic.cpp`. Rules worth adopting as written: model-space acceptance uses the
file's declared or a measured tolerance, never a solver threshold; a ladder of faithful
construction, then safe repair, then tagged inference; exactly one accounted result per
entity, so the report says what was repaired or lost. Analytic surfaces are read as exact
finite forms sized from the trim bounding box (OpenNurbsPrimitives.cpp:121), which TeleSAPR
already does. Export: `g-step --native-csg` writes the tree as CSG_SOLID/BOOLEAN_RESULT with
B-rep operands as fallback; TeleSAPR's "CSG and swept solids in STEP" row could take the
export half cheaply once the group tree is walkable.

### Sweep corners (28.4) and the pipe primitive

`src/librt/primitives/pipe/pipe.c`: a corner between two straight runs is never mitred; the
runs are trimmed back by `bendradius·tan(θ/2)` (pipe.c:4531) and a torus segment joins
them, so the run's end section and the bend's end section are the same circle and no
mutual intersection is needed. `rt_pipe_validate_impl` (4399) refuses bends that overlap
and `rt_pipe_snap_bend_overlap` (4553) repairs them. For TeleSAPR this suggests a
"corner radius" on path sweeps (round the path, not the solid) as the sound alternative to
28.4's bent-cap union, for any profile by revolving the profile about the bend axis. It does
not solve the line-meets-arc corner as asked, but gives users the joint they usually want.

### Smaller ideas

- Tolerance policy: calculational `{dist, dist_sq, perp, para}` (include/bn/tol.h) kept
  apart from tessellation `{abs, rel, norm}` with floors for tiny parts
  (primitive_util.c:116, 179) and segment count `θ = 2·acos(1 − err/r)` clamped 6..3600.
- Trim test by monotone trim pieces: trims split at horizontal/vertical tangents, a point
  classified by the nearest piece above it, distance to the nearest trim for free
  (`BBNode::isTrimmed`, `BRNode::getCurveEstimateOfV`). Only if `SBspUv` shows up in profiles.
- Ray–surface hit cleanup checklist from `rt_brep_shot` (brep.cpp:1062): grazing cutoff at
  0.001°, near-miss crack closing on adjacent faces, collapse of repeated INs/OUTs.
- MCP: `search` (find-style predicates over the model tree: -name, -type, -attr, -above,
  -below, -bool), `edit --list-all-prim-ops=json` (a machine-readable operation catalogue),
  `nirt` shotline reports, `lint --raytrace` (Crofton volume vs analytic per primitive).
- Test corpora to re-create as scripts: `src/proc-db/torii.c` (interwoven tori, many
  near-tangencies), `menger.c` (deep subtraction trees), `csgbrep.cpp` (every primitive as
  a B-rep), `gears.c`, `surfaceintersect.cpp`, `brep_invalid.cpp`.
- Bezier-to-biarc approximation (`sketch_tess.cpp`) for DXF/G-code export.
- Bugs seen in BRL-CAD not to copy: `bg_vert_tree` lookup checks one leaf only;
  `tri_tri.c` absolute EPSILON 1e-6; `ON_Surface_IsStraight` ignores its loop indices;
  `nmg_check_radial` can never fail; `bg_trimesh_hanging_nodes` indexing.

## Not worth taking

- NMG's radial-edge structure as such (9.2 kernel topology is left out, and half of
  `fcut.c` is compiled out with the comments admitting sliver and tie-breaking failures).
- libbrep's SSI tracer (closest-pair polyline linking at fixed depth 8 with magic thresholds)
  and its Boolean assembly (faces not stitched; floating inner shells). TeleSAPR's marching
  and trimming are ahead.
- Shape recognition (delegates surface typing to OpenNURBS; "HIGHLY EXPERIMENTAL").
- libpc, the `joint` solver, Bullet `simulate`.
- Any binding of BRL-CAD libraries: contrary to the no-cgo decision, and the useful parts
  are ideas, not APIs.

## Recommended order of work

1. **Projection** (pkg/srf/surface.go `ClosestPointTo`/`ClosestPointToUV`): port the
   seeded Gauss-Newton design, keep the old path behind the `mustConverge` flag until the
   fillet and STEP suites pass; drop `nearestOn`'s polish when they do. Smallest, widest.
2. **F5 spike** on the wedge-on-plate fixture: pre-Boolean fuse plus the shared/anti split
   in `keepEdge`/`keepRegion`; then the cap bore with isocurve overlap boundaries. Fixed
   budget of two days each, as the roadmap rule says.
3. **CSG ray oracle** in `pkg/check`: analytic leaves plus sketch-parity extrusions and
   revolutions, 3-view convergence, raydiff against the B-rep mesh. Gives every later
   kernel change an in-tree volume oracle and the sweeps a failure signal that does not
   depend on FreeCAD.
4. **CDT mesher spike** on the twisted bar and NIST_6: Go constrained Delaunay behind a
   flag, measured against the current mesher.
5. STEP: adopt the acceptance and accounting rules; try `repair_missing_singular_trims`'s
   approach on NIST_6 and Schenkel.
6. Pipe-style corner radius on path sweeps, as a feature, when 28.4 is next looked at.

Each item enters the roadmap as its own step under "Open work" with the two-day spike rule;
none of them changes the definition of done.

## Verification of this investigation

- The claims above about BRL-CAD cite file and line; spot-check by opening the cited
  functions (`intersect.cpp` ~3538/3645, `PullbackCurve.cpp` ~1091, `cdt/edge.cpp`,
  `bool.c:150`, `eval.c:143`, `fuse.c:1894`).
- The claims about TeleSAPR cite `pkg/srf/boolean_trim.go`, `pkg/srf/surfinter.go`,
  `pkg/srf/triangulate.go`, `pkg/srf/surface.go:384`, `pkg/topo/rollingball.go`, and the
  notes in `docs/notes/step-24-spike.md`, `step-28.md`.
- For each recommended item the acceptance test is the existing one: the step 24 sweeps
  (`TELESAPR_SWEEP=1`), `TestFilletRibFootSweep`, the cap-bore sweep (40 of 80 today), the
  STEP corpus count (7 of 9 sound), `testdata/invariants.txt` under `make ci`.
