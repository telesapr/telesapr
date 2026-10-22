package ui

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Every model under testdata, opened as the window opens it and edited
// as a person would (groups made dirty, a dimension of a few groups
// changed and changed back, a full regeneration), gives the same
// params, entities, solids and meshes at each step with its solids kept
// by what they were made from as with every solid made again: a solid
// taken from the cache is the one making it would give. Under -short a
// sample of the models.
func TestSolidCacheSameAsMaking(t *testing.T) {
	t.Parallel()
	var files []string
	err := filepath.WalkDir(filepath.Join("..", "..", "testdata"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".tlspr") {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int64
	t.Run("models", func(t *testing.T) {
		for i, f := range files {
			if testing.Short() && i%10 != 0 {
				continue
			}
			t.Run(filepath.ToSlash(strings.TrimPrefix(f, filepath.Join("..", "..", "testdata")+string(filepath.Separator))), func(t *testing.T) {
				t.Parallel()
				made, _ := editedDigests(t, f, false)
				kept, n := editedDigests(t, f, true)
				hits.Add(int64(n))
				for step := range min(len(made), len(kept)) {
					if made[step] != kept[step] {
						t.Errorf("at step %d the model with its solids kept differs from the one made again", step)
						return
					}
				}
				if len(made) != len(kept) {
					t.Errorf("%d steps made, %d kept", len(made), len(kept))
				}
			})
		}
	})
	if hits.Load() == 0 {
		t.Error("no solid was taken from a cache")
	}
}

// editedDigests opens file, edits it and gives a digest of the model
// after each step, and how many solids were taken from the cache.
func editedDigests(t *testing.T, file string, keep bool) ([]string, int) {
	t.Helper()
	abs, err := filepath.Abs(file)
	if err != nil {
		t.Fatal(err)
	}
	p := headless.New()
	p.OnMessageDialog = func(*headless.MessageDialog) { p.DialogResponses = append(p.DialogResponses, platform.ResponseOK) }
	ss := New(p)
	ss.Windowless = true
	ss.Init()
	ss.SaveFile = abs
	if err := ss.LoadFromFile(abs); err != nil {
		return nil, 0
	}
	sk := ss.SK
	sk.NoSolidCache = !keep
	ss.AfterNewFile()
	out := []string{modelDigest(sk)}
	for i := 1; i < len(sk.GroupOrder); i += max(1, len(sk.GroupOrder)/3) {
		sk.MarkGroupDirty(sk.GroupOrder[i], false)
		sk.Refresh()
		out = append(out, modelDigest(sk))
	}
	sk.GenerateAll()
	out = append(out, modelDigest(sk))
	var dims []int
	for ci := range sk.Constraints {
		if c := &sk.Constraints[ci]; c.ValA != 0 && !c.Reference && sk.Group(c.Group) != nil {
			dims = append(dims, ci)
		}
	}
	for n, ci := range dims {
		if n%max(1, len(dims)/3) != 0 {
			continue
		}
		was := sk.Constraints[ci].ValA
		for _, v := range []float64{was * 1.1, was} {
			c := &sk.Constraints[ci]
			c.ValA = v
			sk.MarkGroupDirty(c.Group, false)
			sk.Refresh()
			out = append(out, modelDigest(sk))
		}
	}
	hits, _ := sk.SolidCacheStats()
	return out, hits
}

// modelDigest is a digest of the sketch's params, entities, groups'
// results, solids and meshes, to the bit.
func modelDigest(sk *sketch.Sketch) string {
	h := sha256.New()
	for _, p := range sk.Params {
		fmt.Fprintf(h, "p%d %x %v %v\n", p.H.V, math.Float64bits(p.Val), p.Known, p.Free)
	}
	for _, e := range sk.Entities {
		fmt.Fprintf(h, "e%+v\n", e)
	}
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		fmt.Fprintf(h, "g%d %v %v %v %d %d %v\n", g.H.V, g.BooleanFailed, g.Problem, g.PolyError.How,
			g.Solved.How, g.Solved.Dof, g.ShellsStale)
		shellDigest(h, &g.RunningShell)
		shellDigest(h, &g.ThisShell)
		fmt.Fprintf(h, "%v\n%v\n", g.RunningMesh.Triangles, g.ThisMesh.Triangles)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func shellDigest(h hash.Hash, sh *srf.SShell) {
	for _, s := range sh.Surface {
		fmt.Fprintf(h, "s %d %d %d %v %v %v %v %v %d %d %v\n", s.H.V, s.DegM, s.DegN, s.Ctrl, s.Weight, s.Trim, s.Color,
			s.Face, s.Origin, s.Sheet, s.Tol)
	}
	for _, c := range sh.Curve {
		fmt.Fprintf(h, "c %d %v %v %d %d %d %v\n", c.H.V, c.Exact.Ctrl, c.Exact.Weight, c.SurfA.V, c.SurfB.V, c.Source, c.Points)
	}
}
