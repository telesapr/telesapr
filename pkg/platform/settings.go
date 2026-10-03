package platform

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// JSONSettings is a core.Settings kept in a JSON object in a file, as
// the GTK build keeps its settings (C++ SettingsImplGtk). Every change
// is written through.
type JSONSettings struct {
	mu     sync.Mutex
	path   string
	values map[string]any
}

// DefaultSettingsPath is $XDG_CONFIG_HOME/telesapr/settings.json, or
// under ~/.config; empty if neither is known. It is not the C++
// application's file, so the two don't overwrite each other's.
func DefaultSettingsPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "telesapr", "settings.json")
}

// OpenJSONSettings loads the settings in path, if it exists; with an
// empty path nothing is saved.
func OpenJSONSettings(path string) *JSONSettings {
	s := &JSONSettings{path: path, values: make(map[string]any)}
	if path == "" {
		return s
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("cannot load settings: %v", err)
		}
		return s
	}
	if err := json.Unmarshal(b, &s.values); err != nil {
		log.Printf("cannot load settings: %v", err)
		s.values = make(map[string]any)
	}
	return s
}

func (s *JSONSettings) set(key string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = v
	s.saveLocked()
}

func (s *JSONSettings) get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[key]
	return v, ok
}

// saveLocked writes the settings atomically: to a temporary file,
// renamed over the old one.
func (s *JSONSettings) saveLocked() {
	if s.path == "" {
		return
	}
	// Map keys are written sorted.
	b, err := json.MarshalIndent(s.values, "", "  ")
	if err != nil {
		log.Printf("cannot save settings: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o777); err != nil {
		log.Printf("cannot save settings: %v", err)
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o666); err != nil {
		log.Printf("cannot save settings: %v", err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		log.Printf("cannot save settings: %v", err)
	}
}

// Has reports whether key is stored.
func (s *JSONSettings) Has(key string) bool {
	_, ok := s.get(key)
	return ok
}

// FreezeInt stores an integer.
func (s *JSONSettings) FreezeInt(key string, value uint32) { s.set(key, float64(value)) }

// ThawInt reads an integer, or defaultValue.
func (s *JSONSettings) ThawInt(key string, defaultValue uint32) uint32 {
	if v, ok := s.get(key); ok {
		switch n := v.(type) {
		case float64:
			return uint32(int64(n))
		case bool:
			if n {
				return 1
			}
			return 0
		}
	}
	return defaultValue
}

// FreezeFloat stores a number.
func (s *JSONSettings) FreezeFloat(key string, value float64) { s.set(key, value) }

// ThawFloat reads a number, or defaultValue.
func (s *JSONSettings) ThawFloat(key string, defaultValue float64) float64 {
	if v, ok := s.get(key); ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return defaultValue
}

// FreezeString stores a string.
func (s *JSONSettings) FreezeString(key, value string) { s.set(key, value) }

// ThawString reads a string, or defaultValue.
func (s *JSONSettings) ThawString(key, defaultValue string) string {
	if v, ok := s.get(key); ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return defaultValue
}

// MemorySettings is a core.Settings kept in memory, for tests.
type MemorySettings struct {
	Values map[string]any
}

// NewMemorySettings returns empty settings.
func NewMemorySettings() *MemorySettings { return &MemorySettings{Values: make(map[string]any)} }

// Has reports whether key is stored.
func (m *MemorySettings) Has(key string) bool {
	_, ok := m.Values[key]
	return ok
}

func (m *MemorySettings) FreezeInt(key string, value uint32) { m.Values[key] = value }
func (m *MemorySettings) ThawInt(key string, def uint32) uint32 {
	if v, ok := m.Values[key].(uint32); ok {
		return v
	}
	return def
}
func (m *MemorySettings) FreezeFloat(key string, value float64) { m.Values[key] = value }
func (m *MemorySettings) ThawFloat(key string, def float64) float64 {
	if v, ok := m.Values[key].(float64); ok {
		return v
	}
	return def
}
func (m *MemorySettings) FreezeString(key, value string) { m.Values[key] = value }
func (m *MemorySettings) ThawString(key, def string) string {
	if v, ok := m.Values[key].(string); ok {
		return v
	}
	return def
}
