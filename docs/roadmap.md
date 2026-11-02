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
- [x] Integration tests on the file system path (`-tags=integration`).
- [x] Diagnostic CLI in `cmd/dxfinspect`.
- [x] Verification file generator in `cmd/dxfgen`.
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

## Phase 7 — F1: R2013 core

Status: complete. G1 (task T11) passed on 2 November 2026 in ZWCAD 2023 and
CadCraft 0.16.0. The detailed task
sheets are in [f1-task-t3-t9.md](f1-task-t3-t9.md) (Italian) and the decisions
in [adr/README.md](adr/README.md).

- T3: package `tags` (group code stream reader and writer).
- T4: character encodings (UTF-8, CP1252, Latin-1, `\U+XXXX`; ADR 0004).
- T5: `Handle`, owners, handle index and `$HANDSEED` (ADR 0006).
- T6: multi-version reader, R12 to R2018, binary DXF rejected (ADR 0008).
- T7: typed tables and `BLOCK_RECORD`.
- T8: `CLASSES`, `OBJECTS`, dictionaries and `LAYOUT`.
- T9: writer for R2000 to R2018 (`WithVersion`, `NewDocumentVersion`; ADR 0009),
  R12 writing unchanged.
- T10: ezdxf oracle, round-trip tests, benchmarks (`scripts/`, `testdata/oracle`,
  [oracle.md](oracle.md)).
- T12: documentation of F1.
- T11 (G1): the four files of `testdata/g1/` open and save correctly in ZWCAD
  2023 and CadCraft 0.16.0 (results table in `testdata/g1/README.md`).

Completion criterion: G1 passed with at least two CAD applications, ZWCAD
included (met). ADR 0002 moves `NewDocument()` to R2013 with a changelog
entry; that change is still to be made and `NewDocument()` returns an R12 document. Written versions other than R2013
are not declared verified until they are opened in a CAD (ADR 0009).

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
| Text code page handling | Partly done in F1 (ADR 0004): writing converts to CP1252 and Latin-1; reading keeps the bytes |
| Reading DXF R13–R14 | Done in F1 on a best-effort basis; `LWPOLYLINE`, `SPLINE`, `ELLIPSE` are kept as `RawEntity` |
| Writing DXF 2000 | Done in F1 for R2000 to R2018 (ADR 0009); R13 and R14 are not written |
| Dimension entities | More complex parsing and model |
| Sections and materials | Separate package, not in `structural` |
| IFC import | Different format, not a DXF extension |

None of these enters `v0.1.0`: widening the scope before having a stable
round-trip would make the release unverifiable.
