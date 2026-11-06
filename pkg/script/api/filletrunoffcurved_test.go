package api

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// curvedBandOnCone is bandOnCone with the band's top a cylinder rising
// sag above the band's edges at height top (a hollow where sag is below
// 0): about an axis along y (kind "y"), its profile the band's in the
// plane y = 0 and the band two-sided 2y wide; or about an axis along x
// (kind "x"), its profile the band's end at x = 6, which the top meets
// in an arc. The band's side on the cone is filleted past its top onto
// that cylinder.
func curvedBandOnCone(d *Doc, kind string, y, top, sag float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(V(0, 0, -2), V(0, 0, 32))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
	d.Features.Lathe(axis, "")
	var band Group
	if kind == "y" {
		d.Features.SketchIn(d.Workplane("ZX"), "")
		// The arc through (6, top), (20, top) and (13, top+sag).
		c := (49 - sag*sag) / (2 * sag)
		d.Sketch.Line(V(6, 0, top), V(6, 0, 0))
		d.Sketch.Line(V(6, 0, 0), V(20, 0, 0))
		d.Sketch.Line(V(20, 0, 0), V(20, 0, top))
		if sag > 0 {
			d.Sketch.Arc(V(13, 0, top-c), V(6, 0, top), V(20, 0, top))
		} else {
			d.Sketch.Arc(V(13, 0, top-c), V(20, 0, top), V(6, 0, top))
		}
		band = d.Features.Extrude(2*y, "band")
		band.SetTwoSided(true)
	} else {
		d.Features.Sketch3D("end")
		d.Features.SketchIn(d.Sketch.NewWorkplane(V(6, 0, 0), V(0, 1, 0), V(0, 0, 1)), "")
		// The arc through (-y, top), (y, top) and (0, top+sag).
		c := (y*y - sag*sag) / (2 * sag)
		d.Sketch.Line(V(6, -y, top), V(6, -y, 0))
		d.Sketch.Line(V(6, -y, 0), V(6, y, 0))
		d.Sketch.Line(V(6, y, 0), V(6, y, top))
		if sag > 0 {
			d.Sketch.Arc(V(6, 0, top-c), V(6, y, top), V(6, -y, top))
		} else {
			d.Sketch.Arc(V(6, 0, top-c), V(6, -y, top), V(6, y, top))
		}
		band = d.Features.Extrude(14, "band")
	}
	return band.Solid().EdgeAt(V(math.Sqrt(64-y*y), y, 10)), band.Analyze().Check().Volume
}

// runOffOracle is a line of testdata/topo/fillet-runoff-curved.txt:
// OpenCASCADE's volumes before and after the fillet, or refused.
type runOffOracle struct {
	kind           string
	y, top, sag, r float64
	before, after  float64
	refused        bool
}

