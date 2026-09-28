package ttf

import "errors"

// Charmaps (ttcmap.c): the subtable formats 0, 2, 4, 6, 8, 10, 12, 13
// and 14.

const (
	encodingUnicode = iota + 1
	encodingMSSymbol
	encodingOther

	platformAppleUnicode = 0
	platformISO          = 2
	platformMicrosoft    = 3

	appleIDUnicode32    = 4
	appleIDFullUnicode  = 6
	msIDSymbol          = 0
	msIDUnicode         = 1
	msIDUCS4            = 10
	cmapFlagUnsorted    = 1
	cmapFlagOverlapping = 2
)

var errInvalidCmap = errors.New("ttf: invalid cmap")

type charmap struct {
	platformID, encodingID uint16
	encoding               int
	format                 uint16
	data                   []byte // from the subtable to the cmap table's end
	flags                  int

	// synthetic is a Unicode charmap built from glyph names.
	synthetic *psUnicodes
}

// findEncoding is sfnt_find_encoding, as far as FT_Select_Charmap's
// FT_ENCODING_UNICODE and the synthesized Unicode charmap go.
func findEncoding(platformID, encodingID uint16) int {
	switch platformID {
	case platformISO, platformAppleUnicode:
		return encodingUnicode
	case platformMicrosoft:
		switch encodingID {
		case msIDUCS4, msIDUnicode:
			return encodingUnicode
		case msIDSymbol:
			return encodingMSSymbol
		}
		return encodingOther
	}
	return encodingOther
}

// buildCmaps is tt_face_build_cmaps: every subtable of a known format
// that validates at FT_VALIDATE_DEFAULT becomes a charmap.
func (f *Face) buildCmaps() {
	table := f.cmapTable
	if len(table) < 4 {
		return
	}
	numCmaps := int(peek16(table, 2))
	p := 4
	for ; numCmaps > 0 && p+8 <= len(table); numCmaps-- {
		cm := &charmap{
			platformID: peek16(table, p),
			encodingID: peek16(table, p+2),
		}
		offset := int64(peek32(table, p+4))
		p += 8
		if offset == 0 || offset > int64(len(table))-2 {
			continue
		}
		cm.data = table[offset:]
		cm.format = peek16(cm.data, 0)
		var flags int
		var err error
		switch cm.format {
		case 0:
			err = validate0(cm.data)
		case 2:
			err = validate2(cm.data)
		case 8:
			err = validate8(cm.data)
		case 10:
			err = validate10(cm.data)
		case 4:
			flags, err = validate4(cm.data)
		case 6:
			err = validate6(cm.data)
		case 12, 13:
			err = validate12(cm.data)
		case 14:
			err = validate14(cm.data)
		default:
			continue
		}
		if err != nil {
			continue
		}
		cm.flags = flags
		f.charmaps = append(f.charmaps, cm)
	}
}

// SelectUnicodeCharmap is FT_Select_Charmap(FT_ENCODING_UNICODE)
// (find_unicode_charmap): the last UCS-4 charmap, else the last
// Unicode one.
func (f *Face) SelectUnicodeCharmap() error {
	if len(f.charmaps) == 0 {
		return errNoCharmap
	}
	for i := len(f.charmaps) - 1; i >= 0; i-- {
		cm := f.charmaps[i]
		if cm.encoding != encodingUnicode {
			continue
		}
		if (cm.platformID == platformMicrosoft && cm.encodingID == msIDUCS4) ||
			(cm.platformID == platformAppleUnicode && cm.encodingID == appleIDUnicode32) ||
			(cm.platformID == platformAppleUnicode && cm.encodingID == appleIDFullUnicode && cm.format == 13) {
			f.charmap = cm
			return nil
		}
	}
	for i := len(f.charmaps) - 1; i >= 0; i-- {
		if f.charmaps[i].encoding == encodingUnicode {
			f.charmap = f.charmaps[i]
			return nil
		}
	}
	return errNoCharmap
}

