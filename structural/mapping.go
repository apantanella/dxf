package structural

import (
	"fmt"
	"strings"
)

// Mapping assigns structural meaning to DXF layers. It is the only place where
// geometry becomes a beam, a column, a panel or an axis.
type Mapping struct {
	NodeLayers            []string
	MemberLayers          []string
	PanelLayers           []string
	GridLayers            []string
	LabelLayers           []string
	BeamLayers            []string
	ColumnLayers          []string
	BraceLayers           []string
	SnapTolerance         float64
	ClassifyByOrientation bool
	LabelHeight           float64
	Unit                  Unit
}

// DefaultMapping returns the conventions documented in docs/structural-model.md.
func DefaultMapping() Mapping {
	return Mapping{
		NodeLayers:    []string{LayerNodes},
		MemberLayers:  []string{LayerBeams, LayerColumns, LayerBraces, LayerMembers},
		PanelLayers:   []string{LayerPanels},
		GridLayers:    []string{LayerGrids},
		LabelLayers:   []string{LayerLabels},
		BeamLayers:    []string{LayerBeams},
		ColumnLayers:  []string{LayerColumns},
		BraceLayers:   []string{LayerBraces},
		SnapTolerance: 1e-6,
		LabelHeight:   0.2,
		Unit:          UnitUnknown,
	}
}

// Export layer names used by the default mapping.
const (
	LayerNodes   = "STR-NODES"
	LayerBeams   = "STR-BEAMS"
	LayerColumns = "STR-COLUMNS"
	LayerBraces  = "STR-BRACES"
	LayerPanels  = "STR-PANELS"
	LayerGrids   = "STR-GRIDS"
	LayerLabels  = "STR-LABELS"
)

// ACI colors assigned by Export to the structural layers, so that overlapping
// elements remain distinguishable.
const (
	ColorNodes   = 2 // yellow
	ColorBeams   = 1 // red
	ColorColumns = 3 // green
	ColorBraces  = 5 // blue
	ColorMembers = 6 // magenta
	ColorPanels  = 4 // cyan
	ColorGrids   = 8 // dark gray
	ColorLabels  = 7 // white/black
)

// LayerStyle is a structural layer with the color and linetype used by Export.
type LayerStyle struct {
	Name     string
	Color    int
	Linetype string
}

// LayerStyles returns the layers created by Export, in creation order.
func LayerStyles() []LayerStyle {
	return []LayerStyle{
		{Name: LayerNodes, Color: ColorNodes, Linetype: "CONTINUOUS"},
		{Name: LayerBeams, Color: ColorBeams, Linetype: "CONTINUOUS"},
		{Name: LayerColumns, Color: ColorColumns, Linetype: "CONTINUOUS"},
		{Name: LayerBraces, Color: ColorBraces, Linetype: "CONTINUOUS"},
		{Name: LayerMembers, Color: ColorMembers, Linetype: "CONTINUOUS"},
		{Name: LayerPanels, Color: ColorPanels, Linetype: "CONTINUOUS"},
		{Name: LayerGrids, Color: ColorGrids, Linetype: "CONTINUOUS"},
		{Name: LayerLabels, Color: ColorLabels, Linetype: "CONTINUOUS"},
	}
}

func (m Mapping) snapTolerance() float64 {
	if m.SnapTolerance <= 0 {
		return 1e-6
	}
	return m.SnapTolerance
}

func (m Mapping) labelHeight() float64 {
	if m.LabelHeight <= 0 {
		return 0.2
	}
	return m.LabelHeight
}

func (m Mapping) inLayers(layers []string, layer string) bool {
	for _, candidate := range layers {
		if strings.EqualFold(strings.TrimSpace(candidate), layer) {
			return true
		}
	}
	return false
}

func (m Mapping) memberKind(layer string) (MemberKind, bool) {
	switch {
	case m.inLayers(m.ColumnLayers, layer):
		return MemberColumn, true
	case m.inLayers(m.BraceLayers, layer):
		return MemberBrace, true
	case m.inLayers(m.BeamLayers, layer):
		return MemberBeam, true
	case m.inLayers(m.MemberLayers, layer):
		return MemberGeneric, true
	default:
		return MemberGeneric, false
	}
}

// Diagnostic describes an entity that Import could not convert.
type Diagnostic struct {
	Entity  string
	Layer   string
	Message string
}

// String renders the diagnostic.
func (d Diagnostic) String() string {
	if d.Layer != "" {
		return fmt.Sprintf("%s (layer %s): %s", d.Entity, d.Layer, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Entity, d.Message)
}

// Report describes the outcome of an import.
type Report struct {
	Skipped     int
	Ambiguous   int
	Diagnostics []Diagnostic
}

func (r *Report) skip(entity, layer, message string) {
	r.Skipped++
	r.Diagnostics = append(r.Diagnostics, Diagnostic{Entity: entity, Layer: layer, Message: message})
}

func (r *Report) ambiguous(entity, layer, message string) {
	r.Ambiguous++
	r.Diagnostics = append(r.Diagnostics, Diagnostic{Entity: entity, Layer: layer, Message: message})
}
