// The script of the ext_script_form scenario: a plate whose size its
// form asks for.
package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/script/api"
)

var Form = api.Form{
	Title: "Plate",
	Fields: []api.Field{
		{Name: "width", Label: "Width (mm)", Value: 40.0},
		{Name: "holes", Label: "Holes", Value: 2},
		{Name: "name", Value: "plate"},
		{Name: "solid", Label: "Extrude", Value: true},
		{Name: "finish", Value: "matte", Choices: []string{"matte", "metal"}},
	},
}

func main() {
	doc := api.Current()
	// Each run sketches anew, in the XY plane.
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	w := Form.Float("width")
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(w, 20, 0))
	for i := 0; i < Form.Int("holes"); i++ {
		doc.Sketch.Circle(api.V(w*float64(i+1)/float64(Form.Int("holes")+1), 10, 0), 3)
	}
	if !Form.Bool("solid") {
		fmt.Println("a sketch only")
		return
	}
	g := doc.Features.Extrude(4, Form.Text("name"))
	g.SetAppearance(Form.Text("finish"))
	fmt.Printf("%s %s, volume %.0f\n", g, g.Appearance(), g.Analyze().Volume())
}
