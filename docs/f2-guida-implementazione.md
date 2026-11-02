# Specifiche F2 – Guida di implementazione

2 novembre 2026

## Obiettivo e ambito

F2 aggiunge alla libreria le entità che servono a una tavola strutturale moderna: dopo F2 si leggono, si scrivono e si riscrivono senza perdite LWPOLYLINE, MTEXT, HATCH, DIMENSION, LEADER, MLEADER, ATTDEF, ELLIPSE e SPLINE, con spessori di linea, colori RGB e tipi di linea standard. Il criterio del piano resta: **ogni entità ha test di lettura, di scrittura e un file di verifica generato da `dxfgen`**. Questo documento è scritto perché tu possa implementare F2 da solo, un task alla volta, senza dover decidere il metodo.

**Dentro F2**

- Modello, lettura e scrittura R2000–R2018 delle nove entità, con round-trip dei tag (ADR 0005).
- Colore vero (420, 430, 440), spessore di linea (370) su entità, layer e DIMSTYLE, tipi di linea standard con i loro elementi.
- Tabella DIMSTYLE tipizzata e blocco anonimo `*Dn` delle quote, conservato in lettura e scritto in scrittura.
- Un generatore minimo del blocco della quota (lineare, allineata, raggio, diametro), perché una quota senza blocco in alcuni CAD non si vede finché non si rigenera. Lo verifichiamo nel task T1.
- Estensione di `extents.go`, della validazione e del comportamento in R12 per i nuovi tipi.

**Fuori da F2**

- Quote complete, in catena, angolari e di livello disegnate dal motore: sono F4 (`annot`).
- Valutazione di SPLINE (campionamento, offset, intersezioni): F3 (`geom`). In F2 la SPLINE si legge e si riscrive, e il suo ingombro si stima dai punti di controllo.
- Layout, VIEWPORT e spazio carta: F4 (`sheet`).
- Variabili di intestazione tipizzate (`$INSUNITS`, `$LWDISPLAY`, `$LTSCALE`): restano accessibili con `Header.Get/Set`; un accesso tipizzato si valuta in F3.
- DXF binario (ADR 0008) e R12 come formato principale di scrittura (ADR 0009).

## Punto di partenza

F1 ha già costruito tutto ciò su cui poggiano le nuove entità: un nuovo tipo si aggancia in quattro punti e non richiede altro.

| Punto di aggancio | Dove | Cosa fa oggi |
| --- | --- | --- |
| Tipo `Entity` | `entities.go` | `Type()`, `Layer()`, `Common()`; i campi comuni sono in `EntityCommon` (layer, colore ACI, handle, `Owner`, `Subclasses`, `Raw`, `RawSubclass`) |
| Decodifica | `decodeEntityMode` in `entities.go` | un `switch` sul nome: i tipi noti hanno `decodeXxx(gs, common)`; gli altri diventano `RawEntity` con tutti i tag (`Verbatim`) |
| Scrittura R2013 | `writeEntity` in `writer_r2013.go` | un `case` per tipo, con le sottoclassi nell'ordine e un `mw.simple(...)` che scrive intestazione, `330`, layer e marcatori |
| Scrittura R12 | `writer.go` | validazione e `entityWriter`; i tipi R2000+ oggi sono rifiutati da `validateModernContent`/`r12Incompatible` |

Strumenti già pronti: `groupSet` (consumo dei gruppi per codice e per sottoclasse), `mw.point`, `mw.groupFloat`, `mw.extrusion`, `mw.thickness`, la verifica con ezdxf (`scripts/oracle_check.py`, `runOracle` in `oracle_test.go`), il confronto di tag (`scripts/tagdiff.py`), le fixture sintetiche in `testdata/oracle` e i file da provare a mano in `testdata/g1`.

### Debiti di F1 da chiudere prima (task T0)

Sono difetti o decisioni che F2 amplifica se restano aperti.

- **Valori uguali al default persi in lettura** (per esempio `41 1.0` su INSERT): il lettore li consuma e non li riscrive. Per i nuovi tipi decidi subito la regola: un valore presente nel file si conserva nel campo, con un indicatore `Set` come `ExtrusionSet`, oppure si ricostruisce sempre.
- **APPID usati negli `1001`** non creati in automatico nel percorso R2013: le quote e le MLEADER usano xdata `ACAD`, `ACAD_DSTYLE_DIMJAG` e simili.
- **ATTRIB**: `decodeAttribute` legge il `72` di `AcDbAttribute` come HAlign e il writer lo reinserisce con un workaround per R2018. ATTDEF ha la stessa struttura: correggi prima ATTRIB.
- **`NewDocument()` ancora R12**: l'ADR 0002 lo porta a R2013 dopo G1. Decidi se farlo in F2 (consigliato: sì, all'inizio, con voce di changelog), perché le nuove entità non esistono in R12.
- **ADR provvisori** 0001, 0003, 0006, 0007, 0009 e le decisioni D1, D3, D6, D7: confermali o cambiali ora.
- **Commit**: F0, `testdata/t1` e tutto T3–T12 non sono ancora committati. Commit prima di cominciare, in pezzi leggibili (uno per task).

