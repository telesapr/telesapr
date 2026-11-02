package tlspr_test

import (
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// damage is one way of breaking a model's file: an item taken out of it
// raw, leaving what referred to it dangling, as a file damaged by hand
// or by another program would.
type damage struct {
	name string
	cut  func(sk *sketch.Sketch)
}

// Handles no model has: a group, an entity of a request, an entity a
// group makes, and a style.
var (
	noGroup       = core.HGroup{V: 0x7ffe}
	noEntity      = core.HEntity{V: 0x7ffd0001}
	noGroupEntity = core.HEntity{V: 0x8ffc0000}
	noStyle       = core.HStyle{V: 0x7ffb}
)

// damages are the ways of breaking sk: each group and each request
// taken out, one at a time, and each of a group's, a request's, a
// constraint's and a datum's references pointed at what isn't there.
func damages(sk *sketch.Sketch) []damage {
	var out []damage
	group := func(h core.HGroup, what string, f func(g *sketch.Group)) {
		out = append(out, damage{fmt.Sprintf("group %x %s", h.V, what), func(sk *sketch.Sketch) { f(sk.Group(h)) }})
	}
	for _, g := range sk.Groups {
		h := g.H
		out = append(out, damage{fmt.Sprintf("group %x", h.V), func(sk *sketch.Sketch) {
			sk.Groups = slices.DeleteFunc(sk.Groups, func(g sketch.Group) bool { return g.H == h })
			sk.GroupOrder = slices.DeleteFunc(sk.GroupOrder, func(o core.HGroup) bool { return o == h })
		}})
		if h.V == core.HGroupReferences {
			continue
		}
		if g.OpA.V != 0 {
			group(h, "op-a", func(g *sketch.Group) { g.OpA = noGroup })
		}
		if g.OpB.V != 0 {
			group(h, "op-b", func(g *sketch.Group) { g.OpB = noGroup })
		}
		if g.Predef.Origin != core.NoEntity {
			group(h, "origin", func(g *sketch.Group) { g.Predef.Origin = noEntity })
		}
		if g.Predef.EntityB != core.NoEntity {
			group(h, "entity-b", func(g *sketch.Group) { g.Predef.EntityB = noGroupEntity })
		}
		group(h, "workplane", func(g *sketch.Group) { g.ActiveWorkplane = noGroupEntity })
		if len(g.Edges) > 0 {
			group(h, "edge", func(g *sketch.Group) { g.Edges[0] = noEntity })
		}
		if len(g.LoftThrough) > 0 {
			group(h, "loft-through", func(g *sketch.Group) { g.LoftThrough[0] = noGroup })
		}
		if len(g.DraftFaces) > 0 {
			group(h, "draft-face", func(g *sketch.Group) { g.DraftFaces[0] = noEntity })
		}
	}
	for _, r := range sk.Requests {
		h := r.H
		out = append(out,
			damage{fmt.Sprintf("request %x", h.V), func(sk *sketch.Sketch) {
				sk.Requests = slices.DeleteFunc(sk.Requests, func(r sketch.Request) bool { return r.H == h })
			}},
			damage{fmt.Sprintf("request %x workplane", h.V), func(sk *sketch.Sketch) { sk.Request(h).Workplane = noGroupEntity }},
			damage{fmt.Sprintf("request %x group", h.V), func(sk *sketch.Sketch) { sk.Request(h).Group = noGroup }},
			damage{fmt.Sprintf("request %x style", h.V), func(sk *sketch.Sketch) { sk.Request(h).Style = noStyle }})
	}
	for _, c := range sk.Constraints {
		h := c.H
		out = append(out,
			damage{fmt.Sprintf("constraint %x point", h.V), func(sk *sketch.Sketch) { sk.Constraint(h).PtA = noEntity }},
			damage{fmt.Sprintf("constraint %x entity", h.V), func(sk *sketch.Sketch) { sk.Constraint(h).EntityA = noGroupEntity }},
			damage{fmt.Sprintf("constraint %x style", h.V), func(sk *sketch.Sketch) { sk.Constraint(h).Disp.Style = noStyle }})
	}
	for i := range sk.Datums {
		out = append(out, damage{fmt.Sprintf("datum %d", i), func(sk *sketch.Sketch) { sk.Datums[i].Entity = noEntity }})
	}
	return out
}

// openDamaged writes sk with d done to it, opens what was written and
// regenerates it with a solve, as the window does; it reports a panic as
// an error.
func openDamaged(path string, data []byte, d damage) (err error) {
	doc, err := tlspr.Decode(data)
	if err != nil {
		return err
	}
	d.cut(doc.Sketch)
	damaged := tlspr.Encode(doc.Sketch, tlspr.Options{})
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic: %v\n%s", rec, debug.Stack())
		}
	}()
	opened, err := tlspr.Load(damaged, path)
	if err != nil {
		return err
	}
	opened.Sketch.GenerateAll()
	return nil
}

// damageEvery thins the damages of each fixture to one in so many,
// a different one in each fixture, unless the whole sweep is asked for:
// all of them take some eight minutes.
const damageEvery = 16

// A file with an item cut out of it or a reference pointed at nothing
// opens: what can't be made is left out and said, never a panic or an
// error.
func TestOpenDamaged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("opens the fixtures with their groups and requests cut out, half a minute")
	}
	paths, err := fixtures.Walk(".tlspr")
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range fixtures.Sample(paths) {
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			path := fixtures.Path(p)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := tlspr.Decode(data)
			if err != nil {
				t.Fatal(err)
			}
			for j, d := range damages(doc.Sketch) {
				if !sweep.Full() && (i+j)%damageEvery != 0 {
					continue
				}
				if err := openDamaged(path, data, d); err != nil {
					t.Errorf("%s cut out: %v", d.name, err)
				}
			}
		})
	}
}

// The damaged fixtures open, each with what it lacked said: what was
// put back, and what used what is missing and was left out.
func TestOpenDamagedFixtures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file           string
		mended, broken []string
		groups         int
		requests       int
		constraints    int
	}{
		{"constraint_missing_entity", nil,
			[]string{"deleted c001-pt-pt-distance of g002-sketch-in-plane, which used r006"},
			2, 5, 1},
		{"group_missing_workplane", nil,
			[]string{"deleted g003-on-missing, which used r007"},
			2, 5, 0},
		{"request_missing_group", nil,
			[]string{"deleted r006-datum-point of g009, which used g009"},
			2, 5, 0},
		{"missing_references", []string{
			"the reference workplane YZ was missing and was made again",
			"the reference workplane ZX was missing and was made again"}, nil,
			2, 5, 0},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			doc, err := tlspr.LoadFile(fixtures.Path("tlspr/damaged/" + tc.file + ".tlspr"))
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			sk.GenerateAll()
			var broken []string
			for _, b := range sk.BrokenReferences {
				broken = append(broken, b.String())
			}
			if !slices.Equal(sk.Mended, tc.mended) || !slices.Equal(broken, tc.broken) {
				t.Errorf("mended %q, broken %q", sk.Mended, broken)
			}
			if len(sk.Groups) != tc.groups || len(sk.Requests) != tc.requests || len(sk.Constraints) != tc.constraints {
				t.Errorf("%d groups, %d requests, %d constraints", len(sk.Groups), len(sk.Requests), len(sk.Constraints))
			}
			if g := sk.Group(core.HGroup{V: 2}); g == nil || !g.IsSolvedOkay() {
				t.Error("the sketch the damage left alone didn't solve")
			}
		})
	}
}
