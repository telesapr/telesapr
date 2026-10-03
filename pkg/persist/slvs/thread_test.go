package slvs

import (
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/thread"
)

// threadCase is a round part, extruded from a circle in the XY plane,
// and the thread made on that circle.
type threadCase struct {
	name string
	// radius and height of the part; a negative height extrudes it
	// downwards.
	radius, height float64
	designation    string
	o              sketch.ThreadOptions
}

var threadCases = []threadCase{
	{"screw_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12}},
	{"stud_m8", 4, 10, "M8", sketch.ThreadOptions{Length: 10, Through: true}},
	{"nut_m8", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true}},
	{"blind_m6", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true}},
	{"screw_m8_lh", 4, 30, "M8-LH", sketch.ThreadOptions{Length: 12}},
	{"screw_m6_down", 3, -20, "M6", sketch.ThreadOptions{Length: 10, Reverse: true}},
	{"nut_m6_down_lh", 6, -5, "M6x0.75-LH", sketch.ThreadOptions{Length: 5, Reverse: true, Through: true, Internal: true}},
	{"lead_tr20", 10, 40, "Tr20x8(P4)", sketch.ThreadOptions{Length: 30}},
	{"nut_tr20", 16, 20, "Tr20x8(P4)", sketch.ThreadOptions{Length: 20, Through: true, Internal: true}},
	{"screw_unc", 3.175, 20, "1/4-20 UNC", sketch.ThreadOptions{Length: 12, Clearance: 0.1}},
	{"nut_unc", 6, 5.5, "1/4-20 UNC", sketch.ThreadOptions{Length: 5.5, Through: true, Internal: true, Clearance: 0.1}},
	// A rod thicker than the thread is turned down to it.
	{"oversize_m5", 4, 20, "M5", sketch.ThreadOptions{Length: 10}},
	{"screw_m3", 1.5, 16, "M3", sketch.ThreadOptions{Length: 12}},
	{"triple_m12", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14}},
}

// build makes the part and its thread, with the given chord tolerance
// (0 for the default), and returns the sketch, the circle, and the
// group of the thread.
func (tc threadCase) build(t testing.TB, chordTolPct float64) (*sketch.Sketch, core.HEntity, sketch.ThreadResult) {
	t.Helper()
	sk := sketch.NewSketch()
	sk.ChordTolPct = chordTolPct
	sk.NewFile()
	sk.GenerateAll()
	g1 := sk.ActiveGroup()
	sk.Active = g1
	sk.GetGroup(g1).ActiveWorkplane = g1.Entity(0)

	circle := sk.AddRequestOfType(sketch.RequestCircle, false)
	sk.MustGetEntity(circle.Entity(1)).PointForceTo(sk, geom.Vector{})
	sk.MustGetEntity(circle.Entity(64)).DistanceForceTo(sk, tc.radius)
	sk.GenerateAllTyped(sketch.GenerateAll)

	ext := sketch.Group{Type: sketch.GroupExtrude, Subtype: sketch.GroupOneSided, Name: "extrude",
		OpA: g1, Visible: true, Scale: 1, Order: len(sk.Groups), Color: core.NewRgbaColor(100, 100, 100, 255)}
	ext.Predef.EntityB = g1.Entity(0)
	part := sk.AddGroup(ext)
	sk.Active = part
	sk.GenerateAllTyped(sketch.GenerateAll)
	// A one-sided extrusion is twice its params long.
	sk.MustGetParam(part.Param(0)).Val = 0
	sk.MustGetParam(part.Param(1)).Val = 0
	sk.MustGetParam(part.Param(2)).Val = tc.height / 2
	sk.GenerateAllTyped(sketch.GenerateAll)

	res, err := sk.AddThread(circle.Entity(0), tc.options(t))
	if err != nil {
		t.Fatal(err)
	}
	return sk, circle.Entity(0), res
}

