package ttf

import (
	"errors"
	"sort"
)

// FreeType's psnames module (psmodule.c): the standard glyph names,
// and the Unicode charmaps synthesized from glyph names.

var errNoUnicodeGlyphName = errors.New("ttf: no Unicode glyph name found")

const (
	numMacNames = 258
	numSIDNames = 391
	variantBit  = 0x80000000
)

// cString is the NUL-terminated string at off in s.
func cString(s string, off int) string {
	end := off
	for end < len(s) && s[end] != 0 {
		end++
	}
	return s[off:end]
}

// macintoshName is ps_get_macintosh_name.
func macintoshName(nameIndex uint) string {
	if nameIndex >= numMacNames {
		nameIndex = 0
	}
	return cString(ftStandardGlyphNames, int(ftMacNames[nameIndex]))
}

// standardString is ps_get_standard_strings; ok is false for a SID
// without one.
func standardString(sid uint) (string, bool) {
	if sid >= numSIDNames {
		return "", false
	}
	return cString(ftStandardGlyphNames, int(ftSIDNames[sid])), true
}

// adobeGlyphIndex is ft_get_adobe_glyph_index: the Unicode value of the
// glyph name name (without its suffix), from the Adobe Glyph List.
func adobeGlyphIndex(name string) uint32 {
	list := ftAdobeGlyphList
	at := func(i int) int { return int(list[i]) }
	if len(name) == 0 {
		return 0
	}
	// Names are C strings of plain chars, which are signed.
	char := func(i int) int { return int(int8(name[i])) }

	c := char(0)
	n := 1
	count := at(1)
	p := 2
	lo, hi := 0, count
	found := false
	for lo < hi {
		mid := (lo + hi) >> 1
		q := p + mid*2
		q = at(q)<<8 | at(q+1)
		c2 := at(q) & 127
		if c2 == c {
			p = q
			found = true
			break
		}
		if c2 < c {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if !found {
		return 0
	}

	for {
		if n >= len(name) {
			if at(p)&128 == 0 && at(p+1)&128 != 0 {
				return uint32(at(p+2)<<8 | at(p+3))
			}
			return 0
		}
		c = char(n)
		n++
		if at(p)&128 != 0 {
			p++
			if c != at(p)&127 {
				return 0
			}
			continue
		}

		p++
		count = at(p) & 127
		if at(p)&128 != 0 {
			p += 2
		}
		p++

		next := -1
		for ; count > 0; count, p = count-1, p+2 {
			offset := at(p)<<8 | at(p+1)
			if c == at(offset)&127 {
				next = offset
				break
			}
		}
		if next < 0 {
			return 0
		}
		p = next
	}
}

// hexDigit is ps_unicode_value's digit test: an uppercase hexadecimal
// digit's value, or 16.
func hexDigit(c byte) uint32 {
	d := uint32(c) - '0'
	if d >= 10 {
		d = uint32(c) - 'A'
		if d >= 6 {
			d = 16
		} else {
			d += 10
		}
	}
	return d
}

// unicodeValue is ps_unicode_value: the Unicode value of a glyph name,
// with variantBit set for a variant like `A.swash'.
func unicodeValue(name string) uint32 {
	at := func(i int) byte {
		if i < len(name) {
			return name[i]
		}
		return 0
	}

	// `uniXXXX'
	if at(0) == 'u' && at(1) == 'n' && at(2) == 'i' {
		var value uint32
		p := 3
		count := 4
		for ; count > 0; count, p = count-1, p+1 {
			d := hexDigit(at(p))
			if d >= 16 {
				break
			}
			value = value<<4 + d
		}
		if count == 0 {
			if at(p) == 0 {
				return value
			}
			if at(p) == '.' {
				return value | variantBit
			}
		}
	}

	// `uXXXX' to `uXXXXXX'
	if at(0) == 'u' {
		var value uint32
		p := 1
		count := 6
		for ; count > 0; count, p = count-1, p+1 {
			d := hexDigit(at(p))
			if d >= 16 {
				break
			}
			value = value<<4 + d
		}
		if count <= 2 {
			if at(p) == 0 {
				return value
			}
			if at(p) == '.' {
				return value | variantBit
			}
		}
	}

	// the Adobe Glyph List, for the name before a non-initial dot
	p := 0
	for p < len(name) && name[p] != 0 && name[p] != '.' {
		p++
	}
	var value uint32
	if p > 0 {
		value = adobeGlyphIndex(name[:p])
		if at(p) == '.' {
			value |= variantBit
		}
	}
	return value
}

// extraGlyphUnicodes and extraGlyphNames are glyphs the Adobe Glyph List
// doesn't handle (well).
var (
	extraGlyphUnicodes = [...]uint32{
		0x0394, 0x03A9, 0x2215, 0x00AD, 0x02C9,
		0x03BC, 0x2219, 0x00A0, 0x021A, 0x021B,
	}
	extraGlyphNames = [...]string{
		"Delta", "Omega", "fraction", "hyphen", "macron",
		"mu", "periodcentered", "space", "Tcommaaccent", "tcommaaccent",
	}
)

type uniMap struct {
	unicode    uint32
	glyphIndex uint32
}

// psUnicodes is a PS_UnicodesRec: a charmap built from glyph names.
type psUnicodes struct {
	maps []uniMap
}

// unicodesInit is ps_unicodes_init; glyphName gives each glyph's name,
// with ok false for none.
func unicodesInit(numGlyphs uint32, glyphName func(uint32) (string, bool)) (*psUnicodes, error) {
	var states [len(extraGlyphUnicodes)]int
	var extraGlyphs [len(extraGlyphUnicodes)]uint32

	var maps []uniMap
	for n := uint32(0); n < numGlyphs; n++ {
		gname, ok := glyphName(n)
		if !ok || gname == "" {
			continue
		}
		// ps_check_extra_glyph_name
		for i, name := range extraGlyphNames {
			if name == gname {
				if states[i] == 0 {
					states[i] = 1
					extraGlyphs[i] = n
				}
				break
			}
		}
		uniChar := unicodeValue(gname)
		if uniChar&^variantBit != 0 {
			// ps_check_extra_glyph_unicode
			for i, u := range extraGlyphUnicodes {
				if uniChar == u {
					states[i] = 2
					break
				}
			}
			maps = append(maps, uniMap{uniChar, n})
		}
	}
	for i := range extraGlyphUnicodes {
		if states[i] == 1 {
			maps = append(maps, uniMap{extraGlyphUnicodes[i], extraGlyphs[i]})
		}
	}
	if len(maps) == 0 {
		return nil, errNoUnicodeGlyphName
	}
	// glibc's qsort is a merge sort, so equal entries keep their order;
	// base glyphs sort before their variants.
	sort.SliceStable(maps, func(i, j int) bool {
		u1, u2 := maps[i].unicode&^variantBit, maps[j].unicode&^variantBit
		if u1 == u2 {
			return maps[i].unicode < maps[j].unicode
		}
		return u1 < u2
	})
	return &psUnicodes{maps: maps}, nil
}

// charIndex is ps_unicodes_char_index.
func (t *psUnicodes) charIndex(unicode uint32) uint32 {
	var result *uniMap
	lo, hi := 0, len(t.maps)
	mid := lo + (hi-lo)>>1
	for lo < hi {
		m := &t.maps[mid]
		if m.unicode == unicode {
			result = m
			break
		}
		baseGlyph := m.unicode &^ variantBit
		if baseGlyph == unicode {
			result = m
		}
		if baseGlyph < unicode {
			lo = mid + 1
		} else {
			hi = mid
		}
		// the pointer mid moves by the unsigned 32-bit difference, so
		// out of the range when unicode < baseGlyph
		mid += int(unicode - baseGlyph)
		if mid >= hi || mid < lo {
			mid = lo + (hi-lo)>>1
		}
	}
	if result != nil {
		return result.glyphIndex
	}
	return 0
}
