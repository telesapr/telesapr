package ui

import (
	"strings"
	"testing"
)

// A script's File > New on unsaved changes is refused and says so,
// rather than asking (nobody would answer) or dropping them; after
// ForgetChanges it runs, as after "Don't Save".
func TestScriptNewFileKeepsUnsavedChanges(t *testing.T) {
	ss, _ := newTestApp(t)
	got := scriptConsole(t, ss,
		`doc.Sketch.Line(V(0, 0, 0), V(10, 0, 0))`,
		`doc.File.Unsaved()`,
		`doc.Edit.RunCommand("File > New")`,
		`len(doc.Active().Requests())`,
	)
	if !strings.Contains(got, "The drawing has unsaved changes, which this would lose.") {
		t.Errorf("File > New on unsaved changes showed:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n1\n") {
		t.Errorf("the line went with the refused File > New:\n%s", got)
	}

	got = scriptConsole(t, ss,
		`doc.File.ForgetChanges()`,
		`doc.File.Unsaved()`,
		`doc.Edit.RunCommand("File > New")`,
		`len(doc.Active().Requests())`,
	)
	if got != "false\nnil\n0\n" {
		t.Errorf("File > New after ForgetChanges showed:\n%s", got)
	}

	// A command's own failure is the call's error too.
	if got := scriptConsole(t, ss, `doc.Edit.RunCommand("No Such Command")`); !strings.Contains(got, "no command") {
		t.Errorf("an unknown command showed:\n%s", got)
	}
}
