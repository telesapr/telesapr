package ui

// The section view (an extension): the active group's solid drawn cut
// by a plane, with the cut capped, its menu command and its screen.
// The state is the sketch's (Sketch.Section), so that scripts set it
// the same way.

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// clipCanvas is a canvas that can clip by a plane, as both renderers
// can; the pickers and export canvases draw everything.
type clipCanvas interface {
	SetClipPlane(h *srf.HalfSpace)
}

// sectionCapColor is the cap's colour by default: the SectionCap
// style's, which the user can change.
var sectionCapColor = core.NewRGBA(205, 100, 80, 255)

// sectionHatchPixels is how far apart the cap's hatch lines are on the
// screen.
const sectionHatchPixels = 10

// sectionCap is the cap last computed, kept while the mesh and the
// plane stay the same.
type sectionCap struct {
	mesh       *srf.SMesh
	triangles  int
	generation uint64
	keep       srf.HalfSpace
	poly       srf.SPolygon
	closed     bool
}

// sectionKeep is the half space the section view keeps; ok is false
// when it is off or can't cut.
func (gw *graphicsWindow) sectionKeep() (keep srf.HalfSpace, ok bool) {
	keep, _, ok, _ = gw.sk().SectionCut()
	return keep, ok
}

// capFor is the cap of g's display mesh by keep's plane.
func (gw *graphicsWindow) capFor(g *sketch.Group, keep srf.HalfSpace) *sectionCap {
	c := &gw.sectionCap
	m := &g.DisplayMesh
	if c.mesh == m && c.triangles == len(m.Triangles) && c.generation == gw.sk().Generation && c.keep == keep {
		return c
	}
	c.mesh, c.triangles, c.generation, c.keep = m, len(m.Triangles), gw.sk().Generation, keep
	c.closed = m.MakeSectionPolygonInto(keep, &c.poly)
	return c
}

// clipToSection makes canvas keep only what the section view keeps,
// if it is on; the returned function undoes it.
func (gw *graphicsWindow) clipToSection(canvas render.Canvas) func() {
	cc, ok := canvas.(clipCanvas)
	if !ok {
		return func() {}
	}
	keep, ok := gw.sectionKeep()
	if !ok {
		return func() {}
	}
	cc.SetClipPlane(&keep)
	return func() { cc.SetClipPlane(nil) }
}

// drawSectionCap fills the cut of g's solid and outlines it.
func (gw *graphicsWindow) drawSectionCap(g *sketch.Group, canvas render.Canvas) {
	if _, ok := canvas.(clipCanvas); !ok {
		return
	}
	keep, ok := gw.sectionKeep()
	if !ok || g.DisplayMesh.IsEmpty() {
		return
	}
	c := gw.capFor(g, keep)
	if len(c.poly.Contours) == 0 {
		return
	}
	capStyle := gw.ss.styleStroke(hstyle(core.StyleSectionCap))
	if gw.showShaded {
		canvas.DrawPolygon(&c.poly, canvas.InternFill(render.Fill{Color: capStyle.Color}))
	}
	// The hatch: lines at 45 degrees across the cap, in the style's
	// colour darkened, so that a cut reads as one in print too.
	hatch := capStyle
	hatch.Color = core.NewRGBA(capStyle.Color.Red/2, capStyle.Color.Green/2, capStyle.Color.Blue/2, 255)
	hatch.ZIndex = 1
	var hl srf.SEdgeList
	hatchLines(&c.poly, keep.N, sectionHatchPixels/gw.view.Scale, &hl)
	canvas.DrawEdges(&hl, canvas.InternStroke(hatch))
	stroke := gw.ss.styleStroke(hstyle(core.StyleSolidEdge))
	stroke.ZIndex = 1
	hcs := canvas.InternStroke(stroke)
	var el srf.SEdgeList
	c.poly.MakeEdgesInto(&el)
	canvas.DrawEdges(&el, hcs)
}

// MenuSectionView turns the section view off, or on, cut parallel to
// the selected face or workplane, or the active workplane when nothing
// is selected.
func (gw *graphicsWindow) MenuSectionView() {
	ss := gw.ss
	sk := gw.sk()
	if sk.Section != nil {
		sk.ClearSection()
		gw.sectionChanged()
		return
	}
	gw.GroupSelection()
	gs := &gw.gs
	var plane core.HEntity
	switch {
	case gs.n == 1 && gs.faces == 1:
		plane = gs.face[0]
	case gs.n == 1 && gs.workplanes == 1:
		plane = gs.entity[0]
	case gs.n == 0 && gw.LockedInWorkplane():
		plane = gw.ActiveWorkplane()
	default:
		ss.Error("Bad selection for a section view. Select:\n\n" +
			"    * a face (cut parallel to it)\n" +
			"    * a workplane (cut in it)\n" +
			"    * nothing, with a workplane active (cut in it)\n")
		return
	}
	if err := sk.SetSection(plane, "", false); err != nil {
		ss.Report(err)
		return
	}
	gw.ClearSelection()
	gw.sectionChanged()
	ss.TW.GoToScreen(screenSection)
}

// syncSectionMenu checks View > Section View when the view is on, which
// a script or a file load may have changed.
func (gw *graphicsWindow) syncSectionMenu() {
	on := gw.sk().Section != nil
	if gw.sectionMenuItem != nil && on != gw.sectionMenuOn {
		gw.sectionMenuItem.SetActive(on)
		gw.sectionMenuOn = on
	}
}

