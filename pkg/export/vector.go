package export

// The file format-specific stuff for the 2d vector output formats but
// DXF.

import (
	"math"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/srf"
)

var veryPositiveVector = geom.Vector{X: geom.VeryPositive, Y: geom.VeryPositive, Z: geom.VeryPositive}

// makeStipplePattern is a dash pattern in the format's units.
func makeStipplePattern(pattern core.StipplePattern, scale float64, delimiter byte,
	inkscapeWorkaround bool) string {
	scale /= 2.0

	// Inkscape ignores all elements that are exactly zero instead of
	// drawing them as dots.
	zero := 0.0
	if inkscapeWorkaround {
		zero = 1e-6
	}

	var result string
	switch pattern {
	case core.StippleContinuous:
		return ""

	case core.StippleShortDash:
		result = cfmt.Sprintf("%.3f_%.3f", scale, scale*2.0)
	case core.StippleDash:
		result = cfmt.Sprintf("%.3f_%.3f", scale, scale)
	case core.StippleDashDot:
		result = cfmt.Sprintf("%.3f_%.3f_%.6f_%.3f", scale, scale*0.5, zero, scale*0.5)
	case core.StippleDashDotDot:
		result = cfmt.Sprintf("%.3f_%.3f_%.6f_%.3f_%.6f_%.3f",
			scale, scale*0.5, zero, scale*0.5, scale*0.5, zero)
	case core.StippleDot:
		result = cfmt.Sprintf("%.6f_%.3f", zero, scale*0.5)
	case core.StippleLongDash:
		result = cfmt.Sprintf("%.3f_%.3f", scale*2.0, scale*0.5)

	case core.StippleFreehand, core.StippleZigzag:
		// Freehand and zigzag export not implemented; exporting as
		// continuous line.
	}
	return strings.ReplaceAll(result, "_", string(delimiter))
}

// ---------------------------------------------------------------------
// EPS
// ---------------------------------------------------------------------

type epsFileWriter struct {
	vectorWriter
	prevPt geom.Vector
}

func (w *epsFileWriter) HasCanvasSize() bool { return true }
func (w *epsFileWriter) CanOutputMesh() bool { return true }

// cppInt converts f to a 32-bit int as x86 does: what doesn't fit, such
// as the extent of an empty sketch, is INT_MIN.
func cppInt(f float64) int {
	if !(f >= math.MinInt32 && f <= math.MaxInt32) {
		return math.MinInt32
	}
	return int(f)
}

func (w *epsFileWriter) StartFile() {
	w.printf(
		"%%!PS-Adobe-2.0\r\n"+
			"%%%%Creator: TeleSAPR\r\n"+
			"%%%%Title: title\r\n"+
			"%%%%Pages: 0\r\n"+
			"%%%%PageOrder: Ascend\r\n"+
			"%%%%BoundingBox: 0 0 %d %d\r\n"+
			"%%%%HiResBoundingBox: 0 0 %.3f %.3f\r\n"+
			"%%%%EndComments\r\n"+
			"\r\n"+
			"gsave\r\n"+
			"\r\n",
		cppInt(math.Ceil(MmToPts(w.ptMax.X-w.ptMin.X))),
		cppInt(math.Ceil(MmToPts(w.ptMax.Y-w.ptMin.Y))),
		MmToPts(w.ptMax.X-w.ptMin.X),
		MmToPts(w.ptMax.Y-w.ptMin.Y))
}

func (w *epsFileWriter) Background(color core.RGBA) {
	width := w.ptMax.X - w.ptMin.X
	height := w.ptMax.Y - w.ptMin.Y

	w.printf(
		"%.3f %.3f %.3f setrgbcolor\r\n"+
			"newpath\r\n"+
			"    %.3f %.3f moveto\r\n"+
			"    %.3f %.3f lineto\r\n"+
			"    %.3f %.3f lineto\r\n"+
			"    %.3f %.3f lineto\r\n"+
			"    closepath\r\n"+
			"gsave fill grestore\r\n",
		color.RedF(), color.GreenF(), color.BlueF(),
		MmToPts(0), MmToPts(0),
		MmToPts(width), MmToPts(0),
		MmToPts(width), MmToPts(height),
		MmToPts(0), MmToPts(height))

	// same issue with cracks, stroke it to avoid them
	sw := max(width, height) / 1000
	w.printf(
		"1 setlinejoin\r\n"+
			"1 setlinecap\r\n"+
			"%.3f setlinewidth\r\n"+
			"gsave stroke grestore\r\n",
		MmToPts(sw))
}

