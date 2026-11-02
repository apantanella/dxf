# Changelog

All notable changes for users of the library are documented in this file. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### F1 (in progress)

- New package `tags`: `Tag`, `Reader`, `Writer`, `Kind`, character encodings
  (CP1252/Latin-1, UTF-8, `\U+XXXX` escapes). `dxf.Group` is now an alias of
  `tags.Tag`; `SyntaxError`, `LimitError`, `ErrSyntax` and `ErrLimit` are aliases
  of the `tags` types.
- Handles: `Handle`, `Lookup`, `HandleIndex`, `ValidateHandles`, `Owner`,
  `SeqendHandle`, `EndHandle`. Decode and Encode reject duplicate handles.
- ENDBLK and SEQEND handles are kept, so a written file reads back identically.
- ADR 0001-0008 in `docs/adr` and the R2013 notes in `docs/dxf-r2013.md`.

### Added

- Go module `github.com/apantanella/dxf` with Go 1.23 as the minimum version.
- Package comments for `dxf` and `structural`.
- Architecture documentation in `docs/architecture.md`.
- DXF R12 subset specification in `docs/dxf-r12.md`.
- Structural model definition in `docs/structural-model.md`.
- Interoperability and units guide in `docs/interoperability.md`.
- Roadmap and acceptance criteria in `docs/roadmap.md`.
- ADR 0001 on DXF R12 ASCII and ADR 0002 on the package layout.
- Contributing guide, security policy and changelog.
- CI pipeline with formatting, `go vet`, build and tests with the race detector.

### Implemented

- Group code/value scanner with line numbers, LF and CRLF endings and
  configurable limits (`Limits`, `WithLimits`).
- Parser for the `HEADER`, `TABLES`, `BLOCKS` and `ENTITIES` sections, with
  unknown sections preserved in `Document.UnknownSections`.
- Strict and lenient modes (`WithLenient`) with non-fatal `Diagnostic` values.
- Structured errors: `ErrNotDXF`, `ErrUnsupportedVersion`, `ErrSyntax`,
  `ErrLimit`, `ErrValidation`.
- Entities `LINE`, `POINT`, `CIRCLE`, `ARC`, `POLYLINE`/`VERTEX`/`SEQEND`,
  `TEXT`, `SOLID`, `TRACE`, `3DFACE`, `INSERT`/`ATTRIB` and `RawEntity` for
  everything else.
- Deterministic R12 writer with validation, minimal symbol tables, automatic
  addition of referenced layers and optional handle support.
- Stable semantic round-trip: `Encode(Decode(Encode(doc))) == Encode(doc)`.
- `structural` package with `Model`, `Node`, `Member`, `Panel`, `Grid`, `Unit`,
  `Mapping`, `Report`, `Import` and `Export`.
- Classification of beams, columns and braces by layer or by orientation.
- Table-driven tests, runnable examples, decoder fuzz test and DXF and
  structural round-trip tests.
- OCS/WCS geometry helpers in `geometry.go`: `ArbitraryAxis`, `OCS2WCS` and
  `WCS2OCS` following the DXF arbitrary axis algorithm.
- OCS → WCS conversion in the structural import for 2D polylines (elevation and
  normal) and `TEXT` positions.
- Hand-written fixtures in `testdata/` with tests for decoding, entity types,
  round-trip stability and invalid input.
- Benchmarks on a document with 100,000 `LINE` entities: decode ~189 ms
  (28.1 MB/s), encode ~79 ms, round-trip ~271 ms.
- Integration tests behind the `integration` build tag, covering the file system
  path and the fixtures on disk, run by CI.
- `dxfinspect` command that prints the version, blocks, layers, entity counts per
  type and diagnostics of one or more DXF files, with a `-lenient` mode.
- `dxfgen` command that generates verification files: a structural frame exported
  in a chosen unit, a 2D plan of the same frame with square column footprints,
  and a document containing every entity supported by the writer.
- Distinct, documented ACI colors for the `STR-*` layers created by
  `structural.Export`, so overlapping elements stay distinguishable.
- `Document.Extents` and `Document.UpdateExtents`; `Encode` writes `$EXTMIN` and
  `$EXTMAX` when the document does not define them.
- Definitions of the standard AutoCAD linetypes (`DASHED`, `CENTER`, `HIDDEN`,
  ...), written when a layer or an entity refers to them.
- Warning diagnostic from `Decode` when a file contains non-ASCII bytes.
- Regression tests for every defect listed under "Fixed" in
  `regression_f0_test.go`.

### Changed

- `README.md` rewritten with the project description, the scope and the
  implemented API.
- `Encode` rejects a document that refers to a linetype that is neither defined
  in its `LTYPE` table nor one of the standard linetypes.
- Symbol tables are written in the R12 order (`LTYPE` before `LAYER`).
- A group code that appears twice in an entity is no longer dropped: the first
  occurrence is interpreted and the others are kept in `Raw`.

### Fixed

- `POLYLINE` and `INSERT` were written with their handle and layer groups twice,
  and with two different handles when the document used handles.
- Unhandled groups (such as XDATA) of a `POLYLINE` or `INSERT` were written after
  `SEQEND`, which attached them to the wrong record.
- `Encode` added layers and linetypes to the tables of the document it received,
  which was also a data race for concurrent calls.
- `ATTRIB` field length and vertical alignment used each other's group code
  (`73` is the field length, `74` the vertical alignment).
- `LAYER` records could refer to a linetype that was not defined, or defined
  later in the file.
- `BLOCK` did not repeat its name with group code `3`.
- The scanner read a whole line before checking `MaxValueBytes`; a file without
  line breaks was buffered completely. Lines are now rejected as soon as they
  cross the limit.
- `$HANDSEED` ignored the handles of table records and blocks.
- The documentation promised a diagnostic for non-ASCII bytes that was not
  implemented.

## Versioning notes

- `v0.1.0` will be the first working release and will include the R12 codec and
  the basic structural conversion.
- Before `v1.0.0` the public API may change; every breaking change will be
  documented in this file.