## Requisiti e criteri di accettazione

Ogni requisito ha un test che lo dimostra; il test porta il nome del requisito (`TestF2R3...`), come in F1.

| N. | Requisito | Come si verifica |
| --- | --- | --- |
| Q1 | Le nove entità si leggono da file R2000–R2018 reali e sintetici, in modalità rigorosa, senza avvisi | fixture ZWCAD in `testdata/f2/real`, fixture ezdxf in `testdata/f2/synth` |
| Q2 | Round-trip: decode→encode→decode dà la stessa sequenza di tag su file di altri CAD, e gli stessi byte su file scritti dalla libreria (ADR 0005) | `tagdiff.py --ignore-handles` sulle fixture |
| Q3 | I gruppi non interpretati (xdata, `102 {...}`, `360`, campi di versioni successive) sopravvivono | test con tag finti inseriti in ogni entità |
| Q4 | Un valore fuori intervallo o incoerente dà `ErrValidation` con riga e codice, non un file corrotto (per esempio `90` diverso dal numero di vertici) | test di validazione per entità |
| Q5 | Un file scritto dalla libreria passa `doc.audit()` di ezdxf senza errori né correzioni | `runOracle` su ogni file di `dxfgen` |
| Q6 | Un file scritto con le nuove entità si apre senza errori in ZWCAD 2023 e CadCraft 0.16.0 e si vede come previsto (gate G1b) | tabella in `testdata/g2/README.md` |
| Q7 | In scrittura R12: LWPOLYLINE diventa POLYLINE, le altre entità nuove danno `ErrValidation` con un messaggio che nomina il tipo | test su `Encode` con `WithVersion(VersionR12)` |
| Q8 | `Extents` include tutte le nuove entità | test con valori calcolati a mano |
| Q9 | Nessun peggioramento oltre il 10% dei benchmark `DecodeLines` e `EncodeLinesR2013`; fuzz senza panic per 60 secondi per tipo | benchmark e `go test -fuzz` |
| Q10 | `gofmt`, `go vet`, `go test -race`, `go test -tags=oracle` e `-tags=integration` verdi | CI |

**Criterio di fine di F2.** Q1–Q10 soddisfatti, G1b superato in due CAD, roadmap e changelog aggiornati. Il piano aggiunge un'unica verifica a mano rispetto a F1: la tabella di G1b.

## Progetto

### Decisioni da prendere prima di scrivere codice

Sono proposte; adottale provvisoriamente se non hai ragioni contrarie e registra l'esito in un ADR (0010 in poi).

| N. | Decisione | Proposta | Perché |
| --- | --- | --- | --- |
| D9 | Colore | `EntityCommon` mantiene `Color`/`HasColor` (ACI) e aggiunge `TrueColor *RGB` e `ColorName string`; `Transparency *int` per il `440` | non rompe l'API F1; il `420` coesiste con il `62` nel file |
| D10 | LWPOLYLINE | tipo proprio `LWPolyline`, con `ToPolyline()` e `FromPolyline()`; nessuna conversione implicita in lettura | le due entità hanno sottoclassi e round-trip diversi |
| D11 | Entità nuove in R12 | LWPOLYLINE → POLYLINE 2D automatico; le altre `ErrValidation` | R12 non ha una forma equivalente; meglio un errore che una perdita muta |
| D12 | Geometria delle quote | la libreria conserva il blocco `*Dn` letto e ne genera uno minimo per lineare, allineata, raggio e diametro | i CAD mostrano il blocco; la quota completa è F4 |
| D13 | MLEADER | campi tipizzati per testo, punto di aggancio, linea con N punti, stile e scala; tutto il resto in `Raw` nell'ordine originale | la struttura è annidata e varia con la versione; servono fixture reali |
| D14 | Campiture predefinite | catalogo scritto da noi (`patterns.go`) per ANSI31, ANSI32, ANSI37, EARTH, AR-CONC, AR-SAND, STEEL e LINE; niente copia del file `acad.pat` | evita problemi di licenza; sono righe di segmenti definite dalla geometria |

### Modello dei dati

Schema indicativo; i nomi si possono cambiare, la struttura no.

