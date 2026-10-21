package srf

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// MakeFromWrapOf adds to s the loop set sbls wrapped onto w's face and
// raised between the heights h0 and h1 along the face's outward normal.
// It is the extrusion MakeFromExtrusionOf makes along the sketch's
// normal, every piece mapped by the wrap: the caps are the face's
// surface lifted to h0 and h1, trimmed by the wrapped loops lifted
// alike, and each wrapped curve sweeps a ruled side between its two
// lifts. The wrap keeps orientation and is the identity at the facing
// line, so the extrusion's choice of which way each cap faces and each
// trim runs carries over.
func (s *SShell) MakeFromWrapOf(sbls *SBezierLoopSet, w *Wrap, h0, h1 float64, color core.RGBA) error {
	if sbls == nil {
		return nil
	}
	if w.LiftFactor(h0) <= 1e-6 || w.LiftFactor(h1) <= 1e-6 {
		return fmt.Errorf("a depth of %g reaches the face's axis", max(-h0, -h1))
	}
	capAt0, err := w.capPatch(sbls, max(math.Abs(h0), math.Abs(h1)))
	if err != nil {
		return err
	}
	// The extrusion runs along the sketch's normal; the wrap carries the
	// direction sigma*n, outward from the face, to its outward normal.
	along := w.n.ScaledBy(w.Face.Outward)
	if (h0-h1)*along.Dot(sbls.Normal) < 0 {
		h0, h1 = h1, h0
	}
	// The flat extrusion's bottom cap faces as FromPlane(u, v) does, its
	// top the other way; the wrapped caps face as those directions are carried.
	nn := sbls.Normal.ScaledBy(-1)
	flat := FromPlane(geom.Vector{}, nn.NormalDir(0), nn.NormalDir(1))
	bottomUp := flat.NormalAt(0.5, 0.5).Dot(along) > 0

	s0 := w.liftedSurface(capAt0, h0)
	s1 := w.liftedSurface(capAt0, h1)
	w.faceCap(&s0, bottomUp)
	w.faceCap(&s1, !bottomUp)
	for _, c := range []*SSurface{&s0, &s1} {
		c.Color = color
		c.TrimBuilt = true
	}
	// Every curve wrapped and lifted, before anything is added.
	loops := sbls.loops()
	los, his := make([][]SBezier, len(loops)), make([][]SBezier, len(loops))
	for i, l := range loops {
		for bi := range l.Curves {
			lo, hi, err := w.bandCurves(&l.Curves[bi], h0, h1)
			if err != nil {
				return err
			}
			los[i] = append(los[i], lo...)
			his[i] = append(his[i], hi...)
		}
	}
	hs0 := s.addAndAssignSurfaceID(s0)
	hs1 := s.addAndAssignSurfaceID(s1)

	for li := range loops {
		var trimLines []shellTrimLine
		for bi := range los[li] {
			b0, b1 := los[li][bi], his[li][bi]

			ss := ruledBetween(&b0, &b1)
			ss.Color = color
			ss.TrimBuilt = true
			hsext := s.addAndAssignSurfaceID(ss)

			hc0 := s.addAndAssignCurveID(SCurve{
				IsExact: true,
				Exact:   b0,
				Points:  makePwlSCurvePoints(&b0, s.Tol, 0),
				SurfA:   hs0,
				SurfB:   hsext,
			})
			hc1 := s.addAndAssignCurveID(SCurve{
				IsExact: true,
				Exact:   b1,
				Points:  makePwlSCurvePoints(&b1, s.Tol, 0),
				SurfA:   hs1,
				SurfB:   hsext,
			})
			appendTrim(s, hs0, STrimByEntireCurveFromShell(s, hc0, false))
			appendTrim(s, hs1, STrimByEntireCurveFromShell(s, hc1, true))
			appendTrim(s, hsext, STrimByEntireCurveFromShell(s, hc0, true))
			appendTrim(s, hsext, STrimByEntireCurveFromShell(s, hc1, false))

			lineBez := SBezierFrom2(b0.Finish(), b1.Finish())
			hl := s.addAndAssignCurveID(SCurve{
				IsExact: true,
				Exact:   lineBez,
				Points:  makePwlSCurvePoints(&lineBez, s.Tol, 0),
			})
			trimLines = append(trimLines, shellTrimLine{hc: hl, hs: hsext})
		}

		nl := len(trimLines)
		for i := range nl {
			tl := trimLines[i]
			tlp := trimLines[(i-1+nl)%nl]
			appendTrim(s, tl.hs, STrimByEntireCurveFromShell(s, tl.hc, true))
			appendTrim(s, tl.hs, STrimByEntireCurveFromShell(s, tlp.hc, false))
			if c := s.FindCurve(tl.hc); c != nil {
				c.SurfA = tl.hs
			}
			if c := s.FindCurve(tlp.hc); c != nil {
				c.SurfB = tl.hs
			}
		}
	}
	return nil
}

// faceCap turns the cap c to face away from the axis's side of the face
// (along the face's outward normal) when up, and towards it otherwise.
func (w *Wrap) faceCap(c *SSurface, up bool) {
	p := c.PointAt(0.5, 0.5)
	want := w.Face.normalAt(p).ScaledBy(w.Face.Outward)
	if !up {
		want = want.ScaledBy(-1)
	}
	if c.NormalAt(0.5, 0.5).Dot(want) < 0 {
		c.Reverse()
	}
}

// ruledBetween is the patch ruled from a to b, curves of one degree and
// weights; a at v = 0, as FromExtrusionOf's profile at t0.
func ruledBetween(a, b *SBezier) SSurface {
	s := SSurface{DegM: a.Degree, DegN: 1, Sheet: a.Sheet}
	for i := 0; i <= s.DegM; i++ {
		s.Ctrl[i][0] = a.Ctrl[i]
		s.Ctrl[i][1] = b.Ctrl[i]
		s.Weight[i][0] = a.Weight[i]
		s.Weight[i][1] = a.Weight[i]
	}
	return s
}
