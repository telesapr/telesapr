package ttf

// FreeType's Adobe CFF engine (psaux's cf2 files: psfont.c, psintrp.c,
// pshints.c, psblues.c, psstack.c) for CFF and CFF2 charstrings; the
// Type 1 paths aren't ported. Its numbers are 16.16 in 32 bits, so
// sums wrap (ADD_INT32) and FT_MulFix/FT_DivFix results are truncated.

type cf2Fixed = int32

const (
	cf2FixedMax     = cf2Fixed(0x7FFFFFFF)
	cf2FixedEpsilon = cf2Fixed(1)

	cf2OperandStackSize = 48
	cf2MaxSubr          = 16
	cf2StorageSize      = 32
	cf2MaxHints         = 96
	cf2MaxHintEdges     = cf2MaxHints * 2

	cf2FlagsHinted   = 1
	cf2FlagsDarkened = 2

	cf2GhostBottom = 0x1
	cf2GhostTop    = 0x2
	cf2PairBottom  = 0x4
	cf2PairTop     = 0x8
	cf2Locked      = 0x10
	cf2Synthetic   = 0x20

	cf2MaxBlues      = 7
	cf2MaxOtherBlues = 5
)

var (
	cf2ICFTop      = cf2IntToFixed(880)
	cf2ICFBottom   = cf2IntToFixed(-120)
	cf2MinCounter  = cf2DoubleToFixed(0.5)
	errStackOvfl   = errInvalidGlyphFmt
	errCf2Invalid  = errInvalidGlyphFmt
	errCf2TooBig   = errInvalidArg
	cf2Darken07    = cf2DoubleToFixed(0.7)
	cf2Darken03    = cf2DoubleToFixed(1.0 - 0.7)
	cf2Darken17    = cf2DoubleToFixed(1.0 + 0.7)
	cf2DarkenM07   = cf2DoubleToFixed(-0.7)
	cf2SnapTenth   = cf2DoubleToFixed(0.1)
	cf2EmRatioMin  = cf2DoubleToFixed(.01)
	cf2BoostFactor = cf2DoubleToFixed(.6)
)

func cf2IntToFixed(i int32) cf2Fixed { return cf2Fixed(uint32(i) << 16) }

func cf2FixedToInt(x cf2Fixed) int16 { return int16((uint32(x) + 0x8000) >> 16) }

func cf2FixedRound(x cf2Fixed) cf2Fixed {
	return cf2Fixed((uint32(x) + 0x8000) & 0xFFFF0000)
}

// cf2DoubleToFixed is cf2_doubleToFixed, computed in doubles.
func cf2DoubleToFixed(f float64) cf2Fixed { return cf2Fixed(f*65536.0 + 0.5) }

func cf2FixedAbs(x cf2Fixed) cf2Fixed {
	if x < 0 {
		return -x
	}
	return x
}

func cf2FixedFloor(x cf2Fixed) cf2Fixed { return cf2Fixed(uint32(x) & 0xFFFF0000) }

func cf2FixedFraction(x cf2Fixed) cf2Fixed { return x - cf2FixedFloor(x) }

// cf2MulFix and cf2DivFix are FT_MulFix and FT_DivFix on cf2 values,
// truncated to 32 bits as the C assignments do.
func cf2MulFix(a, b cf2Fixed) cf2Fixed { return cf2Fixed(mulFix(int64(a), int64(b))) }

func cf2DivFix(a, b cf2Fixed) cf2Fixed { return cf2Fixed(divFix(int64(a), int64(b))) }

// cf2SetError is cf2_setError: the first error sticks.
func cf2SetError(e *error, v error) {
	if e != nil && *e == nil {
		*e = v
	}
}

// cf2Matrix is a CF2_Matrix.
type cf2Matrix struct{ a, b, c, d, tx, ty cf2Fixed }

// cf2Buffer is a CF2_BufferRec: a charstring and the read position.
type cf2Buffer struct {
	data []byte
	ptr  int
}

func (b *cf2Buffer) readByte() int32 {
	if b.ptr < len(b.data) {
		v := b.data[b.ptr]
		b.ptr++
		return int32(v)
	}
	// the buffers have no error pointer
	return 0
}

func (b *cf2Buffer) isEnd() bool { return b.ptr >= len(b.data) }

// The operand stack (psstack.c).

type cf2NumberType int

const (
	cf2NumberFixed cf2NumberType = iota
	cf2NumberFrac
	cf2NumberInt
)

type cf2StackNumber struct {
	v   int32
	typ cf2NumberType
}

type cf2Stack struct {
	buffer []cf2StackNumber
	top    int
	err    *error
}

func newCf2Stack(err *error, size int) *cf2Stack {
	return &cf2Stack{buffer: make([]cf2StackNumber, size), err: err}
}

func (s *cf2Stack) count() uint32 { return uint32(s.top) }

func (s *cf2Stack) pushInt(val int32) {
	if s.top == len(s.buffer) {
		cf2SetError(s.err, errStackOvfl)
		return
	}
	s.buffer[s.top] = cf2StackNumber{val, cf2NumberInt}
	s.top++
}

func (s *cf2Stack) pushFixed(val cf2Fixed) {
	if s.top == len(s.buffer) {
		cf2SetError(s.err, errStackOvfl)
		return
	}
	s.buffer[s.top] = cf2StackNumber{val, cf2NumberFixed}
	s.top++
}

func (s *cf2Stack) popInt() int32 {
	if s.top == 0 {
		cf2SetError(s.err, errStackUnderflow)
		return 0
	}
	if s.buffer[s.top-1].typ != cf2NumberInt {
		cf2SetError(s.err, errSyntax)
		return 0
	}
	s.top--
	return s.buffer[s.top].v
}

func cf2FracToFixed(x int32) cf2Fixed {
	var neg int32
	if x < 0 {
		neg = 1
	}
	return (x + 0x2000 - neg) >> 14
}

func (n cf2StackNumber) real() cf2Fixed {
	switch n.typ {
	case cf2NumberInt:
		return cf2IntToFixed(n.v)
	case cf2NumberFrac:
		return cf2FracToFixed(n.v)
	}
	return n.v
}

func (s *cf2Stack) popFixed() cf2Fixed {
	if s.top == 0 {
		cf2SetError(s.err, errStackUnderflow)
		return 0
	}
	s.top--
	return s.buffer[s.top].real()
}

func (s *cf2Stack) getReal(idx uint32) cf2Fixed {
	if idx >= s.count() {
		cf2SetError(s.err, errStackOvfl)
		return 0
	}
	return s.buffer[idx].real()
}

func (s *cf2Stack) setReal(idx uint32, val cf2Fixed) {
	if idx > s.count() {
		cf2SetError(s.err, errStackOvfl)
		return
	}
	s.buffer[idx] = cf2StackNumber{val, cf2NumberFixed}
}

func (s *cf2Stack) pop(num uint32) {
	if num > s.count() {
		cf2SetError(s.err, errStackUnderflow)
		return
	}
	s.top -= int(num)
}

func (s *cf2Stack) roll(count, shift int32) {
	last := cf2StackNumber{0, cf2NumberInt}
	if count < 2 {
		return
	}
	if uint32(count) > s.count() {
		cf2SetError(s.err, errStackOvfl)
		return
	}
	if shift < 0 {
		shift = -((-shift) % count)
	} else {
		shift %= count
	}
	if shift == 0 {
		return
	}
	startIdx, idx := int32(-1), int32(-1)
	for i := int32(0); i < count; i++ {
		if startIdx == idx {
			startIdx++
			idx = startIdx
			last = s.buffer[idx]
		}
		idx += shift
		if idx >= count {
			idx -= count
		} else if idx < 0 {
			idx += count
		}
		tmp := s.buffer[idx]
		s.buffer[idx] = last
		last = tmp
	}
}

func (s *cf2Stack) clear() { s.top = 0 }

// Hint masks.

type cf2HintMask struct {
	err       *error
	isValid   bool
	isNew     bool
	bitCount  int
	byteCount int
	mask      [(cf2MaxHints + 7) / 8]byte
}

func (m *cf2HintMask) init(err *error) { *m = cf2HintMask{err: err} }

func (m *cf2HintMask) setCounts(bitCount int) int {
	if bitCount > cf2MaxHints {
		cf2SetError(m.err, errInvalidGlyphFmt)
		return 0
	}
	m.bitCount = bitCount
	m.byteCount = (bitCount + 7) / 8
	m.isValid = true
	m.isNew = true
	return bitCount
}

func (m *cf2HintMask) read(cs *cf2Buffer, bitCount int) {
	if m.setCounts(bitCount) == 0 {
		return
	}
	for i := 0; i < m.byteCount; i++ {
		m.mask[i] = byte(cs.readByte())
	}
}

func (m *cf2HintMask) setAll(bitCount int) {
	mask := uint32(1)<<(uint32(-bitCount)&7) - 1
	if m.setCounts(bitCount) == 0 {
		return
	}
	for i := 0; i < m.byteCount; i++ {
		m.mask[i] = 0xFF
	}
	m.mask[m.byteCount-1] &^= byte(mask)
}

// Stem hints, hints and hint maps (pshints.c).

type cf2StemHint struct {
	used         bool
	min, max     cf2Fixed
	minDS, maxDS cf2Fixed
}

type cf2Hint struct {
	flags   uint32
	index   int
	csCoord cf2Fixed
	dsCoord cf2Fixed
	scale   cf2Fixed
}

func (h *cf2Hint) isValid() bool     { return h.flags != 0 }
func (h *cf2Hint) isPair() bool      { return h.flags&(cf2PairBottom|cf2PairTop) != 0 }
func (h *cf2Hint) isPairTop() bool   { return h.flags&cf2PairTop != 0 }
func (h *cf2Hint) isTop() bool       { return h.flags&(cf2PairTop|cf2GhostTop) != 0 }
func (h *cf2Hint) isBottom() bool    { return h.flags&(cf2PairBottom|cf2GhostBottom) != 0 }
func (h *cf2Hint) isLocked() bool    { return h.flags&cf2Locked != 0 }
func (h *cf2Hint) isSynthetic() bool { return h.flags&cf2Synthetic != 0 }
func (h *cf2Hint) lock()             { h.flags |= cf2Locked }

// stemHintAt is cf2_arrstack_getPointer on a stem hint array.
func stemHintAt(arr *[]cf2StemHint, idx int, err *error) *cf2StemHint {
	if idx >= len(*arr) {
		cf2SetError(err, errStackOvfl)
		idx = 0
	}
	return &(*arr)[idx]
}