func (tc threadCase) options(t testing.TB) sketch.ThreadOptions {
	t.Helper()
	spec, err := thread.Parse(tc.designation)
	if err != nil {
		t.Fatal(err)
	}
	o := tc.o
	o.Spec = spec
	return o
}

// meshStats triangulates a group's solid as the display does, and
// returns its mesh and how many naked or self-intersecting edges it
// has.
func meshStats(sk *sketch.Sketch, hg core.HGroup) (*srf.SMesh, int) {
	g := sk.GetGroup(hg)
	g.DisplayDirty = true
	g.GenerateDisplayItems(sk)
	var el srf.SEdgeList
	var inters, leaks bool
	srf.SKdNodeFrom(&g.DisplayMesh).MakeCertainEdgesInto(&el, srf.EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	bad := len(el.Edges)
	if bad == 0 && (inters || leaks || len(g.DisplayMesh.Triangles) == 0) {
		bad = 1
	}
	return &g.DisplayMesh, bad
}

// TestThreadSolids makes each thread and requires its four groups
// solved with nothing left to drag, a watertight solid, the volume of
// the turned or bored part with its ridges, and the crest of each
// ridge where the pitch and the hand of the thread put it.
func TestThreadSolids(t *testing.T) {
	for _, tc := range raceSample(threadCases) {
		t.Run(tc.name, func(t *testing.T) {
			// The default chord tolerance, of a thousandth of the part,
			// leaves a cylinder's mesh most of a percent short.
			sk, _, res := tc.build(t, 0.01)
			hel := res.Group
			if !res.Sound {
				t.Error("the solid isn't sound")
			}
			o := tc.options(t)
			s := o.Spec
			p := s.Profile(o.Internal, o.Clearance)

			order := sk.GetGroup(hel).Order
			var names []string
			for _, hg := range sk.GroupOrder {
				g := sk.GetGroup(hg)
				if g.Order <= order-4 {
					continue
				}
				names = append(names, g.Name)
				if g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 {
					t.Errorf("group %s: solved %v with %d degrees of freedom", g.Name, g.Solved.How, g.Solved.Dof)
				}
				if g.BooleanFailed {
					t.Errorf("group %s: Boolean failed", g.Name)
				}
			}
			what := "core"
			if o.Internal {
				what = "bore"
			}
			d := s.String()
			if want := []string{d + "-blank", d + "-" + what, d + "-profile", d + "-thread"}; strings.Join(names, " ") != strings.Join(want, " ") {
				t.Errorf("groups %q, want %q", names, want)
			}
			if sk.ActiveGroup() != hel {
				t.Errorf("the thread's group isn't the active one")
			}

			mesh, bad := meshStats(sk, hel)
			if bad != 0 {
				t.Fatalf("%d naked or self-intersecting edges", bad)
			}

			height := math.Abs(tc.height)
			top := o.Length
			if o.Through {
				top = height
			}
			want := math.Pi * tc.radius * tc.radius * height
			if o.Internal {
				want -= math.Pi * p.RootRadius * p.RootRadius * top
			} else {
				want -= math.Pi * (tc.radius*tc.radius - p.RootRadius*p.RootRadius) * top
			}
			// A ridge, swept: its area times the way its centroid goes.
			h := p.Height()
			area := (p.RootWidth + p.CrestWidth) / 2 * h
			centroid := h * (p.RootWidth + 2*p.CrestWidth) / (3 * (p.RootWidth + p.CrestWidth))
			if o.Internal {
				centroid = p.RootRadius - centroid
			} else {
				centroid = p.RootRadius + centroid
			}
			// The foot of the first ridge, which is drawn first in the
			// profile's sketch, gives where and how wide the ridges start.
			profile := sk.GroupOrder[order-1]
			var foot [2]geom.Vector
			for i := range sk.Requests {
				if r := &sk.Requests[i]; r.Group == profile {
					foot[0] = sk.MustGetEntity(r.H.Entity(1)).PointGetNum(sk)
					foot[1] = sk.MustGetEntity(r.H.Entity(2)).PointGetNum(sk)
					break
				}
			}
			baseW := math.Abs(foot[1].Z - foot[0].Z)
			sweep := o.Length - s.Pitch - float64(s.NumStarts()-1)*s.Pitch - baseW
			angle := sweep / s.Lead() * 2 * math.Pi
			want += float64(s.NumStarts()) * area * centroid * angle
			if got := mesh.CalculateVolume(); math.Abs(got-want) > 0.002*want {
				t.Errorf("volume %.4f, want %.4f", got, want)
			}

			// A right-hand thread rises, along the axis, as it turns
			// counterclockwise about it; the first ridge starts half a
			// pitch from the circle's plane.
			along, hand := 1.0, 1.0
			if o.Reverse {
				along = -1
			}
			if s.LeftHand {
				hand = -1
			}
			z0 := (foot[0].Z + foot[1].Z) / 2
			if math.Abs(z0-along*(s.Pitch/2+baseW/2)) > 1e-9 {
				t.Errorf("first ridge at %g along the axis, want %g", z0, along*(s.Pitch/2+baseW/2))
			}
			turned := math.Atan2(foot[0].Y, foot[0].X)
			between := 2 * math.Pi / float64(s.NumStarts())
			// The kernel's helix is off a true one by up to half a percent
			// of the lead.
			limit := (p.CrestWidth/2+0.006*s.Lead())/s.Lead()*2*math.Pi + 1e-6
			crests := 0
			for _, tr := range mesh.Triangles {
				for _, v := range []geom.Vector{tr.A, tr.B, tr.C} {
					if math.Abs(math.Hypot(v.X, v.Y)-p.CrestRadius) > 1e-6 ||
						along*v.Z < s.Pitch/4 || along*v.Z > o.Length-s.Pitch/4 {
						continue
					}
					crests++
					phase := (v.Z-z0)/s.Lead()*2*math.Pi - hand*(math.Atan2(v.Y, v.X)-turned)
					phase -= between * math.Round(phase/between)
					if math.Abs(phase) > limit {
						t.Fatalf("crest at (%.4f %.4f %.4f) is %.4f rad off the thread (limit %.4f)", v.X, v.Y, v.Z, phase, limit)
					}
				}
			}
			if crests < 100 {
				t.Errorf("%d vertices on the crests", crests)
			}
		})
	}
}

// threadGoldens are the cases that have a golden in testdata/thread:
// enough kinds of thread to pin the generator and the kernel on.
var threadGoldens = []string{"screw_m8", "nut_m8", "blind_m6", "screw_m8_lh", "screw_m6_down",
	"lead_tr20", "screw_unc", "triple_m12"}

// TestThreadGoldens saves the file a thread makes, opens it and saves
// it again, and requires the golden, testdata/thread/<name>.tlspr. Until
// the C++ comparison was frozen, the same steps gave the bytes C++
// saved after opening the file (testdata/thread/<name>.slvs, now plain
// fixtures). TELESAPR_REGEN_GOLDEN=1 rewrites the goldens.
func TestThreadGoldens(t *testing.T) {
	for _, tc := range raceSample(threadCases) {
		if !slices.Contains(threadGoldens, tc.name) {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			sk, _, _ := tc.build(t, 0)
			// Saved, which regenerates first.
			sk.GenerateAll()
			path := filepath.Join(t.TempDir(), tc.name+".tlspr")
			if err := tlspr.SaveFile(sk, "", path, tlspr.Options{}); err != nil {
				t.Fatal(err)
			}
			srf.SetChordTolMm(0)
			doc, err := tlspr.LoadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Opened, then saved: each regenerates.
			doc.Sketch.GenerateAll()
			doc.Sketch.GenerateAll()
			golden.Check(t, fixture("thread/"+tc.name+".tlspr"), tlspr.Encode(doc.Sketch, tlspr.Options{}))
		})
	}
}

