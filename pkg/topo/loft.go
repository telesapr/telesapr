package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A ruled loft joins two closed profiles, curve by curve: curve i of
// the first and curve i of the second bound a ruled patch, straight
// across between them, which the kernel's patch holds exactly once the
// two curves have one degree. The profiles' planes close the ends. The
// shell is built face by face (AddFace), so the patches share their
// edges and no Boolean is needed.

// elevate raises a Bezier's degree by one, keeping its curve: in
// homogeneous coordinates each new point is a blend of two old ones.
func elevate(sb srf.SBezier) srf.SBezier {
	n := sb.Degree
	var h [5]geom.Vector
	var w [5]float64
	for i := 0; i <= n; i++ {
		h[i] = sb.Ctrl[i].ScaledBy(sb.Weight[i])
		w[i] = sb.Weight[i]
	}
	out := sb
	out.Degree = n + 1
	for i := 0; i <= n+1; i++ {
		a := float64(i) / float64(n+1)
		var hp geom.Vector
		var wp float64
		if i > 0 {
			hp = hp.Plus(h[i-1].ScaledBy(a))
			wp += w[i-1] * a
		}
		if i <= n {
			hp = hp.Plus(h[i].ScaledBy(1 - a))
			wp += w[i] * (1 - a)
		}
		out.Ctrl[i] = hp.ScaledBy(1 / wp)
		out.Weight[i] = wp
	}
	return out
}

// toDegree raises sb to degree d.
func toDegree(sb srf.SBezier, d int) srf.SBezier {
	for sb.Degree < d {
		sb = elevate(sb)
	}
	return sb
}

// ruledPatch is the surface straight across between curves a and b,
// each point of a joined to the point of b at the same parameter. Two
// lines in one plane make that plane: the kernel takes a patch of
// degree 1 both ways for a plane parametrized affinely, as an extrusion
// makes it, which a trapezoid between two lines of different lengths
// is not, so the face is a plane trimmed to the trapezoid. Otherwise
// the patch's u runs across, from b at u = 0 to a at u = 1, and its v
// along the curves, so that the mesher, which checks the twist of a
// surface of degree 1 in u along v, follows a patch that twists; two
// lines that twist against each other make one of degree 2 along them.
func ruledPatch(a, b srf.SBezier) srf.SSurface {
	d := max(a.Degree, b.Degree)
	if d == 1 {
		if coplanar(a.Start(), a.Finish(), b.Start(), b.Finish()) {
			return planeThrough(a.Start(), a.Finish(), b.Finish(), b.Start())
		}
		d = 2
	}
	a, b = toDegree(a, d), toDegree(b, d)
	s := srf.SSurface{DegM: 1, DegN: d}
	for j := 0; j <= d; j++ {
		s.Ctrl[0][j], s.Weight[0][j] = b.Ctrl[j], b.Weight[j]
		s.Ctrl[1][j], s.Weight[1][j] = a.Ctrl[j], a.Weight[j]
	}
	return s
}

// planeThrough is the plane of the quadrilateral p0 p1 p2 p3, over it.
// AddFace turns it over if the loop it trims runs the other way.
func planeThrough(p0, p1, p2, p3 geom.Vector) srf.SSurface {
	quad := []geom.Vector{p0, p1, p2, p3}
	return planeOver(quad, newellOf(quad))
}

// planeOver is the plane through pts with normal n, its four control
// points the corners of a rectangle about them. The kernel finds where
// surfaces meet by the boxes about their control points, as an
// extrusion's planes lie over their faces, so a plane must cover its
// face.
func planeOver(pts []geom.Vector, n geom.Vector) srf.SSurface {
	n = n.WithMagnitude(1)
	u := n.NormalDir(0).WithMagnitude(1)
	v := n.Cross(u)
	lo := geom.Vector{X: math.Inf(1), Y: math.Inf(1)}
	hi := lo.ScaledBy(-1)
	for _, p := range pts {
		d := p.Minus(pts[0])
		x, y := d.Dot(u), d.Dot(v)
		lo.X, lo.Y = math.Min(lo.X, x), math.Min(lo.Y, y)
		hi.X, hi.Y = math.Max(hi.X, x), math.Max(hi.Y, y)
	}
	o := pts[0].Plus(u.ScaledBy(lo.X)).Plus(v.ScaledBy(lo.Y))
	return srf.FromPlane(o, u.ScaledBy(hi.X-lo.X), v.ScaledBy(hi.Y-lo.Y))
}

// newellOf is the normal of a closed polygon by Newell's formula.
func newellOf(pts []geom.Vector) geom.Vector {
	var n geom.Vector
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		n.X += (a.Y - b.Y) * (a.Z + b.Z)
		n.Y += (a.Z - b.Z) * (a.X + b.X)
		n.Z += (a.X - b.X) * (a.Y + b.Y)
	}
	return n
}

