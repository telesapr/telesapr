package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
)

// The command search screen finds an entry of the main menu by words of
// its label and runs it. The editor tells only when Enter is pressed,
// so the first Enter lists what matches and the second, on the same
// text, runs the first of them; a single match runs at once.

// commandEntry is a menu entry the command search can run.
type commandEntry struct {
	label string // "Menu > Entry", without mnemonics
	cmd   Command
	accel int
	fn    menuHandler
}

// commandEntries lists the entries of the main menu that run a
// command, in the menus' order; the recent-file submenus are not
// commands.
func (ss *Application) commandEntries() []commandEntry {
	var out []commandEntry
	var path []string
	for _, m := range ss.menu {
		if m.label == "" {
			continue
		}
		path = append(path[:m.level], strings.ReplaceAll(m.label, "&", ""))
		if m.cmd == cmdNone || m.fn == nil || m.cmd == cmdOpenRecent || m.cmd == cmdGroupRecent {
			continue
		}
		out = append(out, commandEntry{label: strings.Join(path, " > "), cmd: m.cmd, accel: m.accel, fn: m.fn})
	}
	return out
}

// CommandLabels lists the labels the command search knows, "Menu >
// Entry", in the menus' order.
func (ss *Application) CommandLabels() []string {
	var out []string
	for _, e := range ss.commandEntries() {
		out = append(out, e.label)
	}
	return out
}

// findCommands returns the entries whose label holds every word of
// query, ignoring case: those that can run now before those that can't,
// and among each an entry whose own name starts with the query first,
// then the others in the menus' order.
func (ss *Application) findCommands(query string) []commandEntry {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var found []commandEntry
	for _, e := range ss.commandEntries() {
		l := strings.ToLower(e.label)
		ok := true
		for _, w := range words {
			if !strings.Contains(l, w) {
				ok = false
				break
			}
		}
		if ok {
			found = append(found, e)
		}
	}
	name := func(e commandEntry) string {
		l := strings.ToLower(e.label)
		return l[strings.LastIndex(l, " > ")+3:]
	}
	// What can run now first, then an entry whose own name starts with
	// the query, then the menus' order.
	rank := func(e commandEntry) int {
		r := 0
		if ss.GW.commandUnavailable(e.cmd) != "" {
			r += 2
		}
		if !strings.HasPrefix(name(e), q) {
			r++
		}
		return r
	}
	slices.SortStableFunc(found, func(a, b commandEntry) int { return rank(a) - rank(b) })
	return found
}

// formerCommandLabels are the labels, "Menu > Entry" in lower case, that
// entries had before the menus were put in the order of the work, so that
// a script written against them still runs.
var formerCommandLabels = map[string]Command{
	"file > export image...":                    cmdExportImage,
	"file > export 2d view...":                  cmdExportView,
	"file > export 2d section...":               cmdExportSection,
	"file > drawing sheet...":                   cmdDrawingSheet,
	"file > export drawing...":                  cmdExportDrawing,
	"file > export 3d wireframe...":             cmdExportWireframe,
	"file > export triangle mesh...":            cmdExportMesh,
	"file > export surfaces...":                 cmdExportSurfaces,
	"edit > configuration...":                   cmdConfiguration,
	"edit > move group earlier":                 cmdMoveGroupEarlier,
	"edit > move group later":                   cmdMoveGroupLater,
	"edit > move group to end":                  cmdMoveGroupToEnd,
	"edit > broken references...":               cmdBrokenReferences,
	"new group > sketch in 3d":                  cmdGroup3D,
	"new group > sketch in new workplane":       cmdGroupWrkpl,
	"new group > step translating":              cmdGroupTrans,
	"new group > step rotating":                 cmdGroupRot,
	"new group > extrude":                       cmdGroupExtrude,
	"new group > helix":                         cmdGroupHelix,
	"new group > lathe":                         cmdGroupLathe,
	"new group > revolve":                       cmdGroupRevolve,
	"new group > screw thread...":               cmdGroupThread,
	"new group > sweep along path":              cmdGroupSweep,
	"new group > mirror":                        cmdGroupMirror,
	"new group > chamfer edges":                 cmdGroupChamfer,
	"new group > fillet edges":                  cmdGroupFillet,
	"new group > loft to selected profile":      cmdGroupLoft,
	"new group > shell solid":                   cmdGroupShell,
	"new group > wrap sketch onto face":         cmdGroupWrap,
	"new group > pattern along path":            cmdGroupPathPattern,
	"new group > draft selected sides":          cmdDraftSelectedSides,
	"new group > guide loft by selected curves": cmdLoftGuides,
	"new group > link / assemble...":            cmdGroupLink,
	"constrain > mate concentric":               cmdMateConcentric,
	"constrain > mate flush":                    cmdMateFlush,
	"constrain > mate hinge":                    cmdMateHinge,
	"analyze > show interfering parts":          cmdInterference,
	"analyze > bill of materials":               cmdBillOfMaterials,
	"analyze > motion study...":                 cmdMotionStudy,
	"export image...":                           cmdExportImage,
	"export 2d view...":                         cmdExportView,
	"export 2d section...":                      cmdExportSection,
	"export drawing...":                         cmdExportDrawing,
	"export 3d wireframe...":                    cmdExportWireframe,
	"export triangle mesh...":                   cmdExportMesh,
	"export surfaces...":                        cmdExportSurfaces,
	"file > export > image...":                  cmdExportImage,
	"file > export > 2d view...":                cmdExportView,
	"file > export > 2d section...":             cmdExportSection,
	"file > export > 3d wireframe...":           cmdExportWireframe,
	"file > export > triangle mesh...":          cmdExportMesh,
	"file > export > surfaces...":               cmdExportSurfaces,
	"file > export > drawing...":                cmdExportDrawing,
	"configuration...":                          cmdConfiguration,
}

