// The sweep of path_sweep_path.script, made by a script
// (TestPathEquivalence/sweep_path): a circle swept along a line, a bend
// and a line from testdata/ui/sweep_path.tlspr, the last line then
// taken from the path (ROADMAP F7).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_path.tlspr"); err != nil {
		panic(err)
	}
	path := []Entity{doc.Entity(0x40000), doc.Entity(0x50000), doc.Entity(0x60000)}
	sw := doc.Features.Sweep(path, "")
	sw.AsSweep().SetPath(path[:2], []SolidEdge{})
}
