package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// textwinFixtures are the models TestTextWindow shows the screens of,
// by their paths from testdata; a golden is named for the model's path
// in the corpus, testdata/tlspr/golden.
var textwinFixtures = []string{
	"tlspr/golden/analysis/contour_area/normal.tlspr",
	"tlspr/golden/constraint/angle/normal.tlspr",
	"tlspr/golden/constraint/comment/normal.tlspr",
	"tlspr/golden/constraint/diameter/normal.tlspr",
	"tlspr/golden/constraint/diameter/reference.tlspr",
	"tlspr/golden/constraint/length_ratio/normal.tlspr",
	"tlspr/golden/constraint/parallel/normal.tlspr",
	"tlspr/golden/constraint/proj_pt_distance/normal.tlspr",
	"tlspr/golden/constraint/pt_face_distance/normal.tlspr",
	"tlspr/golden/constraint/pt_line_distance/normal.tlspr",
	"tlspr/golden/constraint/pt_on_circle/normal.tlspr",
	"tlspr/golden/constraint/pt_plane_distance/normal.tlspr",
	"tlspr/golden/constraint/same_orientation/normal.tlspr",
	"tlspr/golden/constraint/where_dragged/normal.tlspr",
	"modify/tangent_arc.tlspr",
	"tlspr/golden/group/boolean_knife_edge/normal.tlspr",
	"tlspr/golden/group/force_to_mesh/union.tlspr",
	"tlspr/golden/group/link/normal.tlspr",
	"tlspr/golden/group/link_stl/normal.tlspr",
	"tlspr/golden/group/translate_nd/normal.tlspr",
	"tlspr/golden/request/arc_of_circle/normal.tlspr",
	"tlspr/golden/request/circle/normal.tlspr",
	"tlspr/golden/request/cubic/normal.tlspr",
	"tlspr/golden/request/cubic_periodic/normal.tlspr",
	"tlspr/golden/request/datum_point/normal.tlspr",
	"tlspr/golden/request/image/linked.tlspr",
	"tlspr/golden/request/image/normal.tlspr",
	"tlspr/golden/request/line_segment/normal.tlspr",
	"tlspr/golden/request/ttf_text/extrude.tlspr",
	"tlspr/golden/request/ttf_text/normal.tlspr",
	"tlspr/golden/request/workplane/normal.tlspr",
}

// TestTextWindow shows each text window screen for the fixtures, and
// compares the rows, their text and each cell's colours and links with
// the golden (testdata/textwin).
func TestTextWindow(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata")
	for _, f := range textwinFixtures {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			name := strings.TrimSuffix(strings.TrimPrefix(f, "tlspr/golden/"), ".tlspr")
			goldenPath := filepath.Join(root, "textwin", name+".textwin")
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

// cellKey names a cell's colours and link, for the dump.
func (tw *textWindow) cellKey(r, c int) string {
	m := &tw.lines[r].meta[c]
	key := fmt.Sprintf("%02x %02x", m.fg, m.bg)
	if m.bg == 'z' {
		key += fmt.Sprintf(" %02x%02x%02x%02x", m.bgRgb.Red, m.bgRgb.Green, m.bgRgb.Blue, m.bgRgb.Alpha)
	}
	key += fmt.Sprintf(" %d", m.link)
	if m.link != 0 {
		b2i := func(f linkFunction) int {
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
func (tw *textWindow) sameCellKey(r, c0, c1 int) bool {
	a, b := &tw.lines[r].meta[c0], &tw.lines[r].meta[c1]
	if a.fg != b.fg || a.bg != b.bg || a.link != b.link {
		return false
	}
	if a.bg == 'z' && a.bgRgb != b.bgRgb {
		return false
	}
	return a.link == 0 || (a.data == b.data && (a.f != nil) == (b.f != nil) && (a.h != nil) == (b.h != nil))
}

// dumpRows writes the rows of the text window, with their cells.
func (tw *textWindow) dumpRows(b *strings.Builder) {
	for r := range tw.rows {
		row := &tw.lines[r]
		end := maxCols
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
		for c0 < maxCols {
			c1 := c0 + 1
			for c1 < maxCols && tw.sameCellKey(r, c0, c1) {
				c1++
			}
			fmt.Fprintf(b, " %d-%d %s\n", c0, c1-1, tw.cellKey(r, c0))
			c0 = c1
		}
	}
}

// textwinScreensFor shows each screen of the text window for a model
// and dumps them.
func textwinScreensFor(t testing.TB, path string) string {
	p := headless.New()
	ss := New(p)
	ss.LoadSettings()
	// The failed-solve screens list what the search for the constraints
	// to remove found before its time ran out; under the race detector,
	// with every package's tests running, a second is not enough.
	ss.findToFixTimeout = time.Hour
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ss.Load(abs); err != nil {
		t.Fatal(err)
	}
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

	showScreen("groups", screenListOfGroups)

	groups := append([]core.HGroup(nil), sk.GroupOrder...)
	for _, hg := range groups {
		tw.shown.group = hg
		showScreen(fmt.Sprintf("group %x", hg.V), screenGroupInfo)
		tw.shown.group = hg
		showScreen(fmt.Sprintf("solve %x", hg.V), screenGroupSolveInfo)
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
			showScreen(fmt.Sprintf("variant %x %d", hg.V, i), screenGroupInfo)
		}
		*g = saved
	}

	showScreen("configuration", screenConfiguration)
	showScreen("styles", screenListOfStyles)
	var styles []core.HStyle
	for i := range sk.Styles {
		styles = append(styles, sk.Styles[i].H)
	}
	for _, hs := range styles {
		tw.shown.style = hs
		showScreen(fmt.Sprintf("style %x", hs.V), screenStyleInfo)
	}
	showScreen("view", screenEditView)
	showScreen("tangent", screenTangentArc)
	ss.TangentArcManual = !ss.TangentArcManual
	showScreen("tangent manual", screenTangentArc)
	ss.TangentArcManual = !ss.TangentArcManual

	tw.shown.paste.times = 3
	tw.shown.paste.trans = geom.Vector{X: 1.5, Y: -2}
	tw.shown.paste.theta = 0.5
	tw.shown.paste.origin = geom.Vector{X: 0.25, Y: 0.75}
	tw.shown.paste.scale = -1.5
	showScreen("paste", screenPasteTransformed)

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
		tw.shown.screen = screenStepDimension
		tw.Show()
		for r := range tw.lines {
			for col := range maxCols {
				tw.lines[r].meta[col].data = 0
			}
		}
		b.WriteString("screen stepdim\n")
		tw.dumpRows(&b)
		break
	}
	tw.shown.screen = screenListOfGroups

	gw.pending.operation = pendingCommand
	gw.pending.description = "click first point of line segment"
	dump("pending")
	gw.pending.operation = pendingNone
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
	showScreen("custom style", screenStyleInfo)
	gw.ClearSelection()
	showScreen("custom style info", screenStyleInfo)
	showScreen("styles with custom", screenListOfStyles)

	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Reference || c.Type == core.ConstraintComment {
			continue
		}
		dup := *c
		dup.H = core.HConstraint{}
		sk.AddConstraintRemember(&dup, false)
		ss.GenerateAll(sketch.GenerateAll)
		showScreen("failed groups", screenListOfGroups)
		tw.shown.group = dup.Group
		showScreen("failed solve", screenGroupSolveInfo)
		break
	}
	return b.String()
}
