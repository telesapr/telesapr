package ttf

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// hexFloat formats x as C's printf %a does.
func hexFloat(x float64) string {
	if x == 0 {
		if math.Signbit(x) {
			return "-0x0p+0"
		}
		return "0x0p+0"
	}
	s := strconv.FormatFloat(x, 'x', -1, 64)
	// Go writes 0x1.8p+01; C writes 0x1.8p+1.
	i := strings.IndexByte(s, 'p')
	exp, _ := strconv.Atoi(s[i+1:])
	return fmt.Sprintf("%sp%+d", s[:i], exp)
}

// dump is what the C++ harness that recorded the first goldens printed,
// for the fonts in files and the strings strs.
func dump(files []string, strs []string) string {
	SetFontFiles(files)
	defer SetFontFiles(nil)

	var b strings.Builder
	var names []string
	for _, tf := range Fonts.List() {
		fmt.Fprintf(&b, "font %s|%s\n", tf.FontFileBaseName(), tf.Name)
		names = append(names, tf.FontFileBaseName())
	}

	origin := geom.Vector{X: 1.5, Y: -2.25, Z: 3.0}
	u := geom.Vector{X: 0.3, Y: 0.4, Z: 0.1}
	v := geom.Vector{X: -0.2, Y: 0.7, Z: 0.5}
	for _, file := range names {
		tf := Fonts.LoadFont(file)
		capHeight := 0.0
		if tf != nil {
			capHeight = tf.CapHeight
		}
		fmt.Fprintf(&b, "load %s %s\n", file, hexFloat(capHeight))
		for i, str := range strs {
			for kerning := 0; kerning < 2; kerning++ {
				ar := Fonts.AspectRatio(file, str, kerning != 0)
				var sbl srf.SBezierList
				Fonts.PlotString(file, str, &sbl, kerning != 0, origin, u, v)
				fmt.Fprintf(&b, "str %d %d %s %d\n", i, kerning, hexFloat(ar), len(sbl.Curves))
				for _, sb := range sbl.Curves {
					fmt.Fprintf(&b, "b %d", sb.Degree)
					for j := 0; j <= sb.Degree; j++ {
						c := sb.Ctrl[j]
						fmt.Fprintf(&b, " %s %s %s %s", hexFloat(c.X), hexFloat(c.Y),
							hexFloat(c.Z), hexFloat(sb.Weight[j]))
					}
					b.WriteString("\n")
				}
			}
		}
	}
	return b.String()
}
