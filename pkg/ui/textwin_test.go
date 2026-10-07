package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// textwinFixtures are the fixtures TestTextWindow shows the screens
// of.
var textwinFixtures = []string{
	"analysis/contour_area/normal.slvs",
	"constraint/angle/normal.slvs",
	"constraint/comment/normal.slvs",
	"constraint/diameter/normal.slvs",
	"constraint/diameter/reference.slvs",
	"constraint/length_ratio/normal.slvs",
	"constraint/parallel/normal.slvs",
	"constraint/proj_pt_distance/normal.slvs",
	"constraint/pt_face_distance/normal.slvs",
	"constraint/pt_line_distance/normal.slvs",
	"constraint/pt_on_circle/normal.slvs",
	"constraint/pt_plane_distance/normal.slvs",
	"constraint/same_orientation/normal.slvs",
	"constraint/where_dragged/normal.slvs",
	"edit/tangent_arc.slvs",
	"group/boolean_knife_edge/normal.slvs",
	"group/force_to_mesh/union.slvs",
	"group/link/normal.slvs",
	"group/link_stl/normal.slvs",
	"group/translate_nd/normal.slvs",
	"request/arc_of_circle/normal.slvs",
	"request/circle/normal.slvs",
	"request/cubic/normal.slvs",
	"request/cubic_periodic/normal.slvs",
	"request/datum_point/normal.slvs",
	"request/image/linked.slvs",
	"request/image/normal.slvs",
	"request/line_segment/normal.slvs",
	"request/ttf_text/extrude.slvs",
	"request/ttf_text/normal.slvs",
	"request/workplane/normal.slvs",
}

