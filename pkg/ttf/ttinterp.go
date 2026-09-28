package ttf

import "errors"

// FreeType's TrueType bytecode interpreter (ttinterp.c), version 40
// with minimal subpixel hinting, as the TrueType driver runs it for
// `tricky' fonts, which need hinting to render.

var (
	errBadArgument         = errors.New("ttf: bad argument")
	errInvalidCodeRange    = errors.New("ttf: invalid code range")
	errCodeOverflow        = errors.New("ttf: code overflow")
	errInvalidReference    = errors.New("ttf: invalid reference")
	errDivideByZeroIns     = errors.New("ttf: division by zero")
	errDebugOpCode         = errors.New("ttf: debug opcode")
	errExecutionTooLong    = errors.New("ttf: execution too long")
	errDEFInGlyfBytecode   = errors.New("ttf: FDEF or IDEF in glyf bytecode")
	errTooManyFunctionDefs = errors.New("ttf: too many function definitions")
	errNestedDEFS          = errors.New("ttf: nested FDEF or IDEF")
	errENDFInExecStream    = errors.New("ttf: ENDF in exec stream")
	errTTStackOverflow     = errors.New("ttf: stack overflow")
	errTooManyInsDefs      = errors.New("ttf: too many instruction definitions")
	errTooFewArguments     = errors.New("ttf: too few arguments")
	errInvalidOpcode       = errors.New("ttf: invalid opcode")
)

const (
	ttRoundOff           = 5
	ttRoundToHalfGrid    = 0
	ttRoundToGrid        = 1
	ttRoundToDoubleGrid  = 2
	ttRoundUpToGrid      = 4
	ttRoundDownToGrid    = 3
	ttRoundSuper         = 6
	ttRoundSuper45       = 7
	ttCoderangeFont      = 1
	ttCoderangeCvt       = 2
	ttCoderangeGlyph     = 3
	ttInterpreterVersion = 40
	ttMaxRunnableOpcodes = 1000000

	curveTagHasScanmode = 0x04
	curveTagTouchX      = 0x08
	curveTagTouchY      = 0x10
	curveTagTouchBoth   = curveTagTouchX | curveTagTouchY

	renderModeNormal = 0
	renderModeMono   = 2
	renderModeLCD    = 3
	renderModeLCDV   = 4
)

type ttUnitVector struct{ x, y int16 }

// ttGraphicsState is a TT_GraphicsState.
type ttGraphicsState struct {
	rp0, rp1, rp2                      uint16
	gep0, gep1, gep2                   uint16
	dualVector, projVector, freeVector ttUnitVector
	loop                               int64
	roundState                         int
	compensation                       [4]int64
	minimumDistance                    int64
	controlValueCutin                  int64
	singleWidthCutin                   int64
	singleWidthValue                   int64
	deltaBase, deltaShift              uint16
	autoFlip                           bool
	instructControl                    byte
	scanControl                        bool
	scanType                           int32
}

var ttDefaultGraphicsState = ttGraphicsState{
	0, 0, 0, 1, 1, 1,
	ttUnitVector{0x4000, 0}, ttUnitVector{0x4000, 0}, ttUnitVector{0x4000, 0},
	1, 1, [4]int64{0, 0, 0, 0},
	64, 68, 0, 0, 9, 3,
	true, 0, false, 0,
}

// ttGlyphZone is a TT_GlyphZoneRec; copies share the arrays, as C's do.
type ttGlyphZone struct {
	nPoints    uint16
	nContours  uint16
	org, cur   []Vector
	orus       []Vector
	tags       []byte
	contours   []uint16
	firstPoint uint16
	twilight   bool // the twilight zone (C compares the cur pointers)
}

// ttSizeMetrics is a TT_Size_Metrics.
type ttSizeMetrics struct {
	xRatio, yRatio     int64
	ratio              int64
	scale              int64
	ppem               uint16
	rotated, stretched bool
}

// sizeMetrics is the part of an FT_Size_Metrics the interpreter reads.
type sizeMetrics struct {
	xPpem, yPpem   uint16
	xScale, yScale int64
}

type ttDefRecord struct {
	rng    int
	start  int64
	end    int64
	opc    uint32
	active bool
}

type ttCallRec struct {
	callerRange int
	callerIP    int64
	curCount    int64
	def         *ttDefRecord
}

type ttCodeRange struct {
	base []byte
	size int64
}

// ttExecContext is a TT_ExecContextRec.
type ttExecContext struct {
	face *Face
	size *ttSize

	err       error
	top       int64
	stackSize int64
	stack     []int64
	args      int64
	newTop    int64

	zp0, zp1, zp2, pts, twilight ttGlyphZone

	pointSize int64
	metrics   sizeMetrics
	ttMetrics ttSizeMetrics
	GS        ttGraphicsState

	iniRange, curRange int
	code               []byte
	IP                 int64
	codeSize           int64
	opcode             byte
	length             int64

	cvtSize  uint64
	cvt      []int64
	baseCvt  []int64 // the size's CVT
	glyfCvt  []int64
	glyphIns []byte

	numFDefs, maxFDefs uint32
	FDefs              []ttDefRecord
	numIDefs, maxIDefs uint32
	IDefs              []ttDefRecord
	maxFunc, maxIns    uint32

	callTop, callSize int
	callStack         []ttCallRec

	codeRangeTable [3]ttCodeRange

	storeSize   uint16
	storage     []int64
	baseStorage []int64
	glyfStorage []int64

	period, phase, threshold int64

	isComposite     bool
	pedanticHinting bool

	funcRound             func(exc *ttExecContext, distance, compensation int64) int64
	moveVector            Vector
	funcProject           func(exc *ttExecContext, dx, dy int64) int64
	funcDualproj          func(exc *ttExecContext, dx, dy int64) int64
	funcMove              func(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64)
	funcMoveOrig          func(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64)
	funcCurPpem           func(exc *ttExecContext) int64
	funcReadCVT           func(exc *ttExecContext, idx uint64) int64
	funcWriteCVT          func(exc *ttExecContext, idx uint64, value int64)
	funcMoveCVT           func(exc *ttExecContext, idx uint64, value int64)
	grayscale             bool
	backwardCompatibility int
	mode                  int

	loopcallCounter, loopcallCounterMax uint64
	negJumpCounter, negJumpCounterMax   uint64
}

// setCodeRange is TT_Set_CodeRange.
func (exc *ttExecContext) setCodeRange(rng int, base []byte) {
	exc.codeRangeTable[rng-1] = ttCodeRange{base, int64(len(base))}
	exc.code = base
	exc.codeSize = int64(len(base))
	exc.IP = 0
	exc.curRange = rng
	exc.iniRange = rng
}

// clearCodeRange is TT_Clear_CodeRange.
func (exc *ttExecContext) clearCodeRange(rng int) {
	exc.codeRangeTable[rng-1] = ttCodeRange{}
}

// loadContext is TT_Load_Context.
func (exc *ttExecContext) loadContext(face *Face, size *ttSize) {
	exc.face = face
	exc.size = size
	exc.storage = exc.baseStorage
	exc.cvt = exc.baseCvt
	exc.glyphIns = nil
	exc.pointSize = size.pointSize
	exc.ttMetrics = size.ttmetrics
	exc.metrics = size.metrics
	exc.twilight = size.twilight
}

// saveContext is TT_Save_Context.
func (exc *ttExecContext) saveContext(size *ttSize) {
	size.GS.minimumDistance = exc.GS.minimumDistance
	size.GS.controlValueCutin = exc.GS.controlValueCutin
	size.GS.singleWidthCutin = exc.GS.singleWidthCutin
	size.GS.singleWidthValue = exc.GS.singleWidthValue
	size.GS.deltaBase = exc.GS.deltaBase
	size.GS.deltaShift = exc.GS.deltaShift
	size.GS.autoFlip = exc.GS.autoFlip
	size.GS.instructControl = exc.GS.instructControl
	size.GS.scanControl = exc.GS.scanControl
	size.GS.scanType = exc.GS.scanType
}

func pack(x, y byte) byte { return x<<4 | y }

// popPushCount is Pop_Push_Count.
var popPushCount = [256]byte{
	/* 0x00 */ pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(2, 0), pack(2, 0),
	pack(2, 0), pack(2, 0), pack(2, 0), pack(2, 0), pack(0, 2), pack(0, 2), pack(0, 0), pack(5, 0),
	/* 0x10 */ pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0),
	pack(0, 0), pack(0, 0), pack(1, 0), pack(0, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0),
	/* 0x20 */ pack(1, 2), pack(1, 0), pack(0, 0), pack(2, 2), pack(0, 1), pack(1, 1), pack(1, 0), pack(2, 0),
	pack(0, 0), pack(1, 0), pack(2, 0), pack(1, 0), pack(1, 0), pack(0, 0), pack(1, 0), pack(1, 0),
	/* 0x30 */ pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0),
	pack(1, 0), pack(0, 0), pack(2, 0), pack(2, 0), pack(0, 0), pack(0, 0), pack(2, 0), pack(2, 0),
	/* 0x40 */ pack(0, 0), pack(0, 0), pack(2, 0), pack(1, 1), pack(2, 0), pack(1, 1), pack(1, 1), pack(1, 1),
	pack(2, 0), pack(2, 1), pack(2, 1), pack(0, 1), pack(0, 1), pack(0, 0), pack(0, 0), pack(1, 0),
	/* 0x50 */ pack(2, 1), pack(2, 1), pack(2, 1), pack(2, 1), pack(2, 1), pack(2, 1), pack(1, 1), pack(1, 1),
	pack(1, 0), pack(0, 0), pack(2, 1), pack(2, 1), pack(1, 1), pack(1, 0), pack(1, 0), pack(1, 0),
	/* 0x60 */ pack(2, 1), pack(2, 1), pack(2, 1), pack(2, 1), pack(1, 1), pack(1, 1), pack(1, 1), pack(1, 1),
	pack(1, 1), pack(1, 1), pack(1, 1), pack(1, 1), pack(1, 1), pack(1, 1), pack(1, 1), pack(1, 1),
	/* 0x70 */ pack(2, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0), pack(1, 0),
	pack(2, 0), pack(2, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(1, 0), pack(1, 0),
	/* 0x80 */ pack(0, 0), pack(2, 0), pack(2, 0), pack(0, 0), pack(0, 0), pack(1, 0), pack(2, 0), pack(2, 0),
	pack(1, 1), pack(1, 0), pack(3, 3), pack(2, 1), pack(2, 1), pack(1, 0), pack(2, 0), pack(0, 0),
	/* 0x90 */ pack(0, 0), pack(0, 0), pack(0, 1), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0),
	pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0), pack(0, 0),
	/* 0xA0 */ 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	/* 0xB0 */ pack(0, 1), pack(0, 2), pack(0, 3), pack(0, 4), pack(0, 5), pack(0, 6), pack(0, 7), pack(0, 8),
	pack(0, 1), pack(0, 2), pack(0, 3), pack(0, 4), pack(0, 5), pack(0, 6), pack(0, 7), pack(0, 8),
	/* 0xC0 */ 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10,
	/* 0xD0 */ 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10,
	/* 0xE0 */ 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	/* 0xF0 */ 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
}

