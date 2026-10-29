package sketch_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// lastOf is the last of a thread's groups, which holds its solid.
func lastOf(t *testing.T, sk *sketch.Sketch, hel core.HGroup) *sketch.Group {
	t.Helper()
	gs, err := sk.ThreadGroups(hel)
	if err != nil {
		t.Fatal(err)
	}
	return sk.Group(gs[len(gs)-1])
}

// requireSoundThread requires every group of the thread solved with
// nothing to drag, no Boolean failed, and the oracle's sound solid.
func requireSoundThread(t *testing.T, sk *sketch.Sketch, res sketch.ThreadResult) {
	t.Helper()
	if !res.Sound {
		t.Error("the solid isn't sound")
	}
	gs, err := sk.ThreadGroups(res.Group)
	if err != nil {
		t.Fatal(err)
	}
	for _, hg := range gs {
		g := sk.Group(hg)
		if g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 || g.BooleanFailed {
			t.Errorf("group %s: solved %v with %d degrees of freedom, Boolean failed %v",
				g.Name, g.Solved.How, g.Solved.Dof, g.BooleanFailed)
		}
	}
	if r := check.Group(sk, lastOf(t, sk, res.Group)); !r.Sound() || r.Empty() {
		t.Errorf("oracle: %s", r.String())
	}
}

var chordRunOut = 0.0

// runOutCases are threads that end at a shoulder, each with a run-out.
var runOutCases = []threadCase{
	{"screw_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, RunOut: 3}},
	{"blind_m6", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true, RunOut: 2}},
	{"screw_m8_lh", 4, 30, "M8-LH", sketch.ThreadOptions{Length: 12, RunOut: 2}},
	{"screw_m6_down", 3, -20, "M6", sketch.ThreadOptions{Length: 10, Reverse: true, RunOut: 2.5}},
	{"lead_tr20", 10, 40, "Tr20x8(P4)", sketch.ThreadOptions{Length: 30, RunOut: 6}},
	{"screw_unc", 3.175, 20, "1/4-20 UNC", sketch.ThreadOptions{Length: 12, Clearance: 0.1, RunOut: 3}},
	{"oversize_m5", 4, 20, "M5", sketch.ThreadOptions{Length: 10, RunOut: 2}},
	{"screw_m3", 1.5, 16, "M3", sketch.ThreadOptions{Length: 12, RunOut: 1}},
	{"rounded_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, RunOut: 3, Crest: thread.CrestRounded}},
	{"level_m12", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, RunOut: 3, LevelStarts: true}},
	// A flank of the ridge's end on the run-out's circle (2026-10-23).
	{"screw_m10_flank", 5, 30, "M10", sketch.ThreadOptions{Length: 20, RunOut: 2}},
	// Phased: the ridges' end is cleared of the circle where the phase
	// moves it (2026-10-29).
	{"phased_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, RunOut: 3, Phase: 120}},
	{"phased_blind_m6", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true, RunOut: 2, Phase: 300}},
	{"phased_m10_flank", 5, 30, "M10", sketch.ThreadOptions{Length: 20, RunOut: 2, Phase: 45}},
	{"phased_chamfer_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, RunOut: 3, Phase: 200, Chamfer: true}},
}

// TestThreadRunOut makes threads with a run-out and requires sound
// solids, solved, whose turned or bored surface tapers from the root
// to near the crest over the run-out: more than the thread without it
// and less than that and the cone's ring, and nothing of the part cut
// past the cone.
func TestThreadRunOut(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	for _, tc := range fixtures.Sample(runOutCases) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, res := tc.build(t, chordRunOut)
			requireSoundThread(t, sk, res)
			plain := tc
			plain.o.RunOut = 0
			psk, _, pres := plain.build(t, chordRunOut)

			o := tc.options(t)
			p := o.Spec.Profile(o.Internal, o.Clearance)
			to := p.CrestRadius
			if !o.Internal {
				to -= 0.1 * (p.CrestRadius - p.RootRadius)
			}
			// The ring between the root and the cone, revolved: a frustum
			// less a cylinder, or the other way about in a nut.
			r0, r1, h := p.RootRadius, to, o.RunOut
			frustum := math.Pi * h / 3 * (r0*r0 + r0*r1 + r1*r1)
			ring := math.Abs(frustum - math.Pi*r0*r0*h)

			m, _ := meshStats(sk, lastOf(t, sk, res.Group).H)
			pm, _ := meshStats(psk, lastOf(t, psk, pres.Group).H)
			got, without := m.CalculateVolume(), pm.CalculateVolume()
			if !(got > without) || got > without+ring*1.002 {
				t.Errorf("volume %.4f, want between %.4f and %.4f", got, without, without+ring)
			}

			// Inside the cone in a screw, outside it in a nut, is the
			// part.
			along := 1.0
			if o.Reverse {
				along = -1
			}
			from := o.Length - o.RunOut
			for _, tr := range m.Triangles {
				for _, v := range [][3]float64{{tr.A.X, tr.A.Y, tr.A.Z}, {tr.B.X, tr.B.Y, tr.B.Z}, {tr.C.X, tr.C.Y, tr.C.Z}} {
					z := along * v[2]
					if z <= from+1e-6 || z >= o.Length-1e-6 {
						continue
					}
					cone := r0 + (r1-r0)*(z-from)/h
					r := math.Hypot(v[0], v[1])
					if (!o.Internal && r < cone-1e-3) || (o.Internal && r > cone+1e-3) {
						t.Fatalf("a vertex at r %.4f, z %.4f is past the run-out's cone at %.4f", r, z, cone)
					}
				}
			}
		})
	}
}