func (w *epsFileWriter) StartPath(strokeRgb core.RGBA, lineWidth float64, filled bool,
	fillRgb core.RGBA, hs core.HStyle) {
	w.printf("newpath\r\n")
	w.prevPt = veryPositiveVector
}

func (w *epsFileWriter) FinishPath(strokeRgb core.RGBA, lineWidth float64, filled bool,
	fillRgb core.RGBA, hs core.HStyle, closed bool) {
	pattern := w.sk.StylePatternType(hs)
	stippleScale := MmToPts(w.styleStippleScaleMm(hs))

	w.printf("    %.3f setlinewidth\r\n"+
		"    %.3f %.3f %.3f setrgbcolor\r\n"+
		"    1 setlinejoin\r\n"+ // rounded
		"    1 setlinecap\r\n"+ // rounded
		"    [%s] 0 setdash\r\n"+
		"    gsave stroke grestore\r\n",
		MmToPts(lineWidth),
		strokeRgb.RedF(), strokeRgb.GreenF(), strokeRgb.BlueF(),
		makeStipplePattern(pattern, stippleScale, ' ', false))
	if filled {
		w.printf("    %.3f %.3f %.3f setrgbcolor\r\n"+
			"    gsave fill grestore\r\n",
			fillRgb.RedF(), fillRgb.GreenF(), fillRgb.BlueF())
	}
}

func (w *epsFileWriter) maybeMoveTo(st, fi geom.Vector) {
	if !w.prevPt.Equals(st) {
		w.printf("    %.3f %.3f moveto\r\n", MmToPts(st.X-w.ptMin.X), MmToPts(st.Y-w.ptMin.Y))
	}
	w.prevPt = fi
}

func (w *epsFileWriter) Triangle(tr *srf.STriangle) {
	c := tr.Meta.RGBA
	w.printf(
		"%.3f %.3f %.3f setrgbcolor\r\n"+
			"newpath\r\n"+
			"    %.3f %.3f moveto\r\n"+
			"    %.3f %.3f lineto\r\n"+
			"    %.3f %.3f lineto\r\n"+
			"    closepath\r\n"+
			"gsave fill grestore\r\n",
		c.RedF(), c.GreenF(), c.BlueF(),
		MmToPts(tr.A.X-w.ptMin.X), MmToPts(tr.A.Y-w.ptMin.Y),
		MmToPts(tr.B.X-w.ptMin.X), MmToPts(tr.B.Y-w.ptMin.Y),
		MmToPts(tr.C.X-w.ptMin.X), MmToPts(tr.C.Y-w.ptMin.Y))

	// same issue with cracks, stroke it to avoid them
	sw := max(w.ptMax.X-w.ptMin.X, w.ptMax.Y-w.ptMin.Y) / 1000
	w.printf(
		"1 setlinejoin\r\n"+
			"1 setlinecap\r\n"+
			"%.3f setlinewidth\r\n"+
			"gsave stroke grestore\r\n",
		MmToPts(sw))
}

func (w *epsFileWriter) Bezier(sb *srf.SBezier) {
	var c geom.Vector
	n := geom.Vector{Z: 1}
	var r float64
	switch {
	case sb.Degree == 1:
		w.maybeMoveTo(sb.Ctrl[0], sb.Ctrl[1])
		w.printf("    %.3f %.3f lineto\r\n",
			MmToPts(sb.Ctrl[1].X-w.ptMin.X), MmToPts(sb.Ctrl[1].Y-w.ptMin.Y))
	case sb.IsCircle(n, &c, &r):
		p0, p1 := sb.Ctrl[0], sb.Ctrl[2]
		theta0 := libm.Atan2(p0.Y-c.Y, p0.X-c.X)
		theta1 := libm.Atan2(p1.Y-c.Y, p1.X-c.X)
		dtheta := srf.WrapSymmetric(theta1-theta0, 2*math.Pi)
		w.maybeMoveTo(p0, p1)
		dir := "arc"
		if dtheta < 0 {
			dir = "arcn"
		}
		w.printf("    %.3f %.3f %.3f %.3f %.3f %s\r\n",
			MmToPts(c.X-w.ptMin.X), MmToPts(c.Y-w.ptMin.Y),
			MmToPts(r),
			theta0*180/math.Pi, theta1*180/math.Pi,
			dir)
	case sb.Degree == 3 && !sb.IsRational():
		w.maybeMoveTo(sb.Ctrl[0], sb.Ctrl[3])
		w.printf("    %.3f %.3f %.3f %.3f %.3f %.3f curveto\r\n",
			MmToPts(sb.Ctrl[1].X-w.ptMin.X), MmToPts(sb.Ctrl[1].Y-w.ptMin.Y),
			MmToPts(sb.Ctrl[2].X-w.ptMin.X), MmToPts(sb.Ctrl[2].Y-w.ptMin.Y),
			MmToPts(sb.Ctrl[3].X-w.ptMin.X), MmToPts(sb.Ctrl[3].Y-w.ptMin.Y))
	default:
		w.BezierAsNonrationalCubic(sb, 0)
	}
}