// opcodeLength is opcode_length.
var opcodeLength = [256]int8{
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,

	-1, -2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,

	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	2, 3, 4, 5, 6, 7, 8, 9, 3, 5, 7, 9, 11, 13, 15, 17,

	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
}

// ttMulFix14 is TT_MulFix14 (64-bit).
func ttMulFix14(a int64, b int16) int64 {
	ab := a * int64(b)
	ab += 0x2000 + (ab >> 63)
	return ab >> 14
}

// ttDotFix14 is TT_DotFix14 (64-bit).
func ttDotFix14(ax, ay int64, bx, by int16) int64 {
	c := ax*int64(bx) + ay*int64(by)
	c += 0x2000 + (c >> 63)
	return c >> 14
}

func pixFloor(x int64) int64     { return x &^ 63 }
func pixRoundLong(x int64) int64 { return pixFloor(x + 32) }
func pixCeilLong(x int64) int64  { return pixFloor(x + 63) }
func padRoundLong(x, n int64) int64 {
	return (x + n/2) &^ (n - 1)
}

// mulDivNoRound is FT_MulDiv_No_Round.
func mulDivNoRound(a_, b_, c_ int64) int64 {
	s := 1
	a, s := moveSign(a_, s)
	b, s := moveSign(b_, s)
	c, s := moveSign(c_, s)
	var d uint64 = 0x7FFFFFFF
	if c > 0 {
		d = a * b / c
	}
	if s < 0 {
		return -int64(d)
	}
	return int64(d)
}

// vectorNormLen is FT_Vector_NormLen.
func vectorNormLen(v *Vector) {
	x_ := int32(v.X)
	y_ := int32(v.Y)
	sx, sy := int32(1), int32(1)
	x := uint32(x_)
	if x_ < 0 {
		x = 0 - x
		sx = -sx
	}
	y := uint32(y_)
	if y_ < 0 {
		y = 0 - y
		sy = -sy
	}
	if x == 0 {
		if y > 0 {
			v.Y = int64(sy) * 0x10000
		}
		return
	} else if y == 0 {
		if x > 0 {
			v.X = int64(sx) * 0x10000
		}
		return
	}
	var l uint32
	if x > y {
		l = x + y>>1
	} else {
		l = y + x>>1
	}
	shift := 31 - msb32(l)
	var ge int
	if l >= 0xAAAAAAAA>>uint(shift) {
		ge = 1
	}
	shift -= 15 + ge
	if shift > 0 {
		x <<= uint(shift)
		y <<= uint(shift)
		if x > y {
			l = x + y>>1
		} else {
			l = y + x>>1
		}
	} else {
		x >>= uint(-shift)
		y >>= uint(-shift)
		l >>= uint(-shift)
	}
	b := int32(0x10000) - int32(l)
	x_ = int32(x)
	y_ = int32(y)
	var u, w uint32
	for {
		u = uint32(x_ + (x_ * b >> 16))
		w = uint32(y_ + (y_ * b >> 16))
		z := -int32(u*u+w*w) / 0x200
		z = z * ((0x10000 + b) >> 8) / 0x10000
		b += z
		if z <= 0 {
			break
		}
	}
	if sx < 0 {
		v.X = -int64(u)
	} else {
		v.X = int64(u)
	}
	if sy < 0 {
		v.Y = -int64(w)
	} else {
		v.Y = int64(w)
	}
}

// currentRatio is Current_Ratio.
func (exc *ttExecContext) currentRatio() int64 {
	if exc.ttMetrics.ratio == 0 {
		if exc.GS.projVector.y == 0 {
			exc.ttMetrics.ratio = exc.ttMetrics.xRatio
		} else if exc.GS.projVector.x == 0 {
			exc.ttMetrics.ratio = exc.ttMetrics.yRatio
		} else {
			x := ttMulFix14(exc.ttMetrics.xRatio, exc.GS.projVector.x)
			y := ttMulFix14(exc.ttMetrics.yRatio, exc.GS.projVector.y)
			exc.ttMetrics.ratio = hypot(x, y)
		}
	}
	return exc.ttMetrics.ratio
}

func currentPpem(exc *ttExecContext) int64 { return int64(exc.ttMetrics.ppem) }

func currentPpemStretched(exc *ttExecContext) int64 {
	return mulFix(int64(exc.ttMetrics.ppem), exc.currentRatio())
}

func readCVT(exc *ttExecContext, idx uint64) int64 { return exc.cvt[idx] }

func readCVTStretched(exc *ttExecContext, idx uint64) int64 {
	return mulFix(exc.cvt[idx], exc.currentRatio())
}

// modifyCVTCheck is Modify_CVT_Check: glyph programs write a copy.
func (exc *ttExecContext) modifyCVTCheck() {
	if exc.iniRange == ttCoderangeGlyph && !sameSlice(exc.cvt, exc.glyfCvt) {
		exc.glyfCvt = append(exc.glyfCvt[:0], exc.cvt[:exc.cvtSize]...)
		exc.cvt = exc.glyfCvt
	}
}

// sameSlice reports whether a and b share their first element.
func sameSlice(a, b []int64) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == 0 && len(b) == 0 && a != nil && b != nil
	}
	return &a[0] == &b[0]
}

func writeCVT(exc *ttExecContext, idx uint64, value int64) {
	exc.modifyCVTCheck()
	if exc.err != nil {
		return
	}
	exc.cvt[idx] = value
}

func writeCVTStretched(exc *ttExecContext, idx uint64, value int64) {
	exc.modifyCVTCheck()
	if exc.err != nil {
		return
	}
	exc.cvt[idx] = divFix(value, exc.currentRatio())
}

func moveCVT(exc *ttExecContext, idx uint64, value int64) {
	exc.modifyCVTCheck()
	if exc.err != nil {
		return
	}
	exc.cvt[idx] += value
}

func moveCVTStretched(exc *ttExecContext, idx uint64, value int64) {
	exc.modifyCVTCheck()
	if exc.err != nil {
		return
	}
	exc.cvt[idx] += divFix(value, exc.currentRatio())
}

// gotoCodeRange is Ins_Goto_CodeRange; it reports failure.
func (exc *ttExecContext) gotoCodeRange(aRange int, aIP int64) bool {
	if aRange < 1 || aRange > 3 {
		exc.err = errBadArgument
		return true
	}
	rng := &exc.codeRangeTable[aRange-1]
	if rng.base == nil {
		exc.err = errInvalidCodeRange
		return true
	}
	if aIP > rng.size {
		exc.err = errCodeOverflow
		return true
	}
	exc.code = rng.base
	exc.codeSize = rng.size
	exc.IP = aIP
	exc.length = 0
	exc.curRange = aRange
	return false
}

// directMove is Direct_Move.
func directMove(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64) {
	v := exc.moveVector.X
	if v != 0 {
		if exc.backwardCompatibility == 0 {
			zone.cur[point].X += mulFix(distance, v)
		}
		zone.tags[point] |= curveTagTouchX
	}
	v = exc.moveVector.Y
	if v != 0 {
		if exc.backwardCompatibility != 0x7 {
			zone.cur[point].Y += mulFix(distance, v)
		}
		zone.tags[point] |= curveTagTouchY
	}
}

// directMoveOrig is Direct_Move_Orig.
func directMoveOrig(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64) {
	if v := exc.moveVector.X; v != 0 {
		zone.org[point].X += mulFix(distance, v)
	}
	if v := exc.moveVector.Y; v != 0 {
		zone.org[point].Y += mulFix(distance, v)
	}
}

func directMoveX(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64) {
	if exc.backwardCompatibility == 0 {
		zone.cur[point].X += distance
	}
	zone.tags[point] |= curveTagTouchX
}

func directMoveY(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64) {
	if exc.backwardCompatibility != 0x7 {
		zone.cur[point].Y += distance
	}
	zone.tags[point] |= curveTagTouchY
}

func directMoveOrigX(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64) {
	zone.org[point].X += distance
}

func directMoveOrigY(exc *ttExecContext, zone *ttGlyphZone, point uint16, distance int64) {
	zone.org[point].Y += distance
}

// The rounding functions.

func roundNone(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = distance + compensation
		if val < 0 {
			val = 0
		}
	} else {
		val = distance - compensation
		if val > 0 {
			val = 0
		}
	}
	return val
}

func roundToGrid(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = pixRoundLong(distance + compensation)
		if val < 0 {
			val = 0
		}
	} else {
		val = -pixRoundLong(compensation - distance)
		if val > 0 {
			val = 0
		}
	}
	return val
}

func roundToHalfGrid(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = pixFloor(distance+compensation) + 32
		if val < 0 {
			val = 32
		}
	} else {
		val = -(pixFloor(compensation-distance) + 32)
		if val > 0 {
			val = -32
		}
	}
	return val
}

func roundDownToGrid(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = pixFloor(distance + compensation)
		if val < 0 {
			val = 0
		}
	} else {
		val = -pixFloor(compensation - distance)
		if val > 0 {
			val = 0
		}
	}
	return val
}

func roundUpToGrid(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = pixCeilLong(distance + compensation)
		if val < 0 {
			val = 0
		}
	} else {
		val = -pixCeilLong(compensation - distance)
		if val > 0 {
			val = 0
		}
	}
	return val
}

func roundToDoubleGrid(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = padRoundLong(distance+compensation, 32)
		if val < 0 {
			val = 0
		}
	} else {
		val = -padRoundLong(compensation-distance, 32)
		if val > 0 {
			val = 0
		}
	}
	return val
}

func roundSuper(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = (distance + (exc.threshold - exc.phase + compensation)) & -exc.period
		val += exc.phase
		if val < 0 {
			val = exc.phase
		}
	} else {
		val = -((exc.threshold - exc.phase + compensation - distance) & -exc.period)
		val -= exc.phase
		if val > 0 {
			val = -exc.phase
		}
	}
	return val
}

func roundSuper45(exc *ttExecContext, distance, compensation int64) int64 {
	var val int64
	if distance >= 0 {
		val = ((distance + (exc.threshold - exc.phase + compensation)) / exc.period) * exc.period
		val += exc.phase
		if val < 0 {
			val = exc.phase
		}
	} else {
		val = -(((exc.threshold - exc.phase + compensation - distance) / exc.period) * exc.period)
		val -= exc.phase
		if val > 0 {
			val = -exc.phase
		}
	}
	return val
}

