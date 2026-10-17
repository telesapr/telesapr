// Text, edited after it is drawn: its string, its font and its kerning
// are read and changed as the text window does, with the corner held
// where it is. Then a dimension is shown as a radius and its label is
// moved, and a tangent is made at the end of an arc that shares none
// with the line.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/text/text.go
//
// In the application, on the sketch that is open: Script > Run File,
// or
//
//	telesapr --script examples/scripts/text/text.go
//
// It writes nothing.
package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()

	text := doc.Sketch.Text("Label", "", api.V(0, 0, 0), 8)
	doc.Constrain.Lock(text.Points()[1])
	fonts := doc.Sketch.Fonts()
	fmt.Printf("%d fonts to set it in; it is %q in %s\n", len(fonts), text.Text(), text.Font())

	text.SetText("Label 2")
	text.SetKerning(true)
	text.SetFont(fonts[0])
	fmt.Printf("now %q in %s, kerning %v\n", text.Text(), text.Font(), text.Kerning())

	// A diameter shown as a radius, its label moved clear of the circle.
	circle := doc.Sketch.Circle(api.V(40, 10, 0), 6)
	dia := doc.Constrain.Diameter(circle, 12)
	dia.SetShowAsRadius(true)
	dia.SetLabelOffset(api.V(10, 8, 0))
	fmt.Printf("the circle's dimension reads %g, at %v\n", dia.Value(), dia.LabelOffset())

	// A line and an arc that do not meet: tangent where the arc ends.
	line := doc.Sketch.Line(api.V(0, -20, 0), api.V(10, -20, 0))
	arc := doc.Sketch.Arc(api.V(20, -20, 0), api.V(30, -20, 0), api.V(20, -10, 0))
	doc.Constrain.Tangent(line, arc, arc.End())
	doc.Constrain.CommentOn(arc.End(), "tangent here")
}
