// Package dxf implements reading and writing of DXF R12 ASCII files.
//
// The package models a DXF document as sections, symbol tables, blocks and
// entities. It does not depend on any third-party library: the scanner, the
// parser and the writer use only the Go standard library.
//
// The public surface is intentionally small:
//
//   - Decode reads a DXF R12 ASCII stream into a Document.
//   - Encode writes a Document as a DXF R12 ASCII stream.
//   - NewDocument returns an empty document with the default header and tables.
//   - The Document, its sections and its entities are plain data structures
//     that callers can inspect and modify before writing.
//
// Supported entities: LINE, POINT, CIRCLE, ARC, POLYLINE with VERTEX and
// SEQEND, TEXT, SOLID, TRACE, 3DFACE and INSERT with ATTRIB. Any other entity
// is preserved as RawEntity, so a round-trip does not lose data.
//
// The package never interprets coordinates as a physical unit. DXF R12 carries
// no drawing unit, so the caller must supply and document the unit used by the
// application. See docs/interoperability.md.
//
// Structural semantics such as beams, columns and slabs are out of scope for
// this package and belong to github.com/apantanella/dxf/structural.
package dxf