// sectionChanged redraws for a new section, and shows its state.
func (gw *graphicsWindow) sectionChanged() {
	gw.syncSectionMenu()
	gw.persistentDirty = true
	gw.Invalidate(false)
	gw.ss.ScheduleShowTW()
}

// ShowSection is the section view's screen.
func (tw *textWindow) ShowSection() {
	ss := tw.ss
	sk := ss.SK
	s := sk.Section
	tw.Printf(true, "%FtSECTION VIEW%E")
	if s == nil {
		tw.Printf(true, "The section view is off. Select a face or a workplane and")
		tw.Printf(false, "choose View > Section View to cut the solid there.")
		tw.Printf(true, "%Fl%Ll%f(back to home screen)%E", lf(tw.ScreenHome))
		return
	}
	tw.Printf(true, "%Ba   %Ftparallel to%E %s", sk.SectionPlaneName())
	offset := s.Offset
	if offset == "" {
		offset = "0"
	}
	tw.Printf(false, "%Bd   %Ftoffset%E      %s %Fl%Ll%f[change]%E", offset, lf(tw.ScreenChangeSectionOffset))
	tw.Printf(false, "%Ba   %Fd%f%Ll%s  keep the other side%E", lf(tw.ScreenChangeSectionFlip), checkMark(s.Flip))
	if _, _, ok, err := sk.SectionCut(); !ok && err != nil {
		tw.Printf(true, "%FxThe section can't cut: %s.%E", err.Error())
	} else if g := sk.Group(ss.GW.activeGroupH()); g != nil && ss.GW.sectionCap.mesh == &g.DisplayMesh &&
		len(ss.GW.sectionCap.poly.Contours) > 0 && !ss.GW.sectionCap.closed {
		tw.Printf(true, "%FxThe solid is not closed where it is cut, so the cap may be partial.%E")
	}
	tw.Printf(true, "%Fl%Ll%fturn the section view off%E", lf(tw.ScreenSectionOff))
	tw.Printf(false, "%Fl%Ll%f(back to home screen)%E", lf(tw.ScreenHome))
}

func (tw *textWindow) ScreenChangeSectionOffset(link int, v uint32) {
	s := tw.ss.SK.Section
	if s == nil {
		return
	}
	text := s.Offset
	if text == "" {
		text = "0"
	}
	tw.ShowEditControl(17, text)
	tw.edit.meaning = editSectionOffset
}

func (tw *textWindow) ScreenChangeSectionFlip(link int, v uint32) {
	sk := tw.ss.SK
	if s := sk.Section; s != nil {
		if err := sk.SetSection(s.Plane, s.Offset, !s.Flip); err != nil {
			tw.ss.Report(err)
		}
	}
	tw.ss.GW.sectionChanged()
}

func (tw *textWindow) ScreenSectionOff(link int, v uint32) {
	tw.ss.SK.ClearSection()
	tw.ss.GW.sectionChanged()
}

// EditControlDoneForSection takes the edited offset, and reports
// whether the edit was the section's.
func (tw *textWindow) EditControlDoneForSection(s string) bool {
	if tw.edit.meaning != editSectionOffset {
		return false
	}
	sk := tw.ss.SK
	if sec := sk.Section; sec != nil {
		if err := sk.SetSection(sec.Plane, s, sec.Flip); err != nil {
			tw.ss.Report(err)
		}
	}
	tw.ss.GW.sectionChanged()
	return true
}

// hatchLines adds to el the parts inside poly, a set of loops in a
// plane of normal n, of lines at 45 degrees across it, spacing apart:
// each line is cut where it crosses the loops, and kept between every
// other crossing.
func hatchLines(poly *srf.SPolygon, n geom.Vector, spacing float64, el *srf.SEdgeList) {
	if spacing <= 0 || len(poly.Contours) == 0 {
		return
	}
	n = n.WithMagnitude(1)
	u := n.NormalDir(0).WithMagnitude(1)
	v := n.Cross(u)
	dir := u.Plus(v).WithMagnitude(1)
	across := n.Cross(dir)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, c := range poly.Contours {
		for _, p := range c.Points {
			d := p.P.Dot(across)
			lo, hi = math.Min(lo, d), math.Max(hi, d)
		}
	}
	if hi-lo > 1e4*spacing {
		spacing = (hi - lo) / 1e4
	}
	for d := math.Ceil(lo/spacing) * spacing; d <= hi; d += spacing {
		var at []float64
		var base geom.Vector
		found := false
		for _, c := range poly.Contours {
			for i := range c.Points {
				a, b := c.Points[i].P, c.Points[(i+1)%len(c.Points)].P
				da, db := a.Dot(across)-d, b.Dot(across)-d
				// Half open, so a crossing at a vertex counts once.
				if (da < 0) == (db < 0) {
					continue
				}
				p := a.Plus(b.Minus(a).ScaledBy(da / (da - db)))
				if !found {
					base, found = p, true
				}
				at = append(at, p.Minus(base).Dot(dir))
			}
		}
		slices.Sort(at)
		for k := 0; k+1 < len(at); k += 2 {
			el.AddEdge(base.Plus(dir.ScaledBy(at[k])), base.Plus(dir.ScaledBy(at[k+1])), 0, 0, 0)
		}
	}
}
