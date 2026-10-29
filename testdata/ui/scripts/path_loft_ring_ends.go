// The loft of path_loft_ring_ends.script, made by a script
// (TestPathEquivalence/loft_ring_ends): loft_ring.tlspr's smooth loft
// through six circles round the z axis, closed into a ring and held
// square at its first and last profiles.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_ring.tlspr"); err != nil {
		panic(err)
	}
	ring := doc.Group("ring").AsLoft()
	ring.SetClosed(true)
	ring.SetTangency(1, 0.5)
}
