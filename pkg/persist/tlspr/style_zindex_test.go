package tlspr

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A style in a file saved before the level was kept has no z-index and is
// made at the level a new custom style has.
func TestStyleWithoutZIndexGetsDefault(t *testing.T) {
	t.Parallel()
	doc, err := Decode([]byte("telesapr 1\nstyle 0x100 \"s\" {\n  width 2\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	st := doc.Sketch.Style(core.HStyle{V: 0x100})
	if st == nil {
		t.Fatal("style not loaded")
	}
	if want := core.StyleDefaults[0].ZIndex; st.ZIndex != want {
		t.Errorf("ZIndex = %d, want the default %d", st.ZIndex, want)
	}
}

// A style set to level 0 on purpose comes back as 0, not as the default.
func TestStyleZIndexRoundTrip(t *testing.T) {
	t.Parallel()
	for _, z := range []int{0, 1, 7, -2} {
		sk := sketch.NewSketch()
		h := sk.NewCustomStyle()
		sk.StyleGet(h).ZIndex = z
		doc, err := Decode(Encode(sk, Options{}))
		if err != nil {
			t.Fatal(err)
		}
		st := doc.Sketch.Style(h)
		if st == nil || st.ZIndex != z {
			t.Errorf("z-index %d came back as %+v", z, st)
		}
	}
}
