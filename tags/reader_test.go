package tags_test

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/apantanella/dxf/tags"
)

func readAll(r *tags.Reader) ([]tags.Tag, error) {
	var out []tags.Tag
	for {
		t, err := r.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, t)
	}
}

func TestReaderRead(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []tags.Tag
	}{
		{"empty", "", nil},
		{"single", "0\nSECTION\n", []tags.Tag{{0, "SECTION"}}},
		{"several LF", "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1027\n",
			[]tags.Tag{{0, "SECTION"}, {2, "HEADER"}, {9, "$ACADVER"}, {1, "AC1027"}}},
		{"CRLF", "0\r\nSECTION\r\n2\r\nHEADER\r\n", []tags.Tag{{0, "SECTION"}, {2, "HEADER"}}},
		{"mixed EOL", "0\r\nSECTION\n2\nHEADER\r\n", []tags.Tag{{0, "SECTION"}, {2, "HEADER"}}},
		{"no final newline", "0\nEOF", []tags.Tag{{0, "EOF"}}},
		{"no final newline CRLF", "0\r\nEOF\r", []tags.Tag{{0, "EOF"}}},
		{"blank lines between pairs", "\n0\nA\n\n\n2\nB\n\n", []tags.Tag{{0, "A"}, {2, "B"}}},
		{"blank CRLF lines", "\r\n0\r\nA\r\n\r\n2\r\nB\r\n", []tags.Tag{{0, "A"}, {2, "B"}}},
		{"whitespace-only lines", "  \n0\nA\n \t \n2\nB\n", []tags.Tag{{0, "A"}, {2, "B"}}},
		{"padded codes", "  0\nSECTION\n 10\n1.5\n100\nAcDb\n", []tags.Tag{{0, "SECTION"}, {10, "1.5"}, {100, "AcDb"}}},
		{"code with spaces", " 10 \n1\n", []tags.Tag{{10, "1"}}},
		{"empty value", "1\n\n2\nx\n", []tags.Tag{{1, ""}, {2, "x"}}},
		{"empty value then blank", "0\n\n\n2\nB\n", []tags.Tag{{0, ""}, {2, "B"}}},
		{"value keeps spaces", "1\n  a b  \n", []tags.Tag{{1, "  a b  "}}},
		{"all trailing CRs removed", "1\nx\r\r\n", []tags.Tag{{1, "x"}}},
		{"inner CR kept", "1\na\rb\n", []tags.Tag{{1, "a\rb"}}},
		{"boundary codes", "-5\nA\n1071\nB\n", []tags.Tag{{-5, "A"}, {1071, "B"}}},
		{"utf8 value", "1\nàè€\n", []tags.Tag{{1, "àè€"}}},
		{"code zero padded digits", "007\nx\n", []tags.Tag{{7, "x"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readAll(tags.NewReader(strings.NewReader(tc.input)))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReaderEOF(t *testing.T) {
	r := tags.NewReader(strings.NewReader("0\nA\n"))
	if _, err := r.Next(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := r.Next(); err != io.EOF {
			t.Fatalf("call %d: got %v, want io.EOF", i, err)
		}
	}
}

func TestReaderNilOption(t *testing.T) {
	r := tags.NewReader(strings.NewReader("0\nA\n"), nil)
	if tag, err := r.Next(); err != nil || tag != (tags.Tag{Code: 0, Value: "A"}) {
		t.Fatalf("got %v, %v", tag, err)
	}
}

func TestReaderSyntaxErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantBefore int
		wantLine   int
		wantCode   int
		reason     string
	}{
		{"non numeric", "abc\nX\n", 0, 1, 0, "invalid group code"},
		{"non numeric after tag", "0\nA\nabc\nX\n", 1, 3, 0, "invalid group code"},
		{"real code", "1.5\nx\n", 0, 1, 0, "invalid group code"},
		{"code overflow", "99999999999999999999\nx\n", 0, 1, 0, "invalid group code"},
		{"above range", "1072\nx\n", 0, 1, 1072, "outside the valid range"},
		{"below range", "-6\nx\n", 0, 1, -6, "outside the valid range"},
		{"out of range later", "0\nA\n2\nB\n5000\nC\n", 2, 5, 5000, "outside the valid range"},
		{"CRLF line number", "0\r\nA\r\nzz\r\nB\r\n", 1, 3, 0, "invalid group code"},
		{"blank lines count", "\n\nzz\nx\n", 0, 3, 0, "invalid group code"},
		{"incomplete pair", "0\nA\n2\n", 1, 3, 2, "incomplete"},
		{"incomplete no newline", "0", 0, 1, 0, "incomplete"},
		{"incomplete after blanks", "\n\n5", 0, 3, 5, "incomplete"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readAll(tags.NewReader(strings.NewReader(tc.input)))
			if !errors.Is(err, tags.ErrSyntax) {
				t.Fatalf("err = %v, want ErrSyntax", err)
			}
			if errors.Is(err, tags.ErrLimit) {
				t.Errorf("syntax error must not match ErrLimit")
			}
			var se *tags.SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("err %T is not *SyntaxError", err)
			}
			if se.Line != tc.wantLine {
				t.Errorf("Line = %d, want %d", se.Line, tc.wantLine)
			}
			if se.Code != tc.wantCode {
				t.Errorf("Code = %d, want %d", se.Code, tc.wantCode)
			}
			if !strings.Contains(se.Reason, tc.reason) {
				t.Errorf("Reason = %q, want it to contain %q", se.Reason, tc.reason)
			}
			if len(got) != tc.wantBefore {
				t.Errorf("read %d tags before the error, want %d", len(got), tc.wantBefore)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Errorf("message %q has no line", err.Error())
			}
		})
	}
}