```go
// common.go: aggiunte a EntityCommon
TrueColor    *RGB   // 420
ColorName    string // 430
Transparency *int   // 440
LineWeight   int    // 370, in 1/100 mm; -1 ByLayer, -2 ByBlock, -3 Default
HasLineWeight bool
LinetypeScale float64 // 48; HasLinetypeScale bool

type RGB struct{ R, G, B uint8 }

type LWPolyline struct {
    EntityCommon
    Closed        bool
    ConstWidth    float64   // 43
    Elevation     float64   // 38
    Vertices      []LWVertex
}
type LWVertex struct{ X, Y, StartWidth, EndWidth, Bulge float64; ID int }

type MText struct {
    EntityCommon
    Insert        Point3D
    Height        float64 // 40
    Width         float64 // 41
    Attachment    int     // 71, 1..9
    Direction     int     // 72
    Text          string  // 1 + 3 già concatenati
    Style         string  // 7
    XAxis         Point3D // 11
    LineSpacing   float64 // 44
    Background    *MTextBackground
}

type Hatch struct {
    EntityCommon
    PatternName  string        // 2
    Solid        bool          // 70
    Associative  bool          // 71
    Paths        []HatchPath   // 91..
    Style        int           // 75
    PatternType  int           // 76
    Angle, Scale float64       // 52, 41
    Double       bool          // 77
    Lines        []PatternLine // 78..
    Seeds        []Point2D     // 98
    Gradient     *Gradient     // 450..
}
```

Per le altre entità vale lo stesso schema: i campi tipizzati sono una vista, i gruppi sconosciuti restano in `Raw`/`RawSubclass` (ADR 0007). Una regola, da non violare: **un tipo si decodifica solo se si riscrive identico**; un sottoblocco che non sai riscrivere resta `Raw`.

### Gruppi ripetuti: la novità tecnica di F2

Finora `groupSet` consuma i gruppi per codice e per sottoclasse. LWPOLYLINE, HATCH, SPLINE e MLEADER ripetono gli stessi codici (`10`, `20`, `42`, `93`, `72`) con significato che dipende dall'**ordine** e dai contatori `90`, `91`, `93`, `94`. Serve un cursore ordinato sullo stesso insieme di gruppi, per esempio `gs.cursor()` con `Next()`, `Peek()` e `Take(code)`, e una regola unica per i contatori: il contatore è un dato letto e **si ricalcola in scrittura**; se il numero di elementi letti non coincide, in modalità rigorosa è un errore e in quella tollerante un avviso.

### Procedura per ogni entità (ripetila per ogni tipo)

1. **Fixture reale.** Disegna l'entità in ZWCAD in tre varianti (minima, tipica, con opzioni rare) e salva DXF 2013 e 2018 in `testdata/f2/real/<tipo>-*.dxf`.
2. **Tabella dei gruppi.** Sotto "Riferimento per entità" annota, per ogni sottoclasse, codice, significato, default, ripetizione. Confrontala con la fixture e con ezdxf; dove diverge vince la fixture.
3. **Tipo e lettura.** Struct in un file nuovo (`lwpolyline.go`, `mtext.go`, …), `decodeXxx`, `case` in `decodeEntityMode`. Aggiungi il tipo all'elenco dei tipi noti sopra il `switch`.
4. **Scrittura.** `case` in `writeEntity` con le sottoclassi nell'ordine della fixture e `mw.simple`. Contatori ricalcolati.
5. **Test.** Lettura della fixture reale; scrittura e lettura di una costruita a mano; round-trip dei tag; test Q3 (tag sconosciuti); test Q4 (incoerenze).
6. **Oracolo.** `runOracle` sul file scritto: zero errori.
7. **dxfgen.** Un sottocomando o una scena che scrive l'entità in `testdata/g2/` per la prova nei CAD.
8. **Documentazione.** Aggiorna `docs/dxf-r2013.md` con la tabella dei gruppi (non più "da verificare").

### Riferimento per entità: geometria

Tabelle dei gruppi da verificare con la fixture reale (punto 2 della procedura). Sono scritte dalla specifica DXF e da ezdxf; non è stato possibile provarle in un CAD, quindi trattale come ipotesi da confermare.

**LWPOLYLINE** (sottoclassi `AcDbEntity`, `AcDbPolyline`)

| Codice | Significato | Note |
| --- | --- | --- |
| 90 | numero di vertici | ricalcolato in scrittura |
| 70 | flag: 1 = chiusa, 128 = generazione linea continua |  |
| 43 | larghezza costante | scritto solo se diverso da zero |
| 38, 39 | elevazione, spessore | opzionali |
| 10, 20 | vertice X, Y | ripetuto per ogni vertice, in ordine |
| 40, 41 | larghezza iniziale e finale del tratto | opzionali per vertice |
| 42 | bulge (tangente di un quarto dell'angolo) | opzionale per vertice |
| 91 | identificatore del vertice | solo R2010+, opzionale |
| 210 | estrusione | come le altre entità |

Attenzione: i gruppi `40`, `41`, `42` appartengono al vertice che li precede. Un vertice senza larghezze non li scrive; il cursore deve quindi agganciarli al `10` corrente, non contarli per posizione.

**ELLIPSE** (`AcDbEllipse`): `10` centro; `11` estremo dell'asse maggiore **relativo al centro**; `210` estrusione; `40` rapporto asse minore/maggiore; `41`, `42` parametro iniziale e finale in radianti (da 0 a 2π per l'ellisse intera).

