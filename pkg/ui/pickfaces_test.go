package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// pickViews are the directions TestEveryFacePicked looks from: along
// each axis both ways and from each corner of a box.
func pickViews() [][2]geom.Vector {
	var out [][2]geom.Vector
	axes := []geom.Vector{{X: 1}, {Y: 1}, {Z: 1}}
	for i := range axes {
		right, up := axes[(i+1)%3], axes[(i+2)%3]
		out = append(out, [2]geom.Vector{right, up}, [2]geom.Vector{right.ScaledBy(-1), up})
	}
	for _, sx := range []float64{-1, 1} {
		for _, sy := range []float64{-1, 1} {
			for _, sz := range []float64{-1, 1} {
				toward := geom.Vector{X: sx, Y: sy, Z: sz}.WithMagnitude(1)
				right := geom.Vector{Z: 1}.Cross(toward)
				if right.Magnitude() < 1e-9 {
					right = geom.Vector{X: 1}
				}
				right = right.WithMagnitude(1)
				up := toward.Cross(right).WithMagnitude(1)
				out = append(out, [2]geom.Vector{right, up})
			}
		}
	}
	return out
}

// TestEveryFacePicked opens each solid golden and, for every face its
// display mesh draws, hovers the middle of that face's largest visible
// triangle from each of several views, and requires the hover to pick
// the face in one of them: an entity's face by its entity, a face no
// entity names by its identity (roadmap 14.1).
func TestEveryFacePicked(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "solid", "*.tlspr"))
	if err != nil || len(files) == 0 {
		t.Fatal("no solid goldens", err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".tlspr"), func(t *testing.T) {
			ss := New(headless.New())
			ss.Init()
			if _, err := os.Stat(file); err != nil || !ss.LoadFromFile(file) {
				t.Fatal("load failed")
			}
			ss.AfterNewFile()
			gw := ss.GW
			w := gw.window.(*headless.Window)
			w.Width, w.Height = 1200, 900
			g := ss.SK.GetGroup(gw.activeGroupH())
			g.GenerateDisplayItems(ss.SK)
			picked := map[uint32]bool{}
			faces := map[uint32]bool{}
			for _, tr := range g.DisplayMesh.Triangles {
				faces[uint32(tr.Meta.Face)] = true
			}
			delete(faces, 0)
			for _, v := range pickViews() {
				gw.view.ProjRight, gw.view.ProjUp = v[0], v[1]
				gw.ZoomToFit(false, false)
				w.Render()
				// The largest triangles of each face on the screen; the
				// first eight are tried, a face's largest may lie behind
				// another or under an entity.
				type cand struct {
					p geom.Vector
					a float64
				}
				cands := map[uint32][]cand{}
				for _, tr := range g.DisplayMesh.Triangles {
					f := uint32(tr.Meta.Face)
					if f == 0 || picked[f] {
						continue
					}
					a, b, c := gw.ProjectPoint(tr.A), gw.ProjectPoint(tr.B), gw.ProjectPoint(tr.C)
					s := math.Abs((b.U-a.U)*(c.V-a.V)-(c.U-a.U)*(b.V-a.V)) / 2
					cands[f] = append(cands[f], cand{tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0 / 3), s})
				}
				for f, cs := range cands {
					sort.Slice(cs, func(i, j int) bool { return cs[i].a > cs[j].a })
					for k := 0; k < len(cs) && k < 8 && !picked[f]; k++ {
						q := gw.ProjectPoint(cs[k].p)
						w.Mouse(platform.MouseEvent{Type: platform.MouseMotion, X: w.Width/2 + q.U, Y: w.Height/2 - q.V})
						if gw.hover.Entity.V == f || uint32(gw.hover.Face) == f {
							picked[f] = true
						}
					}
				}
			}
			// A face no view shows, as the inside of a bent pipe, is
			// reached through the section view: cut by each reference
			// plane just beyond a point of the face, looking at the cut.
			planes := []struct {
				h      core.HEntity
				n      geom.Vector
				right  geom.Vector
				coordA func(geom.Vector) float64
			}{
				{sketch.HRequestReferenceXY.Entity(0), geom.Vector{Z: 1}, geom.Vector{X: 1}, func(p geom.Vector) float64 { return p.Z }},
				{sketch.HRequestReferenceYZ.Entity(0), geom.Vector{X: 1}, geom.Vector{Y: 1}, func(p geom.Vector) float64 { return p.X }},
				{sketch.HRequestReferenceZX.Entity(0), geom.Vector{Y: 1}, geom.Vector{Z: 1}, func(p geom.Vector) float64 { return p.Y }},
			}
			for _, tr := range g.DisplayMesh.Triangles {
				f := uint32(tr.Meta.Face)
				if f == 0 || picked[f] {
					continue
				}
				p := tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0 / 3)
				for _, pl := range planes {
					for _, flip := range []bool{false, true} {
						off, n := pl.coordA(p)+0.5, pl.n
						if flip {
							off, n = pl.coordA(p)-0.5, n.ScaledBy(-1)
						}
						if err := ss.SK.SetSection(pl.h, fmt.Sprint(off), flip); err != nil {
							t.Fatal(err)
						}
						gw.view.ProjRight, gw.view.ProjUp = pl.right, n.Cross(pl.right)
						gw.ZoomToFit(false, false)
						w.Render()
						q := gw.ProjectPoint(p)
						w.Mouse(platform.MouseEvent{Type: platform.MouseMotion, X: w.Width/2 + q.U, Y: w.Height/2 - q.V})
						if gw.hover.Entity.V == f || uint32(gw.hover.Face) == f {
							picked[f] = true
						}
					}
				}
				ss.SK.Section = nil
			}
			for f := range faces {
				if !picked[f] {
					t.Errorf("face %#x was picked from no view", f)
				}
			}
		})
	}
}
