// The sweep of path_sweep_partway.script, made by a script
// (TestPathEquivalence/sweep_partway): a rectangle in XY drawn across a
// line 5 above its start, swept along the line, a bend and a line drawn
// in ZX, from testdata/ui/sweep_partway.tlspr.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_partway.tlspr"); err != nil {
		panic(err)
	}
	path := []Entity{doc.Entity(0x40000), doc.Entity(0x50000), doc.Entity(0x60000)}
	doc.Features.Sweep(path, "")
}
