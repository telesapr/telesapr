// Package dxf writes ASCII DXF files, AutoCAD 2007 (AC1021) format, as
// the part of libdxfrw that SolveSpace uses does, byte for byte.
package dxf

import (
	"bytes"
	_ "embed"
	"math"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/internal/cfmt"
)

// Version is the libdxfrw version the files name.
const Version = "0.6.3"

// firstHandle is where handles start.
const firstHandle = 48

// ARAD converts radians to degrees.
const ARAD = 57.29577951308232

// header is the HEADER section libdxfrw writes for AC1021 when the
// application sets no header variables: it depends on nothing else.
// scripts/regen-dxf-golden.sh extracts it from a SolveSpace export.
//
//go:embed header.txt
var header string

// Coord is a point.
type Coord struct{ X, Y, Z float64 }

// Line widths as DXF writes them: by layer, by block and the default.
const (
	WidthByLayer = -1
	WidthByBlock = -2
	WidthDefault = -3
)

// WidthMm is a line width in mm rounded down to a standard DXF line
// weight, in hundredths of a mm.
func WidthMm(millimeters float64) int {
	if millimeters < 0.0 {
		return WidthByLayer
	}
	if millimeters > 2.11 {
		millimeters = 2.11
	}
	i := int(math.Floor(millimeters * 100.0))
	steps := []struct{ below, width int }{
		{3, 0}, {7, 5}, {11, 9}, {14, 13}, {16, 15}, {19, 18}, {22, 20}, {27, 25},
		{32, 30}, {37, 35}, {45, 40}, {52, 50}, {57, 53}, {65, 60}, {75, 70}, {85, 80},
		{95, 90}, {103, 100}, {112, 106}, {130, 120}, {149, 140}, {180, 158}, {205, 200},
	}
	for _, s := range steps {
		if i < s.below {
			return s.width
		}
	}
	return 211
}

// Entity is what every entity has.
type Entity struct {
	Space    int // 1 for paper space
	Layer    string
	LineType string
	Color    int
	// Color24 is the 24-bit colour, or -1 for none.
	Color24 int
	// LWeight is the line weight as DXF writes it.
	LWeight int
}

// NewEntity is an entity with libdxfrw's defaults.
func NewEntity() Entity {
	return Entity{Layer: "0", LineType: "BYLAYER", Color: 256, Color24: -1, LWeight: WidthByLayer}
}

// Line is a LINE.
type Line struct {
	Entity
	BasePoint, SecPoint Coord
}

// Arc is an ARC, with its angles in radians.
type Arc struct {
	Entity
	BasePoint          Coord
	Radius             float64
	StaAngle, EndAngle float64
}

// Vertex is a polyline vertex.
type Vertex struct {
	BasePoint Coord
	Bulge     float64
}

// Polyline is a POLYLINE; flag 8 makes it 3d.
type Polyline struct {
	Entity
	Flags    int
	Vertices []Vertex
}

// Spline is a SPLINE.
type Spline struct {
	Entity
	Flags    int
	Degree   int
	Knots    []float64
	Weights  []float64
	Controls []Coord
}

// DimensionKind is the kind of a dimension.
type DimensionKind int

const (
	DimAligned DimensionKind = iota
	DimLinear
	DimRadial
	DimDiametric
	DimAngular
)

// Dimension is a DIMENSION. Its points are named after the DXF codes
// they go in: Def is 10, Text 11, Pt3 13, Pt4 14, Pt5 15 and Pt6 16.
type Dimension struct {
	Entity
	Kind      DimensionKind
	Def       Coord
	TextPoint Coord
	Pt3, Pt4  Coord
	Pt5, Pt6  Coord
	Text      string
	// Angle and Oblique are the linear dimension's rotation and oblique
	// angles.
	Angle, Oblique float64
	// Actual is the measured value; HasActual says whether it's set.
	Actual    float64
	HasActual bool
}

// Text alignments.
const (
	HLeft   = 0
	HCenter = 1
	HRight  = 2

	VBaseLine = 0
	VMiddle   = 2
	VTop      = 3
)

