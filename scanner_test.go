package dxf

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestGroupReaderNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []Group
	}{
		{
			name:  "LF",
			input: "0\nSECTION\n2\nHEADER\n",
			want:  []Group{{Code: 0, Value: "SECTION"}, {Code: 2, Value: "HEADER"}},
		},
		{
			name:  "CRLF",
			input: "0\r\nSECTION\r\n2\r\nHEADER\r\n",
			want:  []Group{{Code: 0, Value: "SECTION"}, {Code: 2, Value: "HEADER"}},
		},
		{
			name:  "no trailing newline",
			input: "0\nEOF",
			want:  []Group{{Code: 0, Value: "EOF"}},
		},
		{
			name:  "padded group code",
			input: "  0\nSECTION\n",
			want:  []Group{{Code: 0, Value: "SECTION"}},
		},
		{
			name:  "blank lines ignored",
			input: "\n0\nSECTION\n\n",
			want:  []Group{{Code: 0, Value: "SECTION"}},
		},
		{
			name:  "negative group code",
			input: "-1\nENTITY\n",
			want:  []Group{{Code: -1, Value: "ENTITY"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader := newGroupReader(strings.NewReader(tt.input), DefaultLimits())
			for i, want := range tt.want {
				got, err := reader.next()
				if err != nil {
					t.Fatalf("next() #%d: unexpected error: %v", i, err)
				}
				if got != want {
					t.Errorf("next() #%d = %+v, want %+v", i, got, want)
				}
			}
			if _, err := reader.next(); !errors.Is(err, io.EOF) {
				t.Errorf("final next() = %v, want io.EOF", err)
			}
		})
	}
}

func TestGroupReaderPeek(t *testing.T) {
	t.Parallel()

	reader := newGroupReader(strings.NewReader("0\nSECTION\n"), DefaultLimits())
	first, ok, err := reader.peek()
	if err != nil || !ok {
		t.Fatalf("peek() = %v, %v, %v", first, ok, err)
	}
	again, ok, err := reader.peek()
	if err != nil || !ok || again != first {
		t.Fatalf("repeated peek() = %v, %v, %v", again, ok, err)
	}
	got, err := reader.next()
	if err != nil || got != first {
		t.Fatalf("next() after peek = %v, %v", got, err)
	}
	if _, ok, err := reader.peek(); err != nil || ok {
		t.Fatalf("peek() at end of stream = ok %v, err %v", ok, err)
	}
}

func TestGroupReaderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		limits  Limits
		wantErr error
	}{
		{
			name:    "non-numeric group code",
			input:   "abc\n0\n",
			limits:  DefaultLimits(),
			wantErr: ErrSyntax,
		},
		{
			name:    "group code out of range",
			input:   "9999\n0\n",
			limits:  DefaultLimits(),
			wantErr: ErrSyntax,
		},
		{
			name:    "incomplete pair",
			input:   "0\n",
			limits:  DefaultLimits(),
			wantErr: ErrSyntax,
		},
		{
			name:    "value over the limit",
			input:   "0\nVALUE_TOO_LONG\n",
			limits:  Limits{MaxEntities: 1, MaxBlocks: 1, MaxValueBytes: 4, MaxNestingDepth: 1},
			wantErr: ErrLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader := newGroupReader(strings.NewReader(tt.input), tt.limits)
			_, err := reader.next()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("next() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupValueKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want GroupKind
	}{
		{code: 0, want: GroupKindString},
		{code: 10, want: GroupKindFloat},
		{code: 62, want: GroupKindInt},
		{code: 70, want: GroupKindInt},
		{code: 105, want: GroupKindHandle},
		{code: 210, want: GroupKindFloat},
		{code: 310, want: GroupKindBinary},
		{code: 330, want: GroupKindHandle},
		{code: 1001, want: GroupKindString},
		{code: 1071, want: GroupKindInt},
	}

	for _, tt := range tests {
		if got := KindOf(tt.code); got != tt.want {
			t.Errorf("KindOf(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}
