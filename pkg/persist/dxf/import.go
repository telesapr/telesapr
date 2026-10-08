// Package dxf imports a DXF drawing into a sketch: its lines, arcs,
// circles, splines, text and dimensions become requests and
// constraints of the active group, in its workplane or in 3d.
package dxf

// DXF import, reading through pkg/dxf/dxfread.

import (
	"errors"
	"math"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	dxffmt "github.com/telesapr/telesapr/pkg/dxf"
	"github.com/telesapr/telesapr/pkg/dxf/dxfread"
	"github.com/telesapr/telesapr/pkg/export"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// normalFromExtPoint is the DXF arbitrary axis algorithm for turning a
// Z vector into a rotated coordinate system.
func normalFromExtPoint(extPoint geom.Vector) geom.Quaternion {
	var ax, ay geom.Vector
	az := extPoint.WithMagnitude(1.0)

	if math.Abs(az.X) < 1/64. && math.Abs(az.Y) < 1/64. {
		ax = geom.Vector{Y: 1}.Cross(az).WithMagnitude(1.0)
	} else {
		ax = geom.Vector{Z: 1}.Cross(az).WithMagnitude(1.0)
	}
	ay = az.Cross(ax).WithMagnitude(1.0)
	return geom.QuaternionFromBasis(ax, ay)
}

func coordVec(c dxfread.Coord) geom.Vector { return geom.Vector{X: c.X, Y: c.Y, Z: c.Z} }

// importPointKey is a point's cell: points are found only among those
// whose coordinates fall in the same cell.
func importPointKey(v geom.Vector) uint64 {
	const size = uint64(math.MaxUint64/2 - 1)
	const eps = 4.0 * core.LengthEps

	x := math.Abs(v.X) / eps
	y := math.Abs(v.Y) / eps

	xs := uint64(math.Mod(x, float64(size)))
	ys := uint64(math.Mod(y, float64(size)))

	return ys*size + xs
}

type importPoint struct {
	pos geom.Vector
	he  core.HEntity
}

// importBlock is a block's entities, kept to insert later.
type importBlock struct {
	entities []any
	data     dxfread.Block
}

// dxfImport makes requests and constraints from a DXF file's entities.
type dxfImport struct {
	sketch     *sketch.Sketch
	textHeight float64

	blockX, blockY, blockZ, blockT geom.Vector

	asConstruction  bool
	unknownEntities int
	styles          map[string]core.HStyle
	blocks          map[string]*importBlock
	layers          map[string]*dxfread.Layer
	readBlock       *importBlock
	insertInsert    *dxfread.Insert

	// points are the points by cell, newest first, as a lookup meets
	// them.
	points map[uint64][]importPoint
}

func newDxfImport(sk *sketch.Sketch, textHeight float64) *dxfImport {
	return &dxfImport{
		sketch:     sk,
		textHeight: textHeight,
		styles:     map[string]core.HStyle{},
		blocks:     map[string]*importBlock{},
		layers:     map[string]*dxfread.Layer{},
		points:     map[uint64][]importPoint{},
	}
}

func (d *dxfImport) sk() *sketch.Sketch { return d.sketch }

// activeWorkplane is the active group's workplane, none in 3d.
func (d *dxfImport) activeWorkplane() core.HEntity {
	if g := d.sketch.Group(d.sketch.ActiveGroup()); g != nil {
		return g.ActiveWorkplane
	}
	return core.HEntity{}
}

// lockedInWorkplane reports whether the active group draws in a
// workplane.
func (d *dxfImport) lockedInWorkplane() bool { return !core.IsFreeIn3D(d.activeWorkplane()) }

func (d *dxfImport) invertXTransform() {
	d.blockX.X = -d.blockX.X
	d.blockY.X = -d.blockY.X
	d.blockT.X = -d.blockT.X
}

func (d *dxfImport) multBlockTransform(x, y, sx, sy, angle float64) {
	oldX, oldY, oldT := d.blockX, d.blockY, d.blockT

	newX := geom.Vector{X: sx}.RotatedAroundAxis(geom.Vector{Z: 1}, angle)
	newY := geom.Vector{Y: sy}.RotatedAroundAxis(geom.Vector{Z: 1}, angle)
	newT := geom.Vector{X: x, Y: y}

	d.blockX = oldX.ScaledBy(newX.X).Plus(oldY.ScaledBy(newX.Y))
	d.blockY = oldX.ScaledBy(newY.X).Plus(oldY.ScaledBy(newY.Y))
	d.blockT = oldX.ScaledBy(newT.X).Plus(oldY.ScaledBy(newT.Y)).Plus(oldT)
}

func (d *dxfImport) clearBlockTransform() {
	d.blockX = geom.Vector{X: 1}
	d.blockY = geom.Vector{Y: 1}
	d.blockZ = geom.Vector{Z: 1}
	d.blockT = geom.Vector{}
}

func (d *dxfImport) blockTransform(v geom.Vector) geom.Vector {
	r := d.blockT
	r = r.Plus(d.blockX.ScaledBy(v.X))
	r = r.Plus(d.blockY.ScaledBy(v.Y))
	r = r.Plus(d.blockZ.ScaledBy(v.Z))
	return r
}

func (d *dxfImport) blockTransformArc(c, p0, p1 *geom.Vector) {
	oldSign := p0.Minus(*c).Cross(p1.Minus(*c)).Z > 0.0

	*c = d.blockTransform(*c)
	*p0 = d.blockTransform(*p0)
	*p1 = d.blockTransform(*p1)

	newSign := p0.Minus(*c).Cross(p1.Minus(*c)).Z > 0.0
	if oldSign != newSign {
		*p0, *p1 = *p1, *p0
	}
}

func (d *dxfImport) toVector(c dxfread.Coord) geom.Vector { return d.blockTransform(coordVec(c)) }

func angleTo(v0, v1 geom.Vector) float64 {
	dv := v1.Minus(v0)
	a := libm.Atan2(dv.Y, dv.X)
	return math.Pi + math.Remainder(a-math.Pi, 2*math.Pi)
}

func polar(radius, angle float64) geom.Vector {
	return geom.Vector{X: radius * libm.Cos(angle), Y: radius * libm.Sin(angle)}
}

func (d *dxfImport) createBulge(p0, p1 geom.Vector, bulge float64) core.HRequest {
	reversed := bulge < 0.0
	alpha := libm.Atan(bulge) * 4.0

	middle := p1.Plus(p0).ScaledBy(0.5)
	dist := p1.Minus(p0).Magnitude() / 2.0
	angle := angleTo(p0, p1)

	// alpha can't be 0.0 at this point
	radius := math.Abs(dist / libm.Sin(alpha/2.0))
	wu := math.Abs(radius*radius - dist*dist)
	h := math.Sqrt(wu)

	if bulge > 0.0 {
		angle += math.Pi / 2
	} else {
		angle -= math.Pi / 2
	}

	if math.Abs(alpha) > math.Pi {
		h *= -1.0
	}

	center := polar(h, angle)
	center = center.Plus(middle)

	if reversed {
		p0, p1 = p1, p0
	}
	d.blockTransformArc(&center, &p0, &p1)

	sk := d.sk()
	hr := d.sk().AddRequestOfType(sketch.RequestArcOfCircle, false)
	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, center)
	sk.MustEntity(hr.Entity(2)).PointForceTo(sk, p0)
	sk.MustEntity(hr.Entity(3)).PointForceTo(sk, p1)
	d.processPoint(hr.Entity(1))
	d.processPoint(hr.Entity(2))
	d.processPoint(hr.Entity(3))
	return hr
}