// hintInit is cf2_hint_init.
func (h *cf2Hint) init(stemHintArray *[]cf2StemHint, indexStemHint int, font *cf2Font,
	hintOrigin, scale cf2Fixed, bottom bool) {
	*h = cf2Hint{}
	stemHint := stemHintAt(stemHintArray, indexStemHint, &font.err)
	width := stemHint.max - stemHint.min
	switch {
	case width == cf2IntToFixed(-21):
		if bottom {
			h.csCoord = stemHint.max
			h.flags = cf2GhostBottom
		} else {
			h.flags = 0
		}
	case width == cf2IntToFixed(-20):
		if bottom {
			h.flags = 0
		} else {
			h.csCoord = stemHint.min
			h.flags = cf2GhostTop
		}
	case width < 0:
		if bottom {
			h.csCoord = stemHint.max
			h.flags = cf2PairBottom
		} else {
			h.csCoord = stemHint.min
			h.flags = cf2PairTop
		}
	default:
		if bottom {
			h.csCoord = stemHint.min
			h.flags = cf2PairBottom
		} else {
			h.csCoord = stemHint.max
			h.flags = cf2PairTop
		}
	}
	if h.isTop() {
		h.csCoord += 2 * font.darkenY
	}
	h.csCoord += hintOrigin
	h.scale = scale
	h.index = indexStemHint
	if h.flags != 0 && stemHint.used {
		if h.isTop() {
			h.dsCoord = stemHint.maxDS
		} else {
			h.dsCoord = stemHint.minDS
		}
		h.lock()
	} else {
		h.dsCoord = cf2MulFix(h.csCoord, scale)
	}
}

type cf2HintMove struct {
	j      int
	moveUp cf2Fixed
}

type cf2HintMap struct {
	font           *cf2Font
	initialHintMap *cf2HintMap
	hintMoves      *[]cf2HintMove
	isValid        bool
	hinted         bool
	scale          cf2Fixed
	count          int
	lastIndex      int
	edge           [cf2MaxHintEdges]cf2Hint
}

func (m *cf2HintMap) init(font *cf2Font, initialMap *cf2HintMap, hintMoves *[]cf2HintMove,
	scale cf2Fixed) {
	*m = cf2HintMap{hinted: font.hinted, scale: scale, font: font,
		initialHintMap: initialMap, hintMoves: hintMoves}
}

// mapCoord is cf2_hintmap_map.
func (m *cf2HintMap) mapCoord(csCoord cf2Fixed) cf2Fixed {
	if m.count == 0 || !m.hinted {
		return cf2MulFix(csCoord, m.scale)
	}
	i := m.lastIndex
	for i < m.count-1 && csCoord >= m.edge[i+1].csCoord {
		i++
	}
	for i > 0 && csCoord < m.edge[i].csCoord {
		i--
	}
	m.lastIndex = i
	if i == 0 && csCoord < m.edge[0].csCoord {
		return cf2MulFix(csCoord-m.edge[0].csCoord, m.scale) + m.edge[0].dsCoord
	}
	return cf2MulFix(csCoord-m.edge[i].csCoord, m.edge[i].scale) + m.edge[i].dsCoord
}

// adjustHints is cf2_hintmap_adjustHints.
func (m *cf2HintMap) adjustHints() {
	*m.hintMoves = (*m.hintMoves)[:0]
	for i := 0; i < m.count; i++ {
		isPair := m.edge[i].isPair()
		var move cf2Fixed
		j := i
		if isPair {
			j = i + 1
		}
		dsCoordI := m.edge[i].dsCoord
		dsCoordJ := m.edge[j].dsCoord
		if !m.edge[i].isLocked() {
			fracDown := cf2FixedFraction(dsCoordI)
			fracUp := cf2FixedFraction(dsCoordJ)
			downMoveDown := 0 - fracDown
			upMoveDown := 0 - fracUp
			var downMoveUp, upMoveUp cf2Fixed
			if fracDown != 0 {
				downMoveUp = cf2IntToFixed(1) - fracDown
			}
			if fracUp != 0 {
				upMoveUp = cf2IntToFixed(1) - fracUp
			}
			moveUp := downMoveUp
			if upMoveUp < moveUp {
				moveUp = upMoveUp
			}
			moveDown := downMoveDown
			if upMoveDown > moveDown {
				moveDown = upMoveDown
			}
			downMinCounter := cf2MinCounter
			upMinCounter := cf2MinCounter
			saveEdge := false

			if j >= m.count-1 || m.edge[j+1].dsCoord >= dsCoordJ+(moveUp+upMinCounter) {
				if i == 0 || m.edge[i-1].dsCoord <= dsCoordI+(moveDown-downMinCounter) {
					if -moveDown < moveUp {
						move = moveDown
					} else {
						move = moveUp
					}
				} else {
					move = moveUp
				}
			} else {
				if i == 0 || m.edge[i-1].dsCoord <= dsCoordI+(moveDown-downMinCounter) {
					move = moveDown
					saveEdge = moveUp < -moveDown
				} else {
					move = 0
					saveEdge = true
				}
			}
			if saveEdge && j < m.count-1 && !m.edge[j+1].isLocked() {
				*m.hintMoves = append(*m.hintMoves, cf2HintMove{j, moveUp - move})
			}
			m.edge[i].dsCoord = dsCoordI + move
			if isPair {
				m.edge[j].dsCoord = dsCoordJ + move
			}
		}

		if i > 0 {
			if m.edge[i].csCoord != m.edge[i-1].csCoord {
				m.edge[i-1].scale = cf2DivFix(m.edge[i].dsCoord-m.edge[i-1].dsCoord,
					m.edge[i].csCoord-m.edge[i-1].csCoord)
			}
		}
		if isPair {
			if m.edge[j].csCoord != m.edge[j-1].csCoord {
				m.edge[j-1].scale = cf2DivFix(m.edge[j].dsCoord-m.edge[j-1].dsCoord,
					m.edge[j].csCoord-m.edge[j-1].csCoord)
			}
			i++
		}
	}

	for i := len(*m.hintMoves); i > 0; i-- {
		hintMove := &(*m.hintMoves)[i-1]
		j := hintMove.j
		if m.edge[j+1].dsCoord >= m.edge[j].dsCoord+(hintMove.moveUp+cf2MinCounter) {
			m.edge[j].dsCoord += hintMove.moveUp
			if m.edge[j].isPair() {
				m.edge[j-1].dsCoord += hintMove.moveUp
			}
		}
	}
}

// insertHint is cf2_hintmap_insertHint.
func (m *cf2HintMap) insertHint(bottomHintEdge, topHintEdge *cf2Hint) {
	isPair := true
	firstHintEdge := bottomHintEdge
	secondHintEdge := topHintEdge
	if !bottomHintEdge.isValid() {
		firstHintEdge = topHintEdge
		isPair = false
	} else if !topHintEdge.isValid() {
		isPair = false
	}
	if isPair && topHintEdge.csCoord < bottomHintEdge.csCoord {
		return
	}
	indexInsert := 0
	for ; indexInsert < m.count; indexInsert++ {
		if m.edge[indexInsert].csCoord >= firstHintEdge.csCoord {
			break
		}
	}
	if indexInsert < m.count {
		if m.edge[indexInsert].csCoord == firstHintEdge.csCoord {
			return
		}
		if isPair && m.edge[indexInsert].csCoord <= secondHintEdge.csCoord {
			return
		}
		if m.edge[indexInsert].isPairTop() {
			return
		}
	}
	if m.initialHintMap.isValid && !firstHintEdge.isLocked() {
		if isPair {
			midpoint := m.initialHintMap.mapCoord(firstHintEdge.csCoord +
				(secondHintEdge.csCoord-firstHintEdge.csCoord)/2)
			halfWidth := cf2MulFix((secondHintEdge.csCoord-firstHintEdge.csCoord)/2, m.scale)
			firstHintEdge.dsCoord = midpoint - halfWidth
			secondHintEdge.dsCoord = midpoint + halfWidth
		} else {
			firstHintEdge.dsCoord = m.initialHintMap.mapCoord(firstHintEdge.csCoord)
		}
	}
	if indexInsert > 0 {
		if firstHintEdge.dsCoord < m.edge[indexInsert-1].dsCoord {
			return
		}
	}
	if indexInsert < m.count {
		if isPair {
			if secondHintEdge.dsCoord > m.edge[indexInsert].dsCoord {
				return
			}
		} else {
			if firstHintEdge.dsCoord > m.edge[indexInsert].dsCoord {
				return
			}
		}
	}

	iSrc := m.count - 1
	iDst := m.count
	if isPair {
		iDst = m.count + 1
	}
	count := m.count - indexInsert
	if iDst >= cf2MaxHintEdges {
		return
	}
	for ; count > 0; count-- {
		m.edge[iDst] = m.edge[iSrc]
		iDst--
		iSrc--
	}
	m.edge[indexInsert] = *firstHintEdge
	m.count++
	if isPair {
		m.edge[indexInsert+1] = *secondHintEdge
		m.count++
	}
}

