# ADR 0009 – Versioni scrivibili e differenze tra R2000 e R2018

## Stato

Confermata, 2026-11-02 (F2, T0). Adottata da T9.

## Contesto

T1 ha provato il formato solo per AC1027 (R2013). Le versioni AC1015, AC1018, AC1021, AC1024 e AC1032 hanno la stessa struttura di sezioni, handle e marcatori per il sottoinsieme che la libreria scrive.

## Decisione

- `Encode` scrive AC1009 (R12) e tutte le versioni da AC1015 in poi: AC1015, AC1018, AC1021, AC1024, AC1027, AC1032. La versione si sceglie con `WithVersion(v)` oppure con `Document.Version`; se le due sono incoerenti con l'`$ACADVER` dell'intestazione, e' un errore di validazione.
- Le versioni AC1006, AC1012 e AC1014 (R13/R14) non si scrivono: `Encode` restituisce un errore che soddisfa sia `errors.Is(err, ErrUnsupportedVersion)` sia `errors.Is(err, ErrValidation)`.
- R2000, R2004, R2007, R2010 e R2018 differiscono da R2013 solo per `$ACADVER` e, prima di R2007, per la codifica del testo (`ANSI_1252`, ADR 0004). Il resto dei gruppi e' quello di R2013. Queste versioni sono verificate solo con ezdxf (audit), non con un CAD.
- `NewDocumentVersion(v)` crea un documento vuoto della versione richiesta. Per R2013 produce, a meno degli handle, lo stesso file di `testdata/t1/v00-baseline.dxf` (`scripts/tagdiff.py --ignore-handles`).
- Fine riga CRLF e codici su 3 colonne (`tags.WithCRLF`, `tags.WithPaddedCodes`) come in v00. I valori reali usano il formato Python (`1e+20`), accettato da ZWCAD e CadCraft nei test di T1; R12 mantiene il formato storico.

## Alternative scartate

- Scrivere solo AC1027: rifiuta inutilmente versioni identiche nella struttura.
- Scrivere R13/R14: layout diverso (nessun marcatore obbligatorio come in R2000), non provato.

## Conseguenze

- Prima di dichiarare supportata una versione diversa da R2013 serve la prova in un CAD (G1).
