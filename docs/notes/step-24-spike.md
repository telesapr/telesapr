# Step 24: Booleans on curved surfaces, findings

Written 2026-10-04 on branch `step-24-kernel-robustness`.

## Do the failures share a cause?

Partly. Two causes covered most of them, and both are in meshing and
trimming near seams and poles, not in the intersector:

1. **Edges between two points of one planar trim (24.1).** A curved face
   cut by a plane has its trim in that plane. Where the uv grid keeps clear
   of the trim (a small cap, a band near a pole), ear clipping joined two
   trim points that are not neighbours. That edge lies in the plane's face,
   inside its outline, so the mesh crossed itself even though the Boolean
   was right. Fix: `splitTrimDiagonals` (triangulate.go) splits such edges
   at their middle in uv. It applies only when both points are on curves
   along the same face beyond; a rule based on "same curve" missed cuts
   that were split at seams.
2. **Poles (24.2).** On a patch that folds one side to a point (a cone's
   apex, a sphere's pole), the closest-point search put every trim through
   the pole at one corner. Seams then crossed the uv square on a diagonal,
   and the outline cut off part of the face. Fix: `openPoles` (shell.go)
   gives each trim end at a pole the other end's parameter along the
   folded side, and joins the trims there with an edge along that side.
   Triangles of no area are dropped. Folded patches have no grid along
   their straight lines, so their trim-only edges are also split where they
   stray from the surface by more than the chord tolerance. Applied to all
   patches, that rule made lofts and sweeps cross themselves.
3. **Test bug.** The analytic STEP set's cylinder cutter had its caps at
   z = 0, which is coincident contact, not a cut through. Three of its
   "failures" came from that.

## Numbers (before → after)

- Analytic STEP Booleans (`solidBooleanMaxFail`): 10 → 1 of 28. The one
  left is a sphere's pole inside the cylinder cutter.
- Sphere through face: 1 → 0 of 135.
- STEP corpus read as sound solids: 5 → 6 of 9 (screw.step).
- Cylinder through cylinder: still 2 of 54. This is a full-height strip
  triangle beside the crossed seam, a near-degenerate ear in uv. Not mended.
- Band over a lathe seam (new sweep, `latheseam_test.go`): 14 → 8 of 108.
  The remaining failures are low on the pot. There, the trims of the inner
  wall's patch beside the seam come out mixed with its neighbour's. Not
  traced further.
- Bore with its cap in a lathed tip's plane (new sweep): 40 of 80. All the
  failures are off-axis bores, which is coincident contact across the
  tip's four flat patches (F5). Not mended.

## Not done

- 24.4: twisted ruled faces, columns as well as rows.
- The 24.3 kernel failures above. Only the reporting part of 24.3 was
  done: `Group.Problem()` and Describe now say when a Boolean failed.

The debug patch used for Boolean trimming (per-chain classification
prints) is not kept. Re-create it by adding prints after `keepEdge` in
`makeCopyTrimAgainst`.
