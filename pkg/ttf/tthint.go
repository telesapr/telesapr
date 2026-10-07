package ttf

import (
	"sort"
	"strings"
)

// The TrueType driver's bytecode set-up (ttobjs.c, ttpload.c): FreeType
// hints the glyphs of `tricky' fonts even with FT_LOAD_NO_HINTING, as
// they need their bytecode to render.

// frameAt is FT_STREAM_SEEK and FT_FRAME_ENTER on the font file.
func (f *Face) frameAt(pos int64, n uint64) ([]byte, error) {
	s := &cffStream{data: f.data}
	if err := s.seek(pos); err != nil {
		return nil, err
	}
	return s.frame(n)
}

// loadMaxp is tt_face_load_maxp.
func (f *Face) loadMaxp() error {
	_, pos, ok := f.table("maxp")
	if !ok {
		return errTableMissing
	}
	p, err := f.frameAt(pos, 6)
	if err != nil {
		return err
	}
	f.maxp = maxProfile{version: int64(int32(peek32(p, 0)))}
	f.numGlyphs = int64(peek16(p, 4))
	f.maxpNumGlyphs = f.numGlyphs
	if f.maxp.version >= 0x10000 {
		p, err := f.frameAt(pos+6, 26)
		if err != nil {
			return err
		}
		m := &f.maxp
		m.maxTwilightPoints = peek16(p, 10)
		m.maxStorage = peek16(p, 12)
		m.maxFunctionDefs = peek16(p, 14)
		m.maxInstructionDefs = peek16(p, 16)
		m.maxStackElements = peek16(p, 18)
		if m.maxFunctionDefs < 64 {
			m.maxFunctionDefs = 64
		}
		if m.maxTwilightPoints > 0xFFFF-4 {
			m.maxTwilightPoints = 0xFFFF - 4
		}
	}
	return nil
}

// loadVertical is sfnt_load_face's loading of vhea and vmtx.
func (f *Face) loadVertical() error {
	_, pos, ok := f.table("vhea")
	if !ok {
		return nil
	}
	p, err := f.frameAt(pos, 36)
	if err != nil {
		return err
	}
	f.vheaNumberOfMetrics = peek16(p, 34)
	for _, e := range f.tables {
		if e.tag == tag("vmtx") && e.length != 0 {
			f.vmtxOffset = int64(e.offset)
			f.vmtxSize = int64(e.length)
			f.vertInfo = true
			break
		}
	}
	return nil
}

// metrics is tt_face_get_metrics: a glyph's side bearing and advance in
// font units, horizontal or vertical.
func (f *Face) metrics(vertical bool, gindex uint32) (bearing int16, advance uint16) {
	tablePos, tableSize, k := f.hmtxOffset, f.hmtxSize, uint32(f.numberOfHMetrics)
	if vertical {
		tablePos, tableSize, k = f.vmtxOffset, f.vmtxSize, uint32(f.vheaNumberOfMetrics)
	}
	tableEnd := tablePos + tableSize
	if k == 0 {
		return 0, 0
	}
	if gindex < k {
		pos := tablePos + 4*int64(gindex)
		if pos+4 > tableEnd {
			return 0, 0
		}
		return int16(peek16(f.data, int(pos)+2)), peek16(f.data, int(pos))
	}
	pos := tablePos + 4*int64(k-1)
	if pos+2 > tableEnd {
		return 0, 0
	}
	advance = peek16(f.data, int(pos))
	pos += 4 + 2*int64(gindex-k)
	if pos+2 > tableEnd {
		return 0, advance
	}
	return int16(peek16(f.data, int(pos))), advance
}

// hMetrics is TT_Get_HMetrics.
func (f *Face) hMetrics(gindex uint32) (lsb int16, aw uint16) {
	return f.metrics(false, gindex)
}

// vMetrics is TT_Get_VMetrics.
func (f *Face) vMetrics(gindex uint32, yMax int64) (tsb int16, ah uint16) {
	if f.vertInfo {
		return f.metrics(true, gindex)
	}
	asc, desc := int64(f.hheaAscender), int64(f.hheaDescender)
	if f.os2Version != 0xFFFF {
		asc, desc = int64(f.typoAscender), int64(f.typoDescender)
	}
	return int16(asc - yMax), uint16(abs64(asc - desc))
}

