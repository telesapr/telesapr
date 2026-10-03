package step

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A shape read as a solid (roadmap D3), not a mesh: each face's surface
// cut into patches along a grid of its parameters (solidgeom.go), and
// each patch trimmed by the pieces of the face's bounds that cross it,
// joined by the patch's own edges where the bounds leave it. The steps:
//
//   - every edge's curve as Beziers, shared by the faces on both sides;
//   - each face's bounds laid out in its surface's parameters, with
//     the face on their left, and where they cross the grid's lines
//     found on the exact curves;
//   - each edge cut where any face of it crossed a line, so that both
//     faces have its pieces alike;
//   - in each cell of a face's grid, the runs of pieces inside it
//     closed into loops along the cell's sides, turning the way that
//     keeps the face inside; cells no bound reaches are in or out as
//     their neighbours say;
//   - each loop given to the kernel with its patch (topo.FaceAdder),
//     the edges of one face's patches seams of one sheet.
//
// A plane needs no grid: its face is one, bounded by the curves. What
// the importer can't hold exactly, an offset surface or a curve of
// degree above 5 say, makes the shape an error, and the caller falls
// back to the mesh.

// solidUse is an edge as a bound of a face runs along it.
type solidUse struct {
	e       *Edge
	forward bool
}

// solidEdge is an edge's curve as Beziers from its start to its end,
// and where faces cut it.
type solidEdge struct {
	bez  []srf.SBezier
	cuts [][]float64
	// split is bez cut at cuts, each piece's ends shared exactly with
	// its neighbours'.
	split []srf.SBezier
}

// uvSample is a point of a bound with its parameters on the face's
// surface, unwrapped along the bound.
type uvSample struct {
	t    float64 // the Bezier's own parameter
	p    geom.Vector
	u, v float64
}

// solidBuilder builds one shape's shell.
type solidBuilder struct {
	edges map[*Edge]*solidEdge
	// turned are the faces whose bounds the file runs the wrong way
	// round: clockwise about the face, as some writers do.
	turned map[*Face]bool
	sh     *srf.SShell
	fa     *topo.FaceAdder
	next   uint32
}

// ShellOf is shape i of the model as a solid, faces' identities from
// origin. It fails for anything it can't hold exactly or close.
func (m *Model) ShellOf(i int, origin uint32) (*srf.SShell, error) {
	if i < 0 || i >= len(m.Shapes) {
		return nil, fmt.Errorf("no shape %d", i)
	}
	return buildSolid(m.Shapes[i], origin)
}

func buildSolid(shape *Shape, origin uint32) (sh *srf.SShell, err error) {
	defer func() {
		if r := recover(); r != nil {
			sh, err = nil, fmt.Errorf("step: %v", r)
		}
	}()
	if len(shape.Tris) > 0 {
		return nil, errUnsupported("a tessellated shape, triangles with no surfaces")
	}
	b := &solidBuilder{edges: map[*Edge]*solidEdge{}, turned: map[*Face]bool{}, sh: &srf.SShell{}, next: origin}
	b.fa = topo.NewFaceAdder(b.sh)
	shape = withPolygonEdges(shape)
	for _, f := range shape.Faces {
		for _, bd := range f.Bounds {
			for _, oe := range bd.Edges {
				if b.edges[oe.E] != nil {
					continue
				}
				bez, err := edgeBeziers(oe.E)
				if err != nil {
					return nil, err
				}
				b.edges[oe.E] = &solidEdge{bez: bez, cuts: make([][]float64, len(bez))}
			}
		}
	}
	grids := make([]*faceGrid, len(shape.Faces))
	for i, f := range shape.Faces {
		if _, ok := f.Surf.(*plane); ok {
			continue
		}
		g, err := b.layout(f)
		if err != nil {
			return nil, fmt.Errorf("face #%d: %w", f.ID, err)
		}
		grids[i] = g
	}
	for _, se := range b.edges {
		se.splitAtCuts()
	}
	for i, f := range shape.Faces {
		var err error
		if grids[i] == nil {
			err = b.planeFace(f)
		} else {
			err = b.gridFace(f, grids[i])
		}
		if err != nil {
			return nil, fmt.Errorf("face #%d: %w", f.ID, err)
		}
	}
	t, err := topo.Build(b.sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the faces don't close: %s at %v", probs[0].Msg, probs[0].At)
	}
	return b.sh, nil
}

// withPolygonEdges is the shape with each face bounded by polygons, as
// a faceted solid's are, on its polygon's plane and bounded by lines,
// one edge for each side, which the face beyond it shares.
func withPolygonEdges(shape *Shape) *Shape {
	poly := false
	for _, f := range shape.Faces {
		for _, bd := range f.Bounds {
			poly = poly || bd.Poly != nil
		}
	}
	if !poly {
		return shape
	}
	type key struct{ a, b geom.Vector }
	edges := map[key]*Edge{}
	side := func(a, b geom.Vector) OrientedEdge {
		if e, ok := edges[key{b, a}]; ok {
			return OrientedEdge{E: e, Forward: false}
		}
		e := edges[key{a, b}]
		if e == nil {
			e = &Edge{ID: -len(edges) - 1, Start: a, End: b, Curve: &line{p: a, d: b.Minus(a)}, Sense: true}
			edges[key{a, b}] = e
		}
		return OrientedEdge{E: e, Forward: true}
	}
	out := &Shape{ID: shape.ID, Kind: shape.Kind}
	for _, f := range shape.Faces {
		g := f
		if pf, ok := polygonFace(f); ok {
			g = pf
			var bounds []*Bound
			for _, bd := range f.Bounds {
				nb := &Bound{}
				for i, p := range bd.Poly {
					nb.Edges = append(nb.Edges, side(p, bd.Poly[(i+1)%len(bd.Poly)]))
				}
				bounds = append(bounds, nb)
			}
			cp := *g
			cp.Bounds = bounds
			g = &cp
		}
		out.Faces = append(out.Faces, g)
	}
	return out
}

// loopsOf are a face's bounds as uses, each running with the face on
// its left in the surface's parameters: as the file gives them when the
// face's normal is the surface's, else turned round.
func loopsOf(f *Face) [][]solidUse {
	var out [][]solidUse
	for _, bd := range f.Bounds {
		var l []solidUse
		for _, oe := range bd.Edges {
			l = append(l, solidUse{e: oe.E, forward: oe.Forward})
		}
		if !f.Sense {
			for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
				l[i], l[j] = l[j], l[i]
			}
			for i := range l {
				l[i].forward = !l[i].forward
			}
		}
		out = append(out, l)
	}
	return out
}

