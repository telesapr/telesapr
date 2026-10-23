# Assemblies

An assembly is a model of parts: files linked into it (Assembly > Link / Assemble), or groups
that combine as an assembly. This page is about linking them, putting them together, checking
them and counting them.

## Linked parts

Example: [`examples/scripts/partsizes`](../../examples/scripts/partsizes/partsizes.go), one
plate file linked three times in three sizes, and a fourth link pointed at another file.

A link group's screen names the file it links, with `[relink]` beside it: choose another file,
and the link reads it in place of the old one, keeping where the part is placed, its scale and
what the link passes into it. Use it for a newer revision of a part saved under another name, or
a STEP file in place of a model. What later groups use of the old part that the new one lacks is
removed with them, as when a linked file is missing.

A link to a model may pass it a configuration and values for its parameters (see
[configurations](parameters-and-units.md#configurations)). The model is then regenerated with
them, not read as it was saved, so one part file placed twice can be two sizes. The link's
screen lists:

- **configuration**, `as saved` or one of the model's configurations, for a model that has
  them;
- each of the model's **parameters** with its value: `in the model [pass a value]` while the
  model's own stands, or the value passed with `[change]` and `[model's]`, which gives the model
  its own again.

A value passed is a number or an expression in this model's parameters, as `W*2`: when they
change, the linked model is regenerated again, so an assembly drives its parts from its own
table. Each change is one step of undo.

From a script:

```go
long, err := doc.Assembly.Link("parts/plate.tlspr", "long")
long.AsLink().SetConfiguration("long") // "" reads it as saved
driven, err := doc.Assembly.Link("parts/plate.tlspr", "driven")
driven.AsLink().SetParam("L", "W*2")   // "" lets the model's own stand
swapped, err := doc.Assembly.Link("parts/plate.tlspr", "swapped")
swapped.AsLink().SetFile("parts/block.tlspr") // relink
fmt.Println(driven.AsLink().ModelParams(), driven.AsLink().ModelConfigurations())
```

`File`, `Configuration` and `Param` read back what the link has.

## Mates

Example: [`examples/scripts/mates`](../../examples/scripts/mates/mates.go), a plate stacked on a
copy of itself, flush and with the holes on one axis.

A mate is a shortcut to the constraints that put two parts together. Select the first of the
part that stays, then the second of the part the active group places (a link group places the
part it links), and choose from the Assembly menu:

- **Mate Concentric**, two circles or arcs: their axes parallel, and the second's centre on the
  first's axis; the part may still turn about the axis and slide along it. In a workplane, the
  centres on each other.
- **Mate Flush**, two faces: parallel, with a point of the first lying on the second; the part may
  still slide and turn on the face.
- **Mate Hinge**, two line segments: the starts on each other and the second's end on the first's
  line; the part may only turn about it.
- **Mate Screw**, two circles or arcs: on one axis, as Mate Concentric, with the second part's
  turn about it tied to its travel along it, as a nut's on a screw. Two new parameters drive it
  (`turn2` and `lead2` for a second screw, and so on): `turn`, in degrees from where the part
  was, starting at 0, and `lead`, how far a whole turn moves it along the first circle's normal,
  negative for a left-hand screw. Without threads on the circles the window takes the
  lead as the coarse metric pitch of the first circle's diameter, or 1 where there is none; edit
  `lead` in the parameter table (Edit > Parameters). Assembly > Motion Study on `turn` screws the
  part in ([motion study](parameters-and-units.md#motion-study)). The mate is made in 3d, and the
  second part needs a point off the axis. Of two circles that carry screw threads, a bolt's and
  its nut's, the lead is the threads' and the part is first moved to where the threads fit
  ([fitting a nut to its bolt](threads.md#fitting-a-nut-to-its-bolt)); a lead of 0 from a script
  is the threads'. Example:
  [`examples/scripts/screw`](../../examples/scripts/screw/screw.go), a nut run two turns up
  another's thread.

Each makes ordinary constraints, one step of undo, which are listed, edited and deleted as any;
one that what is already there makes redundant is left out.

```go
doc.Assembly.Flush(top, bottom)
doc.Assembly.Concentric(holeA, holeB)
mate := doc.Assembly.Screw(holeA, holeB, 1.25) // lead in mm
frames, err := doc.Assembly.MotionStudy(mate.Turn, 0, 720, 8, "")
```

## Bill of materials

Example: [`examples/scripts/bom`](../../examples/scripts/bom/bom.go), which saves a plate and a
bolt as part files, links them into a bracket with the bolt repeated, links the bracket twice
into the model, and writes the bill.

Assembly > Bill of Materials lists the parts the model places by linking files, one line a file:
how many, the part's name (the file's, without its folder and extension), the file, and the
groups that place them. Each link group places one part. A step and repeat, a mirror or a
pattern along a path of a link group places its copies too, but for a copy at the place of the
part it copies; a mirror that leaves the original out places none more; a repeat of a repeat
counts the copies of what it repeats. Groups left out of the model place none. A linked model
is a part, a sub-assembly, and the parts it links in turn count too, once for each place the
model is, at any depth: their groups read as `g003-bracket > g004-bolt`, the group here and the
group in the linked file. A file that links itself, by way of others, is followed once. Where a
link group placing the part has a [density](mass.md) set, the screen gives one part's mass
under it ("12.4 g each"). The screen's `[export to a CSV file...]` writes the list with the
columns `part`, `count`, `file`, `groups` and `mass_g`, the last empty where no density is set.

From a script:

```go
for _, it := range doc.Assembly.BillOfMaterials() {
	fmt.Println(it.Count, it.Part, it.File, it.Mass)
}
must(doc.Assembly.ExportBOM("bom.csv"))
```

## Interfering parts

Assembly > Show Interfering Parts checks the parts of an assembly against each other by their
shells: each group that combines as an assembly is a part, and so is the solid made before the
first of them. Two parts interfere where they share volume; parts that only touch don't. The
message lists each pair with the volume they share, and the shared solid is outlined. A group
that cuts the assembly after a part cuts the part too, so a hole cut after a bracket is a hole in
the bracket and the screw through it doesn't interfere. A part that is itself an assembly, as a
linked model of parts, is checked solid by solid. Where the Boolean of two parts fails, as for a
shank lying on its nut's hole or a screw's flanks on its nut's, their meshes are intersected
instead, and the volume is marked "about". What the meshes share counts as touching when it is on
average no thicker than the chord tolerance, as where faces lie on each other; a screw and a nut
of clearance 0 put half a pitch out of their fit collide by tens of mm³ and are listed. A volume summed over
several solids of one part is marked "about" too, as it counts twice what they share.
A pair that can't be checked either way is listed as failed. A part
linked from a mesh or a STEP file has no shell; with one in the model, the check falls back to
the triangles of the whole model, as SolveSpace's does.

```go
found, meshOnly := doc.Assembly.Interferences()
for _, f := range found {
	fmt.Println(f.A, f.B, f.Volume, f.Approximate, f.Failed)
}
```

## STEP export of an assembly

File > Export > Surfaces... writes a model with parts as an assembly: the model is the top
product, and each part a product of its own, written once in its own coordinates, with its faces
in their colours; each place a part goes is an occurrence of it, with the rotation and offset
that put it there. A file linked twice is one product used twice. The parts are those the
interference check takes. A model with no assembled parts is written as one solid, as before.

## Not yet

A part linked from two paths to one file counts as two parts. A copy made by a group whose source is not a
link group, such as a translate of the whole model, is not counted. One file linked in two sizes,
by configuration or by values passed, is one line of the bill, with the mass of the first link
that has a density. A part placed only inside a linked sub-assembly has no mass in the bill.
