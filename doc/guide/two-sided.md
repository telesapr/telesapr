# Two-sided groups

Example: [`examples/scripts/spacer`](../../examples/scripts/spacer/spacer.go), a ring extruded
both ways from its sketch.

An extrusion's screen has "one-sided" and "two-sided". Two-sided runs the extrusion both ways
from its sketch, with the sketch in the middle, and keeps its whole depth: an extrusion of 8
runs from -4 to 4. One-sided puts it back.

The same choice is on a revolve, a helix, and a step and repeat. A revolve or a helix turns as
far each way. A step and repeat lays its copies on both sides of the original.

```go
spacer := doc.Features.Extrude(8, "spacer")
spacer.SetTwoSided(true)
```

Leaning sides (`SetSkewed`) stay whichever way the group runs.

## Not yet

Nothing is missing from this feature.
