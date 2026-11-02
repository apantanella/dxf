package tags

import (
	"strings"
	"unicode/utf8"
)

// Encoding identifies the character encoding of the text values of a DXF file
// before AutoCAD 2007 (AC1021), which depends on the $DWGCODEPAGE header
// variable. Files from AC1021 on are UTF-8.
//
// The tables are built in: the package needs no external dependency. Encoding
// is not yet used by Reader and Writer, which pass values through as bytes.
type Encoding int

// Supported encodings.
const (
	// EncodingUTF8 is UTF-8.
	EncodingUTF8 Encoding = iota
	// EncodingCP1252 is Windows-1252 ($DWGCODEPAGE ANSI_1252).
	EncodingCP1252
	// EncodingLatin1 is ISO 8859-1 ($DWGCODEPAGE ISO8859-1).
	EncodingLatin1
	// EncodingUnknown is the encoding of an unrecognised code page. Decode and
	// Encode leave text unchanged.
	EncodingUnknown
)

// String returns a short name for the encoding.
func (e Encoding) String() string {
	switch e {
	case EncodingUTF8:
		return "UTF-8"
	case EncodingCP1252:
		return "CP1252"
	case EncodingLatin1:
		return "ISO-8859-1"
	default:
		return "unknown"
	}
}

// EncodingFromCodePage maps the value of $DWGCODEPAGE to an Encoding, ignoring
// case and surrounding blanks. It returns (EncodingUnknown, false) for an empty
// or unrecognised name.
func EncodingFromCodePage(name string) (Encoding, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "ANSI_1252":
		return EncodingCP1252, true
	case "ISO8859-1", "ISO-8859-1", "LATIN1":
		return EncodingLatin1, true
	case "UTF-8", "UTF8":
		return EncodingUTF8, true
	default:
		return EncodingUnknown, false
	}
}

// cp1252High maps bytes 0x80..0x9F of Windows-1252 to runes. The five bytes
// that Windows leaves undefined map to the C1 control of the same value.
var cp1252High = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, // 80-87
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F, // 88-8F
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, // 90-97
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178, // 98-9F
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// Decode converts s, which holds the bytes of the file, to UTF-8. Text that is
// already ASCII is returned unchanged without allocating. For UTF-8 and for an
// unknown encoding valid text is also returned unchanged; with UTF-8 every
// invalid byte becomes U+FFFD.
func (e Encoding) Decode(s string) string {
	switch e {
	case EncodingCP1252, EncodingLatin1:
		if isASCII(s) {
			return s
		}
		var b strings.Builder
		b.Grow(len(s) + len(s)/2)
		for i := 0; i < len(s); i++ {
			c := s[i]
			switch {
			case c < utf8.RuneSelf:
				b.WriteByte(c)
			case e == EncodingCP1252 && c < 0xA0:
				b.WriteRune(cp1252High[c-0x80])
			default:
				b.WriteRune(rune(c))
			}
		}
		return b.String()
	case EncodingUTF8:
		if utf8.ValidString(s) {
			return s
		}
		var b strings.Builder
		b.Grow(len(s) + 8)
		for _, r := range s { // an invalid byte is yielded as U+FFFD
			b.WriteRune(r)
		}
		return b.String()
	default:
		return s
	}
}

