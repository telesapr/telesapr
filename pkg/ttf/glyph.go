package ttf

// Vector is an FT_Vector, in 26.6 pixels once scaled.
type Vector struct{ X, Y int64 }

// BBox is an FT_BBox.
type BBox struct{ XMin, YMin, XMax, YMax int64 }

// Outline is an FT_Outline: its points, their tags (bit 0 set for a
// point on the curve), and the index of each contour's last point.
type Outline struct {
	Points   []Vector
	Tags     []byte
	Contours []int
}

// Glyph is what FT_Load_Glyph leaves in the glyph slot, as far as
// ttf.cpp reads it.
type Glyph struct {
	Outline      Outline
	HoriBearingX int64
	AdvanceX     int64
}

// RequestRealDimSize is FT_Request_Size with FT_SIZE_REQUEST_TYPE_REAL_DIM
// (FT_Request_Metrics; tt_size_reset only changes the hinted metrics,
// which unhinted loading doesn't use).
func (f *Face) RequestRealDimSize(width, height int64, horiResolution, vertResolution uint32) error {
	if width < 0 || height < 0 {
		return errInvalidArg
	}
	w := f.ascender - f.descender
	h := w
	if w < 0 {
		w = -w
	}
	if h < 0 {
		h = -h
	}
	scaledW, scaledH := width, height
	if horiResolution != 0 {
		scaledW = (width*int64(horiResolution) + 36) / 72
	}
	if vertResolution != 0 {
		scaledH = (height*int64(vertResolution) + 36) / 72
	}

	var xScale, yScale int64
	if height != 0 || width == 0 {
		if h == 0 {
			return errDivideByZero
		}
		yScale = divFix(scaledH, h)
	}
	if width != 0 {
		if w == 0 {
			return errDivideByZero
		}
		xScale = divFix(scaledW, w)
	} else {
		xScale = yScale
	}
	if height == 0 {
		yScale = xScale
	}

	scaledW = mulFix(f.unitsPerEM, xScale)
	scaledH = mulFix(f.unitsPerEM, yScale)
	scaledW = (scaledW + 32) >> 6
	scaledH = (scaledH + 32) >> 6
	if scaledW > 0xFFFF || scaledH > 0xFFFF {
		return errInvalidPpem
	}
	f.xScale, f.yScale = xScale, yScale
	f.xPpem, f.yPpem = scaledW, scaledH
	if f.xPpem == 0 || f.yPpem == 0 {
		// tt_size_reset
		return errInvalidPpem
	}
	return nil
}

// Kerning is FT_Get_Kerning with FT_KERNING_DEFAULT: the kern table's
// value for the pair, scaled and rounded to the pixel grid.
func (f *Face) Kerning(left, right uint32) Vector {
	var k Vector
	k.X = f.kerning(left, right)
	k.X = mulFix(k.X, f.xScale)
	k.Y = mulFix(k.Y, f.yScale)
	origX, origY := k.X, k.Y
	if f.xPpem < 25 {
		k.X = mulDiv(origX, f.xPpem, 25)
	}
	if f.yPpem < 25 {
		k.Y = mulDiv(origY, f.yPpem, 25)
	}
	k.X = pixRound(k.X)
	k.Y = pixRound(k.Y)
	return k
}

// Composite glyph flags.
const (
	argsAreWords          = 0x0001
	argsAreXYValues       = 0x0002
	weHaveAScale          = 0x0008
	moreComponents        = 0x0020
	weHaveAnXYScale       = 0x0040
	weHaveA2x2            = 0x0080
	useMyMetrics          = 0x0200
	scaledComponentOffset = 0x0800

	repeatFlag   = 0x08
	xShortVector = 0x02
	yShortVector = 0x04
	sameX        = 0x10
	sameY        = 0x20
	onCurvePoint = 0x01
)

type subGlyph struct {
	index          uint32
	flags          uint16
	arg1, arg2     int64
	xx, xy, yx, yy int64
}

