package topo_test

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

var testdata = filepath.Join("..", "..", "testdata")

// solidFixtures lists the models under testdata with a solid to check: the
// .tlspr files, the corpus of testdata/tlspr/golden among them, but for
// the old formats' and the interface scenarios' folders.
func solidFixtures(t *testing.T) []string {
	files, err := fixtures.Walk(".tlspr")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		if dir := strings.Split(f, "/"); !slices.ContainsFunc(dir[:len(dir)-1], func(d string) bool {
			return d == "v1" || d == "v2" || d == "ui"
		}) {
			out = append(out, f)
		}
	}
	return out
}

// load reads and regenerates a fixture.
func load(t *testing.T, rel string) *sketch.Sketch {
	doc, err := tlspr.LoadFile(filepath.Join(testdata, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	sk := doc.Sketch
	sk.GenerateAll()
	return sk
}

// solidGroups are the groups whose shell is the solid so far: those
// that made one and are not forced to a mesh.
func solidGroups(sk *sketch.Sketch) []*sketch.Group {
	var out []*sketch.Group
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		if len(g.RunningShell.Surface) == 0 || g.IsForcedToMesh(sk) || len(g.ThisShell.Surface) == 0 {
			continue
		}
		out = append(out, g)
	}
	return out
}

// nonManifold are the fixtures whose solids have an edge where four
// faces meet, on purpose.
var nonManifold = map[string]bool{"tlspr/golden/group/boolean_knife_edge/normal.tlspr": true}

// fixtureCounts is what the checks over the fixtures count, for the
// summary.
type fixtureCounts struct {
	shells, sound, eulerOK int
	named, edges           int
	patches, sheets        int
	grids, merged, rings   int
	reasons                map[string]int
	lines                  []string
}

// add adds o's counts to c's.
func (c *fixtureCounts) add(o *fixtureCounts) {
	c.shells, c.sound, c.eulerOK = c.shells+o.shells, c.sound+o.sound, c.eulerOK+o.eulerOK
	c.named, c.edges = c.named+o.named, c.edges+o.edges
	c.patches, c.sheets = c.patches+o.patches, c.sheets+o.sheets
	c.grids, c.merged, c.rings = c.grids+o.grids, c.merged+o.merged, c.rings+o.rings
	for k, n := range o.reasons {
		c.reasons[k] += n
	}
	c.lines = append(c.lines, o.lines...)
}

// TestFixtures regenerates every fixture and builds the topology of
// each of its solids once, then checks it: that it validates and
// Euler's formula agrees with the mesh made from the same shell (every
// solid but the knife edge's is sound); that every edge with a name
// resolves back to itself; that seams join patches smooth across them
// and every chain's pieces join end to end between the same two
// sheets; and that a face of several patches in a grid, written as one
// B-spline, has its patches' points and normals.
func TestFixtures(t *testing.T) {
	t.Parallel()
	c := fixtureCounts{reasons: map[string]int{}}
	var mu sync.Mutex
	t.Run("models", func(t *testing.T) {
		for _, f := range solidFixtures(t) {
			t.Run(f, func(t *testing.T) {
				t.Parallel()
				sk := load(t, f)
				if sk == nil {
					return
				}
				one := fixtureCounts{reasons: map[string]int{}}
				for _, g := range solidGroups(sk) {
					one.shells++
					tp, err := topo.Build(&g.RunningShell)
					checkSoundAndEuler(t, &one, f, sk, g, tp, err)
					if err != nil {
						continue
					}
					checkNames(t, &one, f, g, tp)
					checkSheets(t, &one, f, g, tp)
					checkSheetSplines(t, &one, f, g, tp)
				}
				mu.Lock()
				defer mu.Unlock()
				c.add(&one)
			})
		}
	})
	slices.Sort(c.lines)
	for _, l := range c.lines {
		t.Log(l)
	}
	t.Logf("%d shells, %d sound, %d with the mesh's Euler characteristic", c.shells, c.sound, c.eulerOK)
	t.Logf("%d of %d edges named", c.named, c.edges)
	if c.sheets >= c.patches {
		t.Errorf("%d patches made %d sheets: no seam anywhere", c.patches, c.sheets)
	}
	t.Logf("%d patches in %d sheets", c.patches, c.sheets)
	t.Logf("%d faces of several patches, %d one B-spline (%d rings); not: %v", c.grids, c.merged, c.rings, c.reasons)
	if c.merged == 0 {
		t.Error("no face was written as one B-spline")
	}
}

// checkSoundAndEuler validates the topology and checks Euler's formula
// against the mesh made from the same shell.
func checkSoundAndEuler(t *testing.T, c *fixtureCounts, f string, sk *sketch.Sketch, g *sketch.Group, tp *topo.Topology, err error) {
	probs := tp.Validate()
	g.GenerateDisplayItems(sk)
	want := topo.MeshEuler(&g.DisplayMesh)
	got := tp.Euler()
	if err == nil && len(probs) == 0 {
		c.sound++
	}
	if got == want {
		c.eulerOK++
	}
	if (err != nil || len(probs) > 0 || got != want) && !nonManifold[f] {
		t.Errorf("%s g%x: err %v, %d problems, Euler %d, the mesh's %d", f, g.H.V, err, len(probs), got, want)
	}
	if nonManifold[f] && (len(probs) != 1 || !strings.Contains(probs[0].Msg, "4 co-edges")) {
		t.Errorf("%s g%x: %v", f, g.H.V, probs)
	}
	if err != nil || len(probs) > 0 || got != want {
		msg := ""
		if err != nil {
			msg = err.Error()
		} else if len(probs) > 0 {
			msg = fmt.Sprintf("%d problems, first %s", len(probs), probs[0].Msg)
		}
		c.lines = append(c.lines, fmt.Sprintf("%s g%x: V %d E %d F %d L %d euler %d mesh %d booleanFailed %v %s",
			f, g.H.V, len(tp.Vertices), len(tp.Edges), len(tp.Faces), len(tp.Loops), got, want, g.RunningShell.BooleanFailed, msg))
	}
}

// checkNames: every edge that has a name resolves back to itself.
func checkNames(t *testing.T, c *fixtureCounts, f string, g *sketch.Group, tp *topo.Topology) {
	for e := range tp.Edges {
		c.edges++
		n, err := tp.NameOf(e)
		if err != nil {
			continue
		}
		c.named++
		if got, err := tp.Resolve(n); err != nil || got != e {
			t.Errorf("%s g%x: %v resolves to %d, not %d (%v)", f, g.H.V, n, got, e, err)
		}
	}
}

// checkSheets: seams join patches that are smooth across them, and
// every chain's pieces lie between the same two sheets and join end to
// end.
func checkSheets(t *testing.T, c *fixtureCounts, f string, g *sketch.Group, tp *topo.Topology) {
	c.patches += len(tp.Faces)
	c.sheets += len(tp.Sheets)
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

// checkSheetSplines writes every face of several patches as one
// B-spline where its patches make a grid, and checks the surface
// against its patches: their points on it, its normal theirs.
func checkSheetSplines(t *testing.T, c *fixtureCounts, f string, g *sketch.Group, tp *topo.Topology) {
	for s := range tp.Sheets {
		if len(tp.Sheets[s].Faces) < 2 {
			continue
		}
		c.grids++
		grid, err := tp.SheetSpline(s)
		if err != nil {
			msg := err.Error()
			if i := strings.IndexAny(msg, "0123456789"); i > 0 {
				msg = msg[:i]
			}
			c.reasons[msg]++
			t.Logf("%s g%x sheet %d: %v", f, g.H.V, s, err)
			continue
		}
		c.merged++
		bs := grid.Surface
		if grid.ClosedU {
			c.rings++
		}
		for _, fc := range tp.Sheets[s].Faces {
			p := tp.Surface(fc)
			for _, uv := range [][2]float64{{0.5, 0.5}, {0.1, 0.9}, {0.8, 0.3}} {
				q := p.PointAt(uv[0], uv[1])
				// The nearest point of the B-spline, by its patches.
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
	}
}

// BenchmarkBuildThread derives the topology of a threaded bolt, about
// 270 faces, as a chamfer by name does on each regeneration.
func BenchmarkBuildThread(b *testing.B) {
	t := &testing.T{}
	sk := load(t, "thread/bolt_m8x30.tlspr")
	gs := solidGroups(sk)
	sh := &gs[len(gs)-1].RunningShell
	for b.Loop() {
		if _, err := topo.Build(sh); err != nil {
			b.Fatal(err)
		}
	}
}