func (w *epsFileWriter) FinishAndCloseFile() error {
	w.printf("\r\n" +
		"grestore\r\n" +
		"\r\n")
	return w.closeFile()
}

// ---------------------------------------------------------------------
// PDF: some extra complexity because we have to generate a correct xref
// table.
// ---------------------------------------------------------------------

type pdfFileWriter struct {
	vectorWriter
	xref      [10]uint32
	bodyStart uint32
	prevPt    geom.Vector
}

func (w *pdfFileWriter) HasCanvasSize() bool { return true }
func (w *pdfFileWriter) CanOutputMesh() bool { return true }

func (w *pdfFileWriter) ftell() uint32 { return uint32(w.out.Len()) }

func (w *pdfFileWriter) StartFile() {
	if (w.ptMax.X-w.ptMin.X) > 200*25.4 || (w.ptMax.Y-w.ptMin.Y) > 200*25.4 {
		w.warn("PDF page size exceeds 200 by 200 inches; many viewers may " +
			"reject this file.")
	}

	w.printf("%%PDF-1.1\r\n"+
		"%%%c%c%c%c\r\n",
		0xe2, 0xe3, 0xcf, 0xd3)

	w.xref[1] = w.ftell()
	w.printf("1 0 obj\r\n" +
		"  << /Type /Catalog\r\n" +
		"     /Outlines 2 0 R\r\n" +
		"     /Pages 3 0 R\r\n" +
		"  >>\r\n" +
		"endobj\r\n")

	w.xref[2] = w.ftell()
	w.printf("2 0 obj\r\n" +
		"  << /Type /Outlines\r\n" +
		"     /Count 0\r\n" +
		"  >>\r\n" +
		"endobj\r\n")

	w.xref[3] = w.ftell()
	w.printf("3 0 obj\r\n" +
		"  << /Type /Pages\r\n" +
		"     /Kids [4 0 R]\r\n" +
		"     /Count 1\r\n" +
		"  >>\r\n" +
		"endobj\r\n")

	w.xref[4] = w.ftell()
	w.printf("4 0 obj\r\n"+
		"  << /Type /Page\r\n"+
		"     /Parent 3 0 R\r\n"+
		"     /MediaBox [0 0 %.3f %.3f]\r\n"+
		"     /Contents 5 0 R\r\n"+
		"     /Resources << /ProcSet 7 0 R\r\n"+
		"                   /Font << /F1 8 0 R >>\r\n"+
		"                >>\r\n"+
		"  >>\r\n"+
		"endobj\r\n",
		MmToPts(w.ptMax.X-w.ptMin.X),
		MmToPts(w.ptMax.Y-w.ptMin.Y))

	w.xref[5] = w.ftell()
	w.printf("5 0 obj\r\n" +
		"  << /Length 6 0 R >>\r\n" +
		"stream\r\n")
	w.bodyStart = w.ftell()
}

