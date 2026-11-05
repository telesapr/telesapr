package tlspr_test

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"strings"
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
		{"constraint_missing_circle", nil,
			[]string{"deleted c001-diameter of g002-sketch-in-plane, which used r009"},
			2, 4, 0},
		{"group_missing_operand", nil,
			[]string{"deleted g003-extrude, which used g009"},
			2, 4, 0},
		{"lathe_missing_axis", nil,
			[]string{"deleted g003-lathe, which used r009"},
			2, 5, 0},
		{"mirror_missing_plane", nil,
			[]string{"deleted g004-mirror, which used r009"},
			3, 4, 0},
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

// What a damaged file's load leaves out is kept: a save writes it back,
// the saved file opens to the same model with the same left out, and a
// save of that is the same file again.
func TestDamagedSaveKeepsLeftOut(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file    string
		leftOut []string
	}{
		{"constraint_missing_entity", []string{"c001-pt-pt-distance of g002-sketch-in-plane"}},
		{"group_missing_workplane", []string{"g003-on-missing", "r008-datum-point of g003-on-missing"}},
		{"request_missing_group", []string{"r006-datum-point of g009"}},
		{"constraint_missing_circle", []string{"c001-diameter of g002-sketch-in-plane"}},
		{"group_missing_operand", []string{"g003-extrude"}},
		{"lathe_missing_axis", []string{"g003-lathe"}},
		{"mirror_missing_plane", []string{"g004-mirror"}},
		{"missing_references", nil},
		{"chamfer_missing_edge", []string{"edge of g004-chamfer: entity 3853 of g003-extrude"}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			path := fixtures.Path("tlspr/damaged/" + tc.file + ".tlspr")
			doc, err := tlspr.LoadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc.Sketch.GenerateAll()
			if got := doc.Sketch.LeftOutNames(); !slices.Equal(got, tc.leftOut) {
				t.Fatalf("left out %q, want %q", got, tc.leftOut)
			}
			saved := tlspr.Encode(doc.Sketch, tlspr.Options{})
			again, err := tlspr.Load(saved, path)
			if err != nil {
				t.Fatal(err)
			}
			again.Sketch.GenerateAll()
			if got := again.Sketch.LeftOutNames(); !slices.Equal(got, tc.leftOut) {
				t.Errorf("after a save, left out %q, want %q", got, tc.leftOut)
			}
			if len(again.Sketch.Groups) != len(doc.Sketch.Groups) ||
				len(again.Sketch.Requests) != len(doc.Sketch.Requests) ||
				len(again.Sketch.Constraints) != len(doc.Sketch.Constraints) {
				t.Error("the saved file opens to another model")
			}
			if resaved := tlspr.Encode(again.Sketch, tlspr.Options{}); string(resaved) != string(saved) {
				t.Error("a second save writes another file")
			}
		})
	}
}

// A damaged file saved and mended by hand opens whole: what was left out
// comes back once what it used is there.
func TestDamagedMendedByHand(t *testing.T) {
	t.Parallel()
	path := fixtures.Path("tlspr/damaged/group_missing_workplane.tlspr")
	doc, err := tlspr.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(tlspr.Encode(doc.Sketch, tlspr.Options{}))
	// The group was placed on the origin of a workplane the file
	// lacks; the mend places it on the reference XY's.
	mended := strings.Replace(saved, "origin 0x70001", "origin 0x10001", 1)
	if mended == saved {
		t.Fatal("the saved file doesn't hold the left-out group's origin")
	}
	opened, err := tlspr.Load([]byte(mended), path)
	if err != nil {
		t.Fatal(err)
	}
	sk := opened.Sketch
	sk.GenerateAll()
	if sk.LeftOut != nil || len(sk.BrokenReferences) != 0 {
		t.Errorf("left out %q, broken %v", sk.LeftOutNames(), sk.BrokenReferences)
	}
	if sk.Group(core.HGroup{V: 3}) == nil || sk.Request(core.HRequest{V: 8}) == nil {
		t.Error("the mended group and its request didn't come back")
	}
}

