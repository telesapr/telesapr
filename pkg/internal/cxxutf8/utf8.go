// Package cxxutf8 decodes UTF-8 as SolveSpace's C++ did, which the
// fonts and the text drawing keep to.
package cxxutf8

// Decode decodes str as C++'s utf8_iterator does, which doesn't check
// the encoding; the byte after the string reads as 0, like a C
// string's terminator.
func Decode(str string) []uint32 {
	at := func(i int) uint32 {
		if i < len(str) {
			return uint32(str[i])
		}
		return 0
	}
	var out []uint32
	for p := 0; p < len(str); {
		it := p
		result := at(it)
		if result&0x80 != 0 {
			mask := uint32(0x40)
			for {
				result <<= 6
				it++
				c := at(it)
				mask <<= 5
				result += c - 0x80
				if result&mask == 0 {
					break
				}
			}
			result &= mask - 1
		}
		out = append(out, result)
		p = it + 1
	}
	return out
}
