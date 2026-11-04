// The model of path_shell_ring_on_block.script, made by a script
// (TestPathEquivalence/shell_ring_on_block): ring_on_block.tlspr's block
// and ring hollowed to walls of the thickness the menu starts with, open
// at the block's top. The ring is revolved three quarters of a turn, its
// end on the block's side moved out into the block.
package main

import (
	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/ring_on_block.tlspr"); err != nil {
		panic(err)
	}
	block := doc.Group("extrude")
	top := block.Solid().FaceAt(V(12, 10, 10))
	doc.Features.ShellSolid([]SolidFace{top}, 1, "")
}
