package dxfread

// Text decoding (C++ DRW_TextCodec and its converters, reading side).

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"io"
	"slices"
	"strings"
	"sync"
)

// Version is a DXF/DWG version (C++ DRW::Version).
type Version int

const (
	UnknownVersion Version = iota
	MC00
	AC12
	AC14
	AC150
	AC210
	AC1002
	AC1003
	AC1004
	AC1006
	AC1009
	AC1012
	AC1014
	AC1015
	AC1018
	AC1021
	AC1024
	AC1027
	AC1032
)

var versionStrings = map[string]Version{
	"MC0.0": MC00, "AC1.2": AC12, "AC1.4": AC14, "AC1.50": AC150, "AC2.10": AC210,
	"AC1002": AC1002, "AC1003": AC1003, "AC1004": AC1004, "AC1006": AC1006,
	"AC1009": AC1009, "AC1012": AC1012, "AC1014": AC1014, "AC1015": AC1015,
	"AC1018": AC1018, "AC1021": AC1021, "AC1024": AC1024, "AC1027": AC1027,
	"AC1032": AC1032,
}

// converterKind is which C++ DRW_Converter subclass decodes.
type converterKind int

const (
	convUTF8  converterKind = iota // DRW_Converter
	convTable                      // DRW_ConvTable
	conv932                        // DRW_Conv932Table
	convDBCS                       // DRW_ConvDBCSTable
)

// codec decodes the file's strings to UTF-8 (C++ DRW_TextCodec).
type codec struct {
	version Version
	cp      string
	kind    converterKind
	table   *[128]int32
	dbcs    *dbcsTable
}

func newCodec() codec { return codec{version: AC1021, kind: convUTF8} }

// setVersionString is C++ DRW_TextCodec::setVersion(string, dxfFormat).
func (c *codec) setVersionString(v string) {
	c.version = UnknownVersion
	if ver, ok := versionStrings[v]; ok {
		c.version = ver
		c.setVersion(ver)
	}
}

// setVersion is C++ DRW_TextCodec::setVersion(Version, dxfFormat=true).
func (c *codec) setVersion(v Version) {
	switch v {
	case AC1006, AC1009:
		c.version = AC1009
		c.cp = "ANSI_1252"
		c.setCodePage(c.cp)
	case AC1012, AC1014, AC1015, AC1018:
		c.version = AC1015
		c.cp = "ANSI_1252"
		c.setCodePage(c.cp)
	case AC1021, AC1024, AC1027, AC1032:
		c.version = AC1021
		c.cp = "UTF-8"
		c.setCodePage(c.cp)
	}
}

// setCodePage is C++ DRW_TextCodec::setCodePage(c, dxfFormat=true).
func (c *codec) setCodePage(cp string) {
	c.cp = correctCodePage(cp)
	c.table, c.dbcs = nil, nil
	if c.version == AC1009 || c.version == AC1015 {
		switch c.cp {
		case "ANSI_932":
			c.kind = conv932
			c.dbcs = dbcsTables()["932"]
		case "ANSI_936", "ANSI_949", "ANSI_950":
			c.kind = convDBCS
			c.dbcs = dbcsTables()[c.cp[5:]]
		case "UTF-8": // DXF older than 2007 are write in win codepages
			c.cp = "ANSI_1252"
			c.kind = convUTF8
		default:
			c.kind = convTable
			if t, ok := singleByteTables[c.cp]; ok {
				c.table = t
			} else {
				c.table = singleByteTables["ANSI_1252"]
			}
		}
	} else {
		c.kind = convUTF8
	}
}

