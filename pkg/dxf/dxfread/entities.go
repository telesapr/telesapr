package dxfread

// The entities SolveSpace imports, as libdxfrw parses them (C++
// drw_entities.cpp and the LAYER table entry of drw_objects.cpp).

import (
	"math"

	"github.com/telesapr/telesapr/pkg/libm"
)

// ARAD converts radians to degrees (C++ ARAD).
const ARAD = 57.29577951308232

// Coord is a point (C++ DRW_Coord).
type Coord struct{ X, Y, Z float64 }

// unitize scales c to unit length (C++ DRW_Coord::unitize).
func (c *Coord) unitize() {
	dist := libm.Hypot(libm.Hypot(c.X, c.Y), c.Z)
	if dist > 0.0 {
		c.X = c.X / dist
		c.Y = c.Y / dist
		c.Z = c.Z / dist
	}
}

// LineWidth is a line weight (C++ DRW_LW_Conv::lineWidth): the standard
// weights 0 to 23, and by layer, by block and the default.
type LineWidth int

const (
	WidthByLayer LineWidth = 29
	WidthByBlock LineWidth = 30
	WidthDefault LineWidth = 31
)

// dxfInt2lineWidth is C++ DRW_LW_Conv::dxfInt2lineWidth.
func dxfInt2lineWidth(i int) LineWidth {
	if i < 0 {
		switch i {
		case -1:
			return WidthByLayer
		case -2:
			return WidthByBlock
		case -3:
			return WidthDefault
		}
		return WidthDefault
	}
	limits := []int{3, 7, 11, 14, 16, 19, 22, 27, 32, 37, 45, 52, 57, 65, 75, 85, 95, 103, 112, 130, 149, 180, 205}
	for w, l := range limits {
		if i < l {
			return LineWidth(w)
		}
	}
	return 23
}

// DXFInt is the line weight as DXF writes it, in hundredths of a mm (C++
// DRW_LW_Conv::lineWidth2dxfInt).
func (lw LineWidth) DXFInt() int {
	switch lw {
	case WidthByLayer:
		return -1
	case WidthByBlock:
		return -2
	case WidthDefault:
		return -3
	}
	widths := []int{0, 5, 9, 13, 15, 18, 20, 25, 30, 35, 40, 50, 53, 60, 70, 80, 90, 100, 106, 120, 140, 158, 200, 211}
	if lw >= 0 && int(lw) < len(widths) {
		return widths[lw]
	}
	return -3
}

// Colours by block and by layer (C++ DRW::ColorByBlock and
// ColorByLayer).
const (
	ColorByBlock = 0
	ColorByLayer = 256
)

// ModelSpace and PaperSpace are the spaces (C++ DRW::Space).
const (
	ModelSpace = 0
	PaperSpace = 1
)

// Entity is what every entity has (C++ DRW_Entity).
type Entity struct {
	Handle   int
	Space    int
	Layer    string
	LineType string
	Color    int
	LWeight  LineWidth
	Color24  int

	// The arbitrary axis the extrusion uses (C++ extAxisX and
	// extAxisY).
	extAxisX, extAxisY Coord
}

func newEntity() Entity {
	return Entity{Layer: "0", LineType: "BYLAYER", Color: ColorByLayer, LWeight: WidthByLayer, Color24: -1}
}

// calculateAxis finds the arbitrary axis of an extrusion (C++
// DRW_Entity::calculateAxis).
func (e *Entity) calculateAxis(extPoint Coord) {
	// Follow the arbitrary DXF definitions for extrusion axes.
	if math.Abs(extPoint.X) < 0.015625 && math.Abs(extPoint.Y) < 0.015625 {
		// Ax = Wy x N
		e.extAxisX.X = extPoint.Z
		e.extAxisX.Y = 0
		e.extAxisX.Z = -extPoint.X
	} else {
		// Ax = Wz x N
		e.extAxisX.X = -extPoint.Y
		e.extAxisX.Y = extPoint.X
		e.extAxisX.Z = 0
	}

	e.extAxisX.unitize()

	// Ay = N x Ax
	e.extAxisY.X = (extPoint.Y * e.extAxisX.Z) - (e.extAxisX.Y * extPoint.Z)
	e.extAxisY.Y = (extPoint.Z * e.extAxisX.X) - (e.extAxisX.Z * extPoint.X)
	e.extAxisY.Z = (extPoint.X * e.extAxisX.Y) - (e.extAxisX.X * extPoint.Y)

	e.extAxisY.unitize()
}

