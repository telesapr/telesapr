// The loft of path_loft_ring_wide.script, made by a script
// (TestPathEquivalence/loft_ring_wide): loft_ring_wide.tlspr's smooth
// loft through three circles within a sixth of the ring round the z
// axis, closed into a ring the long way back to the first.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_ring_wide.tlspr"); err != nil {
		panic(err)
	}
	doc.Group("ring").AsLoft().SetClosed(true)
}
