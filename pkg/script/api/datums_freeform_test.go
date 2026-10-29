package api

import (
	"math"
	"path/filepath"
	"testing"
)

// Datums on a lens's curved wall and rim, neither flat, round nor
// straight: the wall gives the plane touching it at its middle, the
// rim the line along it there. Each keeps its kind through the file and
// a link, and mates refuse a flat face's plane as an axis and an edge's
// line as a plane, here and through links.
func TestDatumsOnFreeFormFaces(t *testing.T) {
	t.Parallel()
	d := New()
	g := lens(t, d)
	s := g.Solid()
	d.Datums.NameFace("wall", s.FaceAt(V(10, 6, 2.5)))
	d.Datums.NameEdge("rim", s.EdgeAt(V(10, 6, 5)))
	d.Datums.NameFace("top", s.FaceAt(V(10, 0, 5)))
	if at, n := frameOf(d.Datums.Get("wall")); at.Minus(V(10, 6, 2.5)).Magnitude() > 1e-6 || math.Abs(n.Y-1) > 1e-6 {
		t.Errorf("the wall's plane at %v, its normal %v", at, n)
	}
	if at, n := frameOf(d.Datums.Get("rim")); at.Minus(V(10, 6, 5)).Magnitude() > 1e-6 || math.Abs(math.Abs(n.X)-1) > 1e-6 {
		t.Errorf("the rim's line at %v along %v", at, n)
	}
	kinds := func(d *Doc) map[string]string {
		m := map[string]string{}
		for _, dt := range d.Datums.List() {
			m[dt.Name] = dt.Kind
		}
		return m
	}
	want := map[string]string{"wall": "face-point", "rim": "edge-point", "top": "plane"}
	for name, k := range want {
		if got := kinds(d)[name]; got != k {
			t.Errorf("%s is a %q, want %q", name, got, k)
		}
	}
	failsWith(t, "flat face", func() { d.Assembly.Concentric(d.Datums.Get("top"), d.Datums.Get("wall")) })
	failsWith(t, "datum of an edge", func() { d.Assembly.Flush(d.Datums.Get("wall"), d.Datums.Get("rim")) })

	path := filepath.Join(t.TempDir(), "lens.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	back := New()
	if err := back.File.Load(path); err != nil {
		t.Fatal(err)
	}
	for name, k := range want {
		if got := kinds(back)[name]; got != k {
			t.Errorf("loaded, %s is a %q, want %q", name, got, k)
		}
	}

	asm := New()
	if _, err := asm.Assembly.Link(path, "a"); err != nil {
		t.Fatal(err)
	}
	b, err := asm.Assembly.Link(path, "b")
	if err != nil {
		t.Fatal(err)
	}
	b.AsLink().Move(V(0, 40, 0))
	failsWith(t, "flat face", func() { asm.Assembly.Concentric(asm.Datums.Get("a.top"), asm.Datums.Get("b.top")) })
	failsWith(t, "datum of an edge", func() { asm.Assembly.Flush(asm.Datums.Get("a.rim"), asm.Datums.Get("b.rim")) })
	asm.Assembly.Flush(asm.Datums.Get("a.wall"), asm.Datums.Get("b.wall"))
	pa, na := frameOf(asm.Datums.Get("a.wall"))
	pb, nb := frameOf(asm.Datums.Get("b.wall"))
	if math.Abs(pb.Minus(pa).Dot(na)) > 1e-6 || math.Abs(math.Abs(na.Dot(nb))-1) > 1e-9 {
		t.Errorf("the walls' planes at %v along %v and %v along %v", pa, na, pb, nb)
	}
}

// A datum on the solid in a group after the active one follows its face
// while groups before it change, though the groups after the active one
// stay hidden: a mate finds it where the face is, and so does a model
// linking the saved file.
func TestDatumAfterActiveFollows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := saveRoundedPlate(t, dir)
	p := New()
	if err := p.File.Load(path); err != nil {
		t.Fatal(err)
	}
	p.Activate(p.Group("plate"))
	if err := p.Params.Set("w", "50"); err != nil {
		t.Fatal(err)
	}
	if at, _ := frameOf(p.Datums.Get("rounded")); math.Abs(at.X-46) > 1e-6 {
		t.Errorf("after the active group, the axis is at %v", at)
	}
	if active := p.Active(); active.H != p.Group("plate").H {
		t.Errorf("following the datum made %s active", active)
	}
	wider := filepath.Join(dir, "wider.tlspr")
	if err := p.File.Save(wider); err != nil {
		t.Fatal(err)
	}
	if at, _ := frameOf(p.Datums.Get("rounded")); math.Abs(at.X-46) > 1e-6 {
		t.Errorf("saved, the axis is at %v", at)
	}
	// Through links, the axis of the wider plate is 10 further along x
	// than the first's.
	asm := New()
	for name, file := range map[string]string{"first": path, "wider": wider} {
		if _, err := asm.Assembly.Link(file, name); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := frameOf(asm.Datums.Get("first.rounded"))
	if at, _ := frameOf(asm.Datums.Get("wider.rounded")); at.Minus(first).Minus(V(10, 0, 0)).Magnitude() > 1e-6 {
		t.Errorf("through links, the axes at %v and %v", first, at)
	}
}
