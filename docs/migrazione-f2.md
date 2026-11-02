# Migrazione da F1 a F2

Per chi usava l'API del package `dxf` alla chiusura di F1 (lettura R12-R2018, scrittura R12 e R2000-R2018). Non esiste ancora una versione 1.x pubblicata (ADR 0002), quindi nessun cambio di percorso di import: il codice F1 che compilava continua in generale a compilare. Le differenze sono sotto.

Questa pagina nasce con il task T0 e cresce con i task di F2 (T14 la completa).

## Riepilogo

| Argomento | F1 | F2 |
|-----------|----|----|
| `NewDocument()` | documento vuoto **R12** con le tabelle LAYER 0, LTYPE CONTINUOUS, STYLE STANDARD | documento vuoto **R2013** (ADR 0002); le tabelle non sono memorizzate, le aggiunge `Encode` |
| Documento R12 vuoto | `NewDocument()` | `NewDocumentVersion(VersionR12)` |
| `ATTRIB` allineamento `72` | letto con fallback sulla sottoclasse `AcDbAttribute`; il writer lo rimetteva con un workaround per R2018 | letto solo da `AcDbText`; il `72` di `AcDbAttribute` resta in `Raw` |
| APPID da dati estesi | aggiunti a mano con `EnsureAppID` | in scrittura R2013 aggiunti in automatico per ogni `1001` |

## `NewDocument()`

Da F2 il documento predefinito e' R2013, perche' le entita' che F2 aggiunge (LWPOLYLINE, MTEXT, HATCH, ...) non esistono in R12.

```go
// F1
doc := dxf.NewDocument()                 // R12
err := dxf.Encode(w, doc)                // scrive R12

// F2
doc := dxf.NewDocument()                 // R2013
err := dxf.Encode(w, doc)                // scrive R2013
```

Per ottenere il comportamento di F1 (R12) in modo esplicito:

```go
doc := dxf.NewDocumentVersion(dxf.VersionR12) // R12, con le tabelle predefinite
err := dxf.Encode(w, doc)                     // scrive R12
```

Un documento scritto senza opzioni cambia quindi versione e formato (CRLF, codici su tre colonne, handle, marcatori di sottoclasse). Chi confrontava l'output byte per byte deve passare `WithVersion(dxf.VersionR12)` o usare `NewDocumentVersion(dxf.VersionR12)`.

Inoltre `NewDocument()` non crea piu' le tabelle `LAYER`, `LTYPE` e `STYLE`: le aggiunge `Encode` quando servono. Il codice che legge `doc.Tables.Find("LAYER")` subito dopo `NewDocument()` deve passare a `NewDocumentVersion(VersionR12)` o creare la tabella con `EnsureLayer`.

## `ATTRIB` e il codice `72`

L'allineamento orizzontale (`72`) appartiene ad `AcDbText`. I file R2018 scrivono anche un `72` dentro `AcDbAttribute`: in F1 quel valore veniva letto come allineamento e il writer doveva rimetterlo con un workaround. Da F2 il `72` di `AcDbAttribute` resta nei gruppi non consumati (`Raw`) e si riscrive dove si trovava. L'effetto sui dati utente e' nullo: il round-trip resta identico; cambia solo dove finisce il valore (`Attribute.Raw` invece del campo `HAlign`).

## APPID e dati estesi

In scrittura R2013 la libreria aggiunge un record APPID per ogni applicazione nominata da dati estesi (`1001`) che il documento non definisce. Non serve piu' chiamare `EnsureAppID` per `ACAD` o `ACAD_DSTYLE_*`. Il percorso R12 e' invariato: li' `EnsureAppID` resta a carico del chiamante.