**SPLINE** (`AcDbSpline`)

| Codice | Significato |
| --- | --- |
| 210 | normale |
| 70 | flag: 1 chiusa, 2 periodica, 4 razionale, 8 planare, 16 lineare |
| 71, 72, 73, 74 | grado, numero di nodi, numero di punti di controllo, numero di punti di passaggio |
| 42, 43, 44 | tolleranze sui nodi, sui punti di controllo e di passaggio |
| 12, 13 | tangente iniziale e finale (opzionali) |
| 40 | nodi (ripetuto) |
| 41 | pesi (ripetuto, solo se razionale) |
| 10 | punti di controllo (ripetuto) |
| 11 | punti di passaggio (ripetuto) |

I tre contatori `72`, `73`, `74` si ricalcolano in scrittura. Il grado e il numero di nodi devono soddisfare *nodi = punti di controllo + grado + 1*: se non vale, errore di validazione.

**MTEXT** (`AcDbMText`)

| Codice | Significato | Note |
| --- | --- | --- |
| 10 | punto di inserimento |  |
| 40 | altezza del testo |  |
| 41 | larghezza del rettangolo di riferimento | 0 = senza a capo automatico |
| 71 | punto di attacco 1–9 (1 alto-sinistra … 9 basso-destra) |  |
| 72 | direzione del flusso | 1 da sinistra a destra, 3 dall'alto in basso, 5 da stile |
| 1 | ultimo frammento di testo |  |
| 3 | frammenti precedenti, al massimo 250 caratteri ciascuno | ripetuto, **prima** dell'`1` |
| 7 | nome dello stile di testo |  |
| 11 | direzione dell'asse X | in alternativa a `50` |
| 210 | estrusione |  |
| 73, 44 | stile e fattore di interlinea |  |
| 90, 63, 45, 421, 441 | sfondo: modo, colore, scala, colore vero e trasparenza | opzionali |

Il testo contiene codici di formato: `\P` capoverso, `\~` spazio rigido, `{\fArial|b1|i0;testo}` carattere, `\H2.5x;` altezza, `\S1/2;` frazione, `\U+00D8` carattere Unicode. Per F2 bastano: spezzare e riunire i frammenti da 250 caratteri, una funzione `PlainText()` che toglie i codici, e un piccolo costruttore (`NewMText(text)` con `Paragraph()`, `Bold()`, `Stack()`) che emette solo codici validi. Il resto del formato non si interpreta: si conserva nella stringa.

**HATCH** (`AcDbHatch`) — l'entità più complessa di F2. Ordine dei gruppi:

| Blocco | Codici | Significato |
| --- | --- | --- |
| Testata | 10, 20, 30; 210; 2; 70; 71 | punto di elevazione, estrusione, nome del pattern (`SOLID` per il pieno), 1 = pieno, 1 = associativa |
| Contorni | 91 | numero di percorsi; poi per ciascuno i gruppi qui sotto |
| Percorso | 92 | tipo: bit 1 esterno, 2 polilinea, 16 più esterno, 4 derivato |
| Se polilinea (bit 2) | 72, 73, 93, poi 10, 20 (e 42) per vertice | con bulge, chiusa, numero di vertici |
| Altrimenti | 93, poi per spigolo 72 | numero di spigoli; tipo: 1 linea, 2 arco, 3 ellisse, 4 spline |
| Spigolo linea | 10, 20, 11, 21 | estremi |
| Spigolo arco | 10, 20, 40, 50, 51, 73 | centro, raggio, angoli, senso antiorario |
| Spigolo ellisse | 10, 20, 11, 21, 40, 50, 51, 73 | come ELLIPSE |
| Spigolo spline | 94, 73, 74, 95, 96, 40, 10, 20, 42 | grado, razionale, periodica, nodi, punti, pesi |
| Oggetti sorgente | 97, poi 330 per ciascuno | gli oggetti del contorno se associativa; 0 altrimenti |
| Stile | 75, 76 | 0 normale, 1 esterno, 2 ignora; 0 utente, 1 predefinito, 2 personalizzato |
| Se non pieno | 52, 41, 77, 78 | angolo, scala, doppia, numero di righe |
| Riga del pattern | 53, 43, 44, 45, 46, 79, 49 | angolo, origine, passo, numero di trattini, lunghezze |
| Chiusura | 47, 98, 10, 20 | dimensione del pixel, numero di punti di semina, punti |
| Sfumatura (opzionale) | 450, 451, 452, 453, 460, 461, 462, 463, 470 | conservata così com'è in `Raw` in F2 |