// build is cf2_hintmap_build.
func (m *cf2HintMap) build(hStemHintArray, vStemHintArray *[]cf2StemHint, hintMask *cf2HintMask,
	hintOrigin cf2Fixed, initialMap bool) {
	font := m.font
	var tempHintMask cf2HintMask

	if !initialMap && !m.initialHintMap.isValid {
		tempHintMask.init(hintMask.err)
		m.initialHintMap.build(hStemHintArray, vStemHintArray, &tempHintMask, hintOrigin, true)
	}
	if !hintMask.isValid {
		hintMask.setAll(len(*hStemHintArray) + len(*vStemHintArray))
		if !hintMask.isValid {
			return
		}
	}
	m.count = 0
	m.lastIndex = 0

	tempHintMask = *hintMask
	maskPtr := 0
	bitCount := len(*hStemHintArray)
	if bitCount > hintMask.bitCount {
		return
	}

	if font.blues.doEmBoxHints {
		var dummy cf2Hint
		m.insertHint(&font.blues.emBoxBottomEdge, &dummy)
		m.insertHint(&dummy, &font.blues.emBoxTopEdge)
	}

	maskByte := byte(0x80)
	for i := 0; i < bitCount; i++ {
		if maskByte&tempHintMask.mask[maskPtr] != 0 {
			var bottomHintEdge, topHintEdge cf2Hint
			bottomHintEdge.init(hStemHintArray, i, font, hintOrigin, m.scale, true)
			topHintEdge.init(hStemHintArray, i, font, hintOrigin, m.scale, false)
			if bottomHintEdge.isLocked() || topHintEdge.isLocked() ||
				font.blues.capture(&bottomHintEdge, &topHintEdge) {
				m.insertHint(&bottomHintEdge, &topHintEdge)
				tempHintMask.mask[maskPtr] &^= maskByte
			}
		}
		if i&7 == 7 {
			maskPtr++
			maskByte = 0x80
		} else {
			maskByte >>= 1
		}
	}

	if initialMap {
		if m.count == 0 || m.edge[0].csCoord > 0 || m.edge[m.count-1].csCoord < 0 {
			edge := cf2Hint{flags: cf2GhostBottom | cf2Locked | cf2Synthetic, scale: m.scale}
			var invalid cf2Hint
			m.insertHint(&edge, &invalid)
		}
	} else {
		maskPtr = 0
		maskByte = 0x80
		for i := 0; i < bitCount; i++ {
			if maskByte&tempHintMask.mask[maskPtr] != 0 {
				var bottomHintEdge, topHintEdge cf2Hint
				bottomHintEdge.init(hStemHintArray, i, font, hintOrigin, m.scale, true)
				topHintEdge.init(hStemHintArray, i, font, hintOrigin, m.scale, false)
				m.insertHint(&bottomHintEdge, &topHintEdge)
			}
			if i&7 == 7 {
				maskPtr++
				maskByte = 0x80
			} else {
				maskByte >>= 1
			}
		}
	}

	m.adjustHints()

	if !initialMap {
		for i := 0; i < m.count; i++ {
			if !m.edge[i].isSynthetic() {
				stemhint := stemHintAt(hStemHintArray, m.edge[i].index, &font.err)
				if m.edge[i].isTop() {
					stemhint.maxDS = m.edge[i].dsCoord
				} else {
					stemhint.minDS = m.edge[i].dsCoord
				}
				stemhint.used = true
			}
		}
	}
	m.isValid = true
	hintMask.isNew = false
}

// Blue zones (psblues.c).

type cf2Blue struct {
	csBottomEdge, csTopEdge, csFlatEdge, dsFlatEdge cf2Fixed
	bottomZone                                      bool
}

type cf2Blues struct {
	scale                           cf2Fixed
	count                           int
	suppressOvershoot, doEmBoxHints bool
	blueScale, blueShift, blueFuzz  cf2Fixed
	boost                           cf2Fixed
	emBoxTopEdge, emBoxBottomEdge   cf2Hint
	zone                            [cf2MaxBlues + cf2MaxOtherBlues]cf2Blue
}

// init is cf2_blues_init.
func (b *cf2Blues) init(font *cf2Font) {
	sub := font.subfont
	priv := &sub.privateDict
	*b = cf2Blues{}
	b.scale = font.innerTransform.d

	// cf2_getBlueMetrics
	b.blueScale = cf2Fixed(divFix(priv.blueScale, int64(cf2IntToFixed(1000))))
	b.blueShift = cf2IntToFixed(int32(priv.blueShift))
	b.blueFuzz = cf2IntToFixed(int32(priv.blueFuzz))

	// the counts index the fixed arrays, as C's pointers do
	blueValues, numBlueValues := &priv.blueValues, int(priv.numBlueValues)
	otherBlues, numOtherBlues := &priv.otherBlues, int(priv.numOtherBlues)
	familyBlues, numFamilyBlues := &priv.familyBlues, int(priv.numFamilyBlues)
	familyOtherBlues, numFamilyOtherBlues := &priv.familyOtherBlues, int(priv.numFamilyOtherBlues)

	emBoxBottom := cf2ICFBottom
	emBoxTop := cf2ICFTop
	var maxZoneHeight cf2Fixed

	if priv.languageGroup == 1 &&
		(numBlueValues == 0 ||
			(numBlueValues == 4 &&
				blueValues[0] < int64(emBoxBottom) &&
				blueValues[1] < int64(emBoxBottom) &&
				blueValues[2] > int64(emBoxTop) &&
				blueValues[3] > int64(emBoxTop))) {
		b.emBoxBottomEdge.csCoord = emBoxBottom - cf2FixedEpsilon
		b.emBoxBottomEdge.dsCoord = cf2FixedRound(cf2MulFix(b.emBoxBottomEdge.csCoord, b.scale)) -
			cf2MinCounter
		b.emBoxBottomEdge.scale = b.scale
		b.emBoxBottomEdge.flags = cf2GhostBottom | cf2Locked | cf2Synthetic

		b.emBoxTopEdge.csCoord = emBoxTop + cf2FixedEpsilon + 2*font.darkenY
		b.emBoxTopEdge.dsCoord = cf2FixedRound(cf2MulFix(b.emBoxTopEdge.csCoord, b.scale)) +
			cf2MinCounter
		b.emBoxTopEdge.scale = b.scale
		b.emBoxTopEdge.flags = cf2GhostTop | cf2Locked | cf2Synthetic

		b.doEmBoxHints = true
		return
	}

	for i := 0; i < numBlueValues; i += 2 {
		z := &b.zone[b.count]
		z.csBottomEdge = cf2Fixed(blueValues[i])
		z.csTopEdge = cf2Fixed(blueValues[i+1])
		zoneHeight := z.csTopEdge - z.csBottomEdge
		if zoneHeight < 0 {
			continue
		}
		if zoneHeight > maxZoneHeight {
			maxZoneHeight = zoneHeight
		}
		if i != 0 {
			z.csTopEdge += 2 * font.darkenY
			z.csBottomEdge += 2 * font.darkenY
		}
		if i == 0 {
			z.bottomZone = true
			z.csFlatEdge = z.csTopEdge
		} else {
			z.bottomZone = false
			z.csFlatEdge = z.csBottomEdge
		}
		b.count++
	}
	for i := 0; i < numOtherBlues; i += 2 {
		z := &b.zone[b.count]
		z.csBottomEdge = cf2Fixed(otherBlues[i])
		z.csTopEdge = cf2Fixed(otherBlues[i+1])
		zoneHeight := z.csTopEdge - z.csBottomEdge
		if zoneHeight < 0 {
			continue
		}
		if zoneHeight > maxZoneHeight {
			maxZoneHeight = zoneHeight
		}
		z.bottomZone = true
		z.csFlatEdge = z.csTopEdge
		b.count++
	}

	csUnitsPerPixel := cf2DivFix(cf2IntToFixed(1), b.scale)
	for i := 0; i < b.count; i++ {
		flatEdge := b.zone[i].csFlatEdge
		if b.zone[i].bottomZone {
			minDiff := cf2FixedMax
			for j := 0; j < numFamilyOtherBlues; j += 2 {
				flatFamilyEdge := cf2Fixed(familyOtherBlues[j+1])
				diff := cf2FixedAbs(flatEdge - flatFamilyEdge)
				if diff < minDiff && diff < csUnitsPerPixel {
					b.zone[i].csFlatEdge = flatFamilyEdge
					minDiff = diff
					if diff == 0 {
						break
					}
				}
			}
			if numFamilyBlues >= 2 {
				flatFamilyEdge := cf2Fixed(familyBlues[1])
				diff := cf2FixedAbs(flatEdge - flatFamilyEdge)
				if diff < minDiff && diff < csUnitsPerPixel {
					b.zone[i].csFlatEdge = flatFamilyEdge
				}
			}
		} else {
			minDiff := cf2FixedMax
			for j := 2; j < numFamilyBlues; j += 2 {
				flatFamilyEdge := cf2Fixed(familyBlues[j])
				flatFamilyEdge += 2 * font.darkenY
				diff := cf2FixedAbs(flatEdge - flatFamilyEdge)
				if diff < minDiff && diff < csUnitsPerPixel {
					b.zone[i].csFlatEdge = flatFamilyEdge
					minDiff = diff
					if diff == 0 {
						break
					}
				}
			}
		}
	}

	if maxZoneHeight > 0 {
		if b.blueScale > cf2DivFix(cf2IntToFixed(1), maxZoneHeight) {
			b.blueScale = cf2DivFix(cf2IntToFixed(1), maxZoneHeight)
		}
	}
	if b.scale < b.blueScale {
		b.suppressOvershoot = true
		b.boost = cf2BoostFactor - cf2Fixed(mulDiv(int64(cf2BoostFactor), int64(b.scale), int64(b.blueScale)))
		if b.boost > 0x7FFF {
			b.boost = 0x7FFF
		}
	}
	if font.stemDarkened {
		b.boost = 0
	}
	for i := 0; i < b.count; i++ {
		if b.zone[i].bottomZone {
			b.zone[i].dsFlatEdge = cf2FixedRound(cf2MulFix(b.zone[i].csFlatEdge, b.scale) - b.boost)
		} else {
			b.zone[i].dsFlatEdge = cf2FixedRound(cf2MulFix(b.zone[i].csFlatEdge, b.scale) + b.boost)
		}
	}
}

// capture is cf2_blues_capture.
func (b *cf2Blues) capture(bottomHintEdge, topHintEdge *cf2Hint) bool {
	csFuzz := b.blueFuzz
	var dsNew, dsMove cf2Fixed
	captured := false
	for i := 0; i < b.count; i++ {
		z := &b.zone[i]
		if z.bottomZone && bottomHintEdge.isBottom() {
			if z.csBottomEdge-csFuzz <= bottomHintEdge.csCoord &&
				bottomHintEdge.csCoord <= z.csTopEdge+csFuzz {
				if b.suppressOvershoot {
					dsNew = z.dsFlatEdge
				} else if z.csTopEdge-bottomHintEdge.csCoord >= b.blueShift {
					dsNew = cf2FixedRound(bottomHintEdge.dsCoord)
					if lim := z.dsFlatEdge - cf2IntToFixed(1); lim < dsNew {
						dsNew = lim
					}
				} else {
					dsNew = cf2FixedRound(bottomHintEdge.dsCoord)
				}
				dsMove = dsNew - bottomHintEdge.dsCoord
				captured = true
				break
			}
		}
		if !z.bottomZone && topHintEdge.isTop() {
			if z.csBottomEdge-csFuzz <= topHintEdge.csCoord &&
				topHintEdge.csCoord <= z.csTopEdge+csFuzz {
				if b.suppressOvershoot {
					dsNew = z.dsFlatEdge
				} else if topHintEdge.csCoord-z.csBottomEdge >= b.blueShift {
					dsNew = cf2FixedRound(topHintEdge.dsCoord)
					if lim := z.dsFlatEdge + cf2IntToFixed(1); lim > dsNew {
						dsNew = lim
					}
				} else {
					dsNew = cf2FixedRound(topHintEdge.dsCoord)
				}
				dsMove = dsNew - topHintEdge.dsCoord
				captured = true
				break
			}
		}
	}
	if captured {
		if bottomHintEdge.isValid() {
			bottomHintEdge.dsCoord += dsMove
			bottomHintEdge.lock()
		}
		if topHintEdge.isValid() {
			topHintEdge.dsCoord += dsMove
			topHintEdge.lock()
		}
	}
	return captured
}

