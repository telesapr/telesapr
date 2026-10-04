package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A face of several patches is one B-spline surface where its patches
// make a grid: each side of a patch the whole side of the next, their
// parameters running alike, as the knot spans of a B-spline's patches
// do, and as the quarters of an extruded circle do around it. SheetSpline
// finds that grid and writes the patches as one surface, every knot
// between them a knot of full multiplicity, for STEP export (step
// 11.4).

// Grid is a face of several patches written as one B-spline surface.
type Grid struct {
	Surface *srf.BSplineSurface
	// ClosedU is set when the patches close up into a ring, as a
	// cylinder's do, which then runs along u: the surface's first and
	// last rows of control points along u are one.
	ClosedU bool
	// Cells are the patches' places, by span along u and along v, in
	// the order given or, for a sheet, by surface handle.
	Cells []Cell
}

// Cell is a patch's place in a grid.
type Cell struct {
	H    core.HSSurface
	I, J int
}

// SheetSpline is sheet s as one B-spline surface, or an error saying
// why its patches are no grid. A surface whose patches close up both
// ways, a whole torus, is none.
func (t *Topology) SheetSpline(s int) (*Grid, error) {
	var ps []srf.SSurface
	seen := map[uint32]bool{}
	for _, f := range t.Sheets[s].Faces {
		sf := t.Surface(f)
		if sf == nil || seen[sf.H.V] {
			continue
		}
		seen[sf.H.V] = true
		ps = append(ps, *sf)
	}
	return GridSpline(ps)
}

// cell is a patch placed in the grid, turned to run as the grid does.
type cell struct {
	i, j int
	p    srf.SSurface
}

// GridSpline writes patches that make a grid as one B-spline surface;
// see SheetSpline.
func GridSpline(ps []srf.SSurface) (*Grid, error) {
	if len(ps) == 0 {
		return nil, errors.New("topo: no patches")
	}
	placed := make([]*cell, len(ps))
	placed[0] = &cell{p: ps[0]}
	at := map[[2]int]int{{0, 0}: 0}
	queue := []int{0}
	closedU, closedV := false, false
	// The neighbour across each side: side 1 (u = 1) is the next along
	// u, its side 3 the same curve, and so on.
	step := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		c := placed[k]
		for side := 0; side < 4; side++ {
			want := c.p.EdgeCurve(side)
			opp := (side + 2) % 4
			for r := range ps {
				if r == k {
					continue
				}
				var cand srf.SSurface
				found := false
				if placed[r] != nil {
					// Already in the grid: it must be the neighbour there,
					// or the grid closes up into a ring.
					cand = placed[r].p
					found = sameCurve(want, cand.EdgeCurve(opp))
				} else {
					for o := 0; o < 8 && !found; o++ {
						cand = turned(ps[r], o)
						found = sameCurve(want, cand.EdgeCurve(opp))
					}
				}
				if !found {
					continue
				}
				ni, nj := c.i+step[side][0], c.j+step[side][1]
				if placed[r] != nil {
					if placed[r].i == ni && placed[r].j == nj {
						continue
					}
					if step[side][1] == 0 && placed[r].j == c.j {
						// The ring closes along u.
						closedU = true
						continue
					}
					if step[side][0] == 0 && placed[r].i == c.i {
						closedV = true
						continue
					}
					return nil, fmt.Errorf("topo: patch %d meets patch %d twice", k, r)
				}
				if prev, ok := at[[2]int{ni, nj}]; ok {
					return nil, fmt.Errorf("topo: patches %d and %d both lie at %d,%d", prev, r, ni, nj)
				}
				if err := matchWeights(&cand, &c.p, side); err != nil {
					return nil, err
				}
				placed[r] = &cell{i: ni, j: nj, p: cand}
				at[[2]int{ni, nj}] = r
				queue = append(queue, r)
			}
		}
	}
	// A whole rectangle of cells, every patch in it.
	i0, j0, i1, j1 := 0, 0, 0, 0
	for k, c := range placed {
		if c == nil {
			return nil, fmt.Errorf("topo: patch %d meets no other along a whole side", k)
		}
		i0, j0, i1, j1 = min(i0, c.i), min(j0, c.j), max(i1, c.i), max(j1, c.j)
	}
	if closedV && !closedU {
		// A ring along v, as a lathe's: turn it to run along u, and v
		// backwards, so that the normal stays.
		for _, c := range placed {
			c.i, c.j, c.p = c.j, -c.i, turned(turned(c.p, 4), 2)
		}
		i0, j0, i1, j1 = j0, -i1, j1, -i0
		closedU, closedV = true, false
	}
	if closedV {
		return nil, errors.New("topo: the patches close up both ways")
	}
	ni, nj := i1-i0+1, j1-j0+1
	if ni*nj != len(ps) {
		return nil, fmt.Errorf("topo: %d patches don't fill a grid of %d by %d", len(ps), ni, nj)
	}
	grid := make([][]*cell, ni)
	for i := range grid {
		grid[i] = make([]*cell, nj)
	}
	for _, c := range placed {
		grid[c.i-i0][c.j-j0] = c
	}
	// One degree each way: raise the patches to the highest.
	du, dv := 0, 0
	for _, c := range placed {
		du, dv = max(du, c.p.DegM), max(dv, c.p.DegN)
	}
	for _, c := range placed {
		up, err := c.p.ElevatedTo(du, dv)
		if err != nil {
			return nil, err
		}
		c.p = up
	}
	bs := &srf.BSplineSurface{DegU: du, DegV: dv}
	// Each span as long as its patches are along it, so the parameters
	// go about as fast as the surface everywhere: a tessellator works
	// in them, and a short arc between long lines in spans of one size
	// meshes badly.
	lenU := make([]float64, ni)
	lenV := make([]float64, nj)
	for i := range grid {
		for j := range grid[i] {
			lu, lv := patchLengths(&grid[i][j].p)
			lenU[i] += lu / float64(nj)
			lenV[j] += lv / float64(ni)
		}
	}
	nu, nv := ni*du+1, nj*dv+1
	bs.Ctrl = make([][]geom.Vector, nu)
	bs.Weight = make([][]float64, nu)
	for a := range bs.Ctrl {
		bs.Ctrl[a] = make([]geom.Vector, nv)
		bs.Weight[a] = make([]float64, nv)
	}
	for i := range grid {
		for j := range grid[i] {
			p := &grid[i][j].p
			for a := 0; a <= du; a++ {
				for b := 0; b <= dv; b++ {
					bs.Ctrl[i*du+a][j*dv+b] = p.Ctrl[a][b]
					bs.Weight[i*du+a][j*dv+b] = p.Weight[a][b]
				}
			}
		}
	}
	// Where the patches join smoothly, spans in the proportion they
	// join in, so that the surface is smooth in its parameters too, and
	// then the knots between them out as often as they go (package 27.1).
	bs.KnotsU = fullKnots(du, smoothSpans(lenU, func(s int) float64 { return joinRatio(bs, true, s) }))
	bs.KnotsV = fullKnots(dv, smoothSpans(lenV, func(s int) float64 { return joinRatio(bs, false, s) }))
	g := &Grid{Surface: bs, ClosedU: closedU}
	for k, c := range placed {
		g.Cells = append(g.Cells, Cell{H: ps[k].H, I: c.i - i0, J: c.j - j0})
	}
	if err := bs.Check(); err != nil {
		return nil, err
	}
	bs.RemoveKnots(KnotRemovalTol)
	if err := bs.Check(); err != nil {
		return nil, err
	}
	return g, nil
}

