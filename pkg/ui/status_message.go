package ui

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
)

// statusMargin is how far, in pixels, a status message keeps from the
// edges of the window's free area.
const statusMargin = 10

// statusLine is one line of a status message and where its baseline
// starts, in the camera's pixels from the middle, y up.
type statusLine struct {
	Text string
	At   geom.Point2d
}

// drawStatusMessage writes msg in the top left corner of the part of
// the window the toolbar leaves free (freeArea), wrapped to its width.
// Drawn beside what it is about, a message lay across the model, which
// may be anywhere in the window; the corner is clear of it unless the
// model is zoomed to fill the window, and is where the eye looks for it.
func (gw *graphicsWindow) drawStatusMessage(msg string, hcs render.HStroke, canvas render.Canvas) {
	camera := canvas.Camera()
	h := gw.ss.defaultTextHeight()
	for _, l := range statusLines(msg, h, gw.freeArea(camera)) {
		canvas.DrawVectorText(l.Text, h/camera.Scale, camera.UnProjectPoint(l.At),
			camera.ProjRight, camera.ProjUp, hcs)
	}
}

// statusLines breaks msg, in text of cap height h pixels, into lines
// that fit area less its margins, and places them down from its top
// left corner.
func statusLines(msg string, h float64, area screenArea) []statusLine {
	vf := res.BuiltinVectorFont()
	avail := area.Width - 2*statusMargin
	// The vector font is not monospaced: wrap at the message's mean
	// character width, which is near enough for words.
	n := len([]rune(msg))
	chars := n
	if w := vf.Width(h, msg); w > avail && n > 0 {
		chars = max(10, int(avail/(w/float64(n))))
	}
	left := area.Mid.U - area.Width/2 + statusMargin
	top := area.Mid.V + area.Height/2 - statusMargin
	step := vf.Height(h) * 1.3
	var lines []statusLine
	for i, text := range wrapWords(msg, max(chars, 1)) {
		lines = append(lines, statusLine{
			Text: text,
			At:   geom.Point2d{U: left, V: top - h - float64(i)*step},
		})
	}
	return lines
}
