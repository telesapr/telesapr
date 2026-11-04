// The sweep of path_sweep_sharp_arc.script, made by a script
// (TestPathEquivalence/sweep_sharp_arc): a rectangle in XY swept along a
// line and an arc in ZX that meet at a sharp corner of 150 degrees, from
// testdata/ui/sweep_sharp_arc.tlspr.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_sharp_arc.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Sweep([]Entity{doc.Entity(0x40000), doc.Entity(0x50000)}, "")
}
