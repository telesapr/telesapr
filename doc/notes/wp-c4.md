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

## Done: a loft along a solid's sides

- `topo.SideTangents`: for a point of a profile drawn on a face, the direction the side face meeting
  the edge there carries on in. That is the profile's normal made square to the side's normal, or at
  a corner the line two sides meet in, so that a straight edge between two corners stays in its
  side's plane. Faces are counted once, and a side must lie below the face.
- `topo.LoftSides` and `MakeSmoothLoftSides`: the end derivatives at each control point's Greville
  point. `LoftEnds` stays comparable.
- `Group.LoftAlongSides [2]bool`, saved as `start-along-sides`/`end-along-sides`, kept by undo.
  Entry point `Sketch.SetLoftAlongSides`; window `[along sides]`/`[square]` on the loft's screen
  (`ScreenLoftAlongSides`); script `LoftGroup.SetAlongSides`/`AlongSides`. A tangency of 0 means 1.
- Tests: `TestSideTangents`, `TestLoftAlongSides*` (slope just above a drafted top against
  -tan(draft); the grid gave 0 of 16 failures), `TestPathEquivalenceLoftSides`, `ext_loft_sides`,
  the example `spire`, and the golden `loft-sides`.

## Not done

- Closed lofts (the last profile back to the first).
- A guided loft along the sides (refused).
- Tangency at a profile between the ends.