// loadBytecodeTables is the rest of tt_face_init after sfnt_load_face:
// the trickiness check, hdmx, and the CVT, font and CVT programs.
func (f *Face) loadBytecodeTables() error {
	f.tricky = f.checkTrickyness()
	f.loadHdmx()
	if cvt, pos, ok := f.table("cvt "); ok {
		n := uint64(len(cvt)) / 2
		p, err := f.frameAt(pos, n*2)
		if err != nil {
			return err
		}
		f.cvt = make([]int32, n)
		for i := range f.cvt {
			f.cvt[i] = int32(int16(peek16(p, 2*i))) * 64
		}
	}
	if fpgm, _, ok := f.table("fpgm"); ok {
		f.fontProgram = fpgm
	}
	if prep, _, ok := f.table("prep"); ok {
		f.cvtProgram = prep
	}
	return nil
}

// loadHdmx is tt_face_load_hdmx.
func (f *Face) loadHdmx() {
	p, _, ok := f.table("hdmx")
	if !ok || len(p) < 8 {
		return
	}
	numRecords := int(peek16(p, 2))
	recordSize := uint64(peek32(p, 4))
	if recordSize >= 0xFFFF0000 {
		recordSize &= 0xFFFF
	}
	if numRecords > 255 || numRecords == 0 {
		return
	}
	if int64(recordSize) != (f.numGlyphs+2+3)&^3 {
		return
	}
	var records [][]byte
	pos := uint64(8)
	for range numRecords {
		if pos+recordSize > uint64(len(p)) {
			break
		}
		records = append(records, p[pos:pos+recordSize])
		pos += recordSize
	}
	// glibc's qsort is a merge sort
	sort.SliceStable(records, func(i, j int) bool { return records[i][0] < records[j][0] })
	f.hdmxRecords = records
}

// deviceMetrics is tt_face_get_device_metrics for glyph 0: the widths
// of the hdmx record for a ppem, or nil.
func (f *Face) deviceMetrics(ppem uint32) []byte {
	lo, hi := 0, len(f.hdmxRecords)
	for lo < hi {
		mid := (lo + hi) >> 1
		r := f.hdmxRecords[mid]
		if uint32(r[0]) > ppem {
			hi = mid
		} else if uint32(r[0]) < ppem {
			lo = mid + 1
		} else {
			return r[2:]
		}
	}
	return nil
}

// trickNames are the family names of tt_check_trickyness_family.
var trickNames = [...]string{
	"cpop", "DFGirl-W6-WIN-BF", "DFGothic-EB", "DFGyoSho-Lt", "DFHei",
	"DFHSGothic-W5", "DFHSMincho-W3", "DFHSMincho-W7", "DFKaiSho-SB",
	"DFKaiShu", "DFKai-SB", "DFMing", "DLC", "HuaTianKaiTi?",
	"HuaTianSongTi?", "Ming(for ISO10646)", "MingLiU", "MingMedium",
	"PMingLiU", "MingLi43",
}

// checkTrickynessFamily is tt_check_trickyness_family.
func checkTrickynessFamily(name string) bool {
	isUpper := func(c byte) bool { return 'A' <= c && c <= 'Z' }
	if len(name) > 7 && isUpper(name[0]) && isUpper(name[1]) && isUpper(name[2]) &&
		isUpper(name[3]) && isUpper(name[4]) && isUpper(name[5]) && name[6] == '+' {
		name = name[7:]
	}
	for _, t := range trickNames {
		if strings.Contains(name, t) {
			return true
		}
	}
	return false
}

