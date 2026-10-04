// The sweep of path_sweep_corner.script, made by a script
// (TestPathEquivalenceSweepCorner): a rectangle in XY swept along two
// lines in ZX that meet at a corner, from testdata/ui/sweep_corner.tlspr
// (roadmap 28.4).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/ui/sweep_corner.tlspr"); err != nil {
		panic(err)
	}
	doc.Sweep([]Entity{doc.Entity(0x40000), doc.Entity(0x50000)}, "")
}
