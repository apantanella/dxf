package dxf

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// DecodeOption configures Decode.
type DecodeOption func(*decodeConfig)

type decodeConfig struct {
	lenient bool
	limits  Limits
}

// WithLenient makes Decode collect recoverable anomalies as Diagnostics instead
// of returning an error. Unsupported versions, missing EOF and malformed
// structural records become warnings.
func WithLenient() DecodeOption {
	return func(c *decodeConfig) { c.lenient = true }
}

// WithLimits replaces the default reading limits.
func WithLimits(limits Limits) DecodeOption {
	return func(c *decodeConfig) { c.limits = limits }
}

// Decode reads a DXF R12 ASCII stream into a Document.
//
// The decoder accepts LF and CRLF line endings, optional handles, and unknown
// entities, which are preserved as RawEntity. It rejects versions other than
// AC1009 unless WithLenient is used.
func Decode(r io.Reader, opts ...DecodeOption) (*Document, error) {
	if r == nil {
		return nil, &ValidationError{Reason: "nil reader"}
	}
	cfg := decodeConfig{limits: DefaultLimits()}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	cfg.limits = normalizeLimits(cfg.limits)
	p := &parser{gr: newGroupReader(r, cfg.limits), cfg: cfg}
	return p.decode()
}

func normalizeLimits(l Limits) Limits {
	def := DefaultLimits()
	if l.MaxEntities <= 0 {
		l.MaxEntities = def.MaxEntities
	}
	if l.MaxBlocks <= 0 {
		l.MaxBlocks = def.MaxBlocks
	}
	if l.MaxValueBytes <= 0 {
		l.MaxValueBytes = def.MaxValueBytes
	}
	if l.MaxNestingDepth <= 0 {
		l.MaxNestingDepth = def.MaxNestingDepth
	}
	return l
}

type parser struct {
	gr         *groupReader
	cfg        decodeConfig
	doc        *Document
	section    string
	sawEOF     bool
	sawSection bool
}

func (p *parser) decode() (*Document, error) {
	p.doc = &Document{Header: Header{Variables: map[string]Variable{}}}
	first := true
	for !p.sawEOF {
		grp, err := p.gr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if first && errors.Is(err, ErrSyntax) {
				return nil, ErrNotDXF
			}
			return nil, err
		}
		first = false
		if grp.Code != groupType {
			return nil, p.syntax(grp, fmt.Sprintf("expected group code 0, found %d", grp.Code))
		}
		switch strings.ToUpper(strings.TrimSpace(grp.Value)) {
		case "SECTION":
			if err := p.readSection(); err != nil {
				return nil, err
			}
		case "EOF":
			p.sawEOF = true
		default:
			return nil, p.syntax(grp, fmt.Sprintf("unexpected record %q outside a section", grp.Value))
		}
	}
	return p.finish()
}

func (p *parser) readSection() error {
	nameGroup, err := p.gr.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return p.syntaxLine(p.gr.lastLine, "", "", "end of stream after SECTION")
		}
		return err
	}
	if nameGroup.Code != groupName {
		return p.syntax(nameGroup, "expected group code 2 with the section name")
	}
	name := strings.ToUpper(strings.TrimSpace(nameGroup.Value))
	p.section = name
	p.sawSection = true
	defer func() { p.section = "" }()

	switch name {
	case "HEADER":
		return p.readHeader()
	case "TABLES":
		return p.readTables()
	case "BLOCKS":
		return p.readBlocks()
	case "ENTITIES":
		entities, err := p.readEntities("ENDSEC")
		if err != nil {
			return err
		}
		p.doc.Entities = entities
		return nil
	default:
		groups, err := p.readRawSection()
		if err != nil {
			return err
		}
		p.doc.UnknownSections = append(p.doc.UnknownSections, RawSection{Name: name, Groups: groups})
		p.addDiagnostic(SeverityInfo, p.gr.lastLine, name, "", "section not interpreted, preserved as raw data")
		return nil
	}
}

// readRawSection consumes groups until ENDSEC and returns them.
func (p *parser) readRawSection() ([]Group, error) {
	var groups []Group
	for {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.syntaxLine(p.gr.lastLine, p.section, "", "end of stream without ENDSEC")
		}
		if grp.Code == groupType && strings.EqualFold(strings.TrimSpace(grp.Value), "ENDSEC") {
			p.gr.next()
			return groups, nil
		}
		p.gr.next()
		groups = append(groups, grp)
	}
}

