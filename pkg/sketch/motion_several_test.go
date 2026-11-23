package sketch_test

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// Two parameters driven together, each through its own range over the
// same steps: a block's width from 10 to 30 and its depth from 5 to 15
// in four steps, its volume each step's width by depth by height, both
// parameters given their expressions back after; and misuse refused.
func TestMotionStudySeveral(t *testing.T) {
	t.Parallel()
	d := api.New()
	for _, p := range [][2]string{{"w", "15"}, {"h", "10"}} {
		if err := d.Params.Set(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(15, 10, 0))
	if err := d.Constrain.Length(sides[0], 15).SetExpr("w"); err != nil {
		t.Fatal(err)
	}
	if err := d.Constrain.Length(sides[1], 10).SetExpr("h"); err != nil {
		t.Fatal(err)
	}
	d.Features.Extrude(5, "")
	frames, err := d.Assembly.MotionStudyOf([]api.Drive{{Param: "w", From: 10, To: 30}, {Param: "h", From: 5, To: 15}}, 4, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 5 {
		t.Fatalf("%d frames", len(frames))
	}
	for i, f := range frames {
		w, h := 10+5*float64(i), 5+2.5*float64(i)
		if !slices.Equal(f.Values, []float64{w, h}) || f.Value != w || !f.Solved || math.Abs(f.Volume-w*h*5) > 1e-6 {
			t.Errorf("frame %d: %+v, want w %g, h %g, volume %g", i, f, w, h, w*h*5)
		}
	}
	for _, n := range []string{"w", "h"} {
		if p, _ := d.Params.Get(n); p.Expr != map[string]string{"w": "15", "h": "10"}[n] {
			t.Errorf("after the study %s = %s", n, p.Expr)
		}
	}
	for name, c := range map[string]struct {
		drives []api.Drive
		frames string
		want   string
	}{
		"none":      {nil, "", "at least one"},
		"twice":     {[]api.Drive{{Param: "w", From: 1, To: 2}, {Param: "w", From: 2, To: 3}}, "", "driven twice"},
		"unknown":   {[]api.Drive{{Param: "w", From: 1, To: 2}, {Param: "x", From: 0, To: 1}}, "", "no parameter"},
		"no number": {[]api.Drive{{Param: "w", From: 1, To: 2}}, "f.stl", "verb"},
		"no parts":  {[]api.Drive{{Param: "w", From: 1, To: 2}}, filepath.Join(t.TempDir(), "a.glb"), "linked parts"},
	} {
		if _, err := d.Assembly.MotionStudyOf(c.drives, 2, c.frames); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// gltfFile is what a test reads of a glTF document.
type gltfFile struct {
	Scenes []struct {
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Nodes []struct {
		Name        string    `json:"name"`
		Mesh        *int      `json:"mesh"`
		Translation []float64 `json:"translation"`
		Rotation    []float64 `json:"rotation"`
	} `json:"nodes"`
	Meshes []struct {
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
		} `json:"primitives"`
	} `json:"meshes"`
	Accessors []struct {
		BufferView    int       `json:"bufferView"`
		ComponentType int       `json:"componentType"`
		Count         int       `json:"count"`
		Type          string    `json:"type"`
		Min           []float64 `json:"min"`
		Max           []float64 `json:"max"`
	} `json:"accessors"`
	BufferViews []struct {
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
	} `json:"bufferViews"`
	Buffers []struct {
		ByteLength int    `json:"byteLength"`
		URI        string `json:"uri"`
	} `json:"buffers"`
	Animations []struct {
		Channels []struct {
			Sampler int `json:"sampler"`
			Target  struct {
				Node int    `json:"node"`
				Path string `json:"path"`
			} `json:"target"`
		} `json:"channels"`
		Samplers []struct {
			Input         int    `json:"input"`
			Output        int    `json:"output"`
			Interpolation string `json:"interpolation"`
		} `json:"samplers"`
	} `json:"animations"`
}

// readGltf reads a .glb or a .gltf whose buffer is a data URI: its
// document and its buffer.
func readGltf(t *testing.T, path string) (gltfFile, []byte) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc gltfFile
	var bin []byte
	if strings.HasSuffix(path, ".glb") {
		u32 := func(at int) int { return int(binary.LittleEndian.Uint32(b[at:])) }
		if len(b) < 28 || u32(0) != 0x46546C67 || u32(4) != 2 || u32(8) != len(b) {
			t.Fatalf("not a glb: %d bytes", len(b))
		}
		jl := u32(12)
		if err := json.Unmarshal(b[20:20+jl], &doc); err != nil {
			t.Fatal(err)
		}
		bin = b[28+jl:]
		if u32(20+jl) != len(bin) || len(bin)%4 != 0 {
			t.Fatalf("BIN chunk of %d bytes", u32(20+jl))
		}
	} else {
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		const prefix = "data:application/octet-stream;base64,"
		if len(doc.Buffers) != 1 || !strings.HasPrefix(doc.Buffers[0].URI, prefix) {
			t.Fatalf("buffers %+v", doc.Buffers)
		}
		if bin, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(doc.Buffers[0].URI, prefix)); err != nil {
			t.Fatal(err)
		}
	}
	if len(doc.Buffers) != 1 || doc.Buffers[0].ByteLength > len(bin) {
		t.Fatalf("buffer of %+v in %d bytes", doc.Buffers, len(bin))
	}
	return doc, bin
}

// floatsOf is accessor a's floats.
func floatsOf(doc gltfFile, bin []byte, a int) []float64 {
	acc := doc.Accessors[a]
	v := doc.BufferViews[acc.BufferView]
	out := make([]float64, v.ByteLength/4)
	for i := range out {
		out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(bin[v.ByteOffset+4*i:])))
	}
	return out
}

