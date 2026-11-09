package api

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ringAxis is how far in y from the x band's middle ringBandOnCone's
// axis lies, and ringTurn the degrees it turns the band's end by.
const ringAxis, ringTurn = 40.0, 20.0

// ringBandOnCone is curvedBandOnCone's x band, 6 wide, its top an arc
// rising sag above its sides at height top (flat at sag 0), its end
// at x = 6 turned by ringTurn about the upright axis through
// (6, -ringAxis): a ring's sector on the cone, whose sides are
// cylinders and whose top is a torus, its edges with the sides circles.
// It gives the edge of the outer side on the cone and the volume
// before.
func ringBandOnCone(d *Doc, top, sag float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(V(0, 0, -2), V(0, 0, 32))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
	d.Features.Lathe(axis, "")
	d.Features.Sketch3D("end")
	d.Features.SketchIn(d.Sketch.NewWorkplane(V(6, 0, 0), V(0, 1, 0), V(0, 0, 1)), "")
	ax := d.Sketch.Line(V(6, -ringAxis, -2), V(6, -ringAxis, 32))
	ax.SetConstruction(true)
	d.Sketch.Line(V(6, -3, top), V(6, -3, 0))
	d.Sketch.Line(V(6, -3, 0), V(6, 3, 0))
	d.Sketch.Line(V(6, 3, 0), V(6, 3, top))
	if sag == 0 {
		d.Sketch.Line(V(6, 3, top), V(6, -3, top))
	} else {
		c := (9 - sag*sag) / (2 * sag)
		d.Sketch.Arc(V(6, 0, top-c), V(6, 3, top), V(6, -3, top))
	}
	band := d.Features.Revolve(ax, ringTurn, "band")
	// The outer side, ringAxis+3 from the axis, meets the cone (radius
	// 8 at z = 10) where it leaves it.
	rho := ringAxis + 3
	lo, hi := 0.0, math.Pi/4
	at := func(th float64) Vec { return V(6+rho*math.Sin(th), -ringAxis+rho*math.Cos(th), 10) }
	for range 100 {
		m := (lo + hi) / 2
		if p := at(m); math.Hypot(p.X, p.Y) < 8 {
			lo = m
		} else {
			hi = m
		}
	}
	return band.Solid().EdgeAt(at(lo)), band.Analyze().Check().Volume
}

// ringBox is the box in x and y over which ringOracle measures: the
// whole band and the blends beside it.
var ringBox = [4]float64{5, 22, 0, 12}

// makeFlatRingBlend is the blend of size r of the ring band with a flat
// top at 30, which no blend runs off: what it adds, as ringAdded
// measures it, and its mesh over ringBox. It depends on neither the
// top's height nor its sag.
func makeFlatRingBlend(chamfer bool, r float64) (flatTopBlend, error) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	d.Edit.SetMaxSegments(200)
	edge, _ := ringBandOnCone(d, 30, 0)
	g, err := blendOrRefusal(d, edge, r, chamfer)
	if err != nil {
		return flatTopBlend{}, err
	}
	rep := g.Analyze().Check()
	if !rep.Sound() {
		return flatTopBlend{}, fmt.Errorf("the ring band with a flat top: %s", rep.Summary())
	}
	tris := meshOver(g, ringBox[0], ringBox[1], ringBox[2], ringBox[3])
	return flatTopBlend{ringAdded(tris), tris}, nil
}

// ringAdded is what a blend of the ring band's outer edge adds, from the
// mesh tris over ringBox: the volume over the band's span out from its
// outer side by up to 6, above the lathe's exact cone (radius 10 - z/5),
// where nothing but the blend lies; by vertical lines as ringOracle's.
// The solid's volume less the band's would also count the slivers
// between the exact side and the side's chords in the band's mesh,
// which the blend fills where it takes the side away: more where the
// end runs off and takes the side up to the top (0.09 mm³, 0.18%, of a
// chamfer of 2.5 at the top of 22), and between the cone's chords and
// the cone under the blend.
func ringAdded(tris []srf.STriangle) float64 {
	const n, step = 1200, 0.005
	col := meshColumns(tris, ringBox[0], ringBox[1], ringBox[2], ringBox[3], n)
	turn := ringTurn * math.Pi / 180
	vol := 0.0
	for i := range int(turn * (ringAxis + 3) / step) {
		th := (float64(i) + 0.5) * step / (ringAxis + 3)
		for out := 3 + step/2; out < 9; out += step {
			rho := ringAxis + out
			x, y := 6+rho*math.Sin(th), -ringAxis+rho*math.Cos(th)
			vol += lengthAbove(col(x, y), (10-math.Hypot(x, y))*5) * step * step * rho / (ringAxis + 3)
		}
	}
	return vol
}

