// Package dxf implements reading and writing of DXF ASCII files, from R12
// (AC1009) to R2018 (AC1032).
//
// The package models a DXF document as sections, symbol tables, blocks,
// entities and, from R13 on, classes and objects. It does not depend on any
// third-party library: the tokenizer (package tags), the parser and the writer
// use only the Go standard library.
//
// # Versions
//
// One Document type serves every version; Document.Version holds the $ACADVER
// value (VersionR12, VersionR2000 ... VersionR2018). Decode reads the nine
// versions from AC1009 to AC1032 (R13 and R14 on a best-effort basis). Encode
// writes VersionR12 and the versions from VersionR2000 on; it does not write
// R13 and R14. The R2013 format (AC1027) is the one checked against the
// reference files of docs/dxf-r2013.md; the other versions differ from it only
// in $ACADVER and, before R2007, in the text encoding (docs/adr/0009).
//
// # Surface
//
//   - Decode reads a DXF ASCII stream into a Document.
//   - Encode writes a Document; WithVersion chooses the version written.
//   - NewDocument returns an empty R12 document; NewDocumentVersion returns an
//     empty document of the given version.
//   - Handle, Document.Lookup and Document.ValidateHandles give access to the
//     handles (group code 5) and the owners (330) of R13+ files.
//   - Document.Classes and Document.Objects hold the CLASSES and OBJECTS
//     sections; Dictionary, Layout and Placeholder are interpreted, the rest is
//     kept as RawObject.
//   - Tables.Layer, Tables.Linetype, Tables.TextStyle and Tables.BlockRecords
//     are typed views of the symbol tables.
//   - The Document, its sections and its entities are plain data structures
//     that callers can inspect and modify before writing.
//
// Supported entities: LINE, POINT, CIRCLE, ARC, POLYLINE with VERTEX and
// SEQEND, TEXT, SOLID, TRACE, 3DFACE and INSERT with ATTRIB. Any other entity
// is preserved as RawEntity with all its groups, so a round-trip does not lose
// data (ADR 0007). A document that holds R13+ content that R12 cannot
// represent is rejected by Encode with an error that wraps ErrValidation.
//
// # Limits
//
// DXF binary is not supported: Decode returns an error that satisfies
// errors.Is(err, ErrNotDXF) (ADR 0008). Before R2007 the text is not
// transcoded when read: the bytes are kept and a warning is recorded; Encode
// converts to the code page of $DWGCODEPAGE (ANSI_1252 or ISO8859-1) when
// writing those versions (ADR 0004). The R2000 to R2018 files written by this
// package are verified with ezdxf; only R2013 files have been designed against
// files opened in ZWCAD and CadCraft, and the hands-on check of the written
// files (G1) is still to be done. See docs/interoperability.md.
//
// The package never interprets coordinates as a physical unit. Only
// $INSUNITS, written by NewDocumentVersion for R2000 and later, carries a unit
// (4, millimetres); the caller must supply and document the unit used by the
// application.
//
// Structural semantics such as beams, columns and slabs are out of scope for
// this package and belong to github.com/apantanella/dxf/structural.
package dxf
