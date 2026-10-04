# Writes the STEP fixtures of offset surfaces and of parabolas and
# hyperbolas (roadmap package 27.5), testdata/step/offset, with FreeCAD
# (OpenCASCADE's STEP writer), and their volumes and areas in
# expected.txt, in the format of testdata/step/shapes/expected.txt. Run
# with FreeCAD's command line:
#
#     FreeCADCmd scripts/step-fixtures-offset.py
#
# OpenCASCADE writes an offset of a plane, cylinder or sphere as the
# surface it is; those fixtures are written as the plain shape and their
# surfaces then rewritten as OFFSET_SURFACEs of a smaller one, which is
# the same surface. A volume measured by OpenCASCADE is marked m.
import math
import os
import re

import FreeCAD
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "step", "offset")
out = os.path.normpath(out)
os.makedirs(out, exist_ok=True)
V = Base.Vector

shapes = []  # name, shape, volume, measured, rewrite


def add(name, shape, volume, rewrite=None):
    measured = volume is None
    shapes.append((name, shape, shape.Volume if measured else volume, measured, rewrite))


def offset_of(kind, smaller):
    """Rewrites each surface of a kind, of radius r, as an offset by r -
    smaller of one of radius smaller, about the same placement, the
    offset's sense away from the axis or centre."""
    def rewrite(text):
        new = []
        nxt = [90000]

        def sub(m):
            ident, place, r = m.group(1), m.group(2), float(m.group(3))
            nxt[0] += 1
            new.append("#%d = %s('',%s,%s);" % (nxt[0], kind, place, repr(smaller)))
            return "%s = OFFSET_SURFACE('',#%d,%s,.F.);" % (ident, nxt[0], repr(r - smaller))
        text = re.sub(r"(#\d+) = %s\('',(#\d+),([0-9.Ee+-]+)\);" % kind, sub, text)
        return text.replace("ENDSEC;\nEND-ISO", "\n".join(new) + "\nENDSEC;\nEND-ISO", 1)
    return rewrite


# A cylinder whose side is an offset of a narrower one by 2.
add("cylinder_offset", Part.makeCylinder(7, 10), math.pi * 49 * 10, offset_of("CYLINDRICAL_SURFACE", 5.0))

# A sphere that is an offset of a smaller one by 3. (OpenCASCADE reads
# it back as no solid; the importer reads it.)
add("sphere_offset", Part.makeSphere(7), 4.0 / 3 * math.pi * 343, offset_of("SPHERICAL_SURFACE", 4.0))

# OpenCASCADE's offset of a B-spline solid rounds its edges with faces
# that miss the offset's edges by more than a file's tolerance, so the
# importer's offsets of other surfaces are tested in Go
# (TestOffsetCells), not here.

# A parabolic segment extruded: x = y^2 / 8 up to x = 9/8, y from -3 to
# 3, area 4.5, 4 high.
pb = Part.Parabola()
pb.Focal = 2.0
arc = Part.ArcOfParabola(pb, -3, 3).toShape()
seg = Part.Face(Part.Wire([arc, Part.LineSegment(arc.Vertexes[-1].Point, arc.Vertexes[0].Point).toShape()]))
add("parabola_extruded", seg.extrude(V(0, 0, 4)), 4.5 * 4)

# A hyperbolic segment extruded: x = 2 cosh t, y = sinh t for t from
# -1.2 to 1.2, closed by the line between its ends; area
# ab (sinh T cosh T - T), 3 high.
hb = Part.Hyperbola()
hb.MajorRadius, hb.MinorRadius = 2.0, 1.0
T = 1.2
arc = Part.ArcOfHyperbola(hb, -T, T).toShape()
seg = Part.Face(Part.Wire([arc, Part.LineSegment(arc.Vertexes[-1].Point, arc.Vertexes[0].Point).toShape()]))
add("hyperbola_extruded", seg.extrude(V(0, 0, 3)), 2 * 1 * (math.sinh(T) * math.cosh(T) - T) * 3)

with open(os.path.join(out, "expected.txt"), "w") as f:
    f.write("# name volume area (mm3, mm2): volumes worked out from the dimensions;\n")
    f.write("# a measured one is marked m. Written by scripts/step-fixtures-offset.py.\n")
    for name, shape, vol, measured, rewrite in shapes:
        path = os.path.join(out, name + ".step")
        shape.exportStep(path)
        if rewrite:
            with open(path) as g:
                text = g.read()
            with open(path, "w") as g:
                g.write(rewrite(text))
        f.write("%s %.9g %.9g%s\n" % (name, vol, shape.Area, " m" if measured else ""))
        print(name, vol, shape.Volume, shape.Area, shape.isValid())
