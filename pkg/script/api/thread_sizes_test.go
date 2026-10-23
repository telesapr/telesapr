package api

import (
	"slices"
	"strings"
	"testing"
)

// The series, sizes and pitches a script can pick a thread from are
// those of the screen's [choose], and every one of them is read.
func TestThreadSizes(t *testing.T) {
	t.Parallel()
	series := ThreadSeries()
	if !slices.Equal(series, []string{"metric", "unified", "trapezoidal", "Whitworth", "pipe", "ACME"}) {
		t.Errorf("series %v", series)
	}
	for _, sr := range series {
		sizes := ThreadSizes(strings.ToLower(sr))
		if len(sizes) == 0 {
			t.Errorf("%s: no sizes", sr)
		}
		for _, size := range sizes {
			if ThreadDesignation(size) != size {
				t.Errorf("%s: written as %s", size, ThreadDesignation(size))
			}
			if !slices.Contains(ThreadPitches(size), size) {
				t.Errorf("%s: not among its pitches %v", size, ThreadPitches(size))
			}
		}
	}
	if got := ThreadPitches("M8-LH"); !slices.Equal(got, []string{"M8-LH", "M8x1-LH", "M8x0.75-LH"}) {
		t.Errorf("ThreadPitches(M8-LH) = %v", got)
	}
	if got := ThreadDesignation("m8x1.25"); got != "M8" {
		t.Errorf("ThreadDesignation(m8x1.25) = %s", got)
	}
	for name, fn := range map[string]func(){
		"series":      func() { ThreadSizes("BSPT") },
		"pitches":     func() { ThreadPitches("Q9") },
		"designation": func() { ThreadDesignation("M7.5") },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}

// LeftHand makes a left-hand thread as -LH does; Options gives the
// hand apart from the designation, so that it can be changed alone,
// and a model written as a script keeps it.
func TestThreadLeftHand(t *testing.T) {
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 6.35)
	d.Features.Extrude(30, "rod")
	th := d.Features.Thread(rod, ThreadOptions{Designation: "1/2-10 ACME", LeftHand: true, Length: 20}).AsThread()
	o := th.Options()
	if o.Designation != "1/2-10 ACME" || !o.LeftHand {
		t.Fatalf("options %+v", o)
	}
	if src, _ := d.Script(); !strings.Contains(src, `Designation: "1/2-10 ACME", LeftHand: true`) {
		t.Errorf("the script lost the hand:\n%s", src)
	}
	if r := th.Groups()[3].Analyze().Check(); !r.Sound() {
		t.Errorf("the ACME thread is not sound: %s", r.String())
	}
	o.LeftHand = false
	th.Edit(o)
	if o := th.Options(); o.LeftHand {
		t.Errorf("still left-hand: %+v", o)
	}
	// An edit that keeps the designation keeps the hand of one with -LH.
	th.Edit(ThreadOptions{Designation: "1/2-10 ACME-LH"})
	th.Edit(ThreadOptions{Crest: "rounded"})
	if o := th.Options(); !o.LeftHand || o.Designation != "1/2-10 ACME" {
		t.Errorf("the edit of the crest changed the hand: %+v", o)
	}
}