func limitErr(t *testing.T, err error) *tags.LimitError {
	t.Helper()
	if !errors.Is(err, tags.ErrLimit) {
		t.Fatalf("err = %v, want ErrLimit", err)
	}
	if errors.Is(err, tags.ErrSyntax) {
		t.Errorf("limit error must not match ErrSyntax")
	}
	var le *tags.LimitError
	if !errors.As(err, &le) {
		t.Fatalf("err %T is not *LimitError", err)
	}
	return le
}

func TestReaderValueLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		value string
		eol   string
		ok    bool
	}{
		{"at limit", 10, strings.Repeat("a", 10), "\n", true},
		{"at limit CRLF", 10, strings.Repeat("a", 10), "\r\n", true},
		{"over limit", 10, strings.Repeat("a", 11), "\n", false},
		{"over limit CRLF", 10, strings.Repeat("a", 11), "\r\n", false},
		{"small limit at", 5, "abcde", "\n", true},
		{"small limit over", 5, "abcdef", "\n", false},
		{"small limit over CRLF", 5, "abcdef", "\r\n", false},
		{"limit 1 at", 1, "a", "\n", true},
		{"limit 1 over", 1, "ab", "\n", false},
		{"limit 0 empty value", 0, "", "\n", true},
		{"limit 0 over", 0, "a", "\n", false},
		{"small limit, line longer than 64", 5, strings.Repeat("a", 70), "\n", false},
		{"limit 100 at", 100, strings.Repeat("a", 100), "\r\n", true},
		{"limit 100 over", 100, strings.Repeat("a", 101), "\r\n", false},
		{"limit 100 over LF", 100, strings.Repeat("a", 101), "\n", false},
		{"large value with big limit", 1 << 20, strings.Repeat("a", 100000), "\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := "0\n\n1\n" + tc.value + tc.eol + "2\n\n"
			r := tags.NewReader(strings.NewReader(in), tags.WithMaxValueBytes(tc.limit))
			got, err := readAll(r)
			if tc.ok {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(got) != 3 || got[1].Value != tc.value {
					t.Fatalf("got %d tags, value ok = %v", len(got), len(got) > 1 && got[1].Value == tc.value)
				}
				return
			}
			le := limitErr(t, err)
			if le.Kind != "value" || le.Limit != tc.limit {
				t.Errorf("LimitError = %+v, want Kind value, Limit %d", le, tc.limit)
			}
			if le.Line < 3 || le.Line > 4 {
				t.Errorf("Line = %d, want the code line (3) or the value line (4)", le.Line)
			}
			if len(got) != 1 {
				t.Errorf("read %d tags before the error, want 1", len(got))
			}
		})
	}
}

