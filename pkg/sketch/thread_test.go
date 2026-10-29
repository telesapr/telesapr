package sketch_test

import (
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
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
	{"screw_bsw", 4.7625, 25, "3/8 BSW", sketch.ThreadOptions{Length: 15}},
	{"nut_g", 16, 10, "G1/2", sketch.ThreadOptions{Length: 10, Through: true, Internal: true}},
	{"screw_acme", 6.35, 30, "1/2-10 ACME", sketch.ThreadOptions{Length: 20}},
	{"nut_acme_lh", 12, 12, "1/2-10 ACME-LH", sketch.ThreadOptions{Length: 12, Through: true, Internal: true}},
	{"screw_m10_fine", 5, 25, "M10x1", sketch.ThreadOptions{Length: 15}},
	// A class for the clearance, and a phase.
	{"screw_m10_6g", 5, 25, "M10", sketch.ThreadOptions{Length: 15, Class: "6g", Phase: 120}},
	{"nut_m10_6h", 10, 10, "M10", sketch.ThreadOptions{Length: 10, Through: true, Internal: true, Class: "6H", Phase: 300}},
	// Buttress threads, whose ridges lean: DIN 513 and ASME B1.9.
	{"screw_s20", 10, 40, "S20x4", sketch.ThreadOptions{Length: 30}},
	{"nut_s20", 16, 20, "S20x4", sketch.ThreadOptions{Length: 20, Through: true, Internal: true}},
	{"nut_s20_back", 16, 20, "S20x4", sketch.ThreadOptions{Length: 20, Through: true, Internal: true, LoadFlankBack: true}},
	{"screw_butt", 12.7, 40, "1-12 BUTT", sketch.ThreadOptions{Length: 25, Clearance: 0.05}},
	{"nut_butt_lh", 20, 15, "1-12 BUTT-LH", sketch.ThreadOptions{Length: 15, Through: true, Internal: true, Clearance: 0.05}},
	// B1.9's rounded roots, and the load's way in the designation.
	// A coarse pitch on a short part, which the default chord tolerance
	// takes.
	{"screw_butt_round", 9.525, 20, "3/4-10 PULL-BUTT", sketch.ThreadOptions{Length: 12, Clearance: 0.05, RoundRoot: true}},
	{"nut_butt_round", 20, 15, "1-12 BUTT", sketch.ThreadOptions{Length: 15, Through: true, Internal: true, Clearance: 0.05, RoundRoot: true}},
}

// build makes the part and its thread, with the given chord tolerance
// (0 for the default), and returns the sketch, the circle, and the
// group of the thread.
func (tc threadCase) build(t testing.TB, chordTolPct float64) (*sketch.Sketch, core.HEntity, sketch.ThreadResult) {
	t.Helper()
	sk, circle, res, err := tc.tryBuild(t, chordTolPct)
	if err != nil {
		t.Fatal(err)
	}
	return sk, circle, res
}