// Text is a TEXT.
type Text struct {
	Entity
	BasePoint, SecPoint Coord
	Text                string
	Height              float64
	Angle               float64
	Style               string
	AlignH, AlignV      int
}

// LType is a line type.
type LType struct {
	Name string
	Path []float64
}

// Layer is a layer, with libdxfrw's defaults but its name.
type Layer struct {
	Name string
}

// Textstyle is a text style, with libdxfrw's defaults but its name and
// font.
type Textstyle struct {
	Name, Font string
}

// Interface supplies what the file holds.
type Interface interface {
	WriteLTypes(w *Writer)
	WriteLayers(w *Writer)
	WriteTextstyles(w *Writer)
	WriteEntities(w *Writer)
}

// Writer writes a DXF file.
type Writer struct {
	out          bytes.Buffer
	entCount     int
	wlayer0      bool
	dimstyleStd  bool
	textStyleMap map[string]int
}

func toHexStr(n int) string { return strings.ToUpper(strconv.FormatInt(int64(n), 16)) }

// The dxfWriterAscii group writers.

func (w *Writer) writeString(code int, text string) {
	w.out.WriteString(cfmt.Sprintf("%3d\n", code))
	w.out.WriteString(text)
	w.out.WriteByte('\n')
}

func (w *Writer) writeInt16(code, data int) {
	w.out.WriteString(cfmt.Sprintf("%3d\n%5d\n", code, data))
}

func (w *Writer) writeInt32(code, data int) { w.writeInt16(code, data) }

func (w *Writer) writeDouble(code int, data float64) {
	w.out.WriteString(cfmt.Sprintf("%3d\n%.16g\n", code, data))
}

func (w *Writer) writeBool(code int, data bool) {
	v := 0
	if data {
		v = 1
	}
	w.out.WriteString(cfmt.Sprintf("%d\n%d\n", code, v))
}

// Write writes a file with what iface supplies.
func Write(iface Interface) []byte {
	w := &Writer{textStyleMap: map[string]int{}}
	w.writeString(999, "dxfrw "+Version)
	w.writeString(0, "SECTION")
	w.entCount = firstHandle
	w.out.WriteString(header)
	w.writeString(0, "ENDSEC")
	w.writeString(0, "SECTION")
	w.writeString(2, "CLASSES")
	w.writeString(0, "ENDSEC")
	w.writeString(0, "SECTION")
	w.writeString(2, "TABLES")
	w.writeTables(iface)
	w.writeString(0, "ENDSEC")
	w.writeString(0, "SECTION")
	w.writeString(2, "BLOCKS")
	w.writeBlocks()
	w.writeString(0, "ENDSEC")

	w.writeString(0, "SECTION")
	w.writeString(2, "ENTITIES")
	iface.WriteEntities(w)
	w.writeString(0, "ENDSEC")

	w.writeString(0, "SECTION")
	w.writeString(2, "OBJECTS")
	w.writeObjects()
	w.writeString(0, "ENDSEC")
	w.writeString(0, "EOF")
	return w.out.Bytes()
}

func (w *Writer) writeEntity(ent *Entity) {
	w.entCount++
	w.writeString(5, toHexStr(w.entCount))
	w.writeString(100, "AcDbEntity")
	if ent.Space == 1 {
		w.writeInt16(67, 1)
	}
	w.writeString(8, ent.Layer)
	w.writeString(6, ent.LineType)
	w.writeInt16(62, ent.Color)
	if ent.Color24 >= 0 {
		w.writeInt32(420, ent.Color24)
	}
	w.writeInt16(370, ent.LWeight)
}

