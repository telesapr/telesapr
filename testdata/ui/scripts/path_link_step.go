// The link of path_link_step.script, made by a script
// (TestPathEquivalenceLinkStep): a STEP file linked as a reference
// mesh.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if _, err := doc.Assembly.Link("../../testdata/step/shapes/cylinder.step", ""); err != nil {
		panic(err)
	}
}
