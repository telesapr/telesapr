#!/usr/bin/env python3
"""Generate testdata/ttf/synth/: small synthetic fonts that reach the
parts of pkg/ttf's FreeType port the system's fonts don't. For the CFF
driver: the charstring operators, seac, subroutines, too many hints,
FontMatrix scaling, CID fonts with several FontDicts, CFF2 blends, bare
CFF files, and Unicode charmaps synthesized from glyph names. For the
SFNT loader: Unicode charmaps synthesized from `post' glyph names, the
cmap formats with multi-byte encodings, and WOFF files. For the TrueType
driver: `tricky' fonts, whose bytecode runs the interpreter. For the
Type 1 driver: PFA and PFB files, and multiple masters. For the Type 42
driver: TrueType fonts in PostScript. For the PFR driver: PFR fonts,
assembled here.
TestSynthFontsVsCpp pins pkg/ttf to C++ on them.

Usage: gen-ttf-testfonts.py <output dir>   (needs fontTools)
"""
import os
import sys

from fontTools.cffLib import FDArrayIndex, FDSelect, FontDict, PrivateDict
from fontTools.fontBuilder import FontBuilder
from fontTools.misc.psCharStrings import T2CharString
from fontTools.ttLib.tables._k_e_r_n import KernTable_format_0, table__k_e_r_n

out = sys.argv[1]
os.makedirs(out, exist_ok=True)


def new_builder(upm):
    b = FontBuilder(upm, isTTF=False)
    b.font.recalcBBoxes = False
    b.font.recalcTimestamp = False
    b.updateHead(created=0, modified=0)
    return b


def enc_num(v):
    """A Type 2 charstring number."""
    if isinstance(v, float):
        f = int(round(v * 65536)) & 0xFFFFFFFF
        return bytes([255, f >> 24, (f >> 16) & 255, (f >> 8) & 255, f & 255])
    if -107 <= v <= 107:
        return bytes([v + 139])
    if 108 <= v <= 1131:
        v -= 108
        return bytes([(v >> 8) + 247, v & 255])
    if -1131 <= v <= -108:
        v = -v - 108
        return bytes([(v >> 8) + 251, v & 255])
    return bytes([28, (v >> 8) & 255, v & 255])


OPS = {
    "hstem": [1], "vstem": [3], "vmoveto": [4], "rlineto": [5], "hlineto": [6],
    "vlineto": [7], "rrcurveto": [8], "closepath": [9], "callsubr": [10],
    "return": [11], "hsbw": [13], "endchar": [14], "vsindex": [15], "blend": [16],
    "hstemhm": [18], "hintmask": [19], "cntrmask": [20], "rmoveto": [21],
    "hmoveto": [22], "vstemhm": [23], "rcurveline": [24], "rlinecurve": [25],
    "vvcurveto": [26], "hhcurveto": [27], "callgsubr": [29], "vhcurveto": [30],
    "hvcurveto": [31],
    "dotsection": [12, 0], "and": [12, 3], "or": [12, 4], "not": [12, 5],
    "abs": [12, 9], "add": [12, 10], "sub": [12, 11], "div": [12, 12],
    "neg": [12, 14], "eq": [12, 15], "drop": [12, 18], "put": [12, 20],
    "get": [12, 21], "ifelse": [12, 22], "mul": [12, 24], "sqrt": [12, 26],
    "dup": [12, 27], "exch": [12, 28], "index": [12, 29], "roll": [12, 30],
    "hflex": [12, 34], "flex": [12, 35], "hflex1": [12, 36], "flex1": [12, 37],
}


def cs(*prog):
    """Assemble a charstring: numbers, operator names, ('raw', bytes...)
    and ('mask', bytes) for hint mask bytes."""
    b = bytearray()
    for tok in prog:
        if isinstance(tok, tuple):
            b += bytes(tok[1:])
        elif isinstance(tok, str):
            b += bytes(OPS[tok])
        else:
            b += enc_num(tok)
    return bytes(b)


def charstring(code, private=None, global_subrs=None):
    c = T2CharString(bytecode=code, private=private, globalSubrs=global_subrs)
    return c


# glyphs of the name-keyed font: name -> (charstring, advance)
BOX = (100, 0, "rmoveto", 400, 700, -400, "hlineto")
glyphs = {
    ".notdef": (cs(*BOX, "endchar"), 500),
    "space": (cs("endchar"), 500),
    # width argument on hstem; lines
    "A": (cs(450, 0, 50, 600, 50, "hstem", 20, 0, "rmoveto", 250, 700, "rlineto",
             250, -700, "rlineto", -100, 0, "rlineto", -150, 450, -150, -450,
             "rlineto", "endchar"), 550),
    # every curve operator, with odd and even operand counts
    "B": (cs(40, 0, 0, "rmoveto", 0, 700, "vlineto",
             100, 0, 150, 50, 0, 100, "rrcurveto",
             50, 100, 50, -120, 20, "hvcurveto",
             0, -60, -40, -60, -80, 10, "vhcurveto",
             10, 20, 30, 40, 50, 60, -20, -30, "rcurveline",
             -10, -10, -20, -10, 10, -20, 30, -40, 50, -60, "rlinecurve",
             15, 10, -30, 20, -40, 5, -10, 10, -30, "vvcurveto",
             12, -30, -10, -20, -40, -5, 10, -20, -30, "hhcurveto",
             -30, 20, -40, -50, 60, -70, 40, -30, -50, "hvcurveto",
             "endchar"), 600),
    # flex operators
    "C": (cs(50, 300, "rmoveto",
             50, 100, 60, 0, 70, 20, 80, -20, 70, 0, 60, -100, 50, "flex",
             60, 30, 40, 10, 50, 40, 60, "hflex",
             20, -40, 30, -50, 40, -10, 30, 10, 20, 30, 10, "hflex1",
             -30, -20, -40, -30, -50, 10, -40, 20, -30, 30, -20, "flex1",
             "endchar"), 700),
    # arithmetic, storage and stack operators; 16.16 numbers
    "D": (cs(100, 20, "add", 7, 3, "sub", "mul", 10, "div", 50, "neg", "abs",
             "exch", 5, "dup", "drop", "drop", 0, "put", 0, "get",
             12.5, "rmoveto",
             300, 64, "sqrt", "mul", 0, "rlineto",
             1, 2, 3, 3, 1, "roll", "add", "add", 100, "mul", 1, "index", "drop", 0, "rlineto",
             600, 1, 0, "and", 1, 0, "or", "add", 0, "not", "add", 2, 2, "eq",
             "add", "mul", 0, "rlineto",
             -250.25, 10, 20, 30, 31, "ifelse", 1, 2, 2, 1, "ifelse", "add",
             "rlineto", "endchar"), 650),
    # hint substitution with hintmask and cntrmask; zero-length lines
    "E": (cs(0, 50, 300, 50, 300, 50, "hstemhm", 60, 80, 200, 80, "vstemhm",
             "hintmask", ("mask", 0b11110000), 60, 0, "rmoveto", 80, 700, "rlineto",
             0, 0, "rlineto",
             "hintmask", ("mask", 0b10101000), 300, 0, "rlineto", 0, -50, "rlineto",
             "cntrmask", ("mask", 0b01010000), -220, 0, "rlineto", 0, -300,
             "rlineto", "hintmask", ("mask", 0b11000000), -160, 0, "rlineto",
             "endchar"), 500),
    # local and global subroutines, nested, and endchar in a subroutine
    "F": (cs(60, 0, "rmoveto", -107 + 0, "callsubr", -107 + 0, "callgsubr",
             -107 + 2, "callsubr", -107 + 1, "callsubr"), 520),
    # moveto without a path, closing lines onto the start, several contours
    "G": (cs(10, 10, "rmoveto", 50, "hmoveto", 100, 0, "rlineto", 0, 100, "rlineto",
             -100, 0, "rlineto", 0, -100, "rlineto",
             200, 200, "rmoveto", 50, 50, 100, -50, "rlineto",
             50, "vmoveto", 30, "hmoveto", "endchar"), 480),
    # more than 96 stems fails the glyph
    "H": (cs(*([10, 5] * 24), "hstem", *([10, 5] * 24), "hstem", *([10, 5] * 24), "vstem",
             *([10, 5] * 24), "vstem", 10, 5, "vstem", *BOX, "endchar"), 500),
    # unknown, reserved and Type 1 operators
    "I": (cs(("raw", 0), ("raw", 2), 30, 40, "rmoveto", ("raw", 17),
             ("raw", 12, 0), 5, 5, ("raw", 12, 8), 100, 0, "rlineto",
             7, 13, ("raw", 12, 38), 0, 100, "rlineto", 1, 2, "hsbw",
             3, ("raw", 9), -100, "hlineto", "endchar"), 400),
    # ends without endchar
    "J": (cs(20, 20, "rmoveto", 300, 0, 0, 300, "rlineto"), 360),
    "acute": (cs(100, 600, "rmoveto", 100, 100, "rlineto", -40, 0, "rlineto",
                 -80, -100, "rlineto", "endchar"), 300),
    # a seac: accent at (150, 120)
    "Aacute": (cs(560, 150, 120, 65, 194, "endchar"), 550),
    # named for the synthesized Unicode charmap
    "uni0416": (cs(*BOX, "endchar"), 500),
    "u1F600": (cs(10, 10, "rmoveto", 480, 480, "rlineto", "endchar"), 500),
    "Delta": (cs(10, 0, "rmoveto", 200, 500, 200, -500, "rlineto", "endchar"), 420),
    "A.swash": (cs(*BOX, 50, 50, "rlineto", "endchar"), 520),
    "B.alt": (cs(0, 0, "rmoveto", 10, 10, "rlineto", "endchar"), 500),
}
local_subrs = [
    cs(0, 300, "rlineto", "return"),
    cs(200, 0, 0, -300, "rlineto", "endchar"),
    cs(-107 + 3, "callsubr", "return"),
    cs(100, 100, "rlineto", "return"),
]
global_subrs = [
    cs(100, 0, "rlineto", -107 + 3, "callsubr", "return"),
]
cmap = {
    ord("A"): "A", ord("B"): "B", ord("C"): "C", ord("D"): "D", ord("E"): "E",
    ord("F"): "F", ord("G"): "G", ord("H"): "H", ord("I"): "I", ord("J"): "J",
    0xC1: "Aacute", 0xB4: "acute", 0x416: "uni0416", 0x1F600: "u1F600",
    0x394: "Delta", ord(" "): "space",
}
order = list(glyphs)


