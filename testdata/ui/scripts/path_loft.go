// The loft of path_loft.script, made by a script
// (TestPathEquivalenceLoft): the top square of
// testdata/ui/loft_profiles.spcs, the active sketch, lofted to the
// bottom one.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/ui/loft_profiles.spcs"); err != nil {
		panic(err)
	}
	doc.Loft(doc.Group("top"), doc.Group("bottom"), "")
}
