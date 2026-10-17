package ui

// The triangle mesh export dialog (an extension): File > Export >
// Triangle Mesh... pops up the options the mesh is made with, so that a
// preset or a tolerance is chosen where the export is, and then asks
// where to write it. The settings are the configuration's, the same
// ones the preferences screen shows.

import (
	"strconv"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Keys of the fields of the mesh export dialog.
const (
	meshFieldQuality = "quality"
	meshFieldChord   = "chord"
	meshFieldAngle   = "angle"
	meshFieldSeg     = "segments"
	meshFieldStrict  = "strict"
)

// exportMeshForm is the dialog of the mesh options, set to the current
// ones. Taking a preset fills in its tolerances, and changing a
// tolerance by hand makes the quality custom.
func (ss *Application) exportMeshForm() platform.FormDialog {
	f := ss.Platform.CreateFormDialog(ss.GW.window)
	f.SetTitle("Export Triangle Mesh")
	f.SetButtons("&Export...", "Cancel")
	cur := sketch.QualityOf(ss.ExportChordTol, ss.ExportAngleTol, ss.ExportMaxSegments)
	f.AddChoice(meshFieldQuality, "Quality", []string{"Draft", "Standard", "Fine", "Custom"}, int(cur))
	f.AddEntry(meshFieldChord, "Chord tolerance", cFormatG(6, ss.ExportChordTol), "mm")
	f.AddEntry(meshFieldAngle, "Angular tolerance", cFormatG(6, ss.ExportAngleTol), "degrees, 0 for none")
	f.AddEntry(meshFieldSeg, "Max linear segments", strconv.Itoa(ss.ExportMaxSegments), "")
	f.AddCheck(meshFieldStrict, "Refuse a mesh that isn't watertight or crosses itself", ss.ExportStrictMesh)
	f.OnChange(func(key, value string) platform.FormValues {
		custom := strconv.Itoa(int(sketch.MeshQualityCustom))
		switch key {
		case meshFieldQuality:
			q, err := strconv.Atoi(value)
			if err != nil || sketch.MeshQuality(q) == sketch.MeshQualityCustom {
				return nil
			}
			chord, angle, seg := sketch.MeshQualitySettings(sketch.MeshQuality(q))
			return platform.FormValues{meshFieldChord: cFormatG(6, chord), meshFieldAngle: cFormatG(6, angle),
				meshFieldSeg: strconv.Itoa(seg)}
		case meshFieldChord, meshFieldAngle, meshFieldSeg:
			return platform.FormValues{meshFieldQuality: custom}
		}
		return nil
	})
	return f
}

// takeExportMeshForm puts the dialog's values into the settings; the
// fields are clamped as the preferences screen clamps them.
func (ss *Application) takeExportMeshForm(v platform.FormValues) {
	ss.ExportChordTol = geom.Max(0.0, atof(v[meshFieldChord]))
	ss.ExportAngleTol = min(180, geom.Max(0.0, atof(v[meshFieldAngle])))
	ss.ExportMaxSegments = min(1000, max(7, atoi(v[meshFieldSeg])))
	ss.ExportStrictMesh = v[meshFieldStrict] == "1"
	ss.applyEngineSettings()
}

// menuExportMesh asks for the mesh options and then for the file to
// write, and writes it.
func (ss *Application) menuExportMesh() {
	v, ok := ss.exportMeshForm().RunModal()
	if !ok {
		return
	}
	ss.takeExportMeshForm(v)
	if filename, ok := ss.exportDialog(platform.MeshFileFilters, "ExportMesh"); ok {
		ss.Report(ss.ExportMeshTo(filename))
	}
}