// ringOracle is what a blend of the ring band's edge adds where its top,
// at top with sag, is carried on past the band's outer side by its
// tangents across the edge: the cone about the ring's axis that meets
// the torus there smoothly. It is the same blend of the band with a
// flat top at 30 less what of it lies above that cone out from the side:
// along each line square to the side, outward up to where the blend
// first lies under the cone. The cone falls past the side so steeply
// (67 degrees at a sag of 2) that further out it passes under the
// blend's far part again, which the grown top doesn't reach and the
// blend keeps. By vertical lines every 0.005 along the lines square to
// the side, out from it and along it (meshColumns).
func ringOracle(flat flatTopBlend, top, sag float64) float64 {
	c := (9 - sag*sag) / (2 * sag)
	const n, step = 1200, 0.005
	col := meshColumns(flat.tris, ringBox[0], ringBox[1], ringBox[2], ringBox[3], n)
	turn := ringTurn * math.Pi / 180
	above := 0.0
	for i := range int(turn * (ringAxis + 3) / step) {
		th := (float64(i) + 0.5) * step / (ringAxis + 3)
		for out := 3 + step/2; out < 9; out += step {
			rho := ringAxis + out
			x, y := 6+rho*math.Sin(th), -ringAxis+rho*math.Cos(th)
			l := lengthAbove(col(x, y), top-3*(out-3)/c)
			if l == 0 {
				break
			}
			above += l * step * step * rho / (ringAxis + 3)
		}
	}
	return flat.added - above
}

// ringRefused are the blends of TestBlendRunsOffPastRingTop refused, and
// what the refusal says: a fillet whose end runs off the band's side only
// just, as the x band's at the same top, sag and size, which the face
// the tangents rule would hold as a sliver of no area.
var ringRefused = map[string]string{
	"fillet, top 22 sag 2 r 2.5": "only just runs off",
}

// Chamfers and fillets of the ring band's outer edge on the cone whose
// end runs off past its round top, a torus, which curves down across
// the band so tightly that, carried on as its own circle does, it comes
// round under the band again and a wide blend's end section lies
// wholly past it; its edge with the band's side is a circle, so no
// tangent plane takes it on either. 23 of the 54 were refused before
// 2026-11-09 as running off "the faces on both sides". The top is
// carried on by its tangents across the edge (topo's ruledIfPast), a
// new face of the top's sheet beside it: every blend but one
// (ringRefused) is sound and adds what the same blend of the ring band
// with a flat top at 30 adds less what of it lies above the cone those
// tangents make out from the side (ringOracle), within 0.1% (0.04%
// measured), both measured over the blend's region above the exact cone
// (ringAdded). Measured as the solid's volume less the band's, they
// were up to 0.42% apart, the same at every sag of a top and for the 31
// made before the tangents (2026-11-09): that also counts the slivers
// between the band's exact side and its chords, which the blend fills
// where it takes the side away, further up the side where the end runs
// off. The ordinary run takes the chamfers and fillets of 2.5 at each
// top and sag, -short two, make test-sweeps the sizes of 2 and 3 too.
func TestBlendRunsOffPastRingTop(t *testing.T) {
	t.Parallel()
	type key struct {
		chamfer     bool
		top, sag, r float64
	}
	var keys []key
	for _, chamfer := range []bool{true, false} {
		for _, top := range []float64{20, 21, 22} {
			for _, sag := range []float64{0.5, 1, 2} {
				for _, r := range []float64{2, 2.5, 3} {
					if r == 2.5 || sweep.Full() {
						keys = append(keys, key{chamfer, top, sag, r})
					}
				}
			}
		}
	}
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		keys = []key{{true, 20, 2, 2.5}, {false, 22, 2, 2.5}}
	}
	type blend struct {
		chamfer bool
		r       float64
	}
	flat := map[blend]func() (flatTopBlend, error){}
	for _, k := range keys {
		b := blend{k.chamfer, k.r}
		if flat[b] == nil {
			flat[b] = sync.OnceValues(func() (flatTopBlend, error) { return makeFlatRingBlend(b.chamfer, b.r) })
		}
	}
	for _, k := range keys {
		what := "fillet"
		if k.chamfer {
			what = "chamfer"
		}
		name := fmt.Sprintf("%s, top %g sag %g r %g", what, k.top, k.sag, k.r)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, _ := ringBandOnCone(d, k.top, k.sag)
			g, err := blendOrRefusal(d, edge, k.r, k.chamfer)
			if want, ok := ringRefused[name]; ok {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%v, want a refusal saying %q", err, want)
				}
				return
			}
			if err != nil {
				t.Fatalf("%v", err)
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Fatalf("%s", rep.Summary())
			}
			added := ringAdded(meshOver(g, ringBox[0], ringBox[1], ringBox[2], ringBox[3]))
			f, err := flat[blend{k.chamfer, k.r}]()
			if err != nil {
				t.Fatalf("oracle: %v", err)
			}
			want := ringOracle(f, k.top, k.sag)
			if math.Abs(added-want) > 0.001*want {
				t.Errorf("added %g, the oracle %g", added, want)
			}
		})
	}
}