// setSuperRound is SetSuperRound.
func (exc *ttExecContext) setSuperRound(gridPeriod int64, selector int64) {
	switch int32(selector & 0xC0) {
	case 0:
		exc.period = gridPeriod / 2
	case 0x40:
		exc.period = gridPeriod
	case 0x80:
		exc.period = gridPeriod * 2
	case 0xC0:
		exc.period = gridPeriod
	}
	switch int32(selector & 0x30) {
	case 0:
		exc.phase = 0
	case 0x10:
		exc.phase = exc.period / 4
	case 0x20:
		exc.phase = exc.period / 2
	case 0x30:
		exc.phase = exc.period * 3 / 4
	}
	if selector&0x0F == 0 {
		exc.threshold = exc.period - 1
	} else {
		exc.threshold = (int64(int32(selector&0x0F)) - 4) * exc.period / 8
	}
	exc.period >>= 8
	exc.phase >>= 8
	exc.threshold >>= 8
}

func project(exc *ttExecContext, dx, dy int64) int64 {
	return ttDotFix14(dx, dy, exc.GS.projVector.x, exc.GS.projVector.y)
}

func dualProject(exc *ttExecContext, dx, dy int64) int64 {
	return ttDotFix14(dx, dy, exc.GS.dualVector.x, exc.GS.dualVector.y)
}

func projectX(exc *ttExecContext, dx, dy int64) int64 { return dx }
func projectY(exc *ttExecContext, dx, dy int64) int64 { return dy }

func (exc *ttExecContext) proj(v1, v2 Vector) int64 {
	return exc.funcProject(exc, v1.X-v2.X, v1.Y-v2.Y)
}

func (exc *ttExecContext) dualProj(v1, v2 Vector) int64 {
	return exc.funcDualproj(exc, v1.X-v2.X, v1.Y-v2.Y)
}

// computeFuncs is Compute_Funcs.
func (exc *ttExecContext) computeFuncs() {
	fDotP := (int64(exc.GS.projVector.x)*int64(exc.GS.freeVector.x) +
		int64(exc.GS.projVector.y)*int64(exc.GS.freeVector.y) + 0x2000) >> 14
	if fDotP >= 0x3FFE {
		exc.moveVector.X = int64(exc.GS.freeVector.x) * 4
		exc.moveVector.Y = int64(exc.GS.freeVector.y) * 4
	} else if -0x400 < fDotP && fDotP < 0x400 {
		exc.moveVector = Vector{}
	} else {
		exc.moveVector.X = int64(exc.GS.freeVector.x) * 0x10000 / fDotP
		exc.moveVector.Y = int64(exc.GS.freeVector.y) * 0x10000 / fDotP
	}

	if fDotP >= 0x3FFE && exc.GS.freeVector.x == 0x4000 {
		exc.funcMove, exc.funcMoveOrig = directMoveX, directMoveOrigX
	} else if fDotP >= 0x3FFE && exc.GS.freeVector.y == 0x4000 {
		exc.funcMove, exc.funcMoveOrig = directMoveY, directMoveOrigY
	} else {
		exc.funcMove, exc.funcMoveOrig = directMove, directMoveOrig
	}

	switch {
	case exc.GS.projVector.x == 0x4000:
		exc.funcProject = projectX
	case exc.GS.projVector.y == 0x4000:
		exc.funcProject = projectY
	default:
		exc.funcProject = project
	}
	switch {
	case exc.GS.dualVector.x == 0x4000:
		exc.funcDualproj = projectX
	case exc.GS.dualVector.y == 0x4000:
		exc.funcDualproj = projectY
	default:
		exc.funcDualproj = dualProject
	}
	exc.ttMetrics.ratio = 0
}

// normalize is Normalize.
func normalize(vx, vy int64, r *ttUnitVector) {
	if vx == 0 && vy == 0 {
		return
	}
	v := Vector{vx, vy}
	vectorNormLen(&v)
	r.x = int16(v.X / 4)
	r.y = int16(v.Y / 4)
}

func bounds(x, n int64) bool  { return uint32(x) >= uint32(n) }
func boundsL(x, n int64) bool { return uint64(x) >= uint64(n) }

