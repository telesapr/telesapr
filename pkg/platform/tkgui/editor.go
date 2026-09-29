package tkgui

import (
	"fmt"

	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/geometry/place"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/widget/entry"

	"github.com/spacesolve/spacesolve/pkg/platform"
)

// editor is the text entry shown over the drawing area to edit a
// dimension or a value (C++ GtkEditorOverlay).
type editor struct {
	e       *entry.Entry
	font    string
	visible bool
}

func (w *Window) editorVisible() bool { return w.editor != nil && w.editor.visible }

// IsEditorVisible reports whether the editor is shown.
func (w *Window) IsEditorVisible() bool { return w.editorVisible() }

// ShowEditor shows the editor with its text's baseline at x, y, in a
// font of fontHeight pixels (C++ start_editing).
func (w *Window) ShowEditor(x, y, fontHeight, minWidth float64, isMonospace bool, text string) {
	app := w.p.app
	family := "sans-serif"
	if isMonospace {
		family = "monospace"
	}
	desc := fmt.Sprintf("{%s} %d", family, -max(int(fontHeight), 1))
	if w.editor == nil || w.editor.font != desc {
		if w.editor != nil {
			place.Forget(w.editor.e)
			w.editor.e.Destroy()
		}
		ed := &editor{font: desc}
		ed.e = entry.New(w.canvas, w.p.newName("editor"), entry.FontOpt(desc), entry.BorderWidth(1))
		app.Dispatcher().Bind(ed.e.Window().PlatformID, event.KeyPressMask, func(ev *event.Event) {
			if ev.Type != event.KeyPressType {
				return
			}
			switch ev.KeySym {
			case tkplatform.XK_Return, 0xff8d:
				if w.events.OnEditingDone != nil {
					w.events.OnEditingDone(ed.e.GetText())
				}
			case tkplatform.XK_Escape:
				// Escape goes to the drawing area, which cancels.
				w.key(platform.KeyPress, ev)
			}
		})
		w.editor = ed
	}
	ed := w.editor

	// The y coordinate denotes baseline.
	if f, err := app.FontRegistry().Get(desc); err == nil {
		y -= float64(f.Metrics().Ascent)
		// One extra character's width to avoid scrolling.
		width := float64(f.MeasureString(text+" ")) + 4
		minWidth = max(minWidth, width)
	}
	ed.e.SetText(text)
	// GTK selects an entry's text when it takes the focus, so typing
	// replaces it.
	ed.e.SelectAll()
	place.Place(ed.e, place.In(w.canvas), place.X(int(x)-2), place.Y(int(y)-2), place.Width(int(minWidth)))
	ed.visible = true
	app.FocusManager().SetFocus(ed.e.Window())
}

// HideEditor hides the editor and gives the keyboard back to the
// drawing area.
func (w *Window) HideEditor() {
	if !w.editorVisible() {
		return
	}
	place.Forget(w.editor.e)
	w.editor.visible = false
	w.p.app.FocusManager().SetFocus(w.canvas.Win)
}
