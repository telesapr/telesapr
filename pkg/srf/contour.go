package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/libm"
)

// SContour is an ordered list of SPoint forming one polygon contour.
// Contours can be open (line strip) or closed (loop). The
// TimesEnclosed field counts how many other contours enclose this
// one (even=outside, odd=hole) when used inside an SPolygon.
type SContour struct {
	Tag           int
	TimesEnclosed int
	XMinPt        geom.Vector
	Points        []SPoint
}

func (c *SContour) Len() int { return len(c.Points) }

// AddPoint appends p with tag 0 (C++ SContour::AddPoint).
func (c *SContour) AddPoint(p geom.Vector) {
	c.Points = append(c.Points, SPoint{P: p})
}

// Reverse reverses the contour direction in place.
func (c *SContour) Reverse() {
	for i, j := 0, len(c.Points)-1; i < j; i, j = i+1, j-1 {
		c.Points[i], c.Points[j] = c.Points[j], c.Points[i]
	}
}

// ComputeNormal returns the unit face normal of the contour's plane
// (best-fit for non-planar contours). Faithful port of C++
// `SContour::ComputeNormal` (polygon.cpp:584): scans every
// consecutive triple of points, normalizing each edge before
// crossing them, and keeps whichever triple's cross product has the
// largest magnitude. This matters because a contour sampled by
// piecewise-linearizing a Bezier loop (MakePwlIntoEdges) often has
// several collinear points in a row along a single straight edge —
// checking only the first triple (as an earlier version of this
// function did) can spuriously return the zero vector even though
// the contour is planar and non-degenerate.
func (c *SContour) ComputeNormal() geom.Vector {
	n := geom.Vector{}
	for i := 0; i+2 < len(c.Points); i++ {
		u := c.Points[i+1].P.Minus(c.Points[i].P).WithMagnitude(1)
		v := c.Points[i+2].P.Minus(c.Points[i+1].P).WithMagnitude(1)
		nt := u.Cross(v)
		if nt.Magnitude() > n.Magnitude() {
			n = nt
		}
	}
	return n.Normal()
}

// SignedAreaProjdToNormal mirrors C++ SContour::SignedAreaProjdToNormal
// (polygon.cpp:611). Contours are stored closed (last point repeats
// the first). Note the C++ sign convention: the sum is negative for a
// loop that winds counterclockwise about n in the right-handed basis
// (n.Normal(0), n.Normal(1)).
func (c *SContour) SignedAreaProjdToNormal(n geom.Vector) float64 {
	// An arbitrary 2d coordinate system that has n as its normal
	u := n.NormalDir(0)
	v := n.NormalDir(1)

	area := 0.0
	for i := 0; i < len(c.Points)-1; i++ {
		u0 := c.Points[i].P.Dot(u)
		v0 := c.Points[i].P.Dot(v)
		u1 := c.Points[i+1].P.Dot(u)
		v1 := c.Points[i+1].P.Dot(v)

		area += ((v0 + v1) / 2) * (u1 - u0)
	}
	return area
}

// IsClockwiseProjdToNormal mirrors C++ SContour::IsClockwiseProjdToNormal
// (polygon.cpp:603), including its sign convention (see
// SignedAreaProjdToNormal).
func (c *SContour) IsClockwiseProjdToNormal(n geom.Vector) bool {
	// Degenerate things might happen as we draw; doesn't really matter
	// what we do then.
	if n.Magnitude() < 0.01 {
		return true
	}
	return c.SignedAreaProjdToNormal(n) < 0
}

// ContainsPointProjdToNormal mirrors C++
// SContour::ContainsPointProjdToNormal (polygon.cpp). The contour must
// be stored exactly closed.
func (c *SContour) ContainsPointProjdToNormal(n, p geom.Vector) bool {
	u := n.NormalDir(0)
	v := n.NormalDir(1)

	up := p.Dot(u)
	vp := p.Dot(v)

	inside := false
	ln := len(c.Points)
	for i := 0; i < ln-1; i++ {
		ua := c.Points[i].P.Dot(u)
		va := c.Points[i].P.Dot(v)
		// The curve needs to be exactly closed; approximation is death.
		ub := c.Points[(i+1)%(ln-1)].P.Dot(u)
		vb := c.Points[(i+1)%(ln-1)].P.Dot(v)

		if (((va <= vp) && (vp < vb)) ||
			((vb <= vp) && (vp < va))) &&
			(up < (ub-ua)*(vp-va)/(vb-va)+ua) {
			inside = !inside
		}
	}

	return inside
}

// AnyEdgeMidpoint returns the midpoint of the first edge.
func (c *SContour) AnyEdgeMidpoint() geom.Vector {
	if len(c.Points) < 2 {
		return geom.Vector{}
	}
	return c.Points[0].P.Plus(c.Points[1].P).ScaledBy(0.5)
}

func wrapMod(a, n int) int {
	r := a % n
	if r < 0 {
		r += n
	}
	return r
}

func containsVec(vs []geom.Vector, v geom.Vector) bool {
	for i := range vs {
		if vs[i].Equals(v) {
			return true
		}
	}
	return false
}

// CopyInto adds the contour's points to dest (C++ SContour::CopyInto).
func (c *SContour) CopyInto(dest *SContour) {
	for _, sp := range c.Points {
		dest.AddPoint(sp.P)
	}
}