// rotateXYZW turns v by the unit quaternion q, given x, y, z, w.
func rotateXYZW(q []float64, v api.Vec) api.Vec {
	x, y, z, w := q[0], q[1], q[2], q[3]
	u := api.V(x, y, z)
	return v.ScaledBy(w*w - u.Dot(u)).Plus(u.ScaledBy(2 * u.Dot(v))).Plus(u.Cross(v).ScaledBy(2 * w))
}

// The driven lid's motion study written as one glTF animation, both as
// a .glb and a .gltf: a node with a mesh for the box and for the lid,
// one animation whose channels move the lid only, a keyframe a step at
// times that increase a tenth of a second apart, unit quaternions, and
// at each step the lid's node placed where its link places it at that
// step's angle.
func TestMotionStudyGLTF(t *testing.T) {
	t.Parallel()
	d, err := api.Open(filepath.Join("..", "..", "testdata", "ui", "mechanism", "boxlid.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	gs := d.Groups()
	lid := gs[len(gs)-1]
	const steps = 10
	type place struct{ t, u, w api.Vec }
	var want []place
	for k := range steps + 1 {
		setParam(t, d, "angle", 11*float64(k))
		u, w := lid.AsLink().Orientation()
		want = append(want, place{lid.AsLink().Offset(), u, w})
	}
	setParam(t, d, "angle", 0)
	dir := t.TempDir()
	for _, name := range []string{"lid.glb", "lid.gltf"} {
		path := filepath.Join(dir, name)
		frames, err := d.Assembly.MotionStudy("angle", 0, 110, steps, path)
		if err != nil {
			t.Fatal(err)
		}
		if len(frames) != steps+1 || frames[steps].File != path || !frames[steps].Solved {
			t.Fatalf("%s: frames %+v", name, frames)
		}
		doc, bin := readGltf(t, path)
		if len(doc.Scenes) != 1 || len(doc.Scenes[0].Nodes) != 2 || len(doc.Nodes) != 2 || len(doc.Animations) != 1 {
			t.Fatalf("%s: %d scenes, %d nodes, %d animations", name, len(doc.Scenes), len(doc.Nodes), len(doc.Animations))
		}
		for i, n := range doc.Nodes {
			if n.Mesh == nil || len(doc.Meshes[*n.Mesh].Primitives) == 0 {
				t.Errorf("%s: node %d %q has no mesh", name, i, n.Name)
			}
		}
		anim := doc.Animations[0]
		if len(anim.Channels) != 2 || len(anim.Samplers) != 2 {
			t.Fatalf("%s: %d channels, %d samplers", name, len(anim.Channels), len(anim.Samplers))
		}
		var tr, rot []float64
		for _, ch := range anim.Channels {
			s := anim.Samplers[ch.Sampler]
			if ch.Target.Node != 1 || s.Interpolation != "LINEAR" {
				t.Errorf("%s: channel %+v, sampler %+v: want the lid's node, linear", name, ch, s)
			}
			in, out := doc.Accessors[s.Input], doc.Accessors[s.Output]
			times := floatsOf(doc, bin, s.Input)
			if in.Count != steps+1 || out.Count != steps+1 || len(in.Min) != 1 || in.Min[0] != 0 ||
				math.Abs(in.Max[0]-0.1*steps) > 1e-6 {
				t.Fatalf("%s: input %+v, output %+v", name, in, out)
			}
			for k := 1; k < len(times); k++ {
				if !(times[k] > times[k-1]) || math.Abs(times[k]-0.1*float64(k)) > 1e-6 {
					t.Errorf("%s: time %d is %g after %g", name, k, times[k], times[k-1])
				}
			}
			switch ch.Target.Path {
			case "translation":
				tr = floatsOf(doc, bin, s.Output)
			case "rotation":
				rot = floatsOf(doc, bin, s.Output)
			}
		}
		if len(tr) != 3*(steps+1) || len(rot) != 4*(steps+1) {
			t.Fatalf("%s: %d translations, %d rotations", name, len(tr), len(rot))
		}
		for k, p := range want {
			q := rot[4*k : 4*k+4]
			if l := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3]); math.Abs(l-1) > 1e-6 {
				t.Errorf("%s: step %d: quaternion of length %g", name, k, l)
			}
			at := api.V(tr[3*k], tr[3*k+1], tr[3*k+2]).ScaledBy(1000)
			if at.Minus(p.t).Magnitude() > 1e-3 || rotateXYZW(q, api.V(1, 0, 0)).Minus(p.u).Magnitude() > 1e-6 ||
				rotateXYZW(q, api.V(0, 1, 0)).Minus(p.w).Magnitude() > 1e-6 {
				t.Errorf("%s: step %d: the lid's node at %v turned %v, want at %v, x %v, y %v", name, k, at, q, p.t, p.u, p.w)
			}
		}
		// The box stands still where it is.
		if box := doc.Nodes[0]; len(box.Translation) != 3 || len(box.Rotation) != 4 {
			t.Errorf("%s: the box's node %+v", name, box)
		}
	}
	if p, _ := d.Params.Get("angle"); p.Expr != "0" {
		t.Errorf("after the study angle = %s", p.Expr)
	}
}