// CharIndex is FT_Get_Char_Index.
func (f *Face) CharIndex(charcode uint32) uint32 {
	if f.charmap == nil {
		return 0
	}
	var result uint32
	if f.charmap.synthetic != nil {
		result = f.charmap.synthetic.charIndex(charcode)
		if int64(result) >= f.numGlyphs {
			result = 0
		}
		return result
	}
	switch f.charmap.format {
	case 0:
		if charcode < 256 {
			result = uint32(byteAt(f.charmap.data, 6+int(charcode)))
		}
	case 4:
		result = f.cmap4CharIndex(f.charmap, charcode)
	case 6:
		p := 6
		start := uint32(peek16(f.charmap.data, p))
		count := uint32(peek16(f.charmap.data, p+2))
		idx := charcode - start
		if idx < count {
			result = uint32(peek16(f.charmap.data, p+4+2*int(idx)))
		}
	case 2:
		result = cmap2CharIndex(f.charmap.data, charcode)
	case 8:
		result = cmap8CharIndex(f.charmap.data, charcode)
	case 10:
		result = cmap10CharIndex(f.charmap.data, charcode)
	case 12, 13:
		result = cmap12CharIndex(f.charmap, charcode)
	}
	if int64(result) >= f.numGlyphs {
		result = 0
	}
	return result
}

func byteAt(b []byte, p int) byte {
	if p < 0 || p >= len(b) {
		return 0
	}
	return b[p]
}

func validate0(table []byte) error {
	if len(table) < 4 {
		return errInvalidCmap
	}
	length := int(peek16(table, 2))
	if length > len(table) || length < 262 {
		return errInvalidCmap
	}
	return nil
}

func validate6(table []byte) error {
	if len(table) < 10 {
		return errInvalidCmap
	}
	length := int(peek16(table, 2))
	count := int(peek16(table, 8))
	if length > len(table) || length < 10+count*2 {
		return errInvalidCmap
	}
	return nil
}

// validate4 is tt_cmap4_validate at FT_VALIDATE_DEFAULT; it returns
// the charmap's flags.
func validate4(table []byte) (int, error) {
	flags := 0
	if len(table) < 4 {
		return 0, errInvalidCmap
	}
	// At the default level, a length out of bounds or too small is
	// taken to be the rest of the cmap table.
	length := len(table)
	if length < 16 {
		return 0, errInvalidCmap
	}
	numSegs := int(peek16(table, 6)) / 2
	if length < 16+numSegs*2*4 {
		return 0, errInvalidCmap
	}
	ends := 14
	starts := 16 + numSegs*2
	deltas := starts + numSegs*2
	offsets := deltas + numSegs*2
	glyphIDs := offsets + numSegs*2

	var lastStart, lastEnd int
	for n := 0; n < numSegs; n++ {
		start := int(peek16(table, starts+2*n))
		end := int(peek16(table, ends+2*n))
		offset := int(peek16(table, offsets+2*n))
		if start > end {
			return 0, errInvalidCmap
		}
		if start <= lastEnd && n > 0 {
			if lastStart > start || lastEnd > end {
				flags |= cmapFlagUnsorted
			} else {
				flags |= cmapFlagOverlapping
			}
		}
		if offset != 0 && offset != 0xFFFF {
			p := offsets + 2*n + offset
			if n != numSegs-1 || !(start == 0xFFFF && end == 0xFFFF) {
				if p < glyphIDs || p+(end-start+1)*2 > len(table) {
					return 0, errInvalidCmap
				}
			}
		} else if offset == 0xFFFF {
			if n != numSegs-1 || !(start == 0xFFFF && end == 0xFFFF) {
				return 0, errInvalidCmap
			}
		}
		lastStart, lastEnd = start, end
	}
	return flags, nil
}

func validate12(table []byte) error {
	if len(table) < 16 {
		return errInvalidCmap
	}
	length := uint64(peek32(table, 4))
	numGroups := uint64(peek32(table, 12))
	if length > uint64(len(table)) || length < 16 || (length-16)/12 < numGroups {
		return errInvalidCmap
	}
	var last uint32
	for n := uint64(0); n < numGroups; n++ {
		p := 16 + 12*int(n)
		start := peek32(table, p)
		end := peek32(table, p+4)
		if start > end {
			return errInvalidCmap
		}
		if n > 0 && start <= last {
			return errInvalidCmap
		}
		last = end
	}
	return nil
}

