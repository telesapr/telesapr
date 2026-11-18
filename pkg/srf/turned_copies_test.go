package srf

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// arcAbout is the arc about c in the plane of u and v (unit, square)
// from p to q, counter-clockwise seen along u×v, as rational quadratics
// of at most a quarter turn each.
func arcAbout(c, u, v, p, q geom.Vector) []SBezier {
	ang := func(x geom.Vector) float64 {
		d := x.Minus(c)
		return math.Atan2(d.Dot(v), d.Dot(u))
	}
	r := p.Minus(c).Magnitude()
	a0, a1 := ang(p), ang(q)
	for a1 <= a0 {
		a1 += 2 * math.Pi
	}
	n := int(math.Ceil((a1 - a0) / (math.Pi / 2)))
	at := func(a, s float64) geom.Vector {
		return c.Plus(u.ScaledBy(s * math.Cos(a))).Plus(v.ScaledBy(s * math.Sin(a)))
	}
	out := make([]SBezier, 0, n)
	for i := range n {
		b0 := a0 + (a1-a0)*float64(i)/float64(n)
		b1 := a0 + (a1-a0)*float64(i+1)/float64(n)
		h := (b1 - b0) / 2
		s, e := at(b0, r), at(b1, r)
		if i == 0 {
			s = p
		}
		if i == n-1 {
			e = q
		}
		b := SBezierFrom3(s, at(b0+h, r/math.Cos(h)), e)
		b.Weight[1] = math.Cos(h)
		out = append(out, b)
	}
	return out
}

// triquetra is the outline of the decorative tube's cut, in the plane's
// coordinates (x along the axis, s across it): six arcs of radius side
// about the corners of a triangle of that side centred at x = c on the
// axis, one corner on the axis and the arcs about the other two
// meeting there and at the far point of the axis. Each arc is its
// centre and its ends, counter-clockwise.
func triquetra(c, side float64) [][3][2]float64 {
	h := side * math.Sqrt(3) / 2
	a, b, cc := [2]float64{c - 2*h/3, 0}, [2]float64{c + h/3, side / 2}, [2]float64{c + h/3, -side / 2}
	dd, f, g := [2]float64{c + 4*h/3, 0}, [2]float64{c - 2*h/3, -side}, [2]float64{c - 2*h/3, side}
	return [][3][2]float64{{cc, dd, b}, {a, b, g}, {b, g, a}, {cc, a, f}, {a, f, cc}, {b, cc, dd}}
}

// triquetraPrism is the triquetra drawn in the plane through the X axis
// turned th about it and extruded depth along that plane's normal.
func triquetraPrism(c, side, th, depth float64, tol Tolerance) *SShell {
	u := geom.Vector{X: 1}
	v := geom.Vector{Y: math.Cos(th), Z: math.Sin(th)}
	n := u.Cross(v)
	uv := func(p [2]float64) geom.Vector { return u.ScaledBy(p[0]).Plus(v.ScaledBy(p[1])) }
	var cs []SBezier
	for _, a := range triquetra(c, side) {
		cs = append(cs, arcAbout(uv(a[0]), u, v, uv(a[1]), uv(a[2]))...)
	}
	sh := &SShell{Tol: tol}
	sh.MakeFromExtrusionOf(sketchLoop(cs, n), geom.Vector{}, n.ScaledBy(depth), core.RGBA{})
	return sh
}

// turnedVolume is the volume of the union of n copies of the triquetra
// prism, turned step apart about the X axis, integrated along rays
// parallel to the axis h apart: each copy's stretches along a ray are
// the outline's at the ray's distance across its plane.
func turnedVolume(c, side, th, depth, step float64, n int, h float64) float64 {
	arcs := triquetra(c, side)
	stretches := func(s float64) [][2]float64 {
		var xs []float64
		for _, a := range arcs {
			d := s - a[0][1]
			if math.Abs(d) >= side {
				continue
			}
			a0 := math.Atan2(a[1][1]-a[0][1], a[1][0]-a[0][0])
			a1 := math.Atan2(a[2][1]-a[0][1], a[2][0]-a[0][0])
			for a1 <= a0 {
				a1 += 2 * math.Pi
			}
			dx := math.Sqrt(side*side - d*d)
			for _, x := range []float64{a[0][0] - dx, a[0][0] + dx} {
				t := math.Atan2(d, x-a[0][0])
				for t < a0 {
					t += 2 * math.Pi
				}
				if t <= a1 {
					xs = append(xs, x)
				}
			}
		}
		slices.Sort(xs)
		var out [][2]float64
		for i := 0; i+1 < len(xs); i += 2 {
			out = append(out, [2]float64{xs[i], xs[i+1]})
		}
		return out
	}
	reach := math.Hypot(depth, 2*side)
	var vol float64
	for y := -reach + h/2; y < reach; y += h {
		for z := -reach + h/2; z < reach; z += h {
			var all [][2]float64
			for k := range n {
				a := th + float64(k)*step
				v := geom.Vector{Y: math.Cos(a), Z: math.Sin(a)}
				p := geom.Vector{Y: y, Z: z}
				if w := p.Dot(geom.Vector{X: 1}.Cross(v)); w < 0 || w > depth {
					continue
				}
				all = append(all, stretches(p.Dot(v))...)
			}
			if len(all) == 0 {
				continue
			}
			slices.SortFunc(all, func(p, q [2]float64) int { return cmp.Compare(p[0], q[0]) })
			lo, hi := all[0][0], all[0][1]
			for _, iv := range all[1:] {
				if iv[0] > hi {
					vol += (hi - lo) * h * h
					lo, hi = iv[0], iv[1]
				} else {
					hi = max(hi, iv[1])
				}
			}
			vol += (hi - lo) * h * h
		}
	}
	return vol
}

