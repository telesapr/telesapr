# Two-sided groups

Example: [`examples/scripts/spacer`](../../examples/scripts/spacer/spacer.go), a ring extruded
both ways from its sketch.

On an extrusion's screen, "two-sided" runs it both ways from its sketch and keeps its whole
depth, the sketch in the middle: an extrusion of 8 runs from -4 to 4. "one-sided" puts it back.
A revolve turns as far each way, and a step and repeat lays its copies on both sides of the
original.

```go
spacer := doc.Features.Extrude(8, "spacer")
spacer.SetTwoSided(true)
```

Leaning sides (`SetSkewed`) keep whichever way the group runs.
