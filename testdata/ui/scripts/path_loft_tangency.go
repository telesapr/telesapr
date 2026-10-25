// The loft of path_loft_tangency.script, made by a script
// (TestPathEquivalence/loft_tangency): path_loft's loft, leaving the top
// square to it and reaching the bottom half as square (roadmap 28.3).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_profiles.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Loft(doc.Group("top"), doc.Group("bottom"), "").AsLoft().SetTangency(1, 0.5)
}
