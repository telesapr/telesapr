package sketch_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// chamferCases are threads with a chamfered free end: screws and nuts
// of every form, either hand, either way along the axis, through and
// blind, with a run-out and with level starts.
var chamferCases = []threadCase{
	{"screw_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, Chamfer: true}},
	{"stud_m8", 4, 10, "M8", sketch.ThreadOptions{Length: 10, Through: true, Chamfer: true}},
	// Its far chamfer's cone met a ridge's flank just past the crest at a
	// grazing angle, at every plane for the ridges and every way of
	// making the chamfer, where 28 and 31 mm were sound.
	{"stud_m8_30", 4, 30, "M8", sketch.ThreadOptions{Length: 30, Through: true, Chamfer: true}},
	{"nut_m8", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true, Chamfer: true}},
	{"blind_m6", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true, Chamfer: true}},
	{"screw_m8_lh", 4, 30, "M8-LH", sketch.ThreadOptions{Length: 12, Chamfer: true}},
	{"screw_m6_down", 3, -20, "M6", sketch.ThreadOptions{Length: 10, Reverse: true, Chamfer: true}},
	{"nut_m6_down_lh", 6, -5, "M6x0.75-LH", sketch.ThreadOptions{Length: 5, Reverse: true, Through: true, Internal: true, Chamfer: true}},
	{"lead_tr20", 10, 40, "Tr20x8(P4)", sketch.ThreadOptions{Length: 30, Chamfer: true}},
	{"nut_tr20", 16, 20, "Tr20x8(P4)", sketch.ThreadOptions{Length: 20, Through: true, Internal: true, Chamfer: true}},
	{"screw_unc", 3.175, 20, "1/4-20 UNC", sketch.ThreadOptions{Length: 12, Clearance: 0.1, Chamfer: true}},
	{"oversize_m5", 4, 20, "M5", sketch.ThreadOptions{Length: 10, Chamfer: true}},
	{"screw_m3", 1.5, 16, "M3", sketch.ThreadOptions{Length: 12, Chamfer: true}},
	{"triple_m12", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, Chamfer: true}},
	{"screw_bsw", 4.7625, 25, "3/8 BSW", sketch.ThreadOptions{Length: 15, Chamfer: true}},
	{"nut_g", 16, 10, "G1/2", sketch.ThreadOptions{Length: 10, Through: true, Internal: true, Chamfer: true}},
	{"screw_acme", 6.35, 30, "1/2-10 ACME", sketch.ThreadOptions{Length: 20, Chamfer: true}},
	{"nut_acme_lh", 12, 12, "1/2-10 ACME-LH", sketch.ThreadOptions{Length: 12, Through: true, Internal: true, Chamfer: true}},
	{"screw_m10_fine", 5, 25, "M10x1", sketch.ThreadOptions{Length: 15, Chamfer: true}},
	{"truncated_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, Crest: thread.CrestTruncated, Chamfer: true}},
	{"rounded_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, Crest: thread.CrestRounded, Chamfer: true}},
	{"rounded_stud_m8", 4, 16, "M8", sketch.ThreadOptions{Length: 16, Through: true, Crest: thread.CrestRounded, Chamfer: true}},
	{"rounded_nut_m10", 10, 10, "M10", sketch.ThreadOptions{Length: 10, Through: true, Internal: true, Crest: thread.CrestRounded, Chamfer: true}},
	{"rounded_blind_m6", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true, Crest: thread.CrestRounded, Chamfer: true}},
	{"runout_m8", 4, 30, "M8", sketch.ThreadOptions{Length: 12, RunOut: 3, Chamfer: true}},
	{"runout_m10", 5, 30, "M10", sketch.ThreadOptions{Length: 20, RunOut: 2, Chamfer: true}},
	{"runout_blind_m6", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true, RunOut: 2, Chamfer: true}},
	{"level_m12", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, LevelStarts: true, Chamfer: true}},
	{"level_nut_tr20", 16, 20, "Tr20x8(P4)", sketch.ThreadOptions{Length: 20, Through: true, Internal: true, LevelStarts: true, Chamfer: true}},
	{"phased_m10", 5, 25, "M10", sketch.ThreadOptions{Length: 15, Class: "6g", Phase: 120, Chamfer: true}},
	{"phased_nut_m10", 10, 10, "M10", sketch.ThreadOptions{Length: 10, Through: true, Internal: true, Class: "6H", Phase: 300, Chamfer: true}},
	{"phased_stud_m8", 4, 16, "M8", sketch.ThreadOptions{Length: 16, Through: true, Phase: 45, Chamfer: true}},
}

