// The sweep of path_sweep_twisted_run.script, made by a script
// (TestPathEquivalence/sweep_twisted_run): a rectangle in XY swept along
// a line and an arc in ZX that meet at a right angle, from
// testdata/ui/sweep_arc_corner.tlspr, twisted by 90 degrees.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_arc_corner.tlspr"); err != nil {
		panic(err)
	}
	s := doc.Features.Sweep([]Entity{doc.Entity(0x40000), doc.Entity(0x50000)}, "")
	s.AsSweep().SetTwist(90)
}
