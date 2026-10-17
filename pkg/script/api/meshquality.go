package api

import (
	"errors"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// MeshQuality names a preset of the mesh export settings (ChordTol,
// AngleTol and MaxSegments of ExportOptions): MeshDraft, MeshStandard,
// MeshFine, or MeshCustom for any settings that match none.
type MeshQuality = sketch.MeshQuality

// The mesh export presets, as the export dialog offers them.
const (
	// MeshDraft is a coarse, quick mesh: chord tolerance 0.5 mm, no
	// angular tolerance, 32 segments.
	MeshDraft = sketch.MeshQualityDraft
	// MeshStandard is the default settings: 0.1 mm, no angular
	// tolerance, 64 segments.
	MeshStandard = sketch.MeshQualityStandard
	// MeshFine follows curves closely: 0.02 mm, 10 degrees, 256
	// segments.
	MeshFine = sketch.MeshQualityFine
	// MeshCustom is what Quality reports for settings that match no
	// preset; it can't be asked for with SetMeshQuality.
	MeshCustom = sketch.MeshQualityCustom
)

// Quality is the preset whose chord tolerance, angular tolerance and
// segment limit the options hold, or MeshCustom when they match none.
func (o ExportOptions) Quality() MeshQuality {
	return sketch.QualityOf(o.ChordTol, o.AngleTol, o.MaxSegments)
}

// SetMeshQuality sets the chord tolerance, angular tolerance and
// segment limit of the export options to those of the preset q
// (MeshDraft, MeshStandard or MeshFine) and leaves the other options
// as they are. MeshCustom is not a preset, so it fails; set the fields
// with SetExportOptions instead.
func (m FileMenu) SetMeshQuality(q MeshQuality) error {
	if q != MeshDraft && q != MeshStandard && q != MeshFine {
		return errors.New("custom is not a preset; set the fields with SetExportOptions")
	}
	o := m.ExportOptions()
	o.ChordTol, o.AngleTol, o.MaxSegments = sketch.MeshQualitySettings(q)
	return m.SetExportOptions(o)
}
