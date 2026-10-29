package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A path may turn at a sharp corner between two straight pieces. Along
// a straight piece with no twist the frames don't turn, so the sweep
// there is a prism, and two prisms meet at a corner in the plane
// halving it, as a rigid sweep's lines are mitred: the frame on past the
// corner is the one before turned about the corner's axis by its angle
// (the double reflection does that across the tangent's jump), so both
// sides' sections, carried along their lines onto that plane, are the
// one curve, and the pieces' surfaces share it with no Boolean between
// them. A twist turns the frames along a line, which makes its sides no
// prism and the two sides' cuts by the plane different curves, so it is
// held still beside a corner (pathsweep_hold.go).

// pathCornerCos is the cosine of the least turn between two pieces'
// tangents that is a corner rather than a smooth join, as the sketch's.
const pathCornerCos = 1 - 1.5e-6

// Errors for a corner the path can't be mitred at.
var (
	// ErrPathCornerCurved is a corner with a piece beside it that is not
	// straight.
	ErrPathCornerCurved = errors.New("a corner of the path has a curved piece beside it")
	// ErrPathCornerTwisted is a corner on a twisted path, or a closed one
	// whose frames are turned to meet round it, where the twist can't be
	// held still beside it: no reach is given, or every piece is beside
	// a corner, leaving nowhere to twist.
	ErrPathCornerTwisted = errors.New("a corner of the path turns frames that are twisted along it")
	// ErrPathCornerHeld is a corner where frames held to a direction
	// turn otherwise than the corner turns the path.
	ErrPathCornerHeld = errors.New("the frames held to a direction turn otherwise than the path at a corner")
	// ErrPathCornerShort is a straight piece too short for the mitres at
	// its ends: they cross on it.
	ErrPathCornerShort = errors.New("a straight piece of the path is too short for the mitres at its ends")
)

// straightPiece reports whether sb is a line: its control points lie on the
// chord between its ends.
func straightPiece(sb *srf.SBezier) bool {
	a, b := sb.Start(), sb.Finish()
	d := b.Minus(a)
	l := d.Magnitude()
	if l == 0 {
		return false
	}
	d = d.ScaledBy(1 / l)
	for _, p := range sb.Ctrl[1:sb.Degree] {
		q := p.Minus(a)
		if q.Minus(d.ScaledBy(q.Dot(d))).Magnitude() > 1e-9*math.Max(1, l) {
			return false
		}
	}
	return true
}

// tangentInto is piece i's unit tangent at its end.
func (sp *SweepPath) tangentInto(i int) geom.Vector {
	return sp.tangent(float64(i) + 1 - 1e-9)
}

// findCorners marks the joins between pieces where the path turns
// sharply, and the straight pieces beside them, which are swept as
// prisms cut by the corners' mitres.
func (sp *SweepPath) findCorners() error {
	n := len(sp.pieces)
	sp.corner = make([]bool, n+1)
	sp.prism = make([]bool, n)
	sp.rigid = make([]bool, n)
	found := false
	for k := 1; k < n; k++ {
		d1, d2 := sp.tangentInto(k-1), sp.tangent(float64(k))
		if d1.Dot(d2) >= pathCornerCos {
			continue
		}
		at := sp.pieces[k].Start()
		sa, sb := straightPiece(&sp.pieces[k-1]), straightPiece(&sp.pieces[k])
		_, _, _, aa := arcOf(&sp.pieces[k-1])
		_, _, _, ab := arcOf(&sp.pieces[k])
		switch {
		case sa && sb:
			sp.prism[k-1], sp.prism[k] = true, true
		case (sa || aa) && (sb || ab):
			// Beside an arc: joined as a rigid sweep is (pathsweep_joint.go).
			sp.rigid[k-1], sp.rigid[k] = true, true
		default:
			return &srf.CornerError{At: at, Err: ErrPathCornerCurved}
		}
		sp.corner[k], found = true, true
	}
	if !found {
		return nil
	}
	sp.growRigid()
	for k := 1; k < n; k++ {
		if !sp.corner[k] || sp.rigid[k] {
			continue
		}
		// The frame on past the corner, against the one before it turned
		// by the corner. A twist is held still beside the corner
		// (holdTwist), so the frames before it are compared.
		d1, d2 := sp.tangentInto(k-1), sp.tangent(float64(k))
		before, after := sp.untwistedAt(float64(k)-0.5), sp.untwistedAt(float64(k))
		axis := d1.Cross(d2).WithMagnitude(1)
		turned := before.r.RotatedAroundAxis(axis, math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2)))))
		if turned.Minus(after.r).Magnitude() > 1e-7 {
			return &srf.CornerError{At: sp.pieces[k].Start(), Err: ErrPathCornerHeld}
		}
	}
	return sp.checkRigid()
}

// mitred is p, on piece i's line, carried along it onto the plane
// halving the corner at boundary k, k = i or i+1.
func (sp *SweepPath) mitred(p geom.Vector, i, k int) geom.Vector {
	d1, d2 := sp.tangentInto(k-1), sp.tangent(float64(k))
	m := d1.Plus(d2).WithMagnitude(1)
	c := sp.pieces[k].Start()
	d := d2
	if k == i+1 {
		d = d1
	}
	return p.Plus(d.ScaledBy(c.Minus(p).Dot(m) / d.Dot(m)))
}

// prismSkins are the surfaces through the profile's loops along the
// straight piece i, beside a corner at one end or both: ruled from the
// section at its start to the one at its end, each moved by the piece's
// frame, which doesn't turn along it, and carried onto a corner's mitre.
// The sections' parameters across are 0 and 1.
func (sp *SweepPath) prismSkins(curves []srf.BSplineCurve, i int) ([]*srf.BSplineSurface, []float64, error) {
	m := sp.Motion(float64(i))
	along := sp.pieces[i].Finish().Minus(sp.pieces[i].Start())
	dir := along.WithMagnitude(1)
	var out []*srf.BSplineSurface
	for _, c := range curves {
		s0, s1 := c, c
		s0.Ctrl = make([]geom.Vector, len(c.Ctrl))
		s1.Ctrl = make([]geom.Vector, len(c.Ctrl))
		der := make([]geom.Vector, len(c.Ctrl))
		for j, p := range c.Ctrl {
			a := m(p)
			b := a.Plus(along)
			if sp.corner[i] {
				a = sp.mitred(a, i, i)
			}
			if sp.corner[i+1] {
				b = sp.mitred(b, i, i+1)
			}
			if b.Minus(a).Dot(dir) <= geom.LengthEps {
				at := sp.pieces[i].Start()
				if sp.corner[i+1] {
					at = sp.pieces[i].Finish()
				}
				return nil, nil, &srf.CornerError{At: at, Err: ErrPathCornerShort}
			}
			s0.Ctrl[j], s1.Ctrl[j], der[j] = a, b, b.Minus(a)
		}
		bs, err := srf.SkinWithEnds([]srf.BSplineCurve{s0, s1}, []float64{0, 1}, der, der)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, bs)
	}
	return out, []float64{0, 1}, nil
}
