// The mechanism of path_mechanism.script, made by a script
// (TestPathEquivalence/mechanism): a bar of 60 and one of 50 linked, the second
// moving in a mechanism and hinged to the first by a driven hinge;
// the second shorter, so that the first's far edge, hinged, lies under
// none of it to be picked in the window, and hinged by its own far
// edge, as its first lies on the first part's.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	ground, err := doc.Assembly.Link("../../testdata/ui/mechanism/bar60.tlspr", "")
	if err != nil {
		panic(err)
	}
	bar, err := doc.Assembly.Link("../../testdata/ui/mechanism/bar50.tlspr", "")
	if err != nil {
		panic(err)
	}
	bar.AsLink().SetMoves(true)
	doc.Assembly.HingeDriven(doc.Entity(0x80030032), doc.Entity(0x80040032))
	_ = ground
}
