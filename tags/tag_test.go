package tags_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

func TestTagString(t *testing.T) {
	if got := (tags.Tag{Code: 10, Value: "1.5"}).String(); got != "10: 1.5" {
		t.Errorf("String = %q", got)
	}
}

func checkSyntaxErr(t *testing.T, err error, tag tags.Tag) {
	t.Helper()
	if !errors.Is(err, tags.ErrSyntax) {
		t.Fatalf("err = %v, want ErrSyntax", err)
	}
	var se *tags.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("err %T is not *SyntaxError", err)
	}
	if se.Code != tag.Code || se.Value != tag.Value {
		t.Errorf("error carries (%d, %q), want (%d, %q)", se.Code, se.Value, tag.Code, tag.Value)
	}
}

func TestTagInt(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"42", 42, true},
		{" 42 ", 42, true},
		{"-7", -7, true},
		{"0", 0, true},
		{"1.0", 1, true},
		{"-3.00", -3, true},
		{"1e2", 100, true},
		{"1.5", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{"NaN", 0, false},
		{"Inf", 0, false},
		{"1 2", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			tag := tags.Tag{Code: 70, Value: tc.in}
			got, err := tag.Int()
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("Int = %d, %v; want %d", got, err, tc.want)
				}
				return
			}
			checkSyntaxErr(t, err, tag)
		})
	}
}

func TestTagFloat(t *testing.T) {
	tests := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"1.5", 1.5, true},
		{" -2e3 ", -2000, true},
		{"3", 3, true},
		{".5", 0.5, true},
		{"0.0", 0, true},
		{"abc", 0, false},
		{"", 0, false},
		{"1,5", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			tag := tags.Tag{Code: 10, Value: tc.in}
			got, err := tag.Float()
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("Float = %v, %v; want %v", got, err, tc.want)
				}
				return
			}
			checkSyntaxErr(t, err, tag)
		})
	}
	if v, err := (tags.Tag{Value: "1e400"}).Float(); err == nil {
		t.Errorf("1e400 = %v, want an error (out of range)", v)
	} else if !math.IsInf(v, 0) && v != 0 {
		t.Errorf("unexpected value %v", v)
	}
}

func TestTagBool(t *testing.T) {
	tests := []struct {
		in   string
		want bool
		ok   bool
	}{
		{"0", false, true},
		{"1", true, true},
		{"-1", true, true},
		{"5", true, true},
		{" 1 ", true, true},
		{"0.0", false, true},
		{"x", false, false},
		{"", false, false},
		{"true", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			tag := tags.Tag{Code: 290, Value: tc.in}
			got, err := tag.Bool()
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("Bool = %v, %v; want %v", got, err, tc.want)
				}
				return
			}
			checkSyntaxErr(t, err, tag)
			var se *tags.SyntaxError
			errors.As(err, &se)
			if !strings.Contains(se.Reason, "boolean") {
				t.Errorf("Reason = %q, want it to mention boolean", se.Reason)
			}
		})
	}
}

func TestTagHex(t *testing.T) {
	tests := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"A", 10, true},
		{"ff", 255, true},
		{"1F", 31, true},
		{" 1F ", 31, true},
		{"0", 0, true},
		{"FFFFFFFFFFFFFFFF", math.MaxUint64, true},
		{"10000000000000000", 0, false},
		{"G", 0, false},
		{"", 0, false},
		{"-1", 0, false},
		{"0x1F", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			tag := tags.Tag{Code: 5, Value: tc.in}
			got, err := tag.Hex()
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("Hex = %d, %v; want %d", got, err, tc.want)
				}
				return
			}
			checkSyntaxErr(t, err, tag)
		})
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		code int
		want tags.Kind
	}{
		{-6, tags.KindUnknown},
		{-5, tags.KindString},
		{-1, tags.KindString},
		{0, tags.KindString},
		{1, tags.KindString},
		{5, tags.KindString},
		{9, tags.KindString},
		{10, tags.KindFloat},
		{40, tags.KindFloat},
		{59, tags.KindFloat},
		{60, tags.KindInt},
		{70, tags.KindInt},
		{79, tags.KindInt},
		{80, tags.KindUnknown},
		{89, tags.KindUnknown},
		{90, tags.KindInt},
		{99, tags.KindInt},
		{100, tags.KindString},
		{102, tags.KindString},
		{105, tags.KindHandle},
		{110, tags.KindFloat},
		{149, tags.KindFloat},
		{150, tags.KindUnknown},
		{170, tags.KindInt},
		{210, tags.KindFloat},
		{250, tags.KindUnknown},
		{280, tags.KindInt},
		{290, tags.KindBool},
		{299, tags.KindBool},
		{300, tags.KindString},
		{310, tags.KindBinary},
		{330, tags.KindHandle},
		{370, tags.KindInt},
		{390, tags.KindHandle},
		{420, tags.KindInt},
		{430, tags.KindString},
		{460, tags.KindFloat},
		{480, tags.KindHandle},
		{500, tags.KindUnknown},
		{999, tags.KindString},
		{1000, tags.KindString},
		{1010, tags.KindFloat},
		{1060, tags.KindInt},
		{1071, tags.KindInt},
		{1072, tags.KindUnknown},
	}
	for _, tc := range tests {
		if got := tags.KindOf(tc.code); got != tc.want {
			t.Errorf("KindOf(%d) = %v, want %v", tc.code, got, tc.want)
		}
		if got := (tags.Tag{Code: tc.code}).Kind(); got != tc.want {
			t.Errorf("Tag{%d}.Kind() = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestKindString(t *testing.T) {
	tests := map[tags.Kind]string{
		tags.KindUnknown: "unknown",
		tags.KindString:  "string",
		tags.KindFloat:   "float",
		tags.KindInt:     "int",
		tags.KindBool:    "bool",
		tags.KindHandle:  "handle",
		tags.KindBinary:  "binary",
		tags.Kind(99):    "unknown",
	}
	for k, want := range tests {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), got, want)
		}
	}
}

func TestIsValidCode(t *testing.T) {
	for code, want := range map[int]bool{-6: false, -5: true, 0: true, 1071: true, 1072: false, 5000: false} {
		if got := tags.IsValidCode(code); got != want {
			t.Errorf("IsValidCode(%d) = %v, want %v", code, got, want)
		}
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{&tags.SyntaxError{Reason: "bad"}, "dxf: bad"},
		{&tags.SyntaxError{Line: 3, Reason: "bad"}, "dxf: line 3: bad"},
		{&tags.SyntaxError{Line: 3, Section: "HEADER", Entity: "LINE", Code: 10, Value: "x", Reason: "bad"},
			`dxf: line 3: section HEADER: entity LINE: group code 10 "x": bad`},
		{&tags.LimitError{Kind: "value", Limit: 5}, "dxf: limit exceeded for value: maximum 5"},
		{&tags.LimitError{Kind: "value", Limit: 5, Line: 2}, "dxf: line 2: limit exceeded for value: maximum 5"},
	}
	for _, tc := range tests {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}