// untypedDimension is a dimension kept in a block: its type is lost in
// the copy, so inserting the block counts it as unknown.
type untypedDimension struct{}

// addPendingDimension keeps a dimension of the block being read, and
// reports whether it did.
func (d *dxfImport) addPendingDimension() bool {
	return d.addPendingBlockEntity(untypedDimension{})
}

// addPendingBlockEntity keeps an entity of the block being read, and
// reports whether it did.
func (d *dxfImport) addPendingBlockEntity(e any) bool {
	if d.readBlock == nil {
		return false
	}
	d.readBlock.entities = append(d.readBlock.entities, e)
	return true
}

// addEntity adds an inserted block's entity.
func (d *dxfImport) addEntity(e any) {
	switch e := e.(type) {
	case dxfread.Point:
		d.AddPoint(&e)
	case dxfread.Line:
		d.AddLine(&e)
	case dxfread.Arc:
		d.AddArc(&e)
	case dxfread.Circle:
		d.AddCircle(&e)
	case dxfread.Polyline:
		d.AddPolyline(&e)
	case dxfread.LWPolyline:
		d.AddLWPolyline(&e)
	case dxfread.Spline:
		d.AddSpline(&e)
	case dxfread.Insert:
		d.AddInsert(&e)
	case dxfread.Text:
		d.AddText(&e)
	case dxfread.MText:
		d.AddMText(&e)
	case dxfread.Dimension:
		// Only a diametric dimension kept as a radial one.
		d.AddDimension(&e)
	default:
		d.unknownEntities++
	}
}

// dxfAlignToOrigin is a text's alignment as a style's origin.
func dxfAlignToOrigin(alignH, alignV int) core.TextOrigin {
	var origin core.TextOrigin
	switch alignH {
	case dxfread.HLeft:
		origin |= core.TextOriginLeft
	case dxfread.HMiddle, dxfread.HCenter:
	case dxfread.HRight:
		origin |= core.TextOriginRight
	default: // HAligned, HFit
		origin |= core.TextOriginLeft
	}

	switch alignV {
	case dxfread.VBaseLine, dxfread.VBottom:
		origin |= core.TextOriginBot
	case dxfread.VMiddle:
	case dxfread.VTop:
		origin |= core.TextOriginTop
	default:
		origin |= core.TextOriginBot
	}

	return origin
}

