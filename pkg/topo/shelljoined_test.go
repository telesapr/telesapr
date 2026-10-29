package topo_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// soundness is the check of sh, meshed.
func soundness(sh *srf.SShell) *check.Report {
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	return &r
}

// facesAt are the faces of sh at the points ps.
func facesAt(t *testing.T, sh *srf.SShell, ps ...geom.Vector) []int {
	t.Helper()
	tp := build(t, sh)
	var out []int
	for _, p := range ps {
		f, d := tp.FaceAt(p)
		if f < 0 || d > 1e-6 {
			t.Fatalf("no face at %v", p)
		}
		out = append(out, f)
	}
	return out
}

// A cylinder with a round post standing up through its top, joined, its
// post's foot on the cylinder's bottom: open at the post's top, the
// inside is the cylinder's and the post's joined; open at a side of the
// post, the shell is refused.
func TestShellJoinedPost(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body, post := cylinder(10, 10, tol), cylinder(3, 16, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromUnionOf(body, post)
	parts := []topo.ShellPart{{Shell: body}, {Shell: post}}
	out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(0, 0, 16)), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	want := math.Pi * (100*10 + 9*6 - (81*8 + 4*7))
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("post on a cylinder: %s, want %g", r.String(), want)
	}
	if _, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(3, 0, 13)), 1, 1000); err == nil ||
		!strings.Contains(err.Error(), "open only at its ends") {
		t.Errorf("open at the post's side: %v", err)
	}
	if _, err := topo.ShellJoined(sh, nil, facesAt(t, sh, v(0, 0, 16)), 1, 1000); err == nil {
		t.Error("no parts: no error")
	}
	if _, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(0, 0, 16)), 0, 1000); err == nil {
		t.Error("no thickness: no error")
	}
}

// A post sunk into a plate thinner than twice the thickness, from inside
// it: the post's foot moved out by the thickness comes out of the plate's
// bottom, and the shell is refused.
func TestShellJoinedThroughPlate(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	plate := &srf.SShell{Tol: tol}
	plate.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 3),
		core.NewRGBA(100, 100, 100, 255))
	post := &srf.SShell{Tol: tol}
	post.MakeFromTransformationOf(cylinder(3, 10, tol), v(0, 0, 0.5), geom.QuaternionIdentity, 1)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromUnionOf(plate, post)
	parts := []topo.ShellPart{{Shell: plate}, {Shell: post}}
	_, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(0, 0, 10.5)), 1, 1000)
	if err == nil || !strings.Contains(err.Error(), "out of the solid") {
		t.Errorf("a post's foot through the plate: %v", err)
	}
}

// A drafted cylinder, its side a cone: hollowed open at its top, the
// inside is the cone moved in square to itself; a solid that is not
// the drafted one is no such shell.
func TestShellDraftedCone(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	a := 5 * math.Pi / 180
	d := topo.DraftedSolid{
		Sets: []topo.DraftedSet{{Loops: [][]srf.SBezier{circle(10, 0, 0)}, Angles: [][]float64{{a, a, a, a}}}},
		Dir:  v(0, 0, 1), Lo: 0, Hi: 10,
	}
	sh, err := topo.DraftedPrism(d.Sets[0].Loops, v(0, 0, 10), a, core.NewRGBA(100, 100, 100, 255), 0, tol)
	if err != nil {
		t.Fatal(err)
	}
	top := facesAt(t, sh, v(0, 0, 10))
	out, err := topo.ShellDrafted(sh, d, top, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	tan, sec := math.Tan(a), 1/math.Cos(a)
	frustum := func(h, r1, r2 float64) float64 { return math.Pi * h / 3 * (r1*r1 + r1*r2 + r2*r2) }
	want := frustum(10, 10, 10-10*tan) - frustum(9, 10-sec-tan, 10-sec-10*tan)
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("drafted cone: %s, want %g", r.String(), want)
	}
	if _, err := topo.ShellDrafted(sh, topo.DraftedSolid{}, top, 1, 1000); !errors.Is(err, topo.ErrNotDrafted) {
		t.Errorf("no profile: %v", err)
	}
	if _, err := topo.ShellDrafted(sh, d, top, -1, 1000); err == nil {
		t.Error("a negative thickness: no error")
	}
	if _, err := topo.ShellDrafted(sh, d, top, 10, 1000); err == nil {
		t.Error("walls that meet: no error")
	}
}
