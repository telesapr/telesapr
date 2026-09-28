package ttf

// FreeType's CFF driver: the face (cffobjs.c's cff_face_init), the
// charstring decoder's set-up (psaux's cffdecode.c and psft.c) and
// glyph loading (cffgload.c's cff_slot_load).

// removeSubsetPrefix is remove_subset_prefix: it strips prefixes like
// `ABCDEF+'.
func removeSubsetPrefix(name string) string {
	b := []byte(name)
	at := func(i int) byte {
		if i < len(b) {
			return b[i]
		}
		return 0
	}
	i, idx := 0, 0
	for {
		ok := true
		for k := 0; k < 6; k++ {
			c := at(i)
			if !('A' <= c) {
				ok = false
				break
			}
			i++
			if !(c <= 'Z') {
				ok = false
				break
			}
		}
		if !ok {
			break
		}
		c := at(i)
		i++
		if c != '+' {
			break
		}
		idx = i
	}
	return name[idx:]
}

// removeStyle is remove_style: the style name's ending, and the
// separators before it, are removed from the family name.
func removeStyle(family, style string) string {
	f := len(family)
	s := len(style)
	for s > 0 {
		if f == 0 {
			return family
		}
		s--
		f--
		if style[s] != family[f] {
			return family
		}
	}
	// terminate and remove special characters
	for f > 0 {
		c := family[f-1]
		if c == '-' || c == ' ' || c == '_' || c == '+' {
			f--
			continue
		}
		break
	}
	return family[:f]
}

// fvarAxes is sfnt_init_face's fvar check: the number of axes of a valid
// fvar table, or 0.
func (f *Face) fvarAxes() uint16 {
	fvar, _, ok := f.table("fvar")
	if !ok || len(fvar) < 20 {
		return 0
	}
	r := &reader{b: fvar}
	version := r.u32()
	offset := uint32(r.u16())
	r.u16()
	numAxes := r.u16()
	axisSize := r.u16()
	numInstances := r.u16()
	instanceSize := r.u16()
	if r.err != nil {
		return 0
	}
	if version != 0x00010000 || axisSize != 20 || numAxes == 0 || numAxes > 0x3FFE ||
		!(uint32(instanceSize) == 4+4*uint32(numAxes) || uint32(instanceSize) == 6+4*uint32(numAxes)) ||
		numInstances > 0x7EFF ||
		offset+uint32(axisSize)*uint32(numAxes)+uint32(instanceSize)*uint32(numInstances) > uint32(len(fvar)) {
		return 0
	}
	return numAxes
}

// normalizedVector is cf2_getNormalizedVector: at the default instance,
// the variation blend's normalized coordinates are all zero, and there
// are none without a variation blend.
func (f *cffFont) normalizedVector() (uint32, []int64) {
	if f.numAxes == 0 {
		return 0, nil
	}
	return uint32(f.numAxes), make([]int64, f.numAxes)
}

