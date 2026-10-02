package slvs_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/internal/sweep"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// loadFixture loads a fixture as a fresh process would, or returns nil
// for the one that does not load on purpose.
func loadFixture(t *testing.T, fixture string) *sketch.Sketch {
	t.Helper()
	srf.SetChordTolMm(0)
	doc, err := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(fixture)))
	if err != nil {
		return nil
	}
	return doc.Sketch
}

// TestGroupDependenciesOnFixtures requires of every group of every
// fixture that it depends only on groups before it, as the regeneration
// that saved the fixture generated them in that order.
func TestGroupDependenciesOnFixtures(t *testing.T) {
	for _, f := range invariantFixtures(t) {
		sk := loadFixture(t, f)
		if sk == nil {
			continue
		}
		groups := 0
		for _, g := range sk.Groups {
			for _, d := range sk.GroupDependencies(g.H) {
				dg := sk.GetGroup(d.Group)
				if dg == nil {
					t.Errorf("%s: %s depends on missing g%03x (%s)", f, g.DescriptionString(), d.Group.V, d.Why)
					continue
				}
				if dg.Order >= g.Order {
					t.Errorf("%s: %s depends on later %s (%s)", f, g.DescriptionString(), dg.DescriptionString(), d.Why)
				}
			}
			groups++
		}
		if groups < 2 {
			t.Errorf("%s: %d groups", f, groups)
		}
	}
}

// move is one move of a group in a fixture's history.
type move struct{ h, before core.HGroup }

// TestMoveGroupSweep tries every move of every group of every fixture.
// A move MoveGroup refuses must leave the file as it was; moving a
// group where it allows, then back, must give the original file. An
// allowed move must also regenerate without pruning anything, which
// would mean GroupDependencies missed a reference. Regenerating is
// slow, so by default one allowed move per fixture regenerates, and
// every one under SPACESOLVE_SWEEP=1.
func TestMoveGroupSweep(t *testing.T) {
	allowed, refused, regenerated := 0, 0, 0
	for fi, f := range invariantFixtures(t) {
		if slvs.RaceDetector && fi%5 != 0 {
			continue
		}
		sk := loadFixture(t, f)
		if sk == nil {
			continue
		}
		original := spcs.Encode(sk, spcs.Options{})
		order := append([]core.HGroup(nil), sk.GroupOrder...)
		var legal []move
		for i, h := range order {
			// Where the group goes back to: before the group after it.
			back := core.HGroup{}
			if i+1 < len(order) {
				back = order[i+1]
			}
			for _, before := range append(order, core.HGroup{}) {
				if before == h || before == back {
					continue
				}
				err := sk.MoveGroup(h, before)
				if err != nil {
					refused++
					if got := spcs.Encode(sk, spcs.Options{}); !bytes.Equal(got, original) {
						t.Fatalf("%s: the refused move of g%03x before g%03x (%v) changed the file", f, h.V, before.V, err)
					}
					if check := sk.CheckMoveGroup(h, before); check == nil || check.Error() != err.Error() {
						t.Errorf("%s: CheckMoveGroup says %v, MoveGroup %v", f, check, err)
					}
					continue
				}
				allowed++
				legal = append(legal, move{h, before})
				if err := sk.MoveGroup(h, back); err != nil {
					t.Fatalf("%s: can't move g%03x back: %v", f, h.V, err)
				}
				if got := spcs.Encode(sk, spcs.Options{}); !bytes.Equal(got, original) {
					t.Fatalf("%s: moving g%03x before g%03x and back changed the file: %s",
						f, h.V, before.V, firstLineDiff(original, got))
				}
			}
		}

		for k, m := range legal {
			if !sweep.Full() && k != fi%len(legal) {
				continue
			}
			regenerated++
			sk := loadFixture(t, f)
			sk.GenerateAll()
			if err := sk.MoveGroup(m.h, m.before); err != nil {
				t.Fatal(err)
			}
			r, c, g := len(sk.Requests), len(sk.Constraints), len(sk.Groups)
			sk.GenerateAll()
			if len(sk.Requests) != r || len(sk.Constraints) != c || len(sk.Groups) != g {
				t.Errorf("%s: moving g%03x before g%03x pruned %d requests, %d constraints, %d groups",
					f, m.h.V, m.before.V, r-len(sk.Requests), c-len(sk.Constraints), g-len(sk.Groups))
			}
		}
	}
	t.Logf("%d moves allowed, %d refused, %d regenerated", allowed, refused, regenerated)
	if allowed == 0 || refused == 0 {
		t.Fatal("the sweep tried nothing")
	}
}

// firstLineDiff describes the first line where a and b differ.
func firstLineDiff(a, b []byte) string {
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return fmt.Sprintf("line %d: %q, then %q", i+1, al[i], bl[i])
		}
	}
	return fmt.Sprintf("%d lines, then %d", len(al), len(bl))
}
