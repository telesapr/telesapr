package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestThreadClassFits makes a thread in a loose class and in a tight
// one of each form that has classes besides the ISO and unified ones,
// and requires the loose one to have less material, a screw's ridges
// thinner and a nut's hole wider, by about what moving the ridges by
// the classes' difference of clearance takes off: the thread's
// cylinder, 2π r long round and as long as the thread, that thick.
func TestThreadClassFits(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	for _, c := range []struct {
		name, designation string
		radius, height    float64
		internal          bool
		loose, tight      string
	}{
		{"acme screw", "1/2-10 ACME", 6.35, 30, false, "2G", "5G"},
		{"acme nut", "1/2-10 ACME", 12, 12, true, "2G", "4G"},
		{"butt screw", "1-12 BUTT", 12.7, 40, false, "2A", "3A"},
		{"butt nut", "1-12 BUTT", 20, 15, true, "2B", "3B"},
		{"s20 screw", "S20x4", 10, 40, false, "9c", "7e"},
		{"s20 nut", "S20x4", 16, 20, true, "9H", "7H"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var length float64
			volume := func(class string) (v, clearance float64) {
				o := sketch.ThreadOptions{Length: 12, Internal: c.internal, Through: c.internal, Class: class}
				if c.internal {
					o.Length = c.height
				}
				tc := threadCase{c.name, c.radius, c.height, c.designation, o}
				sk, _, res := tc.build(t, 0.01)
				if !res.Sound {
					t.Errorf("%s isn't sound", class)
				}
				opts := tc.options(t)
				clearance, err := opts.FitClearance()
				if err != nil {
					t.Fatal(err)
				}
				length = opts.Length
				mesh, _ := meshStats(sk, res.Group)
				return mesh.CalculateVolume(), clearance
			}
			loose, cl := volume(c.loose)
			tight, ct := volume(c.tight)
			s := threadCase{designation: c.designation}.options(t).Spec
			want := 2 * math.Pi * s.Diameter / 2 * length * (cl - ct)
			if got := tight - loose; !(cl > ct) || got < 0.5*want || got > 1.5*want {
				t.Errorf("%s %.3f mm³ (clearance %.4f mm), %s %.3f (%.4f): %.3f less, want about %.3f",
					c.loose, loose, cl, c.tight, tight, ct, got, want)
			}
		})
	}
}
