// The loft of path_loft_guides.script, made by a script
// (TestPathEquivalenceLoftGuides): the top circle lofted to the bottom
// one, held to the guide sketch's cubic (roadmap 28.3).
package main

import "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := api.Current()
	if err := doc.Load("../../testdata/ui/loft_guide_profiles.tlspr"); err != nil {
		panic(err)
	}
	loft := doc.Loft(doc.Group("top"), doc.Group("bottom"), "")
	loft.SetGuides([]api.Entity{doc.Entity(0x50000)})
}