func readRunOffOracle(t *testing.T) []runOffOracle {
	t.Helper()
	f, err := os.Open("../../../testdata/topo/fillet-runoff-curved.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []runOffOracle
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 6 {
			continue
		}
		o := runOffOracle{kind: fs[0], refused: fs[5] == "refused"}
		var x [6]float64
		for i, s := range fs[1:] {
			if i < 4 || !o.refused {
				if x[i], err = strconv.ParseFloat(s, 64); err != nil {
					t.Fatal(err)
				}
			}
		}
		o.y, o.top, o.sag, o.r, o.before, o.after = x[0], x[1], x[2], x[3], x[4], x[5]
		out = append(out, o)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// blendOrRefusal fillets or chamfers edge with size r, giving the group
// or the error that refused it.
func blendOrRefusal(d *Doc, edge SolidEdge, r float64, chamfer bool) (g Group, err error) {
	defer func() {
		if p := recover(); p != nil {
			e, ok := p.(error)
			if !ok {
				panic(p)
			}
			err = e
		}
	}()
	if chamfer {
		g = d.Features.ChamferSolid([]SolidEdge{edge}, r, "")
	} else {
		g = d.Features.FilletSolid([]SolidEdge{edge}, r, "")
	}
	if p := g.Problem(); p != "" {
		return g, errors.New(p)
	}
	return g, nil
}

// runOffRefused are the cases of TestFilletRunsOffOntoCylinder that are
// refused, and what the refusal says: none since 2026-11-05. The x
// band's widest chamfers, whose end sections lay wholly outside the
// band's top carried on as its circle does, were refused as running off
// the faces on both sides; they are made with the top carried on by its
// tangent plane (TestChamferRunsOffPastRoundTop). The chamfers ending
// on the narrowest band's top, the cone going on past it, refused as
// "can't be fitted" before 2026-10-29, are made; so are the fillets
// whose end only just runs off onto the x band's top, refused as "only
// just runs off" before 2026-11-04: their end is taken to the corner of
// the band's side and top (topo's runOffSliver).
var runOffRefused = map[string]string{}

// Fillets and chamfers of the band's edge on the cone whose end meets
// the band's top, a cylinder (roadmap: run-offs onto a curved face). On
// the bands 2.5 and 3 wide at height 22, and on the x bands, the end
// runs off the band's side past the top: the top's surface is carried on
// past its edge with the side, it grows over the end, and the blend is
// cut where it meets that surface; the end face and the top share the
// line where the top's surface meets the end's plane, a ruling of the y
// bands' top, an arc of the x bands'. On the band 2 wide at 18 the edge
// ends on the top itself, with the cone going on past it: the top grows
// the same way, up to where the cone meets its surface carried on.
// Every blend not in runOffRefused is sound; every fillet adds what
// OpenCASCADE's makeFillet adds within 1%, at 200 segments a curve, as
// for the flat top (TestChamferRunsOffBand), and where it refuses (every
// run-off but the y bands' at 18) ours adds no less than at the smaller
// radius. Chamfers are made at one sag each. The blends are made side by
// side and each series compared after. The ordinary run takes the
// oracle's smallest and largest sizes, 1.5 and 2.5, and make
// test-sweeps the 2 between too.
func TestFilletRunsOffOntoCylinder(t *testing.T) {
	t.Parallel()
	cases := readRunOffOracle(t)
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		// One case that runs off onto the cylinder: the grid takes half a
		// minute, ten times that under the race detector.
		cases = slices.DeleteFunc(cases, func(o runOffOracle) bool {
			return o.kind != "y" || o.y != 2.5 || o.sag != 1 || o.r != 2
		})
	} else if !sweep.Full() {
		cases = slices.DeleteFunc(cases, func(o runOffOracle) bool { return o.r == 2 })
	}
	type blend struct {
		series string
		r      float64
		added  float64
		made   bool
	}
	var blends []*blend
	t.Run("blends", func(t *testing.T) {
		for _, o := range cases {
			for _, chamfer := range []bool{false, true} {
				if chamfer && o.sag != 1 {
					continue
				}
				series := fmt.Sprintf("%s band %g wide, top %g sag %g", o.kind, 2*o.y, o.top, o.sag)
				name := fmt.Sprintf("%s r %g", series, o.r)
				if chamfer {
					name, series = "chamfer, "+name, "chamfer, "+series
				}
				b := &blend{series: series, r: o.r}
				blends = append(blends, b)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					d := New()
					d.Edit.SetChordTolerance(0.003)
					d.Edit.SetMaxSegments(200)
					edge, before := curvedBandOnCone(d, o.kind, o.y, o.top, o.sag)
					g, err := blendOrRefusal(d, edge, o.r, chamfer)
					if want, ok := runOffRefused[name]; ok {
						if err == nil || !strings.Contains(err.Error(), want) {
							t.Errorf("%v, want a refusal saying %q", err, want)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					rep := g.Analyze().Check()
					if !rep.Sound() {
						t.Fatal(rep.Summary())
					}
					b.added, b.made = rep.Volume-before, true
					if chamfer || o.refused {
						return
					}
					if occ := o.after - o.before; math.Abs(b.added-occ) > 0.01*occ {
						t.Errorf("added %g, OpenCASCADE's %g", b.added, occ)
					}
				})
			}
		}
	})
	// The oracle's lines rise in size within a series.
	prev := map[string]float64{}
	for _, b := range blends {
		if !b.made {
			continue
		}
		if b.added <= prev[b.series] {
			t.Errorf("%s r %g added %g, no more than at a smaller size, %g", b.series, b.r, b.added, prev[b.series])
		}
		prev[b.series] = b.added
	}
}

// Fillets of the x band's edge on the cone, run off onto its curved top,
// at sizes between the oracle's (TestFilletRunsOffOntoCylinder) and at
// three chord tolerances. Before 2026-11-04 those whose end crosses the
// top within one to two chord tolerances of the band's side were refused
// or made with the sliver the top grew by left open (6 naked edges),
// and some wider ones, 1.725 to 1.8 on the top at 20 and 2.3 on the top
// at 21, were made with 4 naked edges where the curve across the fillet
// on the top was meshed by the two faces 1e-6 apart; and 1.925 on the
// top at 21, whose end crosses the top just short of the band's side,
// was refused as a fillet that takes material away: every one is sound
// now, adding more than the size before. The ordinary run takes each top
// at one of the tolerances, every tolerance once, and make test-sweeps
// all nine.
func TestFilletRunsOffOntoCylinderBetween(t *testing.T) {
	t.Parallel()
	type key struct{ top, sag, ct float64 }
	keys := []key{}
	for i, ts := range [][2]float64{{20, -1}, {20, 1}, {21, 1}} {
		for j, ct := range []float64{0.002, 0.003, 0.005} {
			if i == j || sweep.Full() {
				keys = append(keys, key{ts[0], ts[1], ct})
			}
		}
	}
	rs := []float64{1.475, 1.5, 1.525, 1.725, 1.75, 1.775, 1.8, 1.925, 1.975, 2, 2.025, 2.3}
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		keys, rs = []key{{20, -1, 0.003}}, []float64{1.475, 1.5, 1.725, 1.75}
	}
	for _, k := range keys {
		t.Run(fmt.Sprintf("top %g sag %g at %g%%", k.top, k.sag, k.ct), func(t *testing.T) {
			t.Parallel()
			blendsBetween(t, k.top, k.sag, k.ct, rs)
		})
	}
}