// trickSfntIDs are tt_check_trickyness_sfnt_ids' checksums and lengths of
// the cvt, fpgm and prep tables.
var trickSfntIDs = [31][3][2]uint32{
	{{0x05BCF058, 0x000002E4}, {0x28233BF1, 0x000087C4}, {0xA344A1EA, 0x000001E1}},
	{{0x05BCF058, 0x000002E4}, {0x28233BF1, 0x000087C4}, {0xA344A1EB, 0x000001E1}},
	{{0x12C3EBB2, 0x00000350}, {0xB680EE64, 0x000087A7}, {0xCE939563, 0x00000758}},
	{{0x11E5EAD4, 0x00000350}, {0xCE5956E9, 0x0000BC85}, {0x8272F416, 0x00000045}},
	{{0x1257EB46, 0x00000350}, {0xF699D160, 0x0000715F}, {0xD222F568, 0x000003BC}},
	{{0x1262EB4E, 0x00000350}, {0xE86A5D64, 0x00007940}, {0x7850F729, 0x000005FF}},
	{{0x122DEB0A, 0x00000350}, {0x3D16328A, 0x0000859B}, {0xA93FC33B, 0x000002CB}},
	{{0x125FEB26, 0x00000350}, {0xA5ACC982, 0x00007EE1}, {0x90999196, 0x0000041F}},
	{{0x11E5EAD4, 0x00000350}, {0x5A30CA3B, 0x00009063}, {0x13A42602, 0x0000007E}},
	{{0x11E5EAD4, 0x00000350}, {0xA6E78C01, 0x00008998}, {0x13A42602, 0x0000007E}},
	{{0x11E5EAD4, 0x00000360}, {0x9DB282B2, 0x0000C06E}, {0x53E6D7CA, 0x00000082}},
	{{0x1243EB18, 0x00000350}, {0xBA0A8C30, 0x000074AD}, {0xF3D83409, 0x0000037B}},
	{{0x07DCF546, 0x00000308}, {0x40FE7C90, 0x00008E2A}, {0x608174B5, 0x0000007A}},
	{{0xEB891238, 0x00000308}, {0xD2E4DCD4, 0x0000676F}, {0x8EA5F293, 0x000003B8}},
	{{0xFFFBFFFC, 0x00000008}, {0x9C9E48B8, 0x0000BEA2}, {0x70020112, 0x00000008}},
	{{0xFFFBFFFC, 0x00000008}, {0x0A5A0483, 0x00017C39}, {0x70020112, 0x00000008}},
	{{0x00000000, 0x00000000}, {0x40C92555, 0x000000E5}, {0xA39B58E3, 0x0000117C}},
	{{0x00000000, 0x00000000}, {0x33C41652, 0x000000E5}, {0x26D6C52A, 0x00000F6A}},
	{{0x00000000, 0x00000000}, {0x6DB1651D, 0x0000019D}, {0x6C6E4B03, 0x00002492}},
	{{0x00000000, 0x00000000}, {0x40C92555, 0x000000E5}, {0xDE51FAD0, 0x0000117C}},
	{{0x00000000, 0x00000000}, {0x85E47664, 0x000000E5}, {0xA6C62831, 0x00001CAA}},
	{{0x00000000, 0x00000000}, {0x2D891CFD, 0x0000019D}, {0xA0604633, 0x00001DE8}},
	{{0x00000000, 0x00000000}, {0x40AA774C, 0x000001CB}, {0x9B5CAA96, 0x00001F9A}},
	{{0x00000000, 0x00000000}, {0x0D3DE9CB, 0x00000141}, {0xD4127766, 0x00002280}},
	{{0x00000000, 0x00000000}, {0x4A692698, 0x000001F0}, {0x340D4346, 0x00001FCA}},
	{{0x00000000, 0x00000000}, {0xCD34C604, 0x00000166}, {0x6CF31046, 0x000022B0}},
	{{0x00000000, 0x00000000}, {0x5DA75315, 0x0000019D}, {0x40745A5F, 0x000022E0}},
	{{0x00000000, 0x00000000}, {0xF055FC48, 0x000001C2}, {0x3900DED3, 0x00001E18}},
	{{0x00170003, 0x00000060}, {0xDBB4306E, 0x000058AA}, {0xD643482A, 0x00000035}},
	{{0x1269EB58, 0x00000350}, {0x5CD5957A, 0x00006A4E}, {0xF758323A, 0x00000380}},
	{{0x122FEB0B, 0x00000350}, {0x7F10919A, 0x000070A9}, {0x7CD7E7B7, 0x0000025C}},
}

// sfntChecksum is tt_synth_sfnt_checksum over a table: zero when it
// can't be read.
func (f *Face) sfntChecksum(e tableEntry) uint32 {
	p, err := f.frameAt(int64(e.offset), uint64(e.length))
	if err != nil {
		return 0
	}
	var sum uint32
	i := 0
	for ; len(p)-i > 3; i += 4 {
		sum += peek32(p, i)
	}
	for shift := 24; i < len(p); i, shift = i+1, shift-8 {
		sum += uint32(p[i]) << uint(shift)
	}
	return sum
}

