package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A full-length thread made on a face (AddThreadOnFace) is as long as
// the face, and the face's length changes after the thread is made: a
// parameter lengthens the rod, a hole is drilled deeper. The thread's
// datum finds the face again at each regeneration (facePlane) and
// measures it then as well; when the face's length has changed, every
// dimension that carries the thread's length is moved by the
// difference before the thread's groups are solved, as a length's
// expression moves them. The thread's record keeps which dimensions
// those are (GroupThread.LengthDims). What a length can't follow is
// reported on the thread's group with its solid: a face come to reach
// the part's end, or to stop short of it, changes the shape of the
// blank, and the thread is edited to follow that.

// followThread gives the full-length thread on the datum, if one is
// there, the length of its face in solid, the solid of the groups
// before the datum, found when the datum's frame was; it reports
// whether the thread's dimensions moved, which the groups after the
// datum are then solved for.
func (sk *Sketch) followThread(datum *Group, solid *Group, found bool) bool {
	hg, ok := sk.threadOnDatum(datum.H)
	if !ok {
		return false
	}
	keeper := sk.Group(hg)
	keeper.threadProblem = ""
	if !found {
		// The datum reports its face missing.
		return false
	}
	at, far, ok := sk.datumAxisEnds(datum, solid)
	if !ok {
		return false
	}
	o := keeper.Thread.ThreadOptions
	switch {
	case far.Free && !o.Through:
		keeper.threadProblem = "the face now reaches the end of the part; edit the thread to run it through"
	case !far.Free && o.Through:
		keeper.threadProblem = "the face now ends at a shoulder; edit the thread to stop it there"
	}
	length := math.Round(at.Length*1e6) / 1e6
	if math.Abs(length-o.Length) < 1e-9 {
		return false
	}
	if err := sk.moveThreadLength(keeper, at.Radius, length); err != nil {
		keeper.threadProblem = fmt.Sprintf("the face is now %.4g mm long, which the thread doesn't follow: %v; edit the thread", length, err)
		return false
	}
	return true
}

// threadOnDatum is the group keeping the full-length thread drawn on
// the circle in the datum group hg, if there is one.
func (sk *Sketch) threadOnDatum(hg core.HGroup) (core.HGroup, bool) {
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if !g.Thread.IsThread() || !g.Thread.FullLength {
			continue
		}
		if r := sk.Request(g.Thread.Circle.Request()); r != nil && r.Group == hg {
			return g.H, true
		}
	}
	return core.HGroup{}, false
}

// datumAxisEnds are the ends of the round face a thread's datum names,
// in solid: the end the datum is at, and the far one.
func (sk *Sketch) datumAxisEnds(datum *Group, solid *Group) (at, far topo.AxisEnd, ok bool) {
	t, err := solidTopology(sk, solid)
	if err != nil {
		return at, far, false
	}
	ends, err := t.AxisEnds(datum.SolidFaces[0])
	if err != nil {
		return at, far, false
	}
	i := slices.IndexFunc(ends[:], func(e topo.AxisEnd) bool { return e.End == datum.SolidFaces[1] })
	if i < 0 {
		return at, far, false
	}
	return ends[i], ends[1-i], true
}

// moveThreadLength makes the thread that keeper keeps length mm long,
// on a circle of radius r, by moving the dimensions that carry its
// length, and marks its groups to be solved again. It is an error when
// the thread can't be laid out at that length, or a dimension is gone
// or no longer plain.
func (sk *Sketch) moveThreadLength(keeper *Group, r, length float64) error {
	tg, err := sk.threadGroupsOf(keeper.H)
	if err != nil {
		return err
	}
	o := keeper.Thread.ThreadOptions
	delta := length - o.Length
	o.Length = length
	if _, err := o.layout(r); err != nil {
		return err
	}
	if len(keeper.Thread.LengthDims) == 0 {
		return errors.New("it was made before threads followed their faces")
	}
	dims := make([]*core.Constraint, 0, len(keeper.Thread.LengthDims))
	for _, h := range keeper.Thread.LengthDims {
		c := sk.Constraint(h)
		switch {
		case c == nil:
			return errors.New("a dimension of its length is gone")
		case c.Expr != "":
			return fmt.Errorf("the dimension %s has an expression", c.DescriptionString())
		case math.Abs(c.ValA)+delta <= 0:
			return fmt.Errorf("the dimension %s would be %.4g mm", c.DescriptionString(), math.Abs(c.ValA)+delta)
		}
		dims = append(dims, c)
	}
	for _, c := range dims {
		c.ValA = math.Copysign(math.Abs(c.ValA)+delta, c.ValA)
	}
	keeper.Thread.Length = length
	sk.markDirtyFrom(tg.blank)
	return nil
}
