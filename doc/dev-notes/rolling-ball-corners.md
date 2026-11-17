# Rolling-ball corners: pivots and mitres at an inside corner

Refreshed 2026-11-17 at branch `wrapped-fine-tol`.

Where two rolling-ball chains end at one vertex with a crease between their walls that the ball
goes round (a pocket's rim in a curved face, the foot of a band raised on a tube at the band's
corners, `concaveAt`), `insideMitres` (`pkg/topo/rollingmitre_inside.go`) decides how they meet:

- their two feet on the crease one point within the chain's tolerance (`rb.tol`, a tenth of the
  chord tolerance in mm, at least `pivotSameFoot`): an inside mitre, the two swept surfaces cut
  where they cross;
- otherwise a corner patch swung round the crease (`pkg/topo/rollingpivot_corner.go`): sections
  from the crease to the shared face, each the arc through its two feet tangent to the face, the
  foot on the crease moving evenly from one fillet's to the other's.

So the same corner is a mitre at a coarse tolerance and a patch at a fine one. The band on a
tube is such a corner: one fillet touches the band's end (a plane square to the axis) at the
ball's height, 7.5, the other the band's side (a plane through the axis) at √(7.5² − 0.2²), 0.0027
lower: in `TestFilletWrappedStrip`'s model a patch from a chord tolerance of about 0.045% down
(`rb.tol` is 0.00117 mm at 0.02%).

## The corner patch's ends (2026-11-17)

The patch is skinned along the swing through its sections (`oneSpan`, a quintic; `manySpans`).
Across each end it takes the derivative of its own sections (`ownDerivatives`), not the
fillet's.

Until 2026-11-17 it took the fillet's (`inwardDerivative` scaled to the swing's path), its own
sections moved by Hermite terms to match, but for the crease's point, which runs along the
crease. The fillet runs on along its edge, every control point at about the ball's speed; the
corner turns about the crease, a point at the speed of its distance from it, the arc's middle
control point (on the crease below the vertex) not at all. A quintic held to the fillet's speeds
carried the points near the crease, whose whole swing at a radius of 0.2 is a few hundredths of
a millimetre, a fifth of the ball's path out along the fillet's edge and back:

- the patch folded, by about 0.002 mm (the tangent along the swing turned through 180° at three
  quarters of the way up the section; its normal vanished inside the patch), and a mesh finer
  than that crossed itself: the strip of `TestFilletWrappedStrip`, all four edges, was refused
  as open at chord tolerances of 0.015% and finer (0.02% meshed past the fold);
- its normals turned from the fillets' by up to 0.46° on the strip and 4.7° on the dish's pocket
  (`dishPocket`) near the crease.

With its own derivatives the normals lie within 1e-4° of the fillets' along both ends on both,
the strip is sound at 0.02%, 0.015%, 0.01% and 0.005%, and the dish's corners take 0.3% to 1.1%
less, nearer OpenCASCADE's (`TestFilletPivotCorner`: r 2, four edges, 32.34 → 31.99 mm³ against
its 31.71). Against OpenCASCADE's fillet of the strip built from exact cylinders, which rounds
the corner, our faces lie within 0.016 mm at the corners (0.011 before: the fold bulged towards
its round) and as close as before elsewhere.

## Unlike sections either side of a patch (2026-11-17)

A corner's sections are of the degree and spans of the higher of the two fillets'
(`sectionsLike`). A pointed arch's leg on a tube is fitted in quintics of two spans, its foot
across the tube in quadratics of one: the corner's end at the foot was in two pieces, the foot's
patch's side in one, and the two never shared their edge (`TestFilletWrappedArch` at 0.02% and
finer, open). `splitAtPivots` (`pkg/topo/rollingpivot.go`) splits the fillets' patches' sides at
the corners of the corner's patches on its ends, as `splitAtMitres` does at a mitre of two radii.
`splitAtPoints` takes a point only once and none whose nearest point on the curve wasn't found
(a NaN it used to split at).

## Left

- The corner patches of the strip are joined into the fillets' faces or kept apart (`Faces`)
  differently at different tolerances: 15 faces at 0.02%, 14 at 0.015%.
