package topo_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// noGroup names no faces, for the shells the tests turn.
type noGroup struct{}

func (noGroup) Remap(core.HEntity, int) core.HEntity { return core.NoEntity }
func (noGroup) EntityExists(core.HEntity) bool       { return false }

// meridian is a closed profile in the XZ plane, (x, z) points joined by
// lines, with an arc about a centre where arcs says so: arcs[i], when
// set, is the centre of the arc from point i to point i+1, a quarter
// turn at most. Its loops are found at tol.
func meridian(t *testing.T, pts [][2]float64, arcs map[int][2]float64, tol srf.Tolerance) *srf.SBezierLoopSetSet {
	t.Helper()
	var sbl srf.SBezierList
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		a, b := v(p[0], 0, p[1]), v(q[0], 0, q[1])
		c, ok := arcs[i]
		if !ok {
			sbl.Curves = append(sbl.Curves, srf.SBezierFrom2(a, b))
			continue
		}
		ctr := v(c[0], 0, c[1])
		ra, rb := a.Minus(ctr), b.Minus(ctr)
		half := math.Acos(ra.Dot(rb)/(ra.Magnitude()*rb.Magnitude())) / 2
		w := math.Cos(half)
		mid := ra.Plus(rb).WithMagnitude(ra.Magnitude() / w)
		sb := srf.SBezierFrom3(a, ctr.Plus(mid), b)
		sb.Weight[1] = w
		sbl.Curves = append(sbl.Curves, sb)
	}
	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	allClosed, allCoplanar := false, false
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, tol, &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	if !allClosed || len(sets.Sets) == 0 {
		t.Fatal("the profile does not close")
	}
	return &sets
}

// turned is the solid of the profile turned about the Z axis from
// start to end radians, or a whole turn when whole, at tol: its curves
// take their points then.
func turned(profile *srf.SBezierLoopSetSet, start, end float64, tol srf.Tolerance) (*srf.SShell, topo.Revolution) {
	rev := topo.Revolution{Profile: profile, Pt: v(0, 0, 0), Axis: v(0, 0, 1), Start: start, End: end}
	sh := &srf.SShell{Tol: tol}
	for i := range profile.Sets {
		if rev.Whole() {
			sh.MakeFromRevolutionOf(&profile.Sets[i], rev.Pt, rev.Axis, core.NewRGBA(100, 100, 100, 255), noGroup{})
		} else {
			sh.MakeFromHelicalRevolutionOf(&profile.Sets[i], rev.Pt, rev.Axis, core.NewRGBA(100, 100, 100, 255), noGroup{},
				start, end, 0, 0)
		}
	}
	return sh, rev
}

