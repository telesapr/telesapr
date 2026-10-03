// The sweep of path_sweep_cubic.script, made by a script
// (TestPathEquivalenceSweepCubic): the rectangle of
// testdata/ui/sweep_cubic.tlspr, the active sketch, swept along the
// cubic of its 3d sketch, twisted by 90 degrees, its frames held to x.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/ui/sweep_cubic.tlspr"); err != nil {
		panic(err)
	}
	var cubic Entity
	for _, e := range doc.Group("path").Entities() {
		if e.Kind() == "cubic" {
			cubic = e
		}
	}
	s := doc.Sweep([]Entity{cubic}, "")
	s.SetTwist(90)
	s.SetSweepUp(V(1, 0, 0))
}
