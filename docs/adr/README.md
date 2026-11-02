# Architecture Decision Records

Decisioni che condizionano la fase F1 (nucleo R2013). Fonte: documento "Specifiche F1 – Nucleo R2013", sezione "Decisioni da confermare (ADR)". Le decisioni proposte sono adottate in via provvisoria dai task in corso finché non vengono confermate o sostituite.

| N. | Titolo | Stato |
|----|--------|-------|
| [0001](0001-tipo-del-tag.md) | Tipo del tag: `tags.Tag{Code int; Value string}` | Proposta (2026-11-02), adottata in via provvisoria |
| [0002](0002-evoluzione-api-sul-posto.md) | Evoluzione dell'API sul posto, senza modulo v2 | Confermata (2026-11-02) |
| [0003](0003-modello-unico-per-tutte-le-versioni.md) | Un solo modello per tutte le versioni | Proposta (2026-11-02), adottata in via provvisoria |
| [0004](0004-codifica-dei-caratteri.md) | Codifica dei caratteri | Confermata (2026-11-02) |
| [0005](0005-definizione-di-round-trip.md) | Definizione di round-trip | Confermata (2026-11-02) |
| [0006](0006-handle.md) | Handle | Proposta (2026-11-02), adottata in via provvisoria |
| [0007](0007-contenuto-non-interpretato.md) | Contenuto non interpretato: `RawEntity` e `RawObject` | Proposta (2026-11-02), adottata in via provvisoria |
| [0008](0008-dxf-binario-non-supportato.md) | DXF binario non letto in F1 | Confermata (2026-11-02) |

| [0009](0009-versioni-scrivibili.md) | Versioni scrivibili e differenze tra R2000 e R2018 | Proposta (2026-11-02), adottata in via provvisoria |

Formato di ogni ADR: Titolo, Stato, Contesto, Decisione, Alternative scartate, Conseguenze. Un ADR confermato non si modifica: una decisione diversa si registra con un nuovo ADR che lo sostituisce.

Documenti collegati:

- [`../dxf-r2013.md`](../dxf-r2013.md): struttura del file R2013 scritto dalla libreria.
- [`../f1-task-t3-t9.md`](../f1-task-t3-t9.md): schede dei task T3–T9.
