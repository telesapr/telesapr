# Drawings

Example: [`examples/scripts/drawing`](../../examples/scripts/drawing/drawing.go), an L-shaped
bracket and its drawing on an A3 sheet.

A drawing is a sheet of paper with views of the solid of the active group: front, top, a side
and an isometric, placed as the projection has them, at a scale, in a frame with a title block.
It is part of the model and saved with it; what is drawn is made from the solid each time the
drawing is written, so it follows the model.

File > Drawing Sheet shows its screen:

- **sheet**: A4 to A0, Letter or Tabloid, landscape or portrait;
- **scale**: fitted (the largest of 10:1, 5:1, 2:1, 1:1, 1:2, 1:5 and on down at which the views
  fit above the title block), or one typed as `1:2`, `2:1` or a number;
- **projection**: third angle (the top view above the front, the right view on its right) or
  first angle, as ISO has it (the top view below, the left view on the right);
- **hidden edges**, drawn dashed in the orthographic views;
- the **title block**: title, drawn by, drawing number, revision, material and company; the scale,
  sheet and projection are filled in.

File > Export Drawing writes the sheet as PDF, SVG, DXF, EPS, PostScript or HPGL, the canvas the
sheet's size in millimetres. `telesapr-cli export-drawing -o sheet.pdf model.tlspr` does the same
without a window.

From a script:

```go
d := doc.Drawing()
d.Sheet, d.Scale, d.FirstAngle = "A2", 0.5, true
d.Title, d.Number = "BRACKET", "TS-0001"
doc.SetDrawing(d)
must(doc.ExportDrawing("bracket.pdf"))   // in the application
```

`d.Views` places views where you want them, by kind and the sheet position of their middle in
millimetres from the bottom left; empty, the standard ones are placed.

## Not yet

Section and detail views (roadmap 18.3), and dimensions on the views (18.4). The drawing is not
shown in the window, only written. The text is the vector font's, which has no colon: the title
block writes a scale of 1:2 as 1/2.
