#!/usr/bin/env python3
"""Draw the toolbar icons of the features added after SolveSpace's set.

Each is 24 x 24 RGB, white line art on black, as the icons beside them
under pkg/res/data/icons/graphics-window. The lines are drawn on a grid
four times as fine and averaged, so their edges are smoothed as the
others' are. Only the standard library is used.

Usage:
    scripts/gen-toolbar-icons.py [directory]

The directory defaults to pkg/res/data/icons/graphics-window.
"""
import math
import os
import struct
import sys
import zlib

N = 24
S = 4  # samples per pixel along each axis


class Icon:
    def __init__(self):
        self.a = [[0.0] * (N * S) for _ in range(N * S)]

    def dot(self, x, y, w):
        # x, y in icon pixels; w the pen's width in pixels.
        r = w * S / 2
        cx, cy = x * S, y * S
        for j in range(int(cy - r - 1), int(cy + r + 2)):
            for i in range(int(cx - r - 1), int(cx + r + 2)):
                if 0 <= i < N * S and 0 <= j < N * S:
                    if (i + 0.5 - cx) ** 2 + (j + 0.5 - cy) ** 2 <= r * r:
                        self.a[j][i] = 1.0

    def line(self, x0, y0, x1, y1, w=1.5, dash=None):
        n = max(2, int(math.hypot(x1 - x0, y1 - y0) * S * 2))
        for k in range(n + 1):
            t = k / n
            if dash and int(t * math.hypot(x1 - x0, y1 - y0) / dash) % 2:
                continue
            self.dot(x0 + (x1 - x0) * t, y0 + (y1 - y0) * t, w)

    def poly(self, pts, w=1.5, closed=False):
        for p, q in zip(pts, pts[1:] + (pts[:1] if closed else [])):
            self.line(*p, *q, w=w)

    def arc(self, cx, cy, rx, ry, a0, a1, w=1.5):
        n = max(8, int(abs(a1 - a0) * max(rx, ry) * S / 20))
        pts = []
        for k in range(n + 1):
            a = math.radians(a0 + (a1 - a0) * k / n)
            pts.append((cx + rx * math.cos(a), cy - ry * math.sin(a)))
        self.poly(pts, w)

    def png(self):
        rows = b""
        for y in range(N):
            rows += b"\x00"
            for x in range(N):
                s = sum(self.a[y * S + j][x * S + i] for j in range(S) for i in range(S))
                v = int(round(255 * s / (S * S)))
                rows += bytes((v, v, v))
        def chunk(t, d):
            return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d) & 0xFFFFFFFF)
        return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", N, N, 8, 2, 0, 0, 0))
                + chunk(b"IDAT", zlib.compress(rows, 9)) + chunk(b"IEND", b""))


def fillet(c):
    # A block's corner, rounded.
    c.poly([(4, 20), (4, 12)])
    c.arc(12, 12, 8, 8, 180, 90)
    c.poly([(12, 4), (20, 4)])
    c.line(4, 20, 20, 20, w=1.2, dash=2)
    c.line(20, 4, 20, 20, w=1.2, dash=2)


def chamfer(c):
    # A block's corner, cut at a slant.
    c.poly([(4, 20), (4, 11), (11, 4), (20, 4)])
    c.line(4, 20, 20, 20, w=1.2, dash=2)
    c.line(20, 4, 20, 20, w=1.2, dash=2)


def shell(c):
    # A box hollowed to walls, open at the top.
    c.poly([(3, 6), (3, 21), (21, 21), (21, 6)])
    c.poly([(7, 6), (7, 17), (17, 17), (17, 6)])
    c.line(3, 6, 7, 6)
    c.line(17, 6, 21, 6)


def sweep(c):
    # A profile and the path it moves along.
    c.arc(5, 18, 3, 3, 0, 360)
    c.arc(5, 4, 14, 14, 270, 360)
    c.arc(19, 4, 3, 1.5, 0, 360, w=1.4)


def loft(c):
    # A large profile below joined to a small one above.
    c.arc(12, 19, 9, 2.5, 0, 360)
    c.arc(12, 5, 4, 1.5, 0, 360)
    c.line(3, 19, 8, 5)
    c.line(21, 19, 16, 5)


def mirror(c):
    # A shape and its reflection in a plane.
    c.line(12, 2, 12, 22, w=1.4, dash=2)
    c.poly([(9, 6), (3, 18), (9, 18)], closed=True)
    c.poly([(15, 6), (21, 18), (15, 18)], closed=True)


def pattern(c):
    # A shape repeated along a curve.
    c.arc(12, 22, 16, 16, 150, 30, w=1.2)
    for x, y in ((3.5, 12.5), (12, 6), (20.5, 12.5)):
        c.poly([(x - 2.5, y - 2.5), (x + 2.5, y - 2.5), (x + 2.5, y + 2.5), (x - 2.5, y + 2.5)], closed=True)


def measure(c):
    # A rule with marks.
    c.poly([(2, 9), (22, 9), (22, 16), (2, 16)], closed=True)
    for i, x in enumerate(range(5, 22, 3)):
        c.line(x, 9, x, 12 if i % 2 else 13.5, w=1.4)


def section(c):
    # A block cut open by a plane.
    c.poly([(3, 9), (9, 3), (21, 3), (21, 15), (15, 21), (3, 21)], closed=True)
    c.poly([(3, 9), (15, 9), (21, 3)])
    c.line(15, 9, 15, 21)
    for k in range(4):
        c.line(5 + 3 * k, 21, 5 + 3 * k + 5, 13, w=1.2)


ICONS = {
    "fillet": fillet, "chamfer": chamfer, "shell": shell, "sweep": sweep, "loft": loft,
    "mirror": mirror, "pattern": pattern, "measure": measure, "section": section,
}


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "..", "pkg", "res", "data", "icons", "graphics-window")
    for name, draw in ICONS.items():
        c = Icon()
        draw(c)
        with open(os.path.join(out, name + ".png"), "wb") as f:
            f.write(c.png())


if __name__ == "__main__":
    main()