// WriteLineType writes a line type table entry.
func (w *Writer) WriteLineType(ent *LType) {
	strname := strings.ToUpper(ent.Name)
	// do not write linetypes handled by library
	if strname == "BYLAYER" || strname == "BYBLOCK" || strname == "CONTINUOUS" {
		return
	}
	w.writeString(0, "LTYPE")
	w.entCount++
	w.writeString(5, toHexStr(w.entCount))
	w.writeString(330, "5")
	w.writeString(100, "AcDbSymbolTableRecord")
	w.writeString(100, "AcDbLinetypeTableRecord")
	w.writeString(2, ent.Name)
	w.writeInt16(70, 0)
	w.writeString(3, "")
	length := 0.0
	for _, d := range ent.Path {
		length += math.Abs(d)
	}
	w.writeInt16(72, 65)
	w.writeInt16(73, len(ent.Path))
	w.writeDouble(40, length)

	for _, d := range ent.Path {
		w.writeDouble(49, d)
		w.writeInt16(74, 0)
	}
}

// WriteLayer writes a layer table entry.
func (w *Writer) WriteLayer(ent *Layer) {
	w.writeString(0, "LAYER")
	if !w.wlayer0 && ent.Name == "0" {
		w.wlayer0 = true
		w.writeString(5, "10")
	} else {
		w.entCount++
		w.writeString(5, toHexStr(w.entCount))
	}
	w.writeString(330, "2")
	w.writeString(100, "AcDbSymbolTableRecord")
	w.writeString(100, "AcDbLayerTableRecord")
	w.writeString(2, ent.Name)
	w.writeInt16(70, 0)
	w.writeInt16(62, 7)
	w.writeString(6, "CONTINUOUS")
	w.writeInt16(370, WidthDefault)
	w.writeString(390, "F")
}

// WriteTextstyle writes a text style table entry.
func (w *Writer) WriteTextstyle(ent *Textstyle) {
	w.writeString(0, "STYLE")
	name := strings.ToUpper(ent.Name)
	if !w.dimstyleStd && name == "STANDARD" {
		w.dimstyleStd = true
	}
	w.entCount++
	w.writeString(5, toHexStr(w.entCount))
	w.textStyleMap[name] = w.entCount

	w.writeString(330, "2")
	w.writeString(100, "AcDbSymbolTableRecord")
	w.writeString(100, "AcDbTextStyleTableRecord")
	w.writeString(2, ent.Name)
	w.writeInt16(70, 0)
	w.writeDouble(40, 0.0)
	w.writeDouble(41, 1.0)
	w.writeDouble(50, 0.0)
	w.writeInt16(71, 0)
	w.writeDouble(42, 1.0)
	w.writeString(3, ent.Font)
	w.writeString(4, "")
}

// writeVport writes the default active viewport.
func (w *Writer) writeVport() {
	w.dimstyleStd = true
	w.writeString(0, "VPORT")
	w.entCount++
	w.writeString(5, toHexStr(w.entCount))
	w.writeString(330, "2")
	w.writeString(100, "AcDbSymbolTableRecord")
	w.writeString(100, "AcDbViewportTableRecord")
	w.writeString(2, "*ACTIVE")
	w.writeInt16(70, 0)
	w.writeDouble(10, 0)
	w.writeDouble(20, 0)
	w.writeDouble(11, 1.0)
	w.writeDouble(21, 1.0)
	w.writeDouble(12, 0.651828)
	w.writeDouble(22, -0.16)
	w.writeDouble(13, 0)
	w.writeDouble(23, 0)
	w.writeDouble(14, 10.0)
	w.writeDouble(24, 10.0)
	w.writeDouble(15, 10.0)
	w.writeDouble(25, 10.0)
	w.writeDouble(16, 0)
	w.writeDouble(26, 0)
	w.writeDouble(36, 1)
	w.writeDouble(17, 0)
	w.writeDouble(27, 0)
	w.writeDouble(37, 0)
	w.writeDouble(40, 5.13732)
	w.writeDouble(41, 2.4426877)
	w.writeDouble(42, 50)
	w.writeDouble(43, 0)
	w.writeDouble(44, 0)
	w.writeDouble(50, 0)
	w.writeDouble(51, 0)
	w.writeInt16(71, 0)
	w.writeInt16(72, 100)
	w.writeInt16(73, 1)
	w.writeInt16(74, 3)
	w.writeInt16(75, 0)
	w.writeInt16(76, 0)
	w.writeInt16(77, 0)
	w.writeInt16(78, 0)
	w.writeInt16(281, 0)
	w.writeInt16(65, 1)
	w.writeDouble(110, 0.0)
	w.writeDouble(120, 0.0)
	w.writeDouble(130, 0.0)
	w.writeDouble(111, 1.0)
	w.writeDouble(121, 0.0)
	w.writeDouble(131, 0.0)
	w.writeDouble(112, 0.0)
	w.writeDouble(122, 1.0)
	w.writeDouble(132, 0.0)
	w.writeInt16(79, 0)
	w.writeDouble(146, 0.0)
	w.writeString(348, "10020")
	w.writeInt16(60, 7)
	w.writeInt16(61, 5)
	w.writeBool(292, true)
	w.writeInt16(282, 1)
	w.writeDouble(141, 0.0)
	w.writeDouble(142, 0.0)
	w.writeInt16(63, 250)
	w.writeInt32(421, 3358443)
}

