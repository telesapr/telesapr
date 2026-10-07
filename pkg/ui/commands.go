package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// This file is an extension: C++ SolveSpace has no command search. The
// screen finds an entry of the main menu by words of its label and runs
// it. The editor tells only when Enter is pressed, so the first Enter
// lists what matches and the second, on the same text, runs the first
// of them; a single match runs at once.

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
func commandEntries() []commandEntry {
	var out []commandEntry
	var path []string
	for _, m := range menuTable {
		if m.label == "" {
			continue
		}
		path = append(path[:m.level], strings.ReplaceAll(m.label, "&", ""))
		if m.cmd == CmdNone || m.fn == nil || m.cmd == CmdOpenRecent || m.cmd == CmdGroupRecent {
			continue
		}
		out = append(out, commandEntry{label: strings.Join(path, " > "), cmd: m.cmd, accel: m.accel, fn: m.fn})
	}
	return out
}

// CommandLabels lists the labels the command search knows, "Menu >
// Entry", in the menus' order.
func CommandLabels() []string {
	var out []string
	for _, e := range commandEntries() {
		out = append(out, e.label)
	}
	return out
}

func init() { api.SetCommandList(CommandLabels) }

// findCommands returns the entries whose label holds every word of
// query, ignoring case: an entry whose own name starts with the query
// first, then the others in the menus' order.
func findCommands(query string) []commandEntry {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var found []commandEntry
	for _, e := range commandEntries() {
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
	sort.SliceStable(found, func(i, j int) bool {
		return strings.HasPrefix(name(found[i]), q) && !strings.HasPrefix(name(found[j]), q)
	})
	return found
}

// RunCommand runs the menu entry with the label given, "Menu > Entry"
// as CommandLabels lists it, or the entry's own name when only one
// entry has it; case and mnemonics are ignored.
func (ss *Application) RunCommand(label string) error {
	want := strings.ToLower(strings.TrimSpace(label))
	var byName []commandEntry
	for _, e := range commandEntries() {
		l := strings.ToLower(e.label)
		if l == want {
			e.fn(ss, e.cmd)
			return nil
		}
		if l[strings.LastIndex(l, " > ")+3:] == want {
			byName = append(byName, e)
		}
	}
	switch len(byName) {
	case 1:
		byName[0].fn(ss, byName[0].cmd)
		return nil
	case 0:
		return fmt.Errorf("no command %q", label)
	}
	var labels []string
	for _, e := range byName {
		labels = append(labels, e.label)
	}
	return fmt.Errorf("%q could be %s", label, strings.Join(labels, " or "))
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
	tw.GoToScreen(ScreenCommands)
	ss.GW.ForceTextWindowShown()
	tw.Show()
	tw.ScreenChangeCommandQuery(0, 0)
}

// ScreenChangeCommandQuery opens the editor on the search text.
func (tw *TextWindow) ScreenChangeCommandQuery(link int, v uint32) {
	tw.ShowEditControl(3, tw.shown.commands.query, tw.lineTop(1))
	tw.edit.meaning = EditCommandQuery
}

// ScreenRunCommand runs found command v.
func (tw *TextWindow) ScreenRunCommand(link int, v uint32) {
	found := tw.shown.commands.found
	if int(v) < len(found) {
		tw.runFound(found[v])
	}
}

// runFound leaves the screen and runs e.
func (tw *TextWindow) runFound(e commandEntry) {
	tw.HideEditControl()
	tw.edit.meaning = EditNothing
	tw.GoToScreen(ScreenListOfGroups)
	tw.ss.ScheduleShowTW()
	e.fn(tw.ss, e.cmd)
}

// EditControlDoneForCommands takes the search text, and reports whether
// the edit was the search's.
func (tw *TextWindow) EditControlDoneForCommands(s string) bool {
	if tw.edit.meaning != EditCommandQuery {
		return false
	}
	c := &tw.shown.commands
	again := strings.TrimSpace(s) == strings.TrimSpace(c.query) && len(c.found) > 0
	c.query = s
	c.found = findCommands(s)
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
func (tw *TextWindow) ShowCommands() {
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
			tw.Printf(false, "%Bp   %Fl%Lr%D%f%s%E%Fh%s%E%s", bg, uint32(i), lf(tw.ScreenRunCommand), e.label, accel, first)
		}
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}
