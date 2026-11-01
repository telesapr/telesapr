package ui

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestStatusLines checks that a long message is wrapped to the free
// area and its lines go down from the area's top left corner, inside it.
func TestStatusLines(t *testing.T) {
	t.Parallel()
	area := screenArea{Mid: geom.Point2d{U: 40}, Width: 520, Height: 600}
	const h = 11.5
	msg := "the sketch g002-sketch-in-plane is not closed: a contour has an open end at " +
		"(-20, 20, 0) (or its curves are not all of one style)"
	lines := statusLines(msg, h, area)
	if len(lines) < 2 {
		t.Fatalf("not wrapped: %v", lines)
	}
	vf := res.BuiltinVectorFont()
	var words []string
	for i, l := range lines {
		words = append(words, l.Text)
		if l.At.U != area.Mid.U-area.Width/2+statusMargin {
			t.Errorf("line %d starts at %v, not at the free area's left", i, l.At.U)
		}
		if top := area.Mid.V + area.Height/2; l.At.V+h > top-statusMargin+1e-9 {
			t.Errorf("line %d at %v goes above the area's top %v", i, l.At.V, top)
		}
		if i > 0 && l.At.V >= lines[i-1].At.V {
			t.Errorf("line %d is not below the one before", i)
		}
		if w := vf.Width(h, l.Text); w > area.Width-2*statusMargin+h {
			t.Errorf("line %d is %v wide, wider than the area", i, w)
		}
	}
	if got := strings.Join(words, " "); got != msg {
		t.Errorf("lines say %q, not %q", got, msg)
	}

	if short := statusLines("zero-length edge!", h, area); len(short) != 1 {
		t.Errorf("a short message is %d lines", len(short))
	}
}

// textCanvas records what text a draw writes, where on the screen.
type textCanvas struct {
	recordingCanvas
	texts []string
	at    []geom.Point2d
}

func (tc *textCanvas) DrawVectorText(text string, _ float64, o, _, _ geom.Vector, _ render.HStroke) {
	tc.texts = append(tc.texts, text)
	tc.at = append(tc.at, tc.camera.ProjectPoint(o))
}

func (tc *textCanvas) DrawLine(geom.Vector, geom.Vector, render.HStroke) {}
func (tc *textCanvas) DrawEdges(*srf.SEdgeList, render.HStroke)          {}
func (tc *textCanvas) DrawPoint(geom.Vector, render.HStroke)             {}
func (tc *textCanvas) DrawQuad(_, _, _, _ geom.Vector, _ render.HFill)   {}
func (tc *textCanvas) DrawPolygon(*srf.SPolygon, render.HFill)           {}
func (tc *textCanvas) DrawMesh(*srf.SMesh, render.HFill, render.HFill)   {}
func (tc *textCanvas) DrawFaces(*srf.SMesh, []uint32, render.HFill)      {}
func (tc *textCanvas) DrawOutlines(*srf.SOutlineList, render.HStroke, render.DrawOutlinesAs) {
}

// drawnTexts draws the window as it is and gives the text it writes
// and where.
func drawnTexts(r *uiScriptRunner) (texts []string, at []geom.Point2d) {
	tc := &textCanvas{}
	tc.camera = r.ss.GW.Camera()
	tc.SetBitmapFont(r.ss.GW.bitmapFont())
	r.ss.GW.Draw(tc)
	return tc.texts, tc.at
}

// TestPolyErrorPlacement checks that an open sketch no solid is made
// from says nothing over the model, nor "err" on the home screen, and
// that once it is extruded the extrusion and the sketch say what is
// wrong in the top left corner of the free area, clear of the line.
func TestPolyErrorPlacement(t *testing.T) {
	t.Parallel()
	r := newUIScriptRunner(t)
	for _, line := range []string{
		"key s", "move 200 300", "down L 200 300", "up L 200 300",
		"move 400 300", "down L 400 300", "up L 400 300", "key esc",
	} {
		r.step(line)
		r.settle()
	}
	texts, _ := drawnTexts(r)
	for _, s := range texts {
		if strings.Contains(s, "not closed") {
			t.Errorf("a line no solid is made from says %q", s)
		}
	}
	var rows strings.Builder
	r.ss.TW.GoToScreen(screenListOfGroups)
	r.ss.TW.Show()
	r.ss.TW.dumpRows(&rows)
	if !strings.Contains(rows.String(), "g002-sketch-in-plane") {
		t.Fatalf("the home screen is not shown:\n%s", rows.String())
	}
	if strings.Contains(rows.String(), " err ") {
		t.Errorf("the home screen says err for a line:\n%s", rows.String())
	}

	camera := r.ss.GW.Camera()
	area := r.ss.GW.freeArea(&camera)
	corner := func(what string) {
		t.Helper()
		texts, at := drawnTexts(r)
		for i, s := range texts {
			if !strings.Contains(s, what) {
				continue
			}
			if left := area.Mid.U - area.Width/2 + statusMargin; at[i].U < left-1 || at[i].U > left+1 {
				t.Errorf("%q starts at x %v, not at the free area's left %v", s, at[i].U, left)
			}
			if at[i].V < area.Height/2-60 {
				t.Errorf("%q is at y %v, not at the window's top", s, at[i].V)
			}
			return
		}
		t.Errorf("nothing says %q; drawn: %q", what, texts)
	}

	r.step("key x shift")
	r.settle()
	corner("is not closed")

	sketchH := r.ss.SK.Group(r.ss.GW.activeGroupH()).OpA
	r.ss.TW.ScreenActivateGroup(0, sketchH.V)
	r.settle()
	corner("not closed contour")
	r.ss.TW.GoToScreen(screenListOfGroups)
	r.ss.TW.Show()
	rows.Reset()
	r.ss.TW.dumpRows(&rows)
	if !strings.Contains(rows.String(), " err ") {
		t.Errorf("the home screen says no err for an extruded line:\n%s", rows.String())
	}
}
