# F1 – Schede dei task T3–T9

Schede operative per i task T3–T9 della fase F1. Il perimetro, i requisiti R1–R14 e le decisioni sono nel documento "Specifiche F1 – Nucleo R2013" e negli ADR in [`adr/`](adr/README.md); qui c'è solo quello che serve per implementare. Il formato scritto è in [`dxf-r2013.md`](dxf-r2013.md).

Regole comuni a tutti i task:

- `regression_f0_test.go` e i test esistenti restano verdi senza modifiche alle attese (R13).
- Ogni requisito ha almeno un test che porta il suo codice nel nome (per esempio `TestR4LookupFindsEveryObject`).
- Nessuna dipendenza esterna (ADR 0004).
- Le firme sotto sono proposte: un cambiamento va riportato in questa scheda.
- Nessun test o commento afferma un comportamento di AutoCAD: i CAD di prova sono ZWCAD 2023 e CadCraft 0.16.0.

Dipendenze: T3 → T4, T5 → T6, T7 → T8 → T9 (T9 dipende anche da T1, chiuso).

---

## T3 – Package `dxf/tags`

**Stato al 2 novembre 2026**: il package esiste (`tags/tag.go`, `reader.go`, `writer.go`, `errors.go`). `scanner.go` e `groupcode.go` nel package `dxf` sono già ridotti ad adattatori e alias. Mancano i test del package e il fuzz.

**Obiettivo.** Lettura e scrittura del flusso ordinato di tag ASCII, con numero di riga e limiti, senza conoscenza di sezioni o versioni.

**File e package.** `tags/*.go`, `tags/*_test.go` (nuovi); `scanner.go`, `groupcode.go`, `diagnostics.go` nel package `dxf` (solo adattatori).

**API pubblica (stato attuale).**

```go
package tags

type Tag struct { Code int; Value string }
func (t Tag) String() string
func (t Tag) Int() (int, error)
func (t Tag) Float() (float64, error)
func (t Tag) Bool() (bool, error)
func (t Tag) Hex() (uint64, error)
func (t Tag) Kind() Kind

type Kind int // KindUnknown, KindString, KindFloat, KindInt, KindBool, KindHandle, KindBinary
func KindOf(code int) Kind
func IsValidCode(code int) bool // -5..1071

type Option func(*Reader)
func WithMaxValueBytes(n int) Option
const DefaultMaxValueBytes = 1 << 20
func NewReader(r io.Reader, opts ...Option) *Reader
func (r *Reader) Next() (Tag, error)              // io.EOF a fine flusso pulita
func (r *Reader) Peek() (Tag, bool, error)
func (r *Reader) Line() int                       // riga del codice dell'ultimo tag
func (r *Reader) NonASCII() (count, firstLine int)

type WriterOption func(*Writer)
func WithCRLF() WriterOption
func WithPaddedCodes() WriterOption
func NewWriter(w io.Writer, opts ...WriterOption) *Writer
func (w *Writer) Write(t Tag) error
func (w *Writer) WriteAll(ts []Tag) error
func (w *Writer) Flush() error

var ErrSyntax, ErrLimit error
type SyntaxError struct { Line int; Section, Entity string; Code int; Value, Reason string }
type LimitError struct { Kind string; Limit, Line int }
```

