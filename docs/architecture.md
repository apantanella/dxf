# Architecture

## Goal

Separate three responsibilities that evolve at different speeds:

1. **DXF codec** (`dxf`): reads DXF ASCII from R12 to R2018 and writes R12 and
   R2000 to R2018. It sits on `tags`, the group code stream.
2. **Structural model** (`structural`): represents nodes, members, panels and
   grids with their own invariants.
3. **Mapping** (`structural`): translates between the two, explicitly and
   configurably.

The `dxf` package knows nothing about beams or columns. The `structural` package
knows nothing about group codes, DXF sections or the ASCII format.

## Dependency direction

```text
application
    |
    v
structural  --->  dxf  --->  tags
```

`dxf` does not import `structural`. `tags` imports neither: it knows nothing
about sections, entities or versions. No package imports `structural` except the
application. This rule avoids cycles and keeps the codec reusable for
non-structural uses.

## Flows

### Reading

```text
io.Reader -> tags.Reader (via scanner.go) -> section parser -> Document -> caller
```

1. The scanner produces `(code, value)` pairs and tracks the line number. It is
   an adapter over `tags.Reader`; `dxf.Group` is an alias of `tags.Tag`.
2. The parser recognizes `SECTION`/`ENDSEC` and builds the known sections.
3. Entities are decoded by one function per type.
4. Unknown sections and unknown group codes produce diagnostics.
   From R13 on the parser also keeps subclass markers (`100`), `102 { ... }`
   groups, owners (`330`), extended data, `CLASSES` and `OBJECTS`. A binary DXF
   file is rejected with `ErrNotDXF`.
5. `Document` remains a simple, inspectable, modifiable data structure.

### Writing

```text
Document -> validation -> section writer -> io.Writer
```

1. Validation checks references, layers, blocks and numeric values.
2. The writer emits `HEADER`, `TABLES`, `BLOCKS`, `ENTITIES` and `EOF`. For
   R2000 to R2018 (`writer_r2013.go`) it also emits `CLASSES` and `OBJECTS`,
   handles, owners and subclass markers, with CR LF line endings and group codes
   on three columns. The version is `WithVersion`, else `Document.Version`, else
   `$ACADVER`, else R12. The R12 branch (`writer.go`) is unchanged.
3. The emission order is deterministic, so generated files are comparable.
4. An invalid document must never produce silent partial output.

### Structural conversion

```text
Document + Mapping -> Import  -> Model + Report
Model    + Mapping -> Export  -> Document
```

`Report` lists skipped, ambiguous or rounded entities. The mapping is the only
place where a geometry becomes structural.

## DXF data model

`Document` contains:

- `Header`: known and unknown header variables.
- `Tables`: symbol tables (`LAYER`, `LTYPE`, `STYLE`, `APPID` and others),
  preserved as generic records with the original group codes.
- `Blocks`: block definitions with their internal entities.
- `Entities`: model space entities.
- `Diagnostics`: non-fatal warnings collected while reading.
- `Version`: the `$ACADVER` value; one `Document` type serves every version
  (ADR 0003 in `docs/adr`).
- `Classes` and `Objects` (R13 and later): `Class`, `Dictionary`, `Layout`,
  `Placeholder` and `RawObject`.
- `Tables.BlockRecords` and the typed table views `Tables.Layer`,
  `Tables.Linetype` and `Tables.TextStyle`.

Handles are a `Handle` (`uint64`) type with `Document.Lookup`, `HandleIndex` and
`ValidateHandles`. The `Handle` fields of entities stay strings so that the text
read is preserved; `HandleValue` and `SetHandle` convert (ADR 0006).

The `Entity` interface is small:

```go
type Entity interface {
    Type() string
    Layer() string
    Common() *EntityCommon
}
```

`EntityCommon` collects the properties present on almost every entity: layer,
linetype, color, optional handle, visibility and extrusion. Each concrete entity
adds only its own geometric fields.

## Preserving unknown data

Policy for the first release:

