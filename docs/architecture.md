# Architecture

## Goal

Separate three responsibilities that evolve at different speeds:

1. **DXF codec** (`dxf`): reads and writes the R12 ASCII format.
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
structural  --->  dxf
```

`dxf` does not import `structural`. No package imports `structural` except the
application. This rule avoids cycles and keeps the codec reusable for
non-structural uses.

## Flows

### Reading

```text
io.Reader -> group code scanner -> section parser -> Document -> caller
```

1. The scanner produces `(code, value)` pairs and tracks the line number.
2. The parser recognizes `SECTION`/`ENDSEC` and builds the known sections.
3. Entities are decoded by one function per type.
4. Unknown sections and unknown group codes produce diagnostics.
5. `Document` remains a simple, inspectable, modifiable data structure.

### Writing

```text
Document -> validation -> section writer -> io.Writer
```

1. Validation checks references, layers, blocks and numeric values.
2. The writer emits `HEADER`, `TABLES`, `BLOCKS`, `ENTITIES` and `EOF`.
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
- `ErrUnsupportedVersion`: `$ACADVER` is not `AC1009`.
- `ErrSyntax`: syntax error with line, section and entity.
- `ErrLimit`: a configured limit was exceeded.
- `ErrValidation`: the document cannot be written.

`SyntaxError` implements `Unwrap` and provides a contextual message, for example:
`dxf: line 128: entity LINE: group code 10 "abc": invalid real value`.

## Excluded choices

- No dependency injection: packages are functions and types, not services.
- No premature interface with a single implementation.
- No `init()`.
- No third-party dependency in the core.
- No generic format abstraction: the format is DXF R12.

## Implementation status

Phases 1–4 of the roadmap are implemented:

| Component | File | Status |
| --- | --- | --- |
| Group code scanner | `scanner.go` | complete |
| Diagnostics and errors | `diagnostics.go` | complete |
| Group code kinds | `groupcode.go` | complete |
| OCS/WCS geometry | `geometry.go` | complete |
| Document, header, tables | `dxf.go` | complete |
| Entities and decoders | `entities.go` | complete |
| Section parser | `reader.go` | complete |
| Writer and validation | `writer.go` | complete |
| Structural model | `structural/model.go` | complete |
| Mapping and report | `structural/mapping.go` | complete |
| Import and export | `structural/import.go`, `structural/export.go` | complete |
| Diagnostic CLI | `cmd/dxfinspect/main.go` | complete |
| Verification file generator | `cmd/dxfgen/main.go` | complete |

Open items: manual verification with external CAD applications and the resulting
interoperability matrix.

## References

- [docs/dxf-r12.md](dxf-r12.md)
- [docs/structural-model.md](structural-model.md)
- [docs/decisions/0001-r12-ascii.md](decisions/0001-r12-ascii.md)
- [docs/decisions/0002-package-layout.md](decisions/0002-package-layout.md)
