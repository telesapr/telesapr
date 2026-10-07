package core

import (
	"math"
	"testing"
)

func TestRgbaColorFromFloatAndAccessors(t *testing.T) {
	c := RGBAFromFloat(0.5, 0.25, 1.0, 0.0)

	if c.Red != 127 || c.Green != 63 || c.Blue != 255 || c.Alpha != 0 {
		t.Fatalf("unexpected conversion: %+v", c)
	}

	if diff := math.Abs(float64(c.RedF() - float32(c.Red)/255.0)); diff > 1e-6 {
		t.Fatalf("RedF mismatch: got %f want %f", c.RedF(), float32(c.Red)/255.0)
	}
	if diff := math.Abs(float64(c.GreenF() - float32(c.Green)/255.0)); diff > 1e-6 {
		t.Fatalf("GreenF mismatch: got %f want %f", c.GreenF(), float32(c.Green)/255.0)
	}
	if diff := math.Abs(float64(c.BlueF() - float32(c.Blue)/255.0)); diff > 1e-6 {
		t.Fatalf("BlueF mismatch: got %f want %f", c.BlueF(), float32(c.Blue)/255.0)
	}
	if diff := math.Abs(float64(c.AlphaF() - float32(c.Alpha)/255.0)); diff > 1e-6 {
		t.Fatalf("AlphaF mismatch: got %f want %f", c.AlphaF(), float32(c.Alpha)/255.0)
	}
}

func TestRgbaColorPacking(t *testing.T) {
	c := NewRGBA(10, 20, 30, 40)

	if c.IsEmpty() {
		t.Fatal("expected color to be non-empty")
	}

	wantPacked := uint32(c.Red) |
		(uint32(c.Green) << 8) |
		(uint32(c.Blue) << 16) |
		(uint32(255-c.Alpha) << 24)
	if got := c.ToPackedInt(); got != wantPacked {
		t.Fatalf("ToPackedInt: got %#x want %#x", got, wantPacked)
	}

	wantARGB := uint32(c.Blue) |
		(uint32(c.Green) << 8) |
		(uint32(c.Red) << 16) |
		(uint32(c.Alpha) << 24)
	if got := c.ToARGB32(); got != wantARGB {
		t.Fatalf("ToARGB32: got %#x want %#x", got, wantARGB)
	}

	if !RGBAFromPackedInt(c.ToPackedInt()).Equals(c) {
		t.Fatal("round-trip via packed int did not preserve color")
	}
}

func TestRgbaColorEqualsAndWithAlpha(t *testing.T) {
	c := NewRGBA(5, 6, 7, 0)

	if !c.IsEmpty() {
		t.Fatal("expected IsEmpty to report true when alpha is zero")
	}

	other := c.WithAlpha(128)
	if other.Alpha != 128 {
		t.Fatalf("WithAlpha failed: got alpha %d", other.Alpha)
	}
	if other.Equals(c) {
		t.Fatal("WithAlpha should create a distinct color")
	}
	if !other.WithAlpha(128).Equals(other) {
		t.Fatal("Equals should detect identical colors")
	}
}

// TestRgbaColorFromFloat verifies float-to-byte conversion.
func TestRgbaColorFromFloat(t *testing.T) {
	c := RGBAFromFloat(0.5, 0.0, 1.0, 1.0)
	if c.Red != 127 || c.Red > 128 {
		// 255.1 * 0.5 = 127.55 → uint8 = 127
		if c.Red < 127 || c.Red > 128 {
			t.Errorf("Red = %d, want ~127", c.Red)
		}
	}
	if c.Green != 0 {
		t.Errorf("Green = %d, want 0", c.Green)
	}
	if c.Blue != 255 {
		t.Errorf("Blue = %d, want 255", c.Blue)
	}
}

// TestRgbaColorFloatAccessors verify float accessors.
func TestRgbaColorFloatAccessors(t *testing.T) {
	c := NewRGBA(255, 128, 0, 255)
	if c.RedF() != 1.0 {
		t.Errorf("RedF = %v, want 1.0", c.RedF())
	}
	if c.GreenF() < 0.5 || c.GreenF() > 0.51 {
		t.Errorf("GreenF = %v, want ~0.502", c.GreenF())
	}
	if c.BlueF() != 0.0 {
		t.Errorf("BlueF = %v, want 0.0", c.BlueF())
	}
	if c.AlphaF() != 1.0 {
		t.Errorf("AlphaF = %v, want 1.0", c.AlphaF())
	}
}

// TestRgbaColorIsEmpty verifies IsEmpty.
func TestRgbaColorIsEmpty(t *testing.T) {
	transparent := NewRGBA(0, 0, 0, 0)
	if !transparent.IsEmpty() {
		t.Errorf("transparent color should be IsEmpty")
	}
	opaque := NewRGBA(0, 0, 0, 255)
	if opaque.IsEmpty() {
		t.Errorf("opaque color should not be IsEmpty")
	}
}