- Known entities: decoded into concrete types.
- Unsupported entities: preserved as `RawEntity` with the original group codes.
  In R13+ records `RawEntity.Verbatim` is true and `Groups` is the whole record.
- Unsupported objects of the `OBJECTS` section: preserved as `RawObject`.
- Unrecognized group codes on known entities: preserved in `EntityCommon.Raw`
  for rewriting, unless an ADR decides otherwise.
- Unknown sections: preserved as raw data with an informational diagnostic.

This policy prevents a rewrite from silently dropping data.

## Read modes

- **Strict**: any unrecoverable anomaly is an error. This is the default for
  tests and controlled imports.
- **Lenient**: anomalies become diagnostics and reading continues. This is the
  intended mode for files produced by third-party CAD applications.

The mode does not change the model: it only changes the behavior on errors.

## Limits and security

The parser treats input as untrusted:

- Limit on the number of entities and blocks.
- Limit on the size of a single text value.
- Limit on the nesting depth of structures.
- No allocation based on a count declared in the file without verification.
- No panic on malformed input: errors are always returned.

See [SECURITY.md](../SECURITY.md).

## Errors

The package uses sentinel errors and error types:

- `ErrNotDXF`: the stream does not contain a recognizable DXF structure.
- `ErrUnsupportedVersion`: `$ACADVER` is not one of the nine supported values
  (`AC1009` to `AC1032`), or `Encode` was asked for a version it cannot write
  (R13, R14). `VersionError` carries the value found.
- `ErrSyntax`: syntax error with line, section and entity.
- `ErrLimit`: a configured limit was exceeded.
- `ErrValidation`: the document cannot be written, for example R13+ content
  written as R12.

`SyntaxError` implements `Unwrap` and provides a contextual message, for example:
`dxf: line 128: entity LINE: group code 10 "abc": invalid real value`.

## Excluded choices

- No dependency injection: packages are functions and types, not services.
- No premature interface with a single implementation.
- No `init()`.
- No third-party dependency in the core.
- No generic format abstraction: the format is DXF ASCII, with one model for all
  versions.

## Implementation status

Phases 1–4 of the roadmap and phase 7 (F1, except the hands-on check G1) are
implemented:

| Component | File | Status |
| --- | --- | --- |
| Group code stream | `tags/` | complete |
| Group code scanner (adapter) | `scanner.go` | complete |
| Diagnostics and errors | `diagnostics.go` | complete |
| Group code aliases | `groupcode.go` | complete |
| Handles and index | `handle.go` | complete |
| Typed tables | `tables.go` | complete |
| Classes | `classes.go` | complete |
| Objects, dictionaries | `objects.go` | complete |
| Layouts | `layouts.go` | complete |
| OCS/WCS geometry | `geometry.go` | complete |
| Document, header, tables | `dxf.go` | complete |
| Entities and decoders | `entities.go` | complete |
| Section parser | `reader.go` | complete |
| Writer and validation (R12) | `writer.go` | complete |
| Writer R2000 to R2018 | `writer_r2013.go` | complete, G1 pending |
| Structural model | `structural/model.go` | complete |
| Mapping and report | `structural/mapping.go` | complete |
| Import and export | `structural/import.go`, `structural/export.go` | complete |
| Diagnostic CLI | `cmd/dxfinspect/main.go` | complete |
| Verification file generator | `cmd/dxfgen/main.go` | complete |

Open items: manual verification with external CAD applications and the resulting
interoperability matrix; the G1 check of the files written for R2013
(`testdata/g1/README.md`).

## References

- [docs/dxf-r12.md](dxf-r12.md)
- [docs/structural-model.md](structural-model.md)
- [docs/decisions/0001-r12-ascii.md](decisions/0001-r12-ascii.md)
- [docs/decisions/0002-package-layout.md](decisions/0002-package-layout.md)
- [docs/adr/README.md](adr/README.md): decisions of F1
- [docs/dxf-r2013.md](dxf-r2013.md): the R2013 file written by the library
