package dxf

import "github.com/apantanella/dxf/tags"

// Group is a single DXF group code/value pair. It is an alias of tags.Tag, so
// values from the two packages can be mixed freely.
type Group = tags.Tag

// GroupKind classifies the value type associated with a group code.
type GroupKind = tags.Kind

// Group value kinds.
const (
	GroupKindUnknown = tags.KindUnknown
	GroupKindString  = tags.KindString
	GroupKindFloat   = tags.KindFloat
	GroupKindInt     = tags.KindInt
	GroupKindBool    = tags.KindBool
	GroupKindHandle  = tags.KindHandle
	GroupKindBinary  = tags.KindBinary
)

// KindOf returns the value type associated with a DXF group code.
func KindOf(code int) GroupKind { return tags.KindOf(code) }

// IsValidCode reports whether code is a valid DXF group code.
func IsValidCode(code int) bool { return tags.IsValidCode(code) }