Il documento di specifica nomina le opzioni `WithLimits` e `WithEncoding` del Reader. Proposta: in `tags` resta `WithMaxValueBytes` (l'unico limite che riguarda un singolo tag); gli altri campi di `dxf.Limits` (entità, blocchi, annidamento) restano nel package `dxf`. `WithEncoding` arriva con T4.

**Requisiti.** R1, R14 (benchmark).

**Test.**

1. `TestR1ReadWriteSameSequence`: flusso con LF, CRLF, codici con spazi iniziali, righe vuote tra le coppie; lettura, scrittura, rilettura danno la stessa sequenza di tag.
2. Errori con riga: codice non numerico, codice fuori da -5..1071, coppia incompleta a fine file; `errors.Is(err, ErrSyntax)` e `SyntaxError.Line` corretto.
3. Limite: valore più lungo di `WithMaxValueBytes` → `*LimitError`, anche con limite piccolo (< 64) e su riga senza a capo.
4. `Peek` seguito da `Next` restituisce lo stesso tag e la stessa `Line()`.
5. Accessor: `Int("1.0")` = 1, `Int("1.5")` errore, `Bool("0")`, `Hex("1A")`, `Hex("0x1A")` errore.
6. `KindOf` per un codice di ogni intervallo, inclusi 105, 330, 390, 1071, e codici non assegnati (`KindUnknown`).
7. Writer: rifiuta codice fuori intervallo e valore con `\r` o `\n`; dopo il primo errore ogni chiamata lo restituisce; `WithPaddedCodes` produce `  0`, ` 10`, `100`.
8. `FuzzReader`: nessun panic; ogni tag letto e riscritto si rilegge uguale.
9. Benchmark del package e `BenchmarkDecodeLines` del package `dxf` entro il 10% di F0 (255 ms).

**Criterio di fine.** Test 1–9 verdi; `go vet` pulito; `scanner.go` e `groupcode.go` non contengono logica di tokenizzazione; nessuna regressione F0.

---

## T4 – Codifica dei caratteri (ADR 0004)

**Stato**: in corso (altro agente). Scheda come specifica attesa.

**Obiettivo.** Leggere e scrivere testo non ASCII in modo corretto per ogni versione: UTF-8 da R2007, CP1252 o Latin-1 prima, avviso per le altre code page, `\U+XXXX` per i caratteri non rappresentabili.

**File e package.** `tags/encoding.go` (nuovo) con le tabelle; `tags/reader.go`, `tags/writer.go`; nel package `dxf`: `reader.go` (scelta della codifica dopo l'HEADER), `writer.go` (codifica di scrittura e `$DWGCODEPAGE`).

**API proposta.**

```go
package tags

type Encoding int

const (
	EncodingRaw    Encoding = iota // nessuna conversione: i byte passano invariati (default attuale)
	EncodingUTF8
	EncodingCP1252
	EncodingLatin1
)

// EncodingForCodePage restituisce la codifica per un valore di $DWGCODEPAGE
// (confronto senza distinzione di maiuscole). ok è false per le code page non supportate.
func EncodingForCodePage(name string) (e Encoding, ok bool)

// CodePage restituisce il valore da scrivere in $DWGCODEPAGE.
func (e Encoding) CodePage() string

// DecodeString converte byte nella codifica e in una stringa UTF-8.
func (e Encoding) DecodeString(raw string) string

// EncodeString converte una stringa UTF-8 nei byte della codifica;
// i caratteri non rappresentabili diventano \U+XXXX.
func (e Encoding) EncodeString(s string) string

func WithEncoding(e Encoding) Option            // Reader
func (r *Reader) SetEncoding(e Encoding)        // cambia codifica per i tag successivi
func WithWriterEncoding(e Encoding) WriterOption
```

Comportamento atteso:

- Il Reader parte in `EncodingRaw`. Il parser `dxf` legge l'HEADER, ricava versione (`$ACADVER`) e code page (`$DWGCODEPAGE`), chiama `SetEncoding` e riconverte i valori testuali dell'HEADER già letti con `DecodeString`.
- `$ACADVER` ≥ AC1021: `EncodingUTF8`, `$DWGCODEPAGE` ignorato per la decodifica. Byte UTF-8 non validi: avviso, byte conservati (proposta).
- `$ACADVER` < AC1021: `ANSI_1252` → CP1252; Latin-1 → `EncodingLatin1` (nome esatto in `$DWGCODEPAGE` da verificare sulle fixture). `$DWGCODEPAGE` assente: CP1252 con avviso informativo (proposta). Altra code page: avviso warning, decodifica come CP1252 (proposta: rende leggibili le accentate e conserva i byte in riscrittura).
- La tabella CP1252 mappa i cinque byte non assegnati (0x81, 0x8D, 0x8F, 0x90, 0x9D) sui caratteri di controllo U+0081… con lo stesso valore, così la decodifica è totale e reversibile.
- Writer: R2013 → `EncodingUTF8`, `$DWGCODEPAGE` = `ANSI_1252` (come nei file T1). R12 → `EncodingCP1252`, `$DWGCODEPAGE` = `ANSI_1252`. `EncodeString` scrive `\U+XXXX` con 4 cifre esadecimali maiuscole; per code point oltre U+FFFF la forma è da definire (domanda aperta).
- Lettura di `\U+XXXX` già presenti nel file: proposta, conversione nel carattere solo per le versioni prima di R2007; nei file UTF-8 la sequenza resta testo, per non violare ADR 0005.

**Compatibilità con F0.** `TestF0NonASCIIDiagnostic` legge un R12 senza `$DWGCODEPAGE` con byte `0xE8 0xD8` e si aspetta (a) un solo avviso che contiene "non-ASCII" alla riga 19 e (b) `Text.Value` con i byte originali. Con la decodifica CP1252 l'attesa (b) cade. R13 vieta di cambiare le attese: o il default per R12 senza `$DWGCODEPAGE` resta `EncodingRaw` (con il diagnostico attuale), o il test va cambiato con una decisione esplicita. Da decidere prima di chiudere T4.

**Requisiti.** R12.

**Test.**

1. `TestR12AccentsRoundTrip`: `à è ì ò ù ° ±` in TEXT e nome di layer, round-trip in R12, R2004 (lettura) e R2013.
2. Tabelle: ogni byte 0x00–0xFF CP1252 e Latin-1 → rune → byte uguale.
3. Non rappresentabile: `≥` scritto in R12 diventa `\U+2265`; riletto dà `≥`; seconda scrittura byte-identica alla prima.
4. Code page sconosciuta (`ANSI_1250`): un avviso warning con riga di `$DWGCODEPAGE`, nessun errore in modalità tollerante.
5. R2018 con UTF-8 multibyte e con byte non validi: testo corretto; avviso per i byte non validi.
6. `$DWGCODEPAGE` presente in ogni file scritto, R12 e R2013.
7. `EncodingForCodePage` senza distinzione di maiuscole; `CodePage()` inverso.
8. Fixture "Testo accentato" R2000 e R2007 (quando disponibili in `testdata/real`).

**Criterio di fine.** Test verdi; fixture accentate lette e riscritte senza differenze secondo ADR 0005; decisione su `TestF0NonASCIIDiagnostic` registrata; benchmark entro R14.

---

## T5 – Handle, owner, indice e `$HANDSEED` (ADR 0006)

**Stato**: in corso (altro agente). Esistono `handle.go` e `handle_test.go` con `Handle`, `ParseHandle`, `HandleValue`, `SetHandle`, `EntityCommon.Owner`, `Document.Lookup`, `Document.HandleIndex`, `Document.ValidateHandles`. Scheda come specifica attesa; dove il codice in corso diverge è indicato.

**Obiettivo.** Ogni oggetto ha un handle univoco e il suo owner; il documento si interroga per handle; `Encode` assegna gli handle mancanti e scrive un `$HANDSEED` corretto.

**File e package.** `handle.go`, `handle_test.go`, `entities.go` (campi `Owner`), `writer.go` (assegnazione e `$HANDSEED`), `reader.go` (lettura di `330` dalle versioni R13+).

**API proposta.**

```go
type Handle uint64
func (h Handle) String() string   // esadecimale maiuscolo, senza zeri iniziali
func (h Handle) IsZero() bool
func ParseHandle(s string) (Handle, error)

// EntityCommon, Vertex, Attribute
Handle string        // testo letto, invariato (scelta del codice in corso)
Owner  Handle        // codice 330; 0 = nessun owner
func (c *EntityCommon) HandleValue() Handle
func (c *EntityCommon) SetHandle(h Handle)

func (d *Document) Lookup(h Handle) (any, bool)
func (d *Document) HandleIndex() map[Handle]any
func (d *Document) ValidateHandles() []Diagnostic
```

Divergenze tra codice in corso e specifica, da chiudere:

- Specifica: indice "costruito alla lettura e tenuto aggiornato da `Add`". Codice: `Lookup` scandisce il documento a ogni chiamata (O(n), nessuna cache) e `HandleIndex` restituisce un'istantanea. Non esiste un metodo `Add`. Proposta: accettare la scelta del codice (i campi di `Document` sono pubblici, una cache diventerebbe incoerente) e togliere `Add` dalla specifica.
- `Vertex` e `Attribute` hanno `Handle string` ma non `Owner`: per R2013 l'owner di VERTEX e ATTRIB va scritto (vedi `dxf-r2013.md`); proposta, il writer lo calcola dall'entità contenitore senza nuovo campo.
- `walkHandles` considera solo il codice 5: va aggiunto il `105` dei record DIMSTYLE e, per `$HANDSEED`, il massimo dei codici di tipo handle nei tag grezzi (R5 "anche dentro tag conservati").

Regole di `Encode`:

- Gli handle esistenti non cambiano (testo compreso).
- Primo handle libero = max(`$HANDSEED` letto, massimo handle presente + 1). Assegnazione in ordine di scrittura, deterministica.
- `$HANDSEED` scritto = ultimo handle assegnato + 1.
- Handle duplicati: `ErrValidation` con i due oggetti nel messaggio.
- R12: comportamento F0 invariato (handle solo se il documento li usa, nessun `330`).
- R2013: ogni oggetto riceve un handle, anche se il documento non ne ha (tutte le tabelle, record, BLOCK/ENDBLK, SEQEND, oggetti).

Owner:

- Lettura R13+: `330` dell'entità → `Owner`; gli altri `330` (reattori dentro `102 {...}`) restano nei tag conservati.
- Owner inesistente: avviso in modalità tollerante, `ErrValidation` in modalità rigorosa (R4). Proposta: il controllo si fa in `Decode` alla fine della lettura e in `Encode` prima di scrivere.
- In scrittura R2013 l'owner si ricalcola dal contenitore (BLOCK_RECORD del blocco, R6), non si copia da `Owner`. Se `Owner` letto differisce: proposta, prevale il contenitore e si emette un avviso.

**Requisiti.** R4, R5.

**Test.**

1. `TestR4LookupFindsEveryObject`: su un file R2013 letto, `Lookup` trova ogni entità, record, blocco, vertice e attributo.
2. `TestR4MissingOwner`: owner pendente → avviso (tollerante) e `ErrValidation` (rigoroso).
3. `TestR5HandseedAboveMax`: con handle nei tag grezzi (`RawEntity`, sezioni sconosciute, `105`) `$HANDSEED` è maggiore di tutti.
4. `TestR5NoDuplicates`: documento misto (alcuni handle presenti, altri mancanti) → nessun duplicato, handle esistenti invariati.
5. Idempotenza: `Encode(Decode(Encode(doc)))` byte-identico (ADR 0005).
6. `ParseHandle`: minuscole, spazi, `0`, valore oltre 64 bit, `0x` prefisso.
7. R12 senza handle: output identico a F0 (test esistenti).
8. Test sulle fixture reali (T10): `$HANDSEED` scritto > massimo.

**Criterio di fine.** Test verdi, divergenze sopra risolte e riportate qui, `ValidateHandles` senza errori su tutti i file scritti.

---

## T6 – Reader multi-versione

**Obiettivo.** Leggere R12–R2018 conservando marcatori di sottoclasse, gruppi `102`, `330`, `360`, `1001`; introdurre `RawObject`; riconoscere il DXF binario.

**File e package.** `reader.go` (riconoscimento versione, sezioni CLASSES e OBJECTS, entità con sottoclassi), `entities.go` (decodifica tollerante ai `100`), `objects.go` (nuovo: `Object`, `RawObject`), `dxf.go` (costanti di versione).

**API proposta.**

```go
const (
	VersionR12   = "AC1009"
	VersionR13   = "AC1012"
	VersionR14   = "AC1014"
	VersionR2000 = "AC1015"
	VersionR2004 = "AC1018"
	VersionR2007 = "AC1021"
	VersionR2010 = "AC1024"
	VersionR2013 = "AC1027"
	VersionR2018 = "AC1032"
)

// EntityCommon
Subclasses []string // marcatori 100 letti, nell'ordine; vuoto per R12

type Object interface {
	Type() string
	ObjectHandle() Handle
	ObjectOwner() Handle
}

type RawObject struct {
	TypeName string
	Handle   string
	Owner    Handle
	Groups   []Group // tutti i tag dopo (0, TypeName), nell'ordine originale
}

// Document
Objects []Object
```

Comportamento:

- `$ACADVER` fra le nove versioni elencate: accettata. Altra versione: `*VersionError` (`ErrUnsupportedVersion`) in modalità rigorosa, avviso in modalità tollerante (R2). Il messaggio di `VersionError` oggi dice "expected AC1009": va aggiornato.
- R13 e R14: lettura best effort, senza fixture reali (ZWCAD non li salva).
- Entità note in R2000+: i `100` non sono dati, si registrano in `Subclasses`; i gruppi della seconda sottoclasse (`AcDbArc`, `AcDbText` secondo, `AcDbAttribute`, `AcDb2dVertex`…) si leggono come gli altri. Codici con lo stesso numero in sottoclassi diverse (per esempio `73` di TEXT, `70` di ATTRIB) vanno distinti per sottoclasse (da verificare sulle fixture).
- `102 {...}`, `360`, `1001`+: conservati in `EntityCommon.Raw` con la posizione relativa (ADR 0007).
- Entità sconosciute: `RawEntity` con tutti i tag, inclusi `100`. Oggetti: `RawObject`, tranne quelli interpretati da T8.
- DXF binario (ADR 0008): prima del primo tag il reader guarda i primi byte; se iniziano con la firma `AutoCAD Binary DXF` (seguita da `\r\n\x1a\x00`, da verificare su un file reale) restituisce un errore con `errors.Is(err, ErrNotDXF)` e messaggio che nomina il DXF binario.

**Requisiti.** R2, R3.

**Test.**

1. `TestR2Versions`: per ogni costante, file minimo con quella `$ACADVER` letto senza errori; `AC1099` → errore rigoroso, avviso tollerante.
2. `TestR3ReactorsAndXDict`: file R2018 con `102 {ACAD_REACTORS}`, `102 {ACAD_XDICTIONARY}`, `1001 ACAD` letto e riscritto con la stessa sequenza di tag.
3. `TestR3SubclassesRecorded`: `Subclasses` di LINE, ARC, TEXT, VERTEX, INSERT, ATTRIB come da tabella di `dxf-r2013.md`.
4. Entità sconosciuta R2013 (LWPOLYLINE, MTEXT, HATCH): `RawEntity` con tag identici.
5. Oggetti sconosciuti (`XRECORD`, `MLINESTYLE`, `SCALELIST`): `RawObject` con tag identici.
6. `TestD8BinaryDXF`: firma binaria → `ErrNotDXF`, messaggio esplicito.
7. Fuzz: `FuzzDecode` esteso con un seme R2013 (v00 di T1) e un seme R2018.
8. Fixture "Vuoto" e "Entità supportate" R2000–R2018 lette in modalità rigorosa senza avvisi.

**Criterio di fine.** Test verdi; tutte le fixture reali disponibili lette in modalità rigorosa; nessuna perdita di tag secondo ADR 0005 sulle fixture; benchmark entro R14.

---

## T7 – Tabelle tipizzate e BLOCK_RECORD

**Obiettivo.** Accesso tipizzato a LAYER, LTYPE, STYLE, APPID, VPORT; tabella BLOCK_RECORD generata; VIEW, UCS, DIMSTYLE conservate senza perdite.

**File e package.** `dxf.go` (o `tables.go` nuovo), `linetypes.go`, `writer.go` (tabelle R2013), `reader.go`.

**API proposta.** Le funzioni `EnsureLayer`, `EnsureLinetype`, `EnsureStyle`, `EnsureAppID` e `Table.Records` restano invariate (R7). Si aggiungono viste tipizzate sopra `Record`, senza cambiare la memorizzazione in `[]Group` (così i tag sconosciuti dei record restano):

```go
type Layer struct {
	Name      string
	Flags     int
	Color     int     // 62; negativo = spento
	Linetype  string  // 6
	LineWeight int    // 370
	Plot      bool    // 290; default true
	Handle    string
}
func (t *Tables) Layer(name string) (Layer, bool)
func (t *Tables) SetLayer(l Layer)

type Linetype struct { Name, Description string; Flags int; Pattern []float64; Handle string }
func (t *Tables) Linetype(name string) (Linetype, bool)

type TextStyle struct { Name, Font, BigFont string; Height, WidthFactor, Oblique float64; Flags, GenFlags int; Handle string }
func (t *Tables) TextStyle(name string) (TextStyle, bool)

type BlockRecord struct {
	Name   string
	Handle string
	Layout Handle // 340
	Units  int    // 70
	Explodable bool // 280
	Scalable   bool // 281
}
// Tables
BlockRecords []BlockRecord
```

Comportamento:

- Lettura: la tabella BLOCK_RECORD letta popola `BlockRecords` (con i tag non interpretati conservati, campo `Raw []Group` da aggiungere se servono).
- `Encode` R2013 crea un BLOCK_RECORD per ogni `Block` che non ne ha uno, più `*Model_Space` e `*Paper_Space` (R6).
- `Encode` R2013 aggiunge se mancano LTYPE `ByBlock`, `ByLayer`, `Continuous`, APPID `ACAD`, LAYER `0`, STYLE `Standard`, VPORT `*Active`, DIMSTYLE `Standard` (R8, `dxf-r2013.md`). I nomi si confrontano senza distinzione di maiuscole: `NewDocument()` crea oggi `CONTINUOUS` e `STANDARD` maiuscoli, il file R2013 usa `Continuous` e `Standard`; un documento con `CONTINUOUS` non deve ricevere un secondo `Continuous`.
- Record con intestazione R2013: `5`, `330` = handle tabella, `100 AcDbSymbolTableRecord`, `100` del tipo (tabella in `dxf-r2013.md`). DIMSTYLE usa `105`.
- R12: tabelle scritte come in F0.

**Requisiti.** R6, R7, R8.

**Test.**

1. `TestR6BlockRecordPerBlock`: documento con due blocchi utente → quattro BLOCK_RECORD; owner di ogni entità di blocco = handle del suo BLOCK_RECORD.
2. `TestR7EnsureFunctionsUnchanged`: i test F0 su `EnsureLayer` ecc. passano; `Layer("0")` restituisce colore 7, linetype `Continuous`.
3. `TestR7LayerFields`: 62 negativo, 370, 290 letti e riscritti.
4. `TestR8MinimalFileTables`: `NewDocumentVersion(VersionR2013)` scritto contiene `ByBlock`, `ByLayer`, `Continuous`, `ACAD`.
5. Nessun duplicato per differenza di maiuscole (`CONTINUOUS` / `Continuous`).
6. VIEW, UCS, DIMSTYLE di un file reale riscritte identiche.

**Criterio di fine.** Test verdi; tabelle del file vuoto R2013 uguali, a meno degli handle, a quelle di `v00` di T1.

---

## T8 – CLASSES, OBJECTS, dizionari e LAYOUT

**Obiettivo.** Leggere e conservare CLASSES e OBJECTS; generare in scrittura dizionario radice, `ACAD_GROUP`, `ACAD_LAYOUT`, `ACAD_PLOTSTYLENAME` con `Normal`, LAYOUT `Model` e `Layout1`.

**File e package.** `objects.go` (da T6), `classes.go` (nuovo), `writer.go`, `reader.go`.

**API proposta.**

```go
type Class struct {
	DXFName   string // 1
	CppName   string // 2
	AppName   string // 3
	Flags     int    // 90
	Instances int    // 91
	WasProxy  bool   // 280
	IsEntity  bool   // 281
}
// Document
Classes []Class

type Dictionary struct {
	Handle   string
	Owner    Handle
	Cloning  int               // 281
	Entries  []DictionaryEntry // in ordine
	Default  Handle            // 340, solo ACDBDICTIONARYWDFLT
	WithDefault bool
	Raw      []Group
}
type DictionaryEntry struct { Name string; Value Handle } // 3 / 350 (o 360)

type Layout struct {
	Handle      string
	Owner       Handle
	Name        string  // 1 di AcDbLayout
	TabOrder    int     // 71
	BlockRecord Handle  // 330 finale
	PlotSettings []Group // AcDbPlotSettings conservato
	Raw         []Group
}

func (d *Document) RootDictionary() *Dictionary
```

Comportamento:

- Lettura: CLASSES → `Classes`, conservate e riscritte identiche (R9). OBJECTS: `DICTIONARY`, `ACDBDICTIONARYWDFLT`, `LAYOUT` interpretati; il resto `RawObject` (R10). L'ordine originale degli oggetti si conserva.
- Scrittura R2013: oggetti e classi di `dxf-r2013.md` generati solo se mancano; il dizionario radice è il primo oggetto. `340` dei BLOCK_RECORD `*Model_Space`/`*Paper_Space` → handle del LAYOUT corrispondente; `330` finale del LAYOUT → BLOCK_RECORD. Per i LAYOUT generati i valori sono quelli di `v00` (A3, 420 × 297).
- Il segnaposto `Normal` è referenziato dal `390` di ogni layer (proposta, come il layer `0` di v00: da verificare per gli altri layer).
- R12: CLASSES e OBJECTS non si scrivono; se il documento ne ha, vale la regola di T9 sulla conversione.

**Requisiti.** R9, R10.

**Test.**

1. `TestR9NonStandardClassesPreserved`: classe con nome inventato riscritta identica.
2. `TestR10RawObjectsPreserved`: file con `ACAD_MLINESTYLE`, `SCALELIST`, `XRECORD` riscritto senza modifiche.
3. `TestR10MinimalObjects`: file vuoto R2013 ha radice (owner 0), `ACAD_GROUP`, `ACAD_LAYOUT` con `Model` e `Layout1`, `ACAD_PLOTSTYLENAME` con `Normal`.
4. Coerenza dei riferimenti: ogni `350`, `340`, `330`, `390` del file vuoto punta a un handle esistente (`ValidateHandles` + controllo owner di T5).
5. File letto che ha già il suo dizionario radice: nessun secondo dizionario radice in scrittura.
6. Classi generate solo per oggetti presenti.

**Criterio di fine.** Test verdi; OBJECTS e CLASSES del file vuoto uguali, a meno degli handle, a quelle di `v00`; fixture "Oggetti non interpretati" riscritta senza differenze secondo ADR 0005.

**Stato di implementazione (lato lettura e preparazione, senza `writer.go`).**

- `Document.Classes []Class` (in `classes.go`); `Dictionary`, `Placeholder`, `Layout` (in `objects.go` e `layouts.go`) sono i soli oggetti interpretati, tutti gli altri restano `*RawObject`. Se la sezione CLASSES non è una semplice lista di record `CLASS` resta in `UnknownSections`.
- Ogni tipo interpretato conserva i gruppi letti in `Raw` e i campi tipizzati sono una vista: `Groups()` parte da `Raw` e riscrive solo i valori che non concordano più con i campi (testo degli handle, riferimenti, reattori e dati estesi restano). `Dictionary.Raw` e `Layout.PlotSettings` hanno quindi questo significato, non quello di "gruppi non interpretati" della proposta.
- Per T9: `prepareR2013Objects(doc, tbl, next)` (in `layouts.go`) va chiamata dopo `prepareR2013Tables` con lo stesso contatore. Restituisce `Classes` e `Objects` nell'ordine di scrittura (scrivere ogni oggetto come `0 <Type()>` seguito da `objectGroups(o)`, ogni classe come `0 CLASS` seguito da `Class.Groups()`), imposta il `340` dei due BLOCK_RECORD (`SetLayoutHandles`) e aggiunge il `390` ai layer solo quando genera `Normal`.
- Il documento non viene modificato. Gli oggetti letti restano nella loro posizione; quelli generati si aggiungono dopo (la radice, se manca, è il primo oggetto).
- Gli oggetti `ACAD_MLINESTYLE` e `ACAD_PLOTSETTINGS` non sono generati: non compaiono in `v00` e non sono in questo documento.

---

## T9 – Writer R2013 con marcatori; R12 invariato

**Obiettivo.** Scrivere R2013 completo (sezioni, handle, owner, marcatori di sottoclasse) e mantenere la scrittura R12 di F0.

**File e package.** `writer.go` (diviso, proposta: `writer_r12.go` con il codice attuale e `writer_r2013.go`), `dxf.go` (`NewDocumentVersion`), `doc.go` (descrizione del package), `cmd/dxfgen` (esempio, T12).

**API proposta.**

```go
func WithVersion(v string) EncodeOption        // sovrascrive Document.Version in scrittura
func NewDocumentVersion(v string) *Document    // documento vuoto della versione
```

`NewDocument()` resta R12 fino alla chiusura di F1 (ADR 0002).

Comportamento:

- Versione di scrittura = `WithVersion` se dato, altrimenti `Document.Version`; vuota → R12. Versioni scrivibili: `VersionR12`, `VersionR2013`. Altre → `ErrValidation` (proposta: le versioni lette R2000–R2018 si scrivono come R2013 solo con `WithVersion` esplicito).
- R2013: forma del file come `dxf-r2013.md` (CRLF e codici su tre colonne come i file T1: proposta, da confermare; il writer R12 oggi scrive LF senza allineamento e non va cambiato).
- Marcatori per entità dalla tabella di `dxf-r2013.md`; POLYLINE `AcDb3dPolyline` se il flag `70` ha il bit 8, altrimenti `AcDb2dPolyline`; VERTEX corrispondente. Mesh e polyface (bit 16, 64) restano fuori: `ErrValidation` o `RawEntity` (da decidere).
- `Encode` rifiuta con `ErrValidation`, nominando l'elemento, un contenuto non rappresentabile nella versione richiesta. Caso da decidere: documento letto R2000+ con `RawEntity`/`RawObject` scritto come R12 (ADR 0007).
- Determinismo: due chiamate a `Encode` sullo stesso documento danno gli stessi byte; `Encode` non modifica il documento (test F0 esistente).
- Esempio d'uso atteso:

  ```go
  doc, err := dxf.Decode(in, dxf.WithLenient())
  if err != nil { return err }
  doc.Version = dxf.VersionR2013
  doc.UpdateExtents()
  return dxf.Encode(out, doc)
  ```

  `Document.UpdateExtents` esiste già (`extents.go`).

**Requisiti.** R11, R13.

**Test.**

1. `TestR11SubclassMarkers`: per ogni entità supportata, file atteso in `testdata/r2013/<entità>.dxf`, confronto byte a byte.
2. `TestR11EmptyDocument`: `NewDocumentVersion(VersionR2013)` scritto equivale a `v00` di T1 senza le tre entità, a meno degli handle.
3. `TestR13R12Unchanged`: tutti i test F0 e le fixture R12 passano senza modifiche.
4. Conversione R12 → R2013 di ogni fixture in `testdata`: rilettura senza avvisi, `ValidateHandles` pulito, owner coerenti.
5. Round-trip byte-identico alla seconda scrittura (ADR 0005) su ogni file scritto.
6. `ErrValidation` per versione non scrivibile e per contenuto non rappresentabile.
7. Oracolo `ezdxf` (facoltativo, CI): audit senza errori sui file scritti.
8. Benchmark di scrittura entro R14.

**Criterio di fine.** Test verdi; i file scritti (vuoto, entità supportate, `structural-frame.dxf` convertito) pronti per la prova G1 in ZWCAD 2023 e CadCraft 0.16.0 (T11). La prova nei CAD non fa parte di T9.

### Stato di T9 (implementato)

Codice in `writer_r2013.go`; `Encode` accetta `WithVersion(v)`; `NewDocumentVersion(v)` in `dxf.go`. Errore per versioni non scrivibili: soddisfa `ErrUnsupportedVersion` e `ErrValidation`. Mesh POLYLINE (flag 16/64): scrivibili solo se letti da un file R13+ (marcatori conservati), altrimenti `ErrValidation`. File per la prova G1 in `testdata/g1/`. Decisioni: ADR 0007 (rifiuto), ADR 0009 (versioni, float).
