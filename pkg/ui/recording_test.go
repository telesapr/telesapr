package ui

import (
	"fmt"
	"io"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/render"
	"github.com/spacesolve/spacesolve/pkg/res"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// recordingCanvas prints every draw call, as scripts/cpp-draw-harness
// does for C++.
type recordingCanvas struct {
	render.CanvasBase
	camera render.Camera
	w      io.Writer
}

func newRecordingCanvas(w io.Writer) *recordingCanvas { return &recordingCanvas{w: w} }

func (rc *recordingCanvas) GetCamera() *render.Camera { return &rc.camera }

func (rc *recordingCanvas) p(v geom.Vector) { fmt.Fprintf(rc.w, " %x %x %x", v.X, v.Y, v.Z) }
func (rc *recordingCanvas) c(c core.RgbaColor) {
	fmt.Fprintf(rc.w, " %02x%02x%02x%02x", c.Red, c.Green, c.Blue, c.Alpha)
}
func (rc *recordingCanvas) s(h render.HStroke) {
	s := rc.Stroke(h)
	fmt.Fprintf(rc.w, " | stroke %d %d", s.Layer, s.ZIndex)
	rc.c(s.Color)
	fmt.Fprintf(rc.w, " %x %d %d %x", s.Width, s.Unit, s.StipplePattern, s.StippleScale)
}
func (rc *recordingCanvas) f(h render.HFill) {
	f := rc.Fill(h)
	fmt.Fprintf(rc.w, " | fill %d %d", f.Layer, f.ZIndex)
	rc.c(f.Color)
	tex := 0
	if f.Texture != nil {
		tex = 1
	}
	fmt.Fprintf(rc.w, " %d %d", f.Pattern, tex)
}

func (rc *recordingCanvas) DrawLine(a, b geom.Vector, hcs render.HStroke) {
	fmt.Fprint(rc.w, "line")
	rc.p(a)
	rc.p(b)
	rc.s(hcs)
	fmt.Fprintln(rc.w)
}

func (rc *recordingCanvas) DrawEdges(el *srf.SEdgeList, hcs render.HStroke) {
	fmt.Fprintf(rc.w, "edges %d", len(el.Edges))
	rc.s(hcs)
	fmt.Fprintln(rc.w)
	for _, e := range el.Edges {
		fmt.Fprint(rc.w, " e")
		rc.p(e.A)
		rc.p(e.B)
		fmt.Fprintln(rc.w)
	}
}

func (rc *recordingCanvas) DrawBeziers(*srf.SBezierList, render.HStroke) bool { return false }

func (rc *recordingCanvas) DrawOutlines(ol *srf.SOutlineList, hcs render.HStroke, drawAs render.DrawOutlinesAs) {
	fmt.Fprintf(rc.w, "outlines %d %d", len(ol.Outlines), drawAs)
	rc.s(hcs)
	fmt.Fprintln(rc.w)
	for _, o := range ol.Outlines {
		fmt.Fprintf(rc.w, " o %d", o.Tag)
		rc.p(o.A)
		rc.p(o.B)
		fmt.Fprintln(rc.w)
	}
}

func (rc *recordingCanvas) DrawVectorText(text string, height float64, o, u, v geom.Vector, hcs render.HStroke) {
	fmt.Fprintf(rc.w, "text [%s] %x", text, height)
	rc.p(o)
	rc.p(u)
	rc.p(v)
	rc.s(hcs)
	fmt.Fprintln(rc.w)
}

func (rc *recordingCanvas) DrawQuad(a, b, c, d geom.Vector, hcf render.HFill) {
	fmt.Fprint(rc.w, "quad")
	rc.p(a)
	rc.p(b)
	rc.p(c)
	rc.p(d)
	rc.f(hcf)
	fmt.Fprintln(rc.w)
}

func (rc *recordingCanvas) DrawPoint(o geom.Vector, hcs render.HStroke) {
	fmt.Fprint(rc.w, "point")
	rc.p(o)
	rc.s(hcs)
	fmt.Fprintln(rc.w)
}

func (rc *recordingCanvas) DrawPolygon(p *srf.SPolygon, hcf render.HFill) {
	fmt.Fprintf(rc.w, "polygon %d", len(p.Contours))
	rc.f(hcf)
	fmt.Fprintln(rc.w)
	for _, sc := range p.Contours {
		fmt.Fprintf(rc.w, " c %d", len(sc.Points))
		for _, sp := range sc.Points {
			rc.p(sp.P)
		}
		fmt.Fprintln(rc.w)
	}
}

func (rc *recordingCanvas) DrawMesh(m *srf.SMesh, hcfFront, hcfBack render.HFill) {
	fmt.Fprintf(rc.w, "mesh %d", len(m.Triangles))
	rc.f(hcfFront)
	if hcfBack != 0 {
		rc.f(hcfBack)
	}
	fmt.Fprintln(rc.w)
	for _, t := range m.Triangles {
		fmt.Fprintf(rc.w, " t %d", uint32(t.Meta.Face))
		rc.c(t.Meta.RgbaColor)
		rc.p(t.A)
		rc.p(t.B)
		rc.p(t.C)
		fmt.Fprintln(rc.w)
	}
}

func (rc *recordingCanvas) DrawFaces(m *srf.SMesh, faces []uint32, hcf render.HFill) {
	fmt.Fprintf(rc.w, "faces %d", len(faces))
	for _, f := range faces {
		fmt.Fprintf(rc.w, " %x", f)
	}
	rc.f(hcf)
	fmt.Fprintln(rc.w)
}

func (rc *recordingCanvas) DrawPixmap(pm *res.Pixmap, o, u, v geom.Vector, ta, tb geom.Point2d, hcf render.HFill) {
	w, h := 0, 0
	if pm != nil {
		w, h = pm.Width, pm.Height
	}
	fmt.Fprintf(rc.w, "pixmap %d %d", w, h)
	rc.p(o)
	rc.p(u)
	rc.p(v)
	fmt.Fprintf(rc.w, " %x %x %x %x", ta.U, ta.V, tb.U, tb.V)
	rc.f(hcf)
	fmt.Fprintln(rc.w)
}

func (rc *recordingCanvas) InvalidatePixmap(*res.Pixmap) {}
