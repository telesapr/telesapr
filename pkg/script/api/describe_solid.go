package api

import (
	"fmt"
	"math"
	"strings"

	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/topo"
)

// This file is DescribeSolid: the faces and edges of a solid in words,
// by the names a script picks them by, so that a program writing a
// fillet, a chamfer, a shell or a sketch on a face needn't find them
// by trial. The same walk fills SolidDescription's data.

// The most faces and edges DescribeSolid lists; a model with more is
// to be narrowed with the calls of Solid, which the text names.
const (
	describeMaxFaces = 40
	describeMaxEdges = 60
	// describeMaxProblems is how many of an unsound solid's problems
	// are listed.
	describeMaxProblems = 5
)

// describeNarrow is what the text says after a listing it cut short.
const describeNarrow = "narrow with Solid().EdgesAlong(dir), EdgesBetween(a, b) or FaceAt(p)"

// DescribeSolid lists the faces and edges of the solid at the group,
// the solid of every group up to it, as text: first how many faces,
// edges and vertices it has and whether it is sound, then each face
// with the ID Solid.Face takes, its name, its shape (flat with its
// normal, a cylinder or a cone with its axis, a sphere, or curved), a
// point of it that Solid.FaceAt finds it by and how many edges it has,
// then each edge with the name Solid.Edge takes, its shape, its length
// in mm (an arc's or a circle's as the circle has it), its ends and the
// IDs of its two faces. It lists at most 40 faces and 60 edges. A group
// with no solid up to it says so, with the group's problem if it has
// one. Group.SolidDescription is the same as data.
func (g Group) DescribeSolid() (s string) {
	g.d.read(func(sk *sketch.Sketch) { s, _ = describeSolid(sk, g.get(sk)) })
	return s
}

// describeSolid is DescribeSolid of group, in words and as data.
func describeSolid(sk *sketch.Sketch, group *sketch.Group) (string, Description) {
	at := groupDescription(group)
	info := &SolidInfo{Group: at}
	data := Description{Solid: info}
	src := sk.SolidAt(group.H)
	switch {
	case src == nil || len(src.RunningShell.Surface) == 0 && len(src.RunningMesh.Triangles) == 0:
		s := "no solid yet at " + at
		if msg := problemOf(sk, group); msg != "" {
			s += "; problem: " + msg
		}
		info.Note = s
		return s + "\n", data
	case src.IsForcedToMesh(sk) || len(src.RunningShell.Surface) == 0:
		info.Note = "the solid at " + at + " is a triangle mesh, which has no faces and edges to name"
		return info.Note + "\n", data
	}
	tp, err := src.Topology()
	if tp == nil {
		info.Note = fmt.Sprintf("the faces and edges of the solid at %s could not be found: %v", at, err)
		return info.Note + "\n", data
	}
	var b strings.Builder
	fmt.Fprintf(&b, "solid at %s", at)
	if src != group {
		info.MadeBy = groupDescription(src)
		fmt.Fprintf(&b, " (as %s made it)", info.MadeBy)
	}
	edges := solidEdgeList(tp)
	vertices := 0
	for v := range tp.Vertices {
		if tp.SolidVertex(v) {
			vertices++
		}
	}
	info.Faces, info.Edges, info.Vertices = len(tp.Sheets), len(edges), vertices
	fmt.Fprintf(&b, ": %d faces, %d edges, %d vertices; ", len(tp.Sheets), len(edges), vertices)
	if problems := tp.Validate(); len(problems) > 0 {
		msgs := make([]string, 0, describeMaxProblems)
		for _, p := range problems[:min(len(problems), describeMaxProblems)] {
			msgs = append(msgs, p.Msg)
		}
		info.ProblemCount, info.Problems = len(problems), msgs
		fmt.Fprintf(&b, "NOT SOUND, %d problems: %s\n", len(problems), strings.Join(msgs, "; "))
	} else {
		info.Sound = true
		b.WriteString("sound\n")
	}
	data.Faces = describeFaces(&b, sk, tp)
	data.Edges = describeEdges(&b, tp, edges)
	return b.String(), data
}

// solidEdgeList is the edges of the solid that have names, by their
// lead pieces, as Solid.Edges lists them.
func solidEdgeList(tp *topo.Topology) []int {
	var out []int
	for _, e := range tp.SolidEdges() {
		if _, err := tp.NameOf(tp.Lead(e)); err == nil && !tp.Edges[e].Seam {
			out = append(out, e)
		}
	}
	return out
}

// describeFaces lists the solid's faces, at most describeMaxFaces, and
// returns them as data.
func describeFaces(b *strings.Builder, sk *sketch.Sketch, tp *topo.Topology) []FaceInfo {
	b.WriteString("faces, by the ID Solid().Face takes, with a point Solid().FaceAt finds them by:\n")
	var out []FaceInfo
	seen := map[uint32]int{}
	for sh := range tp.Sheets {
		id := uint32(tp.Sheets[sh].ID)
		seen[id]++
		if sh >= describeMaxFaces {
			continue
		}
		f := FaceInfo{ID: id, Name: sk.FaceName(tp.Sheets[sh].ID)}
		fmt.Fprintf(b, "    %#x %s", id, f.Name)
		if n := len(tp.SheetsWithID(tp.Sheets[sh].ID)); n > 1 {
			f.Piece, f.Pieces = seen[id], n
			fmt.Fprintf(b, " (piece %d of %d with this ID)", seen[id], n)
		}
		b.WriteString(": " + faceShape(tp.SheetShape(sh), &f))
		if p, err := tp.SheetPoint(sh); err == nil {
			f.Point = vec3p(p)
			b.WriteString("; at " + shortVec(p))
		}
		for _, e := range tp.SheetEdges(sh) {
			if _, err := tp.NameOf(tp.Lead(e)); err == nil && !tp.Edges[e].Seam {
				f.Edges++
			}
		}
		if f.Edges == 1 {
			b.WriteString("; 1 edge\n")
		} else {
			fmt.Fprintf(b, "; %d edges\n", f.Edges)
		}
		out = append(out, f)
	}
	if more := len(tp.Sheets) - describeMaxFaces; more > 0 {
		fmt.Fprintf(b, "    … and %d more faces; %s\n", more, describeNarrow)
	}
	return out
}

