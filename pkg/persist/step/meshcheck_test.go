package step

import (
	"fmt"
	"strings"
	"testing"
)

// tessellatedCube is a STEP file of a cube of 10 as a tessellated solid
// of the triangles given (indices into its corners, from 1), with
// corner 7 at top.
func tessellatedCube(tris string, top string) []byte {
	return fmt.Appendf(nil, `ISO-10303-21;
HEADER;
FILE_DESCRIPTION((''),'2;1');
FILE_NAME('','',(''),(''),'','','');
FILE_SCHEMA(('AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF { 1 0 10303 442 1 1 4 }'));
ENDSEC;
DATA;
#1=COORDINATES_LIST('',8,((0.,0.,0.),(10.,0.,0.),(10.,10.,0.),(0.,10.,0.),(0.,0.,10.),(10.,0.,10.),%s,(0.,10.,10.)));
#2=TRIANGULATED_FACE('',#1,0,(),$,(),(%s));
#3=TESSELLATED_SOLID('cube',(#2),$);
ENDSEC;
END-ISO-10303-21;
`, top, tris)
}

// cubeTris are the cube's twelve triangles, outward.
const cubeTris = "(1,3,2),(1,4,3),(5,6,7),(5,7,8),(1,2,6),(1,6,5),(2,3,7),(2,7,6),(3,4,8),(3,8,7),(4,1,5),(4,5,8)"

// A solid whose mesh is open, or crosses itself, says so; a whole one,
// and an open shell, say nothing.
func TestMeshCheck(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, tris, top string
		naked, inter    bool
	}{
		{"whole", cubeTris, "(10.,10.,10.)", false, false},
		// The last triangle left out: three edges with no mate.
		{"open", strings.TrimSuffix(cubeTris, ",(4,5,8)"), "(10.,10.,10.)", true, false},
		// The top corner pulled down through the bottom, the edge from
		// corner 5 piercing it.
		{"crossed", cubeTris, "(5.,5.,-5.)", false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			res, err := Read(tessellatedCube(c.tris, c.top), Options{})
			if err != nil {
				t.Fatal(err)
			}
			r := res.Report
			p := r.Problem()
			if got := r.NakedEdges > 0; got != c.naked {
				t.Errorf("naked edges %d, problem %q", r.NakedEdges, p)
			}
			if got := r.SelfIntersecting > 0; got != c.inter {
				t.Errorf("self-intersecting edges %d, problem %q", r.SelfIntersecting, p)
			}
			broken := c.naked || c.inter
			if (r.BrokenSolids == 1) != broken || strings.Contains(p, "mesh of 1 solid") != broken {
				t.Errorf("broken solids %d, problem %q", r.BrokenSolids, p)
			}
		})
	}
	// The same triangles as a shell may be open.
	shell := strings.Replace(string(tessellatedCube(strings.TrimSuffix(cubeTris, ",(4,5,8)"), "(10.,10.,10.)")),
		"TESSELLATED_SOLID", "TESSELLATED_SHELL", 1)
	res, err := Read([]byte(shell), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r := res.Report; r.BrokenSolids != 0 || r.Problem() != "" {
		t.Errorf("an open shell: broken %d, problem %q", r.BrokenSolids, r.Problem())
	}
}
