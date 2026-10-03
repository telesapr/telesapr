package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A drafted prism is an extrusion whose sides lean into its material,
// as a moulded part's do so that it comes out of the mould (roadmap
// 15.4, E13). The profile's plane is the neutral one: the sides pass
// through the profile there, and the far end's outline is the profile
// offset into the material by the length times the angle's tangent, as
// a shell offsets an end (shellprism.go). A line's side is a plane, an
// arc's a cone about the arc's axis. Each piece of the profile is joined
// to its offset copy by a ruled patch, and the ends close it, face by
// face, with no Boolean. Where an arc meets another piece at a sharp
// corner, the sides meet in a curve no ruled patch follows, so the
// arcs must meet their neighbours tangent; lines meet at any angle.

// DraftedPrism is the solid of a profile, loops in one plane square to
// along, the first the outer one and the rest holes, extruded by along
// with its sides leaning in by angle radians (out when negative). The
// faces get color and origins from origin up, 0 leaving them to the
// caller.
func DraftedPrism(loops [][]srf.SBezier, along geom.Vector, angle float64, color core.RgbaColor, origin uint32) (*srf.SShell, error) {
	h := along.Magnitude()
	if h < geom.LengthEps {
		return nil, errors.New("the extrusion has no length")
	}
	if math.IsNaN(angle) || math.Abs(angle) >= math.Pi/2-1e-6 {
		return nil, fmt.Errorf("a draft of %.6g° leans the sides flat", angle*180/math.Pi)
	}
	if len(loops) == 0 {
		return nil, errors.New("the profile has no loops")
	}
	n := along.ScaledBy(1 / h)
	d := h * math.Tan(angle)
	what := fmt.Sprintf("a draft of %.6g° over %.6g", angle*180/math.Pi, h)
	var bottom, top [][]srf.SBezier
	for k, loop := range loops {
		if err := checkLoop(loop); err != nil {
			return nil, err
		}
		nl := newell(loop)
		if nl.Magnitude() == 0 || nl.WithMagnitude(1).Cross(n).Magnitude() > 1e-6 {
			return nil, errors.New("the profile does not lie square to the extrusion")
		}
		// The outer loop counter-clockwise about along, the holes
		// clockwise: the material lies on the left of each.
		if (nl.Dot(n) > 0) != (k == 0) {
			loop = reverseLoop(loop)
		}
		pieces, err := loopPieces(loop, n)
		if err != nil {
			return nil, err
		}
		keep, corner, err := offsetProfile(pieces, n, d, what)
		if err != nil {
			return nil, err
		}
		if len(keep) != len(pieces) {
			return nil, shellErr(pieces[0].a, "%s takes an arc of the profile away", what)
		}
		var b, t []srf.SBezier
		for i, p := range pieces {
			parts := 1
			if p.arc {
				parts = quarters(p.angle)
			}
			lo, err := movedCurves(p, p.a, p.b, n, parts)
			if err != nil {
				return nil, err
			}
			hi, err := movedCurves(p, corner[i], corner[(i+1)%len(pieces)], n, parts)
			if err != nil {
				return nil, shellErr(p.a, "%s makes the sides meet, closing up the far end", what)
			}
			b = append(b, lo...)
			for _, c := range hi {
				t = append(t, translated(c, along))
			}
		}
		bottom, top = append(bottom, b), append(top, t)
	}

	sh := &srf.SShell{}
	next := origin
	add := func(s srf.SSurface, loops [][]srf.SBezier) error {
		s.Color = color
		s.Origin = next
		if origin != 0 {
			next++
		}
		_, err := AddFace(sh, s, loops)
		return err
	}
	// The near end faces away from along, its outer loop clockwise
	// about its outward normal; AddFace keeps a first face's loops as
	// given.
	if err := add(capPlane(bottom[0], n.ScaledBy(-1)), bottom); err != nil {
		return nil, err
	}
	for k := range bottom {
		a, b := bottom[k], top[k]
		for i := range a {
			seamStart := srf.SBezierFrom2(b[i].Start(), a[i].Start())
			seamEnd := srf.SBezierFrom2(a[i].Finish(), b[i].Finish())
			rb := b[i]
			rb.Reverse()
			if err := add(ruledPatch(a[i], b[i]), [][]srf.SBezier{{a[i], seamEnd, rb, seamStart}}); err != nil {
				return nil, err
			}
		}
	}
	var far [][]srf.SBezier
	for _, l := range top {
		far = append(far, reverseLoop(l))
	}
	if err := add(capPlane(top[0], n), far); err != nil {
		return nil, err
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the drafted solid is not closed: %s", probs[0].Msg)
	}
	return sh, nil
}

// loopPieces are the pieces of a loop in the plane of normal n, lines
// and arcs, its material on the left of each. An arc must meet its
// neighbours tangent.
func loopPieces(loop []srf.SBezier, n geom.Vector) ([]profilePiece, error) {
	var pieces []profilePiece
	for _, sb := range loop {
		a, m, b := sb.Start(), sb.PointAt(0.5), sb.Finish()
		p := profilePiece{a: a, b: b}
		if sb.Degree == 1 || m.DistanceToLine(a, b.Minus(a)) < geom.LengthEps {
			p.into = n.Cross(b.Minus(a)).WithMagnitude(1)
			pieces = append(pieces, p)
			continue
		}
		c, ok := circleThrough(a, m, b, n)
		r := c.Minus(a).Magnitude()
		if !ok || sb.Degree != 2 || math.Abs(sb.PointAt(0.25).Minus(c).Magnitude()-r) > geom.LengthEps {
			return nil, shellErr(m, "a curve of the profile is neither a line nor an arc; a draft leans lines and arcs")
		}
		p.arc, p.centre, p.r = true, c, r
		ra, rb := a.Minus(c), b.Minus(c)
		p.angle = math.Atan2(ra.Cross(rb).Dot(n), ra.Dot(rb))
		if rm := m.Minus(c); math.Signbit(ra.Cross(rm).Dot(n)) != math.Signbit(p.angle) {
			p.angle -= math.Copysign(2*math.Pi, p.angle)
		}
		// Left of the way along at the middle.
		radial := m.Minus(c).WithMagnitude(1)
		left := n.Cross(pieceTangentAt(p, radial, n))
		p.outward = left.Dot(radial) > 0
		p.into = radial
		if !p.outward {
			p.into = radial.ScaledBy(-1)
		}
		pieces = append(pieces, p)
	}
	for i, q := range pieces {
		p := pieces[(i+len(pieces)-1)%len(pieces)]
		if !p.arc && !q.arc {
			continue
		}
		tp, tq := pieceTangent(p, false, n), pieceTangent(q, true, n)
		if tp.Cross(tq).Magnitude() > 1e-6 || tp.Dot(tq) < 0 {
			return nil, shellErr(q.a, "an arc of the profile meets its neighbour at a sharp corner; "+
				"a draft takes arcs that meet their neighbours tangent")
		}
	}
	return pieces, nil
}

// pieceTangentAt is an arc's direction of travel where its radial
// direction is radial.
func pieceTangentAt(p profilePiece, radial, n geom.Vector) geom.Vector {
	t := n.WithMagnitude(1).Cross(radial)
	if p.angle < 0 {
		t = t.ScaledBy(-1)
	}
	return t
}

// translated is sb moved by by.
func translated(sb srf.SBezier, by geom.Vector) srf.SBezier {
	for i := 0; i <= sb.Degree; i++ {
		sb.Ctrl[i] = sb.Ctrl[i].Plus(by)
	}
	return sb
}