func (d *dxfImport) getSourceLayer(e *dxfread.Entity) *dxfread.Layer {
	l := e.Layer
	if d.insertInsert != nil {
		l = d.insertInsert.Layer
	}
	return d.layers[l]
}

func (d *dxfImport) getColor(e *dxfread.Entity) int {
	col := e.Color
	if col == dxfread.ColorByBlock {
		if d.insertInsert != nil {
			col = d.insertInsert.Color
		} else {
			col = 7
		}
	}
	if col == dxfread.ColorByLayer {
		if layer := d.getSourceLayer(e); layer != nil {
			col = layer.Color
		} else {
			col = 7
		}
	}
	return col
}

func (d *dxfImport) getLineWidth(e *dxfread.Entity) dxfread.LineWidth {
	result := e.LWeight
	if result == dxfread.WidthByBlock {
		if d.insertInsert != nil {
			result = d.insertInsert.LWeight
		} else {
			result = dxfread.WidthDefault
		}
	}
	if result == dxfread.WidthByLayer {
		if layer := d.getSourceLayer(e); layer != nil {
			result = layer.LWeight
		} else {
			result = dxfread.WidthDefault
		}
	}
	return result
}

func (d *dxfImport) getLineType(e *dxfread.Entity) string {
	result := e.LineType
	if result == "BYBLOCK" {
		if d.insertInsert != nil {
			result = strings.ToUpper(d.insertInsert.LineType)
		} else {
			result = "CONTINUOUS"
		}
	}
	if result == "BYLAYER" {
		if layer := d.getSourceLayer(e); layer != nil {
			result = strings.ToUpper(layer.LineType)
		} else {
			result = "CONTINUOUS"
		}
	}
	return result
}

func (d *dxfImport) invisibleStyle() core.HStyle {
	const id = "@dxf-invisible"

	if hs, ok := d.styles[id]; ok {
		return hs
	}

	hs := core.HStyle{V: d.sk().NewCustomStyle().V}
	s := d.sk().StyleGet(hs)
	s.Name = id
	s.Visible = false

	d.styles[id] = hs
	return hs
}

// dxfColor is a colour index's colour. A layer that is off has its
// colour negated, so that is read as the colour, and anything else out
// of range as 7.
func dxfColor(col int) core.RGBA {
	if col < 0 {
		col = -col
	}
	if col > 255 {
		col = 7
	}
	c := dxffmt.Colors[col]
	return core.NewRGBA(c[0], c[1], c[2], 255)
}

// styleFor is the custom style for an entity's colour, width, line type
// and, for texts, text properties.
func (d *dxfImport) styleFor(e *dxfread.Entity, text *dxfread.Text) core.HStyle {
	// Color.
	c := dxfColor(d.getColor(e))

	// Line width.
	lw := d.getLineWidth(e)
	width := float64(e.LWeight.DXFInt()) / 100.0
	if width < 0.0 {
		width = 1.0
	}

	// Line stipple.
	lineType := d.getLineType(e)
	stipple := core.StippleContinuous
	for st := core.StippleContinuous; st <= core.StippleZigzag; st++ {
		if lineType == export.DxfLineType(st) {
			stipple = st
			break
		}
	}

	// Text properties.
	alignH := dxfread.HLeft
	alignV := dxfread.VBaseLine
	textAngle := 0.0
	textHeight := d.textHeight

	if text != nil {
		alignH = text.AlignH
		alignV = text.AlignV
		textHeight = text.Height
		textAngle = text.Angle
		// I have no idea why, but works
		if alignH == dxfread.HMiddle {
			alignV = dxfread.VMiddle
		}
	}

	// Unique identifier based on style properties.
	id := "@dxf"
	if lw != dxfread.WidthDefault {
		id += cfmt.Sprintf("-w%.4g", width)
	}
	if lineType != "CONTINUOUS" {
		id += cfmt.Sprintf("-%s", lineType)
	}
	if c.Red != 0 || c.Green != 0 || c.Blue != 0 {
		id += cfmt.Sprintf("-#%02x%02x%02x", c.Red, c.Green, c.Blue)
	}
	if textHeight != d.textHeight {
		id += cfmt.Sprintf("-h%.4g", textHeight)
	}
	if textAngle != 0.0 {
		id += cfmt.Sprintf("-a%.5g", textAngle)
	}
	if alignH != dxfread.HLeft {
		id += cfmt.Sprintf("-oh%d", alignH)
	}
	if alignV != dxfread.VBaseLine {
		id += cfmt.Sprintf("-ov%d", alignV)
	}

	if hs, ok := d.styles[id]; ok {
		return hs
	}

	hs := core.HStyle{V: d.sk().NewCustomStyle().V}
	s := d.sk().StyleGet(hs)
	if lw != dxfread.WidthDefault {
		s.WidthAs = core.UnitsMM
		s.Width = width
		s.StippleScale = 1.0 + width*2.0
	}
	s.Name = id
	s.StippleType = stipple
	if c.Red != 0 || c.Green != 0 || c.Blue != 0 {
		s.Color = c
	}
	s.TextHeightAs = core.UnitsMM
	s.TextHeight = textHeight
	s.TextAngle = textAngle
	s.TextOrigin = dxfAlignToOrigin(alignH, alignV)

	d.styles[id] = hs
	return hs
}

