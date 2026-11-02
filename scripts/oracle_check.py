#!/usr/bin/env python3
"""Oracolo ezdxf: apre un DXF, esegue doc.audit() e stampa un report JSON.

Uso: oracle_check.py FILE.dxf
Codice di uscita: 0 = nessun errore; 1 = errori di audit; 2 = file illeggibile.
"""
import collections
import json
import sys


def main(argv):
    if len(argv) != 2:
        print("uso: oracle_check.py FILE.dxf", file=sys.stderr)
        return 2
    try:
        import ezdxf
        from ezdxf import recover
    except ImportError as exc:
        print(json.dumps({"file": argv[1], "fatal": "ezdxf non disponibile: %s" % exc}))
        return 2
    path = argv[1]
    report = {"file": path}
    try:
        doc = ezdxf.readfile(path)
    except Exception as exc:  # ezdxf solleva vari tipi (DXFStructureError, IOError)
        report["fatal"] = "%s: %s" % (type(exc).__name__, exc)
        print(json.dumps(report, indent=2))
        return 2
    report["version"] = doc.dxfversion
    report["acadver"] = doc.acad_release
    counts = collections.Counter(e.dxftype() for e in doc.modelspace())
    for layout in doc.layouts:
        if layout.name != "Model":
            counts.update(e.dxftype() for e in layout)
    block_counts = collections.Counter()
    for blk in doc.blocks:
        block_counts.update(e.dxftype() for e in blk)
    report["entities"] = dict(sorted(counts.items()))
    report["block_entities"] = dict(sorted(block_counts.items()))
    auditor = doc.audit()
    report["errors"] = [{"code": e.code, "message": e.message} for e in auditor.errors]
    report["fixes"] = [{"code": e.code, "message": e.message} for e in auditor.fixes]
    print(json.dumps(report, indent=2, ensure_ascii=False))
    return 1 if report["errors"] else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
