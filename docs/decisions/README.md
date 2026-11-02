# Architecture decision records

Every structural or format decision that constrains future development is
recorded in this directory as a numbered file.

| ADR | Decision | Status |
| --- | --- | --- |
| [0001](0001-r12-ascii.md) | DXF R12 ASCII as the initial format | accepted |
| [0002](0002-package-layout.md) | Flat layout with two public packages | accepted |

The decisions of phase 7 (F1, R2013 core) are in [../adr/README.md](../adr/README.md).
That directory has its own numbering, which starts again at 0001; ADR 0001 here
(R12 only) is partly superseded for the version scope by ADR 0003 and 0009 there.

## Format

An ADR contains:

1. Date and status.
2. Context and considered alternatives.
3. Decision.
4. Positive and negative consequences.
5. Verification: how the decision is checked.

An accepted ADR is not modified: if the decision changes, a new ADR supersedes
the previous one and the old status is updated.
