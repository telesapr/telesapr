#!/usr/bin/env python3
"""Write DXF files for TestImportDxfVsCpp into testdata/importdxf, made
with ezdxf so that each exercises part of the importer: every entity
kind SolveSpace reads, blocks and nested inserts, bulges, extrusions,
layers with colours, line types and weights, texts and dimensions, and
old versions with their code pages.

Usage: scripts/gen-dxf-import-fixtures.py
"""
import math
import os

import ezdxf

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "testdata", "importdxf")
os.makedirs(OUT, exist_ok=True)


def save(doc, name):
    doc.saveas(os.path.join(OUT, name))


# Lines, arcs, circles and points, on layers with colours, line types
# and weights, some by layer and some by entity. (No layer is off: C++
# reads an off layer's negative colour out of the palette's bounds.)
doc = ezdxf.new("R2010", setup=True)
msp = doc.modelspace()
doc.layers.add("red", color=1, linetype="DASHED", lineweight=35)
doc.layers.add("thin", color=5, lineweight=13)
msp.add_line((0, 0), (10, 0), dxfattribs={"layer": "red"})
msp.add_line((10, 0), (10, 10), dxfattribs={"layer": "thin", "color": 2})
msp.add_line((10, 10), (0, 10), dxfattribs={"lineweight": 50, "linetype": "DOT"})
msp.add_line((0, 10), (0, 0), dxfattribs={"layer": "thin"})
msp.add_line((2, 2), (7, 3.5), dxfattribs={"linetype": "DASHDOT"})
msp.add_arc((5, 5), 3, 30, 150, dxfattribs={"layer": "red"})
msp.add_arc((5, 5), 2, 200, 20)
msp.add_circle((15, 5), 2.5, dxfattribs={"color": 4})
msp.add_point((20, 1))
msp.add_point((10, 0))
save(doc, "basic.dxf")

# Light and old-style polylines, open and closed, with bulges.
doc = ezdxf.new("R2010")
msp = doc.modelspace()
msp.add_lwpolyline([(0, 0), (5, 0), (5, 5), (0, 5)], close=True)
msp.add_lwpolyline([(10, 0, 0, 0, 0.5), (15, 0, 0, 0, -1), (15, 5), (10, 5, 0, 0, 0.3)],
                   format="xyseb", close=True)
msp.add_lwpolyline([(20, 0, 0, 0, 2.0), (25, 0)], format="xyseb")
msp.add_polyline2d([(0, 10), (5, 12), (10, 10)])
pl = msp.add_polyline2d([(0, 20), (5, 20), (5, 25)], close=True)
pl.vertices[0].dxf.bulge = 0.4
msp.add_polyline3d([(0, 30, 0), (5, 30, 0), (5, 35, 0)])
save(doc, "polylines.dxf")

# Blocks: inserted scaled, rotated, mirrored and nested, with entities
# by block.
doc = ezdxf.new("R2010")
msp = doc.modelspace()
inner = doc.blocks.new("INNER")
inner.add_line((0, 0), (1, 0), dxfattribs={"color": 0})
inner.add_circle((0.5, 0.5), 0.25)
outer = doc.blocks.new("OUTER")
outer.add_arc((0, 0), 2, 0, 90)
outer.add_blockref("INNER", (1, 1), dxfattribs={"xscale": 2, "yscale": 2})
outer.add_lwpolyline([(0, 0), (3, 0), (3, 3)])
outer.add_text("in block", dxfattribs={"height": 0.5}).set_placement((0, -1))
msp.add_blockref("OUTER", (10, 10), dxfattribs={"rotation": 30, "color": 1})
msp.add_blockref("OUTER", (20, 0), dxfattribs={"xscale": -1, "yscale": 1.5})
msp.add_blockref("INNER", (0, 20), dxfattribs={"extrusion": (0, 0, -1)})
save(doc, "blocks.dxf")

# Texts and MTEXTs with every alignment, heights and angles.
doc = ezdxf.new("R2010")
msp = doc.modelspace()
aligns = ["LEFT", "CENTER", "RIGHT", "MIDDLE", "TOP_LEFT", "TOP_CENTER", "TOP_RIGHT",
          "MIDDLE_LEFT", "MIDDLE_CENTER", "MIDDLE_RIGHT", "BOTTOM_LEFT", "BOTTOM_CENTER",
          "BOTTOM_RIGHT"]
from ezdxf.enums import TextEntityAlignment
for i, a in enumerate(aligns):
    t = msp.add_text("text %s" % a, dxfattribs={"height": 1 + 0.25 * (i % 3),
                                                "rotation": 15 * (i % 4)})
    t.set_placement((0, 3 * i), align=TextEntityAlignment[a])
for i in range(1, 10):
    msp.add_mtext("mtext %d\\Pline two" % i,
                  dxfattribs={"insert": (30, 3 * i), "attachment_point": i, "char_height": 0.8,
                              "rotation": 10 * i})
msp.add_text("unicode \\U+00E9t\\U+00E9", dxfattribs={"height": 1}).set_placement((0, -5))
save(doc, "texts.dxf")

