package ui

import (
	"math"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/thread"
)

// This file is an extension: C++ SolveSpace has no screw threads. The
// screen follows the paste-transformed one (clipboard.go).

// threadScreen is what the screw thread screen shows and makes.
type threadScreen struct {
	circle      core.HEntity
	designation string
	options     sketch.ThreadOptions
}

// MenuThread starts a screw thread on the selected circle or arc: it
// fills the thread screen in from the circle and shows it.
func (ss *SolveSpaceUI) MenuThread() {
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
	circle := sk.MustGetEntity(gs.entity[0])
	d := 2 * circle.CircleGetRadiusNum(sk)
	spec := thread.Nearest(d)
	t := &ss.TW.shown.thread
	*t = threadScreen{circle: circle.H, designation: spec.String()}
	t.options.Spec = spec
	t.options.Length = math.Ceil(1.5 * spec.Diameter)
	gw.ClearSelection()
	ss.TW.GoToScreen(ScreenThread)
	gw.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// EditControlDoneForThread takes an edited thread parameter, and
// reports whether the edit was one of those.
func (tw *TextWindow) EditControlDoneForThread(s string) bool {
	ss := tw.ss
	t := &tw.shown.thread
	switch tw.edit.meaning {
	case EditThreadDesignation:
		spec, err := thread.Parse(s)
		if err != nil {
			ss.Error("%s", capitalize(err.Error()))
			break
		}
		t.designation = spec.String()
		t.options.Spec = spec

	case EditThreadLength:
		if e := ss.exprFromString(s, true); e != nil {
			if v := ss.ExprToMm(e); v < core.LengthEps {
				ss.Error("Length cannot be zero or negative.")
			} else {
				t.options.Length = v
			}
		}

	case EditThreadClearance:
		if e := ss.exprFromString(s, true); e != nil {
			t.options.Clearance = ss.ExprToMm(e)
		}

	default:
		return false
	}
	return true
}

// capitalize starts a message with a capital, as the dialogs do.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ScreenChangeThread edits a thread parameter.
func (tw *TextWindow) ScreenChangeThread(link int, v uint32) {
	ss := tw.ss
	t := &tw.shown.thread
	switch link {
	case 'd':
		tw.ShowEditControl(14, t.designation)
		tw.edit.meaning = EditThreadDesignation
	case 'l':
		tw.ShowEditControl(14, ss.MmToString(t.options.Length, true))
		tw.edit.meaning = EditThreadLength
	case 'c':
		tw.ShowEditControl(14, ss.MmToString(t.options.Clearance, true))
		tw.edit.meaning = EditThreadClearance
	case 'e':
		t.options.Internal = false
	case 'i':
		t.options.Internal = true
	case 't':
		t.options.Through = !t.options.Through
	case 'r':
		t.options.Reverse = !t.options.Reverse
	}
}

// ScreenMakeThread makes the thread.
func (tw *TextWindow) ScreenMakeThread(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	t := &tw.shown.thread
	if err := sk.CheckThread(t.circle, t.options); err != nil {
		ss.Error("%s", capitalize(err.Error()))
		return
	}
	ss.UndoRemember()
	ss.GW.ClearSelection()
	ss.applyEngineSettings()
	res, err := sk.AddThread(t.circle, t.options)
	if err != nil {
		ss.Error("%s", capitalize(err.Error()))
		return
	}
	ss.afterGenerate()
	g := sk.GetGroup(res.Group)
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
func (tw *TextWindow) ShowThread() {
	ss := tw.ss
	t := &tw.shown.thread
	o := &t.options
	check := func(b bool) string {
		if b {
			return checkTrue
		}
		return checkFalse
	}
	radio := func(b bool) string {
		if b {
			return radioTrue
		}
		return radioFalse
	}
	tw.Printf(true, "%FtSCREW THREAD%E")
	tw.Printf(true, "%Ba   %Ftthread%E    %s %Fl%Ld%f[change]%E", t.designation, lf(tw.ScreenChangeThread))
	hand := "right"
	if o.Spec.LeftHand {
		hand = "left"
	}
	starts := ""
	if n := o.Spec.NumStarts(); n > 1 {
		starts = sprintf(", %d starts", n)
	}
	tw.Printf(false, "%Bd             %s pitch, %s-hand%s", ss.MmToString(o.Spec.Pitch, false), hand, starts)
	tw.Printf(false, "%Ba   %Ftlength%E    %s %Fl%Ll%f[change]%E", ss.MmToString(o.Length, false), lf(tw.ScreenChangeThread))
	tw.Printf(false, "%Bd   %Ftclearance%E %s %Fl%Lc%f[change]%E", ss.MmToString(o.Clearance, false), lf(tw.ScreenChangeThread))
	tw.Printf(false, "%Ba   %f%Le%Fd%s external%E  %f%Li%Fd%s internal%E",
		lf(tw.ScreenChangeThread), radio(!o.Internal), lf(tw.ScreenChangeThread), radio(o.Internal))
	tw.Printf(false, "%Bd   %Fd%f%Lt%s  through the far end%E", lf(tw.ScreenChangeThread), check(o.Through))
	tw.Printf(false, "%Ba   %Fd%f%Lr%s  against the circle's normal%E", lf(tw.ScreenChangeThread), check(o.Reverse))

	tw.Printf(true, " %Fl%Lg%fmake the thread now%E", lf(tw.ScreenMakeThread))

	tw.Printf(true, "(or %Fl%Ll%fcancel operation%E)", lf(tw.ScreenHome))
}