func (p *parser) readHeader() error {
	for {
		grp, err := p.gr.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return p.syntaxLine(p.gr.lastLine, "HEADER", "", "end of stream without ENDSEC")
			}
			return err
		}
		if grp.Code == groupType && strings.EqualFold(strings.TrimSpace(grp.Value), "ENDSEC") {
			return nil
		}
		if grp.Code != groupVariable {
			return p.syntax(grp, "expected group code 9 with the header variable name")
		}
		name := strings.ToUpper(strings.TrimSpace(grp.Value))
		var values []Group
		for {
			next, ok, err := p.gr.peek()
			if err != nil {
				return err
			}
			if !ok {
				return p.syntaxLine(p.gr.lastLine, "HEADER", "", "end of stream inside a header variable")
			}
			if next.Code == groupVariable || next.Code == groupType {
				break
			}
			p.gr.next()
			values = append(values, next)
		}
		p.doc.Header.Variables[name] = Variable{Name: name, Groups: values}
	}
}

func (p *parser) readTables() error {
	for {
		grp, err := p.gr.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return p.syntaxLine(p.gr.lastLine, "TABLES", "", "end of stream without ENDSEC")
			}
			return err
		}
		if grp.Code != groupType {
			return p.syntax(grp, "expected group code 0 with the table name")
		}
		name := strings.ToUpper(strings.TrimSpace(grp.Value))
		if name == "ENDSEC" {
			return nil
		}
		header, err := p.readGroupList()
		if err != nil {
			return err
		}
		tableName := name
		if value, ok := rawValue(header, groupName); ok {
			tableName = strings.ToUpper(value)
		}
		table, err := p.readTable(tableName, header)
		if err != nil {
			return err
		}
		p.doc.Tables.Items = append(p.doc.Tables.Items, table)
	}
}

func (p *parser) readTable(name string, header []Group) (*Table, error) {
	table := &Table{Name: name}
	headerSet := newGroupSet(header)
	headerSet.str(groupName)
	table.Raw = headerSet.leftovers()
	for {
		grp, err := p.gr.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, p.syntaxLine(p.gr.lastLine, "TABLES", name, "end of stream without ENDTAB")
			}
			return nil, err
		}
		if grp.Code != groupType {
			return nil, p.syntax(grp, "expected group code 0 inside a table")
		}
		recordName := strings.ToUpper(strings.TrimSpace(grp.Value))
		if recordName == "ENDTAB" {
			return table, nil
		}
		if recordName == "ENDSEC" {
			if p.cfg.lenient {
				p.addDiagnostic(SeverityWarning, p.gr.lastLine, "TABLES", name, "table without ENDTAB")
				return table, nil
			}
			return nil, p.syntax(grp, "table without ENDTAB")
		}
		groups, err := p.readGroupList()
		if err != nil {
			return nil, err
		}
		table.Records = append(table.Records, Record{TypeName: recordName, Groups: groups})
	}
}

func (p *parser) readBlocks() error {
	for {
		grp, err := p.gr.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return p.syntaxLine(p.gr.lastLine, "BLOCKS", "", "end of stream without ENDSEC")
			}
			return err
		}
		if grp.Code != groupType {
			return p.syntax(grp, "expected group code 0 in the BLOCKS section")
		}
		name := strings.ToUpper(strings.TrimSpace(grp.Value))
		if name == "ENDSEC" {
			return nil
		}
		if name != "BLOCK" {
			return p.syntax(grp, fmt.Sprintf("expected BLOCK, found %q", grp.Value))
		}
		block, err := p.readBlock()
		if err != nil {
			return err
		}
		p.doc.Blocks = append(p.doc.Blocks, block)
		if len(p.doc.Blocks) > p.cfg.limits.MaxBlocks {
			return &LimitError{Kind: "blocks", Limit: p.cfg.limits.MaxBlocks, Line: p.gr.lastLine}
		}
	}
}

