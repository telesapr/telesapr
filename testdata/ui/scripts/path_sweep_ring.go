// The sweep of path_sweep_ring.script, made by a script
// (TestPathEquivalenceSweepRing): a square in XY swept round a circle in
// ZX, from testdata/ui/sweep_ring.tlspr.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_ring.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Sweep([]Entity{doc.Entity(0x40000)}, "")
}
