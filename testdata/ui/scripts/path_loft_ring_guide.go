// The loft of path_loft_ring_guide.script, made by a script
// (TestPathEquivalence/loft_ring_guide): loft_ring_guide.tlspr's closed
// loft through six circles round the z axis, held by the six cubics of
// its guide sketch.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/loft_ring_guide.tlspr"); err != nil {
		panic(err)
	}
	var guide []Entity
	for i := uint32(0x10); i <= 0x15; i++ {
		guide = append(guide, doc.Entity(i<<16))
	}
	doc.Group("ring").AsLoft().SetGuides(guide)
}