// checkTrickynessSfntIDs is tt_check_trickyness_sfnt_ids.
func (f *Face) checkTrickynessSfntIDs() bool {
	var numMatched [len(trickSfntIDs)]int
	var has [3]bool
	for _, e := range f.tables {
		var k int
		switch e.tag {
		case tag("cvt "):
			k = 0
		case tag("fpgm"):
			k = 1
		case tag("prep"):
			k = 2
		default:
			continue
		}
		has[k] = true
		var checksum uint32
		for j := range trickSfntIDs {
			if e.length == trickSfntIDs[j][k][1] {
				if checksum == 0 {
					checksum = f.sfntChecksum(e)
				}
				if trickSfntIDs[j][k][0] == checksum {
					numMatched[j]++
				}
				if numMatched[j] == 3 {
					return true
				}
			}
		}
	}
	for j := range trickSfntIDs {
		for k := range 3 {
			if !has[k] && trickSfntIDs[j][k][1] == 0 {
				numMatched[j]++
			}
		}
		if numMatched[j] == 3 {
			return true
		}
	}
	return false
}

// checkTrickyness is tt_check_trickyness.
func (f *Face) checkTrickyness() bool {
	if checkTrickynessFamily(f.FamilyName) {
		return true
	}
	return f.checkTrickynessSfntIDs()
}

// ttSize is the bytecode part of a TT_SizeRec.
type ttSize struct {
	metrics   sizeMetrics // the hinted metrics
	ttmetrics ttSizeMetrics
	pointSize int64
	widthp    []byte

	// bytecodeReady and cvtReady are -1 until fpgm and prep run, then
	// 0; their errors are kept.
	bytecodeReady, cvtReady int
	bytecodeErr, cvtErr     error

	exec     *ttExecContext
	twilight ttGlyphZone
	GS       ttGraphicsState
}

// sizeReset is tt_size_reset, and tt_size_request's point size.
func (f *Face) sizeReset(horiResolution, vertResolution uint32) error {
	if f.size == nil {
		// tt_size_init
		f.size = &ttSize{bytecodeReady: -1}
	}
	s := f.size
	s.ttmetrics.ppem = 0
	if f.xPpem == 0 || f.yPpem == 0 {
		return errInvalidPpem
	}
	m := sizeMetrics{uint16(f.xPpem), uint16(f.yPpem), f.xScale, f.yScale}
	if f.headFlags&8 != 0 {
		m.xScale = divFix(int64(m.xPpem)<<6, f.unitsPerEM)
		m.yScale = divFix(int64(m.yPpem)<<6, f.unitsPerEM)
	}
	if m.xPpem >= m.yPpem {
		s.ttmetrics.scale = m.xScale
		s.ttmetrics.ppem = m.xPpem
		s.ttmetrics.xRatio = 0x10000
		s.ttmetrics.yRatio = divFix(int64(m.yPpem), int64(m.xPpem))
	} else {
		s.ttmetrics.scale = m.yScale
		s.ttmetrics.ppem = m.yPpem
		s.ttmetrics.xRatio = divFix(int64(m.xPpem), int64(m.yPpem))
		s.ttmetrics.yRatio = 0x10000
	}
	s.widthp = f.deviceMetrics(uint32(m.xPpem))
	s.metrics = m
	s.cvtReady = -1

	resolution := vertResolution
	if m.xPpem > m.yPpem {
		resolution = horiResolution
	}
	if resolution == 0 {
		resolution = 72
	}
	s.pointSize = mulDiv(int64(s.ttmetrics.ppem), 64*72, int64(resolution))
	return nil
}