// writeDimstyle writes the default dimension style, "Standard".
func (w *Writer) writeDimstyle() {
	w.writeString(0, "DIMSTYLE")
	w.dimstyleStd = true
	w.entCount++
	w.writeString(105, toHexStr(w.entCount))

	w.writeString(330, "A")
	w.writeString(100, "AcDbSymbolTableRecord")
	w.writeString(100, "AcDbDimStyleTableRecord")
	w.writeString(2, "Standard")
	w.writeInt16(70, 0)
	w.writeDouble(40, 1.0)                       // dimscale
	w.writeDouble(41, 0.18)                      // dimasz
	w.writeDouble(42, 0.0625)                    // dimexo
	w.writeDouble(43, 0.38)                      // dimdli
	w.writeDouble(44, 0.18)                      // dimexe
	w.writeDouble(45, 0.0)                       // dimrnd
	w.writeDouble(46, 0.0)                       // dimdle
	w.writeDouble(47, 0.0)                       // dimtp
	w.writeDouble(48, 0.0)                       // dimtm
	w.writeDouble(49, 1.0)                       // dimfxl
	w.writeDouble(140, 0.18)                     // dimtxt
	w.writeDouble(141, 0.09)                     // dimcen
	w.writeDouble(142, 0.0)                      // dimtsz
	w.writeDouble(143, 25.4)                     // dimaltf
	w.writeDouble(144, 1.0)                      // dimlfac
	w.writeDouble(145, 0.0)                      // dimtvp
	w.writeDouble(146, 1.0)                      // dimtfac
	w.writeDouble(147, 0.09)                     // dimgap
	w.writeDouble(148, 0.0)                      // dimaltrnd
	w.writeInt16(71, 0)                          // dimtol
	w.writeInt16(72, 0)                          // dimlim
	w.writeInt16(73, 0)                          // dimtih
	w.writeInt16(74, 1)                          // dimtoh
	w.writeInt16(75, 0)                          // dimse1
	w.writeInt16(76, 0)                          // dimse2
	w.writeInt16(77, 0)                          // dimtad
	w.writeInt16(78, 0)                          // dimzin
	w.writeInt16(79, 0)                          // dimazin
	w.writeInt16(170, 0)                         // dimalt
	w.writeInt16(171, 2)                         // dimaltd
	w.writeInt16(172, 0)                         // dimtofl
	w.writeInt16(173, 0)                         // dimsah
	w.writeInt16(174, 0)                         // dimtix
	w.writeInt16(175, 0)                         // dimsoxd
	w.writeInt16(176, 0)                         // dimclrd
	w.writeInt16(177, 0)                         // dimclre
	w.writeInt16(178, 0)                         // dimclrt
	w.writeInt16(179, 0)                         // dimadec
	w.writeInt16(271, 4)                         // dimdec
	w.writeInt16(272, 4)                         // dimtdec
	w.writeInt16(273, 2)                         // dimaltu
	w.writeInt16(274, 2)                         // dimalttd
	w.writeInt16(275, 0)                         // dimaunit
	w.writeInt16(276, 0)                         // dimfrac
	w.writeInt16(277, 2)                         // dimlunit
	w.writeInt16(278, '.')                       // dimdsep
	w.writeInt16(279, 0)                         // dimtmove
	w.writeInt16(280, 0)                         // dimjust
	w.writeInt16(281, 0)                         // dimsd1
	w.writeInt16(282, 0)                         // dimsd2
	w.writeInt16(283, 1)                         // dimtolj
	w.writeInt16(284, 0)                         // dimtzin
	w.writeInt16(285, 0)                         // dimaltz
	w.writeInt16(286, 0)                         // dimaltttz
	w.writeInt16(288, 0)                         // dimupt
	w.writeInt16(289, 3)                         // dimatfit
	if h, ok := w.textStyleMap["STANDARD"]; ok { // dimtxsty
		w.writeString(340, toHexStr(h))
	}
}

