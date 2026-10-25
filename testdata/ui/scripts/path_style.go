// A custom line style drawn at a level of its own, as
// path_style.script makes it on the styles screen
// (TestPathEquivalence/style).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	style := doc.NewStyle()
	// The level a style is drawn at: the highest goes on top, and an
	// exported file's entities are written in that order.
	style.SetZIndex(7)
}
