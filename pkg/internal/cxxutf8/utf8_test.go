package cxxutf8

import (
	"slices"
	"testing"
)

// TestDecode decodes one, two, three and four byte sequences, and a
// sequence cut short, whose missing byte reads as zero and ends it.
func TestDecode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want []uint32
	}{
		{"", nil},
		{"ab", []uint32{'a', 'b'}},
		{"é", []uint32{0xe9}},
		{"€", []uint32{0x20ac}},
		{"😀", []uint32{0x1f600}},
		{"a€b", []uint32{'a', 0x20ac, 'b'}},
		{"\xe2\x82", []uint32{0x2000}},
	} {
		if got := Decode(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("Decode(%q) = %x, want %x", tc.in, got, tc.want)
		}
	}
}
