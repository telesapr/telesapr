// The draft of path_draft.script, made by a script
// (TestPathEquivalence/draft): the extrusion of path_extrude, its sides
// leaning in by 5 degrees (roadmap 15.4).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "").AsExtrude().SetDraft(5)
}
