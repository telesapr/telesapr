package script

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/script/api"
)

var (
	scriptDeclaration = regexp.MustCompile(`(?m)^\t(\w+) := `)
	scriptString      = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	scriptIdentifier  = regexp.MustCompile(`[A-Za-z_]\w*`)
	// threadGroupName is a name a thread's call gives its groups.
	threadGroupName = regexp.MustCompile(`\.SetName\("[^"]*-(blank|core|bore|profile|thread|end|chamfer|cosmetic)(-[0-9]+)?"\)`)
)

// unusedVariables are the variables a script declares that nothing
// after the declaration reads.
func unusedVariables(src string) []string {
	var unused []string
	text := scriptString.ReplaceAllString(src, `""`)
	for _, m := range scriptDeclaration.FindAllStringSubmatch(text, -1) {
		n := 0
		for _, id := range scriptIdentifier.FindAllString(text, -1) {
			if id == m[1] {
				n++
			}
		}
		if n < 2 {
			unused = append(unused, m[1])
		}
	}
	return unused
}

// The script of a model with a thread declares no variable that nothing
// reads, and sets no name that the thread's call gives already, and it
// still builds the model again: modelled threads and cosmetic ones, on
// a circle and on a face's edge, with their sketches' requests named
// only where a constraint refers to them.
func TestScriptThreadTidy(t *testing.T) {
	t.Parallel()
	paths := slices.Concat(roundTripKeptThreads, []string{
		"thread/cosmetic_screw_m10.tlspr", "thread/cosmetic_nut_m8.tlspr", "thread/triple_m12.tlspr",
	})
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			d, err := api.Open(fixtures.Path(p))
			if err != nil {
				t.Fatal(err)
			}
			src, _ := d.Script()
			if u := unusedVariables(src); len(u) > 0 {
				t.Errorf("declared and not used: %v\n%s", u, src)
			}
			if m := threadGroupName.FindString(src); m != "" {
				t.Errorf("a name the call gives is set: %s\n%s", m, src)
			}
			if diff, _ := roundTrip(t, fixtures.Path(p)); diff != "" {
				t.Errorf("does not come back: %s", diff)
			}
		})
	}
}

// A thread group the user renamed keeps its SetName in the script, the
// others have none, and the datum of a thread on a face is told the
// same way.
func TestScriptThreadTidyKeepsRenames(t *testing.T) {
	t.Parallel()
	d, groups := threadedPlate(t)
	groups[2].SetName("my-ridge")
	d.Group("M10-end").SetName("my-datum")
	made, _, src := rebuildNoted(t, d)
	if n := strings.Count(src, ".SetName("); n != 2 {
		t.Errorf("%d names set, want the two renamed:\n%s", n, src)
	}
	for _, name := range []string{"my-ridge", "my-datum"} {
		if !strings.Contains(src, `SetName("`+name+`")`) {
			t.Errorf("%s is not set:\n%s", name, src)
		}
	}
	if got, want := groupNames(made), groupNames(d); !slices.Equal(got, want) {
		t.Errorf("groups %q, want %q\n%s", got, want, src)
	}
}

// The groups of a chamfer, and the datum of a thread on a face, are
// named by the call too: a script of one sets no name.
func TestScriptThreadTidyChamfer(t *testing.T) {
	t.Parallel()
	d, groups := threadedPlateWith(t, api.ThreadOptions{Length: 6, Chamfer: true})
	if len(groups) != 6 {
		t.Fatalf("the thread has %d groups, want 6 with its chamfer", len(groups))
	}
	made, _, src := rebuildNoted(t, d)
	if n := strings.Count(src, ".SetName("); n != 0 {
		t.Errorf("%d names set, want none:\n%s", n, src)
	}
	if u := unusedVariables(src); len(u) > 0 {
		t.Errorf("declared and not used: %v\n%s", u, src)
	}
	if got, want := groupNames(made), groupNames(d); !slices.Equal(got, want) {
		t.Errorf("groups %q, want %q\n%s", got, want, src)
	}
}