Regole: i percorsi sono la parte che più spesso si sbaglia, quindi la lettura rifiuta (rigorosa) o avvisa (tollerante) quando un contatore non coincide. Un pattern predefinito si scrive con le sue righe prese da `patterns.go` (D14), mai da un file esterno. I percorsi si leggono tutti, ma non si verificano geometricamente (chiusura, autointersezioni): lo fa F3.

**DIMENSION** (`AcDbDimension` più una sottoclasse per tipo)

| Codice | Significato | Note |
| --- | --- | --- |
| 2 | nome del blocco anonimo (`*D1`, `*D2`…) | il blocco sta in BLOCKS |
| 10 | punto di definizione |  |
| 11 | punto medio del testo |  |
| 70 | tipo: 0 lineare, 1 allineata, 2 angolare, 3 diametro, 4 raggio, 5 angolare a 3 punti, 6 ordinata; +32 blocco anonimo, +64 ordinata X, +128 testo spostato | i bit alti si conservano |
| 71, 72, 41 | punto di attacco del testo, stile e fattore di interlinea |  |
| 42 | misura reale | calcolata dalla geometria in scrittura |
| 1 | testo sostitutivo | vuoto = misura; `<>` = misura nel testo |
| 53, 51, 210 | rotazione del testo, direzione orizzontale, estrusione |  |
| 3 | nome dello stile di quota | deve esistere in DIMSTYLE |
| 13, 14 | origini delle linee di estensione | lineare, allineata, ordinata |
| 15, 16, 40 | punto sul cerchio / sull'arco, lunghezza di guida | raggio, diametro, angolare |
| 50, 52 | rotazione della linea di quota, obliquo | lineare |

Sottoclassi per tipo: `AcDbAlignedDimension` e `AcDbRotatedDimension` (lineare), `AcDbRadialDimension`, `AcDbDiametricDimension`, `AcDb3PointAngularDimension`, `AcDb2LineAngularDimension`, `AcDbOrdinateDimension`. Il tipo si ricava dal `70` (maschera `& 7`), non dalla sottoclasse. Anche quando F2 non genera il blocco di un tipo, deve leggerlo e riscriverlo.

**DIMSTYLE** (record della tabella, `AcDbDimStyleTableRecord`, handle in `105`): oltre ai gruppi di testata, le variabili sono `3`–`6` (testi e blocchi di freccia), `40`–`48` e `140`–`147` (scala, freccia `DIMASZ`, offset, estensioni, altezza del testo `DIMTXT`, distanza `DIMGAP`), `71`–`79` e `170`–`179` (flag), `271`–`289` (decimali, unità, giustificazione), `340`–`344` (stile del testo e blocchi di freccia per handle) e `371`, `372` (spessori). Tipizza solo le variabili che il generatore usa (D12) e lascia il resto in `Raw`, con una tabella di nomi verificata sulla fixture di ZWCAD.

**LEADER** (`AcDbLeader`): `3` stile di quota; `71` freccia sì/no; `72` tipo di tracciato (0 rette, 1 spline); `73` tipo di creazione (0 testo, 1 tolleranza, 2 blocco, 3 nessuna); `74`, `75` direzione e presenza della linea di aggancio; `40`, `41` altezza e larghezza del testo; `76` numero di vertici; `10` vertici (ripetuto); `77` colore; `340` handle dell'annotazione collegata; `210`, `211`, `212`, `213` normale e scostamenti.

**MLEADER** (`AcDbMLeader`): struttura annidata con `300 CONTEXT_DATA{ ... 301 }`, dentro cui `302 LEADER{ ... 303 }` e `304 LEADER_LINE{ ... }`. I gruppi cambiano tra R2007 e R2010 e non sono verificabili senza un file reale: **non** scrivere il tipo dalla specifica. Parti da una MLEADER disegnata in ZWCAD, tipizza solo testo (`304`, `12`, `340`), punto di aggancio, vertici della linea e stile, e conserva tutto il resto in `Raw` con la posizione relativa.

**ATTDEF** (`AcDbText`, poi `AcDbAttributeDefinition`): i gruppi di TEXT (`10`, `40`, `1`, `50`, `41`, `51`, `7`, `71`, `72`, `11`, `210`), poi `3` testo di richiesta, `2` nome (tag), `70` flag (1 invisibile, 2 costante, 4 verifica, 8 preimpostato), `73` lunghezza del campo, `74` allineamento verticale, `280` blocco della posizione. È identico ad ATTRIB tranne `3` e il flag; il cartiglio di F4 poggia su questo tipo.

### Colore vero, spessore di linea e tipi di linea

