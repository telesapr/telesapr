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

## Measure

`TestFilletOnOtherSolids`: 47 of 59 at the start, and still 47 after `3fdc41eb`, because it fillets
each edge alone, so picks apart don't move it. All 12 refusals are the loft (`vase`, r 2):

| Edges | Refusal | Cause |
|---|---|---|
| 4 foot edges (bottom plane x side patch) | "the ball … doesn't touch the face on side 1" | At the corner, `nearestOnWithin`'s plain Gauss-Newton doesn't settle on the twisted side patch. The foot is 3.9 from the centre, not 2 |
| 4 top ring arcs | "meets the flat face almost along it" | `blendNext` stops where a crease fades to tangent at the ring |
| 4 creases | "the faces meet tangent here" | The crease fades to tangent at the top. Refusing it is right; it would need a vanishing fillet |

## In progress (not committed)

The foot edges: retrying a failed section with `srf.SSurface.Project` (robust) makes the foot fillet
sound (took 9.69 mm³), but it takes 47 s, from about 35,000 retried sections. Next, a cheaper fix:
damped steps in `nearestOnWithin` (halve a step that moves away), or seeding from the previous
section's foot. Goldens must be checked, since the feet's last bits may move.
