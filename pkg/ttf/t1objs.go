package ttf

import "errors"

// FreeType's Type 1 driver faces and glyphs (type1's t1objs.c and
// t1gload.c, and psaux's t1_make_subfont), with the charstrings run by
// the cf2 engine in its Type 1 mode.

// t1FaceInit is T1_Face_Init: a Type 1 font (PFA or PFB) as a face.
// It reports errUnknownFormat for other files.
func t1FaceInit(data []byte) (*Face, error) {
	t1, err := t1OpenFace(data)
	if err != nil {
		return nil, err
	}
	f := &Face{data: data, t1: t1}
	f.numGlyphs = int64(t1.numGlyphs)
	info := &t1.fontInfo

	family := info.familyName
	var style *string
	if family != nil {
		if info.fullName != nil {
			full, fam := *info.fullName, *family
			famAt := func(j int) byte {
				if j < len(fam) {
					return fam[j]
				}
				return 0
			}
			theSame := true
			i, j := 0, 0
			for i < len(full) {
				switch {
				case full[i] == famAt(j):
					i++
					j++
				case full[i] == ' ' || full[i] == '-':
					i++
				case famAt(j) == ' ' || famAt(j) == '-':
					j++
				default:
					theSame = false
					if famAt(j) == 0 {
						s := full[i:]
						style = &s
					}
				}
				if !theSame {
					break
				}
			}
			if theSame {
				s := "Regular"
				style = &s
			}
		}
	} else if t1.fontName != nil {
		family = t1.fontName
	}
	if style == nil {
		if info.weight != nil {
			style = info.weight
		} else {
			s := "Regular"
			style = &s
		}
	}
	if family != nil {
		f.FamilyName = *family
	}
	f.StyleName = *style

	yMin := t1.fontBBox.yMin >> 16
	yMax := (t1.fontBBox.yMax + 0xFFFF) >> 16
	upm := t1.unitsPerEM
	if upm == 0 {
		upm = 1000
	}
	f.unitsPerEM = int64(upm)
	f.ascender = int64(int16(yMax))
	f.descender = int64(int16(yMin))

	// the Unicode charmap from the glyph names
	glyphName := func(idx uint32) (string, bool) {
		if name := t1.glyphNames[idx]; name != nil {
			return *name, true
		}
		return "", false
	}
	u, uerr := unicodesInit(uint32(t1.numGlyphs), glyphName)
	if uerr == nil {
		f.charmaps = append(f.charmaps, &charmap{
			platformID: platformMicrosoft,
			encodingID: msIDUnicode,
			encoding:   encodingUnicode,
			synthetic:  u,
		})
	} else if !errors.Is(uerr, errNoUnicodeGlyphName) {
		return nil, uerr
	}
	if t1.encodingType == t1EncodingISOLatin1 && uerr != nil {
		// its charmap has the Unicode class, whose init fails the same
		return nil, uerr
	}
	return f, nil
}

// t1MakeSubfont is t1_make_subfont: the private dictionary as a CFF
// subfont's, for the cf2 engine.
func t1MakeSubfont(priv *psPrivate) *cffSubFont {
	sub := &cffSubFont{}
	c := &sub.privateDict
	c.numBlueValues = priv.numBlueValues
	for n := range c.numBlueValues {
		c.blueValues[n] = int64(cf2IntToFixed(int32(priv.blueValues[n])))
	}
	c.numOtherBlues = priv.numOtherBlues
	for n := range c.numOtherBlues {
		c.otherBlues[n] = int64(cf2IntToFixed(int32(priv.otherBlues[n])))
	}
	c.numFamilyBlues = priv.numFamilyBlues
	for n := range c.numFamilyBlues {
		c.familyBlues[n] = int64(cf2IntToFixed(int32(priv.familyBlues[n])))
	}
	c.numFamilyOtherBlues = priv.numFamilyOtherBlues
	for n := range c.numFamilyOtherBlues {
		c.familyOtherBlues[n] = int64(cf2IntToFixed(int32(priv.familyOtherBlues[n])))
	}
	c.blueScale = priv.blueScale
	c.blueShift = int64(priv.blueShift)
	c.blueFuzz = int64(priv.blueFuzz)
	c.standardWidth = int64(priv.standardWidth[0])
	c.standardHeight = int64(priv.standardHeight[0])
	c.numSnapWidths = priv.numSnapWidths
	for n := range c.numSnapWidths {
		c.snapWidths[n] = int64(priv.snapWidths[n])
	}
	c.numSnapHeights = priv.numSnapHeights
	for n := range c.numSnapHeights {
		c.snapHeights[n] = int64(priv.snapHeights[n])
	}
	c.forceBold = priv.forceBold
	c.lenIV = priv.lenIV
	c.languageGroup = int32(priv.languageGroup)
	c.expansionFactor = priv.expansionFactor
	c.subfont = sub
	// the random seed comes from memory addresses (the face's is -1)
	sub.random = uint32(driverRandomSeed)
	return sub
}

