package ui

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// The thread screen follows the paste-transformed one (clipboard.go).

// threadScreen is what the screw thread screen shows and makes, or,
// when edit is a thread's helix group, what it makes that thread again
// with.
type threadScreen struct {
	circle      core.HEntity
	designation string
	options     sketch.ThreadOptions
	edit        core.HGroup
}

// MenuThread starts a screw thread on the selected circle or arc: it
// fills the thread screen in from the circle and shows it.
func (ss *Application) MenuThread() {
	gw := ss.GW
	sk := ss.SK
	gw.GroupSelection()
	gs := &gw.gs
	if gs.circlesOrArcs != 1 || gs.n != 1 {
		ss.Error("Bad selection for new screw thread. This group can " +
			"be created with:\n\n" +
			"    * a circle or an arc (thread about its axis, from its plane)\n")
		return
	}
	circle := sk.MustEntity(gs.entity[0])
	d := 2 * circle.CircleGetRadiusNum(sk)
	spec := thread.Nearest(d)
	t := &ss.TW.shown.thread
	*t = threadScreen{circle: circle.H, designation: spec.String()}
	t.options.Spec = spec
	t.options.Length = math.Ceil(1.5 * spec.Diameter)
	gw.ClearSelection()
	ss.TW.GoToScreen(screenThread)
	gw.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// EditControlDoneForThread takes an edited thread parameter, and
// reports whether the edit was one of those.
func (tw *textWindow) EditControlDoneForThread(s string) bool {
	ss := tw.ss
	t := &tw.shown.thread
	switch tw.edit.meaning {
	case editThreadDesignation:
		spec, err := thread.Parse(s)
		if err != nil {
			ss.Report(err)
			break
		}
		t.designation = spec.String()
		t.options.Spec = spec

	case editThreadLength:
		// A plain number, or an expression of the parameters, which the
		// thread's length then follows.
		v, expr, err := ss.SK.ThreadLengthText(s)
		switch {
		case err != nil:
			ss.Report(err)
		case v < core.LengthEps:
			ss.Error("Length cannot be zero or negative.")
		default:
			t.options.Length, t.options.LengthExpr = v, expr
		}

	case editThreadClearance:
		if e := ss.exprFromString(s); e != nil {
			t.options.Clearance = ss.ExprToMm(e)
		}

	default:
		return false
	}
	return true
}

// ScreenChangeThread edits a thread parameter.
func (tw *textWindow) ScreenChangeThread(link int, v uint32) {
	ss := tw.ss
	t := &tw.shown.thread
	switch link {
	case 'd':
		tw.ShowEditControl(14, t.designation)
		tw.edit.meaning = editThreadDesignation
	case 'l':
		text := t.options.LengthExpr
		if text == "" {
			text = ss.MmToString(t.options.Length, true)
		}
		tw.ShowEditControl(14, text)
		tw.edit.meaning = editThreadLength
	case 'c':
		tw.ShowEditControl(14, ss.MmToString(t.options.Clearance, true))
		tw.edit.meaning = editThreadClearance
	case 'e':
		t.options.Internal = false
	case 'i':
		t.options.Internal = true
	case 't':
		t.options.Through = !t.options.Through
	case 'r':
		t.options.Reverse = !t.options.Reverse
	case 'n':
		t.options.Crest = thread.CrestStandard
	case 'u':
		t.options.Crest = thread.CrestTruncated
	case 'o':
		t.options.Crest = thread.CrestRounded
	}
}

// ScreenEditThread shows the thread screen for the thread whose helix
// group is v, filled in from what it was made with.
func (tw *textWindow) ScreenEditThread(link int, v uint32) {
	ss := tw.ss
	hg := core.HGroup{V: v}
	o, err := ss.SK.ThreadOf(hg)
	if err != nil {
		ss.Report(err)
		return
	}
	tw.shown.thread = threadScreen{
		circle: ss.SK.Group(hg).Thread.Circle, designation: o.Spec.String(), options: o, edit: hg}
	tw.GoToScreen(screenThread)
	ss.ScheduleShowTW()
}

// ScreenMakeThread makes the thread.
func (tw *textWindow) ScreenMakeThread(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	t := &tw.shown.thread
	if err := sk.CheckThread(t.circle, t.options); err != nil {
		ss.Report(err)
		return
	}
	ss.UndoRemember()
	ss.GW.ClearSelection()
	ss.applyEngineSettings()
	var res sketch.ThreadResult
	var err error
	if t.edit.V != 0 {
		res, err = sk.EditThread(t.edit, t.options)
	} else {
		res, err = sk.AddThread(t.circle, t.options)
	}
	if err != nil {
		ss.Report(err)
		return
	}
	ss.afterGenerate()
	g := sk.Group(res.Group)
	ss.groupActivate(g)
	tw.ScreenSelectGroup(0, g.H.V)
	ss.ScheduleShowTW()
	if !res.Sound {
		ss.Message("The thread's solid model has naked edges or self-intersections " +
			"at the current chord tolerance.\n\n" +
			"A finer chord tolerance (in the configuration screen) usually " +
			"puts that right; so may forcing the thread's group to a triangle mesh.")
	}
}

// ShowThread is the screen to make a screw thread.
func (tw *textWindow) ShowThread() {
	ss := tw.ss
	t := &tw.shown.thread
	o := &t.options
	if t.edit.V != 0 {
		tw.Printf(true, "%FtSCREW THREAD%E   (editing %s)", tw.ss.SK.Group(t.edit).Name)
	} else {
		tw.Printf(true, "%FtSCREW THREAD%E")
	}
	tw.Printf(true, "%Ba   %Ftthread%E    %s %Fl%Ld%f[change]%E", t.designation, lf(tw.ScreenChangeThread))
	hand := "right"
	if o.Spec.LeftHand {
		hand = "left"
	}
	starts := ""
	if n := o.Spec.NumStarts(); n > 1 {
		starts = fmt.Sprintf(", %d starts", n)
	}
	tw.Printf(false, "%Bd             %s pitch, %s-hand%s", ss.MmToString(o.Spec.Pitch, false), hand, starts)
	tw.Printf(false, "%Ba   %Ftlength%E    %s%s %Fl%Ll%f[change]%E", ss.MmToString(o.Length, false),
		exprNote(o.LengthExpr), lf(tw.ScreenChangeThread))
	tw.Printf(false, "%Bd   %Ftclearance%E %s %Fl%Lc%f[change]%E", ss.MmToString(o.Clearance, false), lf(tw.ScreenChangeThread))
	// The crest at the end of a row, so that the rows below stay put.
	tw.Printf(false, "%Ba   %f%Le%Fd%s external%E  %f%Li%Fd%s internal%E    %Ftcrest%E %f%Ln%Fd%s standard%E %f%Lu%Fd%s truncated%E %f%Lo%Fd%s rounded%E",
		lf(tw.ScreenChangeThread), radioMark(!o.Internal), lf(tw.ScreenChangeThread), radioMark(o.Internal),
		lf(tw.ScreenChangeThread), radioMark(o.Crest == thread.CrestStandard),
		lf(tw.ScreenChangeThread), radioMark(o.Crest == thread.CrestTruncated),
		lf(tw.ScreenChangeThread), radioMark(o.Crest == thread.CrestRounded))
	tw.Printf(false, "%Bd   %Fd%f%Lt%s  through the far end%E", lf(tw.ScreenChangeThread), checkMark(o.Through))
	tw.Printf(false, "%Ba   %Fd%f%Lr%s  against the circle's normal%E", lf(tw.ScreenChangeThread), checkMark(o.Reverse))

	if t.edit.V != 0 {
		tw.Printf(true, " %Fl%Lg%fmake the thread again now%E", lf(tw.ScreenMakeThread))
	} else {
		tw.Printf(true, " %Fl%Lg%fmake the thread now%E", lf(tw.ScreenMakeThread))
	}

	tw.Printf(true, "(or %Fl%Ll%fcancel operation%E)", lf(tw.ScreenHome))
}
