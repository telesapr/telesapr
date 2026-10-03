# Section view

Example: [`examples/scripts/section`](../../examples/scripts/section/section.go), a block with
a stepped bore, cut through the bore's axis.

View > Section View (Ctrl+Shift+X) cuts the model by a plane parallel to the selected workplane
or flat face, and draws the cut face closed. Its screen in the text window sets how far from
the face the plane lies (an expression, in the document's unit) and which side is kept; the
same menu entry turns it off. With nothing fitting selected, it says what to select.

From a script:

```go
doc.SetSection(face, "20", false) // offset, flip
s := doc.Section()
doc.ClearSection()
```

`doc.Render` draws the view, cut, to a PNG file.

## Not yet

The section is part of the view, not of the model: it is not saved in the file, and undo does
not change it. The cut face has one colour; a hatch is roadmap 16.12.
