# Step 28 working notes (branch step-28b-solid-features, the rest of step 28)

Kept so a resumed session starts cheaply. Delete when step 28 merges.

## State

| Package | State | Commits |
|---|---|---|
| 28.1 chamfers of two distances | done | f526098, be7cd87 (goldens) |
| 28.5 pattern along a closed path | done | 6fee564, 30883da (goldens) |
| 28.2 drafts: chosen sides, neutral plane, sharp arc/line corners | done, see below | WIP 2c6fe81 + final |
| 28.4 sweeps along solid edges, paths with corners | done: mitres between lines; edges by name | WIP + final |
| 28.3 smooth lofts: end tangency, guide curves | done; guides 03a3003, 18d69d8 (goldens) | |

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

## 28.3 guides (step 28b)

- topo/guidedloft.go: GuidedLoftSurface samples base.surface.IsoV at 12 per span, finds each
  guide's point where it crosses the section's plane (Newell normal), guideMove (1 guide shift,
  2 similarity about the plane normal), SkinThrough with end derivatives = base's plus the
  guides' shift rate (not where an end is set square). Guides must pass every profile (1e-3).
- sketch: Group.Edges are the loft's guides (save, undo, prune, deps came free); loftguide.go
  chains Beziers into guides; SetLoftGuides refuses entities drawn after the loft.
- ui: CmdLoftGuides "Guide Loft by Selected Curves"; loft screen row "guides".
- Trap: in api.SketchHost a whole script is one undo step, so Undo removes the loft too.

## 28.2 two arcs (step 28b)

- topo/draftsides.go arcArcSeam: piecewise quintics (Chebyshev-Lobatto nodes) through the
  circle-circle crossing at each height, split 1,2,4.. until within seamTol = 1e-6 of both
  cones; seams are chains ([]SBezier) now. ebe6610, golden c444c8e.

## 28.4 corners beside arcs: tried, not done

Tried (reverted): cut the path into runs at corners beside arcs, sweep each run carried
straight on past the corner, cut each by a half-space box at the mitre plane, union the runs.
The cuts work (run volumes exact), the union's volume is right, but the joined shell fails any
later Boolean (even with an empty shell): the two mitre faces are coplanar but not the same
outline, since one run meets the plane in its straight extension and the other in its arc
(plane-torus curve), so the union leaves a partial coplanar face with curved trims, which the
kernel can't take; with a round profile 188 naked edges. Coplanar unions with equal outlines
(boxes) are fine. A real fix needs the mitre inside MakeFromSweepOf for arc pieces: trims of the
arc's revolved side surfaces by the mitre plane (plane-torus curves, e.g. fitted as arcArcSeam
does), i.e. srf work. Fitted sweeps with corners: not started (needs the end frame of a fitted
run and twist split by length).

## 28.4 corners beside arcs: second attempt (kernel leftovers), not done

Why a plain mitre can't work: on the outside of the turn both pieces, carried straight on past
the corner, meet the halving plane M in the same outline (reflection across M maps one section
onto the other and one direction onto the other), but on the inside M cuts into the arc's
revolved sides, whose section there is not the straight piece's. So the only clean joint with
planar cuts is "bent caps": each run cut on the outside by M and on the inside by its own
section at the corner (the two cuts meet along the turn's axis a through the corner), then the
runs joined by a union, which overlaps on the inside and touches on the outside.

Tried (srf.makeSweepInRuns, reverted): each run swept with a straight extension past the
corner and cut by a bent prism (polygon (0,Y),(0,0),(Y tan(θ/2),-Y),(X,-Y),(X,Y) in (d, inside)
coordinates, extruded along a). The cut runs are sound with the right volumes (round profile,
line + arc of radius 8 at 60°), but the union fails in trimming, always on the faces that
touch: the two M faces (coplanar, opposite, equal outlines up to sampling) and a run's inside
cap, whose straight edge lies on the other run's bent edge along a. It fails the same way
forced onto a corner between two lines, round or square profile, while a plain union of two
flush boxes in an L works. So this is coincident contact along an edge plus a face (F5), not
the arc. What would work: building both runs and the joint in one shell with shared exact
curves (no union), which needs the inside sides' mutual intersection (line prism side against
arc revolve side), i.e. surface-surface intersection inside the sweep builder. Not started.

A test of the bent-cap joint's volume by integrating the exact union on a grid, and a sweep
over profiles, angles, bend direction and radius, is kept outside the tree for whoever picks
this up.