func peek24(b []byte, p int) uint32 {
	if p < 0 || p+3 > len(b) {
		return 0
	}
	return uint32(b[p])<<16 | uint32(b[p+1])<<8 | uint32(b[p+2])
}

func validate14(table []byte) error {
	if len(table) < 10 {
		return errInvalidCmap
	}
	length := uint64(peek32(table, 2))
	numSelectors := uint64(peek32(table, 6))
	limit := uint64(len(table))
	if length > limit || length < 10 || (length-10)/11 < numSelectors {
		return errInvalidCmap
	}
	p := 10
	lastVarSel := uint64(1)
	for n := uint64(0); n < numSelectors; n++ {
		varSel := uint64(peek24(table, p))
		defOff := uint64(peek32(table, p+3))
		nondefOff := uint64(peek32(table, p+7))
		p += 11
		if defOff >= length || nondefOff >= length {
			return errInvalidCmap
		}
		if varSel < lastVarSel {
			return errInvalidCmap
		}
		lastVarSel = varSel + 1
		if defOff != 0 {
			defp := defOff
			if defp+4 > limit {
				return errInvalidCmap
			}
			numRanges := uint64(peek32(table, int(defp)))
			defp += 4
			if numRanges > (limit-defp)/4 {
				return errInvalidCmap
			}
			lastBase := uint64(0)
			for i := uint64(0); i < numRanges; i++ {
				base := uint64(peek24(table, int(defp)))
				cnt := uint64(byteAt(table, int(defp)+3))
				defp += 4
				if base+cnt >= 0x110000 || base < lastBase {
					return errInvalidCmap
				}
				lastBase = base + cnt + 1
			}
		}
		if nondefOff != 0 {
			ndp := nondefOff
			if ndp+4 > limit {
				return errInvalidCmap
			}
			numMappings := uint64(peek32(table, int(ndp)))
			ndp += 4
			if numMappings > (limit-ndp)/5 {
				return errInvalidCmap
			}
			lastUni := uint64(0)
			for i := uint64(0); i < numMappings; i++ {
				uni := uint64(peek24(table, int(ndp)))
				ndp += 5
				if uni >= 0x110000 || uni < lastUni {
					return errInvalidCmap
				}
				lastUni = uni + 1
			}
		}
	}
	return nil
}

// cmap4CharIndex is tt_cmap4_char_index.
func (f *Face) cmap4CharIndex(cm *charmap, charcode uint32) uint32 {
	if charcode >= 0x10000 {
		return 0
	}
	if cm.flags&cmapFlagUnsorted != 0 {
		return f.cmap4Linear(cm, charcode)
	}
	return f.cmap4Binary(cm, charcode)
}

// cmap4Glyph maps charcode through a segment (the common tail of
// tt_cmap4_char_map_linear and _binary, without `next').
func (f *Face) cmap4Glyph(cm *charmap, charcode uint32, start, offset int, delta int, p int) uint32 {
	if offset != 0 {
		p += offset + int(charcode-uint32(start))*2
		gindex := uint32(peek16(cm.data, p))
		if gindex != 0 {
			gindex = uint32(int(gindex)+delta) & 0xFFFF
			if int64(gindex) >= f.numGlyphs {
				gindex = 0
			}
		}
		return gindex
	}
	return uint32(int(charcode)+delta) & 0xFFFF
}

func (f *Face) cmap4Linear(cm *charmap, charcode uint32) uint32 {
	data := cm.data
	numSegs := int(peek16(data, 6)) >> 1
	if numSegs == 0 {
		return 0
	}
	numSegs2 := numSegs << 1
	p := 14
	q := 16 + numSegs2
	for i := 0; i < numSegs; i++ {
		end := int(peek16(data, p))
		p += 2
		start := int(peek16(data, q))
		q += 2
		if charcode < uint32(start) {
			break
		}
		if charcode <= uint32(end) {
			r := q - 2 + numSegs2
			delta := int(int16(peek16(data, r)))
			r += numSegs2
			offset := int(peek16(data, r))
			if i >= numSegs-1 && start == 0xFFFF && end == 0xFFFF {
				if offset != 0 && r+offset+2 > len(data) {
					delta = 1
					offset = 0
				}
			}
			if offset == 0xFFFF {
				continue
			}
			return f.cmap4Glyph(cm, charcode, start, offset, delta, r)
		}
	}
	return 0
}

