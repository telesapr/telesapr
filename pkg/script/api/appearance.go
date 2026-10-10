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