func (d *dxfImport) configureRequest(hr core.HRequest, hs core.HStyle) {
	r := d.sk().Request(hr)
	r.Construction = d.asConstruction
	r.Style = hs
}

func (d *dxfImport) findPoint(p geom.Vector) core.HEntity {
	for _, ip := range d.points[importPointKey(p)] {
		if p.EqualsTol(ip.pos, core.LengthEps) {
			return ip.he
		}
	}
	return core.NoEntity
}

func (d *dxfImport) addPoint(p geom.Vector, he core.HEntity) {
	key := importPointKey(p)
	d.points[key] = append([]importPoint{{p, he}}, d.points[key]...)
}

func (d *dxfImport) processPoint(he core.HEntity) {
	sk := d.sk()
	pos := sk.MustEntity(he).PointGetNum(sk)
	p := d.findPoint(pos)
	if p == he {
		return
	}
	if p != core.NoEntity {
		// A point is already at this position: the new one is made
		// coincident with it rather than added.
		sk.ConstrainCoincident(he, p)
		return
	}
	d.addPoint(pos, he)
}

func (d *dxfImport) createOrGetPoint(p geom.Vector) core.HEntity {
	if he := d.findPoint(p); he != core.NoEntity {
		return he
	}

	sk := d.sk()
	hr := d.sk().AddRequestOfType(sketch.RequestDatumPoint, false)
	he := hr.Entity(0)
	sk.MustEntity(he).PointForceTo(sk, p)
	d.addPoint(p, he)
	return he
}

func (d *dxfImport) createLine(p0, p1 geom.Vector, style core.HStyle, constrainHV bool) core.HEntity {
	if p0.Equals(p1) {
		return core.NoEntity
	}
	sk := d.sk()
	hr := sk.AddRequestOfType(sketch.RequestLineSegment, false)
	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, p0)
	sk.MustEntity(hr.Entity(2)).PointForceTo(sk, p1)
	d.processPoint(hr.Entity(1))
	d.processPoint(hr.Entity(2))

	if constrainHV && d.lockedInWorkplane() {
		hasConstraint := false
		var cType core.ConstraintType
		if math.Abs(p0.X-p1.X) < core.LengthEps {
			hasConstraint = true
			cType = core.ConstraintVertical
		} else if math.Abs(p0.Y-p1.Y) < core.LengthEps {
			hasConstraint = true
			cType = core.ConstraintHorizontal
		}
		if hasConstraint {
			sk.Constrain(cType, core.NoEntity, core.NoEntity, hr.Entity(0), core.NoEntity, false, false)
		}
	}

	d.configureRequest(hr, style)
	return hr.Entity(0)
}

func (d *dxfImport) createWorkplane(p geom.Vector, q geom.Quaternion) core.HEntity {
	sk := d.sk()
	hr := d.sk().AddRequestOfType(sketch.RequestWorkplane, false)
	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, p)
	d.processPoint(hr.Entity(1))
	sk.MustEntity(hr.Entity(32)).NormalForceTo(sk, q)
	return hr.Entity(0)
}

func (d *dxfImport) findOrCreateWorkplane(p geom.Vector, q geom.Quaternion) core.HEntity {
	sk := d.sk()
	z := q.RotationN()
	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Type == sketch.RequestWorkplane && r.Group == d.sk().ActiveGroup() {
			wp := sk.MustEntity(r.H.Entity(1)).PointGetNum(sk)
			wz := sk.MustEntity(r.H.Entity(32)).NormalN(sk)

			if p.DistanceToPlane(wz, wp) < core.LengthEps && z.Equals(wz) {
				return r.H.Entity(0)
			}
		}
	}

	return d.createWorkplane(p, q)
}

func (d *dxfImport) activateWorkplane(he core.HEntity) {
	g := d.sk().Group(d.sk().ActiveGroup())
	g.ActiveWorkplane = he
}

func (d *dxfImport) createCircle(c geom.Vector, q geom.Quaternion, r float64, style core.HStyle) core.HEntity {
	sk := d.sk()
	hr := d.sk().AddRequestOfType(sketch.RequestCircle, false)
	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, c)
	d.processPoint(hr.Entity(1))
	sk.MustEntity(hr.Entity(32)).NormalForceTo(sk, q)
	sk.MustEntity(hr.Entity(64)).DistanceForceTo(sk, r)
	d.configureRequest(hr, style)
	return hr.Entity(0)
}

func (d *dxfImport) AddLayer(l *dxfread.Layer) {
	if _, ok := d.layers[l.Name]; !ok {
		copied := *l
		d.layers[l.Name] = &copied
	}
}