// The glyph path (pshints.c).

type cf2PathOp int

const (
	cf2PathOpMoveTo cf2PathOp = iota + 1
	cf2PathOpLineTo
	cf2PathOpCubeTo
)

type cf2CallbackParams struct {
	pt0, pt1, pt2, pt3 Vector
	op                 cf2PathOp
}

type cf2GlyphPath struct {
	font      *cf2Font
	callbacks *cf2Outline

	hintMap, firstHintMap, initialHintMap cf2HintMap
	hintMoves                             []cf2HintMove

	scaleX, scaleC, scaleY cf2Fixed
	fractionalTranslation  Vector

	pathIsOpen, pathIsClosing bool
	darken                    bool
	moveIsPending             bool

	hStemHintArray, vStemHintArray *[]cf2StemHint
	hintMask                       *cf2HintMask
	hintOriginY                    cf2Fixed
	blues                          *cf2Blues

	xOffset, yOffset cf2Fixed
	miterLimit       cf2Fixed
	snapThreshold    cf2Fixed

	offsetStart0, offsetStart1 Vector
	currentCS, currentDS       Vector
	start                      Vector

	elemIsQueued                                   bool
	prevElemOp                                     cf2PathOp
	prevElemP0, prevElemP1, prevElemP2, prevElemP3 Vector
}

func (gp *cf2GlyphPath) init(font *cf2Font, callbacks *cf2Outline, scaleY cf2Fixed,
	hStemHintArray, vStemHintArray *[]cf2StemHint, hintMask *cf2HintMask, hintOriginY cf2Fixed,
	blues *cf2Blues, fractionalTranslation Vector) {
	*gp = cf2GlyphPath{font: font, callbacks: callbacks}
	gp.initialHintMap.init(font, &gp.initialHintMap, &gp.hintMoves, scaleY)
	gp.firstHintMap.init(font, &gp.initialHintMap, &gp.hintMoves, scaleY)
	gp.hintMap.init(font, &gp.initialHintMap, &gp.hintMoves, scaleY)
	gp.scaleX = font.innerTransform.a
	gp.scaleC = font.innerTransform.c
	gp.scaleY = font.innerTransform.d
	gp.fractionalTranslation = fractionalTranslation
	gp.hStemHintArray = hStemHintArray
	gp.vStemHintArray = vStemHintArray
	gp.hintMask = hintMask
	gp.hintOriginY = hintOriginY
	gp.blues = blues
	gp.darken = font.darkened
	gp.xOffset = font.darkenX
	gp.yOffset = font.darkenY
	m := cf2FixedAbs(gp.xOffset)
	if y := cf2FixedAbs(gp.yOffset); y > m {
		m = y
	}
	gp.miterLimit = 2 * m
	gp.snapThreshold = cf2SnapTenth
	gp.moveIsPending = true
}

// hintPoint is cf2_glyphpath_hintPoint.
func (gp *cf2GlyphPath) hintPoint(hintmap *cf2HintMap, x, y cf2Fixed) Vector {
	ptX := cf2MulFix(gp.scaleX, x) + cf2MulFix(gp.scaleC, y)
	ptY := hintmap.mapCoord(y)
	ot := &gp.font.outerTransform
	return Vector{
		int64(cf2MulFix(ot.a, ptX) + (cf2MulFix(ot.c, ptY) + cf2Fixed(gp.fractionalTranslation.X))),
		int64(cf2MulFix(ot.b, ptX) + (cf2MulFix(ot.d, ptY) + cf2Fixed(gp.fractionalTranslation.Y))),
	}
}

// computeIntersection is cf2_glyphpath_computeIntersection.
func (gp *cf2GlyphPath) computeIntersection(u1, u2, v1, v2 Vector) (Vector, bool) {
	scale := func(x int64) int64 { return (int64(cf2Fixed(x)) + 0x10) >> 5 }
	sub := func(a, b int64) int64 { return int64(cf2Fixed(a) - cf2Fixed(b)) }
	perp := func(ax, ay, bx, by int64) int64 { return mulFix(ax, by) - mulFix(ay, bx) }

	ux, uy := scale(sub(u2.X, u1.X)), scale(sub(u2.Y, u1.Y))
	vx, vy := scale(sub(v2.X, v1.X)), scale(sub(v2.Y, v1.Y))
	wx, wy := scale(sub(v1.X, u1.X)), scale(sub(v1.Y, u1.Y))

	denominator := cf2Fixed(perp(ux, uy, vx, vy))
	if denominator == 0 {
		return Vector{}, false
	}
	s := cf2Fixed(divFix(perp(wx, wy, vx, vy), int64(denominator)))

	var in Vector
	in.X = int64(cf2Fixed(u1.X) + cf2MulFix(s, cf2Fixed(u2.X)-cf2Fixed(u1.X)))
	in.Y = int64(cf2Fixed(u1.Y) + cf2MulFix(s, cf2Fixed(u2.Y)-cf2Fixed(u1.Y)))

	if u1.X == u2.X && cf2FixedAbs(cf2Fixed(in.X)-cf2Fixed(u1.X)) < gp.snapThreshold {
		in.X = u1.X
	}
	if u1.Y == u2.Y && cf2FixedAbs(cf2Fixed(in.Y)-cf2Fixed(u1.Y)) < gp.snapThreshold {
		in.Y = u1.Y
	}
	if v1.X == v2.X && cf2FixedAbs(cf2Fixed(in.X)-cf2Fixed(v1.X)) < gp.snapThreshold {
		in.X = v1.X
	}
	if v1.Y == v2.Y && cf2FixedAbs(cf2Fixed(in.Y)-cf2Fixed(v1.Y)) < gp.snapThreshold {
		in.Y = v1.Y
	}
	// intersection->x - ADD_INT32( u2->x, v1->x ) / 2 is in FT_Pos
	if abs64s(in.X-int64((cf2Fixed(u2.X)+cf2Fixed(v1.X))/2)) > int64(gp.miterLimit) ||
		abs64s(in.Y-int64((cf2Fixed(u2.Y)+cf2Fixed(v1.Y))/2)) > int64(gp.miterLimit) {
		return in, false
	}
	return in, true
}

// pushPrevElem is cf2_glyphpath_pushPrevElem.
func (gp *cf2GlyphPath) pushPrevElem(hintmap *cf2HintMap, nextP0 *Vector, nextP1 Vector, close bool) {
	var params cf2CallbackParams
	var prevP0, prevP1 *Vector
	if gp.prevElemOp == cf2PathOpLineTo {
		prevP0, prevP1 = &gp.prevElemP0, &gp.prevElemP1
	} else {
		prevP0, prevP1 = &gp.prevElemP2, &gp.prevElemP3
	}
	var intersection Vector
	useIntersection := false
	if prevP1.X != nextP0.X || prevP1.Y != nextP0.Y {
		intersection, useIntersection = gp.computeIntersection(*prevP0, *prevP1, *nextP0, nextP1)
		if useIntersection {
			*prevP1 = intersection
		}
	}

	params.pt0 = gp.currentDS
	switch gp.prevElemOp {
	case cf2PathOpLineTo:
		params.op = cf2PathOpLineTo
		if close {
			params.pt1 = gp.hintPoint(&gp.firstHintMap, cf2Fixed(gp.prevElemP1.X), cf2Fixed(gp.prevElemP1.Y))
		} else {
			params.pt1 = gp.hintPoint(hintmap, cf2Fixed(gp.prevElemP1.X), cf2Fixed(gp.prevElemP1.Y))
		}
		if params.pt0 != params.pt1 {
			gp.callbacks.lineTo(&params)
			gp.currentDS = params.pt1
		}
	case cf2PathOpCubeTo:
		params.op = cf2PathOpCubeTo
		params.pt1 = gp.hintPoint(hintmap, cf2Fixed(gp.prevElemP1.X), cf2Fixed(gp.prevElemP1.Y))
		params.pt2 = gp.hintPoint(hintmap, cf2Fixed(gp.prevElemP2.X), cf2Fixed(gp.prevElemP2.Y))
		params.pt3 = gp.hintPoint(hintmap, cf2Fixed(gp.prevElemP3.X), cf2Fixed(gp.prevElemP3.Y))
		gp.callbacks.cubeTo(&params)
		gp.currentDS = params.pt3
	}

	if !useIntersection || close {
		if close {
			params.pt1 = gp.hintPoint(&gp.firstHintMap, cf2Fixed(nextP0.X), cf2Fixed(nextP0.Y))
		} else {
			params.pt1 = gp.hintPoint(hintmap, cf2Fixed(nextP0.X), cf2Fixed(nextP0.Y))
		}
		if params.pt1 != gp.currentDS {
			params.op = cf2PathOpLineTo
			params.pt0 = gp.currentDS
			gp.callbacks.lineTo(&params)
			gp.currentDS = params.pt1
		}
	}
	if useIntersection {
		*nextP0 = intersection
	}
}

// pushMove is cf2_glyphpath_pushMove.
func (gp *cf2GlyphPath) pushMove(start Vector) {
	var params cf2CallbackParams
	params.op = cf2PathOpMoveTo
	params.pt0 = gp.currentDS
	if !gp.hintMap.isValid {
		gp.moveTo(cf2Fixed(gp.start.X), cf2Fixed(gp.start.Y))
	}
	params.pt1 = gp.hintPoint(&gp.hintMap, cf2Fixed(start.X), cf2Fixed(start.Y))
	gp.callbacks.moveTo(&params)
	gp.currentDS = params.pt1
	gp.offsetStart0 = start
}

// getWindingMomentum is cf2_getWindingMomentum.
func getWindingMomentum(x1, y1, x2, y2 cf2Fixed) int32 {
	return (x1>>16)*((y2-y1)>>16) - (y1>>16)*((x2-x1)>>16)
}