// loader is a TT_LoaderRec for an unhinted, scaled load.
type loader struct {
	f              *Face
	xScale, yScale int64

	base Outline // the glyph loader's base outline

	cursor      []byte
	byteLen     int64
	nContours   int
	bbox        BBox
	leftBearing int64
	advance     int64
	linear      int64
	linearDef   bool
	pp1, pp2    Vector
	composites  []int64
}

// LoadGlyph is FT_Load_Glyph with FT_LOAD_NO_BITMAP | FT_LOAD_NO_HINTING,
// through tt_glyph_load and TT_Load_Glyph.
func (f *Face) LoadGlyph(gindex uint32) (*Glyph, error) {
	if int64(gindex) >= f.numGlyphs {
		return nil, errInvalidArg
	}
	if f.xPpem == 0 || f.yPpem == 0 {
		return nil, errInvalidPpem
	}
	l := &loader{f: f, xScale: f.xScale, yScale: f.yScale}
	if err := l.loadGlyph(gindex, 0); err != nil {
		return nil, err
	}
	g := &Glyph{Outline: l.base}
	if l.pp1.X != 0 {
		g.Outline.Translate(-l.pp1.X, 0)
	}
	// compute_glyph_metrics
	cbox := g.Outline.CBox()
	g.HoriBearingX = cbox.XMin
	g.AdvanceX = l.pp2.X - l.pp1.X

	if err := g.Outline.check(); err != nil {
		return nil, err
	}
	return g, nil
}

// loadGlyph is load_truetype_glyph.
func (l *loader) loadGlyph(gindex uint32, recurseCount int) error {
	f := l.f
	if recurseCount > 100 {
		return errInvalidComp
	}
	if int64(gindex) >= f.numGlyphs {
		return errInvalidGlyph
	}

	offset, byteLen := f.location(gindex)
	l.byteLen = byteLen
	var glyph []byte
	if byteLen > 0 {
		if f.glyfOffset == 0 {
			return errInvalidTable
		}
		start := f.glyfOffset + offset
		end := start + byteLen
		if start < 0 || end > int64(len(f.data)) {
			return errInvalidTable
		}
		glyph = f.data[start:end]
		// TT_Load_Glyph_Header
		if len(glyph) < 10 {
			return errInvalidOutl
		}
		l.nContours = int(int16(peek16(glyph, 0)))
		l.bbox.XMin = int64(int16(peek16(glyph, 2)))
		l.bbox.YMin = int64(int16(peek16(glyph, 4)))
		l.bbox.XMax = int64(int16(peek16(glyph, 6)))
		l.bbox.YMax = int64(int16(peek16(glyph, 8)))
	}

	if byteLen == 0 || l.nContours == 0 {
		l.bbox = BBox{}
	}

	// tt_get_metrics (the vertical metrics aren't used)
	lsb, aw := f.hMetrics(gindex)
	l.leftBearing = int64(lsb)
	l.advance = int64(aw)
	if !l.linearDef {
		l.linearDef = true
		l.linear = int64(aw)
	}

	// tt_loader_set_pp
	l.pp1 = Vector{l.bbox.XMin - l.leftBearing, 0}
	l.pp2 = Vector{l.pp1.X + l.advance, 0}

	if byteLen == 0 || l.nContours == 0 {
		l.pp1.X = mulFix(l.pp1.X, l.xScale)
		l.pp2.X = mulFix(l.pp2.X, l.xScale)
		return nil
	}

	l.cursor = glyph[10:]
	if l.nContours > 0 {
		cur, err := l.loadSimpleGlyph()
		if err != nil {
			return err
		}
		// TT_Process_Simple_Glyph: scale the points and phantom points.
		for i := range cur.Points {
			cur.Points[i].X = mulFix(cur.Points[i].X, l.xScale)
			cur.Points[i].Y = mulFix(cur.Points[i].Y, l.yScale)
		}
		l.pp1.X = mulFix(l.pp1.X, l.xScale)
		l.pp2.X = mulFix(l.pp2.X, l.xScale)
		l.add(cur)
		return nil
	}

	// A composite glyph.
	l.nContours = -1
	for i := recurseCount; i < len(l.composites); i++ {
		l.composites[i] = -1
	}
	for _, c := range l.composites {
		if c == int64(gindex) {
			return errInvalidComp
		}
	}
	if recurseCount < len(l.composites) {
		l.composites[recurseCount] = int64(gindex)
	} else {
		l.composites = append(l.composites, int64(gindex))
	}

	startPoint := len(l.base.Points)
	subglyphs, err := l.loadCompositeGlyph()
	if err != nil {
		return err
	}

	l.pp1.X = mulFix(l.pp1.X, l.xScale)
	l.pp2.X = mulFix(l.pp2.X, l.xScale)

	for n := range subglyphs {
		sg := &subglyphs[n]
		pp1, pp2 := l.pp1, l.pp2
		linear := l.linear
		numBasePoints := len(l.base.Points)

		if err := l.loadGlyph(sg.index, recurseCount+1); err != nil {
			return err
		}
		if sg.flags&useMyMetrics == 0 {
			l.pp1, l.pp2 = pp1, pp2
			l.linear = linear
		}
		if len(l.base.Points) == numBasePoints {
			continue
		}
		if err := l.processCompositeComponent(sg, startPoint, numBasePoints); err != nil {
			return err
		}
	}
	return nil
}

