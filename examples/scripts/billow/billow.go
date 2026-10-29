// A ring of circles lofted round and closed on itself, held by a guide
// that swells up between them and square to every other circle at once,
// made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/billow/billow.go [profiles]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/billow/billow.go
//
// profiles circles (8 unless given, an even count) of radius 5 stand
// round the z axis at radius 30, each in the upright plane through the
// axis. A guide of cubics, one from the top of each circle to the next,
// swells up between them; a smooth loft goes through the circles, closed
// (LoftGroup.SetClosed), held by the guide (LoftGroup.SetGuides) and
// square to every other circle (LoftGroup.SetProfileTangency), so the
// ring runs straight through those. The script prints the volume, and
// whether the solid is sound.
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
		if err != nil || v < 4 || v%2 == 1 {
			fmt.Fprintln(os.Stderr, "the ring needs an even count of profiles, four at least")
			os.Exit(2)
		}
		n = v
	}
	const big, r = 30.0, 5.0

	doc := api.Current()
	var profiles []api.Group
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		out := api.V(math.Cos(a), math.Sin(a), 0)
		doc.Features.Sketch3D("")
		at := doc.Sketch.Point(out.ScaledBy(big))
		profiles = append(profiles, doc.Features.SketchAt(at, out, api.V(0, 0, 1), fmt.Sprintf("profile%d", i+1)))
		doc.Sketch.Circle(out.ScaledBy(big), r)
	}

	// The guide: a cubic from the top of each circle to the next, its
	// handles along the way round and lifted, so it leaves each top level.
	doc.Features.Sketch3D("guide")
	top := func(a float64) api.Vec { return api.V(big*math.Cos(a), big*math.Sin(a), r) }
	way := func(a float64) api.Vec { return api.V(-math.Sin(a), math.Cos(a), 0) }
	th := 2 * math.Pi / float64(n)
	k := 4.0 / 3 * math.Tan(th/4) * big
	lift := api.V(0, 0, 1.5)
	var guide []api.Entity
	for i := 0; i < n; i++ {
		a, b := th*float64(i), th*float64(i+1)
		guide = append(guide, doc.Sketch.Cubic(top(a), top(a).Plus(way(a).ScaledBy(k)).Plus(lift),
			top(b).Minus(way(b).ScaledBy(k)).Plus(lift), top(b)))
	}

	ring := doc.Features.LoftThrough(profiles, "ring")
	ring.AsLoft().SetClosed(true)
	ring.AsLoft().SetGuides(guide)
	// Every other circle between the first and the last: a ring has no
	// ends to hold.
	for i := 2; i < n-1; i += 2 {
		ring.AsLoft().SetProfileTangency(profiles[i], 1)
	}
	if msg := ring.Problem(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	res := ring.Analyze().Check()
	fmt.Printf("billowing ring of %d profiles: %.0f mm³, sound %v\n", n, res.Volume, res.Sound())
	if !res.Sound() {
		os.Exit(1)
	}
}
