// The model of path_configurations.script, made by a script
// (TestPathEquivalence/configurations): a parameter w driving the
// bottom side of the mouse's rectangle, configurations "small" (w 30)
// and "large" (w 50), and "small" chosen (roadmap 16.2).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	doc := Current()
	sides := doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	must(doc.Params.Set("w", "30"))
	c := doc.Constrain.Length(sides[0], 36)
	doc.Edit.Regen()
	must(c.SetExpr("w"))
	doc.Edit.Regen()
	must(doc.Params.AddConfiguration("small"))
	must(doc.Params.AddConfiguration("large"))
	must(doc.Params.Set("w", "50"))
	doc.Edit.Regen()
	must(doc.Params.ActivateConfiguration("small"))
	doc.Edit.Regen()
}
