package srf

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

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
	capsAt0, seams, err := w.capPatches(sbls, h0, h1)
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

	s0s, s1s := make([]SSurface, len(capsAt0)), make([]SSurface, len(capsAt0))
	for i := range capsAt0 {
		s0s[i] = w.liftedSurface(capsAt0[i], h0)
		s1s[i] = w.liftedSurface(capsAt0[i], h1)
		w.faceCap(&s0s[i], bottomUp)
		w.faceCap(&s1s[i], !bottomUp)
		for _, c := range []*SSurface{&s0s[i], &s1s[i]} {
			c.Color = color
			c.TrimBuilt = true
		}
	}
	// Every curve split at the seams, wrapped and lifted, before
	// anything is added.
	loops := sbls.loops()
	pieces := make([][]wrapPiece, len(loops))
	for i, l := range loops {
		for bi := range l.Curves {
			for _, part := range w.splitAtSeams(&l.Curves[bi], seams) {
				lo, hi, err := w.bandCurves(&part, h0, h1)
				if err != nil {
					return err
				}
				_, al, _ := w.polar(part.Finish())
				pieces[i] = append(pieces[i], wrapPiece{
					lo: lo, hi: hi, patch: w.patchOf(part.PointAt(0.5), seams), along: al,
				})
			}
		}
	}
	hs0s, hs1s := make([]core.HSSurface, len(s0s)), make([]core.HSSurface, len(s1s))
	for i := range s0s {
		hs0s[i] = s.addAndAssignSurfaceID(s0s[i])
		hs1s[i] = s.addAndAssignSurfaceID(s1s[i])
	}

	for li := range loops {
		var trimLines []shellTrimLine
		for _, pc := range pieces[li] {
			hs0, hs1 := hs0s[pc.patch], hs1s[pc.patch]
			for bi := range pc.lo {
				b0, b1 := pc.lo[bi], pc.hi[bi]

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
	return s.addWrapSeams(pieces, len(seams), hs0s, hs1s)
}

// wrapPiece is a part of a curve of a wrapped loop between seams: its
// curves lifted to the two heights, the cap patch it lies on, and how
// far along the face it ends.
type wrapPiece struct {
	lo, hi []SBezier
	patch  int
	along  float64
}

// wrapCrossing is where a loop crosses a seam: its ends there at the two
// heights, how far along the face, and the patches of the loop before
// and after.
type wrapCrossing struct {
	lo, hi        geom.Vector
	along         float64
	before, after int
}

// addWrapSeams trims the caps' patches along the seams between them,
// where the loops' insides cross. Along each seam the loops cross in
// pairs, the inside between; of each pair, the loop runs from the patch
// before the seam into the next at one and back at the other. The
// patch before runs along the seam from where its loop leaves it to
// where its loop comes back, as the loop runs, and the next patch the
// other way.
func (s *SShell) addWrapSeams(pieces [][]wrapPiece, nSeams int, hs0s, hs1s []core.HSSurface) error {
	at := make([][]wrapCrossing, nSeams)
	for _, loop := range pieces {
		for i, pc := range loop {
			next := loop[(i+1)%len(loop)]
			if next.patch == pc.patch {
				continue
			}
			if d := next.patch - pc.patch; d != 1 && d != -1 {
				return errors.New("a curve of the wrapped profile crosses two seams of its caps at once")
			}
			k := min(pc.patch, next.patch)
			at[k] = append(at[k], wrapCrossing{
				lo: pc.lo[len(pc.lo)-1].Finish(), hi: pc.hi[len(pc.hi)-1].Finish(),
				along: pc.along, before: pc.patch, after: next.patch,
			})
		}
	}
	for k, cs := range at {
		slices.SortFunc(cs, func(a, b wrapCrossing) int { return cmp.Compare(a.along, b.along) })
		if len(cs)%2 != 0 {
			return errors.New("the wrapped profile crosses a seam of its caps an odd number of times")
		}
		for i := 0; i < len(cs); i += 2 {
			from, to := cs[i], cs[i+1]
			if from.before != k {
				from, to = to, from
			}
			if from.before != k || to.after != k {
				return errors.New("the wrapped profile's crossings of a seam of its caps don't pair up")
			}
			// Along the seam, at each height: forwards on patch k's cap
			// as its loop's curves run, backwards on patch k+1's.
			for _, sd := range []struct {
				a, b    geom.Vector
				hs      []core.HSSurface
				reverse bool
			}{{from.lo, to.lo, hs0s, false}, {from.hi, to.hi, hs1s, true}} {
				line := SBezierFrom2(sd.a, sd.b)
				hc := s.addAndAssignCurveID(SCurve{
					IsExact: true,
					Exact:   line,
					Points:  makePwlSCurvePoints(&line, s.Tol, 0),
					SurfA:   sd.hs[k],
					SurfB:   sd.hs[k+1],
				})
				appendTrim(s, sd.hs[k], STrimByEntireCurveFromShell(s, hc, sd.reverse))
				appendTrim(s, sd.hs[k+1], STrimByEntireCurveFromShell(s, hc, !sd.reverse))
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