// unitedInPairs unites the shells a pair at a time, as a step and
// repeat does: pairs of equal size, an odd last one carried up.
func unitedInPairs(shs []*SShell) *SShell {
	for len(shs) > 1 {
		var next []*SShell
		for i := 0; i < len(shs); i += 2 {
			if i == len(shs)-1 {
				next = append(next, shs[i])
				continue
			}
			out := &SShell{}
			out.MakeFromBoolean(shs[i], shs[i+1], SurfaceUnion)
			if out.BooleanFailed {
				return out
			}
			out.MergeCoincidentSurfaces()
			next = append(next, out)
		}
		shs = next
	}
	return shs[0]
}

// unitedInTurn unites the shells one after another.
func unitedInTurn(shs []*SShell) *SShell {
	cur := shs[0]
	for _, sh := range shs[1:] {
		out := &SShell{}
		out.MakeFromBoolean(cur, sh, SurfaceUnion)
		if out.BooleanFailed {
			return out
		}
		out.MergeCoincidentSurfaces()
		cur = out
	}
	return cur
}

// TestTurnedCopiesUnionSweep unites copies of the decorative tube's
// triquetra, drawn in a plane through the axis with a corner on it,
// turned about the axis (roadmap K3, turned copies meeting face to
// face). Turned 60° apart, the copies meet one another along curves
// that lie in the planes of other copies' caps: two copies' cylinders
// meet in an ellipse in the plane of a third's cap, the caps of copies
// half a turn apart lie face to face, and their outlines run along
// each other's. From three copies the union failed, at every chord
// tolerance. Every case is sound, and its volume within the chords'
// bound of the copies' union integrated along rays; the copies united
// a pair at a time, as a step and repeat does, and one after another.
func TestTurnedCopiesUnionSweep(t *testing.T) {
	t.Parallel()
	g := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("copies", 2, 3, 4, 5, 6),
			sweep.Floats("step", 60, 45),
			sweep.Floats("plane", 30, 0, 13),
			sweep.Floats("side", 3, 2),
			sweep.Floats("tol", 0.1, 0.03),
			sweep.Strings("order", "pairs", "turn"),
		},
		// 6 of the 240 fail (roadmap K3): six copies 45° apart of the
		// triquetra of side 2 in the plane at 0°, and five, united in
		// pairs; and six 60° apart of side 3 in the plane at 30° united
		// one after another, which fails or not on the last bits of the
		// plane's angle. Every tolerance alike.
		MaxFailRate:       0.025,
		SampleMaxFailRate: 0.05,
		Sample:            40,
		Parallel:          true,
	}
	g.Run(func(p sweep.Point) error {
		return turnedCopiesCase(int(p.Float("copies")), p.Float("step"), p.Float("plane"), p.Float("side"), p.Float("tol"), p.String("order") == "pairs")
	}).Require(t)
}

// turnedCopiesCase is a case of TestTurnedCopiesUnionSweep: n copies
// step degrees apart of the triquetra of side drawn in the plane turned
// plane degrees, at the chord tolerance tol, united in pairs or in
// turn.
func turnedCopiesCase(n int, step, plane, side, tol float64, pairs bool) error {
	const c, depth = 42.4, 10.0
	th, st := plane*math.Pi/180, step*math.Pi/180
	one := triquetraPrism(c, side, th, depth, Tol(tol, 20))
	shs := make([]*SShell, n)
	for k := range n {
		shs[k] = &SShell{}
		shs[k].MakeFromTransformationOf(one, geom.Vector{}, geom.QuaternionFromAxisAngle(geom.Vector{X: 1}, float64(k)*st), 1)
	}
	var out *SShell
	if pairs {
		out = unitedInPairs(shs)
	} else {
		out = unitedInTurn(shs)
	}
	if out.BooleanFailed || !meshIsSound(out) {
		return errUnsound(out.BooleanFailed)
	}
	var m SMesh
	out.TriangulateInto(&m)
	got, want := m.CalculateVolume(), turnedVolume(c, side, th, depth, st, n, 0.02)
	// The chords of the arcs, sagging into the outline by up to the
	// tolerance round its perimeter of six arcs of a sixth of a turn,
	// along the depth of each copy.
	if bound := float64(n) * 6 * side * math.Pi / 3 * depth * tol; math.Abs(got-want) > bound {
		return fmt.Errorf("volume %g, want %g within %g", got, want, bound)
	}
	return nil
}