// TestThreadChamfer makes chamfered threads and requires sound solids,
// solved, of which nothing lies past the cone at a chamfered end, at
// 45° from the radius the thread's band gives at the end's plane out to
// the crest, nor past the end's plane, where the cone meets the end
// face: the ridges, which start past the end, are cut off whole.
func TestThreadChamfer(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	for _, tc := range fixtures.Sample(chamferCases) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, res := tc.build(t, 0)
			requireSoundThread(t, sk, res)
			o := tc.options(t)
			bands := sk.ThreadBands(lastOf(t, sk, res.Group).H)
			if len(bands) != 1 || bands[0].Chamfer == 0 || bands[0].ChamferFar != o.Through {
				t.Fatalf("bands %+v, want one chamfered, at the far end too %v", bands, o.Through)
			}
			requireChamferCone(t, sk, res.Group, o, bands[0])
		})
	}
}

// requireChamferCone requires nothing of the thread's solid past the
// cone at a chamfered end of its band b, at 45° from the radius the
// band gives at the end's plane out to the crest, nor past the end's
// plane, and the cone to meet that plane there.
func requireChamferCone(t *testing.T, sk *sketch.Sketch, keeper core.HGroup, o sketch.ThreadOptions, b sketch.ThreadBand) {
	t.Helper()
	out := 1.0
	if o.Internal {
		out = -1
	}
	reach := out * (b.Crest - b.Chamfer)
	// A rounded crest's cut is a curve on it: the points of its mesh
	// inside the trim's chords lie off the cone by up to their sag, the
	// more as the cone meets the crest at a shallow angle.
	slack := 1e-3
	if o.Crest == thread.CrestRounded {
		slack = max(slack, sk.Tol().ChordTolMm())
	}
	m, _ := meshStats(sk, lastOf(t, sk, keeper).H)
	onEnd := map[bool]bool{}
	for _, tr := range m.Triangles {
		for _, v := range [...]geom.Vector{tr.A, tr.B, tr.C} {
			z := b.Along(v)
			r := v.Minus(b.Origin).Minus(b.Axis.ScaledBy(z)).Magnitude()
			for _, far := range []bool{false, true} {
				if far && !o.Through {
					continue
				}
				h := z
				if far {
					h = o.Length - z
				}
				if h < -1e-6 {
					t.Fatalf("a vertex at r %.4f, %.4f past the end", r, -h)
				}
				if h > reach {
					continue
				}
				if cone := b.Chamfer + out*h; out*(r-cone) > slack {
					t.Fatalf("a vertex at r %.4f, %.4f from the end, is past the chamfer's cone at %.4f", r, h, cone)
				}
				if math.Abs(h) < 1e-6 && math.Abs(r-b.Chamfer) < 1e-3 {
					onEnd[far] = true
				}
			}
		}
	}
	if !onEnd[false] || (o.Through && !onEnd[true]) {
		t.Errorf("the cone meets the end's plane at %.4f: near end %v, far end %v", b.Chamfer, onEnd[false], onEnd[true])
	}
}

// TestThreadChamferRefused requires a chamfer refused, saying why,
// where the chamfers would meet or reach the run-out.
func TestThreadChamferRefused(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		tc   threadCase
		want string
	}{
		{"short stud", threadCase{"", 4, 2.5, "M8", sketch.ThreadOptions{Length: 2.5, Through: true, Chamfer: true}}, "for its chamfers"},
		{"run-out", threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 4, RunOut: 3, Chamfer: true}}, "for its chamfers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, _, err := tc.tc.tryBuild(t, 0)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want an error saying %q", err, tc.want)
			}
		})
	}
}

