package topo_test

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A rib on a plate: a wedge 14 along its foot and h high at its back,
// 10 wide, its width leaning by lean along the foot, either way,
// standing on the plate's top, z = 0. Its back and sides are upright and its top slopes
// down to the plate at the front. The four edges of its foot go in; the
// two along its slope and the two up its back stick out.
type rib struct{ h, lean float64 }

const ribFoot, ribWide = 14.0, 10.0

func (rb rib) shell() *srf.SShell {
	gray := core.NewRGBA(100, 100, 100, 255)
	var plate, wedge, out srf.SShell
	plate.MakePrism([]geom.Vector{v(-10, -8, -3), v(28, -8, -3), v(28, 18, -3), v(-10, 18, -3)}, v(0, 0, 0), v(0, 0, 3), gray)
	// The wedge goes 1 into the plate, for the union.
	wedge.MakePrism([]geom.Vector{v(0, 0, -1), v(ribFoot, 0, -1), v(ribFoot, 0, 0), v(0, 0, rb.h)},
		v(0, 0, 0), v(rb.lean, ribWide, 0), gray)
	out.MakeFromUnionOf(&plate, &wedge)
	return &out
}

// foot are the rib's foot edges: its two sides, its back and its front.
func (rb rib) foot() (sides, ends [][2]geom.Vector) {
	a, b := v(0, 0, 0), v(ribFoot, 0, 0)
	c, d := v(ribFoot+rb.lean, ribWide, 0), v(rb.lean, ribWide, 0)
	return [][2]geom.Vector{{a, b}, {c, d}}, [][2]geom.Vector{{d, a}, {b, c}}
}

// filled is the volume the fillets of radius r at the walls picked (the
// sides, the back and front) add to the rib.
//
// A wall with its foot filled is a half space with a rounded foot, and
// the rib with its foot filled is what they share: the fillets are cut
// where they meet each other and where they meet the other walls'
// planes, as the mitres cut them. Over the plate that is a height, the
// least of the walls' heights: none behind an upright wall, the slope's
// own behind the front one, the fillet's arc from where it touches the
// wall to where it touches the plate, and 0 beyond. The volume is that
// height's sum, less the rib's own, over a grid whose lines the upright
// walls lie along, so that a step never falls inside a cell.
func (rb rib) filled(r float64, sides, ends bool) float64 {
	wall := func(theta, r float64) func(s float64) float64 {
		// s is how far outside the wall's foot a point of the plate lies.
		ss := r * math.Tan(theta/2)
		touch := -ss * math.Cos(theta)
		return func(s float64) float64 {
			switch {
			case s <= touch && theta > math.Pi/2-1e-12:
				if s <= 0 {
					return math.Inf(1)
				}
				return 0
			case s <= touch:
				return math.Max(0, -s*math.Tan(theta))
			case s >= ss:
				return 0
			}
			return r - math.Sqrt(r*r-(s-ss)*(s-ss))
		}
	}
	dl := math.Hypot(rb.lean, ribWide)
	// The front wall's slope across its foot, which runs along the lean.
	slope := math.Atan2(rb.h, ribFoot*ribWide/dl)
	radius := func(on bool) float64 {
		if on {
			return r
		}
		return 0
	}
	side, back := wall(math.Pi/2, radius(sides)), wall(math.Pi/2, radius(ends))
	front := wall(slope, radius(ends))
	side0, back0, front0 := wall(math.Pi/2, 0), wall(math.Pi/2, 0), wall(slope, 0)
	const step, margin = 0.005, 4.0
	nu, ny := int(math.Round((ribFoot+2*margin)/step)), int(math.Round((ribWide+2*margin)/step))
	// The rows are summed side by side, in the order they come.
	rows := make([]float64, ny)
	var wg sync.WaitGroup
	for k := range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			for j := k; j < ny; j += runtime.GOMAXPROCS(0) {
				y := -margin + (float64(j)+0.5)*step
				for i := range nu {
					// u runs along the foot from the back wall, at any y.
					u := -margin + (float64(i)+0.5)*step
					sb, sf := -u*ribWide/dl, (u-ribFoot)*ribWide/dl
					with := math.Min(math.Min(side(-y), side(y-ribWide)), math.Min(back(sb), front(sf)))
					bare := math.Min(math.Min(side0(-y), side0(y-ribWide)), math.Min(back0(sb), front0(sf)))
					rows[j] += with - bare
				}
			}
		}(k)
	}
	wg.Wait()
	sum := 0.0
	for _, x := range rows {
		sum += x
	}
	return sum * step * step
}

