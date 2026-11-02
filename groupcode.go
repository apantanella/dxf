package dxf

import (
	"fmt"
	"strconv"
	"strings"
)

// Group is a single DXF group code/value pair.
type Group struct {
	Code  int
	Value string
}

// String renders the pair as "code: value".
func (g Group) String() string { return fmt.Sprintf("%d: %s", g.Code, g.Value) }

// Float parses the value as a real number.
func (g Group) Float() (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(g.Value), 64)
	if err != nil {
		return 0, &SyntaxError{Code: g.Code, Value: g.Value, Reason: "invalid real value"}
	}
	return v, nil
}

// Int parses the value as an integer. Real values with a zero fractional part,
// such as "1.0", are accepted because some CAD applications write them that way.
func (g Group) Int() (int, error) {
	s := strings.TrimSpace(g.Value)
	if v, err := strconv.Atoi(s); err == nil {
		return v, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, &SyntaxError{Code: g.Code, Value: g.Value, Reason: "invalid integer value"}
	}
	i := int(f)
	if float64(i) != f {
		return 0, &SyntaxError{Code: g.Code, Value: g.Value, Reason: "invalid integer value"}
	}
	return i, nil
}

// GroupKind classifies the value type associated with a group code.
type GroupKind int

// Group value kinds.
const (
	GroupKindUnknown GroupKind = iota
	GroupKindString
	GroupKindFloat
	GroupKindInt
	GroupKindBool
	GroupKindHandle
	GroupKindBinary
)

// String returns the kind name.
func (k GroupKind) String() string {
	switch k {
	case GroupKindString:
		return "string"
	case GroupKindFloat:
		return "float"
	case GroupKindInt:
		return "int"
	case GroupKindBool:
		return "bool"
	case GroupKindHandle:
		return "handle"
	case GroupKindBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// Kind returns the value type expected for the group code.
func (g Group) Kind() GroupKind { return KindOf(g.Code) }

// KindOf returns the value type associated with a DXF group code.
func KindOf(code int) GroupKind {
	switch {
	case code >= -5 && code <= 9:
		return GroupKindString
	case code >= 10 && code <= 59:
		return GroupKindFloat
	case code >= 60 && code <= 79:
		return GroupKindInt
	case code >= 90 && code <= 99:
		return GroupKindInt
	case code == 100:
		return GroupKindString
	case code >= 101 && code <= 104:
		return GroupKindString
	case code == 105:
		return GroupKindHandle
	case code >= 110 && code <= 149:
		return GroupKindFloat
	case code >= 160 && code <= 169:
		return GroupKindInt
	case code >= 170 && code <= 179:
		return GroupKindInt
	case code >= 210 && code <= 239:
		return GroupKindFloat
	case code >= 270 && code <= 289:
		return GroupKindInt
	case code >= 290 && code <= 299:
		return GroupKindBool
	case code >= 300 && code <= 309:
		return GroupKindString
	case code >= 310 && code <= 319:
		return GroupKindBinary
	case code >= 320 && code <= 369:
		return GroupKindHandle
	case code >= 370 && code <= 389:
		return GroupKindInt
	case code >= 390 && code <= 399:
		return GroupKindHandle
	case code >= 400 && code <= 409:
		return GroupKindInt
	case code >= 410 && code <= 419:
		return GroupKindString
	case code >= 420 && code <= 429:
		return GroupKindInt
	case code >= 430 && code <= 439:
		return GroupKindString
	case code >= 440 && code <= 449:
		return GroupKindInt
	case code >= 450 && code <= 459:
		return GroupKindInt
	case code >= 460 && code <= 469:
		return GroupKindFloat
	case code >= 470 && code <= 479:
		return GroupKindString
	case code >= 480 && code <= 481:
		return GroupKindHandle
	case code == 999:
		return GroupKindString
	case code >= 1000 && code <= 1009:
		return GroupKindString
	case code >= 1010 && code <= 1059:
		return GroupKindFloat
	case code >= 1060 && code <= 1071:
		return GroupKindInt
	default:
		return GroupKindUnknown
	}
}

// IsValidCode reports whether code is a valid DXF group code.
func IsValidCode(code int) bool { return code >= -5 && code <= 1071 }
