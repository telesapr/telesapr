package sketch

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// A link group may pass values into the model it links (roadmap item 3
// of doc/notes/modelling_upgrades.md): a configuration of the model to
// regenerate it in, and values for some of its parameters. The linked
// model is then regenerated with them rather than read as it was saved,
// so one part file placed twice can be two sizes. A value is an
// expression in this sketch's parameters, so an assembly can drive its
// parts from its own table; it is evaluated with the table and handed
// to the part as a plain number, which the part's parameter takes as
// it would a number typed into it.

// LinkParam is a value a link group passes to a parameter of the model
// it links.
type LinkParam struct {
	// Name is the linked model's parameter.
	Name string
	// Expr is an expression in this sketch's parameters, and Value its
	// value at the last evaluation.
	Expr  string
	Value float64
}

// LinkParamField is the name in messages of the value a link passes to
// the linked model's parameter name.
func LinkParamField(name string) string { return "link-param " + name }

// IsParametricLink reports whether the group passes a configuration or
// parameter values into the model it links.
func (g *Group) IsParametricLink() bool {
	return g.Type == GroupLinked && (g.LinkConfiguration != "" || len(g.LinkParams) > 0)
}

// LinkOverridesKey names what the group passes into the model it links,
// "" for nothing: the same key for the same configuration and values.
// Whoever loads the links records it in LinkLoaded.
func (g *Group) LinkOverridesKey() string {
	if !g.IsParametricLink() {
		return ""
	}
	var b strings.Builder
	b.WriteString("configuration=")
	b.WriteString(strconv.Quote(g.LinkConfiguration))
	ps := slices.Clone(g.LinkParams)
	slices.SortFunc(ps, func(a, b LinkParam) int { return strings.Compare(a.Name, b.Name) })
	for _, p := range ps {
		fmt.Fprintf(&b, ";%s=%s", p.Name, strconv.FormatFloat(p.Value, 'g', -1, 64))
	}
	return b.String()
}

// LinkParam is the value the group passes to the linked model's
// parameter name, or nil.
func (g *Group) LinkParam(name string) *LinkParam {
	for i := range g.LinkParams {
		if g.LinkParams[i].Name == name {
			return &g.LinkParams[i]
		}
	}
	return nil
}

// linkGroup is the link group hg, or an error saying why there is none.
func (sk *Sketch) linkGroup(hg core.HGroup) (*Group, error) {
	g := sk.Group(hg)
	if g == nil {
		return nil, fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupLinked {
		return nil, fmt.Errorf("group %s is not a link", g.Name)
	}
	return g, nil
}

// reloadLink reads the links again after hg's changed, and regenerates
// from hg.
func (sk *Sketch) reloadLink(hg core.HGroup) {
	if sk.ReloadAllLinked != nil {
		sk.ReloadAllLinked(sk)
	}
	sk.MarkGroupDirty(hg, false)
}

// SetLinkFile points the link group hg at another file, named as the
// group names its file (relative to the sketch's file, or absolute),
// and reads it, keeping the group's placement, scale and what it passes
// into the model. What later groups refer to in the old file and the new
// one lacks is removed by the regeneration, as for a missing file. The
// caller remembers the undo state.
func (sk *Sketch) SetLinkFile(hg core.HGroup, file string) error {
	g, err := sk.linkGroup(hg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(file) == "" {
		return errors.New("a link needs a file")
	}
	if g.LinkFile == file {
		return nil
	}
	g.LinkFile = file
	sk.reloadLink(hg)
	return nil
}

// SetLinkConfiguration has the link group hg regenerate the model it
// links in the model's configuration name, or with "" in the one it was
// saved in. A name the model has no configuration of is refused when
// the model has been read. The caller remembers the undo state.
func (sk *Sketch) SetLinkConfiguration(hg core.HGroup, name string) error {
	g, err := sk.linkGroup(hg)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name != "" && g.LinkModelRead && !slices.Contains(g.LinkConfigurations, name) {
		return fmt.Errorf("the linked model has no configuration %q", name)
	}
	if g.LinkConfiguration == name {
		return nil
	}
	g.LinkConfiguration = name
	sk.reloadLink(hg)
	return nil
}

// SetLinkParam has the link group hg pass the value of expr, an
// expression in this sketch's parameters, to the linked model's
// parameter name; with expr "" the model's own expression stands again.
// A name the model has no parameter of is refused when the model has
// been read. The caller remembers the undo state.
func (sk *Sketch) SetLinkParam(hg core.HGroup, name, expr string) error {
	g, err := sk.linkGroup(hg)
	if err != nil {
		return err
	}
	expr = cleanText(expr)
	if expr == "" {
		i := slices.IndexFunc(g.LinkParams, func(p LinkParam) bool { return p.Name == name })
		if i < 0 {
			return nil
		}
		g.LinkParams = slices.Delete(g.LinkParams, i, i+1)
		sk.reloadLink(hg)
		return nil
	}
	if g.LinkModelRead {
		if !slices.ContainsFunc(g.LinkPartParameters, func(p Parameter) bool { return p.Name == name }) {
			return fmt.Errorf("the linked model has no parameter %q", name)
		}
	} else if !core.ValidVariableName(name) {
		return fmt.Errorf("%q can't name a parameter", name)
	}
	if _, _, err := sk.parseNamed(expr); err != nil {
		return err
	}
	v, err := sk.evaluateLinkExpr(expr)
	if err != nil {
		return err
	}
	if p := g.LinkParam(name); p != nil {
		p.Expr, p.Value = expr, v
	} else {
		g.LinkParams = append(g.LinkParams, LinkParam{Name: name, Expr: expr, Value: v})
	}
	sk.reloadLink(hg)
	return nil
}

// evaluateLinkExpr is the value of an expression in the parameter
// table, as the next evaluation will give it.
func (sk *Sketch) evaluateLinkExpr(text string) (float64, error) {
	vars, _ := sk.evaluateTable()
	return evalNamed(text, vars)
}

// reloadChangedLinks reads the links again when a link group passes
// other values than its model was read with: a parameter its values
// name changed.
func (sk *Sketch) reloadChangedLinks() {
	if sk.ReloadAllLinked == nil {
		return
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.Type == GroupLinked && g.LinkOverridesKey() != g.LinkLoaded {
			sk.ReloadAllLinked(sk)
			return
		}
	}
}