// loops are the face's bounds as loopsOf gives them, turned round for
// a face whose file runs them the wrong way.
func (b *solidBuilder) loops(f *Face) [][]solidUse {
	ls := loopsOf(f)
	if b.turned[f] {
		for _, l := range ls {
			for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
				l[i], l[j] = l[j], l[i]
			}
			for i := range l {
				l[i].forward = !l[i].forward
			}
		}
	}
	return ls
}

// beziersOf are a use's Beziers the way it runs, from the edge's
// pieces (cut, once they are).
func (b *solidBuilder) beziersOf(u solidUse, split bool) []srf.SBezier {
	se := b.edges[u.e]
	src := se.bez
	if split {
		src = se.split
	}
	out := append([]srf.SBezier(nil), src...)
	if !u.forward {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		for i := range out {
			out[i].Reverse()
		}
	}
	return out
}

// fileTol is how far a file's curves may stray from the plane of the
// face they bound, in millimetres: files are written to a tolerance.
const fileTol = 1e-3

// cutGap is the shortest piece an edge is cut into, in millimetres: a
// file's tolerance; the mesher merges points much closer than that in a
// patch's parameters, which would part it from the face beyond.
const cutGap = fileTol

// splitAtCuts cuts the edge's Beziers where faces asked, the pieces
// sharing each cut's point exactly.
func (se *solidEdge) splitAtCuts() {
	se.split = nil
	for i, sb := range se.bez {
		cs := append([]float64(nil), se.cuts[i]...)
		sort.Float64s(cs)
		// A cut closer than cutGap to an end or to the cut before
		// would leave a sliver, as where a vertex misses a line of the
		// grid by a file's tolerance: the vertex then counts as on it.
		ts := []float64{0}
		last := sb.Start()
		for _, c := range cs {
			if !(c > 0 && c < 1) {
				continue
			}
			p := sb.PointAt(c)
			if p.Minus(last).Magnitude() > cutGap && p.Minus(sb.Finish()).Magnitude() > cutGap {
				ts = append(ts, c)
				last = p
			}
		}
		ts = append(ts, 1)
		if len(ts) == 2 {
			se.split = append(se.split, sb)
			continue
		}
		first := len(se.split)
		for k := 0; k+1 < len(ts); k++ {
			se.split = append(se.split, sb.Sub(ts[k], ts[k+1]))
		}
		pieces := se.split[first:]
		pieces[0].Ctrl[0] = sb.Ctrl[0]
		pieces[len(pieces)-1].Ctrl[sb.Degree] = sb.Ctrl[sb.Degree]
		for k := 1; k < len(pieces); k++ {
			p := sb.PointAt(ts[k])
			pieces[k-1].Ctrl[sb.Degree] = p
			pieces[k].Ctrl[0] = p
		}
	}
}

// faceGrid is a face's surface cut into cells.
type faceGrid struct {
	s      *faceSurface
	cells  *surfaceCells
	us, vs []float64 // the grid's lines, first and last its ends
	// pu and pv are the periods the grid wraps by, or zero.
	pu, pv float64
	// epsU and epsV tell a point on a line from one off it.
	epsU, epsV float64
}

// samplesPerBezier are the points of a Bezier laid out in a face's
// parameters, to find where it crosses the grid's lines.
const samplesPerBezier = 24

// sampleUV lays out a Bezier in the surface's parameters from prev, the
// point before it, keeping each point within half a period of the last.
func (g *faceGrid) sampleUV(sb srf.SBezier, prev *uvSample, n int) []uvSample {
	out := make([]uvSample, 0, n+1)
	var guess *[2]float64
	if prev != nil {
		guess = &[2]float64{prev.u, prev.v}
	}
	for k := 0; k <= n; k++ {
		t := float64(k) / float64(n)
		p := sb.PointAt(t)
		if k == 0 {
			p = sb.Start()
		} else if k == n {
			p = sb.Finish()
		}
		q := g.s.point(p, guess)
		smp := uvSample{t: t, p: p, u: q.u, v: q.v}
		if k == 0 && prev != nil && prev.p.Equals(p) {
			// The joint with the piece before: one point, one place.
			smp.u, smp.v = prev.u, prev.v
			out = append(out, smp)
			guess = &[2]float64{q.u, q.v}
			continue
		}
		if len(out) > 0 || prev != nil {
			last := prev
			if len(out) > 0 {
				last = &out[len(out)-1]
			}
			smp.u += g.s.shift(last.u, smp.u, g.s.pu)
			smp.v += g.s.shift(last.v, smp.v, g.s.pv)
			if q.singU {
				smp.u = last.u
			}
			if q.singV {
				smp.v = last.v
			}
		}
		out = append(out, smp)
		guess = &[2]float64{q.u, q.v}
	}
	// A point where u (v) is undefined, as at a pole, takes it from
	// the point next to it inside the curve.
	for k := range out {
		q := g.s.point(out[k].p, &[2]float64{out[k].u, out[k].v})
		nb := k + 1
		if k == len(out)-1 {
			nb = k - 1
		}
		if q.singU && nb >= 0 && nb < len(out) {
			out[k].u = out[nb].u
		}
		if q.singV && nb >= 0 && nb < len(out) {
			out[k].v = out[nb].v
		}
	}
	return out
}

