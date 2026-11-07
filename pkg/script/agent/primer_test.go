package agent

import "testing"

// TestPrimerNamesRealCalls checks that every call the hand-written
// primer and instructions name exists in package api.
func TestPrimerNamesRealCalls(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{"Instructions": Instructions, "Primer": Primer()} {
		if bad := UnknownCalls(text); len(bad) > 0 {
			t.Errorf("%s names calls that do not exist: %v", name, bad)
		}
	}
}

// TestUnknownCalls checks that the check sees a wrong call.
func TestUnknownCalls(t *testing.T) {
	t.Parallel()
	const text = `a := g.Analyze()  a.Analyze().Volume()  g.SolidEdgez()  doc.Features.Extrudes(1)  "Group.SetCombine" "Features.Nope" Features > No Such Entry`
	bad := UnknownCalls(text)
	for _, want := range []string{"g.SolidEdgez", "doc.Features.Extrudes", `"Features.Nope"`, "Features > No Such Entry"} {
		found := false
		for _, b := range bad {
			found = found || b == want
		}
		if !found {
			t.Errorf("%q is not reported in %v", want, bad)
		}
	}
	if len(bad) != 5 {
		t.Errorf("reported %v, want 4 wrong calls and a.Analyze", bad)
	}
}
