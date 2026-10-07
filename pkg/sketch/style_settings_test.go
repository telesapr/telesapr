package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// mapSettings keeps settings in memory.
type mapSettings struct {
	ints    map[string]uint32
	floats  map[string]float64
	strings map[string]string
}

func newMapSettings() *mapSettings {
	return &mapSettings{map[string]uint32{}, map[string]float64{}, map[string]string{}}
}

func (m *mapSettings) FreezeInt(k string, v uint32) { m.ints[k] = v }
func (m *mapSettings) ThawInt(k string, d uint32) uint32 {
	if v, ok := m.ints[k]; ok {
		return v
	}
	return d
}
func (m *mapSettings) FreezeFloat(k string, v float64) { m.floats[k] = v }
func (m *mapSettings) ThawFloat(k string, d float64) float64 {
	if v, ok := m.floats[k]; ok {
		return v
	}
	return d
}
func (m *mapSettings) FreezeString(k, v string) { m.strings[k] = v }
func (m *mapSettings) ThawString(k, d string) string {
	if v, ok := m.strings[k]; ok {
		return v
	}
	return d
}

// TestDefaultStylesFromSettings checks that default styles come from the
// user's settings, that FreezeDefaultStyles saves them, and that
// LoadFactoryDefaults restores the factory settings.
func TestDefaultStylesFromSettings(t *testing.T) {
	st := newMapSettings()
	core.SetSettings(st)
	defer core.SetSettings(nil)

	red := core.NewRGBA(255, 0, 0, 255)
	core.FreezeColor(st, core.CnfColor("ActiveGrp"), red)
	st.FreezeFloat(core.CnfWidth("ActiveGrp"), 2.5)
	st.FreezeString(core.CnfStippleType("ActiveGrp"), "dashdot")
	core.FreezeBool(st, core.CnfExportable("ActiveGrp"), false)

	sk := NewSketch()
	s := sk.StyleGet(core.HStyle{V: core.StyleActiveGrp})
	if !s.Color.Equals(red) || s.Width != 2.5 || s.StippleType != core.StippleDashDot || s.Exportable {
		t.Fatalf("style from settings: %+v", *s)
	}
	// A custom style starts as the active group's.
	if c := sk.StyleGet(core.HStyle{V: core.StyleFirstCustom}); c.Width != 2.5 || c.Name != "new-custom-style" {
		t.Errorf("custom style: %+v", *c)
	}

	saved := newMapSettings()
	sk.FreezeDefaultStyles(saved)
	if saved.floats[core.CnfWidth("ActiveGrp")] != 2.5 || saved.strings[core.CnfStippleType("ActiveGrp")] != "DashDot" ||
		saved.ints[core.CnfExportable("ActiveGrp")] != 0 || saved.floats[core.CnfTextHeight("Datum")] != 11.5 {
		t.Errorf("frozen settings: %v %v %v", saved.floats, saved.strings, saved.ints)
	}

	sk.LoadFactoryDefaults()
	s = sk.StyleGet(core.HStyle{V: core.StyleActiveGrp})
	d := core.StyleDefaults[0]
	if !s.Color.Equals(d.Color) || s.Width != d.Width || s.StippleType != d.StippleType || !s.Exportable {
		t.Errorf("factory style: %+v", *s)
	}
}