// computeOffset is cf2_glyphpath_computeOffset.
func (gp *cf2GlyphPath) computeOffset(x1, y1, x2, y2 cf2Fixed) (cf2Fixed, cf2Fixed) {
	dx := x2 - x1
	dy := y2 - y1
	if gp.font.reverseWinding {
		dx = -dx
		dy = -dy
	}
	if !gp.darken {
		return 0, 0
	}
	gp.callbacks.windingMomentum += getWindingMomentum(x1, y1, x2, y2)
	var x, y cf2Fixed
	if dx >= 0 {
		if dy >= 0 {
			switch {
			case dx > 2*dy:
				x, y = 0, 0
			case dy > 2*dx:
				x, y = gp.xOffset, gp.yOffset
			default:
				x = cf2MulFix(cf2Darken07, gp.xOffset)
				y = cf2MulFix(cf2Darken03, gp.yOffset)
			}
		} else {
			switch {
			case dx > -2*dy:
				x, y = 0, 0
			case -dy > 2*dx:
				x, y = -gp.xOffset, gp.yOffset
			default:
				x = cf2MulFix(cf2DarkenM07, gp.xOffset)
				y = cf2MulFix(cf2Darken03, gp.yOffset)
			}
		}
	} else {
		if dy >= 0 {
			switch {
			case -dx > 2*dy:
				x, y = 0, 2*gp.yOffset
			case dy > -2*dx:
				x, y = gp.xOffset, gp.yOffset
			default:
				x = cf2MulFix(cf2Darken07, gp.xOffset)
				y = cf2MulFix(cf2Darken17, gp.yOffset)
			}
		} else {
			switch {
			case -dx > -2*dy:
				x, y = 0, 2*gp.yOffset
			case -dy > -2*dx:
				x, y = -gp.xOffset, gp.yOffset
			default:
				x = cf2MulFix(cf2DarkenM07, gp.xOffset)
				y = cf2MulFix(cf2Darken17, gp.yOffset)
			}
		}
	}
	return x, y
}

// moveTo is cf2_glyphpath_moveTo.
func (gp *cf2GlyphPath) moveTo(x, y cf2Fixed) {
	gp.closeOpenPath()
	gp.currentCS = Vector{int64(x), int64(y)}
	gp.start = gp.currentCS
	gp.moveIsPending = true
	if !gp.hintMap.isValid || gp.hintMask.isNew {
		gp.hintMap.build(gp.hStemHintArray, gp.vStemHintArray, gp.hintMask, gp.hintOriginY, false)
	}
	gp.firstHintMap = gp.hintMap
}

// lineTo is cf2_glyphpath_lineTo.
func (gp *cf2GlyphPath) lineTo(x, y cf2Fixed) {
	newHintMap := gp.hintMask.isNew && !gp.pathIsClosing
	if cf2Fixed(gp.currentCS.X) == x && cf2Fixed(gp.currentCS.Y) == y && !newHintMap {
		return
	}
	xOffset, yOffset := gp.computeOffset(cf2Fixed(gp.currentCS.X), cf2Fixed(gp.currentCS.Y), x, y)
	P0 := Vector{int64(cf2Fixed(gp.currentCS.X) + xOffset), int64(cf2Fixed(gp.currentCS.Y) + yOffset)}
	P1 := Vector{int64(x + xOffset), int64(y + yOffset)}
	if gp.moveIsPending {
		gp.pushMove(P0)
		gp.moveIsPending = false
		gp.pathIsOpen = true
		gp.offsetStart1 = P1
	}
	if gp.elemIsQueued {
		gp.pushPrevElem(&gp.hintMap, &P0, P1, false)
	}
	gp.elemIsQueued = true
	gp.prevElemOp = cf2PathOpLineTo
	gp.prevElemP0 = P0
	gp.prevElemP1 = P1
	if newHintMap {
		gp.hintMap.build(gp.hStemHintArray, gp.vStemHintArray, gp.hintMask, gp.hintOriginY, false)
	}
	gp.currentCS = Vector{int64(x), int64(y)}
}

// curveTo is cf2_glyphpath_curveTo.
func (gp *cf2GlyphPath) curveTo(x1, y1, x2, y2, x3, y3 cf2Fixed) {
	xOffset1, yOffset1 := gp.computeOffset(cf2Fixed(gp.currentCS.X), cf2Fixed(gp.currentCS.Y), x1, y1)
	xOffset3, yOffset3 := gp.computeOffset(x2, y2, x3, y3)
	gp.callbacks.windingMomentum += getWindingMomentum(x1, y1, x2, y2)

	P0 := Vector{int64(cf2Fixed(gp.currentCS.X) + xOffset1), int64(cf2Fixed(gp.currentCS.Y) + yOffset1)}
	P1 := Vector{int64(x1 + xOffset1), int64(y1 + yOffset1)}
	P2 := Vector{int64(x2 + xOffset3), int64(y2 + yOffset3)}
	P3 := Vector{int64(x3 + xOffset3), int64(y3 + yOffset3)}

	if gp.moveIsPending {
		gp.pushMove(P0)
		gp.moveIsPending = false
		gp.pathIsOpen = true
		gp.offsetStart1 = P1
	}
	if gp.elemIsQueued {
		gp.pushPrevElem(&gp.hintMap, &P0, P1, false)
	}
	gp.elemIsQueued = true
	gp.prevElemOp = cf2PathOpCubeTo
	gp.prevElemP0 = P0
	gp.prevElemP1 = P1
	gp.prevElemP2 = P2
	gp.prevElemP3 = P3
	if gp.hintMask.isNew {
		gp.hintMap.build(gp.hStemHintArray, gp.vStemHintArray, gp.hintMask, gp.hintOriginY, false)
	}
	gp.currentCS = Vector{int64(x3), int64(y3)}
}

// closeOpenPath is cf2_glyphpath_closeOpenPath.
func (gp *cf2GlyphPath) closeOpenPath() {
	if gp.pathIsOpen {
		gp.pathIsClosing = true
		gp.lineTo(cf2Fixed(gp.start.X), cf2Fixed(gp.start.Y))
		if gp.elemIsQueued {
			gp.pushPrevElem(&gp.hintMap, &gp.offsetStart0, gp.offsetStart1, true)
		}
		gp.moveIsPending = true
		gp.pathIsOpen = false
		gp.pathIsClosing = false
		gp.elemIsQueued = false
	}
}

// The outline builder (psft.c's callbacks and psobjs.c's ps_builder).

type cf2Outline struct {
	windingMomentum int32
	err             *error

	outline   Outline
	pathBegun bool
}

// maxOutlinePoints is FT_OUTLINE_POINTS_MAX, which the glyph loader's
// FT_GLYPHLOADER_CHECK_POINTS enforces.
const maxOutlinePoints = 0xFFFF

func (o *cf2Outline) checkPoints(n int) error {
	if len(o.outline.Points)+n > maxOutlinePoints {
		return errInvalidArg
	}
	return nil
}

// addPoint is ps_builder_add_point: 16.16 to 26.6.
func (o *cf2Outline) addPoint(x, y int64, on bool) {
	tag := byte(curveTagCubic)
	if on {
		tag = curveTagOn
	}
	o.outline.Points = append(o.outline.Points, Vector{x >> 10, y >> 10})
	o.outline.Tags = append(o.outline.Tags, tag)
}

// addContour is ps_builder_add_contour.
func (o *cf2Outline) addContour() error {
	if len(o.outline.Contours)+1 > maxOutlinePoints {
		return errInvalidArg
	}
	if len(o.outline.Contours) > 0 {
		o.outline.Contours[len(o.outline.Contours)-1] = len(o.outline.Points) - 1
	}
	o.outline.Contours = append(o.outline.Contours, 0)
	return nil
}

// startPoint is ps_builder_start_point.
func (o *cf2Outline) startPoint(x, y int64) error {
	if !o.pathBegun {
		o.pathBegun = true
		if err := o.addContour(); err != nil {
			return err
		}
		if err := o.checkPoints(1); err != nil {
			return err
		}
		o.addPoint(x, y, true)
	}
	return nil
}

// closeContour is ps_builder_close_contour.
func (o *cf2Outline) closeContour() {
	out := &o.outline
	nContours := len(out.Contours)
	first := 0
	if nContours > 1 {
		first = out.Contours[nContours-2] + 1
	}
	if nContours > 0 && first == len(out.Points) {
		out.Contours = out.Contours[:nContours-1]
		return
	}
	if len(out.Points) > 1 {
		p1 := out.Points[first]
		p2 := out.Points[len(out.Points)-1]
		if p1 == p2 && out.Tags[len(out.Tags)-1] == curveTagOn {
			out.Points = out.Points[:len(out.Points)-1]
			out.Tags = out.Tags[:len(out.Tags)-1]
		}
	}
	if nContours > 0 {
		if first == len(out.Points)-1 {
			out.Contours = out.Contours[:nContours-1]
			out.Points = out.Points[:len(out.Points)-1]
			out.Tags = out.Tags[:len(out.Tags)-1]
		} else {
			out.Contours[nContours-1] = len(out.Points) - 1
		}
	}
}

func (o *cf2Outline) moveTo(params *cf2CallbackParams) {
	o.closeContour()
	o.pathBegun = false
}

func (o *cf2Outline) lineTo(params *cf2CallbackParams) {
	if !o.pathBegun {
		if err := o.startPoint(params.pt0.X, params.pt0.Y); err != nil {
			cf2SetError(o.err, err)
			return
		}
	}
	if err := o.checkPoints(1); err != nil {
		cf2SetError(o.err, err)
		return
	}
	o.addPoint(params.pt1.X, params.pt1.Y, true)
}

func (o *cf2Outline) cubeTo(params *cf2CallbackParams) {
	if !o.pathBegun {
		if err := o.startPoint(params.pt0.X, params.pt0.Y); err != nil {
			cf2SetError(o.err, err)
			return
		}
	}
	if err := o.checkPoints(3); err != nil {
		cf2SetError(o.err, err)
		return
	}
	o.addPoint(params.pt1.X, params.pt1.Y, false)
	o.addPoint(params.pt2.X, params.pt2.Y, false)
	o.addPoint(params.pt3.X, params.pt3.Y, true)
}

// The font instance (psfont.c).

// cf2Font is a CF2_FontRec, kept across glyphs in cffFont.cf2.
type cf2Font struct {
	err error

	cff     *cffFont
	decoder *cffDecoder
	subfont *cffSubFont

	isCFF2         bool
	renderingFlags int
	hinted         bool
	stemDarkened   bool
	darkened       bool
	reverseWinding bool

	unitsPerEm int32
	ppem       cf2Fixed

	lastSubfont      *cffSubFont
	currentTransform cf2Matrix
	innerTransform   cf2Matrix
	outerTransform   cf2Matrix

	stdVW, stdHW     cf2Fixed
	darkenX, darkenY cf2Fixed
	darkenParams     [8]int32

	blend   cffBlend
	vsindex uint32
	lenNDV  uint32
	ndv     []int64

	blues   cf2Blues
	outline cf2Outline
}

