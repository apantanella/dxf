# Interoperability

## Units

DXF coordinates carry no intrinsic unit. R12 does not contain `$INSUNITS`;
`$MEASUREMENT` concerns hatch patterns and linetypes, not the coordinate scale.

Operational consequences:

1. The unit is an explicit parameter of the application, never an inference.
2. `structural.Model.Unit` declares the unit of the model.
3. `structural.Mapping.Unit` declares the unit expected in imported and exported
   documents.
4. Conversion happens only between known units; with `UnitUnknown` the operation
   fails.
5. Exported files must document the convention in the caller project README, not
   in the DXF.

Factors relative to the meter:

| Unit | Factor |
| --- | --- |
| Millimeter | 0.001 |
| Centimeter | 0.01 |
| Meter | 1 |
| Inch | 0.0254 |
| Foot | 0.3048 |

Do not mix units in the same project. If a block uses different units from the
model, the scale must be applied explicitly through `INSERT` (`41`, `42`, `43`);
no CAD application applies implicit conversions.

## Text encoding

- R12 is not UTF-8: text depends on the system code page.
- The first release handles ASCII text. When a file contains non-ASCII bytes,
  `Decode` records one warning that gives the number of values and the line of
  the first one; the bytes are preserved unchanged.
- `$DWGCODEPAGE` is preserved without automatic conversion in R12.
- Full code page support requires a dedicated ADR.

F1 (ADR 0004 in `docs/adr`):

- From R2007 the files are UTF-8, read and written.
- Writing R2000 and R2004 converts to the code page of `$DWGCODEPAGE`
  (`ANSI_1252` or `ISO8859-1`; `ANSI_1252` when the variable is missing). A
  character the code page cannot represent is written as `\U+XXXX`. Other code
  pages are not translated.
- Reading before R2007 does not convert: the bytes are kept and a warning is
  recorded, as for R12. `\U+XXXX` sequences in a file are not decoded.
- `$DWGCODEPAGE` is always written, also from R2007, where it does not govern
  the encoding. There is no dependency on `golang.org/x/text`.
- Binary DXF is not supported (ADR 0008): `Decode` returns an error that
  satisfies `errors.Is(err, ErrNotDXF)` and says the file is binary.

## Numbers and precision

- Real values are written with the fewest digits that preserve the round-trip,
  using `strconv.FormatFloat` with format `'f'` and precision `-1`, or the
  configured precision.
- The decimal separator is always the point, never the locale separator.
- Non-finite values (`NaN`, `Inf`) are a validation error.
- Precision must not change topology: the node merge tolerance is a parameter,
  not a hidden constant.

## Writable versions (F1)

`Encode` writes `AC1009` (R12) and `AC1015`, `AC1018`, `AC1021`, `AC1024`,
`AC1027` (R2013) and `AC1032`. `AC1006`, `AC1012` and `AC1014` are not written:
the error satisfies both `errors.Is(err, ErrUnsupportedVersion)` and
`errors.Is(err, ErrValidation)` (ADR 0009). `Decode` accepts the nine versions
from `AC1009` to `AC1032`; R13 and R14 on a best-effort basis.

How each is verified:

| Version | Verification |
| --- | --- |
| R12 | F0 regression suite and the matrix below |
| R2013 | Structure from the T1 reference files, opened in ZWCAD 2023 and CadCraft 0.16.0 ([dxf-r2013.md](dxf-r2013.md)); written files audited with ezdxf; G1 (hands-on check of the written files, `testdata/g1/README.md`) **not done yet** |
| R2000, R2004, R2007, R2010, R2018 | ezdxf audit only; they differ from R2013 in `$ACADVER` and, before R2007, in the text encoding. Not declared supported until opened in a CAD |

ezdxf is permissive: a clean audit is necessary, not sufficient
([oracle.md](oracle.md)). The CAD applications available for F1 are ZWCAD 2023
and CadCraft 0.16.0; the rows of the matrix below were not produced by the F1
checks.

Format differences between the R12 and the R2000+ writers:

| Aspect | R12 | R2000 to R2018 |
| --- | --- | --- |
| Line ending | LF | CR LF |
| Group codes | not aligned | right-aligned on three columns |
| Reals | format `'f'` (`1e20` becomes `100000000000000000000`) | Python style: `0.0`, `1.34`, `1e+20` |
| Handles, owners | only if the document uses them | always |
| Subclass markers (`100`) | none | always; without them ZWCAD shows an empty drawing (T1, variant v06) |
| Sections | `HEADER`, `TABLES`, `BLOCKS`, `ENTITIES` | plus `CLASSES` and `OBJECTS` |

