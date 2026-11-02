package tags

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors. The dxf package re-exports them, so errors.Is works with
// either name.
var (
	// ErrSyntax reports a malformed group code/value pair or section structure.
	ErrSyntax = errors.New("dxf: syntax error")

	// ErrLimit reports that a configured reading limit was exceeded.
	ErrLimit = errors.New("dxf: limit exceeded")
)

// SyntaxError describes a malformed group code/value pair or record.
type SyntaxError struct {
	Line    int
	Section string
	Entity  string
	Code    int
	Value   string
	Reason  string
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	var b strings.Builder
	b.WriteString("dxf: ")
	if e.Line > 0 {
		fmt.Fprintf(&b, "line %d: ", e.Line)
	}
	if e.Section != "" {
		fmt.Fprintf(&b, "section %s: ", e.Section)
	}
	if e.Entity != "" {
		fmt.Fprintf(&b, "entity %s: ", e.Entity)
	}
	if e.Value != "" {
		fmt.Fprintf(&b, "group code %d %q: ", e.Code, e.Value)
	}
	b.WriteString(e.Reason)
	return b.String()
}

// Unwrap returns ErrSyntax.
func (e *SyntaxError) Unwrap() error { return ErrSyntax }

// LimitError reports that a reading limit was exceeded.
type LimitError struct {
	Kind  string
	Limit int
	Line  int
}

// Error implements the error interface.
func (e *LimitError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("dxf: line %d: limit exceeded for %s: maximum %d", e.Line, e.Kind, e.Limit)
	}
	return fmt.Sprintf("dxf: limit exceeded for %s: maximum %d", e.Kind, e.Limit)
}

// Unwrap returns ErrLimit.
func (e *LimitError) Unwrap() error { return ErrLimit }