// levelCases are threads of several starts, each start from the
// circle's plane.
var levelCases = []threadCase{
	{"triple_m12", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, LevelStarts: true}},
	{"lead_tr20", 10, 40, "Tr20x8(P4)", sketch.ThreadOptions{Length: 30, LevelStarts: true}},
	{"nut_tr20", 16, 20, "Tr20x8(P4)", sketch.ThreadOptions{Length: 20, Through: true, Internal: true, LevelStarts: true}},
	{"double_m8_lh", 4, 30, "M8xPh2.5P1.25-LH", sketch.ThreadOptions{Length: 12, LevelStarts: true}},
}

// TestThreadLevelStarts makes threads whose starts all begin at the
// circle's plane: a sketch and a helix per start, each start's ridge
// at the same height, turned a part of a turn from the last, and the
// volume of the part with every ridge swept the whole length.
func TestThreadLevelStarts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	for _, tc := range fixtures.Sample(levelCases) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, res := tc.build(t, 0.01)
			requireSoundThread(t, sk, res)
			o := tc.options(t)
			s := o.Spec
			n := s.NumStarts()
			gs, _ := sk.ThreadGroups(res.Group)
			if len(gs) != 4+2*(n-1) {
				t.Fatalf("%d groups, want %d", len(gs), 4+2*(n-1))
			}
			d := s.String()
			for i := 1; i < n; i++ {
				p, h := sk.Group(gs[2+2*i]), sk.Group(gs[3+2*i])
				if p.Name != fmt.Sprintf("%s-profile-%d", d, i+1) || h.Name != fmt.Sprintf("%s-thread-%d", d, i+1) {
					t.Errorf("start %d is %q and %q", i+1, p.Name, h.Name)
				}
				if h.Thread.IsThread() {
					t.Errorf("start %d's helix keeps the thread's options", i+1)
				}
			}
			if sk.ActiveGroup() != gs[len(gs)-1] {
				t.Error("the thread's last group isn't the active one")
			}

			// The foot of each start's ridge, the first request of its
			// sketch: at one height, a part of a turn apart.
			foot := func(prof core.HGroup) (a, b [3]float64) {
				for j := range sk.Requests {
					if r := &sk.Requests[j]; r.Group == prof {
						va := sk.MustEntity(r.H.Entity(1)).PointGetNum(sk)
						vb := sk.MustEntity(r.H.Entity(2)).PointGetNum(sk)
						return [3]float64{va.X, va.Y, va.Z}, [3]float64{vb.X, vb.Y, vb.Z}
					}
				}
				t.Fatal("a ridge sketch has no requests")
				return
			}
			first, firstEnd := foot(gs[2])
			between := 2 * math.Pi / float64(n)
			for i := 1; i < n; i++ {
				f, _ := foot(gs[2+2*i])
				if math.Abs(f[2]-first[2]) > 1e-9 {
					t.Errorf("start %d at %g along the axis, the first at %g", i+1, f[2], first[2])
				}
				turn := math.Remainder(math.Atan2(f[1], f[0])-math.Atan2(first[1], first[0]), 2*math.Pi)
				if off := math.Remainder(turn, between); math.Abs(off) > 1e-9 || math.Abs(turn) < 1e-9 {
					t.Errorf("start %d turned %g from the first, not a multiple of %g", i+1, turn, between)
				}
			}

			m, _ := meshStats(sk, lastOf(t, sk, res.Group).H)
			p := s.Profile(o.Internal, o.Clearance)
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
			hh := p.Height()
			area := (p.RootWidth + p.CrestWidth) / 2 * hh
			centroid := hh * (p.RootWidth + 2*p.CrestWidth) / (3 * (p.RootWidth + p.CrestWidth))
			if o.Internal {
				centroid = p.RootRadius - centroid
			} else {
				centroid = p.RootRadius + centroid
			}
			sweep := o.Length - s.Pitch - math.Abs(firstEnd[2]-first[2])
			want += float64(n) * area * centroid * sweep / s.Lead() * 2 * math.Pi
			if got := m.CalculateVolume(); math.Abs(got-want) > 0.002*want {
				t.Errorf("volume %.4f, want %.4f", got, want)
			}
		})
	}
}