// cf2Darkening are the driver's default darkening-parameters.
var cf2Darkening = [8]int32{500, 400, 1000, 275, 1667, 275, 2333, 0}

// computeDarkening is cf2_computeDarkening.
func computeDarkening(emRatio, ppem, stemWidth cf2Fixed, boldenAmount cf2Fixed, stemDarkened bool,
	darkenParams *[8]int32) cf2Fixed {
	var darkenAmount cf2Fixed
	if boldenAmount == 0 && !stemDarkened {
		return 0
	}
	if emRatio < cf2EmRatioMin {
		return 0
	}
	if stemDarkened {
		x1, y1 := darkenParams[0], darkenParams[1]
		x2, y2 := darkenParams[2], darkenParams[3]
		x3, y3 := darkenParams[4], darkenParams[5]
		x4, y4 := darkenParams[6], darkenParams[7]

		stemWidthPer1000 := cf2MulFix(stemWidth+boldenAmount, emRatio)
		var scaledStem cf2Fixed
		if msb32(uint32(stemWidthPer1000))+msb32(uint32(ppem)) >= 46 {
			scaledStem = cf2IntToFixed(x4)
		} else {
			scaledStem = cf2MulFix(stemWidthPer1000, ppem)
		}

		segment := func(xa, ya, xb, yb int32) (cf2Fixed, bool) {
			xdelta := xb - xa
			ydelta := yb - ya
			x := stemWidthPer1000 - cf2DivFix(cf2IntToFixed(xa), ppem)
			if xdelta == 0 {
				return 0, false
			}
			return cf2Fixed(mulDiv(int64(x), int64(ydelta), int64(xdelta))) +
				cf2DivFix(cf2IntToFixed(ya), ppem), true
		}
		stage := 0
		switch {
		case scaledStem < cf2IntToFixed(x1):
			darkenAmount = cf2DivFix(cf2IntToFixed(y1), ppem)
			stage = 5
		case scaledStem < cf2IntToFixed(x2):
			stage = 1
		case scaledStem < cf2IntToFixed(x3):
			stage = 2
		case scaledStem < cf2IntToFixed(x4):
			stage = 3
		default:
			stage = 4
		}
		if stage == 1 {
			if v, ok := segment(x1, y1, x2, y2); ok {
				darkenAmount = v
				stage = 5
			} else {
				stage = 2
			}
		}
		if stage == 2 {
			if v, ok := segment(x2, y2, x3, y3); ok {
				darkenAmount = v
				stage = 5
			} else {
				stage = 3
			}
		}
		if stage == 3 {
			if v, ok := segment(x3, y3, x4, y4); ok {
				darkenAmount = v
				stage = 5
			} else {
				stage = 4
			}
		}
		if stage == 4 {
			darkenAmount = cf2DivFix(cf2IntToFixed(y4), ppem)
		}
		darkenAmount = cf2DivFix(darkenAmount, 2*emRatio)
	}
	darkenAmount += boldenAmount / 2
	return darkenAmount
}

// setup is cf2_font_setup.
func (font *cf2Font) setup(transform *cf2Matrix) {
	d := font.decoder
	needExtraSetup := false
	font.err = nil

	subFont := d.currentSubfont
	font.subfont = subFont
	if font.lastSubfont != subFont {
		font.lastSubfont = subFont
		needExtraSetup = true
	}

	vstore := &font.cff.vstore
	if vstore.dataCount != 0 {
		lenNDV, ndv := font.cff.normalizedVector()
		if subFont.blend.checkVector(subFont.privateDict.vsindex, lenNDV, ndv) {
			font.cff.loadPrivateDict(subFont, lenNDV, ndv)
			needExtraSetup = true
		}
		font.blend.font = subFont.blend.font
		font.blend.usedBV = false
		font.vsindex = subFont.privateDict.vsindex
		font.lenNDV = lenNDV
		font.ndv = ndv
	}

	ppem := cf2IntToFixed(int32(d.yPpem))
	if font.ppem != ppem {
		font.ppem = ppem
		needExtraSetup = true
	}
	font.hinted = font.renderingFlags&cf2FlagsHinted != 0

	if transform.a != font.currentTransform.a || transform.b != font.currentTransform.b ||
		transform.c != font.currentTransform.c || transform.d != font.currentTransform.d {
		font.currentTransform = *transform
		font.currentTransform.tx = 0
		font.currentTransform.ty = 0
		font.innerTransform = *transform
		font.outerTransform.a = cf2IntToFixed(1)
		font.outerTransform.d = cf2IntToFixed(1)
		font.outerTransform.b = 0
		font.outerTransform.c = 0
		needExtraSetup = true
	}

	if font.stemDarkened != (font.renderingFlags&cf2FlagsDarkened != 0) {
		font.stemDarkened = font.renderingFlags&cf2FlagsDarkened != 0
		needExtraSetup = true
	}

	if needExtraSetup {
		unitsPerEm := font.unitsPerEm
		if unitsPerEm == 0 {
			unitsPerEm = 1000
		}
		ppem = cf2IntToFixed(4)
		if font.ppem > ppem {
			ppem = font.ppem
		}
		emRatio := cf2IntToFixed(1000) / unitsPerEm
		font.stdVW = cf2IntToFixed(int32(subFont.privateDict.standardHeight))
		if font.stdVW <= 0 {
			font.stdVW = cf2DivFix(cf2IntToFixed(75), emRatio)
		}
		// no synthetic emboldening
		font.darkenX = computeDarkening(emRatio, ppem, font.stdVW, 0, font.stemDarkened,
			&font.darkenParams)

		stdHW := cf2IntToFixed(int32(subFont.privateDict.standardWidth))
		if stdHW > 0 && font.stdVW > 2*stdHW {
			font.stdHW = cf2DivFix(cf2IntToFixed(75), emRatio)
		} else {
			font.stdHW = cf2DivFix(cf2IntToFixed(110), emRatio)
		}
		font.darkenY = computeDarkening(emRatio, ppem, font.stdHW, 0, font.stemDarkened,
			&font.darkenParams)
		font.darkened = font.darkenX != 0 || font.darkenY != 0
		font.reverseWinding = false
		font.blues.init(font)
	}
}

// getGlyphOutline is cf2_getGlyphOutline.
func (font *cf2Font) getGlyphOutline(charstring *cf2Buffer, transform *cf2Matrix) (cf2Fixed, error) {
	var advWidth cf2Fixed
	translation := Vector{int64(transform.tx), int64(transform.ty)}
	font.setup(transform)
	if font.err != nil {
		return advWidth, font.err
	}
	font.reverseWinding = false
	needWinding := font.darkened
	for {
		// cf2_outline_reset (which leaves path_begun)
		font.outline.windingMomentum = 0
		font.outline.outline = Outline{}

		advWidth = font.interpT2CharString(charstring, translation, false, 0, 0)
		if font.err != nil {
			return advWidth, font.err
		}
		if !needWinding {
			break
		}
		if font.outline.windingMomentum >= 0 {
			break
		}
		font.reverseWinding = true
		needWinding = false
	}
	// cf2_outline_close
	font.outline.closeContour()
	return advWidth, font.err
}

// The interpreter (psintrp.c).

const (
	cmdHSTEM       = 1
	cmdVSTEM       = 3
	cmdVMOVETO     = 4
	cmdRLINETO     = 5
	cmdHLINETO     = 6
	cmdVLINETO     = 7
	cmdRRCURVETO   = 8
	cmdCLOSEPATH   = 9
	cmdCALLSUBR    = 10
	cmdRETURN      = 11
	cmdESC         = 12
	cmdHSBW        = 13
	cmdENDCHAR     = 14
	cmdVSINDEX     = 15
	cmdBLEND       = 16
	cmdHSTEMHM     = 18
	cmdHINTMASK    = 19
	cmdCNTRMASK    = 20
	cmdRMOVETO     = 21
	cmdHMOVETO     = 22
	cmdVSTEMHM     = 23
	cmdRCURVELINE  = 24
	cmdRLINECURVE  = 25
	cmdVVCURVETO   = 26
	cmdHHCURVETO   = 27
	cmdEXTENDEDNMB = 28
	cmdCALLGSUBR   = 29
	cmdVHCURVETO   = 30
	cmdHVCURVETO   = 31

	escDOTSECTION   = 0
	escVSTEM3       = 1
	escHSTEM3       = 2
	escAND          = 3
	escOR           = 4
	escNOT          = 5
	escSEAC         = 6
	escSBW          = 7
	escRESERVED8    = 8
	escABS          = 9
	escADD          = 10
	escSUB          = 11
	escDIV          = 12
	escRESERVED13   = 13
	escNEG          = 14
	escEQ           = 15
	escCALLOTHERSUB = 16
	escPOP          = 17
	escDROP         = 18
	escRESERVED19   = 19
	escPUT          = 20
	escGET          = 21
	escIFELSE       = 22
	escRANDOM       = 23
	escMUL          = 24
	escRESERVED25   = 25
	escSQRT         = 26
	escDUP          = 27
	escEXCH         = 28
	escINDEX        = 29
	escROLL         = 30
	escRESERVED31   = 31
	escRESERVED32   = 32
	escSETCURRENTPT = 33
	escHFLEX        = 34
	escFLEX         = 35
	escHFLEX1       = 36
	escFLEX1        = 37
	escRESERVED38   = 38
)

// doStems is cf2_doStems.
func (font *cf2Font) doStems(opStack *cf2Stack, stemHintArray *[]cf2StemHint, width *cf2Fixed,
	haveWidth *bool, hintOffset cf2Fixed) {
	count := opStack.count()
	hasWidthArg := count&1 != 0
	position := hintOffset
	if hasWidthArg && !*haveWidth {
		*width = opStack.getReal(0) + font.decoder.nominalWidthX()
	}
	i := uint32(0)
	if hasWidthArg {
		i = 1
	}
	for ; i < count; i += 2 {
		var stemhint cf2StemHint
		position += opStack.getReal(i)
		stemhint.min = position
		position += opStack.getReal(i + 1)
		stemhint.max = position
		*stemHintArray = append(*stemHintArray, stemhint)
	}
	opStack.clear()
	*haveWidth = true
}