func (w *pdfFileWriter) FinishAndCloseFile() error {
	bodyEnd := w.ftell()

	w.printf("endstream\r\n" +
		"endobj\r\n")

	w.xref[6] = w.ftell()
	w.printf("6 0 obj\r\n"+
		"  %d\r\n"+
		"endobj\r\n",
		bodyEnd-w.bodyStart)

	w.xref[7] = w.ftell()
	w.printf("7 0 obj\r\n" +
		"  [/PDF /Text]\r\n" +
		"endobj\r\n")

	w.xref[8] = w.ftell()
	w.printf("8 0 obj\r\n" +
		"  << /Type /Font\r\n" +
		"     /Subtype /Type1\r\n" +
		"     /Name /F1\r\n" +
		"     /BaseFont /Helvetica\r\n" +
		"     /Encoding /WinAnsiEncoding\r\n" +
		"  >>\r\n" +
		"endobj\r\n")

	w.xref[9] = w.ftell()
	w.printf("9 0 obj\r\n" +
		"  << /Creator (TeleSAPR)\r\n" +
		"  >>\r\n")

	xrefStart := w.ftell()
	w.printf("xref\r\n" +
		"0 10\r\n" +
		"0000000000 65535 f\r\n")

	for i := 1; i <= 9; i++ {
		w.printf("%010d %05d n\r\n", w.xref[i], 0)
	}

	w.printf("\r\n"+
		"trailer\r\n"+
		"  << /Size 10\r\n"+
		"     /Root 1 0 R\r\n"+
		"     /Info 9 0 R\r\n"+
		"  >>\r\n"+
		"startxref\r\n"+
		"%d\r\n"+
		"%%%%EOF\r\n",
		xrefStart)

	return w.closeFile()
}

func (w *pdfFileWriter) Background(color core.RGBA) {
	width := w.ptMax.X - w.ptMin.X
	height := w.ptMax.Y - w.ptMin.Y
	sw := max(width, height) / 1000

	w.printf("1 J 1 j\r\n"+
		"%.3f %.3f %.3f RG\r\n"+
		"%.3f %.3f %.3f rg\r\n"+
		"%.3f w\r\n"+
		"%.3f %.3f m\r\n"+
		"%.3f %.3f l\r\n"+
		"%.3f %.3f l\r\n"+
		"%.3f %.3f l\r\n"+
		"b\r\n",
		color.RedF(), color.GreenF(), color.BlueF(),
		color.RedF(), color.GreenF(), color.BlueF(),
		MmToPts(sw),
		MmToPts(0), MmToPts(0),
		MmToPts(width), MmToPts(0),
		MmToPts(width), MmToPts(height),
		MmToPts(0), MmToPts(height))
}

func (w *pdfFileWriter) StartPath(strokeRgb core.RGBA, lineWidth float64, filled bool,
	fillRgb core.RGBA, hs core.HStyle) {
	pattern := w.sk.StylePatternType(hs)
	stippleScale := MmToPts(w.styleStippleScaleMm(hs))

	w.printf("1 J 1 j "+ // round endcaps and joins
		"%.3f w [%s] 0 d "+
		"%.3f %.3f %.3f RG\r\n",
		MmToPts(lineWidth),
		makeStipplePattern(pattern, stippleScale, ' ', false),
		strokeRgb.RedF(), strokeRgb.GreenF(), strokeRgb.BlueF())
	if filled {
		w.printf("%.3f %.3f %.3f rg\r\n", fillRgb.RedF(), fillRgb.GreenF(), fillRgb.BlueF())
	}

	w.prevPt = veryPositiveVector
}

func (w *pdfFileWriter) FinishPath(strokeRgb core.RGBA, lineWidth float64, filled bool,
	fillRgb core.RGBA, hs core.HStyle, closed bool) {
	if filled {
		w.printf("b\r\n")
	} else {
		w.printf("S\r\n")
	}
}

func (w *pdfFileWriter) maybeMoveTo(st, fi geom.Vector) {
	if !w.prevPt.Equals(st) {
		w.printf("%.3f %.3f m\r\n", MmToPts(st.X-w.ptMin.X), MmToPts(st.Y-w.ptMin.Y))
	}
	w.prevPt = fi
}

