package api

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// AssemblyMenu holds the calls of the Assembly menu: linked parts,
// mates, interference, the bill of materials and motion. A script
// reaches it as doc.Assembly.
type AssemblyMenu struct{ d *Doc }

// BillOfMaterials is the parts the model places by linking files, one
// item a file: each link group places one, and a step and repeat, a
// mirror or a pattern of a link group its copies, but for one where the
// part already is. Groups left out of the model place none. The parts a
// linked model links in turn count too, once for each place the model
// is, their groups named "here > there".
func (m AssemblyMenu) BillOfMaterials() (items []sketch.BOMItem) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { items = sk.BillOfMaterials() })
	return items
}

// ExportBOM writes the bill of materials to a CSV file: part, count,
// file and the groups that place them.
func (m AssemblyMenu) ExportBOM(path string) error {
	d := m.d
	if err := d.checkFile(path, true); err != nil {
		return err
	}
	var b bytes.Buffer
	if err := sketch.WriteBOMCSV(&b, d.Assembly.BillOfMaterials()); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// Link brings in another sketch file, a STEP file, or a mesh file (STL
// or OBJ), as
// a group, named after the file if name is empty. The new group becomes
// active. A STEP file comes in as a reference mesh, its faces
// tessellated, with a point at each vertex and at the centre of each
// circular edge for constraints to use; LinkInfo and LinkProblem say
// what was read.
func (m AssemblyMenu) Link(path, name string) (g Group, err error) {
	d := m.d
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if !ok {
				panic(r)
			}
			err = e
		}
	}()
	if err := d.checkFile(path, false); err != nil {
		return Group{}, err
	}
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupLinked, name, "")
		g.MeshCombine = sketch.CombineAssemble
		abs, err := filepath.Abs(path)
		if err != nil {
			fail("%v", err)
		}
		g.LinkFile = abs
		if name == "" {
			stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			g.Name = strings.Map(func(r rune) rune {
				if r >= 0x80 || unicode.IsLetter(r) || unicode.IsDigit(r) {
					return r
				}
				return '-'
			}, stem)
		}
		return g
	}, nil), nil
}

// LinkStep links a STEP file as Link does, as a solid when asSolid is
// set: its faces exact, so the solid takes part in Booleans, fillets
// and the rest as a solid made here does. A file that can't be read as
// a solid comes in as a reference mesh, and LinkProblem says why.
func (m AssemblyMenu) LinkStep(path, name string, asSolid bool) (Group, error) {
	d := m.d
	g, err := d.Assembly.Link(path, name)
	if err != nil || !asSolid {
		return g, err
	}
	g.AsLink().SetAsSolid(true)
	return g, nil
}

// Concentric puts two circles or arcs on one axis. Drawing free in 3d,
// their axes are made parallel and the second's centre put on the
// first's axis, by its distance from the first's centre along two
// directions square to the axis, each zero; one part may still turn
// about the axis and slide along it, as a pin in a hole. The directions
// are lines or normals of the first circle's group, or the reference
// axes; with none square to the axis, the centres are put on each
// other. In a workplane, the centres are put on each other.
func (m AssemblyMenu) Concentric(a, b Entity) []Constraint {
	d := m.d
	var ca, cb Entity
	var across []core.HEntity
	free := false
	d.read(func(sk *sketch.Sketch) {
		for _, e := range []Entity{a, b} {
			if t := e.get(sk).Type; t != core.EntityCircle && t != core.EntityArcOfCircle {
				fail("a concentric mate is of two circles or arcs, and %s isn't one", e.describe(sk))
			}
		}
		ca, cb = a.Center(), b.Center()
		free = core.IsFreeIn3D(sk.ActiveWorkplane())
		ea := a.get(sk)
		axis := ea.NormalEntity(sk).NormalN(sk).WithMagnitude(1)
		for i := range sk.Entities {
			e := &sk.Entities[i]
			if e.Group != ea.Group && e.Group.V != core.HGroupReferences || !e.HasVector() {
				continue
			}
			v := e.VectorGetNum(sk)
			if v.Magnitude() < core.LengthEps || math.Abs(v.WithMagnitude(1).Dot(axis)) > 1e-6 {
				continue
			}
			if len(across) == 1 {
				u := sk.Entity(across[0]).VectorGetNum(sk).WithMagnitude(1)
				if u.Cross(v.WithMagnitude(1)).Magnitude() < 0.1 {
					continue
				}
			}
			across = append(across, e.H)
			if len(across) == 2 {
				break
			}
		}
	})
	if !free {
		return []Constraint{d.Constrain.Coincident(ca, cb)}
	}
	// The axes parallel first, as for Flush.
	out := d.addUnlessRedundant(nil, func() Constraint { return d.Constrain.Parallel(a.Normal(), b.Normal()) })
	if len(across) < 2 {
		return append(out, d.Constrain.Coincident(ca, cb))
	}
	for _, dir := range across {
		out = append(out, d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
			c.Type = core.ConstraintProjPtDistance
			c.Workplane = core.NoEntity // free in 3d
			c.PtA, c.PtB, c.EntityA = ca.H, cb.H, dir
		}))
	}
	return out
}