// blendsBetween fillets the x band's edge on the cone, its top at top
// with sag, at chord tolerance ct, at each of the sizes rs, rising, side
// by side: each sound, adding more than the one before.
func blendsBetween(t *testing.T, top, sag, ct float64, rs []float64) {
	t.Helper()
	added := make([]float64, len(rs))
	made := make([]bool, len(rs))
	t.Run("sizes", func(t *testing.T) {
		for i, r := range rs {
			t.Run(fmt.Sprint(r), func(t *testing.T) {
				t.Parallel()
				d := New()
				d.Edit.SetChordTolerance(ct)
				edge, before := curvedBandOnCone(d, "x", 3, top, sag)
				g, err := blendOrRefusal(d, edge, r, false)
				if err != nil {
					t.Fatal(err)
				}
				rep := g.Analyze().Check()
				if !rep.Sound() {
					t.Fatal(rep.Summary())
				}
				added[i], made[i] = rep.Volume-before, true
			})
		}
	})
	prev := 0.0
	for i, r := range rs {
		if !made[i] {
			continue
		}
		if added[i] <= prev {
			t.Errorf("x band, top %g sag %g, %g%%, r %g added %g, no more than at a smaller size, %g", top, sag, ct, r, added[i], prev)
		}
		prev = added[i]
	}
}
