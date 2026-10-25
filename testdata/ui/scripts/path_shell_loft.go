// The model of path_shell_loft.script, made by a script
// (TestPathEquivalence/shell_loft).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_profiles.tlspr"); err != nil {
		panic(err)
	}
	loft := doc.Features.Loft(doc.Group("top"), doc.Group("bottom"), "")
	side := loft.Solid().FaceAt(V(0, -15, 15))
	doc.Features.ShellSolid([]SolidFace{side}, 1, "")
}