// TestThreadErrors checks what AddThread refuses, and that it then
// leaves the sketch alone.
func TestThreadErrors(t *testing.T) {
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, circle, _ := tc.build(t, 0)
	groups := len(sk.Groups)
	m8 := tc.options(t)
	for _, c := range []struct {
		what string
		he   core.HEntity
		edit func(o *sketch.ThreadOptions)
	}{
		{"a point", sk.MustGetEntity(circle).Point[0], func(o *sketch.ThreadOptions) {}},
		{"no entity", core.HEntity{V: 0x7fffffff}, func(o *sketch.ThreadOptions) {}},
		{"no length", circle, func(o *sketch.ThreadOptions) { o.Length = 0 }},
		{"a negative length", circle, func(o *sketch.ThreadOptions) { o.Length = -5 }},
		{"a length without a turn", circle, func(o *sketch.ThreadOptions) { o.Length = 2 }},
		{"no pitch", circle, func(o *sketch.ThreadOptions) { o.Spec.Pitch = 0 }},
		{"too coarse a pitch", circle, func(o *sketch.ThreadOptions) { o.Spec.Pitch = 8 }},
		{"a clearance that leaves nothing", circle, func(o *sketch.ThreadOptions) { o.Clearance = 4 }},
		{"a clearance that isn't a number", circle, func(o *sketch.ThreadOptions) { o.Clearance = math.NaN() }},
	} {
		o := m8
		c.edit(&o)
		if _, err := sk.AddThread(c.he, o); err == nil {
			t.Errorf("%s: no error", c.what)
		}
		if len(sk.Groups) != groups {
			t.Fatalf("%s: groups were added", c.what)
		}
	}
}

