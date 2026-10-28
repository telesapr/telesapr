package ui

import (
	"errors"
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
	// note says how the designation typed last was read, when it is
	// written otherwise (thread.Note).
	note string
	// picking shows the list of sizes and pitches to choose from, of
	// the series shown, in place of the screen (thread_picker.go).
	picking bool
	series  thread.Series
	// pickingClass shows the tolerance classes to choose from
	// (thread_class.go).
	pickingClass bool
	// onFace is set for a thread on a cylindrical face of the solid,
	// which starts at place.At (sketch.AddThreadOnFace).
	onFace bool
	place  sketch.ThreadPlace
	// otherEnd is set when place starts at the end it was not first
	// found at.
	otherEnd bool
	// made is what the screen made last, which the thread's own screen
	// then says (showThreadSummary).
	made sketch.ThreadResult
}

// MenuThread starts a screw thread on the selected circle or arc, or
// cylindrical face or circular edge of the solid: it fills the thread
// screen in from it and shows it.
func (ss *Application) MenuThread() {
	gw := ss.GW
	sk := ss.SK
	gw.GroupSelection()
	gs := &gw.gs
	t := &ss.TW.shown.thread
	switch {
	case gs.circlesOrArcs == 1 && gs.n == 1 && len(gs.solidEdge) == 0 && len(gs.solidFace) == 0:
		circle := sk.MustEntity(gs.entity[0])
		d := 2 * circle.CircleGetRadiusNum(sk)
		spec := thread.Nearest(d)
		*t = threadScreen{circle: circle.H, designation: spec.String()}
		t.options.Spec = spec
		t.options.Length = math.Ceil(1.5 * spec.Diameter)
	default:
		p, what, err := ss.selectedThreadPlace()
		if what == "" {
			ss.Error("Bad selection for new screw thread. This group can " +
				"be created with:\n\n" +
				"    * a circle or an arc (thread about its axis, from its plane)\n" +
				"    * a cylindrical face of the solid (a screw's on a shaft, a nut's " +
				"in a hole, from the part's end along the face)\n" +
				"    * a circular edge at an end of a cylindrical face (from that end)\n")
			return
		}
		if err != nil {
			ss.Error("Can't make a screw thread on %s. %s.\n\n"+
				"A thread is made on a cylindrical face, a shaft or a hole, or on "+
				"the circular edge at one of its ends.", what, capitalize(err.Error()))
			return
		}
		*t = faceThreadScreen(p)
	}
	gw.ClearSelection()
	ss.TW.GoToScreen(screenThread)
	gw.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// selectedThreadPlace is where a thread starts on the one cylindrical
// face or circular edge of the solid selected, and what that is, for a
// message; what is "" when no such thing is selected.
func (ss *Application) selectedThreadPlace() (p sketch.ThreadPlace, what string, err error) {
	gs := &ss.GW.gs
	sk := ss.SK
	if face, ok := gs.onlyFace(); ok {
		p, err = sk.ThreadPlaceOnFace(face, false)
		return p, sk.FaceName(face), err
	}
	if gs.n == 0 && gs.constraints == 0 && len(gs.solidFace) == 0 && len(gs.solidEdge) == 1 {
		n := gs.solidEdge[0]
		p, err = sk.ThreadPlaceAtEdge(n)
		return p, n.String(), err
	}
	return p, "", nil
}

// threadSelected reports whether what is selected carries a thread: a
// circle or an arc, or a cylindrical face or a circular edge at its end,
// as the context menu offers Screw Thread for.
func (ss *Application) threadSelected() bool {
	gs := &ss.GW.gs
	if gs.circlesOrArcs == 1 && gs.n == 1 && len(gs.solidEdge) == 0 && len(gs.solidFace) == 0 {
		return true
	}
	_, what, err := ss.selectedThreadPlace()
	return what != "" && err == nil
}

// faceThreadScreen is the thread screen filled in for a thread at p:
// the standard thread nearest the shaft, or the one a hole is drawn or
// drilled for, one and a half diameters long or as long as the face.
func faceThreadScreen(p sketch.ThreadPlace) threadScreen {
	spec := thread.Nearest(p.Diameter())
	if p.At.Hole {
		spec = thread.NearestForHole(p.Diameter())
	}
	t := threadScreen{designation: spec.String(), onFace: true, place: p}
	t.options.Spec = spec
	t.options.Length = math.Ceil(1.5 * spec.Diameter)
	t.options.FullLength = t.options.Length >= p.At.Length
	t.options = p.Options(t.options)
	return t
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
		t.note = thread.Note(s, spec)

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
			t.options.FullLength = false
		}

	case editThreadClearance:
		// A clearance typed is the thread's own, in place of a class's.
		if e := ss.exprFromString(s); e != nil {
			t.options.Clearance, t.options.Class = ss.ExprToMm(e), ""
		}

	case editThreadPhase:
		if e := ss.exprFromString(s); e != nil {
			t.options.Phase = math.Mod(e.Eval(sketch.NewRuntime(ss.SK), nil), 360)
		}

	case editThreadRunOut:
		if e := ss.exprFromString(s); e != nil {
			if v := ss.ExprToMm(e); v < 0 {
				ss.Error("A run-out cannot be negative.")
			} else {
				t.options.RunOut = v
			}
		}

	default:
		return false
	}
	t.keepClass()
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
		tw.ShowEditControl(14, ss.MmToString(t.shownClearance(), true))
		tw.edit.meaning = editThreadClearance
	case 'f':
		tw.ShowEditControl(14, ss.DegreeToString(t.options.Phase))
		tw.edit.meaning = editThreadPhase
	case 'q':
		t.pickingClass = true
	case 'b':
		tw.ShowEditControl(14, ss.MmToString(t.options.RunOut, true))
		tw.edit.meaning = editThreadRunOut
	case 'm':
		t.picking, t.series = true, thread.SeriesOf(t.options.Spec)
	case 'h', 'k':
		t.options.Spec.LeftHand = link == 'k'
		t.designation, t.note = t.options.Spec.String(), ""
	case 's':
		t.options.LevelStarts = !t.options.LevelStarts
	case 'w':
		t.options.LoadFlankBack = !t.options.LoadFlankBack
	case 'y', 'z':
		t.options.Cosmetic = link == 'z'
	case 'x':
		t.options.Chamfer = !t.options.Chamfer
	case 'e':
		t.options.Internal = false
	case 'i':
		t.options.Internal = true
	case 't':
		if t.onFace {
			// Off, the thread keeps the face's length, ending short of
			// the part's far end.
			t.options.FullLength = !t.options.FullLength
			t.options.Through = false
			t.options = t.place.Options(t.options)
			break
		}
		t.options.Through = !t.options.Through
	case 'r':
		if t.onFace {
			t.place.At, t.place.Far = t.place.Far, t.place.At
			t.otherEnd = !t.otherEnd
			t.options = t.place.Options(t.options)
			break
		}
		t.options.Reverse = !t.options.Reverse
	case 'n':
		t.options.Crest = thread.CrestStandard
	case 'u':
		t.options.Crest = thread.CrestTruncated
	case 'o':
		t.options.Crest = thread.CrestRounded
	}
	t.keepClass()
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
	if p, ok := ss.SK.ThreadPlaceOf(hg); ok {
		tw.shown.thread.onFace, tw.shown.thread.place = true, p
	}
	tw.GoToScreen(screenThread)
	ss.ScheduleShowTW()
}

