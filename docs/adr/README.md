# Architecture Decision Records

Indice unico delle decisioni di progetto. Le decisioni di F1 (nucleo R2013) nascono dal documento "Specifiche F1 – Nucleo R2013", sezione "Decisioni da confermare (ADR)". Una decisione proposta e' adottata in via provvisoria dai task in corso finche' non viene confermata o sostituita.

Le cartelle sono due: `docs/decisions/` raccoglie gli ADR delle fasi 0-6 (formato R12; indice in [`../decisions/README.md`](../decisions/README.md)), `docs/adr/` raccoglie quelli di F1 (nucleo R2013). La numerazione delle due serie e' indipendente (entrambe partono da 0001): citare sempre la cartella. L'ADR 0001 di `decisions` (solo R12) e' superato per le versioni supportate dagli ADR 0003 e 0009 di questa cartella.

| N. | Titolo | Stato |
|----|--------|-------|
| [0001](0001-tipo-del-tag.md) | Tipo del tag: `tags.Tag{Code int; Value string}` | Confermata (2026-11-02) |
| [0002](0002-evoluzione-api-sul-posto.md) | Evoluzione dell'API sul posto, senza modulo v2 | Confermata (2026-11-02) |
| [0003](0003-modello-unico-per-tutte-le-versioni.md) | Un solo modello per tutte le versioni | Confermata (2026-11-02) |
| [0004](0004-codifica-dei-caratteri.md) | Codifica dei caratteri | Confermata (2026-11-02) |
| [0005](0005-definizione-di-round-trip.md) | Definizione di round-trip | Confermata (2026-11-02) |
| [0006](0006-handle.md) | Handle | Confermata (2026-11-02) |
| [0007](0007-contenuto-non-interpretato.md) | Contenuto non interpretato: `RawEntity` e `RawObject` | Confermata (2026-11-02) |
| [0008](0008-dxf-binario-non-supportato.md) | DXF binario non letto in F1 | Confermata (2026-11-02) |
| [0009](0009-versioni-scrivibili.md) | Versioni scrivibili e differenze tra R2000 e R2018 | Confermata (2026-11-02) |
| [0010](0010-entita-di-f2.md) | Entità di F2: colore vero, spessore, entità nuove (D9–D14, D15, D16) | Proposta (2026-11-02), adottata in via provvisoria |

Formato di ogni ADR: Titolo, Stato, Contesto, Decisione, Alternative scartate, Conseguenze. Un ADR confermato non si modifica: una decisione diversa si registra con un nuovo ADR che lo sostituisce.

## Documenti collegati

- [`../architecture.md`](../architecture.md): come le decisioni si riflettono nei package.
- [`../interoperability.md`](../interoperability.md): versioni scrivibili, codifica, DXF binario (ADR 0004, 0008, 0009).
- [`../migrazione-f1.md`](../migrazione-f1.md): effetti sull'API di F0 (ADR 0001, 0002, 0006).
- [`../dxf-r2013.md`](../dxf-r2013.md): struttura del file R2013 scritto dalla libreria.
- [`../f1-task-t3-t9.md`](../f1-task-t3-t9.md): schede dei task T3-T9.
- [`../roadmap.md`](../roadmap.md): stato delle fasi.
