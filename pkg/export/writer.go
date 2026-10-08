package export

import (
	"bytes"
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// writeExportCurves writes the projected edges and curves, assembled
// into loops where they close, and the mesh, to out (the end of C++
// ExportLinesAndMesh).
func WriteCurves(sel *srf.SEdgeList, sbl *srf.SBezierList, sms *srf.SMesh, tol srf.Tolerance, out Writer) error {
	// We kept the line segments and Beziers separate until now; but put
	// them all together, and also project everything into the xy plane,
	// since not all export targets ignore the z component of the points.
	if sbl == nil {
		panic("ui: adding line segments to beziers assumes bezier list is non-nil")
	}
	for _, e := range sel.Edges {
		sb := srf.SBezierFrom2(e.A, e.B)
		sb.AuxA = e.AuxA
		sbl.Curves = append(sbl.Curves, sb)
	}
	for i := range sbl.Curves {
		b := &sbl.Curves[i]
		for k := 0; k <= b.Degree; k++ {
			b.Ctrl[k].Z = 0
		}
	}

	// If possible, then we will assemble these output curves into loops.
	// They will then get exported as closed paths.
	var sblss srf.SBezierLoopSetSet
	var leftovers []srf.SBezierLoop
	plane := srf.FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	var spxyz srf.SPolygon
	var allClosed bool
	var notClosedAt srf.SEdge
	for i := range sbl.Curves {
		sbl.Curves[i].Tag = 0
	}
	sblss.FindOuterFacesFrom(sbl, &spxyz, &plane, tol,
		&allClosed, &notClosedAt, nil, nil, &leftovers)
	if len(leftovers) > 0 {
		sblss.Sets = append(sblss.Sets, srf.SBezierLoopSet{Outer: leftovers[0], Holes: leftovers[1:]})
	}

	// Now write the lines and triangles to the output file
	return out.vw().OutputLinesAndMesh(&sblss, sms)
}

// Writer is a 2d vector file format (C++ VectorFileWriter).
type Writer interface {
	vw() *vectorWriter
	StartPath(strokeRgb core.RGBA, lineWidth float64, filled bool, fillRgb core.RGBA, hs core.HStyle)
	FinishPath(strokeRgb core.RGBA, lineWidth float64, filled bool, fillRgb core.RGBA,
		hs core.HStyle, closed bool)
	Bezier(sb *srf.SBezier)
	Triangle(tr *srf.STriangle)
	OutputConstraints(constraints []core.Constraint) bool
	Background(color core.RGBA)
	StartFile()
	FinishAndCloseFile() error
	HasCanvasSize() bool
	CanOutputMesh() bool
	// SetModelviewProjection sets the projection the curves are
	// written in; OutputLinesAndMesh writes the loops and the mesh and
	// finishes the file.
	SetModelviewProjection(u, v, n, origin geom.Vector, cameraTan, scale float64)
	OutputLinesAndMesh(sblss *srf.SBezierLoopSetSet, sm *srf.SMesh) error
}

// vectorWriter is what the formats share (C++ VectorFileWriter's
// fields and non-virtual methods). The file is written when it's
// finished.
type vectorWriter struct {
	sk   *sketch.Sketch
	o    Options
	self Writer

	u, v, n, origin  geom.Vector
	cameraTan, scale float64

	f        *os.File
	out      bytes.Buffer
	filename string

	ptMin, ptMax geom.Vector
}

func (w *vectorWriter) vw() *vectorWriter { return w }

// OutputConstraints reports whether the format writes constraints
// itself; most don't.
func (w *vectorWriter) OutputConstraints([]core.Constraint) bool { return false }

func (w *vectorWriter) printf(format string, args ...any) {
	w.out.WriteString(cfmt.Sprintf(format, args...))
}

// closeFile writes the file.
func (w *vectorWriter) closeFile() error {
	if w.f == nil {
		return nil
	}
	_, err := w.f.Write(w.out.Bytes())
	if cerr := w.f.Close(); err == nil {
		err = cerr
	}
	w.f = nil
	w.out.Reset()
	if err != nil {
		return fmt.Errorf("couldn't write to '%s'", w.filename)
	}
	return nil
}

// MmToPts converts mm to points, 72 to the inch (C++
// VectorFileWriter::MmToPts).
func MmToPts(mm float64) float64 {
	// 72 points in an inch
	return (mm / 25.4) * 72
}

// NewWriter is the writer for a file's format, told by its extension,
// with the file opened.
func NewWriter(filename string, sk *sketch.Sketch, o Options) (Writer, error) {
	var ret Writer
	needOpen := true
	switch {
	case hasExtension(filename, "dxf"):
		ret = &dxfFileWriter{}
		needOpen = false
	case hasExtension(filename, "ps"), hasExtension(filename, "eps"):
		ret = &epsFileWriter{}
	case hasExtension(filename, "pdf"):
		ret = &pdfFileWriter{}
	case hasExtension(filename, "svg"):
		ret = &svgFileWriter{}
	case hasExtension(filename, "plt"), hasExtension(filename, "hpgl"):
		ret = &hpglFileWriter{}
	case hasExtension(filename, "step"), hasExtension(filename, "stp"):
		ret = &step2dFileWriter{}
	case hasExtension(filename, "txt"), hasExtension(filename, "ngc"):
		ret = &gcodeFileWriter{}
	default:
		return nil, fmt.Errorf("can't identify output file type from file extension of "+
			"filename '%s'; try "+
			".step, .stp, .dxf, .svg, .plt, .hpgl, .pdf, .txt, .ngc, "+
			".eps, or .ps", filename)
	}
	w := ret.vw()
	w.sk, w.o = sk, o
	w.self = ret
	w.filename = filename
	if !needOpen {
		return ret, nil
	}

	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("couldn't write to '%s'", filename)
	}
	w.f = f
	return ret, nil
}

