package ui

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/thread"
)

// The thread screen's class [choose]: the tolerance classes of the
// thread's form and side, to pick from, or none for a clearance typed.

// shownClearance is the clearance the thread is made with: its class's,
// or the one typed when it has none or the class doesn't fit the thread
// as it now is (which making it then reports).
func (t *threadScreen) shownClearance() float64 {
	c, err := t.options.FitClearance()
	if err != nil {
		return t.options.Clearance
	}
	return c
}

// showThreadClasses lists the classes a thread of its form and side
// takes, the default first.
func (tw *textWindow) showThreadClasses() {
	ss := tw.ss
	t := &tw.shown.thread
	o := &t.options
	tw.Printf(true, "%Ba   %Ftthread%E    %s, %s", t.designation, map[bool]string{false: "external", true: "internal"}[o.Internal])
	classes := threadClasses(o.Spec, o.Internal)
	if len(classes) == 0 {
		tw.Printf(true, "%Bd   %Fd(a thread of this form has no classes; give its clearance)%E")
	} else {
		line, args := "", []any{}
		for i, c := range classes {
			line += "  %f%LC%D%Fd%s %s%E"
			args = append(args, lf(tw.ScreenPickThreadClass), i, radioMark(c == o.Class), c)
		}
		tw.Printf(true, "%Bd   %Ftclass%E "+line, args...)
	}
	tw.Printf(false, "%Ba   %f%LN%Fd%s none: the clearance typed, %s%E", lf(tw.ScreenPickThreadClass),
		radioMark(o.Class == ""), ss.MmToString(o.Clearance, true))
	tw.Printf(true, " %Fl%LB%fback to the thread%E", lf(tw.ScreenPickThreadClass))
}

// threadClasses are the classes of a thread of s, the default first.
func threadClasses(s thread.Spec, internal bool) []string {
	def := thread.DefaultClass(s, internal)
	if def == "" {
		return nil
	}
	out := []string{def}
	for _, c := range thread.Classes(s, internal) {
		if c != def {
			out = append(out, c)
		}
	}
	return out
}

// ScreenPickThreadClass takes a class, or none, or goes back to the
// thread.
func (tw *textWindow) ScreenPickThreadClass(link int, v uint32) {
	t := &tw.shown.thread
	switch link {
	case 'C':
		if classes := threadClasses(t.options.Spec, t.options.Internal); int(v) < len(classes) {
			t.options.Class = classes[v]
		}
	case 'N':
		t.options.Class = ""
	case 'B':
		t.pickingClass = false
	}
}

// keepClass follows a change of the thread's form or side with its
// class: one the thread no longer takes becomes the default of its
// form and side (none for a form without classes), as 6g becomes 6H
// when the thread is made internal.
func (t *threadScreen) keepClass() {
	o := &t.options
	if o.Class == "" || slices.Contains(thread.Classes(o.Spec, o.Internal), o.Class) {
		return
	}
	o.Class = thread.DefaultClass(o.Spec, o.Internal)
}
