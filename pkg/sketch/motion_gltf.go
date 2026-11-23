package sketch

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A motion study written as one animated glTF: a node per linked part,
// its mesh in the part's own frame, and for each part that moves in a
// mechanism a translation and a rotation channel with a keyframe per
// step, where its link placed it at that step. A viewer then plays the
// study: the lid opens, the engine turns. The parts are rigid, so this
// is the whole of what changes from step to step.

// MotionSecondsPerStep is how long a step of a motion study lasts in its
// animation: ten steps a second, so the lid's 14 steps of examples/
// scripts/lid open it in 1.4 s.
const MotionSecondsPerStep = 0.1

// ErrNotAnimatable is the refusal of an animation of a model whose
// solid isn't an assembly of linked parts read as solids only.
var ErrNotAnimatable = errors.New("an animation is of an assembly of linked parts, each read as a solid")

// MotionTrack is where each part of an assembly is at each step of a
// motion study, recorded as the study runs (Record), for
// ExportMotionTo.
type MotionTrack struct {
	parts []core.HGroup
	moves []bool
	// at holds, for each step, each part's move and turn.
	at [][]partPlacement
}

// partPlacement is where a link places its part.
type partPlacement struct {
	t geom.Vector
	q geom.Quaternion
}

// NewMotionTrack is an empty track of the parts of the active group's
// solid, or ErrNotAnimatable where that solid is not an assembly of
// linked parts (linkedParts): a solid of the model's own has no frame to
// move it by.
func (sk *Sketch) NewMotionTrack() (*MotionTrack, error) {
	parts := sk.linkedParts(sk.Group(sk.ActiveGroup()))
	if len(parts) == 0 {
		return nil, ErrNotAnimatable
	}
	mt := &MotionTrack{}
	for _, g := range parts {
		mt.parts = append(mt.parts, g.H)
		mt.moves = append(mt.moves, movesInMechanism(g))
	}
	return mt, nil
}

// Record adds a step: where each part is now.
func (mt *MotionTrack) Record(sk *Sketch) {
	row := make([]partPlacement, len(mt.parts))
	for i, hg := range mt.parts {
		t, q := sk.Group(hg).placement(sk)
		row[i] = partPlacement{t: t, q: q.WithMagnitude(1)}
	}
	mt.at = append(mt.at, row)
}

// Steps is how many steps the track holds.
func (mt *MotionTrack) Steps() int { return len(mt.at) }

// ExportMotionTo writes the track as an animated glTF to filename, a
// .glb or a .gltf (its buffer a data URI in the file), each part's mesh
// made at the export settings' chord tolerance in its own frame. A part
// that doesn't move stands where the first step has it, with no
// channels; one that moves has a translation and a rotation channel of
// one keyframe a step, MotionSecondsPerStep apart.
func (sk *Sketch) ExportMotionTo(filename string, mt *MotionTrack, es ExportSettings) error {
	if mt == nil || len(mt.at) == 0 {
		return errors.New("a motion study's animation has no steps")
	}
	glb := hasExtension(filename, "glb")
	if !glb && !hasExtension(filename, "gltf") {
		return fmt.Errorf("an animation is written to a .glb or a .gltf, not '%s'", filename)
	}
	sk.Exporting = srf.Tol(es.ChordTolMm(), es.MaxSegments).WithAngle(es.AngleTol)
	defer func() {
		sk.Exporting = srf.Tolerance{}
		sk.GenerateAllTyped(GenerateAll)
	}()
	sk.GenerateAllTyped(GenerateAll)
	meshes := make([]srf.SMesh, len(mt.parts))
	for i, hg := range mt.parts {
		g := sk.Group(hg)
		if g == nil || len(g.ImpShell.Surface) == 0 {
			return ErrNotAnimatable
		}
		sk.partMeshInto(g, &meshes[i])
	}
	var buf bytes.Buffer
	if err := writeMotionGltf(&buf, mt, meshes, glb, sk.GroupName); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0o644)
}

// partMeshInto triangulates link group g's part in the part's own
// frame, scaled but not moved or turned, at the tolerance now.
func (sk *Sketch) partMeshInto(g *Group, m *srf.SMesh) {
	var sh srf.SShell
	sh.Tol = sk.tol
	sh.MakeFromTransformationOf(&g.ImpShell, geom.Vector{}, geom.QuaternionIdentity, g.Scale)
	sh.RemapFaces(shellGroup{g: g, sk: sk}, 0)
	giveBack := sk.lendPace(&sh)
	sh.TriangulateInto(m)
	giveBack()
}

// glTF's animation keyframes, from the specification.
type gltfAnimation struct {
	Name     string            `json:"name,omitempty"`
	Channels []gltfChannel     `json:"channels"`
	Samplers []gltfAnimSampler `json:"samplers"`
}

