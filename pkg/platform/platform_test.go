package platform

import (
	"path/filepath"
	"testing"
)

func TestAcceleratorDescription(t *testing.T) {
	for _, tc := range []struct {
		ev   KeyboardEvent
		want string
	}{
		{KeyboardEvent{Key: KeyCharacter, Chr: 's', ControlDown: true}, "Ctrl+S"},
		{KeyboardEvent{Key: KeyCharacter, Chr: 's', ControlDown: true, ShiftDown: true}, "Ctrl+Shift+S"},
		{KeyboardEvent{Key: KeyFunction, Num: 3}, "F3"},
		{KeyboardEvent{Key: KeyCharacter, Chr: '\t'}, "Tab"},
		{KeyboardEvent{Key: KeyCharacter, Chr: ' '}, "Space"},
		{KeyboardEvent{Key: KeyCharacter, Chr: '\x1b'}, "Esc"},
		{KeyboardEvent{Key: KeyCharacter, Chr: '\x7f'}, "Del"},
		{KeyboardEvent{Key: KeyCharacter, Chr: '['}, "["},
	} {
		if got := AcceleratorDescription(tc.ev); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.ev, got, tc.want)
		}
	}
}

func TestJSONSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	s := OpenJSONSettings(path)
	s.FreezeInt("Color", 0xff804020)
	s.FreezeFloat("Scale", 0.1)
	s.FreezeString("RecentFile_0", "/tmp/a b.slvs")

	s2 := OpenJSONSettings(path)
	if got := s2.ThawInt("Color", 0); got != 0xff804020 {
		t.Errorf("int: %#x", got)
	}
	if got := s2.ThawFloat("Scale", 0); got != 0.1 {
		t.Errorf("float: %v", got)
	}
	if got := s2.ThawString("RecentFile_0", ""); got != "/tmp/a b.slvs" {
		t.Errorf("string: %q", got)
	}
	if got := s2.ThawInt("Missing", 7); got != 7 {
		t.Errorf("default: %v", got)
	}
	if got := s2.ThawString("Color", "x"); got != "x" {
		t.Errorf("type mismatch should give the default, got %q", got)
	}
}