// extrudePoint takes a point by the arbitrary axis (C++
// DRW_Entity::extrudePoint).
func (e *Entity) extrudePoint(extPoint Coord, point *Coord) {
	px := (e.extAxisX.X * point.X) + (e.extAxisY.X * point.Y) + (extPoint.X * point.Z)
	py := (e.extAxisX.Y * point.X) + (e.extAxisY.Y * point.Y) + (extPoint.Y * point.Z)
	pz := (e.extAxisX.Z * point.X) + (e.extAxisY.Z * point.Y) + (extPoint.Z * point.Z)

	point.X = px
	point.Y = py
	point.Z = pz
}

// parseEntity is C++ DRW_Entity::parseCode, for what SolveSpace uses.
func (e *Entity) parseEntity(code int, r *reader) bool {
	switch code {
	case 5:
		e.Handle = r.getHandleString()
	case 8:
		e.Layer = r.getUtf8String()
	case 6:
		e.LineType = r.getUtf8String()
	case 62:
		e.Color = r.getInt32()
	case 370:
		e.LWeight = dxfInt2lineWidth(r.getInt32())
	case 420:
		e.Color24 = r.getInt32()
	case 67:
		e.Space = r.getInt32()
	}
	return true
}

// Point is a POINT, and what the entities built on it have (C++
// DRW_Point).
type Point struct {
	Entity
	BasePoint     Coord
	ExtPoint      Coord
	Thickness     float64
	HaveExtrusion bool
}

func newPoint() Point {
	return Point{Entity: newEntity(), ExtPoint: Coord{Z: 1}}
}

func (p *Point) parsePoint(code int, r *reader) bool {
	switch code {
	case 10:
		p.BasePoint.X = r.getDouble()
	case 20:
		p.BasePoint.Y = r.getDouble()
	case 30:
		p.BasePoint.Z = r.getDouble()
	case 39:
		p.Thickness = r.getDouble()
	case 210:
		p.HaveExtrusion = true
		p.ExtPoint.X = r.getDouble()
	case 220:
		p.ExtPoint.Y = r.getDouble()
	case 230:
		p.ExtPoint.Z = r.getDouble()
	default:
		return p.parseEntity(code, r)
	}
	return true
}

// Line is a LINE (C++ DRW_Line).
type Line struct {
	Point
	SecPoint Coord
}

func newLine() Line { return Line{Point: newPoint()} }

func (l *Line) parseLine(code int, r *reader) bool {
	switch code {
	case 11:
		l.SecPoint.X = r.getDouble()
	case 21:
		l.SecPoint.Y = r.getDouble()
	case 31:
		l.SecPoint.Z = r.getDouble()
	default:
		return l.parsePoint(code, r)
	}
	return true
}

// Circle is a CIRCLE (C++ DRW_Circle).
type Circle struct {
	Point
	Radius float64
}

func newCircle() Circle { return Circle{Point: newPoint()} }

func (c *Circle) parseCircle(code int, r *reader) bool {
	if code == 40 {
		c.Radius = r.getDouble()
		return true
	}
	return c.parsePoint(code, r)
}

// applyExtrusion is C++ DRW_Circle::applyExtrusion.
func (c *Circle) applyExtrusion() {
	if c.HaveExtrusion {
		// NOTE: Commenting these out causes the the arcs being tested to
		// be located on the other side of the y axis (all x dimensions
		// are negated).
		c.calculateAxis(c.ExtPoint)
		c.extrudePoint(c.ExtPoint, &c.BasePoint)
	}
}

// Arc is an ARC, with its angles in radians (C++ DRW_Arc).
type Arc struct {
	Circle
	StaAngle, EndAngle float64
}

func newArc() Arc { return Arc{Circle: newCircle()} }

func (a *Arc) parseArc(code int, r *reader) bool {
	switch code {
	case 50:
		a.StaAngle = r.getDouble() / ARAD
	case 51:
		a.EndAngle = r.getDouble() / ARAD
	default:
		return a.parseCircle(code, r)
	}
	return true
}

