# Sweep

Example: [`examples/scripts/sweep`](../../examples/scripts/sweep/sweep.go), a bent tube: a
ring swept along a straight, a bend and a straight.

New Group > Sweep Along Path (Shift+U) moves the active group's sketch, in its workplane, along
a path. Select the line segments and arcs of the path, joined end to end and meeting tangent.
The profile's workplane must be active (Sketch > In Workplane). The group's screen lists the
path. The path is an ordinary sketch, so its dimensions drive the sweep.

From a script:

```go
path := []api.Entity{
	doc.Line(api.V(0, 0, 0), api.V(0, 0, 30)),
	bend,
	doc.Line(end, end.Plus(api.V(40, 0, 0))),
}
doc.SketchIn(doc.Workplane("XY"), "ring")
doc.Circle(api.V(0, 0, 0), 6)
doc.Sweep(path, "tube")
```

## Not yet

The path is lines and arcs, tangent where they meet; splines and corners are roadmap 20
(E10). Repeating a solid along a path is [Pattern Along Path](pattern.md).
