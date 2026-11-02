# ADR 0004 – Codifica dei caratteri

## Stato

Confermata, 2026-11-02.

## Contesto

Le tavole contengono testo italiano (`à è ì ò ù`) e simboli tecnici (`° ± Ø`). Fino a R2004 (AC1018) i file DXF usano la code page indicata da `$DWGCODEPAGE`; da R2007 (AC1021) il file è UTF-8. La libreria oggi non ha dipendenze esterne. Il reader attuale (`tags.Reader.NonASCII`) conta già i valori con byte ≥ 0x80, ma non li converte.

## Decisione

- R2007 e successivi: lettura e scrittura in UTF-8.
- Prima di R2007: la code page si ricava da `$DWGCODEPAGE`. La libreria ha tabelle interne per CP1252 (`ANSI_1252`) e Latin-1 (ISO 8859-1). Qualsiasi altra code page produce un avviso diagnostico e i byte vengono letti con una regola documentata (da fissare in T4).
- Zero dipendenze esterne: niente `golang.org/x/text`.
- In scrittura, un carattere non rappresentabile nella code page di destinazione si scrive come sequenza `\U+XXXX` (quattro cifre esadecimali maiuscole del code point).
- `$DWGCODEPAGE` si scrive sempre, anche nei file R2007+ dove non governa la codifica.

## Alternative scartate

- Dipendere da `golang.org/x/text/encoding/charmap`: copre tutte le code page, ma è la prima dipendenza di una libreria che oggi non ne ha. Le code page diverse da CP1252 e Latin-1 non servono ai casi d'uso attuali.

## Conseguenze

- Una stringa con `à è ì ò ù ° ±` deve fare il giro in R12, R2004 e R2013 (R12, criterio di accettazione).
- Esito T1: in ZWCAD 2023 i caratteri accentati di un file R2013 UTF-8 sono corretti anche senza `$DWGCODEPAGE` (variante v10). La scelta di scriverlo sempre si basa sul fatto che costa una riga e che, secondo il documento di specifica, `ezdxf` senza di esso legge male le lettere accentate (non verificato in questa fase). Il valore scritto nei file di T1 era `ANSI_1252`.
- Ipotesi da verificare in T4: la gestione dei caratteri fuori dal BMP (oltre U+FFFF), per cui `\U+XXXX` non basta, e la lettura delle sequenze `\U+XXXX` già presenti nei file letti (decodificarle o lasciarle come testo).
- Il nome esatto con cui Latin-1 compare in `$DWGCODEPAGE` è da verificare sulle fixture reali.
- Gli avvisi di code page usano `Diagnostic` con severità warning in modalità tollerante.
