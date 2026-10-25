package res

import (
	"sync"
	"testing"
)

// The builtin vector font is the process's: glyphs asked for on several
// goroutines at once, as text drawn side by side asks for them, are
// parsed once and the same for each.
func TestVectorFontConcurrentGlyphs(t *testing.T) {
	t.Parallel()
	f := VectorFontFrom(LoadStringFromGzip("fonts/unicode.lff.gz"))
	text := []rune("TeleSAPR Ø∅±°µ")
	var wg sync.WaitGroup
	got := make([][]*VectorGlyph, 8)
	for i := range got {
		wg.Go(func() {
			for _, r := range text {
				got[i] = append(got[i], f.Glyph(r))
			}
		})
	}
	wg.Wait()
	for i := range got {
		for j, g := range got[i] {
			if g != got[0][j] {
				t.Fatalf("goroutine %d: glyph %q parsed apart", i, text[j])
			}
		}
	}
}