// ScreenMakeThread makes the thread.
func (tw *textWindow) ScreenMakeThread(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	t := &tw.shown.thread
	check := func() error { return sk.CheckThread(t.circle, t.options) }
	if t.edit.V != 0 {
		check = func() error { return sk.CheckThreadEdit(t.edit, t.options) }
	} else if t.onFace {
		check = func() error { return sk.CheckThreadOnFace(t.place, t.options) }
	}
	if err := check(); err != nil {
		ss.reportThread(err)
		return
	}
	ss.UndoRemember()
	ss.GW.ClearSelection()
	ss.applyEngineSettings()
	var res sketch.ThreadResult
	var err error
	switch {
	case t.edit.V != 0:
		res, err = sk.EditThread(t.edit, t.options)
	case t.onFace:
		res, err = sk.AddThreadOnFace(t.place, t.options)
	default:
		res, err = sk.AddThread(t.circle, t.options)
	}
	if err != nil {
		ss.Report(err)
		return
	}
	ss.afterGenerate()
	// The thread's last group is active, as after any feature; the
	// screen shows its first helix, which says what the thread is and
	// edits it.
	if hs, err := sk.ThreadGroups(res.Group); err == nil {
		ss.groupActivate(sk.Group(hs[len(hs)-1]))
	}
	// Selecting the group clears the text window, this screen's state
	// with it: what follows reads o, and the thread's screen what was
	// made.
	o := t.options
	tw.ScreenSelectGroup(0, res.Group.V)
	tw.shown.thread.made = res
	ss.ScheduleShowTW()
	if beyond := sk.ThreadPartBeyond(res.Group); beyond > o.Spec.Pitch {
		// Through carries the turning or the boring a pitch past the
		// thread, which on a part that goes on is a groove.
		ss.Message("The part goes on %s past the thread, so \"to the end of the part\" "+
			"left a groove a pitch long where the thread ends.\n\nTurn it off to end the "+
			"thread at a shoulder.", ss.MmToString(beyond, true)+" "+ss.UnitName())
	}
	if !res.Sound {
		hint := "A finer chord tolerance (in the configuration screen) usually " +
			"puts that right; so may forcing the thread's group to a triangle mesh."
		switch {
		case o.RunOut > 0:
			hint = "Where the ridge runs into the run-out's cone the Boolean can fail; " +
				"a run-out a little longer or shorter usually puts that right."
		case o.Chamfer:
			hint = "Where the chamfer's cone cuts the ridges the Boolean can fail; " +
				"a thread a little longer or shorter, or a finer chord tolerance, usually puts that right."
		}
		ss.Message("The thread's solid model has naked edges or self-intersections "+
			"at the current chord tolerance.\n\n%s", hint)
	}
}

