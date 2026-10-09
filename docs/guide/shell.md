# Shell

Example: [`examples/scripts/shell`](../../examples/scripts/shell/shell.go), an enclosure: a
block hollowed to 2 mm walls, open at the top; [`examples/scripts/pot`](../../examples/scripts/pot/pot.go),
a pot with rounded corners and a drain hole.

Features > Shell Solid (Shift+G) hollows the solid so far to walls of one thickness, open
where faces are taken away. Select one or more flat faces of the solid to take away. A solid
of flat faces can be open at any of them; a prism whose sides are flat or round, as an extruded
circle, rounded rectangle or a block with a round hole, can be open at its ends: its arcs keep
their centres and lose or gain the thickness on their radius, and a round corner smaller than the
thickness becomes a sharp one inside. The
group's screen lists them and takes the thickness, as a number or an expression. A thickness
at which the walls meet is a problem on the group, and the solid passes through.

From a script:

```go
box := doc.Extrude(25, "box")
top := box.Solid().FaceAt(api.V(30, 20, 25))
sh := doc.ShellSolid([]api.SolidFace{top}, 2, "")
sh.SetThickness(1.5)
```

## Not yet

A solid with curved faces must be such a prism: its sides all run one way, and it is open only at
its ends; a cone, a sphere or a revolved solid is refused. Beside a round hole, walls thinner than
about twice the chord tolerance can come out unsound: the hole's chords and the wall's cross. Where four faces meet at a
vertex and, moved in, don't meet at one point, as at an off-centre pyramid's apex, the inside gets
a short new edge there; five or more such faces are refused. Filleted and lofted solids wait for offsets of other
surfaces (roadmap 22.1).