// applyExtrusion is C++ DRW_Arc::applyExtrusion.
func (a *Arc) applyExtrusion() {
	a.Circle.applyExtrusion()
	if a.HaveExtrusion {
		// If the extrusion vector has a z value less than 0, the angles
		// for the arc have to be mirrored since DXF files use the right
		// hand rule.
		if math.Abs(a.ExtPoint.X) < 0.015625 && math.Abs(a.ExtPoint.Y) < 0.015625 && a.ExtPoint.Z < 0.0 {
			a.StaAngle = math.Pi - a.StaAngle
			a.EndAngle = math.Pi - a.EndAngle
			a.StaAngle, a.EndAngle = a.EndAngle, a.StaAngle
		}
	}
}

// Block is a BLOCK's header (C++ DRW_Block).
type Block struct {
	Point
	Name  string
	Flags int
}

func newBlock() Block {
	b := Block{Point: newPoint(), Name: "*U0"}
	b.Layer = "0"
	return b
}

func (b *Block) parseBlock(code int, r *reader) bool {
	switch code {
	case 2:
		b.Name = r.getUtf8String()
	case 70:
		b.Flags = r.getInt32()
	default:
		return b.parsePoint(code, r)
	}
	return true
}

// Insert is an INSERT of a block (C++ DRW_Insert).
type Insert struct {
	Point
	Name                   string
	XScale, YScale, ZScale float64
	Angle                  float64 // radians
	ColCount, RowCount     int
	ColSpace, RowSpace     float64
}

func newInsert() Insert {
	return Insert{Point: newPoint(), XScale: 1, YScale: 1, ZScale: 1, ColCount: 1, RowCount: 1}
}

func (in *Insert) parseInsert(code int, r *reader) bool {
	switch code {
	case 2:
		in.Name = r.getUtf8String()
	case 41:
		in.XScale = r.getDouble()
	case 42:
		in.YScale = r.getDouble()
	case 43:
		in.ZScale = r.getDouble()
	case 50:
		in.Angle = r.getDouble()
		in.Angle = in.Angle / ARAD // convert to radian
	case 70:
		in.ColCount = r.getInt32()
	case 71:
		in.RowCount = r.getInt32()
	case 44:
		in.ColSpace = r.getDouble()
	case 45:
		in.RowSpace = r.getDouble()
	default:
		return in.parsePoint(code, r)
	}
	return true
}

// Vertex2D is a light polyline's vertex (C++ DRW_Vertex2D).
type Vertex2D struct {
	X, Y               float64
	StaWidth, EndWidth float64
	Bulge              float64
}

// LWPolyline is an LWPOLYLINE (C++ DRW_LWPolyline).
type LWPolyline struct {
	Entity
	VertexNum     int
	Flags         int
	Width         float64
	Elevation     float64
	Thickness     float64
	ExtPoint      Coord
	HaveExtrusion bool
	Vertices      []*Vertex2D
	vertex        *Vertex2D
}

func newLWPolyline() LWPolyline {
	return LWPolyline{Entity: newEntity(), ExtPoint: Coord{Z: 1}}
}

// reserveOK is whether std::vector::reserve succeeds for n elements.
func reserveOK(n int) bool { return n >= 0 && n <= 1<<28 }

func (p *LWPolyline) parseLWPolyline(code int, r *reader) bool {
	switch code {
	case 10:
		p.vertex = &Vertex2D{}
		p.Vertices = append(p.Vertices, p.vertex)
		p.vertex.X = r.getDouble()
	case 20:
		if p.vertex != nil {
			p.vertex.Y = r.getDouble()
		}
	case 40:
		if p.vertex != nil {
			p.vertex.StaWidth = r.getDouble()
		}
	case 41:
		if p.vertex != nil {
			p.vertex.EndWidth = r.getDouble()
		}
	case 42:
		if p.vertex != nil {
			p.vertex.Bulge = r.getDouble()
		}
	case 38:
		p.Elevation = r.getDouble()
	case 39:
		p.Thickness = r.getDouble()
	case 43:
		p.Width = r.getDouble()
	case 70:
		p.Flags = r.getInt32()
	case 90:
		p.VertexNum = r.getInt32()
		return reserveOK(p.VertexNum)
	case 210:
		p.HaveExtrusion = true
		p.ExtPoint.X = r.getDouble()
	case 220:
		p.ExtPoint.Y = r.getDouble()
	case 230:
		p.ExtPoint.Z = r.getDouble()
	default:
		return p.parseEntity(code, r)
	}
	return true
}

