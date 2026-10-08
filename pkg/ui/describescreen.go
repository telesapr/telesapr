package ui

// The screens shown when something is selected: a description of it,
// the endpoints of the lines, diameter of the circle, etc. (C++
// describescreen.cpp).

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ScreenUnselectAll clears the selection (C++
// TextWindow::ScreenUnselectAll).
func (tw *TextWindow) ScreenUnselectAll(link int, v uint32) {
	tw.ss.GW.MenuEdit(CmdUnselectAll)
}

// ScreenEditTtfText edits a text request's string (C++
// TextWindow::ScreenEditTtfText).
func (tw *TextWindow) ScreenEditTtfText(link int, v uint32) {
	hr := core.HRequest{V: v}
	r := tw.ss.SK.Request(hr)

	tw.ShowEditControl(10, r.Str)
	tw.edit.meaning = EditTTFText
	tw.edit.request = hr
}

// ScreenToggleTtfKerning turns a text request's kerning on or off (C++
// TextWindow::ScreenToggleTtfKerning).
func (tw *TextWindow) ScreenToggleTtfKerning(link int, v uint32) {
	ss := tw.ss
	r := ss.SK.Request(core.HRequest{V: v})

	ss.UndoRemember()
	if r.ExtraPoints != 0 {
		r.ExtraPoints = 0
	} else {
		r.ExtraPoints = 1
	}

	ss.SK.MarkGroupDirty(r.Group, false)
	ss.ScheduleShowTW()
}

// ScreenSetTtfFont sets the selected text's font (C++
// TextWindow::ScreenSetTtfFont).
func (tw *TextWindow) ScreenSetTtfFont(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	i := int(int32(v))
	fonts := ss.Fonts().List()
	if i < 0 || i >= len(fonts) {
		return
	}

	ss.GW.GroupSelection()
	gs := &ss.GW.gs
	if gs.entities != 1 || gs.n != 1 {
		return
	}

	e := sk.Entity(gs.entity[0])
	if e == nil || e.Type != core.EntityTTFText || !e.H.IsFromRequest() {
		return
	}

	r := sk.Request(e.H.Request())
	if r == nil {
		return
	}

	ss.UndoRemember()
	r.Font = fonts[i].FontFileBaseName()
	sk.MarkGroupDirty(r.Group, false)
	ss.ScheduleShowTW()
}

// ScreenConstraintToggleReference makes a dimension a reference, or
// not (C++ TextWindow::ScreenConstraintToggleReference).
func (tw *TextWindow) ScreenConstraintToggleReference(link int, v uint32) {
	ss := tw.ss
	c := ss.SK.Constraint(core.HConstraint{V: v})

	ss.UndoRemember()
	c.Reference = !c.Reference

	ss.SK.MarkGroupDirty(c.Group, false)
	ss.ScheduleShowTW()
}

// ScreenConstraintShowAsRadius shows a diameter as a radius, or not
// (C++ TextWindow::ScreenConstraintShowAsRadius).
func (tw *TextWindow) ScreenConstraintShowAsRadius(link int, v uint32) {
	ss := tw.ss
	c := ss.SK.Constraint(core.HConstraint{V: v})

	ss.UndoRemember()
	c.Other = !c.Other

	ss.ScheduleShowTW()
}

// projectInto projects p into workplane wrkpl (C++ Vector::ProjectInto).
func (ss *Application) projectInto(p geom.Vector, wrkpl core.HEntity) geom.Vector {
	w := ss.SK.MustEntity(wrkpl)
	p0 := w.WorkplaneGetOffset(ss.SK)

	f := p.Minus(p0)

	return p0.Plus(core.ProjectVectorInto(ss.SK, f, wrkpl))
}

// Point formats of a selection's description: a point's coordinates as
// strings or as numbers, and the same as a link to the entity.
const (
	ptAsStrNoLink = "(%Fi%s%Fd, %Fi%s%Fd, %Fi%s%Fd)"
	ptAsNum       = "(%Fi%3%Fd, %Fi%3%Fd, %Fi%3%Fd)"
	ptAsStr       = "%Ll%D%f%h" + ptAsStrNoLink + "%E"
	ptAsNumLink   = "%Ll%D%f%h" + ptAsNum + "%E"
)

// describer writes the description of what is selected to the text
// window.
type describer struct {
	tw *TextWindow
	ss *Application
	sk *sketch.Sketch
	gs *groupedSelection
}

// mm is a length in the sketch's units.
func (d *describer) mm(v float64) string { return d.ss.MmToString(v, false) }

// costrNoLink is a point's coordinates as strings, for ptAsStrNoLink.
func (d *describer) costrNoLink(p geom.Vector) []any {
	return []any{d.mm(p.X), d.mm(p.Y), d.mm(p.Z)}
}

// costr is a point's coordinates as strings with a link to entity e,
// for ptAsStr.
func (d *describer) costr(e *core.Entity, p geom.Vector) []any {
	return append([]any{e.H.V, lf(d.tw.ScreenSelectEntity), lf(d.tw.ScreenHoverEntity)}, d.costrNoLink(p)...)
}

// co is a point's coordinates as numbers, for ptAsNum.
func co(p geom.Vector) []any { return []any{p.X, p.Y, p.Z} }

// coLink is a point's coordinates as numbers with a link to entity e,
// for ptAsNumLink.
func (d *describer) coLink(e *core.Entity, p geom.Vector) []any {
	return append([]any{e.H.V, lf(d.tw.ScreenSelectEntity), lf(d.tw.ScreenHoverEntity)}, co(p)...)
}

// flatArgs joins format arguments, splicing in the lists among them.
func flatArgs(parts ...any) []any {
	var out []any
	for _, p := range parts {
		if l, ok := p.([]any); ok {
			out = append(out, l...)
		} else {
			out = append(out, p)
		}
	}
	return out
}

// parity is the colour code of a row: the rows alternate.
func parity(a int) byte {
	if a&1 != 0 {
		return 'd'
	}
	return 'a'
}

// ent is the entity with handle h, which the selection holds.
func (d *describer) ent(h core.HEntity) *core.Entity { return d.sk.MustEntity(h) }

// listFaces lists the selected faces' normals and points.
func (d *describer) listFaces() {
	tw, sk, gs := d.tw, d.sk, d.gs
	abc := byte('A')
	for _, fc := range gs.face {
		n := d.ent(fc).FaceGetNormalNum(sk)
		tw.Printf(true, " plane%c normal = "+ptAsNum, flatArgs(abc, co(n))...)
		p := d.ent(fc).FaceGetPointNum(sk)
		tw.Printf(false, "   plane%c thru = "+ptAsStr, flatArgs(abc, d.costr(d.ent(fc), p))...)
		abc++
	}
}

// DescribeSelection describes what is selected.
func (tw *TextWindow) DescribeSelection() {
	ss := tw.ss
	sk := ss.SK
	tw.Printf(false, "")
	gs := &ss.GW.gs
	d := &describer{tw: tw, ss: ss, sk: sk, gs: gs}
	d.describe()

	if tw.shown.screen == ScreenStyleInfo && tw.shown.style.V >= core.StyleFirstCustom && gs.stylables > 0 {
		// If we are showing a screen for a particular style, then offer
		// the option to assign our selected entities to that style.
		s := sk.StyleGet(tw.shown.style)
		tw.Printf(true, "%Fl%D%f%Ll(assign to style %s)%E",
			tw.shown.style.V, lf(tw.ScreenAssignSelectionToStyle), s.DescriptionString())
	}
	// If any of the selected entities have an assigned style, then offer
	// the option to remove that style.
	styleAssigned := false
	for i := range gs.entities {
		if d.ent(gs.entity[i]).Style.V != 0 {
			styleAssigned = true
		}
	}
	for i := range gs.constraints {
		c := sk.Constraint(gs.constraint[i])
		if c.Type == core.ConstraintComment && c.Disp.Style.V != 0 {
			styleAssigned = true
		}
	}
	if styleAssigned {
		tw.Printf(true, "%Fl%D%f%Ll(remove assigned style)%E", 0, lf(tw.ScreenAssignSelectionToStyle))
	}

	tw.Printf(true, "%Fl%f%Ll(unselect all)%E", lf(tw.ScreenUnselectAll))
}

// describe writes the description of the selection by its shape.
func (d *describer) describe() {
	tw, ss, gs := d.tw, d.ss, d.gs
	switch {
	case gs.n == 1 && (gs.points == 1 || gs.entities == 1):
		d.describeOne()

	case gs.n == 2 && gs.points == 2:
		d.describeTwoPoints()

	case gs.n == 2 && gs.points == 1 && gs.circlesOrArcs == 1:
		d.describePointAndCircle()

	case gs.n == 2 && gs.faces == 1 && gs.points == 1:
		d.describePointAndFace()

	case gs.n == 3 && gs.points == 2 && gs.vectors == 1:
		d.describeTwoPointsAndVector()

	case gs.n == 2 && gs.lineSegments == 1 && gs.points == 1:
		d.describePointAndLine()

	case gs.n == 2 && gs.vectors == 2:
		d.describeTwoVectors()

	case gs.n == 2 && gs.faces == 2:
		d.describeTwoFaces()

	case gs.n == 3 && gs.faces == 3:
		d.describeThreeFaces()

		// We should probably compute and show the intersection point if
		// there is one.

	case gs.n == 0 && gs.constraints == 0 && len(gs.solidEdge) == 1:
		d.describeSolidEdge()

	case gs.n == 0 && gs.constraints == 1:
		d.describeConstraint()

	default:
		n := len(ss.GW.selection)
		plural := "s"
		if n == 1 {
			plural = ""
		}
		tw.Printf(false, "%FtSELECTED:%E %d item%s", n, plural)
	}
}

// describeOne describes one selected entity or point.
func (d *describer) describeOne() {
	tw, ss, sk, gs := d.tw, d.ss, d.sk, d.gs
	var e *core.Entity
	if gs.points == 1 {
		e = d.ent(gs.point[0])
	} else {
		e = d.ent(gs.entity[0])
	}
	var p geom.Vector

	switch e.Type {
	case core.EntityPointIn3D, core.EntityPointIn2D, core.EntityPointNTrans,
		core.EntityPointNRotTrans, core.EntityPointNCopy, core.EntityPointNRotAA,
		core.EntityPointNRotAxisTrans:
		p = e.PointGetNum(sk)
		tw.Printf(false, "%FtPOINT%E at "+ptAsStr, d.costr(e, p)...)

	case core.EntityNormalIn3D, core.EntityNormalIn2D, core.EntityNormalNCopy,
		core.EntityNormalNRot, core.EntityNormalNRotAA:
		q := e.NormalGetNum(sk)
		p = q.RotationN()
		tw.Printf(false, "%FtNORMAL / COORDINATE SYSTEM%E")
		tw.Printf(true, "  basis n = "+ptAsNum, co(p)...)
		p = q.RotationU()
		tw.Printf(false, "        u = "+ptAsNum, co(p)...)
		p = q.RotationV()
		tw.Printf(false, "        v = "+ptAsNum, co(p)...)

	case core.EntityWorkplane:
		p = d.ent(e.Point[0]).PointGetNum(sk)
		tw.Printf(false, "%FtWORKPLANE%E")
		tw.Printf(true, "   origin = "+ptAsStr, d.costr(d.ent(e.Point[0]), p)...)
		normal := d.ent(e.Normal)
		q := normal.NormalGetNum(sk)
		p = q.RotationN()
		tw.Printf(true, "   normal = "+ptAsNumLink, d.coLink(normal, p)...)

	case core.EntityLineSegment:
		p0 := d.ent(e.Point[0]).PointGetNum(sk)
		p = p0
		tw.Printf(false, "%FtLINE SEGMENT%E")
		tw.Printf(true, "   thru "+ptAsStr, d.costr(d.ent(e.Point[0]), p)...)
		p1 := d.ent(e.Point[1]).PointGetNum(sk)
		p = p1
		tw.Printf(false, "        "+ptAsStr, d.costr(d.ent(e.Point[1]), p)...)
		tw.Printf(true, "   len = %Fi%s%E", d.mm(p1.Minus(p0).Magnitude()))

	case core.EntityCubicPeriodic, core.EntityCubic:
		var pts int
		switch {
		case e.Type == core.EntityCubicPeriodic:
			tw.Printf(false, "%FtPERIODIC C2 CUBIC SPLINE%E")
			pts = 3 + e.ExtraPoints
		case e.ExtraPoints > 0:
			tw.Printf(false, "%FtINTERPOLATING C2 CUBIC SPLINE%E")
			pts = 4 + e.ExtraPoints
		default:
			tw.Printf(false, "%FtCUBIC BEZIER CURVE%E")
			pts = 4
		}
		for i := range pts {
			p = d.ent(e.Point[i]).PointGetNum(sk)
			tw.Printf(i == 0, "   p%d = "+ptAsStr, flatArgs(i, d.costr(d.ent(e.Point[i]), p))...)
		}

	case core.EntityArcOfCircle:
		tw.Printf(false, "%FtARC OF A CIRCLE%E")
		p = d.ent(e.Point[0]).PointGetNum(sk)
		tw.Printf(true, "     center = "+ptAsStr, d.costr(d.ent(e.Point[0]), p)...)
		p = d.ent(e.Point[1]).PointGetNum(sk)
		tw.Printf(true, "  endpoints = "+ptAsStr, d.costr(d.ent(e.Point[1]), p)...)
		p = d.ent(e.Point[2]).PointGetNum(sk)
		tw.Printf(false, "              "+ptAsStr, d.costr(d.ent(e.Point[2]), p)...)
		r := e.CircleGetRadiusNum(sk)
		tw.Printf(true, "   diameter =  %Fi%s", d.mm(r*2))
		tw.Printf(false, "     radius =  %Fi%s", d.mm(r))
		_, _, dtheta := e.ArcGetAngles(sk)
		tw.Printf(false, "    arc len =  %Fi%s", d.mm(dtheta*r))

	case core.EntityCircle:
		tw.Printf(false, "%FtCIRCLE%E")
		p = d.ent(e.Point[0]).PointGetNum(sk)
		tw.Printf(true, "        center = "+ptAsStr, d.costr(d.ent(e.Point[0]), p)...)
		r := e.CircleGetRadiusNum(sk)
		tw.Printf(true, "      diameter =  %Fi%s", d.mm(r*2))
		tw.Printf(false, "        radius =  %Fi%s", d.mm(r))
		tw.Printf(false, " circumference =  %Fi%s", d.mm(2*math.Pi*r))

	case core.EntityFaceNormalPt, core.EntityFaceXProd, core.EntityFaceNRotTrans,
		core.EntityFaceNRotAA, core.EntityFaceNTrans, core.EntityFaceRotNormalPt,
		core.EntityFaceNRotAxisTrans:
		tw.Printf(false, "%FtPLANE FACE%E")
		p = e.FaceGetNormalNum(sk)
		tw.Printf(true, "   normal = "+ptAsNum, co(p)...)
		p = e.FaceGetPointNum(sk)
		tw.Printf(false, "     thru = "+ptAsStr, d.costr(e, p)...)

	case core.EntityTTFText:
		tw.Printf(false, "%FtTRUETYPE FONT TEXT%E")
		tw.Printf(true, "  font = '%Fi%s%E'", e.Font)
		if e.H.IsFromRequest() {
			tw.Printf(true, "  text = '%Fi%s%E' %Fl%Ll%f%D[change]%E",
				e.Str, lf(tw.ScreenEditTtfText), e.H.Request().V)
			tw.Printf(true, "  %Fd%f%D%Ll%s  apply kerning",
				lf(tw.ScreenToggleTtfKerning), e.H.Request().V, checkMark(e.ExtraPoints != 0))
			tw.Printf(true, "  select new font")
			ss.Fonts().LoadAll()
			fonts := ss.Fonts().List()
			for i := range fonts {
				tf := &fonts[i]
				if e.Font == tf.FontFileBaseName() {
					tw.Printf(false, "%Bp    %s", parity(i), tf.Name)
				} else {
					tw.Printf(false, "%Bp    %f%D%Fl%Ll%s%E%Bp",
						parity(i), lf(tw.ScreenSetTtfFont), i, tf.Name, parity(i))
				}
			}
		} else {
			tw.Printf(false, "  text = '%Fi%s%E'", e.Str)
		}

	case core.EntityImage:
		tw.Printf(false, "%FtIMAGE%E")
		tw.Printf(true, "  file = '%Fi%s%E'", ss.linkDisplayPath(e.File))

	default:
		tw.Printf(true, "%Ft?? ENTITY%E")
	}

	tw.Printf(false, "")
	if e.H.IsFromRequest() {
		r := sk.Request(e.H.Request())
		if e.H == r.H.Entity(0) {
			tw.Printf(false, "%FtFROM REQUEST%E  %s", r.DescriptionString())
		} else {
			tw.Printf(false, "%FtFROM REQUEST%E  %Fl%Ll%D%f%h%s%E",
				r.H.V, lf(tw.ScreenSelectRequest), lf(tw.ScreenHoverRequest),
				r.DescriptionString())
		}
	}
	g := sk.Group(e.Group)
	tw.Printf(false, "%FtIN GROUP%E      %Fl%Ll%D%f%s%E",
		g.H.V, lf(tw.ScreenSelectGroup), groupDescription(g))
	if core.IsFreeIn3D(e.Workplane) {
		tw.Printf(false, "%FtNOT LOCKED IN WORKPLANE%E")
	} else {
		w := d.ent(e.Workplane)
		if w.H.IsFromRequest() {
			tw.Printf(false, "%FtIN WORKPLANE%E  %Fl%Ll%D%f%h%s%E",
				w.H.Request().V, lf(tw.ScreenSelectRequest), lf(tw.ScreenHoverRequest),
				ss.entityDescription(w))
		} else {
			tw.Printf(false, "%FtIN WORKPLANE%E  %Fl%Ll%D%f%h%s%E",
				w.H.Group().V, lf(tw.ScreenSelectGroup), lf(tw.ScreenHoverGroupWorkplane),
				ss.entityDescription(w))
		}
	}
	if entityIsStylable(e) {
		if e.Style.V != 0 {
			s := sk.StyleGet(e.Style)
			tw.Printf(false, "%FtIN STYLE%E      %Fl%Ll%D%f%s%E",
				s.H.V, lf(tw.ScreenShowStyleInfo), s.DescriptionString())
		} else {
			tw.Printf(false, "%FtIN STYLE%E      none")
		}
	}
	if e.Construction {
		tw.Printf(false, "%FtCONSTRUCTION")
	}

	var lhc []core.HConstraint
	findConstraints := func(he core.HEntity) {
		for i := range sk.Constraints {
			c := &sk.Constraints[i]
			if c.PtA != he && c.PtB != he && c.EntityA != he && c.EntityB != he &&
				c.EntityC != he && c.EntityD != he {
				continue
			}
			lhc = append(lhc, c.H)
		}
	}
	findConstraints(e.H)
	if !e.IsPoint() {
		for i := range core.MaxPointsInEntity {
			if e.Point[i].V == 0 {
				break
			}
			findConstraints(e.Point[i])
		}
	}

	slices.SortFunc(lhc, func(a, b core.HConstraint) int {
		switch {
		case a.V < b.V:
			return -1
		case a.V > b.V:
			return 1
		}
		return 0
	})
	lhc = slices.Compact(lhc)

	listConstraints := func(reference bool) {
		first := true
		a := 0
		for _, hc := range lhc {
			c := sk.Constraint(hc)
			if c.Reference != reference {
				continue
			}
			if first {
				first = false
				if reference {
					tw.Printf(true, "%FtMEASURED BY:%E")
				} else {
					tw.Printf(true, "%FtCONSTRAINED BY:%E")
				}
			}
			tw.Printf(false, "%Bp   %Fl%Ll%D%f%h%s%E",
				parity(a), c.H.V, lf(tw.ScreenSelectConstraint), lf(tw.ScreenHoverConstraint),
				c.DescriptionString())
			a++
		}
	}
	listConstraints(false)
	listConstraints(true)
}

// describeTwoPoints describes two selected points and their distance.
func (d *describer) describeTwoPoints() {
	tw, sk, gs := d.tw, d.sk, d.gs
	tw.Printf(false, "%FtTWO POINTS")
	p0 := d.ent(gs.point[0]).PointGetNum(sk)
	tw.Printf(true, "   at "+ptAsStr, d.costr(d.ent(gs.point[0]), p0)...)
	p1 := d.ent(gs.point[1]).PointGetNum(sk)
	tw.Printf(false, "      "+ptAsStr, d.costr(d.ent(gs.point[1]), p1)...)
	dv := p1.Minus(p0)
	tw.Printf(true, "  d = %Fi%s", d.mm(dv.Magnitude()))
	tw.Printf(false, "  d(x, y, z) = "+ptAsStrNoLink, d.costrNoLink(dv)...)
}

// describePointAndCircle describes a selected point and circle.
func (d *describer) describePointAndCircle() {
	tw, sk, gs := d.tw, d.sk, d.gs
	ec := d.ent(gs.entity[0])
	switch ec.Type {
	case core.EntityCircle:
		tw.Printf(false, "%FtPOINT AND A CIRCLE")
	case core.EntityArcOfCircle:
		tw.Printf(false, "%FtPOINT AND AN ARC")
	default:
		panic("ui: unexpected entity type")
	}
	p := d.ent(gs.point[0]).PointGetNum(sk)
	tw.Printf(true, "        pt at "+ptAsStr, d.costr(d.ent(gs.point[0]), p)...)
	c := d.ent(ec.Point[0]).PointGetNum(sk)
	tw.Printf(true, "     center = "+ptAsStr, d.costr(d.ent(ec.Point[0]), c)...)
	r := ec.CircleGetRadiusNum(sk)
	tw.Printf(false, "   diameter =  %Fi%s", d.mm(r*2))
	tw.Printf(false, "     radius =  %Fi%s", d.mm(r))
	dist := p.Minus(c).Magnitude() - r
	tw.Printf(true, "   distance = %Fi%s", d.mm(dist))
}

// describePointAndFace describes a selected point and face.
func (d *describer) describePointAndFace() {
	tw, sk, gs := d.tw, d.sk, d.gs
	tw.Printf(false, "%FtA POINT AND A PLANE FACE")
	pt := d.ent(gs.point[0]).PointGetNum(sk)
	tw.Printf(true, "        point = "+ptAsStr, d.costr(d.ent(gs.point[0]), pt)...)
	n := d.ent(gs.face[0]).FaceGetNormalNum(sk)
	tw.Printf(true, " plane normal = "+ptAsNum, co(n)...)
	pl := d.ent(gs.face[0]).FaceGetPointNum(sk)
	tw.Printf(false, "   plane thru = "+ptAsStr, d.costr(d.ent(gs.face[0]), pl)...)
	dd := n.Dot(pl) - n.Dot(pt)
	tw.Printf(true, "     distance = %Fi%s", d.mm(dd))
}

// describeTwoPointsAndVector describes two selected points and a direction.
func (d *describer) describeTwoPointsAndVector() {
	tw, sk, gs := d.tw, d.sk, d.gs
	tw.Printf(false, "%FtTWO POINTS AND A VECTOR")
	p0 := d.ent(gs.point[0]).PointGetNum(sk)
	tw.Printf(true, "  pointA = "+ptAsStr, d.costr(d.ent(gs.point[0]), p0)...)
	p1 := d.ent(gs.point[1]).PointGetNum(sk)
	tw.Printf(false, "  pointB = "+ptAsStr, d.costr(d.ent(gs.point[1]), p1)...)
	v := d.ent(gs.vector[0]).VectorGetNum(sk)
	v = v.WithMagnitude(1)
	tw.Printf(true, "  vector = "+ptAsNumLink, d.coLink(d.ent(gs.vector[0]), v)...)
	dist := p1.Minus(p0).Dot(v)
	tw.Printf(true, "  proj_d = %Fi%s", d.mm(dist))
}

// describePointAndLine describes a selected point and line.
func (d *describer) describePointAndLine() {
	tw, ss, sk, gs := d.tw, d.ss, d.sk, d.gs
	ln := d.ent(gs.entity[0])
	lp0 := d.ent(ln.Point[0]).PointGetNum(sk)
	lp1 := d.ent(ln.Point[1]).PointGetNum(sk)
	tw.Printf(false, "%FtLINE SEGMENT AND POINT%E")
	tw.Printf(true, "   ln thru "+ptAsStr, d.costr(d.ent(ln.Point[0]), lp0)...)
	tw.Printf(false, "           "+ptAsStr, d.costr(d.ent(ln.Point[1]), lp1)...)
	p := d.ent(gs.point[0])
	pp := p.PointGetNum(sk)
	tw.Printf(true, "     point "+ptAsStr, d.costr(p, pp)...)
	tw.Printf(true, " pt-ln distance = %Fi%s%E", d.mm(pp.DistanceToLine(lp0, lp1.Minus(lp0))))
	wrkpl := ss.GW.ActiveWorkplane()
	if !core.IsFreeIn3D(wrkpl) && (p.Workplane != wrkpl || ln.Workplane != wrkpl) {
		ppw := ss.projectInto(pp, wrkpl)
		lp0w := ss.projectInto(lp0, wrkpl)
		lp1w := ss.projectInto(lp1, wrkpl)
		tw.Printf(false, "    or distance = %Fi%s%E (in workplane)",
			d.mm(ppw.DistanceToLine(lp0w, lp1w.Minus(lp0w))))
	}
}

// describeTwoVectors describes two selected directions and their angle.
func (d *describer) describeTwoVectors() {
	tw, sk, gs := d.tw, d.sk, d.gs
	tw.Printf(false, "%FtTWO VECTORS")

	v0 := d.ent(gs.entity[0]).VectorGetNum(sk)
	v1 := d.ent(gs.entity[1]).VectorGetNum(sk)
	v0 = v0.WithMagnitude(1)
	v1 = v1.WithMagnitude(1)

	tw.Printf(true, "  vectorA = "+ptAsNumLink, d.coLink(d.ent(gs.entity[0]), v0)...)
	tw.Printf(false, "  vectorB = "+ptAsNumLink, d.coLink(d.ent(gs.entity[1]), v1)...)

	theta := libm.Acos(v0.Dot(v1))
	tw.Printf(true, "    angle = %Fi%2%E degrees", theta*180/math.Pi)
	for theta < math.Pi/2 {
		theta += math.Pi
	}
	for theta > math.Pi/2 {
		theta -= math.Pi
	}
	tw.Printf(false, " or angle = %Fi%2%E (mod 180)", theta*180/math.Pi)
}

// describeTwoFaces describes two selected faces.
func (d *describer) describeTwoFaces() {
	tw, sk, gs := d.tw, d.sk, d.gs
	tw.Printf(false, "%FtTWO PLANE FACES")

	d.listFaces()

	n0 := d.ent(gs.face[0]).FaceGetNormalNum(sk)
	n1 := d.ent(gs.face[1]).FaceGetNormalNum(sk)

	theta := libm.Acos(n0.Dot(n1))
	tw.Printf(true, "         angle = %Fi%2%E degrees", theta*180/math.Pi)
	for theta < math.Pi/2 {
		theta += math.Pi
	}
	for theta > math.Pi/2 {
		theta -= math.Pi
	}
	tw.Printf(false, "      or angle = %Fi%2%E (mod 180)", theta*180/math.Pi)

	if math.Abs(theta) < 0.01 {
		p0 := d.ent(gs.face[0]).FaceGetPointNum(sk)
		p1 := d.ent(gs.face[1]).FaceGetPointNum(sk)

		dist := p1.Minus(p0).Dot(n0)
		tw.Printf(true, "      distance = %Fi%s", d.mm(dist))
	}
}

// describeThreeFaces describes three selected faces.
func (d *describer) describeThreeFaces() {
	tw := d.tw
	tw.Printf(false, "%FtTHREE PLANE FACES")

	d.listFaces()
}

// describeSolidEdge describes a selected edge of the solid.
func (d *describer) describeSolidEdge() {
	tw, ss, sk, gs := d.tw, d.ss, d.sk, d.gs
	n := gs.solidEdge[0]
	tw.Printf(false, "%FtSOLID EDGE%E  %s", n.String())
	for k, line := range wrapWords(sk.SolidEdgeName(n), MaxCols-4) {
		if k == 0 {
			tw.Printf(true, "  %s", line)
		} else {
			tw.Printf(false, "    %s", line)
		}
	}
	if tp := ss.GW.activeTopology(); tp != nil {
		if e, err := tp.Resolve(n); err == nil {
			ed := &tp.Edges[e]
			tw.Printf(true, "  from "+ptAsStrNoLink, d.costrNoLink(tp.Vertices[ed.V[0]].P)...)
			tw.Printf(false, "  to   "+ptAsStrNoLink, d.costrNoLink(tp.Vertices[ed.V[1]].P)...)
			tw.Printf(true, "  length = %Fi%s%E", d.mm(tp.Length(e)))
			if !tp.Straight(e) {
				tw.Printf(false, "  (curved)")
			}
		}
	}
}

// describeConstraint describes one selected constraint.
func (d *describer) describeConstraint() {
	tw, ss, sk, gs := d.tw, d.ss, d.sk, d.gs
	c := sk.Constraint(gs.constraint[0])
	desc := c.DescriptionString()

	switch {
	case c.Type == core.ConstraintComment:
		tw.Printf(false, "%FtCOMMENT%E  %s", desc)
		if c.PtA.V != 0 {
			p := d.ent(c.PtA).PointGetNum(sk)
			tw.Printf(true, "  attached to point at: "+ptAsStr, d.costr(d.ent(c.PtA), p)...)
			dv := c.Disp.Offset
			tw.Printf(false, "    distance = %Fi%s", d.mm(dv.Magnitude()))
			tw.Printf(false, "  d(x, y, z) = "+ptAsStrNoLink, d.costrNoLink(dv)...)
		}
	case c.HasLabel():
		if c.Reference {
			tw.Printf(false, "%FtREFERENCE%E  %s", desc)
		} else {
			tw.Printf(false, "%FtDIMENSION%E  %s", desc)
		}
		tw.Printf(true, "  %Fd%f%D%Ll%s  reference",
			lf(tw.ScreenConstraintToggleReference), gs.constraint[0].V, checkMark(c.Reference))
		if c.Type == core.ConstraintDiameter {
			tw.Printf(false, "  %Fd%f%D%Ll%s  use radius",
				lf(tw.ScreenConstraintShowAsRadius), gs.constraint[0].V, checkMark(c.Other))
		}
	default:
		tw.Printf(false, "%FtCONSTRAINT%E  %s", desc)
	}

	if c.IsProjectible() {
		if core.IsFreeIn3D(c.Workplane) {
			tw.Printf(true, "%FtNOT PROJECTED TO WORKPLANE%E")
		} else {
			w := d.ent(c.Workplane)
			if w.H.IsFromRequest() {
				tw.Printf(true, "%FtIN WORKPLANE%E  %Fl%Ll%D%f%h%s%E",
					w.H.Request().V, lf(tw.ScreenSelectRequest), lf(tw.ScreenHoverRequest),
					ss.entityDescription(w))
			} else {
				tw.Printf(true, "%FtIN WORKPLANE%E  %Fl%Ll%D%f%h%s%E",
					w.H.Group().V, lf(tw.ScreenSelectGroup), lf(tw.ScreenHoverGroupWorkplane),
					ss.entityDescription(w))
			}
		}
	}

	var lhe []core.HEntity
	for _, he := range []core.HEntity{c.PtA, c.PtB, c.EntityA, c.EntityB, c.EntityC, c.EntityD} {
		if he.V == 0 || !he.IsFromRequest() {
			continue
		}
		lhe = append(lhe, he)
	}

	if len(lhe) > 0 {
		if c.Reference {
			tw.Printf(true, "%FtMEASURES:%E")
		} else {
			tw.Printf(true, "%FtCONSTRAINS:%E")
		}

		for a, he := range lhe {
			e := d.ent(he)
			tw.Printf(false, "%Bp   %Fl%Ll%D%f%h%s%E",
				parity(a), e.H.V, lf(tw.ScreenSelectEntity), lf(tw.ScreenHoverEntity),
				ss.entityDescription(e))
		}
	}
}
