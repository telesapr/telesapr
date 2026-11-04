package srf

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// moveOrphanTrims moves the trims a kept surface has on a curve whose
// other surface was dropped onto the curves that run along the same
// points between it and a surface that was kept (roadmap K3, a flush
// cutter's orphaned edge). A cutter flush with a face loses its cap,
// which lay on the face; its side kept the edge it shared with the cap,
// beside the face's intersection curve along the same outline with the
// same points. Left so, the edge is two curves, and the one the side
// uses names the dropped cap, a handle that the next Boolean, finding
// no surface for it, carried into its own result unchanged, where it
// named whichever surface came to have that number. Moved, the side
// and the face meet on one curve, and the orphan, used by no trim,
// goes with the cap.
//
// A trim is moved only if its run of points is covered, point for
// point, by such curves; otherwise it is left as it was.
func (s *SShell) moveOrphanTrims(dropped map[core.HSSurface]bool) {
	orphan := map[core.HSCurve]*SCurve{}
	for i := range s.Curve {
		c := &s.Curve[i]
		if dropped[c.SurfA] != dropped[c.SurfB] {
			orphan[c.H] = c
		}
	}
	if len(orphan) == 0 {
		return
	}
	for i := range s.Surface {
		sf := &s.Surface[i]
		if dropped[sf.H] {
			continue
		}
		var trims []STrimBy
		moved := false
		for _, t := range sf.Trim {
			c := orphan[t.Curve]
			if c == nil {
				trims = append(trims, t)
				continue
			}
			if m, ok := s.coverTrim(sf.H, c, t, dropped); ok {
				trims = append(trims, m...)
				moved = true
				continue
			}
			trims = append(trims, t)
		}
		if moved {
			sf.Trim = trims
		}
	}
}

// trimRun is the points of c that t runs along, in the order it runs,
// or nil if t's ends are not among them.
func trimRun(c *SCurve, t STrimBy) []geom.Vector {
	var run []geom.Vector
	in := false
	n := len(c.Points)
	for k := range n {
		i := k
		if t.Backwards {
			i = n - 1 - k
		}
		p := c.Points[i].P
		if p.Equals(t.StartP) {
			in = true
		}
		if in {
			run = append(run, p)
		}
		if in && p.Equals(t.FinishP) {
			return run
		}
	}
	return nil
}

// coverTrim gives the trims, on curves other than c between surface h
// and a kept surface, that run along t's points in t's order, or false
// if they don't cover every one of them.
func (s *SShell) coverTrim(h core.HSSurface, c *SCurve, t STrimBy, dropped map[core.HSSurface]bool) ([]STrimBy, bool) {
	run := trimRun(c, t)
	if len(run) < 2 {
		return nil, false
	}
	var out []STrimBy
	for k := 0; k < len(run)-1; {
		m, ok := s.coverFrom(h, c, run[k:], dropped)
		if !ok {
			return nil, false
		}
		out = append(out, m.trim)
		k += m.steps
	}
	return out, true
}

// cover is a trim along one curve over the first steps of a run.
type cover struct {
	trim  STrimBy
	steps int
}

// coverFrom finds the curve, between h and a kept surface, that runs
// along the most of run's first points, from its first, one way or the
// other.
func (s *SShell) coverFrom(h core.HSSurface, c *SCurve, run []geom.Vector, dropped map[core.HSSurface]bool) (cover, bool) {
	var best cover
	for i := range s.Curve {
		d := &s.Curve[i]
		if d.H == c.H || (d.SurfA != h && d.SurfB != h) || dropped[d.SurfA] || dropped[d.SurfB] {
			continue
		}
		for j := range d.Points {
			if !d.Points[j].P.Equals(run[0]) {
				continue
			}
			for _, step := range [2]int{1, -1} {
				n := 0
				for n+1 < len(run) {
					at := j + (n+1)*step
					if at < 0 || at >= len(d.Points) || !d.Points[at].P.Equals(run[n+1]) {
						break
					}
					n++
				}
				if n > best.steps {
					best = cover{
						trim: STrimBy{
							Curve:     d.H,
							Backwards: step < 0,
							StartP:    run[0],
							FinishP:   run[n],
						},
						steps: n,
					}
				}
			}
		}
	}
	return best, best.steps > 0
}