// SetModelviewProjection sets the projection Transform applies (C++
// VectorFileWriter::SetModelviewProjection).
func (w *vectorWriter) SetModelviewProjection(u, v, n, origin geom.Vector, cameraTan, scale float64) {
	w.u = u
	w.v = v
	w.n = n
	w.origin = origin
	w.cameraTan = cameraTan
	w.scale = scale
}

// Transform projects a point as the view was exported (C++
// VectorFileWriter::Transform).
func (w *vectorWriter) Transform(pos geom.Vector) geom.Vector {
	return pos.InPerspective(w.u, w.v, w.n, w.origin, w.cameraTan).ScaledBy(1.0 / w.scale)
}

// loopsOf is a loop set's loops in C++'s order: the outer, then the
// holes.
func loopsOf(sbls *srf.SBezierLoopSet) []*srf.SBezierLoop {
	loops := []*srf.SBezierLoop{&sbls.Outer}
	for i := range sbls.Holes {
		loops = append(loops, &sbls.Holes[i])
	}
	return loops
}

// OutputLinesAndMesh works out the canvas and writes the file (C++
// VectorFileWriter::OutputLinesAndMesh).
func (w *vectorWriter) OutputLinesAndMesh(sblss *srf.SBezierLoopSetSet, sm *srf.SMesh) error {
	sk := w.sk

	// First calculate the bounding box.
	w.ptMin = geom.Vector{X: geom.VeryPositive, Y: geom.VeryPositive, Z: geom.VeryPositive}
	w.ptMax = geom.Vector{X: -geom.VeryPositive, Y: -geom.VeryPositive, Z: -geom.VeryPositive}
	if sm != nil {
		for _, tr := range sm.Triangles {
			tr.A.MakeMaxMin(&w.ptMax, &w.ptMin)
			tr.B.MakeMaxMin(&w.ptMax, &w.ptMin)
			tr.C.MakeMaxMin(&w.ptMax, &w.ptMin)
		}
	}
	if sblss != nil {
		for si := range sblss.Sets {
			for _, sbl := range loopsOf(&sblss.Sets[si]) {
				for _, b := range sbl.Curves {
					for i := 0; i <= b.Degree; i++ {
						b.Ctrl[i].MakeMaxMin(&w.ptMax, &w.ptMin)
					}
				}
			}
		}
	}

	// And now we compute the canvas size.
	s := 1.0 / w.o.Scale
	if w.o.CanvasSizeAuto {
		// It's based on the calculated bounding box; we grow it along
		// each boundary by the specified amount.
		w.ptMin.X -= s * w.o.Margin.Left
		w.ptMax.X += s * w.o.Margin.Right
		w.ptMin.Y -= s * w.o.Margin.Bottom
		w.ptMax.Y += s * w.o.Margin.Top
	} else {
		w.ptMin.X = s * w.o.Canvas.Dx
		w.ptMin.Y = s * w.o.Canvas.Dy
		w.ptMax.X = w.ptMin.X + s*w.o.Canvas.Width
		w.ptMax.Y = w.ptMin.Y + s*w.o.Canvas.Height
	}

	w.self.StartFile()
	if w.o.Background != nil {
		w.self.Background(*w.o.Background)
	}
	if sm != nil && w.o.ShadedTriangles {
		for i := range sm.Triangles {
			w.self.Triangle(&sm.Triangles[i])
		}
	}
	if sblss != nil {
		for si := range sblss.Sets {
			for _, sbl := range loopsOf(&sblss.Sets[si]) {
				if len(sbl.Curves) == 0 || !sk.StyleExportable(hstyle(uint32(sbl.Curves[0].AuxA))) {
					continue
				}

				hs := hstyle(uint32(sbl.Curves[0].AuxA))
				stl := sk.StyleGet(hs)
				lineWidth := w.styleWidthMm(hs) * s
				strokeRgb := w.styleExportColor(hs)
				fillRgb := w.styleExportFillColor(hs)

				w.self.StartPath(strokeRgb, lineWidth, stl.Filled, fillRgb, hs)
				for i := range sbl.Curves {
					w.self.Bezier(&sbl.Curves[i])
				}
				w.self.FinishPath(strokeRgb, lineWidth, stl.Filled, fillRgb, hs, sbl.IsClosed())
			}
		}
	}
	return w.self.FinishAndCloseFile()
}