// KnotRemovalTol is how far, in mm, taking a knot out of a face's
// B-spline may move it: no more than the patches' own points agree
// where they meet.
var KnotRemovalTol = 0.1 * geom.LengthEps

// smoothSpans are the lengths of the spans of a grid along one way:
// lengths as given, but through each run of spans that join smoothly,
// in the proportion ratio(s) between span s+1 and span s, as long in
// all as the run's lengths given. ratio is 0 where span s and s+1 meet
// at a crease.
func smoothSpans(lengths []float64, ratio func(s int) float64) []float64 {
	out := append([]float64(nil), lengths...)
	for a := 0; a < len(lengths); {
		b := a
		rel := []float64{1}
		for b+1 < len(lengths) {
			r := ratio(b)
			if r <= 0 {
				break
			}
			rel = append(rel, rel[len(rel)-1]*r)
			b++
		}
		if b > a {
			given, sum := 0.0, 0.0
			for k := a; k <= b; k++ {
				given += lengths[k]
				sum += rel[k-a]
			}
			if given > 0 && sum > 0 && !math.IsInf(sum, 0) {
				for k := a; k <= b; k++ {
					out[k] = rel[k-a] * given / sum
				}
			}
		}
		a = b + 1
	}
	return out
}

// joinRatio is, where span s of the grid's surface meets span s+1
// along u (or v), the ratio of the lengths their parameters must have
// for the surface to be smooth in them there: the ratio of the steps of
// its homogeneous control points on either side of the join, when every
// row of them steps on the same way in the same ratio; else 0, a
// crease.
func joinRatio(bs *srf.BSplineSurface, alongU bool, s int) float64 {
	deg, rows := bs.DegV, len(bs.Ctrl)
	if alongU {
		deg, rows = bs.DegU, len(bs.Ctrl[0])
	}
	c := (s + 1) * deg
	at := func(row, k int) geom.Vector4 {
		if alongU {
			return geom.Vector4FromWeighted(bs.Weight[k][row], bs.Ctrl[k][row])
		}
		return geom.Vector4FromWeighted(bs.Weight[row][k], bs.Ctrl[row][k])
	}
	ratio, size := 0.0, 0.0
	for row := 0; row < rows; row++ {
		size = max(size, len4(at(row, c)))
	}
	tiny := 1e-9 * max(size, 1)
	for row := 0; row < rows; row++ {
		a := at(row, c).Minus(at(row, c-1))
		b := at(row, c+1).Minus(at(row, c))
		la, lb := len4(a), len4(b)
		if la <= tiny && lb <= tiny {
			// A pole: both sides shrink to the point.
			continue
		}
		if la <= tiny || lb <= tiny {
			return 0
		}
		r := lb / la
		if len4(b.Minus(a.ScaledBy(r))) > 1e-9*lb {
			return 0
		}
		if ratio != 0 && math.Abs(r-ratio) > 1e-7*ratio {
			return 0
		}
		if ratio == 0 {
			ratio = r
		}
	}
	return ratio
}

