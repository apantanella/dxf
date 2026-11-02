package dxf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// binarySignature is the beginning of a binary DXF file: the ASCII text
// "AutoCAD Binary DXF" followed by CR LF, 0x1A and NUL. Only the text is
// compared, because it is unambiguous on its own. The trailing bytes are
// documented in the ADR 0008 and not yet checked against a real file.
var binarySignature = []byte("AutoCAD Binary DXF")

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

// Decode reads a DXF ASCII stream, R12 to R2018, into a Document.
//
// The decoder accepts LF and CRLF line endings, optional handles, and unknown
// entities and objects, which are preserved as RawEntity and RawObject. It
// rejects a $ACADVER value other than the nine supported ones (VersionR12 to
// VersionR2018) with a *VersionError unless WithLenient is used, which turns
// the error into a warning. R13 and R14 are read on a best-effort basis.
//
// From R13 on the subclass markers (group code 100), the 102 { ... } groups,
// the owner (330) and the extended data (1001 and following) are kept: see
// EntityCommon. A binary DXF file is not supported: Decode returns an error
// that satisfies errors.Is(err, ErrNotDXF) and says so.
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
	br := bufio.NewReader(r)
	if head, _ := br.Peek(len(binarySignature)); bytes.Equal(head, binarySignature) {
		return nil, fmt.Errorf("%w: the file is binary DXF (it starts with %q), which is not supported; save it as ASCII DXF",
			ErrNotDXF, binarySignature)
	}
	p := &parser{gr: newGroupReader(br, cfg.limits), cfg: cfg}
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
	// modern is true when $ACADVER, read from the HEADER, is R13 or later:
	// records then carry subclass markers and owners. A record that has markers
	// is read as such even if the HEADER is missing.
	modern bool
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
		if err := p.readHeader(); err != nil {
			return err
		}
		if v, ok := p.doc.Header.Get("$ACADVER"); ok {
			p.modern = isModernVersion(v)
		}
		return nil
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
	case "OBJECTS":
		objects, err := p.readObjects()
		if err != nil {
			return err
		}
		p.doc.Objects = append(p.doc.Objects, objects...)
		return nil
	case "CLASSES":
		groups, err := p.readRawSection()
		if err != nil {
			return err
		}
		if classes, ok := classesFromGroups(groups); ok {
			p.doc.Classes = append(p.doc.Classes, classes...)
			return nil
		}
		// Not a plain list of CLASS records: keep the section as it is.
		p.doc.UnknownSections = append(p.doc.UnknownSections, RawSection{Name: name, Groups: groups})
		p.addDiagnostic(SeverityInfo, p.gr.lastLine, name, "", "section not interpreted, preserved as raw data")
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

// readObjects reads the OBJECTS section. DICTIONARY, ACDBDICTIONARYWDFLT,
// ACDBPLACEHOLDER and LAYOUT records become typed objects, every other record a
// RawObject.
func (p *parser) readObjects() ([]Object, error) {
	var out []Object
	for {
		grp, ok, err := p.gr.peek()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, p.syntaxLine(p.gr.lastLine, "OBJECTS", "", "end of stream without ENDSEC")
		}
		if grp.Code != groupType {
			p.gr.next()
			return nil, p.syntax(grp, "expected a new object (group code 0)")
		}
		p.gr.next()
		typeName := strings.TrimSpace(grp.Value)
		if strings.EqualFold(typeName, "ENDSEC") {
			return out, nil
		}
		groups, err := p.readGroupList()
		if err != nil {
			return nil, err
		}
		out = append(out, interpretObject(typeName, groups))
		if len(out) > p.cfg.limits.MaxEntities {
			return nil, &LimitError{Kind: "objects", Limit: p.cfg.limits.MaxEntities, Line: p.gr.lastLine}
		}
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
			p.doc.Tables.LoadBlockRecords()
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
	gs := newGroupSetMode(block.Raw, p.modern)
	block.Subclasses = gs.names
	gs.prefer(prefHeader)
	if h, ok := gs.owner(); ok {
		block.Owner = h
	}
	gs.prefer(prefBlockBegin)
	if name, ok := gs.str(groupName); ok {
		block.Name = name
	}
	gs.prefer(prefEntity)
	if layer, ok := gs.str(groupLayer); ok {
		block.LayerName = layer
	}
	gs.prefer(prefBlockBegin)
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
	block.RawSubclass = gs.leftoverSubclasses()
	entities, err := p.readEntities("ENDBLK")
	if err != nil {
		return nil, err
	}
	block.Entities = entities
	// ENDBLK may carry a handle; any other group is still a syntax error.
	end, err := p.readGroupList()
	if err != nil {
		return nil, err
	}
	if p.modern || hasSubclassMarkers(end) {
		// R13 and later: ENDBLK has an owner, a layer and subclass markers.
		// The handle goes to EndHandle, the rest is kept as it is.
		block.EndHandle, block.EndGroups = splitHandle(end)
		return block, nil
	}
	es := newGroupSet(end)
	if v, ok := es.str(groupHandle); ok {
		block.EndHandle = v
	}
	if rest := es.leftovers(); len(rest) > 0 {
		return nil, p.syntax(rest[0], "expected group code 0 in the BLOCKS section")
	}
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
	return decodeEntityMode(typeName, groups, p.modern)
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
			vertex, err := decodeVertexMode(vertexGroups, p.modern)
			if err != nil {
				return nil, err
			}
			vertices = append(vertices, vertex)
		case "SEQEND":
			p.gr.next()
			seqend, err := p.readGroupList()
			if err != nil {
				return nil, err
			}
			handle, rest := p.seqendParts(seqend)
			return decodePolyline(groups, vertices, handle, rest, p.modern)
		default:
			if !p.cfg.lenient {
				return nil, &SyntaxError{Line: p.gr.lastLine, Section: p.section, Entity: "POLYLINE", Reason: "POLYLINE without SEQEND"}
			}
			p.addDiagnostic(SeverityWarning, p.gr.lastLine, p.section, "POLYLINE", "POLYLINE without SEQEND")
			return decodePolyline(groups, vertices, "", nil, p.modern)
		}
	}
}