// doFlex is cf2_doFlex.
func doFlex(opStack *cf2Stack, curX, curY *cf2Fixed, glyphPath *cf2GlyphPath,
	readFromStack *[12]bool, doConditionalLastRead bool) {
	var vals [14]cf2Fixed
	vals[0] = *curX
	vals[1] = *curY
	idx := uint32(0)
	isHFlex := !readFromStack[9]
	top := 10
	if isHFlex {
		top = 9
	}
	for i := 0; i < top; i++ {
		vals[i+2] = vals[i]
		if readFromStack[i] {
			vals[i+2] += opStack.getReal(idx)
			idx++
		}
	}
	if isHFlex {
		vals[9+2] = *curY
	}
	if doConditionalLastRead {
		lastIsX := cf2FixedAbs(vals[10]-*curX) > cf2FixedAbs(vals[11]-*curY)
		lastVal := opStack.getReal(idx)
		if lastIsX {
			vals[12] = vals[10] + lastVal
			vals[13] = *curY
		} else {
			vals[12] = *curX
			vals[13] = vals[11] + lastVal
		}
	} else {
		if readFromStack[10] {
			vals[12] = vals[10] + opStack.getReal(idx)
			idx++
		} else {
			vals[12] = *curX
		}
		if readFromStack[11] {
			vals[13] = vals[11] + opStack.getReal(idx)
		} else {
			vals[13] = *curY
		}
	}
	for j := 0; j < 2; j++ {
		glyphPath.curveTo(vals[j*6+2], vals[j*6+3], vals[j*6+4], vals[j*6+5], vals[j*6+6], vals[j*6+7])
	}
	opStack.clear()
	*curX = vals[12]
	*curY = vals[13]
}

// doBlend is cf2_doBlend.
func doBlend(blend *cffBlend, opStack *cf2Stack, numBlends uint32) {
	numOperands := numBlends * blend.lenBV
	base := opStack.count() - numOperands
	delta := base + numBlends
	for i := uint32(0); i < numBlends; i++ {
		sum := opStack.getReal(i + base)
		for j := uint32(1); j < blend.lenBV; j++ {
			sum += cf2MulFix(blend.bv[j], opStack.getReal(delta))
			delta++
		}
		opStack.setReal(i+base, sum)
	}
	opStack.pop(numOperands - numBlends)
}

var (
	readHFlex  = [12]bool{true, false, true, true, true, false, true, false, true, false, true, false}
	readFlex   = [12]bool{true, true, true, true, true, true, true, true, true, true, true, true}
	readHFlex1 = [12]bool{true, true, true, true, true, false, true, false, true, true, true, false}
	readFlex1  = [12]bool{true, true, true, true, true, true, true, true, true, true, false, false}
)

