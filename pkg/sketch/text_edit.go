package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
)

// ErrNoRequest is what an edit returns for a request the sketch does
// not have.
var ErrNoRequest = errors.New("no such request")

// ErrNotTextRequest is what the text edits return for a request that
// is not a TrueType text.
var ErrNotTextRequest = errors.New("not a text request")

// textRequest is the text request hr, or an error. It is the one place
// the text edits check what they are given, so the window and a script
// refuse the same things.
func (sk *Sketch) textRequest(hr core.HRequest) (*Request, error) {
	r := sk.Request(hr)
	if r == nil {
		return nil, fmt.Errorf("request %08x: %w", hr.V, ErrNoRequest)
	}
	if r.Type != RequestTTFText {
		return nil, fmt.Errorf("request %08x: %w", hr.V, ErrNotTextRequest)
	}
	return r, nil
}

// SetTextString changes the string of a text request. Its points and
// the constraints on them stay: the text keeps its corner, direction
// and height, and takes the width the new string has at that height.
// The caller remembers the state for undo.
func (sk *Sketch) SetTextString(hr core.HRequest, s string) error {
	r, err := sk.textRequest(hr)
	if err != nil {
		return err
	}
	r.Str = s
	sk.MarkGroupDirty(r.Group, false)
	return nil
}

// SetTextFont changes the font of a text request to the font file whose
// basename is font. A font the program cannot open is refused, because
// the text would then be drawn as a mark for the missing font.
func (sk *Sketch) SetTextFont(hr core.HRequest, font string) error {
	r, err := sk.textRequest(hr)
	if err != nil {
		return err
	}
	if sk.FontList().LoadFont(font) == nil {
		return fmt.Errorf("the font '%s' is not there", font)
	}
	r.Font = font
	sk.MarkGroupDirty(r.Group, false)
	return nil
}

// SetTextKerning turns the kerning of a text request on or off. It is
// kept in the request's ExtraPoints, which a text has no other use for.
func (sk *Sketch) SetTextKerning(hr core.HRequest, kerning bool) error {
	r, err := sk.textRequest(hr)
	if err != nil {
		return err
	}
	r.ExtraPoints = 0
	if kerning {
		r.ExtraPoints = 1
	}
	sk.MarkGroupDirty(r.Group, false)
	return nil
}

// FontFiles is the basenames of the font files a text can be set in, as
// the text window lists them.
func (sk *Sketch) FontFiles() []string {
	fonts := sk.FontList().List()
	names := make([]string, len(fonts))
	for i := range fonts {
		names[i] = fonts[i].FontFileBaseName()
	}
	return names
}