func (w *Writer) writeTables(iface Interface) {
	w.writeString(0, "TABLE")
	w.writeString(2, "VPORT")
	w.writeString(5, "8")
	w.writeString(330, "0")
	w.writeString(100, "AcDbSymbolTable")
	w.writeInt16(70, 1) // end table def
	// VPORT
	w.dimstyleStd = false
	w.writeVport()
	w.writeString(0, "ENDTAB")
	// LTYPE
	w.writeString(0, "TABLE")
	w.writeString(2, "LTYPE")
	w.writeString(5, "5")
	w.writeString(330, "0")
	w.writeString(100, "AcDbSymbolTable")
	w.writeInt16(70, 4) // end table def
	// Mandatory linetypes
	for _, lt := range []struct{ handle, name, desc string }{
		{"14", "ByBlock", ""}, {"15", "ByLayer", ""}, {"16", "Continuous", "Solid line"},
	} {
		w.writeString(0, "LTYPE")
		w.writeString(5, lt.handle)
		w.writeString(330, "5")
		w.writeString(100, "AcDbSymbolTableRecord")
		w.writeString(100, "AcDbLinetypeTableRecord")
		w.writeString(2, lt.name)
		w.writeInt16(70, 0)
		w.writeString(3, lt.desc)
		w.writeInt16(72, 65)
		w.writeInt16(73, 0)
		w.writeDouble(40, 0.0)
	}
	// Application linetypes
	iface.WriteLTypes(w)
	w.writeString(0, "ENDTAB")
	// LAYER
	w.writeString(0, "TABLE")
	w.writeString(2, "LAYER")
	w.writeString(5, "2")
	w.writeString(330, "0")
	w.writeString(100, "AcDbSymbolTable")
	w.writeInt16(70, 1) // end table def
	w.wlayer0 = false
	iface.WriteLayers(w)
	if !w.wlayer0 {
		w.WriteLayer(&Layer{Name: "0"})
	}
	w.writeString(0, "ENDTAB")
	// STYLE
	w.writeString(0, "TABLE")
	w.writeString(2, "STYLE")
	w.writeString(5, "3")
	w.writeString(330, "0")
	w.writeString(100, "AcDbSymbolTable")
	w.writeInt16(70, 3) // end table def
	w.dimstyleStd = false
	iface.WriteTextstyles(w)
	if !w.dimstyleStd {
		w.WriteTextstyle(&Textstyle{Name: "Standard", Font: "txt"})
	}
	w.writeString(0, "ENDTAB")

	for _, t := range []struct{ name, handle string }{{"VIEW", "6"}, {"UCS", "7"}} {
		w.writeString(0, "TABLE")
		w.writeString(2, t.name)
		w.writeString(5, t.handle)
		w.writeString(330, "0")
		w.writeString(100, "AcDbSymbolTable")
		w.writeInt16(70, 0) // end table def
		w.writeString(0, "ENDTAB")
	}

	w.writeString(0, "TABLE")
	w.writeString(2, "APPID")
	w.writeString(5, "9")
	w.writeString(330, "0")
	w.writeString(100, "AcDbSymbolTable")
	w.writeInt16(70, 1) // end table def
	w.writeString(0, "APPID")
	w.writeString(5, "12")
	w.writeString(330, "9")
	w.writeString(100, "AcDbSymbolTableRecord")
	w.writeString(100, "AcDbRegAppTableRecord")
	w.writeString(2, "ACAD")
	w.writeInt16(70, 0)
	w.writeString(0, "ENDTAB")

	w.writeString(0, "TABLE")
	w.writeString(2, "DIMSTYLE")
	w.writeString(5, "A")
	w.writeString(330, "0")
	w.writeString(100, "AcDbSymbolTable")
	w.writeInt16(70, 1) // end table def
	w.writeString(100, "AcDbDimStyleTable")
	w.writeInt16(71, 1) // end table def
	w.dimstyleStd = false
	w.writeDimstyle()
	w.writeString(0, "ENDTAB")

	w.writeString(0, "TABLE")
	w.writeString(2, "BLOCK_RECORD")
	w.writeString(5, "1")
	w.writeString(330, "0")
	w.writeString(100, "AcDbSymbolTable")
	w.writeInt16(70, 2) // end table def
	for _, br := range []struct{ handle, name string }{{"1F", "*Model_Space"}, {"1E", "*Paper_Space"}} {
		w.writeString(0, "BLOCK_RECORD")
		w.writeString(5, br.handle)
		w.writeString(330, "1")
		w.writeString(100, "AcDbSymbolTableRecord")
		w.writeString(100, "AcDbBlockTableRecord")
		w.writeString(2, br.name)
		w.writeInt16(70, 0)
		w.writeInt16(280, 1)
		w.writeInt16(281, 0)
	}
	w.writeString(0, "ENDTAB")
}

