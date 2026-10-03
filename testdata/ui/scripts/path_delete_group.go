// The model of path_delete_group.script, made by a script
// (TestPathEquivalenceDeleteGroup).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	ex := doc.Extrude(80, "")
	doc.Translate(3, V(10, 0, 0), "")
	ex.Delete()
}
