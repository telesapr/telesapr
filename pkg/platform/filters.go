package platform

// The file types spacesolve reads and writes (after C++ gui.cpp).
var (
	// ModelFileFilters are the documents spacesolve opens and saves.
	ModelFileFilters = []FileFilter{
		{"spacesolve models", []string{"spcs"}},
	}
	// ScriptFileFilters are the scripts the application runs.
	ScriptFileFilters = []FileFilter{
		{Name: "Go scripts", Extensions: []string{"go"}},
	}
	// LinkFileFilters are the files a link group links. SolveSpace
	// models are not among them: they are imported first.
	LinkFileFilters = []FileFilter{
		{"ALL", []string{"spcs", "step", "stp", "emn", "stl"}},
		{"spacesolve models", []string{"spcs"}},
		{"STEP model, as a reference mesh", []string{"step", "stp"}},
		{"IDF circuit board", []string{"emn"}},
		{"STL triangle mesh", []string{"stl"}},
	}
	RasterFileFilters = []FileFilter{
		{"PNG image", []string{"png"}},
	}
	MeshFileFilters = []FileFilter{
		{"STL mesh", []string{"stl"}},
		{"Wavefront OBJ mesh", []string{"obj"}},
		{"Three.js-compatible mesh, with viewer", []string{"html"}},
		{"Three.js-compatible mesh, mesh only", []string{"js"}},
		{"VRML text file", []string{"wrl"}},
	}
	SurfaceFileFilters = []FileFilter{
		{"STEP file", []string{"step", "stp"}},
	}
	VectorFileFilters = []FileFilter{
		{"PDF file", []string{"pdf"}},
		{"Encapsulated PostScript", []string{"eps", "ps"}},
		{"Scalable Vector Graphics", []string{"svg"}},
		{"STEP file", []string{"step", "stp"}},
		{"DXF file (AutoCAD 2007)", []string{"dxf"}},
		{"HPGL file", []string{"plt", "hpgl"}},
		{"G Code", []string{"ngc", "txt"}},
	}
	Vector3dFileFilters = []FileFilter{
		{"STEP file", []string{"step", "stp"}},
		{"DXF file (AutoCAD 2007)", []string{"dxf"}},
	}
	// ImportFileFilters are what File > Import reads: drawings into
	// the sketch, and SolveSpace models, which are converted.
	ImportFileFilters = []FileFilter{
		{"ALL", []string{"dxf", "dwg", "slvs"}},
		{"AutoCAD DXF and DWG files", []string{"dxf", "dwg"}},
		{"SolveSpace models", []string{"slvs"}},
	}
	CsvFileFilters = []FileFilter{
		{"Comma-separated values", []string{"csv"}},
	}
)
