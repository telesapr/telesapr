// The drawing of path_drawing_views.script, made by a script
// (TestPathEquivalence/drawing_views): the extrusion of path_extrude, its
// drawing with the front, top, right and back views placed, the
// isometric one left out, and a section of the top view placed.
package main

import (
	. "github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	d := doc.Drawing.Get()
	d.Views = []DrawingView{
		{Kind: "front", X: 50, Y: 100},
		{Kind: "top", X: 50, Y: 180},
		{Kind: "right", X: 130, Y: 100},
		{Kind: "back", X: 210, Y: 150},
	}
	d.Sections = append(d.Sections, sketch.DrawingSection{View: "top", X: 300, Y: 100})
	doc.Drawing.Set(d)
}