// Flush makes two faces parallel and puts a point of the first, one
// lying on it, on the second: the faces lie in one plane, facing as
// they did, and one part may still slide and turn on the other.
func (m AssemblyMenu) Flush(faceA, faceB Entity) []Constraint {
	d := m.d
	var on Entity
	d.read(func(sk *sketch.Sketch) {
		fa := faceA.get(sk)
		for _, e := range []Entity{faceA, faceB} {
			if !e.get(sk).IsFace() {
				fail("a flush mate is of two faces, and %s isn't one", e.describe(sk))
			}
		}
		// A point of the first face's group, lying on it.
		p0, n := fa.FaceGetPointNum(sk), fa.FaceGetNormalNum(sk).WithMagnitude(1)
		for i := range sk.Entities {
			e := &sk.Entities[i]
			if e.Group == fa.Group && e.IsPoint() && math.Abs(e.PointGetNum(sk).Minus(p0).Dot(n)) < core.LengthEps {
				on = Entity{d, e.H}
				return
			}
		}
		fail("%s has no point lying on it to put on the other face", faceA.describe(sk))
	})
	// The faces parallel first: a point put on the plane first would
	// tilt the part to reach it, and the solver might not turn it back.
	out := d.addUnlessRedundant(nil, func() Constraint {
		// Parallel takes faces as vectors, their normals, though the
		// interface offers it for lines and normals only.
		return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
			c.Type = core.ConstraintParallel
			c.Workplane = core.NoEntity // free in 3d
			c.EntityA, c.EntityB = faceA.H, faceB.H
		})
	})
	return append(out, d.Constrain.On(on, faceB))
}

// Hinge puts the start of line b on the start of line a, and the end of
// b on a's line: b may only turn about a, as on a pin.
func (m AssemblyMenu) Hinge(a, b Entity) []Constraint {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, e := range []Entity{a, b} {
			if e.get(sk).Type != core.EntityLineSegment {
				fail("a hinge is of two line segments, and %s isn't one", e.describe(sk))
			}
		}
	})
	out := []Constraint{d.Constrain.Coincident(a.Start(), b.Start())}
	return d.addUnlessRedundant(out, func() Constraint { return d.Constrain.On(b.End(), a) })
}