// faceShape words a face's shape, and puts it in f.
func faceShape(s topo.Shape, f *FaceInfo) string {
	switch s.Kind {
	case topo.ShapeFlat:
		f.Shape, f.Normal = "flat", vec3p(s.Normal)
		return "flat, normal " + shortVec(s.Normal)
	case topo.ShapeCylinder:
		f.Shape, f.Axis, f.Centre, f.Radius = "cylinder", vec3p(s.Axis), vec3p(s.Centre), round3(s.Radius)
		return "cylinder, axis " + shortVec(s.Axis) + " through " + shortVec(s.Centre) + ", radius " + shortNum(s.Radius)
	case topo.ShapeCone:
		half := s.HalfAngle * 180 / math.Pi
		f.Shape, f.Axis, f.Centre, f.HalfAngle = "cone", vec3p(s.Axis), vec3p(s.Centre), round3(half)
		return "cone, axis " + shortVec(s.Axis) + " from its apex " + shortVec(s.Centre) +
			", half angle " + shortNum(half) + "°"
	case topo.ShapeSphere:
		f.Shape, f.Centre, f.Radius = "sphere", vec3p(s.Centre), round3(s.Radius)
		return "sphere, centre " + shortVec(s.Centre) + ", radius " + shortNum(s.Radius)
	}
	f.Shape = "curved"
	return "curved"
}

// describeEdges lists the solid's edges, at most describeMaxEdges, and
// returns them as data.
func describeEdges(b *strings.Builder, tp *topo.Topology, edges []int) []EdgeInfo {
	b.WriteString("edges, by the name Solid().Edge takes:\n")
	var out []EdgeInfo
	for _, e := range edges[:min(len(edges), describeMaxEdges)] {
		n, _ := tp.NameOf(tp.Lead(e))
		chain := tp.Chain(e)
		pts := tp.ChainPoints(chain)
		shape := tp.EdgeShape(e)
		length := edgeLength(shape, pts)
		info := EdgeInfo{Name: n.String(), Length: round3(length)}
		fmt.Fprintf(b, "    %q: %s, %s mm, ", info.Name, edgeShape(shape, &info), shortNum(length))
		if tp.Closed(chain) {
			info.Closed = true
			b.WriteString("closed")
		} else {
			info.Ends = &[2][3]float64{vec3(pts[0]), vec3(pts[len(pts)-1])}
			b.WriteString(shortVec(pts[0]) + " to " + shortVec(pts[len(pts)-1]))
		}
		var ids []string
		for _, sh := range tp.EdgeSheets(e) {
			if sh >= 0 {
				id := uint32(tp.Sheets[sh].ID)
				info.Faces = append(info.Faces, id)
				ids = append(ids, fmt.Sprintf("%#x", id))
			}
		}
		fmt.Fprintf(b, "; faces %s\n", strings.Join(ids, " and "))
		out = append(out, info)
	}
	if more := len(edges) - describeMaxEdges; more > 0 {
		fmt.Fprintf(b, "    … and %d more edges; %s\n", more, describeNarrow)
	}
	return out
}

// edgeLength is the length of the edge through pts: an arc's or a
// circle's as the circle has it, which the points along it, chords of
// it, fall short of; any other edge's along its points, as
// SolidEdge.Length measures it.
func edgeLength(s topo.Shape, pts []Vec) float64 {
	var l float64
	for i := 1; i < len(pts); i++ {
		if s.Kind == topo.ShapeArc || s.Kind == topo.ShapeCircle {
			a, b := pts[i-1].Minus(s.Centre), pts[i].Minus(s.Centre)
			l += s.Radius * math.Atan2(a.Cross(b).Magnitude(), a.Dot(b))
		} else {
			l += pts[i].Minus(pts[i-1]).Magnitude()
		}
	}
	return l
}

// edgeShape words an edge's shape, and puts it in e.
func edgeShape(s topo.Shape, e *EdgeInfo) string {
	switch s.Kind {
	case topo.ShapeStraight:
		e.Shape = "straight"
		return "straight"
	case topo.ShapeArc:
		e.Shape, e.Centre, e.Radius = "arc", vec3p(s.Centre), round3(s.Radius)
		return "arc about " + shortVec(s.Centre) + ", radius " + shortNum(s.Radius)
	case topo.ShapeCircle:
		e.Shape, e.Centre, e.Radius = "circle", vec3p(s.Centre), round3(s.Radius)
		return "circle about " + shortVec(s.Centre) + ", radius " + shortNum(s.Radius)
	}
	e.Shape = "curve"
	return "curve"
}
