package sketch_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// threadState is what an edited thread is compared by: its solid's
// volume and soundness, how its groups solved, and their names.
type threadState struct {
	volume float64
	sound  bool
	dof    []int
	names  []string
}

func stateOf(t *testing.T, sk *sketch.Sketch, hel core.HGroup) threadState {
	t.Helper()
	groups, err := sk.ThreadGroups(hel)
	if err != nil {
		t.Fatal(err)
	}
	var st threadState
	for _, hg := range groups {
		g := sk.Group(hg)
		if g.Solved.How != core.SolveOkay {
			t.Errorf("%s solved %v", g.Name, g.Solved.How)
		}
		st.dof = append(st.dof, g.Solved.Dof)
		st.names = append(st.names, g.Name)
	}
	m, bad := meshStats(sk, hel)
	st.volume, st.sound = m.CalculateVolume(), bad == 0
	return st
}

// TestEditThread edits a thread's every option and requires what a
// thread made with the new options has: the same solid, the same
// groups solved with nothing to drag, the same names. The groups keep
// their handles and, where the thread keeps its shape, its requests.
func TestEditThread(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	base := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	for _, c := range []struct {
		name        string
		designation string
		edit        func(o *sketch.ThreadOptions)
		// sameRequests is whether the thread keeps every request.
		sameRequests bool
	}{
		{"pitch", "M8x1", func(o *sketch.ThreadOptions) {}, true},
		{"length", "M8", func(o *sketch.ThreadOptions) { o.Length = 16 }, true},
		{"clearance", "M8", func(o *sketch.ThreadOptions) { o.Clearance = 0.15 }, true},
		{"left hand", "M8-LH", func(o *sketch.ThreadOptions) {}, true},
		{"through", "M8", func(o *sketch.ThreadOptions) { o.Through = true }, true},
		{"diameter", "M6", func(o *sketch.ThreadOptions) {}, true},
		{"starts", "M8xPh2.5P1.25", func(o *sketch.ThreadOptions) {}, false},
		{"truncated", "M8", func(o *sketch.ThreadOptions) { o.Crest = thread.CrestTruncated }, true},
		{"rounded", "M8", func(o *sketch.ThreadOptions) { o.Crest = thread.CrestRounded }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sk, circle, res := base.build(t, 0)
			groups, err := sk.ThreadGroups(res.Group)
			if err != nil {
				t.Fatal(err)
			}
			before := requestsOf(sk, groups)

			want := base
			want.designation = c.designation
			o := want.options(t)
			c.edit(&o)
			want.o = o
			fresh, _, freshRes := want.build(t, 0)

			got, err := sk.EditThread(res.Group, o)
			if err != nil {
				t.Fatal(err)
			}
			if got.Group != res.Group || !got.Sound || !freshRes.Sound {
				t.Fatalf("edited %v sound %v, fresh sound %v", got.Group, got.Sound, freshRes.Sound)
			}
			after, err := sk.ThreadGroups(res.Group)
			if err != nil || !slices.Equal(after, groups) {
				t.Fatalf("groups %v, were %v (%v)", after, groups, err)
			}
			if same := slices.Equal(requestsOf(sk, groups), before); same != c.sameRequests {
				t.Errorf("requests kept: %v, want %v", same, c.sameRequests)
			}
			if o2, _ := sk.ThreadOf(res.Group); o2 != o {
				t.Errorf("thread kept %+v, want %+v", o2, o)
			}
			if sk.Group(res.Group).Thread.Circle != circle {
				t.Error("the thread lost its circle")
			}
			es, fs := stateOf(t, sk, res.Group), stateOf(t, fresh, freshRes.Group)
			if !slices.Equal(es.dof, fs.dof) || !slices.Equal(es.names, fs.names) {
				t.Errorf("edited %+v, fresh %+v", es, fs)
			}
			if math.Abs(es.volume-fs.volume) > 1e-6*fs.volume {
				t.Errorf("volume %.9g, fresh %.9g", es.volume, fs.volume)
			}
			last := sk.Group(res.Group)
			if r := check.Group(sk, last); !r.Sound() || r.Empty() {
				t.Errorf("oracle: %s", r.String())
			}
		})
	}
}