// tryBuild is build, returning AddThread's error.
func (tc threadCase) tryBuild(t testing.TB, chordTolPct float64) (*sketch.Sketch, core.HEntity, sketch.ThreadResult, error) {
	t.Helper()
	sk := sketch.NewSketch()
	sk.ChordTolPct = chordTolPct
	sk.NewFile()
	sk.GenerateAll()
	g1 := sk.ActiveGroup()
	sk.Active = g1
	sk.Group(g1).ActiveWorkplane = g1.Entity(0)

	circle := sk.AddRequestOfType(sketch.RequestCircle, false)
	sk.MustEntity(circle.Entity(1)).PointForceTo(sk, geom.Vector{})
	sk.MustEntity(circle.Entity(64)).DistanceForceTo(sk, tc.radius)
	sk.GenerateAllTyped(sketch.GenerateAll)

	ext := sketch.Group{Type: sketch.GroupExtrude, Subtype: sketch.GroupOneSided, Name: "extrude",
		OpA: g1, Visible: true, Scale: 1, Order: len(sk.Groups), Color: core.NewRGBA(100, 100, 100, 255)}
	ext.Predef.EntityB = g1.Entity(0)
	part := sk.AddGroup(ext)
	sk.Active = part
	sk.GenerateAllTyped(sketch.GenerateAll)
	// A one-sided extrusion is twice its params long.
	sk.MustParam(part.Param(0)).Val = 0
	sk.MustParam(part.Param(1)).Val = 0
	sk.MustParam(part.Param(2)).Val = tc.height / 2
	sk.GenerateAllTyped(sketch.GenerateAll)

	res, err := sk.AddThread(circle.Entity(0), tc.options(t))
	return sk, circle.Entity(0), res, err
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
	g := sk.Group(hg)
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
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	for _, tc := range fixtures.Sample(threadCases) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// The default chord tolerance, of a thousandth of the part,
			// leaves a cylinder's mesh most of a percent short.
			sk, _, res := tc.build(t, 0.01)
			hel := res.Group
			if !res.Sound {
				t.Error("the solid isn't sound")
			}
			o := tc.options(t)
			s := o.Spec
			clearance, err := o.FitClearance()
			if err != nil {
				t.Fatal(err)
			}
			p := s.Profile(o.Internal, clearance)
			if o.LoadFlankBack || s.Load == thread.LoadPull {
				p = p.Turned()
			}
			if o.RoundRoot {
				p.RootRoundRadius = s.RootRound()
			}

			order := sk.Group(hel).Order
			var names []string
			for _, hg := range sk.GroupOrder {
				g := sk.Group(hg)
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
			// The groups are named for the thread, the load's way kept in
			// its load flank.
			named := s
			named.Load = thread.LoadUnstated
			d := named.String()
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
					foot[0] = sk.MustEntity(r.H.Entity(1)).PointGetNum(sk)
					foot[1] = sk.MustEntity(r.H.Entity(2)).PointGetNum(sk)
					break
				}
			}
			baseW := math.Abs(foot[1].Z - foot[0].Z)
			// A rounded root is two arcs a ridge, of the standard's radius.
			arcs := 0
			for i := range sk.Requests {
				if r := &sk.Requests[i]; r.Group == profile && r.Type == sketch.RequestArcOfCircle {
					arcs++
				}
			}
			if want := 2 * s.NumStarts(); p.RootRoundRadius > 0 && arcs != want {
				t.Errorf("%d arcs round the roots, want %d", arcs, want)
			}
			sweep := o.Length - s.Pitch - float64(s.NumStarts()-1)*s.Pitch - baseW
			// A phase moves the ridges by that part of the lead, within a
			// quarter of a pitch down and three up, in a pitch more room.
			shift := 0.0
			if o.Phase != 0 {
				sweep -= s.Pitch
				shift = s.Lead() * o.Phase / 360
				shift -= s.Pitch * math.Floor((shift+s.Pitch/4)/s.Pitch)
			}
			angle := sweep / s.Lead() * 2 * math.Pi
			want += float64(s.NumStarts()) * area * centroid * angle
			if p.RootRoundRadius > 0 {
				lower, upper := p.RootArcs()
				for _, a := range []thread.RootArc{lower, upper} {
					fa, fc := filletArea(a)
					want += float64(s.NumStarts()) * fa * fc * angle
				}
			}
			if got := mesh.CalculateVolume(); math.Abs(got-want) > 0.002*want {
				t.Errorf("volume %.4f, want %.4f", got, want)
			}

			// A right-hand thread rises, along the axis, as it turns
			// counterclockwise about it; the first ridge starts half a
			// pitch from the circle's plane, and a phase's shift past that.
			along, hand := 1.0, 1.0
			if o.Reverse {
				along = -1
			}
			if s.LeftHand {
				hand = -1
			}
			z0 := (foot[0].Z + foot[1].Z) / 2
			if want := along * (s.Pitch/2 + baseW/2 + shift); math.Abs(z0-want) > 1e-9 {
				t.Errorf("first ridge at %g along the axis, want %g", z0, want)
			}
			// A buttress ridge's crest lies off its foot's middle; a rounded
			// root's foot is under the ends of its tangents.
			footShift := p.MiddleShift(math.Hypot(foot[0].X, foot[0].Y))
			if p.RootRoundRadius > 0 {
				lower, upper := p.RootArcs()
				footShift = (lower.End[1] + upper.End[1]) / 2
			}
			z0 -= along * footShift
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

// filletArea is the area that a rounded root adds to a ridge beyond
// the root, between its flank, the arc and the arc's tangent, and the
// distance of its centroid from the axis.
func filletArea(a thread.RootArc) (area, centroid float64) {
	pts := [][2]float64{a.Foot, a.Cross, a.Turn}
	from := math.Atan2(a.Turn[1]-a.Centre[1], a.Turn[0]-a.Centre[0])
	to := math.Atan2(a.Tangent[1]-a.Centre[1], a.Tangent[0]-a.Centre[0])
	if to-from > math.Pi {
		to -= 2 * math.Pi
	} else if from-to > math.Pi {
		to += 2 * math.Pi
	}
	r := math.Hypot(a.Turn[0]-a.Centre[0], a.Turn[1]-a.Centre[1])
	const n = 200
	for i := 1; i <= n; i++ {
		t := from + (to-from)*float64(i)/n
		pts = append(pts, [2]float64{a.Centre[0] + r*math.Cos(t), a.Centre[1] + r*math.Sin(t)})
	}
	var moment float64
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		c := p[0]*q[1] - q[0]*p[1]
		area += c / 2
		moment += (p[0] + q[0]) * c / 6
	}
	return math.Abs(area), moment / area
}

