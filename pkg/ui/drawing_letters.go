package ui

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// What is written on a drawing's sheet is kept clear of what else is
// there by simple rules, not a general placer: the letters of a
// cutting plane go past the ends of its line, or beside them where that
// is taken, and a view's name moves down below a letter or a
// dimension's text that would cover it.

// sheetBox is a box on the sheet, in mm; the zero box holds nothing.
type sheetBox struct {
	lo, hi geom.Vector
	set    bool
}

// with is the box grown to hold p.
func (b sheetBox) with(p geom.Vector) sheetBox {
	if !b.set {
		return sheetBox{lo: geom.Vector{X: p.X, Y: p.Y}, hi: geom.Vector{X: p.X, Y: p.Y}, set: true}
	}
	b.lo.X, b.lo.Y = math.Min(b.lo.X, p.X), math.Min(b.lo.Y, p.Y)
	b.hi.X, b.hi.Y = math.Max(b.hi.X, p.X), math.Max(b.hi.Y, p.Y)
	return b
}

// overlaps reports whether the boxes share more than an edge.
func (b sheetBox) overlaps(o sheetBox) bool {
	return b.set && o.set && b.lo.X < o.hi.X && o.lo.X < b.hi.X && b.lo.Y < o.hi.Y && o.lo.Y < b.hi.Y
}

// grown is the box m mm bigger all round.
func (b sheetBox) grown(m float64) sheetBox {
	b.lo = b.lo.Minus(geom.Vector{X: m, Y: m})
	b.hi = b.hi.Plus(geom.Vector{X: m, Y: m})
	return b
}

// boxAbout is the box w by h about c.
func boxAbout(c geom.Vector, w, h float64) sheetBox {
	return sheetBox{lo: geom.Vector{X: c.X - w/2, Y: c.Y - h/2}, hi: geom.Vector{X: c.X + w/2, Y: c.Y + h/2}, set: true}
}

// dimOut is what drawing a dimension adds: its edges, and the boxes of
// its texts.
type dimOut struct {
	el    srf.SEdgeList
	texts []sheetBox
}

const (
	// cutReach is how far a cutting plane's line runs past the view's
	// outline, in mm: short of the view's name, 4 mm below it.
	cutReach = 3.0
	// cutLetterHeight is the height of a cutting plane's letters.
	cutLetterHeight = 3.5
	// letterGap is the space kept between a letter and its line or
	// what it keeps clear of.
	letterGap = 1.0
)

// placeLetter is where a letter w by h goes at the end e of a line
// running out along the unit vector o: past the end, or beside the end
// on either side when that is taken by one of avoid. It returns the
// letter's box; past the end when every place is taken.
func placeLetter(e, o geom.Vector, w, h float64, avoid []sheetBox) sheetBox {
	reach := func(u geom.Vector) float64 { return math.Abs(u.X)*w/2 + math.Abs(u.Y)*h/2 }
	p := geom.Vector{X: -o.Y, Y: o.X}
	back := e.Minus(o.ScaledBy(reach(o)))
	candidates := []geom.Vector{
		e.Plus(o.ScaledBy(letterGap + reach(o))),
		back.Plus(p.ScaledBy(letterGap + reach(p))),
		back.Minus(p.ScaledBy(letterGap + reach(p))),
	}
	for _, c := range candidates {
		b := boxAbout(c, w, h)
		if !slices.ContainsFunc(avoid, b.grown(letterGap/2).overlaps) {
			return b
		}
	}
	return boxAbout(candidates[0], w, h)
}

