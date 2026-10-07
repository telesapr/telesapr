package api

import (
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Appearances are the appearances a solid or a face can have, how it
// takes light when the window shades (roadmap 19.2): matte, plastic,
// glossy and metal.
func Appearances() []string { return append([]string(nil), render.MaterialNames...) }

func checkAppearance(name string) {
	if _, ok := render.Materials[name]; !ok && name != "" {
		fail("no appearance %q: they are %v", name, render.MaterialNames)
	}
}

// Appearance is the group's solid's appearance, "" for the default,
// plastic.
func (g Group) Appearance() (name string) {
	g.d.read(func(sk *sketch.Sketch) { name = g.get(sk).Appearance })
	return name
}

// SetAppearance gives the group's solid an appearance: matte, plastic,
// glossy or metal, or "" for the default. Its colour stays.
func (g Group) SetAppearance(name string) {
	checkAppearance(name)
	g.d.change(func(sk *sketch.Sketch) { g.get(sk).Appearance = name })
}

// SetFaceAppearance gives one face of the solid its own appearance, or
// with "" takes it away, so that the face has its group's. The face is
// kept by its identity, in the group that made it.
func (d *Doc) SetFaceAppearance(face SolidFace, name string) {
	checkAppearance(name)
	d.change(func(sk *sketch.Sketch) {
		g := sk.Group(sketch.FaceGroup(face.id))
		if g == nil {
			fail("no group made face %x", uint32(face.id))
		}
		kept := g.FaceAppearances[:0]
		for _, fa := range g.FaceAppearances {
			if fa.Face != face.id {
				kept = append(kept, fa)
			}
		}
		g.FaceAppearances = kept
		if name != "" {
			g.FaceAppearances = append(g.FaceAppearances, sketch.FaceAppearance{Face: face.id, Appearance: name})
		}
	})
}

// FaceAppearance is the appearance the face has: its own, its group's,
// or "" for the default.
func (d *Doc) FaceAppearance(face SolidFace) (name string) {
	d.read(func(sk *sketch.Sketch) { name = sk.AppearanceOf(face.id) })
	return name
}
