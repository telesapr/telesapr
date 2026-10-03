# Bill of materials

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

## Not yet

Only the files the model links are counted, not the files those link in turn; a part linked
from two paths to one file counts as two parts. A copy made by a group whose source is not a
link group, such as a translate of the whole model, is not counted.
