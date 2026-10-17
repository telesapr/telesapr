package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// Alignment on one axis leaves the other as it was, and a switch of
// the text height's units keeps the size the text is drawn at.
func TestStyleTextAlignAndUnits(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	h := sk.NewCustomStyle()
	sk.SetStyleTextHorizontal(h, core.TextOriginRight)
	sk.SetStyleTextVertical(h, core.TextOriginTop)
	sk.SetStyleTextHorizontal(h, core.TextOriginLeft)
	if got, want := sk.StyleGet(h).TextOrigin, core.TextOriginLeft|core.TextOriginTop; got != want {
		t.Errorf("origin %#x, want %#x", got, want)
	}
	sk.SetStyleTextHorizontal(h, core.TextOriginNone)
	sk.SetStyleTextVertical(h, core.TextOriginBot)
	if got, want := sk.StyleGet(h).TextOrigin, core.TextOriginBot; got != want {
		t.Errorf("origin %#x, want %#x", got, want)
	}

	s := sk.StyleGet(h)
	s.TextHeightAs, s.TextHeight = core.UnitsPixels, 30
	before := sk.StyleTextHeight(h, sk.view().Scale)
	sk.SetStyleTextHeightUnits(h, core.UnitsMM)
	if after := sk.StyleTextHeight(h, sk.view().Scale); after < before-1e-9 || after > before+1e-9 {
		t.Errorf("drawn height %v after the switch, was %v", after, before)
	}
}
