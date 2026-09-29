package ui

import (
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/libm"
	"github.com/spacesolve/spacesolve/pkg/render"
	"github.com/spacesolve/spacesolve/pkg/res"
)

// ConstraintDrawAs is how a constraint is drawn (C++
// Constraint::DrawAs).
type ConstraintDrawAs int

const (
	ConstraintDrawDefault ConstraintDrawAs = iota
	ConstraintDrawHovered
	ConstraintDrawSelected
)

// constraintIsStylable is Constraint::IsStylable: only comments.
func constraintIsStylable(c *core.Constraint) bool {
	return c.Type == core.ConstraintComment
}

// constraintStyle is Constraint::GetStyle.
func constraintStyle(c *core.Constraint) core.HStyle {
	if c.Disp.Style.V != 0 {
		return c.Disp.Style
	}
	return hstyle(core.StyleConstraint)
}

// constraintLabel is Constraint::Label.
func (ss *SolveSpaceUI) constraintLabel(c *core.Constraint) string {
	var result string
	switch c.Type {
	case core.ConstraintAngle:
		result = ss.DegreeToString(c.ValA) + "°"
	case core.ConstraintLengthRatio, core.ConstraintArcArcLenRatio, core.ConstraintArcLineLenRatio:
		result = fmt.Sprintf("%.3f:1", c.ValA)
	case core.ConstraintComment:
		result = c.Comment
	case core.ConstraintDiameter:
		if !c.Other {
			result = "⌀" + ss.MmToStringSI(c.ValA, 0)
		} else {
			result = "R" + ss.MmToStringSI(c.ValA/2, 0)
		}
	default:
		// valA has units of distance
		result = ss.MmToStringSI(math.Abs(c.ValA), 0)
	}
	if c.Reference {
		result += " REF"
	}
	return result
}

// constraintIsVisible is Constraint::IsVisible.
func (gw *GraphicsWindow) constraintIsVisible(c *core.Constraint) bool {
	if gw.showConstraints == SCMNoShow {
		return false
	}
	isDim := false
	if gw.showConstraints == SCMShowDim {
		switch c.Type {
		case core.ConstraintAngle, core.ConstraintDiameter, core.ConstraintPtPtDistance,
			core.ConstraintPtFaceDistance, core.ConstraintPtLineDistance, core.ConstraintPtPlaneDistance:
			isDim = true
		}
	}
	if gw.showConstraints == SCMShowAll || isDim {
		sk := gw.sk()
		g := sk.GetGroup(c.Group)
		// If the group is hidden, then the constraints are hidden and not
		// able to be selected.
		if !g.Visible {
			return false
		}
		// And likewise if the group is not the active group; except for
		// comments with an assigned style.
		if g.H != gw.activeGroupH() && !(c.Type == core.ConstraintComment && c.Disp.Style.V != 0) {
			return false
		}
		if c.Disp.Style.V != 0 {
			if s := sk.StyleGet(c.Disp.Style); !s.Visible {
				return false
			}
		}
		return true
	}
	return false
}

func (gw *GraphicsWindow) constraintShouldDrawExploded(c *core.Constraint) bool {
	return gw.groupShouldDrawExploded(gw.sk().GetGroup(c.Group))
}

// constraintDrawer lays out and draws one constraint (the methods of
// C++ Constraint that take a canvas).
type constraintDrawer struct {
	gw     *GraphicsWindow
	c      *core.Constraint
	canvas render.Canvas
	camera *render.Camera
}

func (d *constraintDrawer) projectInto(v geom.Vector, wrkpl core.HEntity) geom.Vector {
	sk := d.gw.sk()
	w := sk.GetEntity(wrkpl)
	p0 := w.WorkplaneGetOffset(sk)
	f := v.Minus(p0)
	return p0.Plus(core.ProjectVectorInto(sk, f, wrkpl))
}

func (d *constraintDrawer) doLine(hcs render.HStroke, a, b geom.Vector) {
	a = d.camera.AlignToPixelGrid(a)
	b = d.camera.AlignToPixelGrid(b)
	d.canvas.DrawLine(a, b, hcs)
}

func (d *constraintDrawer) doStippledLine(hcs render.HStroke, a, b geom.Vector) {
	strokeStippled := *d.canvas.Stroke(hcs)
	strokeStippled.StipplePattern = core.StippleShortDash
	strokeStippled.StippleScale = 4.0
	d.doLine(d.canvas.GetStroke(strokeStippled), a, b)
}

// doLabel draws the label centred at ref; labelPos gets its corner.
func (d *constraintDrawer) doLabel(hcs render.HStroke, ref geom.Vector, labelPos *geom.Vector, gr, gu geom.Vector) {
	ss := d.gw.ss
	c := d.c
	s := ss.constraintLabel(c)
	textHeight := ss.styleTextHeight(constraintStyle(c)) / d.camera.Scale
	vf := res.BuiltinVectorFont()
	swidth := vf.GetWidth(textHeight, s)
	sheight := vf.GetCapHeight(textHeight)

	// By default, the reference is from the center; but the style could
	// specify otherwise if one is present, and it could also specify a
	// rotation.
	if c.Type == core.ConstraintComment && c.Disp.Style.V != 0 {
		st := ss.SK.StyleGet(c.Disp.Style)
		// rotation first
		rads := st.TextAngle * math.Pi / 180
		cs, sn := libm.Cos(rads), libm.Sin(rads)
		pr, pu := gr, gu
		gr = pr.ScaledBy(cs).Plus(pu.ScaledBy(sn))
		gu = pr.ScaledBy(-sn).Plus(pu.ScaledBy(cs))
		// then origin
		o := st.TextOrigin
		if o&core.TextOriginLeft != 0 {
			ref = ref.Plus(gr.WithMagnitude(swidth / 2))
		}
		if o&core.TextOriginRight != 0 {
			ref = ref.Minus(gr.WithMagnitude(swidth / 2))
		}
		if o&core.TextOriginBot != 0 {
			ref = ref.Plus(gu.WithMagnitude(sheight / 2))
		}
		if o&core.TextOriginTop != 0 {
			ref = ref.Minus(gu.WithMagnitude(sheight / 2))
		}
	}

	o := ref.Minus(gr.WithMagnitude(swidth / 2)).Minus(gu.WithMagnitude(sheight / 2))
	d.canvas.DrawVectorText(s, textHeight, o, gr.WithMagnitude(1), gu.WithMagnitude(1), hcs)
	if labelPos != nil {
		*labelPos = o
	}
}

// doProjectedPointToPlane projects r into the plane through o with
// normal n, with a stippled line from where it was.
func (d *constraintDrawer) doProjectedPointToPlane(hcs render.HStroke, r *geom.Vector, n, o geom.Vector) {
	dist := r.DistanceToPlane(n, o)
	p := r.Minus(n.ScaledBy(dist))
	d.doStippledLine(hcs, p, *r)
	*r = p
}

// doProjectedPoint projects r into the constraint's workplane.
func (d *constraintDrawer) doProjectedPoint(hcs render.HStroke, r *geom.Vector) {
	p := d.projectInto(*r, d.c.Workplane)
	d.doStippledLine(hcs, p, *r)
	*r = p
}