// With a limit under 64 the line is read with a bound of 64 bytes but the value
// is checked against the exact limit, reporting the line of the group code.
func TestReaderSmallLimitReportsCodeLine(t *testing.T) {
	r := tags.NewReader(strings.NewReader("0\nabcdef\n"), tags.WithMaxValueBytes(5))
	_, err := r.Next()
	le := limitErr(t, err)
	if le.Line != 1 || le.Limit != 5 {
		t.Errorf("LimitError = %+v, want Line 1, Limit 5", le)
	}
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

type endlessReader struct{ b byte }

func (e endlessReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = e.b
	}
	return len(p), nil
}

func TestReaderLongLineWithoutNewlineStopsEarly(t *testing.T) {
	const total = 4 << 20
	tests := []struct {
		name  string
		src   func() io.Reader
		limit int
		line  int
	}{
		{"value, repeated string", func() io.Reader {
			return strings.NewReader("0\n" + strings.Repeat("a", total))
		}, 1024, 2},
		{"value, endless limited reader", func() io.Reader {
			return io.MultiReader(strings.NewReader("0\n"), io.LimitReader(endlessReader{'a'}, total))
		}, 1024, 2},
		{"value, default limit", func() io.Reader {
			return io.MultiReader(strings.NewReader("0\n"), io.LimitReader(endlessReader{'a'}, total))
		}, tags.DefaultMaxValueBytes, 2},
		{"group code line", func() io.Reader {
			return io.LimitReader(endlessReader{'1'}, total)
		}, 1024, 1},
		{"tiny limit", func() io.Reader {
			return io.MultiReader(strings.NewReader("0\n"), io.LimitReader(endlessReader{'a'}, total))
		}, 3, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cr := &countingReader{r: tc.src()}
			r := tags.NewReader(cr, tags.WithMaxValueBytes(tc.limit))
			_, err := r.Next()
			le := limitErr(t, err)
			if le.Line != tc.line {
				t.Errorf("Line = %d, want %d", le.Line, tc.line)
			}
			// The limit applies to the default 1 MiB case too: stop well before 4 MiB.
			if bound := int64(tc.limit) + 128<<10; cr.n > bound {
				t.Errorf("reader consumed %d bytes with limit %d (bound %d): the line was buffered", cr.n, tc.limit, bound)
			}
		})
	}
}

func TestReaderPeek(t *testing.T) {
	r := tags.NewReader(strings.NewReader("0\nA\n\n2\nB\n"))
	if r.Line() != 0 {
		t.Errorf("Line before reading = %d, want 0", r.Line())
	}
	for i := 0; i < 2; i++ {
		tag, ok, err := r.Peek()
		if err != nil || !ok || tag != (tags.Tag{Code: 0, Value: "A"}) {
			t.Fatalf("Peek %d = %v, %v, %v", i, tag, ok, err)
		}
		if r.Line() != 0 {
			t.Errorf("Line after Peek = %d, want 0 (not consumed)", r.Line())
		}
	}
	tag, err := r.Next()
	if err != nil || tag != (tags.Tag{Code: 0, Value: "A"}) {
		t.Fatalf("Next = %v, %v", tag, err)
	}
	if r.Line() != 1 {
		t.Errorf("Line after first Next = %d, want 1", r.Line())
	}
	tag, ok, err := r.Peek()
	if err != nil || !ok || tag != (tags.Tag{Code: 2, Value: "B"}) {
		t.Fatalf("Peek = %v, %v, %v", tag, ok, err)
	}
	if r.Line() != 1 {
		t.Errorf("Line after Peek = %d, want 1", r.Line())
	}
	if tag, err = r.Next(); err != nil || tag.Code != 2 {
		t.Fatalf("Next = %v, %v", tag, err)
	}
	if r.Line() != 4 {
		t.Errorf("Line after second Next = %d, want 4 (blank line counted)", r.Line())
	}
	if _, ok, err = r.Peek(); ok || err != nil {
		t.Errorf("Peek at EOF = ok %v, err %v; want false, nil", ok, err)
	}
	if _, err = r.Next(); err != io.EOF {
		t.Errorf("Next at EOF = %v, want io.EOF", err)
	}
}

