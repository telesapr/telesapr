package ttf

import (
	_ "embed"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The port of ttf.cpp: TtfFont and TtfFontList.

// builtinFont is C++'s builtin font resource (solvespace/res/fonts), Bitstream
// Vera Sans, whose license allows bundling it with software.
//
//go:embed fonts/BitstreamVeraSans-Roman-builtin.ttf
var builtinFont []byte

const builtinFontResource = "res://fonts/BitstreamVeraSans-Roman-builtin.ttf"

// EmbeddedFont names the builtin font, as text requests store it: by
// its file's basename, as every font is looked up (C++
// Platform::embeddedFont off Windows; on Windows C++ stores the
// resource path, which matches no font).
const EmbeddedFont = "BitstreamVeraSans-Roman-builtin.ttf"

// DefaultFonts is the font list of a sketch that has none of its own:
// the builtin font alone, until the program sets it once at startup
// to the platform's fonts, as the application does, or a test's
// TestMain to the fonts its fixtures use.
var DefaultFonts = NewFontList(nil)

// Font is a TtfFont.
type Font struct {
	FontFile  string // or the resource path, as res://<path>
	Name      string
	face      *Face
	CapHeight float64
}

// FontList is the fonts of a set of font files and the builtin font,
// read when first used.
type FontList struct {
	files  []string
	l      []Font
	loaded bool
}

// NewFontList is the font list of files, read when first used.
func NewFontList(files []string) *FontList {
	return &FontList{files: append([]string(nil), files...)}
}

// FontFileBaseName is the basename of the font's file; that's how the
// requests and entities that reference it store it.
func (tf *Font) FontFileBaseName() string {
	name := tf.FontFile
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// IsResource is whether the font is the builtin one.
func (tf *Font) IsResource() bool {
	return strings.HasPrefix(tf.FontFile, "res://")
}

// load is LoadFromFile and LoadFromResource.
func (tf *Font) load(keepOpen bool) bool {
	var data []byte
	if tf.IsResource() {
		data = builtinFont
	} else {
		var err error
		if data, err = os.ReadFile(tf.FontFile); err != nil {
			return false
		}
	}
	face, err := NewFace(data)
	if err != nil {
		return false
	}
	tf.face = face
	return tf.extractTTFData(keepOpen)
}

// extractTTFData is ExtractTTFData: the font's name and cap height.
func (tf *Font) extractTTFData(keepOpen bool) bool {
	if err := tf.face.SelectUnicodeCharmap(); err != nil {
		tf.face = nil
		return false
	}

	tf.Name = tf.face.FamilyName + " (" + tf.face.StyleName + ")"

	// We always ask Freetype to give us a unit size character.
	// It uses fixed point; put the unit size somewhere in the middle of the dynamic
	// range of its 26.6 fixed point type, and adjust the factors so that the unit
	// matches cap height.
	const size = 1 << 16
	if err := tf.face.RequestRealDimSize(size, size, 128, 128); err != nil {
		tf.face = nil
		return false
	}

	gid := tf.face.CharIndex('A')
	if gid == 0 {
		// Assume the cap height is the same as the requested height
		// (this is likely wrong).
		tf.CapHeight = size
	} else {
		g, err := tf.face.LoadGlyph(gid)
		if err != nil {
			tf.face = nil
			return false
		}
		tf.CapHeight = float64(g.Outline.CBox().YMax)
	}

	// If we just wanted to get the font's name and figure out if it's
	// actually usable, close it now.
	if !keepOpen {
		tf.face = nil
	}
	return true
}

// LoadAll reads the font files, then the builtin font, and sorts them
// by name, keeping one font of each name.
func (fl *FontList) LoadAll() {
	if fl.loaded {
		return
	}
	for _, file := range fl.files {
		tf := Font{FontFile: file}
		if tf.load(false) {
			fl.l = append(fl.l, tf)
		}
	}

	// Add builtin font to end of font list so it is displayed first in the UI
	tf := Font{FontFile: builtinFontResource}
	if tf.load(false) {
		fl.l = append(fl.l, tf)
	}

	// Sort fonts according to their actual name, not filename.
	srf.CxxSort(fl.l, func(a, b *Font) bool { return a.Name < b.Name })

	// Filter out fonts with the same family and style name (std::unique).
	if len(fl.l) > 0 {
		out := 0
		for i := 1; i < len(fl.l); i++ {
			if fl.l[out].Name != fl.l[i].Name {
				out++
				fl.l[out] = fl.l[i]
			}
		}
		fl.l = fl.l[:out+1]
	}

	fl.loaded = true
}

// List is the fonts, loaded if they weren't.
func (fl *FontList) List() []Font {
	fl.LoadAll()
	return fl.l
}

// LoadFont is the font whose file's basename is font, opened, or nil.
func (fl *FontList) LoadFont(font string) *Font {
	fl.LoadAll()
	for i := range fl.l {
		tf := &fl.l[i]
		if tf.FontFileBaseName() != font {
			continue
		}
		if tf.face == nil {
			tf.load(true)
		}
		return tf
	}
	return nil
}

// PlotString adds the outlines of str, set in font, to sbl; with no
// text or no font, a big X marks the error.
func (fl *FontList) PlotString(font, str string, sbl *srf.SBezierList, kerning bool,
	origin, u, v geom.Vector) {
	tf := fl.LoadFont(font)
	if str != "" && tf != nil {
		tf.PlotString(str, sbl, kerning, origin, u, v)
	} else {
		sbl.Add(srf.SBezierFrom2(origin, origin.Plus(u).Plus(v)))
		sbl.Add(srf.SBezierFrom2(origin.Plus(v), origin.Plus(u)))
	}
}

// AspectRatio is the width of str, set in font, over its cap height;
// 0 when the font can't be loaded.
func (fl *FontList) AspectRatio(font, str string, kerning bool) float64 {
	if tf := fl.LoadFont(font); tf != nil {
		return tf.AspectRatio(str, kerning)
	}
	return 0
}

type outlineData struct {
	origin, u, v geom.Vector
	beziers      *srf.SBezierList
	factor       float32 // ratio between freetype and solvespace coordinates
	bx           int64   // x offset of the current glyph
	px, py       int64   // current point
}

func (d *outlineData) transform(x, y int64) geom.Vector {
	r := d.origin
	r = r.Plus(d.u.ScaledBy(float64(float32(d.bx+x) * d.factor)))
	r = r.Plus(d.v.ScaledBy(float64(float32(y) * d.factor)))
	return r
}

// PlotString is TtfFont::PlotString.
func (tf *Font) PlotString(str string, sbl *srf.SBezierList, kerning bool,
	origin, u, v geom.Vector) {
	if tf.face == nil {
		panic("Expected font face to be loaded")
	}
	face := tf.face

	var dx int64
	var prevGid uint32
	for _, cid := range core.ReadUTF8(str) {
		gid := face.CharIndex(cid)
		if gid == 0 {
			// Using the CID as the GID.
			gid = cid
		}

		g, err := face.LoadGlyph(gid)
		if err != nil {
			return
		}

		// A point that has x = xMin should be plotted at (dx0 + lsb); fix
		// up our x-position so that the curve-generating code will put
		// stuff at the right place.
		cbox := g.Outline.CBox()

		// Apply Kerning, if any:
		if kerning {
			dx += face.Kerning(prevGid, gid).X
		}

		bx := dx - cbox.XMin
		// Yes, this is what FreeType calls left-side bearing.
		bx += g.HoriBearingX

		d := &outlineData{
			origin:  origin,
			u:       u,
			v:       v,
			beziers: sbl,
			factor:  float32(1.0 / tf.CapHeight),
			bx:      bx,
		}
		// C++ only logs a failure, keeping what was decomposed.
		_ = g.Outline.Decompose(OutlineFuncs{
			MoveTo: func(p Vector) {
				d.px, d.py = p.X, p.Y
			},
			LineTo: func(p Vector) {
				d.beziers.Add(srf.SBezierFrom2(
					d.transform(d.px, d.py),
					d.transform(p.X, p.Y)))
				d.px, d.py = p.X, p.Y
			},
			ConicTo: func(c, p Vector) {
				d.beziers.Add(srf.SBezierFrom3(
					d.transform(d.px, d.py),
					d.transform(c.X, c.Y),
					d.transform(p.X, p.Y)))
				d.px, d.py = p.X, p.Y
			},
			CubicTo: func(c1, c2, p Vector) {
				d.beziers.Add(srf.SBezierFrom4(
					d.transform(d.px, d.py),
					d.transform(c1.X, c1.Y),
					d.transform(c2.X, c2.Y),
					d.transform(p.X, p.Y)))
				d.px, d.py = p.X, p.Y
			},
		})

		// And we're done, so advance our position by the requested
		// advance width, plus the user-requested extra advance.
		dx += g.AdvanceX
		prevGid = gid
	}
}

// AspectRatio is TtfFont::AspectRatio: since the font is at unit size,
// the advance of the whole string.
func (tf *Font) AspectRatio(str string, kerning bool) float64 {
	if tf.face == nil {
		panic("Expected font face to be loaded")
	}
	face := tf.face

	var dx float64
	var prevGid uint32
	for _, chr := range core.ReadUTF8(str) {
		gid := face.CharIndex(chr)
		g, err := face.LoadGlyph(gid)
		if err != nil {
			break
		}
		if kerning {
			dx += float64(face.Kerning(prevGid, gid).X) / tf.CapHeight
		}
		dx += float64(g.AdvanceX) / tf.CapHeight
		prevGid = gid
	}
	return dx
}
