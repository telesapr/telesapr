// The model of path_configurations.script, made by a script
// (TestPathEquivalenceConfigurations): a parameter w driving the
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
	sides := doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	must(doc.SetParameter("w", "30"))
	c := doc.Length(sides[0], 36)
	doc.Regen()
	must(c.SetExpr("w"))
	doc.Regen()
	must(doc.AddConfiguration("small"))
	must(doc.AddConfiguration("large"))
	must(doc.SetParameter("w", "50"))
	doc.Regen()
	must(doc.ActivateConfiguration("small"))
	doc.Regen()
}