// threadGoldens are the cases that have a golden in testdata/thread:
// enough kinds of thread to pin the generator and the kernel on.
var threadGoldens = []string{"screw_m8", "nut_m8", "blind_m6", "screw_m8_lh", "screw_m6_down",
	"lead_tr20", "screw_unc", "triple_m12", "screw_bsw", "nut_g", "screw_acme", "screw_m10_6g",
	"screw_s20", "nut_s20_back", "screw_butt", "phased_runout_m8", "screw_butt_round"}

// TestThreadGoldens saves the file a thread makes, opens it and saves
// it again, and requires the golden, testdata/thread/<name>.tlspr.
// TELESAPR_REGEN_GOLDEN=1 rewrites the goldens.
func TestThreadGoldens(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	// And a run-out with a phase, of the run-out cases
	// (thread_runout_test.go), whose names are taken by threads without.
	cases := slices.Clone(threadCases)
	for _, tc := range runOutCases {
		if tc.name == "phased_m8" {
			tc.name = "phased_runout_m8"
			cases = append(cases, tc)
		}
	}
	for _, tc := range fixtures.Sample(cases) {
		if !slices.Contains(threadGoldens, tc.name) {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, _ := tc.build(t, 0)
			// Saved, which regenerates first.
			sk.GenerateAll()
			path := filepath.Join(t.TempDir(), tc.name+".tlspr")
			if err := tlspr.SaveFile(sk, "", path, tlspr.Options{}); err != nil {
				t.Fatal(err)
			}
			doc, err := tlspr.LoadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Opened, then saved: each regenerates.
			doc.Sketch.GenerateAll()
			doc.Sketch.GenerateAll()
			golden.Check(t, fixtures.Path("thread/"+tc.name+".tlspr"), tlspr.Encode(doc.Sketch, tlspr.Options{}))
		})
	}
}

// TestThreadErrors checks what AddThread refuses, and that it then
// leaves the sketch alone.
func TestThreadErrors(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, circle, _ := tc.build(t, 0)
	groups := len(sk.Groups)
	m8 := tc.options(t)
	for _, c := range []struct {
		what string
		he   core.HEntity
		edit func(o *sketch.ThreadOptions)
	}{
		{"a point", sk.MustEntity(circle).Point[0], func(o *sketch.ThreadOptions) {}},
		{"no entity", core.HEntity{V: 0x7fffffff}, func(o *sketch.ThreadOptions) {}},
		{"no length", circle, func(o *sketch.ThreadOptions) { o.Length = 0 }},
		{"a negative length", circle, func(o *sketch.ThreadOptions) { o.Length = -5 }},
		{"a length without a turn", circle, func(o *sketch.ThreadOptions) { o.Length = 2 }},
		{"no pitch", circle, func(o *sketch.ThreadOptions) { o.Spec.Pitch = 0 }},
		{"too coarse a pitch", circle, func(o *sketch.ThreadOptions) { o.Spec.Pitch = 8 }},
		{"a clearance that leaves nothing", circle, func(o *sketch.ThreadOptions) { o.Clearance = 4 }},
		{"a clearance that isn't a number", circle, func(o *sketch.ThreadOptions) { o.Clearance = math.NaN() }},
		{"a rounded buttress crest", circle, func(o *sketch.ThreadOptions) {
			o.Spec = thread.Spec{Form: thread.FormButtress, Diameter: 8, Pitch: 1.5}
			o.Crest = thread.CrestRounded
		}},
		// The part, 30 mm long, has a chord tolerance of 0.03 mm at 0.1%,
		// coarser than a fifth of the root's 0.071 mm.
		{"a rounded root at a coarse chord tolerance", circle, func(o *sketch.ThreadOptions) {
			o.Spec = thread.Spec{Form: thread.FormButtressInch, Diameter: 7.9375, Pitch: 25.4 / 24}
			o.RoundRoot = true
		}},
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

// TestThreadRoundRootTolerance checks that a rounded root is refused at
// a chord tolerance coarser than its arcs take, naming the tolerance
// to set, and made at that one.
func TestThreadRoundRootTolerance(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("a thread with rounded roots")
	}
	tc := threadCase{"", 12.7, 40, "1-12 BUTT", sketch.ThreadOptions{Length: 25, Clearance: 0.05, RoundRoot: true}}
	_, _, _, err := tc.tryBuild(t, 0)
	if !errors.Is(err, sketch.ErrRootRoundCoarse) {
		t.Fatalf("at the default chord tolerance: %v", err)
	}
	msg := err.Error()
	words := strings.Fields(msg)
	if !strings.HasSuffix(msg, "% or less") || len(words) < 3 {
		t.Fatalf("%q names no tolerance", msg)
	}
	pct, perr := strconv.ParseFloat(strings.TrimSuffix(words[len(words)-3], "%"), 64)
	if perr != nil || !(pct > 0 && pct < 0.1) {
		t.Fatalf("%q: tolerance %g, %v", msg, pct, perr)
	}
	if _, _, res, err := tc.tryBuild(t, pct); err != nil || !res.Sound {
		t.Errorf("at %g%%: %v, sound %v", pct, err, res.Sound)
	}
}

// TestThreadAfterActiveGroup makes a thread while an earlier group is
// active: its groups follow that one, and the later ones move on.
func TestThreadAfterActiveGroup(t *testing.T) {
	t.Parallel()
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
		g := sk.Group(hg)
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
	if sk.ActiveGroup() != second || sk.Group(first).Order != 10 {
		t.Errorf("active group %v, first thread at %d", sk.ActiveGroup(), sk.Group(first).Order)
	}
}

// TestThreadLengthDimension changes the dimension that a thread's helix
// group carries: the helix follows it, at the same pitch.
// TestThreadOriginsStay lengthens a thread: the helix gets more
// sections, and the surfaces of the sections it had keep their origins,
// so the names of their edges stay; by their place among the group's
// surfaces, those of every curve after the first would have moved
// (roadmap 14.2).
func TestThreadOriginsStay(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 6}}
	sk, _, res := tc.build(t, 0)
	hel := res.Group
	origins := func() map[uint32]geom.Vector {
		out := map[uint32]geom.Vector{}
		for _, s := range sk.Group(hel).ThisShell.Surface {
			if s.Face == 0 {
				out[s.Origin] = s.PointAt(0.5, 0.5)
			}
		}
		return out
	}
	short := origins()
	var length *core.Constraint
	for i := range sk.Constraints {
		if c := &sk.Constraints[i]; c.Group == hel {
			length = c
		}
	}
	length.ValA = math.Copysign(12, length.ValA)
	sk.MarkGroupDirty(hel, false)
	sk.GenerateAllTyped(sketch.GenerateAll)
	long := origins()
	if len(short) == 0 || len(long) <= len(short) {
		t.Fatalf("%d surfaces with origins, then %d", len(short), len(long))
	}
	// Each section spans the helix's angle over their count, so a
	// section's surface moves a little along the helix; it is still the
	// j-th of its curve, with the same origin.
	for o, p := range short {
		if q, ok := long[o]; !ok || q.Minus(p).Magnitude() > tc.radius {
			t.Errorf("origin %#x was at %v, now %v (%v)", o, p, q, ok)
		}
	}
}