func (d *dxfImport) AddBlock(b *dxfread.Block) {
	blk, ok := d.blocks[b.Name]
	if !ok {
		blk = &importBlock{}
		d.blocks[b.Name] = blk
	}
	d.readBlock = blk
	d.readBlock.data = *b
}

func (d *dxfImport) EndBlock() {
	d.readBlock = nil
}

func (d *dxfImport) AddPoint(data *dxfread.Point) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	sk := d.sk()
	hr := d.sk().AddRequestOfType(sketch.RequestDatumPoint, false)
	sk.MustEntity(hr.Entity(0)).PointForceTo(sk, d.toVector(data.BasePoint))
	d.processPoint(hr.Entity(0))
}

func (d *dxfImport) AddLine(data *dxfread.Line) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	d.createLine(d.toVector(data.BasePoint), d.toVector(data.SecPoint), d.styleFor(&data.Entity, nil), true)
}

func (d *dxfImport) AddArc(data *dxfread.Arc) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	sk := d.sk()
	r := data.Radius
	sa := data.StaAngle
	ea := data.EndAngle
	c := d.toVector(data.BasePoint)
	nz := d.toVector(data.ExtPoint)
	q := normalFromExtPoint(nz)

	planar := q.RotationN().Equals(geom.Vector{Z: 1})
	onPlane := c.Z < core.LengthEps

	oldWorkplane := d.activeWorkplane()
	if !planar || !onPlane {
		d.activateWorkplane(d.findOrCreateWorkplane(c, q))
	}

	hr := sk.AddRequestOfType(sketch.RequestArcOfCircle, false)
	u, v := q.RotationU(), q.RotationV()
	rvs := c.Plus(u.ScaledBy(r * libm.Cos(sa))).Plus(v.ScaledBy(r * libm.Sin(sa)))
	rve := c.Plus(u.ScaledBy(r * libm.Cos(ea))).Plus(v.ScaledBy(r * libm.Sin(ea)))

	if data.ExtPoint.Z == -1.0 {
		c.X = -c.X
		rvs.X = -rvs.X
		rve.X = -rve.X
		rvs, rve = rve, rvs
	}

	d.blockTransformArc(&c, &rvs, &rve)

	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, c)
	sk.MustEntity(hr.Entity(2)).PointForceTo(sk, rvs)
	sk.MustEntity(hr.Entity(3)).PointForceTo(sk, rve)
	d.processPoint(hr.Entity(1))
	d.processPoint(hr.Entity(2))
	d.processPoint(hr.Entity(3))
	d.configureRequest(hr, d.styleFor(&data.Entity, nil))
	d.activateWorkplane(oldWorkplane)
}

func (d *dxfImport) AddCircle(data *dxfread.Circle) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	nz := d.toVector(data.ExtPoint)
	normal := normalFromExtPoint(nz)
	d.createCircle(d.toVector(data.BasePoint), normal, data.Radius, d.styleFor(&data.Entity, nil))
}

func (d *dxfImport) AddLWPolyline(data *dxfread.LWPolyline) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	n := len(data.Vertices)
	if n == 0 {
		// An empty polyline counts its segments down past zero, as the
		// goldens have it.
		return
	}
	vNum := n

	// Check for closed polyline.
	if data.Flags&1 != 1 {
		vNum--
	}

	// Correct coordinate system for the case where z=-1, as described
	// in http://paulbourke.net/dataformats/dxf/dxf10.html.
	needSwapX := data.ExtPoint.Z == -1.0

	for i := range vNum {
		c0 := *data.Vertices[i]
		c1 := *data.Vertices[(i+1)%n]

		if needSwapX {
			c0.X = -c0.X
			c1.X = -c1.X
			c0.Bulge = -c0.Bulge
		}

		p0 := geom.Vector{X: c0.X, Y: c0.Y}
		p1 := geom.Vector{X: c1.X, Y: c1.Y}
		hs := d.styleFor(&data.Entity, nil)

		if data.Vertices[i].Bulge == 0.0 {
			d.createLine(d.blockTransform(p0), d.blockTransform(p1), hs, true)
		} else {
			hr := d.createBulge(p0, p1, c0.Bulge)
			d.configureRequest(hr, hs)
		}
	}
}

func (d *dxfImport) AddPolyline(data *dxfread.Polyline) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	n := len(data.Vertices)
	if n == 0 {
		return
	}
	vNum := n

	// Check for closed polyline.
	if data.Flags&1 != 1 {
		vNum--
	}

	// Correct coordinate system for the case where z=-1, as described
	// in http://paulbourke.net/dataformats/dxf/dxf10.html.
	needSwapX := data.ExtPoint.Z == -1.0

	for i := range vNum {
		c0 := data.Vertices[i].BasePoint
		c1 := data.Vertices[(i+1)%n].BasePoint

		bulge := data.Vertices[i].Bulge
		if needSwapX {
			c0.X = -c0.X
			c1.X = -c1.X
			bulge = -bulge
		}

		p0 := coordVec(c0)
		p1 := coordVec(c1)
		hs := d.styleFor(&data.Entity, nil)

		if bulge == 0.0 {
			d.createLine(d.blockTransform(p0), d.blockTransform(p1), hs, true)
		} else {
			hr := d.createBulge(p0, p1, bulge)
			d.configureRequest(hr, hs)
		}
	}
}

