// Prints C++ render.cpp's camera projections, Bezier projection and
// ObjectPicker distances for a few cameras and points. See
// scripts/regen-render-golden.sh.
#include "solvespace.h"
using namespace SolveSpace;

static void P(const Vector &v) { printf(" %a %a %a", v.x, v.y, v.z); }

int main() {
    Camera cams[3] = {};
    cams[0].LoadIdentity();
    cams[0].width = 600; cams[0].height = 600; cams[0].pixelRatio = 1; cams[0].scale = 5;
    cams[1] = cams[0];
    cams[1].offset = Vector::From(-3.25, 7.5, 1.125);
    cams[1].projRight = Vector::From(0.8, 0.1, -0.3);
    cams[1].projUp = Vector::From(0.2, 0.9, 0.4);
    cams[1].NormalizeProjectionVectors();
    cams[1].scale = 2.75;
    cams[1].gridFit = true;
    cams[2] = cams[1];
    cams[2].tangent = 0.3 / 1e3;
    cams[2].gridFit = false;

    Vector pts[] = { Vector::From(0, 0, 0), Vector::From(10, -20, 30),
                     Vector::From(-7.5, 3.25, -12.125), Vector::From(123.4, 56.7, -89.1) };
    for(Camera &c : cams) {
        printf("camera");
        P(c.projRight); P(c.projUp);
        printf("\n");
        for(Vector p : pts) {
            double w;
            Vector r = c.ProjectPoint4(p, &w);
            Vector p3 = c.ProjectPoint3(p);
            printf("pt"); P(r); printf(" %a", w); P(p3);
            P(c.UnProjectPoint3(p3));
            P(c.UnProjectPoint(c.ProjectPoint(p)));
            P(c.AlignToPixelGrid(p));
            P(c.VectorFromProjs(p));
            printf("\n");
        }
        SBezier b = SBezier::From(pts[0], pts[1], pts[2], pts[3]);
        b.weight[1] = 0.75; b.weight[2] = 1.5;
        SBezier pb = c.ProjectBezier(b);
        printf("bezier");
        for(int i = 0; i <= pb.deg; i++) { P(pb.ctrl[i]); printf(" %a", pb.weight[i]); }
        printf("\n");

        ObjectPicker op = {};
        op.camera = c;
        op.selRadius = 10;
        Canvas::Stroke s = {};
        s.width = 3; s.zIndex = 2; s.unit = Canvas::Unit::PX;
        Canvas::hStroke hcs = op.GetStroke(s);
        Canvas::Fill f = {};
        f.zIndex = 1;
        Canvas::hFill hcf = op.GetFill(f);
        for(int i = 0; i < 6; i++) {
            op.point = Point2d::From(-40.0 + 17.5 * i, 25.0 - 11.25 * i);
            bool hit = op.Pick([&] {
                op.DrawLine(pts[0], pts[1], hcs);
                op.DrawPoint(pts[2], hcs);
                op.DrawQuad(pts[0], pts[1], pts[2], pts[3], hcf);
                op.DrawVectorText("abc", 2.0, pts[0], Vector::From(1, 0, 0), Vector::From(0, 1, 0), hcs);
            });
            printf("pick %d %a %a %d\n", hit, op.minDistance, op.minDepth, op.maxZIndex);
        }
    }
    return 0;
}
