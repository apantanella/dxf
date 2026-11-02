# ADR 0007 – Contenuto non interpretato: `RawEntity` e `RawObject`

## Stato

Proposta, 2026-11-02. Da confermare. Adottata in via provvisoria dalle fasi di sviluppo in corso. Scelta di T9 (scrittura come R12) registrata in fondo.

## Contesto

F1 interpreta solo le entità già supportate (LINE, POINT, CIRCLE, ARC, TEXT, SOLID, TRACE, 3DFACE, POLYLINE/VERTEX, INSERT/ATTRIB) e pochi oggetti (dizionari, LAYOUT). Le tavole reali contengono LWPOLYLINE, MTEXT, HATCH, DIMENSION, XRECORD, MLINESTYLE, SCALELIST e altri oggetti, oltre a dati di reattori (`102 {ACAD_REACTORS ... 102 }`), dizionari di estensione (`102 {ACAD_XDICTIONARY ... 360 ...}`) e dati estesi (`1001` e seguenti). F0 ha già corretto per R12 il difetto di scartare le entità sconosciute: oggi diventano `RawEntity`.

## Decisione

- Una entità che la libreria non interpreta resta `RawEntity` con tutti i suoi tag, nell'ordine originale, inclusi `100`, `102`, `330`, `360` e `1001` con i dati estesi.
- Un oggetto della sezione OBJECTS che la libreria non interpreta resta `RawObject`, con la stessa regola.
- In scrittura i tag di `RawEntity` e `RawObject` si riscrivono identici, senza riordinarli né aggiungere marcatori.
- Anche nelle entità e negli oggetti interpretati i tag non consumati (`102`, `360`, `1001`, codici sconosciuti) restano conservati, e si riscrivono nella posizione prevista dal formato.

## Alternative scartate

- Scartare il contenuto sconosciuto con un avviso: perde dati sui file reali e lascia riferimenti (owner, reattori) che puntano a oggetti spariti, con il rischio che il file riscritto non si apra.

## Conseguenze

- Un file R2018 con reattori e dizionari di estensione deve fare il giro senza perdere un tag (R3), secondo il criterio di ADR 0005.
- Gli handle presenti nei tag grezzi entrano nell'indice degli handle e nel calcolo di `$HANDSEED` (ADR 0006).
- Scrivere come R12 un documento che contiene `RawEntity` o `RawObject` letti da un file R2000+ può produrre un file non valido: `Encode` deve rifiutarlo con `ErrValidation` oppure scartarli con un avviso. Scelta da fare in T9.
- La memoria aumenta su disegni grandi: il benchmark di T10 misura l'effetto (R14).
- Per conservare la posizione dei tag non consumati nelle entità interpretate serve sapere dopo quale marcatore di sottoclasse si trovavano: dettaglio di implementazione di T6.

## Scelta di T9: scrittura come R12

`Encode` in R12 rifiuta il documento con `ErrValidation` (`ValidationError` con motivo e suggerimento "convertire il documento a R2000 o successivo o ricostruirlo senza quel contenuto") quando contiene `RawEntity`, `RawObject`, classi, oggetti, sottoclassi (`Subclasses`) o dati verbatim letti da R2000+. Niente viene scartato in silenzio: lo scarto lascerebbe riferimenti (owner, reattori) verso oggetti spariti. L'`Owner` da solo non rende il documento incompatibile, perche' in R12 viene ignorato. R12 verso R12 resta byte per byte invariato. Un documento R2000+ senza contenuto non rappresentabile si scrive in R12 senza errori.