// MotionStudy drives the parameter param from from to to in steps
// equal steps, steps+1 frames with both ends, regenerating the model at
// each (roadmap 16.3). With frames not "", each frame is saved to the
// file frames names with the step's number in place of its verb, as
// "frames/f%03d.png": a picture of the view the window shows for .png,
// in the application, and otherwise what Export writes for the
// extension. The parameter gets its expression back after, whatever
// happened. A step whose groups don't all solve is kept, marked so.
func (m AssemblyMenu) MotionStudy(param string, from, to float64, steps int, frames string) (out []MotionFrame, err error) {
	d := m.d
	p, ok := d.Params.Get(param)
	if !ok {
		return nil, fmt.Errorf("no parameter %q", param)
	}
	if steps < 1 || steps > 10000 {
		return nil, fmt.Errorf("a motion study takes from 1 to 10000 steps, not %d", steps)
	}
	if frames != "" && !strings.Contains(frames, "%") {
		return nil, errors.New("the frames' file name needs a verb for the step's number, as f%03d.png")
	}
	defer func() {
		if rerr := d.Params.Set(param, p.Expr); rerr != nil && err == nil {
			err = rerr
		}
		d.Edit.Regen()
	}()
	for i := 0; i <= steps; i++ {
		v := from + (to-from)*float64(i)/float64(steps)
		if err := d.Params.Set(param, strconv.FormatFloat(v, 'g', -1, 64)); err != nil {
			return out, fmt.Errorf("step %d, %s = %g: %w", i, param, v, err)
		}
		d.Edit.Regen()
		f := MotionFrame{Step: i, Value: v, Solved: true}
		for _, g := range d.Groups() {
			f.Solved = f.Solved && g.Solved()
		}
		f.Volume = d.Active().Analyze().Check().Volume
		if frames != "" {
			f.File = fmt.Sprintf(frames, i)
			var ferr error
			if strings.EqualFold(filepath.Ext(f.File), ".png") {
				ferr = d.File.Render(f.File, 800, 600, "window")
			} else {
				ferr = d.File.Export(f.File)
			}
			if ferr != nil {
				return out, fmt.Errorf("frame %d: %w", i, ferr)
			}
		}
		out = append(out, f)
	}
	return out, nil
}

// Interferences checks the parts of the assembly up to the active
// group against each other by their shells (Analyze > Show Interfering
// Parts): each group that combines as an assembly is a part, and so is
// the solid made before the first of them. It lists the pairs that
// overlap, not those that only touch. A cut made after the parts cuts
// each one it reaches. Where the Boolean of two parts' shells fails, as
// of surfaces lying on each other, their meshes are intersected
// instead and the pair marked Approximate; a pair whose meshes fail too
// is listed as Failed. A part is checked by its pieces, the solids its
// shell holds, so that a linked assembly's own parts may stand in each
// other. A part with no shell, as a
// linked mesh or STEP file, can't be checked so; meshOnly lists them.
func (m AssemblyMenu) Interferences() (found []Interference, meshOnly []Group) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		fs, mo := sk.Interferences(sk.ActiveGroup())
		for _, f := range fs {
			found = append(found, Interference{A: Group{d, f.A}, B: Group{d, f.B}, Volume: f.Volume,
				Approximate: f.Approximate, Failed: f.Failed})
		}
		for _, h := range mo {
			meshOnly = append(meshOnly, Group{d, h})
		}
	})
	return found, meshOnly
}

// Clearances are the gaps between the parts of the assembly up to the
// active group that are less than below mm, the nearest first (Analyze
// > Clearance and Walls); a below of 0 lists every pair's. The parts
// are those Interferences checks, and a part with no shell, as a linked
// mesh, is taken by its triangles. Parts that touch or overlap have a
// gap of 0.
func (m AssemblyMenu) Clearances(below float64) (found []Clearance) {
	d := m.d
	d.readMesh(func(sk *sketch.Sketch) {
		for _, c := range sk.Clearances(sk.ActiveGroup(), below) {
			found = append(found, clearanceOf(d, c))
		}
	})
	return found
}

// ClearancesWithin is Clearances with each part of several solids, as
// a linked assembly, taken by its solids (Analyze > Clearance and Walls,
// "solids within parts"): the gaps between the solids of one part are
// listed too, and Clearance.SolidA and SolidB say which solid of a part
// a gap is to.
func (m AssemblyMenu) ClearancesWithin(below float64) (found []Clearance) {
	d := m.d
	d.readMesh(func(sk *sketch.Sketch) {
		for _, c := range sk.ClearancesWithin(sk.ActiveGroup(), below) {
			found = append(found, clearanceOf(d, c))
		}
	})
	return found
}

// clearanceOf is a sketch's clearance as a script sees it.
func clearanceOf(d *Doc, c sketch.Clearance) Clearance {
	return Clearance{A: Group{d, c.A}, B: Group{d, c.B}, Gap: c.Gap, PA: c.PA, PB: c.PB,
		Tolerance: c.Tolerance, Refined: c.Refined, SolidA: c.SolidA, SolidB: c.SolidB}
}
