package ui

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// Analyze > Group Dependencies says what the script's DependsOn, UsedBy,
// OwnSources and Bounds say of the active group.
func TestAnalyzeGroupDependencies(t *testing.T) {
	ss, doc := drawingModel(t)
	g := doc.Active()
	text := ss.groupDependenciesText(ss.GW.activeGroupH())
	deps := g.History().DependsOn()
	if len(deps) == 0 {
		t.Fatal("the hole depends on nothing")
	}
	for _, dep := range deps {
		if !strings.Contains(text, dep.Name()) {
			t.Errorf("the dialog leaves out %q, which the group depends on:\n%s", dep.Name(), text)
		}
	}
	lo, hi := g.Analyze().Bounds()
	for _, want := range []string{ss.MmToString(lo.X, false), ss.MmToString(hi.X, false), ss.MmToString(hi.Z, false)} {
		if !strings.Contains(text, want) {
			t.Errorf("the dialog leaves out %q of the box %v to %v:\n%s", want, lo, hi, text)
		}
	}
	// A group that something is built on says so.
	var used bool
	for _, o := range doc.Groups() {
		if len(o.History().UsedBy()) > 0 {
			used = true
			if text := ss.groupDependenciesText(o.H); !strings.Contains(text, "Used by:\n") {
				t.Errorf("%s is used, and its dialog has no list of what uses it:\n%s", o.Name(), text)
			}
		}
	}
	if !used {
		t.Error("no group is used by another")
	}
	p := ss.Platform.(*headless.Platform)
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	ss.MenuAnalyze(cmdGroupDependencies)
	if d := p.DialogsShown[len(p.DialogsShown)-1]; !strings.HasPrefix(d.Message, "Group ") {
		t.Errorf("message %q", d.Message)
	}
}