// OffsetInto adds to dest the contour, in the xy plane, offset by r:
// outside corners are rounded, inside ones mitred (C++
// SContour::OffsetInto).
func (c *SContour) OffsetInto(dest *SContour, r float64) {
	// C++ computes (1*PI)/180 and (6*PI)/180 in doubles, rounding twice;
	// Go's constant arithmetic would round once.
	pi := math.Pi
	oneDegree := (1 * pi) / 180
	sixDegrees := (6 * pi) / 180
	n := len(c.Points)
	wrap := func(v int) int { return wrapIdx(v, n-1) }
	for i := 0; i < n; i++ {
		a := c.Points[wrap(i-1)].P
		b := c.Points[wrap(i)].P
		cc := c.Points[wrap(i+1)].P

		dp := a.Minus(b)
		thetap := libm.Atan2(dp.Y, dp.X)

		dn := b.Minus(cc)
		thetan := libm.Atan2(dn.Y, dn.X)

		// A short line segment in a badly-generated polygon might look
		// okay but screw up our sense of direction.
		if dp.Magnitude() < geom.LengthEps || dn.Magnitude() < geom.LengthEps {
			continue
		}

		if thetan > thetap && (thetan-thetap) > math.Pi {
			thetap += 2 * math.Pi
		}
		if thetan < thetap && (thetap-thetan) > math.Pi {
			thetan += 2 * math.Pi
		}

		if math.Abs(thetan-thetap) < oneDegree {
			dest.AddPoint(geom.Vector{X: b.X - r*libm.Sin(thetap), Y: b.Y + r*libm.Cos(thetap)})
		} else if thetan < thetap {
			// This is an inside corner. We have two edges, Ep and En. Move
			// out from their intersection by radius, normal to En, and
			// then draw a line parallel to En. Move out from their
			// intersection by radius, normal to Ep, and then draw a second
			// line parallel to Ep. The point that we want to generate is
			// the intersection of these two lines--it removes as much
			// material as we can without removing any that we shouldn't.
			px0 := b.X - r*libm.Sin(thetap)
			py0 := b.Y + r*libm.Cos(thetap)
			pdx := libm.Cos(thetap)
			pdy := libm.Sin(thetap)

			nx0 := b.X - r*libm.Sin(thetan)
			ny0 := b.Y + r*libm.Cos(thetan)
			ndx := libm.Cos(thetan)
			ndy := libm.Sin(thetan)

			x, y := 0.0, 0.0
			intersectionOfLines(px0, py0, pdx, pdy, nx0, ny0, ndx, ndy, &x, &y)

			dest.AddPoint(geom.Vector{X: x, Y: y})
		} else {
			if math.Abs(thetap-thetan) < sixDegrees {
				dest.AddPoint(geom.Vector{X: b.X - r*libm.Sin(thetap), Y: b.Y + r*libm.Cos(thetap)})
				dest.AddPoint(geom.Vector{X: b.X - r*libm.Sin(thetan), Y: b.Y + r*libm.Cos(thetan)})
			} else {
				for theta := thetap; theta <= thetan; theta += sixDegrees {
					dest.AddPoint(geom.Vector{X: b.X - r*libm.Sin(theta), Y: b.Y + r*libm.Cos(theta)})
				}
			}
		}
	}
}

// intersectionOfLines is where the lines (x0A, y0A) + t*(dxA, dyA) and
// (x0B, y0B) + t*(dxB, dyB) cross; false if they're parallel (C++'s
// static IntersectionOfLines in polygon.cpp).
func intersectionOfLines(x0A, y0A, dxA, dyA, x0B, y0B, dxB, dyB float64, xi, yi *float64) bool {
	var A [2][2]float64
	var b [2]float64

	// The line is given to us in the form:
	//    (x(t), y(t)) = (x0, y0) + t*(dx, dy)
	// so first rewrite it as
	//    (x - x0, y - y0) dot (dy, -dx) = 0
	//    x*dy - x0*dy - y*dx + y0*dx = 0
	//    x*dy - y*dx = (x0*dy - y0*dx)

	// So write the matrix, pre-pivoted.
	if math.Abs(dyA) > math.Abs(dyB) {
		A[0][0], A[0][1], b[0] = dyA, -dxA, x0A*dyA-y0A*dxA
		A[1][0], A[1][1], b[1] = dyB, -dxB, x0B*dyB-y0B*dxB
	} else {
		A[1][0], A[1][1], b[1] = dyA, -dxA, x0A*dyA-y0A*dxA
		A[0][0], A[0][1], b[0] = dyB, -dxB, x0B*dyB-y0B*dxB
	}

	// Check the determinant; if it's zero then no solution.
	if math.Abs(A[0][0]*A[1][1]-A[0][1]*A[1][0]) < geom.LengthEps {
		return false
	}

	// Solve
	v := A[1][0] / A[0][0]
	A[1][0] -= A[0][0] * v
	A[1][1] -= A[0][1] * v
	b[1] -= b[0] * v

	// Back-substitute.
	*yi = b[1] / A[1][1]
	*xi = (b[0] - A[0][1]*(*yi)) / A[0][0]

	return true
}

// FindPointWithMinX sets XMinPt to the first point with the least x
// (C++ SContour::FindPointWithMinX).
func (c *SContour) FindPointWithMinX() {
	c.XMinPt = geom.Vector{X: veryPositive, Y: veryPositive, Z: veryPositive}
	for _, sp := range c.Points {
		if sp.P.X < c.XMinPt.X {
			c.XMinPt = sp.P
		}
	}
}

// MakeEdgesInto adds an edge for each pair of consecutive points; the
// contour repeats its first point at the end, so it is closed (C++
// SContour::MakeEdgesInto).
func (c *SContour) MakeEdgesInto(el *SEdgeList) {
	for i := 0; i < len(c.Points)-1; i++ {
		el.AddEdge(c.Points[i].P, c.Points[i+1].P, 0, 0, 0)
	}
}