func (w *Writer) writeBlocks() {
	blocks := []struct{ handle, owner, name, endHandle string }{
		{"20", "1F", "*Model_Space", "21"},
		{"1C", "1B", "*Paper_Space", "1D"},
	}
	for _, b := range blocks {
		w.writeString(0, "BLOCK")
		w.writeString(5, b.handle)
		w.writeString(330, b.owner)
		w.writeString(100, "AcDbEntity")
		w.writeString(8, "0")
		w.writeString(100, "AcDbBlockBegin")
		w.writeString(2, b.name)
		w.writeInt16(70, 0)
		w.writeDouble(10, 0.0)
		w.writeDouble(20, 0.0)
		w.writeDouble(30, 0.0)
		w.writeString(3, b.name)
		w.writeString(1, "")
		w.writeString(0, "ENDBLK")
		w.writeString(5, b.endHandle)
		w.writeString(330, "1F")
		w.writeString(100, "AcDbEntity")
		w.writeString(8, "0")
		w.writeString(100, "AcDbBlockEnd")
	}
}

func (w *Writer) writeObjects() {
	w.writeString(0, "DICTIONARY")
	w.writeString(5, "C")
	w.writeString(330, "0")
	w.writeString(100, "AcDbDictionary")
	w.writeInt16(281, 1)
	w.writeString(3, "ACAD_GROUP")
	w.writeString(350, "D")
	w.writeString(0, "DICTIONARY")
	w.writeString(5, "D")
	w.writeString(330, "C")
	w.writeString(100, "AcDbDictionary")
	w.writeInt16(281, 1)
}

// WriteLine writes a LINE.
func (w *Writer) WriteLine(ent *Line) {
	w.writeString(0, "LINE")
	w.writeEntity(&ent.Entity)
	w.writeString(100, "AcDbLine")
	w.writeDouble(10, ent.BasePoint.X)
	w.writeDouble(20, ent.BasePoint.Y)
	if ent.BasePoint.Z != 0.0 || ent.SecPoint.Z != 0.0 {
		w.writeDouble(30, ent.BasePoint.Z)
		w.writeDouble(11, ent.SecPoint.X)
		w.writeDouble(21, ent.SecPoint.Y)
		w.writeDouble(31, ent.SecPoint.Z)
	} else {
		w.writeDouble(11, ent.SecPoint.X)
		w.writeDouble(21, ent.SecPoint.Y)
	}
}

