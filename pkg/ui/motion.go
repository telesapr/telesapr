package ui

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// The motion study (roadmap 16.3): parameters driven together through
// their ranges in steps, the model regenerated at each, and a picture
// or a mesh of each saved, or the whole study as one glTF animation, if
// asked (api.AssemblyMenu.MotionStudyOf). Its screen takes the
// parameters, their ranges and the steps, runs it, and lists what each
// step gave.

// motionStudy is the screen's state: the drives, the first always
// there, and the steps.
type motionStudy struct {
	drives []api.Drive
	steps  int
	frames []api.MotionFrame
	err    string
}

// motionFilters are what the study can save: a picture or a mesh a
// step, or one glTF animation of it all.
var motionFilters = append(append(append([]platform.FileFilter{}, platform.RasterFileFilters...),
	platform.MeshFileFilters[:2]...), platform.FileFilter{Name: "glTF animation of the whole study", Extensions: []string{"glb", "gltf"}})

// MenuMotionStudy shows the motion study's screen, starting from the
// first parameter, from its value to twice it.
func (ss *Application) MenuMotionStudy() {
	tw := ss.TW
	m := &tw.motion
	if len(m.drives) == 0 || slices.ContainsFunc(m.drives, func(d api.Drive) bool { return ss.SK.Parameter(d.Param) == nil }) {
		*m = motionStudy{steps: 10}
		if len(ss.SK.Parameters) > 0 {
			p := ss.SK.Parameters[0]
			m.drives = []api.Drive{{Param: p.Name, From: p.Value, To: 2 * p.Value}}
		}
	}
	ss.GW.ClearSelection()
	tw.GoToScreen(screenMotionStudy)
	ss.GW.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// ShowMotionStudy is the motion study's screen.
func (tw *textWindow) ShowMotionStudy() {
	m := &tw.motion
	tw.Printf(true, "%FtMOTION STUDY%E")
	if len(tw.ss.SK.Parameters) == 0 || len(m.drives) == 0 {
		tw.Printf(false, "%Ba   (no parameters: a motion study drives one; Edit > Parameters adds them)")
		tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
		return
	}
	num := func(v float64) string { return strconv.FormatFloat(v, 'g', 10, 64) }
	ch := lf(tw.ScreenChangeMotion)
	for i, d := range m.drives {
		if len(m.drives) > 1 {
			tw.Printf(true, "%Ba   %Ftparameter%E %s %Fl%Ln%f%D[change]%E %Fl%Lx%f%D[remove]%E", d.Param, ch, uint32(i), ch, uint32(i))
		} else {
			tw.Printf(true, "%Ba   %Ftparameter%E %s %Fl%Ln%f%D[change]%E", d.Param, ch, uint32(i))
		}
		tw.Printf(false, "%Bd   %Ftfrom%E      %s %Fl%Lf%f%D[change]%E", num(d.From), ch, uint32(i))
		tw.Printf(false, "%Ba   %Ftto%E        %s %Fl%Lt%f%D[change]%E", num(d.To), ch, uint32(i))
	}
	tw.Printf(false, "%Bd   %Ftsteps%E     %d %Fl%Ls%f[change]%E", m.steps, ch)
	if len(m.drives) < len(tw.ss.SK.Parameters) {
		tw.Printf(false, "%Ba   %Fl%La%f[drive another parameter with it]%E", ch)
	}
	tw.Printf(true, "   %Fl%Lr%f[run]%E   %Fl%LR%f[run, saving each step...]%E", ch, ch)
	if m.err != "" {
		tw.Printf(true, "   %Fx%s%E", m.err)
	}
	if len(m.frames) > 0 {
		var head strings.Builder
		head.WriteString("%Ft   step  ")
		for _, d := range m.drives {
			head.WriteString(pad(d.Param, 12) + "  ")
		}
		tw.Printf(true, head.String()+"solved   %s  file%E", pad("volume", 12))
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
			if f.File != "" && (i == 0 || f.File != m.frames[i-1].File) {
				file = filepath.Base(f.File)
			}
			var vals strings.Builder
			for _, v := range f.Values {
				vals.WriteString(pad(num(v), 12) + "  ")
			}
			tw.Printf(false, "%Bp   %s  %s"+ok+"      %s  %s", bg, pad(strconv.Itoa(f.Step), 4), vals.String(),
				pad(strconv.FormatFloat(f.Volume, 'f', 1, 64), 12), file)
		}
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}

// ScreenChangeMotion edits drive v's parameter ('n') or range ('f',
// 't'), takes it out ('x'), adds a drive ('a'), edits the steps ('s'),
// or runs the study ('r'), saving each step or an animation ('R').
func (tw *textWindow) ScreenChangeMotion(link int, v uint32) {
	m := &tw.motion
	i := int(v)
	if i >= len(m.drives) {
		i = 0
	}
	tw.edit.i = i
	switch link {
	case 'n':
		tw.ShowEditControl(13, m.drives[i].Param)
		tw.edit.meaning = editMotionParameter
	case 'f':
		tw.ShowEditControl(13, strconv.FormatFloat(m.drives[i].From, 'g', -1, 64))
		tw.edit.meaning = editMotionFrom
	case 't':
		tw.ShowEditControl(13, strconv.FormatFloat(m.drives[i].To, 'g', -1, 64))
		tw.edit.meaning = editMotionTo
	case 's':
		tw.ShowEditControl(13, strconv.Itoa(m.steps))
		tw.edit.meaning = editMotionSteps
	case 'x':
		if len(m.drives) > 1 {
			m.drives = slices.Delete(m.drives, i, i+1)
		}
		tw.ss.ScheduleShowTW()
	case 'a':
		// The first parameter not driven yet, from its value to twice it.
		for _, p := range tw.ss.SK.Parameters {
			if !slices.ContainsFunc(m.drives, func(d api.Drive) bool { return d.Param == p.Name }) {
				m.drives = append(m.drives, api.Drive{Param: p.Name, From: p.Value, To: 2 * p.Value})
				break
			}
		}
		tw.ss.ScheduleShowTW()
	case 'r':
		tw.runMotionStudy("")
	case 'R':
		ss := tw.ss
		dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
		dialog.AddFilters(motionFilters)
		dialog.ThawChoices(ss.Settings, "MotionStudy")
		dialog.SetFilename(ss.SaveFile)
		if !dialog.RunModal() {
			return
		}
		dialog.FreezeChoices(ss.Settings, "MotionStudy")
		name := dialog.Filename()
		ext := filepath.Ext(name)
		if strings.EqualFold(ext, ".glb") || strings.EqualFold(ext, ".gltf") {
			// One file, the whole study as an animation.
			tw.runMotionStudy(strings.ReplaceAll(name, "%", ""))
			return
		}
		// frame.png saves frame-000.png, frame-001.png and on.
		tw.runMotionStudy(strings.ReplaceAll(strings.TrimSuffix(name, ext), "%", "%%") + "-%03d" + ext)
	}
}

// runMotionStudy runs the study, saving its frames by the pattern if
// it isn't "", and shows what it gave.
func (tw *textWindow) runMotionStudy(frames string) {
	m := &tw.motion
	doc := api.NewDoc(tw.ss.ScriptHost())
	m.frames, m.err = nil, ""
	fs, err := doc.Assembly.MotionStudyOf(m.drives, m.steps, frames)
	m.frames = fs
	if err != nil {
		m.err = capitalize(err.Error()) + "."
	}
	tw.ss.GW.Invalidate(false)
	tw.ss.ScheduleShowTW()
}

// editMotion takes an edit of the study's screen, and reports whether
// the edit was one of those.
func (tw *textWindow) editMotion(s string) bool {
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
	i := tw.edit.i
	switch tw.edit.meaning {
	case editMotionParameter, editMotionFrom, editMotionTo:
		if i >= len(m.drives) {
			// The drive edited was taken out meanwhile.
			return true
		}
	}
	switch tw.edit.meaning {
	case editMotionParameter:
		switch {
		case ss.SK.Parameter(s) == nil:
			ss.Error("There is no parameter %q.", s)
		case slices.ContainsFunc(m.drives, func(d api.Drive) bool { return d.Param == s }) && m.drives[i].Param != s:
			ss.Error("The study drives %q already.", s)
		default:
			m.drives[i].Param = s
		}
	case editMotionFrom:
		if v, ok := number(); ok {
			m.drives[i].From = v
		}
	case editMotionTo:
		if v, ok := number(); ok {
			m.drives[i].To = v
		}
	case editMotionSteps:
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