// layout lays out a face's bounds on its surface, chooses its grid,
// and asks the edges to be cut where the bounds cross its lines.
func (b *solidBuilder) layout(f *Face) (*faceGrid, error) {
	cells, err := cellsOf(f.Surf)
	if err != nil {
		return nil, err
	}
	s := &faceSurface{Surface: f.Surf}
	s.pu, s.pv = f.Surf.Period()
	s.u0, s.u1, s.v0, s.v1 = f.Surf.Domain()
	g := &faceGrid{s: s, cells: cells}

	type sampled struct {
		use  solidUse
		bez  int
		smps []uvSample
	}
	var all []sampled
	var lo, hi geom.Vector
	first := true
	var loopSmps [][]uvSample
	for _, l := range loopsOf(f) {
		var prev *uvSample
		loopSmps = append(loopSmps, nil)
		for _, u := range l {
			bz := b.beziersOf(u, false)
			for i, sb := range bz {
				smps := g.sampleUV(sb, prev, samplesPerBezier)
				prev = &smps[len(smps)-1]
				idx := i
				if !u.forward {
					idx = len(bz) - 1 - i
				}
				all = append(all, sampled{use: u, bez: idx, smps: smps})
				loopSmps[len(loopSmps)-1] = append(loopSmps[len(loopSmps)-1], smps...)
				for _, q := range smps {
					p := geom.Vector{X: q.u, Y: q.v, Z: 0}
					if first {
						lo, hi, first = p, p, false
					}
					growBox(p, &lo, &hi)
					s.scale = math.Max(s.scale, q.p.Magnitude())
				}
			}
		}
	}
	if s.scale == 0 {
		s.scale = 1
	}
	b.turned[f] = turnedLoops(loopSmps, s.pu, s.pv)
	// The grid along each parameter: a period's worth where the surface
	// closes on itself, from where the bounds leave the widest gap; else
	// the bounds' extent and a little more, but not past a pole.
	lines := func(period, dlo, dhi float64, smpLo, smpHi float64, breaks func(lo, hi float64) []float64,
		vals func() []float64, isBspline bool, poleLo, poleHi float64, atLo, atHi bool) ([]float64, float64, error) {
		if period > 0 {
			c := dlo
			if !isBspline {
				c = widestGap(vals(), period, dlo)
			}
			out := append([]float64{c}, breaks(c, c+period)...)
			return append(out, c+period), period, nil
		}
		if first {
			// No bounds: the whole domain.
			if math.IsInf(dlo, 0) || math.IsInf(dhi, 0) {
				return nil, 0, errors.New("an unbounded face")
			}
			return append(append([]float64{dlo}, breaks(dlo, dhi)...), dhi), 0, nil
		}
		w := smpHi - smpLo
		d := 1e-3*w + 1e-9*(1+math.Abs(smpLo)+math.Abs(smpHi))
		a, z := smpLo-d, smpHi+d
		// Not past a pole, and for a surface that ends, not past its
		// end; a B-spline reaches past its end by extending its last
		// piece.
		// A pole only clips the side it lies on: a cone gives its apex
		// for both.
		atLo = atLo && poleLo <= smpLo+1e-6*(w+1)
		atHi = atHi && poleHi >= smpHi-1e-6*(w+1)
		if atLo {
			a = math.Max(a, poleLo)
		} else if !isBspline && !math.IsInf(dlo, 0) {
			a = math.Max(a, dlo)
		}
		if atHi {
			z = math.Min(z, poleHi)
		} else if !isBspline && !math.IsInf(dhi, 0) {
			z = math.Min(z, dhi)
		}
		return append(append([]float64{a}, breaks(a, z)...), z), 0, nil
	}
	_, isB := f.Surf.(*bsplineSurface)
	qu0, pu0 := s.Pole(poleUMin)
	qu1, pu1 := s.Pole(poleUMax)
	qv0, pv0 := s.Pole(poleVMin)
	qv1, pv1 := s.Pole(poleVMax)
	vals := func(v bool) func() []float64 {
		return func() []float64 {
			var out []float64
			for _, a := range all {
				for _, q := range a.smps {
					if v {
						out = append(out, q.v)
					} else {
						out = append(out, q.u)
					}
				}
			}
			return out
		}
	}
	if g.us, g.pu, err = lines(s.pu, s.u0, s.u1, lo.X, hi.X, cells.breaksU, vals(false), isB, qu0, qu1, pu0, pu1); err != nil {
		return nil, err
	}
	if g.vs, g.pv, err = lines(s.pv, s.v0, s.v1, lo.Y, hi.Y, cells.breaksV, vals(true), isB, qv0, qv1, pv0, pv1); err != nil {
		return nil, err
	}
	g.epsU = 1e-9 * (1 + math.Abs(g.us[0]) + math.Abs(g.us[len(g.us)-1]-g.us[0]))
	g.epsV = 1e-9 * (1 + math.Abs(g.vs[0]) + math.Abs(g.vs[len(g.vs)-1]-g.vs[0]))

	// Where each Bezier crosses a line, on the exact curve.
	for _, a := range all {
		sb := b.beziersOf(a.use, false)
		k := a.bez
		if !a.use.forward {
			k = len(sb) - 1 - a.bez
		}
		curve := sb[k]
		for i := 0; i+1 < len(a.smps); i++ {
			p, q := a.smps[i], a.smps[i+1]
			for _, ln := range g.crossed(p.u, q.u, g.us, g.pu, g.epsU) {
				t, err := g.refine(curve, p, q, ln, true)
				if err != nil {
					return nil, err
				}
				b.cut(a.use, a.bez, t)
			}
			for _, ln := range g.crossed(p.v, q.v, g.vs, g.pv, g.epsV) {
				t, err := g.refine(curve, p, q, ln, false)
				if err != nil {
					return nil, err
				}
				b.cut(a.use, a.bez, t)
			}
		}
	}
	return g, nil
}

// turnedLoops reports whether a face's bounds, laid out in its
// surface's parameters, run the wrong way: the largest clockwise, where
// the face on their left would have it counter-clockwise. Bounds that go
// round a closed surface have no inside to tell by, and are taken as
// the file gives them.
func turnedLoops(loops [][]uvSample, pu, pv float64) bool {
	best, area := -1.0, 0.0
	for _, l := range loops {
		if len(l) < 3 {
			continue
		}
		a, z := l[0], l[len(l)-1]
		if (pu > 0 && math.Abs(z.u-a.u) > pu/2) || (pv > 0 && math.Abs(z.v-a.v) > pv/2) {
			return false
		}
		poly := make([]geom.Vector, len(l))
		for i, q := range l {
			poly[i] = geom.Vector{X: q.u, Y: q.v}
		}
		if x := polyArea(poly); math.Abs(x) > best {
			best, area = math.Abs(x), x
		}
	}
	return area < 0
}

// widestGap is the middle of the widest gap, a period round, between
// the values, or start when there are none.
func widestGap(vals []float64, period, start float64) float64 {
	if len(vals) == 0 {
		return start
	}
	m := make([]float64, len(vals))
	for i, x := range vals {
		m[i] = x - math.Floor((x-start)/period)*period
	}
	sort.Float64s(m)
	best, at := -1.0, start
	for i := range m {
		next := m[(i+1)%len(m)]
		if i == len(m)-1 {
			next += period
		}
		if gap := next - m[i]; gap > best {
			best, at = gap, m[i]+gap/2
		}
	}
	return at
}