// initBytecode is tt_size_init_bytecode.
func (f *Face) initBytecode() error {
	s := f.size
	maxp := &f.maxp
	exec := &ttExecContext{callSize: 32, callStack: make([]ttCallRec, 32)}
	s.exec = exec

	exec.maxFDefs = uint32(maxp.maxFunctionDefs)
	exec.maxIDefs = uint32(maxp.maxInstructionDefs)
	if n := exec.maxFDefs + exec.maxIDefs; n > 0 {
		defs := make([]ttDefRecord, n)
		exec.FDefs = defs[:exec.maxFDefs]
		exec.IDefs = defs[exec.maxFDefs:]
	}

	extra := max(int64(maxp.maxStackElements/2), 128)
	exec.stackSize = int64(maxp.maxStackElements) + extra
	exec.storeSize = maxp.maxStorage
	exec.cvtSize = uint64(len(f.cvt))
	exec.stack = make([]int64, exec.stackSize)
	exec.baseStorage = make([]int64, exec.storeSize)
	exec.baseCvt = make([]int64, exec.cvtSize)

	n := int(maxp.maxTwilightPoints) + 4
	s.twilight = ttGlyphZone{
		nPoints:  uint16(n),
		org:      make([]Vector, n),
		cur:      make([]Vector, n),
		orus:     make([]Vector, n),
		tags:     make([]byte, n),
		twilight: true,
	}

	s.GS = ttDefaultGraphicsState
	s.cvtReady = -1
	s.ttmetrics.rotated = false
	s.ttmetrics.stretched = false
	return f.runFpgm()
}

// runFpgm is tt_size_run_fpgm.
func (f *Face) runFpgm() error {
	s := f.size
	exec := s.exec
	exec.loadContext(f, s)
	exec.clearCodeRange(ttCoderangeCvt)
	exec.clearCodeRange(ttCoderangeGlyph)
	var err error
	if len(f.fontProgram) > 0 {
		exec.setCodeRange(ttCoderangeFont, f.fontProgram)
		exec.pts.nPoints = 0
		exec.pts.nContours = 0
		err = exec.runContext(s)
	}
	s.bytecodeReady, s.bytecodeErr = 0, err
	if err == nil {
		exec.saveContext(s)
	}
	return err
}

// runPrep is tt_size_run_prep.
func (f *Face) runPrep() error {
	s := f.size
	exec := s.exec
	s.GS = ttDefaultGraphicsState
	for i := 0; i < int(s.twilight.nPoints); i++ {
		s.twilight.org[i] = Vector{}
		s.twilight.cur[i] = Vector{}
	}
	exec.loadContext(f, s)
	for i := range exec.storage[:exec.storeSize] {
		exec.storage[i] = 0
	}
	for i := range exec.cvt[:exec.cvtSize] {
		exec.cvt[i] = mulFix(int64(f.cvt[i]/64), s.ttmetrics.scale)
	}
	exec.clearCodeRange(ttCoderangeGlyph)
	var err error
	if len(f.cvtProgram) > 0 {
		exec.setCodeRange(ttCoderangeCvt, f.cvtProgram)
		exec.pts.nPoints = 0
		exec.pts.nContours = 0
		err = exec.runContext(s)
	}
	s.cvtReady, s.cvtErr = 0, err
	if err == nil {
		exec.saveContext(s)
	}
	return err
}

// initHinting is tt_loader_init's execution context set-up, for a load
// with hinting in FT_RENDER_MODE_NORMAL.
func (l *loader) initHinting() error {
	f := l.f
	s := f.size
	if s.bytecodeReady == 0 && s.bytecodeErr != nil {
		return s.bytecodeErr
	}
	if s.bytecodeReady < 0 {
		if err := f.initBytecode(); err != nil {
			return err
		}
	}
	exec := s.exec
	exec.backwardCompatibility = 0
	// the v40 interpreter's grayscale is false and the mode unchanged,
	// so prep runs again only for a new size

	if s.cvtReady == 0 && s.cvtErr != nil {
		return s.cvtErr
	}
	if s.cvtReady < 0 {
		if err := f.runPrep(); err != nil {
			return err
		}
	}
	exec.loadContext(f, s)

	l.hinted = true
	if s.GS.instructControl&1 != 0 {
		l.hinted = false
	}
	if s.GS.instructControl&2 != 0 {
		s.GS = ttDefaultGraphicsState
	}
	// a tricky font keeps backward compatibility off
	l.exec = exec

	if l.hinted && exec.backwardCompatibility == 0 && f.isFixedPitch == 0 {
		l.widthp = s.widthp
	}
	return nil
}
