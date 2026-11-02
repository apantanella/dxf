#!/usr/bin/env python3
"""Genera le fixture SINTETICHE di testdata/oracle/ con ezdxf.

Per ogni versione (R2000..R2018): <ver>-empty.dxf e <ver>-entities.dxf.
Uso: gen_fixtures.py [DIR]   (default: testdata/oracle accanto a scripts/)
"""
import os
import sys

import ezdxf

VERSIONS = ["R2000", "R2004", "R2007", "R2010", "R2013", "R2018"]


def populate(doc):
    doc.layers.add("MURI", color=1)
    doc.layers.add("QUOTE", color=3, linetype="DASHED")
    doc.layers.add("TESTI", color=5)
    msp = doc.modelspace()
    msp.add_line((0, 0), (100, 50), dxfattribs={"layer": "MURI"})
    msp.add_line((0, 0, 0), (0, 80, 10), dxfattribs={"layer": "QUOTE", "linetype": "DASHED"})
    msp.add_circle((50, 50), 25, dxfattribs={"layer": "MURI", "linetype": "CENTER"})
    msp.add_arc((0, 0), 40, 15, 120, dxfattribs={"layer": "QUOTE"})
    msp.add_text("Testo", height=2.5, dxfattribs={"layer": "TESTI", "insert": (10, 10)})
    msp.add_point((5, 5), dxfattribs={"layer": "TESTI"})
    pl = msp.add_polyline2d([(0, 0), (10, 0), (10, 10), (0, 10)], close=True,
                            dxfattribs={"layer": "MURI"})
    pl.append_vertex((5, 15))
    blk = doc.blocks.new("PORTA")
    blk.add_line((0, 0), (0, 90))
    blk.add_arc((0, 0), 90, 0, 90)
    blk.add_attdef("TAG1", (0, 100), dxfattribs={"height": 2.5, "prompt": "Nome"})
    ins = msp.add_blockref("PORTA", (200, 100), dxfattribs={"layer": "MURI"})
    ins.add_attrib("TAG1", "P01", (200, 200), dxfattribs={"height": 2.5})


def main(argv):
    here = os.path.dirname(os.path.abspath(__file__))
    out = argv[1] if len(argv) > 1 else os.path.join(here, "..", "testdata", "oracle")
    os.makedirs(out, exist_ok=True)
    for ver in VERSIONS:
        doc = ezdxf.new(ver, setup=True)
        doc.saveas(os.path.join(out, "%s-empty.dxf" % ver.lower()))
        doc = ezdxf.new(ver, setup=True)
        populate(doc)
        doc.saveas(os.path.join(out, "%s-entities.dxf" % ver.lower()))
    print("fixture scritte in", os.path.normpath(out))


if __name__ == "__main__":
    main(sys.argv)
