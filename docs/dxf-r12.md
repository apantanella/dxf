# Supported DXF R12 subset

This document is the operational specification of the codec. Every statement here
must have a corresponding test.

## Version identification

- The version is declared in the header by the `$ACADVER` variable (group code 1).
- For R11/R12 the value is `AC1009` (source: DXF Reference, HEADER Section Group
  Codes).
- The codec rejects a document whose `$ACADVER` differs from `AC1009` with
  `ErrUnsupportedVersion`, unless a tolerant mode is explicitly requested.
- A missing `$ACADVER` is an error in strict mode and a warning in lenient mode.

## File structure

An R12 ASCII file is a sequence of `group code` / value pairs, one per line. The
R12 sections are:

| Section | Mandatory on write | Content |
| --- | --- | --- |
| `HEADER` | Yes | Version and drawing variables |
| `TABLES` | Yes | Symbol tables, including `LAYER`, `LTYPE`, `STYLE`, `APPID` |
| `BLOCKS` | Yes | Block definitions |
| `ENTITIES` | Yes | Model space entities |
| `EOF` | Yes | File terminator |

`CLASSES` and `OBJECTS` are not part of the R12 structure: the reader recognizes
them, skips them and emits an informational diagnostic.

Syntax rules:

- Every section starts with `0` / `SECTION`, then `2` / section name, and ends
  with `0` / `ENDSEC`.
- The file ends with `0` / `EOF`.
- An invalid group code or an incomplete pair produces `ErrSyntax`.
- Some historical generators pad group codes with spaces: the parser must trim
  and must not depend on the alignment.
- Both `LF` and `CRLF` line endings are accepted.
- Text values are not interpreted beyond the declared code page: R12 is not
  UTF-8.

## Handles

- In R10–R12 handles are **optional** and declared by the `$HANDLING` variable
  (1 = present, 0 or missing = absent).
- A handle is group code 5 on an entity, with a hexadecimal value unique in the
  file, up to 16 digits.
- `$HANDSEED` is a hint and may be wrong: it must not be used as a source of
  truth.
- The writer must be able to produce documents without handles.
- The reader must accept present handles even when `$HANDLING` is missing or
  zero, emitting a warning.

## Handled header variables

| Variable | Group code | Use |
| --- | --- | --- |
| `$ACADVER` | 1 | Version, always `AC1009` |
| `$INSBASE` | 10, 20, 30 | Insertion base point |
| `$EXTMIN` | 10, 20, 30 | Minimum extents |
| `$EXTMAX` | 10, 20, 30 | Maximum extents |
| `$LIMMIN` | 10, 20 | Drawing limits |
| `$LIMMAX` | 10, 20 | Drawing limits |
| `$MEASUREMENT` | 70 | 0 = imperial, 1 = metric |
| `$LUNITS` | 70 | Length display format |
| `$LUPREC` | 70 | Length precision |
| `$AUNITS` | 70 | Angle display format |
| `$AUPREC` | 70 | Angle precision |
| `$CLAYER` | 8 | Current layer |
| `$CELTYPE` | 6 | Current linetype |
| `$CECOLOR` | 62 | Current color |
| `$TEXTSTYLE` | 7 | Current text style |
| `$HANDLING` | 70 | Handle presence (optional) |
| `$HANDSEED` | 5 | Handle seed (optional) |
| `$DWGCODEPAGE` | 3 | Drawing code page |

`$INSUNITS` **does not exist in R12**: coordinates have no declared unit.
`$MEASUREMENT` controls hatch patterns and linetypes, not coordinates. See
[interoperability.md](interoperability.md).

Unhandled variables are preserved in `Header` as `(name, group code, value)`
tuples and written back unchanged.

## Common entity group codes

| Group code | Meaning | Default when absent |
| --- | --- | --- |
| `0` | Entity type | mandatory |
| `5` | Handle | absent in R12 |
| `8` | Layer | `0` |
| `6` | Linetype | `BYLAYER` |
| `62` | ACI color | `BYLAYER` |
| `39` | Thickness | 0 |
| `60` | Visibility: 0 = visible, 1 = invisible | 0 |
| `210`, `220`, `230` | Extrusion direction | (0, 0, 1) |
| `1001` | Start of XDATA with the application name | absent |

Group codes that are not handled on known entities are preserved in
`EntityCommon.Raw` and written back, unless an ADR decides otherwise.

## Supported entities

### `POINT`

| Group code | Meaning |
| --- | --- |
| `10`, `20`, `30` | Position |
| `50` | Symbol angle |
| `210`, `220`, `230` | Extrusion |

### `LINE`

| Group code | Meaning |
| --- | --- |
| `10`, `20`, `30` | Start point |
| `11`, `21`, `31` | End point |
| `39` | Thickness |
| `210`, `220`, `230` | Extrusion |

### `CIRCLE`

| Group code | Meaning |
| --- | --- |
| `10`, `20`, `30` | Center |
| `40` | Radius, must be greater than zero |
| `210`, `220`, `230` | Extrusion |

### `ARC`

| Group code | Meaning |
| --- | --- |
| `10`, `20`, `30` | Center |
| `40` | Radius |
| `50` | Start angle in degrees |
| `51` | End angle in degrees |
| `210`, `220`, `230` | Extrusion |

### `POLYLINE`, `VERTEX`, `SEQEND`

`POLYLINE` is the R12 polyline: `LWPOLYLINE` does not exist in this version.

