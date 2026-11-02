// Package tags reads and writes the ordered stream of group code/value pairs
// that makes up a DXF file.
//
// The package knows nothing about sections, entities or versions: it turns a
// stream of text lines into []Tag and back. Everything above it (the dxf
// package) is built on that stream, which is also what lets unknown content be
// preserved exactly.
package tags

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Tag is a single DXF group code/value pair. The value is kept as text and
// converted only on request, so reading a tag costs no parsing and writing an
// unchanged tag reproduces its text.
type Tag struct {
	Code  int
	Value string
}

// String renders the pair as "code: value".
func (t Tag) String() string { return fmt.Sprintf("%d: %s", t.Code, t.Value) }

// Float parses the value as a real number.
func (t Tag) Float() (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(t.Value), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, &SyntaxError{Code: t.Code, Value: t.Value, Reason: "invalid real value"}
	}
	return v, nil
}

// Int parses the value as an integer. Real values with a zero fractional part,
// such as "1.0", are accepted because some CAD applications write them that way.
func (t Tag) Int() (int, error) {
	s := strings.TrimSpace(t.Value)
	if v, err := strconv.Atoi(s); err == nil {
		return v, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, &SyntaxError{Code: t.Code, Value: t.Value, Reason: "invalid integer value"}
	}
	// 2^53 is the largest range in which a float64 holds integers exactly; it
	// also keeps the conversion below independent of the platform.
	if f > 1<<53 || f < -(1<<53) {
		return 0, &SyntaxError{Code: t.Code, Value: t.Value, Reason: "invalid integer value"}
	}
	i := int(f)
	if float64(i) != f {
		return 0, &SyntaxError{Code: t.Code, Value: t.Value, Reason: "invalid integer value"}
	}
	return i, nil
}

// Bool parses the value of a Boolean group (codes 290-299) or of an integer
// flag: "0" is false and any other integer is true.
func (t Tag) Bool() (bool, error) {
	v, err := t.Int()
	if err != nil {
		return false, &SyntaxError{Code: t.Code, Value: t.Value, Reason: "invalid boolean value"}
	}
	return v != 0, nil
}

// Hex parses the value as a hexadecimal number, the form of handles (codes 5,
// 105, 320-369, 390-399, 480-481).
func (t Tag) Hex() (uint64, error) {
	s := strings.TrimSpace(t.Value)
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0, &SyntaxError{Code: t.Code, Value: t.Value, Reason: "invalid hexadecimal value"}
	}
	return v, nil
}

// Kind returns the value type expected for the tag's group code.
func (t Tag) Kind() Kind { return KindOf(t.Code) }

// Kind classifies the value type associated with a group code.
type Kind int

// Value kinds.
const (
	KindUnknown Kind = iota
	KindString
	KindFloat
	KindInt
	KindBool
	KindHandle
	KindBinary
)

// String returns the kind name.
func (k Kind) String() string {
	switch k {
	case KindString:
		return "string"
	case KindFloat:
		return "float"
	case KindInt:
		return "int"
	case KindBool:
		return "bool"
	case KindHandle:
		return "handle"
	case KindBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// IsValidCode reports whether code is a valid DXF group code.
func IsValidCode(code int) bool { return code >= -5 && code <= 1071 }

// KindOf returns the value type associated with a DXF group code.
func KindOf(code int) Kind {
	switch {
	case code >= -5 && code <= 9:
		return KindString
	case code >= 10 && code <= 59:
		return KindFloat
	case code >= 60 && code <= 79:
		return KindInt
	case code >= 90 && code <= 99:
		return KindInt
	case code == 100:
		return KindString
	case code >= 101 && code <= 104:
		return KindString
	case code == 105:
		return KindHandle
	case code >= 110 && code <= 149:
		return KindFloat
	case code >= 160 && code <= 169:
		return KindInt
	case code >= 170 && code <= 179:
		return KindInt
	case code >= 210 && code <= 239:
		return KindFloat
	case code >= 270 && code <= 289:
		return KindInt
	case code >= 290 && code <= 299:
		return KindBool
	case code >= 300 && code <= 309:
		return KindString
	case code >= 310 && code <= 319:
		return KindBinary
	case code >= 320 && code <= 369:
		return KindHandle
	case code >= 370 && code <= 389:
		return KindInt
	case code >= 390 && code <= 399:
		return KindHandle
	case code >= 400 && code <= 409:
		return KindInt
	case code >= 410 && code <= 419:
		return KindString
	case code >= 420 && code <= 429:
		return KindInt
	case code >= 430 && code <= 439:
		return KindString
	case code >= 440 && code <= 449:
		return KindInt
	case code >= 450 && code <= 459:
		return KindInt
	case code >= 460 && code <= 469:
		return KindFloat
	case code >= 470 && code <= 479:
		return KindString
	case code >= 480 && code <= 481:
		return KindHandle
	case code == 999:
		return KindString
	case code >= 1000 && code <= 1009:
		return KindString
	case code >= 1010 && code <= 1059:
		return KindFloat
	case code >= 1060 && code <= 1071:
		return KindInt
	default:
		return KindUnknown
	}
}