// crossed are the lines of the grid (shifted by whole periods when it
// wraps) that the step from a to b crosses: a point on a line counts as
// past it, so each crossing is found once, at a sample on it too.
func (g *faceGrid) crossed(a, b float64, lines []float64, period, eps float64) []float64 {
	if a == b {
		return nil
	}
	lo, hi := math.Min(a, b), math.Max(a, b)
	var out []float64
	check := func(ln float64) {
		if (a < ln) != (b < ln) {
			out = append(out, ln)
		}
	}
	if period > 0 {
		base := lines[0]
		for m := math.Floor((lo-base)/period) - 1; base+m*period <= hi+period; m++ {
			for _, ln := range lines[:len(lines)-1] {
				check(ln + m*period)
			}
		}
		return out
	}
	for _, ln := range lines[1 : len(lines)-1] {
		check(ln)
	}
	return out
}

// refine is the parameter of the curve between samples p and q where
// it crosses the line at ln of u (byU) or v, by bisection on the
// surface's parameters of the curve's points.
func (g *faceGrid) refine(c srf.SBezier, p, q uvSample, ln float64, byU bool) (float64, error) {
	val := func(s uvSample) float64 {
		if byU {
			return s.u
		}
		return s.v
	}
	period := g.pv
	if byU {
		period = g.pu
	}
	side := func(x float64) bool { return x >= ln }
	ta, tb := p.t, q.t
	pa := p
	sa := side(val(p))
	for it := 0; it < 60; it++ {
		tm := (ta + tb) / 2
		pt := c.PointAt(tm)
		uv := g.s.point(pt, &[2]float64{pa.u, pa.v})
		m := uvSample{t: tm, p: pt, u: uv.u, v: uv.v}
		m.u += g.s.shift(pa.u, m.u, g.s.pu)
		m.v += g.s.shift(pa.v, m.v, g.s.pv)
		_ = period
		if side(val(m)) == sa {
			ta, pa = tm, m
		} else {
			tb = tm
		}
		if tb-ta < 1e-15 {
			break
		}
	}
	return (ta + tb) / 2, nil
}

// cut asks for the use's Bezier bez (counted along the edge) to be cut
// at its parameter t, counted the way the use runs.
func (b *solidBuilder) cut(u solidUse, bez int, t float64) {
	se := b.edges[u.e]
	if !u.forward {
		t = 1 - t
	}
	se.cuts[bez] = append(se.cuts[bez], t)
}

// planeFace adds a flat face: its bounds' pieces as they are, the
// largest the outer bound.
func (b *solidBuilder) planeFace(f *Face) error {
	pl := f.Surf.(*plane)
	var loops [][]srf.SBezier
	var areas []float64
	outer, best := -1, -1.0
	for _, l := range loopsOf(f) {
		var cs []srf.SBezier
		for _, u := range l {
			cs = append(cs, b.beziersOf(u, true)...)
		}
		var poly []geom.Vector
		for _, c := range cs {
			for k := 0; k < 8; k++ {
				poly = append(poly, pl.f.local(c.PointAt(float64(k)/8)))
			}
		}
		a := polyArea(poly)
		if math.Abs(a) > best {
			best, outer = math.Abs(a), len(loops)
		}
		loops = append(loops, cs)
		areas = append(areas, a)
	}
	if outer < 0 {
		return errors.New("a flat face with no bounds")
	}
	// The bounds run with the face on their left about the plane's
	// normal, the outer one counter-clockwise; a file that runs them the
	// other way has them turned. The kernel's run clockwise about the
	// face's normal.
	turn := areas[outer] < 0
	for i := range loops {
		if turn != f.Sense {
			loops[i] = reverseCurves(loops[i])
		}
	}
	loops[0], loops[outer] = loops[outer], loops[0]
	id := b.next
	b.next++
	_, err := b.fa.AddPlanarFaceWithin(loops, id, faceColor(f), pl.f.z, fileTol)
	return err
}

// faceColor is the face's colour, or the default.
func faceColor(f *Face) core.RgbaColor {
	if f.HasColor {
		return f.Color
	}
	return DefaultColor
}

// reverseCurves runs a chain of curves the other way.
func reverseCurves(cs []srf.SBezier) []srf.SBezier {
	out := make([]srf.SBezier, len(cs))
	for i, c := range cs {
		c.Reverse()
		out[len(cs)-1-i] = c
	}
	return out
}

// polyArea is the signed area of a polygon's x and y.
func polyArea(pts []geom.Vector) float64 {
	a := 0.0
	for i := range pts {
		p, q := pts[i], pts[(i+1)%len(pts)]
		a += p.X*q.Y - q.X*p.Y
	}
	return a / 2
}

// gridPiece is a piece of a bound inside one cell of the grid.
type gridPiece struct {
	sb   srf.SBezier
	smps []uvSample
	cell int
}

// cellLoop is a loop of a cell's region: its curves, and its points in
// the surface's parameters for telling what lies inside it.
type cellLoop struct {
	curves []srf.SBezier
	poly   []geom.Vector
}

// gridCell is one cell of a face's grid.
type gridCell struct {
	i, j           int
	u0, u1, v0, v1 float64
	patch          *srf.SSurface
	// chains are the runs of pieces inside the cell, open (from its
	// side to its side) or closed.
	open, closed [][]gridPiece
	// state is 0 unknown, 1 in the face, -1 out; sides, for a cell the
	// bounds cross, is whether each side's middle is in the face.
	state int
	sides [4]bool
}

func (c *gridCell) contains(u, v, epsU, epsV float64) bool {
	return u >= c.u0-epsU && u <= c.u1+epsU && v >= c.v0-epsV && v <= c.v1+epsV
}

// cellIndex is the cell of the grid that holds (u, v), and the shifts,
// in whole periods, that bring it there.
func (g *faceGrid) cellIndex(u, v float64) (i, j int, du, dv float64) {
	find := func(x float64, lines []float64, period float64) (int, float64) {
		d := 0.0
		if period > 0 {
			d = -math.Floor((x-lines[0])/period) * period
		}
		x += d
		k := sort.SearchFloat64s(lines, x) - 1
		k = max(0, min(k, len(lines)-2))
		return k, d
	}
	i, du = find(u, g.us, g.pu)
	j, dv = find(v, g.vs, g.pv)
	return
}