// cffFaceInit is cff_face_init for an SFNT file (sfnt) or a pure CFF
// file; f has been through sfnt_init_face for an SFNT one.
func (f *Face) cffFaceInit(sfntFormat bool) error {
	pureCFF := true
	cff2 := false
	s := &cffStream{data: f.data}

	if sfntFormat {
		if _, _, ok := f.table("head"); ok {
			pureCFF = false
			if err := f.loadFace(true); err != nil {
				return err
			}
		} else {
			// sfnt->load_cmap
			if cmap, _, ok := f.table("cmap"); ok {
				f.cmapTable = cmap
			}
		}
		_, pos, ok := f.table("CFF2")
		if ok {
			cff2 = true
		} else if _, pos, ok = f.table("CFF "); !ok {
			return errTableMissing
		}
		if err := s.seek(pos); err != nil {
			return err
		}
	}

	cff, err := loadCFFFont(s, 0, pureCFF, cff2)
	if err != nil {
		return err
	}
	f.cff = cff
	f.isCFF2 = cff2
	cff.face = f
	f.numGlyphs = int64(cff.numGlyphs)
	dict := &cff.topFont.fontDict

	// FT_Set_Named_Instance( face, 0 ) for a variation font sets up a
	// blend at the default instance.
	if sfntFormat {
		cff.numAxes = f.fvarAxes()
	}

	if !dict.hasFontMatrix {
		if pureCFF {
			dict.unitsPerEm = 1000
		} else {
			dict.unitsPerEm = uint64(f.unitsPerEM)
		}
	}

	normalize := func(d *cffFontDict) {
		matrix := &d.fontMatrix
		offset := &d.fontOffset
		temp := abs64s(matrix.YY)
		if matrix.YY == 0 {
			temp = abs64s(matrix.YX)
		}
		if temp != 0x10000 {
			d.unitsPerEm = uint64(divFix(int64(d.unitsPerEm), temp))
			matrix.XX = divFix(matrix.XX, temp)
			matrix.YX = divFix(matrix.YX, temp)
			matrix.XY = divFix(matrix.XY, temp)
			matrix.YY = divFix(matrix.YY, temp)
			offset.X = divFix(offset.X, temp)
			offset.Y = divFix(offset.Y, temp)
		}
		offset.X >>= 16
		offset.Y >>= 16
	}
	normalize(dict)

	for i := cff.numSubfonts; i > 0; i-- {
		sub := &cff.subfonts[i-1].fontDict
		top := &cff.topFont.fontDict
		if sub.hasFontMatrix {
			if top.hasFontMatrix {
				var scaling int64 = 1
				if top.unitsPerEm > 1 && sub.unitsPerEm > 1 {
					scaling = int64(top.unitsPerEm)
					if int64(sub.unitsPerEm) < scaling {
						scaling = int64(sub.unitsPerEm)
					}
				}
				matrixMultiplyScaled(&top.fontMatrix, &sub.fontMatrix, scaling)
				vectorTransformScaled(&sub.fontOffset, &top.fontMatrix, scaling)
				sub.unitsPerEm = uint64(mulDiv(int64(sub.unitsPerEm), int64(top.unitsPerEm), scaling))
			}
		} else {
			sub.fontMatrix = top.fontMatrix
			sub.fontOffset = top.fontOffset
			sub.unitsPerEm = top.unitsPerEm
		}
		normalize(sub)
	}

	if pureCFF {
		var styleName string
		hasStyle := false
		if dict.cidRegistry != 0xFFFF {
			f.numGlyphs = int64(cff.charset.maxCID) + 1
		} else {
			f.numGlyphs = int64(cff.charstringsIndex.count)
		}
		bboxYMin := dict.fontBBox.YMin >> 16
		bboxYMax := (dict.fontBBox.YMax + 0xFFFF) >> 16
		f.unitsPerEM = int64(uint16(dict.unitsPerEm))
		f.ascender = int64(int16(bboxYMax))
		f.descender = int64(int16(bboxYMin))

		var family string
		hasFamily := false
		if dict.familyName != 0 {
			family, hasFamily = cff.getSIDString(dict.familyName)
		}
		if !hasFamily {
			family, hasFamily = cff.getName(0)
			if hasFamily {
				family = removeSubsetPrefix(family)
			}
		}
		if hasFamily {
			full, hasFull := cff.getSIDString(dict.fullName)
			if hasFull {
				fullp, fam := 0, 0
				at := func(s string, i int) byte {
					if i < len(s) {
						return s[i]
					}
					return 0
				}
				for at(full, fullp) != 0 {
					if at(full, fullp) == at(family, fam) {
						fam++
						fullp++
						continue
					}
					if at(full, fullp) == ' ' || at(full, fullp) == '-' {
						fullp++
						continue
					}
					if at(family, fam) == ' ' || at(family, fam) == '-' {
						fam++
						continue
					}
					if at(family, fam) == 0 && at(full, fullp) != 0 {
						styleName = full[fullp:]
						hasStyle = true
						family = removeStyle(family, styleName)
					}
					break
				}
			}
		} else {
			if name, ok := cff.getSIDString(dict.cidFontName); ok {
				family, hasFamily = name, true
			}
		}
		if !hasStyle {
			styleName = "Regular"
		}
		f.FamilyName, f.StyleName = family, styleName
	}

	// Synthesize a Unicode charmap if there is none.
	for _, cm := range f.charmaps {
		if (cm.platformID == platformMicrosoft && cm.encodingID == msIDUnicode) ||
			cm.platformID == platformAppleUnicode {
			return nil
		}
	}
	if pureCFF && dict.cidRegistry != 0xFFFF {
		return nil
	}
	// cff_cmap_unicode_init
	if cff.charset.sids == nil {
		return nil // No_Unicode_Glyph_Name
	}
	u, err := unicodesInit(cff.numGlyphs, func(n uint32) (string, bool) {
		return cff.getSIDString(uint32(cff.charset.sids[n]))
	})
	if err != nil {
		return nil
	}
	f.charmaps = append(f.charmaps, &charmap{
		platformID: platformMicrosoft,
		encodingID: msIDUnicode,
		encoding:   encodingUnicode,
		synthetic:  u,
	})
	return nil
}

// matrixMultiplyScaled is FT_Matrix_Multiply_Scaled.
func matrixMultiplyScaled(a, b *Matrix, scaling int64) {
	val := 0x10000 * scaling
	xx := mulDiv(a.XX, b.XX, val) + mulDiv(a.XY, b.YX, val)
	xy := mulDiv(a.XX, b.XY, val) + mulDiv(a.XY, b.YY, val)
	yx := mulDiv(a.YX, b.XX, val) + mulDiv(a.YY, b.YX, val)
	yy := mulDiv(a.YX, b.XY, val) + mulDiv(a.YY, b.YY, val)
	b.XX, b.XY, b.YX, b.YY = xx, xy, yx, yy
}