// applyExtrusion is C++ DRW_LWPolyline::applyExtrusion.
func (p *LWPolyline) applyExtrusion() {
	if p.HaveExtrusion {
		p.calculateAxis(p.ExtPoint)
		for _, vert := range p.Vertices {
			v := Coord{vert.X, vert.Y, p.Elevation}
			p.extrudePoint(p.ExtPoint, &v)
			vert.X = v.X
			vert.Y = v.Y
		}
	}
}

// Vertex is a POLYLINE's VERTEX (C++ DRW_Vertex).
type Vertex struct {
	Point
	StaWidth, EndWidth float64
	Bulge              float64
	Flags              int
	TgDir              float64
}

func newVertex() *Vertex { return &Vertex{Point: newPoint()} }

func (v *Vertex) parseVertex(code int, r *reader) bool {
	switch code {
	case 70:
		v.Flags = r.getInt32()
	case 40:
		v.StaWidth = r.getDouble()
	case 41:
		v.EndWidth = r.getDouble()
	case 42:
		v.Bulge = r.getDouble()
	case 50:
		v.TgDir = r.getDouble()
	case 71, 72, 73, 74, 91:
		// Mesh indices and identifier; SolveSpace doesn't use them.
	default:
		return v.parsePoint(code, r)
	}
	return true
}

// Polyline is a POLYLINE (C++ DRW_Polyline).
type Polyline struct {
	Point
	Flags    int
	Vertices []*Vertex
}

func newPolyline() Polyline { return Polyline{Point: newPoint()} }

func (p *Polyline) parsePolyline(code int, r *reader) bool {
	switch code {
	case 70:
		p.Flags = r.getInt32()
	case 40, 41, 71, 72, 73, 74, 75:
		// Widths, mesh counts, smoothing; SolveSpace doesn't use them.
	default:
		return p.parsePoint(code, r)
	}
	return true
}

// Spline is a SPLINE (C++ DRW_Spline).
type Spline struct {
	Entity
	Degree   int
	Controls []*Coord
	control  *Coord
}

func newSpline() Spline { return Spline{Entity: newEntity()} }

func (s *Spline) parseSpline(code int, r *reader) bool {
	switch code {
	case 71:
		s.Degree = r.getInt32()
	case 10:
		s.control = &Coord{}
		s.Controls = append(s.Controls, s.control)
		s.control.X = r.getDouble()
	case 20:
		if s.control != nil {
			s.control.Y = r.getDouble()
		}
	case 30:
		if s.control != nil {
			s.control.Z = r.getDouble()
		}
	case 210, 220, 230, 12, 22, 32, 13, 23, 33, 70, 72, 73, 74, 42, 43, 44, 11, 21, 31, 40, 41:
		// The normal, tangents, flags, counts, tolerances, fit points,
		// knots and weights; SolveSpace doesn't use them.
	default:
		return s.parseEntity(code, r)
	}
	return true
}

// Text alignments (C++ DRW_Text::HAlign and VAlign).
const (
	HLeft    = 0
	HCenter  = 1
	HRight   = 2
	HAligned = 3
	HMiddle  = 4
	HFit     = 5

	VBaseLine = 0
	VBottom   = 1
	VMiddle   = 2
	VTop      = 3
)

// Text is a TEXT (C++ DRW_Text).
type Text struct {
	Line
	Height     float64
	Text       string
	Angle      float64 // degrees
	WidthScale float64
	Oblique    float64
	Style      string
	TextGen    int
	AlignH     int
	AlignV     int
}

func newText() Text {
	return Text{Line: newLine(), WidthScale: 1, Style: "STANDARD", AlignH: HLeft, AlignV: VBaseLine}
}

func (t *Text) parseText(code int, r *reader) bool {
	switch code {
	case 40:
		t.Height = r.getDouble()
	case 41:
		t.WidthScale = r.getDouble()
	case 50:
		t.Angle = r.getDouble()
	case 51:
		t.Oblique = r.getDouble()
	case 71:
		t.TextGen = r.getInt32()
	case 72:
		t.AlignH = r.getInt32()
	case 73:
		t.AlignV = r.getInt32()
	case 1:
		t.Text = r.getUtf8String()
	case 7:
		t.Style = r.getUtf8String()
	default:
		return t.parseLine(code, r)
	}
	return true
}

