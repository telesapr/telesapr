// The loft of path_loft.script, made by a script
// (TestPathEquivalence/loft): the top square of
// testdata/ui/loft_profiles.tlspr, the active sketch, lofted to the
// bottom one.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_profiles.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Loft(doc.Group("top"), doc.Group("bottom"), "")
}
