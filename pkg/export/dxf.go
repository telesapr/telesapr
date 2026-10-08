package export

// DXF export, written through pkg/dxf.

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/dxf"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/srf"
)

// dxfFileWriter collects the curves, and writes them with the
// constraints when the file is finished.
type dxfFileWriter struct {
	vectorWriter
	// paths are the curves of each path.
	paths       [][]*srf.SBezier
	constraints []core.Constraint
}

func (w *dxfFileWriter) HasCanvasSize() bool { return false }
func (w *dxfFileWriter) CanOutputMesh() bool { return false }

func (w *dxfFileWriter) OutputConstraints(constraints []core.Constraint) bool {
	w.constraints = constraints
	return true
}

func (w *dxfFileWriter) StartFile() {
	w.paths = nil
}

func (w *dxfFileWriter) Background(core.RGBA) {}

func (w *dxfFileWriter) StartPath(core.RGBA, float64, bool, core.RGBA, core.HStyle) {
	w.paths = append(w.paths, nil)
}

func (w *dxfFileWriter) FinishPath(core.RGBA, float64, bool, core.RGBA, core.HStyle, bool) {
}

func (w *dxfFileWriter) Triangle(*srf.STriangle) {}

func (w *dxfFileWriter) Bezier(sb *srf.SBezier) {
	w.paths[len(w.paths)-1] = append(w.paths[len(w.paths)-1], sb)
}

func (w *dxfFileWriter) FinishAndCloseFile() error {
	iface := &dxfWriteInterface{writer: w, messages: map[string]bool{}}
	data := dxf.Write(iface)
	w.paths = nil
	w.constraints = nil

	if err := os.WriteFile(w.filename, data, 0o644); err != nil {
		return fmt.Errorf("couldn't write to '%s'", w.filename)
	}

	if len(iface.messages) > 0 {
		var text strings.Builder
		text.WriteString("Some aspects of the drawing have no DXF equivalent and " +
			"were not exported:\n")
		var msgs []string
		for m := range iface.messages {
			msgs = append(msgs, m)
		}
		slices.Sort(msgs)
		for _, m := range msgs {
			text.WriteString(" * " + m + "\n")
		}
		w.warn(text.String())
	}
	return nil
}

// needToOutput reports whether a constraint goes in the file as a
// dimension or text.
func (w *dxfFileWriter) needToOutput(c *core.Constraint) bool {
	switch c.Type {
	case core.ConstraintPtPtDistance, core.ConstraintPtLineDistance, core.ConstraintDiameter,
		core.ConstraintAngle, core.ConstraintComment:
		return w.o.Constraints.Visible(c)
	}
	return false
}

// DxfLineType is a stipple pattern's DXF line type.
func DxfLineType(stippleType core.StipplePattern) string {
	switch stippleType {
	case core.StippleContinuous:
		return "CONTINUOUS"
	case core.StippleShortDash:
		return "DASHED"
	case core.StippleDash:
		return "DASHED"
	case core.StippleLongDash:
		return "DASHEDX2"
	case core.StippleDashDot:
		return "DASHDOT"
	case core.StippleDashDotDot:
		return "DIVIDE"
	case core.StippleDot:
		return "DOT"
	}
	// FREEHAND and ZIGZAG have no corresponding DXF line type
	return "CONTINUOUS"
}

// dxfWriteInterface supplies the file's contents.
type dxfWriteInterface struct {
	writer   *dxfFileWriter
	dxf      *dxf.Writer
	messages map[string]bool
}

func toCoord(v geom.Vector) dxf.Coord { return dxf.Coord{X: v.X, Y: v.Y, Z: v.Z} }

func (d *dxfWriteInterface) xfrm(v geom.Vector) geom.Vector { return d.writer.Transform(v) }

func (d *dxfWriteInterface) WriteTextstyles(w *dxf.Writer) {
	w.WriteTextstyle(&dxf.Textstyle{Name: "unicode", Font: "unicode"})
}