// correctCodePage names a code page as libdxfrw does (C++
// DRW_TextCodec::correctCodePage).
func correctCodePage(s string) string {
	cp := strings.ToUpper(s)
	in := func(names ...string) bool {
		return slices.Contains(names, cp)
	}
	switch {
	case in("ANSI_874", "CP874", "ISO8859-11", "TIS-620"):
		return "ANSI_874"
	case in("ANSI_1250", "CP1250", "ISO8859-2"):
		return "ANSI_1250"
	case in("ANSI_1251", "CP1251", "ISO8859-5", "KOI8-R", "KOI8-U", "IBM 866"):
		return "ANSI_1251"
	case in("ANSI_1252", "CP1252", "LATIN1", "ISO-8859-1", "CP819", "CSISO", "IBM819",
		"ISO_8859-1", "APPLE ROMAN", "ISO8859-1", "ISO8859-15", "ISO-IR-100", "L1", "IBM 850"):
		return "ANSI_1252"
	case in("ANSI_1253", "CP1253", "ISO8859-7"):
		return "ANSI_1253"
	case in("ANSI_1254", "CP1254", "ISO8859-9", "ISO8859-3"):
		return "ANSI_1254"
	case in("ANSI_1255", "CP1255", "ISO8859-8"):
		return "ANSI_1255"
	case in("ANSI_1256", "CP1256", "ISO8859-6"):
		return "ANSI_1256"
	case in("ANSI_1257", "CP1257", "ISO8859-4", "ISO8859-10", "ISO8859-13"):
		return "ANSI_1257"
	case in("ANSI_1258", "CP1258"):
		return "ANSI_1258"
	case in("ANSI_932", "SHIFT-JIS", "SHIFT_JIS", "CSSHIFTJIS", "CSWINDOWS31J", "MS_KANJI",
		"X-MS-CP932", "X-SJIS", "EUCJP", "EUC-JP", "CSEUCPKDFMTJAPANESE", "X-EUC", "X-EUC-JP", "JIS7"):
		return "ANSI_932"
	case in("ANSI_936", "GBK", "GB2312", "CHINESE", "CN-GB", "CSGB2312", "CSGB231280",
		"CSISO58BG231280", "GB_2312-80", "GB231280", "GB2312-80", "ISO-IR-58", "GB18030"):
		return "ANSI_936"
	case in("ANSI_949", "EUCKR"):
		return "ANSI_949"
	case in("ANSI_950", "BIG5", "CN-BIG5", "CSBIG5", "X-X-BIG5", "BIG5-HKSCS"):
		return "ANSI_950"
	case in("UTF-8", "UTF8", "UTF8-BIT"):
		return "UTF-8"
	case in("UTF-16", "UTF16", "UTF16-BIT"):
		return "UTF-16"
	}
	return "ANSI_1252"
}

// dbcsTable is a double-byte code page (C++ DRW_LeadTableNNN and
// DRW_DoubleTableNNN).
type dbcsTable struct {
	lead   []int32
	double [][2]int32
}

//go:embed cpdbcs.bin.gz
var dbcsBlob []byte

var dbcsTables = sync.OnceValue(func() map[string]*dbcsTable {
	zr, err := gzip.NewReader(bytes.NewReader(dbcsBlob))
	if err != nil {
		panic(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		panic(err)
	}
	u32 := func() int32 {
		v := binary.LittleEndian.Uint32(raw)
		raw = raw[4:]
		return int32(v)
	}
	tables := map[string]*dbcsTable{}
	for _, p := range []string{"932", "936", "949", "950"} {
		t := &dbcsTable{}
		n := int(u32())
		for range n {
			t.lead = append(t.lead, u32())
		}
		m := int(u32())
		for range m {
			code := u32()
			t.double = append(t.double, [2]int32{code, u32()})
		}
		tables[p] = t
	}
	return tables
})

// The characters libdxfrw puts for bytes it can't decode (C++
// NOTFOUND932 and NOTFOUND936).
const (
	notFound932 = 0x30FB
	notFound936 = 0x003F
	cpOffset932 = 0xFEC0
)

// encodeNum is a code point as UTF-8, as C++ DRW_Converter::encodeNum
// builds it: a C string, so 0 gives nothing.
func encodeNum(c int) string {
	var ret []byte
	switch {
	case c < 128:
		ret = []byte{byte(c)}
	case c < 0x800:
		ret = []byte{byte(0xC0 | (c >> 6)), byte(0x80 | (c & 0x3f))}
	case c < 0x10000:
		ret = []byte{byte(0xe0 | (c >> 12)), byte(0x80 | ((c >> 6) & 0x3f)), byte(0x80 | (c & 0x3f))}
	default:
		ret = []byte{byte(0xf0 | (c >> 18)), byte(0x80 | ((c >> 12) & 0x3f)),
			byte(0x80 | ((c >> 6) & 0x3f)), byte(0x80 | (c & 0x3f))}
	}
	if i := bytes.IndexByte(ret, 0); i >= 0 {
		ret = ret[:i]
	}
	return string(ret)
}

// encodeText decodes a "\U+XXXX" escape (C++
// DRW_Converter::encodeText): the hex digits after "\U+" as far as
// they go, as istream >> std::hex reads them.
func encodeText(stmp string) string {
	s := stmp[3:]
	if len(s) > 4 {
		s = s[:4]
	}
	code := 0
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' ||
		s[i] == '\v' || s[i] == '\f') {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	// std::hex accepts an optional 0x prefix.
	if i+1 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
		i += 2
	}
	for ; i < len(s); i++ {
		d := s[i]
		var v int
		switch {
		case d >= '0' && d <= '9':
			v = int(d - '0')
		case d >= 'a' && d <= 'f':
			v = int(d-'a') + 10
		case d >= 'A' && d <= 'F':
			v = int(d-'A') + 10
		default:
			i = len(s)
			continue
		}
		code = code*16 + v
	}
	if neg {
		code = -code
	}
	return encodeNum(code)
}

