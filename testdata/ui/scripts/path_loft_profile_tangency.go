// The loft of path_loft_profile_tangency.script, made by a script
// (TestPathEquivalence/loft_profile_tangency): loft_ring.tlspr's smooth
// loft held square to its third profile.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_ring.tlspr"); err != nil {
		panic(err)
	}
	doc.Group("ring").AsLoft().SetProfileTangency(doc.Group("p3"), 1)
}