func (d *dxfWriteInterface) WriteLayers(w *dxf.Writer) {
	w.WriteLayer(&dxf.Layer{Name: "dimensions"})
	w.WriteLayer(&dxf.Layer{Name: "text"})

	usedStyles := map[uint32]bool{}
	for _, path := range d.writer.paths {
		for _, sb := range path {
			usedStyles[uint32(sb.AuxA)] = true
		}
	}
	var styles []uint32
	for v := range usedStyles {
		styles = append(styles, v)
	}
	slices.Sort(styles)

	for _, v := range styles {
		s := d.writer.sk.StyleGet(hstyle(v))
		w.WriteLayer(&dxf.Layer{Name: s.DescriptionString()})
	}
}

func (d *dxfWriteInterface) WriteLTypes(w *dxf.Writer) {
	for st := core.StippleContinuous; st <= core.StippleZigzag; st++ {
		// LibreCAD requires the line type to have one of these exact
		// names, or otherwise it overwrites it with its own (continuous)
		// style.
		typ := dxf.LType{Name: DxfLineType(st)}
		sw := 1.0
		switch st {
		case core.StippleContinuous:
		case core.StippleShortDash:
			typ.Path = []float64{sw, -sw * 2.0}
		case core.StippleDash:
			typ.Path = []float64{sw, -sw}
		case core.StippleLongDash:
			typ.Path = []float64{sw * 2.0, -sw}
		case core.StippleDashDot:
			typ.Path = []float64{sw, -sw, 0.0, -sw}
		case core.StippleDot:
			typ.Path = []float64{sw, 0.0}
		case core.StippleDashDotDot:
			typ.Path = []float64{sw, -sw, 0.0, -sw, 0.0, -sw}
		case core.StippleFreehand, core.StippleZigzag:
			// Not implemented; exported as continuous.
		}
		w.WriteLineType(&typ)
	}
}

func (d *dxfWriteInterface) writePolylines() {
	var builder srf.PolylineBuilder

	for _, path := range d.writer.paths {
		for _, sb := range path {
			if sb.Degree != 1 {
				continue
			}
			builder.AddEdge(sb.Ctrl[0], sb.Ctrl[1], uint32(sb.AuxA))
		}
	}

	var polyline dxf.Polyline

	startFn := func(start, next *srf.PolylineVertex, e *srf.PolylineEdge) {
		hs := hstyle(e.Kind)
		polyline = dxf.Polyline{}
		d.assignEntityDefaults(&polyline.Entity, hs)

		if !(start.Pos.Z == 0.0 && next.Pos.Z == 0.0) {
			polyline.Flags |= 8 // 3d polyline
		}
		polyline.Vertices = append(polyline.Vertices,
			dxf.Vertex{BasePoint: toCoord(start.Pos)},
			dxf.Vertex{BasePoint: toCoord(next.Pos)})
	}

	nextFn := func(next *srf.PolylineVertex, e *srf.PolylineEdge) {
		if next.Pos.Z != 0.0 {
			polyline.Flags |= 8 // 3d polyline
		}
		polyline.Vertices = append(polyline.Vertices, dxf.Vertex{BasePoint: toCoord(next.Pos)})
	}

	endFn := func() {
		d.dxf.WritePolyline(&polyline)
	}

	aloneFn := func(e *srf.PolylineEdge) {
		d.writeLine(e.A.Pos, e.B.Pos, hstyle(e.Kind))
	}

	builder.Generate(startFn, nextFn, aloneFn, endFn)
}

