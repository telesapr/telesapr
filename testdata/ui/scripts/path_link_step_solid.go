// The link of path_link_step_solid.script, made by a script
// (TestPathEquivalenceLinkStepSolid): a STEP file linked as a solid.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if _, err := doc.LinkStep("../../testdata/step/shapes/box_hole.step", "", true); err != nil {
		panic(err)
	}
}