// pieceCell is the cell a piece of a bound lies in, and the shifts that
// bring it there: the cell of its middle, or for a piece along a line of
// the grid the cell whose side it runs along counter-clockwise, which
// keeps the face, on its left, inside the cell.
func (g *faceGrid) pieceCell(smps []uvSample) (i, j int, du, dv float64) {
	mid := smps[len(smps)/2]
	i, j, du, dv = g.cellIndex(mid.u, mid.v)
	a, z := smps[0], smps[len(smps)-1]
	onLine := func(x float64, lines []float64, period, eps float64) (float64, bool) {
		for _, ln := range lines {
			d := x - ln
			if period > 0 {
				d -= math.Round(d/period) * period
			}
			if math.Abs(d) <= eps {
				return ln, true
			}
		}
		return 0, false
	}
	allOn := func(get func(uvSample) float64, lines []float64, period, eps float64) bool {
		for _, q := range smps {
			if _, ok := onLine(get(q), lines, period, eps); !ok {
				return false
			}
		}
		return true
	}
	nu, nv := len(g.us)-1, len(g.vs)-1
	wrapI := func(k int) int {
		if g.pu > 0 {
			return (k + nu) % nu
		}
		return max(0, min(k, nu-1))
	}
	wrapJ := func(k int) int {
		if g.pv > 0 {
			return (k + nv) % nv
		}
		return max(0, min(k, nv-1))
	}
	switch {
	case allOn(func(q uvSample) float64 { return q.u }, g.us, g.pu, g.epsU*1e3):
		// Along a line of u: up it is the right side of the cell to
		// its left, down the left side of the cell to its right.
		x := mid.u + du
		k := sort.SearchFloat64s(g.us, x-g.epsU*1e3)
		if z.v > a.v {
			i = wrapI(k - 1)
		} else {
			i = wrapI(k)
		}
		// On the period's first line, the last cell's side is its end.
		if i == nu-1 && g.pu > 0 && math.Abs(x-g.us[0]) <= g.epsU*1e3 {
			du += g.pu
		}
		if i == 0 && g.pu > 0 && math.Abs(x-g.us[nu]) <= g.epsU*1e3 {
			du -= g.pu
		}
	case allOn(func(q uvSample) float64 { return q.v }, g.vs, g.pv, g.epsV*1e3):
		y := mid.v + dv
		k := sort.SearchFloat64s(g.vs, y-g.epsV*1e3)
		if z.u > a.u {
			j = wrapJ(k)
		} else {
			j = wrapJ(k - 1)
		}
		if j == nv-1 && g.pv > 0 && math.Abs(y-g.vs[0]) <= g.epsV*1e3 {
			dv += g.pv
		}
		if j == 0 && g.pv > 0 && math.Abs(y-g.vs[nv]) <= g.epsV*1e3 {
			dv -= g.pv
		}
	}
	return i, j, du, dv
}