func TestReaderPeekError(t *testing.T) {
	r := tags.NewReader(strings.NewReader("zz\nA\n"))
	_, ok, err := r.Peek()
	if ok || !errors.Is(err, tags.ErrSyntax) {
		t.Fatalf("Peek = ok %v, err %v; want false, ErrSyntax", ok, err)
	}
	r = tags.NewReader(strings.NewReader("0\n"))
	_, ok, err = r.Peek()
	if ok || !errors.Is(err, tags.ErrSyntax) {
		t.Fatalf("Peek of incomplete pair = ok %v, err %v; want false, ErrSyntax", ok, err)
	}
}

func TestReaderLine(t *testing.T) {
	r := tags.NewReader(strings.NewReader("0\r\nA\r\n\r\n  \r\n 2\r\nB\r\n999\nC"))
	want := []int{1, 5, 7}
	for i, wl := range want {
		if _, err := r.Next(); err != nil {
			t.Fatal(err)
		}
		if r.Line() != wl {
			t.Errorf("tag %d: Line = %d, want %d", i, r.Line(), wl)
		}
	}
}

func TestReaderNonASCII(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantFirst int
	}{
		{"none", "0\nabc\n1\nxyz\n", 0, 0},
		{"one", "0\nabc\n1\nà\n", 1, 3},
		{"two values, first line kept", "0\nabc\n1\nà\n1\nè\n", 2, 3},
		{"counted per value", "1\nààà\n", 1, 1},
		{"first value", "1\n€\n2\nx\n3\nü\n", 2, 1},
		{"after blank lines", "\n\n1\nü\n", 1, 3},
		{"CRLF", "0\r\nA\r\n1\r\nÄ\r\n", 1, 3},
		{"high byte not utf8", "1\nabc\xe8def\n", 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := tags.NewReader(strings.NewReader(tc.input))
			if _, err := readAll(r); err != nil {
				t.Fatal(err)
			}
			count, first := r.NonASCII()
			if count != tc.wantCount || first != tc.wantFirst {
				t.Errorf("NonASCII = (%d, %d), want (%d, %d)", count, first, tc.wantCount, tc.wantFirst)
			}
		})
	}
}

func TestReaderIOError(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name  string
		input io.Reader
	}{
		{"on group code", iotest.ErrReader(boom)},
		{"on value", io.MultiReader(strings.NewReader("0\n"), iotest.ErrReader(boom))},
		{"mid line", io.MultiReader(strings.NewReader("0\nabc"), iotest.ErrReader(boom))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tags.NewReader(tc.input).Next()
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v, want boom", err)
			}
		})
	}
}

func TestReaderOneByteReads(t *testing.T) {
	in := "0\r\nSECTION\r\n\r\n2\r\nHEADER\r\n9\r\n$X\r\n40\r\n1.5"
	got, err := readAll(tags.NewReader(iotest.OneByteReader(strings.NewReader(in))))
	if err != nil {
		t.Fatal(err)
	}
	want := []tags.Tag{{0, "SECTION"}, {2, "HEADER"}, {9, "$X"}, {40, "1.5"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
