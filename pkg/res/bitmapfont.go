package res

import "github.com/telesapr/telesapr/pkg/internal/cxxutf8"

// BitmapGlyph is a glyph's width in 8-pixel cells and its slot in the
// texture (C++ BitmapFont::Glyph).
type BitmapGlyph struct {
	AdvanceCells uint8
	Position     uint16
}

// BitmapFont is the text window's font: GNU Unifont's 8x16 and 16x16
// glyphs, rendered on demand into a 1024x1024 alpha texture (C++
// BitmapFont).
type BitmapFont struct {
	unifontData    string
	glyphs         map[rune]BitmapGlyph
	Texture        *Pixmap
	TextureUpdated bool
	nextPosition   uint16
}

// BitmapFontFrom makes a font from Unifont's .hex data (C++
// BitmapFont::From).
func BitmapFontFrom(unifontData string) *BitmapFont {
	return &BitmapFont{
		unifontData: unifontData,
		glyphs:      make(map[rune]BitmapGlyph),
		Texture:     NewPixmap(FormatA, 1024, 1024),
	}
}

// NewBitmapFont loads the bundled Unifont and adds the private glyphs:
// U+0020, and at U+E000..U+E007 the checkboxes, radio buttons and
// stipple samples (C++ BitmapFont::Create).
func NewBitmapFont() *BitmapFont {
	f := BitmapFontFrom(LoadStringFromGzip("fonts/unifont.hex.gz"))
	// Unifont doesn't have a glyph for U+0020.
	f.AddGlyph(0x0020, NewPixmap(FormatRGB, 8, 16))
	for i, name := range []string{
		"0-check-false", "1-check-true", "2-radio-false", "3-radio-true",
		"4-stipple-dot", "5-stipple-dash-long", "6-stipple-dash", "7-stipple-zigzag",
	} {
		f.AddGlyph(rune(0xE000+i), LoadPng("fonts/private/"+name+".png"))
	}
	return f
}

// IsEmpty reports whether the font has no data (C++ IsEmpty).
func (f *BitmapFont) IsEmpty() bool { return f == nil || f.unifontData == "" }

func (f *BitmapFont) textureRow(position uint16, y int) []byte {
	perRow := uint16(f.Texture.Width / 16)
	col := int(position % perRow)
	row := int(position / perRow)
	return f.Texture.Data[f.Texture.Stride*(16*row+y)+16*col:]
}

// AddGlyph adds an 8x16 or 16x16 RGB glyph; any non-black pixel is
// ink (C++ BitmapFont::AddGlyph).
func (f *BitmapFont) AddGlyph(codepoint rune, pm *Pixmap) {
	if (pm.Width != 8 && pm.Width != 16) || pm.Height != 16 {
		panic("res: unexpected pixmap dimensions")
	}
	if pm.Format != FormatRGB {
		panic("res: unexpected pixmap format")
	}
	if _, ok := f.glyphs[codepoint]; ok {
		panic("res: glyph with this codepoint already exists")
	}
	if f.nextPosition == 0xffff {
		panic("res: too many glyphs for current texture size")
	}
	g := BitmapGlyph{AdvanceCells: uint8(pm.Width / 8), Position: f.nextPosition}
	f.nextPosition++
	f.glyphs[codepoint] = g
	for y := range pm.Height {
		row := f.textureRow(g.Position, y)
		for x := range pm.Width {
			if pm.Pixel(x, y).ToPackedInt()&0xffffff != 0 {
				row[x] = 255
			}
		}
	}
}

// Glyph returns the glyph, rendering it into the texture the first
// time; a missing one is U+FFFD (C++ BitmapFont::Glyph).
func (f *BitmapFont) Glyph(codepoint rune) BitmapGlyph {
	if g, ok := f.glyphs[codepoint]; ok {
		return g
	}
	if f.nextPosition == 0xffff {
		panic("res: too many glyphs for current texture size")
	}

	// Find the hex representation in the (sorted) Unifont file.
	data := f.unifontData
	first, last := 0, len(data)
	for first < last {
		mid := first + (last-first)/2
		for mid != 0 {
			if data[mid] == '\n' {
				mid++
				break
			}
			mid--
		}

		r := asciiReader{s: data, pos: mid}
		if r.atEnd() {
			break
		}
		found := rune(r.read16HexBits())
		r.expectChar(':')

		if found > codepoint {
			last = mid - 1
			continue
		}
		if found < codepoint {
			first = mid + 1
			for first != len(data) {
				if data[first] == '\n' {
					first++
					break
				}
				first++
			}
			continue
		}

		g := BitmapGlyph{Position: f.nextPosition}
		f.nextPosition++

		var bits [16]uint16
		switch r.countUntilEol() {
		case 4 * 16:
			g.AdvanceCells = 2
			for i := range bits {
				bits[i] = r.read16HexBits()
			}
		case 2 * 16:
			g.AdvanceCells = 1
			for i := range bits {
				bits[i] = uint16(r.read8HexBits()) << 8
			}
		default:
			panic("res: unexpected glyph bitmap length")
		}

		for y := range 16 {
			row := f.textureRow(g.Position, y)
			for x := range 16 {
				if bits[y]&(1<<(15-x)) != 0 {
					row[x] = 255
				}
			}
		}
		f.glyphs[codepoint] = g
		f.TextureUpdated = true
		return g
	}

	// Glyph doesn't exist; return replacement glyph instead.
	if codepoint == 0xfffd {
		panic("res: cannot parse replacement glyph")
	}
	return f.Glyph(0xfffd)
}

// LocateGlyph returns where the glyph is in the texture, as texture
// coordinates, and its size in pixels (C++ BitmapFont::LocateGlyph).
func (f *BitmapFont) LocateGlyph(codepoint rune) (s0, t0, s1, t1 float64, w, h int) {
	g := f.Glyph(codepoint)
	w = int(g.AdvanceCells) * 8
	h = 16
	perRow := uint16(f.Texture.Width / 16)
	s0 = (16.0 * float64(g.Position%perRow)) / float64(f.Texture.Width)
	s1 = s0 + float64(w)/float64(f.Texture.Width)
	t0 = (16.0 * float64(g.Position/perRow)) / float64(f.Texture.Height)
	t1 = t0 + float64(h)/float64(f.Texture.Height)
	return
}

// Width is the glyph's width in cells; the private checkboxes and
// radio buttons count as one, since Printf pads them (C++
// BitmapFont::Width).
func (f *BitmapFont) Width(codepoint rune) int {
	if codepoint >= 0xe000 && codepoint <= 0xefff {
		return 1
	}
	return int(f.Glyph(codepoint).AdvanceCells)
}

// StringWidth is the string's width in cells (C++
// BitmapFont::GetWidth(const std::string &)).
func (f *BitmapFont) StringWidth(str string) int {
	w := 0
	for _, c := range cxxutf8.Decode(str) {
		w += f.Width(rune(c))
	}
	return w
}
