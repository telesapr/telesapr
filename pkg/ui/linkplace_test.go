package ui

import (
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// TestRotateImportedPaths turns a linked part a quarter through
// Edit > Rotate Imported 90° and through LinkGroup.Rotate90, from the
// view's direction, and requires the same placement; the window's turn
// is one step of undo.
func TestRotateImportedPaths(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	g, err := doc.Assembly.Link(filepath.Join("..", "..", "testdata", "step", "shapes", "box.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	l := g.AsLink()
	l.Move(api.V(3, 0, 0))
	ss.UndoRemember()

	// A view from the side, so that the axis is not z.
	v := &ss.GW.view
	v.ProjRight = api.V(0, 1, 0)
	v.ProjUp = api.V(0, 0, 1)
	ss.GW.NormalizeProjectionVectors()
	normal := v.ProjRight.Cross(v.ProjUp)

	ss.GW.rotateImported90()
	uWin, wWin := l.Orientation()
	if uWin.Minus(api.V(1, 0, 0)).Magnitude() < 0.5 && wWin.Minus(api.V(0, 1, 0)).Magnitude() < 0.5 {
		t.Fatal("the window did not turn the part")
	}
	ss.UndoUndo()
	if u, w := l.Orientation(); u.Minus(api.V(1, 0, 0)).Magnitude() > 1e-9 || w.Minus(api.V(0, 1, 0)).Magnitude() > 1e-9 {
		t.Errorf("undo left the part turned: %v %v", u, w)
	}

	l.Rotate90(normal)
	uScr, wScr := l.Orientation()
	if uWin.Minus(uScr).Magnitude() > 1e-12 || wWin.Minus(wScr).Magnitude() > 1e-12 {
		t.Errorf("the window turned the part to %v %v, the script to %v %v", uWin, wWin, uScr, wScr)
	}
	if o := l.Offset(); o.Minus(api.V(3, 0, 0)).Magnitude() > 1e-12 {
		t.Errorf("the turn moved the part to %v", o)
	}
}