// openFixture opens a damaged fixture and regenerates it.
func openFixture(t *testing.T, name string) *sketch.Sketch {
	t.Helper()
	doc, err := tlspr.LoadFile(fixtures.Path("tlspr/damaged/" + name + ".tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	doc.Sketch.GenerateAll()
	return doc.Sketch
}

// lineOf is the line of a saved file that starts with prefix, after
// its indent, or "".
func lineOf(saved []byte, prefix string) string {
	for l := range strings.SplitSeq(string(saved), "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

// A chamfer's edge the file names and doesn't hold is left out of the
// chamfer and kept in the file, with its own distances, where it was in
// the list; the edges kept keep their own values, edited or not, and
// an edit of the edges lets it go.
func TestDamagedChamferEdgeKept(t *testing.T) {
	t.Parallel()
	sk := openFixture(t, "chamfer_missing_edge")
	hg := core.HGroup{V: 4}
	if g := sk.Group(hg); g == nil || len(g.Edges) != 3 || len(g.EdgeRadii) != 0 {
		t.Fatalf("the chamfer has %v, want the three edges the file holds and no edge's own distance", g)
	}
	if got := sk.LeftOutEdgeNames(hg); !slices.Equal(got, []string{"entity 3853 of g003-extrude"}) {
		t.Errorf("left-out edges %q", got)
	}
	saved := tlspr.Encode(sk, tlspr.Options{})
	for _, want := range []string{"edges 0x80030f0d 0x80030004 0x8003001f 0x80030016",
		"edge-distance 1 = 2", "edge-second-distance 1 = 1.5"} {
		if got := lineOf(saved, strings.Fields(want)[0]+" "); got != want {
			t.Errorf("saved %q, want %q", got, want)
		}
	}

	// An edge kept, the file's second, takes a distance of its own.
	if err := sk.SetEdgeDistanceText(hg, 0, "3"); err != nil {
		t.Fatal(err)
	}
	saved = tlspr.Encode(sk, tlspr.Options{})
	if !strings.Contains(string(saved), "edge-distance 1 = 2\n  edge-distance 2 = 3\n") {
		t.Errorf("after an edit of a kept edge's distance, saved\n%s", lineOf(saved, "edge-distance"))
	}

	// The edges picked again: the one dropped goes.
	g := sk.Group(hg)
	if err := sk.SetChamferEdges(hg, g.Edges[:2], nil); err != nil {
		t.Fatal(err)
	}
	if sk.LeftOutNames() != nil {
		t.Errorf("after the edges were picked again, left out %q", sk.LeftOutNames())
	}
	if got := lineOf(tlspr.Encode(sk, tlspr.Options{}), "edges "); got != "edges 0x80030004 0x8003001f" {
		t.Errorf("saved %q", got)
	}
}

// What a damaged file's load left out comes back when what it lacks is
// replaced by something the model holds, as a window's or a script's
// mend does; it is one undo step, which redo makes again.
func TestDamagedMendInModel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file          string
		missing, with core.HEntity
		back          func(sk *sketch.Sketch) bool
	}{
		{"group_missing_workplane", core.HEntity{V: 0x70001}, core.HEntity{V: 0x10001},
			func(sk *sketch.Sketch) bool {
				return sk.Group(core.HGroup{V: 3}) != nil && sk.Request(core.HRequest{V: 8}) != nil
			}},
		{"constraint_missing_entity", core.HEntity{V: 0x60000}, core.HEntity{V: 0x50000},
			func(sk *sketch.Sketch) bool { return sk.Constraint(core.HConstraint{V: 1}) != nil }},
		{"constraint_missing_circle", core.HEntity{V: 0x90000}, core.HEntity{V: 0x40000},
			func(sk *sketch.Sketch) bool { return sk.Constraint(core.HConstraint{V: 1}) != nil }},
		{"lathe_missing_axis", core.HEntity{V: 0x90000}, core.HEntity{V: 0x50000},
			func(sk *sketch.Sketch) bool {
				g := sk.Group(core.HGroup{V: 3})
				return g != nil && g.Predef.EntityB.V == 0x50000 && g.IsSolvedOkay()
			}},
		{"mirror_missing_plane", core.HEntity{V: 0x90000}, core.HEntity{V: 0x20000},
			func(sk *sketch.Sketch) bool {
				g := sk.Group(core.HGroup{V: 4})
				return g != nil && g.Predef.EntityB.V == 0x20000 && g.Problem.Msg == ""
			}},
		{"chamfer_missing_edge", core.HEntity{V: 0x80030f0d}, core.HEntity{V: 0x8003000d},
			func(sk *sketch.Sketch) bool {
				g := sk.Group(core.HGroup{V: 4})
				return len(g.Edges) == 4 && len(g.EdgeRadii) == 1 && g.EdgeRadii[0].Edge == 0
			}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			sk := openFixture(t, tc.file)
			if got := sk.LeftOutMissing(); !slices.Equal(got, []core.HEntity{tc.missing}) {
				t.Fatalf("missing %v, want %v", got, tc.missing)
			}
			before := string(tlspr.Encode(sk, tlspr.Options{}))
			if err := sk.UndoRememberFor(func() error { return sk.MendLeftOut(tc.missing, tc.with) }); err != nil {
				t.Fatal(err)
			}
			sk.GenerateAll()
			if !tc.back(sk) || sk.LeftOut != nil {
				t.Fatalf("not brought back; left out %q", sk.LeftOutNames())
			}
			after := string(tlspr.Encode(sk, tlspr.Options{}))
			sk.UndoUndo()
			if got := string(tlspr.Encode(sk, tlspr.Options{})); got != before {
				t.Error("undo didn't bring back the damaged model")
			}
			sk.UndoRedo()
			if got := string(tlspr.Encode(sk, tlspr.Options{})); got != after {
				t.Error("redo didn't make the mend again")
			}
		})
	}
}

// A mend is refused, changing nothing, for an entity nothing left out
// uses and for one of the wrong kind.
func TestDamagedMendRefused(t *testing.T) {
	t.Parallel()
	sk := openFixture(t, "group_missing_workplane")
	kept := sk.LeftOut
	if err := sk.MendLeftOut(core.HEntity{V: 0x10001}, core.HEntity{V: 0x10001}); !errors.Is(err, sketch.ErrNotMissing) {
		t.Errorf("mend of what isn't missing: %v", err)
	}
	// The group's origin is a point; the XY workplane is none.
	if err := sk.MendLeftOut(core.HEntity{V: 0x70001}, core.HEntity{V: 0x10000}); !errors.Is(err, sketch.ErrMendKind) {
		t.Errorf("mend with a workplane for a point: %v", err)
	}
	if sk.LeftOut != kept {
		t.Error("a refused mend changed what was left out")
	}
}

// What a damaged file's load left out says what kind each entity it
// lacks was, from how it was used, and a mend takes only that kind: a
// diameter's entity is a circle or an arc.
func TestDamagedMissingKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file    string
		missing core.HEntity
		kind    string
	}{
		{"group_missing_workplane", core.HEntity{V: 0x70001}, "a point"},
		{"constraint_missing_entity", core.HEntity{V: 0x60000}, "a point"},
		{"constraint_missing_circle", core.HEntity{V: 0x90000}, "a circle or an arc"},
		{"chamfer_missing_edge", core.HEntity{V: 0x80030f0d}, "a curve"},
		{"lathe_missing_axis", core.HEntity{V: 0x90000}, "a line or a normal"},
		{"mirror_missing_plane", core.HEntity{V: 0x90000}, "a workplane or a planar face"},
	} {
		if got := openFixture(t, tc.file).LeftOutMissingKind(tc.missing); got != tc.kind {
			t.Errorf("%s: kind %q, want %q", tc.file, got, tc.kind)
		}
	}
	sk := openFixture(t, "constraint_missing_circle")
	// The circle's centre is a point, not a circle.
	err := sk.MendLeftOut(core.HEntity{V: 0x90000}, core.HEntity{V: 0x40001})
	if !errors.Is(err, sketch.ErrMendKind) || !strings.Contains(err.Error(), "as a circle or an arc") {
		t.Errorf("mend of a circle with a point: %v", err)
	}
	// A lathe's axis is a line or a normal, not a circle; a mirror's
	// plane a workplane or a face, not a line.
	for _, tc := range []struct {
		file string
		with core.HEntity
		need string
	}{
		{"lathe_missing_axis", core.HEntity{V: 0x40000}, "as a line or a normal"},
		{"mirror_missing_plane", core.HEntity{V: 0x30000 + 0x20}, "as a workplane or a planar face"},
	} {
		sk := openFixture(t, tc.file)
		kept := sk.LeftOut
		err := sk.MendLeftOut(core.HEntity{V: 0x90000}, tc.with)
		if !errors.Is(err, sketch.ErrMendKind) || !strings.Contains(err.Error(), tc.need) {
			t.Errorf("%s: mend with %x: %v", tc.file, tc.with.V, err)
		}
		if sk.LeftOut != kept {
			t.Errorf("%s: a refused mend changed what was left out", tc.file)
		}
	}
}