func (d *dxfImport) AddSpline(data *dxfread.Spline) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if data.Degree != 3 {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}
	if len(data.Controls) < 4 {
		// Control points past the end read as zero, as the goldens have
		// it.
		return
	}

	sk := d.sk()
	hr := d.sk().AddRequestOfType(sketch.RequestCubic, false)
	for i := range 4 {
		sk.MustEntity(hr.Entity(i+1)).PointForceTo(sk, d.toVector(*data.Controls[i]))
		d.processPoint(hr.Entity(i + 1))
	}
	d.configureRequest(hr, d.styleFor(&data.Entity, nil))
}

func (d *dxfImport) AddInsert(data *dxfread.Insert) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	block, ok := d.blocks[data.Name]
	if !ok {
		// An inserted block that doesn't exist is skipped.
		return
	}

	// Push transform.
	x, y, t := d.blockX, d.blockY, d.blockT

	oldInsert := d.insertInsert
	d.insertInsert = data

	if data.ExtPoint.Z == -1.0 {
		d.invertXTransform()
	}
	d.multBlockTransform(data.BasePoint.X, data.BasePoint.Y, data.XScale, data.YScale, data.Angle)
	for _, e := range block.entities {
		d.addEntity(e)
	}

	d.insertInsert = oldInsert

	// Pop transform.
	d.blockX, d.blockY, d.blockT = x, y, t
}

func (d *dxfImport) AddMText(data *dxfread.MText) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	text := *data
	text.SecPoint = text.BasePoint
	d.addText(&text.Text)
}

func (d *dxfImport) AddText(data *dxfread.Text) {
	d.addText(data)
}

// addText adds TEXT, and the MTEXT passed on to it.
func (d *dxfImport) addText(data *dxfread.Text) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingBlockEntity(*data) {
		return
	}

	sk := d.sk()
	var c core.Constraint
	c.Group = sk.ActiveGroup()
	c.Workplane = d.activeWorkplane()
	c.Type = core.ConstraintComment
	if data.AlignH == dxfread.HLeft && data.AlignV == dxfread.VBaseLine {
		c.Disp.Offset = d.toVector(data.BasePoint)
	} else {
		c.Disp.Offset = d.toVector(data.SecPoint)
	}
	c.Comment = data.Text
	c.Disp.Style = d.styleFor(&data.Entity, data)
	sk.AddConstraintRemember(&c, false)
}

// AddDimension dispatches by kind.
func (d *dxfImport) AddDimension(data *dxfread.Dimension) {
	switch data.Kind {
	case dxfread.DimAligned:
		d.addDimAlign(data)
	case dxfread.DimLinear:
		d.addDimLinear(data)
	case dxfread.DimAngular:
		d.addDimAngular(data)
	case dxfread.DimRadial:
		d.addDimRadial(data)
	case dxfread.DimDiametric:
		d.addDimDiametric(data)
	case dxfread.DimAngular3P:
		d.addDimAngular3P(data)
	}
}

func (d *dxfImport) addDimAlign(data *dxfread.Dimension) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingDimension() {
		return
	}

	sk := d.sk()
	p0 := d.toVector(data.Def1)
	p1 := d.toVector(data.Def2)
	p2 := d.toVector(data.TextPoint)
	// The points are made right to left, the order the goldens have.
	ptB := d.createOrGetPoint(p1)
	ptA := d.createOrGetPoint(p0)
	hc := sk.Constrain(core.ConstraintPtPtDistance, ptA, ptB, core.NoEntity, core.NoEntity, false, false)

	c := sk.Constraint(hc)
	if data.HasMeasureValue {
		c.ValA = data.MeasureValue
	} else {
		c.ModifyToSatisfy(sk)
	}
	c.Disp.Offset = p2.Minus(p0.Plus(p1).ScaledBy(0.5))
}

