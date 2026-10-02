package slvs_test

import (
	"path/filepath"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
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