// vectorTransformScaled is FT_Vector_Transform_Scaled.
func vectorTransformScaled(v *Vector, m *Matrix, scaling int64) {
	val := 0x10000 * scaling
	xz := mulDiv(v.X, m.XX, val) + mulDiv(v.Y, m.XY, val)
	yz := mulDiv(v.X, m.YX, val) + mulDiv(v.Y, m.YY, val)
	v.X, v.Y = xz, yz
}

// cffDecoder is the part of a CFF_Decoder (and the PS_Decoder made from
// it) that the cf2 engine reads.
type cffDecoder struct {
	cff            *cffFont
	currentSubfont *cffSubFont
	globalsBias    int32
	localsBias     int32
	numGlobals     uint32
	numLocals      uint32
	yPpem          int64
}

// computeBias is cff_compute_bias.
func computeBias(charstringType int32, numSubrs uint32) int32 {
	switch {
	case charstringType == 1:
		return 0
	case numSubrs < 1240:
		return 107
	case numSubrs < 33900:
		return 1131
	}
	return 32768
}

func (d *cffDecoder) nominalWidthX() cf2Fixed {
	return cf2IntToFixed(int32(d.currentSubfont.privateDict.nominalWidth))
}

func (d *cffDecoder) defaultWidthX() cf2Fixed {
	return cf2IntToFixed(int32(d.currentSubfont.privateDict.defaultWidth))
}

// initGlobalRegionBuffer is cf2_initGlobalRegionBuffer: it reports an
// error.
func (d *cffDecoder) initGlobalRegionBuffer(subrNum int32, buf *cf2Buffer) bool {
	*buf = cf2Buffer{}
	idx := uint32(subrNum + d.globalsBias)
	if idx >= d.numGlobals {
		return true
	}
	cff := d.cff
	bytes := cff.globalSubrsIndex.bytes
	*buf = cf2Buffer{data: bytes[cff.globalSubrs[idx]:cff.globalSubrs[idx+1]]}
	return false
}

// initLocalRegionBuffer is cf2_initLocalRegionBuffer.
func (d *cffDecoder) initLocalRegionBuffer(subrNum int32, buf *cf2Buffer) bool {
	*buf = cf2Buffer{}
	idx := uint32(subrNum + d.localsBias)
	if idx >= d.numLocals {
		return true
	}
	sub := d.currentSubfont
	bytes := sub.localSubrsIndex.bytes
	*buf = cf2Buffer{data: bytes[sub.localSubrs[idx]:sub.localSubrs[idx+1]]}
	return false
}

// lookupGlyphByStdCharcode is cff_lookup_glyph_by_stdcharcode.
func (cff *cffFont) lookupGlyphByStdCharcode(charcode int32) int32 {
	if cff.charset.sids == nil {
		return -1
	}
	if charcode < 0 || charcode > 255 {
		return -1
	}
	glyphSID := cffStandardEncoding[charcode]
	for n := uint32(0); n < cff.numGlyphs; n++ {
		if cff.charset.sids[n] == glyphSID {
			return int32(n)
		}
	}
	return -1
}

// getSeacComponent is cf2_getSeacComponent.
func (d *cffDecoder) getSeacComponent(code int32) (*cf2Buffer, error) {
	gid := d.cff.lookupGlyphByStdCharcode(code)
	if gid < 0 {
		return nil, errInvalidGlyphFmt
	}
	data, err := d.cff.charstringsIndex.accessElement(d.cff.stream, uint32(gid))
	if err != nil {
		return nil, err
	}
	return &cf2Buffer{data: data}, nil
}

