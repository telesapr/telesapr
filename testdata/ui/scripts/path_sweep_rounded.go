// The sweep of path_sweep_rounded.script, made by a script
// (TestPathEquivalence/sweep_rounded): path_sweep_corner's sweep with its
// corner rounded at 3 (F4).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/sweep_corner.tlspr"); err != nil {
		panic(err)
	}
	s := doc.Features.Sweep([]Entity{doc.Entity(0x40000), doc.Entity(0x50000)}, "")
	doc.Edit.Regen()
	s.AsSweep().SetCornerRadius(3)
}
