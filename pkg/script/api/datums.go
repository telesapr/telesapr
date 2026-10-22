package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// DatumsMenu holds the calls of Edit > Datums: names given to the
// entities a model offers others, a point, a workplane, a face or an
// axis, so that a part declares its interfaces and an assembly linking
// it finds them by name rather than by where they are. A script reaches
// it as doc.Datums.
type DatumsMenu struct{ d *Doc }

// Datum is a name and the entity it names, as Datums.List gives them.
type Datum struct {
	Name   string
	Entity Entity
	// Kind is what a datum on a face or an edge of a solid stands for:
	// "plane", "axis", "line", "face-point" (the plane touching a face)
	// or "edge-point" (the line along an edge); "" for a datum of any
	// other entity.
	Kind string
}

// List lists the drawing's own datums, in the order they were named.
func (m DatumsMenu) List() (ds []Datum) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, dt := range sk.Datums {
			ds = append(ds, Datum{Name: dt.Name, Entity: Entity{d: d, H: dt.Entity}, Kind: dt.Kind.String()})
		}
	})
	return ds
}

// Names lists every name Get finds: the drawing's own datums, then
// those of each linked model, as "group.name" for the link group of
// that name.
func (m DatumsMenu) Names() (names []string) {
	m.d.read(func(sk *sketch.Sketch) { names = sk.DatumNames() })
	return names
}

// Name names entity e as the datum name, a word of letters, digits and
// underscores: a point, a workplane, a normal, a face of the solid
// (SolidFace.Entity), or a line, circle or arc as an axis. An entity
// named before takes the new name; a name given before moves to e. It
// is saved with the drawing, and a drawing linking this one finds it as
// "group.name".
func (m DatumsMenu) Name(name string, e Entity) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		e.get(sk)
		if err := sk.SetDatum(name, e.H); err != nil {
			fail("%v", err)
		}
	})
}

// NameFace names face f of a solid as the datum name, a face no entity
// need make, as a fillet's or a Boolean's: a workplane that follows the
// face at each regeneration is added after the active group, and the
// datum names it. In a flat face the workplane lies as a sketch on the
// face does, a plane; on a face round about an axis, as a cylinder's, a
// cone's or a fillet's, its normal runs along the axis and its origin
// is on it; on any other face, as a loft's, a blend's or a sphere's, it
// touches the face at the face's middle, its normal out of the solid.
// Assembly.Flush takes each as a plane, a round face's square to its
// axis at its origin, and Assembly.Concentric each but a flat face's as
// an axis. The new group
// becomes active, as any new group does; while a group before it is
// active it stays where it was, and follows its face again when it is
// active or the model is saved, so a model linking this one finds it
// where the face is.
func (m DatumsMenu) NameFace(name string, f SolidFace) Group {
	return m.nameSolid(name, f.s, f.id, nil)
}

// NameEdge names edge e of a solid as the datum name, as NameFace does
// a face: the workplane follows the edge, its normal along it from its
// start where it is straight, about its axis where it is a circle or an
// arc, and along it at its middle where it is any other curve, as a
// cubic's or a helix's. Assembly.Concentric takes it as an axis;
// Assembly.Flush only a circle's, as the circle's plane.
func (m DatumsMenu) NameEdge(name string, e SolidEdge) Group {
	n := e.name
	return m.nameSolid(name, e.s, 0, &n)
}

// nameSolid adds the workplane of a datum on a face or edge of solid s,
// and names it.
func (m DatumsMenu) nameSolid(name string, s Solid, face srf.FaceID, edge *topo.Name) (g Group) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		if sk.RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		if err := sketch.CheckDatumName(name); err != nil {
			fail("%v", err)
		}
		was := sk.ActiveGroup()
		if solid := sk.Group(s.g); solid == nil || sk.Group(was).Order < solid.Order {
			fail("the datum's solid comes after the active group %s", sk.GroupName(was))
		}
		sk.Active = was
		ng, err := sk.NewDatumWorkplane(face, edge, name)
		if err != nil {
			fail("%v", err)
		}
		hg := d.h.AddGroup(ng, nil)
		sk = d.h.Sketch()
		if err := sk.SetDatum(name, hg.Entity(0)); err != nil {
			fail("%v", err)
		}
		g = Group{d, hg}
	})
	return g
}

// Delete takes the datum called name away; the entity stays.
func (m DatumsMenu) Delete(name string) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		if err := sk.DeleteDatum(name); err != nil {
			fail("%w", err)
		}
	})
}

// Get is the entity a datum names: "name" for one of the drawing's
// own, "group.name" for a datum of the model the link group called
// group links, as that link places it. Mates take it as they take any
// entity: doc.Assembly.Flush(doc.Datums.Get("base.top"),
// doc.Datums.Get("lid.bottom")).
func (m DatumsMenu) Get(name string) (e Entity) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		var he core.HEntity
		var err error
		if he, err = sk.ResolveDatum(name); err != nil {
			fail("%w", err)
		}
		e = Entity{d: d, H: he}
	})
	return e
}