func (w *pdfFileWriter) Triangle(tr *srf.STriangle) {
	sw := max(w.ptMax.X-w.ptMin.X, w.ptMax.Y-w.ptMin.Y) / 1000
	c := tr.Meta.RGBA

	w.printf("1 J 1 j\r\n"+
		"%.3f %.3f %.3f RG\r\n"+
		"%.3f %.3f %.3f rg\r\n"+
		"%.3f w\r\n"+
		"%.3f %.3f m\r\n"+
		"%.3f %.3f l\r\n"+
		"%.3f %.3f l\r\n"+
		"b\r\n",
		c.RedF(), c.GreenF(), c.BlueF(),
		c.RedF(), c.GreenF(), c.BlueF(),
		MmToPts(sw),
		MmToPts(tr.A.X-w.ptMin.X), MmToPts(tr.A.Y-w.ptMin.Y),
		MmToPts(tr.B.X-w.ptMin.X), MmToPts(tr.B.Y-w.ptMin.Y),
		MmToPts(tr.C.X-w.ptMin.X), MmToPts(tr.C.Y-w.ptMin.Y))
}

func (w *pdfFileWriter) Bezier(sb *srf.SBezier) {
	switch {
	case sb.Degree == 1:
		w.maybeMoveTo(sb.Ctrl[0], sb.Ctrl[1])
		w.printf("%.3f %.3f l\r\n",
			MmToPts(sb.Ctrl[1].X-w.ptMin.X), MmToPts(sb.Ctrl[1].Y-w.ptMin.Y))
	case sb.Degree == 3 && !sb.IsRational():
		w.maybeMoveTo(sb.Ctrl[0], sb.Ctrl[3])
		w.printf("%.3f %.3f %.3f %.3f %.3f %.3f c\r\n",
			MmToPts(sb.Ctrl[1].X-w.ptMin.X), MmToPts(sb.Ctrl[1].Y-w.ptMin.Y),
			MmToPts(sb.Ctrl[2].X-w.ptMin.X), MmToPts(sb.Ctrl[2].Y-w.ptMin.Y),
			MmToPts(sb.Ctrl[3].X-w.ptMin.X), MmToPts(sb.Ctrl[3].Y-w.ptMin.Y))
	default:
		w.BezierAsNonrationalCubic(sb, 0)
	}
}

// ---------------------------------------------------------------------
// SVG
// ---------------------------------------------------------------------

type svgFileWriter struct {
	vectorWriter
	prevPt geom.Vector
}

func (w *svgFileWriter) HasCanvasSize() bool { return true }
func (w *svgFileWriter) CanOutputMesh() bool { return true }

func (w *svgFileWriter) StartFile() {
	w.printf("<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.0//EN\" "+
		"\"http://www.w3.org/TR/2001/REC-SVG-20010904/DTD/svg10.dtd\">\r\n"+
		"<svg xmlns=\"http://www.w3.org/2000/svg\"  "+
		"xmlns:xlink=\"http://www.w3.org/1999/xlink\" "+
		"width='%.3fmm' height='%.3fmm' "+
		"viewBox=\"0 0 %.3f %.3f\">\r\n"+
		"\r\n"+
		"<title>Exported SVG</title>\r\n"+
		"\r\n",
		w.ptMax.X-w.ptMin.X, w.ptMax.Y-w.ptMin.Y,
		w.ptMax.X-w.ptMin.X, w.ptMax.Y-w.ptMin.Y)

	w.printf("<style><![CDATA[\r\n")
	w.printf("polygon {\r\n")
	w.printf("shape-rendering:crispEdges;\r\n")
	// crispEdges turns of anti-aliasing, which tends to cause hairline
	// cracks between triangles; but there still is some cracking, so
	// specify a stroke width too, hope for around a pixel
	sw := max(w.ptMax.X-w.ptMin.X, w.ptMax.Y-w.ptMin.Y) / 1000
	w.printf("stroke-width:%f;\r\n", sw)
	w.printf("}\r\n")

	exportStyle := func(hs core.HStyle) {
		s := w.sk.StyleGet(hs)
		strokeRgb := w.styleExportColor(hs)
		fillRgb := w.styleExportFillColor(hs)
		pattern := w.sk.StylePatternType(hs)
		stippleScale := w.styleStippleScaleMm(hs)

		w.printf(".s%x {\r\n", hs.V)
		w.printf("stroke:#%02x%02x%02x;\r\n", strokeRgb.Red, strokeRgb.Green, strokeRgb.Blue)
		// A style's own width, a view's on the screen, is written at
		// half; one the options give is a weight on paper, written as
		// it is.
		width := w.styleWidthMm(hs) / 2
		if mm, ok := w.o.WidthsMm[hs]; ok {
			width = mm
		}
		w.printf("stroke-width:%f;\r\n", width)
		w.printf("stroke-linecap:round;\r\n")
		w.printf("stroke-linejoin:round;\r\n")
		patternStr := makeStipplePattern(pattern, stippleScale, ',', true)
		if patternStr != "" {
			w.printf("stroke-dasharray:%s;\r\n", patternStr)
		}
		if s.Filled {
			w.printf("fill:#%02x%02x%02x;\r\n", fillRgb.Red, fillRgb.Green, fillRgb.Blue)
		} else {
			w.printf("fill:none;\r\n")
		}
		w.printf("}\r\n")
	}

	exportStyle(core.HStyle{V: core.StyleNoStyle})
	// Style::Get can add styles; the list is ranged over as it grows.
	for i := range len(w.sk.Styles) {
		exportStyle(w.sk.Styles[i].H)
	}
	w.printf("]]></style>\r\n")
}

