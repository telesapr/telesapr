package slvs

import (
	"bytes"
	"math"
	"os"
	"regexp"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// solidVolume is the oracle's volume of the solid of the thread's last
// group, which must be sound.
func solidVolume(t *testing.T, sk *sketch.Sketch, keeper core.HGroup) float64 {
	t.Helper()
	r := check.Group(sk, lastOf(t, sk, keeper))
	if !r.Sound() || r.Empty() {
		t.Fatalf("oracle: %s", r.String())
	}
	return r.Volume
}

// minorRadius is the radius a cosmetic nut's thread bores to.
func minorRadius(t *testing.T, designation string) float64 {
	t.Helper()
	s, err := thread.Parse(designation)
	if err != nil {
		t.Fatal(err)
	}
	return s.Profile(true, 0).CrestRadius
}

// TestCosmeticThread makes cosmetic threads and requires the groups
// and the solid each kind takes: a rod of the thread's size left as it
// is, a thicker one turned down to the major diameter, a nut's part
// bored to the minor one, and a hole already that wide left; none has a
// helix, and each is a band drawings show by the convention.
func TestCosmeticThread(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		threadCase
		groups int
		volume float64
	}{
		{threadCase{"rod as thick", 5, 30, "M10", sketch.ThreadOptions{Length: 20}}, 1, math.Pi * 25 * 30},
		// Turned down to 5 over the thread's 20 mm.
		{threadCase{"rod thicker", 6, 30, "M10", sketch.ThreadOptions{Length: 20}}, 2, math.Pi * (36*10 + 25*20)},
		{threadCase{"downwards", 3, -20, "M6", sketch.ThreadOptions{Length: 10, Reverse: true}}, 1, math.Pi * 9 * 20},
		{threadCase{"nut", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true}}, 2, 0},
		{threadCase{"blind", 8, 12, "M6", sketch.ThreadOptions{Length: 8, Internal: true}}, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.o.Cosmetic = true
			sk, _, res := tc.build(t, 0)
			if !res.Sound {
				t.Error("not sound")
			}
			gs, err := sk.ThreadGroups(res.Group)
			if err != nil {
				t.Fatal(err)
			}
			if len(gs) != tc.groups {
				t.Errorf("%d groups, want %d", len(gs), tc.groups)
			}
			for _, hg := range gs {
				g := sk.Group(hg)
				if g.Type == sketch.GroupHelix {
					t.Errorf("a cosmetic thread has a helix, %s", g.Name)
				}
				if g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 {
					t.Errorf("group %s: solved %v with %d degrees of freedom", g.Name, g.Solved.How, g.Solved.Dof)
				}
			}
			if g := sk.Group(res.Group); g.Type != sketch.GroupDrawingWorkplane || !g.Thread.Cosmetic {
				t.Errorf("the thread is kept on a %v", g.Type)
			}
			want := tc.volume
			if tc.o.Internal {
				rm := minorRadius(t, tc.designation)
				want = math.Pi * (tc.radius*tc.radius*math.Abs(tc.height) - rm*rm*tc.o.Length)
			}
			if v := solidVolume(t, sk, res.Group); math.Abs(v-want) > 0.01*want {
				t.Errorf("volume %.2f, want %.2f", v, want)
			}
			bands := sk.ThreadBands(gs[len(gs)-1])
			if len(bands) != 1 || !bands[0].Cosmetic || bands[0].Length != tc.o.Length {
				t.Errorf("bands %+v", bands)
			}
		})
	}
}

// TestCosmeticThreadOnHole requires a nut's cosmetic thread on a hole
// already as wide as its minor diameter to leave the part as it is.
func TestCosmeticThreadOnHole(t *testing.T) {
	t.Parallel()
	tc := threadCase{"nut", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true, Cosmetic: true}}
	sk, circle, first := tc.build(t, 0)
	before := solidVolume(t, sk, first.Group)
	res, err := sk.AddThread(circle, tc.options(t))
	if err != nil {
		t.Fatal(err)
	}
	if gs, _ := sk.ThreadGroups(res.Group); len(gs) != 1 {
		t.Errorf("%d groups on a hole there already, want the sketch alone", len(gs))
	}
	if v := solidVolume(t, sk, res.Group); math.Abs(v-before) > 1e-6*before {
		t.Errorf("volume %.4f, was %.4f", v, before)
	}
}

