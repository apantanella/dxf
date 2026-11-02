package dxf

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

// t9Record is one record (a 0 group and what follows up to the next 0) of a
// written file.
type t9Record struct {
	section string
	typ     string
	handle  string // first 5 (105 for DIMSTYLE) before any extended data
	owner   string // first 330 outside 102 groups
	name    string // first 2
	tags    []tags.Tag
}

func t9Records(ts []tags.Tag) []t9Record {
	var out []t9Record
	section := ""
	for i := 0; i < len(ts); i++ {
		tg := ts[i]
		if tg.Code == 0 && tg.Value == "SECTION" && i+1 < len(ts) {
			section = ts[i+1].Value
			continue
		}
		if tg.Code != 0 || tg.Value == "ENDSEC" || tg.Value == "EOF" || tg.Value == "ENDTAB" || tg.Value == "SECTION" {
			continue
		}
		r := t9Record{section: section, typ: tg.Value}
		depth := 0
		j := i + 1
		for ; j < len(ts) && ts[j].Code != 0; j++ {
			g := ts[j]
			r.tags = append(r.tags, g)
			switch {
			case g.Code == 102 && strings.HasPrefix(g.Value, "{"):
				depth++
			case g.Code == 102 && g.Value == "}":
				depth--
			case g.Code == 330 && depth == 0 && r.owner == "":
				r.owner = g.Value
			case (g.Code == 5 || g.Code == 105) && r.handle == "":
				r.handle = g.Value
			case g.Code == 2 && r.name == "":
				r.name = g.Value
			}
		}
		out = append(out, r)
		i = j - 1
	}
	return out
}

// t9CheckStructure verifies the handle relations of a written R2000+ file:
// unique handles, every reference (320-369, 390-399) resolves, and the owner
// rules of docs/dxf-r2013.md. It returns the list of problems.
func t9CheckStructure(ts []tags.Tag) []string {
	var bad []string
	recs := t9Records(ts)
	byHandle := map[string]*t9Record{}
	for i := range recs {
		r := &recs[i]
		if r.handle == "" {
			// Records without a handle: TABLE headers and CLASS have one or none.
			continue
		}
		key := strings.ToUpper(strings.TrimLeft(r.handle, "0"))
		if prev, dup := byHandle[key]; dup {
			bad = append(bad, fmt.Sprintf("duplicate handle %s: %s and %s", r.handle, prev.typ, r.typ))
		}
		byHandle[key] = r
	}
	find := func(h string) *t9Record {
		return byHandle[strings.ToUpper(strings.TrimLeft(strings.TrimSpace(h), "0"))]
	}
	// References.
	for _, r := range recs {
		for _, g := range r.tags {
			if g.Code >= 1000 {
				break
			}
			isRef := (g.Code >= 320 && g.Code <= 369) || (g.Code >= 390 && g.Code <= 399)
			if !isRef || strings.Trim(g.Value, "0 ") == "" {
				continue
			}
			if find(g.Value) == nil {
				bad = append(bad, fmt.Sprintf("%s %q: reference %d=%s does not resolve", r.typ, r.handle, g.Code, g.Value))
			}
		}
	}
	// Owners.
	for _, r := range recs {
		o := find(r.owner)
		switch r.section {
		case "BLOCKS":
			switch r.typ {
			case "BLOCK", "ENDBLK":
				if o == nil || o.typ != "BLOCK_RECORD" {
					bad = append(bad, fmt.Sprintf("%s %s: owner %s is not a BLOCK_RECORD", r.typ, r.name, r.owner))
				} else if r.typ == "BLOCK" && o.name != r.name {
					bad = append(bad, fmt.Sprintf("BLOCK %s: owner is the BLOCK_RECORD %s", r.name, o.name))
				}
			case "VERTEX", "ATTRIB", "SEQEND":
				if o == nil || (o.typ != "POLYLINE" && o.typ != "INSERT" && o.typ != "BLOCK_RECORD") {
					bad = append(bad, fmt.Sprintf("%s %s: owner %s", r.typ, r.handle, r.owner))
				}
			default:
				if o == nil || o.typ != "BLOCK_RECORD" {
					bad = append(bad, fmt.Sprintf("%s %s in a block: owner %s is not a BLOCK_RECORD", r.typ, r.handle, r.owner))
				}
			}
		case "ENTITIES":
			switch r.typ {
			case "VERTEX", "ATTRIB", "SEQEND":
				if o == nil || (o.typ != "POLYLINE" && o.typ != "INSERT" && o.typ != "BLOCK_RECORD") {
					bad = append(bad, fmt.Sprintf("%s %s: owner %s", r.typ, r.handle, r.owner))
				}
			default:
				if o == nil || o.typ != "BLOCK_RECORD" {
					bad = append(bad, fmt.Sprintf("%s %s: owner %s is not a BLOCK_RECORD", r.typ, r.handle, r.owner))
				}
			}
		case "TABLES":
			if r.typ != "TABLE" && r.owner != "" {
				if o == nil {
					bad = append(bad, fmt.Sprintf("table record %s %s: owner %s unknown", r.typ, r.name, r.owner))
				}
			}
		}
	}
	return bad
}

