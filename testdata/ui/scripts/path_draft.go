// The draft of path_draft.script, made by a script
// (TestPathEquivalenceDraft): the extrusion of path_extrude, its sides
// leaning in by 5 degrees (roadmap 15.4).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "").SetDraft(5)
}
