// The sweep of path_sweep.script, made by a script
// (TestPathEquivalenceSweep): a circle in XY swept along a line, a
// bend and a line drawn in ZX, from testdata/ui/sweep_path.spcs.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/ui/sweep_path.spcs"); err != nil {
		panic(err)
	}
	path := []Entity{doc.Entity(0x40000), doc.Entity(0x50000), doc.Entity(0x60000)}
	doc.Sweep(path, "")
}