// TestThreadChamferSweep makes chamfered threads of many sizes and
// forms, screws and nuts, at chord tolerances from coarse to fine, and
// requires sound solids but at the rate fixed: an even sample by
// default, the whole grid with TELESAPR_SWEEP=1.
func TestThreadChamferSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	cases := map[string]threadCase{}
	var names []string
	add := func(tc threadCase) {
		cases[tc.name] = tc
		names = append(names, tc.name)
	}
	for _, d := range []string{"M3", "M5", "M8", "M10", "M12", "M16", "M8x1", "1/2-13 UNC", "Tr12x3", "1/2 BSF", "3/4-6 ACME"} {
		spec, err := thread.Parse(d)
		if err != nil {
			t.Fatal(err)
		}
		r := spec.Diameter / 2
		add(threadCase{d + " screw", r, 3 * r * 2, d, sketch.ThreadOptions{Length: 3 * r, Chamfer: true}})
		add(threadCase{d + " stud", r, 3 * r, d, sketch.ThreadOptions{Length: 3 * r, Through: true, Chamfer: true}})
		add(threadCase{d + " nut", 2 * r, 1.6 * r, d, sketch.ThreadOptions{Length: 1.6 * r, Through: true, Internal: true, Chamfer: true}})
		add(threadCase{d + " tapped", 2 * r, 4 * r, d, sketch.ThreadOptions{Length: 2.4 * r, Internal: true, Chamfer: true}})
	}
	sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("tol", 0.5, 0.2, 0.1, 0.05, 0.02),
			sweep.Strings("case", names...),
			sweep.Floats("longer", 0, 0.37, 1.41),
		},
		MaxFailRate: threadChamferMaxFail,
		Sample:      12,
	}.Run(func(p sweep.Point) error {
		tc := cases[p.String("case")]
		if !tc.o.Internal && !tc.o.Through {
			tc.height += p.Float("longer")
		} else {
			tc.height += math.Copysign(p.Float("longer"), tc.height)
		}
		tc.o.Length += p.Float("longer")
		_, _, res, err := tc.tryBuild(t, p.Float("tol"))
		if errors.Is(err, sketch.ErrChamferTooFine) {
			// Refused, saying why: a Tr12x3 screw's cones would reach
			// its axis.
			return nil
		}
		if err != nil {
			return err
		}
		if !res.Sound {
			return errors.New("the solid isn't sound")
		}
		return nil
	}).Require(t)
}

// threadChamferMaxFail is the rate of the chamfer sweep allowed to
// fail: none since 2026-10-29, when the Boolean's curves were refined
// beside grazing meets (srf.SShell.refineGrazingMeets). It was 1 of 660
// before, a 1/2 BSF nut at the coarsest chord tolerance, and 4 of 660
// when a march from a seed whose curve leaves it grazing took the wrong
// way and was dropped (srf.marchFrom).
const threadChamferMaxFail = 0

// TestThreadChamferCoarse makes the chamfered threads that were unsound
// at the coarsest chord tolerance the sweeps try, 0.5%: where a ridge's
// foot crosses the circle at which the bore or core turns into the
// chamfer's cone, the two curves leave their meeting 2° apart, and each
// chorded on its own, the circle's chord passed through the ridge's
// flank beside it (srf.SShell.refineGrazingMeets).
func TestThreadChamferCoarse(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	rounded := sketch.ThreadOptions{Crest: thread.CrestRounded, Chamfer: true}
	with := func(o sketch.ThreadOptions, length float64, through, internal bool) sketch.ThreadOptions {
		o.Length, o.Through, o.Internal = length, through, internal
		return o
	}
	// The sizes of TestThreadChamferSweep: r the thread's radius.
	bsf, m8, m8x1 := 6.35, 4.0, 4.0
	// Under the race detector a fifth of them: each took up to 13 minutes
	// there, through the same goroutines.
	for _, tc := range fixtures.Sample([]threadCase{
		{"bsf_nut", 2 * bsf, 1.6*bsf + 1.41, "1/2 BSF", sketch.ThreadOptions{Length: 1.6*bsf + 1.41, Through: true, Internal: true, Chamfer: true}},
		{"rounded_nut_m8", 2 * m8, 1.6 * m8, "M8", with(rounded, 1.6*m8, true, true)},
		{"rounded_screw_bsf", bsf, 6 * bsf, "1/2 BSF", with(rounded, 3*bsf, false, false)},
		{"rounded_tapped_bsf", 2 * bsf, 4 * bsf, "1/2 BSF", with(rounded, 2.4*bsf, false, true)},
		{"rounded_tapped_m8x1", 2 * m8x1, 4 * m8x1, "M8x1", with(rounded, 2.4*m8x1, false, true)},
	}) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, res, err := tc.tryBuild(t, 0.5)
			if err != nil || !res.Sound {
				t.Errorf("sound %v, %v", res.Sound, err)
			}
		})
	}
}