// facesOn are the faces whose vertices all lie on the plane through p
// of normal n.
func facesOn(t *testing.T, tp *topo.Topology, p, n geom.Vector) []int {
	t.Helper()
	var out []int
	for f := range tp.Faces {
		ok := true
		for _, l := range tp.Faces[f].Loops {
			for _, ce := range tp.Loops[l].CoEdges {
				if math.Abs(tp.Vertices[tp.Start(ce)].P.Minus(p).Dot(n)) > 1e-9 {
					ok = false
				}
			}
		}
		if ok {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no face on the plane at %v", p)
	}
	return out
}

// quadrantArea is the area of the disc of radius r with x and y both
// at least d.
func quadrantArea(r, d float64) float64 {
	prim := func(x float64) float64 { return (x*math.Sqrt(r*r-x*x) + r*r*math.Asin(x/r)) / 2 }
	top := math.Sqrt(r*r - d*d)
	return prim(top) - prim(d) - d*(top-d)
}

// strip is the area under the circle of radius r from 0 to d: the part
// of a quarter disc within d of one of its straight sides.
func strip(r, d float64) float64 { return (d*math.Sqrt(r*r-d*d) + r*r*math.Asin(d/r)) / 2 }

// Solids of revolution hollowed: the profile offset into its material,
// turned again and taken away. Volumes by Pappus or closed forms.
func TestShellRevolved(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	cone := func(r, h float64) float64 { return math.Pi * r * r * h / 3 }
	// The cone's slant moved in by 1: 8x + 6z = 48 - 10.
	rIn, hIn := 38.0/8, 38.0/6
	cases := []struct {
		name       string
		pts        [][2]float64
		arcs       map[int][2]float64
		start, end float64
		open       [][2]geom.Vector // a point and normal of each plane whose faces are taken away
		round      [][2]float64     // (r, z) of each curved face taken away, all round
		thick      float64
		want       float64
	}{
		{"cup", [][2]float64{{0, 0}, {5, 0}, {5, 10}, {0, 10}}, nil, 0, 2 * math.Pi,
			[][2]geom.Vector{{v(0, 0, 10), v(0, 0, 1)}}, nil, 1, math.Pi * (25*10 - 16*9)},
		{"bowl", [][2]float64{{0, 0}, {10, 0}, {0, 10}}, map[int][2]float64{1: {0, 0}}, 0, 2 * math.Pi,
			[][2]geom.Vector{{v(0, 0, 0), v(0, 0, 1)}}, nil, 2, 2 * math.Pi / 3 * (1000 - 512)},
		{"cone", [][2]float64{{0, 0}, {6, 0}, {0, 8}}, nil, 0, 2 * math.Pi,
			[][2]geom.Vector{{v(0, 0, 0), v(0, 0, 1)}}, nil, 1, cone(6, 8) - cone(rIn, hIn)},
		{"ring", [][2]float64{{5, 0}, {10, 0}, {10, 4}, {5, 4}}, nil, 0, 2 * math.Pi,
			[][2]geom.Vector{{v(0, 0, 4), v(0, 0, 1)}}, nil, 1, math.Pi*(100-25)*4 - math.Pi*(81-36)*3},
		{"ring open outside", [][2]float64{{10, 0}, {24, 0}, {24, 20}, {10, 20}}, nil, 0, 2 * math.Pi,
			nil, [][2]float64{{24, 10}}, 1, math.Pi * ((24*24-10*10)*20 - (24*24-11*11)*18)},
		{"cone open at its side", [][2]float64{{0, 0}, {6, 0}, {0, 8}}, nil, 0, 2 * math.Pi,
			// The inside runs out past the cone, so all of the cone above
			// z = 1 goes: a frustum 1 thick is left.
			nil, [][2]float64{{3, 4}}, 1, cone(6, 8) - cone(5.25, 7)},
		{"quarter, ends kept", [][2]float64{{0, 0}, {5, 0}, {5, 10}, {0, 10}}, nil, 0, math.Pi / 2,
			[][2]geom.Vector{{v(0, 0, 10), v(0, 0, 1)}}, nil, 1, math.Pi*25*10/4 - quadrantArea(4, 1)*9},
		{"quarter, ends open", [][2]float64{{0, 0}, {5, 0}, {5, 10}, {0, 10}}, nil, 0, math.Pi / 2,
			[][2]geom.Vector{{v(0, 0, 10), v(0, 0, 1)}, {v(0, 0, 0), v(0, 1, 0)}, {v(0, 0, 0), v(1, 0, 0)}}, nil, 1,
			math.Pi*25*10/4 - math.Pi*16*9/4},
		{"half a ring, one end kept", [][2]float64{{5, 0}, {10, 0}, {10, 4}, {5, 4}}, nil, 0, math.Pi,
			[][2]geom.Vector{{v(0, 0, 4), v(0, 0, 1)}, {v(1, 0, 0), v(0, 1, 0)}}, nil, 1,
			// The end at x < 0 keeps a wall: the inside's strip 1 wide
			// beside it, 3 high.
			math.Pi*(100-25)*4/2 - (math.Pi*(81-36)*3/2 - 3*(strip(9, 1)-strip(6, 1)))},
	}
	for _, c := range cases {
		sh, rev := turned(meridian(t, c.pts, c.arcs, tol), c.start, c.end, tol)
		tp := build(t, sh)
		var open []int
		for _, o := range c.open {
			for _, f := range facesOn(t, tp, o[0], o[1]) {
				// A plane through the axis holds both ends of a half turn:
				// only the end on the given point's side.
				if o[1].Z == 0 && o[0].Magnitude() > 0 && tp.Vertices[tp.Start(tp.Loops[tp.Faces[f].Loops[0]].CoEdges[0])].P.Dot(o[0]) < -1e-9 {
					continue
				}
				open = append(open, f)
			}
		}
		for _, rz := range c.round {
			for k := range 4 {
				a := math.Pi/4 + float64(k)*math.Pi/2
				f, d := tp.FaceAt(v(rz[0]*math.Cos(a), rz[0]*math.Sin(a), rz[1]))
				if f < 0 || d > 1e-6 {
					t.Fatalf("%s: no face at %v", c.name, rz)
				}
				open = append(open, f)
			}
		}
		out, err := topo.ShellRevolved(sh, rev, open, c.thick, 1<<30)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := sound(t, out); math.Abs(got-c.want) > 0.002*c.want {
			t.Errorf("%s: volume %g, want %g", c.name, got, c.want)
		}
	}
}

// What a revolved shell refuses, and says why.
func TestShellRevolvedRefusals(t *testing.T) {
	t.Parallel()
	rect := [][2]float64{{0, 0}, {5, 0}, {5, 10}, {0, 10}}
	top := [2]geom.Vector{v(0, 0, 10), v(0, 0, 1)}
	cases := []struct {
		name       string
		pts        [][2]float64
		start, end float64
		thick      float64
		want       string
	}{
		{"three quarters, ends kept", rect, 0, 1.5 * math.Pi, 1, "more than half a turn"},
		{"too thick", rect, 0, 2 * math.Pi, 6, "thickness"},
	}
	for _, c := range cases {
		sh, rev := turned(meridian(t, c.pts, nil, srf.Tolerance{}), c.start, c.end, srf.Tolerance{})
		tp := build(t, sh)
		_, err := topo.ShellRevolved(sh, rev, facesOn(t, tp, top[0], top[1]), c.thick, 1<<30)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
	// A curved face taken away between an inside corner and the top.
	step := [][2]float64{{0, 0}, {10, 0}, {10, 5}, {6, 5}, {6, 10}, {0, 10}}
	sh0, rev0 := turned(meridian(t, step, nil, srf.Tolerance{}), 0, 2*math.Pi, srf.Tolerance{})
	tp0 := build(t, sh0)
	var side []int
	for k := range 4 {
		a := math.Pi/4 + float64(k)*math.Pi/2
		f, _ := tp0.FaceAt(v(6*math.Cos(a), 6*math.Sin(a), 8))
		side = append(side, f)
	}
	if _, err := topo.ShellRevolved(sh0, rev0, side, 1, 1<<30); err == nil || !strings.Contains(err.Error(), "inside corner") {
		t.Errorf("a side at an inside corner: %v", err)
	}
	// A solid that is not the revolution given.
	sh := cylinder(5, 10, srf.Tolerance{})
	_, rev := turned(meridian(t, [][2]float64{{0, 0}, {4, 0}, {4, 10}, {0, 10}}, nil, srf.Tolerance{}), 0, 2*math.Pi, srf.Tolerance{})
	tp := build(t, sh)
	if _, err := topo.ShellRevolved(sh, rev, facesOn(t, tp, top[0], top[1]), 1, 1<<30); !errors.Is(err, topo.ErrNotRevolution) {
		t.Errorf("another solid: %v", err)
	}
}