// doLineTrimmedAgainstBox draws a line from a to b around the label box
// at ref (C++ Constraint::DoLineTrimmedAgainstBox): if the line would
// cross the label, it is trimmed to leave a gap, returning 0; if not,
// it is extended to almost meet the box, returning positive or
// negative as that extension was from a or from b.
func (d *constraintDrawer) doLineTrimmedAgainstBox(hcs render.HStroke, ref, a, b geom.Vector, extend bool) int {
	ss := d.gw.ss
	th := ss.styleTextHeight(constraintStyle(d.c)) / d.camera.Scale
	pixels := 1.0 / d.camera.Scale
	vf := res.BuiltinVectorFont()
	swidth := vf.GetWidth(th, ss.constraintLabel(d.c)) + 8*pixels
	sheight := vf.GetCapHeight(th) + 8*pixels
	gu := d.camera.ProjUp.WithMagnitude(1)
	gr := d.camera.ProjRight.WithMagnitude(1)
	return d.doLineTrimmedAgainstBoxWith(hcs, ref, a, b, extend, gr, gu, swidth, sheight)
}

func (d *constraintDrawer) doLineTrimmedAgainstBoxWith(hcs render.HStroke, ref, a, b geom.Vector, extend bool,
	gr, gu geom.Vector, swidth, sheight float64) int {
	type plane struct {
		n geom.Vector
		d float64
	}
	// Reference pos is the center of box occupied by text; build a
	// rectangle around that, aligned to axes gr and gu, from four planes
	// with all four normals pointing inward.
	planes := [4]plane{
		{gu.ScaledBy(-1), -(gu.Dot(ref) + sheight/2)},
		{gu, gu.Dot(ref) - sheight/2},
		{gr, gr.Dot(ref) - swidth/2},
		{gr.ScaledBy(-1), -(gr.Dot(ref) + swidth/2)},
	}

	tmin, tmax := core.VeryPositive, core.VeryNegative
	dl := b.Minus(a)

	for i := 0; i < 4; i++ {
		p, parallel := geom.AtIntersectionOfPlaneAndLineParallel(planes[i].n, planes[i].d, a, b)
		if parallel {
			continue
		}
		j := 0
		for ; j < 4; j++ {
			dist := planes[j].n.Dot(p) - planes[j].d
			if dist < -geom.LengthEps {
				break
			}
		}
		if j < 4 {
			continue
		}
		t := p.Minus(a).DivProjected(dl)
		tmin = geom.Min(t, tmin)
		tmax = geom.Max(t, tmax)
	}

	// Both in range; so there's pieces of the line on both sides of the
	// label box.
	if tmin >= 0.0 && tmin <= 1.0 && tmax >= 0.0 && tmax <= 1.0 {
		d.doLine(hcs, a, a.Plus(dl.ScaledBy(tmin)))
		d.doLine(hcs, a.Plus(dl.ScaledBy(tmax)), b)
		return 0
	}
	// Only one intersection in range; so the box is right on top of the
	// endpoint
	if tmin >= 0.0 && tmin <= 1.0 {
		d.doLine(hcs, a, a.Plus(dl.ScaledBy(tmin)))
		return 0
	}
	// Likewise.
	if tmax >= 0.0 && tmax <= 1.0 {
		d.doLine(hcs, a.Plus(dl.ScaledBy(tmax)), b)
		return 0
	}
	// The line does not intersect the label; so the line should get
	// extended to just barely meet the label.
	if tmax < 0.0 {
		if extend {
			a = a.Plus(dl.ScaledBy(tmax))
		}
		d.doLine(hcs, a, b)
		return 1
	}
	if tmin > 1.0 {
		if extend {
			b = a.Plus(dl.ScaledBy(tmin))
		}
		d.doLine(hcs, a, b)
		return -1
	}
	// This will happen if the entire line lies within the box.
	return 0
}

func (d *constraintDrawer) doArrow(hcs render.HStroke, p, dir, n geom.Vector, width, angle, da float64) {
	dir = dir.WithMagnitude(width / libm.Cos(angle))
	dir = dir.RotatedAroundAxis(n, da)
	d.doLine(hcs, p, p.Plus(dir.RotatedAroundAxis(n, angle)))
	d.doLine(hcs, p, p.Plus(dir.RotatedAroundAxis(n, -angle)))
}

// doLineWithArrows draws a dimension line: the length from a to b,
// with extension lines out to where the line crosses ref.
func (d *constraintDrawer) doLineWithArrows(hcs render.HStroke, ref, a, b geom.Vector, onlyOneExt bool) {
	pixels := 1.0 / d.camera.Scale

	ab := a.Minus(b)
	ar := a.Minus(ref)
	// Normal to a plane containing the line and the label origin.
	n := ab.Cross(ar)
	// Within that plane, and normal to the line AB; so that's our
	// extension line.
	out := ab.Cross(n).WithMagnitude(1)
	out = out.ScaledBy(-out.Dot(ar))

	ae, be := a.Plus(out), b.Plus(out)

	// Extension lines extend 10 pixels beyond where the arrows get drawn
	// (which is at the same offset perpendicular from AB as the label).
	d.doLine(hcs, a, ae.Plus(out.WithMagnitude(10*pixels)))
	if !onlyOneExt {
		d.doLine(hcs, b, be.Plus(out.WithMagnitude(10*pixels)))
	}

	within := d.doLineTrimmedAgainstBox(hcs, ref, ae, be, true)

	// Arrow heads are 13 pixels long, with an 18 degree half-angle.
	theta := 18 * math.Pi / 180
	arrow := be.Minus(ae).WithMagnitude(13 * pixels)

	if within != 0 {
		arrow = arrow.ScaledBy(-1)
		seg := be.Minus(ae).WithMagnitude(18 * pixels)
		if within < 0 {
			d.doLine(hcs, ae, ae.Minus(seg))
		}
		if within > 0 {
			d.doLine(hcs, be, be.Plus(seg))
		}
	}

	d.doArrow(hcs, ae, arrow, n, 13.0*pixels, theta, 0.0)
	d.doArrow(hcs, be, arrow.Negated(), n, 13.0*pixels, theta, 0.0)
}

func (d *constraintDrawer) doEqualLenTicks(hcs render.HStroke, a, b, gn geom.Vector, refp *geom.Vector) {
	m := a.ScaledBy(1.0 / 3).Plus(b.ScaledBy(2.0 / 3))
	if refp != nil {
		*refp = m
	}
	ab := a.Minus(b)
	n := gn.Cross(ab).WithMagnitude(10 / d.camera.Scale)
	d.doLine(hcs, m.Minus(n), m.Plus(n))
}

