package sketch_test

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A motion study (roadmap 16.3): a block's width driven from 10 to 30
// in four steps, each frame an STL file whose volume is the block's at
// that width; the parameter gets its expression back after. Without a
// window there are no pictures.
func TestMotionStudy(t *testing.T) {
	d := api.New()
	if err := d.Params.Set("w", "15"); err != nil {
		t.Fatal(err)
	}
	sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(15, 10, 0))
	if err := d.Constrain.Length(sides[0], 15).SetExpr("w"); err != nil {
		t.Fatal(err)
	}
	d.Features.Extrude(5, "")
	dir := t.TempDir()
	frames, err := d.Assembly.MotionStudy("w", 10, 30, 4, filepath.Join(dir, "f%02d.stl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 5 {
		t.Fatalf("%d frames", len(frames))
	}
	for i, f := range frames {
		want := 10 + 5*float64(i)
		data, err := os.ReadFile(f.File)
		if err != nil || f.Value != want || !f.Solved || f.Step != i {
			t.Fatalf("frame %+v: %v", f, err)
		}
		// The STL's volume, from its triangles.
		vol := 0.0
		n := int(binary.LittleEndian.Uint32(data[80:84]))
		for k := range n {
			at := 84 + 50*k + 12
			var p [3][3]float64
			for c := range 9 {
				p[c/3][c%3] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[at+4*c:])))
			}
			a, b, c := api.V(p[0][0], p[0][1], p[0][2]), api.V(p[1][0], p[1][1], p[1][2]), api.V(p[2][0], p[2][1], p[2][2])
			vol += a.Dot(b.Cross(c)) / 6
		}
		if math.Abs(vol-want*10*5) > 1e-3 {
			t.Errorf("frame %d: volume %g, want %g", i, vol, want*50)
		}
	}
	if p, _ := d.Params.Get("w"); p.Expr != "15" || p.Value != 15 {
		t.Errorf("after the study w = %s = %g", p.Expr, p.Value)
	}
	for name, c := range map[string]struct {
		param  string
		steps  int
		frames string
		want   string
	}{
		"no parameter": {"x", 3, "", "no parameter"},
		"no steps":     {"w", 0, "", "from 1 to"},
		"no number":    {"w", 2, filepath.Join(dir, "f.stl"), "verb"},
		"no window":    {"w", 2, filepath.Join(dir, "f%d.png"), "frame 0"},
	} {
		if _, err := d.Assembly.MotionStudy(c.param, 1, 2, c.steps, c.frames); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if p, _ := d.Params.Get("w"); p.Expr != "15" {
		t.Errorf("after a failed study w = %s", p.Expr)
	}
}
