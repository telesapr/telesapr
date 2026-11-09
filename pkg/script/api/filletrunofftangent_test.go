package api

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// meshOver is the triangles of the mesh of the model up to and with
// group g whose boxes meet the box [x0, x1] by [y0, y1] in x and y.
func meshOver(g Group, x0, x1, y0, y1 float64) []srf.STriangle {
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
	return tris
}

// volumeAbove is the volume of a mesh, of which tris are the triangles
// over the box [x0, x1] by [y0, y1] (meshOver), that lies over the box
// and above the height zMin(x, y) there, by vertical lines through the
// centres of n by n cells: each crosses the mesh where it goes in and
// out, so that the lengths inside above zMin add up. The mesh must be
// closed there.
func volumeAbove(tris []srf.STriangle, x0, x1, y0, y1 float64, n int, zMin func(x, y float64) float64) float64 {
	col := meshColumns(tris, x0, x1, y0, y1, n)
	dx, dy := (x1-x0)/float64(n), (y1-y0)/float64(n)
	vol := 0.0
	for a := range n {
		for b := range n {
			x, y := x0+(float64(a)+0.5)*dx, y0+(float64(b)+0.5)*dy
			vol += lengthAbove(col(x, y), zMin(x, y)) * dx * dy
		}
	}
	return vol
}

// lengthAbove is how much of a vertical line, which goes in and out of
// a closed mesh at the heights zs, lies inside it above lo.
func lengthAbove(zs []float64, lo float64) float64 {
	l := 0.0
	for i := 0; i+1 < len(zs); i += 2 {
		if a, b := math.Max(zs[i], lo), zs[i+1]; b > a {
			l += b - a
		}
	}
	return l
}

// meshColumns gives, for a point over the box [x0, x1] by [y0, y1], the
// heights at which the vertical line through it crosses the triangles
// tris, from the lowest up, which it finds by the cell, of n by n, it is in.
func meshColumns(tris []srf.STriangle, x0, x1, y0, y1 float64, n int) func(x, y float64) []float64 {
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
	return func(x, y float64) []float64 {
		a, b := cell(x, x0, dx), cell(y, y0, dy)
		var zs []float64
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
		return slices.CompactFunc(zs, func(p, q float64) bool { return math.Abs(q-p) < 1e-9 })
	}
}

// flatTopBlend is the blend of size r on the x band's edge on the cone
// of a band with a flat top at 30, which ends on the end face as no
// run-off does: the volume it adds and its mesh over the box past the
// band's side and the end face's plane that tangentOracle measures in.
type flatTopBlend struct {
	added float64
	tris  []srf.STriangle
}

// makeFlatTopBlend makes the flatTopBlend of size r, a chamfer or a
// fillet. It depends on neither the top's height nor its sag, so a test
// makes it once for every top and sag (TestChamferRunsOffPastRoundTop).
func makeFlatTopBlend(chamfer bool, r float64) (flatTopBlend, error) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	d.Edit.SetMaxSegments(200)
	edge, before := bandOnCone(d, 3, 30)
	g, err := blendOrRefusal(d, edge, r, chamfer)
	if err != nil {
		return flatTopBlend{}, err
	}
	rep := g.Analyze().Check()
	if !rep.Sound() {
		return flatTopBlend{}, fmt.Errorf("the band with a flat top: %s", rep.Summary())
	}
	return flatTopBlend{rep.Volume - before, meshOver(g, 6, 12, 3, 9)}, nil
}

// tangentOracle is what a blend on the x band's edge on the cone adds
// where its top, at top with sag, is carried on past the band's side by
// its tangent plane there: the same blend of the band with a flat top
// at 30 (flat), less what of it lies above that plane (volumeAbove).
// Past the band's side and the end face's plane nothing but the blend
// lies above it.
func tangentOracle(flat flatTopBlend, top, sag float64) float64 {
	// The arc's centre lies c under the band's edge at y = 3.
	c := (9 - sag*sag) / (2 * sag)
	return flat.added - volumeAbove(flat.tris, 6, 12, 3, 9, 600, func(_, y float64) float64 { return top - 3*(y-3)/c })
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
// vertical lines through a grid of 600 by 600 (tangentOracle). The
// blends the top's circle carried on takes are within that too: by the
// band's side the circle lies under its tangent by less than the mesh
// tells. The blend with the flat top depends only on the kind and the
// size, so it is made once for each of the 10 and measured for every
// top and sag. The ordinary run takes every top and sag at 2.5, where
// most were refused and the one still refused is, and each other size
// at one top and sag a kind, 26 of the 90; make test-sweeps all of them.
func TestChamferRunsOffPastRoundTop(t *testing.T) {
	t.Parallel()
	type key struct {
		chamfer     bool
		top, sag, r float64
	}
	// The top and sag each size but 2.5 is tried at, by kind, in the
	// ordinary run: every top, sag and size met at least once a kind.
	at := map[bool]map[float64][2]float64{
		true:  {2: {20, 0.5}, 2.25: {21, 1}, 2.75: {22, 2}, 3: {21, 2}},
		false: {2: {22, 1}, 2.25: {20, 2}, 2.75: {21, 0.5}, 3: {20, 1}},
	}
	var keys []key
	for _, chamfer := range []bool{true, false} {
		for _, top := range []float64{20, 21, 22} {
			for _, sag := range []float64{0.5, 1, 2} {
				for _, r := range []float64{2, 2.25, 2.5, 2.75, 3} {
					if r == 2.5 || at[chamfer][r] == [2]float64{top, sag} || sweep.Full() {
						keys = append(keys, key{chamfer, top, sag, r})
					}
				}
			}
		}
	}
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		// Two seconds a blend, four under the race detector.
		keys = []key{{true, 20, 1, 2.5}, {false, 22, 2, 2.5}}
	}
	type blend struct {
		chamfer bool
		r       float64
	}
	flat := map[blend]func() (flatTopBlend, error){}
	for _, k := range keys {
		b := blend{k.chamfer, k.r}
		if flat[b] == nil {
			flat[b] = sync.OnceValues(func() (flatTopBlend, error) { return makeFlatTopBlend(b.chamfer, b.r) })
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
			f, err := flat[blend{k.chamfer, k.r}]()
			if err != nil {
				t.Fatalf("oracle: %v", err)
			}
			if want := tangentOracle(f, k.top, k.sag); math.Abs(added-want) > 0.001*want {
				t.Errorf("added %g, the oracle %g", added, want)
			}
		})
	}
}