// requestsOf lists the requests of the groups, by handle.
func requestsOf(sk *sketch.Sketch, groups []core.HGroup) []core.HRequest {
	var out []core.HRequest
	for _, r := range sk.Requests {
		if slices.Contains(groups, r.Group) {
			out = append(out, r.H)
		}
	}
	return out
}

// TestEditThreadUndo edits a thread as one step: undo brings back the
// file it was, and redo the edited one.
func TestEditThreadUndo(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	sk.ChordTolPct = 0
	save := func() string {
		sk.GenerateAll()
		return string(tlspr.Encode(sk, tlspr.Options{}))
	}
	was := save()
	o := tc.options(t)
	o.Length, o.Spec.Starts, o.Crest = 15, 2, thread.CrestRounded
	sk.UndoRemember()
	if _, err := sk.EditThread(res.Group, o); err != nil {
		t.Fatal(err)
	}
	edited := save()
	if edited == was {
		t.Fatal("the edit changed nothing")
	}
	sk.UndoUndo()
	if got := save(); got != was {
		t.Error("undo did not bring the thread back")
	}
	sk.UndoRedo()
	if got := save(); got != edited {
		t.Error("redo did not make the edit again")
	}
}

// TestEditThreadRefused checks what EditThread refuses, and that it
// then leaves the thread alone.
func TestEditThreadRefused(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	sk.ChordTolPct = 0
	was := string(tlspr.Encode(sk, tlspr.Options{}))
	o := tc.options(t)
	for what, edit := range map[string]func(o *sketch.ThreadOptions){
		"no length":     func(o *sketch.ThreadOptions) { o.Length = 0 },
		"coarse pitch":  func(o *sketch.ThreadOptions) { o.Spec.Pitch = 8 },
		"no crest":      func(o *sketch.ThreadOptions) { o.Crest = 9 },
		"unknown names": func(o *sketch.ThreadOptions) { o.LengthExpr = "nothing*2" },
	} {
		bad := o
		edit(&bad)
		if _, err := sk.EditThread(res.Group, bad); err == nil {
			t.Errorf("%s: edited", what)
		}
	}
	if _, err := sk.EditThread(sk.GroupOrder[2], o); !errors.Is(err, sketch.ErrNotThread) {
		t.Errorf("an extrusion edited as a thread: %v", err)
	}
	if got := string(tlspr.Encode(sk, tlspr.Options{})); got != was {
		t.Error("a refused edit changed the file")
	}
}