func (d *constraintDrawer) doEqualRadiusTicks(hcs render.HStroke, he core.HEntity, refp *geom.Vector) {
	gw := d.gw
	sk := gw.sk()
	circ := sk.GetEntity(he)

	center := gw.pointGetDrawNum(sk.GetEntity(circ.Point[0]))
	r := circ.CircleGetRadiusNum(sk)
	q := circ.NormalEntity(sk).NormalGetNum(sk)
	u, v := q.RotationU(), q.RotationV()

	var theta float64
	switch circ.Type {
	case core.EntityCircle:
		theta = math.Pi / 2
	case core.EntityArcOfCircle:
		thetaa, _, dtheta := circ.ArcGetAngles(sk)
		theta = thetaa + dtheta/2
	default:
		panic("ui: unexpected entity type")
	}

	dv := u.ScaledBy(libm.Cos(theta)).Plus(v.ScaledBy(libm.Sin(theta)))
	dv = dv.ScaledBy(r)
	p := center.Plus(dv)
	if refp != nil {
		*refp = p
	}
	tick := dv.WithMagnitude(10 / d.camera.Scale)
	d.doLine(hcs, p.Plus(tick), p.Minus(tick))
}

// doLineExtend draws the projection of pt onto the line p0-p1, and the
// line's extension to it if pt is beyond the line's ends.
func (d *constraintDrawer) doLineExtend(hcs render.HStroke, p0, p1, pt geom.Vector, salient float64) bool {
	dir := p1.Minus(p0)
	k := dir.Dot(pt.Minus(p0)) / dir.Dot(dir)
	ptOnLine := p0.Plus(dir.ScaledBy(k))

	// Draw projection line.
	d.doLine(hcs, pt, ptOnLine)

	// Calculate salient direction.
	sd := dir.WithMagnitude(1.0).ScaledBy(salient)

	var from, to geom.Vector
	switch {
	case k < 0.0:
		from, to = p0, ptOnLine.Minus(sd)
	case k > 1.0:
		from, to = p1, ptOnLine.Plus(sd)
	default:
		return false
	}
	// Draw extension line.
	d.doLine(hcs, from, to)
	return true
}

// doArcForAngle draws the arc of an angle dimension between the lines
// a0+t*da and b0+t*db.
func (d *constraintDrawer) doArcForAngle(hcs render.HStroke, a0, da, b0, db, offset geom.Vector,
	ref *geom.Vector, trim bool, explodeOffset geom.Vector) {
	gw := d.gw
	ss := gw.ss
	sk := gw.sk()
	c := d.c
	camera := d.camera
	pixels := 1.0 / camera.Scale
	gr := camera.ProjRight.ScaledBy(1.0)
	gu := camera.ProjUp.ScaledBy(1.0)

	if !core.IsFreeIn3D(c.Workplane) {
		a0 = d.projectInto(a0, c.Workplane)
		b0 = d.projectInto(b0, c.Workplane)
		da = core.ProjectVectorInto(sk, da, c.Workplane)
		db = core.ProjectVectorInto(sk, db, c.Workplane)
	}

	a0 = a0.Plus(explodeOffset)
	b0 = b0.Plus(explodeOffset)

	a1 := a0.Plus(da)
	b1 := b0.Plus(db)

	pi, skew := geom.AtIntersectionOfLinesSkew(a0, a0.Plus(da), b0, b0.Plus(db))

	if !skew {
		*ref = pi.Plus(offset)
		// We draw in a coordinate system centered at the intersection
		// point. One basis vector is da, and the other is normal to da and
		// in the plane that contains our lines (so normal to its normal).
		da = da.WithMagnitude(1)
		db = db.WithMagnitude(1)

		norm := da.Cross(db)

		dna := norm.Cross(da).WithMagnitude(1.0)
		dnb := norm.Cross(db).WithMagnitude(1.0)

		// da and db magnitudes are 1.0
		thetaf := libm.Acos(da.Dot(db))

		// Calculate median
		m := da.ScaledBy(libm.Cos(thetaf / 2)).Plus(dna.ScaledBy(libm.Sin(thetaf / 2)))
		rm := ref.Minus(pi)

		// Test which side we have to place an arc
		if m.Dot(rm) < 0 {
			da = da.ScaledBy(-1)
			dna = dna.ScaledBy(-1)
			db = db.ScaledBy(-1)
			dnb = dnb.ScaledBy(-1)
		}

		rda, rdna := rm.Dot(da), rm.Dot(dna)

		// Introduce minimal arc radius in pixels
		r := geom.Max(math.Sqrt(rda*rda+rdna*rdna), 15.0*pixels)

		th := ss.styleTextHeight(constraintStyle(c)) / camera.Scale
		vf := res.BuiltinVectorFont()
		swidth := vf.GetWidth(th, ss.constraintLabel(c)) + 8*pixels
		sheight := vf.GetCapHeight(th) + 6*pixels
		textR := math.Sqrt(swidth*swidth+sheight*sheight) / 2.0
		*ref = pi.Plus(rm.WithMagnitude(geom.Max(rm.Magnitude(), 15*pixels+textR)))

		// Arrow points
		apa := da.ScaledBy(r).Plus(pi)
		apb := da.ScaledBy(r * libm.Cos(thetaf)).Plus(dna.ScaledBy(r * libm.Sin(thetaf))).Plus(pi)

		arrowW := 13 * pixels
		arrowA := 18.0 * math.Pi / 180.0
		arrowVisible := apb.Minus(apa).Magnitude() > 2.5*arrowW
		// Arrow reversing indicator
		arrowRev := false

		// The minimal extension length in angular representation
		extAngle := 18 * pixels / r

		// Arc additional angle
		addAngle := 0.0
		// Arc start angle
		startAngle := 0.0

		// Arc extension to db. We have just enlarge angle value.
		if c.HasLabel() && rm.Dot(dnb) > 0.0 {
			// rm direction projected to plane with u = da, v = dna
			rmp := da.ScaledBy(rda).Plus(dna.ScaledBy(rdna)).WithMagnitude(1.0)
			// rmp and db magnitudes are 1.0
			addAngle = geom.Max(libm.Acos(rmp.Dot(db)), extAngle)

			if arrowVisible {
				startAngle = -extAngle
				addAngle += extAngle
				arrowRev = true
			}
		}

		// Arc extension to da. We are enlarge angle value and rewrite
		// basis to align along rm projection.
		if c.HasLabel() && rm.Dot(dna) < 0.0 {
			// rm direction projected to plane with u = da, v = dna
			rmp := da.ScaledBy(rda).Plus(dna.ScaledBy(rdna)).WithMagnitude(1.0)
			// rmp and da magnitudes are 1.0
			startAngle = -geom.Max(libm.Acos(rmp.Dot(da)), extAngle)
			addAngle = -startAngle

			if arrowVisible {
				addAngle += extAngle
				arrowRev = true
			}
		}

		var prev geom.Vector
		const n = 30
		for i := 0; i <= n; i++ {
			theta := startAngle + (float64(i)*(thetaf+addAngle))/n
			p := da.ScaledBy(r * libm.Cos(theta)).Plus(dna.ScaledBy(r * libm.Sin(theta))).Plus(pi)
			if i > 0 {
				if trim {
					d.doLineTrimmedAgainstBoxWith(hcs, *ref, prev, p, false, gr, gu, swidth, sheight+2*pixels)
				} else {
					d.doLine(hcs, prev, p)
				}
			}
			prev = p
		}

		d.doLineExtend(hcs, a0, a1, apa, 5.0*pixels)
		d.doLineExtend(hcs, b0, b1, apb, 5.0*pixels)

		// Draw arrows only when we have enough space.
		if arrowVisible {
			angleCorr := arrowW / (2.0 * r)
			if arrowRev {
				dna = dna.ScaledBy(-1.0)
				angleCorr = -angleCorr
			}
			d.doArrow(hcs, apa, dna, norm, arrowW, arrowA, angleCorr)
			d.doArrow(hcs, apb, dna, norm, arrowW, arrowA, thetaf+math.Pi-angleCorr)
		}
	} else {
		// The lines are skew; no wonderful way to illustrate that.
		*ref = a0.Plus(b0)
		*ref = ref.ScaledBy(0.5).Plus(c.Disp.Offset)
		gu = gu.WithMagnitude(1)
		textHeight := ss.styleTextHeight(constraintStyle(c)) / camera.Scale
		trans := ref.Plus(gu.ScaledBy(-1.5 * res.BuiltinVectorFont().GetCapHeight(textHeight)))
		d.canvas.DrawVectorText("angle between skew lines", textHeight,
			trans, gr.WithMagnitude(1), gu.WithMagnitude(1), hcs)
	}
}