// TestEditCosmeticThread edits threads from cosmetic to modelled and
// back: the first sketch keeps its handle, a lathe the part needs is
// kept or made, a modelled thread's ridges and helix come and go, and
// the solid is what a thread made so at once has.
func TestEditCosmeticThread(t *testing.T) {
	t.Parallel()
	for _, tc := range []threadCase{
		{"screw", 4, 30, "M8", sketch.ThreadOptions{Length: 12}},
		{"nut", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true}},
		{"level starts", 6, 20, "M12xPh3P1.5", sketch.ThreadOptions{Length: 14, LevelStarts: true}},
		{"chamfered screw", 4, 30, "M8", sketch.ThreadOptions{Length: 12, Chamfer: true}},
		{"chamfered nut", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true, Chamfer: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			modelled := tc.options(t)
			cosmetic := modelled
			cosmetic.Cosmetic = true

			ref, _, refRes := tc.build(t, 0)
			wantModelled := solidVolume(t, ref, refRes.Group)
			refGroups, _ := ref.ThreadGroups(refRes.Group)

			tcC := tc
			tcC.o.Cosmetic = true
			sk, _, res := tcC.build(t, 0)
			wantCosmetic := solidVolume(t, sk, res.Group)
			first := res.Group

			res, err := sk.EditThread(res.Group, modelled)
			if err != nil {
				t.Fatal(err)
			}
			requireSoundThread(t, sk, res)
			gs, _ := sk.ThreadGroups(res.Group)
			if len(gs) != len(refGroups) || gs[0] != first || sk.Group(res.Group).Type != sketch.GroupHelix {
				t.Errorf("modelled: groups %v, kept by %v; want %d from %v", gs, res.Group, len(refGroups), first)
			}
			if g := sk.Group(first); g.Thread.IsThread() {
				t.Error("the sketch still keeps the thread")
			}
			if v := solidVolume(t, sk, res.Group); math.Abs(v-wantModelled) > 1e-3*wantModelled {
				t.Errorf("modelled volume %.3f, made so at once %.3f", v, wantModelled)
			}

			res, err = sk.EditThread(res.Group, cosmetic)
			if err != nil {
				t.Fatal(err)
			}
			if res.Group != first || !sk.Group(first).Thread.Cosmetic {
				t.Errorf("cosmetic again: kept by %v, want %v", res.Group, first)
			}
			gs, _ = sk.ThreadGroups(res.Group)
			for _, hg := range gs {
				if sk.Group(hg).Type == sketch.GroupHelix {
					t.Error("a helix is left")
				}
			}
			if len(sk.Groups) != 3+len(gs) {
				t.Errorf("%d groups in all, want the part's 3 and the thread's %d", len(sk.Groups), len(gs))
			}
			if v := solidVolume(t, sk, res.Group); math.Abs(v-wantCosmetic) > 1e-6*wantCosmetic {
				t.Errorf("cosmetic volume %.4f, made so at once %.4f", v, wantCosmetic)
			}
		})
	}
}

// TestCosmeticThreadSaved requires a cosmetic thread to come back from
// its file as it was saved, and to edit from there.
func TestCosmeticThreadSaved(t *testing.T) {
	t.Parallel()
	tc := threadCase{"rod thicker", 6, 30, "M10", sketch.ThreadOptions{Length: 20, Cosmetic: true}}
	sk, _, res := tc.build(t, 0)
	data := (&tlspr.Document{Sketch: sk, Version: tlspr.Version, Units: sk.Units}).Encode()
	doc, err := tlspr.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	got := doc.Sketch
	got.GenerateAll()
	o, err := got.ThreadOf(res.Group)
	if err != nil || !o.Cosmetic {
		t.Fatalf("read back %+v, %v", o, err)
	}
	if again := (&tlspr.Document{Sketch: got, Version: tlspr.Version, Units: got.Units}).Encode(); string(again) != string(data) {
		t.Errorf("saved again:\n%s", firstDiff(string(again), string(data)))
	}
	o.Cosmetic = false
	r, err := got.EditThread(res.Group, o)
	if err != nil {
		t.Fatal(err)
	}
	requireSoundThread(t, got, r)
}

// TestCosmeticThreadPastPart requires a cosmetic thread longer than
// its part to be refused like a modelled one, and a part made shorter
// later to put the problem on the thread's sketch.
func TestCosmeticThreadPastPart(t *testing.T) {
	t.Parallel()
	tc := threadCase{"longer", 5, 10, "M10", sketch.ThreadOptions{Length: 20, Cosmetic: true}}
	if _, _, _, err := tc.tryBuild(t, 0); err == nil {
		t.Error("a cosmetic thread longer than its part was made")
	}
	tc.o.Length = 8
	sk, _, res := tc.build(t, 0)
	part := sk.Group(res.Group).PreviousGroup(sk)
	sk.MustParam(part.H.Param(2)).Val = 3
	sk.GenerateAllTyped(sketch.GenerateAll)
	if p := sk.Group(res.Group).Problem; p.Msg == "" {
		t.Error("no problem on the thread's sketch once the part is shorter")
	}
}

// cosmeticGoldens are cosmetic threads kept as files: a rod turned
// down to the thread, and a nut bored to its minor diameter.
var cosmeticGoldens = []threadCase{
	{"cosmetic_screw_m10", 6, 30, "M10", sketch.ThreadOptions{Length: 20, Cosmetic: true}},
	{"cosmetic_nut_m8", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true, Cosmetic: true}},
}

// noise matches a number of magnitude below 1e-12 in the file's text: a
// coordinate the solver left at zero to within its last bits, which a
// fused multiply-add (arm64) leaves at another of them.
var noise = regexp.MustCompile(`-?\d(?:\.\d+)?e-(?:1[3-9]|[2-9]\d|\d{3})\b`)

// checkWithoutNoise is golden.Check with those numbers read as 0, so
// that the platform's rounding of a zero is not a difference.
func checkWithoutNoise(t *testing.T, path string, got []byte) {
	t.Helper()
	if golden.Regen() {
		golden.Check(t, path, got)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zero := func(b []byte) []byte { return noise.ReplaceAll(b, []byte("0")) }
	if !bytes.Equal(zero(want), zero(got)) {
		t.Errorf("differs from %s: %s", path, golden.FirstDifference(zero(want), zero(got)))
	}
}

// TestCosmeticThreadGoldens saves each cosmetic golden, opens and
// saves it again, and compares it with its file in testdata/thread.
func TestCosmeticThreadGoldens(t *testing.T) {
	t.Parallel()
	for _, tc := range cosmeticGoldens {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, _ := tc.build(t, 0)
			sk.GenerateAll()
			doc, err := tlspr.Decode(tlspr.Encode(sk, tlspr.Options{}))
			if err != nil {
				t.Fatal(err)
			}
			doc.Sketch.GenerateAll()
			checkWithoutNoise(t, fixture("thread/"+tc.name+".tlspr"), tlspr.Encode(doc.Sketch, tlspr.Options{}))
		})
	}
}