| Tema | Codici | Regola |
| --- | --- | --- |
| Colore ACI | 62 | già gestito; 256 = DaLayer, 0 = DaBlocco |
| Colore vero | 420 (intero `0x00RRGGBB`), 430 (nome del colore) | se presente, vince sul `62` nella visualizzazione; il `62` si scrive comunque |
| Trasparenza | 440 | `0x02000000` più il valore alfa (0–255); -1 DaLayer, -2 DaBlocco |
| Spessore | 370, in 1/100 di mm | valori ammessi: 0, 5, 9, 13, 15, 18, 20, 25, 30, 35, 40, 50, 53, 60, 70, 80, 90, 100, 106, 120, 140, 158, 200, 211; -1 DaLayer, -2 DaBlocco, -3 predefinito. Un valore non ammesso è un errore di validazione. Vale anche per LAYER e per DIMSTYLE (`371`, `372`) |
| Scala del tipo di linea | 48 sull'entità, `$LTSCALE` nell'intestazione | `48` di default è 1 |
| Tipo di linea standard | LTYPE: `72` = 65, `73` elementi, `40` lunghezza totale, per elemento `49` lunghezza (positiva tratto, negativa spazio, 0 punto) e `74` = 0 | catalogo: `CENTER`, `DASHED`, `HIDDEN`, `PHANTOM`, `DASHDOT`, `DOT`, `BORDER`, `DIVIDE` e le varianti `2` e `X2`; scritte da noi, non copiate da `acad.lin` |

Attenzione al layer `0` e a `Continuous`: si confrontano senza distinzione di maiuscole (già in `prepareR2013Tables`).

## Task

I task sono in ordine di dipendenza. La dimensione è relativa (S, M, L), senza date. Dopo T2, i task T3–T7 sono indipendenti tra loro e si possono fare in qualunque ordine; T8, T9 e T11 hanno dipendenze indicate.

| Task | Contenuto | File principali | Dipende da | Dim. |
| --- | --- | --- | --- | --- |
| T0 | Debiti di F1, decisioni D9–D14, commit | `entities.go`, `writer_r2013.go`, ADR | nessuno | S |
| T1 | Fixture reali da ZWCAD e prove sulle opzioni minime | `testdata/f2/real`, `testdata/f2/README.md` | T0 | M |
| T2 | Colore vero, spessore, scala linea; cursore ordinato per gruppi ripetuti | `common.go` (nuovo), `entities.go`, `writer_r2013.go` | T1 | M |
| T3 | LWPOLYLINE, con conversione da e a POLYLINE | `lwpolyline.go` | T2 | M |
| T4 | ELLIPSE e SPLINE (lettura, scrittura, ingombro) | `curves.go` | T2 | M |
| T5 | MTEXT con frammenti, `PlainText`, costruttore | `mtext.go` | T2 | M |
| T6 | ATTDEF, ATTRIB corretto, collegamento con INSERT | `attdef.go`, `entities.go` | T2 | S |
| T7 | Catalogo dei tipi di linea standard | `linetypes.go`, `tables.go` | T2 | S |
| T8 | HATCH: percorsi, pieno, pattern, catalogo (D14) | `hatch.go`, `patterns.go` | T3, T4, T7 | L |
| T9 | DIMSTYLE tipizzato; DIMENSION, 7 tipi, blocco `*Dn` conservato | `dimstyle.go`, `dimension.go`, `tables.go` | T2, T5, T7 | L |
| T10 | Generatore minimo del blocco della quota (D12) | `dimblock.go` | T9 | M |
| T11 | LEADER e MLEADER (D13) | `leader.go`, `mleader.go` | T5, T9 | M |
| T12 | Integrazione: ingombro, validazione, R12 (D11), `structural` | `extents.go`, `writer.go`, `structural/` | T3–T11 | M |
| T13 | `dxfgen`, oracolo, benchmark, fuzz, G1b | `cmd/dxfgen`, `testdata/g2`, `*_test.go` | T12 | M |
| T14 | ADR 0010 e seguenti, `docs/dxf-r2013.md`, README, changelog, roadmap | `docs/`, `CHANGELOG.md` | T13 | S |

### Dettaglio dei task

**T0 — Debiti e decisioni.** Esegui l'elenco di "Punto di partenza". Scrivi ADR 0010 con D9–D14 (provvisorie va bene). Passa `NewDocument()` a R2013 se decidi così: cambia i test F0 che confrontano byte R12. Commit. *Fine:* test verdi; ADR scritto.

