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

Document helpers:

```go
// Bounding box of the model-space entities, in WCS.
min, max, ok := doc.Extents()

// Recompute $EXTMIN and $EXTMAX after editing a document.
doc.UpdateExtents()
```

`Encode` fills `$EXTMIN` and `$EXTMAX` from `Extents` when the document does not
define them, and leaves them untouched otherwise. Call `UpdateExtents` after
editing a document that already carries them.

The writer defines the standard AutoCAD linetypes (`CONTINUOUS`, `DASHED`,
`CENTER`, `HIDDEN`, `PHANTOM`, `DASHDOT`, `DOT`, `DIVIDE`, `BORDER`, `DASHDOT2`)
when a layer or an entity refers to them. A document that refers to any other
linetype must define it in its `LTYPE` table, otherwise `Encode` fails with an
error that wraps `ErrValidation`.

`Decode` records one warning when a file contains non-ASCII bytes: R12 text
depends on the code page, and the bytes are preserved unchanged.

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

## Generating verification files

`dxfgen` writes DXF R12 files used to check the library against a CAD
application. The frame is defined in meters and exported in the requested unit,
so the same geometry exercises the unit conversion.

```powershell
# Structural frame expressed in meters (3D: use an isometric view)
go run ./cmd/dxfgen -out frame-m.dxf -kind frame -unit m

# The same frame converted to millimeters
go run ./cmd/dxfgen -out frame-mm.dxf -kind frame -unit mm

# 2D plan of the same frame: columns are square footprints, visible in top view
go run ./cmd/dxfgen -out plan-m.dxf -kind plan -unit m

# Every entity supported by the writer, one per layer
go run ./cmd/dxfgen -out entities.dxf -kind entities
```

`frame` and `plan` use distinct, documented layer colors, so overlapping
elements stay distinguishable: nodes yellow (2), beams red (1), columns green
(3), braces blue (5), panels cyan (4), grids dark gray (8), labels white (7).
In the `frame` kind the columns are vertical and the beams lie at the slab
level, so open an isometric or 3D view to see the whole structure; the `plan`
kind draws everything in the XY plane for the default top view.

What to check in the CAD application, with the recorded result:

| Check | Expected | Result |
| --- | --- | --- |
| Opening | The frame opens without errors and without a repair prompt | pass |
| Layers | `STR-NODES`, `STR-BEAMS`, `STR-COLUMNS`, `STR-BRACES`, `STR-PANELS`, `STR-GRIDS` and `STR-LABELS` exist, each with its own color | pass |
| Units | `frame-m.dxf`: 5 units between adjacent columns; `frame-mm.dxf`: 5000 units | pass |
| 2D visibility | `plan-m.dxf`: four column footprints, four beams, the slab, two grids and the labels in the default top view | pass |
| Entities | `entities.dxf`: line, point, circle, arc, closed polyline with a bulge, text, solid, trace, 3D face and block insert with one attribute | pass |
| Drawing check | The CAD audit command (`AUDIT` or equivalent) reports no errors | pass |
| Round-trip | Saving again as R12 and inspecting with `dxfinspect` keeps the same entity counts | pass |

Results recorded on 2026-11-02. Update the interoperability matrix in
[docs/interoperability.md](docs/interoperability.md) when a new application or
version is tested.

## Diagnostic CLI

`dxfinspect` prints a summary of one or more DXF R12 files: version, blocks,
layers, entity counts per type and diagnostics.

```powershell
go run ./cmd/dxfinspect testdata/structural-frame.dxf
go run ./cmd/dxfinspect -lenient drawing.dxf
```

Example output:

```text
testdata/structural-frame.dxf
  version: AC1009
  blocks: 0
  layers: 1
  entities: 6
    LINE       3
    POINT      1
    POLYLINE   1
    TEXT       1
```

Install both commands with:

```bash
go install github.com/apantanella/dxf/cmd/dxfinspect@latest
go install github.com/apantanella/dxf/cmd/dxfgen@latest
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
