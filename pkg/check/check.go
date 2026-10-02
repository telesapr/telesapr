// Package check judges the solid a group makes by invariants that hold
// for any sound solid, whatever produced it: the mesh is closed and does
// not cross itself, the Boolean that built the shell succeeded, and the
// volume, centre of mass and extent are those expected.
//
// Tests call it on every solid they build, the Analyze menu's Check
// Solid shows it, and scripts get it as Group.Check.
package check

import (
	"fmt"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/internal/cfmt"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

// Report is what Solid finds. Lengths are in millimetres, volume in
// cubic millimetres.
type Report struct {
	// Triangles is the number of triangles in the mesh.
	Triangles int
	// Faces is the number of surfaces in the shell; zero for a group
	// that makes a mesh and no shell.
	Faces int
	// NakedEdges is the number of mesh edges with no mate running the
	// other way, so where the mesh is open.
	NakedEdges int
	// SelfIntersecting is the number of mesh edges that pass through a
	// triangle, or lie on a coplanar one.
	SelfIntersecting int
	// BooleanFailed is set when the Boolean that made the shell could
	// not classify every edge.
	BooleanFailed bool
	// Volume is the volume the mesh encloses.
	Volume float64
	// CenterOfMass is the centroid of that volume, zero when the volume
	// is zero.
	CenterOfMass geom.Vector
	// Min and Max are the corners of the mesh's bounding box, both zero
	// for an empty mesh.
	Min, Max geom.Vector
	// TopologyProblems counts what the shell's topology gets wrong
	// (topo.Validate): an edge not shared by exactly two faces running
	// it opposite ways, a loop that doesn't close; and, for a closed
	// mesh, an Euler characteristic that isn't the mesh's. Zero for a
	// group with no shell.
	TopologyProblems int
	// Euler is the shell's Euler characteristic, V - E + F - H: two for
	// each closed piece, less two for each hole through it.
	Euler int
	// Topology lists the topology's problems, for showing.
	Topology []topo.Problem

	// Problems holds the naked and self-intersecting edges, for drawing.
	Problems srf.SEdgeList
}

// Empty reports whether there is no solid at all.
func (r *Report) Empty() bool { return r.Triangles == 0 }

// Sound reports whether the solid is closed, does not cross itself, and
// came from a Boolean that succeeded. An empty solid is sound.
func (r *Report) Sound() bool {
	return r.NakedEdges == 0 && r.SelfIntersecting == 0 && !r.BooleanFailed && r.TopologyProblems == 0
}

// Solid checks a shell and the mesh made from it. shell may be nil, for
// a group that makes only a mesh.
func Solid(shell *srf.SShell, mesh *srf.SMesh) Report {
	var r Report
	if shell != nil {
		r.Faces = len(shell.Surface)
		r.BooleanFailed = shell.BooleanFailed
	}
	r.Triangles = len(mesh.Triangles)
	if r.Triangles == 0 {
		return r
	}

	root := srf.SKdNodeFrom(mesh)
	// One pass lists each naked edge once and each self-intersecting
	// edge once, an edge that is both twice; the second counts only the
	// self-intersecting ones.
	root.MakeCertainEdgesInto(&r.Problems, srf.EdgeNakedOrSelfInter, true, nil, nil, 0)
	var inter srf.SEdgeList
	root.MakeCertainEdgesInto(&inter, srf.EdgeSelfInter, true, nil, nil, 0)
	r.SelfIntersecting = len(inter.Edges)
	r.NakedEdges = len(r.Problems.Edges) - r.SelfIntersecting

	r.Volume = mesh.CalculateVolume()
	if r.Volume != 0 {
		r.CenterOfMass = mesh.GetCenterOfMass()
	}
	r.Max, r.Min = mesh.GetBounding()
	if shell != nil && len(shell.Surface) > 0 {
		tp, err := topo.Build(shell)
		r.Topology = tp.Validate()
		if err != nil && len(r.Topology) == 0 {
			r.Topology = append(r.Topology, topo.Problem{Msg: err.Error()})
		}
		r.Euler = tp.Euler()
		if r.NakedEdges == 0 && len(r.Topology) == 0 {
			if me := topo.MeshEuler(mesh); me != r.Euler {
				r.Topology = append(r.Topology, topo.Problem{
					Msg: fmt.Sprintf("the shell's Euler characteristic is %d, its mesh's %d", r.Euler, me)})
			}
		}
		r.TopologyProblems = len(r.Topology)
	}
	return r
}

// Group checks the solid a group's regeneration left, which is the
// solid of every group up to it: the shell of the last group that
// contributed solid model, and the mesh shown for g. Regenerate the
// sketch first; Group makes the display mesh if it is out of date.
func Group(sk *sketch.Sketch, g *sketch.Group) Report {
	g.GenerateDisplayItems(sk)
	src := g
	for len(src.ThisMesh.Triangles) == 0 && len(src.ThisShell.Surface) == 0 {
		pg := src.RunningMeshGroup(sk)
		if pg == nil {
			break
		}
		src = pg
	}
	shell := &src.RunningShell
	if src.IsForcedToMesh(sk) {
		shell = nil
	}
	return Solid(shell, &g.DisplayMesh)
}

// String is the report on one line, every number written so it reads
// back to the same value: the form of testdata/invariants.txt.
func (r *Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "tri=%d faces=%d naked=%d inter=%d failed=%t",
		r.Triangles, r.Faces, r.NakedEdges, r.SelfIntersecting, r.BooleanFailed)
	if !r.Empty() {
		fmt.Fprintf(&b, " vol=%s com=%s min=%s max=%s",
			g17(r.Volume), v17(r.CenterOfMass), v17(r.Min), v17(r.Max))
	}
	if r.Faces > 0 {
		fmt.Fprintf(&b, " topo=%d euler=%d", r.TopologyProblems, r.Euler)
	}
	return b.String()
}