// doLayout is Constraint::DoLayout: it draws the constraint, and gives
// its label's corner and its reference points.
func (d *constraintDrawer) doLayout(how ConstraintDrawAs, labelPos *geom.Vector, refs *[]geom.Vector) {
	gw := d.gw
	ss := gw.ss
	sk := gw.sk()
	c := d.c
	canvas := d.canvas
	if !(how == ConstraintDrawHovered || how == ConstraintDrawSelected) && !gw.constraintIsVisible(c) {
		return
	}
	addRef := func(p geom.Vector) {
		if refs != nil {
			*refs = append(*refs, p)
		}
	}
	pointNum := func(h core.HEntity) geom.Vector { return sk.GetEntity(h).PointGetNum(sk) }
	drawNum := func(h core.HEntity) geom.Vector { return gw.pointGetDrawNum(sk.GetEntity(h)) }
	explodeOffset := func(h core.HEntity) geom.Vector { return gw.entityExplodeOffset(sk.GetEntity(h)) }

	// Unit vectors that describe our current view of the scene. One pixel
	// long, not one actual unit.
	camera := d.camera
	gr := camera.ProjRight.ScaledBy(1 / camera.Scale)
	gu := camera.ProjUp.ScaledBy(1 / camera.Scale)
	gn := gr.Cross(gu).WithMagnitude(1 / camera.Scale)

	textHeight := ss.styleTextHeight(constraintStyle(c)) / camera.Scale

	var color core.RgbaColor
	switch how {
	case ConstraintDrawDefault:
		color = ss.styleColor(constraintStyle(c))
	case ConstraintDrawHovered:
		color = ss.styleColor(hstyle(core.StyleHovered))
	case ConstraintDrawSelected:
		color = ss.styleColor(hstyle(core.StyleSelected))
	}
	stroke := ss.styleStroke(constraintStyle(c))
	stroke.Layer = render.LayerFront
	stroke.Color = color
	stroke.ZIndex = 4
	hcs := canvas.GetStroke(stroke)

	fill := render.Fill{Layer: render.LayerFront, Color: color, ZIndex: stroke.ZIndex}
	hcf := canvas.GetFill(fill)

	switch c.Type {
	case core.ConstraintPtPtDistance:
		ap := pointNum(c.PtA)
		bp := pointNum(c.PtB)

		if !core.IsFreeIn3D(c.Workplane) {
			d.doProjectedPoint(hcs, &ap)
			d.doProjectedPoint(hcs, &bp)
		}

		if gw.constraintShouldDrawExploded(c) {
			// Offset A and B by the same offset so the constraint is drawn
			// in the plane of one of the exploded points (rather than at an
			// angle)
			offset := explodeOffset(c.PtA)
			ap = ap.Plus(offset)
			bp = bp.Plus(offset)
		}

		ref := ap.Plus(bp).ScaledBy(0.5).Plus(c.Disp.Offset)
		addRef(ref)

		d.doLineWithArrows(hcs, ref, ap, bp, false)
		d.doLabel(hcs, ref, labelPos, gr, gu)
		return

	case core.ConstraintProjPtDistance:
		ap := pointNum(c.PtA)
		bp := pointNum(c.PtB)
		dp := bp.Minus(ap)
		pp := sk.GetEntity(c.EntityA).VectorGetNum(sk)

		if gw.constraintShouldDrawExploded(c) {
			// explode for whichever point is in the workplane (or the first
			// if both are)
			pt := sk.GetEntity(c.PtA)
			if pt.Group != c.Group {
				pt = sk.GetEntity(c.PtB)
			}
			if pt.Group == c.Group {
				offset := gw.entityExplodeOffset(pt)
				ap = ap.Plus(offset)
				bp = bp.Plus(offset)
			}
		}

		ref := ap.Plus(bp).ScaledBy(0.5).Plus(c.Disp.Offset)
		addRef(ref)

		pp = pp.WithMagnitude(1)
		dist := dp.Dot(pp)
		bpp := ap.Plus(pp.ScaledBy(dist))
		d.doStippledLine(hcs, ap, bpp)
		d.doStippledLine(hcs, bp, bpp)

		d.doLineWithArrows(hcs, ref, ap, bpp, false)
		d.doLabel(hcs, ref, labelPos, gr, gu)
		return

	case core.ConstraintPtFaceDistance, core.ConstraintPtPlaneDistance:
		pt := drawNum(c.PtA)
		enta := sk.GetEntity(c.EntityA)
		var n, p geom.Vector
		if c.Type == core.ConstraintPtPlaneDistance {
			n = enta.NormalEntity(sk).NormalN(sk)
			p = enta.WorkplaneGetOffset(sk)
		} else {
			n = enta.FaceGetNormalNum(sk)
			p = enta.FaceGetPointNum(sk)
		}

		dist := p.Minus(pt).Dot(n)
		closest := pt.Plus(n.WithMagnitude(dist))

		ref := closest.Plus(pt).ScaledBy(0.5).Plus(c.Disp.Offset)
		addRef(ref)

		if !pt.Equals(closest) {
			d.doLineWithArrows(hcs, ref, pt, closest, true)
		}
		d.doLabel(hcs, ref, labelPos, gr, gu)
		return

	case core.ConstraintPtLineDistance:
		ptEntity := sk.GetEntity(c.PtA)
		pt := ptEntity.PointGetNum(sk)
		line := sk.GetEntity(c.EntityA)
		lA := pointNum(line.Point[0])
		lB := pointNum(line.Point[1])
		dl := lB.Minus(lA)

		if !core.IsFreeIn3D(c.Workplane) {
			lA = d.projectInto(lA, c.Workplane)
			lB = d.projectInto(lB, c.Workplane)
			d.doProjectedPoint(hcs, &pt)
		}

		// Only explode if the point and line are in the same group (and
		// that group is a sketch with explode enabled) otherwise it's too
		// visually confusing to figure out what the correct projections
		// should be.
		shouldExplode := gw.constraintShouldDrawExploded(c) &&
			ptEntity.Group == c.Group && line.Group == c.Group
		if shouldExplode {
			offset := gw.entityExplodeOffset(ptEntity)
			pt = pt.Plus(offset)
			lA = lA.Plus(offset)
			lB = lB.Plus(offset)
		}

		// Find the closest point on the line
		closest := pt.ClosestPointOnLine(lA, dl)

		ref := closest.Plus(pt).ScaledBy(0.5).Plus(c.Disp.Offset)
		addRef(ref)
		d.doLabel(hcs, ref, labelPos, gr, gu)

		if !pt.Equals(closest) {
			d.doLineWithArrows(hcs, ref, pt, closest, true)

			// Draw projected point
			a := pt
			b := closest
			ab := a.Minus(b)
			ar := a.Minus(ref)
			n := ab.Cross(ar)
			out := ab.Cross(n).WithMagnitude(1)
			out = out.ScaledBy(-out.Dot(ar))
			be := b.Plus(out)
			np := lA.Minus(pt).Cross(lB.Minus(pt)).WithMagnitude(1.0)
			d.doProjectedPointToPlane(hcs, &be, np, pt)

			// Extensions to line
			pixels := 1.0 / camera.Scale
			refClosest := ref.ClosestPointOnLine(lA, dl)
			ddl := dl.Dot(dl)
			if math.Abs(ddl) > geom.LengthEps*geom.LengthEps {
				t := refClosest.Minus(lA).Dot(dl) / ddl
				if t < 0.0 {
					d.doLine(hcs, refClosest.Minus(dl.WithMagnitude(10.0*pixels)), lA)
				} else if t > 1.0 {
					d.doLine(hcs, refClosest.Plus(dl.WithMagnitude(10.0*pixels)), lB)
				}
			}
		}

		if !core.IsFreeIn3D(c.Workplane) {
			// Draw the projection marker from the closest point on the
			// projected line to the projected point on the real line.
			lAB := lA.Minus(lB)
			t := lA.Minus(closest).DivProjected(lAB)

			lA := pointNum(line.Point[0])
			lB := pointNum(line.Point[1])

			c2 := lA.ScaledBy(1 - t).Plus(lB.ScaledBy(t))
			d.doProjectedPoint(hcs, &c2)
		}
		return

	case core.ConstraintDiameter:
		circle := sk.GetEntity(c.EntityA)
		center := drawNum(circle.Point[0])
		q := sk.GetEntity(circle.Normal).NormalGetNum(sk)
		n := q.RotationN().WithMagnitude(1)
		r := circle.CircleGetRadiusNum(sk)

		ref := center.Plus(c.Disp.Offset)
		// Force the label into the same plane as the circle.
		ref = ref.Minus(n.ScaledBy(n.Dot(ref) - n.Dot(center)))
		addRef(ref)

		mark := ref.Minus(center)
		mark = mark.WithMagnitude(mark.Magnitude() - r)
		d.doLineTrimmedAgainstBox(hcs, ref, ref, ref.Minus(mark), true)

		var topLeft geom.Vector
		d.doLabel(hcs, ref, &topLeft, gr, gu)
		if labelPos != nil {
			*labelPos = topLeft
		}
		return

	case core.ConstraintPointsCoincident:
		if how == ConstraintDrawDefault {
			// Let's adjust the color of this constraint to have the same
			// rough luma as the point color, so that the constraint does
			// not stand out in an ugly way.
			cd := ss.styleColor(hstyle(core.StyleDatum))
			cc := ss.styleColor(hstyle(core.StyleConstraint))
			// convert from 8-bit color to a vector
			vd := geom.Vector{X: float64(cd.RedF()), Y: float64(cd.GreenF()), Z: float64(cd.BlueF())}
			vc := geom.Vector{X: float64(cc.RedF()), Y: float64(cc.GreenF()), Z: float64(cc.BlueF())}
			// and scale the constraint color to have the same magnitude as
			// the datum color, maybe a bit dimmer
			vc = vc.WithMagnitude(vd.Magnitude() * 0.9)
			// and set the color to that.
			fill.Color = core.RgbaColorFromFloat(float32(vc.X), float32(vc.Y), float32(vc.Z), 1.0)
			hcf = canvas.GetFill(fill)
		}

		for a := 0; a < 2; a++ {
			r := camera.ProjRight.ScaledBy(float64(a+1) / camera.Scale)
			dd := camera.ProjUp.ScaledBy(float64(2-a) / camera.Scale)
			for i := 0; i < 2; i++ {
				h := c.PtA
				if i != 0 {
					h = c.PtB
				}
				p := drawNum(h)
				addRef(p)
				canvas.DrawQuad(p.Plus(r).Plus(dd), p.Plus(r).Minus(dd),
					p.Minus(r).Minus(dd), p.Minus(r).Plus(dd), hcf)
			}
		}
		return

	case core.ConstraintPtOnCircle, core.ConstraintPtOnLine, core.ConstraintPtOnFace, core.ConstraintPtInPlane:
		s := 8 / camera.Scale
		p := drawNum(c.PtA)
		addRef(p)
		var r, dd geom.Vector
		switch c.Type {
		case core.ConstraintPtOnFace:
			n := sk.GetEntity(c.EntityA).FaceGetNormalNum(sk)
			r = n.NormalDir(0)
			dd = n.NormalDir(1)
		case core.ConstraintPtInPlane:
			n := sk.GetEntity(c.EntityA).NormalEntity(sk)
			r = n.NormalU(sk)
			dd = n.NormalV(sk)
		default:
			r = gr
			dd = gu
			s *= 6.0 / 8 // draw these a little smaller
		}
		r = r.WithMagnitude(s)
		dd = dd.WithMagnitude(s)
		d.doLine(hcs, p.Plus(r).Plus(dd), p.Plus(r).Minus(dd))
		d.doLine(hcs, p.Plus(r).Minus(dd), p.Minus(r).Minus(dd))
		d.doLine(hcs, p.Minus(r).Minus(dd), p.Minus(r).Plus(dd))
		d.doLine(hcs, p.Minus(r).Plus(dd), p.Plus(r).Plus(dd))
		return

	case core.ConstraintWhereDragged:
		p := drawNum(c.PtA)
		addRef(p)
		u := p.Plus(gu.WithMagnitude(8 / camera.Scale)).Plus(gr.WithMagnitude(8 / camera.Scale))
		uu := u.Minus(gu.WithMagnitude(5 / camera.Scale))
		ur := u.Minus(gr.WithMagnitude(5 / camera.Scale))
		// Draw four little crop marks, uniformly spaced (by ninety degree
		// rotations) around the point.
		for i := 0; i < 4; i++ {
			d.doLine(hcs, u, uu)
			d.doLine(hcs, u, ur)
			u = u.RotatedAbout(p, gn, math.Pi/2)
			ur = ur.RotatedAbout(p, gn, math.Pi/2)
			uu = uu.RotatedAbout(p, gn, math.Pi/2)
		}
		return

	case core.ConstraintSameOrientation:
		for i := 0; i < 2; i++ {
			h := c.EntityA
			if i != 0 {
				h = c.EntityB
			}
			e := sk.GetEntity(h)
			q := e.NormalGetNum(sk)
			n := q.RotationN().WithMagnitude(25 / camera.Scale)
			u := q.RotationU().WithMagnitude(6 / camera.Scale)
			p := pointNum(e.Point[0])
			p = p.Plus(n.WithMagnitude(10 / camera.Scale))
			addRef(p)

			d.doLine(hcs, p.Plus(u), p.Minus(u).Plus(n))
			d.doLine(hcs, p.Minus(u), p.Plus(u).Plus(n))
		}
		return

	case core.ConstraintEqualAngle:
		var ref geom.Vector
		a := sk.GetEntity(c.EntityA)
		b := sk.GetEntity(c.EntityB)
		cc := sk.GetEntity(c.EntityC)
		dd := sk.GetEntity(c.EntityD)

		a0 := a.VectorGetStartPoint(sk)
		b0 := b.VectorGetStartPoint(sk)
		c0 := cc.VectorGetStartPoint(sk)
		d0 := dd.VectorGetStartPoint(sk)
		da := a.VectorGetNum(sk)
		db := b.VectorGetNum(sk)
		dc := cc.VectorGetNum(sk)
		ddv := dd.VectorGetNum(sk)

		if c.Other {
			a0 = a0.Plus(da)
			da = da.ScaledBy(-1)
		}

		d.doArcForAngle(hcs, a0, da, b0, db, da.WithMagnitude(40/camera.Scale), &ref, false, gw.entityExplodeOffset(a))
		addRef(ref)
		d.doArcForAngle(hcs, c0, dc, d0, ddv, dc.WithMagnitude(40/camera.Scale), &ref, false, gw.entityExplodeOffset(cc))
		addRef(ref)
		return

	case core.ConstraintAngle:
		a := sk.GetEntity(c.EntityA)
		b := sk.GetEntity(c.EntityB)

		a0 := a.VectorGetStartPoint(sk)
		b0 := b.VectorGetStartPoint(sk)
		da := a.VectorGetNum(sk)
		db := b.VectorGetNum(sk)
		if c.Other {
			a0 = a0.Plus(da)
			da = da.ScaledBy(-1)
		}

		var ref geom.Vector
		d.doArcForAngle(hcs, a0, da, b0, db, c.Disp.Offset, &ref, true, gw.entityExplodeOffset(a))
		d.doLabel(hcs, ref, labelPos, gr, gu)
		addRef(ref)
		return

	case core.ConstraintPerpendicular:
		var u, v geom.Vector
		var rn, ru geom.Vector
		if core.IsFreeIn3D(c.Workplane) {
			rn = gn
			ru = gu
		} else {
			normal := sk.GetEntity(c.Workplane).NormalEntity(sk)
			rn = normal.NormalN(sk)
			ru = normal.NormalV(sk) // ru meaning r_up, not u/v
		}

		for i := 0; i < 2; i++ {
			h := c.EntityA
			if i != 0 {
				h = c.EntityB
			}
			e := sk.GetEntity(h)

			if i == 0 {
				// Calculate orientation of perpendicular sign only once, so
				// that it's the same both times it's drawn
				u = e.VectorGetNum(sk)
				u = u.WithMagnitude(16 / camera.Scale)
				v = rn.Cross(u).WithMagnitude(16 / camera.Scale)
				// a bit of bias to stop it from flickering between the two
				// possibilities
				if math.Abs(u.Dot(ru)) < math.Abs(v.Dot(ru))+geom.LengthEps {
					u, v = v, u
				}
				if u.Dot(ru) < 0 {
					u = u.ScaledBy(-1)
				}
			}

			p := e.VectorGetRefPoint(sk).Plus(gw.entityExplodeOffset(e))
			s := p.Plus(u).Plus(v)
			d.doLine(hcs, s, s.Plus(v))
			m := s.Plus(v.ScaledBy(0.5))
			d.doLine(hcs, m, m.Plus(u))
			addRef(m)
		}
		return

	case core.ConstraintCurveCurveTangent, core.ConstraintCubicLineTangent, core.ConstraintArcLineTangent:
		var textAt, u, v geom.Vector

		switch c.Type {
		case core.ConstraintArcLineTangent:
			arc := sk.GetEntity(c.EntityA)
			norm := sk.GetEntity(arc.Normal)
			ctr := drawNum(arc.Point[0])
			pi := 1
			if c.Other {
				pi = 2
			}
			p := drawNum(arc.Point[pi])
			r := p.Minus(ctr)
			textAt = p.Plus(r.WithMagnitude(14 / camera.Scale))
			u = norm.NormalU(sk)
			v = norm.NormalV(sk)
		case core.ConstraintCubicLineTangent:
			var n geom.Vector
			if core.IsFreeIn3D(c.Workplane) {
				u, v, n = gr, gu, gn
			} else {
				wn := sk.GetEntity(c.Workplane).NormalEntity(sk)
				u, v, n = wn.NormalU(sk), wn.NormalV(sk), wn.NormalN(sk)
			}

			cubic := sk.GetEntity(c.EntityA)
			var p geom.Vector
			if c.Other {
				p = cubic.CubicGetFinishNum(sk)
			} else {
				p = cubic.CubicGetStartNum(sk)
			}
			p = p.Plus(gw.entityExplodeOffset(cubic))
			dir := sk.GetEntity(c.EntityB).VectorGetNum(sk)
			out := n.Cross(dir)
			textAt = p.Plus(out.WithMagnitude(14 / camera.Scale))
		default:
			wn := sk.GetEntity(c.Workplane).NormalEntity(sk)
			u, v = wn.NormalU(sk), wn.NormalV(sk)
			n := wn.NormalN(sk)
			eA := sk.GetEntity(c.EntityA)
			// Big pain; we have to get a vector tangent to the curve at
			// the shared point, which could be from either a cubic or an
			// arc.
			var dir geom.Vector
			if c.Other {
				textAt = eA.EndpointFinish(sk).Plus(gw.entityExplodeOffset(eA))
				if eA.Type == core.EntityCubic {
					dir = eA.CubicGetFinishTangentNum(sk)
				} else {
					dir = pointNum(eA.Point[0]).Minus(pointNum(eA.Point[2]))
					dir = n.Cross(dir)
				}
			} else {
				textAt = eA.EndpointStart(sk).Plus(gw.entityExplodeOffset(eA))
				if eA.Type == core.EntityCubic {
					dir = eA.CubicGetStartTangentNum(sk)
				} else {
					dir = pointNum(eA.Point[0]).Minus(pointNum(eA.Point[1]))
					dir = n.Cross(dir)
				}
			}
			dir = n.Cross(dir)
			textAt = textAt.Plus(dir.WithMagnitude(14 / camera.Scale))
		}

		ex := res.BuiltinVectorFont().GetExtents(textHeight, "T")
		canvas.DrawVectorText("T", textHeight, textAt.Minus(ex.ScaledBy(0.5)),
			u.WithMagnitude(1), v.WithMagnitude(1), hcs)
		addRef(textAt)
		return

	case core.ConstraintParallel:
		for i := 0; i < 2; i++ {
			h := c.EntityA
			if i != 0 {
				h = c.EntityB
			}
			e := sk.GetEntity(h)
			n := e.VectorGetNum(sk)
			n = n.WithMagnitude(25 / camera.Scale)
			u := gn.Cross(n).WithMagnitude(4 / camera.Scale)
			p := e.VectorGetRefPoint(sk)

			if gw.constraintShouldDrawExploded(c) {
				p = p.Plus(gw.entityExplodeOffset(e))
			}

			d.doLine(hcs, p.Plus(u), p.Plus(u).Plus(n))
			d.doLine(hcs, p.Minus(u), p.Minus(u).Plus(n))
			addRef(p.Plus(n.ScaledBy(0.5)))
		}
		return

	case core.ConstraintEqualRadius:
		for i := 0; i < 2; i++ {
			var ref geom.Vector
			h := c.EntityA
			if i != 0 {
				h = c.EntityB
			}
			d.doEqualRadiusTicks(hcs, h, &ref)
			addRef(ref)
		}
		return

	case core.ConstraintEqualLineArcLen:
		line := sk.GetEntity(c.EntityA)
		var ref geom.Vector
		d.doEqualLenTicks(hcs, drawNum(line.Point[0]), drawNum(line.Point[1]), gn, &ref)
		addRef(ref)
		d.doEqualRadiusTicks(hcs, c.EntityB, &ref)
		addRef(ref)
		return

	case core.ConstraintLengthRatio, core.ConstraintLengthDifference, core.ConstraintEqualLengthLines:
		var a, b geom.Vector
		for i := 0; i < 2; i++ {
			h := c.EntityA
			if i != 0 {
				h = c.EntityB
			}
			e := sk.GetEntity(h)
			a = pointNum(e.Point[0])
			b = pointNum(e.Point[1])

			if !core.IsFreeIn3D(c.Workplane) {
				d.doProjectedPoint(hcs, &a)
				d.doProjectedPoint(hcs, &b)
			}

			if gw.constraintShouldDrawExploded(c) {
				offset := gw.entityExplodeOffset(e)
				a = a.Plus(offset)
				b = b.Plus(offset)
			}

			var ref geom.Vector
			d.doEqualLenTicks(hcs, a, b, gn, &ref)
			addRef(ref)
		}
		if c.Type == core.ConstraintLengthRatio || c.Type == core.ConstraintLengthDifference {
			ref := a.Plus(b).ScaledBy(0.5).Plus(c.Disp.Offset)
			d.doLabel(hcs, ref, labelPos, gr, gu)
		}
		return

	case core.ConstraintArcArcLenRatio, core.ConstraintArcArcDifference:
		circle := sk.GetEntity(c.EntityA)
		center := pointNum(circle.Point[0])
		q := sk.GetEntity(circle.Normal).NormalGetNum(sk)
		n := q.RotationN().WithMagnitude(1)

		var ref2 geom.Vector
		d.doEqualRadiusTicks(hcs, c.EntityA, &ref2)
		d.doEqualRadiusTicks(hcs, c.EntityB, &ref2)

		ref := center.Plus(c.Disp.Offset)
		// Force the label into the same plane as the circle.
		ref = ref.Minus(n.ScaledBy(n.Dot(ref) - n.Dot(center)))
		addRef(ref)
		var topLeft geom.Vector
		d.doLabel(hcs, ref, &topLeft, gr, gu)
		if labelPos != nil {
			*labelPos = topLeft
		}
		return

	case core.ConstraintArcLineLenRatio, core.ConstraintArcLineDifference:
		e := sk.GetEntity(c.EntityA)
		a := pointNum(e.Point[0])
		b := pointNum(e.Point[1])
		var ref geom.Vector
		d.doEqualLenTicks(hcs, a, b, gn, &ref)
		addRef(ref)
		d.doEqualRadiusTicks(hcs, c.EntityB, &ref)
		addRef(ref)
		ref = a.Plus(b).ScaledBy(0.5).Plus(c.Disp.Offset)
		d.doLabel(hcs, ref, labelPos, gr, gu)
		return

	case core.ConstraintEqLenPtLineD:
		forLen := sk.GetEntity(c.EntityA)
		a := pointNum(forLen.Point[0])
		b := pointNum(forLen.Point[1])
		if !core.IsFreeIn3D(c.Workplane) {
			d.doProjectedPoint(hcs, &a)
			d.doProjectedPoint(hcs, &b)
		}
		if gw.constraintShouldDrawExploded(c) {
			offset := gw.entityExplodeOffset(forLen)
			a = a.Plus(offset)
			b = b.Plus(offset)
		}
		var refa geom.Vector
		d.doEqualLenTicks(hcs, a, b, gn, &refa)
		addRef(refa)

		ln := sk.GetEntity(c.EntityB)
		la := pointNum(ln.Point[0])
		lb := pointNum(ln.Point[1])
		pt := pointNum(c.PtA)
		if !core.IsFreeIn3D(c.Workplane) {
			d.doProjectedPoint(hcs, &pt)
			la = d.projectInto(la, c.Workplane)
			lb = d.projectInto(lb, c.Workplane)
		}

		closest := pt.ClosestPointOnLine(la, lb.Minus(la))
		if gw.constraintShouldDrawExploded(c) {
			offset := explodeOffset(c.PtA)
			pt = pt.Plus(offset)
			closest = closest.Plus(offset)
		}
		d.doLine(hcs, pt, closest)
		var refb geom.Vector
		d.doEqualLenTicks(hcs, pt, closest, gn, &refb)
		addRef(refb)
		return

	case core.ConstraintEqPtLnDistances:
		for i := 0; i < 2; i++ {
			hl, hp := c.EntityA, c.PtA
			if i != 0 {
				hl, hp = c.EntityB, c.PtB
			}
			ln := sk.GetEntity(hl)
			la := pointNum(ln.Point[0])
			lb := pointNum(ln.Point[1])
			pte := sk.GetEntity(hp)
			pt := pte.PointGetNum(sk)

			if !core.IsFreeIn3D(c.Workplane) {
				d.doProjectedPoint(hcs, &pt)
				la = d.projectInto(la, c.Workplane)
				lb = d.projectInto(lb, c.Workplane)
			}

			closest := pt.ClosestPointOnLine(la, lb.Minus(la))
			if gw.constraintShouldDrawExploded(c) {
				offset := gw.entityExplodeOffset(pte)
				pt = pt.Plus(offset)
				closest = closest.Plus(offset)
			}
			d.doLine(hcs, pt, closest)

			var ref geom.Vector
			d.doEqualLenTicks(hcs, pt, closest, gn, &ref)
			addRef(ref)
		}
		return

	case core.ConstraintSymmetric, core.ConstraintSymmetricHoriz, core.ConstraintSymmetricVert,
		core.ConstraintSymmetricLine:
		var n geom.Vector
		switch c.Type {
		case core.ConstraintSymmetric:
			n = sk.GetEntity(c.EntityA).NormalEntity(sk).NormalN(sk)
		case core.ConstraintSymmetricHoriz:
			n = sk.GetEntity(c.Workplane).NormalEntity(sk).NormalU(sk)
		case core.ConstraintSymmetricVert:
			n = sk.GetEntity(c.Workplane).NormalEntity(sk).NormalV(sk)
		default:
			ln := sk.GetEntity(c.EntityA)
			la := pointNum(ln.Point[0])
			lb := pointNum(ln.Point[1])
			la = d.projectInto(la, c.Workplane)
			lb = d.projectInto(lb, c.Workplane)
			n = lb.Minus(la)
			nw := sk.GetEntity(c.Workplane).NormalEntity(sk).NormalN(sk)
			n = n.RotatedAroundAxis(nw, math.Pi/2)
		}
		a := drawNum(c.PtA)
		b := drawNum(c.PtB)

		for i := 0; i < 2; i++ {
			tail, dd := a, b
			if i != 0 {
				tail, dd = b, a
			}
			dd = dd.Minus(tail)
			// Project the direction in which the arrow is drawn normal to
			// the symmetry plane; for projected symmetry constraints, they
			// might not be in the same direction, even when the constraint
			// is fully solved.
			dd = n.ScaledBy(dd.Dot(n))
			dd = dd.WithMagnitude(20 / camera.Scale)
			tip := tail.Plus(dd)

			d.doLine(hcs, tail, tip)
			dd = dd.WithMagnitude(9 / camera.Scale)
			d.doLine(hcs, tip, tip.Minus(dd.RotatedAroundAxis(gn, 0.6)))
			d.doLine(hcs, tip, tip.Minus(dd.RotatedAroundAxis(gn, -0.6)))
			addRef(tip)
		}
		return

	case core.ConstraintAtMidpoint, core.ConstraintHorizontal, core.ConstraintVertical:
		if c.EntityA.V != 0 {
			var r, u, n geom.Vector
			if core.IsFreeIn3D(c.Workplane) {
				r, u, n = gr, gu, gn
			} else {
				wn := sk.GetEntity(c.Workplane).NormalEntity(sk)
				r = wn.NormalU(sk)
				u = wn.NormalV(sk)
				n = r.Cross(u)
			}
			// For "at midpoint", this branch is always taken.
			e := sk.GetEntity(c.EntityA)
			a := drawNum(e.Point[0])
			b := drawNum(e.Point[1])
			m := a.ScaledBy(0.5).Plus(b.ScaledBy(0.5))
			offset := a.Minus(b).Cross(n)
			offset = offset.WithMagnitude(textHeight)
			// Draw midpoint constraint on other side of line, so that a
			// line can be midpoint and horizontal at same time.
			if c.Type == core.ConstraintAtMidpoint {
				offset = offset.ScaledBy(-1)
			}

			var s string
			switch c.Type {
			case core.ConstraintHorizontal:
				s = "H"
			case core.ConstraintVertical:
				s = "V"
			case core.ConstraintAtMidpoint:
				s = "M"
			}
			o := m.Plus(offset).Plus(u.WithMagnitude(textHeight / 5))
			ex := res.BuiltinVectorFont().GetExtents(textHeight, s)
			shift := r.WithMagnitude(ex.X).Plus(u.WithMagnitude(ex.Y))

			canvas.DrawVectorText(s, textHeight, o.Minus(shift.ScaledBy(0.5)),
				r.WithMagnitude(1), u.WithMagnitude(1), hcs)
			addRef(o)
		} else {
			a := drawNum(c.PtA)
			b := drawNum(c.PtB)

			w := sk.GetEntity(c.Workplane).NormalEntity(sk)
			cu, cv, cn := w.NormalU(sk), w.NormalV(sk), w.NormalN(sk)

			for i := 0; i < 2; i++ {
				o, oo := a, a.Minus(b)
				if i != 0 {
					o, oo = b, b.Minus(a)
				}
				dd := cv
				if c.Type == core.ConstraintHorizontal {
					dd = cu
				}
				if oo.Dot(dd) < 0 {
					dd = dd.ScaledBy(-1)
				}

				dp := cn.Cross(dd)
				dd = dd.WithMagnitude(14 / camera.Scale)
				cc := o.Minus(dd)
				d.doLine(hcs, o, cc)
				dd = dd.WithMagnitude(3 / camera.Scale)
				dp = dp.WithMagnitude(2 / camera.Scale)
				canvas.DrawQuad(cc.Plus(dd).Plus(dp), cc.Minus(dd).Plus(dp),
					cc.Minus(dd).Minus(dp), cc.Plus(dd).Minus(dp), hcf)
				addRef(cc)
			}
		}
		return

	case core.ConstraintComment:
		var u, v geom.Vector
		if core.IsFreeIn3D(c.Workplane) {
			u, v = gr, gu
		} else {
			norm := sk.GetEntity(c.Workplane).NormalEntity(sk)
			u, v = norm.NormalU(sk), norm.NormalV(sk)
		}

		if c.Disp.Style.V != 0 {
			color := stroke.Color
			stroke = ss.styleStroke(c.Disp.Style)
			stroke.Layer = render.LayerFront
			if how != ConstraintDrawDefault {
				stroke.Color = color
			}
			hcs = canvas.GetStroke(stroke)
		}
		ref := c.Disp.Offset
		if c.PtA.V != 0 {
			ref = pointNum(c.PtA).Plus(c.Disp.Offset)
		}
		d.doLabel(hcs, ref, labelPos, u, v)
		addRef(ref)
		return
	}
	panic("ui: unexpected constraint type")
}

// drawConstraint is Constraint::Draw.
func (gw *GraphicsWindow) drawConstraint(c *core.Constraint, how ConstraintDrawAs, canvas render.Canvas) {
	d := &constraintDrawer{gw: gw, c: c, canvas: canvas, camera: canvas.GetCamera()}
	d.doLayout(how, nil, nil)
}

// constraintLabelPos is Constraint::GetLabelPos.
func (gw *GraphicsWindow) constraintLabelPos(c *core.Constraint, camera *render.Camera) geom.Vector {
	var p geom.Vector
	op := &render.ObjectPicker{Camera: *camera}
	d := &constraintDrawer{gw: gw, c: c, canvas: op, camera: op.GetCamera()}
	d.doLayout(ConstraintDrawDefault, &p, nil)
	return p
}

// constraintReferencePoints is Constraint::GetReferencePoints.
func (gw *GraphicsWindow) constraintReferencePoints(c *core.Constraint, camera *render.Camera) []geom.Vector {
	var refs []geom.Vector
	op := &render.ObjectPicker{Camera: *camera}
	d := &constraintDrawer{gw: gw, c: c, canvas: op, camera: op.GetCamera()}
	d.doLayout(ConstraintDrawDefault, nil, &refs)
	return refs
}
