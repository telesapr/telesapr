package web

import (
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
)

// store keeps strings by key: the page's localStorage in a browser.
type store interface {
	get(key string) (string, bool)
	set(key, value string)
}

// kvSettings is a core.Settings on a store, one string for each value.
// A value that is not of the type asked for is the default, as in the
// other backends' settings.
type kvSettings struct {
	s store
}

var _ core.Settings = kvSettings{}

// settingsPrefix keeps the settings apart from whatever else the
// page's origin stores.
const settingsPrefix = "telesapr."

// Has reports whether key is stored.
func (k kvSettings) Has(key string) bool {
	_, ok := k.s.get(settingsPrefix + key)
	return ok
}

func (k kvSettings) FreezeInt(key string, value uint32) {
	k.s.set(settingsPrefix+key, strconv.FormatUint(uint64(value), 10))
}

func (k kvSettings) ThawInt(key string, defaultValue uint32) uint32 {
	if v, ok := k.s.get(settingsPrefix + key); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			return uint32(n)
		}
	}
	return defaultValue
}

func (k kvSettings) FreezeFloat(key string, value float64) {
	k.s.set(settingsPrefix+key, strconv.FormatFloat(value, 'g', -1, 64))
}

func (k kvSettings) ThawFloat(key string, defaultValue float64) float64 {
	if v, ok := k.s.get(settingsPrefix + key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func (k kvSettings) FreezeString(key, value string) { k.s.set(settingsPrefix+key, value) }

func (k kvSettings) ThawString(key, defaultValue string) string {
	if v, ok := k.s.get(settingsPrefix + key); ok {
		return v
	}
	return defaultValue
}
