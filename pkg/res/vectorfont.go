package res

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cxxutf8"
	"github.com/telesapr/telesapr/pkg/libm"
)

// VectorContour is a polyline of a vector glyph.
type VectorContour struct {
	Points []geom.Point2d
}

// VectorGlyph is a vector glyph and its metrics.
type VectorGlyph struct {
	Contours        []VectorContour
	LeftSideBearing float64
	BoundingWidth   float64
	AdvanceWidth    float64
}

// VectorFont is the stroke font the graphics window labels with,
// LibreCAD's LFF format.
type VectorFont struct {
	lffData          string
	glyphs           map[rune]*VectorGlyph
	RightSideBearing float64
	CapHeight        float64
	Ascender         float64
	Descender        float64
}

// TraceCamera is what VectorFont.TraceAligned needs of a camera
// (render.Camera).
type TraceCamera interface {
	IsGridFit() bool
	ProjVectors() (right, up geom.Vector)
	AlignToPixelGrid(v geom.Vector) geom.Vector
}

const arcPoints = 8

func makePwlArc(c *VectorContour, isReversed bool, cp geom.Point2d, radius, a1, a2 float64) {
	if radius < geom.LengthEps {
		return
	}
	aSign := 1.0
	if isReversed {
		if a1 <= a2+geom.LengthEps {
			a1 += 2.0 * math.Pi
		}
		aSign = -1.0
	} else if a2 <= a1+geom.LengthEps {
		a2 += 2.0 * math.Pi
	}
	aStep := aSign * math.Abs(a2-a1) / float64(arcPoints)
	for i := 0; i <= arcPoints; i++ {
		c.Points = append(c.Points, cp.Plus(geom.Point2dFromPolar(radius, a1+aStep*float64(i))))
	}
}

func makePwlBulge(c *VectorContour, v geom.Point2d, bulge float64) {
	reversed := bulge < 0.0
	alpha := libm.Atan(bulge) * 4.0
	point := c.Points[len(c.Points)-1]

	middle := point.Plus(v).ScaledBy(0.5)
	dist := point.DistanceTo(v) / 2.0
	angle := point.AngleTo(v)

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
		h = -h
	}

	center := geom.Point2dFromPolar(h, angle).Plus(middle)
	a1 := center.AngleTo(point)
	a2 := center.AngleTo(v)
	makePwlArc(c, reversed, center, radius, a1, a2)
}

func glyphBBox(g *VectorGlyph) (minx, maxx, miny, maxy float64) {
	if len(g.Contours) > 0 {
		start := g.Contours[0].Points[0]
		minx, maxx = start.U, start.U
		miny, maxy = start.V, start.V
		for _, c := range g.Contours {
			for _, p := range c.Points {
				maxx = geom.Max(maxx, p.U)
				minx = geom.Min(minx, p.U)
				maxy = geom.Max(maxy, p.V)
				miny = geom.Min(miny, p.V)
			}
		}
	}
	return
}

var lffHeaderRe = regexp.MustCompile(`^#\s*(\w+)\s*:\s*(.+?)\n`)

// VectorFontFrom parses an LFF font's header; glyphs are parsed as they
// are asked for.
func VectorFontFrom(lffData string) *VectorFont {
	f := &VectorFont{lffData: lffData, glyphs: make(map[rune]*VectorGlyph)}
	pos := 0
	for {
		m := lffHeaderRe.FindStringSubmatchIndex(lffData[pos:])
		if m == nil {
			break
		}
		name := strings.ToLower(lffData[pos+m[2] : pos+m[3]])
		value := lffData[pos+m[4] : pos+m[5]]
		pos += m[1]
		switch name {
		case "letterspacing":
			f.RightSideBearing = stod(value)
		case "wordspacing":
			f.glyphs[' '] = &VectorGlyph{AdvanceWidth: stod(value)}
		}
	}

	_, _, _, f.CapHeight = glyphBBox(f.Glyph('A'))
	_, _, _, f.Ascender = glyphBBox(f.Glyph('h'))
	_, _, f.Descender, _ = glyphBBox(f.Glyph('p'))
	return f
}

// stod is std::stod: the number at the start of s.
func stod(s string) float64 {
	n, v := parseDecimalPrefix(s)
	if n == 0 {
		panic("res: stod: no conversion of " + strconv.Quote(s))
	}
	return v
}

var (
	builtinVectorFont     *VectorFont
	builtinVectorFontOnce sync.Once
)

// BuiltinVectorFont is the bundled LibreCAD unicode font.
func BuiltinVectorFont() *VectorFont {
	builtinVectorFontOnce.Do(func() {
		builtinVectorFont = VectorFontFrom(LoadStringFromGzip("fonts/unicode.lff.gz"))
	})
	return builtinVectorFont
}

// IsEmpty reports whether the font has no data.
func (f *VectorFont) IsEmpty() bool { return f == nil || f.lffData == "" }

