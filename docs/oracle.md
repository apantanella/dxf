# Oracolo ezdxf

Infrastruttura di verifica per F1 (task T10, requisito R-oracolo, ADR 0005).
Usa `ezdxf` come lettore/validatore indipendente dei DXF letti e scritti dalla libreria.

## Contenuto

| File | Scopo |
|---|---|
| `scripts/oracle_check.py FILE` | Apre il DXF con ezdxf, esegue `doc.audit()`, stampa JSON (versione, entità per tipo nel modello e nei blocchi, errori, fix). Uscita 0 senza errori, 1 con errori di audit, 2 file illeggibile. |
| `scripts/gen_fixtures.py [DIR]` | Rigenera le fixture in `testdata/oracle/`. |
| `scripts/tagdiff.py [--ignore-handles] [--ignore-code N] [--no-trim] A B` | Confronta due DXF tag per tag `(codice, valore)` e stampa la prima differenza (con righe e contesto). Con `--ignore-handles` salta i codici 5, 105, 320, 330-399 e 1005. Uscita 0 identici, 1 diversi. Per i test di round-trip. |
| `oracle_test.go` (tag `oracle`) | `TestOracleFixtures` e l'helper `runOracle(t, path)`. |

## Uso

```
pip install --break-system-packages ezdxf
python3 scripts/gen_fixtures.py
python3 scripts/oracle_check.py testdata/t1/v00-baseline.dxf
go test -tags=oracle -run TestOracle -v .
```

`TestOracleFixtures` per ogni `testdata/oracle/*.dxf`: (1) esegue l'oracolo, che deve passare l'audit; (2) legge il file con `dxf.Decode` in modalità rigorosa (senza `WithLenient`). Se il lettore restituisce un errore noto (versione non supportata, sintassi, validazione, limiti) il sotto-test viene saltato con `t.Skip` e l'errore è registrato, non fallisce. Un errore di altro tipo fallisce. Se `python3` o `ezdxf` mancano, tutto è saltato.

### Riuso da T9 (file scritti dalla libreria)

```go
path := filepath.Join(t.TempDir(), "out.dxf")
// ... doc.Encode(file) ...
rep := runOracle(t, path) // fallisce se ezdxf non legge o l'audit ha errori
```

`runOracle` salta il test se l'oracolo manca, e restituisce `*oracleReport` (versione, conteggi entità) per ulteriori asserzioni. Per il round-trip: `python3 scripts/tagdiff.py --ignore-handles in.dxf out.dxf`.

## Fixture: sono SINTETICHE

I file `testdata/oracle/r20XX-{empty,entities}.dxf` (R2000, R2004, R2007, R2010, R2013, R2018) sono **generati da ezdxf**, non da un CAD reale. `*-entities` contiene LINE, CIRCLE, ARC, TEXT, POINT, POLYLINE con VERTEX, un blocco (`PORTA`) con INSERT e ATTRIB, più layer e linetype diversi. Riflettono le convenzioni di ezdxf, non quelle di ZWCAD/AutoCAD. **Le fixture reali da ZWCAD arrivano dall'utente** e vanno aggiunte a parte (non rigenerabili).

## Limiti

- ezdxf è **permissivo**: ripara o ignora molte anomalie e l'audit non copre tutte le regole di AutoCAD (ordine di tabelle/oggetti, handle, dizionari obbligatori). Superare l'oracolo è condizione necessaria, non sufficiente.
- **Non sostituisce** l'apertura in ZWCAD/CadCraft (e AutoCAD), che resta la verifica definitiva.
- Gli oggetti non modellati da ezdxf sono ignorati senza errore.
- Le fixture sintetiche non esercitano le stranezze dei file reali (XDATA, oggetti proprietari, estensioni).
