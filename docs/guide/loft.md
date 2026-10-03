# Loft

Example: [`examples/scripts/loft`](../../examples/scripts/loft/loft.go), a round-to-square
duct adapter.

A loft joins the closed loop of the active group's sketch, in a workplane, to that of another
sketch by ruled faces, curve by curve. Activate one sketch, select any entity of the other, and
choose New Group > Loft To Selected Profile (Shift+O). The two loops must have as many curves
each: a circle counts as four arcs, so it lofts to a square. The group's screen lists the
profiles and has "turn the second by [-1] [+1]", which matches the loops' curves with an offset:
a square turned a quarter has saddle faces.

From a script:

```go
l := doc.Loft(bottom, top, "duct")
l.SetTurn(1)
```

## Not yet

The faces are ruled, straight from one profile to the other. Smooth lofts, and lofts through
more than two profiles, are roadmap 20; loops of unequal counts are matched by splitting curves
in 20.1.