def private_dict(**extra):
    d = {
        "BlueValues": [-12, 0, 500, 512, 700, 712],
        "OtherBlues": [-200, -190],
        "FamilyBlues": [-12, 0, 500, 510, 700, 710],
        "BlueScale": 0.0375,
        "BlueShift": 5,
        "BlueFuzz": 0,
        "StdHW": 50,
        "StdVW": 80,
        "defaultWidthX": 500,
        "nominalWidthX": 100,
    }
    d.update(extra)
    return d


def add_kern(font, pairs):
    kern = table__k_e_r_n()
    kern.version = 0
    sub = KernTable_format_0()
    sub.version = 0
    sub.coverage = 1
    sub.format = 0
    sub.kernTable = pairs
    kern.kernTables = [sub]
    font["kern"] = kern


def build(path, *, cmap_entries=True, mac_only=False, upm=1000, font_matrix=None,
          family="CFF Test", style="Regular"):
    b = new_builder(upm)
    b.setupGlyphOrder(order)
    if mac_only:
        from fontTools.ttLib.tables._c_m_a_p import cmap_format_6
        b.setupCharacterMap({})
        sub = cmap_format_6(6)
        sub.platformID, sub.platEncID, sub.language = 1, 0, 0
        sub.cmap = {c: g for c, g in cmap.items() if c < 256}
        b.font["cmap"].tables = [sub]
    else:
        b.setupCharacterMap(cmap if cmap_entries else {})
    b.setupNameTable({"familyName": family, "styleName": style})
    info = {"FullName": family + " " + style, "FamilyName": family, "Weight": style,
            "FontBBox": [-50, -210, 900, 820]}
    if font_matrix:
        info["FontMatrix"] = font_matrix
    b.setupCFF("CFFTest-" + style, info,
               {g: charstring(c) for g, (c, _) in glyphs.items()}, private_dict())
    top = b.font["CFF "].cff.topDictIndex[0]
    from fontTools.cffLib import SubrsIndex
    subrs = SubrsIndex()
    for s in local_subrs:
        subrs.append(T2CharString(bytecode=s))
    top.Private.Subrs = subrs
    gsubrs = b.font["CFF "].cff.GlobalSubrs
    for s in global_subrs:
        gsubrs.append(T2CharString(bytecode=s))
    b.setupHorizontalMetrics({g: (w, 0) for g, (_, w) in glyphs.items()})
    b.setupHorizontalHeader(ascent=800, descent=-200)
    b.setupOS2(sTypoAscender=800, sTypoDescender=-200, usWinAscent=800, usWinDescent=200)
    b.setupPost()
    b.setupMaxp()
    b.font["head"].yMax = 800
    add_kern(b.font, {("A", "B"): -120, ("B", "A"): 50, ("Aacute", "A"): -30})
    b.save(path)
    return b.font


build(os.path.join(out, "cfftest.otf"))
# a bare CFF file, whose Unicode charmap comes from the glyph names
bare = os.path.join(out, "cfftest-bare.otf")
font = build(bare, family="CFF Bare")
with open(os.path.join(out, "cfftest-bare.cff"), "wb") as f:
    f.write(font["CFF "].compile(font))
os.remove(bare)
# only a Mac charmap: the Unicode one is synthesized from the CFF charset
build(os.path.join(out, "cfftest-mac.otf"), mac_only=True, family="CFF Mac")
# a FontMatrix other than 1/1000, with a skew and an offset
build(os.path.join(out, "cfftest-matrix.otf"), upm=2048,
      font_matrix=[0.0005, 0, 0.0001, 0.0005, 0.01, 0], family="CFF Matrix")


def fd_select(gid_fds):
    sel = FDSelect()
    sel.format = 3
    sel.gidArray = gid_fds
    return sel


def finish_sfnt(b, widths, family):
    b.setupHorizontalMetrics(widths)
    b.setupHorizontalHeader(ascent=820, descent=-180)
    if "name" not in b.font:
        b.setupNameTable({"familyName": family, "styleName": "Regular"})
    b.setupOS2(sTypoAscender=820, sTypoDescender=-180, usWinAscent=820, usWinDescent=180)
    b.setupPost()
    b.setupMaxp()


# A CID-keyed font with two FontDicts: the first inherits the Top DICT's
# FontMatrix, the second has its own (a skew and an offset), which
# FreeType concatenates, and a different units per em.
cid_glyphs = [
    (cs(*BOX, "endchar"), 0),
    (cs(900, 0, 0, "rmoveto", 800, 1400, "rlineto", 800, -1400, "rlineto", "endchar"), 0),
    (cs(100, 100, "rmoveto", -107 + 0, "callsubr", 500, 0, "rlineto", "endchar"), 1),
    (cs(1200, 50, 50, "rmoveto", 300, 300, 0, 300, -300, 300, "rrcurveto", "endchar"), 1),
    (cs(50, 50, "rmoveto", 900, 0, 0, 900, "rlineto", "endchar"), 0),
]
b = new_builder(2048)
cid_order = [".notdef"] + ["cid%05d" % i for i in range(1, len(cid_glyphs))]
b.setupGlyphOrder(cid_order)
b.setupCharacterMap({ord("A"): "cid00001", ord("B"): "cid00002", ord("C"): "cid00003",
                     ord("D"): "cid00004"})
b.setupCFF("CFFTest-CID", {"FontMatrix": [0.0005, 0, 0, 0.0005, 0, 0],
                           "FontBBox": [0, -300, 2000, 1700]},
           {g: charstring(c) for g, (c, _) in zip(cid_order, cid_glyphs)}, private_dict())
top = b.font["CFF "].cff.topDictIndex[0]
top.ROS = ("Adobe", "Identity", 0)
top.CIDCount = len(cid_order)
fds = FDArrayIndex()
for i in range(2):
    fd = FontDict()
    fd.Private = PrivateDict()
    for k, v in private_dict(nominalWidthX=0 if i == 0 else 200, defaultWidthX=1000).items():
        setattr(fd.Private, k, v)
    if i == 1:
        fd.FontMatrix = [2, 0, 0.25, 2, 30, 0]
        from fontTools.cffLib import SubrsIndex
        subrs = SubrsIndex()
        subrs.append(T2CharString(bytecode=cs(0, 400, 200, 0, "rlineto", "return")))
        fd.Private.Subrs = subrs
    fds.append(fd)
top.FDArray = fds
top.FDSelect = fd_select([fd for _, fd in cid_glyphs])
top.rawDict.pop("Private", None)
if hasattr(top, "Private"):
    del top.Private
for g, (_, fd) in zip(cid_order, cid_glyphs):
    top.CharStrings[g].private = fds[fd].Private
top.CharStrings.fdArray = fds
top.CharStrings.fdSelect = top.FDSelect
finish_sfnt(b, {g: (1000, 0) for g in cid_order}, "CFF CID")
b.save(os.path.join(out, "cfftest-cid.otf"))
font = b.font
with open(os.path.join(out, "cfftest-cid.cff"), "wb") as f:
    f.write(font["CFF "].compile(font))


# A CFF2 variable font. Region 1 is degenerate (peak 0), which FreeType
# weighs 1 at the default instance of a font with an fvar table; vsindex
# 1 selects the second item variation data.
from fontTools.cffLib import VarStoreData
from fontTools.varLib.builder import buildVarData, buildVarRegionList, buildVarStore

cff2_glyphs = {
    ".notdef": (cs(*BOX), 0),
    "A": (cs(100, 10, 20, 1, "blend", 0, "rmoveto", 300, 0, 30, -40, 50, 5, 2, "blend",
             "rlineto", 0, 400, "rlineto", ("raw", 14), -150, -100, "rlineto",
             ("raw", 11), -150, 100, "rlineto"), 0),
    "B": (cs(1, "vsindex", 50, 60, 5, 1, "blend", "rmoveto", 400, 0, 0, 400, "rlineto",
             -200, 20, 1, "blend", -200, "rlineto"), 1),
    "C": (cs(0, 50, 300, 50, "hstemhm", "hintmask", ("mask", 0b11000000),
             30, 30, "rmoveto", 200, 700, 200, -700, "rlineto", -107, "callgsubr"), 0),
    "D": (cs(40, 40, "rmoveto", 10, 5, 2, 1, "blend", 300, "rlineto"), 0),
    # vsindex after a blend fails the glyph
    "E": (cs(40, 40, "rmoveto", 10, 5, 2, 1, "blend", 300, "rlineto", 1, "vsindex"), 0),
}
cff2_order = list(cff2_glyphs)
b = new_builder(1000)
b.setupGlyphOrder(cff2_order)
b.setupCharacterMap({ord(g): g for g in "ABCDE"})
b.setupNameTable({"familyName": "CFF2 Test", "styleName": "Regular"})
b.setupFvar([("wght", 100, 400, 900, "Weight")], [])
b.setupCFF2({g: charstring(c) for g, (c, _) in cff2_glyphs.items()},
            fdArrayList=[{"BlueValues": [-12, 0, 500, 512]},
                         {"BlueValues": [-10, 0, 700, 710], "BlueScale": 0.05}])
cff2 = b.font["CFF2"].cff
top = cff2.topDictIndex[0]
region_list = buildVarRegionList([{"wght": (0, 1, 1)}, {"wght": (0, 0, 0)}], ["wght"])
store = buildVarStore(region_list, [buildVarData([0, 1], None, optimize=False),
                                    buildVarData([1], None, optimize=False)])
vstore = VarStoreData(otVarStore=store)
top.VarStore = vstore
for fd in top.FDArray:
    fd.Private.vstore = vstore
top.FDSelect = fd_select([fd for _, fd in cff2_glyphs.values()])
for g, (_, fd) in cff2_glyphs.items():
    top.CharStrings[g].private = top.FDArray[fd].Private
top.CharStrings.fdSelect = top.FDSelect
gsubrs = cff2.GlobalSubrs
gsubrs.append(T2CharString(bytecode=cs(-100, 0, "rlineto", 5, 10, 3, 1, "blend", 50, "rlineto")))
finish_sfnt(b, {g: (600, 0) for g in cff2_order}, "CFF2 Test")
b.save(os.path.join(out, "cff2test.otf"))
# the same without its fvar table, so there's no variation blend
del b.font["fvar"]
for rec in b.font["name"].names:
    if rec.nameID in (1, 16):
        rec.string = "CFF2 NoFvar"
b.save(os.path.join(out, "cff2test-nofvar.otf"))


# TrueType fonts for the SFNT loader.
import struct

