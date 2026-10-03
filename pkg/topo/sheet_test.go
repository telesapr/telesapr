package topo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

// named gives the cylinder's side patches one identity, as the circle
// entity a sketch extrudes does, and its caps two others.
func named(sh *srf.SShell) *srf.SShell {
	for i := range sh.Surface {
		s := &sh.Surface[i]
		switch n := s.NormalAt(0.5, 0.5); {
		case n.Z > 0.9:
			s.Face = 1
		case n.Z < -0.9:
			s.Face = 2
		default:
			s.Face = 7
		}
	}
	return sh
}

func TestSheetsOfCylinder(t *testing.T) {
	sh := named(cylinder(5, 10))
	tp := build(t, sh)
	if len(tp.Faces) != 6 || len(tp.Sheets) != 3 {
		t.Fatalf("%d patches in %d sheets, want 6 in 3", len(tp.Faces), len(tp.Sheets))
	}
	seams := 0
	for e := range tp.Edges {
		if tp.Edges[e].Seam {
			seams++
			if _, err := tp.NameOf(e); err == nil {
				t.Errorf("seam %d has a name", e)
			}
		}
	}
	if seams != 4 {
		t.Errorf("%d seams, want 4", seams)
	}
	side := tp.SheetsWithID(7)
	if len(side) != 1 || len(tp.Sheets[side[0]].Faces) != 4 {
		t.Fatalf("the side is %v", side)
	}
	if n := tp.SheetNeighbours(side[0]); len(n) != 2 {
		t.Errorf("the side has neighbours %v, want the two caps", n)
	}
	// The two circles are the solid's edges, each a closed chain of
	// four arcs, and no vertex is the solid's.
	edges := tp.SolidEdges()
	if len(edges) != 2 {
		t.Fatalf("solid edges %v, want 2", edges)
	}
	for _, e := range edges {
		ch := tp.Chain(e)
		if len(ch) != 4 || !tp.Closed(ch) {
			t.Errorf("edge %d's chain %v, want 4 pieces, closed", e, ch)
		}
		pts := tp.ChainPoints(ch)
		l := 0.0
		for i := 1; i < len(pts); i++ {
			l += pts[i].Minus(pts[i-1]).Magnitude()
			if d := math.Hypot(pts[i].X, pts[i].Y); math.Abs(d-5) > 1e-6 {
				t.Fatalf("a chain point %v is off the circle", pts[i])
			}
		}
		if math.Abs(l-10*math.Pi) > 0.01*10*math.Pi {
			t.Errorf("the chain is %v long, want %v", l, 10*math.Pi)
		}
		for _, c := range ch {
			if tp.Lead(c.Edge) != e {
				t.Errorf("piece %d leads to %d, not %d", c.Edge, tp.Lead(c.Edge), e)
			}
		}
	}
	for v := range tp.Vertices {
		if tp.SolidVertex(v) {
			t.Errorf("vertex %v is the solid's", tp.Vertices[v].P)
		}
	}
	if es := tp.SheetEdges(side[0]); len(es) != 2 {
		t.Errorf("the side's edges %v, want the two circles", es)
	}
}

func TestSheetsOfPlateWithHole(t *testing.T) {
	sh := plateWithHole(4, 5)
	// The hole's patches come from the tool, with no identity; give
	// them one, as the circle that cut it would.
	for i := range sh.Surface {
		if sh.Surface[i].DegM > 1 || sh.Surface[i].DegN > 1 {
			sh.Surface[i].Face = 9
		} else if sh.Surface[i].Face == 0 {
			sh.Surface[i].Face = srf.FaceID(100 + i)
		}
	}
	tp := build(t, sh)
	hole := tp.SheetsWithID(9)
	if len(hole) != 1 {
		t.Fatalf("the hole is %d sheets", len(hole))
	}
	// 6 plate faces and the hole.
	if len(tp.Sheets) != 7 {
		t.Errorf("%d sheets, want 7", len(tp.Sheets))
	}
	// 12 box edges and the two rims; 8 box corners.
	if n := len(tp.SolidEdges()); n != 14 {
		t.Errorf("%d solid edges, want 14", n)
	}
	n := 0
	for v := range tp.Vertices {
		if tp.SolidVertex(v) {
			n++
		}
	}
	if n != 8 {
		t.Errorf("%d solid vertices, want 8", n)
	}
}

// TestNoSeamAtAngle gives every face of a box one identity: faces that
// meet at an angle are not one sheet, whatever they carry.
func TestNoSeamAtAngle(t *testing.T) {
	sh := box()
	for i := range sh.Surface {
		sh.Surface[i].Face = 1
	}
	tp := build(t, sh)
	if len(tp.Sheets) != 6 {
		t.Errorf("%d sheets, want 6", len(tp.Sheets))
	}
	for e := range tp.Edges {
		if tp.Edges[e].Seam {
			t.Errorf("edge %d is a seam", e)
		}
	}
	// No names (both faces have one identity), but the edges are the
	// solid's, each its own chain: a corner ends them.
	if n := len(tp.SolidEdges()); n != 12 {
		t.Errorf("%d solid edges, want 12", n)
	}
	for e := range tp.Edges {
		if len(tp.Chain(e)) != 1 {
			t.Errorf("edge %d chains %v", e, tp.Chain(e))
		}
	}
}