// Summary is the report for a person, in millimetres.
func (r *Report) Summary() string {
	if r.Empty() {
		return "The group has no solid."
	}
	var b strings.Builder
	if r.Sound() {
		b.WriteString("The solid is sound: closed, not self-intersecting, and its Boolean succeeded.\n\n")
	} else {
		b.WriteString("The solid is NOT sound:\n")
		if r.NakedEdges > 0 {
			fmt.Fprintf(&b, "    %d naked edges, so it is not closed\n", r.NakedEdges)
		}
		if r.SelfIntersecting > 0 {
			fmt.Fprintf(&b, "    %d self-intersecting edges\n", r.SelfIntersecting)
		}
		if r.BooleanFailed {
			b.WriteString("    the Boolean failed\n")
		}
		if r.TopologyProblems > 0 {
			fmt.Fprintf(&b, "    %d topology problems, the first: %s\n", r.TopologyProblems, r.Topology[0].Msg)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Faces: %d\nTriangles: %d\n", r.Faces, r.Triangles)
	fmt.Fprintf(&b, "Volume: %s mm^3\n", cfmt.Sprintf("%.6g", r.Volume))
	c := r.CenterOfMass
	fmt.Fprintf(&b, "Center of mass: (%s, %s, %s) mm\n",
		cfmt.Sprintf("%.6g", c.X), cfmt.Sprintf("%.6g", c.Y), cfmt.Sprintf("%.6g", c.Z))
	d := r.Max.Minus(r.Min)
	fmt.Fprintf(&b, "Extent: %s x %s x %s mm",
		cfmt.Sprintf("%.6g", d.X), cfmt.Sprintf("%.6g", d.Y), cfmt.Sprintf("%.6g", d.Z))
	return b.String()
}

func g17(v float64) string { return cfmt.Sprintf("%.17g", v) }

func v17(v geom.Vector) string { return g17(v.X) + "," + g17(v.Y) + "," + g17(v.Z) }