func (d *dxfWriteInterface) WriteEntities(w *dxf.Writer) {
	d.dxf = w
	sk := d.writer.sk
	d.writePolylines()

	for _, path := range d.writer.paths {
		for _, sb := range path {
			if sb.Degree == 1 {
				continue
			}
			d.writeBezier(sb)
		}
	}

	if d.writer.constraints == nil {
		return
	}
	for i := range d.writer.constraints {
		c := &(d.writer.constraints)[i]
		if !d.writer.needToOutput(c) {
			continue
		}
		switch c.Type {
		case core.ConstraintPtPtDistance:
			ap := sk.MustEntity(c.PtA).PointGetNum(sk)
			bp := sk.MustEntity(c.PtB).PointGetNum(sk)
			ref := ap.Plus(bp).ScaledBy(0.5).Plus(c.Disp.Offset)
			d.writeAlignedDimension(d.xfrm(ap), d.xfrm(bp), d.xfrm(ref),
				d.xfrm(ref), d.writer.o.Constraints.Label(c), c.StyleOrDefault(), c.ValA)

		case core.ConstraintPtLineDistance:
			pt := sk.MustEntity(c.PtA).PointGetNum(sk)
			line := sk.MustEntity(c.EntityA)
			lA := sk.MustEntity(line.Point[0]).PointGetNum(sk)
			lB := sk.MustEntity(line.Point[1]).PointGetNum(sk)
			dl := lB.Minus(lA)

			closest := pt.ClosestPointOnLine(lA, dl)

			if pt.Equals(closest) {
				break
			}

			ref := closest.Plus(pt).ScaledBy(0.5).Plus(c.Disp.Offset)
			refClosest := ref.ClosestPointOnLine(lA, dl)

			ddl := dl.Dot(dl)
			if math.Abs(ddl) > core.LengthEps*core.LengthEps {
				t := refClosest.Minus(lA).Dot(dl) / ddl
				if t < 0.0 {
					refClosest = lA
				} else if t > 1.0 {
					refClosest = lB
				}
			}

			xdl := d.xfrm(lB).Minus(d.xfrm(lA))
			d.writeLinearDimension(d.xfrm(pt), d.xfrm(refClosest), d.xfrm(ref),
				d.xfrm(ref), d.writer.o.Constraints.Label(c),
				libm.Atan2(xdl.Y, xdl.X)/math.Pi*180.0+90.0, 0.0,
				c.StyleOrDefault(), c.ValA)

		case core.ConstraintDiameter:
			circle := sk.MustEntity(c.EntityA)
			center := sk.MustEntity(circle.Point[0]).PointGetNum(sk)
			q := sk.MustEntity(circle.Normal).NormalGetNum(sk)
			n := q.RotationN().WithMagnitude(1)
			r := circle.CircleGetRadiusNum(sk)

			ref := center.Plus(c.Disp.Offset)
			// Force the label into the same plane as the circle.
			ref = ref.Minus(n.ScaledBy(n.Dot(ref) - n.Dot(center)))

			rad := ref.Minus(center).WithMagnitude(r)
			if c.Other { // isRadius
				d.writeRadialDimension(d.xfrm(center), d.xfrm(center.Plus(rad)),
					d.xfrm(ref), d.writer.o.Constraints.Label(c), c.StyleOrDefault(), c.ValA)
			} else {
				d.writeDiametricDimension(d.xfrm(center.Minus(rad)), d.xfrm(center.Plus(rad)),
					d.xfrm(ref), d.writer.o.Constraints.Label(c), c.StyleOrDefault(), c.ValA)
			}

		case core.ConstraintAngle:
			a := sk.MustEntity(c.EntityA)
			b := sk.MustEntity(c.EntityB)

			a0 := a.VectorGetStartPoint(sk)
			b0 := b.VectorGetStartPoint(sk)
			da := a.VectorGetNum(sk)
			db := b.VectorGetNum(sk)
			if c.Other { // otherAngle
				a0 = a0.Plus(da)
				da = da.ScaledBy(-1)
			}

			ref := c.Disp.Offset
			pi, skew := geom.AtIntersectionOfLinesSkew(a0, a0.Plus(da), b0, b0.Plus(db))
			if !skew {
				ref = pi.Plus(c.Disp.Offset)
			}

			norm := da.Cross(db)
			dna := norm.Cross(da).WithMagnitude(1.0)

			thetaf := libm.Acos(da.DirectionCosineWith(db))

			// Calculate median
			m := da.WithMagnitude(1.0).ScaledBy(libm.Cos(thetaf / 2)).Plus(
				dna.ScaledBy(libm.Sin(thetaf / 2)))
			rm := ref.Minus(pi)

			// Test which side we have to place an arc
			if m.Dot(rm) < 0 {
				da = da.ScaledBy(-1)
				dna = dna.ScaledBy(-1)
			}

			bisect := da.WithMagnitude(1.0).ScaledBy(libm.Cos(thetaf / 2.0)).Plus(
				dna.ScaledBy(libm.Sin(thetaf / 2.0)))

			ref = pi.Plus(bisect.WithMagnitude(c.Disp.Offset.Magnitude()))

			// Get lines again to write exact line.
			a0 = a.VectorGetStartPoint(sk)
			b0 = b.VectorGetStartPoint(sk)
			da = a.VectorGetNum(sk)
			db = b.VectorGetNum(sk)

			d.writeAngularDimension(
				d.xfrm(a0), d.xfrm(a0.Plus(da)), d.xfrm(b0), d.xfrm(b0.Plus(db)), d.xfrm(ref),
				d.xfrm(ref), d.writer.o.Constraints.Label(c), c.StyleOrDefault(), c.ValA)

		case core.ConstraintComment:
			hs := c.StyleOrDefault()
			st := sk.StyleGet(hs)
			d.writeText(d.xfrm(c.Disp.Offset), d.writer.o.Constraints.Label(c),
				d.writer.styleTextHeight(hs)/d.writer.o.ViewScale,
				st.TextAngle, st.TextOrigin, hs)
		}
	}
}

