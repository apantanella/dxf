# ADR 0006 – Handle

## Stato

Proposta, 2026-11-02. Da confermare. Adottata in via provvisoria dalle fasi di sviluppo in corso (T5).

## Contesto

Da R13 in poi ogni oggetto ha un handle (codice `5`, `105` per DIMSTYLE) e quasi ogni oggetto ha un owner (codice `330`); molti altri codici contengono riferimenti ad handle (320–369, 390–399, 480–481). Nei file reali questi riferimenti incrociati legano entità, tabelle, dizionari e layout. Oggi `EntityCommon.Handle` è una `string`, il writer R12 assegna handle solo se il documento li usa (`$HANDLING` = 1 o handle già presenti) e calcola `$HANDSEED` dal massimo trovato.

## Decisione

- `type Handle uint64` nel package `dxf`, con `String()` in esadecimale maiuscolo senza zeri iniziali; il valore 0 indica "nessun handle".
- Sui documenti letti gli handle non si rinumerano mai.
- `Encode` assegna un handle solo agli oggetti che non ne hanno uno, partendo da `$HANDSEED` (o dal massimo handle presente + 1, se maggiore), e scrive un `$HANDSEED` maggiore di ogni handle presente, compresi quelli contenuti nei tag conservati grezzi.
- Due oggetti con lo stesso handle in scrittura sono un errore `ErrValidation`.

## Alternative scartate

- Rinumerare tutti gli handle a ogni scrittura: rompe i riferimenti incrociati dei file reali, in particolare quelli dentro oggetti e tag che la libreria non interpreta e quindi non saprebbe aggiornare.

## Conseguenze

- Il documento mantiene un indice handle → oggetto (`Document.Lookup`), costruito in lettura e aggiornato da `Add` (T5).
- Un owner che punta a un handle inesistente produce un avviso in modalità tollerante e `ErrValidation` in modalità rigorosa.
- Compatibilità (ADR 0002): `EntityCommon.Handle` e `Attribute.Handle` sono oggi `string`. Cambiarli in `Handle` rompe il codice che li assegna come stringa. Opzioni da decidere insieme a questo ADR: (a) cambiare il tipo del campo, accettando l'incompatibilità perché non esiste una 1.x; (b) lasciare `Handle string` e aggiungere `HandleID() (Handle, error)` più un campo `Owner Handle`. Il codice in corso di T5 (`handle.go`, 2 novembre 2026) segue l'opzione (b): `Handle string` resta il testo letto, con `HandleValue()` e `SetHandle(Handle)`, ed `EntityCommon.Owner` è di tipo `Handle`. Se l'ADR viene confermato così, `type Handle uint64` è il tipo dell'API, non della memorizzazione del testo letto.
- Gli handle letti dal file possono avere zeri iniziali o minuscole; la normalizzazione del testo dell'handle rientra nella "formato dei numeri" di ADR 0005 solo se la si dichiara: da chiarire in T5.
- Il calcolo di `$HANDSEED` deve considerare anche il codice `105` (DIMSTYLE) e, contro i riferimenti pendenti, i tag grezzi con codici di tipo handle (`tags.KindHandle`); oggi `visitGroups` e `walkHandles` considerano solo il codice 5.
