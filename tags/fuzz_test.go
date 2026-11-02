package tags_test

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

// FuzzReader checks that the Reader never panics and always terminates, and
// that every tag it returns survives a trip through the Writer.
func FuzzReader(f *testing.F) {
	seeds := []string{
		"",
		"0\nSECTION\n2\nHEADER\n0\nENDSEC\n0\nEOF\n",
		"0\r\nSECTION\r\n2\r\nHEADER\r\n",
		"  0\nSECTION\n 10\n1.5\n",
		"\n\n0\nA\n\n \n2\nB",
		"abc\nx\n",
		"1072\nx\n",
		"-5\nAPP\n1071\nz\n",
		"0\n",
		"1\n\n",
		"1\nx\r\r\n",
		"1\na\rb\n",
		"1\nàèìòù €\n",
		"1\n\xff\xfe\x00\n",
		"99999999999999999999\nx\n",
		strings.Repeat("a", 300),
		"0\n" + strings.Repeat("b", 300) + "\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s), uint8(0))
		f.Add([]byte(s), uint8(7))
	}

	f.Fuzz(func(t *testing.T, data []byte, limit uint8) {
		maxValue := int(limit)%200 + 1
		r := tags.NewReader(bytes.NewReader(data), tags.WithMaxValueBytes(maxValue))

		var read []tags.Tag
		prevLine := 0
		// Each tag needs at least four bytes ("0\nx\n" or two newlines), so a
		// well behaved reader cannot return more than len(data)+1 results.
		for i := 0; ; i++ {
			if i > len(data)+1 {
				t.Fatalf("reader did not terminate after %d calls on %d bytes", i, len(data))
			}
			if i%3 == 0 {
				// Exercise Peek: it must agree with the following Next.
				pk, ok, perr := r.Peek()
				if perr == nil && ok {
					tag, err := r.Next()
					if err != nil || tag != pk {
						t.Fatalf("Peek returned %v but Next returned %v, %v", pk, tag, err)
					}
					read = append(read, tag)
					if r.Line() < prevLine || r.Line() < 1 {
						t.Fatalf("Line went from %d to %d", prevLine, r.Line())
					}
					prevLine = r.Line()
					continue
				}
				if perr == nil && !ok {
					if _, err := r.Next(); err != io.EOF {
						t.Fatalf("Peek reported EOF but Next returned %v", err)
					}
					break
				}
				var se *tags.SyntaxError
				var le *tags.LimitError
				if !errors.As(perr, &se) && !errors.As(perr, &le) {
					t.Fatalf("unexpected error type %T: %v", perr, perr)
				}
				break
			}
			tag, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				var se *tags.SyntaxError
				var le *tags.LimitError
				if !errors.As(err, &se) && !errors.As(err, &le) {
					t.Fatalf("unexpected error type %T: %v", err, err)
				}
				if !errors.Is(err, tags.ErrSyntax) && !errors.Is(err, tags.ErrLimit) {
					t.Fatalf("error %v matches neither sentinel", err)
				}
				break
			}
			read = append(read, tag)
			if r.Line() < prevLine || r.Line() < 1 {
				t.Fatalf("Line went from %d to %d", prevLine, r.Line())
			}
			prevLine = r.Line()
		}

		// Every tag read is valid for the writer, up to the CR normalisation:
		// the reader strips one trailing CR but keeps inner ones, which the
		// writer refuses.
		want := make([]tags.Tag, len(read))
		for i, tag := range read {
			if !tags.IsValidCode(tag.Code) || strings.Contains(tag.Value, "\n") || len(tag.Value) > maxValue {
				t.Fatalf("reader returned an invalid tag %v", tag)
			}
			want[i] = tags.Tag{Code: tag.Code, Value: strings.ReplaceAll(tag.Value, "\r", "")}
		}

		for _, opts := range [][]tags.WriterOption{
			nil,
			{tags.WithCRLF(), tags.WithPaddedCodes()},
		} {
			var buf bytes.Buffer
			w := tags.NewWriter(&buf, opts...)
			if err := w.WriteAll(want); err != nil {
				t.Fatalf("writer refused a tag the reader produced: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatal(err)
			}
			again, err := readAll(tags.NewReader(bytes.NewReader(buf.Bytes()), tags.WithMaxValueBytes(maxValue)))
			if err != nil {
				t.Fatalf("re-reading the written output: %v", err)
			}
			if !slices.Equal(again, want) {
				t.Fatalf("round trip differs:\n got %q\nwant %q", again, want)
			}
		}
	})
}