func (p *parser) readInsert(groups []Group) (Entity, error) {
	var attributes []Attribute
	var seqend string
	var seqendGroups []Group
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
		attribute, err := decodeAttribute(attributeGroups, p.modern)
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
			end, err := p.readGroupList()
			if err != nil {
				return nil, err
			}
			seqend, seqendGroups = p.seqendParts(end)
		}
	}
	gs := newGroupSetMode(groups, p.modern)
	common, err := gs.common()
	if err != nil {
		return nil, err
	}
	entity, err := decodeInsert(gs, common, attributes)
	if err != nil {
		return nil, err
	}
	entity.(*Insert).SeqendHandle = seqend
	entity.(*Insert).SeqendGroups = seqendGroups
	return entity, nil
}

// seqendHandle returns the handle (group code 5) of a SEQEND record.
func seqendHandle(groups []Group) string {
	if v, ok := rawValue(groups, groupHandle); ok {
		return v
	}
	return ""
}

// seqendParts splits the groups of a SEQEND record into its handle and, for
// R13 and later, the other groups (owner, layer, subclass markers), which are
// kept. For R12 the other groups carry no data and are dropped, as before.
func (p *parser) seqendParts(groups []Group) (string, []Group) {
	if p.modern || hasSubclassMarkers(groups) {
		return splitHandle(groups)
	}
	return seqendHandle(groups), nil
}

// splitHandle returns the value of the first group code 5 and the remaining
// groups in their original order.
func splitHandle(groups []Group) (string, []Group) {
	handle := ""
	found := false
	var rest []Group
	for _, g := range groups {
		if !found && g.Code == groupHandle {
			handle = strings.TrimSpace(g.Value)
			found = true
			continue
		}
		rest = append(rest, g)
	}
	return handle, rest
}

func (p *parser) finish() (*Document, error) {
	if !p.sawSection {
		return nil, ErrNotDXF
	}
	version, hasVersion := p.doc.Header.Get("$ACADVER")
	// Text is UTF-8 from R2007 on: non-ASCII bytes are expected there.
	if n, first := p.gr.nonASCII(); n > 0 && !(hasVersion && strings.ToUpper(version) >= VersionR2007) {
		p.addDiagnostic(SeverityWarning, first, "", "", fmt.Sprintf(
			"%d group values contain non-ASCII bytes: text before R2007 depends on the code page, the bytes are preserved unchanged", n))
	}
	if !p.sawEOF {
		if p.cfg.lenient {
			p.addDiagnostic(SeverityWarning, p.gr.lastLine, "", "", "missing EOF terminator")
		} else {
			return nil, &SyntaxError{Line: p.gr.lastLine, Reason: "missing EOF terminator"}
		}
	}
	switch {
	case !hasVersion:
		if p.cfg.lenient {
			p.addDiagnostic(SeverityWarning, 0, "HEADER", "", "missing $ACADVER variable")
			p.doc.Version = VersionR12
		} else {
			return nil, &SyntaxError{Section: "HEADER", Reason: "missing $ACADVER variable"}
		}
	case isSupported(version):
		p.doc.Version, _ = canonicalVersion(version)
		if p.doc.Version == VersionR13 || p.doc.Version == VersionR14 {
			p.addDiagnostic(SeverityInfo, 0, "HEADER", "", fmt.Sprintf(
				"version %s is read on a best-effort basis: no real files of this version have been tested", p.doc.Version))
		}
	default:
		if p.cfg.lenient {
			p.addDiagnostic(SeverityWarning, 0, "HEADER", "", fmt.Sprintf("unsupported version %q", version))
			p.doc.Version = version
		} else {
			return nil, &VersionError{Found: version}
		}
	}
	for _, d := range p.doc.ValidateHandles() {
		if d.Severity == SeverityError && !p.cfg.lenient {
			return nil, &ValidationError{Entity: d.Entity, Reason: d.Message}
		}
		p.doc.Diagnostics = append(p.doc.Diagnostics, d)
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

func isSupported(version string) bool {
	_, ok := canonicalVersion(version)
	return ok
}
