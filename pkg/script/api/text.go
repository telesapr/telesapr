package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// Fonts lists the TrueType fonts a text can be set in, as the names of
// their font files: what Text and Entity.SetFont take, and what the
// text window lists.
func (m SketchMenu) Fonts() (fonts []string) {
	m.d.read(func(sk *sketch.Sketch) { fonts = sk.FontFiles() })
	return fonts
}

// textRequest is the request of a text drawn in the sketch, or a
// failure.
func (e Entity) textRequest(sk *sketch.Sketch) core.HRequest {
	ent := e.get(sk)
	if ent.Type != core.EntityTTFText || !ent.H.IsFromRequest() {
		fail("%s isn't a text drawn in the sketch", e.describe(sk))
	}
	return ent.H.Request()
}

// Text is the string of a text drawn with Sketch.Text.
func (e Entity) Text() (text string) {
	e.d.read(func(sk *sketch.Sketch) {
		e.textRequest(sk)
		text = e.get(sk).Str
	})
	return text
}

// SetText changes the string of a text. The text keeps its corner,
// direction and height, and the constraints on them, and takes the
// width the new string has; the text window's [change] does the same.
func (e Entity) SetText(text string) {
	e.d.change(func(sk *sketch.Sketch) {
		if err := sk.SetTextString(e.textRequest(sk), text); err != nil {
			fail("%v", err)
		}
	})
}

// Font is the name of the font file a text is set in; Sketch.Fonts
// lists the names.
func (e Entity) Font() (font string) {
	e.d.read(func(sk *sketch.Sketch) {
		e.textRequest(sk)
		font = e.get(sk).Font
	})
	return font
}

// SetFont sets a text in another font, given by the name of its font
// file as Sketch.Fonts lists them; "" is the font built in. It fails for
// a font the program doesn't have.
func (e Entity) SetFont(font string) {
	if font == "" {
		font = ttf.EmbeddedFont
	}
	e.d.change(func(sk *sketch.Sketch) {
		if err := sk.SetTextFont(e.textRequest(sk), font); err != nil {
			fail("%v", err)
		}
	})
}

// Kerning reports whether a text is set with the font's kerning, the
// spacing the font gives to particular pairs of letters.
func (e Entity) Kerning() (kerning bool) {
	e.d.read(func(sk *sketch.Sketch) {
		e.textRequest(sk)
		kerning = e.get(sk).ExtraPoints != 0
	})
	return kerning
}

// SetKerning turns the kerning of a text on or off, as the text
// window's "apply kerning" does.
func (e Entity) SetKerning(kerning bool) {
	e.d.change(func(sk *sketch.Sketch) {
		if err := sk.SetTextKerning(e.textRequest(sk), kerning); err != nil {
			fail("%v", err)
		}
	})
}
