package tkgui

import (
	"github.com/takigo/takigo/geometry/pack"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/scrollbar"
)

// scrollbarAdapter gives a Tk scrollbar, which works in fractions,
// GTK's adjustment: a value between min and max-pageSize, stepped by 1
// and paged by 4 (C++ ConfigureScrollbar).
type scrollbarAdapter struct {
	w                  *Window
	sb                 *scrollbar.Scrollbar
	visible            bool
	min, max, pageSize float64
	value              float64
}

func newScrollbar(w *Window) *scrollbarAdapter {
	s := &scrollbarAdapter{w: w, max: 1, pageSize: 1}
	s.sb = scrollbar.New(w.top, w.p.newName("scroll"), scrollbar.OrientOpt(scrollbar.Vertical),
		scrollbar.CommandOpt(s.command))
	return s
}

func (s *scrollbarAdapter) pack() {
	if s.visible {
		_ = pack.Pack(s.sb, pack.SideOpt(pack.Right), pack.FillOpt(pack.FillY))
	}
}

func (s *scrollbarAdapter) clamp(v float64) float64 {
	v = min(v, s.max-s.pageSize)
	return max(v, s.min)
}

func (s *scrollbarAdapter) update() {
	span := s.max - s.min
	if span <= 0 {
		s.sb.Set(0, 1)
		return
	}
	s.sb.Set((s.value-s.min)/span, (s.value+s.pageSize-s.min)/span)
}

func (s *scrollbarAdapter) command(r widget.ScrollRequest) {
	v := s.value
	switch {
	case r.MoveTo:
		v = s.min + r.Fraction*(s.max-s.min)
	case r.Pages:
		v += float64(r.Count) * 4
	default:
		v += float64(r.Count)
	}
	v = s.clamp(v)
	if v == s.value {
		return
	}
	s.value = v
	s.update()
	if s.w.events.OnScrollbarAdjusted != nil {
		s.w.events.OnScrollbarAdjusted(v)
	}
}

// SetScrollbarVisible shows or hides the scrollbar.
func (w *Window) SetScrollbarVisible(v bool) {
	s := w.scrollbar
	if v == s.visible {
		return
	}
	s.visible = v
	if v {
		// The scrollbar must be packed before the canvas to get its edge.
		pack.Forget(w.canvas)
		s.pack()
		w.canvas.pack()
	} else {
		pack.Forget(s.sb)
	}
}

// ConfigureScrollbar sets the range and page size, keeping the value.
func (w *Window) ConfigureScrollbar(lo, hi, pageSize float64) {
	s := w.scrollbar
	s.min, s.max, s.pageSize = lo, hi, pageSize
	s.value = s.clamp(s.value)
	s.update()
}

// ScrollbarPosition is the scrollbar's value.
func (w *Window) ScrollbarPosition() float64 { return w.scrollbar.value }

// SetScrollbarPosition sets the value, reporting it as GTK does.
func (w *Window) SetScrollbarPosition(pos float64) {
	s := w.scrollbar
	pos = s.clamp(pos)
	if pos == s.value {
		return
	}
	s.value = pos
	s.update()
	if w.events.OnScrollbarAdjusted != nil {
		w.events.OnScrollbarAdjusted(pos)
	}
}
