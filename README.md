# dxf

Go library for reading and writing DXF R12 ASCII files, with a conversion layer
for structural models.

Status: **the R12 codec and the basic structural conversion are implemented**.
The core package (`dxf`) reads and writes DXF R12 ASCII; the `structural`
package imports and exports nodes, members, panels and grids. See
[docs/roadmap.md](docs/roadmap.md) for the remaining phases.

## Scope

The first release (`v0.1.0`) supports:

- DXF R12 ASCII, identified by the header value `$ACADVER` `AC1009`.
- Reading R12 documents with optional handles, unknown sections preserved, and
  explicit diagnostics for unhandled data.
- Writing valid R12 documents with the `HEADER`, `TABLES`, `BLOCKS` and
  `ENTITIES` sections and the `EOF` terminator.
- Entities `POINT`, `LINE`, `CIRCLE`, `ARC`, `POLYLINE`/`VERTEX`/`SEQEND`,
  `TEXT`, `SOLID`, `TRACE`, `3DFACE` and `INSERT` with `ATTRIB`.
- DXF ↔ structural model conversion driven by explicit mapping rules.

Out of scope for the first release:

- Binary DXF and versions after R12.
- `LWPOLYLINE`, `SPLINE`, `ELLIPSE`, `MTEXT` and other entities that R12 does
  not provide.
- A structural calculation model: materials, sections, supports, loads,
  combinations and code checks.
- Automatic inference of structural semantics from geometry alone.

## Requirements

- Go 1.23 or later.
- No third-party dependency: the core uses only the standard library.

## Installation

```bash
go get github.com/apantanella/dxf
```

## API

```go
// Reading a document.
doc, err := dxf.Decode(r) // r is an io.Reader

// Writing a document.
err = dxf.Encode(w, doc) // w is an io.Writer
```

Reading example:

```go
f, err := os.Open("frame.dxf")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

doc, err := dxf.Decode(f)
if err != nil {
    log.Fatal(err)
}

for _, ent := range doc.Entities {
    log.Printf("%s on layer %s", ent.Type(), ent.Layer())
}
```

Structural conversion example:

```go
mapping := structural.DefaultMapping()
model, report, err := structural.Import(doc, mapping)
if err != nil {
    log.Fatal(err)
}
log.Printf("nodes: %d, members: %d, skipped: %d",
    len(model.Nodes), len(model.Members), report.Skipped)
```

## Current limitations

- OCS → WCS conversion covers 2D polylines and `TEXT`; other OCS entities are not
  imported structurally.
- There is no implicit unit conversion: `structural.Unit` and
  `structural.Mapping.Unit` must be declared by the application.
- Structural metadata uses layers and blocks, not XDATA.
- Generated files have not yet been verified with external CAD applications.

## Performance

Benchmarks on 100,000 `LINE` entities (Intel Core i7-1355U, Go 1.27):

| Operation | Time | Throughput | Allocated |
| --- | --- | --- | --- |
| `Decode` | ~189 ms | 28.1 MB/s | 96 MB, 2.9M allocs |
| `Encode` | ~79 ms | — | 39 MB, 0.8M allocs |
| Round-trip | ~271 ms | 19.6 MB/s | 135 MB, 3.7M allocs |

Run them with:

```powershell
go test -run=XXX -bench=. -benchmem .
```

## Documentation

| Document | Content |
| --- | --- |
| [docs/architecture.md](docs/architecture.md) | Flows, dependencies and design choices |
| [docs/dxf-r12.md](docs/dxf-r12.md) | Supported DXF R12 subset |
| [docs/structural-model.md](docs/structural-model.md) | Structural model and mapping |
| [docs/interoperability.md](docs/interoperability.md) | Units, verification and CAD compatibility |
| [docs/roadmap.md](docs/roadmap.md) | Phases, acceptance criteria and future work |
| [docs/decisions](docs/decisions) | Architecture decision records |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Setup, tests and conventions |
| [SECURITY.md](SECURITY.md) | Parsing untrusted files |
| [CHANGELOG.md](CHANGELOG.md) | Change history |

## Verification

```powershell
gofmt -d .
go vet ./...
go test ./...
go test -race ./...
```

## License

GPL-3.0. See [LICENSE](LICENSE).