// gridFace adds a curved face, a patch of its surface for each cell its
// bounds reach or enclose.
func (b *solidBuilder) gridFace(f *Face, g *faceGrid) error {
	nu, nv := len(g.us)-1, len(g.vs)-1
	if nu*nv > 1<<16 {
		return errors.New("a face of too many cells")
	}
	cells := make([]*gridCell, nu*nv)
	for i := 0; i < nu; i++ {
		for j := 0; j < nv; j++ {
			cells[i*nv+j] = &gridCell{i: i, j: j, u0: g.us[i], u1: g.us[i+1], v0: g.vs[j], v1: g.vs[j+1]}
		}
	}
	// Each bound's pieces, with their cells.
	for _, l := range b.loops(f) {
		var ps []gridPiece
		var prev *uvSample
		for _, u := range l {
			for _, sb := range b.beziersOf(u, true) {
				smps := g.sampleUV(sb, prev, 8)
				prev = &smps[len(smps)-1]
				i, j, du, dv := g.pieceCell(smps)
				c := cells[i*nv+j]
				for k := range smps {
					smps[k].u += du
					smps[k].v += dv
					eu, ev := g.pointEps(smps[k], c)
					if !c.contains(smps[k].u, smps[k].v, eu, ev) {
						return fmt.Errorf("a bound leaves a cell between the points it was cut at, at %v", smps[k].p)
					}
				}
				ps = append(ps, gridPiece{sb: sb, smps: smps, cell: i*nv + j})
			}
		}
		if len(ps) == 0 {
			continue
		}
		// Runs of pieces in one cell, broken where the cell changes and
		// where a joint lies on the cell's side, as at a pole, where the
		// face's bound runs along the collapsed side between them.
		breakAt := make([]bool, len(ps))
		anyBreak := false
		for k := range ps {
			prev := ps[(k+len(ps)-1)%len(ps)]
			if prev.cell != ps[k].cell {
				breakAt[k] = true
			} else {
				c := cells[ps[k].cell]
				q := ps[k].smps[0]
				eu, ev := g.pointEps(q, c)
				if _, on := c.perimeter(q.u, q.v, eu, ev); on {
					breakAt[k] = true
				}
			}
			anyBreak = anyBreak || breakAt[k]
		}
		if !anyBreak {
			c := cells[ps[0].cell]
			c.closed = append(c.closed, ps)
			continue
		}
		start := 0
		for k := range ps {
			if breakAt[k] {
				start = k
				break
			}
		}
		var run []gridPiece
		for k := 0; k < len(ps); k++ {
			idx := (start + k) % len(ps)
			if len(run) > 0 && breakAt[idx] {
				cells[run[0].cell].open = append(cells[run[0].cell].open, run)
				run = nil
			}
			run = append(run, ps[idx])
		}
		cells[run[0].cell].open = append(cells[run[0].cell].open, run)
	}

	var regions []struct {
		c     *gridCell
		loops []cellLoop
	}
	addRegions := func(c *gridCell, outers, holes []cellLoop) {
		groups := make([][]cellLoop, len(outers))
		for k := range outers {
			groups[k] = []cellLoop{outers[k]}
		}
		for _, h := range holes {
			best, bestA := -1, math.Inf(1)
			for k, o := range outers {
				if insidePoly(h.poly[0], o.poly) {
					if a := math.Abs(polyArea(o.poly)); a < bestA {
						best, bestA = k, a
					}
				}
			}
			if best >= 0 {
				groups[best] = append(groups[best], h)
			}
		}
		for _, gr := range groups {
			regions = append(regions, struct {
				c     *gridCell
				loops []cellLoop
			}{c, gr})
		}
	}
	for _, c := range cells {
		if len(c.open) == 0 && len(c.closed) == 0 {
			continue
		}
		walks, sides, err := b.walk(g, c)
		if err != nil {
			return err
		}
		var outers, holes []cellLoop
		outers = append(outers, walks...)
		var closed []cellLoop
		for _, ch := range c.closed {
			closed = append(closed, chainLoop(ch))
		}
		inside := len(c.open) > 0
		if len(c.open) == 0 {
			// The outermost closed loops say whether the rest of the
			// cell is in: a hole's surroundings are, an island's not.
			for k, cl := range closed {
				outermost := true
				for m, other := range closed {
					if m != k && insidePoly(cl.poly[0], other.poly) {
						outermost = false
					}
				}
				if outermost && polyArea(cl.poly) < 0 {
					inside = true
				}
			}
			if inside {
				r, err := b.rectangle(g, c)
				if err != nil {
					return err
				}
				outers = append(outers, r)
			}
			sides = [4]bool{inside, inside, inside, inside}
		}
		for _, cl := range closed {
			if polyArea(cl.poly) > 0 {
				outers = append(outers, cl)
			} else {
				holes = append(holes, cl)
			}
		}
		c.state, c.sides = -1, sides
		if inside {
			c.state = 1
		}
		addRegions(c, outers, holes)
	}
	// Cells no bound reaches are in or out as a neighbour's side says.
	var queue []*gridCell
	known := false
	for _, c := range cells {
		if c.state != 0 {
			queue = append(queue, c)
			known = true
		}
	}
	neighbour := func(c *gridCell, side int) *gridCell {
		i, j := c.i, c.j
		switch side {
		case 0:
			j--
		case 1:
			i++
		case 2:
			j++
		case 3:
			i--
		}
		if g.pu > 0 {
			i = (i + nu) % nu
		}
		if g.pv > 0 {
			j = (j + nv) % nv
		}
		if i < 0 || i >= nu || j < 0 || j >= nv {
			return nil
		}
		return cells[i*nv+j]
	}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for side := 0; side < 4; side++ {
			n := neighbour(c, side)
			if n == nil || n.state != 0 {
				continue
			}
			if c.sides[side] {
				n.state = 1
			} else {
				n.state = -1
			}
			n.sides = [4]bool{n.state == 1, n.state == 1, n.state == 1, n.state == 1}
			queue = append(queue, n)
		}
	}
	for _, c := range cells {
		if c.state == 0 && known {
			return errors.New("a cell no bound reaches nor joins")
		}
		if c.state == 0 || (c.state == 1 && len(c.open) == 0 && len(c.closed) == 0) {
			r, err := b.rectangle(g, c)
			if err != nil {
				return err
			}
			addRegions(c, []cellLoop{r}, nil)
		}
	}

	sheet := b.next
	col := faceColor(f)
	for _, r := range regions {
		p, err := b.patchOf(g, r.c)
		if err != nil {
			return err
		}
		s := *p
		s.Origin, s.Sheet, s.Color, s.SnapToTrims = b.next, sheet, col, true
		b.next++
		var loops [][]srf.SBezier
		for _, l := range r.loops {
			cs := l.curves
			if f.Sense {
				// The kernel's loops run clockwise in the patch's
				// parameters to keep its normal.
				cs = reverseCurves(cs)
			}
			loops = append(loops, cs)
		}
		if _, err := b.fa.Add(s, loops); err != nil {
			return err
		}
	}
	return nil
}

// chainLoop is a closed run of pieces as a loop.
func chainLoop(ch []gridPiece) cellLoop {
	var l cellLoop
	for _, p := range ch {
		l.curves = append(l.curves, p.sb)
		for _, q := range p.smps[:len(p.smps)-1] {
			l.poly = append(l.poly, geom.Vector{X: q.u, Y: q.v})
		}
	}
	return l
}

// insidePoly reports whether p lies inside the polygon, by crossings.
func insidePoly(p geom.Vector, poly []geom.Vector) bool {
	in := false
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		if (a.Y > p.Y) != (b.Y > p.Y) {
			x := a.X + (p.Y-a.Y)/(b.Y-a.Y)*(b.X-a.X)
			if x > p.X {
				in = !in
			}
		}
	}
	return in
}

// patchOf is the cell's patch, made once.
func (b *solidBuilder) patchOf(g *faceGrid, c *gridCell) (*srf.SSurface, error) {
	if c.patch != nil {
		return c.patch, nil
	}
	p, err := g.cells.patch(c.u0, c.u1, c.v0, c.v1)
	if err != nil {
		return nil, err
	}
	c.patch = &p
	return c.patch, nil
}

// pointEps is how far, in u and in v, a point of a bound may lie from a
// line of the grid and still be on it: twice cutGap along the surface,
// a cut dropped as too near another leaving its piece's end that far
// past the line; at least a millionth of the cell, at most a thousandth.
func (g *faceGrid) pointEps(q uvSample, c *gridCell) (eu, ev float64) {
	_, su, sv := g.s.Derivs(q.u, q.v)
	wu, wv := c.u1-c.u0, c.v1-c.v0
	eu, ev = 1e-3*wu, 1e-3*wv
	if m := su.Magnitude(); m > 0 {
		eu = math.Min(eu, 2*cutGap/m)
	}
	if m := sv.Magnitude(); m > 0 {
		ev = math.Min(ev, 2*cutGap/m)
	}
	return math.Max(eu, 1e-6*wu), math.Max(ev, 1e-6*wv)
}

// perimeter is where (u, v), on the cell's side, lies round it counter-
// clockwise from its corner (u0, v0), in [0, 4): a unit a side.
func (c *gridCell) perimeter(u, v, epsU, epsV float64) (float64, bool) {
	fu := (u - c.u0) / (c.u1 - c.u0)
	fv := (v - c.v0) / (c.v1 - c.v0)
	eu, ev := epsU/(c.u1-c.u0), epsV/(c.v1-c.v0)
	switch {
	case math.Abs(fv) <= ev:
		return math.Max(0, math.Min(1, fu)), true
	case math.Abs(fu-1) <= eu:
		return 1 + math.Max(0, math.Min(1, fv)), true
	case math.Abs(fv-1) <= ev:
		return 2 + math.Max(0, math.Min(1, 1-fu)), true
	case math.Abs(fu) <= eu:
		s := 3 + math.Max(0, math.Min(1, 1-fv))
		if s >= 4 {
			s = 0
		}
		return s, true
	}
	return 0, false
}