// TestThreadLengthExpression drives a thread's length by a parameter:
// the turning and the helix follow it, and the thread's length shows
// the value; the expression survives a save and an open, and a change
// of unit.
func TestThreadLengthExpression(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	if err := sk.AddParameter("len", "12"); err != nil {
		t.Fatal(err)
	}
	o := tc.options(t)
	v, expr, err := sk.ThreadLengthText("len")
	if err != nil || v != 12 || expr != "len" {
		t.Fatalf("length %g %q %v", v, expr, err)
	}
	o.Length, o.LengthExpr = v, expr
	if _, err := sk.EditThread(res.Group, o); err != nil {
		t.Fatal(err)
	}
	ridge := func(sk *sketch.Sketch) float64 {
		return 2 * math.Abs(sk.MustParam(res.Group.Param(7)).Val)
	}
	short := ridge(sk)
	if err := sk.SetParameterExpr("len", "16"); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if got := ridge(sk); math.Abs(got-short-4) > 1e-9 {
		t.Errorf("the ridge swept %g, then %g: want 4 mm more", short, got)
	}
	if o2, _ := sk.ThreadOf(res.Group); o2.Length != 16 || o2.LengthExpr != "len" {
		t.Errorf("thread length %g %q", o2.Length, o2.LengthExpr)
	}
	if _, bad := meshStats(sk, res.Group); bad != 0 {
		t.Errorf("%d naked or self-intersecting edges", bad)
	}
	g := sk.Group(res.Group)
	if g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 {
		t.Errorf("solved %v with %d degrees of freedom", g.Solved.How, g.Solved.Dof)
	}

	sk.SetUnits(core.UnitInches)
	doc, err := tlspr.Decode(tlspr.Encode(sk, tlspr.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	doc.Sketch.GenerateAll()
	if o2, _ := doc.Sketch.ThreadOf(res.Group); math.Abs(o2.Length-16) > 1e-9 {
		t.Errorf("opened in inches: thread length %g mm, from %q", o2.Length, o2.LengthExpr)
	}
	if got := ridge(doc.Sketch); math.Abs(got-short-4) > 1e-9 {
		t.Errorf("opened in inches: ridge %g", got)
	}
}

// ridgesVolume is what a thread's ridges, its group hg, add to the
// turned or bored part, from their meshes.
func ridgesVolume(t *testing.T, sk *sketch.Sketch, hg core.HGroup) float64 {
	t.Helper()
	// The thread's groups: the blank, the turned part, the ridge's
	// sketch and the helix.
	turned, bad := meshStats(sk, sk.GroupOrder[sk.Group(hg).Order-2])
	if bad != 0 {
		t.Fatalf("turned part: %d naked or self-intersecting edges", bad)
	}
	mesh, bad := meshStats(sk, hg)
	if bad != 0 {
		t.Fatalf("%d naked or self-intersecting edges", bad)
	}
	return mesh.CalculateVolume() - turned.CalculateVolume()
}

// ridgeSection is the cross-section of a ridge above the root, in a
// plane through the axis: its area, and the radius of its centroid.
// A rounded crest is taken as a polygon of many sides along its arc.
func ridgeSection(p thread.Profile) (area, centroid float64) {
	toRoot := math.Copysign(1, p.RootRadius-p.CrestRadius)
	// (r, z) around the outline, z from the middle of the crest, or of
	// the ridge where a rounded crest's flanks are CrestWidth apart: the
	// root, the upper flank, the crest, the lower flank.
	top := p.CrestRadius
	if p.CrestRoundRadius > 0 {
		top = p.RoundFootRadius
	}
	deep := toRoot * (p.RootRadius - top)
	tl := math.Tan((p.FlankAngle - p.UpperFlank) * math.Pi / 180)
	tu := math.Tan(p.UpperFlank * math.Pi / 180)
	pts := [][2]float64{{p.RootRadius, -p.CrestWidth/2 - tl*deep}, {p.RootRadius, p.CrestWidth/2 + tu*deep}}
	if p.CrestRoundRadius > 0 {
		c, lower, upper := p.CrestArc()
		a0 := math.Atan2(upper[1]-c[1], upper[0]-c[0])
		a1 := math.Atan2(lower[1]-c[1], lower[0]-c[0])
		// The arc, less than half a turn, the short way round.
		if a1-a0 > math.Pi {
			a1 -= 2 * math.Pi
		} else if a0-a1 > math.Pi {
			a1 += 2 * math.Pi
		}
		const n = 2000
		for i := range n + 1 {
			a := a0 + (a1-a0)*float64(i)/n
			pts = append(pts, [2]float64{c[0] + p.CrestRoundRadius*math.Cos(a), c[1] + p.CrestRoundRadius*math.Sin(a)})
		}
	} else {
		pts = append(pts, [2]float64{p.CrestRadius, p.CrestWidth / 2}, [2]float64{p.CrestRadius, -p.CrestWidth / 2})
	}
	var a, m float64
	for i := range pts {
		x0, y0 := pts[i][0], pts[i][1]
		x1, y1 := pts[(i+1)%len(pts)][0], pts[(i+1)%len(pts)][1]
		cross := x0*y1 - x1*y0
		a += cross
		m += (x0 + x1) * cross
	}
	return math.Abs(a / 2), m / (3 * a)
}

// TestThreadCrests makes threads with truncated and rounded crests and
// requires a sound solid with nothing to drag, of the volume the
// shaped ridges sweep.
func TestThreadCrests(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	for _, tc := range []threadCase{
		{"screw_m8_truncated", 4, 30, "M8", sketch.ThreadOptions{Length: 12, Crest: thread.CrestTruncated}},
		{"screw_m8_rounded", 4, 30, "M8", sketch.ThreadOptions{Length: 12, Crest: thread.CrestRounded}},
		{"nut_m8_rounded", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true, Crest: thread.CrestRounded}},
		{"nut_m8_truncated", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true, Crest: thread.CrestTruncated}},
		{"lead_tr20_rounded", 10, 40, "Tr20x8(P4)", sketch.ThreadOptions{Length: 30, Crest: thread.CrestRounded}},
		{"screw_m6_down_rounded", 3, -20, "M6-LH", sketch.ThreadOptions{Length: 10, Reverse: true, Crest: thread.CrestRounded}},
		// Buttress ridges, whose arcs lean: DIN 513 and B1.9, a screw and
		// a nut, either way round, and with B1.9's rounded roots.
		{"screw_s20_rounded", 10, 40, "S20x4", sketch.ThreadOptions{Length: 30, Crest: thread.CrestRounded}},
		{"nut_s20_back_rounded", 16, 20, "S20x4", sketch.ThreadOptions{Length: 20, Through: true, Internal: true, LoadFlankBack: true, Crest: thread.CrestRounded}},
		{"screw_butt_rounded", 12.7, 40, "1-12 PULL-BUTT", sketch.ThreadOptions{Length: 25, Clearance: 0.05, Crest: thread.CrestRounded}},
		{"nut_butt_lh_rounded", 20, 15, "1-12 BUTT-LH", sketch.ThreadOptions{Length: 15, Through: true, Internal: true, Clearance: 0.05, Crest: thread.CrestRounded}},
		{"screw_butt_round_both", 9.525, 20, "3/4-10 BUTT", sketch.ThreadOptions{Length: 12, Clearance: 0.05, RoundRoot: true, Crest: thread.CrestRounded}},
		{"nut_butt_round_both", 20, 15, "1-12 PULL-BUTT", sketch.ThreadOptions{Length: 15, Through: true, Internal: true, Clearance: 0.05, RoundRoot: true, Crest: thread.CrestRounded}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// The crest's arc is a fraction of a millimetre across: at the
			// usual test tolerance its chords cut a fifth of a percent
			// off the ridges.
			sk, _, res := tc.build(t, 0.003)
			if !res.Sound {
				t.Error("the solid isn't sound")
			}
			for _, hg := range sk.GroupOrder[len(sk.GroupOrder)-4:] {
				g := sk.Group(hg)
				if g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 || g.BooleanFailed {
					t.Errorf("group %s: solved %v with %d degrees of freedom, Boolean failed %v",
						g.Name, g.Solved.How, g.Solved.Dof, g.BooleanFailed)
				}
			}
			o := tc.options(t).LoadStated()
			s := o.Spec
			std := s.Profile(o.Internal, o.Clearance)
			if o.LoadFlankBack {
				std = std.Turned()
			}
			p := std.WithCrest(o.Crest)
			// What the shaped crest takes off the standard one's ridges:
			// the sections above the root, swept. A rounded root's
			// fillets are the same under either crest.
			area, centroid := ridgeSection(p)
			flatArea, flatCentroid := ridgeSection(std)
			sweep := 2 * math.Abs(sk.MustParam(res.Group.Param(7)).Val)
			turns := float64(s.NumStarts()) * sweep / s.Lead() * 2 * math.Pi
			want := (flatArea*flatCentroid - area*centroid) * turns
			// Against the same thread made with the standard crest at the
			// same tolerance, so that what the meshes of the helix and of
			// the turned part miss, as much as a crest takes on the larger
			// parts (1.7 mm³ of a 1-12 BUTT nut's ridges at 0.003%),
			// cancels.
			flatCase := tc
			flatCase.o.Crest = thread.CrestStandard
			fsk, _, fres := flatCase.build(t, 0.003)
			got := ridgesVolume(t, fsk, fres.Group) - ridgesVolume(t, sk, res.Group)
			if math.Abs(got-want) > 0.25*math.Abs(want) {
				t.Errorf("the crest takes %.5f off the ridges, want %.5f", got, want)
			}
			if r := check.Group(sk, sk.Group(res.Group)); !r.Sound() || r.Empty() {
				t.Errorf("oracle: %s", r.String())
			}
		})
	}
}
