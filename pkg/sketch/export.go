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

// ExportSettings are the user's export settings.
type ExportSettings struct {
	// ChordTol is the chord tolerance in export units.
	ChordTol    float64
	MaxSegments int
	// Scale is how many mm an export unit is.
	Scale float64
	// AngleTol is the angular tolerance in degrees: the most a curve's
	// tangent or a surface's normal may turn across one facet; 0 for
	// none, the chord tolerance alone.
	AngleTol float64
	// Strict has the export refuse a mesh with naked edges or
	// self-intersections instead of writing it with a warning.
	Strict bool
}

// ErrMeshNotSound is what an export wraps when Strict refuses a mesh
// that is not watertight or crosses itself.
var ErrMeshNotSound = errors.New("mesh is not sound")

// MeshSoundnessError is nil when a mesh with the given problems (as
// MeshProblems finds them) may be exported, and an error that wraps
// ErrMeshNotSound, saying what is wrong, when it may not.
func MeshSoundnessError(naked srf.SEdgeList, inters, leaks bool) error {
	if !inters && !leaks && len(naked.Edges) == 0 {
		return nil
	}
	what := "has naked edges"
	if inters && leaks {
		what = "has naked edges and crosses itself"
	} else if inters {
		what = "crosses itself"
	}
	return fmt.Errorf("%w: the mesh %s (%d problematic edges); not exported",
		ErrMeshNotSound, what, len(naked.Edges))
}

// ThawExportSettings reads the export settings, with their defaults.
func ThawExportSettings(settings core.Settings) ExportSettings {
	return ExportSettings{
		ChordTol:    settings.ThawFloat("ExportChordTolerance", 0.1),
		MaxSegments: int(settings.ThawInt("ExportMaxSegments", 64)),
		Scale:       settings.ThawFloat("ExportScale", 1.0),
		AngleTol:    settings.ThawFloat("ExportAngleTolerance", 0),
		Strict:      core.ThawBool(settings, "ExportStrictMesh", false),
	}
}

// ChordTolMm is the export chord tolerance in mm.
func (es ExportSettings) ChordTolMm() float64 {
	return es.ChordTol / es.Scale
}

// ExportMeshTo writes the active group's mesh, regenerated at the
// export chord tolerance, to filename; the extension picks the format,
// STL or 3MF. It reports a mesh that isn't watertight or
// self-intersects through Message, but still exports it. Regenerating
// in export mode doesn't solve, so the sketch must have been generated
// since it was loaded or changed.
func (sk *Sketch) ExportMeshTo(filename string, es ExportSettings) error {
	// The display is regenerated on return.
	sk.Exporting = srf.Tol(es.ChordTolMm(), es.MaxSegments).WithAngle(es.AngleTol)
	defer func() {
		sk.Exporting = srf.Tolerance{}
		sk.GenerateAllTyped(GenerateAll)
	}()
	sk.GenerateAllTyped(GenerateAll)

	g := sk.Group(sk.ActiveGroup())
	g.GenerateDisplayItems(sk)

	m := &g.DisplayMesh
	if len(m.Triangles) == 0 {
		return errors.New("active group mesh is empty; nothing to export")
	}

	var write func(w io.Writer) error
	switch {
	case hasExtension(filename, "stl"):
		write = func(w io.Writer) error { return ExportMeshAsStlTo(w, m, es.Scale) }
	case hasExtension(filename, "3mf"):
		write = func(w io.Writer) error { return ExportMeshAs3mfTo(w, m) }
	case hasExtension(filename, "obj"), hasExtension(filename, "js"),
		hasExtension(filename, "html"), hasExtension(filename, "wrl"):
		return fmt.Errorf("exporting a mesh to '%s' isn't supported; try .stl or .3mf", filename)
	default:
		return fmt.Errorf("can't identify output file type from file extension of "+
			"filename '%s'; try .stl, .3mf, .obj, .js, .html", filename)
	}

	if es.Strict {
		naked, inters, leaks := sk.MeshProblems()
		if err := MeshSoundnessError(naked, inters, leaks); err != nil {
			return err
		}
	}
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("couldn't write to '%s'", filename)
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
// case.
func hasExtension(filename, ext string) bool {
	_, ours, _ := strings.CutLast(filename, ".")
	return strings.EqualFold(ours, ext)
}

// ExportMeshAsStlTo writes m as a binary STL file, in export units of
// scale mm. The mesh should be vertex-to-vertex and not
// self-intersecting, so there's not much to do.
func ExportMeshAsStlTo(w io.Writer, m *srf.SMesh, scale float64) error {
	var header [80]byte
	copy(header[:], "STL exported mesh")
	// A facet with two equal corners has no area; some readers reject
	// it, and the 3MF writer drops it too. Corners are equal as that
	// writer welds them, exactly.
	flat := func(tr *srf.STriangle) bool { return tr.A == tr.B || tr.B == tr.C || tr.A == tr.C }
	kept := 0
	for i := range m.Triangles {
		if !flat(&m.Triangles[i]) {
			kept++
		}
	}
	buf := append([]byte{}, header[:]...)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(kept))

	f32 := func(v float64) {
		buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(float32(v)))
	}
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		if flat(tr) {
			continue
		}
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
// only if there are any. It returns the edges.
func (sk *Sketch) ShowNakedEdges(reportOnlyWhenNotOkay bool) srf.SEdgeList {
	g := sk.Group(sk.ActiveGroup())
	nakedEdges, inters, leaks := sk.MeshProblems()

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

	if len(nakedEdges.Edges) == 0 {
		sk.Tell(fmt.Sprintf("%s\n\n%s\n\nZero problematic edges, good.%s",
			intersMsg, leaksMsg, cntMsg))
	} else {
		sk.Tell(fmt.Sprintf("%s\n\n%s\n\n%d problematic edges, bad.%s",
			intersMsg, leaksMsg, len(nakedEdges.Edges), cntMsg))
	}
	return nakedEdges
}

// MeshProblems measures the active group's display mesh: its naked and
// self-intersecting edges, whether it crosses itself and whether it
// leaks (has naked edges).
func (sk *Sketch) MeshProblems() (naked srf.SEdgeList, inters, leaks bool) {
	g := sk.Group(sk.ActiveGroup())
	root := srf.SKdNodeFrom(&g.DisplayMesh)
	root.MakeCertainEdgesInto(&naked,
		srf.EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	return naked, inters, leaks
}