// fixedToInt is FIXED_TO_INT.
func fixedToInt(x int64) int64 { return roundFix(x) >> 16 }

// loadT1Glyph is T1_Load_Glyph with FT_LOAD_NO_BITMAP |
// FT_LOAD_NO_HINTING.
func (f *Face) loadT1Glyph(glyphIndex uint32) (*Glyph, error) {
	t1 := f.t1
	if int64(glyphIndex) >= f.numGlyphs {
		return nil, errInvalidArg
	}
	// FT_Load_Glyph loads unscaled glyphs for a zero ppem.
	scaled := f.xPpem != 0 && f.yPpem != 0

	// t1_decoder_init and ps_decoder_init
	d := &cffDecoder{
		isT1:       true,
		t1:         t1,
		yPpem:      f.yPpem,
		numLocals:  uint32(t1.numSubrs),
		locals:     t1.subrs,
		localsHash: t1.subrsHash,
		blend:      t1.blend,
		buildchar:  t1.buildchar,
	}
	d.currentSubfont = t1MakeSubfont(&t1.privateDict)

	// cf2_decoder_parse_charstrings, with a new cf2 instance
	font := &cf2Font{isT1: true}
	font.outline.err = &font.err
	font.decoder = d
	transform := cf2Matrix{a: 0x0400, d: 0x0400}
	font.darkenParams = cf2Darkening
	font.unitsPerEm = int32(uint16(f.unitsPerEM))
	if scaled {
		// cf2_checkTransform; the retry without hinting fails the same
		if font.unitsPerEm > 0x7FFF {
			return nil, errCf2TooBig
		}
		maxScale := cf2DivFix(cf2IntToFixed(2000), cf2IntToFixed(font.unitsPerEm))
		if transform.a > maxScale || transform.d > maxScale {
			return nil, errCf2TooBig
		}
	}
	if _, err := font.getGlyphOutline(&cf2Buffer{data: t1.charstrings[glyphIndex]},
		&transform); err != nil {
		return nil, errInvalidFileFormat
	}
	g := &Glyph{Outline: font.outline.outline}

	horiAdvance := fixedToInt(d.advance.X)
	m := t1.fontMatrix
	if m.XX != 0x10000 || m.YY != 0x10000 || m.XY != 0 || m.YX != 0 {
		for i := range g.Outline.Points {
			g.Outline.Points[i] = transform2(g.Outline.Points[i], &m)
		}
		horiAdvance = mulFix(horiAdvance, m.XX)
	}
	if off := t1.fontOffset; off.X != 0 || off.Y != 0 {
		g.Outline.Translate(off.X, off.Y)
		horiAdvance += off.X
	}
	if scaled {
		for i := range g.Outline.Points {
			g.Outline.Points[i].X = mulFix(g.Outline.Points[i].X, f.xScale)
			g.Outline.Points[i].Y = mulFix(g.Outline.Points[i].Y, f.yScale)
		}
		horiAdvance = mulFix(horiAdvance, f.xScale)
	}

	cbox := g.Outline.CBox()
	g.HoriBearingX = cbox.XMin
	g.AdvanceX = horiAdvance
	if err := g.Outline.check(); err != nil {
		return nil, err
	}
	return g, nil
}
