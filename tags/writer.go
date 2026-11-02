package tags

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// WriterOption configures a Writer.
type WriterOption func(*Writer)

// WithCRLF makes the writer end lines with CR LF, the usual form on Windows.
// The default is LF alone.
func WithCRLF() WriterOption {
	return func(w *Writer) { w.eol = "\r\n" }
}

// WithPaddedCodes right-aligns group codes in three columns, the layout of the
// reference files tested in ZWCAD. The default writes the code with no padding.
func WithPaddedCodes() WriterOption {
	return func(w *Writer) { w.pad = true }
}

// Writer writes tags to a DXF stream.
//
// Write refuses a tag whose code is out of range or whose value contains a line
// break: such a tag would turn into different tags when read back.
type Writer struct {
	w   *bufio.Writer
	eol string
	pad bool
	err error
}

// NewWriter returns a Writer to w. Call Flush when done.
func NewWriter(w io.Writer, opts ...WriterOption) *Writer {
	wr := &Writer{w: bufio.NewWriter(w), eol: "\n"}
	for _, opt := range opts {
		if opt != nil {
			opt(wr)
		}
	}
	return wr
}

// Write writes one tag. After the first error every later call returns it.
func (w *Writer) Write(t Tag) error {
	if w.err != nil {
		return w.err
	}
	if !IsValidCode(t.Code) {
		w.err = &SyntaxError{Code: t.Code, Reason: "group code outside the valid range (-5..1071)"}
		return w.err
	}
	if strings.ContainsAny(t.Value, "\r\n") {
		w.err = &SyntaxError{Code: t.Code, Value: t.Value, Reason: "value contains a line break"}
		return w.err
	}
	code := strconv.Itoa(t.Code)
	if w.pad {
		for i := len(code); i < 3; i++ {
			if err := w.w.WriteByte(' '); err != nil {
				w.err = err
				return err
			}
		}
	}
	for _, s := range [...]string{code, w.eol, t.Value, w.eol} {
		if _, err := w.w.WriteString(s); err != nil {
			w.err = err
			return err
		}
	}
	return nil
}

// WriteAll writes the tags in order and stops at the first error.
func (w *Writer) WriteAll(ts []Tag) error {
	for _, t := range ts {
		if err := w.Write(t); err != nil {
			return err
		}
	}
	return nil
}

// Flush writes any buffered data and returns the first error met so far.
func (w *Writer) Flush() error {
	// Tags written before a rejected one are valid, so they are flushed even
	// after a sticky error; the first error is still the one returned.
	if err := w.w.Flush(); err != nil && w.err == nil {
		w.err = err
	}
	return w.err
}