// WriteArc writes an ARC.
func (w *Writer) WriteArc(ent *Arc) {
	w.writeString(0, "ARC")
	w.writeEntity(&ent.Entity)
	w.writeString(100, "AcDbCircle")
	w.writeDouble(10, ent.BasePoint.X)
	w.writeDouble(20, ent.BasePoint.Y)
	if ent.BasePoint.Z != 0.0 {
		w.writeDouble(30, ent.BasePoint.Z)
	}
	w.writeDouble(40, ent.Radius)
	w.writeString(100, "AcDbArc")
	w.writeDouble(50, ent.StaAngle*ARAD)
	w.writeDouble(51, ent.EndAngle*ARAD)
}

// WritePolyline writes a POLYLINE and its vertices.
func (w *Writer) WritePolyline(ent *Polyline) {
	w.writeString(0, "POLYLINE")
	w.writeEntity(&ent.Entity)
	is3d := false
	if ent.Flags&8 != 0 || ent.Flags&16 != 0 {
		w.writeString(100, "AcDb3dPolyline")
		is3d = true
	} else {
		w.writeString(100, "AcDb2dPolyline")
	}
	w.writeDouble(10, 0.0)
	w.writeDouble(20, 0.0)
	w.writeDouble(30, 0.0)
	w.writeInt16(70, ent.Flags)

	for _, v := range ent.Vertices {
		w.writeString(0, "VERTEX")
		w.writeEntity(&ent.Entity)
		w.writeString(100, "AcDbVertex")
		if is3d {
			w.writeString(100, "AcDb3dPolylineVertex")
		} else {
			w.writeString(100, "AcDb2dVertex")
		}
		w.writeDouble(10, v.BasePoint.X)
		w.writeDouble(20, v.BasePoint.Y)
		w.writeDouble(30, v.BasePoint.Z)
		if v.Bulge != 0 {
			w.writeDouble(42, v.Bulge)
		}
	}
	w.writeString(0, "SEQEND")
	w.writeEntity(&ent.Entity)
}

// WriteSpline writes a SPLINE.
func (w *Writer) WriteSpline(ent *Spline) {
	w.writeString(0, "SPLINE")
	w.writeEntity(&ent.Entity)
	w.writeString(100, "AcDbSpline")
	w.writeDouble(210, 0)
	w.writeDouble(220, 0)
	w.writeDouble(230, 0)
	w.writeInt16(70, ent.Flags)
	w.writeInt16(71, ent.Degree)
	w.writeInt16(72, len(ent.Knots))
	w.writeInt16(73, len(ent.Controls))
	w.writeInt16(74, 0)
	w.writeDouble(42, 0.0000001)
	w.writeDouble(43, 0.0000001)
	for _, k := range ent.Knots {
		w.writeDouble(40, k)
	}
	for _, wt := range ent.Weights {
		w.writeDouble(41, wt)
	}
	for _, c := range ent.Controls {
		w.writeDouble(10, c.X)
		w.writeDouble(20, c.Y)
		w.writeDouble(30, c.Z)
	}
}