// coplanar reports whether four points lie in one plane, within
// LengthEps.
func coplanar(p0, p1, p2, p3 geom.Vector) bool {
	n := p1.Minus(p0).Cross(p2.Minus(p0))
	if n.Magnitude() < geom.LengthEps {
		n = p1.Minus(p0).Cross(p3.Minus(p0))
	}
	if n.Magnitude() < geom.LengthEps {
		return true
	}
	return math.Abs(p3.Minus(p0).Dot(n.WithMagnitude(1))) < geom.LengthEps &&
		math.Abs(p2.Minus(p0).Dot(n.WithMagnitude(1))) < geom.LengthEps
}

// newell is the normal of a closed chain of curves by Newell's
// formula, through their control points: its direction is the way the
// chain turns counter-clockwise about.
func newell(loop []srf.SBezier) geom.Vector {
	var pts []geom.Vector
	for _, sb := range loop {
		for k := range sb.Degree {
			pts = append(pts, sb.PointAt(float64(k)/float64(sb.Degree)))
		}
	}
	return newellOf(pts)
}

// reverseLoop runs a closed chain of curves the other way.
func reverseLoop(loop []srf.SBezier) []srf.SBezier {
	out := make([]srf.SBezier, len(loop))
	for i := range loop {
		sb := loop[len(loop)-1-i]
		sb.Reverse()
		out[i] = sb
	}
	return out
}

// checkLoop says what is wrong with a chain of curves as a closed loop.
func checkLoop(loop []srf.SBezier) error {
	if len(loop) < 2 {
		return errors.New("a profile needs at least two curves")
	}
	for i, sb := range loop {
		next := loop[(i+1)%len(loop)]
		if !sb.Finish().Equals(next.Start()) {
			return fmt.Errorf("the profile is not closed at %v", sb.Finish())
		}
		if sb.Start().Equals(sb.Finish()) {
			return fmt.Errorf("a curve of the profile starts where it ends, at %v", sb.Start())
		}
	}
	return nil
}

// MakeRuledLoft is the shell of a ruled loft from closed loop a to
// closed loop b, curves of the same count joined in order: curve i of a
// to curve i of b, each running the same way round. The faces get
// origins from origin up and color.
func MakeRuledLoft(a, b []srf.SBezier, color core.RgbaColor, origin uint32) (*srf.SShell, error) {
	if len(a) != len(b) {
		return nil, fmt.Errorf("the profiles have %d and %d curves; a loft joins them one to one", len(a), len(b))
	}
	if err := checkLoop(a); err != nil {
		return nil, err
	}
	if err := checkLoop(b); err != nil {
		return nil, err
	}
	na, nb := newell(a), newell(b)
	if na.Magnitude() == 0 || nb.Magnitude() == 0 {
		return nil, errors.New("a profile encloses no area")
	}
	// Both loops run counter-clockwise about the way from a to b.
	ca, cb := centroid(a), centroid(b)
	along := cb.Minus(ca)
	if along.Magnitude() < geom.LengthEps {
		return nil, errors.New("the profiles lie on each other")
	}
	if na.Dot(along) < 0 {
		a = reverseLoop(a)
	}
	if nb.Dot(along) < 0 {
		b = reverseLoop(b)
	}
	sh := &srf.SShell{}
	next := origin
	add := func(s srf.SSurface, loop []srf.SBezier) error {
		s.Color = color
		s.Origin = next
		next++
		_, err := AddFace(sh, s, [][]srf.SBezier{loop})
		return err
	}
	// The first cap faces away from b, so its loop runs clockwise about
	// its outward normal when it runs counter-clockwise about the way to
	// b; AddFace keeps a first face's loop as given.
	if err := add(capPlane(a, along.ScaledBy(-1)), a); err != nil {
		return nil, err
	}
	for i := range a {
		seamStart := srf.SBezierFrom2(b[i].Start(), a[i].Start())
		seamEnd := srf.SBezierFrom2(a[i].Finish(), b[i].Finish())
		rb := b[i]
		rb.Reverse()
		if err := add(ruledPatch(a[i], b[i]), []srf.SBezier{a[i], seamEnd, rb, seamStart}); err != nil {
			return nil, err
		}
	}
	if err := add(capPlane(b, along), reverseLoop(b)); err != nil {
		return nil, err
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the loft's surface is not closed: %s", probs[0].Msg)
	}
	return sh, nil
}

// centroid is the mean of a loop's curve starts.
func centroid(loop []srf.SBezier) geom.Vector {
	var c geom.Vector
	for _, sb := range loop {
		c = c.Plus(sb.Start())
	}
	return c.ScaledBy(1 / float64(len(loop)))
}

// capPlane is the plane of a loop, over it, its normal along n.
func capPlane(loop []srf.SBezier, n geom.Vector) srf.SSurface {
	nn := newell(loop)
	if nn.Dot(n) < 0 {
		nn = nn.ScaledBy(-1)
	}
	// A curve lies within its control points.
	var pts []geom.Vector
	for _, sb := range loop {
		pts = append(pts, sb.Ctrl[:sb.Degree+1]...)
	}
	return planeOver(pts, nn)
}
