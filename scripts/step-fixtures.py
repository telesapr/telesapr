# Writes the STEP fixtures of pkg/persist/step's tests, testdata/step/shapes,
# with FreeCAD (OpenCASCADE's STEP writer), and their exact volumes and
# areas in expected.txt. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/step-fixtures.py
#
# The volumes are worked out from the shapes' dimensions, not measured,
# so the test checks the reader against geometry, not against
# OpenCASCADE.
import math
import os

import FreeCAD
import Import
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "step", "shapes")
out = os.path.normpath(out)
pi = math.pi
V = Base.Vector

shapes = []  # (name, shape, volume or None)


def add(name, shape, volume):
    shapes.append((name, shape, volume))


# A box: planes only.
add("box", Part.makeBox(10, 20, 30), 10 * 20 * 30)

# A box with a round hole through it: a cylinder with a seam.
hole = Part.makeCylinder(4, 30, V(10, 10, 0))
add("box_hole", Part.makeBox(20, 20, 30).cut(hole), 20 * 20 * 30 - pi * 16 * 30)

# A cylinder: two circles, one seam.
add("cylinder", Part.makeCylinder(5, 12), pi * 25 * 12)

# A sphere: a seam from pole to pole.
add("sphere", Part.makeSphere(7), 4.0 / 3 * pi * 343)

# Half a sphere, cut through its poles: a loop that passes both poles.
half = Part.makeSphere(7).common(Part.makeBox(20, 20, 20, V(0, -10, -10)))
add("sphere_half_poles", half, 2.0 / 3 * pi * 343)

# A hemisphere, cut at the equator: one loop round, closed at a pole.
dome = Part.makeSphere(7).common(Part.makeBox(20, 20, 20, V(-10, -10, 0)))
add("sphere_dome", dome, 2.0 / 3 * pi * 343)

# A cone to its apex.
add("cone_apex", Part.makeCone(6, 0, 9), pi * 36 * 9 / 3)

# A frustum.
add("cone_frustum", Part.makeCone(6, 3, 9), pi * 9 / 3 * (36 + 18 + 9))

# A torus.
add("torus", Part.makeTorus(10, 3), 2 * pi * pi * 10 * 9)

# A quarter of a torus, cut by two planes through its axis.
q = Part.makeTorus(10, 3).common(Part.makeBox(20, 20, 10, V(0, 0, -5)))
add("torus_quarter", q, 2 * pi * pi * 10 * 9 / 4)

# A revolved profile with a B-spline side: a vase. Its volume is
# measured, the B-spline making it hard to work out.
pts = [V(0, 0, 0), V(6, 0, 0), V(8, 0, 4), V(5, 0, 9), V(7, 0, 14), V(0, 0, 14)]
bs = Part.BSplineCurve()
bs.interpolate(pts[1:5])
prof = Part.Wire([Part.LineSegment(pts[0], pts[1]).toShape(), bs.toShape(),
                  Part.LineSegment(pts[4], pts[5]).toShape(),
                  Part.LineSegment(pts[5], pts[0]).toShape()])
vase = Part.Face(prof).revolve(V(0, 0, 0), V(0, 0, 1), 360)
add("vase_revolved", Part.Solid(Part.Shell(vase.Faces)) if vase.ShapeType != "Solid" else vase, None)

# An extruded B-spline outline: surfaces of linear extrusion.
bs2 = Part.BSplineCurve()
bs2.interpolate([V(0, 0, 0), V(5, 3, 0), V(10, 0, 0), V(12, 6, 0), V(8, 10, 0), V(2, 9, 0), V(0, 0, 0)], PeriodicFlag=False)
w = Part.Wire([bs2.toShape()])
if not w.isClosed():
    w = Part.Wire([bs2.toShape(), Part.LineSegment(bs2.EndPoint, bs2.StartPoint).toShape()])
blob = Part.Face(w).extrude(V(0, 0, 5))
# OpenCASCADE's volume of it is 0.1% short; the area inside the
# outline, by Green's theorem on many points of it, is better.
pts = bs2.discretize(200000)
green = sum((pts[i].x * pts[i + 1].y - pts[i + 1].x * pts[i].y) / 2 for i in range(len(pts) - 1))
add("blob_extruded", blob, abs(green) * 5)

# A box with a filleted edge: a cylinder patch between planes.
fb = Part.makeBox(20, 10, 10)
fe = [e for e in fb.Edges if abs(e.BoundBox.ZMin - 10) < 1e-9 and abs(e.BoundBox.YMin - 10) < 1e-9
      and abs(e.BoundBox.YMax - 10) < 1e-9]
fillet = fb.makeFillet(3, fe)
add("box_fillet", fillet, 20 * 10 * 10 - (9 - pi * 9 / 4) * 20)

# A sphere with a hole bored through it along the poles: a sphere face
# between two circles, no poles left.
bored = Part.makeSphere(7).cut(Part.makeCylinder(3, 20, V(0, 0, -10)))
h = math.sqrt(49 - 9)
add("sphere_bored", bored, 4.0 / 3 * pi * h ** 3)

with open(os.path.join(out, "expected.txt"), "w") as f:
    f.write("# name volume area (mm3, mm2): volumes worked out from the dimensions;\n")
    f.write("# a measured one is marked m. Written by scripts/step-fixtures.py.\n")
    for name, shape, vol in shapes:
        path = os.path.join(out, name + ".step")
        shape.exportStep(path)
        mark = ""
        if vol is None:
            vol, mark = shape.Volume, " m"
        f.write("%s %.9g %.9g%s\n" % (name, vol, shape.Area, mark))
        print(name, vol, shape.Volume, shape.Area)