// Glyph returns the glyph, parsing it the first time; a missing one is
// U+FFFD.
func (f *VectorFont) Glyph(codepoint rune) *VectorGlyph {
	if g, ok := f.glyphs[codepoint]; ok {
		return g
	}
	data := f.lffData
	firstGlyph := strings.IndexByte(data, '[')
	if firstGlyph < 0 {
		panic("res: vector font contains no glyphs")
	}

	// Find the serialized representation in the (sorted) lff file.
	first, last := firstGlyph, len(data)
	for first <= last {
		mid := first + (last-first)/2
		for mid > first {
			if mid < len(data) && data[mid] == '[' && data[mid-1] == '\n' {
				break
			}
			mid--
		}

		r := asciiReader{s: data, pos: mid}
		if r.atEnd() {
			break
		}
		r.expectChar('[')
		found := rune(r.read16HexBits())
		r.expectChar(']')
		r.skipUntilEol()

		if found > codepoint {
			last = mid - 1
			continue
		}
		if found < codepoint {
			first = mid + 1
			for first != len(data) {
				if data[first] == '[' && data[first-1] == '\n' {
					break
				}
				first++
			}
			continue
		}

		g := &VectorGlyph{}
		for !r.atEnd() {
			if r.tryChar('\n') {
				continue
			}
			if r.tryChar('[') {
				// End of glyph; drop a useless empty contour, if any.
				if n := len(g.Contours); n > 0 && len(g.Contours[n-1].Points) == 0 {
					g.Contours = g.Contours[:n-1]
				}
				break
			}
			if r.tryChar('C') {
				// Another character is referenced in this one.
				base := f.Glyph(rune(r.read16HexBits()))
				for _, c := range base.Contours {
					g.Contours = append(g.Contours, VectorContour{Points: append([]geom.Point2d(nil), c.Points...)})
				}
				continue
			}
			var c VectorContour
			for {
				var p geom.Point2d
				p.U = r.readFloatDecimal()
				r.expectChar(',')
				p.V = r.readFloatDecimal()
				if r.tryChar(',') {
					// Point with a bulge.
					r.expectChar('A')
					makePwlBulge(&c, p, r.readFloatDecimal())
				} else {
					c.Points = append(c.Points, p)
				}
				if !r.tryChar(';') {
					break
				}
			}
			r.expectChar('\n')
			g.Contours = append(g.Contours, c)
		}

		g.LeftSideBearing, g.BoundingWidth, _, _ = glyphBBox(g)
		g.AdvanceWidth = g.LeftSideBearing + g.BoundingWidth + f.RightSideBearing
		f.glyphs[codepoint] = g
		return g
	}

	// Glyph doesn't exist; return replacement glyph instead.
	if codepoint == 0xfffd {
		panic("res: cannot parse replacement glyph")
	}
	return f.Glyph(0xfffd)
}

// GetCapHeight is the cap height text of that cap height has.
func (f *VectorFont) GetCapHeight(forCapHeight float64) float64 { return forCapHeight }

// Height is the full height of text with that cap height.
func (f *VectorFont) Height(forCapHeight float64) float64 {
	return (f.Ascender - f.Descender) * (forCapHeight / f.CapHeight)
}

// Width is the width of str at that cap height.
func (f *VectorFont) Width(forCapHeight float64, str string) float64 {
	width := 0.0
	for _, c := range cxxutf8.Decode(str) {
		width += f.Glyph(rune(c)).AdvanceWidth
	}
	width -= f.RightSideBearing
	return width * (forCapHeight / f.CapHeight)
}

// Extents is the width and height of str.
func (f *VectorFont) Extents(forCapHeight float64, str string) geom.Vector {
	return geom.Vector{X: f.Width(forCapHeight, str), Y: f.Height(forCapHeight)}
}

// Trace calls traceEdge for every stroke of str, laid out from o along
// u, with v up.
func (f *VectorFont) Trace(forCapHeight float64, o, u, v geom.Vector, str string, traceEdge func(a, b geom.Vector)) {
	scale := forCapHeight / f.CapHeight
	u = u.ScaledBy(scale)
	v = v.ScaledBy(scale)

	for _, cp := range cxxutf8.Decode(str) {
		g := f.Glyph(rune(cp))
		for _, c := range g.Contours {
			var prevp geom.Vector
			penUp := true
			for _, pt := range c.Points {
				p := o.Plus(u.ScaledBy(pt.U)).Plus(v.ScaledBy(pt.V))
				if !penUp {
					traceEdge(prevp, p)
				}
				prevp = p
				penUp = false
			}
		}
		o = o.Plus(u.ScaledBy(g.AdvanceWidth))
	}
}

// TraceAligned is Trace with each glyph grid-fitted to the camera's
// pixels when the text is parallel to the screen.
func (f *VectorFont) TraceAligned(forCapHeight float64, o, u, v geom.Vector, str string, traceEdge func(a, b geom.Vector), camera TraceCamera) {
	right, up := camera.ProjVectors()
	// Perform grid-fitting only when the text is parallel to the view plane.
	if camera.IsGridFit() && !(u.WithMagnitude(1).Equals(right) && v.WithMagnitude(1).Equals(up)) {
		f.Trace(forCapHeight, o, u, v, str, traceEdge)
		return
	}

	scale := forCapHeight / f.CapHeight
	u = u.ScaledBy(scale)
	v = v.ScaledBy(scale)

	for _, cp := range cxxutf8.Decode(str) {
		g := f.Glyph(rune(cp))
		actualWidth := geom.Max(1.0, g.BoundingWidth)

		// Align (o+lsb), (o+lsb+u) and (o+lsb+v) to pixel grid.
		ao := o.Plus(u.ScaledBy(g.LeftSideBearing))
		au := ao.Plus(u.ScaledBy(actualWidth))
		av := ao.Plus(v.ScaledBy(f.CapHeight))

		ao = camera.AlignToPixelGrid(ao)
		au = camera.AlignToPixelGrid(au)
		av = camera.AlignToPixelGrid(av)

		au = au.Minus(ao).ScaledBy(1.0 / actualWidth)
		av = av.Minus(ao).ScaledBy(1.0 / f.CapHeight)

		for _, c := range g.Contours {
			var prevp geom.Vector
			penUp := true
			for _, pt := range c.Points {
				p := ao.Plus(au.ScaledBy(pt.U - g.LeftSideBearing)).Plus(av.ScaledBy(pt.V))
				if !penUp {
					traceEdge(prevp, p)
				}
				prevp = p
				penUp = false
			}
		}
		o = o.Plus(u.ScaledBy(g.AdvanceWidth))
	}
}
