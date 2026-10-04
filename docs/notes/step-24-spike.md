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

## The rest of step 24 (branch `step-24b-kernel-robustness`)

Written 2026-10-04.

- **24.1, cylinder through cylinder: 2 → 0 of 54.** Not the strip
  triangle's height as such: the ear clipper took an ear whose new chord
  passed a hair (under 0.001 mm) from the crossed seam's pierce point,
  the ear's own vertex. In xyz the chord dips below the surface by its
  sag, so the sliver turned over. `SContour.grazed` (triangulate.go)
  ranks such ears after every clean one and before a folded one: a
  point, the ear's own included, closer to the new chord on the surface
  than twice the chord's sag, and off it in uv by under 2% of its length
  (`grazeShare`). The uv condition keeps points near a cone's apex out:
  far apart in uv, near in xyz only because the patch folds there
  (TestConeCutNearApex failed without it). Points at a pole are skipped.
- **24.2, the last analytic STEP cut: 1 → 0 of 28.** The same grazing,
  on a half sphere's patch beside the cylinder cutter's hole: a chord
  from the patch's seam to the hole's rim passed inside the hole.
- **24.3, band over a lathe seam: 8 → 0 of 108.** A seed for marching
  half a millimetre past the band's side: the lathed wall's bottom
  circle met the band's arc, a cylinder patch across y, beyond the
  patch; `AllPointsIntersecting`'s cylinder branch projected the hit
  onto the patch's side, off the line, and the trim took it in. The
  march from it took the curve's own seed and the wall's trims did not
  close. Hits whose projection lies more than `offPatchTol` (1e-3 mm)
  from the line's own are dropped now.
- **24.3, bore with its cap in a lathed tip's plane: still 40 of 80.**
  Coincident contact across the tip's flat patches (F5); not touched.
- **24.4, twisted ruled faces.** The bias was not in the grid's
  interior (a bilinear cell's two triangles have its volume exactly)
  but in its outer cells, which the ear clipper fans from the trim's
  few points (a straight edge has two). `SSurface.twisted` tells a
  twisted ruled face (normal turning along the straight lines) and
  `refineTwistedGrid` halves its outer cells: loft volume worst 1.23% →
  0.55%. Forcing four grid lines across rulings where the grid has none
  (a swept bar's sides, 1.6% over) brought the bar to 0.16%, but made
  three threads' flanks cross, so it is not done; the bar stays at
  1.6%. A refinement by each cell's sag at the diagonals, to the chord
  tolerance, ran for minutes: rejected.
- **NIST_MBE_PMI_6** (from step 27): still `inter=2` with `topo=1`
  allowed, as on master. One crossing is a fan edge on a flat disc at
  y = 31.75 to its centre, the other a sliver at a corner of a 2x2
  patch (ctrl[2][2] = (107.95, 34.925, 43.18)) whose two curved sides
  meet there, its v = 1 side almost straight near the corner. Neither
  is at a folded side of that patch, so `openPoles` is not the cause
  there. Not mended.

Traps: `TELESAPR_SWEEP=1` runs the full grids; `TELESAPR_REGEN_INVARIANTS=1`
regenerates `testdata/invariants.txt` (not in `regen-golden.sh`); a
probe at chord tolerance 0.0005 mm with sag-driven refinement takes
minutes per loft.