from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.ttLib import newTable
from fontTools.ttLib.standardGlyphOrder import standardGlyphOrder
from fontTools.ttLib.tables._c_m_a_p import CmapSubtable, cmap_format_unknown


def tt_glyph(k):
    """A small TrueType glyph that depends on k: a box and a triangle,
    with a quadratic curve."""
    pen = TTGlyphPen(None)
    pen.moveTo((50, 0))
    pen.lineTo((50, 600 + 7 * (k % 20)))
    pen.qCurveTo((250, 700 + 3 * (k % 11)), (450, 600))
    pen.lineTo((450, 0))
    pen.closePath()
    pen.moveTo((150 + 5 * (k % 13), 100))
    pen.lineTo((250, 300 + 11 * (k % 7)))
    pen.lineTo((350, 100))
    pen.closePath()
    return pen.glyph()


def tt_font(order, family, subtables, post_format=2.0, kern=None):
    b = FontBuilder(1000, isTTF=True)
    b.font.recalcTimestamp = False
    b.updateHead(created=0, modified=0)
    b.setupGlyphOrder(order)
    b.setupCharacterMap({})
    b.font["cmap"].tables = subtables
    b.setupGlyf({g: tt_glyph(i) for i, g in enumerate(order)})
    b.setupHorizontalMetrics({g: (500 + 10 * (i % 9), 50) for i, g in enumerate(order)})
    b.setupHorizontalHeader(ascent=800, descent=-200)
    b.setupNameTable({"familyName": family, "styleName": "Regular"})
    b.setupOS2(sTypoAscender=800, sTypoDescender=-200, usWinAscent=800, usWinDescent=200)
    b.setupPost(keepGlyphNames=post_format != 3.0)
    b.font["post"].formatType = post_format
    if kern:
        add_kern(b.font, kern)
    return b.font


def subtable(fmt, platform, encoding, mapping):
    t = CmapSubtable.newSubtable(fmt)
    t.platformID, t.platEncID, t.language = platform, encoding, 0
    t.cmap = mapping
    return t


def raw_subtable(platform, encoding, data):
    t = cmap_format_unknown(struct.unpack(">H", data[:2])[0])
    t.platformID, t.platEncID, t.language = platform, encoding, 0
    t.data = data
    t.decompileHeader = lambda *a: None
    return t


tt_order = [".notdef", "A", "B", "C", "space", "uni0416", "u1F600", "Delta",
            "A.swash", "B.alt", "fraction", "periodcentered"]
# only a Mac charmap: Unicode comes from the post table's glyph names
tt_font(tt_order, "TT PostMac", [subtable(6, 1, 0, {0x41: "A", 0x42: "B"})],
        kern={("A", "B"): -100}).save(os.path.join(out, "ttpost-mac.ttf"))
# a symbol charmap only: no Unicode charmap, so C++ doesn't list the font
tt_font(tt_order, "TT Symbol", [subtable(4, 3, 0, {0xF041: "A", 0xF042: "B"})]
        ).save(os.path.join(out, "ttsymbol.ttf"))
# post format 1: the 258 standard Macintosh names
font = tt_font(standardGlyphOrder, "TT PostOne", [subtable(6, 1, 0, {0x41: "A"})],
               post_format=1.0)
font.save(os.path.join(out, "ttpost1.ttf"))

gid = {g: i for i, g in enumerate(tt_order)}
# format 10 (UCS-4) wins over format 4; its mapping differs
fmt10 = struct.pack(">HHIIII", 10, 0, 20 + 2 * 3, 0, 0x41, 3) + struct.pack(
    ">HHH", gid["B"], gid["C"], gid["A"])
tt_font(tt_order, "TT Cmap10",
        [subtable(4, 3, 1, {0x41: "A", 0x42: "B", 0x43: "C"}), raw_subtable(3, 10, fmt10)]
        ).save(os.path.join(out, "ttcmap10.ttf"))
# format 8 under Unicode 2.0+ full repertoire (0, 4); the is32 array is
# all zeros for 16-bit codes
groups = [(0x41, 0x42, gid["C"]), (0x416, 0x416, gid["Delta"]),
          (0x1F600, 0x1F600, gid["u1F600"])]
fmt8 = struct.pack(">HHII", 8, 0, 16 + 8192 + 12 * len(groups), 0) + bytes(8192) + \
    struct.pack(">I", len(groups)) + b"".join(struct.pack(">III", *g) for g in groups)
tt_font(tt_order, "TT Cmap8", [raw_subtable(0, 4, fmt8)]).save(os.path.join(out, "ttcmap8.ttf"))
# format 2 under Microsoft Unicode: one-byte codes and a high byte 0x04
keys = [0] * 256
keys[4] = 8
fmt2_subs = struct.pack(">HHhH", 0x41, 3, 0, 2 * 8 + 2 - 6) + \
    struct.pack(">HHhH", 0x16, 1, 0, 2 + 6 - 6 + 2)
glyph_ids = struct.pack(">HHH", gid["C"], gid["A"], gid["B"]) + struct.pack(">H", gid["Delta"])
body = struct.pack(">256H", *keys) + fmt2_subs + glyph_ids
fmt2 = struct.pack(">HHH", 2, 6 + len(body), 0) + body
tt_font(tt_order, "TT Cmap2", [raw_subtable(3, 1, fmt2)]).save(os.path.join(out, "ttcmap2.ttf"))

# WOFF and WOFF2 files, with WOFF2's glyf, loca and hmtx transforms
from fontTools.ttLib import TTFont

base = os.path.join(out, "ttpost-mac.ttf")
for flavor, name, family in [("woff", "ttwoff.woff", "TT Woff"),
                             ("woff2", "ttwoff2.woff2", "TT Woff2")]:
    f = TTFont(base, recalcTimestamp=False)
    for rec in f["name"].names:
        if rec.nameID in (1, 16):
            rec.string = family
    f.flavor = flavor
    if flavor == "woff2":
        f.flavorData = __import__("fontTools.ttLib.woff2", fromlist=["WOFF2FlavorData"]
                                  ).WOFF2FlavorData(transformedTables={"glyf", "loca", "hmtx"})
    f.save(os.path.join(out, name))
f = TTFont(os.path.join(out, "cfftest.otf"), recalcTimestamp=False)
for rec in f["name"].names:
    if rec.nameID in (1, 16):
        rec.string = "CFF Woff2"
f.flavor = "woff2"
f.save(os.path.join(out, "cffwoff2.woff2"))

# `Tricky' TrueType fonts (named like the CJK fonts FreeType hints even
# with FT_LOAD_NO_HINTING, or with their bytecode's checksums), whose
# bytecode exercises the interpreter.
from fontTools.ttLib.tables import ttProgram
from fontTools.ttLib.tables._g_l_y_f import GlyphComponent

OPS = {
    "SVTCA_Y": 0x00, "SVTCA_X": 0x01, "SPVTCA_Y": 0x02, "SPVTCA_X": 0x03,
    "SFVTCA_Y": 0x04, "SFVTCA_X": 0x05, "SPVTL0": 0x06, "SPVTL1": 0x07,
    "SFVTL0": 0x08, "SFVTL1": 0x09, "SPVFS": 0x0A, "SFVFS": 0x0B, "GPV": 0x0C,
    "GFV": 0x0D, "SFVTPV": 0x0E, "ISECT": 0x0F, "SRP0": 0x10, "SRP1": 0x11,
    "SRP2": 0x12, "SZP0": 0x13, "SZP1": 0x14, "SZP2": 0x15, "SZPS": 0x16,
    "SLOOP": 0x17, "RTG": 0x18, "RTHG": 0x19, "SMD": 0x1A, "ELSE": 0x1B,
    "JMPR": 0x1C, "SCVTCI": 0x1D, "SSWCI": 0x1E, "SSW": 0x1F, "DUP": 0x20,
    "POP": 0x21, "CLEAR": 0x22, "SWAP": 0x23, "DEPTH": 0x24, "CINDEX": 0x25,
    "MINDEX": 0x26, "ALIGNPTS": 0x27, "UTP": 0x29, "LOOPCALL": 0x2A,
    "CALL": 0x2B, "FDEF": 0x2C, "ENDF": 0x2D, "MDAP0": 0x2E, "MDAP1": 0x2F,
    "IUP_Y": 0x30, "IUP_X": 0x31, "SHP0": 0x32, "SHP1": 0x33, "SHC0": 0x34,
    "SHC1": 0x35, "SHZ0": 0x36, "SHZ1": 0x37, "SHPIX": 0x38, "IP": 0x39,
    "MSIRP0": 0x3A, "MSIRP1": 0x3B, "ALIGNRP": 0x3C, "RTDG": 0x3D,
    "MIAP0": 0x3E, "MIAP1": 0x3F, "WS": 0x42, "RS": 0x43, "WCVTP": 0x44,
    "RCVT": 0x45, "GC0": 0x46, "GC1": 0x47, "SCFS": 0x48, "MD0": 0x49,
    "MD1": 0x4A, "MPPEM": 0x4B, "MPS": 0x4C, "FLIPON": 0x4D, "FLIPOFF": 0x4E,
    "LT": 0x50, "LTEQ": 0x51, "GT": 0x52, "GTEQ": 0x53, "EQ": 0x54,
    "NEQ": 0x55, "ODD": 0x56, "EVEN": 0x57, "IF": 0x58, "EIF": 0x59,
    "AND": 0x5A, "OR": 0x5B, "NOT": 0x5C, "DELTAP1": 0x5D, "SDB": 0x5E,
    "SDS": 0x5F, "ADD": 0x60, "SUB": 0x61, "DIV": 0x62, "MUL": 0x63,
    "ABS": 0x64, "NEG": 0x65, "FLOOR": 0x66, "CEILING": 0x67, "ROUND0": 0x68,
    "ROUND1": 0x69, "NROUND0": 0x6C, "WCVTF": 0x70, "DELTAP2": 0x71,
    "DELTAP3": 0x72, "DELTAC1": 0x73, "DELTAC2": 0x74, "DELTAC3": 0x75,
    "SROUND": 0x76, "S45ROUND": 0x77, "JROT": 0x78, "JROF": 0x79,
    "ROFF": 0x7A, "RUTG": 0x7C, "RDTG": 0x7D, "FLIPPT": 0x80,
    "FLIPRGON": 0x81, "FLIPRGOFF": 0x82, "SCANCTRL": 0x85, "SDPVTL0": 0x86,
    "SDPVTL1": 0x87, "GETINFO": 0x88, "IDEF": 0x89, "ROLL": 0x8A, "MAX": 0x8B,
    "MIN": 0x8C, "SCANTYPE": 0x8D, "INSTCTRL": 0x8E,
}