func (f *Face) cmap4Binary(cm *charmap, charcode uint32) uint32 {
	data := cm.data
	numSegs := int(peek16(data, 6)) >> 1
	if numSegs == 0 {
		return 0
	}
	numSegs2 := numSegs << 1
	lo, hi := 0, numSegs
	for {
		mid := (lo + hi) >> 1
		p := 14 + mid*2
		end := int(peek16(data, p))
		p += 2 + numSegs2
		start := int(peek16(data, p))

		if charcode < uint32(start) {
			hi = mid
		} else if charcode > uint32(end) {
			lo = mid + 1
		} else {
			p += numSegs2
			delta := int(int16(peek16(data, p)))
			p += numSegs2
			offset := int(peek16(data, p))

			if mid >= numSegs-1 && start == 0xFFFF && end == 0xFFFF {
				if offset != 0 && p+offset+2 > len(data) {
					delta = 1
					offset = 0
				}
			}

			if cm.flags&cmapFlagOverlapping != 0 {
				// Search the first segment containing charcode.
				max := mid
				if offset == 0xFFFF {
					mid = max + 1
				}
				i := max
				for ; i > 0; i-- {
					oldP := p
					p = 14 + (i-1)*2
					prevEnd := int(peek16(data, p))
					if charcode > uint32(prevEnd) {
						p = oldP
						break
					}
					end = prevEnd
					p += 2 + numSegs2
					start = int(peek16(data, p))
					p += numSegs2
					delta = int(int16(peek16(data, p)))
					p += numSegs2
					offset = int(peek16(data, p))
					if offset != 0xFFFF {
						mid = i - 1
					}
				}

				if mid == max+1 {
					if i != max {
						p = 14 + max*2
						end = int(peek16(data, p))
						p += 2 + numSegs2
						start = int(peek16(data, p))
						p += numSegs2
						delta = int(int16(peek16(data, p)))
						p += numSegs2
						offset = int(peek16(data, p))
					}
					mid = max
					for i = max + 1; i < numSegs; i++ {
						p = 14 + i*2
						nextEnd := int(peek16(data, p))
						p += 2 + numSegs2
						nextStart := int(peek16(data, p))
						if charcode < uint32(nextStart) {
							break
						}
						end = nextEnd
						start = nextStart
						p += numSegs2
						delta = int(int16(peek16(data, p)))
						p += numSegs2
						offset = int(peek16(data, p))
						if offset != 0xFFFF {
							mid = i
						}
					}
					i--
					if mid == max {
						// Still no luck.
						return 0
					}
				}

				if mid != i {
					p = 14 + mid*2
					end = int(peek16(data, p))
					p += 2 + numSegs2
					start = int(peek16(data, p))
					p += numSegs2
					delta = int(int16(peek16(data, p)))
					p += numSegs2
					offset = int(peek16(data, p))
				}
			} else if offset == 0xFFFF {
				return 0
			}
			_ = end
			return f.cmap4Glyph(cm, charcode, start, offset, delta, p)
		}
		if lo >= hi {
			return 0
		}
	}
}

// cmap12CharIndex is tt_cmap12_char_index and tt_cmap13_char_index.
func cmap12CharIndex(cm *charmap, charcode uint32) uint32 {
	data := cm.data
	numGroups := peek32(data, 12)
	if numGroups == 0 {
		return 0
	}
	lo, hi := uint32(0), numGroups
	for lo < hi {
		mid := (lo + hi) >> 1
		p := 16 + 12*int(mid)
		start := peek32(data, p)
		end := peek32(data, p+4)
		if charcode < start {
			hi = mid
		} else if charcode > end {
			lo = mid + 1
		} else {
			id := peek32(data, p+8)
			if cm.format == 13 {
				return id
			}
			if id > 0xFFFFFFFF-(charcode-start) {
				return 0
			}
			return id + (charcode - start)
		}
	}
	return 0
}

