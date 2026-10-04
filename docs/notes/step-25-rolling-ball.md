# Step 25 (22.2 general): rolling-ball fillets, running notes

Branch `step-25-rolling-ball`. Code: `pkg/topo/rollingball.go`, entry from
`FilletPicks` (`pkg/topo/fillet_network.go`) when step 10's construction
refuses a curved edge; `srf.SkinThrough` (`pkg/srf/skin.go`), a skin whose
sections' weights differ, with end derivatives in homogeneous coordinates.

## Design as built

- The chain: `blendChain` walks across vertices where only smooth edges
  (geometric seams, `smoothEdge`) meet; it must close for now.
- Sides: each piece's two faces, paired with the previous piece's across
  smooth edges (`joinedAt`).
- The edge densely (`trace`): the pieces' points subdivided to r/4 and put
  on both faces (`onEdge`, tangent planes and the plane square to the edge).
- The ball (`section`): centre in the plane square to the edge, iterated
  from the tangent planes at the faces' nearest points (`foot`, with
  `nearestOn`'s Gauss-Newton polish: the kernel's closest point leaves a
  point 2 mm off a cylinder 6 µm short).
- Crossings of seams by the ball's points (`crossings`): scan, then bisect.
  Sections are put at the crossings; near-equal crossings on the two sides
  share one.
- The surface: `SkinThrough` through the arcs, closed (first section =
  last, same derivative), sections added at midpoints until within
  chordTol/10 of the faces and the ball (`misfit`).
- The edit: each crossed seam split where the fillet's corner is
  (`splitNearest`, by the curve through the edge's vertex nearest it: two
  patches meet along two seams either side of a hole, and a curve may run
  along both); on every face at the edge, the edge's trims and the seam
  pieces from the edge to the cut go, the fillet's edge curves come in, in
  the face's own direction (`forward`); then the fillet's patches go in
  with a FaceAdder, one origin and sheet for all.

## Measured (spike, 2026-10-04)

Tee of cylinders R 10, branch 5 (union, concave), volumes against
OpenCASCADE's makeFillet, by slices integrated in FreeCAD (OCC's own Volume
of our STEP is off by up to 5% of the fillet; slices aren't):

| r | OCC added | ours − OCC |
|---|---|---|
| 1 | 3.9617 | +0.011 |
| 2 | 15.769 | −0.061 |
| 3 | 35.370 | −0.192 |

Mesh volumes at chord 0.002, 2000 segments: r 1 4.035, 2 15.944, 3 35.36.

Teapot spout (frustum 30→20 high 40, cylinder r 4 at 48°, union): sound
for r 1, 2, 3 (added 7.27, 30.46, 67.92 by mesh).

## Robustness sweep: rate fixed before building the general path

MaxFailRate 0.10 fixed before; `TestRollingBallSweep` (tee/hole of two
cylinders: branch ratio 0.3/0.5/0.7, angle 90/70/55, axis offset 0/0.2,
radius 0.1/0.25/0.4 of the room) fails 3 of 108 with TELESAPR_SWEEP=1: a
branch of 0.7 at 55 degrees with the largest radius (two refusals, one
self-crossing mesh triangle). Allowance now 3.5/108, 10% for samples.

The radius must fit: beside a hole the wall between the hole's side and
the pipe's is the room (the ball needs centre y = 7 + r inside radius
10 - r); the sweep's radii are shares of that, as the refusals said.

## OpenCASCADE is not the truth everywhere

On the hole of radius 5 in a pipe of 10, r 2, OpenCASCADE's makeFillet
takes 97.78 mm³, ours 93.63 (both by slices in FreeCAD). Points of its
fillet face lie up to 0.093 mm inside the balls' envelope (mean 0.018);
ours within 0.00024 (`TestRollingBallOnEnvelope` checks ours). The test
uses our sliced volume there. OCC's Volume of our STEP files is off by up
to 5% of the fillet; integrate slices instead.

## Teapot handle

A torus (ring) joined to the cone is unsound before any fillet (naked
edges from the Boolean), at 0.005 and 0.02 chord tolerance: a kernel
limit for step 24's follow-ups, not the fillet's. Not tested.

## Next: open chains ending on a flat face (designed, not built)

- At each end vertex: the chain edge, and two edges a (F with side 0)
  and b (F with side 1) of a planar third face F; anything else keeps
  step 10's reason (errNotRolling).
- Let `foot` extrapolate patches (prefer patches whose own (u, v) is
  inside [0,1]², else the least outside), only for open chains: the
  ball runs on past the end, outside the solid, on the faces' surfaces
  continued (rational arcs stay on their circle).
- Extend the sections past each end until a section lies wholly beyond
  F's plane; fit open (SkinThrough with each end's own derivative);
  keep only crossings whose points lie inside both end planes.
- Clip the blend patches by the plane: drop those wholly beyond; for one
  crossing it, its loop is its boundary pieces inside plus the cut curve
  (plane ∩ blend, sampled by u, Hermite cubics whose control points lie
  in the plane exactly); section arcs shared by two clipped patches split
  once at the same parameter.
- Side face k at the end: its contact curve split where it meets the
  plane, at q_k. If q_k lies on a or b, split that edge there and drop
  the piece (q_k, v) from both faces (convex case, and a concave one whose
  face ends at F). Otherwise (the face goes on past F's plane, as the
  cone above a band's top) add the curve S_k ∩ plane from q_k to v to
  both S_k and F.
- F: drop the pieces (q_k, v), add the cut curve, in F's loop's
  direction (from the trim along a).
- First fixture: the band on the lathed cone in
  `pkg/script/api/filletoffaxis_test.go`, whose fillet then succeeds (the
  test's fillet half must change; its chamfer half keeps its message).