Number formatting is an allowed normalization of the round-trip criterion
(ADR 0005): `1.0` and `1` are the same value. Files written by the library
satisfy `Encode(Decode(Encode(doc)))` byte-identical to `Encode(doc)`; for files
from other CAD applications compare with
`python3 scripts/tagdiff.py --ignore-handles`.

## CAD application compatibility

Manual verification matrix for every release:

| Application | Version | Check | Result |
| --- | --- | --- | --- |
| LibreCAD | 2.x | open, layers, entities, units | pass |
| QCAD | 3.x | open, layers, entities, units | pass |
| BricsCAD | current | open, `AUDIT`, entities | pass |
| AutoCAD | current | open, `AUDIT`, entities | pass |

Results recorded on 2026-11-02. Update this table when a new application or
version is tested.

## Generating files for verification

`cmd/dxfgen` writes the files to open in a CAD application:

```powershell
go run ./cmd/dxfgen -out frame-m.dxf -kind frame -unit m
go run ./cmd/dxfgen -out frame-mm.dxf -kind frame -unit mm
go run ./cmd/dxfgen -out plan-m.dxf -kind plan -unit m
go run ./cmd/dxfgen -out entities.dxf -kind entities
```

Checklist for the matrix above:

| Check | How | Expected | Result |
| --- | --- | --- | --- |
| Opening | Open `frame-m.dxf` | No error, no repair prompt | pass |
| Layers | Layer list | `STR-NODES`, `STR-BEAMS`, `STR-COLUMNS`, `STR-BRACES`, `STR-PANELS`, `STR-GRIDS`, `STR-LABELS` | pass |
| Colors | Layer properties | Nodes 2, beams 1, columns 3, braces 5, panels 4, grids 8, labels 7 | pass |
| Units | Measure between two columns in `frame-m.dxf` | 5 drawing units | pass |
| Unit conversion | Measure between two columns in `frame-mm.dxf` | 5000 drawing units | pass |
| 2D visibility | Open `plan-m.dxf` in top view | Four column footprints, four beams, slab, two grids, labels | pass |
| Entities | Open `entities.dxf` | Line, point, circle, arc, closed polyline with a bulge, text, solid, trace, 3D face, block insert with one attribute | pass |
| Drawing check | Run `AUDIT` or equivalent | No errors | pass |
| Round-trip | Save again as R12, then `go run ./cmd/dxfinspect saved.dxf` | Same entity counts as `dxfinspect entities.dxf` | pass |

Note: `frame` is a 3D structure. In the default top view the columns project to
points and the beams coincide with the slab level, so use an isometric view or
the `plan` kind for a 2D check.

Procedure:

1. Open the file produced by `structural.Export` or by `dxfgen`.
2. Run the drawing check command (`AUDIT` or equivalent).
3. Check layers, colors and linetypes.
4. Measure the distance between two known nodes and compare it with the model.
5. Save with the same CAD application in R12 and run the read round-trip.
6. Record every observed difference in `CHANGELOG.md` and in the matrix above.

## Known limitations

| Limitation | Effect |
| --- | --- |
| No `LWPOLYLINE` | Boundaries use `POLYLINE`/`VERTEX` |
| No `SPLINE` | Complex curves are not represented |
| No `MTEXT` | Labels use `TEXT` |
| No dimension entity support | Dimensions remain unsupported entities |
| No georeferencing | The model uses local coordinates |
| No `OBJECTS` | Not applicable to R12; from R2000 `OBJECTS` is read and written |
| No mandatory handle | In R12 references between entities are unavailable; from R2000 handles are always written |
| Binary DXF | Not supported (ADR 0008) |
| Text before R2007 | Not converted on reading (the bytes are kept) |
| R13, R14 | Read only, best effort |
| Entities not interpreted in F1 | `LWPOLYLINE`, `MTEXT`, `HATCH`, `DIMENSION` and others are kept as `RawEntity` and not generated |
| OCS conversion limited to imported entities | Circles, arcs, solids, traces and inserts are not converted |

## Automatic round-trip verification

Every round-trip test must check:

1. Number and type of entities.
2. Layer of every entity.
3. Coordinates within a tolerance declared by the test.
4. Handled header variables.
5. Table records.
6. Block definitions.
7. Absence of unexpected diagnostics.

## Reference files

Fixtures in `testdata/` must have declared provenance:

- Files generated by the library: freely verifiable and modifiable.
- Files produced by external CAD applications: state application, version and
  date.
- Downloaded files: state URL and license.

Do not add third-party files without a license compatible with GPL-3.0.
