package tags

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// DefaultMaxValueBytes is the default limit on the length of a single value.
const DefaultMaxValueBytes = 1 << 20

// minLineBytes is the smallest buffer bound used while reading a line. A value
// longer than the configured limit is still rejected, but the check is made
// after the line is read, so very small limits stay exact.
const minLineBytes = 64

// Option configures a Reader.
type Option func(*Reader)

// WithMaxValueBytes sets the maximum length of a single value. A line longer
// than the limit is rejected as soon as the limit is crossed, so a hostile file
// without line breaks cannot make the reader buffer it whole.
func WithMaxValueBytes(n int) Option {
	return func(r *Reader) { r.maxValue = n }
}

// Reader reads tags from an ASCII DXF stream.
type Reader struct {
	r        *bufio.Reader
	maxValue int

	line          int // lines consumed so far
	lastLine      int // line of the code of the tag returned last
	nonASCII      int // values with bytes >= 0x80
	firstNonASCII int
	pending       *Tag
	pendingLine   int
}

// NewReader returns a Reader over r.
func NewReader(r io.Reader, opts ...Option) *Reader {
	rd := &Reader{r: bufio.NewReader(r), maxValue: DefaultMaxValueBytes}
	for _, opt := range opts {
		if opt != nil {
			opt(rd)
		}
	}
	return rd
}

// Line returns the line number of the group code of the tag returned last by
// Next, or 0 before the first tag.
func (r *Reader) Line() int { return r.lastLine }

// NonASCII returns how many values contained bytes of 0x80 or more, and the
// line of the first of them (0 when there is none). Such values are not valid
// ASCII: they depend on the file's code page or are UTF-8.
func (r *Reader) NonASCII() (count, firstLine int) { return r.nonASCII, r.firstNonASCII }

// readLine reads one line without its end-of-line marker.
func (r *Reader) readLine() (string, error) {
	limit := r.maxValue
	if limit < minLineBytes {
		limit = minLineBytes
	}
	var line []byte
	for {
		fragment, err := r.r.ReadSlice('\n')
		if err == nil && line == nil && len(fragment) <= limit+2 {
			// Common case: the whole line is in the buffer, copy it once.
			r.line++
			return trimEOL(string(fragment)), nil
		}
		line = append(line, fragment...)
		if len(line) > limit+2 { // room for CRLF
			return "", &LimitError{Kind: "value", Limit: r.maxValue, Line: r.line + 1}
		}
		switch {
		case err == nil:
			r.line++
			return trimEOL(string(line)), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) == 0 {
				return "", io.EOF
			}
			r.line++
			return trimEOL(string(line)), nil
		default:
			return "", err
		}
	}
}

func trimEOL(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return strings.TrimRight(s, "\r")
}

func (r *Reader) read() (Tag, int, error) {
	for {
		codeLine, err := r.readLine()
		if err != nil {
			return Tag{}, r.line, err
		}
		if strings.TrimSpace(codeLine) == "" {
			continue
		}
		line := r.line
		code, err := strconv.Atoi(strings.TrimSpace(codeLine))
		if err != nil {
			return Tag{}, line, &SyntaxError{Line: line, Reason: fmt.Sprintf("invalid group code: %q", strings.TrimSpace(codeLine))}
		}
		if !IsValidCode(code) {
			return Tag{}, line, &SyntaxError{Line: line, Code: code, Reason: "group code outside the valid range (-5..1071)"}
		}
		value, err := r.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return Tag{}, line, &SyntaxError{Line: line, Code: code, Reason: "incomplete group code/value pair"}
			}
			return Tag{}, line, err
		}
		if len(value) > r.maxValue {
			return Tag{}, line, &LimitError{Kind: "value", Limit: r.maxValue, Line: line}
		}
		if hasNonASCII(value) {
			if r.nonASCII == 0 {
				r.firstNonASCII = line
			}
			r.nonASCII++
		}
		return Tag{Code: code, Value: value}, line, nil
	}
}

func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}

// Next returns the next tag. It returns io.EOF at a clean end of stream.
func (r *Reader) Next() (Tag, error) {
	if r.pending != nil {
		tag := *r.pending
		r.lastLine = r.pendingLine
		r.pending = nil
		return tag, nil
	}
	tag, line, err := r.read()
	if err != nil {
		return Tag{}, err
	}
	r.lastLine = line
	return tag, nil
}

// Peek returns the next tag without consuming it. ok is false at a clean end of
// stream.
func (r *Reader) Peek() (tag Tag, ok bool, err error) {
	if r.pending != nil {
		return *r.pending, true, nil
	}
	t, line, err := r.read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Tag{}, false, nil
		}
		return Tag{}, false, err
	}
	r.pending = &t
	r.pendingLine = line
	return t, true, nil
}