// TestR11ConvertR12ToR2013 converts every R12 fixture: the result reads back
// without warnings, the handles are valid and unique, the owners are coherent,
// and the entities are the same.
func TestR11ConvertR12ToR2013(t *testing.T) {
	for _, fixture := range validFixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			src := decodeFixture(t, fixture.name)
			for _, version := range []string{VersionR2013, VersionR2000, VersionR2007, VersionR2018} {
				out := t9Encode(t, src, WithVersion(version))
				doc := t9Decode(t, out)
				if doc.Version != version {
					t.Errorf("%s: Version = %q", version, doc.Version)
				}
				for _, d := range doc.Diagnostics {
					if d.Severity >= SeverityWarning {
						t.Errorf("%s: diagnostic %v", version, d)
					}
				}
				if d := doc.ValidateHandles(); len(d) != 0 {
					t.Errorf("%s: ValidateHandles: %v", version, d)
				}
				for _, p := range t9CheckStructure(t9Tags(t, out)) {
					t.Errorf("%s: %s", version, p)
				}
				if len(doc.Entities) != len(src.Entities) {
					t.Fatalf("%s: %d entities, want %d", version, len(doc.Entities), len(src.Entities))
				}
				for i := range doc.Entities {
					if doc.Entities[i].Type() != src.Entities[i].Type() {
						t.Errorf("%s: entity %d is %s, want %s", version, i, doc.Entities[i].Type(), src.Entities[i].Type())
					}
				}
				// The converted document is stable: a second pass changes nothing.
				if again := t9Encode(t, doc); string(again) != string(out) {
					t.Errorf("%s: second write differs: %s", version, t9DiffTags(t9Tags(t, out), t9Tags(t, again)))
				}
			}
			// The source document was not modified by the conversions.
			if fresh := decodeFixture(t, fixture.name); !reflect.DeepEqual(fresh, src) {
				t.Error("Encode modified the document")
			}
		})
	}
}

// TestR11StructureOfWrittenFiles checks the handle relations of the files
// written for the sample document and the entity cases.
func TestR11StructureOfWrittenFiles(t *testing.T) {
	cases := map[string]*Document{"sample": sampleDocumentR2013(), "empty": NewDocumentVersion(VersionR2013)}
	for _, tc := range t9EntityCases() {
		cases[tc.name] = tc.doc()
	}
	for name, doc := range cases {
		name, doc := name, doc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, p := range t9CheckStructure(t9Tags(t, t9Encode(t, doc))) {
				t.Error(p)
			}
		})
	}
	for _, path := range t9Fixtures(t) {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			doc := t9Decode(t, t9ReadFile(t, path))
			for _, p := range t9CheckStructure(t9Tags(t, t9Encode(t, doc))) {
				t.Error(p)
			}
		})
	}
}