// loadCFFGlyph is cff_slot_load (with FT_LOAD_NO_BITMAP |
// FT_LOAD_NO_HINTING, scaled), and FT_Load_Glyph's advance.
func (f *Face) loadCFFGlyph(glyphIndex uint32) (*Glyph, error) {
	cff := f.cff
	if cff.topFont.fontDict.cidRegistry != 0xFFFF && cff.charset.cids != nil {
		if glyphIndex != 0 {
			glyphIndex = cff.charset.cidToGindex(glyphIndex)
			if glyphIndex == 0 {
				return nil, errInvalidArg
			}
		}
	} else if glyphIndex >= cff.numGlyphs {
		return nil, errInvalidArg
	}

	// FT_Load_Glyph loads unscaled glyphs for a zero ppem.
	scaled := f.xPpem != 0 && f.yPpem != 0
	xScale, yScale := int64(0x10000), int64(0x10000)
	if scaled {
		xScale, yScale = f.xScale, f.yScale
	}
	forceScaling := false
	var fontMatrix Matrix
	var fontOffset Vector
	if cff.numSubfonts != 0 {
		fdIndex := uint32(cff.fdSelect.get(glyphIndex))
		if fdIndex >= cff.numSubfonts {
			fdIndex = cff.numSubfonts - 1
		}
		topUpm := int64(cff.topFont.fontDict.unitsPerEm)
		subUpm := int64(cff.subfonts[fdIndex].fontDict.unitsPerEm)
		fontMatrix = cff.subfonts[fdIndex].fontDict.fontMatrix
		fontOffset = cff.subfonts[fdIndex].fontDict.fontOffset
		if topUpm != subUpm {
			xScale = mulDiv(xScale, topUpm, subUpm)
			yScale = mulDiv(yScale, topUpm, subUpm)
			forceScaling = true
		}
	} else {
		fontMatrix = cff.topFont.fontDict.fontMatrix
		fontOffset = cff.topFont.fontDict.fontOffset
	}

	// cff_decoder_init and cff_decoder_prepare
	d := &cffDecoder{cff: cff, yPpem: f.yPpem}
	d.numGlobals = cff.globalSubrsIndex.count
	d.globalsBias = computeBias(cff.topFont.fontDict.charstringType, d.numGlobals)
	sub := &cff.topFont
	if cff.numSubfonts != 0 {
		fdIndex := uint32(cff.fdSelect.get(glyphIndex))
		if fdIndex >= cff.numSubfonts {
			return nil, errInvalidFormat
		}
		sub = cff.subfonts[fdIndex]
	}
	if sub.localSubrsIndex != nil {
		d.numLocals = sub.localSubrsIndex.count
	}
	d.localsBias = computeBias(cff.topFont.fontDict.charstringType, d.numLocals)
	d.currentSubfont = sub

	charstring, err := cff.charstringsIndex.accessElement(cff.stream, glyphIndex)
	if err != nil {
		return nil, err
	}

	// cf2_decoder_parse_charstrings
	font := cff.cf2
	if font == nil {
		font = &cf2Font{cff: cff}
		font.outline.err = &font.err
		cff.cf2 = font
	}
	font.decoder = d
	transform := cf2Matrix{a: 0x0400, d: 0x0400}
	font.isCFF2 = f.isCFF2
	font.renderingFlags = 0
	font.darkenParams = cf2Darkening
	font.unitsPerEm = int32(uint16(f.unitsPerEM))
	if scaled {
		// cf2_checkTransform
		if font.unitsPerEm > 0x7FFF {
			return nil, errCf2TooBig
		}
		maxScale := cf2DivFix(cf2IntToFixed(2000), cf2IntToFixed(font.unitsPerEm))
		if transform.a > maxScale || transform.d > maxScale {
			return nil, errCf2TooBig
		}
	}
	glyphWidth, err := font.getGlyphOutline(&cf2Buffer{data: charstring}, &transform)
	if err != nil {
		return nil, errInvalidFormat
	}
	glyphWidthInt := int64(cf2FixedToInt(glyphWidth))

	g := &Glyph{Outline: font.outline.outline}
	font.outline.outline = Outline{}

	var horiAdvance int64
	if f.numberOfHMetrics != 0 {
		_, aw := f.hMetrics(glyphIndex)
		horiAdvance = int64(aw)
	} else {
		horiAdvance = glyphWidthInt
	}

	if fontMatrix.XX != 0x10000 || fontMatrix.YY != 0x10000 || fontMatrix.XY != 0 || fontMatrix.YX != 0 {
		for i := range g.Outline.Points {
			g.Outline.Points[i] = transform2(g.Outline.Points[i], &fontMatrix)
		}
		horiAdvance = mulFix(horiAdvance, fontMatrix.XX)
	}
	if fontOffset.X != 0 || fontOffset.Y != 0 {
		g.Outline.Translate(fontOffset.X, fontOffset.Y)
		horiAdvance += fontOffset.X
	}

	if scaled || forceScaling {
		for i := range g.Outline.Points {
			g.Outline.Points[i].X = mulFix(g.Outline.Points[i].X, xScale)
			g.Outline.Points[i].Y = mulFix(g.Outline.Points[i].Y, yScale)
		}
		horiAdvance = mulFix(horiAdvance, xScale)
	}

	cbox := g.Outline.CBox()
	g.HoriBearingX = cbox.XMin
	g.AdvanceX = horiAdvance

	if err := g.Outline.check(); err != nil {
		return nil, err
	}
	return g, nil
}

// transform2 is FT_Vector_Transform.
func transform2(v Vector, m *Matrix) Vector {
	return Vector{
		mulFix(v.X, m.XX) + mulFix(v.Y, m.XY),
		mulFix(v.X, m.YX) + mulFix(v.Y, m.YY),
	}
}
