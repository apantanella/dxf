# ADR 0005 – Definizione di round-trip

## Stato

Confermata, 2026-11-02.

## Contesto

Il piano chiedeva un round-trip "byte-identico". Su file prodotti da altri CAD questo non è ottenibile: ogni programma formatta i numeri a modo suo, alcuni omettono handle, l'ordine delle variabili di intestazione non è garantito (il `Header` attuale è una mappa e il writer R12 scrive le variabili in ordine alfabetico). Serve un criterio verificabile da test.

## Decisione

Due criteri distinti, secondo l'origine del file:

1. File prodotti dalla libreria: `Encode(Decode(Encode(doc)))` è byte-identico a `Encode(doc)`. In altre parole, dalla seconda scrittura in poi l'output non cambia più.
2. File prodotti da altri CAD: dopo `Decode` e `Encode`, la sequenza di tag riletta è uguale a quella del file originale, a meno delle normalizzazioni elencate:
   - formato dei numeri (stesso valore, testo diverso: per esempio `1.0` e `1`, `1e+20` e `100000000000000000000`);
   - handle mancanti assegnati (tag `5`, o `105` per DIMSTYLE, aggiunti dove l'originale non li aveva) e conseguente `$HANDSEED`;
   - ordine delle variabili di intestazione.

Ogni altra differenza è un difetto.

## Alternative scartate

- Byte-identico anche sui file esterni: impossibile per la formattazione dei numeri, e costringerebbe a conservare il testo originale di ogni valore anche dopo una modifica.

## Conseguenze

- Il test di round-trip (T10) confronta sequenze di `tags.Tag` dopo una funzione di normalizzazione esplicita, non i byte. La normalizzazione è codice di test, con le tre regole sopra e nient'altro.
- Il criterio 1 richiede che la scrittura sia deterministica: ordine di tabelle, record, oggetti e variabili stabile; handle assegnati in ordine fisso; nessuna dipendenza dall'ordine di iterazione delle mappe.
- Le normalizzazioni ammesse vanno mantenute in un elenco chiuso: aggiungerne una è una modifica a questo ADR.
- Il criterio si applica anche ai contenuti non interpretati (ADR 0007).
