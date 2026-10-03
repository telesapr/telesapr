// The model of path_suppress_all.script, made by a script
// (TestPathEquivalenceSuppressAll).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	sketch := doc.Active()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
	sketch.SetSuppress(SuppressWhole)
}
