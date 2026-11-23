// The ball mate of path_mate_ball.script, made by a script
// (TestPathEquivalence/mate_ball): the centres of the top circles of
// the holes of screw_pair.tlspr's plates put on each other.
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
	doc.Sketch.Anywhere3D()
	doc.Assembly.Ball(top(doc.Group("a")), top(doc.Group("b")))
}
