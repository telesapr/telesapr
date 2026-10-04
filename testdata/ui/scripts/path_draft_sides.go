// The draft of path_draft_sides.script, made by a script
// (TestPathEquivalenceDraftSides): path_draft's extrusion with the
// draft on the two sides the mouse picks, leaning from its far end
// (roadmap 28.2).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	ex := doc.Extrude(80, "")
	ex.SetDraft(5)
	ex.SetDraftFaces([]Entity{doc.Entity(0x80030004), doc.Entity(0x8003001f)})
	ex.SetDraftNeutral(10)
}
