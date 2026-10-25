// The sweep of path_sweep.script, made by a script
// (TestPathEquivalence/sweep): a circle in XY swept along a line, a
// bend and a line drawn in ZX, from testdata/ui/sweep_path.tlspr.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_path.tlspr"); err != nil {
		panic(err)
	}
	path := []Entity{doc.Entity(0x40000), doc.Entity(0x50000), doc.Entity(0x60000)}
	doc.Features.Sweep(path, "")
}
