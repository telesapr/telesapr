package core

// Settings is the user's persistent settings (C++ Platform::Settings).
// Floats are stored as the caller rounds them; C++ stores several as
// float.
type Settings interface {
	FreezeInt(key string, value uint32)
	ThawInt(key string, defaultValue uint32) uint32

	FreezeFloat(key string, value float64)
	ThawFloat(key string, defaultValue float64) float64

	FreezeString(key, value string)
	ThawString(key, defaultValue string) string
}

// NoSettings stores nothing and returns the defaults, as the headless
// C++ build's settings do (SettingsImplDummy).
type NoSettings struct{}

func (NoSettings) FreezeInt(string, uint32)                         {}
func (NoSettings) ThawInt(_ string, defaultValue uint32) uint32     { return defaultValue }
func (NoSettings) FreezeFloat(string, float64)                      {}
func (NoSettings) ThawFloat(_ string, defaultValue float64) float64 { return defaultValue }
func (NoSettings) FreezeString(string, string)                      {}
func (NoSettings) ThawString(_, defaultValue string) string         { return defaultValue }

var settings Settings = NoSettings{}

// GetSettings is the settings store (C++ Platform::GetSettings).
func GetSettings() Settings { return settings }

// SetSettings makes s the settings store; nil restores NoSettings.
func SetSettings(s Settings) {
	if s == nil {
		s = NoSettings{}
	}
	settings = s
}

// FreezeBool, ThawBool, FreezeColor and ThawColor store through the
// integer settings (C++ Settings::FreezeBool and so on).
func FreezeBool(s Settings, key string, value bool) {
	v := uint32(0)
	if value {
		v = 1
	}
	s.FreezeInt(key, v)
}

func ThawBool(s Settings, key string, defaultValue bool) bool {
	v := uint32(0)
	if defaultValue {
		v = 1
	}
	return s.ThawInt(key, v) != 0
}

func FreezeColor(s Settings, key string, value RgbaColor) {
	s.FreezeInt(key, value.ToPackedInt())
}

func ThawColor(s Settings, key string, defaultValue RgbaColor) RgbaColor {
	return RgbaColorFromPackedInt(s.ThawInt(key, defaultValue.ToPackedInt()))
}

// The settings keys of a default style (C++ Style::CnfColor and so on).
func CnfColor(prefix string) string        { return "Style_" + prefix + "_Color" }
func CnfWidth(prefix string) string        { return "Style_" + prefix + "_Width" }
func CnfStippleType(prefix string) string  { return "Style_" + prefix + "_StippleType" }
func CnfStippleScale(prefix string) string { return "Style_" + prefix + "_StippleScale" }
func CnfTextHeight(prefix string) string   { return "Style_" + prefix + "_TextHeight" }
func CnfExportable(prefix string) string   { return "Style_" + prefix + "_Exportable" }
