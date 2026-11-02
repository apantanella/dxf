# T1 – Esperimento sul file R2013 minimo

Scopo: scoprire quali elementi di un file DXF R2013 (AC1027) sono davvero necessari
perché i CAD lo aprano senza errori. Il risultato decide il contenuto del file
vuoto che la libreria scriverà (specifiche F1, sezione "File R2013 minimo").

I file sono scritti a mano da `generate.py` (solo Python 3, nessuna dipendenza).
`v00-baseline.dxf` contiene tutto; ogni altra variante toglie **un solo** elemento.
Ogni file contiene una linea, un cerchio e un testo `Prova àèìòù ±°`.

## Procedura (per ogni CAD, in ordine di numero)

1. Apri il file con il comando di apertura normale del CAD. Annota se compare una
   finestra di errore o di avviso, e il testo del messaggio.
2. Se si apre: controlla che si vedano la linea, il cerchio e il testo con gli
   accenti corretti (`Prova àèìòù ±°`).
3. Esegui `AUDIT` (o il comando equivalente). Annota il numero di errori.
4. Esegui "Zoom estensione" e verifica che il disegno intero sia visibile.
5. Chiudi senza salvare e passa al file successivo.

Si parte da `v00-baseline`: se non si apre quello, **fermati** e dimmelo: il
problema è nella base, non in un elemento tolto.

## Scheda risultati

Legenda: OK = si apre senza avvisi e `AUDIT` a zero; AVV = si apre con avviso o
`AUDIT` non a zero (scrivi il testo); NO = non si apre.

| File | Elemento tolto | ZWCAD 2023 | CadCraft 0.16.0 | Note |
| --- | --- | --- | --- | --- |
| v00-baseline | nulla | | | |
| v01-senza-classes | classi della sezione CLASSES | | | |
| v02-senza-plotstylename | ACAD_PLOTSTYLENAME e segnaposto Normal | | | |
| v03-senza-dimstyle | record DIMSTYLE Standard | | | |
| v04-senza-vport | record VPORT *Active | | | |
| v05-senza-extents | $EXTMIN, $EXTMAX, $LIMMIN, $LIMMAX | | | |
| v06-senza-sottoclassi | marcatori 100 AcDb su entità e blocchi | | | |
| v07-senza-layout | oggetti LAYOUT | | | |
| v08-senza-acad-group | dizionario ACAD_GROUP | | | |
| v09-senza-handseed | $HANDSEED | | | |
| v10-senza-codepage | $DWGCODEPAGE | | | |
| v11-senza-owner | 330 (owner) sulle entità | | | |
| v12-senza-block-record | tabella BLOCK_RECORD | | | |
| v13-senza-appid-acad | record APPID ACAD | | | |
| v14-solo-modello | *Paper_Space e Layout1 | | | |
| v15-senza-bylayer-byblock | LTYPE ByBlock e ByLayer | | | |

## Esito preliminare (non sostituisce la prova nei CAD)

Tutti i 16 file sono stati letti da `ezdxf` 1.4.5 in modalità rigorosa, con
`AUDIT` a zero errori. `ezdxf` è molto tollerante: l'esito conferma che la
struttura è coerente, non che i CAD la accettino.
Una differenza: in `v10-senza-codepage` `ezdxf` legge il testo accentato come
`Ã Ã¨...` (decodifica CP1252 di un file UTF-8). Se anche i CAD fanno così,
`$DWGCODEPAGE` va scritto sempre.

## Dopo la prova

Rimanda la scheda compilata. Un elemento è **indispensabile** se almeno un CAD
dà AVV o NO quando manca; è **facoltativo** se tutti danno OK. La libreria
scrive comunque gli elementi che almeno un CAD richiede, e tutto ciò che
AutoCAD potrebbe pretendere (non disponibile qui) resta nel file per prudenza.
