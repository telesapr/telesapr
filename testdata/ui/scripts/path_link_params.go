// The link of path_link_params.script, made by a script
// (TestPathEquivalence/link_params): a plate regenerated in its
// configuration "long" with L = 60, then relinked to a block.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	g, err := doc.Assembly.Link("../../testdata/linkparams/plate.tlspr", "")
	if err != nil {
		panic(err)
	}
	l := g.AsLink()
	l.SetConfiguration("long")
	l.SetParam("L", "60")
	l.SetFile("../../testdata/linkparams/block.tlspr")
}
