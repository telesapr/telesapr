# Assemblies

An assembly is a model of parts: files linked into it (New Group > Link / Assemble), or groups
that combine as an assembly. This page is about putting them together, checking them and
counting them.

## Mates

Example: [`examples/scripts/mates`](../../examples/scripts/mates/mates.go), a plate stacked on a
copy of itself, flush and with the holes on one axis.

A mate is a shortcut to the constraints that put two parts together. Select the first of the
part that stays, then the second of the part the active group places (a link group places the
part it links), and choose from the Constrain menu:

- **Mate Concentric**, two circles or arcs: their axes parallel, and the second's centre on the
  first's axis; the part may still turn about the axis and slide along it. In a workplane, the
  centres on each other.
- **Mate Flush**, two faces: parallel, with a point of the first lying on the second; the part may
  still slide and turn on the face.
- **Mate Hinge**, two line segments: the starts on each other and the second's end on the first's
  line; the part may only turn about it.

Each makes ordinary constraints, one step of undo, which are listed, edited and deleted as any;
one that what is already there makes redundant is left out.

```go
doc.Flush(top, bottom)
doc.Concentric(holeA, holeB)
```

## Bill of materials

Example: [`examples/scripts/bom`](../../examples/scripts/bom/bom.go), which saves a plate and a
bolt as part files, links them into an assembly with the bolt repeated, and writes the bill.

Analyze > Bill of Materials lists the parts the model places by linking files, one line a file:
how many, the part's name (the file's, without its folder and extension), the file, and the
groups that place them. Each link group places one part. A step and repeat, a mirror or a
pattern along a path of a link group places its copies too, but for a copy at the place of the
part it copies; a mirror that leaves the original out places none more; a repeat of a repeat
counts the copies of what it repeats. Groups left out of the model place none. The screen's
`[export to a CSV file...]` writes the list with the columns `part`, `count`, `file` and
`groups`.

From a script:

```go
for _, it := range doc.BillOfMaterials() {
	fmt.Println(it.Count, it.Part, it.File)
}
must(doc.ExportBOM("bom.csv"))
```

## Interfering parts

Analyze > Show Interfering Parts checks the parts of an assembly against each other by their
shells: each group that combines as an assembly is a part, and so is the solid made before the
first of them. Two parts interfere where they share volume; parts that only touch don't. The
message lists each pair with the volume they share, and the shared solid is outlined. A part
linked from a mesh or a STEP file has no shell; with one in the model, the check falls back to
the triangles of the whole model, as SolveSpace's does.

```go
found, meshOnly := doc.Interferences()
for _, f := range found {
	fmt.Println(f.A, f.B, f.Volume)
}
```

## STEP export of an assembly

File > Export 3d Surfaces... writes a model with parts as an assembly: the model is the top
product, and each part a product of its own, written once in its own coordinates, with its faces
in their colours; each place a part goes is an occurrence of it, with the rotation and offset
that put it there. A file linked twice is one product used twice. The parts are those the
interference check takes. A model with no assembled parts is written as one solid, as before.

## Not yet

Only the files the model links are counted, not the files those link in turn; a part linked
from two paths to one file counts as two parts. A copy made by a group whose source is not a
link group, such as a translate of the whole model, is not counted.
