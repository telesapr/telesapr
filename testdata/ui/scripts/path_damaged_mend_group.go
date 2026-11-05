// The mend of path_damaged_mend_group.script, made by a script
// (TestPathEquivalence/damaged_mend_group): the extrusion whose sketch
// the damaged file lacks is given the file's sketch (ROADMAP D5).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/tlspr/damaged/group_missing_operand.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.MendLeftOutGroup(doc.Features.LeftOutMissingGroups()[0], doc.Group("sketch-in-plane"))
}
