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
// middle in the view's coordinates, and where it is on the sheet. A
// pictorial view (the isometric) measures in the model, not as it
// shows; a detail (clip) shows only what is within clipR of its middle.
type dimView struct {
	right, up geom.Vector
	mid       geom.Vector
	at        geom.Vector
	scale     float64
	pictorial bool
	clip      bool
	clipR     float64
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

// dimElem is what a dimension measures: an edge, or the patches of a
// face with one identity.
type dimElem struct {
	what string
	// pts are the edge's points, or those of the face's edges.
	pts      []geom.Vector
	straight bool
	// A flat face: its outward normal.
	plane bool
	n     geom.Vector
	// circle is the edge's points when it is a circle, or a face's
	// largest circular edge's.
	circle []geom.Vector
}

// dimElems resolves what dm measures in the solid's topology: its
// edges, then its faces.
func dimElems(tp *topo.Topology, dm sketch.DrawingDimension) ([]dimElem, error) {
	var out []dimElem
	for _, n := range dm.Edges {
		e, err := tp.Resolve(n)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n.String(), err)
		}
		el := dimElem{what: "the edge", pts: tp.Edges[e].Points, straight: tp.Straight(e)}
		if _, _, _, ok := circleOf(el.pts); ok {
			el.circle = el.pts
		}
		out = append(out, el)
	}
	for _, id := range dm.Faces {
		fs := tp.FacesWithID(id)
		if len(fs) == 0 {
			return nil, fmt.Errorf("face %x is no longer in the solid", uint32(id))
		}
		el := dimElem{what: "the face"}
		best := -1.0
		for _, f := range fs {
			if n, ok := tp.PlaneOf(f); ok && !el.plane {
				el.plane, el.n = true, n
			}
			for _, e := range tp.FaceEdges(f) {
				pts := tp.Edges[e].Points
				el.pts = append(el.pts, pts...)
				if _, r, _, ok := circleOf(pts); ok && r > best {
					best, el.circle = r, pts
				}
			}
		}
		if len(fs) > 1 && el.plane {
			// Patches joined smoothly: flat only if all are, on one plane.
			for _, f := range fs {
				if n, ok := tp.PlaneOf(f); !ok || n.Dot(el.n) < 1-1e-9 {
					el.plane = false
				}
			}
		}
		out = append(out, el)
	}
	return out, nil
}

// mean is the middle of the points.
func mean(pts []geom.Vector) geom.Vector {
	var m geom.Vector
	for _, p := range pts {
		m = m.Plus(p)
	}
	return m.ScaledBy(1 / float64(len(pts)))
}

// inDetail reports whether any of the points shows in v's detail.
func (v dimView) inDetail(pts []geom.Vector) bool {
	if !v.clip {
		return true
	}
	for _, p := range pts {
		q := v.toView(p).Minus(v.mid)
		if math.Hypot(q.X, q.Y) <= v.clipR {
			return true
		}
	}
	return false
}