func (w *svgFileWriter) Background(color core.RGBA) {
	w.printf("<style><![CDATA[\r\n"+
		"svg {\r\n"+
		"background-color:#%02x%02x%02x;\r\n"+
		"}\r\n"+
		"]]></style>\r\n",
		color.Red, color.Green, color.Blue)
}

func (w *svgFileWriter) StartPath(strokeRgb core.RGBA, lineWidth float64, filled bool,
	fillRgb core.RGBA, hs core.HStyle) {
	w.printf("<path d='")
	w.prevPt = veryPositiveVector
}

func (w *svgFileWriter) FinishPath(strokeRgb core.RGBA, lineWidth float64, filled bool,
	fillRgb core.RGBA, hs core.HStyle, closed bool) {
	if closed {
		w.printf(" Z")
	}
	fill := ""
	if filled {
		fill = cfmt.Sprintf("fill='#%02x%02x%02x'", fillRgb.Red, fillRgb.Green, fillRgb.Blue)
	}
	cls := cfmt.Sprintf("s%x", hs.V)
	w.printf("' class='%s' %s/>\r\n", cls, fill)
}

func (w *svgFileWriter) maybeMoveTo(st, fi geom.Vector) {
	// SVG uses a coordinate system with the origin at top left, +y down
	if !w.prevPt.Equals(st) {
		w.printf("M%.3f %.3f ", st.X-w.ptMin.X, w.ptMax.Y-st.Y)
	}
	w.prevPt = fi
}

func (w *svgFileWriter) Triangle(tr *srf.STriangle) {
	c := tr.Meta.RGBA
	w.printf("<polygon points='%.3f,%.3f %.3f,%.3f %.3f,%.3f' "+
		"stroke='#%02x%02x%02x' "+
		"fill='#%02x%02x%02x'/>\r\n",
		tr.A.X-w.ptMin.X, w.ptMax.Y-tr.A.Y,
		tr.B.X-w.ptMin.X, w.ptMax.Y-tr.B.Y,
		tr.C.X-w.ptMin.X, w.ptMax.Y-tr.C.Y,
		c.Red, c.Green, c.Blue,
		c.Red, c.Green, c.Blue)
}

func (w *svgFileWriter) Bezier(sb *srf.SBezier) {
	var c geom.Vector
	n := geom.Vector{Z: 1}
	var r float64
	switch {
	case sb.Degree == 1:
		w.maybeMoveTo(sb.Ctrl[0], sb.Ctrl[1])
		w.printf("L%.3f,%.3f ", sb.Ctrl[1].X-w.ptMin.X, w.ptMax.Y-sb.Ctrl[1].Y)
	case sb.IsCircle(n, &c, &r):
		p0, p1 := sb.Ctrl[0], sb.Ctrl[2]
		theta0 := libm.Atan2(p0.Y-c.Y, p0.X-c.X)
		theta1 := libm.Atan2(p1.Y-c.Y, p1.X-c.X)
		dtheta := srf.WrapSymmetric(theta1-theta0, 2*math.Pi)
		// The arc must be less than 180 degrees, or else it couldn't
		// have been represented as a single rational Bezier. So
		// large-arc-flag must be false. sweep-flag is determined by the
		// sign of dtheta. Note that clockwise and counter-clockwise are
		// backwards in SVG's mirrored csys.
		w.maybeMoveTo(p0, p1)
		sweep := 0
		if dtheta < 0 {
			sweep = 1
		}
		w.printf("A%.3f,%.3f 0 0,%d %.3f,%.3f ", r, r, sweep, p1.X-w.ptMin.X, w.ptMax.Y-p1.Y)
	case !sb.IsRational() && sb.Degree <= 3:
		switch sb.Degree {
		case 2:
			w.maybeMoveTo(sb.Ctrl[0], sb.Ctrl[2])
			w.printf("Q%.3f,%.3f %.3f,%.3f ",
				sb.Ctrl[1].X-w.ptMin.X, w.ptMax.Y-sb.Ctrl[1].Y,
				sb.Ctrl[2].X-w.ptMin.X, w.ptMax.Y-sb.Ctrl[2].Y)
		case 3:
			w.maybeMoveTo(sb.Ctrl[0], sb.Ctrl[3])
			w.printf("C%.3f,%.3f %.3f,%.3f %.3f,%.3f ",
				sb.Ctrl[1].X-w.ptMin.X, w.ptMax.Y-sb.Ctrl[1].Y,
				sb.Ctrl[2].X-w.ptMin.X, w.ptMax.Y-sb.Ctrl[2].Y,
				sb.Ctrl[3].X-w.ptMin.X, w.ptMax.Y-sb.Ctrl[3].Y)
		}
	default:
		w.BezierAsNonrationalCubic(sb, 0)
	}
}