// sideCurve is the part of side k of the cell's patch from perimeter
// position a to b on it, both in [k, k+1], its ends pa and pb; nil
// where the side is a pole.
func (b *solidBuilder) sideCurve(g *faceGrid, c *gridCell, k int, a, z float64, pa, pz geom.Vector) (*srf.SBezier, error) {
	p, err := b.patchOf(g, c)
	if err != nil {
		return nil, err
	}
	// Bottom and right run with the patch's parameters, top and left
	// against them.
	edge := p.EdgeCurve(k)
	pole := true
	for i := 1; i <= edge.Degree; i++ {
		if !edge.Ctrl[i].Equals(edge.Ctrl[0]) {
			pole = false
		}
	}
	if pole || pa.Equals(pz) {
		return nil, nil
	}
	local := func(s float64) float64 {
		f := s - float64(k)
		if k >= 2 {
			f = 1 - f
		}
		return f
	}
	ta, tz := local(a), local(z)
	exact := func(t float64, pt geom.Vector) float64 {
		if t <= 0 || t >= 1 {
			return t
		}
		r, _ := edge.ClosestPointTo(pt, true)
		return r
	}
	ta, tz = exact(ta, pa), exact(tz, pz)
	sub := edge.Sub(ta, tz)
	sub.Ctrl[0], sub.Ctrl[sub.Degree] = pa, pz
	return &sub, nil
}

// cornerPoint is the point of the grid at the cell's corner k (0 at
// (u0, v0), counter-clockwise), the same for every cell that has it.
func (b *solidBuilder) cornerPoint(g *faceGrid, c *gridCell, k int) (geom.Vector, error) {
	i, j := c.i, c.j
	switch k {
	case 1:
		i++
	case 2:
		i, j = i+1, j+1
	case 3:
		j++
	}
	nu, nv := len(g.us)-1, len(g.vs)-1
	// The corner of the cell up and right of it, or of the last one.
	ci, cj, fi, fj := i, j, 0, 0
	if ci >= nu {
		ci, fi = nu-1, 1
	}
	if cj >= nv {
		cj, fj = nv-1, 1
	}
	if g.pu > 0 && i >= nu {
		ci, fi = 0, 0
	}
	if g.pv > 0 && j >= nv {
		cj, fj = 0, 0
	}
	cc := &gridCell{i: ci, j: cj, u0: g.us[ci], u1: g.us[ci+1], v0: g.vs[cj], v1: g.vs[cj+1]}
	p, err := g.cells.patch(cc.u0, cc.u1, cc.v0, cc.v1)
	if err != nil {
		return geom.Vector{}, err
	}
	return p.PointAt(float64(fi), float64(fj)), nil
}

// boundary are the curves along the cell's sides counter-clockwise from
// perimeter position a, point pa, to z, point pz, past the corners
// between, and their points in the surface's parameters.
func (b *solidBuilder) boundary(g *faceGrid, c *gridCell, a, z float64, pa, pz geom.Vector) ([]srf.SBezier, []geom.Vector, error) {
	if z < a {
		z += 4
	}
	var curves []srf.SBezier
	var poly []geom.Vector
	at := func(s float64) geom.Vector {
		s = math.Mod(s, 4)
		k := int(s)
		f := s - float64(k)
		switch k {
		case 0:
			return geom.Vector{X: c.u0 + f*(c.u1-c.u0), Y: c.v0}
		case 1:
			return geom.Vector{X: c.u1, Y: c.v0 + f*(c.v1-c.v0)}
		case 2:
			return geom.Vector{X: c.u1 - f*(c.u1-c.u0), Y: c.v1}
		}
		return geom.Vector{X: c.u0, Y: c.v1 - f*(c.v1-c.v0)}
	}
	s, ps := a, pa
	for s < z-1e-12 {
		next := math.Min(math.Floor(s)+1, z)
		pn := pz
		if next < z-1e-12 {
			var err error
			if pn, err = b.cornerPoint(g, c, int(next)%4); err != nil {
				return nil, nil, err
			}
		}
		k := int(math.Floor(s)) % 4
		lo, hi := s-math.Floor(s)+float64(k), next-math.Floor(s)+float64(k)
		sc, err := b.sideCurve(g, c, k, lo, hi, ps, pn)
		if err != nil {
			return nil, nil, err
		}
		if sc != nil {
			curves = append(curves, *sc)
		}
		poly = append(poly, at(s))
		s, ps = next, pn
	}
	return curves, poly, nil
}

// chainDir is the way a run leaves its start in the surface's
// parameters, or arrives at its end, scaled to the cell.
func chainDir(ch []gridPiece, atEnd bool) geom.Vector {
	var a, b uvSample
	if atEnd {
		s := ch[len(ch)-1].smps
		a, b = s[len(s)-2], s[len(s)-1]
	} else {
		s := ch[0].smps
		a, b = s[0], s[1]
	}
	return geom.Vector{X: b.u - a.u, Y: b.v - a.v}
}

// sideDir is the way the cell's boundary runs counter-clockwise at
// perimeter position s, in its parameters.
func (c *gridCell) sideDir(s float64) geom.Vector {
	switch int(math.Floor(s+1e-9)) % 4 {
	case 0:
		return geom.Vector{X: c.u1 - c.u0}
	case 1:
		return geom.Vector{Y: c.v1 - c.v0}
	case 2:
		return geom.Vector{X: c.u0 - c.u1}
	}
	return geom.Vector{Y: c.v0 - c.v1}
}

// clockwise is the angle from a to b turning clockwise, in (0, 2pi].
func clockwise(a, b geom.Vector) float64 {
	t := math.Atan2(a.Y, a.X) - math.Atan2(b.Y, b.X)
	for t <= 1e-12 {
		t += 2 * math.Pi
	}
	for t > 2*math.Pi {
		t -= 2 * math.Pi
	}
	return t
}

