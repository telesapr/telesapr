// The sweep of path_sweep_skew.script, made by a script
// (TestPathEquivalence/sweep_skew): a square in XY swept round four lines
// in 3d that close out of a plane, from testdata/ui/sweep_skew.tlspr.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_skew.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Sweep([]Entity{doc.Entity(0x40000), doc.Entity(0x50000), doc.Entity(0x60000), doc.Entity(0x70000)}, "")
}
