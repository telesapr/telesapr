package ui

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// The motion study (roadmap 16.3): a parameter driven through a range
// in steps, the model regenerated at each, and a picture or a mesh of
// each saved if asked (api.Doc.MotionStudy). Its screen takes the
// parameter, the range and the steps, runs it, and lists what each
// step gave.

// motionStudy is the screen's state.
type motionStudy struct {
	param    string
	from, to float64
	steps    int
	frames   []api.MotionFrame
	err      string
}

// MenuMotionStudy shows the motion study's screen, starting from the
// first parameter, from its value to twice it.
func (ss *Application) MenuMotionStudy() {
	tw := ss.TW
	m := &tw.motion
	if ss.SK.Parameter(m.param) == nil {
		*m = motionStudy{steps: 10}
		if len(ss.SK.Parameters) > 0 {
			p := ss.SK.Parameters[0]
			m.param, m.from, m.to = p.Name, p.Value, 2*p.Value
		}
	}
	ss.GW.ClearSelection()
	tw.GoToScreen(ScreenMotionStudy)
	ss.GW.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// ShowMotionStudy is the motion study's screen.
func (tw *TextWindow) ShowMotionStudy() {
	m := &tw.motion
	tw.Printf(true, "%FtMOTION STUDY%E")
	if len(tw.ss.SK.Parameters) == 0 {
		tw.Printf(false, "%Ba   (no parameters: a motion study drives one; Edit > Parameters adds them)")
		tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
		return
	}
	num := func(v float64) string { return strconv.FormatFloat(v, 'g', 10, 64) }
	tw.Printf(true, "%Ba   %Ftparameter%E %s %Fl%Ln%f[change]%E", m.param, lf(tw.ScreenChangeMotion))
	tw.Printf(false, "%Bd   %Ftfrom%E      %s %Fl%Lf%f[change]%E", num(m.from), lf(tw.ScreenChangeMotion))
	tw.Printf(false, "%Ba   %Ftto%E        %s %Fl%Lt%f[change]%E", num(m.to), lf(tw.ScreenChangeMotion))
	tw.Printf(false, "%Bd   %Ftsteps%E     %d %Fl%Ls%f[change]%E", m.steps, lf(tw.ScreenChangeMotion))
	tw.Printf(true, "   %Fl%Lr%f[run]%E   %Fl%LR%f[run, saving each step...]%E",
		lf(tw.ScreenChangeMotion), lf(tw.ScreenChangeMotion))
	if m.err != "" {
		tw.Printf(true, "   %Fx%s%E", m.err)
	}
	if len(m.frames) > 0 {
		tw.Printf(true, "%Ft   step  %s  solved   %s  file%E", pad(m.param, 12), pad("volume", 12))
		for i, f := range m.frames {
			bg := byte('a')
			if i%2 == 1 {
				bg = 'd'
			}
			ok := "yes"
			if !f.Solved {
				ok = "%FxNO%E "
			}
			file := ""
			if f.File != "" {
				file = filepath.Base(f.File)
			}
			tw.Printf(false, "%Bp   %s  %s  "+ok+"      %s  %s", bg, pad(strconv.Itoa(f.Step), 4), pad(num(f.Value), 12),
				pad(strconv.FormatFloat(f.Volume, 'f', 1, 64), 12), file)
		}
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}

// ScreenChangeMotion edits the study's parameter ('n'), its range ('f',
// 't') or steps ('s'), or runs it ('r'), saving each step ('R').
func (tw *TextWindow) ScreenChangeMotion(link int, v uint32) {
	m := &tw.motion
	switch link {
	case 'n':
		tw.ShowEditControl(13, m.param)
		tw.edit.meaning = EditMotionParameter
	case 'f':
		tw.ShowEditControl(13, strconv.FormatFloat(m.from, 'g', -1, 64))
		tw.edit.meaning = EditMotionFrom
	case 't':
		tw.ShowEditControl(13, strconv.FormatFloat(m.to, 'g', -1, 64))
		tw.edit.meaning = EditMotionTo
	case 's':
		tw.ShowEditControl(13, strconv.Itoa(m.steps))
		tw.edit.meaning = EditMotionSteps
	case 'r':
		tw.runMotionStudy("")
	case 'R':
		ss := tw.ss
		dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
		dialog.AddFilters(append(append([]platform.FileFilter{}, platform.RasterFileFilters...), platform.MeshFileFilters[:2]...))
		dialog.ThawChoices(ss.Settings, "MotionStudy")
		dialog.SetFilename(ss.SaveFile)
		if !dialog.RunModal() {
			return
		}
		dialog.FreezeChoices(ss.Settings, "MotionStudy")
		// frame.png saves frame-000.png, frame-001.png and on.
		name := dialog.Filename()
		ext := filepath.Ext(name)
		tw.runMotionStudy(strings.ReplaceAll(strings.TrimSuffix(name, ext), "%", "%%") + "-%03d" + ext)
	}
}

// runMotionStudy runs the study, saving its frames by the pattern if
// it isn't "", and shows what it gave.
func (tw *TextWindow) runMotionStudy(frames string) {
	m := &tw.motion
	doc := api.NewDoc(tw.ss.ScriptHost())
	m.frames, m.err = nil, ""
	fs, err := doc.MotionStudy(m.param, m.from, m.to, m.steps, frames)
	m.frames = fs
	if err != nil {
		m.err = capitalize(err.Error()) + "."
	}
	tw.ss.GW.Invalidate(false)
	tw.ss.ScheduleShowTW()
}

// editMotion takes an edit of the study's screen, and reports whether
// the edit was one of those.
func (tw *TextWindow) editMotion(s string) bool {
	ss := tw.ss
	m := &tw.motion
	s = strings.TrimSpace(s)
	number := func() (float64, bool) {
		e := ss.exprFromString(s)
		if e == nil {
			return 0, false
		}
		return ss.evalExpr(e), true
	}
	switch tw.edit.meaning {
	case EditMotionParameter:
		if ss.SK.Parameter(s) == nil {
			ss.Error("There is no parameter %q.", s)
			break
		}
		m.param = s
	case EditMotionFrom:
		if v, ok := number(); ok {
			m.from = v
		}
	case EditMotionTo:
		if v, ok := number(); ok {
			m.to = v
		}
	case EditMotionSteps:
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 10000 {
			ss.Error("A motion study takes from 1 to 10000 steps.")
			break
		}
		m.steps = n
	default:
		return false
	}
	ss.ScheduleShowTW()
	return true
}