// findDxfColor is the nearest colour in the DXF palette.
func findDxfColor(src core.RGBA) int {
	best := 0
	minDist := geom.VeryPositive
	srcv := vec3(float64(src.RedF()), float64(src.GreenF()), float64(src.BlueF()))
	for i := 1; i < 256; i++ {
		dst := core.NewRGBA(dxf.Colors[i][0], dxf.Colors[i][1], dxf.Colors[i][2], 255)
		dstv := vec3(float64(dst.RedF()), float64(dst.GreenF()), float64(dst.BlueF()))
		dist := srcv.Minus(dstv).Magnitude()
		if dist < minDist || best == 0 {
			best = i
			minDist = dist
		}
	}
	return best
}

// toPackedIntBGRA is a colour as the int libdxfrw keeps it in.
func toPackedIntBGRA(c core.RGBA) int {
	return int(int32(uint32(c.Blue) | uint32(c.Green)<<8 | uint32(c.Red)<<16 | uint32(255-c.Alpha)<<24))
}

func (d *dxfWriteInterface) assignEntityDefaults(entity *dxf.Entity, hs core.HStyle) {
	*entity = dxf.NewEntity()
	s := d.writer.sk.StyleGet(hs)
	color := d.writer.styleExportColor(hs)
	entity.Color24 = toPackedIntBGRA(color)
	entity.Color = findDxfColor(color)
	entity.Layer = s.DescriptionString()
	entity.LineType = DxfLineType(s.StippleType)
	entity.LWeight = dxf.WidthMm(d.writer.styleWidthMm(hs))

	switch s.StippleType {
	case core.StippleFreehand:
		d.messages["freehand lines were replaced with continuous lines"] = true
	case core.StippleZigzag:
		d.messages["zigzag lines were replaced with continuous lines"] = true
	}
}

func (d *dxfWriteInterface) assignDimensionDefaults(dimension *dxf.Entity, hs core.HStyle) {
	d.assignEntityDefaults(dimension, hs)
	dimension.Layer = "dimensions"
}