// TestCosmeticThreadChamfer makes chamfered cosmetic threads: the
// part's end is cut by the chamfer's cone, by the thread's lathe, which
// a part left at its size has for the chamfer alone, and the solid is
// the part less the cone's ring, or the nut's bore less its
// countersink's, to within the 1% its faceted cylinders lose.
func TestCosmeticThreadChamfer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		threadCase
		part float64
	}{
		{threadCase{"rod as thick", 5, 30, "M10", sketch.ThreadOptions{Length: 20}}, math.Pi * 25 * 30},
		// Turned down to the crest over the thread's 20 mm.
		{threadCase{"rod thicker", 6, 30, "M10", sketch.ThreadOptions{Length: 20}}, -1},
		{threadCase{"stud", 4, 16, "M8", sketch.ThreadOptions{Length: 16, Through: true}}, math.Pi * 16 * 16},
		{threadCase{"nut", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true}}, 0},
		{threadCase{"blind", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.o.Cosmetic, tc.o.Chamfer = true, true
			sk, _, res := tc.build(t, 0)
			if !res.Sound {
				t.Error("not sound")
			}
			gs, _ := sk.ThreadGroups(res.Group)
			if len(gs) != 2 || sk.Group(gs[1]).Type != sketch.GroupLathe {
				t.Fatalf("groups %v, want the sketch and its lathe", gs)
			}
			bands := sk.ThreadBands(gs[1])
			if len(bands) != 1 || bands[0].Chamfer == 0 || !bands[0].Cosmetic {
				t.Fatalf("bands %+v", bands)
			}
			b := bands[0]
			o := tc.options(t)
			requireChamferCone(t, sk, res.Group, o, b)
			// The ring a cone at 45° cuts from c at an end to the crest
			// R, revolved: the cylinder of R less the frustum, or the
			// frustum less the cylinder.
			c, rr := b.Chamfer, b.Crest
			h := math.Abs(rr - c)
			ring := math.Abs(math.Pi*rr*rr*h - math.Pi*h/3*(c*c+c*rr+rr*rr))
			ends := 1.0
			if o.Through {
				ends = 2
			}
			part := tc.part
			if part < 0 {
				part = math.Pi * (tc.radius*tc.radius*(math.Abs(tc.height)-o.Length) + rr*rr*o.Length)
			}
			want := part - ends*ring
			if o.Internal {
				want = math.Pi*(tc.radius*tc.radius*math.Abs(tc.height)-rr*rr*o.Length) - ends*ring
			}
			if v := solidVolume(t, sk, res.Group); math.Abs(v-want) > 0.01*want {
				t.Errorf("volume %.3f, want %.3f", v, want)
			}
		})
	}
}

// TestThreadOnFaceChamfer makes a full-length chamfered thread on a
// rod's face: through the rod, so chamfered at both ends, and sound.
func TestThreadOnFaceChamfer(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, part := rodPart(t, 4, 20)
	face, _ := roundFace(t, sk, part)
	p, err := sk.ThreadPlaceOnFace(face, false)
	if err != nil {
		t.Fatal(err)
	}
	o := threadCase{"", 4, 20, "M8", sketch.ThreadOptions{FullLength: true, Length: 5, Chamfer: true}}.options(t)
	res, err := sk.AddThreadOnFace(p, o)
	if err != nil {
		t.Fatal(err)
	}
	requireSoundThread(t, sk, res)
	got, _ := sk.ThreadOf(res.Group)
	if !got.Chamfer || !got.Through {
		t.Fatalf("options %+v", got)
	}
	bands := sk.ThreadBands(lastOf(t, sk, res.Group).H)
	if len(bands) != 1 || bands[0].Chamfer == 0 || !bands[0].ChamferFar {
		t.Fatalf("bands %+v", bands)
	}
	requireChamferCone(t, sk, res.Group, got, bands[0])
}