// A group a damaged file lacks, a request's group or an extrusion's
// sketch, is mended by giving one of the model's groups in its place;
// what then has all it uses comes back, in one undo step that redo
// makes again.
func TestDamagedMendGroup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file string
		back func(sk *sketch.Sketch) bool
	}{
		{"request_missing_group", func(sk *sketch.Sketch) bool {
			r := sk.Request(core.HRequest{V: 6})
			return r != nil && r.Group == core.HGroup{V: 2}
		}},
		{"group_missing_operand", func(sk *sketch.Sketch) bool {
			g := sk.Group(core.HGroup{V: 3})
			return g != nil && g.OpA == core.HGroup{V: 2} && g.IsSolvedOkay()
		}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			sk := openFixture(t, tc.file)
			if got := sk.LeftOutMissingGroups(); !slices.Equal(got, []core.HGroup{{V: 9}}) {
				t.Fatalf("missing groups %v, want g009", got)
			}
			if got := sk.LeftOutMissingGroupName(core.HGroup{V: 9}); got != "g009" {
				t.Errorf("named %q", got)
			}
			before := string(tlspr.Encode(sk, tlspr.Options{}))
			if err := sk.UndoRememberFor(func() error {
				return sk.MendLeftOutGroup(core.HGroup{V: 9}, core.HGroup{V: 2})
			}); err != nil {
				t.Fatal(err)
			}
			sk.GenerateAll()
			if !tc.back(sk) || sk.LeftOut != nil {
				t.Fatalf("not brought back; left out %q", sk.LeftOutNames())
			}
			after := string(tlspr.Encode(sk, tlspr.Options{}))
			sk.UndoUndo()
			if got := string(tlspr.Encode(sk, tlspr.Options{})); got != before {
				t.Error("undo didn't bring back the damaged model")
			}
			sk.UndoRedo()
			if got := string(tlspr.Encode(sk, tlspr.Options{})); got != after {
				t.Error("redo didn't make the mend again")
			}
		})
	}
}

