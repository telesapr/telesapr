# Shell

A shell hollows a solid out, leaving walls of one thickness. It is open wherever you take a face
away.

Examples:

- [`examples/scripts/shell`](../../examples/scripts/shell/shell.go): an enclosure, a block
  hollowed to 2 mm walls and open at the top.
- [`examples/scripts/pot`](../../examples/scripts/pot/pot.go): a pot with rounded corners and a
  drain hole.
- [`examples/scripts/bowl`](../../examples/scripts/bowl/bowl.go): a turned bowl whose wall is a
  zone of a sphere.
- [`examples/scripts/housing`](../../examples/scripts/housing/housing.go): a turned housing
  with a square boss out of its side, hollowed as one.

## In the window

Select one or more faces of the solid to take away, then choose Features > Shell Solid
(Shift+G). A right click on a face offers the same as Shell Solid Open Here.

The group's screen lists the faces taken away and takes the thickness, as a number or an
expression. "[change]" beside the faces takes the current selection as the faces to take away
instead (with nothing suitable selected it activates the group before the shell, says what to
pick, and reads "[take the selection]" until you do). "[remove]" on a face's row keeps that face
as a wall again. A thickness at which the walls would meet is a problem on the group, and the
solid passes through unchanged.

From a script:

```go
box := doc.Features.Extrude(25, "box")
top := box.Solid().FaceAt(api.V(30, 20, 25))
sh := doc.Features.ShellSolid([]api.SolidFace{top}, 2, "")
sh.AsShell().SetThickness(1.5)
```

## What it hollows

**A solid of flat faces** can be open at any of them.

**A prism** whose sides are flat or round, such as an extruded circle, a rounded rectangle, or a
block with a round hole, can be open at its ends. Its arcs keep their centres and lose or gain
the thickness on their radius. A round corner smaller than the thickness becomes a sharp corner
inside.

**A lathe or revolve** of lines and arcs, with no other solid before or after it, is hollowed by
its profile. So are several lathes about one axis joined to each other (a cup with a flange round
its foot, a groove turned out of a cylinder), drawn in any planes through the axis. The lathes'
profiles are joined, then the profile is offset into the material: each line is moved square to
itself, each arc gets a smaller or larger radius about its centre, and the result is turned
again. So a cone gets a coned inside and a sphere a spherical one, each wall exactly the
thickness. Such a solid can be open at any of its faces: a flat one, such as a cup's top, or a
curved one between outside corners, such as a ring open all round its outer side, which leaves a
channel. A revolve's flat ends are walls too, unless they are taken away. Where the ends of a
revolve of more than half a turn meet at the axis, the inside keeps the thickness from each end
and is rounded about the axis.

**A drafted extrusion whose sides are cones**, such as a drafted cylinder, ring or slot, with no
other solid before or after it, is hollowed by its profile too. Each side, plane or cone, is
moved in square to itself. The inside is the same draft of the profile, offset across by the
thickness divided by the cosine of the draft angle, between the ends moved in. It can be open
at its ends, or at a round side between outside corners, past which the inside runs out. A
two-sided draft's inside leans both ways from its neutral plane, as the solid does. (A drafted
extrusion of lines only has flat faces, and is hollowed like any solid of flat faces.)

**A union of such parts.** A solid joined by unions from lathes, revolves and extrusions,
drafted or not, each of which a shell could hollow alone, is hollowed as the union of the parts'
insides. Examples: a turned body with an extruded boss out of its side, a cup with a lug revolved
part of a turn round it, a block with a round boss. Each part's walls are moved in. But a face of
a part that lies inside the others, such as a boss's foot sunk into the body, is moved out by the
thickness instead. So the boss's inside runs into the body's with no wall between them, and the
walls round the joint keep the thickness. The ends of a revolve of half a turn or less standing
inside another part, such as a half ring on a block's side, move out the same way. A prism's
sides move each as it lies: a round post half sunk into a block's side is walled all round, its
inside reaching into the block's. Lathes about one axis are joined by their profiles first, as
above.

**Parts taken away or intersected.** Such parts can also be taken away from the solid before
them, or intersected with it: a round pocket or a groove lathed into a block, a cone or a ball
cut flat by a block. A part taken away takes away from the inside that part grown by the
thickness, every face moved out, so a wall of the thickness stands round the pocket and over its
floor. A revolve of less than a whole turn taken away grows across its ends' planes too. A part
intersected with cuts the inside by that part moved in. The parts are taken in the order they
were made. A face taken away that lies on a part taken away (a pocket's floor, extruded or
turned) or intersected with (the flat cut down a cone's side, a lathe's top) opens the inside
through it.

## Not yet

- A solid with curved faces must be one of these: a prism whose sides all run one way, open
  only at its ends; one lathe or revolve; lathes about one axis; or a solid joined from such
  parts. A solid with a helix, or with a feature after those parts, is refused.
- In a union, the end of a revolve of more than half a turn lying inside another part is
  refused. So is a revolve of half a turn where one end is taken away, a wall or inside another
  part and the other end is not.
- A union opened at a top face that two parts share can fail.
- A face moved out that would come out of the solid is refused. So is one that would leave a
  wall beyond it thinner than the thickness, where a part joined to another is thinner there than
  twice the thickness.
- A revolve's curved face can be opened only between outside corners.
- A profile of curves other than lines and arcs is refused.
- Beside a round hole, walls thinner than about twice the chord tolerance (how far the mesh may
  stray from the true surface, set on the configuration screen) can come out unsound: the hole's
  chords and the wall's cross.
- Where four faces meet at a vertex and, moved in, don't meet at one point, as at an off-centre
  pyramid's apex, the inside gets a short new edge there. Five or more such faces are refused.
- Filleted and lofted solids can't be shelled yet. That waits for offsets of other kinds of
  surface.
