package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// TestThreadLoadStated checks the way of the load a B1.9 designation
// says against the ridge it sets: drawn, a screw pulls (its clearance
// flank leading) and a nut pushes (its load flank leading), so pushing
// turns a screw's ridge and pulling a nut's; a designation that doesn't
// say, as a file's, leaves the ridge; and the thread writes back the
// way its ridge makes.
func TestThreadLoadStated(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		designation string
		internal    bool
		back        bool
	}{
		{"1-12 BUTT", false, false},
		{"1-12 PULL-BUTT", false, false},
		{"1-12 PUSH-BUTT", false, true},
		{"1-12 BUTT", true, true},
		{"1-12 PUSH-BUTT", true, false},
	} {
		s, err := thread.Parse(c.designation)
		if err != nil {
			t.Fatal(err)
		}
		for _, was := range []bool{false, true} {
			o := sketch.ThreadOptions{Spec: s, Internal: c.internal, LoadFlankBack: was}.LoadStated()
			if o.LoadFlankBack != c.back || o.Spec.Load != thread.LoadUnstated {
				t.Errorf("%s internal %v: load flank back %v, load %v; want %v", c.designation, c.internal,
					o.LoadFlankBack, o.Spec.Load, c.back)
			}
			again, err := thread.Parse(o.Designation())
			if err != nil {
				t.Fatal(err)
			}
			if back := (sketch.ThreadOptions{Spec: again, Internal: c.internal}).LoadStated().LoadFlankBack; back != c.back {
				t.Errorf("%s internal %v: written %q, which turns the ridge %v", c.designation, c.internal, o.Designation(), back)
			}
		}
	}
	// A spec not read from a designation, as a file's, keeps the ridge.
	s, _ := thread.Parse("1-12 BUTT")
	s.Load = thread.LoadUnstated
	if o := (sketch.ThreadOptions{Spec: s, LoadFlankBack: true}).LoadStated(); !o.LoadFlankBack {
		t.Error("an unstated way turned the ridge back")
	}
	// DIN 513 designations don't say: the ridge stays as set.
	m, _ := thread.Parse("S20x4")
	if o := (sketch.ThreadOptions{Spec: m, LoadFlankBack: true}).LoadStated(); !o.LoadFlankBack || o.Designation() != "S20x4" {
		t.Errorf("S20x4: %+v, written %q", o.LoadFlankBack, o.Designation())
	}
}
