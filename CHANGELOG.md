# Changelog

All notable changes for users of the library are documented in this file. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
  structural round-trip tests. Coverage: 72.9% (`dxf`), 70.7% (`structural`).

### Changed

- `README.md` rewritten with the project description, the scope and the
  implemented API.

## Versioning notes

- `v0.1.0` will be the first working release and will include the R12 codec and
  the basic structural conversion.
- Before `v1.0.0` the public API may change; every breaking change will be
  documented in this file.