// add is FT_GlyphLoader_Add: the current outline joins the base one.
func (l *loader) add(cur Outline) {
	n := len(l.base.Points)
	l.base.Points = append(l.base.Points, cur.Points...)
	l.base.Tags = append(l.base.Tags, cur.Tags...)
	for _, c := range cur.Contours {
		l.base.Contours = append(l.base.Contours, c+n)
	}
}

// loadSimpleGlyph is TT_Load_Simple_Glyph.
func (l *loader) loadSimpleGlyph() (Outline, error) {
	var o Outline
	p := l.cursor
	pos := 0
	nContours := l.nContours
	if nContours >= 0xFFF || pos+2*nContours+2 > len(p) {
		return o, errInvalidOutl
	}
	last := -1
	for i := 0; i < nContours; i++ {
		c := int(peek16(p, pos))
		pos += 2
		if c <= last {
			return o, errInvalidOutl
		}
		o.Contours = append(o.Contours, c)
		last = c
	}
	nPoints := last + 1

	nIns := int(peek16(p, pos))
	pos += 2
	if pos+nIns > len(p) {
		return o, errTooManyHints
	}
	pos += nIns

	o.Tags = make([]byte, nPoints)
	for i := 0; i < nPoints; {
		if pos+1 > len(p) {
			return o, errInvalidOutl
		}
		c := p[pos]
		pos++
		o.Tags[i] = c
		i++
		if c&repeatFlag != 0 {
			if pos+1 > len(p) {
				return o, errInvalidOutl
			}
			count := int(p[pos])
			pos++
			if i+count > nPoints {
				return o, errInvalidOutl
			}
			for ; count > 0; count-- {
				o.Tags[i] = c
				i++
			}
		}
	}

	o.Points = make([]Vector, nPoints)
	var x int64
	for i := 0; i < nPoints; i++ {
		var delta int64
		fl := o.Tags[i]
		if fl&xShortVector != 0 {
			if pos+1 > len(p) {
				return o, errInvalidOutl
			}
			delta = int64(p[pos])
			pos++
			if fl&sameX == 0 {
				delta = -delta
			}
		} else if fl&sameX == 0 {
			if pos+2 > len(p) {
				return o, errInvalidOutl
			}
			delta = int64(int16(peek16(p, pos)))
			pos += 2
		}
		x += delta
		o.Points[i].X = x
	}

	var y int64
	for i := 0; i < nPoints; i++ {
		var delta int64
		fl := o.Tags[i]
		if fl&yShortVector != 0 {
			if pos+1 > len(p) {
				return o, errInvalidOutl
			}
			delta = int64(p[pos])
			pos++
			if fl&sameY == 0 {
				delta = -delta
			}
		} else if fl&sameY == 0 {
			if pos+2 > len(p) {
				return o, errInvalidOutl
			}
			delta = int64(int16(peek16(p, pos)))
			pos += 2
		}
		y += delta
		o.Points[i].Y = y
		o.Tags[i] = fl & onCurvePoint
	}
	return o, nil
}

