// The loft of path_loft_smooth.script, made by a script
// (TestPathEquivalenceSmoothLoft): the top circle of
// testdata/ui/loft_sections.tlspr, the active sketch, lofted through
// the middle square to the bottom circle.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/ui/loft_sections.tlspr"); err != nil {
		panic(err)
	}
	doc.LoftThrough([]Group{doc.Group("top"), doc.Group("middle"), doc.Group("bottom")}, "")
}
