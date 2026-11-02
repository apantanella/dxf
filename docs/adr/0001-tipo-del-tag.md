# ADR 0001 – Tipo del tag: `tags.Tag{Code int; Value string}`

## Stato

Proposta, 2026-11-02. Da confermare. Adottata in via provvisoria dalle fasi di sviluppo in corso: `tags/tag.go` definisce già `Tag`, e `groupcode.go` dichiara `dxf.Group` come alias di `tags.Tag`.

## Contesto

Un file DXF è una sequenza ordinata di coppie codice di gruppo/valore. Fino a F0 la coppia era `dxf.Group` con il valore come stringa. F1 sposta la lettura e la scrittura del flusso nel package `dxf/tags` (T3), che diventa la base di tutto il resto, incluso il round-trip dei contenuti non interpretati (ADR 0007). Bisogna decidere come rappresentare il valore.

## Decisione

- Il tag è `tags.Tag{Code int; Value string}`: il valore resta il testo letto dal file.
- Le conversioni si fanno solo su richiesta, con accessor espliciti: `Int() (int, error)`, `Float() (float64, error)`, `Bool() (bool, error)`, `Hex() (uint64, error)`. Un errore di conversione è un `*tags.SyntaxError` con codice e valore.
- `tags.KindOf(code)` restituisce il tipo atteso per il codice (`Kind`: string, float, int, bool, handle, binary), a scopo di validazione e diagnostica; non cambia la rappresentazione.
- `dxf.Group` è un alias (`type Group = tags.Tag`), quindi i due nomi sono intercambiabili e il codice esistente che usa `Group.Code` e `Group.Value` continua a compilare.

## Alternative scartate

- Valore tipizzato per codice (per esempio un'unione int/float/string decisa da `KindOf`): costa un parsing a ogni lettura, anche dei tag che non vengono mai usati; cambia la forma testuale dei numeri, e quindi complica il round-trip; rompe il codice esistente che legge `Group.Value` come stringa.

## Conseguenze

- Leggere un tag non costa parsing; un tag non modificato si riscrive con lo stesso testo, condizione necessaria per ADR 0005 e 0007.
- Gli errori sui valori numerici emergono nel momento in cui il valore viene interpretato (decodifica di una entità nota), non durante la tokenizzazione. I tag conservati come grezzi possono contenere valori non validi per il loro codice e vengono riscritti così come sono.
- `Int()` accetta reali con parte frazionaria nulla (`"1.0"`), come fa già l'implementazione attuale; `Bool()` accetta qualsiasi intero (0 = falso).
- Il package `tags` non conosce sezioni, entità né versioni.