// TestThreadAfterActiveGroup makes a thread while an earlier group is
// active: its groups follow that one, and the later ones move on.
func TestThreadAfterActiveGroup(t *testing.T) {
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, circle, res := tc.build(t, 0)
	first := res.Group
	part := sk.GroupOrder[2]
	sk.Active = part
	o := tc.options(t)
	o.Spec.Pitch = 1
	res, err := sk.AddThread(circle, o)
	if err != nil {
		t.Fatal(err)
	}
	second := res.Group
	var names []string
	for i, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		if g.Order != i {
			t.Errorf("group %s has order %d at place %d", g.Name, g.Order, i)
		}
		names = append(names, g.Name)
	}
	want := "#references sketch-in-plane extrude M8x1-blank M8x1-core M8x1-profile M8x1-thread " +
		"M8-blank M8-core M8-profile M8-thread"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("groups %q, want %q", got, want)
	}
	if sk.ActiveGroup() != second || sk.GetGroup(first).Order != 10 {
		t.Errorf("active group %v, first thread at %d", sk.ActiveGroup(), sk.GetGroup(first).Order)
	}
}

// TestThreadLengthDimension changes the dimension that a thread's helix
// group carries: the helix follows it, at the same pitch.
func TestThreadLengthDimension(t *testing.T) {
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	hel := res.Group
	var length *core.Constraint
	for i := range sk.Constraints {
		if c := &sk.Constraints[i]; c.Group == hel {
			if length != nil {
				t.Fatal("the helix group has more than one constraint")
			}
			length = c
		}
	}
	if length == nil || length.Type != core.ConstraintProjPtDistance {
		t.Fatalf("the helix group's constraint is %+v", length)
	}
	before := length.ValA
	length.ValA = math.Copysign(5, before)
	sk.MarkGroupDirty(hel, false)
	sk.GenerateAllTyped(sketch.GenerateAll)
	g := sk.GetGroup(hel)
	if g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 {
		t.Fatalf("solved %v with %d degrees of freedom", g.Solved.How, g.Solved.Dof)
	}
	dist := 2 * sk.MustGetParam(hel.Param(7)).Val
	angle := 4 * sk.MustGetParam(hel.Param(3)).Val
	if math.Abs(dist-5) > 1e-9 || math.Abs(angle-5/1.25*2*math.Pi) > 1e-9 {
		t.Errorf("helix of %g mm and %g rad, want 5 mm and %g rad (it was %g mm)", dist, angle, 5/1.25*2*math.Pi, before)
	}
	if _, bad := meshStats(sk, hel); bad != 0 {
		t.Errorf("%d naked or self-intersecting edges", bad)
	}
}

