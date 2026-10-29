// The sweep of path_sweep_arc_corner.script, made by a script
// (TestPathEquivalence/sweep_arc_corner): a rectangle in XY swept along a
// line and an arc in ZX that meet at a right angle, from
// testdata/ui/sweep_arc_corner.tlspr.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_arc_corner.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Sweep([]Entity{doc.Entity(0x40000), doc.Entity(0x50000)}, "")
}