func (d *dxfWriteInterface) writeLine(p0, p1 geom.Vector, hs core.HStyle) {
	var line dxf.Line
	d.assignEntityDefaults(&line.Entity, hs)
	line.BasePoint = toCoord(p0)
	line.SecPoint = toCoord(p1)
	d.dxf.WriteLine(&line)
}

func (d *dxfWriteInterface) writeArc(c geom.Vector, r, sa, ea float64, hs core.HStyle) {
	var arc dxf.Arc
	d.assignEntityDefaults(&arc.Entity, hs)
	arc.Radius = r
	arc.BasePoint = toCoord(c)
	arc.StaAngle = sa
	arc.EndAngle = ea
	d.dxf.WriteArc(&arc)
}

func (d *dxfWriteInterface) writeBezierAsPwl(sb *srf.SBezier) {
	var lv []geom.Vector
	sb.MakePwlIntoPoints(&lv, d.writer.o.Tol, 0)
	hs := hstyle(uint32(sb.AuxA))
	var polyline dxf.Polyline
	d.assignEntityDefaults(&polyline.Entity, hs)
	for _, v := range lv {
		polyline.Vertices = append(polyline.Vertices, dxf.Vertex{BasePoint: toCoord(v)})
	}
	d.dxf.WritePolyline(&polyline)
}

func (d *dxfWriteInterface) writeSpline(sb *srf.SBezier) {
	isRational := sb.IsRational()
	hs := hstyle(uint32(sb.AuxA))
	var spline dxf.Spline
	d.assignEntityDefaults(&spline.Entity, hs)
	if isRational {
		spline.Flags = 0x04
	} else {
		spline.Flags = 0x08
	}
	spline.Degree = sb.Degree
	// QCad/LibreCAD require the knots for some reason.
	if sb.Degree < 2 {
		panic("ui: unexpected degree of spline")
	}
	for k := range 2 * (sb.Degree + 1) {
		spline.Knots = append(spline.Knots, float64(k/(sb.Degree+1)))
	}
	for i := 0; i <= sb.Degree; i++ {
		spline.Controls = append(spline.Controls, toCoord(sb.Ctrl[i]))
		if isRational {
			spline.Weights = append(spline.Weights, sb.Weight[i])
		}
	}
	d.dxf.WriteSpline(&spline)
}

func (d *dxfWriteInterface) writeBezier(sb *srf.SBezier) {
	hs := hstyle(uint32(sb.AuxA))
	var c geom.Vector
	n := geom.Vector{Z: 1.0}
	var r float64

	switch {
	case sb.Degree == 1:
		// Line
		d.writeLine(sb.Ctrl[0], sb.Ctrl[1], hs)
	case sb.IsInPlane(n, 0) && sb.IsCircle(n, &c, &r):
		// Circle perpendicular to camera
		theta0 := libm.Atan2(sb.Ctrl[0].Y-c.Y, sb.Ctrl[0].X-c.X)
		theta1 := libm.Atan2(sb.Ctrl[2].Y-c.Y, sb.Ctrl[2].X-c.X)
		dtheta := srf.WrapSymmetric(theta1-theta0, 2.0*math.Pi)
		if dtheta < 0.0 {
			theta0, theta1 = theta1, theta0
		}

		d.writeArc(c, r, theta0, theta1, hs)
	case sb.IsRational():
		// Rational bezier. We'd like to export rational beziers exactly,
		// but the resulting DXF files can only be read by AutoCAD;
		// LibreCAD/QCad simply do not implement the feature. So, export
		// as piecewise linear for compatibility.
		d.writeBezierAsPwl(sb)
	default:
		// Any other curve
		d.writeSpline(sb)
	}
}

