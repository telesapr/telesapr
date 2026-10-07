package step

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// DefaultChordTolPct is the chord tolerance, as a percentage of the
// model's size, when Options gives none: the sketch's default.
const DefaultChordTolPct = 0.1

// maxTriangles bounds the mesh of one file.
const maxTriangles = 5000000

// Options say how finely to tessellate.
type Options struct {
	// ChordTol is the largest distance, in millimetres, from the mesh
	// to the surfaces it stands for. Zero takes ChordTolPct of the
	// model's size.
	ChordTol float64
	// ChordTolPct is the chord tolerance as a percentage of the
	// diagonal of the model's bounding box; zero is
	// DefaultChordTolPct.
	ChordTolPct float64
}

// DefaultColor is the colour of faces the file gives none.
var DefaultColor = core.NewRgbaColor(90, 120, 140, 255)

// Result is a file read into a mesh.
type Result struct {
	Mesh srf.SMesh
	// Vertices are the shapes' vertices, placed, each once.
	Vertices []geom.Vector
	// Centers are the centres of the shapes' circular edges, placed,
	// each once.
	Centers []geom.Vector
	Report  Report
}

// Report says what a file held and what of it was read.
type Report struct {
	Schema string
	// Unit is the file's length unit; the mesh is in millimetres.
	Unit string
	// Shapes counts the solids and shells read, Placements their uses
	// in the assembly.
	Shapes, Placements int
	// Faces counts the faces tessellated, once per shape; Skipped those
	// left out.
	Faces, Skipped int
	Triangles      int
	// ChordTol is the tolerance used, in millimetres.
	ChordTol float64
	// Reasons counts what was left out by reason, such as
	// "unsupported OFFSET_CURVE_3D", and Details gives the first
	// message of each.
	Reasons map[string]int
	Details map[string]string
	// Crossed counts the faces kept although their bounds crossed in
	// the surface's parameters, which can leave gaps or overlaps.
	Crossed int
}

// Problem is a sentence for the user about what was not imported, or
// "" when everything was.
func (r *Report) Problem() string {
	if len(r.Reasons) == 0 && r.Shapes > 0 && r.Crossed == 0 {
		return ""
	}
	var unsup, other []string
	keys := make([]string, 0, len(r.Reasons))
	for k := range r.Reasons {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		n := r.Reasons[k]
		switch {
		case strings.HasPrefix(k, "unsupported "):
			unsup = append(unsup, fmt.Sprintf("%s (%d)", strings.TrimPrefix(k, "unsupported "), n))
		case strings.HasPrefix(k, "bad "):
			other = append(other, fmt.Sprintf("%d with a bad %s", n, strings.TrimPrefix(k, "bad ")))
		default:
			other = append(other, fmt.Sprintf("%d %s", n, k))
		}
	}
	var parts []string
	if r.Shapes == 0 {
		parts = append(parts, "the file holds no solid or shell that can be read")
		if len(unsup) > 0 {
			parts = append(parts, fmt.Sprintf("it holds %s, which this reader does not support", strings.Join(unsup, ", ")))
		}
		return strings.Join(append(parts, other...), "; ")
	}
	if r.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d faces were left out", r.Skipped, r.Faces+r.Skipped))
		if len(unsup) > 0 {
			parts = append(parts, fmt.Sprintf("unsupported: %s", strings.Join(unsup, ", ")))
		}
		parts = append(parts, other...)
	}
	if r.Crossed > 0 {
		parts = append(parts, fmt.Sprintf("%d faces have bounds that cross, and may show gaps", r.Crossed))
	}
	return strings.Join(parts, "; ")
}

// ReadFile reads a STEP file. The error is for a file that can't be
// read or parsed; what of a parsed file can't be used is in the
// result's Report.
func ReadFile(path string, o Options) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Read(data, o)
}

// Read reads a STEP file's contents.
func Read(data []byte, o Options) (res *Result, err error) {
	defer func() {
		// A file this reader didn't foresee must not take the program
		// down with it.
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("step: %v", r)
		}
	}()
	f, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return FromModel(f.Schema(), ReadModel(f), o), nil
}

