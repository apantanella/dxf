package structural

import (
	"fmt"

	"github.com/apantanella/dxf"
)

// Unit is the length unit declared by the application. DXF R12 carries no
// drawing unit, so the unit is never inferred from the file.
type Unit int

// Supported units.
const (
	UnitUnknown Unit = iota
	UnitMillimeter
	UnitCentimeter
	UnitMeter
	UnitInch
	UnitFoot
)

// String returns the unit name.
func (u Unit) String() string {
	switch u {
	case UnitMillimeter:
		return "mm"
	case UnitCentimeter:
		return "cm"
	case UnitMeter:
		return "m"
	case UnitInch:
		return "in"
	case UnitFoot:
		return "ft"
	default:
		return "unknown"
	}
}

// Factor returns the number of meters in one unit.
func (u Unit) Factor() (float64, bool) {
	switch u {
	case UnitMillimeter:
		return 0.001, true
	case UnitCentimeter:
		return 0.01, true
	case UnitMeter:
		return 1, true
	case UnitInch:
		return 0.0254, true
	case UnitFoot:
		return 0.3048, true
	default:
		return 0, false
	}
}

// Convert converts a length from one unit to another. Both units must be known.
func Convert(value float64, from, to Unit) (float64, error) {
	if from == to {
		return value, nil
	}
	fromFactor, ok := from.Factor()
	if !ok {
		return 0, fmt.Errorf("structural: unknown source unit")
	}
	toFactor, ok := to.Factor()
	if !ok {
		return 0, fmt.Errorf("structural: unknown target unit")
	}
	return value * fromFactor / toFactor, nil
}

// MemberKind classifies a linear member.
type MemberKind int

// Member kinds. MemberGeneric is the zero value and means "not classified".
const (
	MemberGeneric MemberKind = iota
	MemberBeam
	MemberColumn
	MemberBrace
)

// String returns the kind name.
func (k MemberKind) String() string {
	switch k {
	case MemberBeam:
		return "beam"
	case MemberColumn:
		return "column"
	case MemberBrace:
		return "brace"
	default:
		return "generic"
	}
}

// PanelKind classifies a surface element.
type PanelKind int

// Panel kinds. PanelGeneric is the zero value and means "not classified".
const (
	PanelGeneric PanelKind = iota
	PanelWall
	PanelSlab
)

// String returns the kind name.
func (k PanelKind) String() string {
	switch k {
	case PanelWall:
		return "wall"
	case PanelSlab:
		return "slab"
	default:
		return "generic"
	}
}

// Node is a structural point.
type Node struct {
	ID       string
	Position dxf.Point3D
	Label    string
}

// Member is a linear element between two nodes.
type Member struct {
	ID    string
	Start string
	End   string
	Kind  MemberKind
	Label string
}

// Panel is a surface element with a closed boundary. The boundary does not
// repeat the first point at the end.
type Panel struct {
	ID       string
	Boundary []dxf.Point3D
	Kind     PanelKind
	Label    string
}

// Grid is a reference axis.
type Grid struct {
	ID    string
	Start dxf.Point3D
	End   dxf.Point3D
	Label string
}

// Model is a geometric-structural model. It does not carry materials, sections,
// supports or loads.
type Model struct {
	Nodes   []Node
	Members []Member
	Panels  []Panel
	Grids   []Grid
	Unit    Unit
}

// Node returns the node with the given identifier.
func (m *Model) Node(id string) (Node, bool) {
	for _, node := range m.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return Node{}, false
}

// AddNode appends a node and returns its identifier.
func (m *Model) AddNode(position dxf.Point3D) string {
	id := fmt.Sprintf("N%d", len(m.Nodes)+1)
	m.Nodes = append(m.Nodes, Node{ID: id, Position: position})
	return id
}

// AddMember appends a member between two existing nodes.
func (m *Model) AddMember(startID, endID string, kind MemberKind) (string, error) {
	if _, ok := m.Node(startID); !ok {
		return "", fmt.Errorf("structural: node %q does not exist", startID)
	}
	if _, ok := m.Node(endID); !ok {
		return "", fmt.Errorf("structural: node %q does not exist", endID)
	}
	if startID == endID {
		return "", fmt.Errorf("structural: member with coincident ends")
	}
	id := fmt.Sprintf("M%d", len(m.Members)+1)
	m.Members = append(m.Members, Member{ID: id, Start: startID, End: endID, Kind: kind})
	return id, nil
}

// AddPanel appends a panel with a closed boundary.
func (m *Model) AddPanel(boundary []dxf.Point3D, kind PanelKind) (string, error) {
	if len(boundary) < 3 {
		return "", fmt.Errorf("structural: a panel requires at least three points")
	}
	id := fmt.Sprintf("P%d", len(m.Panels)+1)
	points := make([]dxf.Point3D, len(boundary))
	copy(points, boundary)
	m.Panels = append(m.Panels, Panel{ID: id, Boundary: points, Kind: kind})
	return id, nil
}

// Validate checks the invariants documented in docs/structural-model.md.
func (m *Model) Validate() error {
	if m == nil {
		return fmt.Errorf("structural: nil model")
	}
	seen := make(map[string]bool, len(m.Nodes))
	for _, node := range m.Nodes {
		if node.ID == "" {
			return fmt.Errorf("structural: node without identifier")
		}
		if seen[node.ID] {
			return fmt.Errorf("structural: duplicate node %q", node.ID)
		}
		seen[node.ID] = true
	}
	for _, member := range m.Members {
		if member.ID == "" {
			return fmt.Errorf("structural: member without identifier")
		}
		if !seen[member.Start] || !seen[member.End] {
			return fmt.Errorf("structural: member %q references missing nodes", member.ID)
		}
		if member.Start == member.End {
			return fmt.Errorf("structural: member %q with coincident ends", member.ID)
		}
	}
	for _, panel := range m.Panels {
		if panel.ID == "" {
			return fmt.Errorf("structural: panel without identifier")
		}
		if len(panel.Boundary) < 3 {
			return fmt.Errorf("structural: panel %q with fewer than three points", panel.ID)
		}
	}
	return nil
}