// MText is an MTEXT (C++ DRW_MText).
type MText struct {
	Text
	InterLin    float64
	hasXAxisVec bool
}

func newMText() MText {
	m := MText{Text: newText(), InterLin: 1}
	// C++ sets alignV to (VAlign)TopLeft, which is 1: VBottom.
	m.AlignV = VBottom
	m.TextGen = 1
	return m
}

// The MTEXT attachment points (C++ DRW_MText::Attach).
const (
	topLeft = iota + 1
	topCenter
	topRight
	middleLeft
	middleCenter
	middleRight
	bottomLeft
	bottomCenter
	bottomRight
)

func (m *MText) parseMText(code int, r *reader) bool {
	switch code {
	case 1:
		m.Text.Text += r.getString()
		m.Text.Text = r.decoder.toUtf8(m.Text.Text)
	case 11:
		m.hasXAxisVec = true
		return m.parseText(code, r)
	case 3:
		m.Text.Text += r.getString()
	case 44:
		m.InterLin = r.getDouble()
	case 50: // per dxf docs, last of code 11 or code 50 prevails
		m.hasXAxisVec = false
		m.Angle = r.getDouble()
	case 71:
		// Attachment point
		switch r.getInt32() {
		case topLeft:
			m.AlignV, m.AlignH = VTop, HLeft
		case topCenter:
			m.AlignV, m.AlignH = VTop, HCenter
		case topRight:
			m.AlignV, m.AlignH = VTop, HRight
		case middleLeft:
			m.AlignV, m.AlignH = VMiddle, HLeft
		case middleCenter:
			m.AlignV, m.AlignH = VMiddle, HCenter
		case middleRight:
			m.AlignV, m.AlignH = VMiddle, HRight
		case bottomLeft:
			m.AlignV, m.AlignH = VBottom, HLeft
		case bottomCenter:
			m.AlignV, m.AlignH = VBottom, HCenter
		case bottomRight:
			m.AlignV, m.AlignH = VBottom, HRight
		}
	case 72, 73:
		// Their meanings differ for MTEXT; not passed to TEXT's.
	default:
		return m.parseText(code, r)
	}
	return true
}

// updateAngle is C++ DRW_MText::updateAngle.
func (m *MText) updateAngle() {
	if m.hasXAxisVec {
		m.Angle = libm.Atan2(m.SecPoint.Y, m.SecPoint.X) * ARAD
	}
}

// DimensionKind is which DIMENSION it is, from the low bits of its
// type (the C++ DRW_Dimension subclasses).
type DimensionKind int

const (
	DimLinear DimensionKind = iota
	DimAligned
	DimAngular
	DimDiametric
	DimRadial
	DimAngular3P
	DimOrdinate
)

// Dimension is a DIMENSION (C++ DRW_Dimension). Its points are named
// by their DXF codes: Def is 10, Text 11, Clone 12, Def1 13, Def2 14,
// Circle 15 and Arc 16.
type Dimension struct {
	Entity
	Kind       DimensionKind
	Type       int
	Name       string
	Style      string
	Text       string
	Align      int
	LineSty    int
	LineFactor float64
	Rot        float64
	Angle      float64
	Oblique    float64
	Length     float64
	HDir       float64

	Def, TextPoint, Clone, Def1, Def2, CirclePoint, ArcPoint Coord
	ExtPoint                                                 Coord

	MeasureValue    float64
	HasMeasureValue bool
}

func newDimension() Dimension {
	return Dimension{
		Entity: newEntity(), LineSty: 1, LineFactor: 1, ExtPoint: Coord{Z: 1},
		Align: 5, Style: "STANDARD",
	}
}