// reportThread says why a thread can't be made, with what to do about
// it where the thread doesn't fit its part.
func (ss *Application) reportThread(err error) {
	switch {
	case errors.Is(err, sketch.ErrThreadPastPart):
		ss.Error("%s\n\nMake the thread shorter, or the part longer.", sentence(err.Error()))
	case errors.Is(err, sketch.ErrThreadThinPart):
		ss.Error("The part is thinner than the thread's root: its ridges would stand on " +
			"nothing.\n\nPick a smaller thread, or thread a thicker part.")
	case errors.Is(err, sketch.ErrButtressRounded):
		ss.Error("A buttress thread's crest can't be rounded: its flanks lean at different " +
			"angles.\n\nTake a standard or truncated crest.")
	case errors.Is(err, sketch.ErrThreadNoPart):
		ss.Error("There is no part along the thread from its circle.\n\nThe thread " +
			"may run the other way: check \"flip the direction\".")
	default:
		ss.Report(err)
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
	if t.picking {
		tw.showThreadPicker()
		return
	}
	if t.pickingClass {
		tw.showThreadClasses()
		return
	}
	note := ""
	if t.note != "" {
		note = "   (" + t.note + ")"
	}
	// The chamfer at the end of a row, so that the rows below stay put.
	chamfer := fmt.Sprintf("    %%Fd%%f%%Lx%s  chamfered end%%E", checkMark(o.Chamfer))
	if t.onFace {
		tw.Printf(true, "%Ba   %Fton%E        %s"+chamfer, t.placeText(ss), lf(tw.ScreenChangeThread))
		tw.Printf(false, "%Bd   %Ftthread%E    %s %Fl%Ld%f[change]%E %Fl%Lm%f[choose]%E%s",
			t.designation, lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread), note)
	} else {
		tw.Printf(true, "%Ba   %Ftthread%E    %s %Fl%Ld%f[change]%E %Fl%Lm%f[choose]%E%s",
			t.designation, lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread), note)
	}
	starts := ""
	if n := o.Spec.NumStarts(); n > 1 {
		starts = fmt.Sprintf(", %d starts", n)
	}
	right, left := radioMark(!o.Spec.LeftHand), radioMark(o.Spec.LeftHand)
	// A buttress thread's load flank at the end of the row, so that the
	// rows below stay put; only a buttress ridge has one.
	load, loadArgs := "", []any{}
	if o.Spec.Buttress() {
		load, loadArgs = "    %Fd%f%Lw%s  load flank back%E", []any{lf(tw.ScreenChangeThread), checkMark(o.LoadFlankBack)}
	}
	if o.Spec.NumStarts() > 1 {
		tw.Printf(false, "%Bd             %s pitch%s    %f%Lh%Fd%s right-hand%E  %f%Lk%Fd%s left-hand%E    %Fd%f%Ls%s  starts level%E"+load,
			append([]any{ss.MmToString(o.Spec.Pitch, false), starts, lf(tw.ScreenChangeThread), right,
				lf(tw.ScreenChangeThread), left, lf(tw.ScreenChangeThread), checkMark(o.LevelStarts)}, loadArgs...)...)
	} else {
		tw.Printf(false, "%Bd             %s pitch    %f%Lh%Fd%s right-hand%E  %f%Lk%Fd%s left-hand%E"+load,
			append([]any{ss.MmToString(o.Spec.Pitch, false), lf(tw.ScreenChangeThread), right,
				lf(tw.ScreenChangeThread), left}, loadArgs...)...)
	}
	full := ""
	if t.onFace && o.FullLength {
		full = " (the face's)"
	}
	unit := ss.UnitName()
	// The phase, the run-out, the class and the level starts at the
	// ends of rows, so that the rows below stay put.
	tw.Printf(false, "%Ba   %Ftlength%E    %s %s%s%s %Fl%Ll%f[change]%E    %Ftphase%E %s° %Fl%Lf%f[change]%E",
		ss.MmToString(o.Length, false), unit, exprNote(o.LengthExpr), full, lf(tw.ScreenChangeThread),
		ss.DegreeToString(o.Phase), lf(tw.ScreenChangeThread))
	class := "none"
	if o.Class != "" {
		class = o.Class
	}
	tw.Printf(false, "%Bd   %Ftclearance%E %s %s %Fl%Lc%f[change]%E    %Ftrun-out%E %s %s %Fl%Lb%f[change]%E    %Ftclass%E %s %Fl%Lq%f[choose]%E",
		ss.MmToString(t.shownClearance(), false), unit, lf(tw.ScreenChangeThread), ss.MmToString(o.RunOut, false), unit,
		lf(tw.ScreenChangeThread), class, lf(tw.ScreenChangeThread))
	// The crest at the end of a row, so that the rows below stay put.
	crest := fmt.Sprintf("%%Ftcrest%%E %%f%%Ln%%Fd%s standard%%E %%f%%Lu%%Fd%s truncated%%E %%f%%Lo%%Fd%s rounded%%E",
		radioMark(o.Crest == thread.CrestStandard), radioMark(o.Crest == thread.CrestTruncated),
		radioMark(o.Crest == thread.CrestRounded))
	// Modelled or cosmetic at the end of a row, so that the rows below
	// stay put.
	kind := fmt.Sprintf("%%Ftas%%E %%f%%Ly%%Fd%s modelled%%E  %%f%%Lz%%Fd%s cosmetic%%E",
		radioMark(!o.Cosmetic), radioMark(o.Cosmetic))
	if t.onFace {
		// The face says which: a shaft carries a screw's thread, a hole a
		// nut's.
		side := "external, on a shaft"
		if o.Internal {
			side = "internal, in a hole"
		}
		tw.Printf(false, "%Ba   %Ft%s%E    "+crest, side,
			lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread))
		tw.Printf(false, "%Bd   %Fd%f%Lt%s  the face's full length, %s %s%E    "+kind, lf(tw.ScreenChangeThread),
			checkMark(o.FullLength), ss.MmToString(t.place.At.Length, true), unit,
			lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread))
		if t.edit.V == 0 {
			tw.Printf(false, "%Ba   %Fd%f%Lr%s  from the face's other end%E", lf(tw.ScreenChangeThread),
				checkMark(t.otherEnd))
		}
	} else {
		tw.Printf(false, "%Ba   %f%Le%Fd%s external%E  %f%Li%Fd%s internal%E    "+crest,
			lf(tw.ScreenChangeThread), radioMark(!o.Internal), lf(tw.ScreenChangeThread), radioMark(o.Internal),
			lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread))
		// What through does, in the words of the part: a screw's is
		// turned down, a nut's bored, past the thread with no shoulder.
		through := "turned down past the thread"
		if o.Internal {
			through = "bored on past the thread"
		}
		tw.Printf(false, "%Bd   %Fd%f%Lt%s  to the end of the part: %s%E    "+kind, lf(tw.ScreenChangeThread),
			checkMark(o.Through), through, lf(tw.ScreenChangeThread), lf(tw.ScreenChangeThread))
		tw.Printf(false, "%Ba   %Fd%f%Lr%s  flip the direction: run against the circle's normal%E"+chamfer,
			lf(tw.ScreenChangeThread), checkMark(o.Reverse), lf(tw.ScreenChangeThread))
	}

	if t.edit.V != 0 {
		tw.Printf(true, " %Fl%Lg%fmake the thread again now%E", lf(tw.ScreenMakeThread))
	} else {
		tw.Printf(true, " %Fl%Lg%fmake the thread now%E", lf(tw.ScreenMakeThread))
	}

	tw.Printf(true, "(or %Fl%Ll%fcancel operation%E)", lf(tw.ScreenHome))
}

// placeText says where a thread on a face is: on a shaft or in a hole,
// how wide and long, and from which end.
func (t *threadScreen) placeText(ss *Application) string {
	at := t.place.At
	what := "a shaft"
	if at.Hole {
		what = "a hole"
	}
	from := "from a shoulder"
	if at.Free {
		from = "from the part's end"
	}
	return fmt.Sprintf("%s, %s %s across and %s %s long, %s", what, ss.MmToString(t.place.Diameter(), true),
		ss.UnitName(), ss.MmToString(at.Length, true), ss.UnitName(), from)
}
