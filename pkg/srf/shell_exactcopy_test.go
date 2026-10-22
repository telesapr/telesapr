package srf

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// An exact copy is the shell to the bit, with its tolerance, what was
// built from the trims and each surface's last closest point, and
// shares nothing a later operation on either changes.
func TestShellExactCopy(t *testing.T) {
	t.Parallel()
	s := SShell{Tol: Tol(0.5, 20), BooleanFailed: true}
	s.Surface = []SSurface{{
		DegM: 1, DegN: 1,
		Trim:        []STrimBy{{Start: 0.25, Finish: 0.75}},
		Edges:       SEdgeList{Edges: []SEdge{{A: geom.Vector{X: 1}}}},
		curvedEdges: SEdgeList{Edges: []SEdge{{B: geom.Vector{Y: 1}}}},
		Cached:      geom.Point2d{U: 0.3, V: 0.6},
	}}
	s.Curve = []SCurve{{Points: []SCurvePt{{P: geom.Vector{Z: 1}}}}}

	c := s.ExactCopy()
	if !reflect.DeepEqual(c, s) {
		t.Fatal("the copy differs from the shell")
	}
	c.Surface[0].Trim[0].Start = 0
	c.Surface[0].Edges.Edges[0].A.X = 0
	c.Surface[0].curvedEdges.Edges[0].B.Y = 0
	c.Surface[0].Cached.U = 0
	c.Curve[0].Points[0].P.Z = 0
	sf := s.Surface[0]
	if sf.Trim[0].Start != 0.25 || sf.Edges.Edges[0].A.X != 1 || sf.curvedEdges.Edges[0].B.Y != 1 ||
		sf.Cached.U != 0.3 || s.Curve[0].Points[0].P.Z != 1 {
		t.Error("a change to the copy changed the shell")
	}

	m := SMesh{Triangles: []STriangle{{A: geom.Vector{X: 1}}}, KeepCoplanar: true}
	mc := m.ExactCopy()
	if !reflect.DeepEqual(mc, m) {
		t.Fatal("the mesh's copy differs from it")
	}
	mc.Triangles[0].A.X = 2
	if m.Triangles[0].A.X != 1 {
		t.Error("a change to the mesh's copy changed the mesh")
	}
}
