# Step 28 working notes (branch step-28-solid-features)

Kept so a resumed session starts cheaply. Delete when step 28 merges.

## State

| Package | State | Commits |
|---|---|---|
| 28.1 chamfers of two distances | done | f526098, be7cd87 (goldens) |
| 28.5 pattern along a closed path | done | 6fee564, 30883da (goldens) |
| 28.2 drafts: chosen sides, neutral plane, sharp arc/line corners | done, see below | WIP 2c6fe81 + final |
| 28.4 sweeps along solid edges, paths with corners | done: mitres between lines; edges by name | WIP + final |
| 28.3 smooth lofts: end tangency, guide curves | end tangency done; guide curves not started | |

## 28.2 design

- topo.DraftedPrismAngles (pkg/topo/draftsides.go): an angle per profile curve. DraftedPrism
  now calls it. Arc sides are cones over the turns holding both ends of a conic seam (plus a
  0.1 rad margin), split in quarter parts; a conic-trimmed part is raised to degree 2 along its
  rulings and flagged srf.SSurface.AvoidFlatEars.
- Mesher trap: ears of three points on a planar trim lie in the trim's plane, over the face on
  the other side. A global "no sliver ears" rule broke TestFilletSweep and the C++ volume
  comparison, so it is behind the AvoidFlatEars flag (flatEar in pkg/srf/triangulate.go:
  slivers, or tilted > 30 degrees from the surface normal).
- Sketch: Group.DraftFaces (file draft-faces), ValB = neutral plane (file draft-neutral, a
  length). Two prisms from the neutral plane, unioned. New Group > Draft Selected Sides
  (CmdDraftSelectedSides) chooses sides, because a selection replaces the group screen.
- Two arcs at a sharp corner are refused (cone/cone is quartic).

## 28.4 design

- srf.SweepStep.Corner: a rotation in place between two straight pieces; MakeFromSweepOf cuts
  both pieces by the plane of normal d1+d2 (sections projected along the piece, an affine map),
  extends the side surfaces past it (mitreReach) and flags them AvoidFlatEars.
- sketch: pathChainAny(edges, extra pieces, ..., corners) allows corners between line pieces;
  sweepPath inserts Corner steps, refuses corners for fitted sweeps, checks mitres don't cross.
  Group.SolidEdges for sweeps; solidEdgePieces via topo.EdgeCurve; NewSweepAlong, Doc.SweepEdges.
- Not done: corners beside arcs (torus/plane trims), closed sweeps.

## 28.3 design

- topo.LoftEnds / MakeSmoothLoftEnds / SmoothLoftSurfaceEnds: srf.SkinWithEnds with the end
  derivatives the profile's normal times tangency times the loft's length; a free end takes the
  free skin's own derivative. SkinWithEnds now takes sections of differing weights (a square to
  a circle): the weight's derivative is 0 at the ends.
- Loft group ValB/ValC (start-tangency, end-tangency; not lengths: Group.IsLength excludes loft
  ValB, and the encoder/decoder now follow IsLength for ValB). Screen rows, Group.Tangency /
  SetTangency, example nozzle.

## Next

Guide curves (28.3's rest): a sketch curve from the first profile to the last; plan was to
sample the free loft's iso-sections, move each so its matched vertex lies on the guide, and skin
through them (translation only, exact at the samples). Not started.
