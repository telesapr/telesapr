# Steps 25 (rest) and 26: general fillets, running notes

Branch `step-26-blends`, from master 6d85eac.

## 25 rest: open chains, chamfers between curved faces (done)

Code: `pkg/topo/rollingend.go`, hooks in `pkg/topo/rollingball.go`.

- The chain may be open: `blendChain` then `blendChainBack`; each end must
  be a vertex of three edges whose third face is flat (`rbEndAt`), else
  `errNotRolling` (step 10's reason) or "ends at a curved face".
- The ball runs past the ends: `along` goes on along the end tangent,
  `foot` lets patches extend by 1 in (u, v) (`nearestOnWithin`), taking a
  patch that holds the point first.
- End section: the conic in the flat face's plane through the two feet
  (roots of "foot in plane" along s, `planeRoot`) and the arc's middle,
  tangents plane × face normal; exact (an ellipse) where the edge runs
  straight. Transition: planes through f0(s0(λ)), f1(s1(λ)), mid(sm(λ)),
  each s moved linearly from its λ=0 root to `target` (first regular
  section), λ in [0, 1); keeps feet in order (planes turned about one
  point folded the contact curve back). Chamfer: plane through f0, f1
  turned from the face's normal to the edge's tangent; its sections there
  are straight (rulings between the oblique feet).
- Refinement: misfit at three points along each pair, across too near the
  ends (canal-surface distance via `nearestSection`, at three times the
  tolerance: the oblique conics are not exact on a curved edge).
- Edit: at each end, per side, `alongFrom`: the foot on the side's edge
  with the flat face → split (`splitNearest` now splits every duplicate
  curve along the edge: a Boolean leaves one per face) and drop the piece
  to the vertex; on past the vertex (the face goes on past the plane) →
  `planeCut`, cubics in the plane, shared by the side's face and the flat
  face, the vertex kept; past the edge's far end → refused (22.3).
- Chamfer: `chamferSection`, points at the distance from the edge in the
  plane square to it, on each face.

Tests: `TestRollingBallOpenEdge` (box and leaning prism edges through the
rolling ball: chamfers exact to 1e-6, fillets within the mesh's sag),
`TestRollingBallChamferTee`, `TestFilletFlatOnConeOffAxis` (api), example
`bandfillet`. Band on cone at chord 0.001%-0.005%, r 0.5-2: all sound but
the fillet r 1 at 0.001% (2 self-intersecting mesh edges where the fillet
meets the cone tangentially) and the chamfer r 2 (runs off the band's top:
refused, 22.3).

## 22.5 fillets on lofts, sweeps and imported solids (measured, first fixes)

`TestFilletOnOtherSolids` (pkg/script/api) fillets each edge (up to 16)
of a smooth loft (vase), a swept square bar, and STEP solids read as
solids, alone, in a document of its own; a fillet made must be sound,
and each solid's count may only rise.

| solid | before | after |
|---|---|---|
| loft (square foot to circles) | 0 of 12 | 0 of 12 |
| swept bar | 12 of 12 | 12 of 12 |
| box_hole.step | 12 of 14 | 13 of 14 |
| cylinder.step | 0 of 2 | 2 of 2 |
| blob_extruded.step | 0 of 3 | 1 of 3 |
| sphere_bored.step | 0 of 2 | 2 of 2 |
| vase_revolved.step | 0 of 2 | 2 of 2 |
| lid_degree7.step | 8 of 12 | 8 of 12 |
| all | 32 of 59 (54%) | 40 of 59 (68%) |

Fixes: straight edges with a curved face at them go to the rolling ball
(`generalBlendEdge`); seams that end part way along a chain edge count
(a file's whole circle is one edge); `foot` picks, among faces sharing a
patch (a file's cells trim one patch into several faces), the one whose
trims hold the point (`onFaceUV`, `sameSurface`). `TestFilletStepCylinder`:
Pappus within 1% at chord 0.005% (at 0.002% the fit stalls at 2.7e-5
against 2.4e-5: a floor not found).

What is left there: the loft's foot edges and the blob end where they
meet a curved face (an open chain's end must be flat: 22.4's corners);
the loft's seams between its patches are tangent (no fillet there, but
they are listed as edges); the lid's corners where the edge's end face is
curved (step 10's corners, 22.4); one box_hole edge's seam.

## 22.3 blends that run off a face

Refusal with the place is in: an open fillet's end whose foot lies past
the far end of its side's edge with the end face ("runs off the face on
side k here, past the end of its edge", `alongFrom` = 1;
`TestChamferRunsOffBand`), and a closed or open ball that stops touching a
face mid-edge ("the ball the fillet rolls doesn't touch the face"). The
blend going on onto the next face is not built: the ball then rolls
between three faces for a while, which is 22.4's corner patch in all
but name.

## Tried and taken back

- Open chains ending at a curved face: the end section as the conic
  through the roots on the face's surface, accepted only within half the
  chord tolerance of it. No measured case gained (the loft's corners: the
  ball runs off the extrapolated B-spline faces; the band's top would
  stand out of a curved end face, which planeCut can't follow off a
  plane), so it was not kept.
- More refinement rounds for the STEP cylinder at chord 0.002%: the misfit
  floors at 2.6e-5 whatever the rounds; not found why.

## Next

- 22.3: the blend going on onto the next face (three faces at once).
- 22.4: vertex blends (setback corner patches); also the loft's foot
  corners and the lid's corners of 22.5's measurement.
- 22.5: OpenCASCADE volumes on the measured solids; box_hole.step's one
  edge whose ball skips a face cut from a shared patch between two scan
  samples' faces (faces 7, 9, 8 round the hole).
- The band on cone, fillet r 1 at chord 0.001%: 2 mesh edges cross where
  the fillet meets the cone tangentially (fine elsewhere).