func TestThreadLengthDimension(t *testing.T) {
	t.Parallel()
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
	g := sk.Group(hel)
	if g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 {
		t.Fatalf("solved %v with %d degrees of freedom", g.Solved.How, g.Solved.Dof)
	}
	dist := 2 * sk.MustParam(hel.Param(7)).Val
	angle := 4 * sk.MustParam(hel.Param(3)).Val
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
	t.Parallel()
	cases := threadSweepCases()
	byName := map[string]threadCase{}
	var names []string
	for _, tc := range cases {
		byName[tc.name] = tc
		names = append(names, tc.name)
	}
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("tol", 0.5, 0.3, 0.2, 0.1, 0.05, 0.03, 0.02, 0.01),
			sweep.Strings("case", names...),
		},
		MaxFailRate: threadSweepMaxFailRate,
		Sample:      16,
	}
	grid.Run(func(p sweep.Point) error {
		_, _, res, err := byName[p.String("case")].tryBuild(t, p.Float("tol"))
		switch {
		case errors.Is(err, sketch.ErrRootRoundCoarse):
			// Refused by name, as a rounded root is at coarse tolerances.
			return nil
		case err != nil:
			return err
		}
		if !res.Sound {
			return errors.New("not sound")
		}
		return nil
	}).Require(t)
}

// threadSweepMaxFailRate is the fraction of TestThreadSweep's grid
// allowed to be unsound: none, as the whole grid gave on 2026-10-03.
// On 2026-10-02, after the raycast fix, it was 3 of 400, at 0.03% and
// 0.01%; before that fix AddThread turned the planes up to seven times
// and 24 were unsound, and generated once without it 68 were.
const threadSweepMaxFailRate = 0

// TestThreadGoldensSound checks the solid of every thread golden with
// the invariant oracle.
func TestThreadGoldensSound(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(fixtures.Path("thread/*.tlspr"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no thread goldens: %v", err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			doc, err := tlspr.LoadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			sk.GenerateAll()
			last := sk.Group(sk.GroupOrder[len(sk.GroupOrder)-1])
			if r := check.Group(sk, last); !r.Sound() || r.Empty() {
				t.Errorf("%s: %s", last.Name, r.String())
			}
		})
	}
}
