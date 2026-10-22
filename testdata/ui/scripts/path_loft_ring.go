// The loft of path_loft_ring.script, made by a script
// (TestPathEquivalenceLoftRing): loft_ring.tlspr's smooth loft through
// six circles round the z axis, closed into a ring.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_ring.tlspr"); err != nil {
		panic(err)
	}
	doc.Group("ring").AsLoft().SetClosed(true)
}