// loadCompositeGlyph is TT_Load_Composite_Glyph.
func (l *loader) loadCompositeGlyph() ([]subGlyph, error) {
	p := l.cursor
	pos := 0
	var subs []subGlyph
	for {
		if pos+4 > len(p) {
			return nil, errInvalidComp
		}
		var sg subGlyph
		sg.flags = peek16(p, pos)
		sg.index = uint32(peek16(p, pos+2))
		pos += 4
		if int64(sg.index) >= l.f.numGlyphs {
			return nil, errInvalidComp
		}
		count := 2
		if sg.flags&argsAreWords != 0 {
			count += 2
		}
		if sg.flags&weHaveAScale != 0 {
			count += 2
		} else if sg.flags&weHaveAnXYScale != 0 {
			count += 4
		} else if sg.flags&weHaveA2x2 != 0 {
			count += 8
		}
		if pos+count > len(p) {
			return nil, errInvalidComp
		}

		if sg.flags&argsAreXYValues != 0 {
			if sg.flags&argsAreWords != 0 {
				sg.arg1 = int64(int16(peek16(p, pos)))
				sg.arg2 = int64(int16(peek16(p, pos+2)))
				pos += 4
			} else {
				sg.arg1 = int64(int8(p[pos]))
				sg.arg2 = int64(int8(p[pos+1]))
				pos += 2
			}
		} else {
			if sg.flags&argsAreWords != 0 {
				sg.arg1 = int64(peek16(p, pos))
				sg.arg2 = int64(peek16(p, pos+2))
				pos += 4
			} else {
				sg.arg1 = int64(p[pos])
				sg.arg2 = int64(p[pos+1])
				pos += 2
			}
		}

		sg.xx, sg.yy = 0x10000, 0x10000
		next := func() int64 {
			v := int64(int16(peek16(p, pos))) * 4
			pos += 2
			return v
		}
		if sg.flags&weHaveAScale != 0 {
			sg.xx = next()
			sg.yy = sg.xx
		} else if sg.flags&weHaveAnXYScale != 0 {
			sg.xx = next()
			sg.yy = next()
		} else if sg.flags&weHaveA2x2 != 0 {
			sg.xx = next()
			sg.yx = next()
			sg.xy = next()
			sg.yy = next()
		}
		subs = append(subs, sg)
		if sg.flags&moreComponents == 0 {
			break
		}
	}
	return subs, nil
}

// processCompositeComponent is TT_Process_Composite_Component.
func (l *loader) processCompositeComponent(sg *subGlyph, startPoint, numBasePoints int) error {
	pts := l.base.Points[numBasePoints:]
	haveScale := sg.flags&(weHaveAScale|weHaveAnXYScale|weHaveA2x2) != 0
	if haveScale {
		for i := range pts {
			pts[i] = transform(pts[i], sg.xx, sg.xy, sg.yx, sg.yy)
		}
	}

	var x, y int64
	if sg.flags&argsAreXYValues == 0 {
		numPoints := len(l.base.Points)
		k := int(sg.arg1) + startPoint
		m := int(sg.arg2) + numBasePoints
		if k >= numBasePoints || m >= numPoints {
			return errInvalidComp
		}
		p1 := l.base.Points[k]
		p2 := l.base.Points[m]
		x = p1.X - p2.X
		y = p1.Y - p2.Y
	} else {
		x, y = sg.arg1, sg.arg2
		if x == 0 && y == 0 {
			return nil
		}
		if haveScale && sg.flags&scaledComponentOffset != 0 {
			macXScale := hypot(sg.xx, sg.xy)
			macYScale := hypot(sg.yy, sg.yx)
			x = mulFix(x, macXScale)
			y = mulFix(y, macYScale)
		}
		x = mulFix(x, l.xScale)
		y = mulFix(y, l.yScale)
	}
	if x != 0 || y != 0 {
		for i := range pts {
			pts[i].X += x
			pts[i].Y += y
		}
	}
	return nil
}