// A mend of a group is refused, changing nothing: for a group nothing
// left out uses, a group the model doesn't hold, the references for a
// request's group, and for an operand a group that doesn't come
// before the group using it.
func TestDamagedMendGroupRefused(t *testing.T) {
	t.Parallel()
	sk := openFixture(t, "request_missing_group")
	kept := sk.LeftOut
	if err := sk.MendLeftOutGroup(core.HGroup{V: 2}, core.HGroup{V: 2}); !errors.Is(err, sketch.ErrGroupNotMissing) {
		t.Errorf("mend of a group that isn't missing: %v", err)
	}
	if err := sk.MendLeftOutGroup(core.HGroup{V: 9}, core.HGroup{V: 7}); !errors.Is(err, sketch.ErrMendKind) {
		t.Errorf("mend with a group not in the model: %v", err)
	}
	if err := sk.MendLeftOutGroup(core.HGroup{V: 9}, core.HGroup{V: 1}); !errors.Is(err, sketch.ErrMendKind) {
		t.Errorf("mend with the references: %v", err)
	}
	if sk.LeftOut != kept {
		t.Error("a refused mend changed what was left out")
	}
	sk = openFixture(t, "group_missing_operand")
	later := sk.AddGroup(sketch.Group{Type: sketch.GroupDrawingWorkplane, Order: 5, Name: "later"})
	err := sk.MendLeftOutGroup(core.HGroup{V: 9}, later)
	if !errors.Is(err, sketch.ErrMendKind) || !strings.Contains(err.Error(), "a group before g003-extrude") {
		t.Errorf("mend of an operand with a later group: %v", err)
	}
}
