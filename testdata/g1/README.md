# Prova G1: file R2013 scritti dalla libreria

Questi quattro file sono scritti dalla libreria (`Encode`, versione AC1027) e verificati con l'audit di ezdxf (nessun errore). La prova a mano in un CAD reale non e' ancora stata fatta: serve a decidere se il writer R2013 e' accettato (task T11).

I file si rigenerano con `DXF_UPDATE_GOLDEN=1 go test -run TestG1Files .`; senza la variabile il test controlla che coincidano con l'output attuale del writer.

| File | Contenuto |
|------|-----------|
| `01-vuoto-r2013.dxf` | documento vuoto R2013 (uguale a `testdata/t1/v00-baseline.dxf` senza le 3 entita', a meno degli handle) |
| `02-entita-supportate.dxf` | LINE, POINT, CIRCLE, ARC, TEXT, SOLID, TRACE, 3DFACE, POLYLINE (con VERTEX e SEQEND) su layer STR-BEAMS e STR-PANELS |
| `03-blocco-insert-attrib.dxf` | blocco COLONNA (un cerchio) e INSERT con ATTRIB `TAG` = `C1` |
| `04-structural-frame-r2013.dxf` | `testdata/structural-frame.dxf` (R12) convertito a R2013 |

## Procedura

Per ogni file, in ZWCAD 2023 e poi in CadCraft 0.16.0:

1. Aprire il file (File > Apri). Annotare se compaiono messaggi di errore, di recupero o di avviso.
2. Controllare che il disegno si veda: eseguire zoom estensione e verificare che le entita' elencate siano presenti, sui layer attesi e con i colori attesi.
3. Per il file 03: verificare che l'INSERT mostri il cerchio del blocco e che l'attributo `C1` sia visibile e modificabile; aprire l'editor blocchi e controllare che il blocco COLONNA esista.
4. Eseguire il comando di verifica (AUDIT, o RECOVER se previsto) e annotare gli errori segnalati.
5. Salvare con nome (stesso formato o DWG) e riaprire il file salvato: deve aprirsi senza messaggi.
6. Per ZWCAD: controllare anche la scheda Layout (modello e un layout) e il gestore layer.

## Risultati (da compilare)

Scrivere OK, KO oppure il testo del messaggio mostrato.

| File | ZWCAD 2023: apre | ZWCAD: entita' e layer | ZWCAD: AUDIT | ZWCAD: salva e riapre | CadCraft 0.16.0: apre | CadCraft: entita' e layer | Note |
|------|------|------|------|------|------|------|------|
| 01-vuoto-r2013 | | | | | | | |
| 02-entita-supportate | | | | | | | |
| 03-blocco-insert-attrib | | | | | | | |
| 04-structural-frame-r2013 | | | | | | | |

## Cosa osservare in particolare

- Valori reali in notazione scientifica (`1e+20` in `$EXTMAX`/`$LIMMAX` di default o simili): ZWCAD li ha accettati in T1, da riconfermare in CadCraft.
- POLYLINE con VERTEX: owner dei vertici e SEQEND.
- ATTRIB dell'INSERT: ordine dei gruppi (`72`, `74`) e visibilita'.
- Se un file non si apre, provare prima il numero 01: se anche quello fallisce il problema e' nell'intestazione o nelle tabelle, non nelle entita'.