// transform is FT_Vector_Transform.
func transform(v Vector, xx, xy, yx, yy int64) Vector {
	return Vector{
		mulFix(v.X, xx) + mulFix(v.Y, xy),
		mulFix(v.X, yx) + mulFix(v.Y, yy),
	}
}

// Translate is FT_Outline_Translate.
func (o *Outline) Translate(dx, dy int64) {
	for i := range o.Points {
		o.Points[i].X += dx
		o.Points[i].Y += dy
	}
}

// CBox is FT_Outline_Get_CBox.
func (o *Outline) CBox() BBox {
	if len(o.Points) == 0 {
		return BBox{}
	}
	b := BBox{o.Points[0].X, o.Points[0].Y, o.Points[0].X, o.Points[0].Y}
	for _, v := range o.Points[1:] {
		if v.X < b.XMin {
			b.XMin = v.X
		}
		if v.X > b.XMax {
			b.XMax = v.X
		}
		if v.Y < b.YMin {
			b.YMin = v.Y
		}
		if v.Y > b.YMax {
			b.YMax = v.Y
		}
	}
	return b
}

// check is FT_Outline_Check.
func (o *Outline) check() error {
	nPoints, nContours := len(o.Points), len(o.Contours)
	if nPoints == 0 && nContours == 0 {
		return nil
	}
	if nPoints == 0 || nContours == 0 {
		return errInvalidOutl
	}
	end0 := -1
	for _, end := range o.Contours {
		if end <= end0 || end >= nPoints {
			return errInvalidOutl
		}
		end0 = end
	}
	if end0 != nPoints-1 {
		return errInvalidOutl
	}
	return nil
}

// OutlineFuncs are an FT_Outline_Funcs' callbacks (with no shift or
// delta).
type OutlineFuncs struct {
	MoveTo  func(to Vector)
	LineTo  func(to Vector)
	ConicTo func(control, to Vector)
	CubicTo func(control1, control2, to Vector)
}

const (
	curveTagOn    = 1
	curveTagConic = 0
	curveTagCubic = 2
)

// Decompose is FT_Outline_Decompose.
func (o *Outline) Decompose(fn OutlineFuncs) error {
	last := -1
	for n := 0; n < len(o.Contours); n++ {
		first := last + 1
		last = o.Contours[n]
		if last < first {
			return errInvalidOutl
		}
		limit := last

		vStart := o.Points[first]
		vLast := o.Points[last]
		vControl := vStart

		point := first
		tag := o.Tags[first] & 3
		if tag == curveTagCubic {
			return errInvalidOutl
		}
		if tag == curveTagConic {
			if o.Tags[last]&3 == curveTagOn {
				vStart = vLast
				limit--
			} else {
				vStart.X = (vStart.X + vLast.X) / 2
				vStart.Y = (vStart.Y + vLast.Y) / 2
			}
			point--
		}

		fn.MoveTo(vStart)

		closed := false
	contour:
		for point < limit {
			point++
			tag = o.Tags[point] & 3
			switch tag {
			case curveTagOn:
				fn.LineTo(o.Points[point])
				continue

			case curveTagConic:
				vControl = o.Points[point]
				for {
					if point < limit {
						point++
						tag = o.Tags[point] & 3
						vec := o.Points[point]
						if tag == curveTagOn {
							fn.ConicTo(vControl, vec)
							continue contour
						}
						if tag != curveTagConic {
							return errInvalidOutl
						}
						vMiddle := Vector{(vControl.X + vec.X) / 2, (vControl.Y + vec.Y) / 2}
						fn.ConicTo(vControl, vMiddle)
						vControl = vec
						continue
					}
					fn.ConicTo(vControl, vStart)
					closed = true
					break contour
				}

			default:
				if point+1 > limit || o.Tags[point+1]&3 != curveTagCubic {
					return errInvalidOutl
				}
				point += 2
				vec1 := o.Points[point-2]
				vec2 := o.Points[point-1]
				if point <= limit {
					fn.CubicTo(vec1, vec2, o.Points[point])
					continue
				}
				fn.CubicTo(vec1, vec2, vStart)
				closed = true
				break contour
			}
		}
		if !closed {
			fn.LineTo(vStart)
		}
	}
	return nil
}