func (p *parser) readBlock() (*Block, error) {
	block := &Block{}
	for {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.syntaxLine(p.gr.lastLine, "BLOCKS", "BLOCK", "end of stream inside a block definition")
		}
		if grp.Code == groupType {
			break
		}
		p.gr.next()
		block.Raw = append(block.Raw, grp)
	}
	gs := newGroupSet(block.Raw)
	if name, ok := gs.str(groupName); ok {
		block.Name = name
	}
	if layer, ok := gs.str(groupLayer); ok {
		block.LayerName = layer
	}
	// Code 3 repeats the block name: the writer regenerates it.
	gs.str(groupName2)
	if v, ok, err := gs.float(groupX); err != nil {
		return nil, err
	} else if ok {
		block.BasePoint.X = v
	}
	if v, ok, err := gs.float(groupY); err != nil {
		return nil, err
	} else if ok {
		block.BasePoint.Y = v
	}
	if v, ok, err := gs.float(groupZ); err != nil {
		return nil, err
	} else if ok {
		block.BasePoint.Z = v
	}
	if v, ok, err := gs.int(groupFlags); err != nil {
		return nil, err
	} else if ok {
		block.Flags = v
	}
	block.Raw = gs.leftovers()
	entities, err := p.readEntities("ENDBLK")
	if err != nil {
		return nil, err
	}
	block.Entities = entities
	return block, nil
}

func (p *parser) readEntities(stop string) ([]Entity, error) {
	var out []Entity
	for {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.syntaxLine(p.gr.lastLine, p.section, "", fmt.Sprintf("end of stream without %s", stop))
		}
		if grp.Code == groupType && strings.EqualFold(strings.TrimSpace(grp.Value), stop) {
			p.gr.next()
			return out, nil
		}
		if grp.Code != groupType {
			p.gr.next()
			return nil, p.syntax(grp, "expected a new entity (group code 0)")
		}
		p.gr.next()
		typeName := strings.TrimSpace(grp.Value)
		groups, err := p.readGroupList()
		if err != nil {
			return nil, err
		}
		entity, err := p.decodeOne(typeName, groups)
		if err != nil {
			return nil, err
		}
		if entity != nil {
			out = append(out, entity)
		}
		if len(out) > p.cfg.limits.MaxEntities {
			return nil, &LimitError{Kind: "entities", Limit: p.cfg.limits.MaxEntities, Line: p.gr.lastLine}
		}
	}
}

// readGroupList consumes groups until the next code 0 record.
func (p *parser) readGroupList() ([]Group, error) {
	var groups []Group
	for {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok || grp.Code == groupType {
			return groups, nil
		}
		p.gr.next()
		groups = append(groups, grp)
	}
}

func (p *parser) decodeOne(typeName string, groups []Group) (Entity, error) {
	switch strings.ToUpper(strings.TrimSpace(typeName)) {
	case "POLYLINE":
		return p.readPolyline(groups)
	case "INSERT":
		return p.readInsert(groups)
	case "VERTEX":
		if !p.cfg.lenient {
			return nil, &SyntaxError{Line: p.gr.lastLine, Section: p.section, Entity: typeName, Reason: "VERTEX outside a POLYLINE"}
		}
		p.addDiagnostic(SeverityWarning, p.gr.lastLine, p.section, typeName, "VERTEX outside a POLYLINE")
	case "SEQEND":
		if !p.cfg.lenient {
			return nil, &SyntaxError{Line: p.gr.lastLine, Section: p.section, Entity: typeName, Reason: "unexpected SEQEND"}
		}
		p.addDiagnostic(SeverityWarning, p.gr.lastLine, p.section, typeName, "unexpected SEQEND")
	}
	return decodeEntity(typeName, groups)
}

func (p *parser) readPolyline(groups []Group) (Entity, error) {
	var vertices []Vertex
	for {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.syntaxLine(p.gr.lastLine, p.section, "POLYLINE", "POLYLINE without SEQEND")
		}
		if grp.Code != groupType {
			p.gr.next()
			return nil, p.syntax(grp, "expected VERTEX or SEQEND inside a POLYLINE")
		}
		switch strings.ToUpper(strings.TrimSpace(grp.Value)) {
		case "VERTEX":
			p.gr.next()
			vertexGroups, err := p.readGroupList()
			if err != nil {
				return nil, err
			}
			vertex, err := decodeVertex(vertexGroups)
			if err != nil {
				return nil, err
			}
			vertices = append(vertices, vertex)
		case "SEQEND":
			p.gr.next()
			if _, err := p.readGroupList(); err != nil {
				return nil, err
			}
			return decodePolyline("POLYLINE", groups, vertices)
		default:
			if !p.cfg.lenient {
				return nil, &SyntaxError{Line: p.gr.lastLine, Section: p.section, Entity: "POLYLINE", Reason: "POLYLINE without SEQEND"}
			}
			p.addDiagnostic(SeverityWarning, p.gr.lastLine, p.section, "POLYLINE", "POLYLINE without SEQEND")
			return decodePolyline("POLYLINE", groups, vertices)
		}
	}
}