def asm(*items):
    """Bytecode from opcode names, raw bytes (opcodes), and lists of values
    to push (the last one ends up on top)."""
    out = bytearray()
    for it in items:
        if isinstance(it, str):
            out.append(OPS[it])
        elif isinstance(it, int):
            out.append(it)
        elif isinstance(it, bytes):
            out += it
        else:
            vals = list(it)
            if all(0 <= v <= 255 for v in vals):
                out += bytes([0xB0 + len(vals) - 1]) if len(vals) <= 8 else bytes([0x40, len(vals)])
                out += bytes(vals)
            else:
                out += bytes([0xB8 + len(vals) - 1]) if len(vals) <= 8 else bytes([0x41, len(vals)])
                out += b"".join(struct.pack(">h", v) for v in vals)
    return bytes(out)


def MDRP(rp0=0, mind=0, rnd=0, dist=0):
    return 0xC0 | rp0 << 4 | mind << 3 | rnd << 2 | dist


def MIRP(rp0=0, mind=0, rnd=0, dist=0):
    return 0xE0 | rp0 << 4 | mind << 3 | rnd << 2 | dist


def skip_if(op, cond, seg):
    """cond pushed, then JROT/JROF over seg."""
    return asm(cond, [1 + len(seg)], "SWAP", op) + seg


def program(code):
    p = ttProgram.Program()
    p.fromBytecode(code)
    return p


# fpgm: 0 rounds a point; 1 anchors a point to a CVT value; 2 counts in
# storage 1; 3 picks the rounding by ppem; 4 links a point to rp0 by CVT
# 5; 9 stores the point size. IDEF 0x93 writes storage 5.
FPGM = asm(
    [0], "FDEF", "MDAP1", "ENDF",
    [1], "FDEF", "MIAP1", "ENDF",
    [2], "FDEF", [1], "RS", [1], "ADD", [1], "SWAP", "WS", "ENDF",
    [3], "FDEF", "MPPEM", [1000], "GT", "IF", "RTG", "ELSE", "RDTG", "EIF", "ENDF",
    [4], "FDEF", [5], MIRP(0, 1, 1, 0), "ENDF",
    [0x93], "IDEF", [5, 77], "WS", "ENDF",
    [9], "FDEF", "MPS", [3], "SWAP", "WS", "ENDF",
)

# prep: graphics state, CVT, storage and twilight set-up for the glyphs
PREP = asm(
    [68], "SCVTCI", [5000], "SSWCI", [121], "SSW", [48], "SMD",
    [2, 1100], "WCVTF", [6], "RCVT", [7], "SWAP", "WCVTP",
    [3, 4], "INSTCTRL",
    [0x1FF], "SCANCTRL", [4], "SCANTYPE",
    [0], "SZP0", "SVTCA_Y", [1, 1], "MIAP1", "SVTCA_X", [2, 3], "MIAP0", [1], "SZP0",
    [3, 2], "LOOPCALL", [1], "RS", [8], "SWAP", "WS",
    [3], "CALL", [9], "CALL",
    [1], "GETINFO", [4], "SWAP", "WS", [0x3FFF], "GETINFO", [6], "SWAP", "WS",
    "MPPEM", [10], "SWAP", "WS",
    "MPPEM", [12], "SUB", "SDB", [0xC5, 8, 1], "DELTAC1",
)

CVT = [0, 600, 50, 700, 120, -30, 300, 80, 640, 410]

# the storage counter loop of glyph E: storage 3 counts down from 3
_body = asm([3], "RS", [1], "SUB", "DUP", [3], "SWAP", "WS")
_loop = asm([3, 3], "WS") + _body + asm([-(len(_body) + 4)], "SWAP", "JROT")

# glyph programs over the points of tt_glyph (0-4, then 5-7), with the
# phantom points 8-11
GLYPH_PROGS = {
    "A": asm(
        "SVTCA_X", [0, 0], "CALL", [4, 3], MIRP(1, 1, 1, 1),
        [1], "SRP1", [4], "SRP2", [1, 2, 3], [3], "SLOOP", "IP",
        [5], MDRP(0, 1, 1, 0), [6, 7], [2], "SLOOP", "SHP1",
        "IUP_X",
        "SVTCA_Y", [0, 2], "MIAP0", [1], MDRP(1, 1, 1, 2),
        [2], "SLOOP", [6, 7, 40], "SHPIX",
        [1], "SRP0", [3], "ALIGNRP", [0], "SRP0", [4], MDRP(0, 0, 1, 0),
        "IUP_Y",
    ),
    # projection and freedom vectors off the axes; super rounding
    "B": asm(
        [0, 3], "SPVTL1", [1, 3], "SFVTL0", [2], "MDAP1",
        [0, 2], "SDPVTL1", [5], MDRP(0, 1, 1, 1),
        [0x2D41, 0x2D41], "SPVFS", "SFVTPV", [6, 64], "MSIRP1",
        [0x1000, 0x3000], "SFVFS", [7], "MDAP1", [2], "SRP0", [4, 300], "MSIRP0",
        [0x2D41, -0x2D41], "SFVFS", [3], "MDAP1",
        "GPV", "ADD", [1], "SWAP", "WS", "GFV", "SUB", [2], "SWAP", "WS",
        "SVTCA_Y", [5, 6, 7], [3], "SLOOP", [0], "SRP0", "ALIGNRP",
        [0x58], "SROUND", [1], MDRP(0, 0, 1, 0),
        [0x4C], "S45ROUND", [2], MDRP(0, 0, 1, 0),
        "RUTG", [3], MDRP(0, 0, 1, 0), "RTHG", [4], MDRP(0, 0, 1, 0),
        "IUP_Y", "IUP_X",
    ),
    # contours, zones and the twilight points
    "C": asm(
        "SVTCA_X", [0], "MDAP1", [0], "SRP1", [1], "SHC1", [4], "SRP2", [0], "SHC0",
        [0], "SZP0", [2], "SRP0", [6], MDRP(0, 1, 1, 0), [1], "SZP0",
        "SVTCA_Y", [2], "MDAP1", [2], "SRP1", [1], "SHZ1",
        [0], "SZP2", [1], "SHZ1", [1], "SZP2",
        [0], "SZP0", [2], "SRP0", [5, 70], "MSIRP0", [1], "SZP0",
        [0], "SZPS", [3, 1], MIRP(0, 0, 1, 0), [1], "SZPS",
        [7, 5, 6, 0, 4], "ISECT",
        [3, 4], "ALIGNPTS",
        [2], "MDAP0", [2], "UTP",
        "IUP_Y",
    ),
    # measurements, flips and deltas
    "D": asm(
        "SVTCA_X", [3], "GC0", [1], "GC1", "SUB", [7], "SWAP", "SCFS",
        [0, 4], "MD0", [0, 4], "MD1", "MAX", [2], "SWAP", "SCFS",
        [5], "FLIPPT", [6, 7], "FLIPRGOFF", [6, 6], "FLIPRGON",
        "MPPEM", [12], "SUB", "SDB", [1], "SDS",
        [0xCF, 0, 0xC3, 1, 2], "DELTAP1",
        [0xC9, 3, 1], "DELTAC1", [4, 3], MIRP(0, 0, 0, 0), [7, 4], MIRP(0, 0, 0, 0),
        "FLIPOFF", [5, 5], MIRP(0, 0, 0, 0), "FLIPON", [6, 5], MIRP(0, 0, 0, 0),
        "MPPEM", [28], "SUB", "SDB", [0xC1, 5, 1], "DELTAP2", [0xC6, 9, 1], "DELTAC2",
        [1], "SRP0", [2, 9], MIRP(0, 0, 0, 0), [0], "SRP0", [1, 8], MIRP(0, 0, 0, 0),
        "SVTCA_Y", [0xC7, 4, 1], "DELTAP3", [0xC7, 3, 1], "DELTAP2", [9, 5], "WS",
        [4], "MDAP1", "IUP_Y",
    ),
    # arithmetic, the stack and flow control
    "E": asm(
        [3, 5, 7], [2], "CINDEX", [3], "MINDEX", "ROLL", "DEPTH",
        "ADD", "MUL", "DUP", "NEG", "ABS", "MAX", [100], "MIN",
        [200], "DIV", "FLOOR", [33], "CEILING", "ADD", "ROUND1", "NROUND0",
        "ODD", "NOT", [1], "AND", [0], "OR", "IF", "SVTCA_X", "ELSE", "SVTCA_Y", "EIF",
        [1], "EVEN", "IF", [2], "MDAP1", "EIF",
        skip_if("JROF", [0], asm([3], "MDAP1")),
        skip_if("JROT", [0], asm([4], "MDAP1")),
        _loop,
        [3, 4, 5], "LT", "GTEQ", [1, 1], "EQ", "NEQ", [5], "LTEQ", "CLEAR",
        0x93, [5], "RS", "POP",
        "RTDG", [3], "MDAP1", "ROFF", [4], "MDAP1", "RDTG", [1], "MDAP1",
        [6, 4], "CALL", [3, 2], "LOOPCALL", [9], "CALL",
        [1], "RS", [0x40], "MUL", [7], "SWAP", "SCFS",
        "IUP_X",
    ),
    # points placed from the vertical phantom points; an invalid opcode
    # stops the program, and its effects so far stay
    "F": asm("SVTCA_Y", [10], "SRP0", [2], MDRP(0, 0, 0, 0),
             "SVTCA_X", [11], "SRP0", [3], MDRP(0, 0, 0, 0),
             [1], "MDAP1", [3], "MDAP1", 0x92, [4], "MDAP1"),
    # a stack underflow reads zeros; FDEF in glyph bytecode fails
    "G": asm("SVTCA_Y", "MDAP1", "IUP_Y", [5], "FDEF", "ENDF"),
    # storage values from prep (glyph programs write copies), and a loop
    # that moves a point until the negative jump counter trips
    "I": asm("SVTCA_X", [1], "MDAP1",
             [2], [4], "RS", "SHPIX", [3], [6], "RS", [0x4000], "DIV", "SHPIX",
             [4], [5], "RS", "SHPIX", [5], [3], "RS", "SHPIX", [6], [9], "RS", "SHPIX",
             [7], [10], "RS", "SHPIX", [0], [8], "RS", "SHPIX",
             [3], [2, 3], "DIV", "SHPIX",
             "SVTCA_Y", [5], [1], "SHPIX", [-8], "JMPR"),
}

