package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// The guide's reference pages, doc/guide/reference/*.md, are written
// from the menu bar, the toolbar and api.MenuCalls, so they can't drift
// from them: TestReferencePages fails when they differ, and
// TELESAPR_REGEN_GOLDEN=1 rewrites them (scripts/regen-golden.sh docref).

// referenceDir holds the generated pages.
var referenceDir = filepath.Join("..", "..", "doc", "guide", "reference")

// menuRow is an entry of the menu bar, with its path.
type menuRow struct {
	path  []string // labels without mnemonics, menu first
	accel string
	kind  string // "check", "radio" or ""
}

// menuRows walks the menu bar of a new drawing.
func menuRows(bar *headless.MenuBar) []menuRow {
	var rows []menuRow
	var walk func(items []*headless.MenuItem, path []string)
	walk = func(items []*headless.MenuItem, path []string) {
		for _, it := range items {
			if it.Separator || (!it.Enabled && it.SubMenu == nil && it.OnTrigger == nil) {
				continue
			}
			p := append(slices.Clone(path), it.Label)
			if it.SubMenu != nil {
				// The recent files are the user's, not the program's.
				if !strings.HasPrefix(it.Label, "Open Recent") && !strings.HasPrefix(it.Label, "Link Recent") {
					walk(it.SubMenu.Items, p)
				}
				continue
			}
			kind := map[int]string{1: "check", 2: "radio"}[int(it.Indicator)]
			rows = append(rows, menuRow{path: p, accel: it.Accel, kind: kind})
		}
	}
	for _, m := range bar.Menus {
		if m.SubMenu != nil {
			walk(m.SubMenu.Items, []string{m.Label})
		}
	}
	return rows
}

// scriptCell says what a script calls for a menu entry.
func scriptCell(path []string) string {
	calls, ok := api.MenuCalls[strings.Join(path, "/")]
	switch {
	case !ok:
		return ""
	case strings.HasPrefix(calls, "- "):
		return "none: " + mdCell(strings.TrimPrefix(calls, "- "))
	}
	var cells []string
	for c := range strings.FieldsSeq(calls) {
		cells = append(cells, "`"+c+"`")
	}
	return strings.Join(cells, ", ")
}

// mdCell escapes what would end a Markdown table's cell.
func mdCell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// keyCell writes a key as Markdown.
func keyCell(accel string) string {
	if accel == "" {
		return ""
	}
	var keys []string
	for {
		mod, rest, ok := strings.Cut(accel, "+")
		if !ok || rest == "" || mod == "" {
			break
		}
		keys = append(keys, mod)
		accel = rest
	}
	keys = append(keys, accel)
	for i, k := range keys {
		keys[i] = "<kbd>" + mdCell(k) + "</kbd>"
	}
	return strings.Join(keys, "+")
}

const generatedNote = "<!-- Written by TestReferencePages (pkg/ui/docref_test.go) from the program; " +
	"regenerate with scripts/regen-golden.sh docref, don't edit. -->\n\n"

