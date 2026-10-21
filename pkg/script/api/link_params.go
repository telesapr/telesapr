package api

import (
	"path/filepath"
	"slices"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A link names the file it links, and may pass the linked model a
// configuration and values for its parameters, as the link group's
// screen does with its [relink], configuration and parameter links.

// File is the file the link links, as the group names it: relative to
// the drawing's file once that is saved, else absolute.
func (v LinkGroup) File() (path string) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		path = v.placed(sk, "File").LinkFile
	})
	return path
}

// SetFile points the link at another file and reads it, keeping where
// the part is placed, its scale and what the link passes into it: a
// newer revision of a part saved under another name, or a STEP file in
// place of a model. What later groups use of the old part and the new
// one lacks is removed with them, as when a linked file is missing.
func (v LinkGroup) SetFile(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		fail("%v", err)
	}
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		v.placed(sk, "SetFile")
		if err := sk.SetLinkFile(g.H, abs); err != nil {
			fail("%v", err)
		}
	})
}

// Configuration is the configuration of the linked model it is
// regenerated in, or "" for the one its file was saved in.
func (v LinkGroup) Configuration() (name string) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		name = v.placed(sk, "Configuration").LinkConfiguration
	})
	return name
}

// SetConfiguration regenerates the linked model in its configuration
// name, or with "" reads it as its file was saved. A model linked twice
// can so be two sizes. The model must have a configuration of that name.
func (v LinkGroup) SetConfiguration(name string) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		v.placed(sk, "SetConfiguration")
		if err := sk.SetLinkConfiguration(g.H, name); err != nil {
			fail("%v", err)
		}
	})
}

// Param is the expression the link passes to the linked model's
// parameter name, or "" when the model's own stands.
func (v LinkGroup) Param(name string) (expr string) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		if p := v.placed(sk, "Param").LinkParam(name); p != nil {
			expr = p.Expr
		}
	})
	return expr
}

// SetParam passes the value of expr, a number or an expression in
// this drawing's parameters, to the linked model's parameter name, and
// regenerates the model with it; when one of the drawing's parameters
// changes, the model is regenerated again. With "" the model's own
// expression stands again. The model must have a parameter of that
// name.
func (v LinkGroup) SetParam(name, expr string) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		v.placed(sk, "SetParam")
		if err := sk.SetLinkParam(g.H, name, expr); err != nil {
			fail("%v", err)
		}
	})
}

// ModelParams are the linked model's parameters and their values as
// its file, in the configuration the link chooses, has them, before
// the values the link passes; nil for a file that is not a model.
func (v LinkGroup) ModelParams() (params map[string]float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := v.placed(sk, "ModelParams")
		if !group.LinkModelRead {
			return
		}
		params = map[string]float64{}
		for _, p := range group.LinkPartParameters {
			params[p.Name] = p.Value
		}
	})
	return params
}

// ModelConfigurations are the names of the linked model's
// configurations.
func (v LinkGroup) ModelConfigurations() (names []string) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		names = slices.Clone(v.placed(sk, "ModelConfigurations").LinkConfigurations)
	})
	return names
}