// interpT2CharString is cf2_interpT2CharString; it returns the width.
func (font *cf2Font) interpT2CharString(buf *cf2Buffer, translation Vector, doingSeac bool,
	curX, curY cf2Fixed) (width cf2Fixed) {
	var lastError error
	d := font.decoder
	errp := &font.err

	scaleY := font.innerTransform.d
	nominalWidthX := d.nominalWidthX()
	hintOriginY := curY

	var storage [cf2StorageSize]cf2Fixed
	instructionLimit := uint32(20000000)

	var hStemHintArray, vStemHintArray []cf2StemHint
	var hintMask cf2HintMask
	var glyphPath cf2GlyphPath

	hintMask.init(errp)
	glyphPath.init(font, &font.outline, scaleY, &hStemHintArray, &vStemHintArray, &hintMask,
		hintOriginY, &font.blues, translation)

	haveWidth := font.isCFF2
	width = d.defaultWidthX()

	stackSize := cf2OperandStackSize
	if font.isCFF2 {
		stackSize = int(font.cff.topFont.fontDict.maxstack)
	}
	opStack := newCf2Stack(errp, stackSize)

	subrStack := make([]cf2Buffer, cf2MaxSubr+1)
	charstringIndex := 0
	subrStack[0] = *buf
	charstring := &subrStack[0]

	defer func() {
		cf2SetError(errp, lastError)
	}()

	for {
		var op1 byte
		if charstring.isEnd() {
			if charstringIndex != 0 {
				op1 = cmdRETURN
			} else {
				op1 = cmdENDCHAR
			}
		} else {
			op1 = byte(charstring.readByte())
			if (op1 == cmdRETURN || op1 == cmdENDCHAR) && font.isCFF2 {
				op1 = 0
			}
		}

		if *errp != nil {
			return
		}
		instructionLimit--
		if instructionLimit == 0 {
			lastError = errInvalidGlyphFmt
			return
		}

		clearStack := true
		switch op1 {
		case 0, 2, 17:
			// reserved

		case cmdVSINDEX:
			if !font.isCFF2 {
				break
			}
			if font.blend.usedBV {
				lastError = errInvalidGlyphFmt
				return
			}
			if temp := opStack.popInt(); temp >= 0 {
				font.vsindex = uint32(temp)
			}

		case cmdBLEND:
			if !font.isCFF2 {
				break
			}
			if font.blend.font == nil {
				lastError = errInvalidGlyphFmt
				return
			}
			if font.blend.checkVector(font.vsindex, font.lenNDV, font.ndv) {
				if err := font.blend.buildVector(font.vsindex, font.lenNDV, font.ndv); err != nil {
					lastError = err
					return
				}
			}
			numBlends := uint32(opStack.popInt())
			if numBlends > uint32(stackSize) {
				lastError = errInvalidGlyphFmt
				return
			}
			doBlend(&font.blend, opStack, numBlends)
			font.blend.usedBV = true
			clearStack = false

		case cmdHSTEMHM, cmdHSTEM:
			if hintMask.isValid {
				break
			}
			font.doStems(opStack, &hStemHintArray, &width, &haveWidth, 0)

		case cmdVSTEMHM, cmdVSTEM:
			if hintMask.isValid {
				break
			}
			font.doStems(opStack, &vStemHintArray, &width, &haveWidth, 0)

		case cmdVMOVETO:
			if opStack.count() > 1 && !haveWidth {
				width = opStack.getReal(0) + nominalWidthX
			}
			haveWidth = true
			curY += opStack.popFixed()
			glyphPath.moveTo(curX, curY)

		case cmdRLINETO:
			count := opStack.count()
			for idx := uint32(0); idx < count; idx += 2 {
				curX += opStack.getReal(idx + 0)
				curY += opStack.getReal(idx + 1)
				glyphPath.lineTo(curX, curY)
			}
			opStack.clear()
			clearStack = false

		case cmdHLINETO, cmdVLINETO:
			count := opStack.count()
			isX := op1 == cmdHLINETO
			for idx := uint32(0); idx < count; idx++ {
				v := opStack.getReal(idx)
				if isX {
					curX += v
				} else {
					curY += v
				}
				isX = !isX
				glyphPath.lineTo(curX, curY)
			}
			opStack.clear()
			clearStack = false

		case cmdRCURVELINE, cmdRRCURVETO:
			count := opStack.count()
			idx := uint32(0)
			for idx+6 <= count {
				x1 := opStack.getReal(idx+0) + curX
				y1 := opStack.getReal(idx+1) + curY
				x2 := opStack.getReal(idx+2) + x1
				y2 := opStack.getReal(idx+3) + y1
				x3 := opStack.getReal(idx+4) + x2
				y3 := opStack.getReal(idx+5) + y2
				glyphPath.curveTo(x1, y1, x2, y2, x3, y3)
				curX = x3
				curY = y3
				idx += 6
			}
			if op1 == cmdRCURVELINE {
				curX += opStack.getReal(idx + 0)
				curY += opStack.getReal(idx + 1)
				glyphPath.lineTo(curX, curY)
			}
			opStack.clear()
			clearStack = false

		case cmdCLOSEPATH:
			// Type 1 only

		case cmdCALLGSUBR, cmdCALLSUBR:
			if charstringIndex > cf2MaxSubr {
				lastError = errInvalidGlyphFmt
				return
			}
			// cf2_arrstack_getPointer on the subroutine stack
			next := charstringIndex + 1
			if next >= len(subrStack) {
				cf2SetError(errp, errStackOvfl)
				next = 0
			}
			charstring = &subrStack[next]
			subrNum := opStack.popInt()
			var failed bool
			if op1 == cmdCALLGSUBR {
				failed = d.initGlobalRegionBuffer(subrNum, charstring)
			} else {
				failed = d.initLocalRegionBuffer(subrNum, charstring)
			}
			if failed {
				lastError = errInvalidGlyphFmt
				return
			}
			charstringIndex++
			clearStack = false

		case cmdRETURN:
			if charstringIndex < 1 {
				lastError = errInvalidGlyphFmt
				return
			}
			charstringIndex--
			charstring = &subrStack[charstringIndex]
			clearStack = false

		case cmdESC:
			op2 := byte(charstring.readByte())
			switch op2 {
			case escHFLEX:
				doFlex(opStack, &curX, &curY, &glyphPath, &readHFlex, false)
				clearStack = false
			case escFLEX:
				doFlex(opStack, &curX, &curY, &glyphPath, &readFlex, false)
			case escHFLEX1:
				doFlex(opStack, &curX, &curY, &glyphPath, &readHFlex1, false)
				clearStack = false
			case escFLEX1:
				doFlex(opStack, &curX, &curY, &glyphPath, &readFlex1, true)
				clearStack = false
			case escRESERVED8, escRESERVED13, escRESERVED19, escRESERVED25,
				escRESERVED31, escRESERVED32:
			default:
				if font.isCFF2 || op2 >= escRESERVED38 {
					break
				}
				switch op2 {
				case escDOTSECTION, escVSTEM3, escHSTEM3, escSEAC, escSBW,
					escCALLOTHERSUB, escSETCURRENTPT:
					// ignored, or Type 1 only
					if op2 == escCALLOTHERSUB {
						clearStack = false
					}
				case escAND:
					arg2 := opStack.popFixed()
					arg1 := opStack.popFixed()
					opStack.pushInt(b2i(arg1 != 0 && arg2 != 0))
					clearStack = false
				case escOR:
					arg2 := opStack.popFixed()
					arg1 := opStack.popFixed()
					opStack.pushInt(b2i(arg1 != 0 || arg2 != 0))
					clearStack = false
				case escNOT:
					arg := opStack.popFixed()
					opStack.pushInt(b2i(arg == 0))
					clearStack = false
				case escABS:
					arg := opStack.popFixed()
					if arg < -cf2FixedMax {
						opStack.pushFixed(cf2FixedMax)
					} else {
						opStack.pushFixed(cf2FixedAbs(arg))
					}
					clearStack = false
				case escADD:
					summand2 := opStack.popFixed()
					summand1 := opStack.popFixed()
					opStack.pushFixed(summand1 + summand2)
					clearStack = false
				case escSUB:
					subtrahend := opStack.popFixed()
					minuend := opStack.popFixed()
					opStack.pushFixed(minuend - subtrahend)
					clearStack = false
				case escDIV:
					divisor := opStack.popFixed()
					dividend := opStack.popFixed()
					opStack.pushFixed(cf2DivFix(dividend, divisor))
					clearStack = false
				case escNEG:
					arg := opStack.popFixed()
					if arg < -cf2FixedMax {
						opStack.pushFixed(cf2FixedMax)
					} else {
						opStack.pushFixed(-arg)
					}
					clearStack = false
				case escEQ:
					arg2 := opStack.popFixed()
					arg1 := opStack.popFixed()
					opStack.pushInt(b2i(arg1 == arg2))
					clearStack = false
				case escPOP:
					// Type 1 only
					clearStack = false
				case escDROP:
					opStack.popFixed()
					clearStack = false
				case escPUT:
					idx := uint32(opStack.popInt())
					val := opStack.popFixed()
					if idx < cf2StorageSize {
						storage[idx] = val
					}
					clearStack = false
				case escGET:
					idx := uint32(opStack.popInt())
					if idx < cf2StorageSize {
						opStack.pushFixed(storage[idx])
					}
					clearStack = false
				case escIFELSE:
					cond2 := opStack.popFixed()
					cond1 := opStack.popFixed()
					arg2 := opStack.popFixed()
					arg1 := opStack.popFixed()
					if cond1 <= cond2 {
						opStack.pushFixed(arg1)
					} else {
						opStack.pushFixed(arg2)
					}
					clearStack = false
				case escRANDOM:
					sub := d.currentSubfont
					r := cf2Fixed((sub.random & 0xFFFF) + 1)
					sub.random = cffRandom(sub.random)
					opStack.pushFixed(r)
					clearStack = false
				case escMUL:
					factor2 := opStack.popFixed()
					factor1 := opStack.popFixed()
					opStack.pushFixed(cf2MulFix(factor1, factor2))
					clearStack = false
				case escSQRT:
					arg := opStack.popFixed()
					if arg > 0 {
						arg = cf2Fixed(sqrtFixed(uint32(arg)))
					} else {
						arg = 0
					}
					opStack.pushFixed(arg)
					clearStack = false
				case escDUP:
					arg := opStack.popFixed()
					opStack.pushFixed(arg)
					opStack.pushFixed(arg)
					clearStack = false
				case escEXCH:
					arg2 := opStack.popFixed()
					arg1 := opStack.popFixed()
					opStack.pushFixed(arg2)
					opStack.pushFixed(arg1)
					clearStack = false
				case escINDEX:
					idx := opStack.popInt()
					size := opStack.count()
					if size > 0 {
						var grIdx uint32
						if idx < 0 {
							grIdx = size - 1
						} else if uint32(idx) >= size {
							grIdx = 0
						} else {
							grIdx = size - 1 - uint32(idx)
						}
						opStack.pushFixed(opStack.getReal(grIdx))
					}
					clearStack = false
				case escROLL:
					idx := opStack.popInt()
					count := opStack.popInt()
					opStack.roll(count, idx)
					clearStack = false
				}
			}

		case cmdHSBW:
			// Type 1 only

		case cmdENDCHAR:
			if opStack.count() == 1 || opStack.count() == 5 {
				if !haveWidth {
					width = opStack.getReal(0) + nominalWidthX
				}
			}
			haveWidth = true
			glyphPath.closeOpenPath()

			if !font.isCFF2 && opStack.count() > 1 {
				// an implied seac
				if doingSeac {
					lastError = errInvalidGlyphFmt
					return
				}
				achar := opStack.popInt()
				bchar := opStack.popInt()
				curY = opStack.popFixed()
				curX = opStack.popFixed()

				component, err := d.getSeacComponent(achar)
				if err != nil {
					lastError = err
					return
				}
				font.interpT2CharString(component, translation, true, curX, curY)

				component, err = d.getSeacComponent(bchar)
				if err != nil {
					lastError = err
					return
				}
				font.interpT2CharString(component, translation, true, 0, 0)
			}
			return

		case cmdCNTRMASK, cmdHINTMASK:
			if opStack.count() > 1 && hintMask.isValid {
				break
			}
			font.doStems(opStack, &vStemHintArray, &width, &haveWidth, 0)
			if op1 == cmdHINTMASK {
				hintMask.read(charstring, len(hStemHintArray)+len(vStemHintArray))
			} else {
				var counterHintMap cf2HintMap
				var counterMask cf2HintMask
				counterHintMap.init(font, &glyphPath.initialHintMap, &glyphPath.hintMoves, scaleY)
				counterMask.init(errp)
				counterMask.read(charstring, len(hStemHintArray)+len(vStemHintArray))
				counterHintMap.build(&hStemHintArray, &vStemHintArray, &counterMask, 0, false)
			}

		case cmdRMOVETO:
			if opStack.count() > 2 && !haveWidth {
				width = opStack.getReal(0) + nominalWidthX
			}
			haveWidth = true
			curY += opStack.popFixed()
			curX += opStack.popFixed()
			glyphPath.moveTo(curX, curY)

		case cmdHMOVETO:
			if opStack.count() > 1 && !haveWidth {
				width = opStack.getReal(0) + nominalWidthX
			}
			haveWidth = true
			curX += opStack.popFixed()
			glyphPath.moveTo(curX, curY)

		case cmdRLINECURVE:
			count := opStack.count()
			idx := uint32(0)
			for idx+6 < count {
				curX += opStack.getReal(idx + 0)
				curY += opStack.getReal(idx + 1)
				glyphPath.lineTo(curX, curY)
				idx += 2
			}
			for idx < count {
				x1 := opStack.getReal(idx+0) + curX
				y1 := opStack.getReal(idx+1) + curY
				x2 := opStack.getReal(idx+2) + x1
				y2 := opStack.getReal(idx+3) + y1
				x3 := opStack.getReal(idx+4) + x2
				y3 := opStack.getReal(idx+5) + y2
				glyphPath.curveTo(x1, y1, x2, y2, x3, y3)
				curX = x3
				curY = y3
				idx += 6
			}
			opStack.clear()
			clearStack = false

		case cmdVVCURVETO:
			count1 := opStack.count()
			count := count1 &^ 2
			idx := count1 - count
			for idx < count {
				var x1 cf2Fixed
				if (count-idx)&1 != 0 {
					x1 = opStack.getReal(idx) + curX
					idx++
				} else {
					x1 = curX
				}
				y1 := opStack.getReal(idx+0) + curY
				x2 := opStack.getReal(idx+1) + x1
				y2 := opStack.getReal(idx+2) + y1
				x3 := x2
				y3 := opStack.getReal(idx+3) + y2
				glyphPath.curveTo(x1, y1, x2, y2, x3, y3)
				curX = x3
				curY = y3
				idx += 4
			}
			opStack.clear()
			clearStack = false

		case cmdHHCURVETO:
			count1 := opStack.count()
			count := count1 &^ 2
			idx := count1 - count
			for idx < count {
				var y1 cf2Fixed
				if (count-idx)&1 != 0 {
					y1 = opStack.getReal(idx) + curY
					idx++
				} else {
					y1 = curY
				}
				x1 := opStack.getReal(idx+0) + curX
				x2 := opStack.getReal(idx+1) + x1
				y2 := opStack.getReal(idx+2) + y1
				x3 := opStack.getReal(idx+3) + x2
				y3 := y2
				glyphPath.curveTo(x1, y1, x2, y2, x3, y3)
				curX = x3
				curY = y3
				idx += 4
			}
			opStack.clear()
			clearStack = false

		case cmdVHCURVETO, cmdHVCURVETO:
			count1 := opStack.count()
			count := count1 &^ 2
			idx := count1 - count
			alternate := op1 == cmdHVCURVETO
			for idx < count {
				var x1, x2, x3, y1, y2, y3 cf2Fixed
				if alternate {
					x1 = opStack.getReal(idx+0) + curX
					y1 = curY
					x2 = opStack.getReal(idx+1) + x1
					y2 = opStack.getReal(idx+2) + y1
					y3 = opStack.getReal(idx+3) + y2
					if count-idx == 5 {
						x3 = opStack.getReal(idx+4) + x2
						idx++
					} else {
						x3 = x2
					}
					alternate = false
				} else {
					x1 = curX
					y1 = opStack.getReal(idx+0) + curY
					x2 = opStack.getReal(idx+1) + x1
					y2 = opStack.getReal(idx+2) + y1
					x3 = opStack.getReal(idx+3) + x2
					if count-idx == 5 {
						y3 = opStack.getReal(idx+4) + y2
						idx++
					} else {
						y3 = y2
					}
					alternate = true
				}
				glyphPath.curveTo(x1, y1, x2, y2, x3, y3)
				curX = x3
				curY = y3
				idx += 4
			}
			opStack.clear()
			clearStack = false

		case cmdEXTENDEDNMB:
			byte1 := charstring.readByte()
			byte2 := charstring.readByte()
			opStack.pushInt(int32(int16(byte1<<8 | byte2)))
			clearStack = false

		default:
			switch {
			case op1 <= 246:
				opStack.pushInt(int32(op1) - 139)
			case op1 <= 250:
				v := int32(op1)
				v -= 247
				v *= 256
				v += charstring.readByte()
				v += 108
				opStack.pushInt(v)
			case op1 <= 254:
				v := int32(op1)
				v -= 251
				v *= 256
				v += charstring.readByte()
				v = -v - 108
				opStack.pushInt(v)
			default:
				b1 := uint32(charstring.readByte())
				b2 := uint32(charstring.readByte())
				b3 := uint32(charstring.readByte())
				b4 := uint32(charstring.readByte())
				opStack.pushFixed(cf2Fixed(b1<<24 | b2<<16 | b3<<8 | b4))
			}
			clearStack = false
		}
		if clearStack {
			opStack.clear()
		}
	}
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// sqrtFixed is FT_SqrtFixed.
func sqrtFixed(v uint32) uint32 {
	if v == 0 {
		return 0
	}
	r := uint64(v)<<16 - 1
	q := uint32(1) << ((17 + msb32(v)) >> 1)
	for {
		t := q
		q = (t + uint32(r/uint64(t)) + 1) >> 1
		if q == t {
			break
		}
	}
	return q
}