// TestEditThreadShape edits a thread between stacked and level starts,
// to and from a run-out and between numbers of level starts, and
// requires what a thread made with the new options has; each edit is
// one step that undo takes back to the file it was.
func TestEditThreadShape(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	m12 := threadCase{"", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14}}
	m12Level := threadCase{"", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, LevelStarts: true}}
	m8 := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	m8Chamfer := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12, Chamfer: true}}
	m12LevelChamfer := threadCase{"", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, LevelStarts: true, Chamfer: true}}
	// Under the race detector a fifth of the edits: each rebuilds threads
	// several times, some ten minutes there, through the same goroutines.
	for _, c := range fixtures.Sample([]struct {
		name string
		from threadCase
		to   string
		edit func(o *sketch.ThreadOptions)
		// keepsAll is whether the thread keeps every group.
		keepsAll bool
	}{
		{"run-out", m8, "M8", func(o *sketch.ThreadOptions) { o.RunOut = 3 }, true},
		{"level", m12, "M12xPh3P1", func(o *sketch.ThreadOptions) { o.LevelStarts = true }, false},
		{"phase and run-out", m8, "M8", func(o *sketch.ThreadOptions) { o.Phase, o.RunOut = 90, 3 }, true},
		{"level and run-out", m12, "M12xPh3P1", func(o *sketch.ThreadOptions) { o.LevelStarts, o.RunOut = true, 3 }, false},
		{"fewer level starts", m12Level, "M12xPh2P1", func(o *sketch.ThreadOptions) { o.LevelStarts = true }, false},
		{"level kept", m12Level, "M12xPh3P1", func(o *sketch.ThreadOptions) { o.LevelStarts, o.Length = true, 16 }, true},
		{"stacked again", m12Level, "M12xPh3P1", func(o *sketch.ThreadOptions) {}, false},
		{"chamfer", m8, "M8", func(o *sketch.ThreadOptions) { o.Chamfer = true }, false},
		{"chamfer and run-out", m8, "M8", func(o *sketch.ThreadOptions) { o.Chamfer, o.RunOut = true, 3 }, false},
		{"no chamfer", m8Chamfer, "M8", func(o *sketch.ThreadOptions) {}, false},
		{"chamfer kept", m8Chamfer, "M8", func(o *sketch.ThreadOptions) { o.Chamfer, o.Length = true, 14 }, true},
		{"through with chamfer kept", m8Chamfer, "M8", func(o *sketch.ThreadOptions) { o.Chamfer, o.Through, o.Length = true, true, 28 }, true},
		{"fewer level starts with chamfer", m12LevelChamfer, "M12xPh2P1", func(o *sketch.ThreadOptions) { o.LevelStarts, o.Chamfer = true, true }, false},
		{"chamfer with level starts", m12, "M12xPh3P1", func(o *sketch.ThreadOptions) { o.LevelStarts, o.Chamfer = true, true }, false},
	}) {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sk, _, res := c.from.build(t, 0)
			sk.ChordTolPct = 0
			before, _ := sk.ThreadGroups(res.Group)
			want := c.from
			want.designation = c.to
			want.o.LevelStarts, want.o.RunOut, want.o.Chamfer = false, 0, false
			o := want.options(t)
			c.edit(&o)
			want.o = o
			fresh, _, freshRes := want.build(t, 0)
			fresh.ChordTolPct = 0

			save := func() string {
				sk.GenerateAll()
				return string(tlspr.Encode(sk, tlspr.Options{}))
			}
			// A solve from a solved sketch may move its last bits once,
			// and some params go on cycling through a few last bits from
			// solve to solve (a chamfered thread's through four): any of
			// those is the same file.
			cycle := func() []string {
				var out []string
				for range 4 {
					out = append(out, save())
				}
				return out
			}
			save()
			was := cycle()
			sk.UndoRemember()
			deleted := sk.Deleted
			got, err := sk.EditThread(res.Group, o)
			if err != nil {
				t.Fatal(err)
			}
			// The groups the thread no longer has go with what they
			// hold, which is no deletion of the user's to report.
			if sk.Deleted != deleted {
				t.Errorf("counted as deleted %+v, was %+v", sk.Deleted, deleted)
			}
			if got.Group != res.Group {
				t.Fatalf("edited %v", got.Group)
			}
			requireSoundThread(t, sk, got)
			after, _ := sk.ThreadGroups(res.Group)
			if keeps := slices.Equal(after, before); keeps != c.keepsAll {
				t.Errorf("groups %v, were %v: kept %v, want %v", after, before, keeps, c.keepsAll)
			}
			if !slices.Equal(after[:4], before[:4]) {
				t.Errorf("the first four groups %v, were %v", after[:4], before[:4])
			}
			if o2, _ := sk.ThreadOf(res.Group); o2 != o {
				t.Errorf("thread kept %+v, want %+v", o2, o)
			}
			es, fs := stateOf(t, sk, res.Group), stateOf(t, fresh, freshRes.Group)
			if !slices.Equal(es.dof, fs.dof) || !slices.Equal(es.names, fs.names) {
				t.Errorf("edited %+v, fresh %+v", es, fs)
			}
			fresh.GenerateAll()
			ev, _ := meshStats(sk, lastOf(t, sk, res.Group).H)
			fv, _ := meshStats(fresh, lastOf(t, fresh, freshRes.Group).H)
			if a, b := ev.CalculateVolume(), fv.CalculateVolume(); math.Abs(a-b) > 1e-6*b {
				t.Errorf("volume %.9g, fresh %.9g", a, b)
			}

			save()
			edited := cycle()
			sk.UndoUndo()
			if got := save(); !slices.Contains(was, got) {
				t.Errorf("undo did not bring the thread back:\n%s", golden.FirstDifference([]byte(was[0]), []byte(got)))
			}
			sk.UndoRedo()
			if got := save(); !slices.Contains(edited, got) {
				t.Errorf("redo did not make the edit again:\n%s", golden.FirstDifference([]byte(edited[0]), []byte(got)))
			}
		})
	}
}

