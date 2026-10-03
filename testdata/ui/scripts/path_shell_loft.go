// The model of path_shell_loft.script, made by a script
// (TestPathEquivalenceShellLoftFace).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/ui/loft_profiles.tlspr"); err != nil {
		panic(err)
	}
	loft := doc.Loft(doc.Group("top"), doc.Group("bottom"), "")
	side := loft.Solid().FaceAt(V(0, -15, 15))
	doc.ShellSolid([]SolidFace{side}, 1, "")
}
