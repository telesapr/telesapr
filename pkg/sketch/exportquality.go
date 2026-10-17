package sketch

import (
	"fmt"
	"strings"
)

// MeshQuality names a preset of the mesh export settings: the window,
// the command line and scripts all pick one by the same table.
type MeshQuality int

const (
	// MeshQualityDraft is a coarse mesh, quick to make.
	MeshQualityDraft MeshQuality = iota
	// MeshQualityStandard is the application's default settings, so a
	// fresh install shows it.
	MeshQualityStandard
	// MeshQualityFine follows curves closely, with an angular
	// tolerance.
	MeshQualityFine
	// MeshQualityCustom is any settings that match no preset.
	MeshQualityCustom
)

// String is the name of q: draft, standard, fine or custom.
func (q MeshQuality) String() string {
	switch q {
	case MeshQualityDraft:
		return "draft"
	case MeshQualityStandard:
		return "standard"
	case MeshQualityFine:
		return "fine"
	}
	return "custom"
}

// ParseMeshQuality is the preset named s, ignoring case. Custom is not
// a preset to ask for, so it is an error.
func ParseMeshQuality(s string) (MeshQuality, error) {
	for _, q := range [...]MeshQuality{MeshQualityDraft, MeshQualityStandard, MeshQualityFine} {
		if strings.EqualFold(s, q.String()) {
			return q, nil
		}
	}
	return MeshQualityCustom, fmt.Errorf("unknown mesh quality %q; use draft, standard or fine", s)
}

// MeshQualitySettings is the chord tolerance in export units, the
// angular tolerance in degrees and the segment limit of preset q.
// Standard equals the defaults of ThawExportSettings; custom has no
// values of its own and returns standard's.
func MeshQualitySettings(q MeshQuality) (chordMm, angleDeg float64, segments int) {
	switch q {
	case MeshQualityDraft:
		return 0.5, 0, 32
	case MeshQualityFine:
		return 0.02, 10, 256
	}
	return 0.1, 0, 64
}

// QualityOf is the preset whose settings are the given ones, or
// MeshQualityCustom. Settings are kept in single precision, so the
// chord tolerance is compared as one.
func QualityOf(chordMm, angleDeg float64, segments int) MeshQuality {
	for _, q := range [...]MeshQuality{MeshQualityDraft, MeshQualityStandard, MeshQualityFine} {
		c, a, n := MeshQualitySettings(q)
		if float32(c) == float32(chordMm) && float32(a) == float32(angleDeg) && n == segments {
			return q
		}
	}
	return MeshQualityCustom
}
