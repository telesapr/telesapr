package ui

// The triangle mesh export screen (an extension): File > Export >
// Triangle Mesh... shows the options the mesh is made with, so that a
// preset or a tolerance is chosen where the export is, and a link asks
// where to write it. The settings are the configuration's, the same
// ones the preferences screen shows.

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ShowExportMesh is the screen of the mesh export options.
func (tw *textWindow) ShowExportMesh() {
	ss := tw.ss
	tw.Printf(true, "%FtEXPORT TRIANGLE MESH%E")
	tw.Printf(true, "%Ft quality%E")
	cur := sketch.QualityOf(ss.ExportChordTol, ss.ExportAngleTol, ss.ExportMaxSegments)
	tw.Printf(false, "%Ba  %Fl%Ll%f%D%s draft%E  %Fl%Ll%f%D%s standard%E  %Fl%Ll%f%D%s fine%E  %Fl%Ll%f%D%s custom%E",
		lf(tw.ScreenChangeMeshQuality), int(sketch.MeshQualityDraft), radioMark(cur == sketch.MeshQualityDraft),
		lf(tw.ScreenChangeMeshQuality), int(sketch.MeshQualityStandard), radioMark(cur == sketch.MeshQualityStandard),
		lf(tw.ScreenChangeMeshQuality), int(sketch.MeshQualityFine), radioMark(cur == sketch.MeshQualityFine),
		lf(tw.ScreenChangeMeshQuality), int(sketch.MeshQualityCustom), radioMark(cur == sketch.MeshQualityCustom))

	tw.Printf(true, "%Ft chord tolerance (in mm)%E")
	tw.Printf(false, "%Ba   %@ %Fl%Ll%f%D[change]%E",
		ss.ExportChordTol, lf(tw.ScreenChangeExportChordTolerance), 0)
	tw.Printf(false, "%Ft angular tolerance (in degrees, 0 for none)%E")
	tw.Printf(false, "%Ba   %@ %Fl%Ll%f[change]%E",
		ss.ExportAngleTol, lf(tw.ScreenChangeExportAngleTol))
	tw.Printf(false, "%Ft max piecewise linear segments%E")
	tw.Printf(false, "%Ba   %d %Fl%Ll%f[change]%E",
		ss.ExportMaxSegments, lf(tw.ScreenChangeExportMaxSegments))

	tw.Printf(true, "  %Fd%f%Ll%s  refuse a mesh that isn't watertight or crosses itself%E",
		lf(tw.ScreenChangeExportStrictMesh), checkMark(ss.ExportStrictMesh))
	tw.Printf(false, "%Ft export scale factor 1:%# (preferences)%E", ss.ExportScale)

	tw.Printf(true, "%Fl%Lc%f[export mesh...]%E", lf(tw.ScreenExportMeshNow))
	tw.Printf(true, "%Fl%Ll%f(back to the groups)%E", lf(tw.ScreenHome))
}

// ScreenChangeMeshQuality takes a preset of the mesh settings; the
// custom one has none of its own, so clicking it changes nothing.
func (tw *textWindow) ScreenChangeMeshQuality(link int, v uint32) {
	q := sketch.MeshQuality(v)
	if q == sketch.MeshQualityCustom {
		return
	}
	ss := tw.ss
	ss.ExportChordTol, ss.ExportAngleTol, ss.ExportMaxSegments = sketch.MeshQualitySettings(q)
	ss.applyEngineSettings()
}

// ScreenChangeExportAngleTol edits the angular tolerance.
func (tw *textWindow) ScreenChangeExportAngleTol(link int, v uint32) {
	tw.ShowEditControl(3, cFormatG(6, tw.ss.ExportAngleTol))
	tw.edit.meaning = editExportAngleTol
}

// ScreenChangeExportStrictMesh toggles the refusal of an unsound mesh.
func (tw *textWindow) ScreenChangeExportStrictMesh(link int, v uint32) {
	tw.ss.ExportStrictMesh = !tw.ss.ExportStrictMesh
	tw.ss.applyEngineSettings()
}

// ScreenExportMeshNow asks where to write the mesh and writes it.
func (tw *textWindow) ScreenExportMeshNow(link int, v uint32) {
	ss := tw.ss
	if filename, ok := ss.exportDialog(platform.MeshFileFilters, "ExportMesh"); ok {
		ss.Report(ss.ExportMeshTo(filename))
	}
}

// EditControlDoneForExportMesh takes the edited angular tolerance, and
// reports whether the edit was the export screen's.
func (tw *textWindow) EditControlDoneForExportMesh(s string) bool {
	if tw.edit.meaning != editExportAngleTol {
		return false
	}
	ss := tw.ss
	ss.ExportAngleTol = min(180, geom.Max(0.0, atof(s)))
	ss.applyEngineSettings()
	return true
}
