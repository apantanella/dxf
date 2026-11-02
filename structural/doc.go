// Package structural converts between DXF drawings and a structural model.
//
// The model in this package is independent from DXF naming conventions. It
// describes nodes, members, panels and grids, and it does not encode materials,
// cross sections, supports or loads. Those concepts belong to a calculation
// model, not to a CAD exchange format.
//
// Import and export are driven by an explicit Mapping: a DXF LINE is not a beam
// unless the mapping says so. See docs/structural-model.md for the conventions
// and the invariants of every type.
//
// This package imports github.com/apantanella/dxf. The dependency direction is
// never reversed.
package structural
