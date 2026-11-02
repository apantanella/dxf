# ADR 0003 – Un solo modello per tutte le versioni

## Stato

Proposta, 2026-11-02. Da confermare. Adottata in via provvisoria dalle fasi di sviluppo in corso.

## Contesto

La libreria legge da R12 (AC1009) a R2018 (AC1032) e scrive R12 e R2013 (AC1027). Le versioni differiscono per la struttura del file (CLASSES, OBJECTS, BLOCK_RECORD, marcatori di sottoclasse, owner) più che per la geometria delle entità supportate. Bisogna decidere se il documento in memoria è unico o ha una forma per versione.

## Decisione

- Un solo tipo `Document` per tutte le versioni.
- `Document.Version` (valore di `$ACADVER`, per esempio `VersionR12 = "AC1009"`, `VersionR2013 = "AC1027"`) decide in scrittura quali marcatori, sezioni e oggetti vengono emessi.
- `Encode` accetta `WithVersion(v)` per scrivere una versione diversa da `Document.Version` senza modificare il documento.
- `NewDocumentVersion(v)` crea un documento vuoto con le tabelle e gli oggetti minimi della versione.
- I campi introdotti da R2000 in poi (`Classes`, `Objects`, `Tables.BlockRecords`, handle di owner, sottoclassi lette) restano vuoti o ignorati in un documento R12 e vengono generati da `Encode` quando servono.

## Alternative scartate

- Un tipo per versione (per esempio `DocumentR12` e `DocumentR2013`): raddoppia i test, duplica le API di disegno e impedisce di convertire R12 in R2013 con un semplice `doc.Version = VersionR2013; Encode(...)`.

## Conseguenze

- La conversione tra versioni è un'operazione di scrittura: lettura con `Decode`, cambio di `Version` (o `WithVersion`), `Encode`.
- `Encode` deve verificare che il contenuto sia rappresentabile nella versione richiesta e restituire `ErrValidation` indicando l'elemento (per esempio una entità introdotta dopo R12 scritta come R12).
- Scrivere R12 da un documento letto come R2013 perde le informazioni che R12 non rappresenta (CLASSES, OBJECTS, sottoclassi, owner). L'elenco esatto di cosa si scarta e se va segnalato con un avviso è da definire in T9.
- Il writer ha un ramo per famiglia di versioni (R12 e R2000+); la scrittura R12 esistente resta invariata (R13).
