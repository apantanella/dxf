# ADR 0002 – Evoluzione dell'API sul posto, senza modulo v2

## Stato

Confermata, 2026-11-02.

## Contesto

F1 aggiunge al package `dxf` handle, owner, tabelle tipizzate, CLASSES, OBJECTS e la scrittura R2013. Si poteva farlo nel package esistente oppure in un nuovo modulo `github.com/apantanella/dxf/v2`. La scelta dipendeva da una domanda aperta: esiste già una versione 1.x pubblicata, con utenti che si aspettano stabilità? Risposta del 2 novembre 2026: no, non esiste alcuna versione 1.x pubblicata.

## Decisione

- Il package `dxf` si evolve sul posto, nel modulo `github.com/apantanella/dxf`.
- Le modifiche restano compatibili con l'API attuale: `Decode`, `Encode`, `NewDocument`, `Document.Entities`, `Tables.EnsureLayer`, `EnsureLinetype`, `EnsureStyle`, `EnsureAppID` non cambiano firma. I nuovi elementi sono aggiunte (campi, tipi, opzioni).
- Nessun modulo `v2`.
- `NewDocument()` resta R12 finché F1 non è chiusa; poi diventa R2013, con una voce di CHANGELOG. Per creare un documento di versione esplicita si usa `NewDocumentVersion(v)`.

## Alternative scartate

- Nuovo modulo `v2`: duplica il lavoro di F0 (incluso `regression_f0_test.go`) e obbliga il package `structural` a migrare subito. Senza una 1.x pubblicata non c'è nessun utente da proteggere con un cambio di percorso di import.

## Conseguenze

- La suite di regressione F0 (`regression_f0_test.go`) e i test esistenti devono restare verdi per tutta F1 senza modifiche alle attese (R13).
- Un cambio di tipo di un campo esistente è un'incompatibilità. Caso concreto da risolvere: `EntityCommon.Handle` e `Attribute.Handle` oggi sono `string`, mentre ADR 0006 propone `type Handle uint64` (vedi ADR 0006, conseguenze).
- Il passaggio di `NewDocument()` a R2013 cambia l'output di chi lo usa senza opzioni: va segnalato nel CHANGELOG alla chiusura di F1.