**T1 — Fixture reali.** In ZWCAD disegna ogni entità e salva in DXF 2013 e 2018. Per HATCH: pieno, ANSI31 a 45°, contorno con un'isola, contorno con arco e spline. Per DIMENSION: le sette forme. Per MLEADER: una con testo, una con blocco. In più, tre esperimenti sul modello di T1 di F1, per sapere cosa si può omettere: DIMENSION **senza** blocco `*Dn`; HATCH **senza** punti di semina `98`; MTEXT **senza** `11`. Per ciascuno annota in `testdata/f2/README.md` se ZWCAD e CadCraft lo aprono, lo mostrano, lo rigenerano. Genera anche l'equivalente con ezdxf in `testdata/f2/synth` per l'oracolo. *Fine:* una tabella di verità per le tre domande; le fixture sono nel repository.

**T2 — Infrastruttura comune.** Aggiungi i campi D9 e `LineWeight` a `EntityCommon`; lettura in `gs.common()`, scrittura in un solo punto (non in ogni `case`). Validazione dello spessore contro l'elenco dei valori ammessi. Aggiungi lo spessore e il colore vero a `Layer` in `tables.go` (370 e 420 nel record). Implementa il cursore ordinato di "Gruppi ripetuti". *Test:* `TestF2R1TrueColor`, `...LineWeight`, `...LineWeightInvalid`, `...CursorRepeated`. *Fine:* tutte le entità esistenti leggono e riscrivono 370 e 420 senza perdere nulla (round-trip delle fixture F1 invariato).

**T3 — LWPOLYLINE.** Segui la procedura. Conversioni: `ToPolyline()` produce vertici con bulge e larghezze; `FromPolyline()` lo inverso, con errore se la polilinea è 3D o mesh. Scrittura in R12 secondo D11. *Test:* vertici con e senza bulge e larghezze, chiusa, contatore errato, 0 vertici, 10.000 vertici (limite), conversione round-trip.

**T4 — ELLIPSE e SPLINE.** Due tipi semplici. SPLINE: valida nodi, grado e pesi; `ControlBounds()` per l'ingombro. *Test:* ellisse intera e arco di ellisse, spline cubica aperta, chiusa, razionale, con punti di passaggio.

**T5 — MTEXT.** Frammenti `3`/`1` da 250 caratteri; `PlainText()`; costruttore. Attenzione alle codifiche: dalla R2007 il testo è UTF-8, prima è in code page (ADR 0004); `\U+XXXX` oltre U+FFFF resta aperto da F1, decidilo qui. *Test:* testo lungo oltre 250 caratteri, capoversi, `\U+00D8`, accenti, sfondo, round-trip del testo esatto.

**T6 — ATTDEF.** Prima correggi ATTRIB (il workaround del `72`), poi ATTDEF come variante. Un blocco con ATTDEF e un INSERT con ATTRIB corrispondenti devono restare coerenti: l'ATTRIB riporta il tag dell'ATTDEF. *Test:* flag di visibilità, tag, prompt, INSERT con più attributi.

**T7 — Tipi di linea standard.** Un catalogo in codice (non un file dati esterno) e `Tables.EnsureStandardLinetype(name)`; il writer R2013 scrive `72`, `73`, `40`, `49`, `74`. I tipi con testo o forme (`74` = 2 o 4) sono fuori da F2 e restano in `Raw`. *Test:* ogni nome del catalogo ha lunghezza totale uguale alla somma degli elementi; apertura nei CAD.

**T8 — HATCH.** Lettura e scrittura dei percorsi (polilinea e spigoli), pieno, pattern predefinito dal catalogo, pattern personalizzato come righe lette. Cursore a quattro livelli: testata, percorsi, pattern, semi. La sfumatura resta in `Raw`. Aiuto: `HatchFromPolygon(points, pattern, scale, angle)` per i casi d'uso di F5–F8. *Test:* un percorso con isola, spigoli misti, pieno, ANSI31 con scala, contatori sbagliati, percorso vuoto, 1.000 percorsi.

**T9 — DIMSTYLE e DIMENSION.** Prima `Tables.DimStyle(name)` e `SetDimStyle`, con le variabili usate dal generatore e `Raw` per il resto. Poi i sette tipi di DIMENSION con gli stessi campi comuni e un campo `Block string`; il blocco `*Dn` vive in `Document.Blocks` come blocco normale (nel BLOCK\_RECORD) e si scrive con la quota. Scrivendo una quota senza blocco si segue l'esito di T1. *Test:* sette fixture reali, quota con testo sostitutivo, stile mancante (errore), blocco mancante, rinomina del blocco quando si copia la quota.

**T10 — Blocco della quota.** `BuildLinearDimension(style, p1, p2, offset)` e le tre varianti: restituiscono l'entità e il blocco (linee di estensione, linea di quota, frecce come SOLID o tratti obliqui, testo MTEXT con la misura formattata secondo `DIMDEC`). Solo geometria piana, senza tolleranze né unità alternative. *Test:* coordinate calcolate a mano per un caso; confronto con la quota di ZWCAD della stessa geometria (stesse misure a meno di 1e-6).

