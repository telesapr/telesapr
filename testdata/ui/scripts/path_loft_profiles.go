// The loft of path_loft_profiles.script, made by a script
// (TestPathEquivalence/loft_profiles): the top circle of
// testdata/ui/loft_sections.tlspr lofted through the middle square to
// the bottom circle, then between the square and the bottom circle
// (ROADMAP F7).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_sections.tlspr"); err != nil {
		panic(err)
	}
	l := doc.Features.LoftThrough([]Group{doc.Group("top"), doc.Group("middle"), doc.Group("bottom")}, "")
	l.AsLoft().SetProfiles([]Group{doc.Group("bottom"), doc.Group("middle")})
}