// WriteDimension writes a DIMENSION.
func (w *Writer) WriteDimension(ent *Dimension) {
	w.writeString(0, "DIMENSION")
	w.writeEntity(&ent.Entity)
	w.writeString(100, "AcDbDimension")
	w.writeDouble(10, ent.Def.X)
	w.writeDouble(20, ent.Def.Y)
	w.writeDouble(30, ent.Def.Z)
	w.writeDouble(11, ent.TextPoint.X)
	w.writeDouble(21, ent.TextPoint.Y)
	w.writeDouble(31, ent.TextPoint.Z)
	var typ int
	switch ent.Kind {
	case DimAligned:
		typ = 1
	case DimLinear:
		typ = 0
	case DimRadial:
		typ = 4
	case DimDiametric:
		typ = 3
	case DimAngular:
		typ = 2
	}
	w.writeInt16(70, typ+32)
	if ent.Text != "" {
		w.writeString(1, ent.Text)
	}
	w.writeInt16(71, 5)
	w.writeString(3, "STANDARD")
	w.writeDouble(53, 0)
	w.writeDouble(210, 0)
	w.writeDouble(220, 0)
	w.writeDouble(230, 1.0)
	if ent.HasActual {
		w.writeDouble(42, ent.Actual)
	}

	switch ent.Kind {
	case DimAligned, DimLinear:
		w.writeString(100, "AcDbAlignedDimension")
		w.writeDouble(13, ent.Pt3.X)
		w.writeDouble(23, ent.Pt3.Y)
		w.writeDouble(33, ent.Pt3.Z)
		w.writeDouble(14, ent.Pt4.X)
		w.writeDouble(24, ent.Pt4.Y)
		w.writeDouble(34, ent.Pt4.Z)
		if ent.Kind == DimLinear {
			if ent.Angle != 0 {
				w.writeDouble(50, ent.Angle)
			}
			if ent.Oblique != 0 {
				w.writeDouble(52, ent.Oblique)
			}
			w.writeString(100, "AcDbRotatedDimension")
		}
	case DimRadial:
		w.writeString(100, "AcDbRadialDimension")
		w.writeDouble(15, ent.Pt5.X)
		w.writeDouble(25, ent.Pt5.Y)
		w.writeDouble(35, ent.Pt5.Z)
		w.writeDouble(40, 0.0)
	case DimDiametric:
		w.writeString(100, "AcDbDiametricDimension")
		w.writeDouble(15, ent.Pt5.X)
		w.writeDouble(25, ent.Pt5.Y)
		w.writeDouble(35, ent.Pt5.Z)
		w.writeDouble(40, 0.0)
	case DimAngular:
		w.writeString(100, "AcDb2LineAngularDimension")
		w.writeDouble(13, ent.Pt3.X)
		w.writeDouble(23, ent.Pt3.Y)
		w.writeDouble(33, ent.Pt3.Z)
		w.writeDouble(14, ent.Pt4.X)
		w.writeDouble(24, ent.Pt4.Y)
		w.writeDouble(34, ent.Pt4.Z)
		w.writeDouble(15, ent.Pt5.X)
		w.writeDouble(25, ent.Pt5.Y)
		w.writeDouble(35, ent.Pt5.Z)
		w.writeDouble(16, ent.Pt6.X)
		w.writeDouble(26, ent.Pt6.Y)
		w.writeDouble(36, ent.Pt6.Z)
	}
}

// WriteText writes a TEXT.
func (w *Writer) WriteText(ent *Text) {
	w.writeString(0, "TEXT")
	w.writeEntity(&ent.Entity)
	w.writeString(100, "AcDbText")
	w.writeDouble(10, ent.BasePoint.X)
	w.writeDouble(20, ent.BasePoint.Y)
	w.writeDouble(30, ent.BasePoint.Z)
	w.writeDouble(40, ent.Height)
	w.writeString(1, ent.Text)
	w.writeDouble(50, ent.Angle)
	w.writeDouble(41, 1)
	w.writeDouble(51, 0)
	w.writeString(7, ent.Style)
	w.writeInt16(71, 0)
	if ent.AlignH != HLeft {
		w.writeInt16(72, ent.AlignH)
	}
	if ent.AlignH != HLeft || ent.AlignV != VBaseLine {
		w.writeDouble(11, ent.SecPoint.X)
		w.writeDouble(21, ent.SecPoint.Y)
		w.writeDouble(31, ent.SecPoint.Z)
	}
	w.writeDouble(210, 0)
	w.writeDouble(220, 0)
	w.writeDouble(230, 1)
	w.writeString(100, "AcDbText")
	if ent.AlignV != VBaseLine {
		w.writeInt16(73, ent.AlignV)
	}
}
