// The loft of path_loft_sides.script, made by a script
// (TestPathEquivalenceLoftSides): from the top of the drafted post in
// loft_sides.tlspr to the tip, leaving along the post's sides.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_sides.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Loft(doc.Group("top"), doc.Group("tip"), "").AsLoft().SetAlongSides(true, false)
}
