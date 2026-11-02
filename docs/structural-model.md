# Structural model

This document defines the content of the `structural` package and the conversion
rules to and from DXF. The model is independent from the format: it contains no
group codes, handles or section names.

## What it is and what it is not

It is a geometric-structural model: it describes the position of elements and
their connections. It is not a calculation model. Materials, cross sections,
supports, loads and combinations do not belong to this package: they will be
objects of a separate package that consumes `structural`.

## Types

### `Model`

| Field | Type | Description |
| --- | --- | --- |
| `Nodes` | `[]Node` | Model nodes |
| `Members` | `[]Member` | Linear elements |
| `Panels` | `[]Panel` | Surfaces |
| `Grids` | `[]Grid` | Reference axes |
| `Unit` | `Unit` | Unit declared by the application |

`Unit` is an explicit value: the library never infers the unit from the DXF.
Supported values: `UnitMillimeter`, `UnitCentimeter`, `UnitMeter`, `UnitInch`,
`UnitFoot` and `UnitUnknown`. A model with `UnitUnknown` can be read and
rewritten, but not converted.

### `Node`

| Field | Type | Description |
| --- | --- | --- |
| `ID` | `string` | Unique identifier in the model |
| `Position` | `Point3D` | Coordinates in the model system |
| `Label` | `string` | Optional label |

Two nodes with the same position within the model tolerance are the same node.
The tolerance is a mapping parameter.

### `Member`

| Field | Type | Description |
| --- | --- | --- |
| `ID` | `string` | Unique identifier |
| `Start`, `End` | `string` | IDs of the end nodes |
| `Kind` | `MemberKind` | Beam, column, brace or generic |
| `Label` | `string` | Optional label |

Invariants:

- `Start` and `End` must exist in `Model.Nodes`.
- `Start` must differ from `End`.
- The length must be greater than the tolerance.

### `MemberKind`

| Value | Meaning |
| --- | --- |
| `MemberGeneric` | Unclassified linear element |
| `MemberBeam` | Beam, mostly horizontal |
| `MemberColumn` | Column, mostly vertical |
| `MemberBrace` | Brace |

`MemberGeneric` is the zero value and implies no classification. Automatic
classification by orientation is a mapping choice and must be documented: it is
not an intrinsic property of the geometry.

### `Panel`

| Field | Type | Description |
| --- | --- | --- |
| `ID` | `string` | Unique identifier |
| `Boundary` | `[]Point3D` | Closed boundary, at least three points |
| `Kind` | `PanelKind` | Wall, slab or generic |
| `Label` | `string` | Optional label |

The boundary is implicitly closed: the last point does not repeat the first. A
boundary with coincident or degenerate coplanar points is a validation error.

### `Grid`

| Field | Type | Description |
| --- | --- | --- |
| `ID` | `string` | Axis identifier |
| `Start`, `End` | `Point3D` | Axis endpoints |
| `Label` | `string` | Label, for example `A` or `1` |

### `Report`

Import result:

| Field | Type | Description |
| --- | --- | --- |
| `Skipped` | `int` | Skipped entities |
| `Ambiguous` | `int` | Ambiguously classified entities |
| `Diagnostics` | `[]Diagnostic` | Per-entity detail |

Import does not fail for a single uninterpretable entity: it records it in the
report. It fails only when the document is invalid or the mapping is
inconsistent.

## Mapping

`Mapping` is the only place where geometry is given meaning.

| Field | Type | Description |
| --- | --- | --- |
| `NodeLayers` | `[]string` | Layers that contain nodes |
| `MemberLayers` | `[]string` | Layers that contain linear elements |
| `PanelLayers` | `[]string` | Layers that contain panel boundaries |
| `GridLayers` | `[]string` | Layers that contain axes |
| `LabelLayers` | `[]string` | Layers that contain `TEXT` labels |
| `BeamLayers` | `[]string` | Subset of members interpreted as beams |
| `ColumnLayers` | `[]string` | Subset of members interpreted as columns |
| `BraceLayers` | `[]string` | Subset of members interpreted as braces |
| `SnapTolerance` | `float64` | Node merge tolerance |
| `ClassifyByOrientation` | `bool` | When set, classifies members by orientation |
| `LabelHeight` | `float64` | Label height, in model units |
| `Unit` | `Unit` | Unit expected for imported and exported documents |

`DefaultMapping` returns the conventions described in the next section. A layer
that is not listed in the mapping produces no structural elements.

Labels are imported only when their position coincides with a node or with the
endpoint of an axis within `SnapTolerance`; otherwise they produce a diagnostic
without counting as skipped entities.

## Default export conventions

| Element | Default layer | DXF entity | ACI color |
| --- | --- | --- | --- |
| Nodes | `STR-NODES` | `POINT` | 2 (yellow) |
| Beams | `STR-BEAMS` | `LINE` | 1 (red) |
| Columns | `STR-COLUMNS` | `LINE` | 3 (green) |
| Braces | `STR-BRACES` | `LINE` | 5 (blue) |
| Generic members | `STR-MEMBERS` | `LINE` | 6 (magenta) |
| Panels | `STR-PANELS` | Closed `POLYLINE` | 4 (cyan) |
| Axes | `STR-GRIDS` | `LINE` | 8 (dark gray) |
| Labels | `STR-LABELS` | `TEXT` | 7 (white/black) |

Labels use `TEXT` with height `Mapping.LabelHeight`, converted to the document
unit. The conventions can be changed: they are a default, not a format
constraint.

## API

```go
func Import(doc *dxf.Document, mapping Mapping) (*Model, Report, error)
func Export(model *Model, mapping Mapping) (*dxf.Document, error)
```

`Export` validates the model with `Model.Validate` and returns a modifiable
document, to be passed to `dxf.Encode`.

## Import rules

1. Determine the unit from the mapping. With `UnitUnknown`, convert nothing and
   report it.
2. Collect nodes from the `NodeLayers` layers and from member endpoints.
3. Merge nodes within `SnapTolerance` and keep the first identifier.
4. Build members from the `MemberLayers` layers, with classification by layer or,
   when enabled, by orientation.
5. Build panels from the `PanelLayers` layers, accepting only closed boundaries
   with at least three distinct points.
6. Build axes from the `GridLayers` layers.
7. Attach `TEXT` labels from the `LabelLayers` layers to the matching node or
   axis.
8. Record every excluded entity and the reason in the report.

Known limitation: the OCS → WCS conversion covers 2D polylines and `TEXT`
positions, which are the entities the structural import reads. Entities that are
skipped by the import (circles, arcs, solids, traces, inserts) are not converted.

## Export rules

1. Validate the model before conversion.
2. Convert coordinates from the model unit to the document unit.
3. Create the required layers with distinct, documented colors.
4. Emit nodes, members, panels, axes and labels.
5. Do not emit handles unless explicitly requested.
6. Guarantee that every referenced layer exists in the `LAYER` table.

## Why XDATA is not used in the first release

XDATA (`1001`, `1000`–`1071`) is supported by R12, but it introduces a dependency
on a registered `APPID` and on a proprietary convention. In the first release the
semantics are expressed by layers and blocks, which any CAD application can read.
Using XDATA requires a dedicated ADR.

## Validation errors

| Condition | Outcome |
| --- | --- |
| Member with a missing node | error |
| Member with coincident ends | error |
| Panel with fewer than three points | error |
| Panel with consecutive duplicate points | error |
| `UnitUnknown` during conversion | error |
| Entity not in the mapping | skipped and recorded in the report |
| Duplicate node within tolerance | merged, with a diagnostic |

