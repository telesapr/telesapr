// The appearance of path_appearance.script, made by a script
// (TestPathEquivalenceAppearance): the extrusion of path_extrude made
// metal (roadmap 19.2).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "").SetAppearance("metal")
}
