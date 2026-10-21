package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A sweep along a general path (roadmap E10) moves a profile along a
// chain of curves of any kind in 3d, keeping its attitude to the path by
// rotation-minimising frames: the frame turns with the path's tangent
// and no more, so a profile swept along a helix doesn't spin about it.
// The swept surface is no B-spline, so it is fitted: the profile is
// placed at sections along the path and skinned (srf.Skin), and
// sections are added between those where the fitted surface strays from
// the true sweep by more than a tolerance.

// PathSweepOptions are how the frames turn beyond the path's tangent.
type PathSweepOptions struct {
	// Twist turns the profile about the path by this many radians over
	// its whole length, evenly by arc length.
	Twist float64
	// Up, when not zero, holds the frame's second axis to this
	// direction, as far as the tangent allows, rather than minimising
	// its rotation: a profile swept along a helix with Up its axis keeps
	// facing the axis. The tangent must never lie along it.
	Up geom.Vector
	// Tol is how far the fitted surface may stray from the true sweep;
	// zero is a tenth of the chord tolerance.
	Tol float64
	// MaxSections bounds the sections; zero is 400.
	MaxSections int
	// Closed takes a path that ends where it starts, meeting itself
	// tangent there: the profile is swept round to where it began, with
	// no caps. Rotation-minimising frames carried round a loop come back
	// turned about the tangent by an angle the loop's shape decides (its
	// holonomy), so that turn is taken back evenly along the path, on
	// top of Twist, which must then be whole turns.
	Closed bool
}

// frame is a point of the path with a unit tangent and a unit normal
// across it; the third axis is t x r.
type frame struct {
	s    float64 // the chain's parameter, piece i over [i, i+1]
	p    geom.Vector
	t, r geom.Vector
	arc  float64 // arc length from the start
}

// SweepPath is a chain of curves with its frames, sampled densely by
// double reflection (Wang, Juttler, Zheng and Liu, 2008).
type SweepPath struct {
	tol     srf.Tolerance
	pieces  []srf.SBezier
	samples []frame
	opts    PathSweepOptions
	length  float64
	// closure is the turn about the tangent, in radians, spread along a
	// closed path so that its frames meet where it closes.
	closure float64
}

// framesPerPiece are the frames sampled on each piece of a path; a
// frame between them is reflected on from the one before.
const framesPerPiece = 128

// NewSweepPath reads a chain of curves joined end to end as a path,
// with its first frame's normal r0, which must cross the first tangent.
func NewSweepPath(pieces []srf.SBezier, r0 geom.Vector, opts PathSweepOptions) (*SweepPath, error) {
	if len(pieces) == 0 {
		return nil, errors.New("there is no path")
	}
	for i := 1; i < len(pieces); i++ {
		if !pieces[i-1].Finish().Equals(pieces[i].Start()) {
			return nil, fmt.Errorf("the path breaks at %v", pieces[i-1].Finish())
		}
	}
	if opts.Closed && !pieces[len(pieces)-1].Finish().Equals(pieces[0].Start()) {
		return nil, fmt.Errorf("the path doesn't close: it ends at %v", pieces[len(pieces)-1].Finish())
	}
	if opts.Closed {
		if k := opts.Twist / (2 * math.Pi); math.Abs(k-math.Round(k)) > 1e-9 {
			return nil, errors.New("a closed path's twist must be whole turns, or the profile doesn't meet itself")
		}
	}
	sp := &SweepPath{pieces: pieces, opts: opts}
	t0 := sp.tangent(0)
	if t0.Magnitude() == 0 {
		return nil, errors.New("the path has no direction at its start")
	}
	if opts.Up.Magnitude() > 0 {
		r0 = opts.Up
	}
	r0 = r0.Minus(t0.ScaledBy(r0.Dot(t0)))
	if r0.Magnitude() < 1e-9 {
		if opts.Up.Magnitude() > 0 {
			return nil, errors.New("the path starts along the direction the frames are held to")
		}
		return nil, errors.New("the frame's normal lies along the path's start")
	}
	f := frame{p: pieces[0].Start(), t: t0, r: r0.WithMagnitude(1)}
	sp.samples = append(sp.samples, f)
	n := len(pieces) * framesPerPiece
	for k := 1; k <= n; k++ {
		s := float64(k) / framesPerPiece
		next, err := sp.step(f, s)
		if err != nil {
			return nil, err
		}
		f = next
		sp.samples = append(sp.samples, f)
	}
	sp.length = f.arc
	if opts.Closed {
		t1 := sp.tangent(sp.End())
		if t1.Dot(t0) < 1-1e-6 {
			return nil, errors.New("the path closes at a corner: a closed sweep's path meets itself tangent")
		}
		// The angle from the first frame's normal to the last's, about
		// the tangent, taken back.
		r1 := f.r.Minus(t0.ScaledBy(f.r.Dot(t0)))
		sp.closure = -math.Atan2(sp.samples[0].r.Cross(r1).Dot(t0), sp.samples[0].r.Dot(r1))
	}
	return sp, nil
}

