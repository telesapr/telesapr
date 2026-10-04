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

MaxFailRate 0.10 over: tee and spout, union (concave) and difference
(convex), branch/main radius ratio, angle of the branch, offset of its axis,
fillet radius as a share of the room.

## Not done yet / next

- Open chains ending on a flat face (cut the fillet by the plane).
- Chamfers between curved faces (refused with a message).
- Tests, sweep, path through sketch/api/UI, example script, docs.