// toUtf8 decodes a string from the file (C++ DRW_TextCodec::toUtf8).
func (c *codec) toUtf8(s string) string {
	switch c.kind {
	case convTable:
		return c.tableToUtf8(s)
	case conv932:
		return c.dbcs932ToUtf8(s)
	case convDBCS:
		return c.dbcsToUtf8(s)
	}
	return converterToUtf8(s)
}

// converterToUtf8 is C++ DRW_Converter::toUtf8: UTF-8 as it is, but
// "\U+XXXX" escapes decoded.
func converterToUtf8(s string) string {
	var result strings.Builder
	j := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch < 0x80:
			if ch == '\\' && i+6 < len(s) && s[i+1] == 'U' && s[i+2] == '+' {
				result.WriteString(s[j:i])
				result.WriteString(encodeText(s[i : i+7]))
				i += 6
				j = i + 1
			}
		case ch < 0xE0:
			i++
		case ch < 0xF0:
			i += 2
		case ch < 0xF8:
			i += 3
		}
	}
	if j < len(s) {
		result.WriteString(s[j:])
	}
	return result.String()
}

// escape decodes a "\U+" escape at s[i:], or writes the backslash, and
// returns the index of the escape's last byte.
func escape(res *strings.Builder, s string, i int) int {
	if len(s)-i > 6 && s[i+1] == 'U' && s[i+2] == '+' {
		res.WriteString(encodeText(s[i : i+7]))
		return i + 6
	}
	res.WriteByte('\\')
	return i
}

// tableToUtf8 is C++ DRW_ConvTable::toUtf8.
func (c *codec) tableToUtf8(s string) string {
	var res strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\\':
			i = escape(&res, s, i)
		case ch < 0x80:
			res.WriteByte(ch)
		default:
			res.WriteString(encodeNum(int(c.table[ch-0x80])))
		}
	}
	return res.String()
}

// at is the byte at i, or the string's terminating NUL past its end.
func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// dbcsToUtf8 is C++ DRW_ConvDBCSTable::toUtf8.
func (c *codec) dbcsToUtf8(s string) string {
	var res strings.Builder
	t := c.dbcs
	for i := 0; i < len(s); i++ {
		notFound := true
		ch := s[i]
		switch {
		case ch < 0x80:
			notFound = false
			if ch == '\\' {
				i = escape(&res, s, i)
			} else {
				res.WriteByte(ch)
			}
		case ch == 0x80: // 1 byte table
			notFound = false
			res.WriteString(encodeNum(0x20AC)) // euro sign
		default: // 2 bytes
			i++
			code := int32(ch)<<8 | int32(at(s, i))
			sta := t.lead[ch-0x81]
			end := t.lead[ch-0x80]
			for k := sta; k < end; k++ {
				if t.double[k][0] == code {
					res.WriteString(encodeNum(int(t.double[k][1])))
					notFound = false
					break
				}
			}
		}
		if notFound {
			res.WriteString(encodeNum(notFound936))
		}
	}
	return res.String()
}

// dbcs932ToUtf8 is C++ DRW_Conv932Table::toUtf8.
func (c *codec) dbcs932ToUtf8(s string) string {
	var res strings.Builder
	t := c.dbcs
	for i := 0; i < len(s); i++ {
		notFound := true
		ch := s[i]
		switch {
		case ch < 0x80:
			notFound = false
			if ch == '\\' {
				i = escape(&res, s, i)
			} else {
				res.WriteByte(ch)
			}
		case ch > 0xA0 && ch < 0xE0: // 1 byte table
			notFound = false
			res.WriteString(encodeNum(int(ch) + cpOffset932))
		default: // 2 bytes
			i++
			code := int32(ch)<<8 | int32(at(s, i))
			var sta, end int32
			if ch > 0x80 && ch < 0xA0 {
				sta = t.lead[ch-0x81]
				end = t.lead[ch-0x80]
			} else if ch > 0xDF && ch < 0xFD {
				sta = t.lead[ch-0xC1]
				end = t.lead[ch-0xC0]
			}
			if end > 0 {
				for k := sta; k < end; k++ {
					if t.double[k][0] == code {
						res.WriteString(encodeNum(int(t.double[k][1])))
						notFound = false
						break
					}
				}
			}
		}
		if notFound {
			res.WriteString(encodeNum(notFound932))
		}
	}
	return res.String()
}