// at is the piece and its own parameter at the chain's parameter s.
func (sp *SweepPath) at(s float64) (*srf.SBezier, float64) {
	i := min(int(math.Floor(s)), len(sp.pieces)-1)
	i = max(i, 0)
	return &sp.pieces[i], s - float64(i)
}

// point is the path's point at s.
func (sp *SweepPath) point(s float64) geom.Vector {
	b, t := sp.at(s)
	return b.PointAt(t)
}

// tangent is the path's unit tangent at s.
func (sp *SweepPath) tangent(s float64) geom.Vector {
	b, t := sp.at(s)
	d := b.TangentAt(t)
	if d.Magnitude() < 1e-12 {
		// A control point on an end: the tangent is the limit inside.
		const e = 1e-6
		if t < 0.5 {
			d = b.PointAt(t + e).Minus(b.PointAt(t))
		} else {
			d = b.PointAt(t).Minus(b.PointAt(t - e))
		}
	}
	if d.Magnitude() == 0 {
		return d
	}
	return d.WithMagnitude(1)
}

// step is the frame at s reflected on from f, before s.
func (sp *SweepPath) step(f frame, s float64) (frame, error) {
	x1, t1 := sp.point(s), sp.tangent(s)
	if t1.Magnitude() == 0 {
		return f, fmt.Errorf("the path has no direction at %v", x1)
	}
	out := frame{s: s, p: x1, t: t1, r: f.r}
	// The arc length by chords through the step's middle.
	mid := sp.point((f.s + s) / 2)
	out.arc = f.arc + (mid.Minus(f.p).Magnitude() + x1.Minus(mid).Magnitude())
	if sp.opts.Up.Magnitude() > 0 {
		r := sp.opts.Up.Minus(t1.ScaledBy(sp.opts.Up.Dot(t1)))
		if r.Magnitude() < 1e-9 {
			return f, fmt.Errorf("the path runs along the direction the frames are held to, at %v", x1)
		}
		out.r = r.WithMagnitude(1)
		return out, nil
	}
	v1 := x1.Minus(f.p)
	if c1 := v1.Dot(v1); c1 > 1e-24 {
		rl := f.r.Minus(v1.ScaledBy(2 / c1 * v1.Dot(f.r)))
		tl := f.t.Minus(v1.ScaledBy(2 / c1 * v1.Dot(f.t)))
		v2 := t1.Minus(tl)
		if c2 := v2.Dot(v2); c2 > 1e-24 {
			rl = rl.Minus(v2.ScaledBy(2 / c2 * v2.Dot(rl)))
		}
		out.r = rl
	}
	// Kept exactly across the tangent, and of unit length.
	out.r = out.r.Minus(t1.ScaledBy(out.r.Dot(t1))).WithMagnitude(1)
	return out, nil
}

// Length is the path's arc length.
func (sp *SweepPath) Length() float64 { return sp.length }

// End is the chain's last parameter.
func (sp *SweepPath) End() float64 { return float64(len(sp.pieces)) }

