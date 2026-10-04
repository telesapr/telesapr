# Pattern along a path

Examples: [`examples/scripts/pattern`](../../examples/scripts/pattern/pattern.go), pegs along a
rail of a straight run, a bend and another run, and
[`examples/scripts/flange`](../../examples/scripts/flange/flange.go), bolt holes round a pitch
circle.

New Group > Pattern Along Path repeats the active group's solid along a path: line segments and
arcs joined end to end and meeting tangent, as a sweep's path. Make the group whose solid is to
be repeated active, select the pieces of the path, and choose the menu entry. There are three
copies to begin with: the first where the solid is, the last at the far end of the path, and the
rest spaced evenly by length along it, each turned with the path as a sweep turns its profile.
The path starts at its end nearest the solid. The group's screen lists the path and takes the
number of copies, as a number or an expression. Copies that overlap are joined.

The path is an ordinary sketch, often construction lines, so its dimensions drive the pattern.

## Round a closed path

A path that closes, a circle or lines and arcs that come back to where they began meeting
tangent there, as a rounded rectangle's sides do, has its copies shared out evenly all round it:
the first where the solid is, the one after the last back at the first. The path starts at its
point nearest the solid, so the solid should sit on it, as a bolt hole on its pitch circle. A
closed path with a corner is refused, as an open one is.

From a script:

```go
pegs := doc.PatternAlongPath([]api.Entity{run, bend, tail}, 7, "pegs")
pegs.SetCopies(5)
fmt.Println(pegs.Copies(), pegs.Edges())
```

## Not yet

The solid is repeated, not the sketch's entities, so the copies can't be constrained or
dimensioned one by one. A group forced to a triangle mesh can't be patterned.
