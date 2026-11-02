#!/usr/bin/env python3
"""Genera le varianti del file DXF R2013 minimo per il task T1 di F1.

Ogni variante parte dalla base (baseline) e toglie UN elemento. Il file
risultante si prova in ZWCAD 2023 e CadCraft: i risultati dicono quali
elementi servono davvero al file minimo che la libreria deve scrivere.

Uso: python3 generate.py <cartella di destinazione>
Non ha dipendenze. Strumento usato una volta per T1, non fa parte della libreria.
"""
import os
import sys

BASE = dict(
    classes=True,        # sezione CLASSES con le classi degli oggetti usati
    plotstyle=True,      # dizionario ACAD_PLOTSTYLENAME + segnaposto Normal (+ 390 nel layer)
    dimstyle=True,       # record DIMSTYLE Standard
    vport=True,          # record VPORT *Active
    extents=True,        # $EXTMIN, $EXTMAX, $LIMMIN, $LIMMAX
    subclass=True,       # marcatori 100 AcDb... su entita' e BLOCK/ENDBLK
    layouts=True,        # oggetti LAYOUT (Model, Layout1) e 340 nei BLOCK_RECORD
    group=True,          # dizionario ACAD_GROUP
    handseed=True,       # $HANDSEED
    codepage=True,       # $DWGCODEPAGE
    owners=True,         # 330 (owner) sulle entita' del modello
    blockrecord=True,    # tabella BLOCK_RECORD
    appid=True,          # record APPID ACAD
    paperspace=True,     # *Paper_Space, il suo BLOCK_RECORD e Layout1
    bylayer=True,        # LTYPE ByBlock e ByLayer (Continuous resta sempre)
)

# (codice file, descrizione, opzione da disattivare)
VARIANTS = [
    ("v00-baseline", "Base completa: nulla tolto", None),
    ("v01-senza-classes", "Sezione CLASSES vuota", "classes"),
    ("v02-senza-plotstylename", "Senza ACAD_PLOTSTYLENAME e segnaposto Normal", "plotstyle"),
    ("v03-senza-dimstyle", "Tabella DIMSTYLE senza il record Standard", "dimstyle"),
    ("v04-senza-vport", "Tabella VPORT vuota", "vport"),
    ("v05-senza-extents", "Senza $EXTMIN, $EXTMAX, $LIMMIN, $LIMMAX", "extents"),
    ("v06-senza-sottoclassi", "Senza marcatori 100 AcDb su entita' e blocchi", "subclass"),
    ("v07-senza-layout", "Senza oggetti LAYOUT", "layouts"),
    ("v08-senza-acad-group", "Senza dizionario ACAD_GROUP", "group"),
    ("v09-senza-handseed", "Senza $HANDSEED", "handseed"),
    ("v10-senza-codepage", "Senza $DWGCODEPAGE", "codepage"),
    ("v11-senza-owner", "Entita' senza 330 (owner)", "owners"),
    ("v12-senza-block-record", "Senza tabella BLOCK_RECORD", "blockrecord"),
    ("v13-senza-appid-acad", "Tabella APPID senza ACAD", "appid"),
    ("v14-solo-modello", "Senza *Paper_Space e senza Layout1", "paperspace"),
    ("v15-senza-bylayer-byblock", "LTYPE solo Continuous", "bylayer"),
]


class Out:
    def __init__(self):
        self.lines = []

    def g(self, code, value):
        self.lines.append(str(code).rjust(3))
        self.lines.append(str(value))

    def point(self, base, x, y, z=None):
        self.g(base, x)
        self.g(base + 10, y)
        if z is not None:
            self.g(base + 20, z)

    def text(self):
        return "\r\n".join(self.lines) + "\r\n"