type gltfChannel struct {
	Sampler int               `json:"sampler"`
	Target  gltfChannelTarget `json:"target"`
}

type gltfChannelTarget struct {
	Node int    `json:"node"`
	Path string `json:"path"`
}

type gltfAnimSampler struct {
	Input         int    `json:"input"`
	Output        int    `json:"output"`
	Interpolation string `json:"interpolation"`
}

// writeMotionGltf writes the animation: meshes[i] is part i's mesh in
// its own frame, name names a part's group.
func writeMotionGltf(w io.Writer, mt *MotionTrack, meshes []srf.SMesh, glb bool, name func(core.HGroup) string) error {
	b := newGltfBuilder()
	scene := gltfScene{}
	// A turn's sign is free; a viewer interpolates each pair of
	// keyframes the short way between them, so each quaternion is taken
	// on the side of the one before.
	steps := len(mt.at)
	for i := range mt.parts {
		for k := 1; k < steps; k++ {
			if a, c := mt.at[k-1][i].q, mt.at[k][i].q; a.W*c.W+a.X*c.X+a.Y*c.Y+a.Z*c.Z < 0 {
				mt.at[k][i].q = geom.Quaternion{W: -c.W, X: -c.X, Y: -c.Y, Z: -c.Z}
			}
		}
	}
	for i, hg := range mt.parts {
		n := gltfNode{Name: name(hg)}
		if mi, ok := b.addMesh(&meshes[i]); ok {
			n.Mesh = new(mi)
		}
		n.Translation, n.Rotation = gltfPlacement(mt.at[0][i])
		scene.Nodes = append(scene.Nodes, len(b.doc.Nodes))
		b.doc.Nodes = append(b.doc.Nodes, n)
	}
	b.doc.Scenes = []gltfScene{scene}

	anim := gltfAnimation{Name: "motion study"}
	var times []byte
	for k := range steps {
		times = appendF32(times, float64(k)*MotionSecondsPerStep)
	}
	timeAt := len(b.doc.Accessors)
	b.doc.Accessors = append(b.doc.Accessors, gltfAccessor{BufferView: b.view(times, 0),
		ComponentType: gltfFloat, Count: steps, Type: "SCALAR",
		Min: []float32{0}, Max: []float32{float32(float64(steps-1) * MotionSecondsPerStep)}})
	for i := range mt.parts {
		if !mt.moves[i] {
			continue
		}
		var tr, rot []byte
		for k := range steps {
			t, q := gltfPlacement(mt.at[k][i])
			for _, v := range t {
				tr = appendF32(tr, v)
			}
			for _, v := range q {
				rot = appendF32(rot, v)
			}
		}
		for _, ch := range []struct {
			data []byte
			typ  string
			path string
		}{{tr, "VEC3", "translation"}, {rot, "VEC4", "rotation"}} {
			a := len(b.doc.Accessors)
			b.doc.Accessors = append(b.doc.Accessors, gltfAccessor{BufferView: b.view(ch.data, 0),
				ComponentType: gltfFloat, Count: steps, Type: ch.typ})
			anim.Samplers = append(anim.Samplers, gltfAnimSampler{Input: timeAt, Output: a, Interpolation: "LINEAR"})
			anim.Channels = append(anim.Channels, gltfChannel{Sampler: len(anim.Samplers) - 1,
				Target: gltfChannelTarget{Node: scene.Nodes[i], Path: ch.path}})
		}
	}
	if len(anim.Channels) > 0 {
		b.doc.Animations = []gltfAnimation{anim}
	}
	if glb {
		return b.writeGlb(w)
	}
	for b.bin.Len()%4 != 0 {
		b.bin.WriteByte(0)
	}
	js, err := b.encode("data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(b.bin.Bytes()))
	if err != nil {
		return err
	}
	_, err = w.Write(js)
	return err
}

// gltfPlacement is a part's place as a glTF node's translation, in
// metres, and rotation, x, y, z, w, each rounded to keep the file the
// same on every platform.
func gltfPlacement(p partPlacement) (t, q []float64) {
	r := func(v float64) float64 { return math.Round(v*1e9) / 1e9 }
	t = []float64{r(p.t.X * gltfMetresPerMm), r(p.t.Y * gltfMetresPerMm), r(p.t.Z * gltfMetresPerMm)}
	q = []float64{r(p.q.X), r(p.q.Y), r(p.q.Z), r(p.q.W)}
	return t, q
}

// appendF32 appends v as a little-endian float32.
func appendF32(b []byte, v float64) []byte {
	return binary.LittleEndian.AppendUint32(b, math.Float32bits(float32(v)))
}