// TestTextWindow shows each text window screen for the fixtures, and
// compares the rows, their text and each cell's colours and links with
// the golden (testdata/textwin, first recorded from C++).
func TestTextWindow(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	ttf.DefaultFonts = ttf.NewFontList([]string{filepath.Join(root, "Gentium-R.ttf")})
	for _, f := range textwinFixtures {
		t.Run(f, func(t *testing.T) {
			goldenPath := filepath.Join(root, "textwin", strings.TrimSuffix(f, ".slvs")+".textwin")
			got := textwinScreensFor(t, filepath.Join(root, f))
			if golden.Regen() {
				golden.Check(t, goldenPath, []byte(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("golden: %v", err)
			}
			compareTextwin(t, string(want), got)
		})
	}
}

// compareTextwin reports the first differing line, with the screen it
// is in.
func compareTextwin(t *testing.T, want, got string) {
	t.Helper()
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	screen := ""
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if strings.HasPrefix(w, "screen ") {
			screen = w
		}
		if w != g {
			t.Fatalf("%s, line %d:\n  want: %q\n   got: %q", screen, i+1, w, g)
		}
	}
}

// cellKey is the harness's CellKey.
func (tw *TextWindow) cellKey(r, c int) string {
	m := &tw.lines[r].meta[c]
	key := fmt.Sprintf("%02x %02x", m.fg, m.bg)
	if m.bg == 'z' {
		key += fmt.Sprintf(" %02x%02x%02x%02x", m.bgRgb.Red, m.bgRgb.Green, m.bgRgb.Blue, m.bgRgb.Alpha)
	}
	key += fmt.Sprintf(" %d", m.link)
	if m.link != 0 {
		b2i := func(f LinkFunction) int {
			if f != nil {
				return 1
			}
			return 0
		}
		key += fmt.Sprintf(" %x %d %d", m.data, b2i(m.f), b2i(m.h))
	}
	return key
}

// sameCellKey is cellKey(r, c0) == cellKey(r, c1), without formatting
// either.
func (tw *TextWindow) sameCellKey(r, c0, c1 int) bool {
	a, b := &tw.lines[r].meta[c0], &tw.lines[r].meta[c1]
	if a.fg != b.fg || a.bg != b.bg || a.link != b.link {
		return false
	}
	if a.bg == 'z' && a.bgRgb != b.bgRgb {
		return false
	}
	return a.link == 0 || (a.data == b.data && (a.f != nil) == (b.f != nil) && (a.h != nil) == (b.h != nil))
}

// dumpRows is the harness's DumpRows.
func (tw *TextWindow) dumpRows(b *strings.Builder) {
	for r := range tw.rows {
		row := &tw.lines[r]
		end := MaxCols
		for end > 0 && row.text[end-1] == ' ' {
			end--
		}
		var ascii strings.Builder
		for c := range end {
			if row.text[c] < 0x80 {
				ascii.WriteRune(row.text[c])
			} else {
				ascii.WriteByte('?')
			}
		}
		if s := ascii.String(); strings.Contains(s, "gl vendor") || strings.Contains(s, "   renderer ") ||
			strings.Contains(s, "   version  ") {
			fmt.Fprintf(b, "row %d (renderer)\n", row.top)
			continue
		}
		fmt.Fprintf(b, "row %d ", row.top)
		var buf [4]byte
		for c := range end {
			n := utf8.EncodeRune(buf[:], row.text[c])
			b.Write(buf[:n])
		}
		b.WriteByte('\n')
		c0 := 0
		for c0 < MaxCols {
			c1 := c0 + 1
			for c1 < MaxCols && tw.sameCellKey(r, c0, c1) {
				c1++
			}
			fmt.Fprintf(b, " %d-%d %s\n", c0, c1-1, tw.cellKey(r, c0))
			c0 = c1
		}
	}
}

// textwinScreensFor runs the harness's scenarios on a fixture.
func textwinScreensFor(t testing.TB, path string) string {
	p := headless.New()
	ss := New(p)
	ss.LoadSettings()
	ss.NewFile()
	ss.AfterNewFile()
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	ss.LoadSketchForTest(doc.Sketch, abs)
	sk := ss.SK
	tw := ss.TW
	gw := ss.GW
	v := &gw.view
	v.Scale = 5
	v.Offset = geom.Vector{}
	v.ProjRight = geom.Vector{X: 1}
	v.ProjUp = geom.Vector{Y: 1}
	for _, hg := range sk.GroupOrder {
		sk.Group(hg).GenerateDisplayItems(sk)
	}

	var b strings.Builder
	dump := func(name string) {
		tw.Show()
		fmt.Fprintf(&b, "screen %s\n", name)
		tw.dumpRows(&b)
	}
	showScreen := func(name string, screen Screen) {
		tw.shown.screen = screen
		dump(name)
	}
	selectItems := func(es []core.HEntity, cs []core.HConstraint) {
		gw.ClearSelection()
		for _, he := range es {
			gw.selection = append(gw.selection, Selection{Entity: he})
		}
		for _, hc := range cs {
			gw.selection = append(gw.selection, Selection{Constraint: hc})
		}
	}

	showScreen("groups", ScreenListOfGroups)

	groups := append([]core.HGroup(nil), sk.GroupOrder...)
	for _, hg := range groups {
		tw.shown.group = hg
		showScreen(fmt.Sprintf("group %x", hg.V), ScreenGroupInfo)
		tw.shown.group = hg
		showScreen(fmt.Sprintf("solve %x", hg.V), ScreenGroupSolveInfo)
	}

	for _, hg := range groups {
		g := sk.Group(hg)
		if g.Type != sketch.GroupExtrude {
			continue
		}
		saved := *g
		variants := []struct {
			typ        sketch.GroupType
			subtype    sketch.GroupSubtype
			valA, valB float64
		}{
			{sketch.GroupLathe, sketch.GroupOneSided, 0, 0},
			{sketch.GroupRevolve, sketch.GroupTwoSided, 0, 0},
			{sketch.GroupHelix, sketch.GroupOneSided, 0, 2.5},
			{sketch.GroupRotate, sketch.GroupOneSided, 3, 0},
			{sketch.GroupTranslate, sketch.GroupTwoSided, 1, 0},
			{sketch.GroupExtrude, sketch.GroupTwoSkewed, 0, 0},
		}
		for i, v := range variants {
			g.Type = v.typ
			g.Subtype = v.subtype
			g.ValA = v.valA
			g.ValB = v.valB
			g.SkipFirst = i%2 == 1
			g.BooleanFailed = i == 5
			tw.shown.group = hg
			showScreen(fmt.Sprintf("variant %x %d", hg.V, i), ScreenGroupInfo)
		}
		*g = saved
	}

	showScreen("configuration", ScreenConfiguration)
	showScreen("styles", ScreenListOfStyles)
	var styles []core.HStyle
	for i := range sk.Styles {
		styles = append(styles, sk.Styles[i].H)
	}
	for _, hs := range styles {
		tw.shown.style = hs
		showScreen(fmt.Sprintf("style %x", hs.V), ScreenStyleInfo)
	}
	showScreen("view", ScreenEditView)
	showScreen("tangent", ScreenTangentArc)
	ss.TangentArcManual = !ss.TangentArcManual
	showScreen("tangent manual", ScreenTangentArc)
	ss.TangentArcManual = !ss.TangentArcManual

	tw.shown.paste.times = 3
	tw.shown.paste.trans = geom.Vector{X: 1.5, Y: -2}
	tw.shown.paste.theta = 0.5
	tw.shown.paste.origin = geom.Vector{X: 0.25, Y: 0.75}
	tw.shown.paste.scale = -1.5
	showScreen("paste", ScreenPasteTransformed)

	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if !c.HasLabel() || c.Reference {
			continue
		}
		tw.stepDim.finish = c.ValA * 2
		tw.stepDim.steps = 10
		switch c.Type {
		case core.ConstraintAngle, core.ConstraintLengthRatio, core.ConstraintArcArcLenRatio,
			core.ConstraintArcLineLenRatio, core.ConstraintLengthDifference,
			core.ConstraintArcArcDifference, core.ConstraintArcLineDifference:
			tw.stepDim.isDistance = false
		default:
			tw.stepDim.isDistance = true
		}
		tw.shown.constraint = c.H
		tw.shown.screen = ScreenStepDimension
		tw.Show()
		for r := range tw.lines {
			for col := range MaxCols {
				tw.lines[r].meta[col].data = 0
			}
		}
		b.WriteString("screen stepdim\n")
		tw.dumpRows(&b)
		break
	}
	tw.shown.screen = ScreenListOfGroups

	gw.pending.operation = PendingCommand
	gw.pending.description = "click first point of line segment"
	dump("pending")
	gw.pending.operation = PendingNone
	gw.pending.description = ""

	var entities []core.HEntity
	for i := range sk.Entities {
		entities = append(entities, sk.Entities[i].H)
	}
	const maxEntities = 80
	for i := 0; i < len(entities) && i < maxEntities; i++ {
		selectItems([]core.HEntity{entities[i]}, nil)
		dump(fmt.Sprintf("entity %x", entities[i].V))
	}
	for i := 0; i+1 < len(entities) && i < maxEntities; i++ {
		selectItems([]core.HEntity{entities[i], entities[i+1]}, nil)
		dump(fmt.Sprintf("entities %x %x", entities[i].V, entities[i+1].V))
	}
	for i := 0; i+2 < len(entities) && i < maxEntities; i++ {
		selectItems([]core.HEntity{entities[i], entities[i+1], entities[i+2]}, nil)
		dump(fmt.Sprintf("entities %x %x %x", entities[i].V, entities[i+1].V, entities[i+2].V))
	}
	var constraints []core.HConstraint
	for i := range sk.Constraints {
		constraints = append(constraints, sk.Constraints[i].H)
	}
	for _, hc := range constraints {
		selectItems(nil, []core.HConstraint{hc})
		dump(fmt.Sprintf("constraint %x", hc.V))
	}

	hs := core.HStyle{V: ss.CreateCustomStyle(false)}
	sk.StyleGet(hs).Name = "custom"
	sk.StyleGet(hs).TextOrigin = core.TextOriginLeft | core.TextOriginTop
	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Type == sketch.RequestWorkplane {
			continue
		}
		r.Style = hs
		selectItems([]core.HEntity{r.H.Entity(0)}, nil)
		break
	}
	tw.shown.style = hs
	showScreen("custom style", ScreenStyleInfo)
	gw.ClearSelection()
	showScreen("custom style info", ScreenStyleInfo)
	showScreen("styles with custom", ScreenListOfStyles)

	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Reference || c.Type == core.ConstraintComment {
			continue
		}
		dup := *c
		dup.H = core.HConstraint{}
		sk.AddConstraintRemember(&dup, false)
		ss.GenerateAll(sketch.GenerateAll)
		showScreen("failed groups", ScreenListOfGroups)
		tw.shown.group = dup.Group
		showScreen("failed solve", ScreenGroupSolveInfo)
		break
	}
	return b.String()
}