// TestThreadRunOutRefused checks the run-outs a thread can't have.
func TestThreadRunOutRefused(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	sk.ChordTolPct = 0
	was := string(tlspr.Encode(sk, tlspr.Options{}))
	o := tc.options(t)
	for what, edit := range map[string]func(o *sketch.ThreadOptions){
		"negative":     func(o *sketch.ThreadOptions) { o.RunOut = -1 },
		"not a number": func(o *sketch.ThreadOptions) { o.RunOut = math.NaN() },
		"through":      func(o *sketch.ThreadOptions) { o.RunOut, o.Through = 2, true },
		"whole length": func(o *sketch.ThreadOptions) { o.RunOut = 12 },
	} {
		bad := o
		edit(&bad)
		if _, err := sk.EditThread(res.Group, bad); err == nil {
			t.Errorf("%s: edited", what)
		}
	}
	if got := string(tlspr.Encode(sk, tlspr.Options{})); got != was {
		t.Error("a refused edit changed the file")
	}
}

// TestThreadShapeSaved saves a thread with a run-out, level starts and
// a chamfer,
// opens it and requires its options and groups back, and the thread
// edited from there.
func TestThreadShapeSaved(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	tc := threadCase{"", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, LevelStarts: true, RunOut: 2.5, Chamfer: true}}
	sk, _, res := tc.build(t, 0)
	doc, err := tlspr.Decode(tlspr.Encode(sk, tlspr.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	back := doc.Sketch
	back.GenerateAll()
	o, err := back.ThreadOf(res.Group)
	if err != nil || o != tc.options(t) {
		t.Fatalf("read back %+v (%v), want %+v", o, err, tc.options(t))
	}
	if gs, err := back.ThreadGroups(res.Group); err != nil || len(gs) != 10 {
		t.Fatalf("groups %v, %v", gs, err)
	}
	o.RunOut = 3
	if r, err := back.EditThread(res.Group, o); err != nil || !r.Sound {
		t.Errorf("editing the opened thread: sound %v, %v", r.Sound, err)
	}
}

// runOutSweepCases are the threads the run-out sweep makes, by name:
// screws and nuts, coarse and fine, a rod as thick as its thread and
// one thicker, either hand, either way along the axis.
var runOutSweepCases = map[string]threadCase{
	"M10":   {"", 5, 30, "M10", sketch.ThreadOptions{Length: 14}},
	"M10n":  {"", 10, 24, "M10", sketch.ThreadOptions{Length: 14, Internal: true}},
	"M8":    {"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}},
	"M8-LH": {"", 4, 30, "M8-LH", sketch.ThreadOptions{Length: 12}},
	"M3":    {"", 1.5, 16, "M3", sketch.ThreadOptions{Length: 12}},
	"M5":    {"", 4, 20, "M5", sketch.ThreadOptions{Length: 10}},
	"M6v":   {"", 3, -20, "M6", sketch.ThreadOptions{Length: 10, Reverse: true}},
	"M6n":   {"", 8, 14, "M6", sketch.ThreadOptions{Length: 8, Internal: true}},
	"Tr20":  {"", 10, 40, "Tr20x8(P4)", sketch.ThreadOptions{Length: 30}},
	"UNC":   {"", 3.175, 20, "1/4-20 UNC", sketch.ThreadOptions{Length: 12, Clearance: 0.1}},
}

// threadRunOutMaxFail is the rate of the run-out sweep allowed to
// fail: none since 2026-10-22, when the Boolean took the crossings of
// the turned surface's circle and seams with the ridge's flanks, at the
// thread's helix angle, on the flank's patch they lie on (8 of 72
// before). The run-outs of 1.64 and 2 mm put a flank of an M8's and an
// M10's ridge end on the run-out's circle, where the Boolean dropped a
// quarter of the cone until the end was moved off it (2026-10-23).
const threadRunOutMaxFail = 0.0

// TestThreadRunOutSweep makes run-outs of several lengths on threads
// of several lengths, and requires sound solids but at the rate fixed.
func TestThreadRunOutSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("thread", "M8", "M8-LH", "M3", "M5", "M6v", "M6n", "Tr20", "UNC", "M10", "M10n"),
			sweep.Floats("longer", 0, 0.3, 0.7),
			sweep.Floats("run-out", 1, 1.64, 2, 2.2, 3.5),
		},
		MaxFailRate: threadRunOutMaxFail,
		Sample:      8,
	}.Run(func(p sweep.Point) error {
		tc := runOutSweepCases[p.String("thread")]
		tc.o.Length += p.Float("longer")
		tc.o.RunOut = p.Float("run-out")
		_, _, res := tc.build(t, 0)
		if !res.Sound {
			return errors.New("the solid isn't sound")
		}
		return nil
	}).Require(t)
}

// TestThreadPhasedRunOutSweep makes run-outs on phased threads, the
// phase moving the ridges' end along the thread towards and past the
// run-out's circle, and requires sound solids but at the rate fixed.
func TestThreadPhasedRunOutSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("thread", "M8", "M8-LH", "M3", "M5", "M6v", "M6n", "Tr20", "UNC", "M10", "M10n"),
			sweep.Floats("phase", 30, 90, 150, 210, 270, 330),
			sweep.Floats("run-out", 1.64, 2, 3.5),
		},
		MaxFailRate: threadRunOutMaxFail,
		Sample:      8,
	}.Run(func(p sweep.Point) error {
		tc := runOutSweepCases[p.String("thread")]
		tc.o.Phase = p.Float("phase")
		tc.o.RunOut = p.Float("run-out")
		_, _, res := tc.build(t, 0)
		if !res.Sound {
			return errors.New("the solid isn't sound")
		}
		return nil
	}).Require(t)
}
