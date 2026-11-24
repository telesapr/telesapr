package link

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A link of a file we wrote says its validation properties agree, as
// a mesh and as a solid; the same file with its volume changed says
// how it differs.
func TestLinkStepValidation(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "testdata", "export", "group", "translate_nd", "normal", "surfaces.step")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	agrees := func(info []string) bool {
		return slices.ContainsFunc(info, func(s string) bool { return strings.Contains(s, "as the file declares for 1 part") })
	}
	for _, solid := range []bool{false, true} {
		load := linkStep
		if solid {
			load = linkStepSolid
		}
		var m srf.SMesh
		var sh srf.SShell
		var rep Report
		if _, ok := load(path, &sketch.Sketch{}, &m, &sh, &rep); !ok {
			t.Fatal("not linked")
		}
		if !agrees(rep.Info) || rep.Problem != "" {
			t.Errorf("solid %v: info %q, problem %q", solid, rep.Info, rep.Problem)
		}
	}
	changed := strings.Replace(string(data), "VOLUME_MEASURE(1000.)", "VOLUME_MEASURE(1100.)", 1)
	if changed == string(data) {
		t.Fatal("the fixture declares no volume of 1000")
	}
	bad := filepath.Join(t.TempDir(), "changed.step")
	if err := os.WriteFile(bad, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	var m srf.SMesh
	var rep Report
	if _, ok := linkStep(bad, &sketch.Sketch{}, &m, nil, &rep); !ok {
		t.Fatal("not linked")
	}
	if agrees(rep.Info) || !strings.Contains(rep.Problem, "volume 1100 mm³ declared, 1000 read") {
		t.Errorf("info %q, problem %q", rep.Info, rep.Problem)
	}
}

// A link of a solid whose mesh is open says so.
func TestLinkStepOpenMesh(t *testing.T) {
	t.Parallel()
	const open = `ISO-10303-21;
HEADER;
FILE_DESCRIPTION((''),'2;1');
FILE_NAME('','',(''),(''),'','','');
FILE_SCHEMA(('AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF { 1 0 10303 442 1 1 4 }'));
ENDSEC;
DATA;
#1=COORDINATES_LIST('',4,((0.,0.,0.),(6.,0.,0.),(0.,6.,0.),(0.,0.,6.)));
#2=TRIANGULATED_FACE('',#1,0,(),$,(),((1,3,2),(1,2,4),(1,4,3)));
#3=TESSELLATED_SOLID('a tetrahedron without its slanted face',(#2),$);
ENDSEC;
END-ISO-10303-21;
`
	path := filepath.Join(t.TempDir(), "open.step")
	if err := os.WriteFile(path, []byte(open), 0o644); err != nil {
		t.Fatal(err)
	}
	var m srf.SMesh
	var rep Report
	if _, ok := linkStep(path, &sketch.Sketch{}, &m, nil, &rep); !ok {
		t.Fatal("not linked")
	}
	if rep.Problem != "the mesh of 1 solid has 3 naked edges, so it is not closed" {
		t.Errorf("problem %q", rep.Problem)
	}
}
