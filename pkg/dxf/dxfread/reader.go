// Package dxfread reads DXF files as the part of libdxfrw that
// SolveSpace's DXF import uses does, quirks included.
package dxfread

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"strings"
)

// valueType is what kind of value a record holds.
type valueType int

const (
	typeString valueType = iota
	typeInt32
	typeInt64
	typeDouble
	typeBool
	typeInvalid
)

// reader reads group code and value records.
type reader struct {
	data   []byte
	pos    int
	binary bool
	good   bool

	strData    string
	doubleData float64
	intData    int
	typ        valueType

	ignoreComments bool
	decoder        codec
}

// readLine is std::getline: up to the next newline, which it eats;
// good is false if it hit the end first.
func (r *reader) readLine() string {
	if r.pos >= len(r.data) {
		r.good = false
		return ""
	}
	i := bytes.IndexByte(r.data[r.pos:], '\n')
	if i < 0 {
		line := string(r.data[r.pos:])
		r.pos = len(r.data)
		r.good = false // eofbit
		return line
	}
	line := string(r.data[r.pos : r.pos+i])
	r.pos += i + 1
	return line
}

// readBytes reads n bytes of a binary file, zero past its end.
func (r *reader) readBytes(n int) []byte {
	buf := make([]byte, n)
	if r.pos+n > len(r.data) {
		copy(buf, r.data[r.pos:])
		r.pos = len(r.data)
		r.good = false
		return buf
	}
	copy(buf, r.data[r.pos:r.pos+n])
	r.pos += n
	return buf
}

// atoi is C atoi.
func atoi(s string) int {
	s = strings.TrimLeft(s, " \t\n\r\f\v")
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	v, _ := strconv.ParseInt(s[:end], 10, 64)
	return int(int32(v))
}

// parseDouble is istream >> double: leading space, then the longest
// decimal number; 0 when there isn't one.
func parseDouble(s string) float64 {
	i := 0
	for i < len(s) && strings.IndexByte(" \t\n\r\f\v", s[i]) >= 0 {
		i++
	}
	start := i
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	v, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil && !math.IsInf(v, 0) {
		return 0
	}
	if math.IsInf(v, 0) {
		// istream sets failbit and the largest value on overflow.
		return math.Copysign(math.MaxFloat64, v)
	}
	return v
}

func (r *reader) readCode() int {
	if r.binary {
		b := r.readBytes(2)
		return int(binary.LittleEndian.Uint16(b))
	}
	return atoi(r.readLine())
}

func (r *reader) readText() string {
	if r.binary {
		i := strings.IndexByte(string(r.data[r.pos:]), 0)
		if i < 0 {
			s := string(r.data[r.pos:])
			r.pos = len(r.data)
			r.good = false
			return s
		}
		s := string(r.data[r.pos : r.pos+i])
		r.pos += i + 1
		return s
	}
	s := r.readLine()
	return strings.TrimSuffix(s, "\r")
}

func (r *reader) readString() {
	r.typ = typeString
	r.strData = r.readText()
}

func (r *reader) readBinaryChunk() {
	if r.binary {
		n := int(r.readBytes(1)[0])
		r.pos += n
		if r.pos > len(r.data) {
			r.pos = len(r.data)
			r.good = false
		}
		return
	}
	r.readString()
}

func (r *reader) readInt16() {
	r.typ = typeInt32
	if r.binary {
		b := r.readBytes(2)
		// The bytes combine as signed chars, as the files were written.
		r.intData = int(int8(b[1]))<<8 | int(int8(b[0]))
		return
	}
	r.intData = atoi(r.readText())
}

func (r *reader) readInt32() {
	r.typ = typeInt32
	if r.binary {
		r.intData = int(binary.LittleEndian.Uint32(r.readBytes(4)))
		return
	}
	r.readInt16()
}

func (r *reader) readInt64() {
	r.typ = typeInt64
	if r.binary {
		r.readBytes(8)
		return
	}
	r.readInt16()
	r.typ = typeInt32
}

func (r *reader) readDouble() {
	r.typ = typeDouble
	if r.binary {
		r.doubleData = math.Float64frombits(binary.LittleEndian.Uint64(r.readBytes(8)))
		return
	}
	r.doubleData = parseDouble(r.readText())
}

func (r *reader) readBool() {
	r.typ = typeBool
	if r.binary {
		r.intData = int(int8(r.readBytes(1)[0]))
		return
	}
	r.intData = atoi(r.readText())
}

// readRec reads a record, and reports whether the file is still good.
func (r *reader) readRec() (int, bool) {
	r.good = true
	code := r.readCode()
	if !r.good {
		return code, false
	}

	switch {
	case code < 10:
		r.readString()
	case code < 60:
		r.readDouble()
	case code < 80:
		r.readInt16()
	case code > 89 && code < 100:
		r.readInt32()
	case code == 100 || code == 102 || code == 105:
		r.readString()
	case code > 109 && code < 150:
		r.readDouble()
	case code > 159 && code < 170:
		r.readInt64()
	case code < 180:
		r.readInt16()
	case code > 209 && code < 240:
		r.readDouble()
	case code > 269 && code < 290:
		r.readInt16()
	case code < 300:
		r.readBool()
	case code < 310:
		r.readString()
	case code < 320:
		r.readBinaryChunk()
	case code < 370:
		r.readString()
	case code < 390:
		r.readInt16()
	case code < 400:
		r.readString()
	case code < 410:
		r.readInt16()
	case code < 420:
		r.readString()
	case code < 430:
		r.readInt32()
	case code < 440:
		r.readString()
	case code < 450:
		r.readInt32()
	case code < 460:
		r.readInt32()
	case code < 470:
		r.readDouble()
	case code < 481:
		r.readString()
	case code == 999 && r.ignoreComments:
		r.readString()
		return r.readRec()
	case code == 1004:
		r.readBinaryChunk()
	case code > 998 && code < 1009:
		r.readString()
	case code < 1060:
		r.readDouble()
	case code < 1071:
		r.readInt16()
	case code == 1071:
		r.readInt32()
	default:
		// dxfReader's skip is true: read the value as a string.
		r.readString()
	}
	return code, r.good
}

func (r *reader) getString() string     { return r.strData }
func (r *reader) getUtf8String() string { return r.decoder.toUtf8(r.strData) }
func (r *reader) getDouble() float64    { return r.doubleData }
func (r *reader) getInt32() int         { return r.intData }
func (r *reader) getBool() bool         { return r.intData != 0 }

// getHandleString is the string as a hex handle.
func (r *reader) getHandleString() int {
	s := strings.TrimLeft(r.strData, " \t\n\r\f\v")
	end := 0
	for end < len(s) && strings.IndexByte("0123456789abcdefABCDEF", s[end]) >= 0 {
		end++
	}
	v, err := strconv.ParseInt(s[:end], 16, 64)
	if err != nil || v > math.MaxInt32 {
		return 0
	}
	return int(v)
}
