package tags_test

import (
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

func TestEncodingFromCodePage(t *testing.T) {
	tests := []struct {
		name string
		want tags.Encoding
		ok   bool
	}{
		{"ANSI_1252", tags.EncodingCP1252, true},
		{"ansi_1252", tags.EncodingCP1252, true},
		{"  Ansi_1252 ", tags.EncodingCP1252, true},
		{"ISO8859-1", tags.EncodingLatin1, true},
		{"iso8859-1", tags.EncodingLatin1, true},
		{"ISO-8859-1", tags.EncodingLatin1, true},
		{"iso-8859-1", tags.EncodingLatin1, true},
		{"LATIN1", tags.EncodingLatin1, true},
		{"Latin1", tags.EncodingLatin1, true},
		{"UTF-8", tags.EncodingUTF8, true},
		{"utf-8", tags.EncodingUTF8, true},
		{"UTF8", tags.EncodingUTF8, true},
		{"utf8", tags.EncodingUTF8, true},
		{"", tags.EncodingUnknown, false},
		{"   ", tags.EncodingUnknown, false},
		{"ANSI_932", tags.EncodingUnknown, false},
		{"ANSI_1251", tags.EncodingUnknown, false},
		{"UTF-16", tags.EncodingUnknown, false},
		{"ISO8859-15", tags.EncodingUnknown, false},
	}
	for _, tc := range tests {
		got, ok := tags.EncodingFromCodePage(tc.name)
		if got != tc.want || ok != tc.ok {
			t.Errorf("EncodingFromCodePage(%q) = %v, %v; want %v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestEncodingString(t *testing.T) {
	for e, want := range map[tags.Encoding]string{
		tags.EncodingUTF8:    "UTF-8",
		tags.EncodingCP1252:  "CP1252",
		tags.EncodingLatin1:  "ISO-8859-1",
		tags.EncodingUnknown: "unknown",
	} {
		if got := e.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(e), got, want)
		}
	}
}

func TestEncodingRoundTripText(t *testing.T) {
	const text = "àèìòù °±Ø€"
	t.Run("CP1252", func(t *testing.T) {
		enc := tags.EncodingCP1252.Encode(text)
		want := "\xe0\xe8\xec\xf2\xf9 \xb0\xb1\xd8\x80"
		if enc != want {
			t.Fatalf("Encode = %q, want %q", enc, want)
		}
		if got := tags.EncodingCP1252.Decode(enc); got != text {
			t.Errorf("Decode = %q, want %q", got, text)
		}
	})
	t.Run("Latin1", func(t *testing.T) {
		enc := tags.EncodingLatin1.Encode(text)
		want := "\xe0\xe8\xec\xf2\xf9 \xb0\xb1\xd8" + `\U+20AC`
		if enc != want {
			t.Fatalf("Encode = %q, want %q", enc, want)
		}
		dec := tags.EncodingLatin1.Decode(enc)
		if dec != "àèìòù °±Ø"+`\U+20AC` {
			t.Errorf("Decode = %q: it must not touch escapes", dec)
		}
		if got := tags.DecodeEscapes(dec); got != text {
			t.Errorf("DecodeEscapes(Decode) = %q, want %q", got, text)
		}
	})
	t.Run("UTF8", func(t *testing.T) {
		if got := tags.EncodingUTF8.Encode(text); got != text {
			t.Errorf("Encode = %q", got)
		}
		if got := tags.EncodingUTF8.Decode(text); got != text {
			t.Errorf("Decode = %q", got)
		}
	})
	t.Run("Unknown", func(t *testing.T) {
		raw := "a\xe8b"
		if got := tags.EncodingUnknown.Decode(raw); got != raw {
			t.Errorf("Decode = %q", got)
		}
		if got := tags.EncodingUnknown.Encode(text); got != text {
			t.Errorf("Encode = %q", got)
		}
	})
}

func TestCP1252SpecialRange(t *testing.T) {
	want := "€\u0081‚ƒ„…†‡ˆ‰Š‹Œ\u008DŽ\u008F\u0090‘’“”•–—˜™š›œ\u009DžŸ"
	var in strings.Builder
	for b := 0x80; b <= 0x9F; b++ {
		in.WriteByte(byte(b))
	}
	if got := tags.EncodingCP1252.Decode(in.String()); got != want {
		t.Errorf("Decode(0x80..0x9F) = %+q, want %+q", got, want)
	}
	if got := tags.EncodingCP1252.Encode(want); got != in.String() {
		t.Errorf("Encode = %q, want %q", got, in.String())
	}
	// Latin1 maps the same bytes to C1 controls.
	for b := 0x80; b <= 0x9F; b++ {
		if got := tags.EncodingLatin1.Decode(string([]byte{byte(b)})); got != string(rune(b)) {
			t.Errorf("Latin1 Decode(%#x) = %+q", b, got)
		}
	}
}

func allBytes() string {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return string(b)
}

func TestAllBytesRoundTrip(t *testing.T) {
	// Every byte is representable in both encodings (the undefined CP1252
	// bytes decode to the C1 control of the same value), so nothing is excluded.
	in := allBytes()
	for _, e := range []tags.Encoding{tags.EncodingCP1252, tags.EncodingLatin1} {
		t.Run(e.String(), func(t *testing.T) {
			dec := e.Decode(in)
			if len([]rune(dec)) != 256 {
				t.Fatalf("Decode produced %d runes, want 256", len([]rune(dec)))
			}
			for i, r := range []rune(dec) {
				if i < 0x80 && r != rune(i) {
					t.Fatalf("ASCII byte %#x changed to %U", i, r)
				}
			}
			if got := e.Encode(dec); got != in {
				t.Errorf("Encode(Decode(all bytes)) differs from the input")
			}
		})
	}
}

func TestEncodeUnrepresentable(t *testing.T) {
	tests := []struct {
		enc  tags.Encoding
		in   string
		want string
	}{
		{tags.EncodingCP1252, "A€B", "A\x80B"},
		{tags.EncodingCP1252, "Ω", `\U+03A9`},
		{tags.EncodingCP1252, "日本", `\U+65E5\U+672C`},
		{tags.EncodingCP1252, "\u0080", `\U+0080`}, // not in CP1252
		{tags.EncodingCP1252, "\u0081", "\x81"},
		{tags.EncodingCP1252, "😀", `\U+D83D\U+DE00`},
		{tags.EncodingCP1252, "\ufffd", `\U+FFFD`},
		{tags.EncodingCP1252, "a\xffb", `a\U+FFFDb`}, // invalid UTF-8 input
		{tags.EncodingLatin1, "€", `\U+20AC`},
		{tags.EncodingLatin1, "Š", `\U+0160`},
		{tags.EncodingLatin1, "\u0080", "\x80"},
		{tags.EncodingLatin1, "ÿ", "\xff"},
		{tags.EncodingLatin1, "Ā", `\U+0100`},
		{tags.EncodingLatin1, "a\u0100b€", `a\U+0100b\U+20AC`},
		// Backslashes are never altered, even next to an escape-looking text.
		{tags.EncodingLatin1, `\U+0041`, `\U+0041`},
		{tags.EncodingLatin1, `C:\dir\€`, `C:\dir\` + `\U+20AC`},
		{tags.EncodingCP1252, `\`, `\`},
	}
	for _, tc := range tests {
		if got := tc.enc.Encode(tc.in); got != tc.want {
			t.Errorf("%v.Encode(%q) = %q, want %q", tc.enc, tc.in, got, tc.want)
		}
	}
}

func TestDecodeBytes(t *testing.T) {
	tests := []struct {
		enc  tags.Encoding
		in   string
		want string
	}{
		{tags.EncodingCP1252, "caff\xe8", "caffè"},
		{tags.EncodingLatin1, "caff\xe8", "caffè"},
		{tags.EncodingCP1252, "\x80 5", "€ 5"},
		{tags.EncodingLatin1, "\x80 5", "\u0080 5"},
		{tags.EncodingCP1252, `\U+20AC`, `\U+20AC`},
		{tags.EncodingCP1252, "a\\b\xb0", "a\\b°"},
	}
	for _, tc := range tests {
		if got := tc.enc.Decode(tc.in); got != tc.want {
			t.Errorf("%v.Decode(%q) = %q, want %q", tc.enc, tc.in, got, tc.want)
		}
	}
}

func TestDecodeUTF8InvalidBytes(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ok àè€", "ok àè€"},
		{"a\xffb", "a\ufffdb"},
		{"\xe8", "\ufffd"},
		{"caff\xe8 x", "caff\ufffd x"},
		{"\xff\xfe", "\ufffd\ufffd"},           // one replacement per byte
		{"\xe2\x82", "\ufffd\ufffd"},           // truncated sequence
		{"€\xc3", "€\ufffd"},                   // valid then truncated
		{"\xed\xa0\x80", "\ufffd\ufffd\ufffd"}, // encoded surrogate
	}
	for _, tc := range tests {
		if got := tags.EncodingUTF8.Decode(tc.in); got != tc.want {
			t.Errorf("Decode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNoAllocationsForASCII(t *testing.T) {
	// Built at run time so the compiler cannot treat it as a constant.
	s := strings.Repeat("LAYER_0 \\ plain ascii 123 ", 10)
	if got := testing.AllocsPerRun(100, func() {
		for _, e := range []tags.Encoding{tags.EncodingUTF8, tags.EncodingCP1252, tags.EncodingLatin1, tags.EncodingUnknown} {
			if e.Decode(s) != s || e.Encode(s) != s {
				t.Fatal("ASCII text changed")
			}
		}
	}); got != 0 {
		t.Errorf("Decode/Encode of ASCII allocate %v times, want 0", got)
	}
	if got := testing.AllocsPerRun(100, func() {
		if tags.EncodeEscapes(s) != s || tags.DecodeEscapes(s) != s {
			t.Fatal("ASCII text changed")
		}
	}); got != 0 {
		t.Errorf("EncodeEscapes/DecodeEscapes of ASCII allocate %v times, want 0", got)
	}
	valid := strings.Repeat("àè€", 10)
	if got := testing.AllocsPerRun(100, func() { _ = tags.EncodingUTF8.Decode(valid) }); got != 0 {
		t.Errorf("UTF-8 Decode of valid text allocates %v times, want 0", got)
	}
}

func TestDecodeEscapes(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"plain", "plain"},
		{`\U+20AC`, "€"},
		{`\U+20ac`, "€"},
		{`\U+00e8`, "è"},
		{`\U+00E8`, "è"},
		{`a\U+00E8b`, "aèb"},
		{`\U+0041\U+0042`, "AB"},
		{`\U+20AC5`, "€5"},      // only four digits are consumed
		{`\U+D83D\U+DE00`, "😀"}, // surrogate pair
		{`\U+d83d\U+de00!`, "😀!"},
		// malformed: unchanged
		{`\U+`, `\U+`},
		{`\U+1`, `\U+1`},
		{`\U+12`, `\U+12`},
		{`\U+123`, `\U+123`},
		{`\U+12G4`, `\U+12G4`},
		{`\U+ 123`, `\U+ 123`},
		{`\U-0041`, `\U-0041`},
		{`\u+0041`, `\u+0041`},
		{`\M+0041`, `\M+0041`},
		{`\`, `\`},
		{`abc\`, `abc\`},
		{`\\`, `\\`},
		{`\U+D800`, `\U+D800`},         // lone high surrogate
		{`\U+DC00`, `\U+DC00`},         // lone low surrogate
		{`\U+D83D\U+0041`, `\U+D83DA`}, // high surrogate not followed by a low one
		{`\U+D83D\U+12`, `\U+D83D\U+12`},
		{`x\U+zzzz\U+0041`, `x\U+zzzzA`},
		{`\\U+0041`, `\A`}, // there is no escaping of the backslash
	}
	for _, tc := range tests {
		if got := tags.DecodeEscapes(tc.in); got != tc.want {
			t.Errorf("DecodeEscapes(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEncodeEscapes(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"plain", "plain"},
		{"€", `\U+20AC`},
		{"è", `\U+00E8`},
		{"àè", `\U+00E0\U+00E8`},
		{"a°b", `a\U+00B0b`},
		{"😀", `\U+D83D\U+DE00`},
		{`C:\dir é`, `C:\dir \U+00E9`},
		{"a\xffb", `a\U+FFFDb`},
	}
	for _, tc := range tests {
		got := tags.EncodeEscapes(tc.in)
		if got != tc.want {
			t.Errorf("EncodeEscapes(%q) = %q, want %q", tc.in, got, tc.want)
		}
		for i := 0; i < len(got); i++ {
			if got[i] >= 0x80 {
				t.Errorf("EncodeEscapes(%q) = %q is not ASCII", tc.in, got)
				break
			}
		}
	}
}

func TestEscapesRoundTrip(t *testing.T) {
	for _, s := range []string{"", "ascii", "àèìòù °±Ø€", "日本語", "😀 emoji 𝄞", "Ω\u0000x", "\U0010FFFF", "\u007f\u0080\u00ff"} {
		if got := tags.DecodeEscapes(tags.EncodeEscapes(s)); got != s {
			t.Errorf("round trip of %q gave %q", s, got)
		}
	}
	for _, e := range []tags.Encoding{tags.EncodingCP1252, tags.EncodingLatin1} {
		for _, s := range []string{"àèìòù °±Ø€", "日本語 😀", "Ω é"} {
			if got := tags.DecodeEscapes(e.Decode(e.Encode(s))); got != s {
				t.Errorf("%v: round trip of %q gave %q", e, s, got)
			}
		}
	}
}
