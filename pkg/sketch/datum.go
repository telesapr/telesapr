package sketch

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A datum (roadmap item 2 of doc/notes/modelling_upgrades.md) is a name
// the user gives an entity a part offers to others: a point, a
// workplane, a face of the solid, or a line, normal, circle or arc as an
// axis. A part so declares its interfaces, and an assembly that links it
// finds them by name, "part.name", rather than by where they are; the
// name holds through edits of the part that keep the entity, since the
// entity's handle does. A datum is kept while its entity is gone, as
// after the request that made it is deleted, and says so where it is
// used, since the entity may come back with an undo.

// Datum is a name for an entity.
type Datum struct {
	Name   string
	Entity core.HEntity
	// Kind is what a datum on a face or an edge of the solid stands for
	// (NewDatumWorkplane), as its workplane last found it, saved so that
	// a model linking this one knows it too; DatumUnknown for a datum of
	// any other entity.
	Kind topo.DatumKind
}

// ErrNoDatum is the error for a datum name nothing answers to.
var ErrNoDatum = errors.New("no datum")

// Datum is the datum called name, or nil.
func (sk *Sketch) Datum(name string) *Datum {
	for i := range sk.Datums {
		if sk.Datums[i].Name == name {
			return &sk.Datums[i]
		}
	}
	return nil
}

// DatumOf is the datum that names entity he, or nil.
func (sk *Sketch) DatumOf(he core.HEntity) *Datum {
	for i := range sk.Datums {
		if sk.Datums[i].Entity == he {
			return &sk.Datums[i]
		}
	}
	return nil
}

// CanBeDatum reports whether an entity can be named as a datum: one a
// mate or a constraint between parts can use.
func CanBeDatum(e *core.Entity) bool {
	switch {
	case e.IsPoint(), e.IsFace(), e.IsWorkplane(), e.IsNormal():
		return true
	}
	switch e.Type {
	case core.EntityLineSegment, core.EntityCircle, core.EntityArcOfCircle:
		return true
	}
	return false
}

// SetDatum names entity he as a datum called name; an entity already
// named takes the new name, and a name already given moves to he. The
// caller remembers the undo state.
func (sk *Sketch) SetDatum(name string, he core.HEntity) error {
	name = strings.TrimSpace(name)
	if err := CheckDatumName(name); err != nil {
		return err
	}
	e := sk.Entity(he)
	if e == nil {
		return fmt.Errorf("no entity %08x", he.V)
	}
	if !CanBeDatum(e) {
		return fmt.Errorf("%s can't be a datum: name a point, a workplane, a face, or a line, normal, circle or arc as an axis", sk.EntityName(he))
	}
	kind := topo.DatumUnknown
	if g := sk.Group(e.Group); g != nil && g.Subtype == GroupWorkplaneByDatum && he == g.H.Entity(0) {
		kind = g.datumKind
	}
	if d := sk.Datum(name); d != nil {
		d.Entity, d.Kind = he, kind
	} else {
		sk.Datums = append(sk.Datums, Datum{Name: name, Entity: he, Kind: kind})
	}
	// One name for an entity: an older name of it goes.
	sk.Datums = slices.DeleteFunc(sk.Datums, func(d Datum) bool { return d.Entity == he && d.Name != name })
	return nil
}

// CheckDatumName is why name can't name a datum, or nil.
func CheckDatumName(name string) error {
	if !core.ValidVariableName(strings.TrimSpace(name)) {
		return fmt.Errorf("%q can't name a datum: use a letter or underscore, then letters, digits and underscores", name)
	}
	return nil
}

// DeleteDatum takes the datum called name away; the entity stays. The
// caller remembers the undo state.
func (sk *Sketch) DeleteDatum(name string) error {
	i := slices.IndexFunc(sk.Datums, func(d Datum) bool { return d.Name == name })
	if i < 0 {
		return fmt.Errorf("%w %q", ErrNoDatum, name)
	}
	sk.Datums = slices.Delete(sk.Datums, i, i+1)
	return nil
}

