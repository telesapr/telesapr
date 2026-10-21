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

## Measure

`TestFilletOnOtherSolids`: 47 of 59 at the start, **51 of 59** now. The 8 left are all the loft:

| Edges | Refusal | Cause |
|---|---|---|
| 4 top ring arcs | "meets the flat face almost along it" | `blendNext` stops where a crease fades to tangent at the ring, so the chain isn't closed round the ring |
| 4 creases | "the faces meet tangent here" | The crease fades to tangent at the top. Refusing it is right; it would need a vanishing fillet |
