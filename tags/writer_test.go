package tags_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

func writeString(t *testing.T, ts []tags.Tag, opts ...tags.WriterOption) string {
	t.Helper()
	var buf bytes.Buffer
	w := tags.NewWriter(&buf, opts...)
	if err := w.WriteAll(ts); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestWriterDefault(t *testing.T) {
	var buf bytes.Buffer
	w := tags.NewWriter(&buf)
	for _, tag := range []tags.Tag{{0, "SECTION"}, {2, "HEADER"}, {10, "1.5"}, {1, ""}} {
		if err := w.Write(tag); err != nil {
			t.Fatal(err)
		}
	}
	if buf.Len() != 0 {
		t.Errorf("data reached the destination before Flush: %q", buf.String())
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if want := "0\nSECTION\n2\nHEADER\n10\n1.5\n1\n\n"; buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestWriterOptions(t *testing.T) {
	ts := []tags.Tag{{0, "SECTION"}, {10, "1.5"}, {100, "AcDb"}, {1000, "x"}, {-5, "APP"}}
	tests := []struct {
		name string
		opts []tags.WriterOption
		want string
	}{
		{"default", nil, "0\nSECTION\n10\n1.5\n100\nAcDb\n1000\nx\n-5\nAPP\n"},
		{"CRLF", []tags.WriterOption{tags.WithCRLF()},
			"0\r\nSECTION\r\n10\r\n1.5\r\n100\r\nAcDb\r\n1000\r\nx\r\n-5\r\nAPP\r\n"},
		{"padded", []tags.WriterOption{tags.WithPaddedCodes()},
			"  0\nSECTION\n 10\n1.5\n100\nAcDb\n1000\nx\n -5\nAPP\n"},
		{"padded CRLF", []tags.WriterOption{tags.WithPaddedCodes(), tags.WithCRLF()},
			"  0\r\nSECTION\r\n 10\r\n1.5\r\n100\r\nAcDb\r\n1000\r\nx\r\n -5\r\nAPP\r\n"},
		{"nil option", []tags.WriterOption{nil}, "0\nSECTION\n10\n1.5\n100\nAcDb\n1000\nx\n-5\nAPP\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := writeString(t, ts, tc.opts...); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriterRejectsInvalidTags(t *testing.T) {
	tests := []struct {
		name string
		tag  tags.Tag
	}{
		{"code below range", tags.Tag{Code: -6, Value: "x"}},
		{"code above range", tags.Tag{Code: 1072, Value: "x"}},
		{"LF in value", tags.Tag{Code: 1, Value: "a\nb"}},
		{"CR in value", tags.Tag{Code: 1, Value: "a\rb"}},
		{"CRLF in value", tags.Tag{Code: 1, Value: "a\r\nb"}},
		{"trailing LF", tags.Tag{Code: 1, Value: "a\n"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := tags.NewWriter(&buf)
			if err := w.Write(tags.Tag{Code: 0, Value: "OK"}); err != nil {
				t.Fatal(err)
			}
			err := w.Write(tc.tag)
			if !errors.Is(err, tags.ErrSyntax) {
				t.Fatalf("err = %v, want ErrSyntax", err)
			}
			var se *tags.SyntaxError
			if !errors.As(err, &se) || se.Code != tc.tag.Code {
				t.Errorf("error = %#v, want SyntaxError for code %d", err, tc.tag.Code)
			}
			// The error is sticky: valid tags are refused with the same error.
			if err2 := w.Write(tags.Tag{Code: 0, Value: "AFTER"}); err2 != err {
				t.Errorf("second Write = %v, want the same error %v", err2, err)
			}
			if err2 := w.WriteAll([]tags.Tag{{Code: 0, Value: "AFTER"}}); err2 != err {
				t.Errorf("WriteAll = %v, want the same error", err2)
			}
			if err2 := w.Flush(); err2 != err {
				t.Errorf("Flush = %v, want the same error", err2)
			}
			if strings.Contains(buf.String(), "AFTER") {
				t.Errorf("a tag written after the error reached the output: %q", buf.String())
			}
		})
	}
}

func TestWriterBoundaryCodes(t *testing.T) {
	got := writeString(t, []tags.Tag{{-5, "a"}, {1071, "b"}})
	if want := "-5\na\n1071\nb\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriterWriteAllStopsAtFirstError(t *testing.T) {
	var buf bytes.Buffer
	w := tags.NewWriter(&buf)
	err := w.WriteAll([]tags.Tag{{0, "A"}, {9999, "bad"}, {2, "never"}})
	if !errors.Is(err, tags.ErrSyntax) {
		t.Fatalf("err = %v, want ErrSyntax", err)
	}
	if err := w.Flush(); err == nil {
		t.Fatal("Flush after error returned nil")
	}
	if strings.Contains(buf.String(), "never") {
		t.Errorf("tag after the error was written: %q", buf.String())
	}
}

func TestWriterWriteAllEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := tags.NewWriter(&buf)
	if err := w.WriteAll(nil); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil || buf.Len() != 0 {
		t.Errorf("Flush = %v, output %q", err, buf.String())
	}
}

func TestWriterFlushIdempotent(t *testing.T) {
	var buf bytes.Buffer
	w := tags.NewWriter(&buf)
	_ = w.Write(tags.Tag{Code: 0, Value: "A"})
	for i := 0; i < 3; i++ {
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	if buf.String() != "0\nA\n" {
		t.Errorf("got %q", buf.String())
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestWriterIOErrorIsSticky(t *testing.T) {
	boom := errors.New("disk full")

	t.Run("on Flush", func(t *testing.T) {
		w := tags.NewWriter(failingWriter{boom})
		if err := w.Write(tags.Tag{Code: 0, Value: "A"}); err != nil {
			t.Fatalf("small write must be buffered, got %v", err)
		}
		if err := w.Flush(); !errors.Is(err, boom) {
			t.Fatalf("Flush = %v, want boom", err)
		}
		if err := w.Write(tags.Tag{Code: 0, Value: "B"}); !errors.Is(err, boom) {
			t.Errorf("Write after failed Flush = %v, want boom", err)
		}
	})

	t.Run("on Write", func(t *testing.T) {
		w := tags.NewWriter(failingWriter{boom})
		big := tags.Tag{Code: 1, Value: strings.Repeat("x", 16<<10)}
		if err := w.Write(big); !errors.Is(err, boom) {
			t.Fatalf("Write = %v, want boom", err)
		}
		if err := w.Write(tags.Tag{Code: 0, Value: "B"}); !errors.Is(err, boom) {
			t.Errorf("second Write = %v, want boom", err)
		}
		if err := w.Flush(); !errors.Is(err, boom) {
			t.Errorf("Flush = %v, want boom", err)
		}
	})

	t.Run("on padding byte", func(t *testing.T) {
		w := tags.NewWriter(failingWriter{boom}, tags.WithPaddedCodes())
		// Fill the 4096-byte buffer exactly so the next byte forces a flush.
		if err := w.Write(tags.Tag{Code: 1, Value: strings.Repeat("x", 4096-5)}); err != nil {
			t.Fatalf("Write = %v", err)
		}
		if err := w.Write(tags.Tag{Code: 0, Value: "A"}); !errors.Is(err, boom) {
			t.Errorf("Write = %v, want boom", err)
		}
	})
}

func roundTripTags() []tags.Tag {
	return []tags.Tag{
		{0, "SECTION"}, {2, "HEADER"}, {9, "$ACADVER"}, {1, "AC1027"},
		{-5, "APP"}, {1071, "last"},
		{1, ""}, {1, "  leading and trailing  "}, {1, "àèìòù °±Ø€"},
		{10, "1.5"}, {20, "-2.25"}, {5, "1F"},
		{1, strings.Repeat("v", 5000)},
		{100, "AcDbEntity"}, {0, "ENDSEC"}, {0, "EOF"},
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		opts []tags.WriterOption
	}{
		{"default", nil},
		{"CRLF", []tags.WriterOption{tags.WithCRLF()}},
		{"padded", []tags.WriterOption{tags.WithPaddedCodes()}},
		{"padded CRLF", []tags.WriterOption{tags.WithPaddedCodes(), tags.WithCRLF()}},
	}
	want := roundTripTags()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := writeString(t, want, tc.opts...)
			r := tags.NewReader(strings.NewReader(text))
			got, err := readAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("round trip differs:\n got %d tags\nwant %d tags", len(got), len(want))
			}
			// Line numbers: every tag takes exactly two lines.
			r = tags.NewReader(strings.NewReader(text))
			for i := range want {
				if _, err := r.Next(); err != nil {
					t.Fatal(err)
				}
				if r.Line() != 2*i+1 {
					t.Fatalf("tag %d: Line = %d, want %d", i, r.Line(), 2*i+1)
				}
			}
		})
	}
}
