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

## Not yet

Only the files the model links are counted, not the files those link in turn; a part linked
from two paths to one file counts as two parts. A copy made by a group whose source is not a
link group, such as a translate of the whole model, is not counted.
