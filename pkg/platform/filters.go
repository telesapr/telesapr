package platform

// The file types SolveSpace reads and writes (C++ gui.cpp).
var (
	SolveSpaceModelFileFilters = []FileFilter{
		{"SolveSpace models", []string{"slvs"}},
	}
	SolveSpaceLinkFileFilters = []FileFilter{
		{"ALL", []string{"slvs", "emn", "stl"}},
		{"SolveSpace models", []string{"slvs"}},
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
	ImportFileFilters = []FileFilter{
		{"AutoCAD DXF and DWG files", []string{"dxf", "dwg"}},
	}
	CsvFileFilters = []FileFilter{
		{"Comma-separated values", []string{"csv"}},
	}
)
