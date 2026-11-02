# ADR 0008 – DXF binario non letto in F1

## Stato

Confermata, 2026-11-02.

## Contesto

Il formato DXF esiste anche in forma binaria, con una struttura di codifica diversa da quella ASCII. Nessun requisito delle tavole lo richiede, e i CAD di prova salvano DXF ASCII. Il reader attuale, davanti a un file binario, fallisce al primo tag e restituisce `ErrNotDXF` senza spiegare il motivo.

## Decisione

- In F1 il DXF binario non viene letto né scritto.
- Il reader riconosce un file binario dalla firma iniziale e restituisce un errore che soddisfa `errors.Is(err, ErrNotDXF)`, con un messaggio esplicito che dice che il file è DXF binario e che il formato non è supportato.

## Alternative scartate

- Supportarlo subito: è un secondo formato di codifica da leggere, testare e mantenere, senza un caso d'uso che lo richieda.

## Conseguenze

- Firma da riconoscere (da verificare su un file reale prima di fissarla nei test): la sequenza ASCII `AutoCAD Binary DXF` seguita da `\r\n`, `0x1A`, `0x00` all'inizio del file.
- Il riconoscimento avviene prima della tokenizzazione, leggendo i primi byte senza consumarli per gli altri casi.
- Un test con un file che inizia con la firma verifica `errors.Is(err, ErrNotDXF)` e il testo del messaggio.
- Il supporto al binario potrà essere aggiunto in una fase successiva come lettore separato sopra lo stesso modello `tags.Tag`.
