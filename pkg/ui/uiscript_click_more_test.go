package ui

import (
	"fmt"
	"strings"
	"testing"
)

// TestClickByName checks the scenario commands that click by name: an
// entity by handle and by name picks it, a text window link by its text
// lands on that link, and a name that finds nothing says so.
func TestClickByName(t *testing.T) {
	t.Parallel()
	r := newUIScriptRunner(t)
	for _, line := range []string{"file $testdata/ui/sweep_sharp_arc.tlspr", "menu File|Open...", "view other",
		"scroll 296 262 3"} {
		r.step(line)
		r.settle()
	}
	sk := r.ss.SK
	arc := r.findEntity("50000")
	if arc == nil {
		t.Fatal("no entity 50000 in sweep_sharp_arc.tlspr")
	}
	r.b.Reset()
	r.step("click-entity 40000")
	r.settle()
	r.step("click-entity " + sk.EntityName(arc.H))
	r.settle()
	var got []string
	for _, s := range r.ss.GW.selection {
		got = append(got, fmt.Sprintf("%x", s.Entity.V))
	}
	if out := r.b.String(); out != "" {
		t.Errorf("clicking by name wrote %q", out)
	}
	if len(got) != 2 || r.ss.GW.selection[0].Entity.V != 0x40000 || r.ss.GW.selection[1].Entity.V != 0x50000 {
		t.Errorf("selection after clicking 40000 and %q: %v", sk.EntityName(arc.H), got)
	}

	// Every link the text window shows is found where it is drawn.
	tw := r.ss.TW
	seen := map[string]int{}
	for row := range tw.rows {
		line, meta := tw.lines[row].text[:], tw.lines[row].meta[:]
		for c0 := 0; c0 < len(line); {
			c1 := c0 + 1
			for c1 < len(line) && meta[c1].link == meta[c0].link && meta[c1].data == meta[c0].data {
				c1++
			}
			if text := linkText(line[c0:c1]); meta[c0].link != 0 && text != "" {
				seen[text]++
				x, y, ok := r.twLinkPoint(text, seen[text])
				if !ok {
					t.Errorf("link %q #%d not found", text, seen[text])
				} else if hr, hc := twCellAt(tw, x, y); hr != row || hc < c0 || hc >= c1 {
					t.Errorf("link %q #%d at row %d cols %d-%d; found at row %d col %d", text, seen[text],
						row, c0, c1-1, hr, hc)
				}
			}
			c0 = c1
		}
	}
	if len(seen) == 0 {
		t.Fatal("the text window shows no links")
	}

	r.b.Reset()
	r.step("click-entity nothing-of-that-name")
	r.step("click-tw no such link")
	if out := r.b.String(); !strings.Contains(out, "entity missing nothing-of-that-name") ||
		!strings.Contains(out, "click-tw missing no such link") {
		t.Errorf("names that find nothing: %q", out)
	}
}

// twCellAt is the row and column the text window takes a point at, as
// its MouseEvent does.
func twCellAt(tw *textWindow, x, y float64) (row, col int) {
	col = int((x - leftMargin) / charWidth)
	hh := float64(lineHeight / 2)
	y += float64(tw.scrollPos) * hh
	for r := range tw.rows {
		if y >= float64(tw.lineTop(r))*hh && y <= float64(tw.lineTop(r)+2)*hh {
			return r, col
		}
	}
	return -1, col
}

// TestClickSolidByName checks click-edge and click-face: an edge of the
// solid no entity draws, and faces with and without an entity, are
// selected by their names, shift adds to the selection, and names that
// find nothing say so.
func TestClickSolidByName(t *testing.T) {
	t.Parallel()
	r := newUIScriptRunner(t)
	for _, line := range []string{"file $testdata/topo/slot.tlspr", "menu File|Open...", "view rotated",
		"menu View|Zoom To Fit"} {
		r.step(line)
		r.settle()
	}
	sk := r.ss.SK
	const edge = "the edge between side face of g003-block from r005-line-segment and " +
		"side face of g005-slot from r00a-line-segment"
	r.b.Reset()
	r.step("click-edge " + edge)
	r.settle()
	sel := r.ss.GW.selection
	if len(sel) != 1 || sel[0].Edge.A == 0 || sk.SolidEdgeName(sel[0].Edge) != edge {
		t.Fatalf("click-edge selected %+v", sel)
	}
	r.step("click-face top face of g003-block shift")
	r.settle()
	sel = r.ss.GW.selection
	if len(sel) != 2 || sk.EntityName(sel[1].Entity) != "top face of g003-block" {
		t.Errorf("click-face of an entity's face, shift, selected %+v", sel)
	}
	if out := r.b.String(); out != "" {
		t.Errorf("clicking the solid by name wrote %q", out)
	}

	r.b.Reset()
	r.step("click-edge the edge between nothing and nothing")
	r.step("move-face no face")
	if out := r.b.String(); !strings.Contains(out, "edge missing the edge between nothing and nothing") ||
		!strings.Contains(out, "face missing no face") {
		t.Errorf("names that find nothing: %q", out)
	}
	if got := len(r.ss.GW.selection); got != 2 {
		t.Errorf("a name that finds nothing changed the selection to %d items", got)
	}
}

// TestClickSolidFaceByID checks click-face on a face no entity names,
// by its number in the group that made it: the side of an extruded
// circle, as ext_thread_face picks it.
func TestClickSolidFaceByID(t *testing.T) {
	t.Parallel()
	r := newUIScriptRunner(t)
	for _, line := range []string{"key c", "move 300 300", "down L 300 300", "up L 300 300",
		"move 350 300", "down L 350 300", "up L 350 300", "key esc", "key x shift", "view rotated"} {
		r.step(line)
		r.settle()
	}
	const name = "face 2 of g003-extrude"
	r.b.Reset()
	r.step("click-face " + name)
	r.settle()
	if out := r.b.String(); out != "" {
		t.Fatalf("click-face %s wrote %q", name, out)
	}
	sel := r.ss.GW.selection
	if len(sel) != 1 || sel[0].Face == 0 || r.ss.SK.FaceName(sel[0].Face) != name {
		t.Errorf("click-face %s selected %+v", name, sel)
	}
}
