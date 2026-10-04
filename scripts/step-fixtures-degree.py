# Writes the STEP fixtures of curves and surfaces of a degree above 5
# (roadmap package 27.2), testdata/step/degree, with FreeCAD
# (OpenCASCADE's STEP writer), and their volumes and areas in
# expected.txt, in the format of testdata/step/shapes/expected.txt. Run
# with FreeCAD's command line:
#
#     FreeCADCmd scripts/step-fixtures-degree.py
#
# The volumes are worked out from the control points, not measured.
import math
import os

import FreeCAD
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "step", "degree")
out = os.path.normpath(out)
os.makedirs(out, exist_ok=True)
V = Base.Vector

shapes = []


def add(name, shape, volume):
    shapes.append((name, shape, volume))


def solid_under(top):
    """The solid between a face over the rectangle and z = 0: planar
    sides down from each of its edges, and a floor."""
    faces = [top]
    floor = []
    for e in top.Edges:
        p, q = e.Vertexes[0].Point, e.Vertexes[-1].Point
        p0, q0 = V(p.x, p.y, 0), V(q.x, q.y, 0)
        w = Part.Wire([e, Part.LineSegment(q, q0).toShape(), Part.LineSegment(q0, p0).toShape(),
                       Part.LineSegment(p0, p).toShape()])
        faces.append(Part.Face(w))
        floor.append(Part.LineSegment(p0, q0).toShape())
    faces.append(Part.Face(Part.Wire(Part.__sortEdges__(floor))))
    return Part.Solid(Part.Shell(faces))


# A lid of degree 7 by 6 over a 40 x 30 rectangle: one Bezier patch, its
# control points evenly spaced in x and y, so x and y run linearly in its
# parameters, and the volume under it is the rectangle's area times the
# mean of the heights.
du, dv = 7, 6
heights = [[10 + 2 * math.sin(1.3 * i + 0.7 * j) * math.cos(0.4 * i * j) for j in range(dv + 1)] for i in range(du + 1)]
poles = [[V(40.0 * i / du, 30.0 * j / dv, heights[i][j]) for j in range(dv + 1)] for i in range(du + 1)]
lid = Part.BSplineSurface()
lid.buildFromPolesMultsKnots(poles, [du + 1, du + 1], [dv + 1, dv + 1], [0, 1], [0, 1], False, False, du, dv)
mean = sum(sum(r) for r in heights) / ((du + 1) * (dv + 1))
add("lid_degree7", solid_under(Part.Face(lid)), 40 * 30 * mean)

# A vase revolved from a profile whose side is one Bezier of degree 8:
# a surface of revolution of a high degree curve. Volume by the disc
# method, pi times the integral of x squared along z, on the exact
# polynomial.
side = [V(6, 0, 0), V(9, 0, 2), V(5, 0, 4), V(10, 0, 6), V(4, 0, 8), V(8, 0, 10), V(5, 0, 12), V(9, 0, 13),
        V(7, 0, 15)]
deg = len(side) - 1
bc = Part.BSplineCurve()
bc.buildFromPolesMultsKnots(side, [deg + 1, deg + 1], [0, 1], False, deg)
prof = Part.Wire([Part.LineSegment(V(0, 0, 0), side[0]).toShape(), bc.toShape(),
                  Part.LineSegment(side[-1], V(0, 0, 15)).toShape(),
                  Part.LineSegment(V(0, 0, 15), V(0, 0, 0)).toShape()])
vase = Part.Face(prof).revolve(V(0, 0, 0), V(0, 0, 1), 360)
if vase.ShapeType != "Solid":
    vase = Part.Solid(Part.Shell(vase.Faces))


def bern(n, i, t):
    return math.comb(n, i) * t ** i * (1 - t) ** (n - i)


def bez(pts, t):
    n = len(pts) - 1
    return sum(bern(n, i, t) * pts[i] for i in range(n + 1))


def dbez(pts, t):
    n = len(pts) - 1
    return n * sum(bern(n - 1, i, t) * (pts[i + 1] - pts[i]) for i in range(n))


# Gauss-Legendre on many pieces: the integrand is a polynomial of degree
# 3 * deg - 1, exact on each piece with enough points.
xs = [p.x for p in side]
zs = [p.z for p in side]
nodes, weights = [], []
import numpy
nodes, weights = numpy.polynomial.legendre.leggauss(20)
vol = 0.0
pieces = 64
for k in range(pieces):
    a, b = k / pieces, (k + 1) / pieces
    for x, w in zip(nodes, weights):
        t = (a + b) / 2 + (b - a) / 2 * x
        vol += w * (b - a) / 2 * bez(xs, t) ** 2 * dbez(zs, t)
add("vase_degree8", vase, math.pi * vol)

with open(os.path.join(out, "expected.txt"), "w") as f:
    f.write("# name volume area (mm3, mm2): volumes worked out from the control points.\n")
    f.write("# Written by scripts/step-fixtures-degree.py.\n")
    for name, shape, vol in shapes:
        path = os.path.join(out, name + ".step")
        shape.exportStep(path)
        f.write("%s %.9g %.9g\n" % (name, vol, shape.Area))
        print(name, vol, shape.Volume, shape.Area, shape.isValid())
