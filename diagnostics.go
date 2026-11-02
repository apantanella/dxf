package dxf

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apantanella/dxf/tags"
)

// Sentinel errors returned by the package. Test them with errors.Is.
var (
	// ErrNotDXF reports a stream that does not start with a DXF SECTION record.
	ErrNotDXF = errors.New("dxf: not a DXF stream")

	// ErrUnsupportedVersion reports a $ACADVER value other than AC1009.
	ErrUnsupportedVersion = errors.New("dxf: unsupported DXF version")

	// ErrSyntax reports a malformed group code/value pair or section structure.
	ErrSyntax = tags.ErrSyntax

	// ErrLimit reports that a configured reading limit was exceeded.
	ErrLimit = tags.ErrLimit

	// ErrValidation reports a document that cannot be written.
	ErrValidation = errors.New("dxf: invalid document")
)

// Severity classifies a Diagnostic.
type Severity int

// Diagnostic severities.
const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// String returns the severity name.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// Diagnostic describes a non-fatal anomaly found while decoding.
type Diagnostic struct {
	Severity Severity
	Line     int
	Section  string
	Entity   string
	Message  string
}

// String renders the diagnostic with its context.
func (d Diagnostic) String() string {
	var b strings.Builder
	b.WriteString(d.Severity.String())
	if d.Line > 0 {
		fmt.Fprintf(&b, " line %d", d.Line)
	}
	if d.Section != "" {
		fmt.Fprintf(&b, " section %s", d.Section)
	}
	if d.Entity != "" {
		fmt.Fprintf(&b, " entity %s", d.Entity)
	}
	b.WriteString(": ")
	b.WriteString(d.Message)
	return b.String()
}

// SyntaxError describes a malformed group code/value pair or record. It is
// defined in package tags, which reads the stream.
type SyntaxError = tags.SyntaxError

// LimitError reports that a reading limit was exceeded. It is defined in
// package tags.
type LimitError = tags.LimitError

// VersionError reports a $ACADVER value different from AC1009.
type VersionError struct {
	Found string
}

// Error implements the error interface.
func (e *VersionError) Error() string {
	if e.Found == "" {
		return "dxf: unsupported version: missing $ACADVER"
	}
	return fmt.Sprintf("dxf: unsupported version: %q (expected %s)", e.Found, VersionR12)
}

// Unwrap returns ErrUnsupportedVersion.
func (e *VersionError) Unwrap() error { return ErrUnsupportedVersion }

// ValidationError reports a document that cannot be written.
type ValidationError struct {
	Entity string
	Reason string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Entity != "" {
		return fmt.Sprintf("dxf: invalid document: %s: %s", e.Entity, e.Reason)
	}
	return "dxf: invalid document: " + e.Reason
}

// Unwrap returns ErrValidation.
func (e *ValidationError) Unwrap() error { return ErrValidation }