// TestSheetsOfFixtures checks, over every fixture solid, that seams
// join patches that are smooth across them, and that every chain's
// pieces lie between the same two sheets and join end to end.
func TestSheetsOfFixtures(t *testing.T) {
	patches, sheets := 0, 0
	for _, f := range fixtures(t) {
		sk := load(t, f)
		if sk == nil {
			continue
		}
		for _, g := range solidGroups(sk) {
			tp, err := topo.Build(&g.RunningShell)
			if err != nil {
				continue
			}
			patches += len(tp.Faces)
			sheets += len(tp.Sheets)
			for _, s := range tp.Sheets {
				first := tp.Surface(s.Faces[0])
				for _, fc := range s.Faces[1:] {
					sf := tp.Surface(fc)
					if (sf.Sheet == 0 || sf.Sheet != first.Sheet) && tp.Faces[fc].ID != s.ID {
						t.Errorf("%s: a sheet of %x has a patch of %x from curve %x", f, s.ID, tp.Faces[fc].ID, sf.Sheet)
					}
				}
			}
			for e := range tp.Edges {
				ch := tp.Chain(e)
				pts := tp.ChainPoints(ch)
				for i := 1; i < len(ch); i++ {
					a, b := tp.Edges[ch[i-1].Edge], tp.Edges[ch[i].Edge]
					end := a.V[1]
					if ch[i-1].Reversed {
						end = a.V[0]
					}
					start := b.V[0]
					if ch[i].Reversed {
						start = b.V[1]
					}
					if end != start {
						t.Fatalf("%s g%x: chain of %d breaks at %d", f, g.H.V, e, i)
					}
				}
				if len(pts) < 2 {
					t.Errorf("%s g%x: chain of %d has %d points", f, g.H.V, e, len(pts))
				}
			}
		}
	}
	if sheets >= patches {
		t.Errorf("%d patches made %d sheets: no seam anywhere", patches, sheets)
	}
	t.Logf("%d patches in %d sheets", patches, sheets)
}

// TestSheetSplines writes every face of several patches, of every
// fixture solid, as one B-spline where its patches make a grid, and
// checks the surface against its patches: their points on it, its
// normal theirs.
func TestSheetSplines(t *testing.T) {
	merged, grids, rings := 0, 0, 0
	reasons := map[string]int{}
	for _, f := range fixtures(t) {
		sk := load(t, f)
		if sk == nil {
			continue
		}
		for _, g := range solidGroups(sk) {
			tp, err := topo.Build(&g.RunningShell)
			if err != nil {
				continue
			}
			for s := range tp.Sheets {
				if len(tp.Sheets[s].Faces) < 2 {
					continue
				}
				grids++
				grid, err := tp.SheetSpline(s)
				if err != nil {
					msg := err.Error()
					if i := strings.IndexAny(msg, "0123456789"); i > 0 {
						msg = msg[:i]
					}
					reasons[msg]++
					t.Logf("%s g%x sheet %d: %v", f, g.H.V, s, err)
					continue
				}
				merged++
				bs := grid.Surface
				if grid.ClosedU {
					rings++
				}
				u0, u1, v0, v1 := bs.Domain()
				for _, fc := range tp.Sheets[s].Faces {
					p := tp.Surface(fc)
					for _, uv := range [][2]float64{{0.5, 0.5}, {0.1, 0.9}, {0.8, 0.3}} {
						q := p.PointAt(uv[0], uv[1])
						// The nearest point of the B-spline, by its
						// patches.
						ps, _ := bs.Patches()
						best, bu, bv := math.Inf(1), 0.0, 0.0
						var bp *srf.SSurface
						for a := range ps {
							for b := range ps[a] {
								var u, v float64
								ps[a][b].ClosestPointToUV(q, &u, &v, true)
								if d := ps[a][b].PointAt(u, v).Minus(q).Magnitude(); d < best {
									best, bu, bv, bp = d, u, v, &ps[a][b]
								}
							}
						}
						if best > 1e-6 {
							t.Errorf("%s g%x sheet %d: a patch's point is %g off the surface", f, g.H.V, s, best)
							continue
						}
						n1 := p.NormalAt(uv[0], uv[1]).WithMagnitude(1)
						n2 := bp.NormalAt(bu, bv).WithMagnitude(1)
						if n1.Dot(n2) < 1-1e-6 {
							t.Errorf("%s g%x sheet %d: the surface's normal turns from its patch's (%v, %v)", f, g.H.V, s, n1, n2)
						}
					}
				}
				_, _, _, _ = u0, u1, v0, v1
			}
		}
	}
	t.Logf("%d faces of several patches, %d one B-spline (%d rings); not: %v", grids, merged, rings, reasons)
	if merged == 0 {
		t.Error("no face was written as one B-spline")
	}
}