| Group code | Entity | Meaning |
| --- | --- | --- |
| `66` | `POLYLINE` | 1 = vertices follow |
| `70` | `POLYLINE` | Flags: 1 = closed, 8 = 3D, 16 = mesh |
| `10`, `20`, `30` | `POLYLINE` | Elevation point (often 0,0,0) |
| `40`, `41` | `POLYLINE` | Default start and end width |
| `10`, `20`, `30` | `VERTEX` | Vertex position |
| `42` | `VERTEX` | Bulge for an arc segment |
| `70` | `VERTEX` | Vertex flags |
| `71`–`74` | `VERTEX` | Vertex indices for meshes |
| `0` | `SEQEND` | Closes the vertex sequence |

The sequence is always `POLYLINE`, one or more `VERTEX`, `SEQEND`. A `VERTEX`
outside a `POLYLINE` produces `ErrSyntax`. 2D polylines use the OCS and may have
an extrusion; 3D polylines (flag `70` bit 8) use the WCS.

### `TEXT`

| Group code | Meaning |
| --- | --- |
| `1` | Text, mandatory |
| `10`, `20`, `30` | Insertion point |
| `40` | Height, must be greater than zero |
| `41` | X scale factor |
| `50` | Rotation in degrees |
| `51` | Oblique angle in degrees |
| `7` | Text style |
| `71` | Generation flags |
| `72` | Horizontal alignment |
| `73` | Vertical alignment |
| `11`, `21`, `31` | Alignment point when `72` or `73` are non-zero |

The text value is preserved verbatim: leading and trailing spaces survive a
round-trip.

### `SOLID` and `TRACE`

Four corners in DXF order:

| Group code | Meaning |
| --- | --- |
| `10`, `20`, `30` | First corner |
| `11`, `21`, `31` | Second corner |
| `12`, `22`, `32` | Third corner |
| `13`, `23`, `33` | Fourth corner |
| `39` | Thickness |
| `210`, `220`, `230` | Extrusion, optional |

The four corners are in OCS; the default extrusion is `(0, 0, 1)`.

### `3DFACE`

Same four corners as `SOLID`, plus:

| Group code | Meaning |
| --- | --- |
| `70` | Edge invisibility flags |

The four corners are in WCS and there is no extrusion group code.

### `INSERT`

| Group code | Meaning |
| --- | --- |
| `2` | Block name, mandatory |
| `10`, `20`, `30` | Insertion point |
| `41`, `42`, `43` | X, Y, Z scale factors |
| `50` | Rotation in degrees |
| `70`, `71` | Column and row counts |
| `44`, `45` | Column and row spacing |
| `66` | 1 = attributes follow |

When `66` is 1, the `ATTRIB` records are followed by `SEQEND`. An `INSERT` that
references a missing block is a validation error on write and a warning on read.

## Tables

Expected tables: `LAYER`, `LTYPE`, `STYLE`, `APPID`, plus `VPORT` and `VIEW` on
read. Every table starts with `0` / table name, contains records and ends with
`0` / `ENDTAB`.

`LAYER` record:

| Group code | Meaning |
| --- | --- |
| `2` | Layer name |
| `70` | Flags: 1 = frozen, 2 = frozen in the viewport, 4 = locked |
| `62` | ACI color; a negative value means the layer is off |
| `6` | Linetype |

The writer guarantees that layer `0` and every layer referenced by an entity
exist. Missing layers are added with color 7 and the `CONTINUOUS` linetype. The
`LTYPE` and `STYLE` tables always contain `CONTINUOUS` and `STANDARD`.

## Coordinate systems

- Planar entities store points in **OCS** and may have an extrusion: `CIRCLE`,
  `ARC`, `TEXT`, `SOLID`, `TRACE`, `INSERT`, 2D `POLYLINE` and 2D `VERTEX`.
- 3D entities use the **WCS**: `LINE`, `POINT`, `3DFACE`, 3D `POLYLINE`
  (flag `70` bit 8) and 3D `VERTEX`. `LINE` and `POINT` may have an extrusion
  that defines their thickness, but the coordinates remain in WCS.
- `3DFACE` has no extrusion group code: its four corners are always in WCS.

Arbitrary axis algorithm, with `N` the extrusion normal:

1. If `|Nx| < 1/64` and `|Ny| < 1/64`, then `Ax = Wy × N` with `Wy = (0,1,0)`.
2. Otherwise `Ax = Wz × N` with `Wz = (0,0,1)`.
3. Normalize `Ax`.
4. `Ay = N × Ax`, then normalize `Ay`.
5. The matrix `[Ax Ay N]` maps from OCS to WCS.

Angles are always in degrees, counter-clockwise, unless stated otherwise.

The structural import does not apply this conversion yet: entities with a
non-default extrusion produce an ambiguity diagnostic and keep their stored
coordinates.

## Round-trip

The criterion is the **semantic** round-trip: `Decode -> Encode -> Decode` must
produce an equivalent document in terms of entities, layers, tables, blocks and
handled header variables. Byte-for-byte equality is not a criterion.

The writer is also idempotent on its own output:
`Encode(Decode(Encode(doc))) == Encode(doc)`.

## References

- [DXF Reference, R12](https://damassets.autodesk.net/content/dam/autodesk/www/developer-network/platform-technologies/autocad-dxf-archive/acad_r12_dxf.pdf)
- [HEADER Section Group Codes](https://help.autodesk.com/cloudhelp/2024/ENU/AutoCAD-DXF/files/GUID-A85E8E67-27CD-4C59-BE61-4DC9FADBE74A.htm)
- [Arbitrary Axis Algorithm](https://help.autodesk.com/cloudhelp/2024/ENU/AutoCAD-DXF/files/GUID-E19E5B42-0CC7-4EBA-B29F-5E1D595149EE.htm)
- [ezdxf, Handles](https://ezdxf.readthedocs.io/en/stable/dxfinternals/handles.html)