// menusPage is reference/menus.md: every entry of every menu.
func menusPage(rows []menuRow) string {
	var b strings.Builder
	b.WriteString(generatedNote)
	b.WriteString("# Menu reference\n\n")
	b.WriteString("Every entry of the main menu, its keys, and what a script calls to do the same " +
		"(see [Scripts](../scripts.md)). An entry marked *on/off* is ticked while it is on; one marked " +
		"*choice* is one of a set. Edit > Find Command (<kbd>Ctrl</kbd>+<kbd>K</kbd>) finds an entry " +
		"by words of its name.\n")
	menu := ""
	for _, r := range rows {
		if r.path[0] != menu {
			menu = r.path[0]
			fmt.Fprintf(&b, "\n## %s\n\n| Entry | Keys | In a script |\n|---|---|---|\n", menu)
		}
		label := mdCell(strings.Join(r.path[1:], " > "))
		switch r.kind {
		case "check":
			label += " *(on/off)*"
		case "radio":
			label += " *(choice)*"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", label, keyCell(r.accel), scriptCell(r.path))
	}
	return b.String()
}

// shortcutsPage is reference/shortcuts.md: the menus' keys, by key.
func shortcutsPage(rows []menuRow) string {
	var keyed []menuRow
	for _, r := range rows {
		if r.accel != "" {
			keyed = append(keyed, r)
		}
	}
	// Plain keys first, then Shift, then Ctrl, each in key order.
	rank := func(a string) int {
		n := 0
		if strings.Contains(a, "Ctrl+") {
			n += 2
		}
		if strings.Contains(a, "Shift+") {
			n++
		}
		return n
	}
	slices.SortStableFunc(keyed, func(x, y menuRow) int {
		if d := rank(x.accel) - rank(y.accel); d != 0 {
			return d
		}
		return strings.Compare(x.accel, y.accel)
	})
	var b strings.Builder
	b.WriteString(generatedNote)
	b.WriteString("# Keyboard shortcuts\n\n")
	b.WriteString("The keys of the menu entries, plain keys first. They work while the pointer is " +
		"over the graphics window. See [the menu reference](menus.md) for the entries by menu.\n\n")
	b.WriteString("| Keys | Entry |\n|---|---|\n")
	for _, r := range keyed {
		fmt.Fprintf(&b, "| %s | %s |\n", keyCell(r.accel), mdCell(strings.Join(r.path, " > ")))
	}
	return b.String()
}

// toolbarPage is reference/toolbar.md: the toolbar's buttons, with the
// menu entry each runs.
func toolbarPage(ss *Application, rows []menuRow) string {
	accel := map[string]string{}
	for _, r := range rows {
		accel[strings.Join(r.path, " > ")] = r.accel
	}
	entry := map[Command]string{}
	for _, e := range ss.commandEntries() {
		if _, ok := entry[e.cmd]; !ok {
			entry[e.cmd] = e.label
		}
	}
	var b strings.Builder
	b.WriteString(generatedNote)
	b.WriteString("# Toolbar\n\n")
	b.WriteString("The buttons down the left of the graphics window, top to bottom. Hovering over a " +
		"button shows its name. Each runs the menu entry beside it.\n\n")
	b.WriteString("| | Button | Menu entry | Keys |\n|---|---|---|---|\n")
	for _, ic := range toolbar {
		if ic.name == "" {
			continue
		}
		label := entry[ic.command]
		fmt.Fprintf(&b, "| ![](../images/icons/%s.png) | %s | %s | %s |\n", ic.name, mdCell(ic.tooltip),
			mdCell(label), keyCell(accel[label]))
	}
	return b.String()
}

// TestReferencePages holds the generated reference pages to the
// program, and with TELESAPR_REGEN_GOLDEN rewrites them and copies the
// toolbar's icons beside the guide's pictures.
func TestReferencePages(t *testing.T) {
	t.Parallel()
	r := newUIScriptRunner(t)
	rows := menuRows(r.gw().MenuBar.(*headless.MenuBar))
	pages := map[string]string{
		"menus.md":     menusPage(rows),
		"shortcuts.md": shortcutsPage(rows),
		"toolbar.md":   toolbarPage(r.ss, rows),
	}
	for name, page := range pages {
		golden.Check(t, filepath.Join(referenceDir, name), []byte(page))
	}
	if golden.Regen() {
		copyToolbarIcons(t)
	}
}

// copyToolbarIcons copies the toolbar's icons to doc/guide/images/icons.
func copyToolbarIcons(t *testing.T) {
	src := filepath.Join("..", "res", "data", "icons", "graphics-window")
	dst := filepath.Join(referenceDir, "..", "images", "icons")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ic := range toolbar {
		if ic.name == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, ic.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, ic.name+".png"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