// validate2 is tt_cmap2_validate at FT_VALIDATE_DEFAULT.
func validate2(table []byte) error {
	if len(table) < 4 {
		return errInvalidCmap
	}
	length := int(peek16(table, 2))
	if length > len(table) || length < 6+512 {
		return errInvalidCmap
	}
	maxSubs := 0
	for n := 0; n < 256; n++ {
		if idx := int(peek16(table, 6+2*n)) >> 3; idx > maxSubs {
			maxSubs = idx
		}
	}
	glyphIDs := 518 + (maxSubs+1)*8
	if glyphIDs > len(table) {
		return errInvalidCmap
	}
	for n := 0; n <= maxSubs; n++ {
		p := 518 + 8*n
		codeCount := int(peek16(table, p+2))
		offset := int(peek16(table, p+6))
		if codeCount == 0 {
			continue
		}
		if offset != 0 {
			ids := p + 6 + offset
			if ids < glyphIDs || ids+codeCount*2 > length {
				return errInvalidCmap
			}
		}
	}
	return nil
}

// cmap2CharIndex is tt_cmap2_char_index, with tt_cmap2_get_subheader.
func cmap2CharIndex(table []byte, charcode uint32) uint32 {
	if charcode >= 0x10000 {
		return 0
	}
	charLo := int(charcode & 0xFF)
	charHi := int(charcode >> 8)
	const subs = 518
	var sub int
	if charHi == 0 {
		sub = subs
		if peek16(table, 6+charLo*2) != 0 {
			return 0
		}
	} else {
		sub = subs + int(peek16(table, 6+charHi*2))&^7
		if sub == subs {
			return 0
		}
	}
	start := uint32(peek16(table, sub))
	count := uint32(peek16(table, sub+2))
	delta := int32(int16(peek16(table, sub+4)))
	offset := int(peek16(table, sub+6))
	idx := uint32(charLo) - start
	if idx < count && offset != 0 {
		gid := peek16(table, sub+6+offset+2*int(idx))
		if gid != 0 {
			return uint32(int32(gid)+delta) & 0xFFFF
		}
	}
	return 0
}

// validate8 is tt_cmap8_validate at FT_VALIDATE_DEFAULT.
func validate8(table []byte) error {
	if len(table) < 16+8192 {
		return errInvalidCmap
	}
	length := uint64(peek32(table, 4))
	if length > uint64(len(table)) || length < 8192+16 {
		return errInvalidCmap
	}
	numGroups := uint64(peek32(table, 12+8192))
	if numGroups > uint64(len(table)-8208)/12 {
		return errInvalidCmap
	}
	var last uint32
	for n := uint64(0); n < numGroups; n++ {
		p := 8208 + 12*int(n)
		start := peek32(table, p)
		end := peek32(table, p+4)
		if start > end {
			return errInvalidCmap
		}
		if n > 0 && start <= last {
			return errInvalidCmap
		}
		last = end
	}
	return nil
}

// cmap8CharIndex is tt_cmap8_char_index.
func cmap8CharIndex(table []byte, charcode uint32) uint32 {
	numGroups := peek32(table, 8204)
	p := 8208
	for ; numGroups > 0; numGroups-- {
		start := peek32(table, p)
		end := peek32(table, p+4)
		startID := peek32(table, p+8)
		p += 12
		if charcode < start {
			break
		}
		if charcode <= end {
			if startID > 0xFFFFFFFF-(charcode-start) {
				return 0
			}
			return startID + (charcode - start)
		}
	}
	return 0
}

// validate10 is tt_cmap10_validate at FT_VALIDATE_DEFAULT.
func validate10(table []byte) error {
	if len(table) < 20 {
		return errInvalidCmap
	}
	length := uint64(peek32(table, 4))
	count := uint64(peek32(table, 16))
	if length > uint64(len(table)) || length < 20 || (length-20)/2 < count {
		return errInvalidCmap
	}
	return nil
}

// cmap10CharIndex is tt_cmap10_char_index.
func cmap10CharIndex(table []byte, charcode uint32) uint32 {
	start := peek32(table, 12)
	count := peek32(table, 16)
	if charcode < start {
		return 0
	}
	idx := charcode - start
	if idx < count {
		return uint32(peek16(table, 20+2*int(idx)))
	}
	return 0
}
