// The screw mate of path_mate_screw_threads.script, made by a script
// (TestPathEquivalenceMateScrewThreads): the top rims of
// thread_pair.tlspr's M8 bolt, 6g, and of its nut's hole, 6H, with the
// threads' own lead, which the window takes when the circles carry
// threads. The three files were saved without their solids, which the
// mate doesn't use, to keep them small.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

// top is g's circle of radius 4 highest up.
func top(g Group) Entity {
	var best Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && e.Radius() > 3.99 && e.Radius() < 4.01 &&
			(best.H.V == 0 || e.Center().Pos().Z > best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/thread_pair.tlspr"); err != nil {
		panic(err)
	}
	doc.Assembly.Screw(top(doc.Group("bolt")), top(doc.Group("nut")), 0)
}
