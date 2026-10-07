package res

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"

	"github.com/telesapr/telesapr/pkg/geom"
)

// The strings and code points the golden covers, as the C++ harness that
// recorded it used them.
var fontTestStrings = []string{
	"Hello, world!", "0.123 mm", "\u2220 45\u00b0", "日本語",
	"Ω≈ƒ", "(x) [y] {z}", `@#$%^&*_+=|\/<>?~` + "`", "\ufffe\U0001F600",
	"géométrie", "AVAWAY", "",
}

var fontTestCodepoints = []rune{'A', ' ', 'g', 0xE000, 0xE003, 0xE007, 0x00B0, 0x2220, 0x65E5,
	0x672C, 0x8A9E, 0xFFFE, 0x1F600, 0x0416, 0x05D0, 0x3042}

// TestFonts compares the vector and bitmap fonts with the golden
// (testdata/res/fonts.golden, first recorded from C++ resource.cpp):
// metrics, widths and traced strokes bit for bit, and each bitmap
// glyph's texture slot and pixels.
func TestFonts(t *testing.T) {
	var b strings.Builder
	vf := BuiltinVectorFont()
	fmt.Fprintf(&b, "vector cap %x asc %x desc %x rsb %x\n", vf.CapHeight, vf.Ascender, vf.Descender, vf.RightSideBearing)
	for _, s := range fontTestStrings {
		fmt.Fprintf(&b, "string %d width %x height %x\n", len(s), vf.Width(3.5, s), vf.Height(3.5))
		vf.Trace(3.5, geom.Vector{X: 1, Y: 2, Z: 3}, geom.Vector{X: 1, Y: 0.5}, geom.Vector{Y: 0.25, Z: 1}, s,
			func(a, c geom.Vector) {
				fmt.Fprintf(&b, "%x %x %x %x %x %x\n", a.X, a.Y, a.Z, c.X, c.Y, c.Z)
			})
	}

	bf := NewBitmapFont()
	for _, cp := range fontTestCodepoints {
		s0, t0, s1, t1, w, h := bf.LocateGlyph(cp)
		fmt.Fprintf(&b, "glyph %x width %d loc %x %x %x %x %d %d\n", cp, bf.Width(cp), s0, t0, s1, t1, w, h)
		x0, y0 := int(s0*float64(bf.Texture.Width)), int(t0*float64(bf.Texture.Height))
		for y := range h {
			for x := range w {
				if bf.Texture.Data[(y0+y)*bf.Texture.Stride+x0+x] != 0 {
					b.WriteByte('#')
				} else {
					b.WriteByte('.')
				}
			}
			b.WriteByte('\n')
		}
	}
	golden.CheckDump(t, filepath.Join("..", "..", "testdata", "res", "fonts.golden"), b.String())
}