func (d *dxfImport) addDimLinear(data *dxfread.Dimension) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingDimension() {
		return
	}

	sk := d.sk()
	p0 := coordVec(data.Def1)
	p1 := coordVec(data.Def2)
	p2 := coordVec(data.TextPoint)

	angle := data.Angle * math.Pi / 180.0
	dir := geom.Vector{X: libm.Cos(angle), Y: libm.Sin(angle)}
	p3 := p1.Minus(p1.ClosestPointOnLine(p2, dir)).Plus(p1)
	if p1.Minus(p3).Magnitude() < core.LengthEps {
		p3 = p0.Minus(p0.ClosestPointOnLine(p2, dir)).Plus(p1)
	}

	p4 := p0.ClosestPointOnLine(p1, p3.Minus(p1)).Plus(p0).ScaledBy(0.5)

	p0 = d.blockTransform(p0)
	p1 = d.blockTransform(p1)
	p2 = d.blockTransform(p2)
	p3 = d.blockTransform(p3)
	p4 = d.blockTransform(p4)

	// The points are made right to left, the order the goldens have.
	line := d.createLine(p1, p3, d.invisibleStyle(), false)
	pt := d.createOrGetPoint(p0)
	hc := sk.Constrain(core.ConstraintPtLineDistance, pt, core.NoEntity, line, core.NoEntity, false, false)

	c := sk.Constraint(hc)
	if data.HasMeasureValue {
		c.ValA = data.MeasureValue
	} else {
		c.ModifyToSatisfy(sk)
	}
	c.Disp.Offset = p2.Minus(p4)
}

func (d *dxfImport) addDimAngular(data *dxfread.Dimension) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingDimension() {
		return
	}
	d.dimAngular(data.FirstLine1(), data.FirstLine2(), data.SecondLine1(), data.SecondLine2(), data)
}

// dimAngular adds an angular dimension, with the lines' points given.
func (d *dxfImport) dimAngular(fl1, fl2, sl1, sl2 dxfread.Coord, data *dxfread.Dimension) {
	sk := d.sk()
	l0p0 := d.toVector(fl1)
	l0p1 := d.toVector(fl2)
	l1p0 := d.toVector(sl1)
	l1p1 := d.toVector(sl2)

	// The points are made right to left, the order the goldens have.
	lineB := d.createLine(l1p1, l1p0, d.invisibleStyle(), false)
	lineA := d.createLine(l0p0, l0p1, d.invisibleStyle(), false)
	hc := sk.Constrain(core.ConstraintAngle, core.NoEntity, core.NoEntity, lineA, lineB, false, false)

	c := sk.Constraint(hc)
	c.ModifyToSatisfy(sk)
	if data.HasMeasureValue {
		actual := data.MeasureValue / math.Pi * 180.0
		if math.Abs(180.0-actual-c.ValA) < math.Abs(actual-c.ValA) {
			c.Other = true
		}
		c.ValA = actual
	}

	pi, skew := geom.AtIntersectionOfLinesSkew(l0p0, l0p1, l1p0, l1p1)
	if !skew {
		c.Disp.Offset = d.toVector(data.TextPoint).Minus(pi)
	}
}

func (d *dxfImport) createDiametric(cp geom.Vector, q geom.Quaternion, r float64, tp geom.Vector,
	actual float64, asRadius bool) {
	sk := d.sk()
	he := d.createCircle(cp, q, r, d.invisibleStyle())

	hc := sk.Constrain(core.ConstraintDiameter, core.NoEntity, core.NoEntity, he, core.NoEntity,
		false, false)

	c := sk.Constraint(hc)
	if actual > 0.0 {
		if asRadius {
			c.ValA = actual * 2.0
		} else {
			c.ValA = actual
		}
	} else {
		c.ModifyToSatisfy(sk)
	}
	c.Disp.Offset = tp.Minus(cp)
	if asRadius {
		c.Other = true
	}
}

func (d *dxfImport) addDimRadial(data *dxfread.Dimension) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingDimension() {
		return
	}

	cp := d.toVector(data.CenterPoint())
	dp := d.toVector(data.DiameterPoint())
	tp := d.toVector(data.TextPoint)
	actual := -1.0
	if data.HasMeasureValue {
		actual = data.MeasureValue
	}

	nz := d.toVector(data.ExtPoint)
	q := normalFromExtPoint(nz)
	d.createDiametric(cp, q, cp.Minus(dp).Magnitude(), tp, actual, true)
}

func (d *dxfImport) addDimDiametric(data *dxfread.Dimension) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	// A diametric dimension in a block is kept as a radial one.
	radial := *data
	radial.Kind = dxfread.DimRadial
	if d.addPendingBlockEntity(radial) {
		return
	}

	dp1 := d.toVector(data.Diameter1Point())
	dp2 := d.toVector(data.Diameter2Point())

	cp := dp1.Plus(dp2).ScaledBy(0.5)
	tp := d.toVector(data.TextPoint)
	actual := -1.0
	if data.HasMeasureValue {
		actual = data.MeasureValue
	}

	nz := d.toVector(data.ExtPoint)
	q := normalFromExtPoint(nz)
	d.createDiametric(cp, q, cp.Minus(dp1).Magnitude(), tp, actual, false)
}

func (d *dxfImport) addDimAngular3P(data *dxfread.Dimension) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if d.addPendingDimension() {
		return
	}

	// It is made an angular dimension: its Space and the other fields
	// are the same, which addDimAngular checks again.
	d.dimAngular(data.VertexPoint(), data.FirstLine(), data.VertexPoint(), data.SecondLine(), data)
}

