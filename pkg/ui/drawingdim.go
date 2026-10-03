package ui

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Dimensions on a drawing's views (roadmap 18.4, H2). A dimension
// holds the names of the edges it measures; each time the drawing is
// made they are found in the solid again and measured as the view
// shows them: an edge's length, the distance from a straight edge to
// another square to it, a circle's diameter or radius. A name that no
// longer resolves, or an edge seen so that it can't be measured, is a
// problem, and the dimension isn't drawn.

// DimensionResult is a dimension as the drawing measures it: its value
// in mm, or why it has none.
type DimensionResult struct {
	Value float64
	Err   error
}

// dimView is what a dimension needs of its view: its directions, its
// middle in the view's coordinates, and where it is on the sheet.
type dimView struct {
	right, up geom.Vector
	mid       geom.Vector
	at        geom.Vector
	scale     float64
}

func (v dimView) toView(p geom.Vector) geom.Vector {
	return geom.Vector{X: p.Dot(v.right), Y: p.Dot(v.up)}
}

func (v dimView) toSheet(p geom.Vector) geom.Vector {
	return p.Minus(v.mid).ScaledBy(v.scale).Plus(v.at)
}

// formatValue is a measure as a drawing writes it: to a hundredth of a
// mm, no trailing zeros.
func formatValue(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// circleOf is the circle through a circular edge's points: its middle,
// radius and normal, and false if they don't lie on one.
func circleOf(pts []geom.Vector) (c geom.Vector, r float64, n geom.Vector, ok bool) {
	if len(pts) < 3 {
		return c, 0, n, false
	}
	a, m, b := pts[0], pts[len(pts)/3], pts[2*len(pts)/3]
	n = m.Minus(a).Cross(b.Minus(a))
	if n.Magnitude() < 1e-12 {
		return c, 0, n, false
	}
	n = n.WithMagnitude(1)
	// The centre in the plane: equidistant from a, m and b.
	ab, am := b.Minus(a), m.Minus(a)
	den := 2 * ab.Cross(am).Dot(n)
	c = a.Plus(am.ScaledBy(ab.MagSquared()).Minus(ab.ScaledBy(am.MagSquared())).Cross(n).ScaledBy(1 / den))
	r = c.Minus(a).Magnitude()
	for _, p := range pts {
		if math.Abs(p.Minus(c).Magnitude()-r) > 1e-3*math.Max(1, r) {
			return c, r, n, false
		}
	}
	return c, r, n, true
}

// measureDimension measures dm on view v of the solid's topology, and
// adds what draws it to el (when el isn't nil).
func measureDimension(tp *topo.Topology, dm sketch.DrawingDimension, v dimView, el *srf.SEdgeList) (float64, error) {
	var pts [][]geom.Vector
	var straight []bool
	for _, n := range dm.Edges {
		e, err := tp.Resolve(n)
		if err != nil {
			return 0, fmt.Errorf("%s: %v", n.String(), err)
		}
		pts = append(pts, tp.Edges[e].Points)
		straight = append(straight, tp.Straight(e))
	}
	off := dm.Offset
	if off == 0 {
		off = 10
	}
	switch dm.Kind {
	case "length":
		if !straight[0] {
			return 0, errors.New("the length is of a straight edge")
		}
		a, b := v.toView(pts[0][0]), v.toView(pts[0][len(pts[0])-1])
		l := b.Minus(a).Magnitude()
		if l < geom.LengthEps {
			return 0, errors.New("the edge is seen end on in this view")
		}
		if el != nil {
			linearDim(el, v.toSheet(a), v.toSheet(b), off, formatValue(l), v.at)
		}
		return l, nil
	case "distance":
		if !straight[0] {
			return 0, errors.New("a distance is from a straight edge")
		}
		a, b := v.toView(pts[0][0]), v.toView(pts[0][len(pts[0])-1])
		d := b.Minus(a)
		if d.Magnitude() < geom.LengthEps {
			return 0, errors.New("the first edge is seen end on in this view")
		}
		d = d.WithMagnitude(1)
		var m geom.Vector
		for _, p := range pts[1] {
			m = m.Plus(v.toView(p))
		}
		m = m.ScaledBy(1 / float64(len(pts[1])))
		foot := a.Plus(d.ScaledBy(m.Minus(a).Dot(d)))
		dist := m.Minus(foot).Magnitude()
		if el != nil && dist > geom.LengthEps {
			linearDim(el, v.toSheet(foot), v.toSheet(m), off, formatValue(dist), v.at)
		}
		return dist, nil
	case "diameter", "radius":
		c, r, n, ok := circleOf(pts[0])
		if !ok {
			return 0, errors.New("the edge is not a circle")
		}
		value := r
		text := "R " + formatValue(r)
		if dm.Kind == "diameter" {
			value, text = 2*r, "DIA "+formatValue(2*r)
		}
		facing := math.Abs(n.Dot(v.right.Cross(v.up)))
		switch {
		case facing > 0.99:
			if el != nil {
				cv := v.toView(c)
				edge := v.toSheet(cv.Plus(geom.Vector{X: r / math.Sqrt2, Y: r / math.Sqrt2}))
				leader(el, edge, off, text)
			}
		case facing < 0.01:
			// Seen edge on: its width across.
			across := n.Cross(v.right.Cross(v.up)).WithMagnitude(r)
			a, b := v.toView(c.Minus(across)), v.toView(c.Plus(across))
			if el != nil {
				if dm.Kind == "radius" {
					b = v.toView(c)
				}
				linearDim(el, v.toSheet(a), v.toSheet(b), off, text, v.at)
			}
		default:
			return 0, errors.New("the circle is seen at a slant in this view")
		}
		return value, nil
	}
	return 0, fmt.Errorf("no dimension %q", dm.Kind)
}

// arrow adds an arrowhead at p pointing along t to el.
func arrow(el *srf.SEdgeList, p, t geom.Vector) {
	t = t.WithMagnitude(1)
	n := geom.Vector{X: -t.Y, Y: t.X}
	for _, s := range []float64{-1, 1} {
		el.AddEdge(p, p.Minus(t.ScaledBy(3)).Plus(n.ScaledBy(s*0.9)), core.StyleSolidEdge, 0, 0)
	}
}

// textAlong adds text along u, its baseline's middle at o.
func textAlong(el *srf.SEdgeList, text string, o, u geom.Vector) {
	u = u.WithMagnitude(1)
	v := geom.Vector{X: -u.Y, Y: u.X}
	w := res.BuiltinVectorFont().GetWidth(3, text)
	start := o.Minus(u.ScaledBy(w / 2))
	res.BuiltinVectorFont().Trace(3, start, u, v, text, func(a, b geom.Vector) {
		el.AddEdge(a, b, core.StyleSolidEdge, 0, 0)
	})
}

// linearDim adds a dimension from p to q on the sheet, its line off mm
// out from the view's middle, centre (in toward it when off is
// negative), with extension lines, arrows and the text.
func linearDim(el *srf.SEdgeList, p, q geom.Vector, off float64, text string, centre geom.Vector) {
	d := q.Minus(p).WithMagnitude(1)
	n := geom.Vector{X: -d.Y, Y: d.X}
	if n.Dot(p.Plus(q).ScaledBy(0.5).Minus(centre)) < 0 {
		n = n.ScaledBy(-1)
	}
	s := 1.0
	if off < 0 {
		s = -1
	}
	P, Q := p.Plus(n.ScaledBy(off)), q.Plus(n.ScaledBy(off))
	for _, e := range [][2]geom.Vector{{p, P}, {q, Q}} {
		el.AddEdge(e[0].Plus(n.ScaledBy(s*1.5)), e[1].Plus(n.ScaledBy(s*2)), core.StyleSolidEdge, 0, 0)
	}
	el.AddEdge(P, Q, core.StyleSolidEdge, 0, 0)
	arrow(el, P, d.ScaledBy(-1))
	arrow(el, Q, d)
	// The text above the line, read left to right.
	u, up := d, n
	if u.X < -1e-9 || math.Abs(u.X) < 1e-9 && u.Y < 0 {
		u, up = u.ScaledBy(-1), up.ScaledBy(-1)
	}
	textAlong(el, text, P.Plus(Q).ScaledBy(0.5).Plus(up.ScaledBy(1.2)), u)
}

// leader adds a leader from p, on a circle, out at 45 degrees by off mm,
// then a shelf with the text.
func leader(el *srf.SEdgeList, p geom.Vector, off float64, text string) {
	t := geom.Vector{X: 1, Y: 1}.WithMagnitude(1)
	q := p.Plus(t.ScaledBy(math.Abs(off)))
	el.AddEdge(p, q, core.StyleSolidEdge, 0, 0)
	arrow(el, p, t.ScaledBy(-1))
	w := res.BuiltinVectorFont().GetWidth(3, text)
	shelf := q.Plus(geom.Vector{X: w + 2})
	el.AddEdge(q, shelf, core.StyleSolidEdge, 0, 0)
	sheetText(el, text, 3, q.Plus(geom.Vector{X: 1, Y: 1}))
}

// DrawingDimensions measures the drawing's dimensions as they would be
// drawn, each with its value or why it has none.
func (ss *SolveSpaceUI) DrawingDimensions(d *sketch.Drawing) []DimensionResult {
	if _, _, _, err := ss.DrawingSheet(d); err != nil {
		out := make([]DimensionResult, len(d.Dimensions))
		for i := range out {
			out[i].Err = err
		}
		return out
	}
	return ss.lastDimensions
}

// drawDimensions measures and draws the dimensions on the placed views,
// keeping what each gave in ss.lastDimensions.
func (ss *SolveSpaceUI) drawDimensions(sheet *srf.SEdgeList, g *sketch.Group, d *sketch.Drawing, views []sketch.DrawingView,
	scale float64, mids map[string]geom.Vector) {
	ss.lastDimensions = make([]DimensionResult, len(d.Dimensions))
	if len(d.Dimensions) == 0 {
		return
	}
	tp, terr := g.Topology()
	for i, dm := range d.Dimensions {
		var placed *sketch.DrawingView
		for k := range views {
			if views[k].Kind == dm.View {
				placed = &views[k]
			}
		}
		switch {
		case placed == nil:
			ss.lastDimensions[i].Err = fmt.Errorf("the %s view isn't on the sheet", dm.View)
		case terr != nil:
			ss.lastDimensions[i].Err = terr
		default:
			r, u, _ := sketch.ViewDirection(dm.View)
			dv := dimView{right: vec(r), up: vec(u), mid: mids[dm.View], at: geom.Vector{X: placed.X, Y: placed.Y}, scale: scale}
			// Drawn only when measured: what isn't goes on no sheet.
			var el srf.SEdgeList
			v, err := measureDimension(tp, dm, dv, &el)
			ss.lastDimensions[i] = DimensionResult{Value: v, Err: err}
			if err == nil {
				sheet.Edges = append(sheet.Edges, el.Edges...)
			}
		}
	}
}
