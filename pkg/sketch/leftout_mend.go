package sketch

import (
	"errors"
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file mends what the load of a damaged file left out from the
// window or a script, rather than in the file's text: what it used and
// the file lacks is replaced by something the model holds, as the user
// picks it, and what then has all it uses comes back into the model.

// ErrNotMissing is MendLeftOut's error when nothing left out uses the
// entity it is to replace.
var ErrNotMissing = errors.New("nothing left out uses that entity")

// ErrMendKind is MendLeftOut's error when the entity picked is not of
// the kind what was left out used: a point for a point, a workplane for
// a workplane, a curve for an edge.
var ErrMendKind = errors.New("the entity is not of the kind needed")

// mendRef is a reference of a left-out item that a mend may replace.
type mendRef struct {
	at   *core.HEntity
	role mendRole
}

// mendRefs are the references of what was left out that a mend may
// replace: a group's origin, axis, direction or plane, a request's
// workplane, a constraint's points, workplane and entities, and the
// edges dropped from a group's list.
func (l *LeftOut) mendRefs() []mendRef {
	var out []mendRef
	for i := range l.Groups {
		g := &l.Groups[i]
		b, c := g.predefRoles()
		out = append(out, mendRef{&g.Predef.Origin, mendPoint},
			mendRef{&g.Predef.EntityB, b}, mendRef{&g.Predef.EntityC, c})
	}
	for i := range l.Requests {
		out = append(out, mendRef{&l.Requests[i].Workplane, mendWorkplane})
	}
	for i := range l.Constraints {
		c := &l.Constraints[i]
		out = append(out, mendRef{&c.PtA, mendPoint}, mendRef{&c.PtB, mendPoint},
			mendRef{&c.Workplane, mendWorkplane})
		roles := constraintRoles(c.Type)
		for k, at := range [...]*core.HEntity{&c.EntityA, &c.EntityB, &c.EntityC, &c.EntityD} {
			out = append(out, mendRef{at, roles[k]})
		}
	}
	for i := range l.Edges {
		e := &l.Edges[i]
		for k := range e.Edges {
			if !slices.Contains(e.Kept, e.Edges[k]) {
				out = append(out, mendRef{&e.Edges[k], mendCurve})
			}
		}
	}
	return out
}

// lacks reports whether he is an entity that nothing in the model or
// left out of it makes: no entity at all, nor one of a left-out request
// or group, which come back with them.
func (sk *Sketch) lacks(he core.HEntity) bool {
	if he == core.NoEntity || he == core.FreeIn3D || sk.Entity(he) != nil {
		return false
	}
	l := sk.LeftOut
	if he.IsFromRequest() {
		return !slices.ContainsFunc(l.Requests, func(r Request) bool { return r.H == he.Request() })
	}
	return !slices.ContainsFunc(l.Groups, func(g Group) bool { return g.H == he.Group() })
}

// LeftOutMissing are the entities that what the load of a damaged file
// left out uses, the model doesn't hold, and a mend (MendLeftOut) may
// replace, in the order of their handles.
func (sk *Sketch) LeftOutMissing() []core.HEntity {
	if sk.LeftOut == nil {
		return nil
	}
	var out []core.HEntity
	for _, r := range sk.LeftOut.mendRefs() {
		if sk.lacks(*r.at) && !slices.Contains(out, *r.at) {
			out = append(out, *r.at)
		}
	}
	slices.SortFunc(out, func(a, b core.HEntity) int { return int(a.V) - int(b.V) })
	return out
}

// LeftOutMissingName names an entity LeftOutMissing gives, as what
// the load left out knew it: "r007" or "point 1 of r007".
func (sk *Sketch) LeftOutMissingName(he core.HEntity) string { return sk.entityName(he) }

// MendLeftOut puts with in the place of missing, an entity the file
// lacks, wherever what the load left out used it in a role with fits:
// a group's origin or a constraint's point takes a point, a request's
// or a constraint's workplane a workplane, a dropped edge a curve, a
// group's axis, direction or plane what its kind of group uses (a line
// or a normal for a lathe's axis, a workplane or a planar face for a
// mirror's plane), a constraint's entity what its kind of constraint
// uses there (a line for a point-to-line distance, a circle or an arc
// for a diameter, a face for a point on a face; LeftOutMissingKind
// says which).
// What then has all it uses comes back into the model; the rest stays
// left out, with with in missing's place. The caller remembers the
// undo state.
func (sk *Sketch) MendLeftOut(missing, with core.HEntity) error {
	if sk.LeftOut == nil {
		return ErrNothingLeftOut
	}
	if !slices.Contains(sk.LeftOutMissing(), missing) {
		return ErrNotMissing
	}
	e := sk.Entity(with)
	if e == nil {
		return fmt.Errorf("%w: entity %x isn't in the model", ErrMendKind, with.V)
	}
	// What is mended is a copy, so that a refusal leaves LeftOut, which
	// the undo stack shares, as it was.
	l := sk.LeftOut.clone()
	var needs []string
	n := 0
	for _, r := range l.mendRefs() {
		if *r.at != missing {
			continue
		}
		if !r.role.fits(e) {
			if !slices.Contains(needs, r.role.String()) {
				needs = append(needs, r.role.String())
			}
			continue
		}
		*r.at = with
		n++
	}
	if n == 0 {
		return fmt.Errorf("%w: what was left out uses %s as %s", ErrMendKind, sk.entityName(missing), needs[0])
	}
	sk.takeMend(l)
	return nil
}

// takeMend makes l, a mended copy of what was left out, what is left
// out, and brings back what then has all it uses.
func (sk *Sketch) takeMend(l *LeftOut) {
	sk.LeftOut = l
	sk.bringBackLeftOut()
	for i := range sk.Groups {
		sk.Groups[i].Clean = false
	}
	sk.Unsaved = true
	sk.ScheduleGenerateAll()
}

// clone copies l deep enough for a mend to change its references.
func (l *LeftOut) clone() *LeftOut {
	c := &LeftOut{
		Groups:      make([]Group, len(l.Groups)),
		Requests:    slices.Clone(l.Requests),
		Constraints: slices.Clone(l.Constraints),
		Params:      slices.Clone(l.Params),
		Edges:       slices.Clone(l.Edges),
	}
	for i := range l.Groups {
		c.Groups[i] = l.Groups[i].defining()
		c.Groups[i].LoftThrough = slices.Clone(c.Groups[i].LoftThrough)
	}
	for i := range c.Edges {
		c.Edges[i].Edges = slices.Clone(c.Edges[i].Edges)
	}
	return c
}

// bringBackLeftOut puts what was left out back into the model and
// regenerates it, as a load does; what still lacks something is left
// out again.
func (sk *Sketch) bringBackLeftOut() {
	l := sk.LeftOut
	for i := range sk.Groups {
		if g := &sk.Groups[i]; g != sk.AsSaved(g) {
			*g = *sk.AsSaved(g)
		}
	}
	sk.LeftOut = nil
	sk.Groups = append(sk.Groups, l.Groups...)
	sk.Requests = append(sk.Requests, l.Requests...)
	sk.Constraints = append(sk.Constraints, l.Constraints...)
	sk.Params = append(sk.Params, l.Params...)
	was := sk.loaded()
	sk.GenerateAllTyped(GenerateRegen)
	sk.keepLeftOut(was)
}
