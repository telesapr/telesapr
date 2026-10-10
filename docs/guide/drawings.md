# Drawings

Example: [`examples/scripts/drawing`](../../examples/scripts/drawing/drawing.go), an L-shaped
bracket and its drawing on an A3 sheet.

A drawing is a sheet of paper with views of the solid of the active group: front, top, a side
and an isometric, placed as the projection has them, at a scale, in a frame with a title block.
It is part of the model and saved with it; what is drawn is made from the solid each time the
drawing is written, so it follows the model.

View > Drawing Sheet shows its screen:

- **sheet**: A4 to A0, Letter or Tabloid, landscape or portrait;
- **scale**: fitted (the largest of 10:1, 5:1, 2:1, 1:1, 1:2, 1:5 and on down at which the views
  fit above the title block), or one typed as `1:2`, `2:1` or a number;
- **projection**: third angle (the top view above the front, the right view on its right) or
  first angle, as ISO has it (the top view below, the left view on the right);
- **hidden edges**, drawn dashed in the orthographic views;
- the **title block**: title, drawn by, drawing number, revision, material and company; the scale,
  sheet and projection are filled in.

Under **sections and details**:

- `[add a section]` takes a view and an offset, `top 0`: the solid is cut by the plane square to
  that view's direction through its middle, moved the offset in mm toward the viewer; the half
  beyond is drawn, its cut face hatched every 3 mm on the sheet, labelled SECTION A-A, and the
  plane is drawn dashed and lettered on the views that see it edge on;
- `[add a detail]` takes a view, the point it enlarges (mm right and up from the view's middle),
  a radius and how many times larger, `front 10 5 8 2`: what of the view is within the circle,
  enlarged, in a circle, labelled DETAIL B with its scale, the circle drawn dashed and lettered
  on the view.

They are lettered A, B, ... in that order and placed in columns right of the views, unless a
script gives them a place (`X`, `Y`).

Under **dimensions**: with the screen shown, select one or two edges or faces of the solid in the
model (the screen stays, and counts what is selected), then `[dimension the selected edges or
faces]` and type the view and kind, `front length` or `front distance 15` with how far out in mm.
The view is front, back, top, bottom, left, right or iso, or a section or detail by its letter,
`A diameter`. A dimension holds the edges' names and the faces' identities, so each time the
drawing is made it finds them in the solid as it is and measures them:

- a **length** of a straight edge, along the view; on the isometric, the edge's own length, not
  as it looks foreshortened;
- a **distance** from a straight edge to another edge, square to the first, as the view shows
  it; or, with a face, between two parallel flat faces, or from a straight edge to a flat face it
  runs parallel to, drawn out to the side of what it measures;
- a **diameter** (DIA) or **radius** (R) of a circular edge, or of a face's largest circular edge,
  as a hole's wall or a cylinder's end, with a leader when it faces the view, or across it when it
  is seen edge on.

The screen lists each with what it measures now; one whose edge or face is gone, seen end on or at
a slant, outside its detail, or of faces that aren't parallel, says why and isn't drawn.

View > Show Drawing Sheet, or `[show it in the window]` on the drawing's screen, shows the sheet in
the window in place of the model, fitted to it, until it is chosen again (`[show the model]`). It
follows each change made on the drawing's screen and to the model; while it is shown the mouse
picks nothing in the model. It is a view, not saved with the file. From a script,
`doc.Drawing.Show(true)`, and `doc.File.Render(path, w, h, "window")` pictures it.

File > Export > Drawing writes the sheet as PDF, SVG, DXF, EPS, PostScript or HPGL, the canvas the
sheet's size in millimetres. `telesapr-cli export-drawing -o sheet.pdf model.tlspr` does the same
without a window.

From a script:

```go
d := doc.Drawing.Get()
d.Sheet, d.Scale, d.FirstAngle = "A2", 0.5, true
d.Title, d.Number = "BRACKET", "TS-0001"
doc.Drawing.Set(d)
must(doc.Drawing.Export("bracket.pdf"))   // in the application
```

`doc.Drawing.AddDimension("front", "length", 10, edge)` adds a dimension by solid edges,
`doc.Drawing.AddFaceDimension("top", "distance", 10, upper, lower)` one by faces, and
`doc.Drawing.AddEdgeFaceDistance("top", 10, edge, face)` the distance from an edge to a face;
`doc.Drawing.Dimensions()` measures them. The example
[`examples/scripts/drawing`](../../examples/scripts/drawing/drawing.go) dimensions faces and the
isometric. `d.Sections` and `d.Details` hold the section and detail
views; `d.Views` places views where you
want them, by kind and the sheet position of their middle in
millimetres from the bottom left; empty, the standard ones are placed.

## Not yet

Dimensions are not of vertices or angles, nor between curved faces; a distance from a face to a
curved edge is refused. A section's half beyond is drawn with hidden edges as the
drawing has them; a section is of the whole solid, not a broken-out or offset section. The text is the vector font's, which has no colon: the title
block writes a scale of 1:2 as 1/2.