// rectangle is the whole cell as a loop.
func (b *solidBuilder) rectangle(g *faceGrid, c *gridCell) (cellLoop, error) {
	p0, err := b.cornerPoint(g, c, 0)
	if err != nil {
		return cellLoop{}, err
	}
	cs, poly, err := b.boundary(g, c, 0, 4, p0, p0)
	if err != nil {
		return cellLoop{}, err
	}
	return cellLoop{curves: cs, poly: poly}, nil
}

// walk closes the cell's open runs into loops along its sides, counter-
// clockwise, which keeps the face on the left; sides says, for each
// side, whether its middle is in the face.
func (b *solidBuilder) walk(g *faceGrid, c *gridCell) ([]cellLoop, [4]bool, error) {
	var sides [4]bool
	type end struct {
		s float64
		p geom.Vector
	}
	n := len(c.open)
	starts, ends := make([]end, n), make([]end, n)
	for k, ch := range c.open {
		f, l := ch[0].smps[0], ch[len(ch)-1].smps[len(ch[len(ch)-1].smps)-1]
		eu0, ev0 := g.pointEps(f, c)
		eu1, ev1 := g.pointEps(l, c)
		s0, ok0 := c.perimeter(f.u, f.v, eu0, ev0)
		s1, ok1 := c.perimeter(l.u, l.v, eu1, ev1)
		if !ok0 || !ok1 {
			return nil, sides, fmt.Errorf("a bound stops inside a cell, at %v", f.p)
		}
		starts[k] = end{s0, ch[0].sb.Start()}
		ends[k] = end{s1, ch[len(ch)-1].sb.Finish()}
	}
	used := make([]bool, n)
	var out []cellLoop
	covered := func(s float64, a, z float64) bool {
		if z < a {
			z += 4
		}
		return (s >= a && s <= z) || (s+4 >= a && s+4 <= z)
	}
	for k0 := range c.open {
		if used[k0] {
			continue
		}
		var l cellLoop
		k := k0
		for steps := 0; ; steps++ {
			if steps > n {
				return nil, sides, errors.New("the bounds in a cell don't close")
			}
			used[k] = true
			cl := chainLoop(c.open[k])
			l.curves = append(l.curves, cl.curves...)
			l.poly = append(l.poly, cl.poly...)
			e := ends[k]
			// The next run is the one that starts first counter-
			// clockwise from here. One that starts right here is taken
			// only if it turns least clockwise from the way back, which
			// keeps the face on the left: a seam, run up and then down
			// on the same line, has the face on both sides of it.
			back := chainDir(c.open[k], true).ScaledBy(-1)
			best, bestD := -1, math.Inf(1)
			hereBest, hereAngle := -1, clockwise(back, c.sideDir(e.s))
			for m := range c.open {
				d := starts[m].s - e.s
				if d > 2 {
					d -= 4
				} else if d < -2 {
					d += 4
				}
				if math.Abs(d) <= 1e-6 && starts[m].p.Equals(e.p) {
					// A run along the side itself ties with it, and
					// is taken; its points come from projections, so
					// the tie is to a ten-thousandth of a radian.
					if a := clockwise(back, chainDir(c.open[m], false)); a <= hereAngle+1e-4 {
						hereBest, hereAngle = m, a
					}
					d = 4
				} else if d < 0 {
					d += 4
				}
				if d < bestD {
					best, bestD = m, d
				}
			}
			if hereBest >= 0 {
				best, bestD = hereBest, 0
			}
			cs, poly, err := b.boundary(g, c, e.s, e.s+bestD, e.p, starts[best].p)
			if err != nil {
				return nil, sides, err
			}
			l.curves = append(l.curves, cs...)
			l.poly = append(l.poly, poly...)
			for side := 0; side < 4; side++ {
				if covered(float64(side)+0.5, e.s, e.s+bestD) {
					sides[side] = true
				}
			}
			if best == k0 {
				break
			}
			if used[best] {
				return nil, sides, errors.New("the bounds in a cell cross")
			}
			k = best
		}
		out = append(out, l)
	}
	return out, sides, nil
}

// SolidFile reads a STEP file's shapes, placed, as one shell of solids,
// or says why a shape can't be read as one: the caller keeps the mesh.
func SolidFile(path string) (sh *srf.SShell, faces int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		// A file this reader didn't foresee must not take the program
		// down with it.
		if r := recover(); r != nil {
			sh, err = nil, fmt.Errorf("step: %v", r)
		}
	}()
	f, err := Parse(data)
	if err != nil {
		return nil, 0, err
	}
	return ReadModel(f).Solid()
}

// Solid is the model's placed shapes as one shell of solids, each
// shape's faces given identities of their own, and the faces read.
func (m *Model) Solid() (*srf.SShell, int, error) {
	if len(m.Placements) == 0 {
		return nil, 0, errors.New("the file holds no solid or shell that can be read")
	}
	shapes := map[int]*srf.SShell{}
	out := &srf.SShell{}
	faces := 0
	for k, pl := range m.Placements {
		sh := shapes[pl.Shape]
		if sh == nil {
			var err error
			if sh, err = m.ShellOf(pl.Shape, srf.OriginFor(uint32(pl.Shape+1), 0)); err != nil {
				return nil, 0, fmt.Errorf("shape %d: %w", pl.Shape+1, err)
			}
			shapes[pl.Shape] = sh
		}
		q, ok := pl.T.rotation()
		if !ok {
			return nil, 0, fmt.Errorf("placement %d mirrors or stretches its shape", k+1)
		}
		var placed srf.SShell
		placed.MakeFromTransformationOf(sh, pl.T.T, q, 1)
		faces += len(m.Shapes[pl.Shape].Faces)
		if k == 0 {
			out = &placed
			continue
		}
		var both srf.SShell
		both.MakeFromAssemblyOf(out, &placed)
		out = &both
	}
	return out, faces, nil
}

// rotation is the transform's matrix as a rotation, if it is one.
func (t Transform) rotation() (geom.Quaternion, bool) {
	col := func(j int) geom.Vector { return geom.Vector{X: t.M[0][j], Y: t.M[1][j], Z: t.M[2][j]} }
	x, y, z := col(0), col(1), col(2)
	const eps = 1e-9
	if math.Abs(x.Magnitude()-1) > eps || math.Abs(y.Magnitude()-1) > eps || math.Abs(x.Dot(y)) > eps ||
		x.Cross(y).Minus(z).Magnitude() > 1e-6 {
		return geom.Quaternion{}, false
	}
	return geom.QuaternionFromBasis(x, y), true
}
