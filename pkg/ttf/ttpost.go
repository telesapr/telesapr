package ttf

// The PostScript glyph names of an SFNT font's `post' table (ttload.c's
// tt_face_load_post and ttpost.c), which sfnt_load_face uses to
// synthesize a Unicode charmap for a font without one.

// loadPost is tt_face_load_post: it reports an error for a missing or
// short table, or an unknown format.
func (f *Face) loadPost() bool {
	_, pos, ok := f.table("post")
	if !ok {
		return false
	}
	s := &cffStream{data: f.data, pos: pos}
	hdr, err := s.frame(32)
	if err != nil {
		return false
	}
	f.postFormat = int64(int32(peek32(hdr, 0)))
	f.isFixedPitch = peek32(hdr, 12)
	switch f.postFormat {
	case 0x00030000, 0x00025000, 0x00020000, 0x00010000:
		return true
	}
	return false
}

// postNames is a TT_Post_NamesRec.
type postNames struct {
	loaded       bool
	numGlyphs    int
	glyphIndices []uint16
	pool         []byte // the name strings, as C strings
	names        []int  // each name's start in pool
}

// loadPostNames is load_post_names.
func (f *Face) loadPostNames() {
	names := &f.postNames
	defer func() { names.loaded = true }()
	postTable, pos, ok := f.table("post")
	if !ok {
		return
	}
	postLen := uint64(len(postTable))
	s := &cffStream{data: f.data, pos: pos}
	if postLen < 34 || s.skip(32) != nil {
		return
	}
	n, err := s.readU16()
	if err != nil || int64(n) > f.maxpNumGlyphs || n == 0 {
		return
	}
	numGlyphs := int(n)
	switch f.postFormat {
	case 0x00020000:
		f.loadFormat20(s, numGlyphs, postLen-34)
	case 0x00025000:
		f.loadFormat25(s, numGlyphs, postLen-34)
	}
}

// loadFormat20 is load_format_20.
func (f *Face) loadFormat20(s *cffStream, numGlyphs int, postLen uint64) {
	if uint64(numGlyphs)*2 > postLen {
		return
	}
	q, err := s.frame(uint64(numGlyphs) * 2)
	if err != nil {
		return
	}
	indices := make([]uint16, numGlyphs)
	numNames := uint16(0)
	for n := range indices {
		idx := peek16(q, 2*n)
		if idx > numNames {
			numNames = idx
		}
		indices[n] = idx
	}
	if numNames > 257 {
		numNames -= 257
	} else {
		numNames = 0
	}

	var pool []byte
	var nameStarts []int
	if numNames != 0 {
		postLen -= uint64(numGlyphs) * 2
		p, err := s.read(int64(postLen))
		if err != nil {
			return
		}
		pool = make([]byte, postLen+1)
		copy(pool, p)
		pEnd := int(postLen)
		i := 0
		n := 0
		for ; i < pEnd && n < int(numNames); n++ {
			length := int(pool[i])
			pool[i] = 0
			i++
			nameStarts = append(nameStarts, i)
			i += length
		}
		pool[pEnd] = 0
		for ; n < int(numNames); n++ {
			nameStarts = append(nameStarts, pEnd)
		}
	}
	names := &f.postNames
	names.numGlyphs = numGlyphs
	names.glyphIndices = indices
	names.pool = pool
	names.names = nameStarts
}

// loadFormat25 is load_format_25.
func (f *Face) loadFormat25(s *cffStream, numGlyphs int, postLen uint64) {
	if uint64(numGlyphs) > postLen || numGlyphs > 258+128 {
		return
	}
	q, err := s.frame(uint64(numGlyphs))
	if err != nil {
		return
	}
	indices := make([]uint16, numGlyphs)
	for n := range indices {
		idx := n + int(int8(q[n]))
		if idx < 0 || idx > 257 {
			idx = 0
		}
		indices[n] = uint16(idx)
	}
	f.postNames.numGlyphs = numGlyphs
	f.postNames.glyphIndices = indices
}

// psName is tt_face_get_ps_name (through tt_get_glyph_name): ok is false
// for an invalid glyph index.
func (f *Face) psName(idx uint32) (string, bool) {
	if int64(idx) >= f.maxpNumGlyphs {
		return "", false
	}
	name := macintoshName(0)
	switch {
	case f.postFormat == 0x00020000 || f.postFormat == 0x00025000:
		names := &f.postNames
		if !names.loaded {
			f.loadPostNames()
		}
		if int(idx) < names.numGlyphs {
			nameIndex := names.glyphIndices[idx]
			if nameIndex < 258 {
				name = macintoshName(uint(nameIndex))
			} else {
				// a name past the table can only come from a bad font
				i := int(nameIndex) - 258
				if i < len(names.names) {
					name = cString(string(names.pool), names.names[i])
				} else {
					name = ""
				}
			}
		}
	case f.postFormat == 0x00010000 && f.maxpNumGlyphs == 258:
		name = macintoshName(uint(idx))
	}
	return name, true
}

// synthesizeUnicodeCmap is sfnt_load_face's Unicode charmap from glyph
// names, for a font whose charmaps have neither a Unicode nor a symbol
// encoding.
func (f *Face) synthesizeUnicodeCmap(glyphNames bool) {
	for _, cm := range f.charmaps {
		if cm.encoding == encodingUnicode || cm.encoding == encodingMSSymbol {
			return
		}
	}
	if !glyphNames {
		return
	}
	u, err := unicodesInit(uint32(f.maxpNumGlyphs), f.psName)
	if err != nil {
		return
	}
	f.charmaps = append(f.charmaps, &charmap{
		platformID: platformMicrosoft,
		encodingID: msIDUnicode,
		encoding:   encodingUnicode,
		synthetic:  u,
	})
}