// dxfCheck3D finds whether a file has anything out of the xy plane.
type dxfCheck3D struct {
	is3d bool
}

func (d *dxfCheck3D) checkCoord(c dxfread.Coord) {
	if math.Abs(c.Z) > core.LengthEps {
		d.is3d = true
	}
}

func (d *dxfCheck3D) checkExt(c dxfread.Coord) {
	if math.Abs(c.X) > 1/64. || math.Abs(c.Y) > 1/64. {
		d.is3d = true
	}
}

func (d *dxfCheck3D) AddLayer(*dxfread.Layer) {}
func (d *dxfCheck3D) AddBlock(*dxfread.Block) {}
func (d *dxfCheck3D) EndBlock()               {}

func (d *dxfCheck3D) AddPoint(data *dxfread.Point) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	d.checkCoord(data.BasePoint)
}

func (d *dxfCheck3D) AddLine(data *dxfread.Line) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	d.checkCoord(data.BasePoint)
	d.checkCoord(data.SecPoint)
}

func (d *dxfCheck3D) AddArc(data *dxfread.Arc) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	d.checkCoord(data.BasePoint)
	d.checkExt(data.ExtPoint)
}

func (d *dxfCheck3D) AddCircle(data *dxfread.Circle) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	d.checkCoord(data.BasePoint)
	d.checkExt(data.ExtPoint)
}

// AddLWPolyline does nothing: light polylines are not checked.
func (d *dxfCheck3D) AddLWPolyline(*dxfread.LWPolyline) {}

func (d *dxfCheck3D) AddPolyline(data *dxfread.Polyline) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	for _, v := range data.Vertices {
		d.checkCoord(v.BasePoint)
	}
}

func (d *dxfCheck3D) AddSpline(data *dxfread.Spline) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	if data.Degree != 3 {
		return
	}
	for i := 0; i < 4 && i < len(data.Controls); i++ {
		d.checkCoord(*data.Controls[i])
	}
}

func (d *dxfCheck3D) AddInsert(data *dxfread.Insert) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	d.checkCoord(data.BasePoint)
}

func (d *dxfCheck3D) AddMText(data *dxfread.MText) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	text := *data
	text.SecPoint = text.BasePoint
	d.AddText(&text.Text)
}

func (d *dxfCheck3D) AddText(data *dxfread.Text) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	d.checkCoord(data.BasePoint)
	d.checkCoord(data.SecPoint)
}

func (d *dxfCheck3D) AddDimension(data *dxfread.Dimension) {
	if data.Space != dxfread.ModelSpace {
		return
	}
	switch data.Kind {
	case dxfread.DimAligned, dxfread.DimLinear:
		d.checkCoord(data.Def1)
		d.checkCoord(data.Def2)
		d.checkCoord(data.TextPoint)
	case dxfread.DimAngular:
		d.checkCoord(data.FirstLine1())
		d.checkCoord(data.FirstLine2())
		d.checkCoord(data.SecondLine1())
		d.checkCoord(data.SecondLine2())
		d.checkCoord(data.TextPoint)
	case dxfread.DimRadial:
		d.checkCoord(data.CenterPoint())
		d.checkCoord(data.DiameterPoint())
		d.checkCoord(data.TextPoint)
		d.checkExt(data.ExtPoint)
	case dxfread.DimDiametric:
		d.checkCoord(data.Diameter1Point())
		d.checkCoord(data.Diameter2Point())
		d.checkCoord(data.TextPoint)
		d.checkExt(data.ExtPoint)
	case dxfread.DimAngular3P:
		d.checkCoord(data.VertexPoint())
		d.checkCoord(data.FirstLine())
		d.checkCoord(data.VertexPoint())
		d.checkCoord(data.SecondLine())
		d.checkCoord(data.TextPoint)
	}
}

// Options is how a file is imported.
type Options struct {
	// AsConstruction makes every entity construction, as a 3d file's
	// are.
	AsConstruction bool
	// TextHeight is the height of text the file gives no height for:
	// the constraint style's.
	TextHeight float64
}

// Report is what an import says about the file.
type Report struct {
	// Unknown counts the entities of kinds the importer doesn't know.
	Unknown int
}

// ErrCorrupt is the error of a file that doesn't parse as DXF.
var ErrCorrupt = errors.New("corrupted DXF file")

// Has3D reports whether the file's entities leave the XY plane.
func Has3D(data []byte) bool {
	checker := &dxfCheck3D{}
	dxfread.Read(data, checker, true)
	return checker.is3d
}

// Import adds the file's entities to the sketch's active group, in
// its workplane or, if none is active, in 3d.
func Import(sk *sketch.Sketch, data []byte, o Options) (Report, error) {
	importer := newDxfImport(sk, o.TextHeight)
	importer.asConstruction = o.AsConstruction
	importer.clearBlockTransform()
	if !dxfread.Read(data, importer, true) {
		return Report{}, ErrCorrupt
	}
	return Report{Unknown: importer.unknownEntities}, nil
}