def build(o):
    # --- handle -----------------------------------------------------------
    names = [
        "root", "dict_group", "dict_layout", "dict_plot", "plot_normal",
        "t_vport", "t_ltype", "t_layer", "t_style", "t_view", "t_ucs",
        "t_appid", "t_dimstyle", "t_blockrec",
        "vport_active", "lt_byblock", "lt_bylayer", "lt_cont", "layer0",
        "style_std", "appid_acad", "dimstyle_std",
        "br_model", "br_paper",
        "blk_model", "end_model", "blk_paper", "end_paper",
        "lay_model", "lay_paper",
        "e_line", "e_circle", "e_text",
    ]
    h = {n: format(i + 1, "X") for i, n in enumerate(names)}
    seed = format(len(names) + 1, "X")

    w = Out()

    def entity(kind, name, sub, layer="0", owner="br_model", paper=False):
        w.g(0, kind)
        w.g(5, h[name])
        if o["owners"] and o["blockrecord"]:
            w.g(330, h[owner])
        if o["subclass"]:
            w.g(100, "AcDbEntity")
        if paper:
            w.g(67, 1)
        w.g(8, layer)
        if o["subclass"] and sub:
            w.g(100, sub)

    # --- HEADER -----------------------------------------------------------
    w.g(0, "SECTION"); w.g(2, "HEADER")
    w.g(9, "$ACADVER"); w.g(1, "AC1027")
    if o["handseed"]:
        w.g(9, "$HANDSEED"); w.g(5, seed)
    if o["codepage"]:
        w.g(9, "$DWGCODEPAGE"); w.g(3, "ANSI_1252")
    w.g(9, "$INSBASE"); w.g(10, 0.0); w.g(20, 0.0); w.g(30, 0.0)
    if o["extents"]:
        w.g(9, "$EXTMIN"); w.g(10, 0.0); w.g(20, 0.0); w.g(30, 0.0)
        w.g(9, "$EXTMAX"); w.g(10, 100.0); w.g(20, 100.0); w.g(30, 0.0)
        w.g(9, "$LIMMIN"); w.g(10, 0.0); w.g(20, 0.0)
        w.g(9, "$LIMMAX"); w.g(10, 420.0); w.g(20, 297.0)
    w.g(9, "$LTSCALE"); w.g(40, 1.0)
    w.g(9, "$TEXTSTYLE"); w.g(7, "Standard")
    w.g(9, "$CLAYER"); w.g(8, "0")
    w.g(9, "$INSUNITS"); w.g(70, 4)
    w.g(9, "$MEASUREMENT"); w.g(70, 1)
    w.g(0, "ENDSEC")

    # --- CLASSES ----------------------------------------------------------
    w.g(0, "SECTION"); w.g(2, "CLASSES")
    if o["classes"]:
        classes = [("ACDBDICTIONARYWDFLT", "AcDbDictionaryWithDefault"),
                   ("ACDBPLACEHOLDER", "AcDbPlaceHolder"),
                   ("LAYOUT", "AcDbLayout")]
        for cpp, app in classes:
            if cpp != "LAYOUT" and not o["plotstyle"]:
                continue
            if cpp == "LAYOUT" and not o["layouts"]:
                continue
            w.g(0, "CLASS"); w.g(1, cpp); w.g(2, app); w.g(3, "ObjectDBX Classes")
            w.g(90, 0); w.g(91, 0); w.g(280, 0); w.g(281, 0)
    w.g(0, "ENDSEC")

    # --- TABLES -----------------------------------------------------------
    w.g(0, "SECTION"); w.g(2, "TABLES")

    def table(name, th, count, extra=None):
        w.g(0, "TABLE"); w.g(2, name); w.g(5, h[th]); w.g(330, 0)
        w.g(100, "AcDbSymbolTable"); w.g(70, count)
        if extra:
            extra()

    def rec_head(kind, rh, th, sub):
        w.g(0, kind); w.g(5, h[rh]); w.g(330, h[th])
        w.g(100, "AcDbSymbolTableRecord"); w.g(100, sub)

    # VPORT
    table("VPORT", "t_vport", 1 if o["vport"] else 0)
    if o["vport"]:
        rec_head("VPORT", "vport_active", "t_vport", "AcDbViewportTableRecord")
        w.g(2, "*Active"); w.g(70, 0)
        w.g(10, 0.0); w.g(20, 0.0); w.g(11, 1.0); w.g(21, 1.0)
        w.g(12, 0.0); w.g(22, 0.0); w.g(13, 0.0); w.g(23, 0.0)
        w.g(14, 0.5); w.g(24, 0.5); w.g(15, 0.5); w.g(25, 0.5)
        w.g(16, 0.0); w.g(26, 0.0); w.g(36, 1.0)
        w.g(17, 0.0); w.g(27, 0.0); w.g(37, 0.0)
        w.g(40, 300.0); w.g(41, 1.34); w.g(42, 50.0); w.g(43, 0.0); w.g(44, 0.0)
        w.g(50, 0.0); w.g(51, 0.0); w.g(71, 0); w.g(72, 1000); w.g(73, 1)
        w.g(74, 3); w.g(75, 0); w.g(76, 0); w.g(77, 0); w.g(78, 0)
        w.g(281, 0); w.g(65, 0); w.g(146, 0.0)
    w.g(0, "ENDTAB")

    # LTYPE
    n_lt = 3 if o["bylayer"] else 1
    table("LTYPE", "t_ltype", n_lt)
    def ltype(rh, name, desc):
        rec_head("LTYPE", rh, "t_ltype", "AcDbLinetypeTableRecord")
        w.g(2, name); w.g(70, 0); w.g(3, desc); w.g(72, 65); w.g(73, 0); w.g(40, 0.0)
    if o["bylayer"]:
        ltype("lt_byblock", "ByBlock", "")
        ltype("lt_bylayer", "ByLayer", "")
    ltype("lt_cont", "Continuous", "Solid line")
    w.g(0, "ENDTAB")

    # LAYER
    table("LAYER", "t_layer", 1)
    rec_head("LAYER", "layer0", "t_layer", "AcDbLayerTableRecord")
    w.g(2, "0"); w.g(70, 0); w.g(62, 7); w.g(6, "Continuous"); w.g(370, -3)
    if o["plotstyle"]:
        w.g(390, h["plot_normal"])
    w.g(0, "ENDTAB")

    # STYLE
    table("STYLE", "t_style", 1)
    rec_head("STYLE", "style_std", "t_style", "AcDbTextStyleTableRecord")
    w.g(2, "Standard"); w.g(70, 0); w.g(40, 0.0); w.g(41, 1.0); w.g(50, 0.0)
    w.g(71, 0); w.g(42, 2.5); w.g(3, "txt"); w.g(4, "")
    w.g(0, "ENDTAB")

    # VIEW, UCS (vuote)
    table("VIEW", "t_view", 0); w.g(0, "ENDTAB")
    table("UCS", "t_ucs", 0); w.g(0, "ENDTAB")

    # APPID
    table("APPID", "t_appid", 1 if o["appid"] else 0)
    if o["appid"]:
        rec_head("APPID", "appid_acad", "t_appid", "AcDbRegAppTableRecord")
        w.g(2, "ACAD"); w.g(70, 0)
    w.g(0, "ENDTAB")

    # DIMSTYLE
    def dim_extra():
        w.g(100, "AcDbDimStyleTable"); w.g(71, 1 if o["dimstyle"] else 0)
    table("DIMSTYLE", "t_dimstyle", 1 if o["dimstyle"] else 0, dim_extra)
    if o["dimstyle"]:
        w.g(0, "DIMSTYLE"); w.g(105, h["dimstyle_std"]); w.g(330, h["t_dimstyle"])
        w.g(100, "AcDbSymbolTableRecord"); w.g(100, "AcDbDimStyleTableRecord")
        w.g(2, "Standard"); w.g(70, 0); w.g(340, h["style_std"])
    w.g(0, "ENDTAB")

    # BLOCK_RECORD
    if o["blockrecord"]:
        n_br = 2 if o["paperspace"] else 1
        table("BLOCK_RECORD", "t_blockrec", n_br)
        def blockrec(rh, name, lay):
            rec_head("BLOCK_RECORD", rh, "t_blockrec", "AcDbBlockTableRecord")
            w.g(2, name)
            if o["layouts"]:
                w.g(340, h[lay])
            w.g(70, 0); w.g(280, 1); w.g(281, 0)
        blockrec("br_model", "*Model_Space", "lay_model")
        if o["paperspace"]:
            blockrec("br_paper", "*Paper_Space", "lay_paper")
        w.g(0, "ENDTAB")
    w.g(0, "ENDSEC")

    # --- BLOCKS -----------------------------------------------------------
    w.g(0, "SECTION"); w.g(2, "BLOCKS")

    def block(bn, en, br, name, paper):
        w.g(0, "BLOCK"); w.g(5, h[bn])
        if o["blockrecord"]:
            w.g(330, h[br])
        if o["subclass"]:
            w.g(100, "AcDbEntity")
        if paper:
            w.g(67, 1)
        w.g(8, "0")
        if o["subclass"]:
            w.g(100, "AcDbBlockBegin")
        w.g(2, name); w.g(70, 0); w.g(10, 0.0); w.g(20, 0.0); w.g(30, 0.0)
        w.g(3, name); w.g(1, "")
        w.g(0, "ENDBLK"); w.g(5, h[en])
        if o["blockrecord"]:
            w.g(330, h[br])
        if o["subclass"]:
            w.g(100, "AcDbEntity")
        if paper:
            w.g(67, 1)
        w.g(8, "0")
        if o["subclass"]:
            w.g(100, "AcDbBlockEnd")

    block("blk_model", "end_model", "br_model", "*Model_Space", False)
    if o["paperspace"]:
        block("blk_paper", "end_paper", "br_paper", "*Paper_Space", True)
    w.g(0, "ENDSEC")

    # --- ENTITIES ---------------------------------------------------------
    w.g(0, "SECTION"); w.g(2, "ENTITIES")
    entity("LINE", "e_line", "AcDbLine")
    w.point(10, 0.0, 0.0, 0.0); w.point(11, 100.0, 0.0, 0.0)
    entity("CIRCLE", "e_circle", "AcDbCircle")
    w.point(10, 50.0, 50.0, 0.0); w.g(40, 25.0)
    entity("TEXT", "e_text", "AcDbText")
    w.point(10, 0.0, 10.0, 0.0); w.g(40, 5.0)
    w.g(1, "Prova àèìòù ±°")
    if o["subclass"]:
        w.g(100, "AcDbText")
    w.g(0, "ENDSEC")

    # --- OBJECTS ----------------------------------------------------------
    w.g(0, "SECTION"); w.g(2, "OBJECTS")

    def dict_head(name, owner):
        w.g(0, "DICTIONARY"); w.g(5, h[name]); w.g(330, owner)
        w.g(100, "AcDbDictionary"); w.g(281, 1)

    w.g(0, "DICTIONARY"); w.g(5, h["root"]); w.g(330, 0)
    w.g(100, "AcDbDictionary"); w.g(281, 1)
    if o["group"]:
        w.g(3, "ACAD_GROUP"); w.g(350, h["dict_group"])
    w.g(3, "ACAD_LAYOUT"); w.g(350, h["dict_layout"])
    if o["plotstyle"]:
        w.g(3, "ACAD_PLOTSTYLENAME"); w.g(350, h["dict_plot"])
    if o["group"]:
        dict_head("dict_group", h["root"])
    dict_head("dict_layout", h["root"])
    if o["layouts"]:
        w.g(3, "Model"); w.g(350, h["lay_model"])
        if o["paperspace"]:
            w.g(3, "Layout1"); w.g(350, h["lay_paper"])
    if o["plotstyle"]:
        w.g(0, "ACDBDICTIONARYWDFLT"); w.g(5, h["dict_plot"]); w.g(330, h["root"])
        w.g(100, "AcDbDictionary"); w.g(281, 1)
        w.g(3, "Normal"); w.g(350, h["plot_normal"])
        w.g(100, "AcDbDictionaryWithDefault"); w.g(340, h["plot_normal"])
        w.g(0, "ACDBPLACEHOLDER"); w.g(5, h["plot_normal"]); w.g(330, h["dict_plot"])

    def layout(lh, name, order, br, flags):
        w.g(0, "LAYOUT"); w.g(5, h[lh]); w.g(330, h["dict_layout"])
        w.g(100, "AcDbPlotSettings")
        w.g(1, ""); w.g(4, "A3"); w.g(6, "")
        w.g(40, 7.5); w.g(41, 20.0); w.g(42, 7.5); w.g(43, 20.0)
        w.g(44, 420.0); w.g(45, 297.0); w.g(46, 0.0); w.g(47, 0.0)
        w.g(48, 0.0); w.g(49, 0.0); w.g(140, 0.0); w.g(141, 0.0)
        w.g(142, 1.0); w.g(143, 1.0)
        w.g(70, flags); w.g(72, 1); w.g(73, 0); w.g(74, 5); w.g(7, "")
        w.g(75, 16); w.g(76, 0); w.g(77, 2); w.g(78, 300)
        w.g(147, 1.0); w.g(148, 0.0); w.g(149, 0.0)
        w.g(100, "AcDbLayout")
        w.g(1, name); w.g(70, 1); w.g(71, order)
        w.g(10, 0.0); w.g(20, 0.0); w.g(11, 420.0); w.g(21, 297.0)
        w.g(12, 0.0); w.g(22, 0.0); w.g(32, 0.0)
        w.g(14, 1e20); w.g(24, 1e20); w.g(34, 1e20)
        w.g(15, -1e20); w.g(25, -1e20); w.g(35, -1e20)
        w.g(146, 0.0)
        w.g(13, 0.0); w.g(23, 0.0); w.g(33, 0.0)
        w.g(16, 1.0); w.g(26, 0.0); w.g(36, 0.0)
        w.g(17, 0.0); w.g(27, 1.0); w.g(37, 0.0)
        w.g(76, 1)
        if o["blockrecord"]:
            w.g(330, h[br])

    if o["layouts"]:
        layout("lay_model", "Model", 0, "br_model", 1024)
        if o["paperspace"]:
            layout("lay_paper", "Layout1", 1, "br_paper", 0)
    w.g(0, "ENDSEC")
    w.g(0, "EOF")
    return w.text()


def main():
    dest = sys.argv[1] if len(sys.argv) > 1 else "."
    os.makedirs(dest, exist_ok=True)
    rows = []
    for code, desc, flag in VARIANTS:
        opts = dict(BASE)
        if flag:
            opts[flag] = False
        path = os.path.join(dest, code + ".dxf")
        with open(path, "w", encoding="utf-8", newline="") as f:
            f.write(build(opts))
        rows.append((code, desc))
    with open(os.path.join(dest, "varianti.tsv"), "w", encoding="utf-8") as f:
        for code, desc in rows:
            f.write(f"{code}\t{desc}\n")
    print(f"{len(rows)} file scritti in {dest}")


if __name__ == "__main__":
    main()
