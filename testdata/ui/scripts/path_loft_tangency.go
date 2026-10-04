// The loft of path_loft_tangency.script, made by a script
// (TestPathEquivalenceLoftTangency): path_loft's loft, leaving the top
// square to it and reaching the bottom half as square (roadmap 28.3).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/ui/loft_profiles.tlspr"); err != nil {
		panic(err)
	}
	doc.Loft(doc.Group("top"), doc.Group("bottom"), "").SetTangency(1, 0.5)
}
