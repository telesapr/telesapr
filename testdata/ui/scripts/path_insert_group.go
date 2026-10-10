// The model of path_insert_group.script, made by a script
// (TestPathEquivalenceInsertGroup): a group made with an earlier group
// active goes into the history after it (roadmap 16.4).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	sketch := doc.Active()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	doc.Activate(sketch)
	doc.Features.Extrude(80, "")
}