// shapeMesh is a shape tessellated once, in its own space.
type shapeMesh struct {
	faces   [][]meshTri
	colours []core.RgbaColor
}

// FromModel tessellates a model.
func FromModel(schema string, m *Model, o Options) *Result {
	res := &Result{}
	rep := &res.Report
	rep.Schema = schema
	rep.Unit = m.Unit
	rep.Reasons = map[string]int{}
	rep.Details = map[string]string{}
	for k, n := range m.unsupported {
		rep.Reasons[k] += n
		rep.Skipped += n
		if d, ok := m.errDetail[k]; ok {
			rep.Details[k] = d
		}
	}
	rep.Shapes = len(m.Shapes)
	rep.Placements = len(m.Placements)

	// The model's size, from its vertices where placed.
	lo, hi := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)},
		geom.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	verts := make([][]geom.Vector, len(m.Shapes))
	centers := make([][]geom.Vector, len(m.Shapes))
	for i, sh := range m.Shapes {
		verts[i], centers[i] = shapePoints(sh)
	}
	for _, pl := range m.Placements {
		for _, v := range verts[pl.Shape] {
			growBox(pl.T.Apply(v), &lo, &hi)
		}
	}
	tol := o.ChordTol
	if tol <= 0 {
		pct := o.ChordTolPct
		if pct <= 0 {
			pct = DefaultChordTolPct
		}
		size := hi.Minus(lo).Magnitude()
		if math.IsInf(size, 0) || math.IsNaN(size) || size == 0 {
			size = 100
		}
		tol = size * pct / 100
	}
	tol = math.Max(tol, 1e-4)
	rep.ChordTol = tol

	meshes := make([]shapeMesh, len(m.Shapes))
	for i, sh := range m.Shapes {
		meshes[i] = tessellateShape(sh, tol, rep)
	}

	seenV := map[geom.Vector]bool{}
	for _, pl := range m.Placements {
		sm := meshes[pl.Shape]
		mirror := pl.T.Det() < 0
		for fi, tris := range sm.faces {
			meta := srf.STriMeta{RgbaColor: sm.colours[fi]}
			for _, t := range tris {
				if len(res.Mesh.Triangles) >= maxTriangles {
					rep.Reasons["past the limit of triangles"]++
					rep.Skipped++
					break
				}
				var st srf.STriangle
				st.Meta = meta
				st.A, st.B, st.C = pl.T.Apply(t.p[0]), pl.T.Apply(t.p[1]), pl.T.Apply(t.p[2])
				st.NA = unit(pl.T.ApplyDir(t.n[0]))
				st.NB = unit(pl.T.ApplyDir(t.n[1]))
				st.NC = unit(pl.T.ApplyDir(t.n[2]))
				if mirror {
					st.B, st.C = st.C, st.B
					st.NB, st.NC = st.NC, st.NB
				}
				res.Mesh.AddTriangle(st)
			}
		}
		for _, v := range verts[pl.Shape] {
			p := pl.T.Apply(v)
			if !seenV[p] {
				seenV[p] = true
				res.Vertices = append(res.Vertices, p)
			}
		}
		for _, c := range centers[pl.Shape] {
			p := pl.T.Apply(c)
			dup := false
			for _, q := range res.Centers {
				if q.EqualsTol(p, 1e-6) {
					dup = true
					break
				}
			}
			if !dup {
				res.Centers = append(res.Centers, p)
			}
		}
	}
	// Where one face's loop has a vertex on an edge that the next face
	// runs past, as a Boolean can leave in the file it was exported
	// from, the meshes meet at a T-junction (roadmap 13.5).
	res.Mesh.SnapTJunctions()
	rep.Triangles = len(res.Mesh.Triangles)
	return res
}

func unit(v geom.Vector) geom.Vector {
	if m := v.Magnitude(); m > 0 {
		return v.ScaledBy(1 / m)
	}
	return v
}