// RunCommand runs the menu entry with the label given, "Menu > Entry"
// as CommandLabels lists it, the entry's own name when only one entry
// has it, or a label the entry had before (formerCommandLabels); case
// and mnemonics are ignored.
func (ss *Application) RunCommand(label string) error {
	want := strings.ToLower(strings.TrimSpace(label))
	var byName []commandEntry
	for _, e := range ss.commandEntries() {
		l := strings.ToLower(e.label)
		if l == want {
			return ss.runEntry(e)
		}
		if l[strings.LastIndex(l, " > ")+3:] == want {
			byName = append(byName, e)
		}
	}
	if cmd, ok := formerCommandLabels[want]; ok && len(byName) == 0 {
		for _, e := range ss.commandEntries() {
			if e.cmd == cmd {
				return ss.runEntry(e)
			}
		}
	}
	switch len(byName) {
	case 1:
		return ss.runEntry(byName[0])
	case 0:
		return fmt.Errorf("no command %q", label)
	}
	var labels []string
	for _, e := range byName {
		labels = append(labels, e.label)
	}
	return fmt.Errorf("%q could be %s", label, strings.Join(labels, " or "))
}

// runEntry runs a menu entry, or says why it can't run now.
func (ss *Application) runEntry(e commandEntry) error {
	if why := ss.GW.commandUnavailable(e.cmd); why != "" {
		return fmt.Errorf("%s: %s", e.label, why)
	}
	e.fn(ss, e.cmd)
	return nil
}

// commandScreen is what the command search shows.
type commandScreen struct {
	query string
	found []commandEntry
}

// MenuFindCommand shows the command search with its editor open.
func (ss *Application) MenuFindCommand() {
	ss.GW.ClearSelection()
	tw := ss.TW
	tw.shown.commands = commandScreen{}
	tw.GoToScreen(screenCommands)
	ss.GW.ForceTextWindowShown()
	tw.Show()
	tw.ScreenChangeCommandQuery(0, 0)
}

// ScreenChangeCommandQuery opens the editor on the search text.
func (tw *textWindow) ScreenChangeCommandQuery(link int, v uint32) {
	tw.ShowEditControl(3, tw.shown.commands.query, tw.lineTop(1))
	tw.edit.meaning = editCommandQuery
}

// ScreenRunCommand runs found command v.
func (tw *textWindow) ScreenRunCommand(link int, v uint32) {
	found := tw.shown.commands.found
	if int(v) < len(found) {
		tw.runFound(found[v])
	}
}

// runFound leaves the screen and runs e; one that can't run now leaves
// the screen as it is, which says why.
func (tw *textWindow) runFound(e commandEntry) {
	if tw.ss.GW.commandUnavailable(e.cmd) != "" {
		return
	}
	tw.HideEditControl()
	tw.edit.meaning = editNothing
	tw.GoToScreen(screenListOfGroups)
	tw.ss.ScheduleShowTW()
	e.fn(tw.ss, e.cmd)
}

// EditControlDoneForCommands takes the search text, and reports whether
// the edit was the search's.
func (tw *textWindow) EditControlDoneForCommands(s string) bool {
	if tw.edit.meaning != editCommandQuery {
		return false
	}
	c := &tw.shown.commands
	again := strings.TrimSpace(s) == strings.TrimSpace(c.query) && len(c.found) > 0
	c.query = s
	c.found = tw.ss.findCommands(s)
	if len(c.found) == 1 || (again && len(c.found) > 0) {
		tw.runFound(c.found[0])
		return true
	}
	// Keep the editor open on the text, for a narrower search or a
	// second Enter.
	tw.edit.showAgain = true
	tw.ss.ScheduleShowTW()
	return true
}

// ShowCommands is the command search screen.
func (tw *textWindow) ShowCommands() {
	c := &tw.shown.commands
	tw.Printf(false, "%Ft FIND A COMMAND%E  %Fl%Le%f[type]%E", lf(tw.ScreenChangeCommandQuery))
	tw.Printf(false, "")
	switch {
	case strings.TrimSpace(c.query) == "":
		tw.Printf(false, "   Type words of a menu entry, then Enter to list the matches;")
		tw.Printf(false, "   Enter again runs the first.")
	case len(c.found) == 0:
		tw.Printf(false, "   %FxNo command matches '%s'.%E", c.query)
	default:
		for i, e := range c.found {
			bg := byte('d')
			if i%2 == 0 {
				bg = 'a'
			}
			accel := ""
			if e.accel != 0 {
				accel = "  " + platform.AcceleratorDescription(tw.ss.GW.AcceleratorForCommand(e.cmd))
			}
			first := ""
			if i == 0 {
				first = "  (Enter)"
			}
			if why := tw.ss.GW.commandUnavailable(e.cmd); why != "" {
				tw.Printf(false, "%Bp   %Fh%s%E%Fh%s  (%s)%E", bg, e.label, accel, why)
				continue
			}
			tw.Printf(false, "%Bp   %Fl%Lr%D%f%s%E%Fh%s%E%s", bg, uint32(i), lf(tw.ScreenRunCommand), e.label, accel, first)
		}
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}
