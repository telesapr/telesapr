package ui

import (
	"github.com/telesapr/telesapr/pkg/thread"
)

// The thread screen's [choose]: the sizes of a series and the pitches
// of the size chosen, to pick from rather than type.

// threadPickerColumns is how wide the text window's rows of sizes are,
// in characters.
const threadPickerColumns = 84

// showThreadPicker lists the series, the sizes of the series shown,
// and the standard pitches of the thread's size.
func (tw *textWindow) showThreadPicker() {
	t := &tw.shown.thread
	tw.Printf(true, "%Ba   %Ftthread%E    %s", t.designation)
	line, args := "", []any{}
	for _, sr := range thread.AllSeries() {
		line += "  %f%LS%D%Fd%s %s%E"
		args = append(args, lf(tw.ScreenPickThread), int(sr), radioMark(sr == t.series), sr.String())
	}
	tw.Printf(true, "%Bd   %Ftseries%E "+line, args...)

	sizes := t.series.Sizes()
	width := 2
	for _, s := range sizes {
		width = max(width, len([]rune(s.String()))+4)
	}
	perRow := max(threadPickerColumns/width, 1)
	for row := 0; row*perRow < len(sizes); row++ {
		line, args = "", []any{byte('a' + row%2*3)}
		for i := row * perRow; i < min((row+1)*perRow, len(sizes)); i++ {
			line += "%f%LZ%D%Fd%s %s%E"
			args = append(args, lf(tw.ScreenPickThread), i, radioMark(sameSize(sizes[i], t.options.Spec)),
				pad(sizes[i].String(), width-2))
		}
		label := "         "
		if row == 0 {
			label = "%Ftsize%E     "
		}
		tw.Printf(row == 0, "%Bp   "+label+line, args...)
	}

	pitches := thread.Pitches(t.options.Spec)
	if len(pitches) == 0 {
		tw.Printf(true, "%Bd   %Ftpitch%E    %Fd(no standard pitches for %s)%E", t.designation)
	} else {
		line, args = "", []any{}
		for i, p := range pitches {
			line += "  %f%LP%D%Fd%s %s%E"
			args = append(args, lf(tw.ScreenPickThread), i, radioMark(p.Pitch == t.options.Spec.Pitch), p.String())
		}
		tw.Printf(true, "%Bd   %Ftpitch%E  "+line, args...)
	}
	tw.Printf(true, " %Fl%LB%fback to the thread%E", lf(tw.ScreenPickThread))
}

// sameSize reports whether two threads are of the same form and size.
func sameSize(a, b thread.Spec) bool {
	return a.Form == b.Form && a.Diameter == b.Diameter
}

// ScreenPickThread takes a series, a size or a pitch from the list of
// sizes, or goes back to the thread. A size or a pitch keeps the
// thread's starts and hand.
func (tw *textWindow) ScreenPickThread(link int, v uint32) {
	t := &tw.shown.thread
	take := func(s thread.Spec) {
		s.Starts, s.LeftHand = t.options.Spec.Starts, t.options.Spec.LeftHand
		t.options.Spec, t.designation, t.note = s, s.String(), ""
	}
	switch link {
	case 'S':
		if all := thread.AllSeries(); int(v) < len(all) {
			t.series = all[v]
		}
	case 'Z':
		if sizes := t.series.Sizes(); int(v) < len(sizes) {
			take(sizes[v])
		}
	case 'P':
		if pitches := thread.Pitches(t.options.Spec); int(v) < len(pitches) {
			take(pitches[v])
		}
	case 'B':
		t.picking = false
	}
	t.keepClass()
}