// cuttingPlanes draws each section's plane, dashed and lettered at its
// ends, on each placed view that sees it edge on, the letters clear of
// avoid and of each other. It returns the letters' boxes.
func (ss *Application) cuttingPlanes(sheet *srf.SEdgeList, g *sketch.Group, d *sketch.Drawing, views []sketch.DrawingView,
	scale float64, sizes map[string][2]float64, mids map[string]geom.Vector, avoid []sheetBox) []sheetBox {
	var letters []sheetBox
	for i, sec := range d.Sections {
		letter := sketch.Letter(i)
		w := res.BuiltinVectorFont().Width(cutLetterHeight, letter)
		r, u, _ := sketch.ViewDirection(sec.View)
		keep := sectionPlane(&g.DisplayMesh, vec(r), vec(u), sec.Offset)
		for _, v := range views {
			ends, ok := cutLine(keep, v, scale, sizes, mids)
			if !ok {
				continue
			}
			sheet.AddEdge(ends[0], ends[1], core.StyleHiddenEdge, 0, 0)
			for k, e := range ends {
				o := e.Minus(ends[1-k]).WithMagnitude(1)
				b := placeLetter(e, o, w, cutLetterHeight, slices.Concat(avoid, letters))
				sheetText(sheet, letter, cutLetterHeight, b.lo)
				letters = append(letters, b)
			}
		}
	}
	return letters
}

// cutLine is the line of the section plane keep on view v, on the
// sheet: across the view's box and cutReach mm past it, and false when
// v does not see the plane edge on.
func cutLine(keep srf.HalfSpace, v sketch.DrawingView, scale float64, sizes map[string][2]float64,
	mids map[string]geom.Vector) ([2]geom.Vector, bool) {
	var ends [2]geom.Vector
	if v.Kind == "iso" {
		return ends, false
	}
	vr, vu, _ := sketch.ViewDirection(v.Kind)
	rr, uu := vec(vr), vec(vu)
	if math.Abs(rr.Cross(uu).Dot(keep.N)) > 1e-9 {
		return ends, false
	}
	// In the view: a x + b y = D, its points' x and y the view's.
	a, b := keep.N.Dot(rr), keep.N.Dot(uu)
	base := geom.Vector{X: a * keep.D, Y: b * keep.D}
	dir := geom.Vector{X: -b, Y: a}
	size := sizes[v.Kind]
	half := math.Hypot(size[0], size[1])/2 + 2*cutReach/scale
	mid := mids[v.Kind]
	// From the view's middle along the line, clipped to the view's box
	// grown by cutReach.
	t0 := mid.Minus(base).Dot(dir)
	hw, hh := size[0]/2+cutReach/scale, size[1]/2+cutReach/scale
	for k, t := range []float64{t0 - half, t0 + half} {
		p := base.Plus(dir.ScaledBy(t))
		p.X = math.Max(mid.X-hw, math.Min(mid.X+hw, p.X))
		p.Y = math.Max(mid.Y-hh, math.Min(mid.Y+hh, p.Y))
		q := p.Minus(mid).ScaledBy(scale).Plus(geom.Vector{X: v.X, Y: v.Y})
		q.Z = 0
		ends[k] = q
	}
	return ends, true
}

// viewLabel is a view's name, centred under it.
type viewLabel struct {
	text string
	// x is its middle, y its baseline.
	x, y float64
}

const (
	// viewNameHeight is the height of a view's name.
	viewNameHeight = 3.0
	// nameGap is the space kept above a view's name moved down.
	nameGap = 2.0
)

func (l viewLabel) box() sheetBox {
	w := res.BuiltinVectorFont().Width(viewNameHeight, l.text)
	return sheetBox{lo: geom.Vector{X: l.x - w/2, Y: l.y}, hi: geom.Vector{X: l.x + w/2, Y: l.y + viewNameHeight}, set: true}
}

// drawViewLabels writes the views' names, each moved down below what
// of avoid it would cover, and returns their boxes.
func drawViewLabels(sheet *srf.SEdgeList, labels []viewLabel, avoid []sheetBox) []sheetBox {
	out := make([]sheetBox, 0, len(labels))
	for _, l := range labels {
		// Each move takes the name below a box, so this ends.
		for {
			i := slices.IndexFunc(avoid, l.box().grown(nameGap/2).overlaps)
			if i < 0 {
				break
			}
			l.y = avoid[i].lo.Y - nameGap - viewNameHeight
		}
		b := l.box()
		sheetText(sheet, l.text, viewNameHeight, b.lo)
		out = append(out, b)
	}
	return out
}
