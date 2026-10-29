# Shell

Example: [`examples/scripts/shell`](../../examples/scripts/shell/shell.go), an enclosure: a
block hollowed to 2 mm walls, open at the top; [`examples/scripts/pot`](../../examples/scripts/pot/pot.go),
a pot with rounded corners and a drain hole; [`examples/scripts/bowl`](../../examples/scripts/bowl/bowl.go),
a turned bowl whose wall is a zone of a sphere.

Features > Shell Solid (Shift+G) hollows the solid so far to walls of one thickness, open
where faces are taken away. Select one or more flat faces of the solid to take away. A solid
of flat faces can be open at any of them; a prism whose sides are flat or round, as an extruded
circle, rounded rectangle or a block with a round hole, can be open at its ends: its arcs keep
their centres and lose or gain the thickness on their radius, and a round corner smaller than the
thickness becomes a sharp one inside.

A solid made by one lathe or revolve of lines and arcs, with no other solid before or after it,
or by lathes about one axis joined to each other (a cup and a flange round its foot, a groove
turned out of a cylinder), drawn in any planes through the axis, is hollowed by its profile, the
lathes' profiles joined: the profile is offset into its material, each line moved square to
itself and each arc given a smaller or larger radius about its centre, and turned again. So a
cone gets a coned inside, a sphere a spherical one, each wall exactly the thickness. Such a solid
can be open at any of its faces: a flat one, as a cup's top, or a curved one between outside
corners, as a ring open all round its outer side, which leaves a channel. A revolve's flat ends
are walls too, unless taken away; where the ends of a revolve of more than half a turn meet at
the axis, the inside keeps the thickness from each and is rounded about the axis.

An extrusion with a draft whose sides are cones, as a drafted cylinder, ring or slot, with no
other solid before or after it, is hollowed by its profile too: each side, plane or cone, is moved
in square to itself, so the inside is the same draft of the profile offset across by the thickness
over the cosine of the draft angle, between the ends moved in. It can be open at its ends; a
two-sided draft's inside leans both ways from its neutral plane, as the solid does. (A drafted
extrusion of lines only has flat faces, and is hollowed as any solid of flat faces is.)

The group's screen lists the faces taken away and takes the thickness, as a number or an expression;
"[change]" beside the faces takes the selected faces instead, and "[remove]" on a face's row
keeps it as a wall again. A thickness
at which the walls meet is a problem on the group, and the solid passes through.

From a script:

```go
box := doc.Features.Extrude(25, "box")
top := box.Solid().FaceAt(api.V(30, 20, 25))
sh := doc.Features.ShellSolid([]api.SolidFace{top}, 2, "")
sh.AsShell().SetThickness(1.5)
```

## Not yet

A solid with curved faces must be such a prism, its sides all running one way and open only at
its ends, or one lathe or revolve, or lathes about one axis; a lathe joined to other solids, as
an extruded block or a revolve of less than a whole turn, and a helix are refused, and so is a
drafted extrusion joined to another solid or open at a side. A revolve's curved face is opened only between outside corners. A
profile of other curves than lines and arcs is refused. Beside a round hole, walls thinner than
about twice the chord tolerance can come out unsound: the hole's chords and the wall's cross. Where four faces meet at a
vertex and, moved in, don't meet at one point, as at an off-centre pyramid's apex, the inside gets
a short new edge there; five or more such faces are refused. Filleted and lofted solids wait for offsets of other
surfaces (roadmap 22.1).
