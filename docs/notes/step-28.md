# Step 28 working notes (branch step-28-solid-features)

Kept so a resumed session starts cheaply. Delete when step 28 merges.

## State

| Package | State | Commits |
|---|---|---|
| 28.1 chamfers of two distances | done | f526098, be7cd87 (goldens) |
| 28.5 pattern along a closed path | done | 6fee564, 30883da (goldens) |
| 28.2 drafts: chosen sides, neutral plane, sharp arc/line corners | done, see below | WIP 2c6fe81 + final |
| 28.4 sweeps along solid edges, paths with corners | not started | |
| 28.3 smooth lofts: end tangency, guide curves | not started | |

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

## Next

28.4: sketch/sweep.go pathChain (pathChainAny has a closed opt-in). Sweeps along a chain of the
solid's edges by name (Group.SolidEdges, topo.Name) and paths with corners (mitred joints).
28.3: topo/smoothloft.go SkinWithEnds takes end derivatives; guide curves need a skin through
sections constrained to curves.
