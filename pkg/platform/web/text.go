package web

// commonPrefix is the longest prefix two strings share, cut at a rune
// boundary.
func commonPrefix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	for i > 0 && i < len(a) && a[i]&0xC0 == 0x80 {
		i--
	}
	return a[:i]
}

// utf16ToByteOffset converts an offset in UTF-16 code units, how the
// browser counts in a text box, to one in bytes of the UTF-8 string.
func utf16ToByteOffset(s string, units int) int {
	n := 0
	for i, r := range s {
		if n >= units {
			return i
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return len(s)
}

// byteToUTF16Offset is the inverse of utf16ToByteOffset.
func byteToUTF16Offset(s string, offset int) int {
	n := 0
	for i, r := range s {
		if i >= offset {
			return n
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