// measureDimension measures dm on view v of the solid's topology, and
// adds what draws it to el (when el isn't nil).
func measureDimension(tp *topo.Topology, dm sketch.DrawingDimension, v dimView, el *srf.SEdgeList) (float64, error) {
	elems, err := dimElems(tp, dm)
	if err != nil {
		return 0, err
	}
	for _, e := range elems {
		if !v.inDetail(e.pts) {
			return 0, fmt.Errorf("%s is outside the detail", e.what)
		}
	}
	off := dm.Offset
	if off == 0 {
		off = 10
	}
	switch dm.Kind {
	case "length":
		if !elems[0].straight {
			return 0, errors.New("the length is of a straight edge")
		}
		pts := elems[0].pts
		a3, b3 := pts[0], pts[len(pts)-1]
		a, b := v.toView(a3), v.toView(b3)
		l := b.Minus(a).Magnitude()
		if l < geom.LengthEps {
			return 0, errors.New("the edge is seen end on in this view")
		}
		if v.pictorial {
			l = b3.Minus(a3).Magnitude()
		}
		if el != nil {
			linearDim(el, v.toSheet(a), v.toSheet(b), off, formatValue(l), v.at)
		}
		return l, nil
	case "distance":
		if len(dm.Faces) == 0 && !v.pictorial {
			return edgeDistance(elems, v, off, el)
		}
		return elemDistance(elems, v, off, el)
	case "diameter", "radius":
		pts := elems[0].circle
		if pts == nil && len(dm.Faces) > 0 {
			return 0, errors.New("the face has no circular edge")
		} else if pts == nil {
			return 0, errors.New("the edge is not a circle")
		}
		c, r, n, _ := circleOf(pts)
		value := r
		text := "R " + formatValue(r)
		if dm.Kind == "diameter" {
			value, text = 2*r, "DIA "+formatValue(2*r)
		}
		look := v.right.Cross(v.up)
		facing := math.Abs(n.Dot(look))
		switch {
		case v.pictorial:
			// A leader from the point of the circle seen at its widest.
			u := n.Cross(look)
			if u.Magnitude() < 1e-9 {
				u = v.right
			}
			if el != nil {
				leader(el, v.toSheet(v.toView(c.Plus(u.WithMagnitude(r)))), off, text)
			}
		case facing > 0.99:
			if el != nil {
				cv := v.toView(c)
				edge := v.toSheet(cv.Plus(geom.Vector{X: r / math.Sqrt2, Y: r / math.Sqrt2}))
				leader(el, edge, off, text)
			}
		case facing < 0.01:
			// Seen edge on: its width across.
			across := n.Cross(look).WithMagnitude(r)
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

// edgeDistance is the distance between two edges as an orthographic
// view shows it: from the first, straight, to the second's middle,
// square to the first in the view (roadmap 18.4).
func edgeDistance(elems []dimElem, v dimView, off float64, el *srf.SEdgeList) (float64, error) {
	if !elems[0].straight {
		return 0, errors.New("a distance is from a straight edge")
	}
	pts := elems[0].pts
	a, b := v.toView(pts[0]), v.toView(pts[len(pts)-1])
	d := b.Minus(a)
	if d.Magnitude() < geom.LengthEps {
		return 0, errors.New("the first edge is seen end on in this view")
	}
	d = d.WithMagnitude(1)
	var m geom.Vector
	for _, p := range elems[1].pts {
		m = m.Plus(v.toView(p))
	}
	m = m.ScaledBy(1 / float64(len(elems[1].pts)))
	foot := a.Plus(d.ScaledBy(m.Minus(a).Dot(d)))
	dist := m.Minus(foot).Magnitude()
	if el != nil && dist > geom.LengthEps {
		linearDim(el, v.toSheet(foot), v.toSheet(m), off, formatValue(dist), v.at)
	}
	return dist, nil
}

// elemDistance is the distance in the model between two straight edges
// or flat faces (roadmap 29.2): from the first edge to the second's
// middle square to it; between parallel faces; from an edge to a face
// it runs parallel to. An orthographic view must show it at its length.
func elemDistance(elems []dimElem, v dimView, off float64, el *srf.SEdgeList) (float64, error) {
	a, b := elems[0], elems[1]
	if !a.straight && !a.plane {
		return 0, errors.New("a distance is from a straight edge or a flat face")
	}
	var from, to geom.Vector // the dimension's ends, in the model
	switch {
	case a.plane && b.plane:
		if math.Abs(a.n.Dot(b.n)) < 1-1e-6 {
			return 0, errors.New("the faces are not parallel")
		}
		to = mean(b.pts)
		from = to.Minus(a.n.ScaledBy(to.Minus(a.pts[0]).Dot(a.n)))
	case a.plane || b.plane:
		line, face := a, b
		if a.plane {
			line, face = b, a
		}
		if !line.straight {
			return 0, errors.New("a distance from a face is to a flat face or a straight edge")
		}
		p, q := line.pts[0], line.pts[len(line.pts)-1]
		if math.Abs(q.Minus(p).WithMagnitude(1).Dot(face.n)) > 1e-6 {
			return 0, errors.New("the edge is not parallel to the face")
		}
		to = p.Plus(q).ScaledBy(0.5)
		from = to.Minus(face.n.ScaledBy(to.Minus(face.pts[0]).Dot(face.n)))
	default:
		p, q := a.pts[0], a.pts[len(a.pts)-1]
		d := q.Minus(p).WithMagnitude(1)
		to = mean(b.pts)
		from = p.Plus(d.ScaledBy(to.Minus(p).Dot(d)))
	}
	dist := to.Minus(from).Magnitude()
	P, Q := v.toView(from), v.toView(to)
	shown := Q.Minus(P).Magnitude()
	switch {
	case dist < geom.LengthEps:
		return 0, nil
	case shown < geom.LengthEps:
		return 0, errors.New("the distance is seen end on in this view")
	case !v.pictorial && math.Abs(shown-dist) > 1e-6*math.Max(1, dist):
		return 0, errors.New("the distance is seen at a slant in this view")
	}
	if el != nil {
		if off > 0 {
			// Moved across, out from the view's middle, to the side of
			// what it measures, so that it stands clear of the view.
			d := Q.Minus(P).WithMagnitude(1)
			across := geom.Vector{X: -d.Y, Y: d.X}
			if across.Dot(P.Plus(Q).ScaledBy(0.5).Minus(v.mid)) < 0 {
				across = across.ScaledBy(-1)
			}
			shift := 0.0
			for _, e := range elems {
				for _, p := range e.pts {
					shift = math.Max(shift, v.toView(p).Minus(P).Dot(across))
				}
			}
			P, Q = P.Plus(across.ScaledBy(shift)), Q.Plus(across.ScaledBy(shift))
		}
		linearDim(el, v.toSheet(P), v.toSheet(Q), off, formatValue(dist), v.at)
	}
	return dist, nil
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
	w := res.BuiltinVectorFont().Width(3, text)
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
	w := res.BuiltinVectorFont().Width(3, text)
	shelf := q.Plus(geom.Vector{X: w + 2})
	el.AddEdge(q, shelf, core.StyleSolidEdge, 0, 0)
	sheetText(el, text, 3, q.Plus(geom.Vector{X: 1, Y: 1}))
}

// DrawingDimensions measures the drawing's dimensions as they would be
// drawn, each with its value or why it has none.
func (ss *Application) DrawingDimensions(d *sketch.Drawing) []DimensionResult {
	if _, _, _, err := ss.DrawingSheet(d); err != nil {
		out := make([]DimensionResult, len(d.Dimensions))
		for i := range out {
			out[i].Err = err
		}
		return out
	}
	return ss.lastDimensions
}

// drawDimensions measures and draws the dimensions on the placed views
// and on the sections and details (extras, where drawExtras put them),
// keeping what each gave in ss.lastDimensions.
func (ss *Application) drawDimensions(sheet *srf.SEdgeList, g *sketch.Group, d *sketch.Drawing, views []sketch.DrawingView,
	scale float64, mids map[string]geom.Vector, extras []dimView) {
	ss.lastDimensions = make([]DimensionResult, len(d.Dimensions))
	if len(d.Dimensions) == 0 {
		return
	}
	tp, terr := g.Topology()
	for i, dm := range d.Dimensions {
		var dv dimView
		found := false
		if n, _ := d.DimensionView(dm.View); n >= 0 {
			if n < len(extras) {
				dv, found = extras[n], true
			}
		} else {
			for k := range views {
				if views[k].Kind == dm.View {
					r, u, _ := sketch.ViewDirection(dm.View)
					dv = dimView{right: vec(r), up: vec(u), mid: mids[dm.View], at: geom.Vector{X: views[k].X, Y: views[k].Y},
						scale: scale, pictorial: dm.View == "iso"}
					found = true
				}
			}
		}
		switch {
		case !found:
			ss.lastDimensions[i].Err = fmt.Errorf("the %s view isn't on the sheet", dm.View)
		case terr != nil:
			ss.lastDimensions[i].Err = terr
		default:
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

// selectedFaceIDs are the identities of the selected faces of the
// solid, whether an entity names them or not, after GroupSelection.
func (gw *GraphicsWindow) selectedFaceIDs() []srf.FaceID {
	var out []srf.FaceID
	for _, h := range gw.gs.face {
		out = append(out, srf.FaceID(h.V))
	}
	return append(out, gw.gs.solidFace...)
}
