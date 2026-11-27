package sketch_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// dragCase is a mechanism and a drag of one of its points: the targets
// it is pulled to in turn, each a step of a drag, and where along its
// path the point can reach each (reach), for how far off it ends.
type dragCase struct {
	name  string
	build func(b testing.TB) (d *api.Doc, pt api.Entity, targets []api.Vec)
}

// dragCases are the example mechanisms, each dragged in steps of a few
// millimetres or degrees and then by one long jump back.
func dragCases() []dragCase {
	return []dragCase{
		{"hinged", func(b testing.TB) (*api.Doc, api.Entity, []api.Vec) {
			d, _, _, far, pivot := hingedBar(b)
			start := far.Pos()
			var ts []api.Vec
			for k := 1; k <= 20; k++ {
				ts = append(ts, turned(start, pivot, 3*float64(k)))
			}
			return d, far, append(ts, start)
		}},
		{"ball", func(b testing.TB) (*api.Doc, api.Entity, []api.Vec) {
			d, ground, bar := twoBars(b)
			ug, ub := uprights(b, ground), uprights(b, bar)
			pivot := ug[60].Start().Pos()
			d.Assembly.Ball(ug[60].Start(), ub[0].Start())
			far := ub[60].Start()
			start := far.Pos()
			var ts []api.Vec
			for k := 1; k <= 20; k++ {
				// Round the ball, up and aside.
				a := 3 * float64(k) * math.Pi / 180
				v := start.Minus(pivot)
				r := v.Magnitude()
				ts = append(ts, pivot.Plus(api.V(r*math.Cos(a)*v.X/r, r*math.Cos(a)*v.Y/r+r*math.Sin(a)*0.6, r*math.Sin(a)*0.8)))
			}
			return d, far, append(ts, start)
		}},
		{"fourbar", func(b testing.TB) (*api.Doc, api.Entity, []api.Vec) {
			d, _, _, coupler, _, _ := fourBar(b, true)
			var pt api.Entity
			for _, e := range coupler.Entities() {
				if e.Kind() == "point" {
					pt = e
					break
				}
			}
			start := pt.Pos()
			var ts []api.Vec
			for k := 1; k <= 20; k++ {
				ts = append(ts, start.Plus(api.V(0, float64(k), 0)))
			}
			return d, pt, append(ts, start)
		}},
		{"lid", func(b testing.TB) (*api.Doc, api.Entity, []api.Vec) {
			dir := b.TempDir()
			if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "..", "testdata", "ui", "mechanism"))); err != nil {
				b.Fatal(err)
			}
			d, err := api.Open(filepath.Join(dir, "boxlid.tlspr"))
			if err != nil {
				b.Fatal(err)
			}
			gs := d.Groups()
			lid := gs[len(gs)-1]
			var front api.Entity
			hi := math.Inf(-1)
			for _, e := range lid.Entities() {
				if e.Kind() != "point" {
					continue
				}
				p := e.Pos()
				hi = math.Max(hi, p.Y)
				if front.H.V == 0 || p.Z > front.Pos().Z+1e-6 || math.Abs(p.Z-front.Pos().Z) < 1e-6 && p.Y < front.Pos().Y-1e-6 {
					front = e
				}
			}
			at := front.Pos()
			hingeY, hingeZ := hi, at.Z-3
			opened := func(deg float64) api.Vec {
				s, c := math.Sincos(deg * math.Pi / 180)
				dy, dz := at.Y-hingeY, at.Z-hingeZ
				return api.V(at.X, hingeY+c*dy+s*dz, hingeZ-s*dy+c*dz)
			}
			var ts []api.Vec
			for k := 1; k <= 20; k++ {
				ts = append(ts, opened(5*float64(k)))
			}
			return d, front, append(ts, opened(0))
		}},
		{"engine", func(b testing.TB) (*api.Doc, api.Entity, []api.Vec) {
			d, _, _, piston, bearing := engine(b)
			setParam(b, d, "angle", 40)
			pin := nearestPoint(piston, bearing.Plus(api.V(0, 1000, 0)))
			at := pin.Pos()
			var ts []api.Vec
			for k := 1; k <= 10; k++ {
				ts = append(ts, at.Plus(api.V(0, -float64(k), 0)))
			}
			return d, pin, append(ts, at)
		}},
	}
}

// BenchmarkDragMechanisms drags a point of each example mechanism in
// steps, with the plan (mechanism_plan.go) and without it, and reports
// the time of a step, its pull and the regeneration after, and of the
// pull alone (pull-ms), and how far
// the point ends from where it was pulled to, on average (off-mm) and
// at most (max-off-mm), and the share of the steps the plan took
// (by-plan).
func BenchmarkDragMechanisms(b *testing.B) {
	for _, tc := range dragCases() {
		for _, plan := range []bool{true, false} {
			name := tc.name + "/plan"
			if !plan {
				name = tc.name + "/null-space"
			}
			b.Run(name, func(b *testing.B) {
				d, pt, ts := tc.build(b)
				d.With(func(sk *sketch.Sketch) { sk.NoMechanismPlan = !plan })
				var off, most float64
				var dt, dp time.Duration
				n := 0
				b.ResetTimer()
				for range b.N {
					for _, to := range ts {
						t0 := time.Now()
						pt.MoveTo(to)
						dp += time.Since(t0)
						at := pt.Pos()
						dt += time.Since(t0)
						o := at.Minus(to).Magnitude()
						off += o
						most = math.Max(most, o)
						n++
					}
				}
				b.StopTimer()
				pulls := 0
				d.With(func(sk *sketch.Sketch) { pulls = sk.PlanPulls })
				b.ReportMetric(float64(pulls)/float64(n), "by-plan")
				b.ReportMetric(float64(dt.Microseconds())/float64(n)/1000, "ms/step")
				b.ReportMetric(float64(dp.Microseconds())/float64(n)/1000, "pull-ms/step")
				b.ReportMetric(off/float64(n), "off-mm")
				b.ReportMetric(most, "max-off-mm")
			})
		}
	}
}
