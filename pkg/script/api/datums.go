package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
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
}

// List lists the drawing's own datums, in the order they were named.
func (m DatumsMenu) List() (ds []Datum) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, dt := range sk.Datums {
			ds = append(ds, Datum{Name: dt.Name, Entity: Entity{d: d, H: dt.Entity}})
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
