package topo_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/check"
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/internal/sweep"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

// filletLoss is the volume a fillet of radius r takes off a straight
// edge of length l whose faces meet at interior angle beta: the area
// between the corner and the arc, times l.
func filletLoss(r, beta, l float64) float64 {
	s := r / math.Tan(beta/2)
	return l * (r*s - r*r*(math.Pi-beta)/2)
}

func TestFilletBoxEdge(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	sh := box()
	if err := topo.FilletEdge(sh, v(0, 0, 30), v(10, 0, 30), 2, 1<<30|0x77<<16); err != nil {
		t.Fatal(err)
	}
	tp := build(t, sh)
	if len(tp.Faces) != 7 || len(tp.Edges) != 15 || len(tp.Vertices) != 10 {
		t.Errorf("V %d E %d F %d", len(tp.Vertices), len(tp.Edges), len(tp.Faces))
	}
	var m srf.SMesh
	sh.TriangulateInto(&m)
	want := 6000 - filletLoss(2, math.Pi/2, 10)
	// The mesh's chords cut inside the arc by up to the tolerance.
	if got := m.CalculateVolume(); math.Abs(got-want) > 2*0.001*(math.Pi/2*2)*10 {
		t.Errorf("volume %.6f, want %.6f", got, want)
	}
	if e, me := tp.Euler(), topo.MeshEuler(&m); e != 2 || me != 2 {
		t.Errorf("Euler %d, the mesh's %d", e, me)
	}
}

func TestFilletRefusals(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b geom.Vector
		r    float64
		want string
	}{
		{"no edge", v(1, 1, 1), v(2, 2, 2), 1, "no edge"},
		{"part of an edge", v(0, 0, 30), v(5, 0, 30), 1, "not a whole edge"},
		{"too wide", v(0, 0, 30), v(10, 0, 30), 25, "wider than a face"},
		{"no radius", v(0, 0, 30), v(10, 0, 30), 0, "more than zero"},
	} {
		sh := box()
		err := topo.FilletEdge(sh, c.a, c.b, c.r, 0)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	// Two edges that meet at a corner.
	sh := box()
	err := topo.FilletEdges(sh, [][2]geom.Vector{{v(0, 0, 30), v(10, 0, 30)}, {v(10, 0, 30), v(10, 20, 30)}}, 1, 0)
	if err == nil || !strings.Contains(err.Error(), "meet") {
		t.Errorf("edges meeting: %v", err)
	}
	var fe *topo.FilletError
	if !errors.As(err, &fe) || !fe.At.Equals(v(10, 0, 30)) {
		t.Errorf("where: %+v", fe)
	}
}

func TestFilletEdges(t *testing.T) {
	sh := box()
	// Two parallel edges of the top, apart.
	if err := topo.FilletEdges(sh, [][2]geom.Vector{{v(0, 0, 30), v(10, 0, 30)}, {v(0, 20, 30), v(10, 20, 30)}}, 3, 1<<30); err != nil {
		t.Fatal(err)
	}
	tp := build(t, sh)
	if len(tp.Faces) != 8 {
		t.Errorf("%d faces", len(tp.Faces))
	}
	// The fillets' faces have names through their origins.
	named := 0
	for e := range tp.Edges {
		if _, err := tp.NameOf(e); err == nil {
			named++
		}
	}
	if named != len(tp.Edges) {
		t.Errorf("%d of %d edges named", named, len(tp.Edges))
	}
}

// TestFilletSweep fillets the edge of a triangular prism along a grid
// of angles, radii, lengths and leans of the prism, which makes the end
// faces oblique to the edge. Every point must give a closed, valid
// solid with the volume the formula gives; the rate allowed is 0, set
// before the fillet was built.
func TestFilletSweep(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("angle", 20, 45, 60, 90, 120, 150, 165),
			sweep.Floats("k", 0.05, 0.4, 0.8),
			sweep.Floats("length", 0.5, 5, 40),
			sweep.Floats("lean", 0, 0.6),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		th := p.Float("angle") * math.Pi / 180
		length := p.Float("length")
		lean := v(p.Float("lean")*length, 0, length)
		// The directions into the two faces square to the edge, and the
		// largest radius whose fillet fits their 10 long ends.
		e := lean.WithMagnitude(1)
		d1, d2 := v(1, 0, 0), v(math.Cos(th), math.Sin(th), 0)
		u1 := d1.Minus(e.ScaledBy(d1.Dot(e))).WithMagnitude(1)
		u2 := d2.Minus(e.ScaledBy(d2.Dot(e))).WithMagnitude(1)
		half := math.Acos(math.Max(-1, math.Min(1, u1.Dot(u2)))) / 2
		r := p.Float("k") * 10 * math.Min(d1.Dot(u1), d2.Dot(u2)) * math.Tan(half)
		var sh srf.SShell
		sh.MakePrism([]geom.Vector{v(0, 0, 0), v(10, 0, 0), v(10*math.Cos(th), 10*math.Sin(th), 0)},
			v(0, 0, 0), lean, core.NewRgbaColor(100, 100, 100, 255))
		if err := topo.FilletEdge(&sh, v(0, 0, 0), lean, r, 1<<30); err != nil {
			return err
		}
		tp, err := topo.Build(&sh)
		if err != nil {
			return err
		}
		if probs := tp.Validate(); len(probs) > 0 {
			return fmt.Errorf("%s", probs[0].Msg)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(&sh, &m)
		if !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		// The faces' angle across the edge, which leans with the prism.
		n1 := lean.Cross(v(1, 0, 0)).WithMagnitude(1)
		n2 := v(math.Cos(th), math.Sin(th), 0).Cross(lean).WithMagnitude(1)
		beta := math.Pi - math.Acos(math.Max(-1, math.Min(1, n1.Dot(n2))))
		want := 50*math.Sin(th)*length - filletLoss(r, beta, lean.Magnitude())
		arcLen := r * (math.Pi - beta)
		if d := math.Abs(rep.Volume - want); d > 2*0.001*arcLen*lean.Magnitude()+1e-9*want {
			return fmt.Errorf("volume %g, want %g", rep.Volume, want)
		}
		return nil
	}).Require(t)
}
