// The sweep of path_sweep_twisted_arcs.script, made by a script
// (TestPathEquivalence/sweep_twisted_arcs): a rectangle in XY swept along
// two arcs in ZX that meet at a right angle, from
// testdata/ui/sweep_twisted_arcs.tlspr, twisted by 90 degrees.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_twisted_arcs.tlspr"); err != nil {
		panic(err)
	}
	s := doc.Features.Sweep([]Entity{doc.Entity(0x40000), doc.Entity(0x50000)}, "")
	s.AsSweep().SetTwist(90)
}
