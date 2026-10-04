// The chamfer of path_chamfer_two.script, made by a script
// (TestPathEquivalenceChamferTwo): path_chamfer's chamfer with a second
// distance of 2, its distances then swapped.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
	c := doc.Chamfer([]Entity{doc.Entity(0x80030004), doc.Entity(0x8003001f)}, 1, "")
	c.SetSecondDistance(2)
	c.SwapDistances()
}
