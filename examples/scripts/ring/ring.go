// A ring of circles that swell and narrow, lofted round and closed on
// itself, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/ring/ring.go [profiles]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/ring/ring.go
//
// profiles circles (8 unless given) stand round the z axis at radius 30,
// each in the upright plane through the axis, of radius 6 and 4 by turns.
// A smooth loft goes through them in order; closed
// (LoftGroup.SetClosed), it goes from the last back to the first, a ring
// with no caps, as smooth where it closes as anywhere else. The narrow
// profiles are held square (LoftGroup.SetProfileTangency), the last of
// them too where there is an even number, so the ring runs straight
// through each waist. The script prints the volume, and whether the solid is
// sound.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	n := 8
	if len(os.Args) > 1 {
		v, err := strconv.Atoi(os.Args[1])
		if err != nil || v < 4 {
			fmt.Fprintln(os.Stderr, "the ring needs four profiles at least")
			os.Exit(2)
		}
		n = v
	}

	doc := api.Current()
	var profiles []api.Group
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		out := api.V(math.Cos(a), math.Sin(a), 0)
		doc.Features.Sketch3D("")
		at := doc.Sketch.Point(out.ScaledBy(30))
		profiles = append(profiles, doc.Features.SketchAt(at, out, api.V(0, 0, 1), fmt.Sprintf("profile%d", i+1)))
		r := 6.0
		if i%2 == 1 {
			r = 4
		}
		doc.Sketch.Circle(out.ScaledBy(30), r)
	}

	ring := doc.Features.LoftThrough(profiles, "ring")
	ring.AsLoft().SetClosed(true)
	for i := 1; i < n; i += 2 {
		ring.AsLoft().SetProfileTangency(profiles[i], 0.8)
	}
	if msg := ring.Problem(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	r := ring.Analyze().Check()
	fmt.Printf("ring of %d profiles: %.0f mm³, sound %v\n", n, r.Volume, r.Sound())
	if !r.Sound() {
		os.Exit(1)
	}
}
