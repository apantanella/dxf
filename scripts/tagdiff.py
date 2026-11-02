#!/usr/bin/env python3
"""Confronta due DXF a livello di tag (code, value) e stampa la prima differenza.

Uso: tagdiff.py [--ignore-handles] [--ignore-code N ...] [--no-trim] A.dxf B.dxf
--ignore-handles ignora i codici 5, 105 e i riferimenti 330, 340, 350, 360, 390,
                 1005 (handle in XDATA), 320, 331-339, 341-349, 361-369, 380-389, 391-399
                 (riferimenti handle).
I valori sono confrontati con trim degli spazi (disattivabile con --no-trim).
Codice di uscita: 0 identici, 1 differenti, 2 errore di lettura.
"""
import sys

HANDLE_CODES = {5, 105, 320, 330, 340, 350, 360, 390, 1005}
HANDLE_CODES |= set(range(331, 340)) | set(range(341, 350)) | set(range(361, 370))
HANDLE_CODES |= set(range(380, 390)) | set(range(391, 400))


def read_tags(path, ignore_handles, ignored, trim):
    with open(path, "r", encoding="utf-8", errors="replace", newline=None) as f:
        lines = f.read().split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    if len(lines) % 2:
        raise ValueError("%s: numero dispari di righe (%d)" % (path, len(lines)))
    tags = []
    for i in range(0, len(lines), 2):
        try:
            code = int(lines[i].strip())
        except ValueError:
            raise ValueError("%s:%d: codice di gruppo non numerico %r" % (path, i + 1, lines[i]))
        value = lines[i + 1].strip() if trim else lines[i + 1].rstrip("\r")
        lineno = i + 1
        if ignore_handles and (code in HANDLE_CODES or code == 5):
            continue
        if code in ignored:
            continue
        tags.append((code, value, lineno))
    return tags


def main(argv):
    ih, trim, ignored, paths = False, True, set(), []
    it = iter(argv[1:])
    for a in it:
        if a == "--ignore-handles":
            ih = True
        elif a == "--no-trim":
            trim = False
        elif a == "--ignore-code":
            ignored.add(int(next(it)))
        else:
            paths.append(a)
    if len(paths) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    try:
        a = read_tags(paths[0], ih, ignored, trim)
        b = read_tags(paths[1], ih, ignored, trim)
    except (OSError, ValueError) as exc:
        print("errore:", exc, file=sys.stderr)
        return 2
    for n, (x, y) in enumerate(zip(a, b)):
        if x[:2] != y[:2]:
            print("prima differenza al tag #%d" % n)
            print("  %s:%d: %d / %r" % (paths[0], x[2], x[0], x[1]))
            print("  %s:%d: %d / %r" % (paths[1], y[2], y[0], y[1]))
            ctx = a[max(0, n - 3):n]
            if ctx:
                print("  contesto: " + " ".join("%d=%r" % (c, v) for c, v, _ in ctx))
            return 1
    if len(a) != len(b):
        longer, p = (a, paths[0]) if len(a) > len(b) else (b, paths[1])
        n = min(len(a), len(b))
        print("uno e' prefisso dell'altro: %s ha %d tag in piu'; primo extra %s:%d: %d / %r" % (
            p, abs(len(a) - len(b)), p, longer[n][2], longer[n][0], longer[n][1]))
        return 1
    print("identici (%d tag)" % len(a))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
