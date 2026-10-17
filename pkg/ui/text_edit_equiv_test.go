package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// buildTextDoc is a text with a locked corner, made by script on a new
// application.
func buildTextDoc(t *testing.T) (*Application, api.Entity) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	text := doc.Sketch.Text("Abc", "", api.V(5, 5, 0), 10)
	doc.Constrain.Lock(text.Points()[1])
	return ss, text
}

// The text window's [change], apply kerning and font links and the
// script's SetText, SetKerning and SetFont reach the same Sketch
// calls: the files they save are the same, and the window's edits are
// each one undo step that redo brings back.
func TestTextEditsMatchScript(t *testing.T) {
	win, wtext := buildTextDoc(t)
	hr := wtext.Request().H
	fonts := win.Fonts().List()
	font := -1
	for i := range fonts {
		if fonts[i].FontFileBaseName() == ttf.EmbeddedFont {
			font = i
		}
	}
	if font < 0 {
		t.Fatal("the built-in font is not listed")
	}

	win.TW.ScreenEditTtfText(0, hr.V)
	win.TW.EditControlDone("Hello")
	win.TW.ScreenToggleTtfKerning(0, hr.V)
	win.GW.MakeSelectedEntity(wtext.H)
	win.TW.ScreenSetTtfFont(0, uint32(font))
	if r := win.SK.Request(hr); r.Str != "Hello" || r.ExtraPoints != 1 || r.Font != ttf.EmbeddedFont {
		t.Fatalf("the window left %q, kerning %d, font %q", r.Str, r.ExtraPoints, r.Font)
	}

	scr, stext := buildTextDoc(t)
	stext.SetText("Hello")
	stext.SetKerning(true)
	stext.SetFont(ttf.EmbeddedFont)

	got := string(tlspr.Encode(win.SK, tlspr.Options{}))
	want := string(tlspr.Encode(scr.SK, tlspr.Options{}))
	if got != want {
		t.Errorf("the window and the script save different files, at %s", firstDiffLine(got, want))
	}

	// Each window edit is one undo step, and redo brings it back.
	win.UndoUndo()
	if r := win.SK.Request(hr); r.Str != "Hello" || r.Font != ttf.EmbeddedFont {
		t.Errorf("one undo left %q in %q", r.Str, r.Font)
	}
	win.UndoUndo()
	if r := win.SK.Request(hr); r.Str != "Hello" || r.ExtraPoints != 0 {
		t.Errorf("two undos left %q, kerning %d", r.Str, r.ExtraPoints)
	}
	win.UndoUndo()
	if r := win.SK.Request(hr); r.Str != "Abc" {
		t.Errorf("three undos left %q", r.Str)
	}
	win.UndoRedo()
	win.UndoRedo()
	win.UndoRedo()
	if r := win.SK.Request(hr); r.Str != "Hello" || r.ExtraPoints != 1 {
		t.Errorf("redo left %q, kerning %d", r.Str, r.ExtraPoints)
	}
}
