# WP-C3: fillets running onto curved faces

Branch `wp-c3-fillet-curved`, from devel 83b573fd. State as of 2026-10-22.

## Done

- `3fdc41eb` picks apart (`pkg/topo/fillet_apart.go`): a rolling-ball edge picked with others whose
  fillets don't reach it is no longer refused ("pick it alone"). The picks step 10 takes are rounded
  first, then each rolling one alone, found again by `edgeAlong`. A rolling chain sharing a vertex
  with another pick is refused at that vertex. Tests: `TestFilletEndsOnCurvedFaceApart` (lid, four
  uprights together, 8.308 mm³ against OpenCASCADE's 8.296 for each alone, +0.15%),
  `TestFilletChainsNotMitred` (lens top and bottom chains), `TestRollingBallRefusals` (the rim with
  the branch now gives the rim's own refusal).

- Loft foot edges: long Gauss-Newton steps that take the point further away are halved
  (`nearestOnWithin`), and a section whose feet don't settle is made again with
  `srf.SSurface.Project`. All four foot edges are sound. The fillet's surface lies within 4e-6 mm of
  OpenCASCADE's, whose own result is not valid there.

- Loft top ring: a chain goes on past a crease that fades out at the vertex (`blendNext`,
  `joinedAt`, `smoothAt`). A foot on a patch edge is allowed the sag of `seamCos`. OpenCASCADE takes
  241.787 mm³ and ours takes 241.807, both measured by OpenCASCADE. Test: `TestFilletLoftTopRing`.

## Measure

`TestFilletOnOtherSolids`: 47 of 59 at the start, **55 of 59** now. The 4 left are the loft's
creases, refused as "the faces meet tangent here": each fades to tangent at the top, so its fillet
would have to vanish along the edge. That is a different construction (a variable section shrinking
to nothing), and refusing it is honest. OpenCASCADE refuses them too ("command not done", checked
with FreeCADCmd).

## Not done (left for later)

- Fillet setbacks (`fillet_setback.go`) onto curved end faces: no measured case reaches them. The
  multi-pick cases that meet at a vertex are refused at that vertex ("pick it alone").
- Run-offs onto a curved face, and a run-off of a blend that takes material away: as in ROADMAP.
- Chamfers of edges that end on a curved face (`filletApart` is for fillets only).