// frameAt is the frame at s, twisted as the options say.
func (sp *SweepPath) frameAt(s float64) frame {
	s = math.Max(0, math.Min(s, sp.End()))
	k := min(int(math.Floor(s*framesPerPiece)), len(sp.samples)-1)
	f := sp.samples[k]
	if s > f.s {
		if g, err := sp.step(f, s); err == nil {
			f = g
		}
	}
	if tw := sp.opts.Twist + sp.closure; tw != 0 && sp.length > 0 {
		a := tw * f.arc / sp.length
		b := f.t.Cross(f.r)
		f.r = f.r.ScaledBy(math.Cos(a)).Plus(b.ScaledBy(math.Sin(a)))
	}
	return f
}

// Motion is the rigid motion that takes the profile from the path's
// start to s: the start's frame onto the frame at s.
func (sp *SweepPath) Motion(s float64) func(geom.Vector) geom.Vector {
	f0, f := sp.frameAt(0), sp.frameAt(s)
	b0, b := f0.t.Cross(f0.r), f.t.Cross(f.r)
	return func(p geom.Vector) geom.Vector {
		d := p.Minus(f0.p)
		x, y, z := d.Dot(f0.t), d.Dot(f0.r), d.Dot(b0)
		return f.p.Plus(f.t.ScaledBy(x)).Plus(f.r.ScaledBy(y)).Plus(b.ScaledBy(z))
	}
}

// Rigid is Motion(s) as a rotation and a translation.
func (sp *SweepPath) Rigid(s float64) srf.Rigid {
	m := sp.Motion(s)
	o := m(geom.Vector{})
	u := m(geom.Vector{X: 1}).Minus(o)
	v := m(geom.Vector{Y: 1}).Minus(o)
	return srf.Rigid{Q: geom.QuaternionFromBasis(u, v), T: o}
}

// SweptSurfaces are the surfaces a profile, an outer loop and its
// holes, sweeps along the path: for each loop, one for each piece of the
// path, u along the loop, v along the piece. Pieces meet with their
// tangents but not their curvatures, which one surface across them all
// would smooth over. The outer loop runs counter-clockwise about the
// path's start, the holes the other way, so that each surface faces out
// of the solid. The sections' parameters along the path come back too,
// those of each piece.
func (sp *SweepPath) SweptSurfaces(loops [][]srf.SBezier) ([][]*srf.BSplineSurface, [][]float64, error) {
	if len(loops) == 0 {
		return nil, nil, errors.New("there is no profile")
	}
	t0 := sp.tangent(0)
	var curves []srf.BSplineCurve
	for i, l := range loops {
		if err := checkLoop(l); err != nil {
			return nil, nil, fmt.Errorf("profile loop %d: %w", i+1, err)
		}
		n := newell(l)
		if math.Abs(n.WithMagnitude(1).Dot(t0)) < 1e-3 {
			return nil, nil, errors.New("the path runs along the profile's plane: it must leave it")
		}
		if (n.Dot(t0) < 0) == (i == 0) {
			l = reverseLoop(l)
		}
		deg := 1
		for _, sb := range l {
			deg = max(deg, sb.Degree)
		}
		c, err := srf.ChainCurve(l, deg)
		if err != nil {
			return nil, nil, err
		}
		curves = append(curves, c)
	}
	tol := sp.opts.Tol
	if tol <= 0 {
		tol = sp.tol.ChordTolMm() / 10
	}
	maxSections := sp.opts.MaxSections
	if maxSections <= 0 {
		maxSections = 400
	}
	out := make([][]*srf.BSplineSurface, len(curves))
	var all [][]float64
	total := 0
	for i := range sp.pieces {
		// Four sections a piece to begin with.
		var ss []float64
		for k := range 5 {
			ss = append(ss, float64(i)+float64(k)/4)
		}
		for {
			surfs, ts, err := sp.skins(curves, ss)
			if err != nil {
				return nil, nil, err
			}
			var next []float64
			split := 0
			for k := 0; k+1 < len(ss); k++ {
				next = append(next, ss[k])
				if sp.strays(curves, surfs, ss[k], ss[k+1], ts[k], ts[k+1], tol) {
					next = append(next, (ss[k]+ss[k+1])/2)
					split++
				}
			}
			next = append(next, ss[len(ss)-1])
			if split == 0 {
				for li := range surfs {
					out[li] = append(out[li], surfs[li])
				}
				all = append(all, ss)
				total += len(ss)
				break
			}
			if total+len(next) > maxSections {
				return nil, nil, fmt.Errorf("the sweep needs more than %d sections to stay within %g mm of the path's: "+
					"the path may bend too tightly for the profile", maxSections, tol)
			}
			ss = next
		}
	}
	return out, all, nil
}