// Encode is the inverse of Decode: it converts UTF-8 text to the bytes of the
// encoding. A character the encoding cannot represent becomes the escape
// \U+XXXX (see EncodeEscapes). Backslashes are never altered. UTF-8 text and
// text in an unknown encoding are returned unchanged, as is ASCII text.
func (e Encoding) Encode(s string) string {
	if (e != EncodingCP1252 && e != EncodingLatin1) || isASCII(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s { // an invalid byte is yielded as U+FFFD
		if c, ok := e.byteFor(r); ok {
			b.WriteByte(c)
		} else {
			writeEscape(&b, r)
		}
	}
	return b.String()
}

// byteFor returns the byte that represents r, for CP1252 and Latin1 only.
func (e Encoding) byteFor(r rune) (byte, bool) {
	switch {
	case r < utf8.RuneSelf:
		return byte(r), true
	case e == EncodingLatin1:
		if r < 0x100 {
			return byte(r), true
		}
	case r >= 0xA0 && r < 0x100:
		return byte(r), true
	default:
		// CP1252, 0x80..0x9F: the special characters and the five C1 controls
		// of the undefined bytes.
		for i, c := range cp1252High {
			if c == r {
				return byte(0x80 + i), true
			}
		}
	}
	return 0, false
}

const upperHex = "0123456789ABCDEF"

func writeEscape1(b *strings.Builder, u uint16) {
	b.WriteString(`\U+`)
	b.WriteByte(upperHex[u>>12&0xF])
	b.WriteByte(upperHex[u>>8&0xF])
	b.WriteByte(upperHex[u>>4&0xF])
	b.WriteByte(upperHex[u&0xF])
}

// writeEscape writes r as \U+XXXX. A rune beyond the Basic Multilingual Plane
// is written as the escapes of its UTF-16 surrogate pair, so that every escape
// has exactly four digits.
func writeEscape(b *strings.Builder, r rune) {
	if r >= 0x10000 && r <= utf8.MaxRune {
		r -= 0x10000
		writeEscape1(b, uint16(0xD800+(r>>10)))
		writeEscape1(b, uint16(0xDC00+(r&0x3FF)))
		return
	}
	writeEscape1(b, uint16(r))
}

// EncodeEscapes replaces every non-ASCII character of s with its \U+XXXX
// escape, for text that has to stay pure ASCII. Characters beyond U+FFFF become
// two escapes (a UTF-16 surrogate pair); invalid UTF-8 becomes \U+FFFD.
func EncodeEscapes(s string) string {
	if isASCII(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		if s[i] < utf8.RuneSelf {
			b.WriteByte(s[i])
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		writeEscape(&b, r)
		i += size - 1
	}
	return b.String()
}

func hexVal(c byte) (uint16, bool) {
	switch {
	case c >= '0' && c <= '9':
		return uint16(c - '0'), true
	case c >= 'A' && c <= 'F':
		return uint16(c-'A') + 10, true
	case c >= 'a' && c <= 'f':
		return uint16(c-'a') + 10, true
	}
	return 0, false
}

// parseEscape parses a \U+XXXX escape at the start of s.
func parseEscape(s string) (uint16, bool) {
	if len(s) < 7 || s[0] != '\\' || s[1] != 'U' || s[2] != '+' {
		return 0, false
	}
	var v uint16
	for i := 3; i < 7; i++ {
		d, ok := hexVal(s[i])
		if !ok {
			return 0, false
		}
		v = v<<4 | d
	}
	return v, true
}

// DecodeEscapes replaces the escapes \U+XXXX (four hexadecimal digits, either
// case) in s with the character they denote. An escape pair that forms a UTF-16
// surrogate pair gives one character. A malformed escape, or one that denotes a
// lone surrogate, is left unchanged.
func DecodeEscapes(s string) string {
	if !strings.Contains(s, `\U+`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			i++
			continue
		}
		v, ok := parseEscape(s[i:])
		if !ok {
			b.WriteByte('\\')
			i++
			continue
		}
		switch {
		case v >= 0xD800 && v <= 0xDBFF:
			if lo, ok := parseEscape(s[i+7:]); ok && lo >= 0xDC00 && lo <= 0xDFFF {
				b.WriteRune(0x10000 + (rune(v)-0xD800)<<10 + rune(lo) - 0xDC00)
				i += 14
				continue
			}
		case v >= 0xDC00 && v <= 0xDFFF:
		default:
			b.WriteRune(rune(v))
			i += 7
			continue
		}
		b.WriteString(s[i : i+7]) // lone surrogate: keep as written
		i += 7
	}
	return b.String()
}
