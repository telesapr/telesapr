// The screw mate of path_mate_screw.script, made by a script
// (TestPathEquivalence/mate_screw): the top circles of the holes of
// screw_pair.tlspr's plates, the lead an M4's coarse pitch, as the
// window takes it for a hole of that diameter.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

// top is g's circle highest up, the top of the hole.
func top(g Group) Entity {
	var best Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && (best.H.V == 0 || e.Center().Pos().Z > best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/screw_pair.tlspr"); err != nil {
		panic(err)
	}
	doc.Assembly.Screw(top(doc.Group("a")), top(doc.Group("b")), 0.7)
}
