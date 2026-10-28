package thread

// A buttress thread carries its load on one flank, nearly square to the
// axis, and has a trailing flank at a wide angle: a screw jack's, a
// vice's or a breech's thread, strong one way. Its ridge is drawn with
// the load flank facing along the thread, away from where it starts, on
// a screw, and towards where it starts in a nut, whose ridge fills the
// screw's groove: a nut and a screw made the same way round mate.

// buttressProfile is a buttress standard's basic profile: its flanks'
// angles to the plane square to the axis, in degrees, and its depths in
// pitches from the external thread's crest, at the nominal diameter.
type buttressProfile struct {
	load, trailing float64
	// pitchDepth is where the ridge and the groove are equally wide;
	// engagement where the internal thread's crest is; externalDepth
	// where the external thread's root is. internalRoot is how far the
	// internal thread's root lies beyond the nominal diameter.
	pitchDepth, engagement, externalDepth, internalRoot float64
}

// din513 is the S thread of DIN 513-1: H1 = 0.75 P of engagement, the
// external root h3 = 0.86777 P deep (a clearance a_c = 0.11777 P under
// the internal crest), and the internal root at the nominal diameter.
// The pitch line is half the engagement deep: the crest of either ridge
// is then 0.26384 P wide, DIN 513's e.
var din513 = buttressProfile{
	load: 3, trailing: 30,
	pitchDepth: 0.375, engagement: 0.75, externalDepth: 0.86777,
}

// asmeB19 is the buttress inch thread of ASME B1.9: a sharp triangle
// H = 0.89064 P high, truncated by f = 0.14532 P at either crest for
// h = 0.6 P of engagement and by s = 0.08261 P at either root, which is
// rounded by the standard and flat here. The pitch line is half the
// engagement deep, the external root 0.66271 P deep, and the internal
// one 0.06271 P past the nominal diameter; either crest is then
// 0.16316 P wide, B1.9's F.
var asmeB19 = buttressProfile{
	load: 7, trailing: 45,
	pitchDepth: 0.3, engagement: 0.6, externalDepth: 0.66271, internalRoot: 0.06271,
}

// buttress is the thread's buttress profile, if it has one.
func (s Spec) buttress() (buttressProfile, bool) {
	switch s.Form {
	case FormButtress:
		return din513, true
	case FormButtressInch:
		return asmeB19, true
	}
	return buttressProfile{}, false
}

// flanks are the angles of the external thread's lower flank, facing
// where the thread starts, and of its upper one, in degrees from the
// plane square to the axis.
func (s Spec) flanks() (lower, upper float64) {
	if b, ok := s.buttress(); ok {
		return b.trailing, b.load
	}
	half := s.FlankAngle() / 2
	return half, half
}

// Buttress tells whether the thread is a buttress one, whose ridge's
// flanks lie at different angles.
func (s Spec) Buttress() bool {
	_, ok := s.buttress()
	return ok
}

// buttressMetric are the diameters of DIN 513-2's S threads with their
// pitches, the usual one first: those of ISO 2902's trapezoidal threads,
// which DIN 513-2 takes from 10 mm.
func buttressMetric() []struct {
	d, pitch float64
	others   []float64
} {
	return trapezoidal[1:]
}

// ButtressPitch is the usual pitch of the S thread of diameter d, and
// whether the series has that diameter.
func ButtressPitch(d float64) (float64, bool) {
	for _, m := range buttressMetric() {
		if same(m.d, d) {
			return m.pitch, true
		}
	}
	return 0, false
}

// buttressInch are ASME B1.9's preferred diameters, each with the
// threads per inch a designation without them takes: the middle of
// those B1.9 recommends for its range of diameters (buttressInchTPI).
var buttressInch = []inchSize{
	{"1/2", 1.0 / 2, 16}, {"5/8", 5.0 / 8, 16}, {"3/4", 3.0 / 4, 12},
	{"7/8", 7.0 / 8, 12}, {"1", 1, 12}, {"1 1/4", 1 + 1.0/4, 10},
	{"1 3/8", 1 + 3.0/8, 10}, {"1 1/2", 1 + 1.0/2, 8}, {"1 3/4", 1 + 3.0/4, 8},
	{"2", 2, 8}, {"2 1/4", 2 + 1.0/4, 8}, {"2 1/2", 2 + 1.0/2, 8},
	{"2 3/4", 2 + 3.0/4, 6}, {"3", 3, 6}, {"3 1/2", 3 + 1.0/2, 6}, {"4", 4, 6},
}

// buttressInchTPI are the threads per inch ASME B1.9 recommends for a
// diameter in inches, coarsest first.
func buttressInchTPI(d float64) []float64 {
	switch {
	case d < 0.5-1e-9 || d > 4+1e-9:
		return nil
	case d < 0.75-1e-9:
		return []float64{12, 16, 20}
	case d < 1.0625-1e-9:
		return []float64{10, 12, 16}
	case d < 1.5-1e-9:
		return []float64{6, 8, 10, 12, 16}
	}
	return []float64{4, 5, 6, 8, 10, 12, 16}
}