// shapePoints lists a shape's vertices and the centres of its circles,
// in the order the faces reach them.
func shapePoints(sh *Shape) (verts, centers []geom.Vector) {
	seen := map[geom.Vector]bool{}
	seenC := map[geom.Vector]bool{}
	add := func(p geom.Vector) {
		if !seen[p] {
			seen[p] = true
			verts = append(verts, p)
		}
	}
	if len(sh.Tris) > 0 {
		// The corners of the triangles' box: a tessellation has no
		// vertices of its own to tell.
		lo, hi := sh.Tris[0][0], sh.Tris[0][0]
		for _, t := range sh.Tris {
			for _, p := range t {
				growBox(p, &lo, &hi)
			}
		}
		for i := range 8 {
			p := lo
			if i&1 != 0 {
				p.X = hi.X
			}
			if i&2 != 0 {
				p.Y = hi.Y
			}
			if i&4 != 0 {
				p.Z = hi.Z
			}
			add(p)
		}
	}
	for _, f := range sh.Faces {
		for _, b := range f.Bounds {
			for _, p := range b.Poly {
				add(p)
			}
			for _, oe := range b.Edges {
				add(oe.E.Start)
				add(oe.E.End)
				if el, ok := oe.E.Curve.(*ellipse); ok && el.a == el.b && !seenC[el.f.o] {
					seenC[el.f.o] = true
					centers = append(centers, el.f.o)
				}
			}
		}
	}
	return verts, centers
}

// tessellateShape tessellates each face of a shape, side by side,
// keeping the faces' order. Faces that fail are counted in rep.
func tessellateShape(sh *Shape, tol float64, rep *Report) shapeMesh {
	if len(sh.Tris) > 0 {
		// Triangles already: each flat, its normal its own.
		tris := make([]meshTri, 0, len(sh.Tris))
		for _, t := range sh.Tris {
			n := t[1].Minus(t[0]).Cross(t[2].Minus(t[0]))
			if n.Magnitude() == 0 {
				continue
			}
			n = n.WithMagnitude(1)
			tris = append(tris, meshTri{p: t, n: [3]geom.Vector{n, n, n}})
		}
		rep.Faces++
		return shapeMesh{faces: [][]meshTri{tris}, colours: []core.RgbaColor{DefaultColor}}
	}
	sm := shapeMesh{faces: make([][]meshTri, len(sh.Faces)), colours: make([]core.RgbaColor, len(sh.Faces))}
	errs := make([]error, len(sh.Faces))
	var wg sync.WaitGroup
	next := make(chan int)
	workers := runtime.GOMAXPROCS(0)
	for range workers {
		wg.Go(func() {
			for i := range next {
				sm.faces[i], errs[i] = safeTessellate(sh.Faces[i], tol)
			}
		})
	}
	for i := range sh.Faces {
		next <- i
	}
	close(next)
	wg.Wait()
	for i, f := range sh.Faces {
		sm.colours[i] = DefaultColor
		if f.HasColor {
			sm.colours[i] = f.Color
		}
		err := errs[i]
		switch {
		case err == nil:
			rep.Faces++
		case errors.Is(err, errTrim) && len(sm.faces[i]) > 0:
			// Triangulated, but its bounds crossed: kept, and counted.
			rep.Faces++
			rep.Crossed++
		default:
			reason := "that could not be tessellated"
			if errors.Is(err, errTrim) {
				reason = "whose bounds could not be laid out"
			}
			rep.Skipped++
			rep.Reasons[reason]++
			if _, ok := rep.Details[reason]; !ok {
				rep.Details[reason] = fmt.Sprintf("face #%d: %v", f.ID, err)
			}
			sm.faces[i] = nil
		}
	}
	return sm
}

func safeTessellate(f *Face, tol float64) (tris []meshTri, err error) {
	defer func() {
		if r := recover(); r != nil {
			tris, err = nil, fmt.Errorf("%v", r)
		}
	}()
	return tessellateFace(f, tol)
}