func (p *parser) readInsert(groups []Group) (Entity, error) {
	var attributes []Attribute
	for {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if grp.Code != groupType || !strings.EqualFold(strings.TrimSpace(grp.Value), "ATTRIB") {
			break
		}
		p.gr.next()
		attributeGroups, err := p.readGroupList()
		if err != nil {
			return nil, err
		}
		attribute, err := decodeAttribute(attributeGroups)
		if err != nil {
			return nil, err
		}
		attributes = append(attributes, attribute)
	}
	if len(attributes) > 0 {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok || grp.Code != groupType || !strings.EqualFold(strings.TrimSpace(grp.Value), "SEQEND") {
			if !p.cfg.lenient {
				return nil, &SyntaxError{Line: p.gr.lastLine, Section: p.section, Entity: "INSERT", Reason: "INSERT with ATTRIB but without SEQEND"}
			}
			p.addDiagnostic(SeverityWarning, p.gr.lastLine, p.section, "INSERT", "INSERT with ATTRIB but without SEQEND")
		} else {
			p.gr.next()
			if _, err := p.readGroupList(); err != nil {
				return nil, err
			}
		}
	}
	gs := newGroupSet(groups)
	common, err := gs.common()
	if err != nil {
		return nil, err
	}
	return decodeInsert(gs, common, attributes)
}

func (p *parser) finish() (*Document, error) {
	if !p.sawSection {
		return nil, ErrNotDXF
	}
	if n := p.gr.nonASCII; n > 0 {
		p.addDiagnostic(SeverityWarning, p.gr.firstNonASCII, "", "", fmt.Sprintf(
			"%d group values contain non-ASCII bytes: R12 text depends on the code page, the bytes are preserved unchanged", n))
	}
	if !p.sawEOF {
		if p.cfg.lenient {
			p.addDiagnostic(SeverityWarning, p.gr.lastLine, "", "", "missing EOF terminator")
		} else {
			return nil, &SyntaxError{Line: p.gr.lastLine, Reason: "missing EOF terminator"}
		}
	}
	version, ok := p.doc.Header.Get("$ACADVER")
	switch {
	case !ok:
		if p.cfg.lenient {
			p.addDiagnostic(SeverityWarning, 0, "HEADER", "", "missing $ACADVER variable")
			p.doc.Version = VersionR12
		} else {
			return nil, &SyntaxError{Section: "HEADER", Reason: "missing $ACADVER variable"}
		}
	case strings.EqualFold(version, VersionR12):
		p.doc.Version = VersionR12
	default:
		if p.cfg.lenient {
			p.addDiagnostic(SeverityWarning, 0, "HEADER", "", fmt.Sprintf("unsupported version %q", version))
			p.doc.Version = version
		} else {
			return nil, &VersionError{Found: version}
		}
	}
	return p.doc, nil
}

func (p *parser) syntax(group Group, reason string) error {
	return &SyntaxError{
		Line:    p.gr.lastLine,
		Section: p.section,
		Code:    group.Code,
		Value:   group.Value,
		Reason:  reason,
	}
}

func (p *parser) syntaxLine(line int, section, entity, reason string) error {
	return &SyntaxError{Line: line, Section: section, Entity: entity, Reason: reason}
}

func (p *parser) addDiagnostic(severity Severity, line int, section, entity, message string) {
	p.doc.Diagnostics = append(p.doc.Diagnostics, Diagnostic{
		Severity: severity,
		Line:     line,
		Section:  section,
		Entity:   entity,
		Message:  message,
	})
}

func rawValue(groups []Group, code int) (string, bool) {
	for _, group := range groups {
		if group.Code == code {
			return strings.TrimSpace(group.Value), true
		}
	}
	return "", false
}
