# WP-C4: closed sweeps, lofts tangent to a face

Branch `wp-c4-closed-sweep`, from devel 83b573fd. Item 8 of `modelling_upgrades.md`.

## Done: sweeps round closed paths

- `srf.SShell.MakeFromClosedSweepOf`: the exact sweep along lines and arcs, with no caps, the last
  piece's surfaces ending on the curves the first's start from.
- `topo.PathSweepOptions.Closed`: the fitted sweep round a closed path. The rotation-minimising frames
  come back turned by the loop's holonomy, and that turn is taken out evenly along the path. A twist
  must be whole turns. The closing section is the first one exactly (`joinSkins`).
- `pkg/sketch/sweep_closed.go`: the chain is cut where the profile's plane crosses it, nearest the
  profile, and started there, so the closing joint is smooth. Mitres and corner radii work on the
  rest; corners are rounded on the whole loop before the cut (`roundClosedCorners`). Lines and arcs
  out of a plane, where the profile doesn't come back as it left, go to the fitted sweep, or are
  refused if they have corners. A profile at a corner, or one whose plane misses the path, is a
  problem on the group.
- Tests: `TestClosedSweep` (srf), `TestPathSweepClosed*` (topo; a saddle path with real holonomy),
  `TestSweepClosed*` (sketch, with the grid `TestSweepClosedSweep`, 0 of 32 failed in full),
  `TestOracleSweep` ring and frame (check, ray oracle agrees), `TestPathEquivalenceSweepRing`,
  `ext_sweep_ring`, the example `frame`, and goldens `sweep-ring` and `sweep-frame`.

## Not done: a loft tangent to the solid's faces

Design, not built:
- An end condition beside the scalar one: the derivative across at each control point follows the
  side face of the solid that meets the profile's edge, `d = n_T - (n_T·n_S) n_S` (n_T is the profile
  plane's normal toward the loft, n_S the side face's normal), times the scalar times the loft's
  length.
- The side faces come from `topo.Build` of the solid before the loft: edges whose points pass
  through the curve's Greville points, and their faces that aren't coplanar with the profile. At a
  vertex, average the faces' directions.
- Refuse it when no such face is found, or when d points back against the loft.
- Save it as two flags on the group; it goes into `topo.smoothLoftBase` as direction functions
  (`LoftEnds` must stay comparable).
