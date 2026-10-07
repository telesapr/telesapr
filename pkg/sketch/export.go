package sketch

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// This file ports the triangle mesh export of solvespace/src/export.cpp:
// ExportMeshTo and ExportMeshAsStlTo, with the naked edge report of
// SolveSpaceUI::ShowNakedEdges. The other mesh formats (OBJ, three.js,
// VRML) aren't ported.

// ExportSettings are the user's export settings (C++
// SS.exportChordTol, exportMaxSegments and exportScale).
type ExportSettings struct {
	// ChordTol is the chord tolerance in export units.
	ChordTol    float64
	MaxSegments int
	// Scale is how many mm an export unit is.
	Scale float64
}

// ThawExportSettings reads the export settings, with C++'s defaults (C++
// SolveSpaceUI::Init).
func ThawExportSettings(settings core.Settings) ExportSettings {
	return ExportSettings{
		ChordTol:    settings.ThawFloat("ExportChordTolerance", 0.1),
		MaxSegments: int(settings.ThawInt("ExportMaxSegments", 64)),
		Scale:       settings.ThawFloat("ExportScale", 1.0),
	}
}

// ChordTolMm is the export chord tolerance in mm (C++
// SS.ExportChordTolMm).
func (es ExportSettings) ChordTolMm() float64 {
	return es.ChordTol / es.Scale
}

// ExportMeshTo writes the active group's mesh, regenerated at the export
// chord tolerance, to filename; the extension picks the format, STL or
// 3MF. Like C++, it reports a mesh that isn't watertight or
// self-intersects through Message, but still exports it (C++
// SolveSpaceUI::ExportMeshTo). As in C++, regenerating in export mode
// doesn't solve, so the sketch must have been generated since it was
// loaded or changed (C++ AfterNewFile does that on load).
func (sk *Sketch) ExportMeshTo(filename string, es ExportSettings) error {
	// C++ stays in export mode until the user next does something, and
	// then regenerates for display; here that happens on return.
	srf.SetExportMode(true, es.ChordTolMm(), es.MaxSegments)
	defer func() {
		srf.SetExportMode(false, 0, 0)
		sk.GenerateAllTyped(GenerateAll)
	}()
	sk.GenerateAllTyped(GenerateAll)

	g := sk.Group(sk.ActiveGroup())
	g.GenerateDisplayItems(sk)

	m := &g.DisplayMesh
	if len(m.Triangles) == 0 {
		return errors.New("Active group mesh is empty; nothing to export.")
	}

	var write func(w io.Writer) error
	switch {
	case hasExtension(filename, "stl"):
		write = func(w io.Writer) error { return ExportMeshAsStlTo(w, m, es.Scale) }
	case hasExtension(filename, "3mf"):
		write = func(w io.Writer) error { return ExportMeshAs3mfTo(w, m) }
	case hasExtension(filename, "obj"), hasExtension(filename, "js"),
		hasExtension(filename, "html"), hasExtension(filename, "wrl"):
		return fmt.Errorf("Exporting a mesh to '%s' isn't supported; try .stl or .3mf.", filename)
	default:
		return fmt.Errorf("Can't identify output file type from file extension of "+
			"filename '%s'; try .stl, .3mf, .obj, .js, .html.", filename)
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Couldn't write to '%s'", filename)
	}
	sk.ShowNakedEdges(true)
	bw := bufio.NewWriter(f)
	if err := write(bw); err != nil {
		_ = f.Close()
		return err
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// hasExtension reports whether filename has extension ext, ignoring
// case (C++ Platform::Path::HasExtension).
func hasExtension(filename, ext string) bool {
	ours := ""
	if dot := strings.LastIndexByte(filename, '.'); dot >= 0 {
		ours = filename[dot+1:]
	}
	return strings.EqualFold(ours, ext)
}

// ExportMeshAsStlTo writes m as a binary STL file, in export units of
// scale mm (C++ SolveSpaceUI::ExportMeshAsStlTo). The mesh should be
// vertex-to-vertex and not self-intersecting, so there's not much to
// do.
func ExportMeshAsStlTo(w io.Writer, m *srf.SMesh, scale float64) error {
	var header [80]byte
	copy(header[:], "STL exported mesh")
	buf := append([]byte{}, header[:]...)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(m.Triangles)))

	f32 := func(v float64) {
		buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(float32(v)))
	}
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		n := tr.Normal().WithMagnitude(1)
		f32(n.X)
		f32(n.Y)
		f32(n.Z)
		f32(tr.A.X / scale)
		f32(tr.A.Y / scale)
		f32(tr.A.Z / scale)
		f32(tr.B.X / scale)
		f32(tr.B.Y / scale)
		f32(tr.B.Z / scale)
		f32(tr.C.X / scale)
		f32(tr.C.Y / scale)
		f32(tr.C.Z / scale)
		buf = append(buf, 0, 0)
	}
	_, err := w.Write(buf)
	return err
}

// ShowNakedEdges finds the active group's naked and self-intersecting
// edges, and reports them through Message; with reportOnlyWhenNotOkay,
// only if there are any. It returns the edges (C++
// SolveSpaceUI::ShowNakedEdges, which keeps them to draw).
func (sk *Sketch) ShowNakedEdges(reportOnlyWhenNotOkay bool) srf.SEdgeList {
	var nakedEdges srf.SEdgeList

	g := sk.Group(sk.ActiveGroup())
	m := &g.DisplayMesh
	root := srf.SKdNodeFrom(m)
	var inters, leaks bool
	root.MakeCertainEdgesInto(&nakedEdges,
		srf.EdgeNakedOrSelfInter, true, &inters, &leaks, 0)

	if reportOnlyWhenNotOkay && !inters && !leaks && len(nakedEdges.Edges) == 0 {
		return nakedEdges
	}

	intersMsg := "The mesh is not self-intersecting (okay, valid)."
	if inters {
		intersMsg = "The mesh is self-intersecting (NOT okay, invalid)."
	}
	leaksMsg := "The mesh is watertight (okay, valid)."
	if leaks {
		leaksMsg = "The mesh has naked edges (NOT okay, invalid)."
	}

	cntMsg := fmt.Sprintf("\n\nThe model contains %d triangles, from %d surfaces.",
		len(g.DisplayMesh.Triangles), len(g.RunningShell.Surface))

	if sk.Message != nil {
		if len(nakedEdges.Edges) == 0 {
			sk.Message(fmt.Sprintf("%s\n\n%s\n\nZero problematic edges, good.%s",
				intersMsg, leaksMsg, cntMsg))
		} else {
			sk.Message(fmt.Sprintf("%s\n\n%s\n\n%d problematic edges, bad.%s",
				intersMsg, leaksMsg, len(nakedEdges.Edges), cntMsg))
		}
	}
	return nakedEdges
}