// ResolveDatum is the entity a datum name answers to: "name" for this
// sketch's own, "group.name" for a datum of the model the link group
// called group links, as the link placed it. A link copies no
// workplanes, so a datum of the linked model that names one, as a datum
// on a face or edge of its solid does (NewDatumWorkplane), answers with
// the workplane's normal, which runs through its origin.
func (sk *Sketch) ResolveDatum(path string) (core.HEntity, error) {
	// A datum's own name has no dot; a group's name may.
	group, name, linked := strings.CutLast(path, ".")
	if !linked {
		d := sk.Datum(path)
		if d == nil {
			return core.HEntity{}, fmt.Errorf("%w %q", ErrNoDatum, path)
		}
		if sk.Entity(d.Entity) == nil {
			return core.HEntity{}, fmt.Errorf("datum %s: its entity, %s, is gone", path, sk.EntityName(d.Entity))
		}
		return d.Entity, nil
	}
	g := sk.GroupNamed(group)
	if g == nil || g.Type != GroupLinked {
		return core.HEntity{}, fmt.Errorf("%w %q: no link called %s", ErrNoDatum, path, group)
	}
	i := slices.IndexFunc(g.LinkDatums, func(d Datum) bool { return d.Name == name })
	if i < 0 {
		return core.HEntity{}, fmt.Errorf("%w %q: the model %s links has no datum %s", ErrNoDatum, path, group, name)
	}
	in := g.LinkDatums[i].Entity
	if j := slices.IndexFunc(g.ImpEntity, func(e core.Entity) bool { return e.H == in }); j >= 0 && g.ImpEntity[j].IsWorkplane() {
		in = g.ImpEntity[j].Normal
	}
	id, ok := g.Remap[core.EntityKey{Input: in}]
	he := g.H.Entity(int(id.V))
	if !ok || sk.Entity(he) == nil {
		return core.HEntity{}, fmt.Errorf("datum %s: the model %s links has no such entity now", path, group)
	}
	return he, nil
}

// DatumNames are the names of every datum a ResolveDatum finds: this
// sketch's own, then each link's, as "group.name" (LinkLabel).
func (sk *Sketch) DatumNames() []string {
	var names []string
	for _, d := range sk.Datums {
		names = append(names, d.Name)
	}
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		if g.Type != GroupLinked {
			continue
		}
		label := sk.LinkLabel(g)
		for _, d := range g.LinkDatums {
			names = append(names, label+"."+d.Name)
		}
	}
	return names
}

// LinkLabel names a link group in a datum's name: by its name, or when
// another group has the same name, as a part linked twice from the
// window has, by its handle and name, "g004-tile".
func (sk *Sketch) LinkLabel(g *Group) string {
	for i := range sk.Groups {
		if o := &sk.Groups[i]; o.H != g.H && o.Name == g.Name {
			return g.DescriptionString()
		}
	}
	return g.Name
}

// lastSolidDatum is the place in GroupOrder of the last group that
// makes the workplane of a datum on a face or an edge of the solid, or
// -1: a regeneration makes the groups up to it, past the active group,
// so that the datum follows its face.
func (sk *Sketch) lastSolidDatum() int {
	last := -1
	for i, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		if g.Subtype == GroupWorkplaneByDatum && g.followsSolid() && sk.DatumOf(g.H.Entity(0)) != nil {
			last = i
		}
	}
	return last
}

// keepDatumKind gives the datum naming the workplane of g, a datum on
// the solid, the kind its frame was last found as.
func (sk *Sketch) keepDatumKind(g *Group) {
	for i := range sk.Datums {
		if sk.Datums[i].Entity == g.H.Entity(0) {
			sk.Datums[i].Kind = g.datumKind
		}
	}
}

// DatumKind is what the datum on a face or an edge of the solid that g
// makes stands for, as its frame was last found; DatumUnknown for any
// other group.
func (g *Group) DatumKind() topo.DatumKind {
	if g.Subtype != GroupWorkplaneByDatum {
		return topo.DatumUnknown
	}
	return g.datumKind
}

// DatumKindOf is what the entity he stands for as a datum on a face or
// an edge of a solid, so that a mate can refuse a plane for an axis: he
// the workplane of such a datum or its normal, or the normal a link's
// datum answers with (ResolveDatum). It is DatumUnknown for any other
// entity, which mates take as they always have.
func (sk *Sketch) DatumKindOf(he core.HEntity) topo.DatumKind {
	e := sk.Entity(he)
	if e == nil {
		return topo.DatumUnknown
	}
	g := sk.Group(e.Group)
	switch {
	case g == nil:
		return topo.DatumUnknown
	case g.Subtype == GroupWorkplaneByDatum && g.Type == GroupDrawingWorkplane:
		if wp := sk.Entity(g.H.Entity(0)); wp != nil && (he == wp.H || he == wp.Normal) {
			return g.datumKind
		}
	case g.Type == GroupLinked:
		label := sk.LinkLabel(g)
		for _, d := range g.LinkDatums {
			if d.Kind == topo.DatumUnknown {
				continue
			}
			if at, err := sk.ResolveDatum(label + "." + d.Name); err == nil && at == he {
				return d.Kind
			}
		}
	}
	return topo.DatumUnknown
}