// threadSweepCases are threadCases and, for each of several metric
// sizes, a blind and a through screw, a nut and a tapped hole.
func threadSweepCases() []threadCase {
	cases := append([]threadCase(nil), threadCases...)
	for _, d := range []float64{3, 4, 5, 6, 8, 10, 12, 16, 20} {
		name := "M" + strconv.FormatFloat(d, 'f', -1, 64)
		cases = append(cases,
			threadCase{name + " blind", d / 2, 3 * d, name, sketch.ThreadOptions{Length: 1.5 * d}},
			threadCase{name + " through", d / 2, 1.5 * d, name, sketch.ThreadOptions{Length: 1.5 * d, Through: true}},
			threadCase{name + " nut", d, 0.8 * d, name, sketch.ThreadOptions{Length: 0.8 * d, Through: true, Internal: true}},
			threadCase{name + " tapped", d, 2 * d, name, sketch.ThreadOptions{Length: 1.2 * d, Internal: true}})
	}
	return cases
}

// TestThreadSweep makes threads of many kinds and sizes at chord
// tolerances from coarse to fine, and requires a sound solid from
// nearly all: an even sample by default, the whole grid with
// TELESAPR_SWEEP=1 (two minutes). AddThread generates each thread
// once, in the first placement of its sketch planes; the target fixed
// for package 5.3 of ROADMAP-JUMP.md was no unsound solid at the
// default chord tolerance (0.1%) or at five times it.
func TestThreadSweep(t *testing.T) {
	cases := threadSweepCases()
	byName := map[string]threadCase{}
	var names []string
	for _, tc := range cases {
		byName[tc.name] = tc
		names = append(names, tc.name)
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("tol", 0.5, 0.3, 0.2, 0.1, 0.05, 0.03, 0.02, 0.01),
			sweep.Strings("case", names...),
		},
		MaxFailRate:       threadSweepMaxFailRate,
		Sample:            16,
		SampleMaxFailRate: 1.0 / 16,
	}
	grid.Run(func(p sweep.Point) error {
		_, _, res := byName[p.String("case")].build(t, p.Float("tol"))
		if !res.Sound {
			return errors.New("not sound")
		}
		return nil
	}).Require(t)
}

// threadSweepMaxFailRate is the fraction of TestThreadSweep's grid
// allowed to be unsound: what the whole grid gave on 2026-10-02 after the
// raycast fix, 3 of 400, at 0.03% and 0.01%, none at 0.05% or coarser.
// Before it, AddThread turned the planes up to seven times and 24 were
// unsound; generated once, without the fix, 68 were, 27 of them at 0.5%.
// The sample of 16 had none.
const threadSweepMaxFailRate = 3.0 / 400

// TestThreadGoldensSound checks the solid of every thread golden, the
// .tlspr ones and the C++ saves, with the invariant oracle.
func TestThreadGoldensSound(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "testdata", "thread", "*.s[lp]?s"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no thread goldens: %v", err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			var sk *sketch.Sketch
			if strings.HasSuffix(f, ".tlspr") {
				doc, err := tlspr.LoadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				sk = doc.Sketch
			} else {
				doc, err := LoadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				sk = doc.Sketch
			}
			sk.GenerateAll()
			last := sk.GetGroup(sk.GroupOrder[len(sk.GroupOrder)-1])
			if r := check.Group(sk, last); !r.Sound() || r.Empty() {
				t.Errorf("%s: %s", last.Name, r.String())
			}
		})
	}
}
