// The drawing's line styles, made from a script: a style of your own,
// its colour, width, dashes and level, and which of what is drawn goes
// in it.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/styles/styles.go
//
// In the application, on the sketch that is open: Script > Run File,
// or
//
//	telesapr --script examples/scripts/styles/styles.go
//
// A plate is drawn with a comment on it, and a style is made for the
// comment: drawn above the geometry, in another colour, dashed, and not
// exported with the drawing. As in the window, only comments and the
// lines, arcs and circles of requests can be given a style, not a
// dimension or a point. It writes nothing.
package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(80, 40, 0))
	doc.Constrain.Horizontal(sides[0])
	doc.Constrain.Vertical(sides[1])
	doc.Constrain.Lock(sides[0].Start())
	doc.Constrain.Length(sides[0], 80)
	doc.Constrain.Length(sides[1], 40)
	doc.Features.Extrude(6, "plate")
	note := doc.Constrain.Comment(api.V(0, -10, 0), "plate, 80 x 40")

	// A style of your own, on the styles list.
	dimension := doc.NewStyle()
	dimension.SetName("notes")
	// Drawn at level 7, above the geometry it notes: the highest level
	// goes on top.
	dimension.SetZIndex(7)
	dimension.SetColor(200, 30, 30)
	// The unit before the numbers in it: switching it converts them.
	dimension.SetWidthAs(api.UnitsMM)
	dimension.SetWidth(0.6)
	dimension.SetStipple(api.StippleDash)
	dimension.SetStippleScale(3)
	// A style may be exported, and a comment in it left out of the
	// exported drawing all the same.
	dimension.SetExportable(false)

	// The comment goes in the new style: what the window does with it
	// selected. A dimension can't be given one, as in the window, and a
	// script that asks for it is stopped with an error.
	note.SetStyle(dimension)

	red, green, blue := dimension.Color()
	fmt.Printf("the style %q: colour %d,%d,%d, width %g, stipple %d, level %d, exported %t\n",
		dimension, red, green, blue, dimension.Width(), dimension.Stipple(),
		dimension.ZIndex(), dimension.Exportable())
	fmt.Printf("the drawing has %d styles, and %d comments are in this one\n",
		len(doc.Styles()), len(dimension.Constraints()))

	// The styles of the program's own can be read and changed too: a
	// solid edge is drawn at a level of its own, above the faces.
	edges := doc.Style("#def-solid-edge")
	er, eg, eb := edges.Color()
	fmt.Printf("a solid edge is drawn at level %d in %d,%d,%d\n", edges.ZIndex(), er, eg, eb)

	// The text of a dimension in a style: its height in millimetres
	// (the number is converted, so the text keeps its size on the
	// screen), and which side of its anchor it sits on.
	dimension.SetTextHeightAs(api.UnitsMM)
	dimension.SetTextAlign("left", "top")
	h, v := dimension.TextAlign()
	fmt.Printf("the notes' text is %s, %s of its anchor\n", h, v)

	// Load Factory Defaults: the program's own styles go back to what
	// they were made as; the ones of your own are kept.
	edges.SetColor(1, 2, 3)
	doc.ResetStyles()
	er, eg, eb = edges.Color()
	fmt.Printf("after the reset a solid edge is %d,%d,%d again\n", er, eg, eb)
}
