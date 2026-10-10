package export

// G code: slightly complicated by our ability to generate multiple
// passes, and to specify the feeds and depth; those parameters get set
// in the configuration screen. Lines are cut with G01, circular arcs
// with G02 (clockwise) and G03 (counter-clockwise), and every other
// curve is fitted with arcs first (biarc.go), so that a controller cuts
// it smoothly and at a steady feed instead of along a chain of chords.

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// gcodeMove is one cut in the xy plane: a straight line from a to b, or
// when arc is set a circular arc about c, counter-clockwise when ccw.
type gcodeMove struct {
	a, b geom.Vector
	arc  bool
	c    geom.Vector
	ccw  bool
	tag  bool
}

// reversed is the same cut run from b to a.
func (m gcodeMove) reversed() gcodeMove {
	m.a, m.b = m.b, m.a
	m.ccw = !m.ccw
	return m
}

type gcodeFileWriter struct {
	vectorWriter
	moves []gcodeMove
}

func (w *gcodeFileWriter) HasCanvasSize() bool { return false }
func (w *gcodeFileWriter) CanOutputMesh() bool { return false }

func (w *gcodeFileWriter) StartFile() {
	w.moves = nil
}
func (w *gcodeFileWriter) StartPath(core.RGBA, float64, bool, core.RGBA, core.HStyle) {
}
func (w *gcodeFileWriter) Background(core.RGBA) {}
func (w *gcodeFileWriter) FinishPath(core.RGBA, float64, bool, core.RGBA, core.HStyle, bool) {
}
func (w *gcodeFileWriter) Triangle(*srf.STriangle) {}

func (w *gcodeFileWriter) Bezier(sb *srf.SBezier) {
	var c geom.Vector
	var r float64
	switch {
	case sb.Degree == 1:
		w.moves = append(w.moves, gcodeMove{a: sb.Ctrl[0], b: sb.Ctrl[1]})
	case sb.IsCircle(geom.Vector{Z: 1}, &c, &r):
		// A circle's piece is less than half a turn, so the side its
		// middle control point lies on says which way it turns.
		a, b := sb.Ctrl[0], sb.Ctrl[2]
		ccw := b.Minus(a).Cross(sb.Ctrl[1].Minus(a)).Z < 0
		w.moves = append(w.moves, gcodeMove{a: a, b: b, arc: true, c: c, ccw: ccw})
	default:
		fitBiarcs(sb, w.o.Tol.ChordTolMm(), func(m gcodeMove) { w.moves = append(w.moves, m) })
	}
}

// chains joins the cuts end to end, each chain going on from its last
// point until it closes or no cut goes on from there, turning a cut
// round where its far end is the one that meets.
func (w *gcodeFileWriter) chains() [][]gcodeMove {
	var out [][]gcodeMove
	for i := range w.moves {
		if w.moves[i].tag {
			continue
		}
		w.moves[i].tag = true
		chain := []gcodeMove{w.moves[i]}
		first, last := w.moves[i].a, w.moves[i].b
		for !last.Equals(first) {
			j := i + 1
			for ; j < len(w.moves); j++ {
				m := &w.moves[j]
				if m.tag {
					continue
				}
				if m.a.Equals(last) {
					chain = append(chain, *m)
					break
				}
				if m.b.Equals(last) {
					chain = append(chain, m.reversed())
					break
				}
			}
			if j >= len(w.moves) {
				break
			}
			w.moves[j].tag = true
			last = chain[len(chain)-1].b
		}
		out = append(out, chain)
	}
	return out
}

func (w *gcodeFileWriter) FinishAndCloseFile() error {
	mm := func(v float64) string { return w.format(v) }
	chains := w.chains()

	for i := range w.o.GCode.Passes {
		depth := (w.o.GCode.Depth / float64(w.o.GCode.Passes)) * float64(i+1)

		for _, chain := range chains {
			pt := chain[0].a
			w.printf("G00 X%s Y%s\r\n", mm(pt.X), mm(pt.Y))
			w.printf("G01 Z%s F%s\r\n", mm(depth), mm(w.o.GCode.PlungeFeed))

			for _, m := range chain {
				switch {
				case !m.arc:
					w.printf("G01 X%s Y%s F%s\r\n", mm(m.b.X), mm(m.b.Y), mm(w.o.GCode.Feed))
				default:
					g := "G02"
					if m.ccw {
						g = "G03"
					}
					// I and J are the centre from the arc's start.
					w.printf("%s X%s Y%s I%s J%s F%s\r\n", g, mm(m.b.X), mm(m.b.Y),
						mm(m.c.X-m.a.X), mm(m.c.Y-m.a.Y), mm(w.o.GCode.Feed))
				}
			}
			// Move up to a clearance plane above the work.
			w.printf("G00 Z%s\r\n", mm(w.o.GCode.SafeHeight))
		}
	}

	w.moves = nil
	return w.closeFile()
}
