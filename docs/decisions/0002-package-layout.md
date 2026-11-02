# ADR 0002 — Flat layout with two public packages

Date: 2026-11-01

Status: accepted

## Context

The library has two responsibilities: the DXF format and the structural model.
The considered alternatives were:

1. Flat layout: package `dxf` at the root, package `structural` in a
   subdirectory.
2. Clean architecture: separate domain, use cases and adapters.
3. Hexagonal architecture with ports and adapters.
4. A single `dxf` package that also contains the structural types.

## Decision

The **flat layout** is adopted:

```text
github.com/apantanella/dxf            -> package dxf
github.com/apantanella/dxf/structural -> package structural
```

Rationale:

- The project is a library, not an application: it needs no use cases, services
  or dependency injection container.
- Two distinct responsibilities justify two packages, not four.
- The dependency is one-directional: `structural` imports `dxf`.
- The module root matches the main public package name, so
  `import "github.com/apantanella/dxf"` reads naturally.
- Interfaces are extracted only when a second concrete implementation exists,
  following "accept interfaces, return structs".

## Consequences

Positive:

- Immediately readable structure.
- No artificial indirection.
- Tests and fixtures live next to the code they verify.

Negative:

- If an adapter for another format is needed later, an additional package or a
  move of shared types will be required. The cost is acceptable because the
  current separation does not prevent it.

## Derived rules

- No `init()`.
- No interface with a single implementation.
- No third-party dependency in the core.
- `dxf` never imports `structural`.
- `internal/` only if code that truly cannot be exported appears.

## Verification

- `go list -deps github.com/apantanella/dxf` must not show `structural`.
- `go vet ./...` must complete without errors.