func len4(v geom.Vector4) float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z + v.W*v.W)
}

// fullKnots is a clamped knot vector of spans of the lengths given,
// scaled to run from 0 to the number of spans, each inner knot deg
// times: the B-spline is the Bezier pieces joined end to end.
func fullKnots(deg int, lengths []float64) []float64 {
	total := 0.0
	for _, l := range lengths {
		total += l
	}
	n := float64(len(lengths))
	at := func(l float64) float64 {
		if total <= 0 {
			return 0
		}
		return l / total * n
	}
	var k []float64
	for i := 0; i <= deg; i++ {
		k = append(k, 0)
	}
	sum := 0.0
	for s := 0; s+1 < len(lengths); s++ {
		sum += lengths[s]
		for i := 0; i < deg; i++ {
			k = append(k, at(sum))
		}
	}
	for i := 0; i <= deg; i++ {
		k = append(k, n)
	}
	return k
}

// patchLengths are how long a patch is along u and along v: its
// control polygon's lengths, averaged over its rows.
func patchLengths(p *srf.SSurface) (lu, lv float64) {
	for j := 0; j <= p.DegN; j++ {
		for i := 1; i <= p.DegM; i++ {
			lu += p.Ctrl[i][j].Minus(p.Ctrl[i-1][j]).Magnitude()
		}
	}
	for i := 0; i <= p.DegM; i++ {
		for j := 1; j <= p.DegN; j++ {
			lv += p.Ctrl[i][j].Minus(p.Ctrl[i][j-1]).Magnitude()
		}
	}
	return lu / float64(p.DegN+1), lv / float64(p.DegM+1)
}

// turned is patch p with its parameters turned by orientation o, 0 to
// 7: bit 0 runs u backwards, bit 1 v, bit 2 swaps them.
func turned(p srf.SSurface, o int) srf.SSurface {
	out := p
	m, n := p.DegM, p.DegN
	if o&4 != 0 {
		out.DegM, out.DegN = n, m
	}
	for a := 0; a <= out.DegM; a++ {
		for b := 0; b <= out.DegN; b++ {
			i, j := a, b
			if o&4 != 0 {
				i, j = b, a
			}
			if o&1 != 0 {
				i = m - i
			}
			if o&2 != 0 {
				j = n - j
			}
			out.Ctrl[a][b], out.Weight[a][b] = p.Ctrl[i][j], p.Weight[i][j]
		}
	}
	return out
}

// sameCurve reports whether two curves are one, running the same way:
// the same points, and weights in proportion, raised to one degree. A
// side that is a point, as the pole of a disc's quarter, meets nothing.
func sameCurve(a, b srf.SBezier) bool {
	if pole(&a) || pole(&b) {
		return false
	}
	if a.Degree != b.Degree {
		var err error
		if a, err = a.ElevatedTo(max(a.Degree, b.Degree)); err != nil {
			return false
		}
		if b, err = b.ElevatedTo(a.Degree); err != nil {
			return false
		}
	}
	k := b.Weight[0] / a.Weight[0]
	for i := 0; i <= a.Degree; i++ {
		if !a.Ctrl[i].EqualsTol(b.Ctrl[i], geom.LengthEps) {
			return false
		}
		if w := a.Weight[i] * k; w-b.Weight[i] > 1e-9*w || b.Weight[i]-w > 1e-9*w {
			return false
		}
	}
	return true
}

// matchWeights scales the weights of p, whose side opposite side meets
// q's side side, to q's along that side, which leaves p the same
// surface.
func matchWeights(p, q *srf.SSurface, side int) error {
	a, b := q.EdgeCurve(side), p.EdgeCurve((side+2)%4)
	k := a.Weight[0] / b.Weight[0]
	if k == 1 {
		return nil
	}
	for i := 0; i <= p.DegM; i++ {
		for j := 0; j <= p.DegN; j++ {
			p.Weight[i][j] *= k
		}
	}
	return nil
}
