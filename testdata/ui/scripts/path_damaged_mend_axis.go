// The mend of path_damaged_mend_axis.script, made by a script
// (TestPathEquivalence/damaged_mend_axis): the lathe whose axis the
// damaged file lacks is given the file's construction line (ROADMAP D5).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/tlspr/damaged/lathe_missing_axis.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.MendLeftOut(doc.Features.LeftOutMissing()[0], doc.Entity(0x50000))
}