**T11 — LEADER e MLEADER.** LEADER secondo la tabella; MLEADER secondo D13. Se la fixture reale mostra campi diversi da quelli qui riportati, la fixture vince. *Test:* LEADER a due e tre vertici, spline, con testo; MLEADER con testo, con blocco, con due linee.

**T12 — Integrazione.** `extents.go`: LWPOLYLINE con bulge (arco), ELLIPSE (parametri), SPLINE (poligono di controllo), MTEXT (punto più larghezza e altezza stimate), HATCH (contorno), DIMENSION (blocco), LEADER (vertici). Validazione moderna e comportamento in R12 (D11). `structural`: nessuna modifica obbligatoria, ma verifica che un file con le nuove entità si importi senza errori e le ignori con un avviso.

**T13 — Verifica.** `dxfgen` scrive in `testdata/g2/` un file per entità e uno con tutte (`g2-tutte.dxf`); `runOracle` su ciascuno; benchmark con una scena di 100.000 entità miste; fuzz di 60 secondi per ogni nuovo `decodeXxx`; `testdata/g2/README.md` con la tabella di G1b.

**T14 — Documentazione.** ADR per le decisioni D9–D14, `docs/dxf-r2013.md` con le tabelle verificate, README, `docs/migrazione-f1.md` affiancato da `docs/migrazione-f2.md` (nuovi campi di `EntityCommon`, R12), roadmap e changelog.

## Verifica nei CAD (G1b) e rischi

Come per G1, la tabella si compila a mano in ZWCAD 2023 e CadCraft 0.16.0, con questi controlli per file di `testdata/g2/`. Un file passa se si apre senza avvisi, `AUDIT` non segnala errori e l'aspetto è quello della colonna "Che cosa guardare".

| File | Che cosa guardare |
| --- | --- |
| `lwpolyline` | tratti ad arco con la curvatura giusta; larghezze visibili; chiusura |
| `ellipse-spline` | ellisse intera e arco; spline che passa dai punti attesi |
| `mtext` | a capo alla larghezza data; capoversi; `Ø` e lettere accentate; sfondo |
| `attdef-insert` | l'INSERT mostra il valore dell'attributo; `EATTEDIT` lo apre |
| `hatch` | pieno, ANSI31 con la scala giusta, isola non campita, contorno con arco |
| `dimension` | le quote si vedono **senza** rigenerare; dopo `DIMREGEN` la misura è invariata; stirando un estremo la quota si aggiorna |
| `leader` | freccia, linea e testo; la MLEADER si sposta intera |
| `lineweight-color` | con visualizzazione degli spessori attiva, i tratti hanno spessori diversi; il colore RGB compare nel pannello proprietà |
| `linetypes` | CENTER, DASHED e HIDDEN con tratti nella scala attesa |
| `g2-tutte` | tutte le entità insieme, `ZOOM E` mostra il disegno intero |

### Rischi principali

| Rischio | Effetto | Contromisura |
| --- | --- | --- |
| Le tabelle di questo documento sono scritte dalla specifica e da ezdxf, non da un CAD | gruppi mancanti o in ordine diverso fanno aprire il file vuoto, come è successo con i marcatori in T1 | T1 prima di ogni codice; la fixture vince sulla tabella |
| MLEADER dipende dalla versione e ha struttura annidata | lettura sbagliata, perdita di dati | D13: pochi campi tipizzati, il resto in `Raw`; fixture reale obbligatoria |
| Una quota senza blocco `*Dn` può non vedersi | quote invisibili in alcuni CAD | T1 lo misura; T10 genera il blocco |
| HATCH con contatori incoerenti | file rifiutato dal CAD | contatori ricalcolati in scrittura, errore in lettura rigorosa |
| Cursore ordinato e `groupSet` per codice convivono | doppio consumo dei gruppi | un solo punto di ingresso per entità; test Q3 con tag sconosciuti |
| Prestazioni: tipi con molti vertici, percorsi, nodi | regressione dei benchmark | Q9, benchmark misto da 100.000 entità |
| Licenza del file dei pattern | problemi di redistribuzione | D14: pattern scritti da noi |
| CadCraft più permissivo di ZWCAD | falsi positivi | ZWCAD è il riferimento; ezdxf non lo sostituisce |

### Come procedere, concretamente

1. Fai una branch `f2` e un commit per task, con il nome del task nel messaggio.
2. Inizia da T0 e T1: sono i due task che evitano di scrivere codice sulle ipotesi.
3. Dopo ogni task: `gofmt -l .`, `go vet ./...`, `go test -race -count=1 ./...`, `go test -tags=oracle,integration -count=1 ./...`.
4. Alla fine di T8, T9 e T11 (i tre più rischiosi) fermati e prova il file nei CAD prima di proseguire, senza aspettare T13.
5. Se una fixture reale contraddice una tabella di questo documento, correggi il documento nello stesso commit.
