package api

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// meshAbove is the volume of the model up to and with group g that
// lies over the box [x0, x1] by [y0, y1] and above the height zMin(y)
// there, by vertical lines through the centres of n by n cells: each
// crosses the mesh where it goes in and out, so that the lengths inside
// above zMin add up. The mesh must be closed there.
func meshAbove(g Group, x0, x1, y0, y1 float64, n int, zMin func(y float64) float64) float64 {
	var tris []srf.STriangle
	g.d.readMesh(func(sk *sketch.Sketch) {
		for _, tr := range displayMesh(sk, g.get(sk)).Triangles {
			if max(tr.A.X, tr.B.X, tr.C.X) < x0 || min(tr.A.X, tr.B.X, tr.C.X) > x1 ||
				max(tr.A.Y, tr.B.Y, tr.C.Y) < y0 || min(tr.A.Y, tr.B.Y, tr.C.Y) > y1 {
				continue
			}
			tris = append(tris, tr)
		}
	})
	// The triangles by the cells their boxes cover.
	dx, dy := (x1-x0)/float64(n), (y1-y0)/float64(n)
	cells := make([][]int, n*n)
	cell := func(v, lo, d float64) int { return max(0, min(n-1, int(math.Floor((v-lo)/d)))) }
	for i, tr := range tris {
		for a := cell(min(tr.A.X, tr.B.X, tr.C.X), x0, dx); a <= cell(max(tr.A.X, tr.B.X, tr.C.X), x0, dx); a++ {
			for b := cell(min(tr.A.Y, tr.B.Y, tr.C.Y), y0, dy); b <= cell(max(tr.A.Y, tr.B.Y, tr.C.Y), y0, dy); b++ {
				cells[a*n+b] = append(cells[a*n+b], i)
			}
		}
	}
	vol := 0.0
	var zs []float64
	for a := range n {
		for b := range n {
			x, y := x0+(float64(a)+0.5)*dx, y0+(float64(b)+0.5)*dy
			zs = zs[:0]
			for _, i := range cells[a*n+b] {
				tr := &tris[i]
				// Where the line through (x, y) crosses the triangle, by its
				// barycentric coordinates in the plane z = 0.
				d := (tr.B.Y-tr.C.Y)*(tr.A.X-tr.C.X) + (tr.C.X-tr.B.X)*(tr.A.Y-tr.C.Y)
				if d == 0 {
					continue
				}
				l1 := ((tr.B.Y-tr.C.Y)*(x-tr.C.X) + (tr.C.X-tr.B.X)*(y-tr.C.Y)) / d
				l2 := ((tr.C.Y-tr.A.Y)*(x-tr.C.X) + (tr.A.X-tr.C.X)*(y-tr.C.Y)) / d
				l3 := 1 - l1 - l2
				if l1 < 0 || l2 < 0 || l3 < 0 {
					continue
				}
				zs = append(zs, l1*tr.A.Z+l2*tr.B.Z+l3*tr.C.Z)
			}
			slices.Sort(zs)
			zs = slices.CompactFunc(zs, func(p, q float64) bool { return math.Abs(q-p) < 1e-9 })
			lo := zMin(y)
			for i := 0; i+1 < len(zs); i += 2 {
				if a, b := math.Max(zs[i], lo), zs[i+1]; b > a {
					vol += (b - a) * dx * dy
				}
			}
		}
	}
	return vol
}

// tangentOracle is what a blend of size r on the x band's edge on the
// cone adds where its top, at top with sag, is carried on past the
// band's side by its tangent plane there: the same blend of the band
// with a flat top at 30, which ends on the end face as no run-off does,
// less what of it lies above that plane (meshAbove). Past the band's
// side and the end face's plane nothing but the blend lies above it.
func tangentOracle(chamfer bool, top, sag, r float64) (float64, error) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	d.Edit.SetMaxSegments(200)
	edge, before := bandOnCone(d, 3, 30)
	g, err := blendOrRefusal(d, edge, r, chamfer)
	if err != nil {
		return 0, err
	}
	rep := g.Analyze().Check()
	if !rep.Sound() {
		return 0, fmt.Errorf("the band with a flat top: %s", rep.Summary())
	}
	// The arc's centre lies c under the band's edge at y = 3.
	c := (9 - sag*sag) / (2 * sag)
	above := meshAbove(g, 6, 12, 3, 9, 600, func(y float64) float64 { return top - 3*(y-3)/c })
	return rep.Volume - before - above, nil
}

// pastRoundTopRefused are the blends of TestChamferRunsOffPastRoundTop
// that are refused, and what the refusal says: a fillet whose end runs
// off the band's side only just, 0.014 along the top's edge, which the
// tangent plane's face would hold as a sliver of no area.
var pastRoundTopRefused = map[string]string{
	"fillet, top 22 sag 2 r 2.5": "only just runs off",
}

// Chamfers and fillets of the x band's edge on the cone
// (curvedBandOnCone) whose end runs off past the band's round top, at
// three heights, three sags and five sizes. Carried on as its own
// circle does, the top comes round under the band again, and the end
// section of a wide blend, or of most at a sag of 2, crossed it
// otherwise than an end that runs off does: refused as running off the
// faces on both sides, 37 of the 90 before 2026-11-05 (the chamfers
// of 2.5 at 20 and 21 of TestFilletRunsOffOntoCylinder among them).
// There the top is carried on by its tangent plane along the band's
// side, a flat face that is one face of the solid with the top (topo's
// tangentIfPast): every blend of the grid but one is sound and adds,
// within 0.1% (0.06% measured), what the same blend of a band with a
// flat top at 30 adds less what of it lies above that plane, by
// vertical lines through a grid of 600 by 600 (meshAbove). The blends
// the top's circle carried on takes are within that too: by the band's
// side the circle lies under its tangent by less than the mesh tells.
func TestChamferRunsOffPastRoundTop(t *testing.T) {
	t.Parallel()
	type key struct {
		chamfer     bool
		top, sag, r float64
	}
	var keys []key
	for _, chamfer := range []bool{true, false} {
		for _, top := range []float64{20, 21, 22} {
			for _, sag := range []float64{0.5, 1, 2} {
				for _, r := range []float64{2, 2.25, 2.5, 2.75, 3} {
					keys = append(keys, key{chamfer, top, sag, r})
				}
			}
		}
	}
	if testing.Short() || fixtures.RaceDetector {
		// Two seconds a blend, four under the race detector.
		keys = []key{{true, 20, 1, 2.5}, {false, 22, 2, 2.5}}
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
			edge, before := curvedBandOnCone(d, "x", 3, k.top, k.sag)
			g, err := blendOrRefusal(d, edge, k.r, k.chamfer)
			if want, ok := pastRoundTopRefused[name]; ok {
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
			added := rep.Volume - before
			want, err := tangentOracle(k.chamfer, k.top, k.sag, k.r)
			if err != nil {
				t.Fatalf("oracle: %v", err)
			}
			if math.Abs(added-want) > 0.001*want {
				t.Errorf("added %g, the oracle %g", added, want)
			}
		})
	}
}