TRICKY_ORDER = [".notdef", "A", "B", "C", "D", "E", "F", "G", "I", "J", "space", "H", "Aacute"]


def tricky_font(family, *, upm=1000, ascent=800, descent=-200, hdmx=False,
                prep=PREP, fpgm=FPGM, head_flags=0x000B, cvt=CVT, vertical=False):
    order = TRICKY_ORDER
    b = FontBuilder(upm, isTTF=True)
    b.font.recalcTimestamp = False
    b.updateHead(created=0, modified=0, flags=head_flags)
    b.setupGlyphOrder(order)
    cmap = {ord(c): c for c in "ABCDEFGIJH"}
    cmap[0x20] = "space"
    cmap[0xC1] = "Aacute"
    b.setupCharacterMap(cmap)
    glyphs = {}
    for i, g in enumerate(order):
        glyphs[g] = TTGlyphPen(None).glyph() if g == "space" else tt_glyph(i * 3)
    # J: a composite of A and B, rounded to the grid, with instructions
    pen = TTGlyphPen(glyphs)
    pen.addComponent("A", (1, 0, 0, 1, 13, 7))
    pen.addComponent("B", (0.75, 0, 0, 1.25, 517, -21))
    glyphs["J"] = pen.glyph()
    pen = TTGlyphPen(glyphs)
    pen.addComponent("A", (1, 0, 0, 1, 0, 0))
    pen.addComponent("C", (1, 0, 0, 1, 0, 0))
    glyphs["Aacute"] = pen.glyph()
    b.setupGlyf(glyphs)
    glyf = b.font["glyf"]
    for g, code in GLYPH_PROGS.items():
        glyf[g].program = program(code)
    j = glyf["J"]
    j.components[0].flags |= 0x0004 | 0x0200  # ROUND_XY_TO_GRID, USE_MY_METRICS
    j.components[1].flags |= 0x0004
    j.program = program(asm("SVTCA_X", [0], "MDAP1", [9], MDRP(0, 1, 1, 0),
                            [0], "SRP1", [9], "SRP2", [3], "IP", "IUP_X",
                            "SVTCA_Y", [2], "MDAP1", [12, 64], "SHPIX", "IUP_Y"))
    # Aacute: a scaled component matched to a point of the first
    c2 = GlyphComponent()
    c2.glyphName, c2.firstPt, c2.secondPt, c2.flags = "C", 2, 0, 0
    c2.transform = [[0.5, 0.25], [0, 0.5]]
    glyf["Aacute"].components[1] = c2
    glyf["Aacute"].program = program(asm("SVTCA_Y", [9], "MDAP1", "IUP_Y"))

    b.setupHorizontalMetrics({g: (500 + 37 * (i % 7), 50 - 9 * (i % 4))
                              for i, g in enumerate(order)})
    b.setupHorizontalHeader(ascent=ascent, descent=descent)
    b.setupNameTable({"familyName": family, "styleName": "Regular"})
    b.setupOS2(sTypoAscender=ascent, sTypoDescender=descent, usWinAscent=ascent,
               usWinDescent=-descent)
    if vertical:
        b.setupVerticalHeader(ascent=500, descent=-500)
        b.setupVerticalMetrics({g: (900 + 11 * i, 70 + 13 * (i % 5))
                                for i, g in enumerate(order)})
    b.setupPost()
    font = b.font
    if cvt is not None:
        font["cvt "] = newTable("cvt ")
        font["cvt "].values = __import__("array").array("h", cvt)
    if fpgm is not None:
        font["fpgm"] = newTable("fpgm")
        font["fpgm"].program = program(fpgm)
    if prep is not None:
        font["prep"] = newTable("prep")
        font["prep"].program = program(prep)
    maxp = font["maxp"]
    maxp.maxZones, maxp.maxTwilightPoints, maxp.maxStorage = 2, 6, 12
    maxp.maxFunctionDefs, maxp.maxInstructionDefs, maxp.maxStackElements = 12, 2, 40
    if hdmx:
        # a record for the size C++ asks for (FT_Request_Size with
        # height 1<<16 at 128 dpi), whose ppem this is
        h = ascent - descent
        y_scale = ((116508 << 16) + h // 2) // h
        ppem = (upm * y_scale + 0x8000 >> 16) + 32 >> 6
        font["hdmx"] = newTable("hdmx")
        font["hdmx"].hdmx = {p: {g: 60 + 7 * i + p % 5 for i, g in enumerate(order)}
                             for p in (ppem - 1, ppem, ppem + 3)}
    return font


tricky_font("DFKaiShu Test").save(os.path.join(out, "tricky.ttf"))
# ppem 227 with an hdmx record for it: the advances come from hdmx
tricky_font("MingLiU Hdmx", ascent=6000, descent=-2000, hdmx=True).save(
    os.path.join(out, "tricky-hdmx.ttf"))
# no integer ppem scaling (head flags bit 3 clear), and a PDF subset tag
tricky_font("ABCDEF+DFKai-SB", head_flags=0x0003).save(os.path.join(out, "tricky-noround.ttf"))
# prep turns hinting off (the scales are still the hinted ones)
tricky_font("DFMing NoHint", prep=PREP + asm([1, 1], "INSTCTRL")).save(
    os.path.join(out, "tricky-nohint.ttf"))
# prep asks for the default graphics state in glyph programs; vertical
# metrics place the vertical phantom points
tricky_font("DLC GSReset", prep=PREP + asm([2, 2], "INSTCTRL"), vertical=True).save(
    os.path.join(out, "tricky-gsreset.ttf"))
# fpgm and prep errors: no glyph loads
tricky_font("DFHei BadFpgm", fpgm=FPGM + asm(0x92)).save(os.path.join(out, "tricky-badfpgm.ttf"))
tricky_font("DFGothic-EB BadPrep", prep=PREP + asm("ENDF")).save(
    os.path.join(out, "tricky-badprep.ttf"))


def fill(code, length, checksum):
    """code, then NPUSHB and CLEAR runs up to length bytes, with a word in
    the last run's data that gives the table's checksum."""
    out = bytearray(code)
    while length - len(out) > 110:
        out += bytes([0x40, 100]) + bytes(100) + bytes([OPS["CLEAR"]])
    rest = length - len(out)
    assert rest >= 12
    out += bytes([0x40, rest - 3]) + bytes(rest - 3) + bytes([OPS["CLEAR"]])
    pos = (len(out) - 1 - 4) & ~3  # an aligned word in the pushed data
    assert pos >= length - rest + 2
    padded = bytes(out) + bytes(-len(out) % 4)
    total = sum(struct.unpack(">%dI" % (len(padded) // 4), padded)) & 0xFFFFFFFF
    word = (checksum - total) & 0xFFFFFFFF
    out[pos:pos + 4] = struct.pack(">I", word)
    return bytes(out)


# tricky by the checksums of its fpgm and prep (NEC fadpop7.ttf's; no cvt)
tricky_font("Checksum Trick", cvt=None, fpgm=fill(FPGM, 0xE5, 0x40C92555),
            prep=fill(PREP, 0x117C, 0xA39B58E3)).save(os.path.join(out, "tricky-sum.ttf"))


# Type 1 fonts (PFA and PFB), written out here: the charstrings exercise
# the cf2 engine's Type 1 mode (flex and hint replacement through the
# standard Subrs, seac, sbw, div, othersubr results, multiple masters).

T1_OPS = {
    "hstem": [1], "vstem": [3], "vmoveto": [4], "rlineto": [5], "hlineto": [6],
    "vlineto": [7], "rrcurveto": [8], "closepath": [9], "callsubr": [10],
    "return": [11], "hsbw": [13], "endchar": [14], "rmoveto": [21],
    "hmoveto": [22], "vhcurveto": [30], "hvcurveto": [31],
    "dotsection": [12, 0], "vstem3": [12, 1], "hstem3": [12, 2], "seac": [12, 6],
    "sbw": [12, 7], "div": [12, 12], "callothersubr": [12, 16], "pop": [12, 17],
    "setcurrentpoint": [12, 33],
}


def t1_cs(*items):
    """A Type 1 charstring from ints and operator names."""
    out = bytearray()
    for it in items:
        if isinstance(it, str):
            out += bytes(T1_OPS[it])
        elif -107 <= it <= 107:
            out.append(it + 139)
        elif 108 <= it <= 1131:
            v = it - 108
            out += bytes([247 + (v >> 8), v & 0xFF])
        elif -1131 <= it <= -108:
            v = -it - 108
            out += bytes([251 + (v >> 8), v & 0xFF])
        else:
            out.append(255)
            out += struct.pack(">i", it)
    return bytes(out)


def t1_encrypt(data, key, prefix=b""):
    r = key
    out = bytearray()
    for b in prefix + data:
        c = b ^ (r >> 8)
        out.append(c)
        r = ((c + r) * 52845 + 22719) & 0xFFFF
    return bytes(out)


# the standard flex and hint replacement Subrs
T1_STD_SUBRS = [
    t1_cs(3, 0, "callothersubr", "pop", "pop", "setcurrentpoint", "return"),
    t1_cs(0, 1, "callothersubr", "return"),
    t1_cs(0, 2, "callothersubr", "return"),
    t1_cs("return"),
    t1_cs(1, 3, "callothersubr", "pop", "callsubr", "return"),
]


def t1_glyph_a(k):
    """A box with a notch, a curved contour, and stems."""
    return t1_cs(40 + k, 600 + 10 * k, "hsbw",
                 0, 30, "hstem", 700, 25, "hstem", 60, 40, "vstem",
                 60, 0, "rmoveto", 400, "hlineto", 120 + k, 300, "rlineto",
                 -40, 400, -100, 20, -80, 0, "rrcurveto",
                 -200, "hlineto", -80, -200, "rlineto", "closepath",
                 150, 150, "rmoveto", 100, 0, 50, 100, "vhcurveto",
                 -60, 30, -50, -80, "hvcurveto", "closepath", "endchar")


def t1_glyph_flex():
    """Flex (Subrs 1, 2 and 0) between two lines."""
    pts = [(100, 0), (50, 20), (50, 30), (50, 0), (50, -30), (50, -20), (50, 0)]
    cs = [50, 700, "hsbw", 50, 100, "rmoveto", 100, "hlineto", 1, "callsubr"]
    for dx, dy in pts:
        cs += [dx, dy, "rmoveto", 2, "callsubr"]
    cs += [50, 550, 100, 0, "callsubr", 0, 300, "rlineto", -500, "hlineto", "closepath",
           "endchar"]
    return t1_cs(*cs)


def t1_glyph_hints():
    """Hint replacement through Subr 4, and the three-stem hints."""
    return t1_cs(30, 620, "hsbw", 0, 20, "hstem", 5, 4, "callsubr",
                 40, 0, "rmoveto", 500, 0, "rlineto", 0, 650, "rlineto", "dotsection",
                 6, 4, "callsubr", -500, 0, "rlineto", "dotsection", "closepath",
                 0, 30, 300, 30, 600, 30, "hstem3", 0, 40, 200, 40, 400, 40, "vstem3",
                 100, 100, "rmoveto", 50, 50, "rlineto", -50, 50, "rlineto", "closepath",
                 "endchar")


def t1_glyph_sbw():
    """sbw, div of large integers, othersubr results and arithmetic."""
    return t1_cs(25, 5, 580, 10, "sbw",
                 60000, 100, "div", 0, "rmoveto",
                 # an unknown othersubr leaves its two arguments to pop
                 30, 40, 2, 99, "callothersubr", "pop", "pop", "rlineto",
                 # othersubrs 20 (add), 22 (mul) and 27 (ifelse)
                 150, 50, 2, 20, "callothersubr", "pop", 0, "rlineto",
                 3, 70, 2, 22, "callothersubr", "pop", 0, "rlineto",
                 11, 22, 1, 2, 4, 27, "callothersubr", "pop", 300, "rlineto",
                 -700, 7, "div", -100, "rlineto", "closepath", "endchar")


def t1_glyph_seac():
    """Aacute from A and acute (standard codes 65 and 194), after sbw."""
    return t1_cs(40, 15, 660, 0, "sbw", 40, 180, 230, 65, 194, "seac")


def t1_glyph_acute():
    # an advance of 300.5, from div
    return t1_cs(60, 601, 2, "div", "hsbw", 60, 500, "rmoveto", 80, 120, "rlineto", 40, -30,
                 "rlineto", "closepath", "endchar")


def t1_glyph_subr(n):
    """Calls a Subr of the sparse array by its index."""
    return t1_cs(50, 600, "hsbw", 50, 50, "rmoveto", n, "callsubr", "closepath", "endchar")


def t1_glyph_mm():
    """Multiple-master blends (othersubrs 14 and 15)."""
    return t1_cs(40, 640, "hsbw", 100, 0,
                 300, 100, 2, 14, "callothersubr", "pop", "rmoveto",
                 400, 300, 90, -50, 4, 15, "callothersubr", "pop", "pop", "rlineto",
                 -200, 250, "rlineto", "closepath", "endchar")


def t1_font(path, *, family, full=None, weight=None, font_name="T1Test", pfb=False,
            hex_eexec=True, glyphs, subrs=None, encoding="StandardEncoding",
            matrix="0.001 0 0 0.001 0 0", bbox="-50 -250 1000 900", len_iv=4,
            num_subrs=None, private_extra="", font_info_extra="", fontdict_extra=""):
    info = ["/version (001.000) readonly def"]
    if full is not None:
        info.append("/FullName (%s) readonly def" % full)
    if family is not None:
        info.append("/FamilyName (%s) readonly def" % family)
    if weight is not None:
        info.append("/Weight (%s) readonly def" % weight)
    info += ["/ItalicAngle 0 def", "/isFixedPitch false def", font_info_extra]
    clear = "\n".join([
        "%!PS-AdobeFont-1.0: " + font_name + " 001.000",
        "12 dict begin",
        "/FontInfo 10 dict dup begin",
        *info,
        "end readonly def",
        "/FontName /%s def" % font_name,
        "/Encoding %s def" % encoding if isinstance(encoding, str) else encoding[0],
        "/PaintType 0 def",
        "/FontType 1 def",
        "/FontMatrix [%s] readonly def" % matrix,
        "/FontBBox {%s} readonly def" % bbox,
        fontdict_extra,
        "currentdict end",
        "currentfile eexec",
        "",
    ])
    iv = bytes(len_iv) if len_iv > 0 else b""

    def enc(cs):
        return t1_encrypt(cs, 4330, iv) if len_iv >= 0 else cs

    subrs = T1_STD_SUBRS if subrs is None else subrs
    if isinstance(subrs, list):
        subrs = dict(enumerate(subrs))
    priv = bytearray()
    priv += ("dup /Private 12 dict dup begin\n"
             "/RD{string currentfile exch readstring pop}executeonly def\n"
             "/ND{noaccess def}executeonly def\n"
             "/NP{noaccess put}executeonly def\n"
             "/BlueValues [-20 0 700 720] def\n/OtherBlues [-250 -240] def\n"
             "/StdHW [30] def\n/StdVW [40] def\n/StemSnapH [30 50] def\n"
             "/MinFeature {16 16} def\n/password 5839 def\n").encode()
    if len_iv != 4:
        priv += b"/lenIV %d def\n" % len_iv
    priv += private_extra.encode()
    priv += b"/Subrs %d array\n" % (num_subrs if num_subrs is not None else len(subrs))
    for i, cs in sorted(subrs.items()):
        e = enc(cs)
        priv += b"dup %d %d RD " % (i, len(e)) + e + b" NP\n"
    priv += b"ND\n2 index /CharStrings %d dict dup begin\n" % len(glyphs)
    for name, cs in glyphs:
        e = enc(cs)
        priv += b"/%s %d RD " % (name.encode(), len(e)) + e + b" ND\n"
    priv += (b"end\nend\nreadonly put\nnoaccess put\n"
             b"dup /FontName get exch definefont pop\nmark currentfile closefile\n")
    eexec = t1_encrypt(bytes(priv), 55665, b"\x00\x00\x00\x00")
    trailer = ("0" * 64 + "\n") * 8 + "cleartomark\n"
    with open(path, "wb") as fh:
        if pfb:
            for kind, seg in ((1, clear.encode()), (2, eexec), (1, trailer.encode())):
                fh.write(bytes([0x80, kind]) + struct.pack("<I", len(seg)) + seg)
            fh.write(b"\x80\x03")
        else:
            fh.write(clear.encode())
            if hex_eexec:
                h = eexec.hex()
                fh.write("\n".join(h[i:i + 64] for i in range(0, len(h), 64)).encode())
                fh.write(b"\n")
            else:
                fh.write(eexec + b"\n")
            fh.write(trailer.encode())


T1_GLYPHS = [
    (".notdef", t1_cs(0, 500, "hsbw", "endchar")),
    ("A", t1_glyph_a(0)),
    ("B", t1_glyph_flex()),
    ("C", t1_glyph_hints()),
    ("D", t1_glyph_sbw()),
    ("E", t1_glyph_a(7)),
    ("Aacute", t1_glyph_seac()),
    ("acute", t1_glyph_acute()),
    ("uni0416", t1_glyph_a(3)),
    ("Delta", t1_glyph_flex()),
    ("space", t1_cs(0, 250, "hsbw", "endchar")),
]
# Subrs 5 and 6 are hint sets for hint replacement
T1_SUBRS = T1_STD_SUBRS + [t1_cs(0, 25, "hstem", 50, 40, "vstem", "return"),
                           t1_cs(10, 25, "hstem", 480, 40, "vstem", "return")]

t1_font(os.path.join(out, "t1test.pfa"), family="T1 Test", full="T1 Test Bold",
        weight="Bold", glyphs=T1_GLYPHS, subrs=T1_SUBRS)
# PFB, a 2048-unit skewed font matrix with an offset, and a style from
# the weight
t1_font(os.path.join(out, "t1test.pfb"), family="T1 Pfb", full="T1 Pfb", weight="Light",
        font_name="T1Pfb", pfb=True, glyphs=T1_GLYPHS, subrs=T1_SUBRS,
        matrix="0.00048828125 0 0.0001 0.00048828125 0.01 0.005",
        bbox="-100 -500 2000 1800")
# a binary eexec section in a PFA, no FamilyName (the FontName is the
# family), .notdef not first, unencrypted charstrings, and a custom
# encoding array
enc_arr = ("/Encoding 256 array\n0 1 255 {1 index exch /.notdef put} for\n"
           "dup 65 /A put\ndup 66 /B put\ndup 67 /C put\nreadonly def",)
t1_font(os.path.join(out, "t1bin.pfa"), family=None, full="Binary Eexec Italic",
        font_name="T1Binary", hex_eexec=False, len_iv=-1, encoding=enc_arr,
        glyphs=[T1_GLYPHS[1], T1_GLYPHS[0]] + T1_GLYPHS[2:], subrs=T1_SUBRS)
# no .notdef at all (one is made), and a sparse Subrs array (indices
# hashed), with the style from the full name
sparse = dict(enumerate(T1_SUBRS))
sparse[2000] = t1_cs(300, 0, "rlineto", 0, 300, "rlineto", "return")
sparse[1500] = t1_cs(200, 100, "rlineto", -100, 200, "rlineto", "return")
t1_font(os.path.join(out, "t1sparse.pfb"), family="T1 Sparse", full="T1 Sparse Semi Bold",
        font_name="T1Sparse", pfb=True, num_subrs=3000, subrs=sparse,
        glyphs=[("A", t1_glyph_subr(2000)), ("B", t1_glyph_subr(1500))] + T1_GLYPHS[2:])
# a multiple-master font: two designs on one axis, a blended FontBBox
mm_dict = ("/BlendDesignPositions [[0] [1]] def\n"
           "/BlendDesignMap [[[100 0] [900 1]]] def\n"
           "/BlendAxisTypes [/Weight] def\n")
t1_font(os.path.join(out, "t1mm.pfa"), family="T1 MM", full="T1 MM", font_name="T1MM",
        glyphs=[T1_GLYPHS[0], ("A", t1_glyph_mm()), ("B", t1_glyph_a(2))] + T1_GLYPHS[3:],
        subrs=T1_SUBRS, font_info_extra=mm_dict,
        fontdict_extra="/WeightVector [0.3 0.7] def\n"
                       "/Blend 3 dict dup begin\n"
                       "/FontBBox {{-60 -40} {-300 -200} {1000 1100} {950 850}} def\n"
                       "end def")


# Type 42 fonts: TrueType fonts in a PostScript font dictionary, whose
# CharStrings map glyph names (and through them Unicode) to the
# TrueType glyphs.
import io


def sfnt_bytes(font):
    buf = io.BytesIO()
    font.save(buf)
    return buf.getvalue()


def sfnt_pieces(data):
    """The SFNT split into strings at table boundaries (as the Type 42
    spec asks), each padded to an odd length with a zero byte."""
    num_tables = struct.unpack(">H", data[4:6])[0]
    cuts = sorted({struct.unpack(">I", data[12 + 16 * i + 8:12 + 16 * i + 12])[0]
                   for i in range(num_tables)} | {len(data)})
    pieces, start = [], 0
    for c in cuts:
        if c > start:
            pieces.append(data[start:c])
            start = c
    return [p + b"\0" if len(p) % 2 == 0 else p for p in pieces]


def t42_font(path, sfnt, charstrings, *, family="T42 Test", full=None, font_name="T42Test",
             encoding="StandardEncoding", binary=False, lilypond=False,
             matrix="1 0 0 1 0 0", font_type=42, extra=""):
    info = ["/version (001.000) readonly def"]
    if full is not None:
        info.append("/FullName (%s) readonly def" % full)
    if family is not None:
        info.append("/FamilyName (%s) readonly def" % family)
    info += ["/ItalicAngle 0 def", "/isFixedPitch false def",
             "/UnderlinePosition -100 def", "/UnderlineThickness 50 def"]
    out = bytearray()
    out += ("%!PS-TrueTypeFont-65536-65536-1\n"
            "%%Creator: gen-ttf-testfonts.py\n"
            "11 dict begin\n"
            "/FontName /" + font_name + " def\n"
            "/FontType %d def\n" % font_type +
            "/FontMatrix [" + matrix + "] def\n"
            "/FontBBox [0 -200 1000 900] def\n"
            "/PaintType 0 def\n"
            "/FontInfo 10 dict dup begin\n" + "\n".join(info) + "\nend readonly def\n"
            + extra).encode()
    out += (encoding if encoding.startswith("/Encoding") else
            "/Encoding " + encoding + " def\n").encode()
    out += b"/sfnts ["
    for piece in sfnt_pieces(sfnt):
        if binary:
            out += b"%d RD " % len(piece) + piece + b"\n"
        else:
            h = piece.hex().upper()
            out += b"<\n" + "\n".join(h[i:i + 72] for i in range(0, len(h), 72)).encode() + b"\n>\n"
    out += b"] def\n"
    if lilypond:
        out += b"/CharStrings %d dict dup begin\n" % len(charstrings)
        for name, gid in charstrings:
            out += b"(%s) cvn %d def\n" % (name.encode(), gid)
        out += b"end readonly def\n"
    else:
        out += b"/CharStrings << "
        for name, gid in charstrings:
            out += b"/%s %d\n" % (name.encode(), gid)
        out += b">> def\n"
    out += b"FontName currentdict end definefont pop\n"
    with open(path, "wb") as fh:
        fh.write(bytes(out))


t42_base = tt_font(tt_order, "T42 Inner", [subtable(4, 3, 1, {0x41: "A"})],
                   kern={("A", "B"): -100, ("B", "A"): 60})
t42_sfnt = sfnt_bytes(t42_base)
tt_gid = {g: i for i, g in enumerate(tt_order)}
# Every character of synth-strings.txt has a glyph: for a missing one,
# ttf.cpp loads the glyph whose index is the code point, and FreeType's
# Type 42 driver reads past its CharStrings for it.
T42_NAMES = ["space"] + list("ABCDEFGHIJ") + ["acute", "Aacute", "uni0394", "uni0416",
                                              "u1F600"]


def t42_charstrings(names, first=(".notdef",)):
    return [(n, 0) for n in first] + [
        (n, (7 * i + 3) % (len(tt_order) - 1) + 1) for i, n in enumerate(names)]


# hex strings, a `<< >>' CharStrings dictionary whose names differ from
# the TrueType glyphs', the style from the full name, and the kern
# table unused (the driver has no kerning)
t42_font(os.path.join(out, "t42test.t42"), t42_sfnt, t42_charstrings(T42_NAMES),
         full="T42 Test Bold")
# binary strings after RD, LilyPond's `(name) cvn' CharStrings with
# .notdef not first (FreeType swaps it to 0), a custom encoding array,
# a scaled font matrix, no FamilyName (the FontName is the family), and
# a FontDirectory check
cs = t42_charstrings(T42_NAMES[::-1], first=())
t42_font(os.path.join(out, "t42bin.t42"), t42_sfnt, cs[:5] + [(".notdef", 0)] + cs[5:],
         family=None, full="Whatever", font_name="T42Binary", binary=True, lilypond=True,
         encoding="/Encoding 256 array\n0 1 255 {1 index exch /.notdef put} for\n"
                  "dup 65 /A put\ndup 66 /B put\ndup 67 /C put\nreadonly def\n",
         matrix="2 0 0.5 2 10 20",
         extra="FontDirectory /T42Binary known {/T42Binary findfont dup /UniqueID known"
               " {dup /UniqueID get 4242 eq exch /FontType get 42 eq and} {pop false}"
               " ifelse {save true} {false} ifelse} {false} ifelse\n")
# a tricky TrueType font inside (its glyphs are hinted), in an
# ISOLatin1Encoding font
tricky = sfnt_bytes(tricky_font("DFKaiShu T42"))
t42_font(os.path.join(out, "t42tricky.t42"), tricky,
         [(g, i) for i, g in enumerate(TRICKY_ORDER)] +
         [(n, 1 + i % 9) for i, n in enumerate(["acute", "uni0394", "uni0416", "u1F600"])],
         family="T42 Tricky", full="T42 Tricky", encoding="ISOLatin1Encoding")
# not a Type 42 font (FontType 3): no driver takes it
t42_font(os.path.join(out, "t42type3.t42"), t42_sfnt, [(".notdef", 0), ("A", 1)],
         family="T42 Type3", font_type=3)


# PFR fonts (Bitstream's Portable Font Resources), assembled here: glyph
# programs with every instruction and argument form, compound glyphs,
# kerning items, and the header, logical and physical font variants.

def u8(v): return struct.pack(">B", v & 0xFF)
def s8(v): return struct.pack(">b", v)
def u16(v): return struct.pack(">H", v & 0xFFFF)
def s16(v): return struct.pack(">h", v)
def u24(v): return struct.pack(">I", v & 0xFFFFFF)[1:]


# glyph program arguments: (kind, value) for x and y; kinds are
# 0 (control index), 1 (absolute), 2 (delta from the last point), 3 (same)
def pfr_args(*coords):
    fmt, data = 0, b""
    for i, (kind, val) in enumerate(coords):
        fmt |= kind << (2 * i)
        data += {0: u8, 1: s16, 2: s8}.get(kind, lambda v: b"")(val)
    return fmt, data


def pfr_point(x, y):
    return pfr_args(x, y)


def MOVE(x, y, inside=False):
    fmt, data = pfr_point(x, y)
    return u8((0x40 if inside else 0x50) | fmt) + data


def LINE(x, y):
    fmt, data = pfr_point(x, y)
    return u8(0x10 | fmt) + data


def HLINE(i): return u8(0x20 | i)
def VLINE(i): return u8(0x30 | i)


def CURVE(x1, y1, x2, y2, x3, y3):
    f1, d1 = pfr_point(x1, y1)
    f2, d2 = pfr_point(x2, y2)
    f3, d3 = pfr_point(x3, y3)
    return u8(0x80 | f1) + d1 + u8(f2 | f3 << 4) + d2 + d3


def HVCURVE(dx1, xi2, dy2, dy3):
    # formats 0xE, 0x8, 0xB: (dx, same), (index, dy), (same, dy)
    return u8(0x60) + s8(dx1) + u8(xi2) + s8(dy2) + s8(dy3)


def VHCURVE(dy1, dx2, yi2, dx3):
    # formats 0xB, 0x2, 0xE: (same, dy), (dx, index), (dx, same)
    return u8(0x70) + s8(dy1) + s8(dx2) + u8(yi2) + s8(dx3)


END = b"\x00"


def pfr_simple(xs, ys, code, *, one_byte_counts=True, extra=None):
    flags = 0
    if one_byte_counts and len(xs) < 16 and len(ys) < 16:
        flags |= 0x04
        counts = u8(len(xs) | len(ys) << 4)
    else:
        counts = b""
        if xs:
            flags |= 0x02
            counts += u8(len(xs))
        if ys:
            flags |= 0x01
            counts += u8(len(ys))
    ctrl, prev = b"", 0
    vals = list(xs) + list(ys)
    for i in range(0, len(vals), 8):
        mask, body = 0, b""
        for j, v in enumerate(vals[i:i + 8]):
            if 0 <= v - prev <= 255 and (i + j) % 3 != 2:
                body += u8(v - prev)
            else:
                mask |= 1 << j
                body += s16(v)
            prev = v
        ctrl += u8(mask) + body
    if extra is not None:
        flags |= 0x08
    return u8(flags) + counts + ctrl + (extra if extra is not None else b"") + code + END


def pfr_compound(parts, extra=None):
    """parts: (gps_offset, gps_size, dx, dy, xscale, yscale); scales are
    in 1/4096."""
    out = bytearray(u8(0x80 | len(parts) | (0x40 if extra is not None else 0)))
    if extra is not None:
        out += extra
    for off, size, dx, dy, xs, ys in parts:
        fmt, body = 0, b""
        if xs is not None:
            fmt |= 0x10
            body += s16(xs)
        if ys is not None:
            fmt |= 0x20
            body += s16(ys)
        for shift, d in ((0, dx), (2, dy)):
            if d == 0:
                continue
            if -128 <= d <= 127:
                fmt |= 2 << shift
                body += s8(d)
            else:
                fmt |= 1 << shift
                body += s16(d)
        if size > 255:
            fmt |= 0x40
            body += u16(size)
        else:
            body += u8(size)
        if off > 0xFFFF:
            fmt |= 0x80
            body += u24(off)
        else:
            body += u16(off)
        out += u8(fmt) + body
    return bytes(out)


def extra_items(items):
    return u8(len(items)) + b"".join(u8(len(d)) + u8(t) + d for t, d in items)


def aux_record(typ, data):
    if len(data) % 2:
        data += b"\0"
    return u16(len(data) + 4) + u16(typ) + data


# the glyph programs: a box with a counter; curves of each kind; a glyph
# whose second contour ends on the first contour's last point (FreeType
# drops that point); an accent
PFR_BOX = pfr_simple(
    [40, 60, 440, 460, 150, 350], [0, 20, 680, 700, 200, 450],
    MOVE((0, 0), (0, 0)) + HLINE(3) + VLINE(3) + HLINE(0) + LINE((3, 0), (0, 0))
    + MOVE((0, 4), (0, 4), inside=True) + VLINE(5) + HLINE(5) + VLINE(4))
PFR_CURVES = pfr_simple(
    [50, 250, 450], [0, 350, 700],
    MOVE((0, 0), (0, 0)) + HVCURVE(80, 1, 60, 90) + VHCURVE(70, 40, 2, 60)
    + CURVE((1, 420), (2, -30), (0, 2), (2, 50), (2, -60), (1, 300))
    + CURVE((3, 0), (2, -80), (2, -100), (3, 0), (0, 0), (0, 1))
    + LINE((0, 0), (0, 0)))
PFR_QUIRK = pfr_simple(
    [0, 100, 300, 400], [0, 100, 500, 600],
    MOVE((0, 0), (0, 0)) + LINE((0, 3), (0, 0)) + LINE((0, 3), (0, 3))
    + LINE((0, 0), (0, 3))
    + MOVE((0, 1), (0, 1)) + LINE((0, 2), (0, 1)) + LINE((0, 0), (0, 3)),
    one_byte_counts=False)
PFR_ACUTE = pfr_simple(
    [150, 250, 300], [750, 900],
    MOVE((0, 0), (0, 0)) + LINE((0, 1), (0, 0)) + LINE((0, 2), (0, 1)) + LINE((2, -60), (3, 0)),
    extra=extra_items([(7, b"\x01\x02\x03")]))
PFR_NARROW = pfr_simple(
    [30, 230], [-100, 500],
    MOVE((0, 0), (1, -150)) + LINE((0, 1), (1, -150)) + LINE((0, 1), (0, 1))
    + CURVE((2, -20), (2, 40), (2, -80), (2, 10), (0, 0), (1, 400)))


def pfr_font(path, glyphs, *, family="PFR Test", style="Regular", font_id="PFR Test Font v1",
             outline_res=1000, metrics_res=1000, two_byte_codes=False, proportional=True,
             ascii=False, gps_2byte=False, gps_3byte=False, kern=None, kern_2byte=False,
             kern_2byte_adj=False, bbox=(0, -200, 1000, 900), vertical=False,
             log_flags=0, size_high=False, strikes=0, stem_snaps=True, ascent=None,
             sort=True, header_version=4):
    """glyphs: (char code, advance, program or index into the programs
    so far, or a compound given as parts with glyph indices)."""
    # the glyph program section
    gps = bytearray(b"\0" * 4)  # offset 0 means no outline
    where = []
    for code, adv, prog in glyphs:
        if prog is None:
            where.append((0, 0))
            continue
        if isinstance(prog, int):
            where.append(where[prog])
            continue
        if isinstance(prog, list):
            prog = pfr_compound([(where[g][0], where[g][1], dx, dy, xs, ys)
                                 for g, dx, dy, xs, ys in prog])
        where.append((len(gps), len(prog)))
        gps += prog
    if gps_3byte:
        gps = bytearray(b"\0" * 0x10000) + gps
        where = [(o + 0x10000 if o else 0, s) for o, s in where]

    flags = 0
    if vertical:
        flags |= 0x01
    if two_byte_codes:
        flags |= 0x02
    if proportional:
        flags |= 0x04
    if ascii:
        flags |= 0x08
    if gps_2byte:
        flags |= 0x10
    if gps_3byte:
        flags |= 0x20
    items = []
    if strikes:
        items.append((1, u24(0) + u8(0) + u8(strikes) + b"".join(
            u8(10 + i) + u8(10 + i) + u8(0) + u16(0) + u16(0) + u8(0) for i in range(strikes))))
    if font_id is not None:
        items.append((2, font_id.encode()))
    if stem_snaps:
        items.append((3, u8(0x21) + s16(40) + s16(60) + s16(90)))
    if kern:
        for pairs in kern:
            body = u8(len(pairs)) + s16(-20) + u8((1 if kern_2byte else 0) |
                                                  (2 if kern_2byte_adj else 0))
            for (a, b), v in sorted(pairs.items()):
                body += (u16(a) + u16(b)) if kern_2byte else (u8(a) + u8(b))
                body += s16(v) if kern_2byte_adj else u8(v)
            items.append((4, body))
    if items:
        flags |= 0x80
    phy = bytearray(u16(1) + u16(outline_res) + u16(metrics_res) +
                    b"".join(s16(v) for v in bbox) + u8(flags))
    if not proportional:
        phy += s16(glyphs[0][1])
    if items:
        phy += extra_items(items)
    aux = b""
    if family is not None:
        aux += aux_record(1, family.encode())
    if ascent is not None:
        aux += aux_record(2, bytes(10) + s16(ascent[0]) + s16(ascent[1]) + s16(0) + bytes(16))
    if style is not None:
        aux += aux_record(3, style.encode())
    phy += u24(len(aux)) + aux
    phy += u8(2) + s16(-10) + s16(0) + u8(1) + u8(20) + u16(40) + u16(30)
    chars = sorted(zip(glyphs, where), key=lambda g: g[0][0]) if sort else list(zip(glyphs, where))
    phy += u16(len(chars))
    for (code, adv, _), (off, size) in chars:
        phy += u16(code) if two_byte_codes else u8(code)
        if proportional:
            phy += s16(adv)
        if ascii:
            phy += u8(code & 0x7F)
        phy += u16(size) if gps_2byte else u8(size)
        phy += u24(off) if gps_3byte else u16(off)

    # the logical font
    log = bytearray(b"".join(s24 for s24 in
                             (struct.pack(">i", v)[1:] for v in (1000, 0, 0, 1000))))
    log += u8(log_flags)
    if log_flags & 0x04:
        log += (s16(12) if log_flags & 0x08 else u8(12))
        if log_flags & 0x03 == 0:
            log += u24(0x20000)
    if log_flags & 0x10:
        log += (s16(7) if log_flags & 0x20 else u8(7))
    if log_flags & 0x40:
        log += extra_items([(9, b"xyz")])

    log_dir_off = 58
    log_off = log_dir_off + 2 + 5
    log_size = len(log) + 5 + (1 if size_high else 0)
    phy_off = log_off + log_size
    gps_off = phy_off + len(phy)
    log += u16(len(phy)) + u24(phy_off)
    if size_high:
        log += u8(0)
    header = (b"PFR0" + u16(header_version) + u16(0x0D0A) + u16(58) + u16(7) + u16(log_dir_off)
              + u16(log_size) + u24(log_size) + u24(log_off)
              + u16(len(phy)) + u24(len(phy)) + u24(phy_off)
              + u16(max(s for _, s in where)) + u24(len(gps)) + u24(gps_off)
              + u8(2) + u8(15) + u8(15) + u8(1 if size_high else 0) + u8(0)
              + u24(0) + u24(0) + u24(0) + u16(1) + u8(1) + u8(2) + u16(len(glyphs)))
    assert len(header) == 58
    data = header + u16(1) + u16(log_size) + u24(log_off) + bytes(log) + bytes(phy) + bytes(gps)
    data += b"$PFR" + u16(0) + u16(0)
    with open(path, "wb") as fh:
        fh.write(data)


# glyph programs by index: 0 box, 1 curves, 2 quirk, 3 acute, 4 narrow
PFR_PROGS = [PFR_BOX, PFR_CURVES, PFR_QUIRK, PFR_ACUTE, PFR_NARROW]


def pfr_glyphs(codes):
    """The test glyphs for the characters of synth-strings.txt (codes maps
    their code points to the codes the font has)."""
    g = [(codes(0x20), 250, PFR_NARROW)]
    g += [(codes(0x41), 520, PFR_BOX), (codes(0x42), 540, PFR_CURVES),
          (codes(0x43), 500, PFR_QUIRK), (codes(0xB4), 300, PFR_ACUTE),
          (codes(0x44), 480, PFR_NARROW)]
    # E-J: the programs again, as the same offsets
    g += [(codes(c), 470 + 9 * i, i % 5) for i, c in enumerate(range(0x45, 0x4B))]
    # Á: A with the acute raised and scaled
    g += [(codes(0xC1), 520, [(1, 0, 0, None, None), (4, 120, 200, 3072, 5120)])]
    return g


# one-byte codes; kerning (one-byte pairs and adjustments); metrics in
# other units than outlines; stroke and bold flags and extra items in
# the logical font
pfr_font(os.path.join(out, "pfrtest.pfr"), pfr_glyphs(lambda c: c),
         metrics_res=2048, log_flags=0x04 | 0x10 | 0x40,
         kern=[{(0x41, 0x42): 30, (0x42, 0x41): 10, (0x41, 0x41): 200, (0x42, 0x43): 5}])
# two-byte codes (the Cyrillic and Greek letters have glyphs, the emoji
# has none), a monospaced font with ASCII codes, two-byte GPS sizes and
# three-byte offsets, two kerning items with two-byte characters and
# adjustments (ten pairs: A-Á is found by the binary search's first
# probe, of the pairs past the largest power of two), the
# physical font size's high byte, and no family name (the font ID is)
wide = pfr_glyphs(lambda c: c) + [(0x394, 610, [(1, 50, 0, 2048, 4096)]), (0x416, 600, 0)]
pfr_font(os.path.join(out, "pfrwide.pfr"), wide, family=None, style="Book",
         font_id="PFR Wide ID", outline_res=2048, two_byte_codes=True, proportional=False,
         ascii=True, gps_2byte=True, gps_3byte=True, kern_2byte=True, kern_2byte_adj=True,
         kern=[{(0x41, c): -40 - (c & 0xFF) for c in list(range(0x41, 0x49)) + [0xC1, 0x394]},
               {(0x42, 0x41): 70, (0x394, 0x416): -300, (0x416, 0x394): 150}],
         size_high=True, bbox=(-50, -300, 1100, 1800), ascent=(1500, -250),
         header_version=0)
# a vertical font (no horizontal advances)
pfr_font(os.path.join(out, "pfrvert.pfr"), pfr_glyphs(lambda c: c), family="PFR Vertical",
         vertical=True, stem_snaps=False)
# characters out of order: the charmap fails, and with it the font
pfr_font(os.path.join(out, "pfrunsorted.pfr"), pfr_glyphs(lambda c: c), family="PFR Unsorted",
         sort=False)
# bitmaps only: not scalable, so the size request fails
pfr_font(os.path.join(out, "pfrbitmap.pfr"), [(c, 500, None) for c in range(0x41, 0x44)],
         family="PFR Bitmap", strikes=2)
