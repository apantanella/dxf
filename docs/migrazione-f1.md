# Migrazione da F0 a F1

Per chi usava l'API del package `dxf` alla chiusura di F0 (solo R12). Non esiste una versione 1.x pubblicata (ADR 0002), quindi nessun cambio di percorso di import: il codice F0 che compilava continua in generale a compilare. Le differenze sono sotto.

## Riepilogo

| Argomento | F0 | F1 |
|-----------|----|----|
| `Group` | `struct{Code int; Value string}` definita in `dxf` | alias: `type Group = tags.Tag` (ADR 0001) |
| `GroupKind` e costanti `GroupKind*` | tipo e costanti di `dxf` | alias di `tags.Kind` e delle costanti `tags.Kind*`; stessi nomi |
| Metodi di `Group` | `String`, `Int`, `Float`, `Kind` | gli stessi, piu' `Bool` e `Hex` (da `tags.Tag`) |
| `SyntaxError`, `LimitError` | strutture di `dxf` | alias di `tags.SyntaxError` e `tags.LimitError`; stessi campi e stessi messaggi |
| `ErrSyntax`, `ErrLimit` | errori propri di `dxf` | stessi valori di `tags.ErrSyntax` e `tags.ErrLimit`; `errors.Is` funziona con entrambi i nomi |
| Handle | solo `EntityCommon.Handle`, `Attribute.Handle`, `Vertex.Handle`: `string` | i campi restano `string` (testo letto); nuovo `type Handle uint64` con `String` e `IsZero`, `ParseHandle`, `HandleValue()` e `SetHandle(Handle)` (ADR 0006) |
| Owner | non esiste | `EntityCommon.Owner`, `Attribute.Owner`, `Block.Owner` di tipo `Handle` (gruppo `330`, da R13) |
| Indice degli handle | non esiste | `Document.Lookup`, `HandleIndex`, `ValidateHandles` |
| `Decode` di un file R2000+ | `*VersionError` (era ammesso solo `AC1009`) | accettato: nove versioni da `AC1009` a `AC1032` (`VersionR12` ... `VersionR2018`); `*VersionError` solo per altri valori |
| `Encode` con `Document.Version` diverso da R12 | `*VersionError` | scrive R2000-R2018; per R13/R14 e altri valori restituisce un errore che soddisfa `ErrUnsupportedVersion` **e** `ErrValidation` |
| Scelta della versione scritta | solo R12 | `WithVersion(v)`, poi `Document.Version`, poi `$ACADVER`, poi R12 |
| Documento vuoto | `NewDocument()` (R12) | `NewDocument()` ancora R12; `NewDocumentVersion(v)` per le altre versioni |
| Entita' non riconosciute | `RawEntity.Groups` con i gruppi non consumati | invariato in R12; da R13 `RawEntity.Verbatim` e' `true` e `Groups` e' il record completo dopo `(0, TypeName)` (handle, `330`, `100`, `102`, dati estesi) |
| Sezioni CLASSES e OBJECTS | `UnknownSections` | `Document.Classes` (`Class`) e `Document.Objects` (`Dictionary`, `Layout`, `Placeholder`, `RawObject`) |
| Tabelle | `Tables.Items`, `Ensure*` | invariati; in piu' `Tables.Layer`, `SetLayer`, `Linetype`, `TextStyle`, `BlockRecords`, `LoadBlockRecords` |
| Marcatori di sottoclasse | non conservati | `Subclasses`, `Raw`, `RawSubclass` su entita', blocchi e attributi |
| DXF binario | `ErrNotDXF` senza spiegazione (ADR 0008) | sempre `errors.Is(err, ErrNotDXF)`, con messaggio che dice che il file e' binario |
| Scrivere come R12 contenuto R13+ | non applicabile | `ErrValidation` con motivo e suggerimento (ADR 0007) |
| Package nuovi | `structural` | `tags`; `structural` invariato |

## Cosa puo' non compilare o cambiare comportamento

1. **Metodi su `Group`.** `Group` e' un alias di un tipo di un altro package: i metodi si aggiungono solo in `tags`. Un fork che aggiungeva metodi a `Group` dentro `dxf` va adattato.
2. **`Group` e `tags.Tag` sono lo stesso tipo.** I due nomi sono intercambiabili: una funzione che accettava `Group` accetta `tags.Tag` e viceversa. Un `switch` su tipo con entrambi i casi non compila (caso duplicato).
3. **`SyntaxError` e `LimitError`.** Si definiscono in `tags`; `errors.As(err, &se)` con `var se *dxf.SyntaxError` continua a funzionare perche' e' lo stesso tipo.
4. **Chi si aspettava un errore da `Decode` per i file non R12** deve ora controllare `doc.Version`. Chi voleva rifiutare le versioni diverse da R12 puo' confrontare `doc.Version != dxf.VersionR12` dopo `Decode`.
5. **Chi chiamava `Encode` con `Version` diversa da R12 e si aspettava `*VersionError`** ora ottiene un file R2000-R2018, tranne per R13/R14 e per i valori non riconosciuti. `errors.As(err, &ve)` con `var ve *dxf.VersionError` funziona ancora per questi ultimi.
6. **Handle di tipo stringa.** Il codice che assegnava `Handle: "2A"` continua a compilare. Per lavorare con il valore numerico usare `HandleValue()` e `SetHandle(h)`; `SetHandle(0)` svuota il campo. Il testo letto (maiuscole, zeri iniziali) si conserva finche' non lo si cambia.
7. **Scrittura R12.** Output byte per byte invariato (suite `regression_f0_test.go`).
8. **`NewDocument()`.** In F1 resta R12. ADR 0002 prevedeva il passaggio a R2013 alla chiusura di F1 (dopo G1): il passaggio e' avvenuto in F2, task T0 (vedi [migrazione-f2.md](migrazione-f2.md) e la voce di CHANGELOG). Per non dipendere dal valore predefinito usare `NewDocumentVersion(dxf.VersionR12)` o `NewDocumentVersion(dxf.VersionR2013)`.

## Esempi

Prima (F0), solo R12:

```go
doc := dxf.NewDocument()
doc.Entities = append(doc.Entities, &dxf.Line{End: dxf.Point3D{X: 1}})
err := dxf.Encode(w, doc)
```

Dopo (F1), stessa API, versione a scelta:

```go
doc := dxf.NewDocument()                              // ancora R12
err := dxf.Encode(w, doc)                             // R12, invariato
err = dxf.Encode(w, doc, dxf.WithVersion(dxf.VersionR2013)) // stessa geometria scritta come R2013
```

Gestione degli errori nuovi:

```go
doc, err := dxf.Decode(r)
switch {
case errors.Is(err, dxf.ErrNotDXF):            // anche DXF binario
case errors.As(err, new(*dxf.VersionError)):   // $ACADVER fuori da AC1009-AC1032
case errors.Is(err, dxf.ErrSyntax):
case errors.Is(err, dxf.ErrLimit):
}

err = dxf.Encode(w, doc, dxf.WithVersion(dxf.VersionR12))
if errors.Is(err, dxf.ErrValidation) {
	var ve *dxf.ValidationError
	if errors.As(err, &ve) {
		log.Println(ve.Entity, ve.Reason) // elemento e motivo
	}
}
```

Un file R2013 letto con reattori o oggetti non interpretati non si puo' scrivere come R12: `Encode` lo rifiuta con `ErrValidation` invece di scartare i dati.
