# Roadmap

The phases are sequential. Every phase ends with its own tests green and the
documentation updated. There are no dates: there are completion criteria.

## Phase 0 — Foundations

Status: complete.

- Module `github.com/apantanella/dxf`.
- Package comments for `dxf` and `structural`.
- Documentation for architecture, format, structural model, interoperability and
  roadmap.
- ADRs on the format and the package layout.
- CI pipeline.
- Contributing and security conventions.

Completion criterion: `go build ./...`, `go vet ./...` and `go test ./...` pass;
the documentation matches the cited sources.

## Phase 1 — Scanner and parser

Status: complete.

- `(group code, value)` pair scanner with line numbers.
- Recognition of `SECTION`, `ENDSEC`, `EOF`.
- `HEADER`, `TABLES`, `BLOCKS`, `ENTITIES` sections.
- Strict and lenient modes.
- Errors `ErrSyntax`, `ErrNotDXF`, `ErrUnsupportedVersion`, `ErrLimit`.
- Limits on entities, blocks, value size and nesting.
- Fuzz test for the scanner and the parser.

Completion criterion: a real R12 document is read without panics and with zero
unexpected diagnostics; malformed input always produces an error.

## Phase 2 — Model and writer

Status: complete.

- `Document`, `Header`, `Tables`, `Blocks`, `Entities`.
- `Entity` and `EntityCommon`.
- Document validation.
- Deterministic writer for `HEADER`, `TABLES`, `BLOCKS`, `ENTITIES`, `EOF`.
- Implicit layer handling.
- Semantic round-trip on the test documents.

Completion criterion: `Decode -> Encode -> Decode` is stable; a generated file
opens in at least two CAD applications without errors.

## Phase 3 — Entities

Status: complete.

- `POINT`, `LINE`, `CIRCLE`, `ARC`.
- `POLYLINE`, `VERTEX`, `SEQEND`.
- `TEXT`, `SOLID`, `TRACE`, `3DFACE`.
- `INSERT` and block definitions.
- OCS/WCS conversion according to the arbitrary axis algorithm.

Completion criterion: every entity has tests for reading, writing, default
values and edge cases.

## Phase 4 — Structural model

Status: complete.

- `Model`, `Node`, `Member`, `Panel`, `Grid`, `Unit` types.
- `Mapping` and `DefaultMapping`.
- `Import` with `Report`.
- `Export` with default layer conventions.
- Invariant validation.
- OCS → WCS conversion for 2D polylines and `TEXT` during import.

Completion criterion: a simple frame is imported, modified and exported; the
report describes every exclusion.

## Phase 5 — Quality and interoperability

Status: in progress.

- [x] Valid and malformed fixtures in `testdata/`.
- [x] Fuzz tests on arbitrary input.
- [x] Benchmarks on a document with 100,000 entities.
- [x] OCS → WCS conversion in the structural import.
- [ ] Manual verification with LibreCAD, QCAD and an Autodesk product.
- [ ] Documentation of the observed differences and of the interoperability
      matrix.

Completion criterion: the interoperability matrix is filled in; no panics;
memory usage proportional to the file size.

## Phase 6 — Release v0.1.0

- Freeze of the public API.
- Final README with runnable examples.
- Green `Example` tests.
- Updated changelog.
- Tag `v0.1.0`.

Completion criterion: every acceptance criterion in the README is satisfied.

## Acceptance criteria for v0.1.0

1. Reads and writes DXF R12 ASCII without third-party dependencies.
2. Supports every entity listed in [dxf-r12.md](dxf-r12.md).
3. Imports and exports a simple structural frame.
4. Never silently drops unhandled entities or sections.
5. Syntax errors report line and group code.
6. Round-trip and fuzz tests find no errors.
7. `gofmt`, `go vet`, `go test` and the race detector complete successfully.
8. Generated files are verified with external CAD applications.
9. README, GoDoc, structural guide and R12 limits are complete.

## Future work

Outside the scope of `v0.1.0`, in order of likelihood:

| Feature | Note |
| --- | --- |
| XDATA for structural metadata | Requires an ADR and a dedicated `APPID` |
| Text code page handling | Requires an ADR on encoding |
| Reading DXF R13–R14 | `LWPOLYLINE`, `SPLINE`, `ELLIPSE` |
| Writing DXF 2000 | Mandatory handles, `OBJECTS` section |
| Dimension entities | More complex parsing and model |
| Sections and materials | Separate package, not in `structural` |
| IFC import | Different format, not a DXF extension |
| `dxfinspect` CLI | Optional diagnostic utility |

None of these enters `v0.1.0`: widening the scope before having a stable
round-trip would make the release unverifiable.