func b2l(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// skipCode is SkipCode; it reports failure.
func (exc *ttExecContext) skipCode() bool {
	exc.IP += exc.length
	if exc.IP < exc.codeSize {
		exc.opcode = exc.code[exc.IP]
		exc.length = int64(opcodeLength[exc.opcode])
		if exc.length < 0 {
			if exc.IP+1 >= exc.codeSize {
				exc.err = errCodeOverflow
				return true
			}
			exc.length = 2 - exc.length*int64(exc.code[exc.IP+1])
		}
		return false
	}
	exc.err = errCodeOverflow
	return true
}

// The instructions. args is the index in the stack of the first
// argument.

func (exc *ttExecContext) insIF(args int64) {
	if exc.stack[args] != 0 {
		return
	}
	nIfs := 1
	out := false
	for !out {
		if exc.skipCode() {
			return
		}
		switch exc.opcode {
		case 0x58:
			nIfs++
		case 0x1B:
			out = nIfs == 1
		case 0x59:
			nIfs--
			out = nIfs == 0
		}
	}
}

func (exc *ttExecContext) insELSE() {
	nIfs := 1
	for nIfs != 0 {
		if exc.skipCode() {
			return
		}
		switch exc.opcode {
		case 0x58:
			nIfs++
		case 0x59:
			nIfs--
		}
	}
}

func (exc *ttExecContext) insJMPR(args int64) {
	a := exc.stack[args]
	if a == 0 && exc.args == 0 {
		exc.err = errBadArgument
		return
	}
	exc.IP += a
	if exc.IP < 0 || (exc.callTop > 0 && exc.IP > exc.callStack[exc.callTop-1].def.end) {
		exc.err = errBadArgument
		return
	}
	exc.length = 0
	if a < 0 {
		exc.negJumpCounter++
		if exc.negJumpCounter > exc.negJumpCounterMax {
			exc.err = errExecutionTooLong
		}
	}
}

func (exc *ttExecContext) insFDEF(args int64) {
	if exc.iniRange == ttCoderangeGlyph {
		exc.err = errDEFInGlyfBytecode
		return
	}
	n := uint64(exc.stack[args])
	i := uint32(0)
	for ; i < exc.numFDefs; i++ {
		if uint64(exc.FDefs[i].opc) == n {
			break
		}
	}
	if i == exc.numFDefs {
		if exc.numFDefs >= exc.maxFDefs {
			exc.err = errTooManyFunctionDefs
			return
		}
		exc.numFDefs++
	}
	if n > 0xFFFF {
		exc.err = errTooManyFunctionDefs
		return
	}
	rec := &exc.FDefs[i]
	rec.rng = exc.curRange
	rec.opc = uint32(uint16(n))
	rec.start = exc.IP + 1
	rec.active = true
	if uint32(n) > exc.maxFunc {
		exc.maxFunc = uint32(uint16(n))
	}
	for !exc.skipCode() {
		switch exc.opcode {
		case 0x89, 0x2C:
			exc.err = errNestedDEFS
			return
		case 0x2D:
			rec.end = exc.IP
			return
		}
	}
}

func (exc *ttExecContext) insENDF() {
	if exc.callTop <= 0 {
		exc.err = errENDFInExecStream
		return
	}
	exc.callTop--
	pRec := &exc.callStack[exc.callTop]
	pRec.curCount--
	if pRec.curCount > 0 {
		exc.callTop++
		exc.IP = pRec.def.start
		exc.length = 0
	} else {
		exc.gotoCodeRange(pRec.callerRange, pRec.callerIP)
	}
}

// findFDef is the function lookup of Ins_CALL and Ins_LOOPCALL.
func (exc *ttExecContext) findFDef(f uint64) *ttDefRecord {
	if boundsL(int64(f), int64(exc.maxFunc)+1) {
		return nil
	}
	if exc.FDefs == nil {
		return nil
	}
	var def *ttDefRecord
	if f < uint64(len(exc.FDefs)) {
		def = &exc.FDefs[f]
	}
	if exc.maxFunc+1 != exc.numFDefs || def == nil || uint64(def.opc) != f {
		def = nil
		for i := uint32(0); i < exc.numFDefs; i++ {
			if uint64(exc.FDefs[i].opc) == f {
				def = &exc.FDefs[i]
				break
			}
		}
		if def == nil {
			return nil
		}
	}
	if !def.active {
		return nil
	}
	return def
}

func (exc *ttExecContext) insCALL(args int64) {
	def := exc.findFDef(uint64(exc.stack[args]))
	if def == nil {
		exc.err = errInvalidReference
		return
	}
	if exc.callTop >= exc.callSize {
		exc.err = errTTStackOverflow
		return
	}
	exc.callStack[exc.callTop] = ttCallRec{exc.curRange, exc.IP + 1, 1, def}
	exc.callTop++
	exc.gotoCodeRange(def.rng, def.start)
}

func (exc *ttExecContext) insLOOPCALL(args int64) {
	def := exc.findFDef(uint64(exc.stack[args+1]))
	if def == nil {
		exc.err = errInvalidReference
		return
	}
	if exc.callTop >= exc.callSize {
		exc.err = errTTStackOverflow
		return
	}
	if n := exc.stack[args]; n > 0 {
		exc.callStack[exc.callTop] = ttCallRec{exc.curRange, exc.IP + 1, int64(int32(n)), def}
		exc.callTop++
		exc.gotoCodeRange(def.rng, def.start)
		exc.loopcallCounter += uint64(n)
		if exc.loopcallCounter > exc.loopcallCounterMax {
			exc.err = errExecutionTooLong
		}
	}
}

func (exc *ttExecContext) insIDEF(args int64) {
	if exc.iniRange == ttCoderangeGlyph {
		exc.err = errDEFInGlyfBytecode
		return
	}
	a := exc.stack[args]
	i := uint32(0)
	for ; i < exc.numIDefs; i++ {
		if uint64(exc.IDefs[i].opc) == uint64(a) {
			break
		}
	}
	if i == exc.numIDefs {
		if exc.numIDefs >= exc.maxIDefs {
			exc.err = errTooManyInsDefs
			return
		}
		exc.numIDefs++
	}
	if 0 > a || a > 0xFF {
		exc.err = errTooManyInsDefs
		return
	}
	def := &exc.IDefs[i]
	def.opc = uint32(byte(a))
	def.start = exc.IP + 1
	def.rng = exc.curRange
	def.active = true
	if uint64(a) > uint64(exc.maxIns) {
		exc.maxIns = uint32(byte(a))
	}
	for !exc.skipCode() {
		switch exc.opcode {
		case 0x89, 0x2C:
			exc.err = errNestedDEFS
			return
		case 0x2D:
			def.end = exc.IP
			return
		}
	}
}

func (exc *ttExecContext) insNPUSHB(args int64) {
	IP := exc.IP
	IP++
	if IP >= exc.codeSize {
		exc.err = errCodeOverflow
		return
	}
	L := int64(exc.code[IP])
	if IP+L >= exc.codeSize {
		exc.err = errCodeOverflow
		return
	}
	if bounds(L, exc.stackSize+1-exc.top) {
		exc.err = errTTStackOverflow
		return
	}
	for K := int64(0); K < L; K++ {
		IP++
		exc.stack[args+K] = int64(exc.code[IP])
	}
	exc.newTop += L
	exc.IP = IP
}

func (exc *ttExecContext) insNPUSHW(args int64) {
	IP := exc.IP
	IP++
	if IP >= exc.codeSize {
		exc.err = errCodeOverflow
		return
	}
	L := int64(exc.code[IP])
	if IP+2*L >= exc.codeSize {
		exc.err = errCodeOverflow
		return
	}
	if bounds(L, exc.stackSize+1-exc.top) {
		exc.err = errTTStackOverflow
		return
	}
	for K := int64(0); K < L; K, IP = K+1, IP+2 {
		exc.stack[args+K] = int64(int16(uint16(exc.code[IP+1])<<8 | uint16(exc.code[IP+2])))
	}
	exc.newTop += L
	exc.IP = IP
}

func (exc *ttExecContext) insPUSHB(args int64) {
	IP := exc.IP
	L := int64(exc.opcode) - 0xB0 + 1
	if IP+L >= exc.codeSize {
		exc.err = errCodeOverflow
		return
	}
	if bounds(L, exc.stackSize+1-exc.top) {
		exc.err = errTTStackOverflow
		return
	}
	for K := int64(0); K < L; K++ {
		IP++
		exc.stack[args+K] = int64(exc.code[IP])
	}
	exc.IP = IP
}

func (exc *ttExecContext) insPUSHW(args int64) {
	IP := exc.IP
	L := int64(exc.opcode) - 0xB8 + 1
	if IP+2*L >= exc.codeSize {
		exc.err = errCodeOverflow
		return
	}
	if bounds(L, exc.stackSize+1-exc.top) {
		exc.err = errTTStackOverflow
		return
	}
	for K := int64(0); K < L; K, IP = K+1, IP+2 {
		exc.stack[args+K] = int64(int16(uint16(exc.code[IP+1])<<8 | uint16(exc.code[IP+2])))
	}
	exc.IP = IP
}

// insSxVTL is Ins_SxVTL; it reports failure.
func (exc *ttExecContext) insSxVTL(aIdx1, aIdx2 uint16, vec *ttUnitVector) bool {
	opcode := exc.opcode
	if bounds(int64(aIdx1), int64(exc.zp2.nPoints)) || bounds(int64(aIdx2), int64(exc.zp1.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return true
	}
	p1 := exc.zp1.cur[aIdx2]
	p2 := exc.zp2.cur[aIdx1]
	A := p1.X - p2.X
	B := p1.Y - p2.Y
	if A == 0 && B == 0 {
		A = 0x4000
		opcode = 0
	}
	if opcode&1 != 0 {
		C := B
		B = A
		A = -C
	}
	normalize(A, B, vec)
	return false
}

func (exc *ttExecContext) insSxyTCA() {
	opcode := exc.opcode
	AA := int16(opcode&1) << 14
	BB := AA ^ 0x4000
	if opcode < 4 {
		exc.GS.projVector = ttUnitVector{AA, BB}
		exc.GS.dualVector = ttUnitVector{AA, BB}
	}
	if opcode&2 == 0 {
		exc.GS.freeVector = ttUnitVector{AA, BB}
	}
	exc.computeFuncs()
}

func (exc *ttExecContext) zoneFor(n int64) (ttGlyphZone, bool) {
	switch int32(n) {
	case 0:
		return exc.twilight, true
	case 1:
		return exc.pts, true
	}
	if exc.pedanticHinting {
		exc.err = errInvalidReference
	}
	return ttGlyphZone{}, false
}

func (exc *ttExecContext) insINSTCTRL(args int64) {
	K := uint64(exc.stack[args+1])
	L := uint64(exc.stack[args])
	if K < 1 || K > 3 {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	Kf := uint64(1) << (K - 1)
	if L != 0 && L != Kf {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	if exc.iniRange == ttCoderangeCvt {
		exc.GS.instructControl &^= byte(Kf)
		exc.GS.instructControl |= byte(L)
	} else if exc.iniRange == ttCoderangeGlyph && K == 3 {
		exc.backwardCompatibility = int((L & 4) ^ 4)
	} else if exc.pedanticHinting {
		exc.err = errInvalidReference
	}
}

func (exc *ttExecContext) insSCANCTRL(args int64) {
	a0 := exc.stack[args]
	A := int32(a0 & 0xFF)
	if A == 0xFF {
		exc.GS.scanControl = true
		return
	} else if A == 0 {
		exc.GS.scanControl = false
		return
	}
	if a0&0x100 != 0 && int32(exc.ttMetrics.ppem) <= A {
		exc.GS.scanControl = true
	}
	if a0&0x200 != 0 && exc.ttMetrics.rotated {
		exc.GS.scanControl = true
	}
	if a0&0x400 != 0 && exc.ttMetrics.stretched {
		exc.GS.scanControl = true
	}
	if a0&0x800 != 0 && int32(exc.ttMetrics.ppem) > A {
		exc.GS.scanControl = false
	}
	if a0&0x1000 != 0 && exc.ttMetrics.rotated {
		exc.GS.scanControl = false
	}
	if a0&0x2000 != 0 && exc.ttMetrics.stretched {
		exc.GS.scanControl = false
	}
}

func (exc *ttExecContext) insFLIPPT(args int64) {
	loop := exc.GS.loop
	if exc.newTop < loop {
		if exc.pedanticHinting {
			exc.err = errTooFewArguments
		}
		exc.GS.loop = 1
		return
	}
	exc.newTop -= loop
	if exc.backwardCompatibility == 0x7 {
		exc.GS.loop = 1
		return
	}
	for ; loop > 0; loop-- {
		args--
		point := uint16(exc.stack[args])
		if bounds(int64(point), int64(exc.pts.nPoints)) {
			if exc.pedanticHinting {
				exc.err = errInvalidReference
				return
			}
		} else {
			exc.pts.tags[point] ^= curveTagOn
		}
	}
	exc.GS.loop = 1
}

func (exc *ttExecContext) insFLIPRG(args int64, on bool) {
	if exc.backwardCompatibility == 0x7 {
		return
	}
	K := uint16(exc.stack[args+1])
	L := uint16(exc.stack[args])
	if bounds(int64(K), int64(exc.pts.nPoints)) || bounds(int64(L), int64(exc.pts.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	for I := L; I <= K; I++ {
		if on {
			exc.pts.tags[I] |= curveTagOn
		} else {
			exc.pts.tags[I] &^= curveTagOn
		}
		if I == 0xFFFF {
			break
		}
	}
}

// computePointDisplacement is Compute_Point_Displacement; it reports
// failure.
func (exc *ttExecContext) computePointDisplacement() (dx, dy int64, zone ttGlyphZone, refp uint16, fail bool) {
	var zp ttGlyphZone
	var p uint16
	if exc.opcode&1 != 0 {
		zp = exc.zp0
		p = exc.GS.rp1
	} else {
		zp = exc.zp1
		p = exc.GS.rp2
	}
	if bounds(int64(p), int64(zp.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return 0, 0, zone, 0, true
	}
	d := exc.proj(zp.cur[p], zp.org[p])
	return mulFix(d, exc.moveVector.X), mulFix(d, exc.moveVector.Y), zp, p, false
}

// moveZp2Point is Move_Zp2_Point.
func (exc *ttExecContext) moveZp2Point(point uint16, dx, dy int64, touch bool) {
	if exc.GS.freeVector.x != 0 {
		if exc.backwardCompatibility == 0 {
			exc.zp2.cur[point].X += dx
		}
		if touch {
			exc.zp2.tags[point] |= curveTagTouchX
		}
	}
	if exc.GS.freeVector.y != 0 {
		if exc.backwardCompatibility != 0x7 {
			exc.zp2.cur[point].Y += dy
		}
		if touch {
			exc.zp2.tags[point] |= curveTagTouchY
		}
	}
}

func (exc *ttExecContext) insSHP(args int64) {
	loop := exc.GS.loop
	if exc.newTop < loop {
		if exc.pedanticHinting {
			exc.err = errTooFewArguments
		}
		exc.GS.loop = 1
		return
	}
	exc.newTop -= loop
	dx, dy, _, _, fail := exc.computePointDisplacement()
	if fail {
		return
	}
	for ; loop > 0; loop-- {
		args--
		point := uint16(exc.stack[args])
		if bounds(int64(point), int64(exc.zp2.nPoints)) {
			if exc.pedanticHinting {
				exc.err = errInvalidReference
				return
			}
		} else {
			exc.moveZp2Point(point, dx, dy, true)
		}
	}
	exc.GS.loop = 1
}

func (exc *ttExecContext) insSHC(args int64) {
	contour := uint16(exc.stack[args])
	bnds := exc.zp2.nContours
	if exc.GS.gep2 == 0 {
		bnds = 1
	}
	if bounds(int64(contour), int64(bnds)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	dx, dy, zp, refp, fail := exc.computePointDisplacement()
	if fail {
		return
	}
	var start, limit uint16
	if contour == 0 {
		start = 0
	} else {
		start = exc.zp2.contours[contour-1] + 1 - exc.zp2.firstPoint
	}
	if exc.GS.gep2 == 0 {
		limit = exc.zp2.nPoints
	} else {
		limit = exc.zp2.contours[contour] + 1 - exc.zp2.firstPoint
	}
	for i := start; i < limit; i++ {
		if zp.twilight != exc.zp2.twilight || refp != i {
			exc.moveZp2Point(i, dx, dy, true)
		}
	}
}

func (exc *ttExecContext) insSHZ(args int64) {
	if bounds(exc.stack[args], 2) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	dx, dy, zp, refp, fail := exc.computePointDisplacement()
	if fail {
		return
	}
	var limit uint16
	if exc.GS.gep2 == 0 {
		limit = exc.zp2.nPoints
	} else if exc.GS.gep2 == 1 && exc.zp2.nContours > 0 {
		limit = exc.zp2.contours[exc.zp2.nContours-1] + 1
	}
	for i := uint16(0); i < limit; i++ {
		if zp.twilight != exc.zp2.twilight || refp != i {
			exc.moveZp2Point(i, dx, dy, false)
		}
	}
}

func (exc *ttExecContext) insSHPIX(args int64) {
	loop := exc.GS.loop
	inTwilight := exc.GS.gep0 == 0 || exc.GS.gep1 == 0 || exc.GS.gep2 == 0
	if exc.newTop < loop {
		if exc.pedanticHinting {
			exc.err = errTooFewArguments
		}
		exc.GS.loop = 1
		return
	}
	exc.newTop -= loop
	dx := ttMulFix14(exc.stack[args], exc.GS.freeVector.x)
	dy := ttMulFix14(exc.stack[args], exc.GS.freeVector.y)
	for ; loop > 0; loop-- {
		args--
		point := uint16(exc.stack[args])
		if bounds(int64(point), int64(exc.zp2.nPoints)) {
			if exc.pedanticHinting {
				exc.err = errInvalidReference
				return
			}
		} else if exc.backwardCompatibility != 0 {
			if inTwilight || (exc.backwardCompatibility != 0x7 &&
				((exc.isComposite && exc.GS.freeVector.y != 0) ||
					exc.zp2.tags[point]&curveTagTouchY != 0)) {
				exc.moveZp2Point(point, 0, dy, true)
			}
		} else {
			exc.moveZp2Point(point, dx, dy, true)
		}
	}
	exc.GS.loop = 1
}

func (exc *ttExecContext) insMSIRP(args int64) {
	point := uint16(exc.stack[args])
	if bounds(int64(point), int64(exc.zp1.nPoints)) || bounds(int64(exc.GS.rp0), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	if exc.GS.gep1 == 0 {
		exc.zp1.org[point] = exc.zp0.org[exc.GS.rp0]
		exc.funcMoveOrig(exc, &exc.zp1, point, exc.stack[args+1])
		exc.zp1.cur[point] = exc.zp1.org[point]
	}
	distance := exc.proj(exc.zp1.cur[point], exc.zp0.cur[exc.GS.rp0])
	exc.funcMove(exc, &exc.zp1, point, exc.stack[args+1]-distance)
	exc.GS.rp1 = exc.GS.rp0
	exc.GS.rp2 = point
	if exc.opcode&1 != 0 {
		exc.GS.rp0 = point
	}
}

func (exc *ttExecContext) insMDAP(args int64) {
	point := uint16(exc.stack[args])
	if bounds(int64(point), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	var distance int64
	if exc.opcode&1 != 0 {
		curDist := exc.funcProject(exc, exc.zp0.cur[point].X, exc.zp0.cur[point].Y)
		distance = exc.funcRound(exc, curDist, 0) - curDist
	}
	exc.funcMove(exc, &exc.zp0, point, distance)
	exc.GS.rp0 = point
	exc.GS.rp1 = point
}

func (exc *ttExecContext) insMIAP(args int64) {
	cvtEntry := uint64(exc.stack[args+1])
	point := uint16(exc.stack[args])
	if bounds(int64(point), int64(exc.zp0.nPoints)) || boundsL(int64(cvtEntry), int64(exc.cvtSize)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		exc.GS.rp0 = point
		exc.GS.rp1 = point
		return
	}
	distance := exc.funcReadCVT(exc, cvtEntry)
	if exc.GS.gep0 == 0 {
		exc.zp0.org[point].X = ttMulFix14(distance, exc.GS.freeVector.x)
		exc.zp0.org[point].Y = ttMulFix14(distance, exc.GS.freeVector.y)
		exc.zp0.cur[point] = exc.zp0.org[point]
	}
	orgDist := exc.funcProject(exc, exc.zp0.cur[point].X, exc.zp0.cur[point].Y)
	if exc.opcode&1 != 0 {
		controlValueCutin := exc.GS.controlValueCutin
		delta := distance - orgDist
		if delta < 0 {
			delta = -delta
		}
		if delta > controlValueCutin {
			distance = orgDist
		}
		distance = exc.funcRound(exc, distance, 0)
	}
	exc.funcMove(exc, &exc.zp0, point, distance-orgDist)
	exc.GS.rp0 = point
	exc.GS.rp1 = point
}

// orusDist is the scaled original distance of MDRP, MD and IP.
func (exc *ttExecContext) orusDist(v1, v2 Vector) int64 {
	if exc.metrics.xScale == exc.metrics.yScale {
		return mulFix(exc.dualProj(v1, v2), exc.metrics.xScale)
	}
	vec := Vector{mulFix(v1.X-v2.X, exc.metrics.xScale), mulFix(v1.Y-v2.Y, exc.metrics.yScale)}
	return exc.funcDualproj(exc, vec.X, vec.Y)
}

func (exc *ttExecContext) insMDRP(args int64) {
	point := uint16(exc.stack[args])
	if bounds(int64(point), int64(exc.zp1.nPoints)) || bounds(int64(exc.GS.rp0), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
	} else {
		var orgDist int64
		if exc.GS.gep0 == 0 || exc.GS.gep1 == 0 {
			orgDist = exc.dualProj(exc.zp1.org[point], exc.zp0.org[exc.GS.rp0])
		} else {
			orgDist = exc.orusDist(exc.zp1.orus[point], exc.zp0.orus[exc.GS.rp0])
		}
		if exc.GS.singleWidthCutin > 0 &&
			orgDist < exc.GS.singleWidthValue+exc.GS.singleWidthCutin &&
			orgDist > exc.GS.singleWidthValue-exc.GS.singleWidthCutin {
			if orgDist >= 0 {
				orgDist = exc.GS.singleWidthValue
			} else {
				orgDist = -exc.GS.singleWidthValue
			}
		}
		compensation := exc.GS.compensation[exc.opcode&3]
		var distance int64
		if exc.opcode&4 != 0 {
			distance = exc.funcRound(exc, orgDist, compensation)
		} else {
			distance = roundNone(exc, orgDist, compensation)
		}
		if exc.opcode&8 != 0 {
			minimumDistance := exc.GS.minimumDistance
			if orgDist >= 0 {
				if distance < minimumDistance {
					distance = minimumDistance
				}
			} else if distance > -minimumDistance {
				distance = -minimumDistance
			}
		}
		orgDist = exc.proj(exc.zp1.cur[point], exc.zp0.cur[exc.GS.rp0])
		exc.funcMove(exc, &exc.zp1, point, distance-orgDist)
	}
	exc.GS.rp1 = exc.GS.rp0
	exc.GS.rp2 = point
	if exc.opcode&16 != 0 {
		exc.GS.rp0 = point
	}
}

func (exc *ttExecContext) insMIRP(args int64) {
	point := uint16(exc.stack[args])
	cvtEntry := uint64(exc.stack[args+1] + 1)
	if bounds(int64(point), int64(exc.zp1.nPoints)) || boundsL(int64(cvtEntry), int64(exc.cvtSize)+1) ||
		bounds(int64(exc.GS.rp0), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
	} else {
		var cvtDist int64
		if cvtEntry != 0 {
			cvtDist = exc.funcReadCVT(exc, cvtEntry-1)
		}
		delta := cvtDist - exc.GS.singleWidthValue
		if delta < 0 {
			delta = -delta
		}
		if delta < exc.GS.singleWidthCutin {
			if cvtDist >= 0 {
				cvtDist = exc.GS.singleWidthValue
			} else {
				cvtDist = -exc.GS.singleWidthValue
			}
		}
		if exc.GS.gep1 == 0 {
			exc.zp1.org[point].X = exc.zp0.org[exc.GS.rp0].X + ttMulFix14(cvtDist, exc.GS.freeVector.x)
			exc.zp1.org[point].Y = exc.zp0.org[exc.GS.rp0].Y + ttMulFix14(cvtDist, exc.GS.freeVector.y)
			exc.zp1.cur[point] = exc.zp1.org[point]
		}
		orgDist := exc.dualProj(exc.zp1.org[point], exc.zp0.org[exc.GS.rp0])
		curDist := exc.proj(exc.zp1.cur[point], exc.zp0.cur[exc.GS.rp0])
		if exc.GS.autoFlip {
			if orgDist^cvtDist < 0 {
				cvtDist = -cvtDist
			}
		}
		compensation := exc.GS.compensation[exc.opcode&3]
		var distance int64
		if exc.opcode&4 != 0 {
			if exc.GS.gep0 == exc.GS.gep1 {
				controlValueCutin := exc.GS.controlValueCutin
				delta = cvtDist - orgDist
				if delta < 0 {
					delta = -delta
				}
				if delta > controlValueCutin {
					cvtDist = orgDist
				}
			}
			distance = exc.funcRound(exc, cvtDist, compensation)
		} else {
			distance = roundNone(exc, cvtDist, compensation)
		}
		if exc.opcode&8 != 0 {
			minimumDistance := exc.GS.minimumDistance
			if orgDist >= 0 {
				if distance < minimumDistance {
					distance = minimumDistance
				}
			} else if distance > -minimumDistance {
				distance = -minimumDistance
			}
		}
		exc.funcMove(exc, &exc.zp1, point, distance-curDist)
	}
	exc.GS.rp1 = exc.GS.rp0
	exc.GS.rp2 = point
	if exc.opcode&16 != 0 {
		exc.GS.rp0 = point
	}
}

func (exc *ttExecContext) insALIGNRP(args int64) {
	loop := exc.GS.loop
	if exc.newTop < loop {
		if exc.pedanticHinting {
			exc.err = errTooFewArguments
		}
		exc.GS.loop = 1
		return
	}
	exc.newTop -= loop
	if bounds(int64(exc.GS.rp0), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		exc.GS.loop = 1
		return
	}
	for ; loop > 0; loop-- {
		args--
		point := uint16(exc.stack[args])
		if bounds(int64(point), int64(exc.zp1.nPoints)) {
			if exc.pedanticHinting {
				exc.err = errInvalidReference
				return
			}
		} else {
			distance := exc.proj(exc.zp1.cur[point], exc.zp0.cur[exc.GS.rp0])
			exc.funcMove(exc, &exc.zp1, point, -distance)
		}
	}
	exc.GS.loop = 1
}

func (exc *ttExecContext) insISECT(args int64) {
	point := uint16(exc.stack[args])
	a0 := uint16(exc.stack[args+1])
	a1 := uint16(exc.stack[args+2])
	b0 := uint16(exc.stack[args+3])
	b1 := uint16(exc.stack[args+4])
	if bounds(int64(b0), int64(exc.zp0.nPoints)) || bounds(int64(b1), int64(exc.zp0.nPoints)) ||
		bounds(int64(a0), int64(exc.zp1.nPoints)) || bounds(int64(a1), int64(exc.zp1.nPoints)) ||
		bounds(int64(point), int64(exc.zp2.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	dbx := exc.zp0.cur[b1].X - exc.zp0.cur[b0].X
	dby := exc.zp0.cur[b1].Y - exc.zp0.cur[b0].Y
	dax := exc.zp1.cur[a1].X - exc.zp1.cur[a0].X
	day := exc.zp1.cur[a1].Y - exc.zp1.cur[a0].Y
	dx := exc.zp0.cur[b0].X - exc.zp1.cur[a0].X
	dy := exc.zp0.cur[b0].Y - exc.zp1.cur[a0].Y
	discriminant := mulDiv(dax, -dby, 0x40) + mulDiv(day, dbx, 0x40)
	dotproduct := mulDiv(dax, dbx, 0x40) + mulDiv(day, dby, 0x40)
	if 19*abs64(discriminant) > abs64(dotproduct) {
		val := mulDiv(dx, -dby, 0x40) + mulDiv(dy, dbx, 0x40)
		rx := mulDiv(val, dax, discriminant)
		ry := mulDiv(val, day, discriminant)
		exc.zp2.cur[point].X = exc.zp1.cur[a0].X + rx
		exc.zp2.cur[point].Y = exc.zp1.cur[a0].Y + ry
	} else {
		exc.zp2.cur[point].X = (exc.zp1.cur[a0].X + exc.zp1.cur[a1].X + (exc.zp0.cur[b0].X + exc.zp0.cur[b1].X)) / 4
		exc.zp2.cur[point].Y = (exc.zp1.cur[a0].Y + exc.zp1.cur[a1].Y + (exc.zp0.cur[b0].Y + exc.zp0.cur[b1].Y)) / 4
	}
	exc.zp2.tags[point] |= curveTagTouchBoth
}

func (exc *ttExecContext) insALIGNPTS(args int64) {
	p1 := uint16(exc.stack[args])
	p2 := uint16(exc.stack[args+1])
	if bounds(int64(p1), int64(exc.zp1.nPoints)) || bounds(int64(p2), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	distance := exc.proj(exc.zp0.cur[p2], exc.zp1.cur[p1]) / 2
	exc.funcMove(exc, &exc.zp1, p1, distance)
	exc.funcMove(exc, &exc.zp0, p2, -distance)
}

func (exc *ttExecContext) insIP(args int64) {
	loop := exc.GS.loop
	if exc.newTop < loop {
		if exc.pedanticHinting {
			exc.err = errTooFewArguments
		}
		exc.GS.loop = 1
		return
	}
	exc.newTop -= loop
	if bounds(int64(exc.GS.rp1), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		exc.GS.loop = 1
		return
	}
	twilight := exc.GS.gep0 == 0 || exc.GS.gep1 == 0 || exc.GS.gep2 == 0
	var orusBase Vector
	if twilight {
		orusBase = exc.zp0.org[exc.GS.rp1]
	} else {
		orusBase = exc.zp0.orus[exc.GS.rp1]
	}
	curBase := exc.zp0.cur[exc.GS.rp1]
	var oldRange, curRange int64
	if !bounds(int64(exc.GS.rp2), int64(exc.zp1.nPoints)) {
		if twilight {
			oldRange = exc.dualProj(exc.zp1.org[exc.GS.rp2], orusBase)
		} else if exc.metrics.xScale == exc.metrics.yScale {
			oldRange = exc.dualProj(exc.zp1.orus[exc.GS.rp2], orusBase)
		} else {
			vec := Vector{mulFix(exc.zp1.orus[exc.GS.rp2].X-orusBase.X, exc.metrics.xScale),
				mulFix(exc.zp1.orus[exc.GS.rp2].Y-orusBase.Y, exc.metrics.yScale)}
			oldRange = exc.funcDualproj(exc, vec.X, vec.Y)
		}
		curRange = exc.proj(exc.zp1.cur[exc.GS.rp2], curBase)
	}
	for ; loop > 0; loop-- {
		args--
		point := uint32(exc.stack[args])
		if bounds(int64(point), int64(exc.zp2.nPoints)) {
			if exc.pedanticHinting {
				exc.err = errInvalidReference
				return
			}
			continue
		}
		var orgDist int64
		if twilight {
			orgDist = exc.dualProj(exc.zp2.org[point], orusBase)
		} else if exc.metrics.xScale == exc.metrics.yScale {
			orgDist = exc.dualProj(exc.zp2.orus[point], orusBase)
		} else {
			vec := Vector{mulFix(exc.zp2.orus[point].X-orusBase.X, exc.metrics.xScale),
				mulFix(exc.zp2.orus[point].Y-orusBase.Y, exc.metrics.yScale)}
			orgDist = exc.funcDualproj(exc, vec.X, vec.Y)
		}
		curDist := exc.proj(exc.zp2.cur[point], curBase)
		var newDist int64
		if orgDist != 0 {
			if oldRange != 0 {
				newDist = mulDiv(orgDist, curRange, oldRange)
			} else {
				newDist = orgDist
			}
		}
		exc.funcMove(exc, &exc.zp2, uint16(point), newDist-curDist)
	}
	exc.GS.loop = 1
}

func (exc *ttExecContext) insUTP(args int64) {
	point := uint16(exc.stack[args])
	if bounds(int64(point), int64(exc.zp0.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	mask := byte(0xFF)
	if exc.GS.freeVector.x != 0 {
		mask &^= curveTagTouchX
	}
	if exc.GS.freeVector.y != 0 {
		mask &^= curveTagTouchY
	}
	exc.zp0.tags[point] &= mask
}

// iupWorker is an IUP_WorkerRec for one axis.
type iupWorker struct {
	orgs, curs, orus []Vector
	y                bool
	maxPoints        uint32
}

func (w *iupWorker) get(v []Vector, i uint32) int64 {
	if w.y {
		return v[i].Y
	}
	return v[i].X
}

func (w *iupWorker) setCur(i uint32, x int64) {
	if w.y {
		w.curs[i].Y = x
	} else {
		w.curs[i].X = x
	}
}

func (w *iupWorker) shift(p1, p2, p uint32) {
	dx := w.get(w.curs, p) - w.get(w.orgs, p)
	if dx != 0 {
		for i := p1; i < p; i++ {
			w.setCur(i, w.get(w.curs, i)+dx)
		}
		for i := p + 1; i <= p2; i++ {
			w.setCur(i, w.get(w.curs, i)+dx)
		}
	}
}

func (w *iupWorker) interpolate(p1, p2, ref1, ref2 uint32) {
	if p1 > p2 {
		return
	}
	if ref1 >= w.maxPoints || ref2 >= w.maxPoints {
		return
	}
	orus1 := w.get(w.orus, ref1)
	orus2 := w.get(w.orus, ref2)
	if orus1 > orus2 {
		orus1, orus2 = orus2, orus1
		ref1, ref2 = ref2, ref1
	}
	org1 := w.get(w.orgs, ref1)
	org2 := w.get(w.orgs, ref2)
	cur1 := w.get(w.curs, ref1)
	cur2 := w.get(w.curs, ref2)
	delta1 := cur1 - org1
	delta2 := cur2 - org2
	if cur1 == cur2 || orus1 == orus2 {
		for i := p1; i <= p2; i++ {
			x := w.get(w.orgs, i)
			if x <= org1 {
				x += delta1
			} else if x >= org2 {
				x += delta2
			} else {
				x = cur1
			}
			w.setCur(i, x)
		}
	} else {
		var scale int64
		scaleValid := false
		for i := p1; i <= p2; i++ {
			x := w.get(w.orgs, i)
			if x <= org1 {
				x += delta1
			} else if x >= org2 {
				x += delta2
			} else {
				if !scaleValid {
					scaleValid = true
					scale = divFix(cur2-cur1, orus2-orus1)
				}
				x = cur1 + mulFix(w.get(w.orus, i)-orus1, scale)
			}
			w.setCur(i, x)
		}
	}
}

func (exc *ttExecContext) insIUP() {
	if exc.backwardCompatibility == 0x7 {
		return
	} else if exc.backwardCompatibility != 0 {
		exc.backwardCompatibility |= 1 << (exc.opcode & 1)
	}
	if exc.pts.nContours == 0 {
		return
	}
	var mask byte
	V := iupWorker{orgs: exc.pts.org, curs: exc.pts.cur, orus: exc.pts.orus}
	if exc.opcode&1 != 0 {
		mask = curveTagTouchX
	} else {
		mask = curveTagTouchY
		V.y = true
	}
	V.maxPoints = uint32(exc.pts.nPoints)

	contour := int16(0)
	point := uint32(0)
	for {
		endPoint := uint32(exc.pts.contours[contour]) - uint32(exc.pts.firstPoint)
		firstPoint := point
		if endPoint >= uint32(exc.pts.nPoints) {
			endPoint = uint32(exc.pts.nPoints) - 1
		}
		for point <= endPoint && exc.pts.tags[point]&mask == 0 {
			point++
		}
		if point <= endPoint {
			firstTouched := point
			curTouched := point
			point++
			for point <= endPoint {
				if exc.pts.tags[point]&mask != 0 {
					V.interpolate(curTouched+1, point-1, curTouched, point)
					curTouched = point
				}
				point++
			}
			if curTouched == firstTouched {
				V.shift(firstPoint, endPoint, curTouched)
			} else {
				V.interpolate(uint32(uint16(curTouched+1)), endPoint, curTouched, firstTouched)
				if firstTouched > 0 {
					V.interpolate(firstPoint, firstTouched-1, curTouched, firstTouched)
				}
			}
		}
		contour++
		if !(int32(contour) < int32(exc.pts.nContours)) {
			break
		}
	}
}

func (exc *ttExecContext) insDELTAP(args int64) {
	nump := exc.stack[args]
	if nump < 0 || nump > exc.newTop/2 {
		if exc.pedanticHinting {
			exc.err = errTooFewArguments
		}
		nump = exc.newTop / 2
	}
	exc.newTop -= 2 * nump
	P := exc.funcCurPpem(exc) - int64(exc.GS.deltaBase)
	switch exc.opcode {
	case 0x71:
		P -= 16
	case 0x72:
		P -= 32
	}
	if P&^0xF != 0 {
		return
	}
	P <<= 4
	F := int64(1) << (6 - exc.GS.deltaShift)
	for ; nump > 0; nump-- {
		args--
		A := uint16(exc.stack[args])
		args--
		B := exc.stack[args]
		if bounds(int64(A), int64(exc.zp0.nPoints)) {
			if exc.pedanticHinting {
				exc.err = errInvalidReference
				return
			}
		} else if B&0xF0 == P {
			B = (B & 0xF) - 8
			if B >= 0 {
				B++
			}
			B *= F
			if exc.backwardCompatibility != 0 {
				if exc.backwardCompatibility != 0x7 &&
					((exc.isComposite && exc.GS.freeVector.y != 0) ||
						exc.zp0.tags[A]&curveTagTouchY != 0) {
					exc.funcMove(exc, &exc.zp0, A, B)
				}
			} else {
				exc.funcMove(exc, &exc.zp0, A, B)
			}
		}
	}
}

func (exc *ttExecContext) insDELTAC(args int64) {
	nump := exc.stack[args]
	if nump < 0 || nump > exc.newTop/2 {
		if exc.pedanticHinting {
			exc.err = errTooFewArguments
		}
		nump = exc.newTop / 2
	}
	exc.newTop -= 2 * nump
	P := exc.funcCurPpem(exc) - int64(exc.GS.deltaBase)
	switch exc.opcode {
	case 0x74:
		P -= 16
	case 0x75:
		P -= 32
	}
	if P&^0xF != 0 {
		return
	}
	P <<= 4
	F := int64(1) << (6 - exc.GS.deltaShift)
	for ; nump > 0; nump-- {
		args--
		A := uint64(exc.stack[args])
		args--
		B := exc.stack[args]
		if boundsL(int64(A), int64(exc.cvtSize)) {
			if exc.pedanticHinting {
				exc.err = errInvalidReference
				return
			}
		} else if B&0xF0 == P {
			B = (B & 0xF) - 8
			if B >= 0 {
				B++
			}
			B *= F
			exc.funcMoveCVT(exc, A, B)
		}
	}
}

func (exc *ttExecContext) insGETINFO(args int64) {
	a0 := exc.stack[args]
	var K int64
	if a0&1 != 0 {
		K = ttInterpreterVersion
	}
	if a0&2 != 0 && exc.ttMetrics.rotated {
		K |= 1 << 8
	}
	if a0&4 != 0 && exc.ttMetrics.stretched {
		K |= 1 << 9
	}
	// VARIATION GLYPH: no variation blend at the default instance of a
	// font the port hints
	if a0&32 != 0 && exc.grayscale {
		K |= 1 << 12
	}
	if exc.mode != renderModeMono {
		if a0&64 != 0 {
			K |= 1 << 13
		}
		if a0&256 != 0 && exc.mode == renderModeLCDV {
			K |= 1 << 15
		}
		if a0&1024 != 0 {
			K |= 1 << 17
		}
		if a0&2048 != 0 && exc.mode != renderModeMono {
			K |= 1 << 18
		}
		if a0&4096 != 0 && exc.mode != renderModeMono && exc.mode != renderModeLCD &&
			exc.mode != renderModeLCDV {
			K |= 1 << 19
		}
	}
	exc.stack[args] = K
}

// insUNKNOWN is Ins_UNKNOWN: a user-defined instruction.
func (exc *ttExecContext) insUNKNOWN() {
	for i := uint32(0); i < exc.numIDefs; i++ {
		def := &exc.IDefs[i]
		if byte(def.opc) == exc.opcode && def.active {
			if exc.callTop >= exc.callSize {
				exc.err = errTTStackOverflow
				return
			}
			exc.callStack[exc.callTop] = ttCallRec{exc.curRange, exc.IP + 1, 1, def}
			exc.callTop++
			exc.gotoCodeRange(def.rng, def.start)
			return
		}
	}
	exc.err = errInvalidOpcode
}

// runIns is TT_RunIns.
func (exc *ttExecContext) runIns() error {
	insCounter := uint64(0)
	for {
		insCounter++
		if insCounter > ttMaxRunnableOpcodes {
			exc.err = errExecutionTooLong
			return exc.err
		}
		exc.err = nil
		exc.opcode = exc.code[exc.IP]
		exc.length = 1

		exc.args = exc.top - int64(popPushCount[exc.opcode]>>4)
		if exc.args < 0 {
			if exc.pedanticHinting {
				exc.err = errTooFewArguments
				return exc.err
			}
			for i := 0; i < int(popPushCount[exc.opcode]>>4); i++ {
				exc.stack[i] = 0
			}
			exc.args = 0
		}
		exc.newTop = exc.args + int64(popPushCount[exc.opcode]&15)
		if exc.newTop > exc.stackSize {
			exc.err = errTTStackOverflow
			return exc.err
		}

		args := exc.args
		s := exc.stack
		switch opcode := exc.opcode; opcode {
		case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05:
			exc.insSxyTCA()
		case 0x06, 0x07:
			if !exc.insSxVTL(uint16(s[args+1]), uint16(s[args]), &exc.GS.projVector) {
				exc.GS.dualVector = exc.GS.projVector
				exc.computeFuncs()
			}
		case 0x08, 0x09:
			if !exc.insSxVTL(uint16(s[args+1]), uint16(s[args]), &exc.GS.freeVector) {
				exc.computeFuncs()
			}
		case 0x0A:
			normalize(int64(int16(s[args])), int64(int16(s[args+1])), &exc.GS.projVector)
			exc.GS.dualVector = exc.GS.projVector
			exc.computeFuncs()
		case 0x0B:
			normalize(int64(int16(s[args])), int64(int16(s[args+1])), &exc.GS.freeVector)
			exc.computeFuncs()
		case 0x0C:
			s[args] = int64(exc.GS.projVector.x)
			s[args+1] = int64(exc.GS.projVector.y)
		case 0x0D:
			s[args] = int64(exc.GS.freeVector.x)
			s[args+1] = int64(exc.GS.freeVector.y)
		case 0x0E:
			exc.GS.freeVector = exc.GS.projVector
			exc.computeFuncs()
		case 0x0F:
			exc.insISECT(args)
		case 0x10:
			exc.GS.rp0 = uint16(s[args])
		case 0x11:
			exc.GS.rp1 = uint16(s[args])
		case 0x12:
			exc.GS.rp2 = uint16(s[args])
		case 0x13:
			if z, ok := exc.zoneFor(s[args]); ok {
				exc.zp0 = z
				exc.GS.gep0 = uint16(s[args])
			}
		case 0x14:
			if z, ok := exc.zoneFor(s[args]); ok {
				exc.zp1 = z
				exc.GS.gep1 = uint16(s[args])
			}
		case 0x15:
			if z, ok := exc.zoneFor(s[args]); ok {
				exc.zp2 = z
				exc.GS.gep2 = uint16(s[args])
			}
		case 0x16:
			if z, ok := exc.zoneFor(s[args]); ok {
				exc.zp0 = z
				exc.zp1 = exc.zp0
				exc.zp2 = exc.zp0
				exc.GS.gep0 = uint16(s[args])
				exc.GS.gep1 = uint16(s[args])
				exc.GS.gep2 = uint16(s[args])
			}
		case 0x17:
			if s[args] < 0 {
				exc.err = errBadArgument
			} else if s[args] > 0xFFFF {
				exc.GS.loop = 0xFFFF
			} else {
				exc.GS.loop = s[args]
			}
		case 0x18:
			exc.GS.roundState = ttRoundToGrid
			exc.funcRound = roundToGrid
		case 0x19:
			exc.GS.roundState = ttRoundToHalfGrid
			exc.funcRound = roundToHalfGrid
		case 0x1A:
			exc.GS.minimumDistance = s[args]
		case 0x1B:
			exc.insELSE()
		case 0x1C:
			exc.insJMPR(args)
		case 0x1D:
			exc.GS.controlValueCutin = s[args]
		case 0x1E:
			exc.GS.singleWidthCutin = s[args]
		case 0x1F:
			exc.GS.singleWidthValue = mulFix(s[args], exc.ttMetrics.scale)
		case 0x20:
			s[args+1] = s[args]
		case 0x21:
		case 0x22:
			exc.newTop = 0
		case 0x23:
			s[args], s[args+1] = s[args+1], s[args]
		case 0x24:
			s[args] = exc.top
		case 0x25:
			L := s[args]
			if L <= 0 || L > exc.args {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
				s[args] = 0
			} else {
				s[args] = s[args-L]
			}
		case 0x26:
			L := s[args]
			if L <= 0 || L > exc.args {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
			} else {
				K := s[args-L]
				copy(s[args-L:args-1], s[args-L+1:args])
				s[args-1] = K
			}
		case 0x27:
			exc.insALIGNPTS(args)
		case 0x28:
			exc.insUNKNOWN()
		case 0x29:
			exc.insUTP(args)
		case 0x2A:
			exc.insLOOPCALL(args)
		case 0x2B:
			exc.insCALL(args)
		case 0x2C:
			exc.insFDEF(args)
		case 0x2D:
			exc.insENDF()
		case 0x2E, 0x2F:
			exc.insMDAP(args)
		case 0x30, 0x31:
			exc.insIUP()
		case 0x32, 0x33:
			exc.insSHP(args)
		case 0x34, 0x35:
			exc.insSHC(args)
		case 0x36, 0x37:
			exc.insSHZ(args)
		case 0x38:
			exc.insSHPIX(args)
		case 0x39:
			exc.insIP(args)
		case 0x3A, 0x3B:
			exc.insMSIRP(args)
		case 0x3C:
			exc.insALIGNRP(args)
		case 0x3D:
			exc.GS.roundState = ttRoundToDoubleGrid
			exc.funcRound = roundToDoubleGrid
		case 0x3E, 0x3F:
			exc.insMIAP(args)
		case 0x40:
			exc.insNPUSHB(args)
		case 0x41:
			exc.insNPUSHW(args)
		case 0x42:
			I := uint64(s[args])
			if boundsL(int64(I), int64(exc.storeSize)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
			} else {
				if exc.iniRange == ttCoderangeGlyph && !sameSlice(exc.storage, exc.glyfStorage) {
					exc.glyfStorage = append(exc.glyfStorage[:0], exc.storage[:exc.storeSize]...)
					exc.storage = exc.glyfStorage
				}
				exc.storage[I] = s[args+1]
			}
		case 0x43:
			I := uint64(s[args])
			if boundsL(int64(I), int64(exc.storeSize)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				} else {
					s[args] = 0
				}
			} else {
				s[args] = exc.storage[I]
			}
		case 0x44:
			I := uint64(s[args])
			if boundsL(int64(I), int64(exc.cvtSize)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
			} else {
				exc.funcWriteCVT(exc, I, s[args+1])
			}
		case 0x45:
			I := uint64(s[args])
			if boundsL(int64(I), int64(exc.cvtSize)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				} else {
					s[args] = 0
				}
			} else {
				s[args] = exc.funcReadCVT(exc, I)
			}
		case 0x46, 0x47:
			L := uint64(s[args])
			var R int64
			if boundsL(int64(L), int64(exc.zp2.nPoints)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
			} else if exc.opcode&1 != 0 {
				R = exc.funcDualproj(exc, exc.zp2.org[L].X, exc.zp2.org[L].Y)
			} else {
				R = exc.funcProject(exc, exc.zp2.cur[L].X, exc.zp2.cur[L].Y)
			}
			s[args] = R
		case 0x48:
			L := uint16(s[args])
			if bounds(int64(L), int64(exc.zp2.nPoints)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
			} else {
				K := exc.funcProject(exc, exc.zp2.cur[L].X, exc.zp2.cur[L].Y)
				exc.funcMove(exc, &exc.zp2, L, s[args+1]-K)
				if exc.GS.gep2 == 0 {
					exc.zp2.org[L] = exc.zp2.cur[L]
				}
			}
		case 0x49, 0x4A:
			K := uint16(s[args+1])
			L := uint16(s[args])
			var D int64
			if bounds(int64(L), int64(exc.zp0.nPoints)) || bounds(int64(K), int64(exc.zp1.nPoints)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
			} else if exc.opcode&1 != 0 {
				D = exc.proj(exc.zp0.cur[L], exc.zp1.cur[K])
			} else if exc.GS.gep0 == 0 || exc.GS.gep1 == 0 {
				D = exc.dualProj(exc.zp0.org[L], exc.zp1.org[K])
			} else {
				D = exc.orusDist(exc.zp0.orus[L], exc.zp1.orus[K])
			}
			s[args] = D
		case 0x4B:
			s[args] = exc.funcCurPpem(exc)
		case 0x4C:
			// v40: the point size
			s[args] = exc.pointSize
		case 0x4D:
			exc.GS.autoFlip = true
		case 0x4E:
			exc.GS.autoFlip = false
		case 0x4F:
			exc.err = errDebugOpCode
		case 0x50:
			s[args] = b2l(s[args] < s[args+1])
		case 0x51:
			s[args] = b2l(s[args] <= s[args+1])
		case 0x52:
			s[args] = b2l(s[args] > s[args+1])
		case 0x53:
			s[args] = b2l(s[args] >= s[args+1])
		case 0x54:
			s[args] = b2l(s[args] == s[args+1])
		case 0x55:
			s[args] = b2l(s[args] != s[args+1])
		case 0x56:
			s[args] = b2l(exc.funcRound(exc, s[args], 0)&64 == 64)
		case 0x57:
			s[args] = b2l(exc.funcRound(exc, s[args], 0)&64 == 0)
		case 0x58:
			exc.insIF(args)
		case 0x59:
		case 0x5A:
			s[args] = b2l(s[args] != 0 && s[args+1] != 0)
		case 0x5B:
			s[args] = b2l(s[args] != 0 || s[args+1] != 0)
		case 0x5C:
			s[args] = b2l(s[args] == 0)
		case 0x5D, 0x71, 0x72:
			exc.insDELTAP(args)
		case 0x5E:
			exc.GS.deltaBase = uint16(s[args])
		case 0x5F:
			if uint64(s[args]) > 6 {
				exc.err = errBadArgument
			} else {
				exc.GS.deltaShift = uint16(s[args])
			}
		case 0x60:
			s[args] += s[args+1]
		case 0x61:
			s[args] -= s[args+1]
		case 0x62:
			if s[args+1] == 0 {
				exc.err = errDivideByZeroIns
			} else {
				s[args] = mulDivNoRound(s[args], 64, s[args+1])
			}
		case 0x63:
			s[args] = mulDiv(s[args], s[args+1], 64)
		case 0x64:
			if s[args] < 0 {
				s[args] = -s[args]
			}
		case 0x65:
			s[args] = -s[args]
		case 0x66:
			s[args] = pixFloor(s[args])
		case 0x67:
			s[args] = pixCeilLong(s[args])
		case 0x68, 0x69, 0x6A, 0x6B:
			s[args] = exc.funcRound(exc, s[args], exc.GS.compensation[exc.opcode&3])
		case 0x6C, 0x6D, 0x6E, 0x6F:
			s[args] = roundNone(exc, s[args], exc.GS.compensation[exc.opcode&3])
		case 0x70:
			I := uint64(s[args])
			if boundsL(int64(I), int64(exc.cvtSize)) {
				if exc.pedanticHinting {
					exc.err = errInvalidReference
				}
			} else {
				exc.cvt[I] = mulFix(s[args+1], exc.ttMetrics.scale)
			}
		case 0x73, 0x74, 0x75:
			exc.insDELTAC(args)
		case 0x76:
			exc.setSuperRound(0x4000, s[args])
			exc.GS.roundState = ttRoundSuper
			exc.funcRound = roundSuper
		case 0x77:
			exc.setSuperRound(0x2D41, s[args])
			exc.GS.roundState = ttRoundSuper45
			exc.funcRound = roundSuper45
		case 0x78:
			if s[args+1] != 0 {
				exc.insJMPR(args)
			}
		case 0x79:
			if s[args+1] == 0 {
				exc.insJMPR(args)
			}
		case 0x7A:
			exc.GS.roundState = ttRoundOff
			exc.funcRound = roundNone
		case 0x7B:
			exc.insUNKNOWN()
		case 0x7C:
			exc.GS.roundState = ttRoundUpToGrid
			exc.funcRound = roundUpToGrid
		case 0x7D:
			exc.GS.roundState = ttRoundDownToGrid
			exc.funcRound = roundDownToGrid
		case 0x7E, 0x7F:
			// SANGW, AA: no longer supported
		case 0x80:
			exc.insFLIPPT(args)
		case 0x81:
			exc.insFLIPRG(args, true)
		case 0x82:
			exc.insFLIPRG(args, false)
		case 0x83, 0x84:
			exc.insUNKNOWN()
		case 0x85:
			exc.insSCANCTRL(args)
		case 0x86, 0x87:
			exc.insSDPVTL(args)
		case 0x88:
			exc.insGETINFO(args)
		case 0x89:
			exc.insIDEF(args)
		case 0x8A:
			A, B, C := s[args+2], s[args+1], s[args]
			s[args+2] = C
			s[args+1] = A
			s[args] = B
		case 0x8B:
			if s[args+1] > s[args] {
				s[args] = s[args+1]
			}
		case 0x8C:
			if s[args+1] < s[args] {
				s[args] = s[args+1]
			}
		case 0x8D:
			if s[args] >= 0 {
				exc.GS.scanType = int32(s[args]) & 0xFFFF
			}
		case 0x8E:
			exc.insINSTCTRL(args)
		case 0x8F, 0x90:
			exc.insUNKNOWN()
		case 0x91, 0x92:
			// GETVARIATION and GETDATA need a variation blend
			exc.insUNKNOWN()
		default:
			switch {
			case opcode >= 0xE0:
				exc.insMIRP(args)
			case opcode >= 0xC0:
				exc.insMDRP(args)
			case opcode >= 0xB8:
				exc.insPUSHW(args)
			case opcode >= 0xB0:
				exc.insPUSHB(args)
			default:
				exc.insUNKNOWN()
			}
		}

		if exc.err != nil {
			if exc.err == errInvalidOpcode {
				found := false
				for i := uint32(0); i < exc.numIDefs; i++ {
					def := &exc.IDefs[i]
					if def.active && exc.opcode == byte(def.opc) {
						if exc.callTop >= exc.callSize {
							exc.err = errInvalidReference
							return exc.err
						}
						exc.callStack[exc.callTop] = ttCallRec{exc.curRange, exc.IP + 1, 1, def}
						if exc.gotoCodeRange(def.rng, def.start) {
							return exc.err
						}
						found = true
						break
					}
				}
				if !found {
					return exc.err
				}
			} else {
				return exc.err
			}
		} else {
			exc.top = exc.newTop
			exc.IP += exc.length
		}

		if exc.IP >= exc.codeSize {
			if exc.callTop > 0 {
				exc.err = errCodeOverflow
				return exc.err
			}
			return nil
		}
	}
}

func (exc *ttExecContext) insSDPVTL(args int64) {
	opcode := exc.opcode
	p1 := uint16(exc.stack[args+1])
	p2 := uint16(exc.stack[args])
	if bounds(int64(p2), int64(exc.zp1.nPoints)) || bounds(int64(p1), int64(exc.zp2.nPoints)) {
		if exc.pedanticHinting {
			exc.err = errInvalidReference
		}
		return
	}
	v1 := exc.zp1.org[p2]
	v2 := exc.zp2.org[p1]
	A := v1.X - v2.X
	B := v1.Y - v2.Y
	if A == 0 && B == 0 {
		A = 0x4000
		opcode = 0
	}
	if opcode&1 != 0 {
		C := B
		B = A
		A = -C
	}
	normalize(A, B, &exc.GS.dualVector)

	v1 = exc.zp1.cur[p2]
	v2 = exc.zp2.cur[p1]
	A = v1.X - v2.X
	B = v1.Y - v2.Y
	if A == 0 && B == 0 {
		A = 0x4000
		opcode = 0
	}
	if opcode&1 != 0 {
		C := B
		B = A
		A = -C
	}
	normalize(A, B, &exc.GS.projVector)
	exc.computeFuncs()
}

// runContext is TT_Run_Context.
func (exc *ttExecContext) runContext(size *ttSize) error {
	exc.zp0 = exc.pts
	exc.zp1 = exc.pts
	exc.zp2 = exc.pts

	numTwilightPoints := uint64(2 * (uint64(exc.pts.nPoints) + exc.cvtSize))
	if numTwilightPoints < 30 {
		numTwilightPoints = 30
	}
	if uint64(exc.twilight.nPoints) > numTwilightPoints {
		if numTwilightPoints > 0xFFFF {
			numTwilightPoints = 0xFFFF
		}
		exc.twilight.nPoints = uint16(numTwilightPoints)
	}

	exc.loopcallCounter = 0
	exc.negJumpCounter = 0
	if exc.pts.nPoints != 0 {
		a := uint64(10 * uint64(exc.pts.nPoints))
		if a < 50 {
			a = 50
		}
		b := exc.cvtSize / 10
		if b < 50 {
			b = 50
		}
		exc.loopcallCounterMax = a + b
	} else {
		exc.loopcallCounterMax = 300 + 22*exc.cvtSize
	}
	if exc.loopcallCounterMax > 100*uint64(exc.face.numGlyphs) {
		exc.loopcallCounterMax = 100 * uint64(exc.face.numGlyphs)
	}
	exc.negJumpCounterMax = exc.loopcallCounterMax

	if exc.metrics.xPpem != exc.metrics.yPpem {
		exc.funcCurPpem = currentPpemStretched
		exc.funcReadCVT = readCVTStretched
		exc.funcWriteCVT = writeCVTStretched
		exc.funcMoveCVT = moveCVTStretched
	} else {
		exc.funcCurPpem = currentPpem
		exc.funcReadCVT = readCVT
		exc.funcWriteCVT = writeCVT
		exc.funcMoveCVT = moveCVT
	}

	exc.GS = size.GS
	exc.funcRound = roundToGrid
	exc.computeFuncs()
	exc.backwardCompatibility &^= 0x3
	exc.top = 0
	exc.callTop = 0
	return exc.runIns()
}
