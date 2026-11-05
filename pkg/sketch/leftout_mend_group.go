package sketch

import (
	"errors"
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file mends a group a damaged file lacks: a request's or a
// constraint's group, or a group's operand, that the file names and
// doesn't hold. The user picks one of the model's groups in its place,
// as a mend of an entity takes one of the model's entities.

// ErrGroupNotMissing is MendLeftOutGroup's error when nothing left out
// uses the group it is to replace.
var ErrGroupNotMissing = errors.New("nothing left out uses that group")

// groupRef is a reference of a left-out item to a group, which a mend
// may replace; user is the left-out group whose operand it is, nil for
// a request's or a constraint's group.
type groupRef struct {
	at   *core.HGroup
	user *Group
}

// groupRefs are the references of what was left out to groups: a
// request's and a constraint's group, and a group's operands and the
// profiles a loft passes through.
func (l *LeftOut) groupRefs() []groupRef {
	var out []groupRef
	for i := range l.Requests {
		out = append(out, groupRef{at: &l.Requests[i].Group})
	}
	for i := range l.Constraints {
		out = append(out, groupRef{at: &l.Constraints[i].Group})
	}
	for i := range l.Groups {
		g := &l.Groups[i]
		out = append(out, groupRef{&g.OpA, g}, groupRef{&g.OpB, g})
		for k := range g.LoftThrough {
			out = append(out, groupRef{&g.LoftThrough[k], g})
		}
	}
	return out
}

// lacksGroup reports whether h is a group that neither the model nor
// what was left out of it holds.
func (sk *Sketch) lacksGroup(h core.HGroup) bool {
	if h.V == 0 || sk.Group(h) != nil {
		return false
	}
	return !slices.ContainsFunc(sk.LeftOut.Groups, func(g Group) bool { return g.H == h })
}

// LeftOutMissingGroups are the groups that what the load of a damaged
// file left out uses and neither the model nor what was left out
// holds, which a mend (MendLeftOutGroup) may replace, in the order of
// their handles.
func (sk *Sketch) LeftOutMissingGroups() []core.HGroup {
	if sk.LeftOut == nil {
		return nil
	}
	var out []core.HGroup
	for _, r := range sk.LeftOut.groupRefs() {
		if sk.lacksGroup(*r.at) && !slices.Contains(out, *r.at) {
			out = append(out, *r.at)
		}
	}
	slices.SortFunc(out, func(a, b core.HGroup) int { return int(a.V) - int(b.V) })
	return out
}

// LeftOutMissingGroupName names a group LeftOutMissingGroups gives, as
// what the load left out knew it: "g009".
func (sk *Sketch) LeftOutMissingGroupName(h core.HGroup) string { return sk.anyGroupName(h) }

// MendLeftOutGroup puts with, a group of the model, in the place of
// missing, a group the file lacks, wherever what the load left out used
// it: as the group a request or a constraint is in, or as a group's
// operand, which must come before the group; the references is none of
// them. What then has all it uses comes back into the
// model; the rest stays left out, with with in missing's place. The
// caller remembers the undo state.
func (sk *Sketch) MendLeftOutGroup(missing, with core.HGroup) error {
	if sk.LeftOut == nil {
		return ErrNothingLeftOut
	}
	if !slices.Contains(sk.LeftOutMissingGroups(), missing) {
		return ErrGroupNotMissing
	}
	w := sk.Group(with)
	if w == nil {
		return fmt.Errorf("%w: group %x isn't in the model", ErrMendKind, with.V)
	}
	l := sk.LeftOut.clone()
	var needs []string
	n := 0
	for _, r := range l.groupRefs() {
		if *r.at != missing {
			continue
		}
		need := ""
		switch {
		case with.V == core.HGroupReferences:
			need = "a group other than the references"
		case r.user != nil && (with == r.user.H || w.Order >= r.user.Order):
			need = "a group before " + r.user.DescriptionString()
		}
		if need != "" {
			if !slices.Contains(needs, need) {
				needs = append(needs, need)
			}
			continue
		}
		*r.at = with
		n++
	}
	if n == 0 {
		return fmt.Errorf("%w: what was left out uses %s as %s", ErrMendKind, sk.anyGroupName(missing), needs[0])
	}
	sk.takeMend(l)
	return nil
}