# Dimensions of every kind, with and without measurements.
doc = ezdxf.new("R2010", setup=True)
msp = doc.modelspace()
msp.add_linear_dim(base=(0, 3), p1=(0, 0), p2=(5, 0)).render()
msp.add_linear_dim(base=(8, 3), p1=(7, 0), p2=(12, 2), angle=30).render()
msp.add_aligned_dim(p1=(0, 10), p2=(5, 14), distance=1).render()
msp.add_radius_dim(center=(20, 5), radius=3, angle=45).render()
msp.add_diameter_dim(center=(30, 5), radius=2, angle=135).render()
msp.add_angular_dim_2l(base=(40, 8), line1=((38, 0), (45, 0)), line2=((38, 0), (43, 5))).render()
msp.add_angular_dim_3p(base=(52, 6), center=(50, 0), p1=(55, 0), p2=(50, 5)).render()
save(doc, "dimensions.dxf")

# Splines, of degree 3 and not.
doc = ezdxf.new("R2010")
msp = doc.modelspace()
msp.add_open_spline([(0, 0), (2, 3), (5, 3), (7, 0)], degree=3)
msp.add_open_spline([(10, 0), (12, 3), (15, 0)], degree=2)
msp.add_spline_control_frame([(20, 0), (22, 4), (26, 4), (28, 0), (30, 2)])
save(doc, "splines.dxf")

# Entities out of the xy plane, and extrusions.
doc = ezdxf.new("R2010")
msp = doc.modelspace()
msp.add_line((0, 0, 0), (5, 0, 3))
msp.add_circle((0, 0), 2, dxfattribs={"extrusion": (0, 1, 1)})
msp.add_arc((3, 3), 2, 0, 120, dxfattribs={"extrusion": (0, 0, -1)})
msp.add_arc((8, 3), 2, 0, 120, dxfattribs={"extrusion": (1, 0, 1)})
msp.add_lwpolyline([(0, 0), (4, 0), (4, 4)], dxfattribs={"extrusion": (0, 0, -1)})
save(doc, "extrusions.dxf")

# Only in the plane, but with a negative extrusion.
doc = ezdxf.new("R2010")
msp = doc.modelspace()
msp.add_arc((3, 3), 2, 0, 120, dxfattribs={"extrusion": (0, 0, -1)})
msp.add_circle((10, 0), 1, dxfattribs={"extrusion": (0, 0, -1)})
msp.add_lwpolyline([(0, 0, 0, 0, 0.5), (4, 0), (4, 4)], format="xyseb",
                   dxfattribs={"extrusion": (0, 0, -1)})
save(doc, "negative_extrusion.dxf")

# Old versions, with code pages for their texts.
for version, codepage, name in [("R12", "ANSI_1251", "r12_cp1251.dxf"),
                                ("R2000", "ANSI_1252", "r2000_cp1252.dxf")]:
    doc = ezdxf.new(version)
    doc.header["$DWGCODEPAGE"] = codepage
    msp = doc.modelspace()
    msp.add_line((0, 0), (5, 5))
    msp.add_text({"ANSI_1251": "Привет", "ANSI_1252": "café ü", "ANSI_932": "日本語"}[codepage],
                 dxfattribs={"height": 1}).set_placement((0, -2))
    msp.add_circle((3, 0), 1)
    save(doc, name)

# Double-byte code pages: ezdxf writes these as \U+ escapes, so the
# file's code page and text bytes are patched in afterwards.
for codepage, codec, text in [("ANSI_932", "cp932", "日本語ﾃｽﾄ"), ("ANSI_936", "gbk", "中文测试"),
                              ("ANSI_949", "cp949", "한국어"), ("ANSI_950", "big5", "繁體中文")]:
    doc = ezdxf.new("R2000")
    msp = doc.modelspace()
    msp.add_line((0, 0), (5, 5))
    msp.add_text("@@TEXT@@", dxfattribs={"height": 1}).set_placement((0, -2))
    path = os.path.join(OUT, "r2000_%s.dxf" % codec)
    doc.saveas(path)
    data = open(path, "rb").read()
    data = data.replace(b"$DWGCODEPAGE\n  3\nANSI_1252", ("$DWGCODEPAGE\n  3\n" + codepage).encode())
    assert codepage.encode() in data
    data = data.replace(b"@@TEXT@@", text.encode(codec))
    open(path, "wb").write(data)

# A binary DXF file.
doc = ezdxf.new("R2010")
msp = doc.modelspace()
msp.add_line((0, 0), (5, 0), dxfattribs={"color": 1, "lineweight": 25})
msp.add_arc((0, 0), 3, 10, 80)
msp.add_lwpolyline([(0, 5, 0, 0, 0.3), (5, 5), (5, 8)], format="xyseb", close=True)
msp.add_text("binary", dxfattribs={"height": 1}).set_placement((0, -2))
msp.add_aligned_dim(p1=(0, 10), p2=(5, 12), distance=1).render()
doc.saveas(os.path.join(OUT, "binary.dxf"), fmt="bin")