// skins are the surfaces through the profile's loops placed at the
// sections ss of one piece of the path, its ends among them, and the
// sections' parameters across, which all of them share: the path's
// own, from 0 at the piece's start to 1 at its end. Each surface starts
// and ends along the true sweep's derivative there, so that the pieces'
// surfaces meet tangent where the true sweep is smooth.
func (sp *SweepPath) skins(curves []srf.BSplineCurve, ss []float64) ([]*srf.BSplineSurface, []float64, error) {
	s0, s1 := ss[0], ss[len(ss)-1]
	ts := make([]float64, len(ss))
	for k, s := range ss {
		ts[k] = (s - s0) / (s1 - s0)
	}
	ts[0], ts[len(ts)-1] = 0, 1
	motions := make([]func(geom.Vector) geom.Vector, len(ss))
	for k, s := range ss {
		motions[k] = sp.Motion(s)
		if sp.opts.Closed && s == sp.End() {
			// Round a closed path the section is the first one exactly,
			// so that the surfaces meet there.
			motions[k] = sp.Motion(0)
		}
	}
	// The derivatives at the ends by one-sided differences inside the
	// piece, of second order, scaled to the parameter across.
	const h = 1e-5
	fwd := []func(geom.Vector) geom.Vector{motions[0], sp.Motion(s0 + h), sp.Motion(s0 + 2*h)}
	back := []func(geom.Vector) geom.Vector{motions[len(ss)-1], sp.Motion(s1 - h), sp.Motion(s1 - 2*h)}
	deriv := func(m []func(geom.Vector) geom.Vector, p geom.Vector, sign float64) geom.Vector {
		d := m[0](p).ScaledBy(-3).Plus(m[1](p).ScaledBy(4)).Minus(m[2](p))
		return d.ScaledBy(sign * (s1 - s0) / (2 * h))
	}
	var out []*srf.BSplineSurface
	for _, c := range curves {
		secs := make([]srf.BSplineCurve, len(ss))
		for k := range ss {
			sc := c
			sc.Ctrl = make([]geom.Vector, len(c.Ctrl))
			for i, p := range c.Ctrl {
				sc.Ctrl[i] = motions[k](p)
			}
			secs[k] = sc
		}
		start := make([]geom.Vector, len(c.Ctrl))
		end := make([]geom.Vector, len(c.Ctrl))
		for i, p := range c.Ctrl {
			start[i], end[i] = deriv(fwd, p, 1), deriv(back, p, -1)
		}
		bs, err := srf.SkinWithEnds(secs, ts, start, end)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, bs)
	}
	return out, ts, nil
}

// strays reports whether the fitted surfaces stray from the true sweep
// by more than tol between sections at sa and sb, across at ta and tb:
// points of the profile placed at three frames between are measured to
// the surface's curve across through the same profile point, the
// nearest point found by golden section.
func (sp *SweepPath) strays(curves []srf.BSplineCurve, surfs []*srf.BSplineSurface, sa, sb, ta, tb, tol float64) bool {
	for _, f := range []float64{0.25, 0.5, 0.75} {
		m := sp.Motion(sa + f*(sb-sa))
		for ci, c := range curves {
			n := len(c.Breaks()) - 1
			for k := range 6 * n {
				u := (float64(k) + 0.5) / float64(6*n)
				want := m(c.PointAt(u))
				dist := func(v float64) float64 { return surfs[ci].PointAt(u, v).Minus(want).Magnitude() }
				lo, hi := ta, tb
				const g = 0.6180339887498949
				x1, x2 := hi-g*(hi-lo), lo+g*(hi-lo)
				d1, d2 := dist(x1), dist(x2)
				for it := 0; it < 40 && hi-lo > 1e-12; it++ {
					if d1 < d2 {
						hi, x2, d2 = x2, x1, d1
						x1 = hi - g*(hi-lo)
						d1 = dist(x1)
					} else {
						lo, x1, d1 = x1, x2, d2
						x2 = lo + g*(hi-lo)
						d2 = dist(x2)
					}
				}
				if math.Min(d1, d2) > tol {
					return true
				}
			}
		}
	}
	return false
}

