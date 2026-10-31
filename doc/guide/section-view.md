# Section view

Example: [`examples/scripts/section`](../../examples/scripts/section/section.go), a block with
a stepped bore, cut through the bore's axis.

The section view cuts the model open so that you can look inside. View > Section View
(Ctrl+Shift+X) cuts the model by a plane parallel to the selected workplane or flat face, and
draws the cut face closed. With nothing selected while you are sketching in a workplane, it cuts
in that workplane. Otherwise it says what to select.

Its screen in the text window sets two things: how far from the face or workplane the plane
lies (an expression, in the document's unit), and which side is kept. Choosing the same menu
entry again turns the section view off.

From a script:

```go
doc.View.SetSection(face, "20", false) // offset, flip
s := doc.View.Section()
doc.View.ClearSection()
```

`doc.File.Render` draws the view, cut, to a PNG file.

The cut face is filled and hatched in the colour of the SectionCap line style (Line Styles on
the home screen). The hatch runs at 45 degrees, with a line every 10 pixels.

## Not yet

The section is part of the view, not of the model. It is not saved in the file, and undo does
not change it.
