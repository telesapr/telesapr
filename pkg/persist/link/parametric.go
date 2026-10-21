package link

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A link that passes a configuration or parameter values into the model
// it links (sketch.Group.LinkOverridesKey) reads the model's sketch,
// gives it those values, and regenerates it, rather than reading the
// solid the model's file was saved with. The model's own links are read
// the same way, without asking for a missing one; a model that links
// itself, through any number of links, is not regenerated again inside
// itself.

// regenerating is the loader of the model g links, regenerated with
// what g passes into it.
func regenerating(g *sketch.Group, ctx *loadCtx) loadFunc {
	cfg := g.LinkConfiguration
	params := slices.Clone(g.LinkParams)
	return func(path string, sk *sketch.Sketch, m *srf.SMesh, sh *srf.SShell, rep *Report, deps *[]stamp) ([]core.Entity, bool) {
		read := readerFor(path)
		if read == nil {
			return nil, false
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if ctx.open[abs] || len(ctx.open) >= maxPartsDepth {
			return nil, false
		}
		part, err := read(path)
		if err != nil || part == nil {
			return nil, false
		}
		ents, problems, ok := regenerate(part, path, sk, cfg, params, rep, &loadCtx{open: ctx.open, deps: deps}, abs)
		if !ok {
			return nil, false
		}
		for _, st := range part.Styles {
			if sk.Style(st.H) == nil {
				sk.Styles = append(sk.Styles, st)
			}
		}
		if len(part.GroupOrder) > 0 {
			if last := part.Group(part.GroupOrder[len(part.GroupOrder)-1]); last != nil {
				m.Triangles = append(m.Triangles, last.RunningMesh.Triangles...)
				sh.Surface = append(sh.Surface, last.RunningShell.Surface...)
				sh.Curve = append(sh.Curve, last.RunningShell.Curve...)
			}
		}
		rep.Problem = strings.Join(problems, "; ")
		return ents, true
	}
}

// regenerate gives part, the sketch of the model at path, the
// configuration cfg and the values params, reads its links and
// regenerates it as sk, the sketch linking it, would: to sk's chord
// tolerance and with its fonts. It returns the part's entities as a
// saved file holds them, and what of the values could not be given.
func regenerate(part *sketch.Sketch, path string, sk *sketch.Sketch, cfg string, params []sketch.LinkParam,
	rep *Report, ctx *loadCtx, abs string) (ents []core.Entity, problems []string, ok bool) {
	rep.Model = true
	for _, c := range part.Configurations {
		rep.Configurations = append(rep.Configurations, c.Name)
	}
	if cfg != "" {
		if err := part.ActivateConfiguration(cfg); err != nil {
			problems = append(problems, err.Error())
		}
	}
	rep.Parameters = slices.Clone(part.Parameters)
	rep.Datums = slices.Clone(part.Datums)
	for _, p := range params {
		if part.Parameter(p.Name) == nil {
			problems = append(problems, fmt.Sprintf("the model has no parameter %q", p.Name))
			continue
		}
		if err := part.SetParameterExpr(p.Name, strconv.FormatFloat(p.Value, 'g', -1, 64)); err != nil {
			problems = append(problems, fmt.Sprintf("parameter %s: %v", p.Name, err))
		}
	}

	part.Entities = part.Entities[:0]
	part.ChordTolPct, part.MaxSegments, part.Mesher = sk.ChordTolPct, sk.MaxSegments, sk.Mesher
	part.Fonts, part.Settings = sk.Fonts, sk.Settings
	ctx.open[abs] = true
	defer delete(ctx.open, abs)
	if !reloadAll(part, path, Options{Quiet: true}, ctx) {
		return nil, nil, false
	}
	defer func() {
		// A model that breaks regeneration is not a model this link can
		// use.
		if r := recover(); r != nil {
			ents, problems, ok = nil, nil, false
		}
	}()
	part.UpgradeLegacyData()
	part.GenerateAll()
	for i := range part.Entities {
		e := &part.Entities[i]
		func() {
			defer func() { _ = recover() }()
			e.CalculateNumerical(part, true)
		}()
	}
	return append([]core.Entity(nil), part.Entities...), problems, true
}

// joinProblems is two descriptions of what could not be read, as one.
func joinProblems(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}