// MakePathSweep is the shell of a profile, an outer loop and its holes
// (loops), swept along the path: one face for each loop, of B-spline
// patches (SweptSurfaces), and the profile's planes at both ends. The
// start cap takes identity startFace, the end endFace, and the surfaces'
// patches identities from origin on. Along a closed path there are no
// caps, and startFace and endFace go unused.
func MakePathSweep(sp *SweepPath, loops [][]srf.SBezier, color core.RGBA, startFace, endFace, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	sp.tol = tol
	surfs, _, err := sp.SweptSurfaces(loops)
	if err != nil {
		return nil, err
	}
	if sp.opts.Closed {
		return joinSkins(surfs, color, origin, tol)
	}
	return closeSkins(surfs, color, startFace, endFace, origin, tol)
}

// joinSkins is the solid bounded by chains of surfaces closed in u and
// closed in v too, each chain's last surface ending on the loop its
// first starts from: a closed sweep's, with no caps.
func joinSkins(chains [][]*srf.BSplineSurface, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	sh := &srf.SShell{Tol: tol}
	fa := NewFaceAdder(sh)
	id := origin
	for _, chain := range chains {
		sheet := id
		for _, bs := range chain {
			hs, err := fa.AddBSplineFace(bs, id, sheet, color)
			if err != nil {
				return nil, err
			}
			id += uint32(len(hs) * len(hs[0]))
		}
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the surface is not closed: %s", probs[0].Msg)
	}
	return sh, nil
}

// closeSkins is the solid bounded by chains of surfaces closed in u,
// each surface from a loop at v = 0 to one at v = 1 where the next
// starts, the first chain the outer, and the planes of the chains' first
// and last loops at both ends. Each chain is one face of its own sheet,
// its patches' identities on from origin; the caps take startFace and
// endFace.
func closeSkins(chains [][]*srf.BSplineSurface, color core.RGBA, startFace, endFace, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	sh := &srf.SShell{Tol: tol}
	fa := NewFaceAdder(sh)
	var first, last [][]srf.SBezier
	id := origin
	for _, chain := range chains {
		sheet := id
		for j, bs := range chain {
			if _, err := fa.AddBSplineFace(bs, id, sheet, color); err != nil {
				return nil, err
			}
			ps, err := bs.Patches()
			if err != nil {
				return nil, err
			}
			id += uint32(len(ps) * len(ps[0]))
			// The surface's normal is along u crossed with v, out of the
			// solid. Its patches run its first loop's edges with u
			// falling, so the cap runs them with u, and the last loop's
			// the other way.
			if j == 0 {
				var f []srf.SBezier
				for a := range ps {
					f = append(f, ps[a][0].EdgeCurve(0))
				}
				first = append(first, f)
			}
			if j == len(chain)-1 {
				var l []srf.SBezier
				for a := range ps {
					l = append(l, ps[a][len(ps[a])-1].EdgeCurve(2))
				}
				last = append(last, reverseLoop(l))
			}
		}
	}
	if _, err := fa.AddPlanarFace(first, startFace, color); err != nil {
		return nil, fmt.Errorf("the first profile's cap: %w", err)
	}
	if _, err := fa.AddPlanarFace(last, endFace, color); err != nil {
		return nil, fmt.Errorf("the last profile's cap: %w", err)
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the surface is not closed: %s", probs[0].Msg)
	}
	return sh, nil
}