func (d *Dimension) parseDimension(code int, r *reader) bool {
	switch code {
	case 1:
		d.Text = r.getUtf8String()
	case 2:
		d.Name = r.getString()
	case 3:
		d.Style = r.getUtf8String()
	case 70:
		d.Type = r.getInt32()
	case 71:
		d.Align = r.getInt32()
	case 72:
		d.LineSty = r.getInt32()
	case 10:
		d.Def.X = r.getDouble()
	case 20:
		d.Def.Y = r.getDouble()
	case 30:
		d.Def.Z = r.getDouble()
	case 11:
		d.TextPoint.X = r.getDouble()
	case 21:
		d.TextPoint.Y = r.getDouble()
	case 31:
		d.TextPoint.Z = r.getDouble()
	case 12:
		d.Clone.X = r.getDouble()
	case 22:
		d.Clone.Y = r.getDouble()
	case 32:
		d.Clone.Z = r.getDouble()
	case 13:
		d.Def1.X = r.getDouble()
	case 23:
		d.Def1.Y = r.getDouble()
	case 33:
		d.Def1.Z = r.getDouble()
	case 14:
		d.Def2.X = r.getDouble()
	case 24:
		d.Def2.Y = r.getDouble()
	case 34:
		d.Def2.Z = r.getDouble()
	case 15:
		d.CirclePoint.X = r.getDouble()
	case 25:
		d.CirclePoint.Y = r.getDouble()
	case 35:
		d.CirclePoint.Z = r.getDouble()
	case 16:
		d.ArcPoint.X = r.getDouble()
	case 26:
		d.ArcPoint.Y = r.getDouble()
	case 36:
		d.ArcPoint.Z = r.getDouble()
	case 41:
		d.LineFactor = r.getDouble()
	case 53:
		d.Rot = r.getDouble()
	case 50:
		d.Angle = r.getDouble()
	case 52:
		d.Oblique = r.getDouble()
	case 40:
		d.Length = r.getDouble()
	case 51:
		d.HDir = r.getDouble()
	case 42:
		d.MeasureValue = r.getDouble()
		d.HasMeasureValue = d.MeasureValue != 0.0
		// C++ falls through to the next case.
		d.ExtPoint.X = r.getDouble()
	case 210:
		d.ExtPoint.X = r.getDouble()
	case 220:
		d.ExtPoint.Y = r.getDouble()
	case 230:
		d.ExtPoint.Z = r.getDouble()
	default:
		return d.parseEntity(code, r)
	}
	return true
}

// The points of each kind of dimension, by their C++ accessors.

func (d *Dimension) DimPoint() Coord       { return d.Def }         // aligned, linear
func (d *Dimension) CenterPoint() Coord    { return d.Def }         // radial
func (d *Dimension) DiameterPoint() Coord  { return d.CirclePoint } // radial
func (d *Dimension) Diameter1Point() Coord { return d.CirclePoint } // diametric
func (d *Dimension) Diameter2Point() Coord { return d.Def }         // diametric
func (d *Dimension) FirstLine1() Coord     { return d.Def1 }        // angular
func (d *Dimension) FirstLine2() Coord     { return d.Def2 }        // angular
func (d *Dimension) SecondLine1() Coord    { return d.CirclePoint } // angular
func (d *Dimension) SecondLine2() Coord    { return d.Def }         // angular
func (d *Dimension) FirstLine() Coord      { return d.Def1 }        // 3-point angular
func (d *Dimension) SecondLine() Coord     { return d.Def2 }        // 3-point angular
func (d *Dimension) VertexPoint() Coord    { return d.CirclePoint } // 3-point angular

// Layer is a LAYER table entry (C++ DRW_Layer).
type Layer struct {
	Name     string
	Flags    int
	LineType string
	Color    int
	Color24  int
	PlotF    bool
	LWeight  LineWidth
}

// reset is C++ DRW_Layer::reset, which keeps the name.
func (l *Layer) reset() {
	l.LineType = "CONTINUOUS"
	l.Color = 7
	l.PlotF = true
	l.LWeight = WidthDefault
	l.Color24 = -1
	l.Flags = 0
}

func (l *Layer) parseLayer(code int, r *reader) bool {
	switch code {
	case 6:
		l.LineType = r.getUtf8String()
	case 62:
		l.Color = r.getInt32()
	case 290:
		l.PlotF = r.getBool()
	case 370:
		l.LWeight = dxfInt2lineWidth(r.getInt32())
	case 420:
		l.Color24 = r.getInt32()
	case 2:
		l.Name = r.getUtf8String()
	case 70:
		l.Flags = r.getInt32()
	}
	return true
}