// TestFilletRibFootSweep fills the foot of a rib all round, or along
// its sides or its ends only, over the rib's height (its slope: 20 to
// 71 degrees), the lean of its width and the radius. Where two of the
// foot's fillets meet below an edge of the slope they turn by different
// angles, and the edge between them, left sharp, sticks out while they
// go in: an uneven mitre that lies outside both walls, the slope growing
// out to it. At the back the two turn alike, and with the sides or ends
// alone each fillet runs on to the wall at its end. The volume added is
// checked against the height summed over the plate (filled); the rate
// allowed, 0, was set before.
func TestFilletRibFootSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("h", 5, 10, 20, 40),
			sweep.Floats("lean", -4, -2, 0, 2, 4),
			sweep.Floats("r", 0.5, 1, 2),
			sweep.Strings("edges", "all", "sides", "ends"),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		rb := rib{p.Float("h"), p.Float("lean")}
		r, which := p.Float("r"), p.String("edges")
		sides, ends := rb.foot()
		var segs [][2]geom.Vector
		length := 0.0
		if which != "ends" {
			segs = append(segs, sides...)
		}
		if which != "sides" {
			segs = append(segs, ends...)
		}
		for _, sg := range segs {
			length += sg[1].Minus(sg[0]).Magnitude()
		}
		tol := 0.0003 * r
		srf.SetMaxSegments(400)
		defer srf.SetMaxSegments(20)
		srf.SetChordTolMm(tol)
		sh := rb.shell()
		before := meshVolume(sh)
		if err := topo.FilletEdges(sh, segs, r, 1<<30); err != nil {
			return err
		}
		tp, err := topo.Build(sh)
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		want := rb.filled(r, which != "ends", which != "sides")
		// The chords lose less than tol over the fillets' faces, and the
		// sum over the plate is good to a few thousandths.
		allow := tol*math.Pi/2*r*(length+8*r) + 0.004
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case len(tp.Validate()) > 0:
			return fmt.Errorf("topology: %s", tp.Validate()[0].Msg)
		case math.Abs(rep.Volume-before-want) > allow:
			return fmt.Errorf("added %g, want %g within %g", rep.Volume-before, want, allow)
		}
		return nil
	}).Require(t)
}

// A flat face a fillet makes larger must still lie under its control
// points, or a later Boolean misses what meets the part that grew: the
// slope a side's fillet runs on to, and the slope grown out to the
// mitre of two. A box cut through the fillet there, and the same box
// kept, are sound and together hold what the rib held.
func TestFilletGrownFaceCut(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	rb := rib{10, 0}
	sides, ends := rb.foot()
	gray := core.NewRGBA(100, 100, 100, 255)
	for _, c := range []struct {
		name string
		segs [][2]geom.Vector
	}{{"sides", sides}, {"all", append(append([][2]geom.Vector{}, sides...), ends...)}} {
		sh := rb.shell()
		if err := topo.FilletEdges(sh, c.segs, 2, 1<<30); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		whole := sound(t, sh)
		// Across the side's fillet where it ends on the slope, and
		// across the corner where the slope's foot begins.
		for _, x := range [][2]float64{{10.5, 11.5}, {12.8, 15.5}} {
			var box, cut, kept srf.SShell
			box.MakePrism([]geom.Vector{v(x[0], -4, 0.4), v(x[1], -4, 0.4), v(x[1], 1.5, 0.4), v(x[0], 1.5, 0.4)},
				v(0, 0, 0), v(0, 0, 8), gray)
			cut.MakeFromDifferenceOf(sh, &box)
			kept.MakeFromIntersectionOf(sh, &box)
			if cut.BooleanFailed || kept.BooleanFailed {
				t.Errorf("%s, a box from x = %g to %g: the Boolean failed", c.name, x[0], x[1])
				continue
			}
			a, b := sound(t, &cut), sound(t, &kept)
			if b < 0.05 || math.Abs(a+b-whole) > 0.01 {
				t.Errorf("%s, a box from x = %g to %g: %g cut and %g kept, of %g", c.name, x[0], x[1], a, b, whole)
			}
		}
	}
}