// BezierAsPwl writes a curve as line segments (C++
// VectorFileWriter::BezierAsPwl).
func (w *vectorWriter) BezierAsPwl(sb *srf.SBezier) {
	var lv []geom.Vector
	sb.MakePwlIntoPoints(&lv, w.o.Tol, 0)

	for i := 1; i < len(lv); i++ {
		sb := srf.SBezierFrom2(lv[i-1], lv[i])
		w.self.Bezier(&sb)
	}
}

// BezierAsNonrationalCubic writes a curve as nonrational cubics (C++
// VectorFileWriter::BezierAsNonrationalCubic).
func (w *vectorWriter) BezierAsNonrationalCubic(sb *srf.SBezier, depth int) {
	t0, t1 := sb.TangentAt(0), sb.TangentAt(1)
	// The curve is correct, and the first derivatives are correct, at
	// the endpoints.
	bnr := srf.SBezierFrom4(
		sb.Start(),
		sb.Start().Plus(t0.ScaledBy(1.0/3)),
		sb.Finish().Minus(t1.ScaledBy(1.0/3)),
		sb.Finish())

	tol := w.o.Tol.ChordTolMm()
	// Arbitrary choice, but make it a little finer than pwl tolerance
	// since it should be easier to achieve that with the smooth curves.
	tol /= 2

	closeEnough := true
	for i := 1; i <= 3; i++ {
		t := float64(i) / 4.0
		p0 := sb.PointAt(t)
		pn := bnr.PointAt(t)
		d := p0.Minus(pn).Magnitude()
		if d > tol {
			closeEnough = false
		}
	}

	if closeEnough || depth > 3 {
		w.self.Bezier(&bnr)
	} else {
		var bef, aft srf.SBezier
		sb.SplitAt(0.5, &bef, &aft)
		w.BezierAsNonrationalCubic(&bef, depth+1)
		w.BezierAsNonrationalCubic(&aft, depth+1)
	}
}
