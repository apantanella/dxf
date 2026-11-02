# ADR 0010 – Entità di F2: colore vero, spessore, entità nuove

## Stato

Proposta, 2026-11-02 (T0). Adottata in via provvisoria dalle fasi di sviluppo in corso. Registra le decisioni D9–D14 di `docs/f2-guida-implementazione.md` più due regole che F2 altrimenti amplificherebbe (D15, D16).

## Contesto

F2 aggiunge le entità di una tavola strutturale moderna: LWPOLYLINE, MTEXT, HATCH, DIMENSION, LEADER, MLEADER, ATTDEF, ELLIPSE e SPLINE, con colore vero (420/430/440), spessore di linea (370) e tipi di linea standard. Le decisioni D9–D14 del documento di specifica vanno registrate prima di scrivere il codice, altrimenti ogni entità viene progettata due volte. Due debiti di F1 rendono la scelta obbligata: la regola per i valori uguali al default, e il `72` di ATTRIB che F1 legge dalla sottoclasse sbagliata.

## Decisione

- **D9 – Colore.** `EntityCommon` mantiene `Color`/`HasColor` (colore ACI, `62`) e aggiunge `TrueColor *RGB` (`420`), `ColorName string` (`430`) e `Transparency *int` (`440`). Il `420` coesiste con il `62` nel file: quando è presente vince nella visualizzazione, ma il `62` si scrive comunque. Non rompe l'API di F1.
- **D10 – LWPOLYLINE.** Tipo proprio `LWPolyline`, con `ToPolyline()` e `FromPolyline()`; nessuna conversione implicita in lettura. Le due entità hanno sottoclassi e round-trip diversi.
- **D11 – Entità nuove in R12.** LWPOLYLINE → POLYLINE 2D automatico; le altre entità nuove danno `ErrValidation` con un messaggio che nomina il tipo. R12 non ha una forma equivalente: meglio un errore che una perdita muta.
- **D12 – Geometria delle quote.** La libreria conserva il blocco anonimo `*Dn` letto e ne genera uno minimo per lineare, allineata, raggio e diametro (una quota senza blocco in alcuni CAD non si vede finché non si rigenera). La quota completa è F4.
- **D13 – MLEADER.** Pochi campi tipizzati (testo, punto di aggancio, linea con N punti, stile, scala); tutto il resto resta in `Raw` nell'ordine originale. La struttura è annidata e varia con la versione: non si scrive dalla specifica senza una fixture reale.
- **D14 – Campiture predefinite.** Catalogo scritto da noi (`patterns.go`) per ANSI31, ANSI32, ANSI37, EARTH, AR-CONC, AR-SAND, STEEL e LINE; niente copia del file `acad.pat`, per non avere problemi di licenza. Sono righe di segmenti definite dalla geometria.
- **D15 – Valori uguali al default.** Un valore presente nel file si conserva nel campo, con un indicatore `Set` (come `ExtrusionSet`) quando non è distinguibile dal default; i nuovi tipi seguono questa regola. Un tipo si decodifica **solo se si riscrive identico**: un sottoblocco che non si sa riscrivere resta in `Raw`. Così un `41 1.0` su INSERT, o una larghezza `40 0.0` su un vertice di LWPOLYLINE, non svaniscono.
- **D16 – ATTRIB e il `72`.** L'allineamento orizzontale (`72`) appartiene ad `AcDbText`. Il `72` che i file R2018 scrivono dentro `AcDbAttribute` (dopo il `71`) **non** è l'allineamento: resta nei gruppi non consumati (`Raw`) e si riscrive dove si trovava. `decodeAttribute` legge il `72` solo dalla sottoclasse preferita (`AcDbText`); il workaround del writer per R2018 è stato rimosso.

## Alternative scartate

- Colore vero come campo unico che sostituisce il `62`: perde l'informazione ACI che i CAD scrivono comunque, e rompe il round-trip dei file che hanno entrambi.
- Convertire LWPOLYLINE in POLYLINE alla lettura: perde bulge/larghezze per vertice e non torna indietro.
- Rifiutare LWPOLYLINE anche in R12: una polilinea 2D è esprimibile, e la conversione è utile a chi scrive R12.
- Generare sempre il blocco della quota anche quando è stato letto: raddoppia il lavoro e può divergere dal blocco del CAD.
- Leggere il `72` di ATTRIB con il fallback di sottoclasse (comportamento di F1): costringe il writer a un workaround e confonde due codici diversi.

## Conseguenze

- `EntityCommon` cresce di campi; il lettore e il writer comuni (`gs.common()` e `writeRecord`) gestiscono colore vero, trasparenza e spessore in un solo punto, non in ogni `case`.
- Lo spessore `370` è validato contro l'elenco dei valori ammessi (`0, 5, 9, ... 211`, più `-1`, `-2`, `-3`); un valore fuori elenco è un errore di validazione. Vale anche per LAYER e DIMSTYLE (`371`, `372`).
- Il writer R2013 aggiunge in automatico un record APPID per ogni applicazione nominata da dati estesi (`1001`), perché quote e MLEADER usano `ACAD` e `ACAD_DSTYLE_*`; un file che usa un'applicazione non definita è rifiutato da alcuni CAD.
- La conversione R12 (D11) e la generazione del blocco della quota (D12) sono i due punti in cui F2 può perdere dati se implementati male: hanno test dedicati (Q7, Q8).
- `NewDocument()` è R2013 da T0 (ADR 0002), perché queste entità non esistono in R12.
