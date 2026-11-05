// The mend of path_damaged_mend_circle.script, made by a script
// (TestPathEquivalence/damaged_mend_circle): the diameter whose circle
// the damaged file lacks is given the file's circle (ROADMAP D5).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/tlspr/damaged/constraint_missing_circle.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.MendLeftOut(doc.Features.LeftOutMissing()[0], doc.Entity(0x40000))
}