func (d *dxfWriteInterface) writeAlignedDimension(def1, def2, dimp, textp geom.Vector,
	text string, hs core.HStyle, actual float64) {
	dim := dxf.Dimension{Kind: dxf.DimAligned}
	d.assignDimensionDefaults(&dim.Entity, hs)
	dim.Pt3 = toCoord(def1)
	dim.Pt4 = toCoord(def2)
	dim.Def = toCoord(dimp)
	dim.TextPoint = toCoord(textp)
	dim.Text = text
	dim.Actual, dim.HasActual = actual, true
	d.dxf.WriteDimension(&dim)
}

func (d *dxfWriteInterface) writeLinearDimension(def1, def2, dimp, textp geom.Vector,
	text string, angle, oblique float64, hs core.HStyle, actual float64) {
	dim := dxf.Dimension{Kind: dxf.DimLinear}
	d.assignDimensionDefaults(&dim.Entity, hs)
	dim.Pt3 = toCoord(def1)
	dim.Pt4 = toCoord(def2)
	dim.Def = toCoord(dimp)
	dim.TextPoint = toCoord(textp)
	dim.Text = text
	dim.Angle = angle
	dim.Oblique = oblique
	dim.Actual, dim.HasActual = actual, true
	d.dxf.WriteDimension(&dim)
}

func (d *dxfWriteInterface) writeRadialDimension(center, radius, textp geom.Vector,
	text string, hs core.HStyle, actual float64) {
	dim := dxf.Dimension{Kind: dxf.DimRadial}
	d.assignDimensionDefaults(&dim.Entity, hs)
	dim.Def = toCoord(center)
	dim.Pt5 = toCoord(radius)
	dim.TextPoint = toCoord(textp)
	dim.Text = text
	dim.Actual, dim.HasActual = actual, true
	d.dxf.WriteDimension(&dim)
}

func (d *dxfWriteInterface) writeDiametricDimension(def1, def2, textp geom.Vector,
	text string, hs core.HStyle, actual float64) {
	dim := dxf.Dimension{Kind: dxf.DimDiametric}
	d.assignDimensionDefaults(&dim.Entity, hs)
	dim.Pt5 = toCoord(def1)
	dim.Def = toCoord(def2)
	dim.TextPoint = toCoord(textp)
	dim.Text = text
	dim.Actual, dim.HasActual = actual, true
	d.dxf.WriteDimension(&dim)
}

func (d *dxfWriteInterface) writeAngularDimension(fl1, fl2, sl1, sl2, dimp, textp geom.Vector,
	text string, hs core.HStyle, actual float64) {
	dim := dxf.Dimension{Kind: dxf.DimAngular}
	d.assignDimensionDefaults(&dim.Entity, hs)
	dim.Pt3 = toCoord(fl1)
	dim.Pt4 = toCoord(fl2)
	dim.Pt5 = toCoord(sl1)
	dim.Def = toCoord(sl2)
	dim.Pt6 = toCoord(dimp)
	dim.TextPoint = toCoord(textp)
	dim.Text = text
	dim.Actual, dim.HasActual = actual*math.Pi/180.0, true
	d.dxf.WriteDimension(&dim)
}

func (d *dxfWriteInterface) writeText(textp geom.Vector, text string,
	height, angle float64, origin core.TextOrigin, hs core.HStyle) {
	var txt dxf.Text
	d.assignEntityDefaults(&txt.Entity, hs)
	txt.Layer = "text"
	txt.Style = "unicode"
	txt.BasePoint = toCoord(textp)
	txt.SecPoint = txt.BasePoint
	txt.Text = text
	txt.Height = height
	txt.Angle = angle
	txt.AlignH = dxf.HCenter
	if origin&core.TextOriginLeft != 0 {
		txt.AlignH = dxf.HLeft
	} else if origin&core.TextOriginRight != 0 {
		txt.AlignH = dxf.HRight
	}
	txt.AlignV = dxf.VMiddle
	if origin&core.TextOriginTop != 0 {
		txt.AlignV = dxf.VTop
	} else if origin&core.TextOriginBot != 0 {
		txt.AlignV = dxf.VBaseLine
	}
	d.dxf.WriteText(&txt)
}