func (w *svgFileWriter) FinishAndCloseFile() error {
	w.printf("\r\n</svg>\r\n")
	return w.closeFile()
}

// ---------------------------------------------------------------------
// HPGL
// ---------------------------------------------------------------------

type hpglFileWriter struct {
	vectorWriter
}

func (w *hpglFileWriter) HasCanvasSize() bool { return false }
func (w *hpglFileWriter) CanOutputMesh() bool { return false }

func mmToHpglUnits(mm float64) float64 { return mm * 40 }

func (w *hpglFileWriter) StartFile() {
	w.printf("IN;\r\n")
	w.printf("SP1;\r\n")
}

func (w *hpglFileWriter) Background(core.RGBA) {}
func (w *hpglFileWriter) StartPath(core.RGBA, float64, bool, core.RGBA, core.HStyle) {
}
func (w *hpglFileWriter) FinishPath(core.RGBA, float64, bool, core.RGBA, core.HStyle, bool) {
}
func (w *hpglFileWriter) Triangle(*srf.STriangle) {}

func (w *hpglFileWriter) Bezier(sb *srf.SBezier) {
	if sb.Degree == 1 {
		w.printf("PU%d,%d;\r\n", int(mmToHpglUnits(sb.Ctrl[0].X)), int(mmToHpglUnits(sb.Ctrl[0].Y)))
		w.printf("PD%d,%d;\r\n", int(mmToHpglUnits(sb.Ctrl[1].X)), int(mmToHpglUnits(sb.Ctrl[1].Y)))
	} else {
		w.BezierAsPwl(sb)
	}
}

func (w *hpglFileWriter) FinishAndCloseFile() error {
	return w.closeFile()
}

// ---------------------------------------------------------------------
// STEP: just a wrapper around the general STEP stuff that can also be
// used for surfaces or 3d curves.
// ---------------------------------------------------------------------

type step2dFileWriter struct {
	vectorWriter
	sfw stepFileWriter
}

func (w *step2dFileWriter) HasCanvasSize() bool { return false }
func (w *step2dFileWriter) CanOutputMesh() bool { return false }

func (w *step2dFileWriter) StartFile() {
	w.sfw = stepFileWriter{sk: w.sk, o: w.o, f: &w.out, exportParts: true}
	w.sfw.WriteHeader()
}

func (w *step2dFileWriter) Background(core.RGBA)    {}
func (w *step2dFileWriter) Triangle(*srf.STriangle) {}
func (w *step2dFileWriter) StartPath(core.RGBA, float64, bool, core.RGBA, core.HStyle) {
}
func (w *step2dFileWriter) FinishPath(core.RGBA, float64, bool, core.RGBA, core.HStyle, bool) {
}

func (w *step2dFileWriter) Bezier(sb *srf.SBezier) {
	c := w.sfw.ExportCurve(sb)
	w.sfw.curves = append(w.sfw.curves, c)
}

func (w *step2dFileWriter) FinishAndCloseFile() error {
	w.sfw.WriteWireframe()
	w.sfw.WriteFooter()
	return w.closeFile()
}
